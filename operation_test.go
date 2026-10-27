package benefit_test

import (
	"testing"

	"github.com/xgfone/go-benefit"
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

func TestResultStatusValidation(t *testing.T) {
	if !benefit.ResultStatusSuccess.IsFinal() || !benefit.ResultStatusFailure.IsFinal() {
		t.Fatal("final result statuses were not reported as final")
	}
	if benefit.ResultStatusPending.IsFinal() || benefit.ResultStatusUnknown.IsFinal() {
		t.Fatal("non-final result status was reported as final")
	}

	redemption := &benefit.Redemption{}
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

	reversal := &benefit.Reversal{}
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
	if err := (benefit.Redemption{}).Validate(); err != nil {
		t.Fatalf("redemption without ids failed validation: %v", err)
	}
	invalidRedemption := benefit.Redemption{
		Outcome: benefit.BenefitOutcome{
			Discount: &benefit.DiscountEffect{Type: "invalid"},
		},
	}
	if err := invalidRedemption.Validate(); err == nil {
		t.Fatal("redemption with an invalid outcome unexpectedly validated")
	}

	if err := (benefit.Reversal{}).Validate(); err != nil {
		t.Fatalf("reversal without ids failed validation: %v", err)
	}

	reversals := []struct {
		name   string
		record benefit.Reversal
	}{
		{"with invalid reversed outcome", benefit.Reversal{Outcome: benefit.ReversalOutcome{
			Reversed: benefit.BenefitOutcome{Discount: &benefit.DiscountEffect{Type: "invalid"}},
		}}},
		{"with invalid restored currency", benefit.Reversal{Outcome: benefit.ReversalOutcome{
			Restored: benefit.BenefitRestoration{Money: &benefit.Money{Amount: 1, Currency: "US1"}},
		}}},
		{"with negative restored amount", benefit.Reversal{Outcome: benefit.ReversalOutcome{
			Restored: benefit.BenefitRestoration{Money: &benefit.Money{Amount: -1, Currency: "USD"}},
		}}},
	}
	for _, test := range reversals {
		t.Run("reversal "+test.name, func(t *testing.T) {
			if err := test.record.Validate(); err == nil {
				t.Fatal("invalid record unexpectedly validated")
			}
		})
	}
}
