package benefit

import (
	"context"
	"errors"
	"fmt"

	"github.com/xgfone/go-currency"
)

// AmountConstraintParams configures a minimum or maximum operation amount.
type AmountConstraintParams struct {
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
}

// AmountExtractor extracts an operation amount from application-defined input.
type AmountExtractor func(EvaluationInput) (Money, bool, error)

type amountConstraintEvaluator struct {
	extract AmountExtractor
	minimum bool
}

// NewMinimumAmountConstraintEvaluator returns an evaluator that uses extract
// to obtain the application-defined operation amount.
func NewMinimumAmountConstraintEvaluator(extract AmountExtractor) ConstraintEvaluator {
	return amountConstraintEvaluator{extract: extract, minimum: true}
}

// NewMaximumAmountConstraintEvaluator returns an evaluator that uses extract
// to obtain the application-defined operation amount.
func NewMaximumAmountConstraintEvaluator(extract AmountExtractor) ConstraintEvaluator {
	return amountConstraintEvaluator{extract: extract}
}

func (e amountConstraintEvaluator) Evaluate(
	ctx context.Context,
	constraint Constraint,
	input EvaluationInput,
) (ConstraintDecision, error) {
	if err := ctx.Err(); err != nil {
		return ConstraintDecision{}, err
	}
	if e.extract == nil {
		return ConstraintDecision{}, errors.New("benefit: amount extractor is nil")
	}

	params, invalid := decodeAmountConstraint(constraint)
	if invalid != nil {
		return *invalid, nil
	}

	actual, found, err := e.extract(input)
	if err != nil {
		return ConstraintDecision{}, err
	}

	if !found {
		return ConstraintDecisionUnsatisfied.Decision("operation amount is unavailable", nil), nil
	}

	if err := actual.Validate(); err != nil {
		return ConstraintDecision{}, fmt.Errorf("benefit: invalid operation amount: %w", err)
	}
	if actual.Amount < 0 {
		return ConstraintDecision{}, errors.New("benefit: operation amount must not be negative")
	}
	if decision := compareCurrency(actual.Currency, params.Currency); decision != nil {
		return *decision, nil
	}

	details := map[string]any{
		"actual_amount": actual.Amount,
		"currency":      params.Currency,
	}
	var satisfied bool
	var reason string
	if e.minimum {
		satisfied = actual.Amount >= params.Amount
		reason = "operation amount is below the minimum"
		details["minimum_amount"] = params.Amount
	} else {
		satisfied = actual.Amount <= params.Amount
		reason = "operation amount exceeds the maximum"
		details["maximum_amount"] = params.Amount
	}

	return constraintDecision(satisfied, reason, details), nil
}

func decodeAmountConstraint(constraint Constraint) (AmountConstraintParams, *ConstraintDecision) {
	var params AmountConstraintParams
	if err := constraint.DecodeParams(&params); err != nil {
		decision := invalidConstraint(err.Error())
		return params, &decision
	}

	if params.Amount < 0 {
		decision := invalidConstraint("amount must not be negative")
		return params, &decision
	}

	c, ok := currency.Get(params.Currency)
	if !ok {
		decision := invalidConstraint(fmt.Sprintf("unsupported currency %q", params.Currency))
		return params, &decision
	}

	params.Currency = c.Code
	return params, nil
}

func compareCurrency(actual, required string) *ConstraintDecision {
	actualCurrency, actualOK := currency.Get(actual)
	requiredCurrency, requiredOK := currency.Get(required)
	if !actualOK || !requiredOK || actualCurrency.Code != requiredCurrency.Code {
		decision := ConstraintDecisionUnsatisfied.Decision(
			"operation currency does not match the constraint currency",
			map[string]any{
				"actual_currency":   actual,
				"required_currency": required,
			},
		)
		return &decision
	}
	return nil
}
