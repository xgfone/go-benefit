package constraint

import (
	"time"

	"github.com/xgfone/go-benefit"
)

// Input contains in-process data used by constraint evaluators.
// Evaluators must treat the input and all values reachable from it as immutable
// and must not retain them after Evaluate returns.
type Input struct {
	Benefit benefit.BenefitInfo
	Input   benefit.DriverInput `json:"-"`
	Now     time.Time           `json:",omitzero"`
}

func (in *Input) evaluationTime() time.Time {
	if !in.Now.IsZero() {
		return in.Now
	}
	return time.Now()
}
