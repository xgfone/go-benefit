package benefit

import "fmt"

// Money stores an integer amount and its optional ISO 4217 currency code.
// The containing model defines the meaning of an empty currency.
type Money struct {
	Amount   int64
	Currency string `json:",omitempty"`
}

// IsZero reports whether both fields have their zero values.
func (m Money) IsZero() bool {
	return m == (Money{})
}

// Validate verifies the syntax of a non-empty currency. ISO 4217 membership
// and major/minor-unit conversion are the caller's responsibility.
// Invalid syntax wraps ErrInvalidCurrency.
func (m Money) Validate() error {
	return validateCurrencyCode(m.Currency)
}

// DiscountEffectType identifies a monetary benefit outcome.
type DiscountEffectType string

const (
	DiscountEffectFree       DiscountEffectType = "free"
	DiscountEffectAmount     DiscountEffectType = "amount"
	DiscountEffectPercentage DiscountEffectType = "percentage"
)

// DiscountEffect is the immutable monetary component of a benefit outcome.
// An empty Currency inherits the transaction currency. All three amounts use
// the same effective currency.
type DiscountEffect struct {
	Type DiscountEffectType

	Currency       string `json:",omitzero"`
	OriginalAmount int64  `json:",omitzero"`
	DiscountAmount int64  `json:",omitzero"`
	PayableAmount  int64  `json:",omitzero"`
}

// Validate verifies the optional effect currency and amounts.
func (e DiscountEffect) Validate() error {
	switch e.Type {
	case
		DiscountEffectFree,
		DiscountEffectAmount,
		DiscountEffectPercentage:

	default:
		return fmt.Errorf("%w: invalid discount effect type %q", ErrValidation, e.Type)
	}

	if err := validateCurrencyCode(e.Currency); err != nil {
		return err
	}
	if e.OriginalAmount < 0 || e.DiscountAmount < 0 || e.PayableAmount < 0 {
		return fmt.Errorf("%w: discount effect amounts must not be negative", ErrValidation)
	}
	if e.PayableAmount > e.OriginalAmount || e.OriginalAmount != e.PayableAmount+e.DiscountAmount {
		return fmt.Errorf("%w: discount effect amounts are inconsistent", ErrValidation)
	}
	if e.Type == DiscountEffectFree && e.PayableAmount != 0 {
		return fmt.Errorf("%w: free effect must have a zero payable amount", ErrValidation)
	}

	return nil
}

// validateCurrencyCode accepts an empty inherited or wildcard currency and
// otherwise verifies the case-insensitive alphabetic form of an ISO 4217 code.
// It deliberately does not carry a registry of assigned codes.
func validateCurrencyCode(code string) error {
	if code == "" {
		return nil
	}
	if len(code) != 3 {
		return fmt.Errorf("%w %q", ErrInvalidCurrency, code)
	}
	for i := range len(code) {
		if (code[i] < 'A' || code[i] > 'Z') && (code[i] < 'a' || code[i] > 'z') {
			return fmt.Errorf("%w %q", ErrInvalidCurrency, code)
		}
	}
	return nil
}
