package discount_test

import (
	"math"
	"math/big"
	"testing"

	"github.com/xgfone/go-benefit"
	"github.com/xgfone/go-benefit/discount"
)

func TestCalculatePercentageDiscount(t *testing.T) {
	for _, amount := range []int64{0, 1, 99, 101, 99999999, math.MaxInt64} {
		for _, rate := range []int64{0, 1, 3333, 9999, 10000} {
			effect, err := discount.CalculatePercentage(benefit.Money{
				Amount:   amount,
				Currency: "cny",
			}, rate)
			if err != nil {
				t.Fatal(err)
			}

			want := new(big.Int).Mul(big.NewInt(amount), big.NewInt(rate))
			want.Quo(want, big.NewInt(10000))
			if effect.DiscountAmount != want.Int64() || effect.Currency != "CNY" {
				t.Fatalf("amount=%d rate=%d: %+v, want %v", amount, rate, effect, want)
			}
			if err := effect.Validate(); err != nil {
				t.Fatal(err)
			}
		}
	}

	for _, rate := range []int64{-1, 10001, math.MaxInt64} {
		if _, err := discount.CalculatePercentage(benefit.Money{Amount: 100}, rate); err == nil {
			t.Fatalf("accepted rate %d", rate)
		}
	}
}

func TestCalculateAmountDiscount(t *testing.T) {
	for _, tc := range []struct {
		original, discount benefit.Money
		valid              bool
	}{
		{benefit.Money{Amount: 100, Currency: "CNY"}, benefit.Money{Amount: 25, Currency: "cny"}, true},
		{benefit.Money{Amount: 100, Currency: "CNY"}, benefit.Money{Amount: 25}, true},
		{benefit.Money{Currency: "CNY"}, benefit.Money{Currency: "CNY"}, true},
		{benefit.Money{Amount: 100, Currency: "CNY"}, benefit.Money{Amount: 25, Currency: "USD"}, false},
		{benefit.Money{Amount: 100}, benefit.Money{Amount: 25, Currency: "CNY"}, false},
		{benefit.Money{Currency: "CNY"}, benefit.Money{Amount: 1}, false},
		{benefit.Money{Amount: 100, Currency: "CNY"}, benefit.Money{Amount: -1}, false},
		{benefit.Money{Amount: math.MinInt64}, benefit.Money{Amount: math.MaxInt64}, false},
	} {
		effect, err := discount.CalculateAmount(tc.original, tc.discount)
		if (err == nil) != tc.valid {
			t.Fatalf("%+v: effect=%+v err=%v", tc, effect, err)
		}
		if err == nil && effect.Validate() != nil {
			t.Fatalf("invalid effect: %+v", effect)
		}
	}
}
