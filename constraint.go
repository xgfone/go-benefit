package benefit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync"
)

// ConstraintType identifies a constraint evaluator in a registry.
type ConstraintType string

const (
	ConstraintWeekday         ConstraintType = "benefit.weekday"
	ConstraintTimeRange       ConstraintType = "benefit.time_range"
	ConstraintMaximumAmount   ConstraintType = "benefit.maximum_amount"
	ConstraintMinimumAmount   ConstraintType = "benefit.minimum_amount"
	ConstraintRedemptionLimit ConstraintType = "benefit.redemption_limit"
)

// Validate verifies that the constraint type has a namespace.
func (t ConstraintType) Validate() error {
	return validateNamespacedValue("constraint type", string(t))
}

// Constraint is a typed, self-contained rule definition.
type Constraint struct {
	Type   ConstraintType  `json:"type"`
	Params json.RawMessage `json:"params,omitempty"`

	Remark string `json:"remark,omitempty"`
}

// Constraints is an ordered list of constraint definitions.
type Constraints []Constraint

// NewConstraint JSON-encodes params into a constraint definition.
func NewConstraint(typ ConstraintType, remark string, params any) (Constraint, error) {
	if typ == "" {
		return Constraint{}, errors.New("benefit: constraint type is empty")
	}

	var raw json.RawMessage
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
	Type ConstraintType         `json:"type"`
	Code ConstraintDecisionCode `json:"code"`

	Diagnostic
}

