package benefit_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	benefit "github.com/xgfone/go-benefit"
)

func TestOperationCapabilityModes(t *testing.T) {
	fullOnly := benefit.OperationCapability{
		Operation: benefit.OperationReverse,
		Modes:     []benefit.OperationMode{benefit.OperationModeReverseFull},
	}
	if !fullOnly.SupportsMode(benefit.OperationModeReverseFull) {
		t.Fatal("reverse capability does not support its declared full mode")
	}
	if fullOnly.SupportsMode(benefit.OperationModeReversePartial) {
		t.Fatal("full-only reverse capability unexpectedly supports partial mode")
	}

	fullAndPartial := fullOnly
	fullAndPartial.Modes = []benefit.OperationMode{
		benefit.OperationModeReverseFull,
		benefit.OperationModeReversePartial,
	}
	if !fullAndPartial.SupportsMode(benefit.OperationModeReverseFull) ||
		!fullAndPartial.SupportsMode(benefit.OperationModeReversePartial) {
		t.Fatal("reverse capability does not support both declared modes")
	}
}

func TestOperationCapabilitiesValidation(t *testing.T) {
	tests := []struct {
		name         string
		capabilities benefit.OperationCapabilities
	}{
		{"empty operation", benefit.OperationCapabilities{{}}},
		{"duplicate operation", benefit.OperationCapabilities{{Operation: "Archive"}, {Operation: "Archive"}}},
		{"core operation", benefit.OperationCapabilities{{Operation: benefit.OperationInspect}}},
		{"empty mode", benefit.OperationCapabilities{{Operation: "Archive", Modes: []benefit.OperationMode{""}}}},
		{"duplicate mode", benefit.OperationCapabilities{{Operation: "Archive", Modes: []benefit.OperationMode{"manual", "manual"}}}},
		{"invalid reverse mode", benefit.OperationCapabilities{{Operation: benefit.OperationReverse, Modes: []benefit.OperationMode{"manual"}}}},
		{"reverse without full mode", benefit.OperationCapabilities{{Operation: benefit.OperationReverse, Modes: []benefit.OperationMode{benefit.OperationModeReversePartial}}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.capabilities.Validate(); err == nil {
				t.Fatal("invalid operation capabilities unexpectedly validated")
			}
		})
	}
}

func TestOperationPoliciesValidation(t *testing.T) {
	condition := benefit.Constraints{{Type: "test.condition"}}
	tests := []struct {
		name     string
		policies benefit.OperationPolicies
	}{
		{"empty operation", benefit.OperationPolicies{{Disabled: true}}},
		{"core operation", benefit.OperationPolicies{{Operation: benefit.OperationInspect, Disabled: true}}},
		{"empty match mode", benefit.OperationPolicies{{Operation: "Archive", MatchModes: []benefit.OperationMode{""}, Disabled: true}}},
		{"duplicate match mode", benefit.OperationPolicies{{Operation: "Archive", MatchModes: []benefit.OperationMode{"manual", "manual"}, Disabled: true}}},
		{"invalid reverse match mode", benefit.OperationPolicies{{Operation: benefit.OperationReverse, MatchModes: []benefit.OperationMode{"manual"}, Disabled: true}}},
		{"no effect", benefit.OperationPolicies{{Operation: "Archive"}}},
		{"disabled and conditional", benefit.OperationPolicies{{Operation: "Archive", Disabled: true, Constraints: condition}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.policies.Validate(); err == nil {
				t.Fatal("invalid operation policies unexpectedly validated")
			}
		})
	}

	valid := benefit.OperationPolicies{
		{Operation: "Archive", Disabled: true},
		{Operation: "Archive", MatchModes: []benefit.OperationMode{"automatic"}, Constraints: condition},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid overlapping policies failed validation: %v", err)
	}
}

