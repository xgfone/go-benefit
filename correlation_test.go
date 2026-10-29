package benefit_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/xgfone/go-benefit"
)

func TestRedeemCorrelation(t *testing.T) {
	req := benefit.RedeemRequest{
		Reference:    benefit.BenefitReference{Value: "coupon"},
		RedemptionId: "caller-redeem",
	}
	for _, tc := range []struct {
		name, requested, returned, provider string
		mismatch                            bool
	}{
		{"missing caller", req.RedemptionId, "", "provider-redeem", false},
		{"matching caller", req.RedemptionId, req.RedemptionId, "provider-redeem", false},
		{"no caller", "", "", "provider-redeem", false},
		{"no provider", req.RedemptionId, "", "", false},
		{"different caller", req.RedemptionId, "different", "provider-redeem", true},
		{"unsolicited caller", "", "unexpected", "provider-redeem", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := req
			req.RedemptionId = tc.requested
			original := benefit.RedeemResult{
				Status: benefit.ResultStatusSuccess,
				Redemption: &benefit.Redemption{
					RedemptionId:         tc.returned,
					ProviderRedemptionId: tc.provider,
					ProviderData:         "opaque",
				},
			}
			result, err := original.Correlate(req)
			if tc.mismatch {
				if !errors.Is(err, benefit.ErrIdMismatch) ||
					!errors.Is(err, benefit.ErrValidation) ||
					!reflect.DeepEqual(result, benefit.RedeemResult{}) {
					t.Fatalf("mismatched result accepted: %+v, %v", result, err)
				}
			} else if err != nil ||
				result.Redemption.RedemptionId != tc.requested ||
				result.Redemption.ProviderRedemptionId != tc.provider ||
				result.Redemption.ProviderData != "opaque" {
				t.Fatalf("incorrect correlation: %+v, %v", result, err)
			} else {
				result.Redemption.RedemptionId = "changed copy"
			}

			if original.Redemption.RedemptionId != tc.returned ||
				original.Redemption.ProviderRedemptionId != tc.provider {
				t.Fatal("original result was modified")
			}
		})
	}
}

func TestReverseCorrelation(t *testing.T) {
	base := benefit.ReverseRequest{
		Reference:            benefit.BenefitReference{Value: "coupon"},
		RedemptionId:         "caller-redeem",
		ReversalId:           "caller-reverse",
		ProviderRedemptionId: "provider-redeem",
	}
	for _, tc := range []struct {
		name, redemption, reversal, provider string
		noCaller, noProvider, mismatch       bool
	}{
		{name: "missing caller", provider: "provider-redeem"},
		{name: "matching", redemption: "caller-redeem", reversal: "caller-reverse", provider: "provider-redeem"},
		{name: "different redemption", redemption: "wrong", mismatch: true},
		{name: "different reversal", reversal: "wrong", mismatch: true},
		{name: "different provider", provider: "wrong", mismatch: true},
		{name: "no returned provider"},
		{name: "provider not requested", provider: "provider-redeem", noProvider: true},
		{name: "no caller", noCaller: true, provider: "provider-redeem"},
		{name: "unsolicited redemption", noCaller: true, redemption: "wrong", mismatch: true},
		{name: "unsolicited reversal", noCaller: true, reversal: "wrong", mismatch: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := base
			if tc.noCaller {
				req.RedemptionId, req.ReversalId = "", ""
			}
			if tc.noProvider {
				req.ProviderRedemptionId = ""
			}

			r := benefit.ReverseResult{
				Status: benefit.ResultStatusSuccess,
				Reversal: &benefit.Reversal{
					RedemptionId:         tc.redemption,
					ReversalId:           tc.reversal,
					ProviderRedemptionId: tc.provider,
					ProviderReversalId:   "provider-reverse",
					ProviderData:         "opaque",
				},
			}
			result, err := r.Correlate(req)
			if tc.mismatch {
				if !errors.Is(err, benefit.ErrIdMismatch) ||
					!errors.Is(err, benefit.ErrValidation) ||
					!reflect.DeepEqual(result, benefit.ReverseResult{}) {
					t.Fatalf("mismatched result accepted: %+v, %v", result, err)
				}
			} else if err != nil ||
				result.Reversal.RedemptionId != req.RedemptionId ||
				result.Reversal.ReversalId != req.ReversalId ||
				result.Reversal.ProviderRedemptionId != tc.provider ||
				result.Reversal.ProviderReversalId != "provider-reverse" ||
				result.Reversal.ProviderData != "opaque" {
				t.Fatalf("incorrect correlation: %+v, %v", result, err)
			} else {
				result.Reversal.ReversalId = "changed copy"
			}

			if r.Reversal.RedemptionId != tc.redemption ||
				r.Reversal.ReversalId != tc.reversal ||
				r.Reversal.ProviderRedemptionId != tc.provider {
				t.Fatal("original result was modified")
			}
		})
	}
}

