package constraint_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/xgfone/go-benefit"
	"github.com/xgfone/go-benefit/constraint"
)

func TestConstraintRegistrationErrorCategories(t *testing.T) {
	registry := constraint.NewRegistry()
	evaluator := constraint.EvaluatorFunc(func(context.Context, benefit.Constraint, constraint.Input) (benefit.ConstraintDecision, error) {
		return benefit.ConstraintSatisfied(), nil
	})
	if err := registry.Register("test.rule", evaluator); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name     string
		err      error
		category error
	}{
		{"duplicate", registry.Register("test.rule", evaluator), constraint.ErrAlreadyRegistered},
		{"invalid type", registry.Register("invalid", evaluator), benefit.ErrValidation},
		{"nil evaluator", registry.Register("test.nil", nil), constraint.ErrInvalidEvaluator},
		{"nil evaluator function", registry.Register("test.nil", constraint.EvaluatorFunc(nil)), constraint.ErrInvalidEvaluator},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := fmt.Errorf("application: %w", test.err)
			if !errors.Is(err, test.category) {
				t.Fatalf("expected %v, got %v", test.category, err)
			}
		})
	}
}

func TestRegisterBuiltinsErrorCategories(t *testing.T) {
	t.Run("nil registry", func(t *testing.T) {
		err := constraint.RegisterBuiltins(nil)
		if !errors.Is(err, constraint.ErrNilRegistry) {
			t.Fatalf("expected nil registry category, got %v", err)
		}
	})

	t.Run("duplicate registration", func(t *testing.T) {
		registry := constraint.NewRegistry()
		if err := constraint.RegisterBuiltins(registry); err != nil {
			t.Fatal(err)
		}

		err := constraint.RegisterBuiltins(registry)
		if !errors.Is(err, constraint.ErrAlreadyRegistered) {
			t.Fatalf("expected duplicate registration category, got %v", err)
		}
	})
}

func TestBuiltinEvaluatorErrorCategories(t *testing.T) {
	rule := mustConstraint(t, constraint.MinimumAmount, constraint.AmountParams{})
	cause := errors.New("extractor failed")
	tests := []struct {
		name      string
		evaluator constraint.Evaluator
		category  error
	}{
		{"nil minimum extractor", constraint.NewMinimumAmountEvaluator(nil), constraint.ErrInvalidEvaluator},
		{"nil maximum extractor", constraint.NewMaximumAmountEvaluator(nil), constraint.ErrInvalidEvaluator},
		{"nil scope extractor", constraint.NewScopeEvaluator(nil), constraint.ErrInvalidEvaluator},
		{"negative amount", constraint.NewMinimumAmountEvaluator(func(constraint.Input) (benefit.Money, bool, error) {
			return benefit.Money{Amount: -1}, true, nil
		}), benefit.ErrValidation},
		{"invalid currency", constraint.NewMinimumAmountEvaluator(func(constraint.Input) (benefit.Money, bool, error) {
			return benefit.Money{Currency: "US1"}, true, nil
		}), benefit.ErrInvalidCurrency},
		{"extractor cause", constraint.NewMinimumAmountEvaluator(func(constraint.Input) (benefit.Money, bool, error) {
			return benefit.Money{}, false, fmt.Errorf("extract: %w", cause)
		}), cause},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := test.evaluator.Evaluate(context.Background(), rule, constraint.Input{})
			if !errors.Is(fmt.Errorf("application: %w", err), test.category) {
				t.Fatalf("expected %v, got %v", test.category, err)
			}
		})
	}
}
