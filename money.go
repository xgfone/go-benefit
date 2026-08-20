package benefit

import (
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/xgfone/go-currency"
)

// Money stores an amount in the smallest unit of its currency.
type Money struct {
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
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
// PayRate is an exact decimal string where "0.8" means paying 80 percent.
type DiscountEffect struct {
	Type DiscountEffectType `json:"type"`

	Currency       string `json:"currency,omitempty"`
	OriginalAmount int64  `json:"original_amount,omitempty"`
	DiscountAmount int64  `json:"discount_amount,omitempty"`
	PayableAmount  int64  `json:"payable_amount,omitempty"`
	PayRate        string `json:"pay_rate,omitempty"`

	ProviderData string `json:"provider_data,omitempty"`
}

// Validate verifies the effect currency, amounts, and optional pay rate.
func (e DiscountEffect) Validate() error {
	switch e.Type {
	case
		DiscountEffectFree,
		DiscountEffectAmount,
		DiscountEffectPercentage:

	default:
		return fmt.Errorf("benefit: invalid discount effect type %q", e.Type)
	}

	if _, ok := currency.Get(e.Currency); !ok {
		return fmt.Errorf("benefit: unsupported currency %q", e.Currency)
	}
	if e.OriginalAmount < 0 || e.DiscountAmount < 0 || e.PayableAmount < 0 {
		return errors.New("benefit: discount effect amounts must not be negative")
	}
	if e.PayableAmount > e.OriginalAmount || e.OriginalAmount != e.PayableAmount+e.DiscountAmount {
		return errors.New("benefit: discount effect amounts are inconsistent")
	}
	if e.Type == DiscountEffectPercentage {
		if err := validatePayRate(e.PayRate); err != nil {
			return err
		}
	}
	if e.Type == DiscountEffectFree && e.PayableAmount != 0 {
		return errors.New("benefit: free effect must have a zero payable amount")
	}

	return nil
}

func validatePayRate(rate string) error {
	rate = strings.TrimSpace(rate)
	switch rate {
	case "":
		return errors.New("benefit: percentage effect pay rate is empty")

	case "0", "0.0": /* 0% */
		return nil

	case "1", "1.0": /* 100% */
		return nil
	}

	r, ok := new(big.Rat).SetString(rate)
	if !ok {
		return fmt.Errorf("benefit: invalid pay rate %q", rate)
	}

	if r.Sign() < 0 || r.Cmp(big.NewRat(1, 1)) > 0 {
		return fmt.Errorf("benefit: pay rate %q is outside [0,1]", rate)
	}

	return nil
}
