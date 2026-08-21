package benefit

import "context"

// RedemptionLimitConstraintParams sets the maximum cumulative redemption count.
type RedemptionLimitConstraintParams struct {
	MaxCount int64 `json:"max_count"`
}

func evaluateRedemptionLimit(
	ctx context.Context,
	constraint Constraint,
	input EvaluationInput,
) (ConstraintDecision, error) {
	if err := ctx.Err(); err != nil {
		return ConstraintDecision{}, err
	}

	var params RedemptionLimitConstraintParams
	if err := constraint.DecodeParams(&params); err != nil {
		return invalidConstraint(err.Error()), nil
	}
	if params.MaxCount <= 0 {
		return invalidConstraint("max_count must be greater than zero"), nil
	}
	if input.Benefit.Usage.RedeemedCount < 0 {
		return invalidConstraint("redeemed_count must not be negative"), nil
	}

	satisfied := input.Benefit.Usage.RedeemedCount < params.MaxCount
	return constraintDecision(
		satisfied,
		"redemption count would exceed the limit",
		map[string]any{
			"redeemed_count": input.Benefit.Usage.RedeemedCount,
			"max_count":      params.MaxCount,
		},
	), nil
}
