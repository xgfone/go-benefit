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

// OperationCapability describes one optional operation supported by a driver.
// Modes explicitly lists every supported mode. Reverse must include "full" and
// may additionally include "partial".
type OperationCapability struct {
	Operation Operation       `json:"operation"`
	Modes     []OperationMode `json:"modes,omitempty"`
}

// OperationCapabilities is an ordered list of a driver's maximum optional
// capabilities. It must not contain the core Inspect, Evaluate, or Redeem
// operations.
type OperationCapabilities []OperationCapability

// OperationPolicy describes a benefit-specific permanent disablement or
// availability requirement. MatchModes is a selector: an empty list matches
// the whole operation, while a non-empty list matches only those modes.
//
// A policy must either set Disabled or provide Constraints, but not both.
type OperationPolicy struct {
	Operation   Operation       `json:"operation"`
	MatchModes  []OperationMode `json:"match_modes,omitempty"`
	Disabled    bool            `json:"disabled,omitempty"`
	Constraints Constraints     `json:"constraints,omitempty"`
	Remark      string          `json:"remark,omitempty"`
}

// OperationPolicies is an ordered list of benefit-specific operation policies.
// A missing policy leaves the corresponding driver capability unrestricted.
// All matching constraint policies are evaluated.
type OperationPolicies []OperationPolicy

// Validate rejects invalid or duplicate capability declarations and modes.
func (capabilities OperationCapabilities) Validate() error {
	seen := make(map[Operation]struct{}, len(capabilities))
	for i, capability := range capabilities {
		if err := validateOptionalOperation(capability.Operation, i); err != nil {
			return err
		}
		if _, ok := seen[capability.Operation]; ok {
			return fmt.Errorf("benefit: operation %q is duplicated", capability.Operation)
		}
		seen[capability.Operation] = struct{}{}

		if err := validateOperationModes(capability.Operation, capability.Modes); err != nil {
			return err
		}
		if capability.Operation == OperationReverse &&
			!containsOperationMode(capability.Modes, OperationModeReverseFull) {
			return errors.New("benefit: reverse capability must include full mode")
		}
	}
	return nil
}

// Validate rejects invalid policies and duplicate match modes.
func (policies OperationPolicies) Validate() error {
	for i, policy := range policies {
		if err := validateOptionalOperation(policy.Operation, i); err != nil {
			return err
		}
		if err := validateOperationModes(policy.Operation, policy.MatchModes); err != nil {
			return err
		}

		hasConstraints := len(policy.Constraints) > 0
		switch {
		case policy.Disabled && hasConstraints:
			return fmt.Errorf("benefit: operation %q policy cannot be disabled and conditional", policy.Operation)
		case !policy.Disabled && !hasConstraints:
			return fmt.Errorf("benefit: operation %q policy has no effect", policy.Operation)
		}
	}
	return nil
}

func validateOptionalOperation(operation Operation, index int) error {
	if operation == "" {
		return fmt.Errorf("benefit: operation at index %d is empty", index)
	}
	if isCoreOperation(operation) {
		return fmt.Errorf("benefit: core operation %q must not be declared", operation)
	}
	return nil
}

func validateOperationModes(operation Operation, modes []OperationMode) error {
	seen := make(map[OperationMode]struct{}, len(modes))
	for _, mode := range modes {
		if mode == "" {
			return fmt.Errorf("benefit: operation %q has an empty mode", operation)
		}
		if _, ok := seen[mode]; ok {
			return fmt.Errorf("benefit: operation %q mode %q is duplicated", operation, mode)
		}
		if operation == OperationReverse &&
			mode != OperationModeReverseFull &&
			mode != OperationModeReversePartial {
			return fmt.Errorf("benefit: reverse operation has unsupported mode %q", mode)
		}
		seen[mode] = struct{}{}
	}
	return nil
}

