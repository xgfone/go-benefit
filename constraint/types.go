package constraint

import "github.com/xgfone/go-benefit"

// Built-in constraint types retain their stable protocol identifiers.
const (
	Weekday         benefit.ConstraintType = "benefit.weekday"
	TimeRange       benefit.ConstraintType = "benefit.time_range"
	MaximumAmount   benefit.ConstraintType = "benefit.maximum_amount"
	MinimumAmount   benefit.ConstraintType = "benefit.minimum_amount"
	RedemptionLimit benefit.ConstraintType = "benefit.redemption_limit"
)
