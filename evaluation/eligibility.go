package evaluation

import (
	"context"
	"fmt"

	"github.com/xgfone/go-benefit"
	"github.com/xgfone/go-benefit/constraint"
)

// EvaluateLocalEligibility applies normalized status and constraints.
// A nil registry uses constraint.DefaultRegistry. Inactive benefits return an
// ineligible result without evaluating their constraints.
func EvaluateLocalEligibility(
	ctx context.Context,
	registry *constraint.Registry,
	input constraint.Input,
) (benefit.EvaluationResult, error) {
	if registry == nil {
		registry = constraint.DefaultRegistry
	}

	if input.Benefit.Status != benefit.StatusActive {
		return benefit.EvaluationResult{
			Eligible: false,
			Failure: &benefit.EvaluationFailure{
				Code: benefit.EvaluationFailureBenefitInactive,

				Reason: fmt.Sprintf("benefit status is %q", input.Benefit.Status),
			},
			Constraints: benefit.ConstraintReport{Status: benefit.ConstraintReportStatusUnevaluated},
		}, nil
	}

	report := registry.EvaluateAll(ctx, input, input.Benefit.Constraints)
	result := benefit.EvaluationResult{Eligible: report.IsSatisfied(), Constraints: report}
	if !result.Eligible {
		result.Failure = &benefit.EvaluationFailure{
			Code: benefit.EvaluationFailureConstraintUnsatisfied,
		}
	}
	return result, nil
}
