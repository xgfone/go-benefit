package benefit

import (
	"errors"
	"fmt"
	"time"
)

// RedeemFailureType is the stable business classification of a redeem failure.
type RedeemFailureType string

const (
	RedeemFailureConstraintUnsatisfied RedeemFailureType = "constraint.unsatisfied"

	RedeemFailureBenefitNotFound  RedeemFailureType = "benefit.not_found"
	RedeemFailureBenefitPending   RedeemFailureType = "benefit.pending" // NotStarted
	RedeemFailureBenefitSuspended RedeemFailureType = "benefit.suspended"
	RedeemFailureBenefitExhausted RedeemFailureType = "benefit.exhausted" // commonly for multi-use benefits
	RedeemFailureBenefitRedeemed  RedeemFailureType = "benefit.redeemed"  // commonly for single-use benefits
	RedeemFailureBenefitExpired   RedeemFailureType = "benefit.expired"
	RedeemFailureBenefitVoided    RedeemFailureType = "benefit.voided"

	RedeemFailureProviderUnavailable RedeemFailureType = "provider.unavailable"
	RedeemFailureProviderRejected    RedeemFailureType = "provider.rejected"
	RedeemFailureProviderTimeout     RedeemFailureType = "provider.timeout"

	RedeemFailureUnknown RedeemFailureType = "unknown"
)

// RedeemFailure contains both normalized and provider diagnostic information.
type RedeemFailure struct {
	Type RedeemFailureType `json:"type"`

	// Detail contains optional occurrence-specific diagnostic information. It
	// is not stable, is not localized, and must not be used for program logic
	// or directly presented to end users.
	Detail string `json:"detail,omitempty"`

	Violations []ConstraintResult `json:"violations,omitempty"`
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
