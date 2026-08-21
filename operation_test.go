package benefit_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	benefit "github.com/xgfone/go-benefit"
)

func TestEffectiveOperationSupportsOnlyNarrowsCapabilities(t *testing.T) {
	const operationArchive benefit.Operation = "Archive"
	const (
		modeManual    benefit.OperationMode = "manual"
		modeAutomatic benefit.OperationMode = "automatic"
	)
	declared := benefit.OperationSupports{
		benefit.OperationSupport{
			Operation: operationArchive,
			Supported: true,
			Modes: []benefit.OperationMode{
				modeManual,
				modeAutomatic,
			},
		},
		benefit.OperationSupport{
			Operation: benefit.OperationReverse,
			Supported: true,
			Modes: []benefit.OperationMode{
				benefit.OperationModeReversePartial,
			},
		},
	}
	restrictions := benefit.OperationSupports{
		benefit.OperationSupport{
			Operation: operationArchive,
			Supported: true,
			Modes:     []benefit.OperationMode{modeManual},
		},
		benefit.OperationSupport{
			Operation: benefit.OperationReverse,
			Supported: false,
			Remark:    "this benefit is non-reversible",
		},
	}

	effective, err := benefit.EffectiveOperationSupports(declared, restrictions)
	if err != nil {
		t.Fatal(err)
	}
	archive, _ := effective.Get(operationArchive)
	if !archive.Supported || len(archive.Modes) != 1 || archive.Modes[0] != modeManual {
		t.Fatalf("unexpected archive support: %#v", archive)
	}
	reverse, _ := effective.Get(benefit.OperationReverse)
	if reverse.Supported || reverse.Remark == "" {
		t.Fatalf("unexpected reverse support: %#v", reverse)
	}
}

func TestReverseOperationModes(t *testing.T) {
	fullOnly := benefit.OperationSupport{
		Operation: benefit.OperationReverse,
		Supported: true,
	}
	if !fullOnly.SupportsMode(benefit.OperationModeReverseFull) {
		t.Fatal("supported reverse operation does not support full reversal")
	}
	if fullOnly.SupportsMode(benefit.OperationModeReversePartial) {
		t.Fatal("reverse operation with empty modes unexpectedly supports partial reversal")
	}

	partial := fullOnly
	partial.Modes = []benefit.OperationMode{benefit.OperationModeReversePartial}
	if !partial.SupportsMode(benefit.OperationModeReverseFull) ||
		!partial.SupportsMode(benefit.OperationModeReversePartial) {
		t.Fatal("partial reverse operation does not support both full and partial reversal")
	}

	disabled := partial
	disabled.Supported = false
	if disabled.SupportsMode(benefit.OperationModeReverseFull) ||
		disabled.SupportsMode(benefit.OperationModeReversePartial) {
		t.Fatal("disabled reverse operation unexpectedly supports a mode")
	}
}

func TestOperationSupportsValidation(t *testing.T) {
	tests := []struct {
		name     string
		supports benefit.OperationSupports
	}{
		{"empty operation", benefit.OperationSupports{{}}},
		{"duplicate operation", benefit.OperationSupports{{Operation: "Archive"}, {Operation: "Archive"}}},
		{"core operation", benefit.OperationSupports{{Operation: benefit.OperationInspect}}},
		{"empty mode", benefit.OperationSupports{{Operation: "Archive", Modes: []benefit.OperationMode{""}}}},
		{"duplicate mode", benefit.OperationSupports{{Operation: "Archive", Modes: []benefit.OperationMode{"manual", "manual"}}}},
		{"invalid reverse mode", benefit.OperationSupports{{Operation: benefit.OperationReverse, Modes: []benefit.OperationMode{"manual"}}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.supports.Validate(); err == nil {
				t.Fatal("invalid operation support unexpectedly validated")
			}
		})
	}
}

func TestEffectiveOperationSupportsNarrowsPartialReverse(t *testing.T) {
	declared := benefit.OperationSupports{
		benefit.OperationSupport{
			Operation: benefit.OperationReverse,
			Supported: true,
			Modes:     []benefit.OperationMode{benefit.OperationModeReversePartial},
		},
	}
	restrictions := benefit.OperationSupports{
		benefit.OperationSupport{
			Operation: benefit.OperationReverse,
			Supported: true,
		},
	}

	effective, err := benefit.EffectiveOperationSupports(declared, restrictions)
	if err != nil {
		t.Fatal(err)
	}
	reverse, ok := effective.Get(benefit.OperationReverse)
	if !ok || !reverse.Supported {
		t.Fatalf("unexpected reverse support: %#v", reverse)
	}
	if !reverse.SupportsMode(benefit.OperationModeReverseFull) {
		t.Fatal("effective reverse operation does not support full reversal")
	}
	if reverse.SupportsMode(benefit.OperationModeReversePartial) {
		t.Fatal("full-only restriction did not remove partial reversal")
	}
}

