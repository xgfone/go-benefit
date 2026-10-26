package benefit_test

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
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
	if !strings.Contains(encoded, `"StartsAt":`) || strings.Contains(encoded, `"ExpiresAt":`) {
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
		ReversalId:   "RV1",
		RedemptionId: "R1",
	}
	tests := []struct {
		name   string
		value  any
		fields []string
	}{
		{"evaluation input", benefit.EvaluationInput{}, []string{"Now"}},
		{"evaluation result", benefit.EvaluationResult{}, []string{"ExpiresAt", "Outcome"}},
		{"redemption", benefit.Redemption{}, []string{"RedemptionId", "ProviderRedemptionId", "Outcome", "ProviderData"}},
		{"reversal", reversal, []string{"Outcome", "ProviderData"}},
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

	reversal.Outcome.Restored.Money = &benefit.Money{}
	if err := reversal.Validate(); err != nil {
		t.Fatalf("explicit zero restored amount failed validation: %v", err)
	}
	reversalData, err := json.Marshal(reversal)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(reversalData), `"Restored"`) {
		t.Fatalf("explicit zero restored amount was omitted: %s", reversalData)
	}
}

func TestMutationProviderDataJSONPlacement(t *testing.T) {
	tests := []struct {
		name  string
		value any
	}{
		{"redemption", benefit.Redemption{ProviderData: "redeem-provider-data"}},
		{"reversal", benefit.Reversal{ProviderData: "reverse-provider-data"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data, err := json.Marshal(test.value)
			if err != nil {
				t.Fatal(err)
			}

			var fields map[string]jsontext.Value
			if err := json.Unmarshal(data, &fields); err != nil {
				t.Fatal(err)
			}
			if _, ok := fields["ProviderData"]; !ok {
				t.Fatalf("top-level provider data was omitted: %s", data)
			}
		})
	}
}

func TestRequestValidation(t *testing.T) {
	driverInput := struct {
		UserId string `json:"user_id"`
	}{UserId: "U1"}
	reference := benefit.BenefitReference{Value: "B1"}

	if err := (benefit.RedeemRequest{
		DriverRequest: benefit.DriverRequest{Input: driverInput},
	}).Validate(); err == nil {
		t.Fatal("redeem request without a benefit reference unexpectedly validated")
	}
	if err := (benefit.RedeemRequest{
		DriverRequest: benefit.DriverRequest{Reference: reference, Input: driverInput},
	}).Validate(); err != nil {
		t.Fatalf("redeem request without an operation id failed: %v", err)
	}
	if err := (benefit.ReverseRequest{
		ReversalId:   "RV1",
		RedemptionId: "R1",
	}).Validate(); err == nil {
		t.Fatal("reverse request without a benefit reference unexpectedly validated")
	}
	if err := (benefit.ReverseRequest{
		DriverRequest: benefit.DriverRequest{Reference: reference},
	}).Validate(); err != nil {
		t.Fatalf("reverse request without operation ids failed: %v", err)
	}
	for _, mode := range []benefit.OperationMode{
		benefit.OperationModeReverseFull,
		benefit.OperationModeReversePartial,
	} {
		if err := (benefit.ReverseRequest{
			DriverRequest: benefit.DriverRequest{Reference: reference},
			Mode:          mode,
		}).Validate(); err != nil {
			t.Fatalf("reverse request mode %q failed: %v", mode, err)
		}
	}
	if err := (benefit.ReverseRequest{
		DriverRequest: benefit.DriverRequest{Reference: reference},
		Mode:          "invalid",
	}).Validate(); err == nil {
		t.Fatal("reverse request with invalid mode unexpectedly validated")
	}
	if !(benefit.BenefitReference{}).IsZero() || (benefit.BenefitReference{Value: "B1"}).IsZero() {
		t.Fatal("benefit reference zero detection is incorrect")
	}
}

