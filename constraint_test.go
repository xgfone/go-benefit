package benefit_test

import (
	"errors"
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
	if _, err := benefit.NewConstraint("", "", nil); !errors.Is(err, benefit.ErrValidation) {
		t.Fatalf("expected validation category for empty constraint type, got %v", err)
	}
	if _, err := benefit.NewConstraint("test.invalid", "", make(chan int)); !errors.Is(err, benefit.ErrInvalidConstraintParams) {
		t.Fatalf("expected invalid constraint params category, got %v", err)
	}

	for name, rule := range map[string]benefit.Constraint{
		"empty":   {Type: "test.empty"},
		"invalid": {Type: "test.invalid", Params: []byte(`{`)},
	} {
		t.Run(name, func(t *testing.T) {
			var params map[string]any
			err := rule.UnmarshalParams(&params)
			if !errors.Is(err, benefit.ErrInvalidConstraintParams) {
				t.Fatalf("expected invalid constraint params category, got %v", err)
			}
		})
	}

	err := (benefit.Constraint{Params: []byte(`{}`)}).UnmarshalParams(nil)
	if !errors.Is(err, benefit.ErrValidation) {
		t.Fatalf("expected validation category for nil destination, got %v", err)
	}
}