// ConstraintSatisfied returns a successful evaluator decision.
func ConstraintSatisfied() ConstraintDecision {
	return ConstraintDecision{
		Code: ConstraintDecisionSatisfied,
	}
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
	Status       ConstraintReportStatus `json:"status"`
	Unrecognized int                    `json:"unrecognized,omitempty"`
	Violations   []ConstraintDecision   `json:"violations,omitempty"`
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

// ConstraintEvaluator evaluates one constraint against in-process input. It
// must treat the constraint and input as immutable and must not retain them
// after Evaluate returns.
type ConstraintEvaluator interface {
	Evaluate(context.Context, Constraint, EvaluationInput) (ConstraintDecision, error)
}

// ConstraintEvaluatorFunc adapts a function to ConstraintEvaluator.
type ConstraintEvaluatorFunc func(context.Context, Constraint, EvaluationInput) (ConstraintDecision, error)

// Evaluate implements ConstraintEvaluator.
func (f ConstraintEvaluatorFunc) Evaluate(ctx context.Context, c Constraint, input EvaluationInput) (ConstraintDecision, error) {
	return f(ctx, c, input)
}

// ConstraintRegistry is a concurrency-safe evaluator registry.
type ConstraintRegistry struct {
	mu         sync.RWMutex
	evaluators map[ConstraintType]ConstraintEvaluator
}

// NewConstraintRegistry returns an empty registry.
func NewConstraintRegistry() *ConstraintRegistry {
	return &ConstraintRegistry{
		evaluators: make(map[ConstraintType]ConstraintEvaluator, 16),
	}
}

// NewDefaultConstraintRegistry returns a registry containing built-in evaluators.
func NewDefaultConstraintRegistry() *ConstraintRegistry {
	registry := NewConstraintRegistry()
	if err := RegisterBuiltinConstraintEvaluators(registry); err != nil {
		panic(err)
	}
	return registry
}

// Register adds an evaluator and rejects duplicate types.
func (r *ConstraintRegistry) Register(typ ConstraintType, evaluator ConstraintEvaluator) error {
	if r == nil {
		return errors.New("benefit: constraint registry is nil")
	}
	if err := typ.Validate(); err != nil {
		return err
	}
	if evaluator == nil {
		return fmt.Errorf("benefit: evaluator for constraint %q is nil", typ)
	}
	if f, ok := evaluator.(ConstraintEvaluatorFunc); ok && f == nil {
		return fmt.Errorf("benefit: evaluator for constraint %q is nil", typ)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.evaluators[typ]; exists {
		return fmt.Errorf("benefit: constraint %q is already registered", typ)
	}

	r.evaluators[typ] = evaluator
	return nil
}

// Unregister removes an evaluator and reports whether it existed.
func (r *ConstraintRegistry) Unregister(typ ConstraintType) bool {
	if r == nil {
		return false
	}

	r.mu.Lock()
	_, exists := r.evaluators[typ]
	delete(r.evaluators, typ)
	r.mu.Unlock()

	return exists
}

// Get returns a registered evaluator.
func (r *ConstraintRegistry) Get(typ ConstraintType) (ConstraintEvaluator, bool) {
	if r == nil {
		return nil, false
	}

	r.mu.RLock()
	evaluator, ok := r.evaluators[typ]
	r.mu.RUnlock()

	return evaluator, ok
}

// Types returns registered constraint types in lexical order.
func (r *ConstraintRegistry) Types() []ConstraintType {
	if r == nil {
		return nil
	}

	r.mu.RLock()
	types := make([]ConstraintType, 0, len(r.evaluators))
	types = slices.AppendSeq(types, maps.Keys(r.evaluators))
	r.mu.RUnlock()

	slices.Sort(types)
	return types
}

// Evaluate evaluates one constraint. Unknown types are explicitly unsatisfied.
func (r *ConstraintRegistry) Evaluate(
	ctx context.Context,
	constraint Constraint,
	input EvaluationInput,
) ConstraintDecision {
	evaluator, recognized := r.Get(constraint.Type)
	if !recognized {
		reason := fmt.Sprintf("constraint type %q is not registered", constraint.Type)
		decision := ConstraintUnsatisfied(ConstraintDecisionUnrecognized, reason, nil)
		decision.Type = constraint.Type
		return decision
	}

	decision, err := evaluator.Evaluate(ctx, constraint, input)
	if err != nil {
		decision := ConstraintUnsatisfied(ConstraintDecisionError, "constraint evaluator failed", nil)
		decision.Type = constraint.Type
		return decision
	}

	code := decision.Code
	if reason := invalidDecisionCodeReason(code); reason != "" {
		decision := ConstraintUnsatisfied(ConstraintDecisionError, reason, nil)
		decision.Type = constraint.Type
		return decision
	}

	decision.Type = constraint.Type
	if decision.IsSatisfied() {
		decision.Diagnostic = Diagnostic{}
	} else if decision.Reason == "" {
		decision = ConstraintUnsatisfied(
			ConstraintDecisionError,
			"constraint evaluator returned a negative decision without a reason",
			nil,
		)
		decision.Type = constraint.Type
	}
	return decision
}

func invalidDecisionCodeReason(code ConstraintDecisionCode) string {
	switch code {
	case
		ConstraintDecisionSatisfied,
		ConstraintDecisionUnsatisfied,
		ConstraintDecisionInvalid,
		ConstraintDecisionError:
		return ""

	case "":
		return "constraint evaluator returned an empty result code"

	case ConstraintDecisionUnrecognized:
		return "registered constraint evaluator returned the reserved unrecognized result code"

	default:
		return fmt.Sprintf("constraint evaluator returned invalid result code %q", code)
	}
}

// EvaluateAll evaluates every constraint and returns all violations.
func (r *ConstraintRegistry) EvaluateAll(
	ctx context.Context,
	input EvaluationInput,
	constraints Constraints,
) ConstraintReport {
	report := ConstraintReport{
		Status: ConstraintReportStatusSatisfied,
	}

	for _, constraint := range constraints {
		decision := r.Evaluate(ctx, constraint, input)
		if !decision.IsSatisfied() {
			report.Status = ConstraintReportStatusUnsatisfied
			report.Violations = append(report.Violations, decision)
		}
		if !decision.IsRecognized() {
			report.Unrecognized++
		}
	}

	return report
}

// DefaultConstraintRegistry is the package-level registry used by helpers.
var DefaultConstraintRegistry = NewDefaultConstraintRegistry()

// RegisterConstraintEvaluator registers an evaluator in the package-level registry.
func RegisterConstraintEvaluator(typ ConstraintType, evaluator ConstraintEvaluator) error {
	return DefaultConstraintRegistry.Register(typ, evaluator)
}

// EvaluateConstraints evaluates constraints with the package-level registry.
func EvaluateConstraints(
	ctx context.Context,
	input EvaluationInput,
	constraints Constraints,
) ConstraintReport {
	return DefaultConstraintRegistry.EvaluateAll(ctx, input, constraints)
}
