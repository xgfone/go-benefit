package constraint_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/xgfone/go-benefit"
	"github.com/xgfone/go-benefit/constraint"
)

func TestUnknownConstraintIsUnsatisfied(t *testing.T) {
	rule := mustConstraint(t, "custom.not_registered", map[string]any{"value": 1})
	report := constraint.Evaluate(context.Background(), constraint.Input{}, benefit.Constraints{rule})

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
	if decision.Type != rule.Type || decision.IsRecognized() || decision.IsSatisfied() ||
		decision.Code != benefit.ConstraintDecisionUnrecognized {
		t.Fatalf("unexpected unknown decision: %#v", decision)
	}
}

func TestUnregisteredConstraintTypeIsUnrecognized(t *testing.T) {
	rule := benefit.Constraint{Type: "not_namespaced"}
	decision := constraint.NewRegistry().Evaluate(
		context.Background(),
		rule,
		constraint.Input{},
	)

	if decision.Type != rule.Type || decision.IsSatisfied() || decision.IsRecognized() ||
		decision.Code != benefit.ConstraintDecisionUnrecognized {
		t.Fatalf("unexpected unregistered constraint decision: %#v", decision)
	}
	if err := constraint.NewRegistry().Register(
		rule.Type,
		constraint.EvaluatorFunc(func(
			context.Context,
			benefit.Constraint,
			constraint.Input,
		) (benefit.ConstraintDecision, error) {
			return benefit.ConstraintSatisfied(), nil
		}),
	); err == nil {
		t.Fatal("constraint type without a namespace unexpectedly registered")
	}
}

func TestConstraintRegistryLifecycle(t *testing.T) {
	var nilRegistry *constraint.Registry
	if err := nilRegistry.Register("test.rule", constraint.EvaluatorFunc(nil)); !errors.Is(err, constraint.ErrNilRegistry) {
		t.Fatalf("expected nil registry category, got %v", err)
	}
	if nilRegistry.Unregister("test.rule") || nilRegistry.Types() != nil {
		t.Fatal("nil registry unexpectedly contained constraints")
	}
	if _, ok := nilRegistry.Get("test.rule"); ok {
		t.Fatal("nil registry unexpectedly returned an evaluator")
	}

	evaluator := constraint.EvaluatorFunc(func(
		context.Context,
		benefit.Constraint,
		constraint.Input,
	) (benefit.ConstraintDecision, error) {
		return benefit.ConstraintSatisfied(), nil
	})
	registry := constraint.NewRegistry()
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
	minimum := mustConstraint(t, constraint.MinimumAmount, constraint.AmountParams{
		Amount:   10000,
		Currency: "CNY",
	})
	products := mustConstraint(t, productScope, constraint.ScopeParams{
		Values: []string{"P1", "P2"},
		Match:  constraint.ScopeMatchAny,
	})
	driverInput := testDriverInput{
		Amount:   benefit.Money{Amount: 12000, Currency: "CNY"},
		Products: []string{"P9", "P2"},
	}
	input := constraint.Input{
		Input: driverInput,
	}
	registry := constraint.NewRegistry()
	if err := registry.Register(
		constraint.MinimumAmount,
		constraint.NewMinimumAmountEvaluator(extractTestAmount),
	); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(
		productScope,
		constraint.NewScopeEvaluator(extractTestProducts),
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
	minimum := mustConstraint(t, constraint.MinimumAmount, constraint.AmountParams{
		Amount:   100,
		Currency: "USD",
	})
	driverInput := testDriverInput{
		Amount: benefit.Money{Amount: 10000, Currency: "CNY"},
	}
	input := constraint.Input{Input: driverInput}
	registry := constraint.NewRegistry()
	if err := registry.Register(
		constraint.MinimumAmount,
		constraint.NewMinimumAmountEvaluator(extractTestAmount),
	); err != nil {
		t.Fatal(err)
	}

	decision := registry.Evaluate(context.Background(), minimum, input)
	if decision.IsSatisfied() || decision.Code != benefit.ConstraintDecisionUnsatisfied {
		t.Fatalf("unexpected currency decision: %#v", decision)
	}
}

func TestAmountConstraintCurrencyMatching(t *testing.T) {
	registry := constraint.NewRegistry()
	if err := registry.Register(
		constraint.MinimumAmount,
		constraint.NewMinimumAmountEvaluator(extractTestAmount),
	); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		required string
		actual   string
	}{
		{name: "case insensitive", required: "usd", actual: "USD"},
		{name: "wildcard with currency", actual: "CNY"},
		{name: "wildcard with inherited currency"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			minimum := mustConstraint(t, constraint.MinimumAmount, constraint.AmountParams{
				Amount:   100,
				Currency: test.required,
			})
			input := constraint.Input{Input: testDriverInput{
				Amount: benefit.Money{Amount: 100, Currency: test.actual},
			}}
			decision := registry.Evaluate(context.Background(), minimum, input)
			if !decision.IsSatisfied() {
				t.Fatalf("currency match unexpectedly failed: %#v", decision)
			}
		})
	}
}

