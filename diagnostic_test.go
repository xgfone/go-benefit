package benefit_test

import (
	"testing"

	benefit "github.com/xgfone/go-benefit"
)

func TestDiagnosticDetails(t *testing.T) {
	diagnostic := benefit.NewDiagnostic("provider rejected request", map[string]any{
		"attempt": 1,
	})
	if diagnostic.Reason != "provider rejected request" || diagnostic.Details["attempt"] != 1 {
		t.Fatalf("unexpected diagnostic: %#v", diagnostic)
	}

	diagnostic.AddDetail("provider", "example")
	diagnostic.AddDetail("provider", "replacement")
	if got := diagnostic.Details["provider"]; got != "replacement" {
		t.Fatalf("provider detail = %#v, want replacement", got)
	}

	var empty benefit.Diagnostic
	empty.AddDetail("operation", "redeem")
	if got := empty.Details["operation"]; got != "redeem" {
		t.Fatalf("operation detail = %#v, want redeem", got)
	}
}
