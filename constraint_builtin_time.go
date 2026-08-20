package benefit

import (
	"context"
	"fmt"
	"time"
)

// TimeRangeConstraintParams configures an inclusive-start, exclusive-end range.
type TimeRangeConstraintParams struct {
	StartsAt  time.Time `json:"starts_at,omitzero"`
	ExpiresAt time.Time `json:"expires_at,omitzero"`
}

// WeekdayConstraintParams configures allowed weekdays and an optional timezone.
type WeekdayConstraintParams struct {
	Weekdays []time.Weekday `json:"weekdays"`
	Timezone string         `json:"timezone,omitempty"`
}

func evaluateTimeRange(
	ctx context.Context,
	constraint Constraint,
	input EvaluationInput,
) (ConstraintDecision, error) {
	if err := ctx.Err(); err != nil {
		return ConstraintDecision{}, err
	}

	var params TimeRangeConstraintParams
	if err := constraint.DecodeParams(&params); err != nil {
		return invalidConstraint(err.Error()), nil
	}
	if params.StartsAt.IsZero() && params.ExpiresAt.IsZero() {
		return invalidConstraint("time range requires starts_at or expires_at"), nil
	}
	if !params.StartsAt.IsZero() && !params.ExpiresAt.IsZero() && !params.StartsAt.Before(params.ExpiresAt) {
		return invalidConstraint("starts_at must be before expires_at"), nil
	}

	now := input.evaluationTime()
	satisfied := (params.StartsAt.IsZero() || !now.Before(params.StartsAt)) &&
		(params.ExpiresAt.IsZero() || now.Before(params.ExpiresAt))
	return constraintDecision(
		satisfied,
		chooseMessage(
			satisfied,
			"operation time is inside the allowed range",
			"operation time is outside the allowed range",
		),
		map[string]any{"evaluated_at": now},
	), nil
}

func evaluateWeekday(ctx context.Context, constraint Constraint, input EvaluationInput) (ConstraintDecision, error) {
	if err := ctx.Err(); err != nil {
		return ConstraintDecision{}, err
	}

	var params WeekdayConstraintParams
	if err := constraint.DecodeParams(&params); err != nil {
		return invalidConstraint(err.Error()), nil
	}
	if len(params.Weekdays) == 0 {
		return invalidConstraint("weekdays must not be empty"), nil
	}

	evaluatedAt := input.evaluationTime()
	location := evaluatedAt.Location()
	if params.Timezone != "" {
		var err error
		location, err = time.LoadLocation(params.Timezone)
		if err != nil {
			return invalidConstraint(fmt.Sprintf("invalid timezone %q", params.Timezone)), nil
		}
		evaluatedAt = evaluatedAt.In(location)
	}

	allowed := make(map[time.Weekday]struct{}, len(params.Weekdays))
	for _, weekday := range params.Weekdays {
		if weekday < time.Sunday || weekday > time.Saturday {
			return invalidConstraint(fmt.Sprintf("invalid weekday %d", weekday)), nil
		}
		allowed[weekday] = struct{}{}
	}

	_, satisfied := allowed[evaluatedAt.Weekday()]
	return constraintDecision(
		satisfied,
		chooseMessage(
			satisfied,
			"operation weekday is allowed",
			"operation weekday is not allowed",
		),
		map[string]any{
			"weekday":  evaluatedAt.Weekday().String(),
			"timezone": location.String(),
		},
	), nil
}
