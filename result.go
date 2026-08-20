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
