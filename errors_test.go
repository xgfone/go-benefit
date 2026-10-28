package benefit_test

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"testing"

	"github.com/xgfone/go-benefit"
)

func TestModelValidationErrorCategory(t *testing.T) {
	tests := map[string]interface{ Validate() error }{
		"driver type":         benefit.DriverType("invalid"),
		"constraint type":     benefit.ConstraintType("invalid"),
		"descriptor":          benefit.DriverDescriptor{Type: "test.coupon"},
		"benefit info":        benefit.BenefitInfo{DriverType: "test.coupon", Status: "invalid"},
		"redeem request":      benefit.RedeemRequest{},
		"reverse request":     benefit.ReverseRequest{},
		"evaluation result":   benefit.EvaluationResult{},
		"redeem result":       benefit.RedeemResult{Status: benefit.ResultStatusSuccess},
		"reverse result":      benefit.ReverseResult{Status: benefit.ResultStatusFailure},
		"discount effect":     benefit.DiscountEffect{Type: benefit.DiscountEffectAmount, OriginalAmount: -1},
		"benefit restoration": benefit.BenefitRestoration{Money: &benefit.Money{Amount: -1}},
	}
	for name, model := range tests {
		t.Run(name, func(t *testing.T) {
			err := model.Validate()
			if !errors.Is(err, benefit.ErrValidation) {
				t.Fatalf("expected validation category, got %v", err)
			}
			if !errors.Is(fmt.Errorf("application: %w", err), benefit.ErrValidation) {
				t.Fatal("application wrapping lost validation category")
			}
			if errors.Is(err, benefit.ErrBenefitNotFound) {
				t.Fatal("validation error matched business lookup outcome")
			}
		})
	}
}

func TestNestedCurrencyErrorCategory(t *testing.T) {
	invalid := &benefit.DiscountEffect{Type: benefit.DiscountEffectAmount, Currency: "US1"}
	outcome := benefit.BenefitOutcome{Discount: invalid}
	tests := map[string]interface{ Validate() error }{
		"money":             benefit.Money{Currency: "US"},
		"effect":            invalid,
		"outcome":           outcome,
		"evaluation result": benefit.EvaluationResult{Eligible: true, Outcome: outcome},
		"redeem result": benefit.RedeemResult{
			Status: benefit.ResultStatusSuccess, Redemption: &benefit.Redemption{Outcome: outcome},
		},
		"reverse result": benefit.ReverseResult{
			Status: benefit.ResultStatusSuccess,
			Reversal: &benefit.Reversal{Outcome: benefit.ReversalOutcome{
				Restored: benefit.BenefitRestoration{Money: &benefit.Money{Currency: "US1"}},
			}},
		},
	}
	for name, model := range tests {
		t.Run(name, func(t *testing.T) {
			err := fmt.Errorf("application: %w", model.Validate())
			if !errors.Is(err, benefit.ErrInvalidCurrency) || !errors.Is(err, benefit.ErrValidation) {
				t.Fatalf("nested validation lost currency category: %v", err)
			}
		})
	}
}

func TestConstraintErrorsPreserveCause(t *testing.T) {
	cause := errors.New("custom encoding failed")
	_, err := benefit.NewConstraint("test.rule", "", failingConstraintParams{cause})
	err = fmt.Errorf("application: %w", err)
	if !errors.Is(err, benefit.ErrInvalidConstraintParams) ||
		!errors.Is(err, benefit.ErrValidation) || !errors.Is(err, cause) {
		t.Fatalf("constraint encoding lost category or cause: %v", err)
	}
	var semantic *json.SemanticError
	if !errors.As(err, &semantic) {
		t.Fatalf("constraint encoding lost JSON error type: %v", err)
	}

	var params struct{ Amount int }
	err = (benefit.Constraint{
		Type:   "test.rule",
		Params: []byte(`{"Amount":"invalid"}`),
	}).UnmarshalParams(&params)
	err = fmt.Errorf("application: %w", err)
	if !errors.Is(err, benefit.ErrInvalidConstraintParams) || !errors.As(err, &semantic) {
		t.Fatalf("constraint decoding lost category or semantic cause: %v", err)
	}

	err = (benefit.Constraint{
		Type:   "test.rule",
		Params: []byte(`{`),
	}).UnmarshalParams(&params)
	var syntactic *jsontext.SyntacticError
	if !errors.Is(err, benefit.ErrInvalidConstraintParams) || !errors.As(err, &syntactic) {
		t.Fatalf("constraint decoding lost category or syntactic cause: %v", err)
	}
}

type failingConstraintParams struct{ err error }

func (p failingConstraintParams) MarshalJSON() ([]byte, error) {
	return nil, p.err
}
