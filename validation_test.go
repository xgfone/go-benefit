package benefit_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	benefit "github.com/xgfone/go-benefit"
)

func TestValidityOmitsZeroTimes(t *testing.T) {
	data, err := json.Marshal(benefit.Validity{})
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{}` {
		t.Fatalf("zero validity encoded as %s, want {}", data)
	}
	if !(benefit.Validity{}).IsZero() {
		t.Fatal("zero validity was not reported as zero")
	}

	startsAt := time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC)
	data, err = json.Marshal(benefit.Validity{StartsAt: startsAt})
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(data)
	if !strings.Contains(encoded, `"starts_at":`) || strings.Contains(encoded, `"expires_at":`) {
		t.Fatalf("unexpected validity JSON: %s", encoded)
	}
	if (benefit.Validity{StartsAt: startsAt}).IsZero() {
		t.Fatal("non-zero validity was reported as zero")
	}
}

func TestSimpleValueValidation(t *testing.T) {
	now := time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		err  error
	}{
		{"invalid status", benefit.Status("invalid").Validate()},
		{"reversed validity", (benefit.Validity{StartsAt: now, ExpiresAt: now}).Validate()},
		{"negative redeemed count", (benefit.Usage{RedeemedCount: -1}).Validate()},
		{"negative remaining count", (benefit.Usage{RemainingCount: -1}).Validate()},
		{"notice without code", (benefit.Notice{Level: benefit.NoticeInfo, Text: "text", Lang: "en"}).Validate()},
		{"notice with invalid level", (benefit.Notice{Code: "test.notice", Level: "invalid", Text: "text", Lang: "en"}).Validate()},
		{"notice without text", (benefit.Notice{Code: "test.notice", Level: benefit.NoticeInfo, Lang: "en"}).Validate()},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.err == nil {
				t.Fatal("invalid value unexpectedly validated")
			}
		})
	}
}

func TestModelsOmitZeroValues(t *testing.T) {
	reversal := benefit.Reversal{
		ReversalID:   "RV1",
		RedemptionID: "R1",
	}
	tests := []struct {
		name   string
		value  any
		fields []string
	}{
		{"evaluation input", benefit.EvaluationInput{}, []string{"now"}},
		{"evaluation result", benefit.EvaluationResult{}, []string{"expires_at", "outcome"}},
		{"redemption", benefit.Redemption{RedemptionID: "R1"}, []string{"outcome"}},
		{"reversal", reversal, []string{"restored_amount"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data, err := json.Marshal(test.value)
			if err != nil {
				t.Fatal(err)
			}
			for _, field := range test.fields {
				if strings.Contains(string(data), `"`+field+`"`) {
					t.Fatalf("zero field %q was not omitted: %s", field, data)
				}
			}
		})
	}

	reversal.RestoredAmount = benefit.Money{Currency: "CNY"}
	if err := reversal.Validate(); err != nil {
		t.Fatalf("explicit zero restored amount failed validation: %v", err)
	}
	reversalData, err := json.Marshal(reversal)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(reversalData), `"restored_amount"`) {
		t.Fatalf("explicit zero restored amount was omitted: %s", reversalData)
	}
}

func TestRequestValidation(t *testing.T) {
	driverInput := struct {
		UserID string `json:"user_id"`
	}{UserID: "U1"}

	if err := (benefit.RedeemRequest{
		Input: driverInput,
	}).Validate(); err == nil {
		t.Fatal("redeem request without a redemption id unexpectedly validated")
	}
	if err := (benefit.RedeemRequest{
		RedemptionID: "R1",
		Input:        driverInput,
	}).Validate(); err != nil {
		t.Fatalf("valid redeem request failed: %v", err)
	}
	if err := (benefit.ReverseRequest{RedemptionID: "R1"}).Validate(); err == nil {
		t.Fatal("reverse request without a reversal id unexpectedly validated")
	}
	if err := (benefit.ReverseRequest{ReversalID: "RV1"}).Validate(); err == nil {
		t.Fatal("reverse request without a redemption id unexpectedly validated")
	}
	if err := (benefit.ReverseRequest{ReversalID: "RV1", RedemptionID: "R1"}).Validate(); err != nil {
		t.Fatalf("valid reverse request failed: %v", err)
	}
	if !(benefit.BenefitReference{}).IsZero() || (benefit.BenefitReference{Value: "B1"}).IsZero() {
		t.Fatal("benefit reference zero detection is incorrect")
	}
}

func TestRequestJSONOmitsInProcessValues(t *testing.T) {
	request := benefit.RedeemRequest{
		RedemptionID: "R1",
		Reference:    benefit.BenefitReference{Value: "BEARER-CODE"},
		Input: struct {
			MerchantID string
		}{MerchantID: "M1"},
	}
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(data)
	if encoded != `{"redemption_id":"R1"}` ||
		strings.Contains(encoded, "BEARER-CODE") || strings.Contains(encoded, "M1") {
		t.Fatalf("in-process values leaked into request JSON: %s", encoded)
	}
}

func TestBenefitInfoDoesNotMarshalConstraintOrOperationPolicyDefinitions(t *testing.T) {
	operationConstraint := benefit.Constraint{
		Type:   "test.operation_rule",
		Params: json.RawMessage(`{"secret":"operation-secret"}`),
		Remark: "operator-only operation constraint",
	}
	value := benefit.BenefitInfo{
		Status:     benefit.StatusActive,
		DriverType: "test.coupon",
		Constraints: benefit.Constraints{
			benefit.Constraint{
				Type:   "test.provider_rule",
				Params: json.RawMessage(`{"secret":"constraint-secret"}`),
				Remark: "operator-only constraint remark",
			},
		},
		OperationPolicies: benefit.OperationPolicies{
			benefit.OperationPolicy{
				Operation:   benefit.OperationReverse,
				MatchModes:  []benefit.OperationMode{benefit.OperationModeReverseFull},
				Constraints: benefit.Constraints{operationConstraint},
				Remark:      "operator-only operation policy",
			},
		},
	}
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(data)
	if strings.Contains(encoded, `"constraints"`) || strings.Contains(encoded, `"operation_policies"`) ||
		strings.Contains(encoded, "constraint-secret") || strings.Contains(encoded, "operation-secret") ||
		strings.Contains(encoded, "operator-only") {
		t.Fatalf("constraint or operation policy definition leaked into benefit JSON: %s", encoded)
	}
}

func TestConstraintDecisionJSONContainsOnlyDecisionData(t *testing.T) {
	decision := benefit.ConstraintDecision{
		Type: "test.example",
		Code: benefit.ConstraintDecisionUnsatisfied,
		Diagnostic: benefit.Diagnostic{
			Reason:  "test constraint was not satisfied",
			Details: map[string]any{"source": "test"},
		},
	}

	data, err := json.Marshal(decision)
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(data)
	if !strings.Contains(encoded, `"type":"test.example"`) ||
		!strings.Contains(encoded, `"code":"unsatisfied"`) ||
		!strings.Contains(encoded, `"reason":"test constraint was not satisfied"`) ||
		strings.Contains(encoded, `"constraint"`) || strings.Contains(encoded, `"params"`) ||
		strings.Contains(encoded, `"remark"`) {
		t.Fatalf("unexpected constraint decision JSON: %s", encoded)
	}

	var decoded benefit.ConstraintDecision
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.IsSatisfied() || decoded.Reason != decision.Reason || decoded.Type != decision.Type {
		t.Fatalf("unexpected decoded decision: %#v", decoded)
	}
}

func TestFailureDiagnosticJSON(t *testing.T) {
	values := []any{
		benefit.EvaluationFailure{
			Code:       benefit.EvaluationFailureBenefitInactive,
			Diagnostic: benefit.Diagnostic{Reason: "benefit status is expired"},
		},
		benefit.RedeemFailure{
			Code:       benefit.RedeemFailureProviderRejected,
			Diagnostic: benefit.Diagnostic{Reason: "provider rejection code 42"},
		},
		benefit.ReversalFailure{
			Code:       benefit.ReversalFailureReversalWindowExpired,
			Diagnostic: benefit.Diagnostic{Reason: "reversal deadline was 2026-08-20T18:00:00+08:00"},
		},
	}

	for _, value := range values {
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		encoded := string(data)
		if !strings.Contains(encoded, `"code":`) || !strings.Contains(encoded, `"reason":`) ||
			strings.Contains(encoded, `"type":`) || strings.Contains(encoded, `"detail":`) ||
			strings.Contains(encoded, `"message":`) {
			t.Fatalf("unexpected failure JSON: %s", encoded)
		}
	}
}

func TestConstraintTypeNamespaceValidation(t *testing.T) {
	if err := (benefit.ConstraintType("test.rule")).Validate(); err != nil {
		t.Fatalf("valid constraint type failed: %v", err)
	}
	if err := (benefit.ConstraintType("rule")).Validate(); err == nil {
		t.Fatal("constraint type without a namespace unexpectedly validated")
	}
}

func TestBenefitInfoDoesNotValidateNoticeCodeFormat(t *testing.T) {
	info := benefit.BenefitInfo{
		ProviderBenefitID: "P1",

		Status:     benefit.StatusActive,
		DriverType: "test.coupon",
		Notices: []benefit.Notice{{
			Code:  "ProviderWarning",
			Level: benefit.NoticeWarning,
			Text:  "Provider-specific warning",
			Lang:  "en-US",
		}},
	}
	if err := info.Validate(); err != nil {
		t.Fatalf("notice code format was unexpectedly validated: %v", err)
	}
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"lang":"en-US"`) {
		t.Fatalf("notice language was not encoded: %s", data)
	}
}

