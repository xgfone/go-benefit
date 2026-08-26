package benefit

// Diagnostic contains non-stable, non-localized information intended for
// troubleshooting. Reason and Details must not be used for program logic or
// shown directly to end users. Details must contain only non-sensitive data.
type Diagnostic struct {
	Reason  string         `json:",omitempty"`
	Details map[string]any `json:",omitempty"`
}

// NewDiagnostic returns a Diagnostic initialized with reason and details.
func NewDiagnostic(reason string, details map[string]any) Diagnostic {
	return Diagnostic{Reason: reason, Details: details}
}

// AddDetail sets a detail, allocating Details if necessary.
func (d *Diagnostic) AddDetail(key string, value any) {
	if d.Details == nil {
		d.Details = make(map[string]any, 1)
	}

	d.Details[key] = value
}
