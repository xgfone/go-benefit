package benefit

// ResultStatus distinguishes final success, final failure, accepted work, and
// an uncertain outcome for a state-changing operation.
type ResultStatus string

const (
	ResultStatusPending ResultStatus = "pending"
	ResultStatusSuccess ResultStatus = "success"
	ResultStatusFailure ResultStatus = "failure"
	ResultStatusUnknown ResultStatus = "unknown"
)

// IsFinal reports whether the operation has a confirmed final result.
func (s ResultStatus) IsFinal() bool {
	return s == ResultStatusSuccess || s == ResultStatusFailure
}

// IsPending reports whether the result status is ResultStatusPending.
func (s ResultStatus) IsPending() bool {
	return s == ResultStatusPending
}

// IsSuccess reports whether the result status is ResultStatusSuccess.
func (s ResultStatus) IsSuccess() bool {
	return s == ResultStatusSuccess
}

// IsFailure reports whether the result status is ResultStatusFailure.
func (s ResultStatus) IsFailure() bool {
	return s == ResultStatusFailure
}

// IsUnknown reports whether the result status is ResultStatusUnknown.
func (s ResultStatus) IsUnknown() bool {
	return s == ResultStatusUnknown
}
