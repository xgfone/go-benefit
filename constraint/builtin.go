package constraint

import "github.com/xgfone/go-benefit"

// RegisterBuiltins registers the three built-in evaluators that do not require
// application-defined extractors: TimeRange, Weekday, and RedemptionLimit.
// Amount and scope evaluators require extractors and explicit registration.
func RegisterBuiltins(registry *Registry) error {
	registrations := []struct {
		typ       benefit.ConstraintType
		evaluator Evaluator
	}{
		{TimeRange, EvaluatorFunc(evaluateTimeRange)},
		{Weekday, EvaluatorFunc(evaluateWeekday)},
		{RedemptionLimit, EvaluatorFunc(evaluateRedemptionLimit)},
	}

	for _, registration := range registrations {
		if err := registry.Register(registration.typ, registration.evaluator); err != nil {
			return err
		}
	}
	return nil
}

func invalidConstraint(message string) benefit.ConstraintDecision {
	return benefit.ConstraintUnsatisfied(benefit.ConstraintDecisionInvalid, message, nil)
}

func constraintDecision(satisfied bool, reason string, details map[string]any) benefit.ConstraintDecision {
	if satisfied {
		return benefit.ConstraintSatisfied()
	}
	return benefit.ConstraintUnsatisfied(benefit.ConstraintDecisionUnsatisfied, reason, details)
}
