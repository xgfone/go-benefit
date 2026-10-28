package evaluation

import (
	"context"
	"fmt"
	"slices"

	"github.com/xgfone/go-benefit"
	"github.com/xgfone/go-benefit/constraint"
)

// EvaluateOperation checks a driver capability, applies matching
// benefit-specific policies, and evaluates their availability constraints.
// An empty mode evaluates the operation as a whole and does not match
// mode-specific policies. A nil registry is rejected.
// A nil registry returns constraint.ErrNilRegistry; invalid capability,
// policy, or operation arguments wrap benefit.ErrValidation.
func EvaluateOperation(
	ctx context.Context,
	registry *constraint.Registry,
	capabilities benefit.OperationCapabilities,
	policies benefit.OperationPolicies,
	operation benefit.Operation,
	mode benefit.OperationMode,
	input constraint.Input,
) (benefit.OperationDecision, error) {
	if operation.IsCore() {
		const msg = "%w: core operation %q does not require capability evaluation"
		return benefit.OperationDecision{}, fmt.Errorf(msg, benefit.ErrValidation, operation)
	}
	if registry == nil {
		return benefit.OperationDecision{}, constraint.ErrNilRegistry
	}
	if err := capabilities.Validate(); err != nil {
		return benefit.OperationDecision{}, err
	}
	if err := policies.Validate(); err != nil {
		return benefit.OperationDecision{}, err
	}
	if err := validateOperationPoliciesAgainstCapabilities(capabilities, policies); err != nil {
		return benefit.OperationDecision{}, err
	}

	capability, ok := capabilities.Get(operation)
	if !ok {
		return unsupportedOperationDecision(operation, mode, "operation is not supported"), nil
	}
	if mode != "" && !capability.SupportsMode(mode) {
		return unsupportedOperationDecision(operation, mode, "operation mode is not supported"), nil
	}

	var constraints benefit.Constraints
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
	decision := benefit.OperationDecision{
		Operation:   operation,
		Mode:        mode,
		Status:      benefit.OperationDecisionStatusEligible,
		Constraints: report,
	}
	if !report.IsSatisfied() {
		decision.Status = benefit.OperationDecisionStatusIneligible
		decision.Reason = "operation constraints are unsatisfied"
	}
	return decision, nil
}

func validateOperationPoliciesAgainstCapabilities(
	capabilities benefit.OperationCapabilities,
	policies benefit.OperationPolicies,
) error {
	for _, policy := range policies {
		capability, ok := capabilities.Get(policy.Operation)
		if !ok {
			const msg = "%w: operation policy targets unsupported operation %q"
			return fmt.Errorf(msg, benefit.ErrValidation, policy.Operation)
		}

		for _, mode := range policy.MatchModes {
			if !capability.SupportsMode(mode) {
				const msg = "%w: operation %q policy targets unsupported mode %q"
				return fmt.Errorf(msg, benefit.ErrValidation, policy.Operation, mode)
			}
		}
	}
	return nil
}

func policyMatches(policy benefit.OperationPolicy, operation benefit.Operation, mode benefit.OperationMode) bool {
	if policy.Operation != operation {
		return false
	}
	if len(policy.MatchModes) == 0 {
		return true
	}
	return mode != "" && slices.Contains(policy.MatchModes, mode)
}

func unsupportedOperationDecision(
	operation benefit.Operation,
	mode benefit.OperationMode,
	reason string,
) benefit.OperationDecision {
	return benefit.OperationDecision{
		Operation:   operation,
		Mode:        mode,
		Status:      benefit.OperationDecisionStatusUnsupported,
		Diagnostic:  benefit.Diagnostic{Reason: reason},
		Constraints: benefit.ConstraintReport{Status: benefit.ConstraintReportStatusUnevaluated},
	}
}