func TestEffectiveOperationSupportsRejectsExpansion(t *testing.T) {
	declared := benefit.OperationSupports{
		benefit.OperationSupport{
			Operation: benefit.OperationReverse,
			Supported: true,
		},
	}
	restriction := benefit.OperationSupports{
		benefit.OperationSupport{
			Operation: "Archive",
			Supported: true,
		},
	}

	if _, err := benefit.EffectiveOperationSupports(declared, restriction); err == nil {
		t.Fatal("capability expansion unexpectedly succeeded")
	}
}

func TestEvaluateOperationRejectsCoreOperation(t *testing.T) {
	if _, err := benefit.EvaluateOperation(
		context.Background(),
		benefit.DefaultConstraintRegistry,
		nil,
		benefit.OperationRedeem,
		benefit.EvaluationInput{},
	); err == nil {
		t.Fatal("core operation unexpectedly used optional capability evaluation")
	}
}

func TestEvaluateOperationDecisionStatuses(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	input := benefit.EvaluationInput{Now: now}

	unsupported, err := benefit.EvaluateOperation(
		ctx,
		benefit.DefaultConstraintRegistry,
		nil,
		benefit.OperationReverse,
		input,
	)
	if err != nil {
		t.Fatal(err)
	}
	if unsupported.Status != benefit.OperationDecisionStatusUnsupported ||
		unsupported.IsSupported() || unsupported.IsEligible() ||
		unsupported.Constraints.Status != benefit.ConstraintReportStatusUnevaluated ||
		unsupported.Reason == "" {
		t.Fatalf("unexpected unsupported decision: %#v", unsupported)
	}
	if err := unsupported.Validate(); err != nil {
		t.Fatalf("unsupported decision failed validation: %v", err)
	}

	future, err := benefit.NewConstraint(
		benefit.ConstraintTimeRange,
		"operator-only operation constraint",
		benefit.TimeRangeConstraintParams{StartsAt: now.Add(time.Hour)},
	)
	if err != nil {
		t.Fatal(err)
	}
	supports := benefit.OperationSupports{
		benefit.OperationSupport{
			Operation:   benefit.OperationReverse,
			Supported:   true,
			Constraints: benefit.Constraints{future},
		},
	}
	ineligible, err := benefit.EvaluateOperation(
		ctx,
		benefit.DefaultConstraintRegistry,
		supports,
		benefit.OperationReverse,
		input,
	)
	if err != nil {
		t.Fatal(err)
	}
	if ineligible.Status != benefit.OperationDecisionStatusIneligible ||
		!ineligible.IsSupported() || ineligible.IsEligible() ||
		ineligible.Constraints.Status != benefit.ConstraintReportStatusUnsatisfied ||
		len(ineligible.Constraints.Violations) != 1 || ineligible.Reason == "" {
		t.Fatalf("unexpected ineligible decision: %#v", ineligible)
	}
	if err := ineligible.Validate(); err != nil {
		t.Fatalf("ineligible decision failed validation: %v", err)
	}
	data, err := json.Marshal(ineligible)
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(data)
	if !strings.Contains(encoded, `"status":"ineligible"`) ||
		strings.Contains(encoded, "operator-only") || strings.Contains(encoded, `"params"`) ||
		strings.Contains(encoded, `"remark"`) || strings.Contains(encoded, `"supported"`) ||
		strings.Contains(encoded, `"eligible"`) {
		t.Fatalf("operation definition leaked into decision JSON: %s", encoded)
	}

	supports[0].Constraints = nil
	eligible, err := benefit.EvaluateOperation(
		ctx,
		benefit.DefaultConstraintRegistry,
		supports,
		benefit.OperationReverse,
		input,
	)
	if err != nil {
		t.Fatal(err)
	}
	if eligible.Status != benefit.OperationDecisionStatusEligible ||
		!eligible.IsSupported() || !eligible.IsEligible() ||
		eligible.Constraints.Status != benefit.ConstraintReportStatusSatisfied ||
		eligible.Reason != "" || eligible.Details != nil {
		t.Fatalf("unexpected eligible decision: %#v", eligible)
	}
	if err := eligible.Validate(); err != nil {
		t.Fatalf("eligible decision failed validation: %v", err)
	}
}

