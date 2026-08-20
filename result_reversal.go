package benefit

import (
	"errors"
	"fmt"
	"time"
)

// ReversalFailureType is the stable business classification of a reversal failure.
type ReversalFailureType string

const (
	ReversalFailureRedemptionNotFound     ReversalFailureType = "redemption.not_found"
	ReversalFailureRedemptionReversed     ReversalFailureType = "redemption.reversed"
	ReversalFailureRedemptionIrreversible ReversalFailureType = "redemption.irreversible"

	ReversalFailureReversalUnsupported        ReversalFailureType = "reversal.unsupported"
	ReversalFailureReversalPartialUnsupported ReversalFailureType = "reversal.partial.unsupported"
	ReversalFailureReversalWindowExpired      ReversalFailureType = "reversal.window_expired"

	ReversalFailureProviderUnavailable ReversalFailureType = "provider.unavailable"
	ReversalFailureProviderRejected    ReversalFailureType = "provider.rejected"
	ReversalFailureProviderTimeout     ReversalFailureType = "provider.timeout"

	ReversalFailureUnknown ReversalFailureType = "unknown"
)

// ReversalFailure contains normalized and provider diagnostic information.
type ReversalFailure struct {
	Type ReversalFailureType `json:"type"`

	// Detail contains optional occurrence-specific diagnostic information. It
	// is not stable, is not localized, and must not be used for program logic
	// or directly presented to end users.
	Detail string `json:"detail,omitempty"`
}

// Reversal is an immutable record of one confirmed reversal operation.
type Reversal struct {
	RedemptionID         string `json:"redemption_id,omitempty"`
	ProviderRedemptionID string `json:"provider_redemption_id,omitempty"`

	ReversalID         string `json:"reversal_id,omitempty"`
	ProviderReversalID string `json:"provider_reversal_id,omitempty"`

	RestoredAmount Money     `json:"restored_amount,omitzero"`
	ReversedAt     time.Time `json:"reversed_at,omitzero"`

	ProviderData string `json:"provider_data,omitempty"`
}

// Validate verifies a confirmed reversal record.
func (r Reversal) Validate() error {
	if r.ReversalID == "" {
		return errors.New("benefit: reversal id is empty")
	}
	if r.RedemptionID == "" {
		return errors.New("benefit: reversal redemption id is empty")
	}
	if !r.RestoredAmount.IsZero() {
		if err := r.RestoredAmount.Validate(); err != nil {
			return err
		}
		if r.RestoredAmount.Amount < 0 {
			return errors.New("benefit: restored amount must not be negative")
		}
	}
	return nil
}

// ReverseResult is the business result of a reverse operation.
type ReverseResult struct {
	Status ResultStatus `json:"status"`

	// Failure
	Failure *ReversalFailure `json:"failure,omitempty"`

	// Success
	Reversal *Reversal `json:"reversal,omitempty"`
}

// Validate verifies the reversal status payload invariant.
func (r ReverseResult) Validate() error {
	switch r.Status {
	case ResultStatusSuccess:
		if r.Reversal == nil {
			return errors.New("benefit: successful reverse result has no reversal")
		}
		if r.Failure != nil {
			return errors.New("benefit: successful reverse result has a failure")
		}
		if err := r.Reversal.Validate(); err != nil {
			return err
		}

	case ResultStatusFailure:
		if r.Failure == nil {
			return errors.New("benefit: failed reverse result has no failure")
		}
		if r.Reversal != nil {
			return errors.New("benefit: failed reverse result has a reversal")
		}

	case ResultStatusPending, ResultStatusUnknown:
		if r.Failure != nil {
			return fmt.Errorf("benefit: %s reverse result must not claim a confirmed failure", r.Status)
		}
		if r.Reversal != nil {
			return fmt.Errorf("benefit: %s reverse result has a reversal", r.Status)
		}

	default:
		return fmt.Errorf("benefit: invalid reverse result status %q", r.Status)
	}
	return nil
}
