package benefit

import (
	"math"
	"math/big"
	"testing"
)

func TestCalculatePercentageDiscount(t *testing.T) {
	for _, amount := range []int64{0, 1, 99, 101, 99999999, math.MaxInt64} {
		for _, rate := range []int64{0, 1, 3333, 9999, 10000} {
			effect, err := CalculatePercentageDiscount(Money{
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
		if _, err := CalculatePercentageDiscount(Money{Amount: 100}, rate); err == nil {
			t.Fatalf("accepted rate %d", rate)
		}
	}
}

func TestCalculateAmountDiscount(t *testing.T) {
	for _, tc := range []struct {
		original, discount Money
		valid              bool
	}{
		{Money{100, "CNY"}, Money{25, "cny"}, true},
		{Money{100, "CNY"}, Money{25, ""}, true},
		{Money{0, "CNY"}, Money{0, "CNY"}, true},
		{Money{100, "CNY"}, Money{25, "USD"}, false},
		{Money{100, ""}, Money{25, "CNY"}, false},
		{Money{0, "CNY"}, Money{1, ""}, false},
		{Money{100, "CNY"}, Money{-1, ""}, false},
		{Money{math.MinInt64, ""}, Money{math.MaxInt64, ""}, false},
	} {
		effect, err := CalculateAmountDiscount(tc.original, tc.discount)
		if (err == nil) != tc.valid {
			t.Fatalf("%+v: effect=%+v err=%v", tc, effect, err)
		}
		if err == nil && effect.Validate() != nil {
			t.Fatalf("invalid effect: %+v", effect)
		}
	}
}
