package benefit

import "time"

// TypeDescriptor identifies a named type with optional display metadata.
type TypeDescriptor struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
	Icon string `json:"icon,omitempty"`
}

// Usage describes consumption independently from lifecycle status.
type Usage struct {
	RedeemedCount  int64 `json:"redeemed_count"`
	RemainingCount int64 `json:"remaining_count"`
	Unlimited      bool  `json:"unlimited,omitempty"`
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
	Code  NoticeCode  `json:"code"`
	Level NoticeLevel `json:"level"`

	Text string `json:"text"`
	Lang string `json:"lang"` // The BCP 47 language tag for Text
}

// BenefitInfo is the normalized result of inspecting a benefit.
type BenefitInfo struct {
	Name       string     `json:"name,omitempty"`
	Status     Status     `json:"status"`
	DriverType DriverType `json:"driver_type"`

	Usage       Usage             `json:"usage,omitzero"`
	Validity    Validity          `json:"validity,omitzero"`
	Constraints Constraints       `json:"-"`
	Operations  OperationSupports `json:"-"`
	Notices     []Notice          `json:"notices,omitempty"`

	// ProviderBenefitID is an optional non-secret provider record identifier.
	// Bearer credentials belong in BenefitReference instead.
	ProviderBenefitID string `json:"provider_benefit_id,omitempty"`
	ProviderData      string `json:"provider_data,omitempty"`
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
	Benefit BenefitInfo      `json:"benefit"`
	Context OperationContext `json:"-"`
	Now     time.Time        `json:"now,omitzero"`
}

func (in *EvaluationInput) evaluationTime() time.Time {
	if !in.Now.IsZero() {
		return in.Now
	}
	return time.Now()
}
