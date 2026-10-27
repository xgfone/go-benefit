package benefit

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
)

// ConstraintType identifies a constraint evaluator in a registry.
type ConstraintType string

// Validate verifies that the constraint type has a namespace.
func (t ConstraintType) Validate() error {
	return validateNamespacedValue("constraint type", string(t))
}

// Constraint is a typed, self-contained rule definition.
type Constraint struct {
	Type   ConstraintType
	Params jsontext.Value `json:",omitempty"`
	Remark string         `json:",omitempty"`
}

// Constraints is an ordered list of constraint definitions.
type Constraints []Constraint

// NewConstraint JSON-encodes params into a constraint definition.
func NewConstraint(typ ConstraintType, remark string, params any) (Constraint, error) {
	if typ == "" {
		return Constraint{}, errors.New("benefit: constraint type is empty")
	}

	var raw jsontext.Value
	if params != nil {
		data, err := json.Marshal(params)
		if err != nil {
			return Constraint{}, fmt.Errorf("benefit: encode constraint %q params: %w", typ, err)
		}
		raw = data
	}
	return Constraint{Type: typ, Remark: remark, Params: raw}, nil
}

// DecodeParams decodes the constraint parameters into out.
func (c Constraint) DecodeParams(out any) error {
	if out == nil {
		return errors.New("benefit: constraint params destination is nil")
	}
	if len(c.Params) == 0 {
		return errors.New("benefit: constraint params are empty")
	}
	if err := json.Unmarshal(c.Params, out); err != nil {
		return fmt.Errorf("benefit: decode constraint %q params: %w", c.Type, err)
	}
	return nil
}

// ConstraintDecisionCode explains how a constraint decision was produced.
type ConstraintDecisionCode string

// Decision returns a constraint decision with diagnostic information. The
// registry assigns the constraint type after evaluation.
func (c ConstraintDecisionCode) Decision(reason string, details map[string]any) ConstraintDecision {
	return ConstraintDecision{
		Code: c,
		Diagnostic: Diagnostic{
			Reason:  reason,
			Details: details,
		},
	}
}

const (
	// ConstraintDecisionSatisfied means the constraint was recognized, evaluated,
	// and satisfied.
	ConstraintDecisionSatisfied ConstraintDecisionCode = "satisfied"

	// ConstraintDecisionUnsatisfied means the constraint was recognized and
	// evaluated, but its condition was not satisfied.
	ConstraintDecisionUnsatisfied ConstraintDecisionCode = "unsatisfied"

	// ConstraintDecisionUnrecognized means no evaluator was registered for the
	// constraint type.
	ConstraintDecisionUnrecognized ConstraintDecisionCode = "unrecognized"

	// ConstraintDecisionInvalid means the constraint type was recognized, but its
	// definition or parameters were invalid.
	ConstraintDecisionInvalid ConstraintDecisionCode = "invalid"

	// ConstraintDecisionError means the registered evaluator could not complete
	// the evaluation because of an execution error.
	ConstraintDecisionError ConstraintDecisionCode = "error"
)

// ConstraintDecision is returned by a registered evaluator.
type ConstraintDecision struct {
	Type ConstraintType
	Code ConstraintDecisionCode

	Diagnostic
}

// ConstraintSatisfied returns a successful evaluator decision.
func ConstraintSatisfied() ConstraintDecision {
	return ConstraintDecision{Code: ConstraintDecisionSatisfied}
}

// ConstraintUnsatisfied returns an unsuccessful evaluator decision.
func ConstraintUnsatisfied(code ConstraintDecisionCode, reason string, details map[string]any) ConstraintDecision {
	if code == "" || code == ConstraintDecisionSatisfied {
		code = ConstraintDecisionUnsatisfied
	}

	return ConstraintDecision{
		Code: code,
		Diagnostic: Diagnostic{
			Reason:  reason,
			Details: details,
		},
	}
}

// IsSatisfied reports whether the constraint was satisfied.
func (d ConstraintDecision) IsSatisfied() bool {
	return d.Code == ConstraintDecisionSatisfied
}

// IsRecognized reports whether the constraint type had a registered evaluator.
func (d ConstraintDecision) IsRecognized() bool {
	return d.Code != ConstraintDecisionUnrecognized
}

// ConstraintReportStatus describes the aggregate constraint evaluation state.
type ConstraintReportStatus string

const (
	// ConstraintReportStatusUnevaluated means constraint evaluation did not run.
	ConstraintReportStatusUnevaluated ConstraintReportStatus = "unevaluated"

	// ConstraintReportStatusSatisfied means evaluation ran and every constraint
	// was satisfied. An evaluated empty constraint list is also satisfied.
	ConstraintReportStatusSatisfied ConstraintReportStatus = "satisfied"

	// ConstraintReportStatusUnsatisfied means evaluation ran and at least one
	// constraint was unsatisfied, unrecognized, invalid, or could not be
	// evaluated.
	ConstraintReportStatusUnsatisfied ConstraintReportStatus = "unsatisfied"
)

// ConstraintReport summarizes all constraint evaluations without
// short-circuiting and includes only decisions that were not satisfied.
type ConstraintReport struct {
	Status     ConstraintReportStatus
	Violations []ConstraintDecision `json:",omitempty"`

	Unrecognized int `json:",omitzero"`
}

// IsZero reports whether the report contains no status, violations, or
// unrecognized constraints.
func (r ConstraintReport) IsZero() bool {
	return r.Status == "" && len(r.Violations) == 0 && r.Unrecognized == 0
}

// IsEvaluated reports whether constraint evaluation ran.
func (r ConstraintReport) IsEvaluated() bool {
	return r.Status == ConstraintReportStatusSatisfied ||
		r.Status == ConstraintReportStatusUnsatisfied
}

// IsSatisfied reports whether evaluation ran and every constraint was satisfied.
func (r ConstraintReport) IsSatisfied() bool {
	return r.Status == ConstraintReportStatusSatisfied
}
