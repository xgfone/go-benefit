package benefit

import (
	"context"
	"errors"
	"fmt"
	"slices"
)

// OperationContext contains optional application-defined, in-process facts.
// Callers, drivers, and constraint evaluators must treat the contained value
// as immutable and must not retain it beyond the operation call.
type OperationContext any

// Operation identifies a driver operation.
type Operation string

const (
	OperationInspect  Operation = "Inspect"
	OperationEvaluate Operation = "Evaluate"
	OperationRedeem   Operation = "Redeem"
	OperationReverse  Operation = "Reverse"
)

// OperationMode is an operation-specific supported mode.
type OperationMode string

const (
	OperationModeReverseFull    OperationMode = "full"
	OperationModeReversePartial OperationMode = "partial"
)

// OperationSupport describes one optional operation and its restrictions.
//
// A supported Reverse operation always supports full reversal.
// Partial reversal is supported only when Modes contains "partial".
type OperationSupport struct {
	Supported   bool            `json:"supported"`
	Operation   Operation       `json:"operation"`
	Constraints Constraints     `json:"constraints,omitempty"`
	Remark      string          `json:"remark,omitempty"`
	Modes       []OperationMode `json:"modes,omitempty"`
}

// OperationSupports is an ordered list of optional capability declarations.
// It must not contain the core Inspect, Evaluate, or Redeem operations.
type OperationSupports []OperationSupport

// Validate rejects empty and duplicate operations or duplicate modes.
func (supports OperationSupports) Validate() error {
	seen := make(map[Operation]struct{}, len(supports))
	for i, support := range supports {
		if support.Operation == "" {
			return fmt.Errorf("benefit: operation at index %d is empty", i)
		}
		if _, ok := seen[support.Operation]; ok {
			return fmt.Errorf("benefit: operation %q is duplicated", support.Operation)
		}
		if isCoreOperation(support.Operation) {
			return fmt.Errorf("benefit: core operation %q must not be declared", support.Operation)
		}
		seen[support.Operation] = struct{}{}

		modes := make(map[OperationMode]struct{}, len(support.Modes))
		for _, mode := range support.Modes {
			if mode == "" {
				return fmt.Errorf("benefit: operation %q has an empty mode", support.Operation)
			}
			if _, ok := modes[mode]; ok {
				return fmt.Errorf("benefit: operation %q mode %q is duplicated", support.Operation, mode)
			}
			if support.Operation == OperationReverse &&
				mode != OperationModeReverseFull &&
				mode != OperationModeReversePartial {
				return fmt.Errorf("benefit: reverse operation has unsupported mode %q", mode)
			}
			modes[mode] = struct{}{}
		}
	}
	return nil
}

// SupportsMode reports whether the operation supports mode.
//
// Full reversal is implicit for every supported Reverse operation.
func (support OperationSupport) SupportsMode(mode OperationMode) bool {
	if !support.Supported {
		return false
	}
	if support.Operation == OperationReverse && mode == OperationModeReverseFull {
		return true
	}
	return containsOperationMode(support.Modes, mode)
}

func isCoreOperation(operation Operation) bool {
	switch operation {
	case
		OperationInspect,
		OperationEvaluate,
		OperationRedeem:
		return true

	default:
		return false
	}
}

// Get returns the declaration for an operation.
func (supports OperationSupports) Get(operation Operation) (OperationSupport, bool) {
	for _, support := range supports {
		if support.Operation == operation {
			return support, true
		}
	}
	return OperationSupport{}, false
}

// EffectiveOperationSupports applies restrictions without expanding declared capabilities.
// A missing operation in a restriction list leaves the current declaration unchanged.
func EffectiveOperationSupports(declared OperationSupports, restrictions ...OperationSupports) (OperationSupports, error) {
	if err := declared.Validate(); err != nil {
		return nil, err
	}

	effective := cloneOperationSupports(declared)
	indices := make(map[Operation]int, len(effective))
	for i := range effective {
		indices[effective[i].Operation] = i
	}

	for _, list := range restrictions {
		if err := list.Validate(); err != nil {
			return nil, err
		}

		for _, restriction := range list {
			i, declaredOperation := indices[restriction.Operation]
			if !declaredOperation {
				if restriction.Supported {
					const msg = "benefit: restriction cannot enable undeclared operation %q"
					return nil, fmt.Errorf(msg, restriction.Operation)
				}
				continue
			}

			current := &effective[i]
			if !current.Supported {
				continue
			}

			if !restriction.Supported {
				current.Supported = false
				current.Modes = nil
				current.Remark = restriction.Remark
				current.Constraints = append(current.Constraints, restriction.Constraints...)
				continue
			}

			if current.Operation == OperationReverse {
				current.Modes = intersectReverseModes(current.Modes, restriction.Modes)
			} else if len(restriction.Modes) > 0 {
				if len(current.Modes) == 0 {
					current.Modes = append([]OperationMode(nil), restriction.Modes...)
				} else {
					current.Modes = intersectModes(current.Modes, restriction.Modes)
					if len(current.Modes) == 0 {
						current.Supported = false
					}
				}
			}

			current.Constraints = append(current.Constraints, restriction.Constraints...)
			if restriction.Remark != "" {
				current.Remark = restriction.Remark
			}
		}
	}

	return effective, nil
}

