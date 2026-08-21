package benefit_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	benefit "github.com/xgfone/go-benefit"
)

func TestUnknownConstraintIsUnsatisfied(t *testing.T) {
	constraint := mustConstraint(t, "custom.not_registered", map[string]any{"value": 1})
	report := benefit.EvaluateConstraints(context.Background(), benefit.EvaluationInput{}, benefit.Constraints{constraint})

	if report.IsSatisfied() || report.Status != benefit.ConstraintReportStatusUnsatisfied {
		t.Fatal("unknown constraint unexpectedly satisfied")
	}
	if !report.IsEvaluated() {
		t.Fatal("unknown constraint report was not marked as evaluated")
	}
	if report.Unrecognized != 1 || len(report.Violations) != 1 {
		t.Fatalf("unexpected report: %#v", report)
	}
	decision := report.Violations[0]
	if decision.Type != constraint.Type || decision.IsRecognized() || decision.IsSatisfied() ||
		decision.Code != benefit.ConstraintDecisionUnrecognized {
		t.Fatalf("unexpected unknown decision: %#v", decision)
	}
}

func TestUnregisteredConstraintTypeIsUnrecognized(t *testing.T) {
	constraint := benefit.Constraint{Type: "not_namespaced"}
	decision := benefit.NewConstraintRegistry().Evaluate(
		context.Background(),
		constraint,
		benefit.EvaluationInput{},
	)

	if decision.Type != constraint.Type || decision.IsSatisfied() || decision.IsRecognized() ||
		decision.Code != benefit.ConstraintDecisionUnrecognized {
		t.Fatalf("unexpected unregistered constraint decision: %#v", decision)
	}
	if err := benefit.NewConstraintRegistry().Register(
		constraint.Type,
		benefit.ConstraintEvaluatorFunc(func(
			context.Context,
			benefit.Constraint,
			benefit.EvaluationInput,
		) (benefit.ConstraintDecision, error) {
			return benefit.ConstraintSatisfied(), nil
		}),
	); err == nil {
		t.Fatal("constraint type without a namespace unexpectedly registered")
	}
}

func TestConstraintDefinitionErrors(t *testing.T) {
	if _, err := benefit.NewConstraint("", "", nil); err == nil {
		t.Fatal("constraint without a type unexpectedly constructed")
	}
	if _, err := benefit.NewConstraint("test.invalid", "", make(chan int)); err == nil {
		t.Fatal("constraint with unencodable params unexpectedly constructed")
	}

	for name, constraint := range map[string]benefit.Constraint{
		"empty":   {Type: "test.empty"},
		"invalid": {Type: "test.invalid", Params: []byte(`{`)},
	} {
		t.Run(name, func(t *testing.T) {
			var params map[string]any
			if err := constraint.DecodeParams(&params); err == nil {
				t.Fatal("invalid params unexpectedly decoded")
			}
		})
	}
	if err := (benefit.Constraint{Params: []byte(`{}`)}).DecodeParams(nil); err == nil {
		t.Fatal("nil params destination unexpectedly accepted")
	}
}

func TestConstraintRegistryLifecycle(t *testing.T) {
	var nilRegistry *benefit.ConstraintRegistry
	if err := nilRegistry.Register("test.rule", benefit.ConstraintEvaluatorFunc(nil)); err == nil {
		t.Fatal("nil registry unexpectedly accepted a registration")
	}
	if nilRegistry.Unregister("test.rule") || nilRegistry.Types() != nil {
		t.Fatal("nil registry unexpectedly contained constraints")
	}
	if _, ok := nilRegistry.Get("test.rule"); ok {
		t.Fatal("nil registry unexpectedly returned an evaluator")
	}

	evaluator := benefit.ConstraintEvaluatorFunc(func(
		context.Context,
		benefit.Constraint,
		benefit.EvaluationInput,
	) (benefit.ConstraintDecision, error) {
		return benefit.ConstraintSatisfied(), nil
	})
	registry := benefit.NewConstraintRegistry()
	for _, typ := range []benefit.ConstraintType{"test.z", "test.a"} {
		if err := registry.Register(typ, evaluator); err != nil {
			t.Fatal(err)
		}
	}
	if types := registry.Types(); len(types) != 2 || types[0] != "test.a" || types[1] != "test.z" {
		t.Fatalf("constraint types were not sorted: %#v", types)
	}
	if _, ok := registry.Get("test.a"); !ok {
		t.Fatal("registered evaluator was not found")
	}
	if !registry.Unregister("test.a") || registry.Unregister("test.a") {
		t.Fatal("constraint unregister result was incorrect")
	}
}

