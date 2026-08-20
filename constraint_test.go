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
	if report.Unrecognized != 1 || len(report.Results) != 1 {
		t.Fatalf("unexpected report: %#v", report)
	}
	result := report.Results[0]
	if result.IsRecognized() || result.IsSatisfied() || result.Code != benefit.ConstraintResultUnrecognized {
		t.Fatalf("unexpected unknown result: %#v", result)
	}
}

func TestUnregisteredConstraintTypeIsUnrecognized(t *testing.T) {
	constraint := benefit.Constraint{Type: "not_namespaced"}
	result := benefit.NewConstraintRegistry().Evaluate(
		context.Background(),
		constraint,
		benefit.EvaluationInput{},
	)

	if result.IsSatisfied() || result.IsRecognized() ||
		result.Code != benefit.ConstraintResultUnrecognized {
		t.Fatalf("unexpected unregistered constraint result: %#v", result)
	}
	if err := benefit.NewConstraintRegistry().Register(
		constraint.Type,
		benefit.ConstraintEvaluatorFunc(func(
			context.Context,
			benefit.Constraint,
			benefit.EvaluationInput,
		) (benefit.ConstraintDecision, error) {
			return benefit.ConstraintSatisfied("satisfied", nil), nil
		}),
	); err == nil {
		t.Fatal("constraint type without a namespace unexpectedly registered")
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
	operationContext := testOperationContext{
		Amount:   benefit.Money{Amount: 12000, Currency: "CNY"},
		Products: []string{"P9", "P2"},
	}
	input := benefit.EvaluationInput{
		Context: operationContext,
	}
	registry := benefit.NewConstraintRegistry()
	registry.MustRegister(benefit.ConstraintMinimumAmount, benefit.NewMinimumAmountConstraintEvaluator(extractTestAmount))
	registry.MustRegister(productScope, benefit.NewScopeConstraintEvaluator(extractTestProducts))

	report := registry.EvaluateAll(
		context.Background(),
		input,
		benefit.Constraints{minimum, products},
	)
	if !report.IsSatisfied() {
		t.Fatalf("constraints unexpectedly failed: %#v", report.Violations())
	}
}

func TestAmountConstraintRejectsCurrencyMismatch(t *testing.T) {
	minimum := mustConstraint(t, benefit.ConstraintMinimumAmount, benefit.AmountConstraintParams{
		Amount:   100,
		Currency: "USD",
	})
	operationContext := testOperationContext{
		Amount: benefit.Money{Amount: 10000, Currency: "CNY"},
	}
	input := benefit.EvaluationInput{Context: operationContext}
	registry := benefit.NewConstraintRegistry()
	registry.MustRegister(benefit.ConstraintMinimumAmount, benefit.NewMinimumAmountConstraintEvaluator(extractTestAmount))

	result := registry.Evaluate(context.Background(), minimum, input)
	if result.IsSatisfied() || result.Code != benefit.ConstraintResultUnsatisfied {
		t.Fatalf("unexpected currency result: %#v", result)
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
		t.Fatalf("constraints unexpectedly failed: %#v", report.Violations())
	}

	input.Benefit.Usage.RedeemedCount = 5
	report = benefit.EvaluateConstraints(context.Background(), input, benefit.Constraints{limit})
	if report.IsSatisfied() || len(report.Violations()) != 1 {
		t.Fatalf("limit unexpectedly satisfied: %#v", report)
	}
}

type testOperationContext struct {
	Amount   benefit.Money `json:"amount"`
	Products []string      `json:"products,omitempty"`
}

func extractTestAmount(input benefit.EvaluationInput) (benefit.Money, bool, error) {
	facts, ok := input.Context.(testOperationContext)
	if !ok {
		return benefit.Money{}, false, fmt.Errorf("unexpected operation context type %T", input.Context)
	}
	return facts.Amount, true, nil
}

func extractTestProducts(input benefit.EvaluationInput) ([]string, error) {
	facts, ok := input.Context.(testOperationContext)
	if !ok {
		return nil, fmt.Errorf("unexpected operation context type %T", input.Context)
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

	result := registry.Evaluate(context.Background(), benefit.Constraint{Type: "test.broken"}, benefit.EvaluationInput{})
	if result.IsSatisfied() || !result.IsRecognized() || result.Code != benefit.ConstraintResultError {
		t.Fatalf("unexpected evaluator error result: %#v", result)
	}
	if err := registry.Register("test.broken", benefit.ConstraintEvaluatorFunc(nil)); err == nil {
		t.Fatal("duplicate constraint registration unexpectedly succeeded")
	}
}

func TestConstraintRegistryValidatesDecisionCode(t *testing.T) {
	registry := benefit.NewConstraintRegistry()
	registry.MustRegister("test.success", benefit.ConstraintEvaluatorFunc(func(
		context.Context,
		benefit.Constraint,
		benefit.EvaluationInput,
	) (benefit.ConstraintDecision, error) {
		return benefit.ConstraintSatisfied("eligible", nil), nil
	}))

	success := registry.Evaluate(context.Background(), benefit.Constraint{Type: "test.success"}, benefit.EvaluationInput{})
	if !success.IsSatisfied() || success.Code != benefit.ConstraintResultSatisfied {
		t.Fatalf("successful decision was not preserved: %#v", success)
	}

	invalidCodes := []benefit.ConstraintResultCode{
		"",
		benefit.ConstraintResultUnrecognized,
		"custom",
	}
	for i, code := range invalidCodes {
		typ := benefit.ConstraintType(fmt.Sprintf("test.invalid_%d", i))
		registry.MustRegister(typ, benefit.ConstraintEvaluatorFunc(func(
			context.Context,
			benefit.Constraint,
			benefit.EvaluationInput,
		) (benefit.ConstraintDecision, error) {
			return benefit.ConstraintDecision{Code: code}, nil
		}))

		result := registry.Evaluate(context.Background(), benefit.Constraint{Type: typ}, benefit.EvaluationInput{})
		if result.IsSatisfied() || result.Code != benefit.ConstraintResultError {
			t.Fatalf("invalid decision code %q was not rejected: %#v", code, result)
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
