package benefit

// RegisterBuiltinConstraintEvaluators registers context-independent constraint
// evaluators. Provider- and application-defined constraints are intentionally
// excluded because the core package cannot safely assume their input schemas.
func RegisterBuiltinConstraintEvaluators(registry *ConstraintRegistry) error {
	registrations := []struct {
		typ       ConstraintType
		evaluator ConstraintEvaluator
	}{
		{ConstraintTimeRange, ConstraintEvaluatorFunc(evaluateTimeRange)},
		{ConstraintWeekday, ConstraintEvaluatorFunc(evaluateWeekday)},
		{ConstraintRedemptionLimit, ConstraintEvaluatorFunc(evaluateRedemptionLimit)},
	}

	for _, registration := range registrations {
		if err := registry.Register(registration.typ, registration.evaluator); err != nil {
			return err
		}
	}
	return nil
}

func invalidConstraint(message string) ConstraintDecision {
	return ConstraintUnsatisfied(ConstraintResultInvalid, message, nil)
}

func constraintDecision(satisfied bool, message string, details map[string]any) ConstraintDecision {
	if satisfied {
		return ConstraintSatisfied(message, details)
	}
	return ConstraintUnsatisfied(ConstraintResultUnsatisfied, message, details)
}

func chooseMessage(condition bool, yes, no string) string {
	if condition {
		return yes
	}
	return no
}