func TestExtractedAmountAndScopeConstraints(t *testing.T) {
	const productScope benefit.ConstraintType = "test.product_scope"
	minimum := mustConstraint(t, benefit.ConstraintMinimumAmount, benefit.AmountConstraintParams{
		Amount:   10000,
		Currency: "CNY",
	})
	products := mustConstraint(t, productScope, benefit.ScopeConstraintParams{
		Values: []string{"P1", "P2"},
		Match:  benefit.ScopeMatchAny,
	})
	driverInput := testDriverInput{
		Amount:   benefit.Money{Amount: 12000, Currency: "CNY"},
		Products: []string{"P9", "P2"},
	}
	input := benefit.EvaluationInput{
		Input: driverInput,
	}
	registry := benefit.NewConstraintRegistry()
	if err := registry.Register(
		benefit.ConstraintMinimumAmount,
		benefit.NewMinimumAmountConstraintEvaluator(extractTestAmount),
	); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(
		productScope,
		benefit.NewScopeConstraintEvaluator(extractTestProducts),
	); err != nil {
		t.Fatal(err)
	}

	report := registry.EvaluateAll(
		context.Background(),
		input,
		benefit.Constraints{minimum, products},
	)
	if !report.IsSatisfied() {
		t.Fatalf("constraints unexpectedly failed: %#v", report.Violations)
	}

	input.Input = testDriverInput{Products: []string{"P9"}}
	report = registry.EvaluateAll(context.Background(), input, benefit.Constraints{products})
	if report.IsSatisfied() || len(report.Violations) != 1 {
		t.Fatalf("out-of-scope product unexpectedly satisfied: %#v", report)
	}
	values, ok := report.Violations[0].Details["values"].([]string)
	if !ok || len(values) != 1 || values[0] != "P9" {
		t.Fatalf("unexpected diagnostic scope values: %#v", report.Violations[0].Details)
	}
	if _, exists := report.Violations[0].Details["value_count"]; exists {
		t.Fatalf("unexpected scope value count: %#v", report.Violations[0].Details)
	}
}

func TestAmountConstraintRejectsCurrencyMismatch(t *testing.T) {
	minimum := mustConstraint(t, benefit.ConstraintMinimumAmount, benefit.AmountConstraintParams{
		Amount:   100,
		Currency: "USD",
	})
	driverInput := testDriverInput{
		Amount: benefit.Money{Amount: 10000, Currency: "CNY"},
	}
	input := benefit.EvaluationInput{Input: driverInput}
	registry := benefit.NewConstraintRegistry()
	if err := registry.Register(
		benefit.ConstraintMinimumAmount,
		benefit.NewMinimumAmountConstraintEvaluator(extractTestAmount),
	); err != nil {
		t.Fatal(err)
	}

	decision := registry.Evaluate(context.Background(), minimum, input)
	if decision.IsSatisfied() || decision.Code != benefit.ConstraintDecisionUnsatisfied {
		t.Fatalf("unexpected currency decision: %#v", decision)
	}
}

func TestAmountConstraintEvaluatorBranches(t *testing.T) {
	validMaximum := mustConstraint(t, benefit.ConstraintMaximumAmount, benefit.AmountConstraintParams{
		Amount:   100,
		Currency: "USD",
	})
	validMinimum := validMaximum
	validMinimum.Type = benefit.ConstraintMinimumAmount
	invalidAmount := mustConstraint(t, benefit.ConstraintMinimumAmount, benefit.AmountConstraintParams{
		Amount:   -1,
		Currency: "USD",
	})
	invalidCurrency := mustConstraint(t, benefit.ConstraintMinimumAmount, benefit.AmountConstraintParams{
		Amount:   1,
		Currency: "invalid",
	})
	maximum := benefit.NewMaximumAmountConstraintEvaluator(
		staticAmountExtractor(benefit.Money{Amount: 101, Currency: "USD"}, true, nil),
	)
	unavailable := benefit.NewMinimumAmountConstraintEvaluator(staticAmountExtractor(benefit.Money{}, false, nil))
	extractorError := benefit.NewMinimumAmountConstraintEvaluator(
		staticAmountExtractor(benefit.Money{}, false, errors.New("extract failed")),
	)
	invalidActual := benefit.NewMinimumAmountConstraintEvaluator(
		staticAmountExtractor(benefit.Money{Amount: -1, Currency: "USD"}, true, nil),
	)
	available := benefit.NewMinimumAmountConstraintEvaluator(
		staticAmountExtractor(benefit.Money{Currency: "USD"}, true, nil),
	)

	tests := []struct {
		name       string
		evaluator  benefit.ConstraintEvaluator
		constraint benefit.Constraint
		wantCode   benefit.ConstraintDecisionCode
		wantError  bool
	}{
		{"maximum exceeded", maximum, validMaximum, benefit.ConstraintDecisionUnsatisfied, false},
		{"amount unavailable", unavailable, validMinimum, benefit.ConstraintDecisionUnsatisfied, false},
		{"nil extractor", benefit.NewMinimumAmountConstraintEvaluator(nil), validMinimum, "", true},
		{"extractor error", extractorError, validMinimum, "", true},
		{"invalid actual amount", invalidActual, validMinimum, "", true},
		{"invalid constraint amount", available, invalidAmount, benefit.ConstraintDecisionInvalid, false},
		{"invalid constraint currency", available, invalidCurrency, benefit.ConstraintDecisionInvalid, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			decision, err := test.evaluator.Evaluate(context.Background(), test.constraint, benefit.EvaluationInput{})
			if (err != nil) != test.wantError {
				t.Fatalf("unexpected error: %v", err)
			}
			if !test.wantError && decision.Code != test.wantCode {
				t.Fatalf("got decision %#v, want code %q", decision, test.wantCode)
			}
		})
	}
}