func TestOperationDecisionRejectsInvalidStatusCombination(t *testing.T) {
	tests := map[string]benefit.OperationDecision{
		"empty operation": {Status: benefit.OperationDecisionStatusUnsupported},
		"invalid status":  {Operation: benefit.OperationReverse, Status: "invalid"},
		"unsupported with evaluated constraints": {
			Operation: benefit.OperationReverse, Status: benefit.OperationDecisionStatusUnsupported,
			Constraints: benefit.ConstraintReport{Status: benefit.ConstraintReportStatusSatisfied},
		},
		"ineligible without violation": {
			Operation: benefit.OperationReverse, Status: benefit.OperationDecisionStatusIneligible,
			Constraints: benefit.ConstraintReport{Status: benefit.ConstraintReportStatusSatisfied},
		},
		"eligible with violation": {
			Operation: benefit.OperationReverse, Status: benefit.OperationDecisionStatusEligible,
			Constraints: benefit.ConstraintReport{Status: benefit.ConstraintReportStatusUnsatisfied},
		},
		"eligible with diagnostics": {
			Operation: benefit.OperationReverse, Status: benefit.OperationDecisionStatusEligible,
			Constraints: benefit.ConstraintReport{Status: benefit.ConstraintReportStatusSatisfied},
			Diagnostic:  benefit.Diagnostic{Reason: "unexpected"},
		},
	}
	for name, decision := range tests {
		t.Run(name, func(t *testing.T) {
			if err := decision.Validate(); err == nil {
				t.Fatal("invalid operation decision unexpectedly validated")
			}
		})
	}
}

func TestEvaluateLocalEligibilityIncludesUnknownConstraints(t *testing.T) {
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	known := mustConstraint(t, benefit.ConstraintTimeRange, benefit.TimeRangeConstraintParams{
		StartsAt:  now.Add(-time.Hour),
		ExpiresAt: now.Add(time.Hour),
	})
	unknown := mustConstraint(t, "provider.special_rule", map[string]any{"enabled": true})
	input := benefit.EvaluationInput{
		Benefit: benefit.BenefitInfo{
			Status:      benefit.StatusActive,
			Constraints: benefit.Constraints{known, unknown},
		},
		Now: now,
	}

	result, err := benefit.EvaluateLocalEligibility(context.Background(), nil, input)
	if err != nil {
		t.Fatal(err)
	}
	if result.Eligible || result.Failure == nil ||
		result.Failure.Code != benefit.EvaluationFailureConstraintUnsatisfied {
		t.Fatalf("unexpected eligibility result: %#v", result)
	}
	if result.Constraints.Status != benefit.ConstraintReportStatusUnsatisfied {
		t.Fatalf("unexpected constraint report status: %#v", result.Constraints)
	}
	if result.Constraints.Unrecognized != 1 {
		t.Fatalf("unexpected unrecognized count: %#v", result.Constraints)
	}
}

