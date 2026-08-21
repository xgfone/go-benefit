package benefit

import (
	"errors"
	"fmt"
)

// Validate verifies a normalized status value.
func (s Status) Validate() error {
	switch s {
	case StatusUnknown,
		StatusPending,
		StatusActive,
		StatusExpired,
		StatusVoided,
		StatusSuspended,
		StatusExhausted:
		return nil

	default:
		return fmt.Errorf("benefit: invalid status %q", s)
	}
}

// Validate verifies the validity window.
func (v Validity) Validate() error {
	if !v.StartsAt.IsZero() && !v.ExpiresAt.IsZero() && !v.StartsAt.Before(v.ExpiresAt) {
		return errors.New("benefit: validity starts_at must be before expires_at")
	}
	return nil
}

// Validate verifies non-negative usage counters.
func (u Usage) Validate() error {
	if u.RedeemedCount < 0 || u.RemainingCount < 0 {
		return errors.New("benefit: usage counters must not be negative")
	}
	return nil
}

// Validate verifies that a notice has display text and its language.
func (n Notice) Validate() error {
	if n.Code == "" {
		return errors.New("benefit: notice code is empty")
	}

	switch n.Level {
	case NoticeInfo, NoticeWarning, NoticeError:
	default:
		return fmt.Errorf("benefit: invalid notice level %q", n.Level)
	}

	if n.Text == "" {
		return errors.New("benefit: notice text is empty")
	}
	if n.Lang == "" {
		return errors.New("benefit: notice language is empty")
	}

	return nil
}

// Validate verifies a benefit inspection result.
func (b BenefitInfo) Validate() error {
	if err := b.DriverType.Validate(); err != nil {
		return err
	}
	if err := b.Status.Validate(); err != nil {
		return err
	}
	if err := b.Validity.Validate(); err != nil {
		return err
	}
	if err := b.Usage.Validate(); err != nil {
		return err
	}
	if err := b.OperationPolicies.Validate(); err != nil {
		return err
	}
	for _, notice := range b.Notices {
		if err := notice.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// IsZero reports whether no benefit reference was supplied.
func (r BenefitReference) IsZero() bool {
	return r.Value == ""
}

// Validate verifies a redeem request and its operation ID.
func (r RedeemRequest) Validate() error {
	if r.RedemptionID == "" {
		return errors.New("benefit: redemption id is empty")
	}
	return nil
}

// Validate verifies a reverse request and its operation IDs.
func (r ReverseRequest) Validate() error {
	if r.ReversalID == "" {
		return errors.New("benefit: reversal id is empty")
	}
	if r.RedemptionID == "" {
		return errors.New("benefit: reverse redemption id is empty")
	}
	return nil
}
