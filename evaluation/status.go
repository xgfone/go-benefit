package evaluation

import (
	"time"

	"github.com/xgfone/go-benefit"
)

// RedeemFailureCodeForStatus returns the redeem failure code implied by an
// inactive benefit status. The boolean is false when the status does not by
// itself represent a redeem failure or is invalid.
func RedeemFailureCodeForStatus(status benefit.Status) (benefit.RedeemFailureCode, bool) {
	switch status {
	case benefit.StatusUnknown:
		return benefit.RedeemFailureUnknown, true

	case benefit.StatusPending:
		return benefit.RedeemFailureBenefitPending, true

	case benefit.StatusVoided:
		return benefit.RedeemFailureBenefitVoided, true

	case benefit.StatusExpired:
		return benefit.RedeemFailureBenefitExpired, true

	case benefit.StatusExhausted:
		return benefit.RedeemFailureBenefitExhausted, true

	case benefit.StatusSuspended:
		return benefit.RedeemFailureBenefitSuspended, true

	case benefit.StatusActive:
		return "", false

	default:
		return "", false
	}
}

// StatusFacts contains provider facts used to compute a normalized status.
type StatusFacts struct {
	UsageExhausted bool
	ProviderStatus benefit.Status
	Validity       benefit.Validity
}

// ResolveStatus applies business-terminal precedence before time-derived state.
func ResolveStatus(now time.Time, facts StatusFacts) benefit.Status {
	switch facts.ProviderStatus {
	case benefit.StatusVoided, benefit.StatusSuspended, benefit.StatusExhausted, benefit.StatusExpired:
		return facts.ProviderStatus
	}

	if facts.UsageExhausted {
		return benefit.StatusExhausted
	}
	if !facts.Validity.StartsAt.IsZero() && now.Before(facts.Validity.StartsAt) {
		return benefit.StatusPending
	}
	if !facts.Validity.ExpiresAt.IsZero() && !now.Before(facts.Validity.ExpiresAt) {
		return benefit.StatusExpired
	}

	switch facts.ProviderStatus {
	case benefit.StatusPending, benefit.StatusActive:
		return facts.ProviderStatus

	default:
		return benefit.StatusUnknown
	}
}