func cloneOperationSupports(supports OperationSupports) OperationSupports {
	cloned := make(OperationSupports, len(supports))
	for i, support := range supports {
		cloned[i] = support
		cloned[i].Modes = append([]OperationMode(nil), support.Modes...)
		cloned[i].Constraints = append(Constraints(nil), support.Constraints...)
	}
	return cloned
}

func intersectModes(left, right []OperationMode) []OperationMode {
	allowed := make(map[OperationMode]struct{}, len(right))
	for _, mode := range right {
		allowed[mode] = struct{}{}
	}

	intersection := make([]OperationMode, 0, len(left))
	for _, mode := range left {
		if _, ok := allowed[mode]; ok {
			intersection = append(intersection, mode)
		}
	}

	return intersection
}

func intersectReverseModes(left, right []OperationMode) []OperationMode {
	if containsOperationMode(left, OperationModeReversePartial) &&
		containsOperationMode(right, OperationModeReversePartial) {
		return []OperationMode{OperationModeReversePartial}
	}
	return nil
}

func containsOperationMode(modes []OperationMode, target OperationMode) bool {
	return slices.ContainsFunc(modes, func(m OperationMode) bool {
		return m == target
	})
}

// OperationDecisionStatus is the mutually exclusive result of evaluating an
// optional operation.
type OperationDecisionStatus string

const (
	// OperationDecisionStatusUnsupported means the driver or benefit does not
	// support the operation, so its constraints were not evaluated.
	OperationDecisionStatusUnsupported OperationDecisionStatus = "unsupported"

	// OperationDecisionStatusIneligible means the operation is supported, but
	// at least one operation-specific constraint was not satisfied.
	OperationDecisionStatusIneligible OperationDecisionStatus = "ineligible"

	// OperationDecisionStatusEligible means the operation is supported and all
	// operation-specific constraints were satisfied.
	OperationDecisionStatusEligible OperationDecisionStatus = "eligible"
)

// Validate verifies an operation decision status.
func (s OperationDecisionStatus) Validate() error {
	switch s {
	case
		OperationDecisionStatusUnsupported,
		OperationDecisionStatusIneligible,
		OperationDecisionStatusEligible:
		return nil

	default:
		return fmt.Errorf("benefit: invalid operation decision status %q", s)
	}
}

// OperationDecision combines optional operation support with its constraint
// report.
type OperationDecision struct {
	Operation Operation               `json:"operation"`
	Status    OperationDecisionStatus `json:"status"`

	Diagnostic

	Constraints ConstraintReport `json:"constraints"`
}

// IsSupported reports whether the operation capability is available.
func (d OperationDecision) IsSupported() bool {
	return d.Status == OperationDecisionStatusIneligible ||
		d.Status == OperationDecisionStatusEligible
}

// IsEligible reports whether the operation is supported and all of its
// constraints were satisfied.
func (d OperationDecision) IsEligible() bool {
	return d.Status == OperationDecisionStatusEligible
}

// Validate verifies the relationship between the operation status, diagnostic,
// and constraint report.
func (d OperationDecision) Validate() error {
	if d.Operation == "" {
		return errors.New("benefit: operation decision operation is empty")
	}
	if err := d.Status.Validate(); err != nil {
		return err
	}

	switch d.Status {
	case OperationDecisionStatusUnsupported:
		if d.Constraints.Status != ConstraintReportStatusUnevaluated {
			return errors.New("benefit: unsupported operation has evaluated constraints")
		}

	case OperationDecisionStatusIneligible:
		if d.Constraints.Status != ConstraintReportStatusUnsatisfied {
			return errors.New("benefit: ineligible operation has no unsatisfied constraints")
		}

	case OperationDecisionStatusEligible:
		if d.Constraints.Status != ConstraintReportStatusSatisfied {
			return errors.New("benefit: eligible operation has unsatisfied constraints")
		}
		if d.Reason != "" || len(d.Details) > 0 {
			return errors.New("benefit: eligible operation has diagnostic information")
		}
	}
	return nil
}

// EvaluateOperation checks one optional capability and evaluates its
// operation-specific constraints.
func EvaluateOperation(
	ctx context.Context,
	registry *ConstraintRegistry,
	supports OperationSupports,
	operation Operation,
	input EvaluationInput,
) (OperationDecision, error) {
	if isCoreOperation(operation) {
		const msg = "benefit: core operation %q does not require capability evaluation"
		return OperationDecision{}, fmt.Errorf(msg, operation)
	}
	if registry == nil {
		return OperationDecision{}, errors.New("benefit: constraint registry is nil")
	}
	if err := supports.Validate(); err != nil {
		return OperationDecision{}, err
	}

	support, ok := supports.Get(operation)
	if !ok || !support.Supported {
		return OperationDecision{
			Operation:   operation,
			Status:      OperationDecisionStatusUnsupported,
			Diagnostic:  Diagnostic{Reason: "operation is not supported"},
			Constraints: ConstraintReport{Status: ConstraintReportStatusUnevaluated},
		}, nil
	}

	report := registry.EvaluateAll(ctx, input, support.Constraints)
	decision := OperationDecision{
		Operation:   operation,
		Status:      OperationDecisionStatusEligible,
		Constraints: report,
	}
	if !report.IsSatisfied() {
		decision.Status = OperationDecisionStatusIneligible
		decision.Reason = "operation constraints are unsatisfied"
	}
	return decision, nil
}
