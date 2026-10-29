package discount_test

import (
	"errors"
	"fmt"
	"math"
	"testing"

	"github.com/xgfone/go-benefit"
	"github.com/xgfone/go-benefit/discount"
)

func TestResolveDiscount(t *testing.T) {
	for _, tc := range []struct {
		name     string
		original benefit.Money
		effect   benefit.DiscountEffect
		currency string
	}{
		{
			"inherit",
			benefit.Money{Amount: 100, Currency: "cny"},
			benefit.DiscountEffect{
				Type: "amount",

				OriginalAmount: 100,
				DiscountAmount: 20,
				PayableAmount:  80,
			},
			"CNY",
		},
		{
			"case insensitive",
			benefit.Money{Amount: 100, Currency: "CNY"},
			benefit.DiscountEffect{
				Type: "percentage",

				Currency:       "cny",
				OriginalAmount: 100,
				DiscountAmount: 20,
				PayableAmount:  80,
			},
			"CNY",
		},
		{
			"zero",
			benefit.Money{Currency: "CNY"},
			benefit.DiscountEffect{Type: "free"},
			"CNY",
		},
		{
			"free",
			benefit.Money{Amount: 100, Currency: "CNY"},
			benefit.DiscountEffect{
				Type: "free",

				OriginalAmount: 100,
				DiscountAmount: 100,
			},
			"CNY",
		},
		{
			"unspecified currency",
			benefit.Money{Amount: 100},
			benefit.DiscountEffect{
				Type: "amount",

				OriginalAmount: 100,
				PayableAmount:  100,
			},
			"",
		},
		{
			"max amount",
			benefit.Money{Amount: math.MaxInt64, Currency: "CNY"},
			benefit.DiscountEffect{
				Type: "amount",

				OriginalAmount: math.MaxInt64,
				PayableAmount:  math.MaxInt64 - 1,
				DiscountAmount: 1,
			},
			"CNY",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original := tc.effect
			outcome := benefit.BenefitOutcome{Discount: &tc.effect}
			got, err := discount.Resolve(tc.original, outcome)
			want := tc.effect
			want.Currency = tc.currency
			if err != nil || got != want || tc.effect != original {
				t.Fatalf("incorrect or mutating resolve: %+v, %v", got, err)
			}
		})
	}
}

func TestResolveRejectsUntrustedDiscount(t *testing.T) {
	valid := benefit.DiscountEffect{Type: "amount", OriginalAmount: 100, DiscountAmount: 20, PayableAmount: 80}
	for _, tc := range []struct {
		name     string
		original benefit.Money
		effect   *benefit.DiscountEffect
		category error
	}{
		{
			"missing discount",
			benefit.Money{},
			nil,
			discount.ErrMissingEffect,
		},
		{
			"wrong original",
			benefit.Money{Amount: 120},
			&valid,
			discount.ErrOriginalAmountMismatch,
		},
		{
			"old currency",
			benefit.Money{Amount: 100, Currency: "USD"},
			&benefit.DiscountEffect{
				Type: "amount",

				Currency:       "CNY",
				OriginalAmount: 100,
				PayableAmount:  100,
			},
			discount.ErrCurrencyMismatch,
		},
		{
			"missing trusted currency",
			benefit.Money{Amount: 100},
			&benefit.DiscountEffect{
				Type: "amount",

				Currency:       "CNY",
				OriginalAmount: 100,
				PayableAmount:  100,
			},
			discount.ErrCurrencyMismatch,
		},
		{
			"negative original",
			benefit.Money{Amount: -1},
			&valid, discount.ErrInvalidAmount,
		},
		{
			"invalid trusted currency",
			benefit.Money{Amount: 100, Currency: "US1"},
			&valid, benefit.ErrInvalidCurrency,
		},
		{
			"invalid effect currency",
			benefit.Money{Amount: 100},
			&benefit.DiscountEffect{
				Type: "amount",

				Currency:       "CN1",
				OriginalAmount: 100,
				PayableAmount:  100,
			},
			benefit.ErrInvalidCurrency,
		},
		{
			"inconsistent",
			benefit.Money{Amount: 100},
			&benefit.DiscountEffect{
				Type: "amount",

				OriginalAmount: 100,
				PayableAmount:  80,
			},
			benefit.ErrValidation,
		},
		{
			"negative effect",
			benefit.Money{Amount: 100},
			&benefit.DiscountEffect{
				Type: "amount",

				OriginalAmount: 100,
				PayableAmount:  101,
				DiscountAmount: -1,
			},
			benefit.ErrValidation,
		},
		{
			"invalid free",
			benefit.Money{Amount: 100},
			&benefit.DiscountEffect{
				Type: "free",

				OriginalAmount: 100,
				PayableAmount:  100,
			},
			benefit.ErrValidation,
		},
		{
			"overflow",
			benefit.Money{Amount: math.MaxInt64},
			&benefit.DiscountEffect{
				Type: "amount",

				OriginalAmount: math.MaxInt64,
				PayableAmount:  math.MaxInt64,
				DiscountAmount: 1,
			},
			benefit.ErrValidation,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			outcome := benefit.BenefitOutcome{Discount: tc.effect}
			got, err := discount.Resolve(tc.original, outcome)
			err = fmt.Errorf("application: %w", err)
			if got != (benefit.DiscountEffect{}) ||
				!errors.Is(err, tc.category) ||
				!errors.Is(err, benefit.ErrValidation) {
				t.Fatalf("unexpected resolve: %+v, %v", got, err)
			}
		})
	}
}

func TestConcurrentResolveUsesOriginalTransaction(t *testing.T) {
	outcome := benefit.BenefitOutcome{
		Discount: &benefit.DiscountEffect{
			Type: "amount",

			OriginalAmount: 100,
			PayableAmount:  80,
			DiscountAmount: 20,
		},
	}

	for _, currency := range []string{"CNY", "USD", "EUR", "JPY"} {
		t.Run(currency, func(t *testing.T) {
			t.Parallel()
			for range 100 {
				got, err := discount.Resolve(benefit.Money{Amount: 100, Currency: currency}, outcome)
				if err != nil || got.Currency != currency || outcome.Discount.Currency != "" {
					t.Fatalf("historical currency was overwritten: %+v %v", got, err)
				}
			}
		})
	}
}

func ExampleResolve() {
	original := benefit.Money{Amount: 100, Currency: "CNY"}
	outcome := benefit.BenefitOutcome{
		Discount: &benefit.DiscountEffect{
			Type: benefit.DiscountEffectAmount,

			OriginalAmount: 100,
			DiscountAmount: 20,
			PayableAmount:  80,
		},
	}
	effect, err := discount.Resolve(original, outcome)
	if err != nil {
		panic(err)
	}

	fmt.Println(effect.PayableAmount, effect.Currency)
	// Output: 80 CNY
}
