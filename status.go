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

// Validity defines an optional half-open validity window [StartsAt, ExpiresAt).
type Validity struct {
	StartsAt  time.Time `json:"starts_at,omitzero"`
	ExpiresAt time.Time `json:"expires_at,omitzero"`
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
	case StatusVoided, StatusSuspended, StatusExhausted:
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
	case StatusPending, StatusActive, StatusExpired:
		return facts.ProviderStatus

	default:
		return StatusUnknown
	}
}
