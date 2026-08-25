package benefit

import (
	"errors"
	"fmt"

	"github.com/xgfone/go-currency"
)

// Money stores an amount in the smallest unit of its currency.
type Money struct {
	Amount   int64
	Currency string
}

// IsZero reports whether no monetary amount was supplied.
func (m Money) IsZero() bool {
	return m == (Money{})
}

// NewMoney constructs and validates a minor-unit monetary amount.
func NewMoney(amount int64, currencyCode string) (Money, error) {
	c, ok := currency.Get(currencyCode)
	if !ok {
		return Money{}, fmt.Errorf("benefit: unsupported currency %q", currencyCode)
	}

	return Money{Amount: amount, Currency: c.Code}, nil
}

// ParseMajorMoney converts a major-unit decimal string into Money.
func ParseMajorMoney(majorAmount, currencyCode string) (Money, error) {
	amount, err := currency.ParseMajorToMinor(majorAmount, currencyCode)
	if err != nil {
		return Money{}, err
	}
	return NewMoney(amount, currencyCode)
}

// Validate reports whether the currency code is registered.
func (m Money) Validate() error {
	if _, ok := currency.Get(m.Currency); !ok {
		return fmt.Errorf("benefit: unsupported currency %q", m.Currency)
	}
	return nil
}

// Major formats the minor-unit amount as a major-unit decimal string.
func (m Money) Major() (string, error) {
	if err := m.Validate(); err != nil {
		return "", err
	}
	return currency.FormatMinorToMajor(m.Amount, m.Currency)
}

// DiscountEffectType identifies a monetary benefit outcome.
type DiscountEffectType string

const (
	DiscountEffectFree       DiscountEffectType = "free"
	DiscountEffectAmount     DiscountEffectType = "amount"
	DiscountEffectPercentage DiscountEffectType = "percentage"
)

// DiscountEffect is the immutable monetary component of a benefit outcome.
type DiscountEffect struct {
	Type DiscountEffectType

	Currency       string `json:",omitempty"`
	OriginalAmount int64  `json:",omitempty"`
	DiscountAmount int64  `json:",omitempty"`
	PayableAmount  int64  `json:",omitempty"`
}

// Validate verifies the optional effect currency and amounts.
func (e DiscountEffect) Validate() error {
	switch e.Type {
	case
		DiscountEffectFree,
		DiscountEffectAmount,
		DiscountEffectPercentage:

	default:
		return fmt.Errorf("benefit: invalid discount effect type %q", e.Type)
	}

	if e.Currency != "" {
		if _, ok := currency.Get(e.Currency); !ok {
			return fmt.Errorf("benefit: unsupported currency %q", e.Currency)
		}
	}
	if e.OriginalAmount < 0 || e.DiscountAmount < 0 || e.PayableAmount < 0 {
		return errors.New("benefit: discount effect amounts must not be negative")
	}
	if e.PayableAmount > e.OriginalAmount || e.OriginalAmount != e.PayableAmount+e.DiscountAmount {
		return errors.New("benefit: discount effect amounts are inconsistent")
	}
	if e.Type == DiscountEffectFree && e.PayableAmount != 0 {
		return errors.New("benefit: free effect must have a zero payable amount")
	}

	return nil
}