func TestAmountConstraintEvaluatorBranches(t *testing.T) {
	validMaximum := mustConstraint(t, constraint.MaximumAmount, constraint.AmountParams{
		Amount:   100,
		Currency: "USD",
	})
	validMinimum := validMaximum
	validMinimum.Type = constraint.MinimumAmount
	invalidAmount := mustConstraint(t, constraint.MinimumAmount, constraint.AmountParams{
		Amount:   -1,
		Currency: "USD",
	})
	invalidCurrency := mustConstraint(t, constraint.MinimumAmount, constraint.AmountParams{
		Amount:   1,
		Currency: "invalid",
	})
	maximum := constraint.NewMaximumAmountEvaluator(
		staticAmountExtractor(benefit.Money{Amount: 101, Currency: "USD"}, true, nil),
	)
	unavailable := constraint.NewMinimumAmountEvaluator(staticAmountExtractor(benefit.Money{}, false, nil))
	extractorError := constraint.NewMinimumAmountEvaluator(
		staticAmountExtractor(benefit.Money{}, false, errors.New("extract failed")),
	)
	invalidActual := constraint.NewMinimumAmountEvaluator(
		staticAmountExtractor(benefit.Money{Amount: -1, Currency: "USD"}, true, nil),
	)
	invalidActualCurrency := constraint.NewMinimumAmountEvaluator(
		staticAmountExtractor(benefit.Money{Amount: 1, Currency: "US1"}, true, nil),
	)
	available := constraint.NewMinimumAmountEvaluator(
		staticAmountExtractor(benefit.Money{Currency: "USD"}, true, nil),
	)

	tests := []struct {
		name       string
		evaluator  constraint.Evaluator
		constraint benefit.Constraint
		wantCode   benefit.ConstraintDecisionCode
		wantError  bool
	}{
		{"maximum exceeded", maximum, validMaximum, benefit.ConstraintDecisionUnsatisfied, false},
		{"amount unavailable", unavailable, validMinimum, benefit.ConstraintDecisionUnsatisfied, false},
		{"nil extractor", constraint.NewMinimumAmountEvaluator(nil), validMinimum, "", true},
		{"extractor error", extractorError, validMinimum, "", true},
		{"invalid actual amount", invalidActual, validMinimum, "", true},
		{"invalid actual currency", invalidActualCurrency, validMinimum, "", true},
		{"invalid constraint amount", available, invalidAmount, benefit.ConstraintDecisionInvalid, false},
		{"invalid constraint currency", available, invalidCurrency, benefit.ConstraintDecisionInvalid, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			decision, err := test.evaluator.Evaluate(context.Background(), test.constraint, constraint.Input{})
			if (err != nil) != test.wantError {
				t.Fatalf("unexpected error: %v", err)
			}
			if !test.wantError && decision.Code != test.wantCode {
				t.Fatalf("got decision %#v, want code %q", decision, test.wantCode)
			}
		})
	}
}

func staticAmountExtractor(money benefit.Money, found bool, err error) constraint.AmountExtractor {
	return func(constraint.Input) (benefit.Money, bool, error) {
		return money, found, err
	}
}