func TestEvaluateOperationRejectsCoreOperation(t *testing.T) {
	if _, err := benefit.EvaluateOperation(
		context.Background(),
		benefit.DefaultConstraintRegistry,
		nil,
		nil,
		benefit.OperationRedeem,
		"",
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
		nil,
		benefit.OperationReverse,
		benefit.OperationModeReverseFull,
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
	capabilities := benefit.OperationCapabilities{
		benefit.OperationCapability{
			Operation: benefit.OperationReverse,
			Modes: []benefit.OperationMode{
				benefit.OperationModeReverseFull,
				benefit.OperationModeReversePartial,
			},
		},
	}
	policies := benefit.OperationPolicies{{
		Operation:   benefit.OperationReverse,
		Constraints: benefit.Constraints{future},
	}}
	ineligible, err := benefit.EvaluateOperation(
		ctx,
		benefit.DefaultConstraintRegistry,
		capabilities,
		policies,
		benefit.OperationReverse,
		benefit.OperationModeReverseFull,
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

	eligibleInput := input
	eligibleInput.Now = now.Add(2 * time.Hour)
	eligible, err := benefit.EvaluateOperation(
		ctx,
		benefit.DefaultConstraintRegistry,
		capabilities,
		policies,
		benefit.OperationReverse,
		benefit.OperationModeReverseFull,
		eligibleInput,
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

func TestEvaluateOperationMatchesPoliciesByMode(t *testing.T) {
	capabilities := benefit.OperationCapabilities{{
		Operation: benefit.OperationReverse,
		Modes: []benefit.OperationMode{
			benefit.OperationModeReverseFull,
			benefit.OperationModeReversePartial,
		},
	}}
	policies := benefit.OperationPolicies{{
		Operation:  benefit.OperationReverse,
		MatchModes: []benefit.OperationMode{benefit.OperationModeReversePartial},
		Disabled:   true,
		Remark:     "operator-only partial reversal policy",
	}}

	partial, err := benefit.EvaluateOperation(
		context.Background(),
		benefit.DefaultConstraintRegistry,
		capabilities,
		policies,
		benefit.OperationReverse,
		benefit.OperationModeReversePartial,
		benefit.EvaluationInput{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if partial.Status != benefit.OperationDecisionStatusUnsupported ||
		partial.Mode != benefit.OperationModeReversePartial ||
		strings.Contains(partial.Reason, "operator-only") {
		t.Fatalf("unexpected partial reverse decision: %#v", partial)
	}

	full, err := benefit.EvaluateOperation(
		context.Background(),
		benefit.DefaultConstraintRegistry,
		capabilities,
		policies,
		benefit.OperationReverse,
		benefit.OperationModeReverseFull,
		benefit.EvaluationInput{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if full.Status != benefit.OperationDecisionStatusEligible || !full.IsEligible() {
		t.Fatalf("unexpected full reverse decision: %#v", full)
	}

	operation, err := benefit.EvaluateOperation(
		context.Background(),
		benefit.DefaultConstraintRegistry,
		capabilities,
		policies,
		benefit.OperationReverse,
		"",
		benefit.EvaluationInput{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if operation.Status != benefit.OperationDecisionStatusEligible {
		t.Fatalf("mode-specific policy unexpectedly matched operation-level evaluation: %#v", operation)
	}
}

func TestEvaluateOperationRejectsPoliciesOutsideCapability(t *testing.T) {
	capabilities := benefit.OperationCapabilities{{
		Operation: benefit.OperationReverse,
		Modes:     []benefit.OperationMode{benefit.OperationModeReverseFull},
	}}
	tests := map[string]benefit.OperationPolicies{
		"unsupported operation": {{Operation: "Archive", Disabled: true}},
		"unsupported mode": {{
			Operation:  benefit.OperationReverse,
			MatchModes: []benefit.OperationMode{benefit.OperationModeReversePartial},
			Disabled:   true,
		}},
	}

	for name, policies := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := benefit.EvaluateOperation(
				context.Background(),
				benefit.DefaultConstraintRegistry,
				capabilities,
				policies,
				benefit.OperationReverse,
				benefit.OperationModeReverseFull,
				benefit.EvaluationInput{},
			); err == nil {
				t.Fatal("operation policy outside driver capability unexpectedly evaluated")
			}
		})
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
