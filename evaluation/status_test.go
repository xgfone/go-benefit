package evaluation_test

import (
	"testing"
	"time"

	"github.com/xgfone/go-benefit"
	"github.com/xgfone/go-benefit/evaluation"
)

func TestResolveStatusPrecedence(t *testing.T) {
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	tests := []struct {
		name  string
		facts evaluation.StatusFacts
		want  benefit.Status
	}{
		{
			name: "voided wins over expired",
			facts: evaluation.StatusFacts{
				ProviderStatus: benefit.StatusVoided,
				Validity:       benefit.Validity{ExpiresAt: past},
			},
			want: benefit.StatusVoided,
		},
		{
			name:  "suspended remains terminal",
			facts: evaluation.StatusFacts{ProviderStatus: benefit.StatusSuspended},
			want:  benefit.StatusSuspended,
		},
		{
			name:  "provider exhausted remains terminal",
			facts: evaluation.StatusFacts{ProviderStatus: benefit.StatusExhausted},
			want:  benefit.StatusExhausted,
		},
		{
			name: "exhausted wins over expired",
			facts: evaluation.StatusFacts{
				ProviderStatus: benefit.StatusActive,
				Validity:       benefit.Validity{ExpiresAt: past},
				UsageExhausted: true,
			},
			want: benefit.StatusExhausted,
		},
		{
			name: "pending is derived from time",
			facts: evaluation.StatusFacts{
				ProviderStatus: benefit.StatusActive,
				Validity:       benefit.Validity{StartsAt: future},
			},
			want: benefit.StatusPending,
		},
		{
			name: "expired is derived from time",
			facts: evaluation.StatusFacts{
				ProviderStatus: benefit.StatusActive,
				Validity:       benefit.Validity{ExpiresAt: past},
			},
			want: benefit.StatusExpired,
		},
		{
			name: "provider expired wins over future validity",
			facts: evaluation.StatusFacts{
				ProviderStatus: benefit.StatusExpired,
				Validity:       benefit.Validity{StartsAt: future},
			},
			want: benefit.StatusExpired,
		},
		{
			name:  "provider active remains active",
			facts: evaluation.StatusFacts{ProviderStatus: benefit.StatusActive},
			want:  benefit.StatusActive,
		},
		{
			name: "unknown remains unknown inside the window",
			facts: evaluation.StatusFacts{
				ProviderStatus: benefit.StatusUnknown,
				Validity:       benefit.Validity{StartsAt: past, ExpiresAt: future},
			},
			want: benefit.StatusUnknown,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := evaluation.ResolveStatus(now, test.facts); got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
		})
	}
}