func TestCorrelationStatusAndValidation(t *testing.T) {
	redeemReq := benefit.RedeemRequest{
		Reference:    benefit.BenefitReference{Value: "coupon"},
		RedemptionId: "caller",
	}
	reverseReq := benefit.ReverseRequest{
		Reference: redeemReq.Reference,
		Input:     redeemReq.Input,

		RedemptionId:         "caller",
		ReversalId:           "reverse",
		ProviderRedemptionId: "provider",
	}
	for _, status := range []benefit.ResultStatus{
		benefit.ResultStatusFailure,
		benefit.ResultStatusPending,
		benefit.ResultStatusUnknown,
	} {
		rd := benefit.RedeemResult{Status: status}
		rv := benefit.ReverseResult{Status: status}
		if status == benefit.ResultStatusFailure {
			rd.Failure = &benefit.RedeemFailure{Code: benefit.RedeemFailureProviderRejected}
			rv.Failure = &benefit.ReversalFailure{Code: benefit.ReversalFailureProviderRejected}
		}

		gotRd, errRd := rd.Correlate(redeemReq)
		gotRv, errRv := rv.Correlate(reverseReq)
		if errRd != nil || errRv != nil ||
			!reflect.DeepEqual(gotRd, rd) ||
			!reflect.DeepEqual(gotRv, rv) {
			t.Fatalf("status changed: %s, %v, %v", status, errRd, errRv)
		}
	}

	for _, rd := range []benefit.RedeemResult{
		{}, {Status: benefit.ResultStatusSuccess},
		{Status: benefit.ResultStatusFailure, Redemption: &benefit.Redemption{}},
		{Status: benefit.ResultStatusPending, Redemption: &benefit.Redemption{}},
		{Status: benefit.ResultStatusUnknown, Failure: &benefit.RedeemFailure{}},
		{Status: benefit.ResultStatusSuccess, Redemption: &benefit.Redemption{
			Outcome: benefit.BenefitOutcome{
				Discount: &benefit.DiscountEffect{Type: "invalid"},
			},
		}},
	} {
		got, err := rd.Correlate(redeemReq)
		if !errors.Is(err, benefit.ErrValidation) ||
			!reflect.DeepEqual(got, benefit.RedeemResult{}) {
			t.Fatalf("invalid redemption accepted: %+v %v", got, err)
		}
	}

	for _, rv := range []benefit.ReverseResult{
		{}, {Status: benefit.ResultStatusSuccess},
		{Status: benefit.ResultStatusPending, Reversal: &benefit.Reversal{}},
		{Status: benefit.ResultStatusFailure, Reversal: &benefit.Reversal{}},
		{Status: benefit.ResultStatusUnknown, Failure: &benefit.ReversalFailure{}},
		{Status: benefit.ResultStatusSuccess, Reversal: &benefit.Reversal{
			Outcome: benefit.ReversalOutcome{
				Restored: benefit.BenefitRestoration{
					Money: &benefit.Money{Amount: -1},
				},
			},
		}},
	} {
		got, err := rv.Correlate(reverseReq)
		if !errors.Is(err, benefit.ErrValidation) ||
			!reflect.DeepEqual(got, benefit.ReverseResult{}) {
			t.Fatalf("invalid reversal accepted: %+v %v", got, err)
		}
	}

	_, err := (benefit.RedeemResult{
		Status:     benefit.ResultStatusSuccess,
		Redemption: &benefit.Redemption{},
	}).Correlate(benefit.RedeemRequest{})
	if !errors.Is(err, benefit.ErrValidation) {
		t.Fatal(err)
	}

	reverseReq.Mode = "invalid"
	_, err = (benefit.ReverseResult{
		Status:   benefit.ResultStatusSuccess,
		Reversal: &benefit.Reversal{},
	}).Correlate(reverseReq)
	if !errors.Is(err, benefit.ErrValidation) {
		t.Fatal(err)
	}
}

func TestConcurrentCorrelationDoesNotMutateHistory(t *testing.T) {
	rd := benefit.RedeemResult{
		Status: benefit.ResultStatusSuccess,
		Redemption: &benefit.Redemption{
			ProviderRedemptionId: "provider-redeem",
		},
	}
	rv := benefit.ReverseResult{
		Status: benefit.ResultStatusSuccess,
		Reversal: &benefit.Reversal{
			ProviderRedemptionId: "provider-redeem",
			ProviderReversalId:   "provider-reverse",
		},
	}
	for range 32 {
		t.Run("replay", func(t *testing.T) {
			t.Parallel()
			req := benefit.DriverRequest{Reference: benefit.BenefitReference{Value: "coupon"}}
			r, err := rd.Correlate(benefit.RedeemRequest{
				Reference: req.Reference,
				Input:     req.Input,

				RedemptionId: t.Name(),
			})
			if err != nil || r.Redemption.RedemptionId != t.Name() {
				t.Fatalf("correlation failed: %+v %v", r, err)
			}

			v, err := rv.Correlate(benefit.ReverseRequest{
				Reference: req.Reference,
				Input:     req.Input,

				RedemptionId:         t.Name(),
				ReversalId:           t.Name(),
				ProviderRedemptionId: "provider-redeem",
			})
			if err != nil ||
				v.Reversal.RedemptionId != t.Name() ||
				v.Reversal.ReversalId != t.Name() {
				t.Fatalf("correlation failed: %+v %v", v, err)
			}
			if rd.Redemption.RedemptionId != "" ||
				rv.Reversal.RedemptionId != "" ||
				rv.Reversal.ReversalId != "" {
				t.Fatal("shared historical result was modified")
			}
		})
	}
}
