package constraint

import "github.com/xgfone/go-benefit"

// RegisterBuiltinEvaluators registers context-independent constraint
// evaluators. Provider- and application-defined constraints are intentionally
// excluded because the package cannot safely assume their input schemas.
func RegisterBuiltinEvaluators(registry *Registry) error {
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