func staticAmountExtractor(money benefit.Money, found bool, err error) benefit.AmountExtractor {
	return func(benefit.EvaluationInput) (benefit.Money, bool, error) {
		return money, found, err
	}
}

func TestScopeConstraintMatching(t *testing.T) {
	tests := []struct {
		name   string
		params benefit.ScopeConstraintParams
		values []string
		want   benefit.ConstraintDecisionCode
	}{
		{"default any", benefit.ScopeConstraintParams{Values: []string{"P1"}}, []string{"P2", "P1"}, benefit.ConstraintDecisionSatisfied},
		{"all", benefit.ScopeConstraintParams{Values: []string{"P1", "P2"}, Match: benefit.ScopeMatchAll}, []string{"P1", "P2"}, benefit.ConstraintDecisionSatisfied},
		{"all rejects one mismatch", benefit.ScopeConstraintParams{Values: []string{"P1"}, Match: benefit.ScopeMatchAll}, []string{"P1", "P2"}, benefit.ConstraintDecisionUnsatisfied},
		{"empty extracted scope", benefit.ScopeConstraintParams{Values: []string{"P1"}}, nil, benefit.ConstraintDecisionUnsatisfied},
		{"empty allow list", benefit.ScopeConstraintParams{}, []string{"P1"}, benefit.ConstraintDecisionInvalid},
		{"blank allow list", benefit.ScopeConstraintParams{Values: []string{" "}}, []string{"P1"}, benefit.ConstraintDecisionInvalid},
		{"invalid match", benefit.ScopeConstraintParams{Values: []string{"P1"}, Match: "none"}, []string{"P1"}, benefit.ConstraintDecisionInvalid},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			evaluator := benefit.NewScopeConstraintEvaluator(func(benefit.EvaluationInput) ([]string, error) {
				return test.values, nil
			})
			decision, err := evaluator.Evaluate(
				context.Background(),
				mustConstraint(t, "test.scope", test.params),
				benefit.EvaluationInput{},
			)
			if err != nil {
				t.Fatal(err)
			}
			if decision.Code != test.want {
				t.Fatalf("got decision %#v, want code %q", decision, test.want)
			}
		})
	}
}

func TestScopeConstraintEvaluatorErrors(t *testing.T) {
	constraint := mustConstraint(t, "test.scope", benefit.ScopeConstraintParams{Values: []string{"P1"}})
	valid := benefit.NewScopeConstraintEvaluator(func(benefit.EvaluationInput) ([]string, error) {
		return []string{"P1"}, nil
	})
	extractorError := benefit.NewScopeConstraintEvaluator(func(benefit.EvaluationInput) ([]string, error) {
		return nil, errors.New("extract failed")
	})
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()

	tests := []struct {
		name      string
		ctx       context.Context
		evaluator benefit.ConstraintEvaluator
	}{
		{"nil extractor", context.Background(), benefit.NewScopeConstraintEvaluator(nil)},
		{"extractor error", context.Background(), extractorError},
		{"cancelled context", cancelled, valid},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := test.evaluator.Evaluate(test.ctx, constraint, benefit.EvaluationInput{}); err == nil {
				t.Fatal("scope evaluation unexpectedly succeeded")
			}
		})
	}
}

