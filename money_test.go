package benefit_test

import (
	"testing"
	"time"

	benefit "github.com/xgfone/go-benefit"
)

func TestMoneyValidation(t *testing.T) {
	if !(benefit.Money{}).IsZero() || (benefit.Money{Currency: "CNY"}).IsZero() {
		t.Fatal("money zero detection is incorrect")
	}

	for _, money := range []benefit.Money{
		{},
		{Amount: 1234},
		{Amount: 1234, Currency: "CNY"},
		{Amount: 1234, Currency: "cny"},
	} {
		if err := money.Validate(); err != nil {
			t.Fatalf("valid money %#v failed: %v", money, err)
		}
	}

	for _, money := range []benefit.Money{
		{Currency: "CN"},
		{Currency: "CNY1"},
		{Currency: "CN1"},
		{Currency: "人民币"},
	} {
		if err := money.Validate(); err == nil {
			t.Fatalf("invalid money %#v unexpectedly validated", money)
		}
	}
}

func TestDiscountEffectValidation(t *testing.T) {
	percentage := benefit.DiscountEffect{
		Type:           benefit.DiscountEffectPercentage,
		Currency:       "CNY",
		OriginalAmount: 12000,
		DiscountAmount: 2400,
		PayableAmount:  9600,
	}
	free := benefit.DiscountEffect{
		Type:           benefit.DiscountEffectFree,
		Currency:       "CNY",
		OriginalAmount: 12000,
		DiscountAmount: 12000,
	}
	withoutCurrency := percentage
	withoutCurrency.Currency = ""
	for name, effect := range map[string]benefit.DiscountEffect{
		"percentage":     percentage,
		"free":           free,
		"empty currency": withoutCurrency,
	} {
		t.Run(name, func(t *testing.T) {
			if err := effect.Validate(); err != nil {
				t.Fatalf("valid effect failed: %v", err)
			}
		})
	}

	inconsistent := percentage
	inconsistent.PayableAmount = 9500
	invalidFree := free
	invalidFree.DiscountAmount = 11000
	invalidFree.PayableAmount = 1000
	providerCalculated := percentage
	providerCalculated.Type = "provider_calculated"
	invalidCurrency := percentage
	invalidCurrency.Currency = "invalid"
	negativeAmount := percentage
	negativeAmount.DiscountAmount = -1
	invalidEffects := map[string]benefit.DiscountEffect{
		"inconsistent amounts": inconsistent,
		"payable free effect":  invalidFree,
		"invalid type":         providerCalculated,
		"invalid currency":     invalidCurrency,
		"negative amount":      negativeAmount,
	}
	for name, effect := range invalidEffects {
		t.Run(name, func(t *testing.T) {
			if err := effect.Validate(); err == nil {
				t.Fatal("invalid discount effect unexpectedly validated")
			}
		})
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

func TestBenefitRestorationValidation(t *testing.T) {
	if !(benefit.BenefitRestoration{}).IsZero() {
		t.Fatal("zero benefit restoration was not reported as zero")
	}

	explicitZero := benefit.BenefitRestoration{Amount: &benefit.Money{}}
	if explicitZero.IsZero() {
		t.Fatal("explicit zero restoration was reported as absent")
	}
	if err := explicitZero.Validate(); err != nil {
		t.Fatalf("explicit zero restoration failed: %v", err)
	}
	inheritedCurrency := benefit.BenefitRestoration{Amount: &benefit.Money{Amount: 2000}}
	if err := inheritedCurrency.Validate(); err != nil {
		t.Fatalf("restoration with inherited currency failed: %v", err)
	}

	valid := benefit.BenefitRestoration{
		Amount: &benefit.Money{Amount: 2000, Currency: "CNY"},
	}
	if valid.IsZero() {
		t.Fatal("non-zero benefit restoration was reported as zero")
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid benefit restoration failed: %v", err)
	}

	invalid := []benefit.BenefitRestoration{
		{Amount: &benefit.Money{Amount: 1, Currency: "US1"}},
		{Amount: &benefit.Money{Amount: -1, Currency: "CNY"}},
	}
	for _, restoration := range invalid {
		if err := restoration.Validate(); err == nil {
			t.Fatal("invalid benefit restoration unexpectedly validated")
		}
	}
}

func TestReversalOutcomeValidation(t *testing.T) {
	if !(benefit.ReversalOutcome{}).IsZero() {
		t.Fatal("zero reversal outcome was not reported as zero")
	}

	valid := benefit.ReversalOutcome{
		Reversed: benefit.BenefitOutcome{Discount: &benefit.DiscountEffect{
			Type:           benefit.DiscountEffectAmount,
			Currency:       "CNY",
			OriginalAmount: 12000,
			DiscountAmount: 2000,
			PayableAmount:  10000,
		}},
		Restored: benefit.BenefitRestoration{
			Amount: &benefit.Money{Amount: 2000, Currency: "CNY"},
		},
	}
	if valid.IsZero() {
		t.Fatal("non-zero reversal outcome was reported as zero")
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid reversal outcome failed: %v", err)
	}

	invalidReversed := valid
	invalidReversed.Reversed.Discount = &benefit.DiscountEffect{Type: "invalid"}
	if err := invalidReversed.Validate(); err == nil {
		t.Fatal("invalid reversed outcome unexpectedly validated")
	}

	invalidRestored := valid
	invalidRestored.Restored.Amount = &benefit.Money{Amount: -1, Currency: "CNY"}
	if err := invalidRestored.Validate(); err == nil {
		t.Fatal("invalid restored outcome unexpectedly validated")
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
			name:  "suspended remains terminal",
			facts: benefit.StatusFacts{ProviderStatus: benefit.StatusSuspended},
			want:  benefit.StatusSuspended,
		},
		{
			name:  "provider exhausted remains terminal",
			facts: benefit.StatusFacts{ProviderStatus: benefit.StatusExhausted},
			want:  benefit.StatusExhausted,
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
			name: "expired is derived from time",
			facts: benefit.StatusFacts{
				ProviderStatus: benefit.StatusActive,
				Validity:       benefit.Validity{ExpiresAt: past},
			},
			want: benefit.StatusExpired,
		},
		{
			name: "provider expired wins over future validity",
			facts: benefit.StatusFacts{
				ProviderStatus: benefit.StatusExpired,
				Validity:       benefit.Validity{StartsAt: future},
			},
			want: benefit.StatusExpired,
		},
		{
			name:  "provider active remains active",
			facts: benefit.StatusFacts{ProviderStatus: benefit.StatusActive},
			want:  benefit.StatusActive,
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
