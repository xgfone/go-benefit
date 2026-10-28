package benefit

import (
	"fmt"
	"time"
)

// ReversalFailureCode is the stable business classification of a reversal failure.
type ReversalFailureCode string

const (
	ReversalFailureRedemptionNotFound     ReversalFailureCode = "redemption.not_found"
	ReversalFailureRedemptionReversed     ReversalFailureCode = "redemption.reversed"
	ReversalFailureRedemptionIrreversible ReversalFailureCode = "redemption.irreversible"

	ReversalFailureReversalUnsupported        ReversalFailureCode = "reversal.unsupported"
	ReversalFailureReversalPartialUnsupported ReversalFailureCode = "reversal.partial.unsupported"
	ReversalFailureReversalWindowExpired      ReversalFailureCode = "reversal.window_expired"

	ReversalFailureProviderUnavailable ReversalFailureCode = "provider.unavailable"
	ReversalFailureProviderRejected    ReversalFailureCode = "provider.rejected"
	ReversalFailureProviderTimeout     ReversalFailureCode = "provider.timeout"

	ReversalFailureUnknown ReversalFailureCode = "unknown"
)

// ReversalFailure contains normalized and provider diagnostic information.
type ReversalFailure struct {
	Code ReversalFailureCode

	Diagnostic
}

// Reversal is an immutable record of one confirmed reversal operation.
// Caller- and provider-assigned IDs are optional and remain empty when they
// were not supplied or returned.
type Reversal struct {
	RedemptionId         string `json:",omitempty"`
	ProviderRedemptionId string `json:",omitempty"`

	ReversalId         string `json:",omitempty"`
	ProviderReversalId string `json:",omitempty"`

	ReversedAt time.Time       `json:",omitzero"`
	Outcome    ReversalOutcome `json:",omitzero"`

	// ProviderData contains opaque provider-specific data for this confirmed
	// reversal operation.
	ProviderData string `json:",omitempty"`
}

// Validate verifies a confirmed reversal record.
func (r Reversal) Validate() error {
	return r.Outcome.Validate()
}

// ReverseResult is the business result of a reverse operation.
type ReverseResult struct {
	Status ResultStatus

	// Failure
	Failure *ReversalFailure `json:",omitzero"`

	// Success
	Reversal *Reversal `json:",omitzero"`
}

// Validate verifies the reversal status payload invariant.
func (r ReverseResult) Validate() error {
	switch r.Status {
	case ResultStatusSuccess:
		if r.Reversal == nil {
			return fmt.Errorf("%w: successful reverse result has no reversal", ErrValidation)
		}
		if r.Failure != nil {
			return fmt.Errorf("%w: successful reverse result has a failure", ErrValidation)
		}
		if err := r.Reversal.Validate(); err != nil {
			return err
		}

	case ResultStatusFailure:
		if r.Failure == nil {
			return fmt.Errorf("%w: failed reverse result has no failure", ErrValidation)
		}
		if r.Reversal != nil {
			return fmt.Errorf("%w: failed reverse result has a reversal", ErrValidation)
		}

	case ResultStatusPending, ResultStatusUnknown:
		if r.Failure != nil {
			return fmt.Errorf("%w: %s reverse result must not claim a confirmed failure", ErrValidation, r.Status)
		}
		if r.Reversal != nil {
			return fmt.Errorf("%w: %s reverse result has a reversal", ErrValidation, r.Status)
		}

	default:
		return fmt.Errorf("%w: invalid reverse result status %q", ErrValidation, r.Status)
	}
	return nil
}
