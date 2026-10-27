package benefit_test

import (
	"testing"

	"github.com/xgfone/go-benefit"
)

func TestConstraintReportIsZero(t *testing.T) {
	tests := []struct {
		name   string
		report benefit.ConstraintReport
		want   bool
	}{
		{name: "zero", want: true},
		{name: "empty violations", report: benefit.ConstraintReport{Violations: []benefit.ConstraintDecision{}}, want: true},
		{name: "status", report: benefit.ConstraintReport{Status: benefit.ConstraintReportStatusUnevaluated}},
		{name: "violation", report: benefit.ConstraintReport{Violations: []benefit.ConstraintDecision{{}}}},
		{name: "unrecognized", report: benefit.ConstraintReport{Unrecognized: 1}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.report.IsZero(); got != test.want {
				t.Fatalf("IsZero() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestConstraintDefinitionErrors(t *testing.T) {
	if _, err := benefit.NewConstraint("", "", nil); err == nil {
		t.Fatal("constraint without a type unexpectedly constructed")
	}
	if _, err := benefit.NewConstraint("test.invalid", "", make(chan int)); err == nil {
		t.Fatal("constraint with unencodable params unexpectedly constructed")
	}

	for name, rule := range map[string]benefit.Constraint{
		"empty":   {Type: "test.empty"},
		"invalid": {Type: "test.invalid", Params: []byte(`{`)},
	} {
		t.Run(name, func(t *testing.T) {
			var params map[string]any
			if err := rule.DecodeParams(&params); err == nil {
				t.Fatal("invalid params unexpectedly decoded")
			}
		})
	}
	if err := (benefit.Constraint{Params: []byte(`{}`)}).DecodeParams(nil); err == nil {
		t.Fatal("nil params destination unexpectedly accepted")
	}
}
