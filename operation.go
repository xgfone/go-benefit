package benefit

import (
	"fmt"
	"slices"
)

// Operation identifies a driver operation.
type Operation string

const (
	OperationInspect  Operation = "Inspect"
	OperationEvaluate Operation = "Evaluate"
	OperationRedeem   Operation = "Redeem"
	OperationReverse  Operation = "Reverse"
)

// IsCore reports whether the operation is required by every driver.
func (operation Operation) IsCore() bool {
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
	Operation Operation
	Modes     []OperationMode `json:",omitempty"`
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
	Operation   Operation
	MatchModes  []OperationMode `json:",omitzero"`
	Constraints Constraints     `json:",omitzero"`
	Remark      string          `json:",omitzero"`
	Disabled    bool            `json:",omitzero"`
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
			return fmt.Errorf("%w: operation %q is duplicated", ErrValidation, capability.Operation)
		}
		seen[capability.Operation] = struct{}{}

		if err := validateOperationModes(capability.Operation, capability.Modes); err != nil {
			return err
		}
		if capability.Operation == OperationReverse &&
			!containsOperationMode(capability.Modes, OperationModeReverseFull) {
			return fmt.Errorf("%w: reverse capability must include full mode", ErrValidation)
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
			return fmt.Errorf("%w: operation %q policy cannot be disabled and conditional", ErrValidation, policy.Operation)
		case !policy.Disabled && !hasConstraints:
			return fmt.Errorf("%w: operation %q policy has no effect", ErrValidation, policy.Operation)
		}
	}
	return nil
}

func validateOptionalOperation(operation Operation, index int) error {
	if operation == "" {
		return fmt.Errorf("%w: operation at index %d is empty", ErrValidation, index)
	}
	if operation.IsCore() {
		return fmt.Errorf("%w: core operation %q must not be declared", ErrValidation, operation)
	}
	return nil
}

func validateOperationModes(operation Operation, modes []OperationMode) error {
	seen := make(map[OperationMode]struct{}, len(modes))
	for _, mode := range modes {
		if mode == "" {
			return fmt.Errorf("%w: operation %q has an empty mode", ErrValidation, operation)
		}
		if _, ok := seen[mode]; ok {
			return fmt.Errorf("%w: operation %q mode %q is duplicated", ErrValidation, operation, mode)
		}
		if operation == OperationReverse &&
			mode != OperationModeReverseFull &&
			mode != OperationModeReversePartial {
			return fmt.Errorf("%w: reverse operation has unsupported mode %q", ErrValidation, mode)
		}
		seen[mode] = struct{}{}
	}
	return nil
}

// SupportsMode reports whether the driver capability explicitly supports mode.
func (capability OperationCapability) SupportsMode(mode OperationMode) bool {
	return containsOperationMode(capability.Modes, mode)
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
		return fmt.Errorf("%w: invalid operation decision status %q", ErrValidation, s)
	}
}

// OperationDecision is the evaluated availability of one optional operation
// and optional mode.
type OperationDecision struct {
	Operation Operation
	Mode      OperationMode `json:",omitempty"`

	Status      OperationDecisionStatus
	Constraints ConstraintReport

	Diagnostic
}

// IsSupported reports whether the operation capability exists and is not
// permanently disabled for this benefit.
func (d OperationDecision) IsSupported() bool {
	return d.Status != OperationDecisionStatusUnsupported
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
		return fmt.Errorf("%w: operation decision operation is empty", ErrValidation)
	}
	if err := d.Status.Validate(); err != nil {
		return err
	}

	switch d.Status {
	case OperationDecisionStatusUnsupported:
		if d.Constraints.Status != ConstraintReportStatusUnevaluated {
			return fmt.Errorf("%w: unsupported operation has evaluated constraints", ErrValidation)
		}

	case OperationDecisionStatusIneligible:
		if d.Constraints.Status != ConstraintReportStatusUnsatisfied {
			return fmt.Errorf("%w: ineligible operation has no unsatisfied constraints", ErrValidation)
		}

	case OperationDecisionStatusEligible:
		if d.Constraints.Status != ConstraintReportStatusSatisfied {
			return fmt.Errorf("%w: eligible operation has unsatisfied constraints", ErrValidation)
		}
		if d.Reason != "" || len(d.Details) > 0 {
			return fmt.Errorf("%w: eligible operation has diagnostic information", ErrValidation)
		}
	}
	return nil
}