func TestRequestJsonOmitsInProcessValues(t *testing.T) {
	request := benefit.RedeemRequest{
		DriverRequest: benefit.DriverRequest{
			Reference: benefit.BenefitReference{Value: "BEARER-CODE"},
			Input: struct {
				MerchantId string
			}{MerchantId: "M1"},
		},
		RedemptionId: "R1",
	}
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(data)
	if encoded != `{"RedemptionId":"R1"}` ||
		strings.Contains(encoded, "BEARER-CODE") || strings.Contains(encoded, "M1") {
		t.Fatalf("in-process values leaked into request JSON: %s", encoded)
	}

	reverseData, err := json.Marshal(benefit.ReverseRequest{
		DriverRequest:        request.DriverRequest,
		Mode:                 benefit.OperationModeReversePartial,
		ProviderRedemptionId: "PR1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(reverseData) != `{"ProviderRedemptionId":"PR1","Mode":"partial"}` {
		t.Fatalf("reverse request encoded unexpectedly: %s", reverseData)
	}
}

func TestBenefitInfoMarshalsConstraintAndOperationPolicyDefinitions(t *testing.T) {
	operationConstraint := benefit.Constraint{
		Type:   "test.operation_rule",
		Params: jsontext.Value(`{"minimum":2}`),
		Remark: "operation constraint",
	}
	value := benefit.BenefitInfo{
		Status:     benefit.StatusActive,
		DriverType: "test.coupon",
		Constraints: benefit.Constraints{
			{
				Type:   "test.provider_rule",
				Params: jsontext.Value(`{"minimum":1}`),
				Remark: "benefit constraint",
			},
		},
		OperationPolicies: benefit.OperationPolicies{
			{
				Operation:   benefit.OperationReverse,
				MatchModes:  []benefit.OperationMode{benefit.OperationModeReverseFull},
				Constraints: benefit.Constraints{operationConstraint},
				Remark:      "operation policy",
			},
		},
	}
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(data)
	want := `{"Status":"active","DriverType":"test.coupon","Constraints":[{"Type":"test.provider_rule","Params":{"minimum":1},"Remark":"benefit constraint"}],"OperationPolicies":[{"Operation":"Reverse","MatchModes":["full"],"Constraints":[{"Type":"test.operation_rule","Params":{"minimum":2},"Remark":"operation constraint"}],"Remark":"operation policy"}]}`
	if encoded != want {
		t.Fatalf("unexpected benefit JSON:\n got: %s\nwant: %s", encoded, want)
	}
}

func TestConstraintDecisionJsonContainsOnlyDecisionData(t *testing.T) {
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
	if !strings.Contains(encoded, `"Type":"test.example"`) ||
		!strings.Contains(encoded, `"Code":"unsatisfied"`) ||
		!strings.Contains(encoded, `"Reason":"test constraint was not satisfied"`) ||
		strings.Contains(encoded, `"Constraint"`) || strings.Contains(encoded, `"Params"`) ||
		strings.Contains(encoded, `"Remark"`) {
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

func TestFailureDiagnosticJson(t *testing.T) {
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
		if !strings.Contains(encoded, `"Code":`) || !strings.Contains(encoded, `"Reason":`) ||
			strings.Contains(encoded, `"Type":`) || strings.Contains(encoded, `"Detail":`) ||
			strings.Contains(encoded, `"Message":`) {
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
		ProviderBenefitId: "P1",

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
	if !strings.Contains(string(data), `"Lang":"en-US"`) {
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
		ProviderBenefitId: "P1",
		DriverType:        "test.coupon",
		Status:            benefit.StatusActive,
	}
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(data)
	if !strings.Contains(encoded, `"ProviderBenefitId":"P1"`) ||
		!strings.Contains(encoded, `"DriverType":"test.coupon"`) {
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

func TestBenefitRestorationMoneyJSON(t *testing.T) {
	for _, test := range []struct {
		restoration benefit.BenefitRestoration
		want        string
	}{
		{benefit.BenefitRestoration{}, `{}`},
		{benefit.BenefitRestoration{Money: &benefit.Money{}}, `{"Money":{"Amount":0}}`},
		{benefit.BenefitRestoration{Money: &benefit.Money{Amount: 2000, Currency: "CNY"}}, `{"Money":{"Amount":2000,"Currency":"CNY"}}`},
	} {
		data, err := json.Marshal(test.restoration)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != test.want {
			t.Fatalf("encoded %s, want %s", data, test.want)
		}
		var decoded benefit.BenefitRestoration
		if err = json.Unmarshal(data, &decoded); err != nil {
			t.Fatal(err)
		}
		if (decoded.Money == nil) != (test.restoration.Money == nil) {
			t.Fatal("lost explicit monetary restoration")
		}
		if decoded.Money != nil && *decoded.Money != *test.restoration.Money {
			t.Fatal("monetary restoration changed")
		}
	}
}

func TestMutationJSONPreservesPayloadPresence(t *testing.T) {
	for _, original := range []benefit.RedeemResult{
		{Status: benefit.ResultStatusSuccess, Redemption: &benefit.Redemption{}},
		{Status: benefit.ResultStatusFailure, Failure: &benefit.RedeemFailure{}},
	} {
		data, err := json.Marshal(original)
		if err != nil {
			t.Fatal(err)
		}

		var decoded benefit.RedeemResult
		if err = json.Unmarshal(data, &decoded); err != nil {
			t.Fatal(err)
		}
		if err = decoded.Validate(); err != nil {
			t.Fatalf("lost payload in %s: %v", data, err)
		}
	}

	for _, original := range []benefit.ReverseResult{
		{Status: benefit.ResultStatusSuccess, Reversal: &benefit.Reversal{}},
		{Status: benefit.ResultStatusFailure, Failure: &benefit.ReversalFailure{}},
	} {
		data, err := json.Marshal(original)
		if err != nil {
			t.Fatal(err)
		}

		var decoded benefit.ReverseResult
		if err = json.Unmarshal(data, &decoded); err != nil {
			t.Fatal(err)
		}
		if err = decoded.Validate(); err != nil {
			t.Fatalf("lost payload in %s: %v", data, err)
		}
	}
}
