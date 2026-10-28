package benefit

import (
	"fmt"
	"time"
)

// RedeemFailureCode is the stable business classification of a redeem failure.
type RedeemFailureCode string

const (
	RedeemFailureConstraintUnsatisfied RedeemFailureCode = "constraint.unsatisfied"

	RedeemFailureBenefitNotFound  RedeemFailureCode = "benefit.not_found"
	RedeemFailureBenefitPending   RedeemFailureCode = "benefit.pending" // NotStarted
	RedeemFailureBenefitSuspended RedeemFailureCode = "benefit.suspended"
	RedeemFailureBenefitExhausted RedeemFailureCode = "benefit.exhausted" // commonly for multi-use benefits
	RedeemFailureBenefitRedeemed  RedeemFailureCode = "benefit.redeemed"  // commonly for single-use benefits
	RedeemFailureBenefitExpired   RedeemFailureCode = "benefit.expired"
	RedeemFailureBenefitVoided    RedeemFailureCode = "benefit.voided"

	RedeemFailureProviderUnavailable RedeemFailureCode = "provider.unavailable"
	RedeemFailureProviderRejected    RedeemFailureCode = "provider.rejected"
	RedeemFailureProviderTimeout     RedeemFailureCode = "provider.timeout"

	RedeemFailureUnknown RedeemFailureCode = "unknown"
)

// RedeemFailure contains both normalized and provider diagnostic information.
type RedeemFailure struct {
	Code RedeemFailureCode

	Diagnostic

	Violations []ConstraintDecision `json:",omitempty"`
}

// Redemption is an immutable record of one confirmed benefit consumption.
// RedemptionId is the optional caller-assigned ID from RedeemRequest;
// ProviderRedemptionId is empty when the provider does not return one.
type Redemption struct {
	RedemptionId         string `json:",omitempty"`
	ProviderRedemptionId string `json:",omitempty"`

	RedeemedAt time.Time      `json:",omitzero"`
	Outcome    BenefitOutcome `json:",omitzero"`

	// ProviderData contains opaque provider-specific data for this confirmed
	// redemption operation.
	ProviderData string `json:",omitempty"`
}

// Validate verifies a confirmed redemption record.
func (r Redemption) Validate() error {
	if err := r.Outcome.Validate(); err != nil {
		return err
	}

	return nil
}

// RedeemResult is the business result of a redeem operation.
type RedeemResult struct {
	Status ResultStatus

	// Failure
	Failure *RedeemFailure `json:",omitzero"`

	// Success
	Redemption *Redemption `json:",omitzero"`
}

// Validate verifies the status payload invariant.
func (r RedeemResult) Validate() error {
	switch r.Status {
	case ResultStatusSuccess:
		if r.Redemption == nil {
			return fmt.Errorf("%w: successful redeem result has no redemption", ErrValidation)
		}
		if r.Failure != nil {
			return fmt.Errorf("%w: successful redeem result has a failure", ErrValidation)
		}
		if err := r.Redemption.Validate(); err != nil {
			return err
		}

	case ResultStatusFailure:
		if r.Failure == nil {
			return fmt.Errorf("%w: failed redeem result has no failure", ErrValidation)
		}
		if r.Redemption != nil {
			return fmt.Errorf("%w: failed redeem result has a redemption", ErrValidation)
		}

	case ResultStatusPending, ResultStatusUnknown:
		if r.Failure != nil {
			return fmt.Errorf("%w: %s redeem result must not claim a confirmed failure", ErrValidation, r.Status)
		}
		if r.Redemption != nil {
			return fmt.Errorf("%w: %s redeem result has a redemption", ErrValidation, r.Status)
		}

	default:
		return fmt.Errorf("%w: invalid redeem result status %q", ErrValidation, r.Status)
	}
	return nil
}
