package benefit_test

import (
	"testing"
	"time"

	benefit "github.com/xgfone/go-benefit"
)

func TestMoneyMajorMinorConversion(t *testing.T) {
	if !(benefit.Money{}).IsZero() || (benefit.Money{Currency: "CNY"}).IsZero() {
		t.Fatal("money zero detection is incorrect")
	}

	money, err := benefit.ParseMajorMoney("12.34", "cny")
	if err != nil {
		t.Fatal(err)
	}
	if money.Amount != 1234 || money.Currency != "CNY" {
		t.Fatalf("unexpected money: %#v", money)
	}

	major, err := money.Major()
	if err != nil {
		t.Fatal(err)
	}
	if major != "12.34" {
		t.Fatalf("got major %q, want 12.34", major)
	}
}

func TestDiscountEffectValidation(t *testing.T) {
	valid := benefit.DiscountEffect{
		Type:           benefit.DiscountEffectPercentage,
		Currency:       "CNY",
		OriginalAmount: 12000,
		PayRate:        "0.8",
		DiscountAmount: 2400,
		PayableAmount:  9600,
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid effect failed: %v", err)
	}

	invalid := valid
	invalid.PayableAmount = 9500
	if err := invalid.Validate(); err == nil {
		t.Fatal("inconsistent effect unexpectedly validated")
	}

	free := benefit.DiscountEffect{
		Type:           benefit.DiscountEffectFree,
		Currency:       "CNY",
		OriginalAmount: 12000,
		DiscountAmount: 12000,
	}
	if err := free.Validate(); err != nil {
		t.Fatalf("valid free effect failed: %v", err)
	}

	invalidFree := free
	invalidFree.DiscountAmount = 11000
	invalidFree.PayableAmount = 1000
	if err := invalidFree.Validate(); err == nil {
		t.Fatal("free effect with a payable amount unexpectedly validated")
	}

	providerCalculated := valid
	providerCalculated.Type = "provider_calculated"
	if err := providerCalculated.Validate(); err == nil {
		t.Fatal("removed provider-calculated effect unexpectedly validated")
	}
}

func TestBenefitOutcomeValidation(t *testing.T) {
	if !(benefit.BenefitOutcome{}).IsZero() {
		t.Fatal("zero benefit outcome was not reported as zero")
	}

	valid := benefit.BenefitOutcome{Discount: &benefit.DiscountEffect{
		Type:           benefit.DiscountEffectAmount,
		Currency:       "CNY",
		OriginalAmount: 12000,
		DiscountAmount: 2000,
		PayableAmount:  10000,
	}}
	if valid.IsZero() {
		t.Fatal("non-zero benefit outcome was reported as zero")
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid benefit outcome failed: %v", err)
	}

	invalid := valid
	invalid.Discount = &benefit.DiscountEffect{
		Type:           benefit.DiscountEffectAmount,
		Currency:       "CNY",
		OriginalAmount: 12000,
		DiscountAmount: 2000,
		PayableAmount:  9000,
	}
	if err := invalid.Validate(); err == nil {
		t.Fatal("invalid benefit outcome unexpectedly validated")
	}
}

func TestResolveStatusPrecedence(t *testing.T) {
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	tests := []struct {
		name  string
		facts benefit.StatusFacts
		want  benefit.Status
	}{
		{
			name: "voided wins over expired",
			facts: benefit.StatusFacts{
				ProviderStatus: benefit.StatusVoided,
				Validity:       benefit.Validity{ExpiresAt: past},
			},
			want: benefit.StatusVoided,
		},
		{
			name: "exhausted wins over expired",
			facts: benefit.StatusFacts{
				ProviderStatus: benefit.StatusActive,
				Validity:       benefit.Validity{ExpiresAt: past},
				UsageExhausted: true,
			},
			want: benefit.StatusExhausted,
		},
		{
			name: "pending is derived from time",
			facts: benefit.StatusFacts{
				ProviderStatus: benefit.StatusActive,
				Validity:       benefit.Validity{StartsAt: future},
			},
			want: benefit.StatusPending,
		},
		{
			name: "unknown remains unknown inside the window",
			facts: benefit.StatusFacts{
				ProviderStatus: benefit.StatusUnknown,
				Validity:       benefit.Validity{StartsAt: past, ExpiresAt: future},
			},
			want: benefit.StatusUnknown,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := benefit.ResolveStatus(test.facts, now); got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
		})
	}
}