func TestScopeConstraintMatching(t *testing.T) {
	tests := []struct {
		name   string
		params constraint.ScopeParams
		values []string
		want   benefit.ConstraintDecisionCode
	}{
		{"default any", constraint.ScopeParams{Values: []string{"P1"}}, []string{"P2", "P1"}, benefit.ConstraintDecisionSatisfied},
		{"all", constraint.ScopeParams{Values: []string{"P1", "P2"}, Match: constraint.ScopeMatchAll}, []string{"P1", "P2"}, benefit.ConstraintDecisionSatisfied},
		{"all rejects one mismatch", constraint.ScopeParams{Values: []string{"P1"}, Match: constraint.ScopeMatchAll}, []string{"P1", "P2"}, benefit.ConstraintDecisionUnsatisfied},
		{"empty extracted scope", constraint.ScopeParams{Values: []string{"P1"}}, nil, benefit.ConstraintDecisionUnsatisfied},
		{"empty allow list", constraint.ScopeParams{}, []string{"P1"}, benefit.ConstraintDecisionInvalid},
		{"blank allow list", constraint.ScopeParams{Values: []string{" "}}, []string{"P1"}, benefit.ConstraintDecisionInvalid},
		{"invalid match", constraint.ScopeParams{Values: []string{"P1"}, Match: "none"}, []string{"P1"}, benefit.ConstraintDecisionInvalid},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			evaluator := constraint.NewScopeEvaluator(func(constraint.Input) ([]string, error) {
				return test.values, nil
			})
			decision, err := evaluator.Evaluate(
				context.Background(),
				mustConstraint(t, "test.scope", test.params),
				constraint.Input{},
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
	rule := mustConstraint(t, "test.scope", constraint.ScopeParams{Values: []string{"P1"}})
	valid := constraint.NewScopeEvaluator(func(constraint.Input) ([]string, error) {
		return []string{"P1"}, nil
	})
	extractorError := constraint.NewScopeEvaluator(func(constraint.Input) ([]string, error) {
		return nil, errors.New("extract failed")
	})
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()

	tests := []struct {
		name      string
		ctx       context.Context
		evaluator constraint.Evaluator
	}{
		{"nil extractor", context.Background(), constraint.NewScopeEvaluator(nil)},
		{"extractor error", context.Background(), extractorError},
		{"cancelled context", cancelled, valid},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := test.evaluator.Evaluate(test.ctx, rule, constraint.Input{}); err == nil {
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
		input      constraint.Input
	}{
		{"empty time range", mustConstraint(t, constraint.TimeRange, constraint.TimeRangeParams{}), constraint.Input{}},
		{"reversed time range", mustConstraint(t, constraint.TimeRange, constraint.TimeRangeParams{StartsAt: now, ExpiresAt: now}), constraint.Input{}},
		{"empty weekdays", mustConstraint(t, constraint.Weekday, constraint.WeekdayParams{}), constraint.Input{}},
		{"invalid weekday", mustConstraint(t, constraint.Weekday, constraint.WeekdayParams{Weekdays: []time.Weekday{7}}), constraint.Input{Now: now}},
		{"invalid timezone", mustConstraint(t, constraint.Weekday, constraint.WeekdayParams{Weekdays: []time.Weekday{time.Monday}, Timezone: "invalid/timezone"}), constraint.Input{Now: now}},
		{"zero redemption limit", mustConstraint(t, constraint.RedemptionLimit, constraint.RedemptionLimitParams{}), constraint.Input{}},
		{"negative redeemed count", mustConstraint(t, constraint.RedemptionLimit, constraint.RedemptionLimitParams{MaxCount: 1}), constraint.Input{Benefit: benefit.BenefitInfo{Usage: benefit.Usage{RedeemedCount: -1}}}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			decision := constraint.DefaultRegistry.Evaluate(context.Background(), test.constraint, test.input)
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

	timeRange := mustConstraint(t, constraint.TimeRange, constraint.TimeRangeParams{
		StartsAt:  starts,
		ExpiresAt: expires,
	})
	weekday := mustConstraint(t, constraint.Weekday, constraint.WeekdayParams{
		Weekdays: []time.Weekday{time.Monday},
		Timezone: "UTC",
	})
	limit := mustConstraint(t, constraint.RedemptionLimit, constraint.RedemptionLimitParams{
		MaxCount: 5,
	})
	input := constraint.Input{
		Now: now,
		Benefit: benefit.BenefitInfo{
			Usage: benefit.Usage{RedeemedCount: 4},
		},
	}

	report := constraint.Evaluate(
		context.Background(),
		input,
		benefit.Constraints{timeRange, weekday, limit},
	)
	if !report.IsSatisfied() {
		t.Fatalf("constraints unexpectedly failed: %#v", report.Violations)
	}

	input.Benefit.Usage.RedeemedCount = 5
	report = constraint.Evaluate(context.Background(), input, benefit.Constraints{limit})
	if report.IsSatisfied() || len(report.Violations) != 1 {
		t.Fatalf("limit unexpectedly satisfied: %#v", report)
	}
}

type testDriverInput struct {
	Amount   benefit.Money `json:"amount"`
	Products []string      `json:"products,omitempty"`
}

func extractTestAmount(input constraint.Input) (benefit.Money, bool, error) {
	facts, ok := input.Input.(testDriverInput)
	if !ok {
		return benefit.Money{}, false, fmt.Errorf("unexpected driver input type %T", input.Input)
	}
	return facts.Amount, true, nil
}

func extractTestProducts(input constraint.Input) ([]string, error) {
	facts, ok := input.Input.(testDriverInput)
	if !ok {
		return nil, fmt.Errorf("unexpected driver input type %T", input.Input)
	}
	return facts.Products, nil
}

func TestConstraintRegistryEvaluatorError(t *testing.T) {
	emptyRegistry := constraint.NewRegistry()
	if err := emptyRegistry.Register("test.nil", constraint.EvaluatorFunc(nil)); err == nil {
		t.Fatal("nil constraint evaluator unexpectedly registered")
	}

	registry := constraint.NewRegistry()
	err := registry.Register("test.broken", constraint.EvaluatorFunc(func(
		context.Context,
		benefit.Constraint,
		constraint.Input,
	) (benefit.ConstraintDecision, error) {
		return benefit.ConstraintDecision{}, errors.New("provider computation failed")
	}))
	if err != nil {
		t.Fatal(err)
	}

	decision := registry.Evaluate(context.Background(), benefit.Constraint{Type: "test.broken"}, constraint.Input{})
	if decision.IsSatisfied() || !decision.IsRecognized() || decision.Code != benefit.ConstraintDecisionError {
		t.Fatalf("unexpected evaluator error decision: %#v", decision)
	}
	if decision.Reason != "constraint evaluator failed" || decision.Details != nil {
		t.Fatalf("unsafe evaluator diagnostic was returned: %#v", decision)
	}
	if err := registry.Register("test.broken", constraint.EvaluatorFunc(nil)); err == nil {
		t.Fatal("duplicate constraint registration unexpectedly succeeded")
	}
}

func TestConstraintRegistryValidatesDecisionCode(t *testing.T) {
	registry := constraint.NewRegistry()
	if err := registry.Register("test.success", constraint.EvaluatorFunc(func(
		context.Context,
		benefit.Constraint,
		constraint.Input,
	) (benefit.ConstraintDecision, error) {
		return benefit.ConstraintDecision{
			Code: benefit.ConstraintDecisionSatisfied,

			Reason:  "must be discarded",
			Details: map[string]any{"secret": "must be discarded"},
		}, nil
	})); err != nil {
		t.Fatal(err)
	}

	success := registry.Evaluate(context.Background(), benefit.Constraint{Type: "test.success"}, constraint.Input{})
	if !success.IsSatisfied() || success.Type != "test.success" ||
		success.Code != benefit.ConstraintDecisionSatisfied ||
		success.Reason != "" || success.Details != nil {
		t.Fatalf("successful decision was not preserved: %#v", success)
	}

	if err := registry.Register("test.missing_reason", constraint.EvaluatorFunc(func(
		context.Context,
		benefit.Constraint,
		constraint.Input,
	) (benefit.ConstraintDecision, error) {
		return benefit.ConstraintDecision{Code: benefit.ConstraintDecisionUnsatisfied}, nil
	})); err != nil {
		t.Fatal(err)
	}
	missingReason := registry.Evaluate(
		context.Background(),
		benefit.Constraint{Type: "test.missing_reason"},
		constraint.Input{},
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
		if err := registry.Register(typ, constraint.EvaluatorFunc(func(
			context.Context,
			benefit.Constraint,
			constraint.Input,
		) (benefit.ConstraintDecision, error) {
			return benefit.ConstraintDecision{Code: code}, nil
		})); err != nil {
			t.Fatal(err)
		}

		decision := registry.Evaluate(context.Background(), benefit.Constraint{Type: typ}, constraint.Input{})
		if decision.IsSatisfied() || decision.Code != benefit.ConstraintDecisionError {
			t.Fatalf("invalid decision code %q was not rejected: %#v", code, decision)
		}
	}
}

func TestEmptyConstraintListIsEvaluatedAndSatisfied(t *testing.T) {
	report := constraint.NewRegistry().EvaluateAll(
		context.Background(),
		constraint.Input{},
		nil,
	)

	if report.Status != benefit.ConstraintReportStatusSatisfied ||
		!report.IsEvaluated() || !report.IsSatisfied() {
		t.Fatalf("unexpected empty constraint report: %#v", report)
	}
}

func mustConstraint(t *testing.T, typ benefit.ConstraintType, params any) benefit.Constraint {
	t.Helper()
	rule, err := benefit.NewConstraint(typ, "", params)
	if err != nil {
		t.Fatal(err)
	}
	return rule
}

func TestDefaultConstraintRegistryContents(t *testing.T) {
	registered := constraint.NewRegistry()
	if err := constraint.RegisterBuiltins(registered); err != nil {
		t.Fatal(err)
	}
	want := []benefit.ConstraintType{
		"benefit.redemption_limit",
		"benefit.time_range",
		"benefit.weekday",
	}
	for _, registry := range []*constraint.Registry{
		registered,
		constraint.DefaultRegistry,
		constraint.NewDefaultRegistry(),
	} {
		if got := registry.Types(); !slices.Equal(got, want) {
			t.Fatalf("unexpected built-in constraint types: %#v", got)
		}
	}
	independent := constraint.NewDefaultRegistry()
	independent.Unregister(constraint.Weekday)
	if _, ok := constraint.DefaultRegistry.Get(constraint.Weekday); !ok {
		t.Fatal("independent registry mutated the default registrations")
	}
}
