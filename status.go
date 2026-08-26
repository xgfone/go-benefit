package benefit

import "time"

// Status is the normalized lifecycle status of a benefit.
type Status string

const (
	StatusUnknown   Status = "unknown"
	StatusPending   Status = "pending"
	StatusActive    Status = "active"
	StatusVoided    Status = "voided"
	StatusExpired   Status = "expired"
	StatusSuspended Status = "suspended"
	StatusExhausted Status = "exhausted"
)

// IsActive reports whether the status is active.
func (s Status) IsActive() bool {
	return s == StatusActive
}

// IsPending reports whether the status is pending.
func (s Status) IsPending() bool {
	return s == StatusPending
}

// RedeemFailureCodeForStatus returns the redeem failure code implied by an
// inactive benefit status. The boolean is false when the status does not by
// itself represent a redeem failure or is invalid.
func RedeemFailureCodeForStatus(status Status) (RedeemFailureCode, bool) {
	switch status {
	case StatusUnknown:
		return RedeemFailureUnknown, true

	case StatusPending:
		return RedeemFailureBenefitPending, true

	case StatusVoided:
		return RedeemFailureBenefitVoided, true

	case StatusExpired:
		return RedeemFailureBenefitExpired, true

	case StatusExhausted:
		return RedeemFailureBenefitExhausted, true

	case StatusSuspended:
		return RedeemFailureBenefitSuspended, true

	case StatusActive:
		return "", false

	default:
		return "", false
	}
}

// Validity defines an optional half-open validity window [StartsAt, ExpiresAt).
type Validity struct {
	StartsAt  time.Time `json:",omitzero"`
	ExpiresAt time.Time `json:",omitzero"`
}

// IsZero reports whether neither validity boundary is set.
func (v Validity) IsZero() bool {
	return v.StartsAt.IsZero() && v.ExpiresAt.IsZero()
}

// StatusFacts contains provider facts used to compute a normalized status.
type StatusFacts struct {
	UsageExhausted bool
	ProviderStatus Status
	Validity       Validity
}

// ResolveStatus applies business-terminal precedence before time-derived state.
func ResolveStatus(facts StatusFacts, now time.Time) Status {
	switch facts.ProviderStatus {
	case StatusVoided, StatusSuspended, StatusExhausted, StatusExpired:
		return facts.ProviderStatus
	}

	if facts.UsageExhausted {
		return StatusExhausted
	}
	if !facts.Validity.StartsAt.IsZero() && now.Before(facts.Validity.StartsAt) {
		return StatusPending
	}
	if !facts.Validity.ExpiresAt.IsZero() && !now.Before(facts.Validity.ExpiresAt) {
		return StatusExpired
	}

	switch facts.ProviderStatus {
	case StatusPending, StatusActive:
		return facts.ProviderStatus

	default:
		return StatusUnknown
	}
}
