package benefit

import (
	"context"
	"fmt"
	"time"
)

// EvaluationFailureCode identifies why an operation is not eligible.
type EvaluationFailureCode string

const (
	// EvaluationFailureBenefitNotFound means the supplied reference did not
	// resolve to a benefit.
	EvaluationFailureBenefitNotFound EvaluationFailureCode = "benefit.not_found"

	// EvaluationFailureBenefitInactive means the benefit is not currently
	// in the active lifecycle state.
	EvaluationFailureBenefitInactive EvaluationFailureCode = "benefit.inactive"

	EvaluationFailureConstraintUnsatisfied EvaluationFailureCode = "constraint.unsatisfied"
)

// EvaluationFailure describes a local or provider eligibility failure.
type EvaluationFailure struct {
	Code EvaluationFailureCode

	Diagnostic
}

// EvaluationResult is the result of evaluating a benefit for one operation.
type EvaluationResult struct {
	Eligible    bool
	Constraints ConstraintReport `json:",omitzero"`

	// Failure
	Failure *EvaluationFailure `json:",omitempty"`

	// Success
	Outcome BenefitOutcome `json:",omitzero"`

	ExpiresAt       time.Time `json:",omitzero"`
	EvaluationToken string    `json:",omitempty"`
}

// EvaluateLocalEligibility applies normalized status and constraints.
func EvaluateLocalEligibility(
	ctx context.Context,
	registry *ConstraintRegistry,
	input EvaluationInput,
) (EvaluationResult, error) {
	if registry == nil {
		registry = DefaultConstraintRegistry
	}

	if input.Benefit.Status != StatusActive {
		return EvaluationResult{
			Eligible: false,
			Failure: &EvaluationFailure{
				Code: EvaluationFailureBenefitInactive,
				Diagnostic: Diagnostic{
					Reason: fmt.Sprintf("benefit status is %q", input.Benefit.Status),
				},
			},
			Constraints: ConstraintReport{Status: ConstraintReportStatusUnevaluated},
		}, nil
	}

	report := registry.EvaluateAll(ctx, input, input.Benefit.Constraints)
	result := EvaluationResult{Eligible: report.IsSatisfied(), Constraints: report}
	if !result.Eligible {
		result.Failure = &EvaluationFailure{
			Code: EvaluationFailureConstraintUnsatisfied,
		}
	}
	return result, nil
}
