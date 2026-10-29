package discount

import (
	"strings"

	"github.com/xgfone/go-benefit"
)

// Resolve validates a monetary outcome against the trusted amount before the
// benefit was applied and returns an independent effect with canonical currency.
// It does not calculate a new price or change provider-confirmed amounts.
//
// A missing Discount is rejected, including for a zero-amount transaction.
// OriginalAmount must equal original.Amount. An empty effect currency inherits
// original.Currency; an explicit effect currency must match it ignoring case.
// If both currencies are empty they remain unspecified, as in CalculateAmount;
// applications requiring a self-contained record must supply a trusted currency.
// Original refers to the amount before this benefit, not a product's list price.
//
// Errors match benefit.ErrValidation and, when applicable,
// benefit.ErrInvalidCurrency, ErrInvalidAmount, ErrMissingEffect,
// ErrOriginalAmountMismatch, or ErrCurrencyMismatch. Errors after a consuming
// operation do not prove that it had no remote effect and must not trigger a
// blind retry. The supplied outcome is never modified.
func Resolve(original benefit.Money, outcome benefit.BenefitOutcome) (benefit.DiscountEffect, error) {
	if err := original.Validate(); err != nil {
		return benefit.DiscountEffect{}, err
	}
	if original.Amount < 0 {
		return benefit.DiscountEffect{}, ErrInvalidAmount
	}
	if outcome.Discount == nil {
		return benefit.DiscountEffect{}, ErrMissingEffect
	}

	effect := *outcome.Discount
	if err := effect.Validate(); err != nil {
		return benefit.DiscountEffect{}, err
	}
	if effect.OriginalAmount != original.Amount {
		return benefit.DiscountEffect{}, ErrOriginalAmountMismatch
	}
	if effect.Currency != "" && !strings.EqualFold(effect.Currency, original.Currency) {
		return benefit.DiscountEffect{}, ErrCurrencyMismatch
	}

	effect.Currency = strings.ToUpper(original.Currency)
	return effect, nil
}
