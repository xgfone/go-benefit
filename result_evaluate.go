package benefit

import (
	"context"
	"fmt"
	"time"
)

// EvaluationFailureType identifies why an operation is not eligible.
type EvaluationFailureType string

const (
	// EvaluationFailureBenefitInactive means the benefit is not currently
	// in the active lifecycle state.
	EvaluationFailureBenefitInactive       EvaluationFailureType = "benefit.inactive"
	EvaluationFailureConstraintUnsatisfied EvaluationFailureType = "constraint.unsatisfied"
)

// EvaluationFailure describes a local or provider eligibility failure.
type EvaluationFailure struct {
	Type EvaluationFailureType `json:"type"`

	// Detail contains optional occurrence-specific diagnostic information. It
	// is not stable, is not localized, and must not be used for program logic
	// or directly presented to end users.
	Detail string `json:"detail,omitempty"`
}

// EvaluationResult is the result of evaluating a benefit for one operation.
type EvaluationResult struct {
	Eligible    bool             `json:"eligible"`
	Constraints ConstraintReport `json:"constraints"`

	// Failure
	Failure *EvaluationFailure `json:"failure,omitempty"`

	// Success
	Outcome BenefitOutcome `json:"outcome,omitzero"`

	ExpiresAt       time.Time `json:"expires_at,omitzero"`
	EvaluationToken string    `json:"evaluation_token,omitempty"`
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
				Type:   EvaluationFailureBenefitInactive,
				Detail: fmt.Sprintf("benefit status is %q", input.Benefit.Status),
			},
			Constraints: ConstraintReport{Status: ConstraintReportStatusUnevaluated},
		}, nil
	}

	report := registry.EvaluateAll(ctx, input, input.Benefit.Constraints)
	result := EvaluationResult{Eligible: report.IsSatisfied(), Constraints: report}
	if !result.Eligible {
		result.Failure = &EvaluationFailure{
			Type:   EvaluationFailureConstraintUnsatisfied,
			Detail: "one or more constraints are unsatisfied",
		}
	}
	return result, nil
}