// SupportsMode reports whether the driver capability explicitly supports mode.
func (capability OperationCapability) SupportsMode(mode OperationMode) bool {
	return containsOperationMode(capability.Modes, mode)
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

// Get returns the capability declaration for an operation.
func (capabilities OperationCapabilities) Get(operation Operation) (OperationCapability, bool) {
	for _, capability := range capabilities {
		if capability.Operation == operation {
			return capability, true
		}
	}
	return OperationCapability{}, false
}

func containsOperationMode(modes []OperationMode, target OperationMode) bool {
	return slices.ContainsFunc(modes, func(m OperationMode) bool {
		return m == target
	})
}

func cloneOperationCapabilities(capabilities OperationCapabilities) OperationCapabilities {
	cloned := make(OperationCapabilities, len(capabilities))
	for i, capability := range capabilities {
		cloned[i] = capability
		cloned[i].Modes = append([]OperationMode(nil), capability.Modes...)
	}
	return cloned
}

// OperationDecisionStatus is the mutually exclusive result of evaluating an
// optional operation.
type OperationDecisionStatus string

const (
	// OperationDecisionStatusUnsupported means the driver does not support the
	// operation or mode, or the benefit permanently disables it. Constraints
	// were not evaluated.
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

// OperationDecision is the evaluated availability of one optional operation
// and optional mode.
type OperationDecision struct {
	Operation   Operation               `json:"operation"`
	Mode        OperationMode           `json:"mode,omitempty"`
	Constraints ConstraintReport        `json:"constraints"`
	Status      OperationDecisionStatus `json:"status"`

	Diagnostic
}

// IsSupported reports whether the operation capability exists and is not
// permanently disabled for this benefit.
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

// EvaluateOperation checks a driver capability, applies matching
// benefit-specific policies, and evaluates their availability constraints.
// An empty mode evaluates the operation as a whole and does not match
// mode-specific policies.
func EvaluateOperation(
	ctx context.Context,
	registry *ConstraintRegistry,
	capabilities OperationCapabilities,
	policies OperationPolicies,
	operation Operation,
	mode OperationMode,
	input EvaluationInput,
) (OperationDecision, error) {
	if isCoreOperation(operation) {
		const msg = "benefit: core operation %q does not require capability evaluation"
		return OperationDecision{}, fmt.Errorf(msg, operation)
	}
	if registry == nil {
		return OperationDecision{}, errors.New("benefit: constraint registry is nil")
	}
	if err := capabilities.Validate(); err != nil {
		return OperationDecision{}, err
	}
	if err := policies.Validate(); err != nil {
		return OperationDecision{}, err
	}
	if err := validateOperationPoliciesAgainstCapabilities(capabilities, policies); err != nil {
		return OperationDecision{}, err
	}

	capability, ok := capabilities.Get(operation)
	if !ok {
		return unsupportedOperationDecision(operation, mode, "operation is not supported"), nil
	}
	if mode != "" && !capability.SupportsMode(mode) {
		return unsupportedOperationDecision(operation, mode, "operation mode is not supported"), nil
	}

	constraints := make(Constraints, 0)
	for _, policy := range policies {
		if !policyMatches(policy, operation, mode) {
			continue
		}
		if policy.Disabled {
			return unsupportedOperationDecision(operation, mode, "operation is disabled for this benefit"), nil
		}
		constraints = append(constraints, policy.Constraints...)
	}

	report := registry.EvaluateAll(ctx, input, constraints)
	decision := OperationDecision{
		Operation:   operation,
		Mode:        mode,
		Status:      OperationDecisionStatusEligible,
		Constraints: report,
	}
	if !report.IsSatisfied() {
		decision.Status = OperationDecisionStatusIneligible
		decision.Reason = "operation constraints are unsatisfied"
	}
	return decision, nil
}

func validateOperationPoliciesAgainstCapabilities(
	capabilities OperationCapabilities,
	policies OperationPolicies,
) error {
	for _, policy := range policies {
		capability, ok := capabilities.Get(policy.Operation)
		if !ok {
			return fmt.Errorf(
				"benefit: operation policy targets unsupported operation %q",
				policy.Operation,
			)
		}
		for _, mode := range policy.MatchModes {
			if !capability.SupportsMode(mode) {
				return fmt.Errorf(
					"benefit: operation %q policy targets unsupported mode %q",
					policy.Operation,
					mode,
				)
			}
		}
	}
	return nil
}

func policyMatches(policy OperationPolicy, operation Operation, mode OperationMode) bool {
	if policy.Operation != operation {
		return false
	}
	if len(policy.MatchModes) == 0 {
		return true
	}
	return mode != "" && containsOperationMode(policy.MatchModes, mode)
}

func unsupportedOperationDecision(
	operation Operation,
	mode OperationMode,
	reason string,
) OperationDecision {
	return OperationDecision{
		Operation:   operation,
		Mode:        mode,
		Status:      OperationDecisionStatusUnsupported,
		Diagnostic:  Diagnostic{Reason: reason},
		Constraints: ConstraintReport{Status: ConstraintReportStatusUnevaluated},
	}
}
