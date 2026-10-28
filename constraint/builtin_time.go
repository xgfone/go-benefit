package constraint

import (
	"context"
	"fmt"
	"time"

	"github.com/xgfone/go-benefit"
)

// TimeRangeParams configures an inclusive-start, exclusive-end range.
type TimeRangeParams struct {
	StartsAt  time.Time `json:",omitzero"`
	ExpiresAt time.Time `json:",omitzero"`
}

// WeekdayParams configures allowed weekdays and an optional timezone.
type WeekdayParams struct {
	Weekdays []time.Weekday
	Timezone string `json:",omitempty"`
}

func evaluateTimeRange(
	ctx context.Context,
	constraint benefit.Constraint,
	input Input,
) (benefit.ConstraintDecision, error) {
	if err := ctx.Err(); err != nil {
		return benefit.ConstraintDecision{}, err
	}

	var params TimeRangeParams
	if err := constraint.UnmarshalParams(&params); err != nil {
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

	const msg = "operation time is outside the allowed range"
	return constraintDecision(satisfied, msg, map[string]any{"evaluated_at": now}), nil
}

func evaluateWeekday(ctx context.Context, constraint benefit.Constraint, input Input) (benefit.ConstraintDecision, error) {
	if err := ctx.Err(); err != nil {
		return benefit.ConstraintDecision{}, err
	}

	var params WeekdayParams
	if err := constraint.UnmarshalParams(&params); err != nil {
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
		"operation weekday is not allowed",
		map[string]any{
			"weekday":  evaluatedAt.Weekday().String(),
			"timezone": location.String(),
		},
	), nil
}