func TestBenefitInfoRequiresNoticeLanguage(t *testing.T) {
	info := benefit.BenefitInfo{
		Status:     benefit.StatusActive,
		DriverType: "test.coupon",
		Notices: []benefit.Notice{{
			Code:  "test.notice",
			Level: benefit.NoticeInfo,
			Text:  "Localized notice",
		}},
	}
	if err := info.Validate(); err == nil {
		t.Fatal("notice without a language unexpectedly validated")
	}
}

func TestBenefitInfoRequiresDriverType(t *testing.T) {
	if err := (benefit.BenefitInfo{Status: benefit.StatusActive}).Validate(); err == nil {
		t.Fatal("benefit info without a driver type unexpectedly validated")
	}

	info := benefit.BenefitInfo{
		ProviderBenefitID: "P1",
		DriverType:        "test.coupon",
		Status:            benefit.StatusActive,
	}
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(data)
	if !strings.Contains(encoded, `"provider_benefit_id":"P1"`) ||
		!strings.Contains(encoded, `"driver_type":"test.coupon"`) {
		t.Fatalf("unexpected benefit info JSON: %s", encoded)
	}
}

func TestDefaultConstraintRegistryContents(t *testing.T) {
	types := benefit.DefaultConstraintRegistry.Types()
	if len(types) != 3 {
		t.Fatalf("got %d built-in constraint types, want 3: %#v", len(types), types)
	}
	for _, typ := range types {
		if !strings.HasPrefix(string(typ), "benefit.") {
			t.Fatalf("built-in constraint type is not namespaced: %q", typ)
		}
	}
}
