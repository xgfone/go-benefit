package benefit

import (
	"errors"
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
	Code RedeemFailureCode `json:"code"`

	Diagnostic

	Violations []ConstraintDecision `json:"violations,omitempty"`
}

// Redemption is an immutable record of one confirmed benefit consumption.
type Redemption struct {
	RedemptionID         string `json:"redemption_id"`
	ProviderRedemptionID string `json:"provider_redemption_id,omitempty"`

	Outcome    BenefitOutcome `json:"outcome,omitzero"`
	RedeemedAt time.Time      `json:"redeemed_at,omitzero"`
}

// Validate verifies a confirmed redemption record.
func (r Redemption) Validate() error {
	if r.RedemptionID == "" {
		return errors.New("benefit: redemption id is empty")
	}

	if err := r.Outcome.Validate(); err != nil {
		return err
	}

	return nil
}

// RedeemResult is the business result of a redeem operation.
type RedeemResult struct {
	Status ResultStatus `json:"status"`

	// Failure
	Failure *RedeemFailure `json:"failure,omitempty"`

	// Success
	Redemption *Redemption `json:"redemption,omitempty"`
}

// Validate verifies the status payload invariant.
func (r RedeemResult) Validate() error {
	switch r.Status {
	case ResultStatusSuccess:
		if r.Redemption == nil {
			return errors.New("benefit: successful redeem result has no redemption")
		}
		if r.Failure != nil {
			return errors.New("benefit: successful redeem result has a failure")
		}
		if err := r.Redemption.Validate(); err != nil {
			return err
		}

	case ResultStatusFailure:
		if r.Failure == nil {
			return errors.New("benefit: failed redeem result has no failure")
		}
		if r.Redemption != nil {
			return errors.New("benefit: failed redeem result has a redemption")
		}

	case ResultStatusPending, ResultStatusUnknown:
		if r.Failure != nil {
			return fmt.Errorf("benefit: %s redeem result must not claim a confirmed failure", r.Status)
		}
		if r.Redemption != nil {
			return fmt.Errorf("benefit: %s redeem result has a redemption", r.Status)
		}

	default:
		return fmt.Errorf("benefit: invalid redeem result status %q", r.Status)
	}
	return nil
}
