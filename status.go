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

// Validity defines an optional half-open validity window [StartsAt, ExpiresAt).
type Validity struct {
	StartsAt  time.Time `json:",omitzero"`
	ExpiresAt time.Time `json:",omitzero"`
}

// IsZero reports whether neither validity boundary is set.
func (v Validity) IsZero() bool {
	return v.StartsAt.IsZero() && v.ExpiresAt.IsZero()
}
