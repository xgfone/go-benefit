package constraint

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/xgfone/go-benefit"
)

// AmountParams configures a minimum or maximum operation amount.
// An empty Currency makes the constraint apply to every transaction currency.
type AmountParams struct {
	Amount   int64
	Currency string
}

// AmountExtractor extracts an operation amount from application-defined input.
type AmountExtractor func(Input) (benefit.Money, bool, error)

type amountConstraintEvaluator struct {
	extract AmountExtractor
	minimum bool
}

// NewMinimumAmountEvaluator returns an evaluator that uses extract
// to obtain the application-defined operation amount.
func NewMinimumAmountEvaluator(extract AmountExtractor) Evaluator {
	return amountConstraintEvaluator{extract: extract, minimum: true}
}

// NewMaximumAmountEvaluator returns an evaluator that uses extract
// to obtain the application-defined operation amount.
func NewMaximumAmountEvaluator(extract AmountExtractor) Evaluator {
	return amountConstraintEvaluator{extract: extract}
}

func (e amountConstraintEvaluator) Evaluate(
	ctx context.Context,
	constraint benefit.Constraint,
	input Input,
) (benefit.ConstraintDecision, error) {
	if err := ctx.Err(); err != nil {
		return benefit.ConstraintDecision{}, err
	}
	if e.extract == nil {
		return benefit.ConstraintDecision{}, errors.New("benefit: amount extractor is nil")
	}

	params, invalid := decodeAmountConstraint(constraint)
	if invalid != nil {
		return *invalid, nil
	}

	actual, found, err := e.extract(input)
	if err != nil {
		return benefit.ConstraintDecision{}, err
	}

	if !found {
		return benefit.ConstraintDecisionUnsatisfied.Decision("operation amount is unavailable", nil), nil
	}

	if err := actual.Validate(); err != nil {
		return benefit.ConstraintDecision{}, fmt.Errorf("benefit: invalid operation amount: %w", err)
	}
	if actual.Amount < 0 {
		return benefit.ConstraintDecision{}, errors.New("benefit: operation amount must not be negative")
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

func decodeAmountConstraint(constraint benefit.Constraint) (AmountParams, *benefit.ConstraintDecision) {
	var params AmountParams
	if err := constraint.DecodeParams(&params); err != nil {
		decision := invalidConstraint(err.Error())
		return params, &decision
	}

	if params.Amount < 0 {
		decision := invalidConstraint("amount must not be negative")
		return params, &decision
	}

	if err := (benefit.Money{Currency: params.Currency}).Validate(); err != nil {
		decision := invalidConstraint(err.Error())
		return params, &decision
	}

	params.Currency = strings.ToUpper(params.Currency)
	return params, nil
}

func compareCurrency(actual, required string) *benefit.ConstraintDecision {
	if required != "" && !strings.EqualFold(actual, required) {
		decision := benefit.ConstraintDecisionUnsatisfied.Decision(
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
