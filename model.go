package benefit

import "time"

// TypeDescriptor identifies a named type with optional display metadata.
type TypeDescriptor struct {
	Type string
	Name string `json:",omitempty"`
	Icon string `json:",omitempty"`
}

// Usage describes consumption independently from lifecycle status.
type Usage struct {
	RedeemedCount  int64
	RemainingCount int64
	Unlimited      bool `json:",omitzero"`
}

// NoticeLevel controls how a human-facing notice should be presented.
type NoticeLevel string

const (
	NoticeInfo    NoticeLevel = "info"
	NoticeWarning NoticeLevel = "warning"
	NoticeError   NoticeLevel = "error"
)

// NoticeCode is a stable machine-readable notice identifier suitable for
// program logic and client localization.
type NoticeCode string

// Notice is user- or operator-facing information associated with a benefit.
// It does not represent an asynchronous notification or a machine capability.
type Notice struct {
	Code  NoticeCode
	Level NoticeLevel

	Text string
	Lang string // The BCP 47 language tag for Text
}

// BenefitInfo is the normalized result of inspecting a benefit.
type BenefitInfo struct {
	Name       string `json:",omitempty"`
	Status     Status
	DriverType DriverType `json:",omitempty"`

	Usage       Usage       `json:",omitzero"`
	Validity    Validity    `json:",omitzero"`
	Constraints Constraints `json:",omitzero"`

	// OperationPolicies contains only benefit-specific exceptions. Driver-wide
	// maximum capabilities come from DriverDescriptor.Operations.
	OperationPolicies OperationPolicies `json:",omitempty"`
	Notices           []Notice          `json:",omitempty"`

	// ProviderBenefitId is an optional non-secret provider record identifier.
	// Bearer credentials belong in BenefitReference instead.
	ProviderBenefitId string `json:",omitempty"`
	ProviderData      string `json:",omitempty"`
}

// BenefitReference optionally identifies a benefit presented to a bound driver.
//
// Value is opaque to the core package and may contain a bearer code,
// provider ID, or another sensitive driver-specific identifier.
type BenefitReference struct {
	Value string `json:"-"`
}

// EvaluationInput contains in-process data used by constraint evaluators.
// Evaluators must treat the input and all values reachable from it as immutable
// and must not retain them after Evaluate returns.
type EvaluationInput struct {
	Benefit BenefitInfo
	Input   DriverInput `json:"-"`
	Now     time.Time   `json:",omitzero"`
}

func (in *EvaluationInput) evaluationTime() time.Time {
	if !in.Now.IsZero() {
		return in.Now
	}
	return time.Now()
}