func TestBuiltinConstraintValidation(t *testing.T) {
	now := time.Date(2026, 8, 17, 10, 0, 0, 0, time.UTC)
	tests := []struct {
		name       string
		constraint benefit.Constraint
		input      benefit.EvaluationInput
	}{
		{"empty time range", mustConstraint(t, benefit.ConstraintTimeRange, benefit.TimeRangeConstraintParams{}), benefit.EvaluationInput{}},
		{"reversed time range", mustConstraint(t, benefit.ConstraintTimeRange, benefit.TimeRangeConstraintParams{StartsAt: now, ExpiresAt: now}), benefit.EvaluationInput{}},
		{"empty weekdays", mustConstraint(t, benefit.ConstraintWeekday, benefit.WeekdayConstraintParams{}), benefit.EvaluationInput{}},
		{"invalid weekday", mustConstraint(t, benefit.ConstraintWeekday, benefit.WeekdayConstraintParams{Weekdays: []time.Weekday{7}}), benefit.EvaluationInput{Now: now}},
		{"invalid timezone", mustConstraint(t, benefit.ConstraintWeekday, benefit.WeekdayConstraintParams{Weekdays: []time.Weekday{time.Monday}, Timezone: "invalid/timezone"}), benefit.EvaluationInput{Now: now}},
		{"zero redemption limit", mustConstraint(t, benefit.ConstraintRedemptionLimit, benefit.RedemptionLimitConstraintParams{}), benefit.EvaluationInput{}},
		{"negative redeemed count", mustConstraint(t, benefit.ConstraintRedemptionLimit, benefit.RedemptionLimitConstraintParams{MaxCount: 1}), benefit.EvaluationInput{Benefit: benefit.BenefitInfo{Usage: benefit.Usage{RedeemedCount: -1}}}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			decision := benefit.DefaultConstraintRegistry.Evaluate(context.Background(), test.constraint, test.input)
			if decision.Code != benefit.ConstraintDecisionInvalid {
				t.Fatalf("got decision %#v, want invalid", decision)
			}
		})
	}
}

func TestTimeWeekdayAndRedemptionLimitConstraints(t *testing.T) {
	now := time.Date(2026, 8, 17, 10, 0, 0, 0, time.UTC)
	starts := now.Add(-time.Hour)
	expires := now.Add(time.Hour)

	timeRange := mustConstraint(t, benefit.ConstraintTimeRange, benefit.TimeRangeConstraintParams{
		StartsAt:  starts,
		ExpiresAt: expires,
	})
	weekday := mustConstraint(t, benefit.ConstraintWeekday, benefit.WeekdayConstraintParams{
		Weekdays: []time.Weekday{time.Monday},
		Timezone: "UTC",
	})
	limit := mustConstraint(t, benefit.ConstraintRedemptionLimit, benefit.RedemptionLimitConstraintParams{
		MaxCount: 5,
	})
	input := benefit.EvaluationInput{
		Now: now,
		Benefit: benefit.BenefitInfo{
			Usage: benefit.Usage{RedeemedCount: 4},
		},
	}

	report := benefit.EvaluateConstraints(
		context.Background(),
		input,
		benefit.Constraints{timeRange, weekday, limit},
	)
	if !report.IsSatisfied() {
		t.Fatalf("constraints unexpectedly failed: %#v", report.Violations)
	}

	input.Benefit.Usage.RedeemedCount = 5
	report = benefit.EvaluateConstraints(context.Background(), input, benefit.Constraints{limit})
	if report.IsSatisfied() || len(report.Violations) != 1 {
		t.Fatalf("limit unexpectedly satisfied: %#v", report)
	}
}

type testDriverInput struct {
	Amount   benefit.Money `json:"amount"`
	Products []string      `json:"products,omitempty"`
}

func extractTestAmount(input benefit.EvaluationInput) (benefit.Money, bool, error) {
	facts, ok := input.Input.(testDriverInput)
	if !ok {
		return benefit.Money{}, false, fmt.Errorf("unexpected driver input type %T", input.Input)
	}
	return facts.Amount, true, nil
}

func extractTestProducts(input benefit.EvaluationInput) ([]string, error) {
	facts, ok := input.Input.(testDriverInput)
	if !ok {
		return nil, fmt.Errorf("unexpected driver input type %T", input.Input)
	}
	return facts.Products, nil
}

