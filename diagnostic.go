package benefit

// Diagnostic contains non-stable, non-localized information intended for
// troubleshooting. Reason and Details must not be used for program logic or
// shown directly to end users. Details must contain only non-sensitive data.
type Diagnostic struct {
	Reason  string         `json:"reason,omitempty"`
	Details map[string]any `json:"details,omitempty"`
}