func TestEvaluateLocalEligibilityDoesNotEvaluateInactiveBenefitConstraints(t *testing.T) {
	result, err := benefit.EvaluateLocalEligibility(
		context.Background(),
		nil,
		benefit.EvaluationInput{
			Benefit: benefit.BenefitInfo{Status: benefit.StatusExpired},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Eligible || result.Constraints.IsEvaluated() ||
		result.Constraints.Status != benefit.ConstraintReportStatusUnevaluated {
		t.Fatalf("unexpected inactive benefit result: %#v", result)
	}
	if result.Failure == nil || result.Failure.Code != benefit.EvaluationFailureBenefitInactive {
		t.Fatalf("unexpected inactive benefit failure: %#v", result.Failure)
	}
}

func TestResultStatusValidation(t *testing.T) {
	if !benefit.ResultStatusSuccess.IsFinal() || !benefit.ResultStatusFailure.IsFinal() {
		t.Fatal("final result statuses were not reported as final")
	}
	if benefit.ResultStatusPending.IsFinal() || benefit.ResultStatusUnknown.IsFinal() {
		t.Fatal("non-final result status was reported as final")
	}

	redemption := &benefit.Redemption{RedemptionID: "R1"}
	redeemFailure := &benefit.RedeemFailure{Code: benefit.RedeemFailureProviderRejected}
	redeemTests := []struct {
		name   string
		result benefit.RedeemResult
		valid  bool
	}{
		{"success", benefit.RedeemResult{Status: benefit.ResultStatusSuccess, Redemption: redemption}, true},
		{"failure", benefit.RedeemResult{Status: benefit.ResultStatusFailure, Failure: redeemFailure}, true},
		{"pending", benefit.RedeemResult{Status: benefit.ResultStatusPending}, true},
		{"success without record", benefit.RedeemResult{Status: benefit.ResultStatusSuccess}, false},
		{"success with failure", benefit.RedeemResult{Status: benefit.ResultStatusSuccess, Redemption: redemption, Failure: redeemFailure}, false},
		{"failure without details", benefit.RedeemResult{Status: benefit.ResultStatusFailure}, false},
		{"failure with record", benefit.RedeemResult{Status: benefit.ResultStatusFailure, Redemption: redemption, Failure: redeemFailure}, false},
		{"pending with failure", benefit.RedeemResult{Status: benefit.ResultStatusPending, Failure: redeemFailure}, false},
		{"unknown with record", benefit.RedeemResult{Status: benefit.ResultStatusUnknown, Redemption: redemption}, false},
		{"invalid status", benefit.RedeemResult{Status: "invalid"}, false},
	}
	for _, test := range redeemTests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.result.Validate(); (err == nil) != test.valid {
				t.Fatalf("unexpected validation result: %v", err)
			}
		})
	}

	reversal := &benefit.Reversal{ReversalID: "RV1", RedemptionID: "R1"}
	reverseFailure := &benefit.ReversalFailure{Code: benefit.ReversalFailureProviderRejected}
	reverseTests := []struct {
		name   string
		result benefit.ReverseResult
		valid  bool
	}{
		{"success", benefit.ReverseResult{Status: benefit.ResultStatusSuccess, Reversal: reversal}, true},
		{"failure", benefit.ReverseResult{Status: benefit.ResultStatusFailure, Failure: reverseFailure}, true},
		{"pending", benefit.ReverseResult{Status: benefit.ResultStatusPending}, true},
		{"success without record", benefit.ReverseResult{Status: benefit.ResultStatusSuccess}, false},
		{"success with failure", benefit.ReverseResult{Status: benefit.ResultStatusSuccess, Reversal: reversal, Failure: reverseFailure}, false},
		{"failure without details", benefit.ReverseResult{Status: benefit.ResultStatusFailure}, false},
		{"failure with record", benefit.ReverseResult{Status: benefit.ResultStatusFailure, Reversal: reversal, Failure: reverseFailure}, false},
		{"pending with failure", benefit.ReverseResult{Status: benefit.ResultStatusPending, Failure: reverseFailure}, false},
		{"unknown with record", benefit.ReverseResult{Status: benefit.ResultStatusUnknown, Reversal: reversal}, false},
		{"invalid status", benefit.ReverseResult{Status: "invalid"}, false},
	}
	for _, test := range reverseTests {
		t.Run("reverse "+test.name, func(t *testing.T) {
			if err := test.result.Validate(); (err == nil) != test.valid {
				t.Fatalf("unexpected validation result: %v", err)
			}
		})
	}
}

func TestMutationRecordValidation(t *testing.T) {
	redemptions := []struct {
		name   string
		record benefit.Redemption
	}{
		{"without id", benefit.Redemption{}},
		{"with invalid outcome", benefit.Redemption{RedemptionID: "R1", Outcome: benefit.BenefitOutcome{Discount: &benefit.DiscountEffect{Type: "invalid"}}}},
	}
	for _, test := range redemptions {
		t.Run(test.name, func(t *testing.T) {
			if err := test.record.Validate(); err == nil {
				t.Fatal("invalid record unexpectedly validated")
			}
		})
	}

	reversals := []struct {
		name   string
		record benefit.Reversal
	}{
		{"without id", benefit.Reversal{RedemptionID: "R1"}},
		{"without redemption id", benefit.Reversal{ReversalID: "RV1"}},
		{"with invalid currency", benefit.Reversal{ReversalID: "RV1", RedemptionID: "R1", RestoredAmount: benefit.Money{Amount: 1}}},
		{"with negative amount", benefit.Reversal{ReversalID: "RV1", RedemptionID: "R1", RestoredAmount: benefit.Money{Amount: -1, Currency: "USD"}}},
	}
	for _, test := range reversals {
		t.Run("reversal "+test.name, func(t *testing.T) {
			if err := test.record.Validate(); err == nil {
				t.Fatal("invalid record unexpectedly validated")
			}
		})
	}
}