func TestConstraintRegistryEvaluatorError(t *testing.T) {
	emptyRegistry := benefit.NewConstraintRegistry()
	if err := emptyRegistry.Register("test.nil", benefit.ConstraintEvaluatorFunc(nil)); err == nil {
		t.Fatal("nil constraint evaluator unexpectedly registered")
	}

	registry := benefit.NewConstraintRegistry()
	err := registry.Register("test.broken", benefit.ConstraintEvaluatorFunc(func(
		context.Context,
		benefit.Constraint,
		benefit.EvaluationInput,
	) (benefit.ConstraintDecision, error) {
		return benefit.ConstraintDecision{}, errors.New("provider computation failed")
	}))
	if err != nil {
		t.Fatal(err)
	}

	decision := registry.Evaluate(context.Background(), benefit.Constraint{Type: "test.broken"}, benefit.EvaluationInput{})
	if decision.IsSatisfied() || !decision.IsRecognized() || decision.Code != benefit.ConstraintDecisionError {
		t.Fatalf("unexpected evaluator error decision: %#v", decision)
	}
	if decision.Reason != "constraint evaluator failed" || decision.Details != nil {
		t.Fatalf("unsafe evaluator diagnostic was returned: %#v", decision)
	}
	if err := registry.Register("test.broken", benefit.ConstraintEvaluatorFunc(nil)); err == nil {
		t.Fatal("duplicate constraint registration unexpectedly succeeded")
	}
}

func TestConstraintRegistryValidatesDecisionCode(t *testing.T) {
	registry := benefit.NewConstraintRegistry()
	if err := registry.Register("test.success", benefit.ConstraintEvaluatorFunc(func(
		context.Context,
		benefit.Constraint,
		benefit.EvaluationInput,
	) (benefit.ConstraintDecision, error) {
		return benefit.ConstraintDecision{
			Code: benefit.ConstraintDecisionSatisfied,
			Diagnostic: benefit.Diagnostic{
				Reason:  "must be discarded",
				Details: map[string]any{"secret": "must be discarded"},
			},
		}, nil
	})); err != nil {
		t.Fatal(err)
	}

	success := registry.Evaluate(context.Background(), benefit.Constraint{Type: "test.success"}, benefit.EvaluationInput{})
	if !success.IsSatisfied() || success.Type != "test.success" ||
		success.Code != benefit.ConstraintDecisionSatisfied ||
		success.Reason != "" || success.Details != nil {
		t.Fatalf("successful decision was not preserved: %#v", success)
	}

	if err := registry.Register("test.missing_reason", benefit.ConstraintEvaluatorFunc(func(
		context.Context,
		benefit.Constraint,
		benefit.EvaluationInput,
	) (benefit.ConstraintDecision, error) {
		return benefit.ConstraintDecision{Code: benefit.ConstraintDecisionUnsatisfied}, nil
	})); err != nil {
		t.Fatal(err)
	}
	missingReason := registry.Evaluate(
		context.Background(),
		benefit.Constraint{Type: "test.missing_reason"},
		benefit.EvaluationInput{},
	)
	if missingReason.Code != benefit.ConstraintDecisionError || missingReason.Reason == "" {
		t.Fatalf("negative decision without a reason was not rejected: %#v", missingReason)
	}

	invalidCodes := []benefit.ConstraintDecisionCode{
		"",
		benefit.ConstraintDecisionUnrecognized,
		"custom",
	}
	for i, code := range invalidCodes {
		typ := benefit.ConstraintType(fmt.Sprintf("test.invalid_%d", i))
		if err := registry.Register(typ, benefit.ConstraintEvaluatorFunc(func(
			context.Context,
			benefit.Constraint,
			benefit.EvaluationInput,
		) (benefit.ConstraintDecision, error) {
			return benefit.ConstraintDecision{Code: code}, nil
		})); err != nil {
			t.Fatal(err)
		}

		decision := registry.Evaluate(context.Background(), benefit.Constraint{Type: typ}, benefit.EvaluationInput{})
		if decision.IsSatisfied() || decision.Code != benefit.ConstraintDecisionError {
			t.Fatalf("invalid decision code %q was not rejected: %#v", code, decision)
		}
	}
}

func TestEmptyConstraintListIsEvaluatedAndSatisfied(t *testing.T) {
	report := benefit.NewConstraintRegistry().EvaluateAll(
		context.Background(),
		benefit.EvaluationInput{},
		nil,
	)

	if report.Status != benefit.ConstraintReportStatusSatisfied ||
		!report.IsEvaluated() || !report.IsSatisfied() {
		t.Fatalf("unexpected empty constraint report: %#v", report)
	}
}

func mustConstraint(t *testing.T, typ benefit.ConstraintType, params any) benefit.Constraint {
	t.Helper()
	constraint, err := benefit.NewConstraint(typ, "", params)
	if err != nil {
		t.Fatal(err)
	}
	return constraint
}
