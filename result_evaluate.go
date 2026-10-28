package benefit

import (
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
	Failure *EvaluationFailure `json:",omitzero"`

	// Success
	Outcome BenefitOutcome `json:",omitzero"`

	ExpiresAt       time.Time `json:",omitzero"`
	EvaluationToken string    `json:",omitempty"`
}

// Validate verifies eligibility and payload consistency. An unevaluated
// constraint report is permitted for provider-authoritative evaluations.
func (r EvaluationResult) Validate() error {
	if r.Eligible {
		if r.Failure != nil {
			return fmt.Errorf("%w: eligible evaluation has a failure", ErrValidation)
		}
		if r.Constraints.Status == ConstraintReportStatusUnsatisfied {
			return fmt.Errorf("%w: eligible evaluation has unsatisfied constraints", ErrValidation)
		}
		return r.Outcome.Validate()
	}

	if r.Failure == nil || r.Failure.Code == "" {
		return fmt.Errorf("%w: ineligible evaluation has no failure code", ErrValidation)
	}

	if !r.Outcome.IsZero() {
		return fmt.Errorf("%w: ineligible evaluation has an outcome", ErrValidation)
	}

	return nil
}
