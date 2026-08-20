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
}

func TestModelsOmitZeroTimes(t *testing.T) {
	inputData, err := json.Marshal(benefit.EvaluationInput{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(inputData), `"now"`) {
		t.Fatalf("zero evaluation time was not omitted: %s", inputData)
	}

	evaluationData, err := json.Marshal(benefit.EvaluationResult{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(evaluationData), `"expires_at"`) {
		t.Fatalf("zero evaluation expiry was not omitted: %s", evaluationData)
	}
	if strings.Contains(string(evaluationData), `"outcome"`) {
		t.Fatalf("zero evaluation outcome was not omitted: %s", evaluationData)
	}

	redemptionData, err := json.Marshal(benefit.Redemption{RedemptionID: "R1"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(redemptionData), `"outcome"`) {
		t.Fatalf("zero redemption outcome was not omitted: %s", redemptionData)
	}

	reversal := benefit.Reversal{
		ReversalID:   "RV1",
		RedemptionID: "R1",
	}
	reversalData, err := json.Marshal(reversal)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(reversalData), `"restored_amount"`) {
		t.Fatalf("zero restored amount was not omitted: %s", reversalData)
	}

	reversal.RestoredAmount = benefit.Money{Currency: "CNY"}
	if err := reversal.Validate(); err != nil {
		t.Fatalf("explicit zero restored amount failed validation: %v", err)
	}
	reversalData, err = json.Marshal(reversal)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(reversalData), `"restored_amount"`) {
		t.Fatalf("explicit zero restored amount was omitted: %s", reversalData)
	}
}

func TestRequestValidation(t *testing.T) {
	operationContext := struct {
		UserID string `json:"user_id"`
	}{UserID: "U1"}

	if err := (benefit.RedeemRequest{
		Context: operationContext,
	}).Validate(); err == nil {
		t.Fatal("redeem request without a redemption id unexpectedly validated")
	}
	if err := (benefit.RedeemRequest{
		RedemptionID: "R1",
		Context:      operationContext,
	}).Validate(); err != nil {
		t.Fatalf("valid redeem request failed: %v", err)
	}
	if err := (benefit.ReverseRequest{RedemptionID: "R1"}).Validate(); err == nil {
		t.Fatal("reverse request without a reversal id unexpectedly validated")
	}
	if err := (benefit.ReverseRequest{ReversalID: "RV1", RedemptionID: "R1"}).Validate(); err != nil {
		t.Fatalf("valid reverse request failed: %v", err)
	}
}

func TestRequestJSONOmitsInProcessValues(t *testing.T) {
	request := benefit.RedeemRequest{
		RedemptionID: "R1",
		Reference:    benefit.BenefitReference{Value: "BEARER-CODE"},
		Context: struct {
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

func TestConstraintsAndOperationsMarshalAsLists(t *testing.T) {
	value := benefit.BenefitInfo{
		Status:     benefit.StatusActive,
		DriverType: "test.coupon",
		Constraints: benefit.Constraints{{
			Type: "test.provider_rule",
		}},
		Operations: benefit.OperationSupports{{
			Operation: benefit.OperationReverse,
			Supported: true,
		}},
	}
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(data)
	if !strings.Contains(encoded, `"constraints":[`) || !strings.Contains(encoded, `"operations":[`) {
		t.Fatalf("constraints or operations were not encoded as lists: %s", encoded)
	}
}

func TestConstraintResultEmbeddingKeepsFlatJSON(t *testing.T) {
	result := benefit.ConstraintResult{
		Constraint: benefit.Constraint{Type: "test.example"},
		ConstraintDecision: benefit.ConstraintSatisfied(
			"constraint is satisfied",
			map[string]any{"source": "test"},
		),
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(data)
	if !strings.Contains(encoded, `"code":"satisfied"`) ||
		strings.Contains(encoded, `"ConstraintDecision"`) {
		t.Fatalf("constraint decision was not flattened: %s", encoded)
	}

	var decoded benefit.ConstraintResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if !decoded.IsSatisfied() || decoded.Message != result.Message {
		t.Fatalf("unexpected decoded result: %#v", decoded)
	}
}

func TestFailureDetailJSON(t *testing.T) {
	values := []any{
		benefit.EvaluationFailure{
			Type:   benefit.EvaluationFailureBenefitInactive,
			Detail: "benefit status is expired",
		},
		benefit.RedeemFailure{
			Type:   benefit.RedeemFailureProviderRejected,
			Detail: "provider rejection code 42",
		},
		benefit.ReversalFailure{
			Type:   benefit.ReversalFailureReversalWindowExpired,
			Detail: "reversal deadline was 2026-08-20T18:00:00+08:00",
		},
	}

	for _, value := range values {
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		encoded := string(data)
		if !strings.Contains(encoded, `"detail":`) || strings.Contains(encoded, `"message":`) {
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
		}},
	}
	if err := info.Validate(); err != nil {
		t.Fatalf("notice code format was unexpectedly validated: %v", err)
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
