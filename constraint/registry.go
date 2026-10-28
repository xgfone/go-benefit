package constraint

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"sync"

	"github.com/xgfone/go-benefit"
)

// Evaluator evaluates one constraint against in-process input. It
// must treat the constraint and input as immutable and must not retain them
// after Evaluate returns.
type Evaluator interface {
	Evaluate(context.Context, benefit.Constraint, Input) (benefit.ConstraintDecision, error)
}

// EvaluatorFunc adapts a function to Evaluator.
type EvaluatorFunc func(context.Context, benefit.Constraint, Input) (benefit.ConstraintDecision, error)

// Evaluate implements Evaluator.
func (f EvaluatorFunc) Evaluate(ctx context.Context, c benefit.Constraint, input Input) (benefit.ConstraintDecision, error) {
	return f(ctx, c, input)
}

// Registry is a concurrency-safe evaluator registry.
type Registry struct {
	mu         sync.RWMutex
	evaluators map[benefit.ConstraintType]Evaluator
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		evaluators: make(map[benefit.ConstraintType]Evaluator, 16),
	}
}

// NewDefaultRegistry returns a registry initialized with RegisterBuiltins.
func NewDefaultRegistry() *Registry {
	registry := NewRegistry()
	if err := RegisterBuiltins(registry); err != nil {
		panic(err)
	}
	return registry
}

// Register adds an evaluator and rejects duplicate types.
// A nil registry returns ErrNilRegistry. Invalid types wrap
// benefit.ErrValidation; invalid evaluators and duplicate types wrap
// ErrInvalidEvaluator and ErrAlreadyRegistered respectively.
func (r *Registry) Register(typ benefit.ConstraintType, evaluator Evaluator) error {
	if r == nil {
		return ErrNilRegistry
	}
	if err := typ.Validate(); err != nil {
		return err
	}
	if evaluator == nil {
		return fmt.Errorf("%w: evaluator for constraint %q is nil", ErrInvalidEvaluator, typ)
	}
	if f, ok := evaluator.(EvaluatorFunc); ok && f == nil {
		return fmt.Errorf("%w: evaluator for constraint %q is nil", ErrInvalidEvaluator, typ)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.evaluators[typ]; exists {
		return fmt.Errorf("%w: %q", ErrAlreadyRegistered, typ)
	}

	r.evaluators[typ] = evaluator
	return nil
}

// Unregister removes an evaluator and reports whether it existed.
func (r *Registry) Unregister(typ benefit.ConstraintType) bool {
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
func (r *Registry) Get(typ benefit.ConstraintType) (Evaluator, bool) {
	if r == nil {
		return nil, false
	}

	r.mu.RLock()
	evaluator, ok := r.evaluators[typ]
	r.mu.RUnlock()

	return evaluator, ok
}

// Types returns registered constraint types in lexical order.
func (r *Registry) Types() []benefit.ConstraintType {
	if r == nil {
		return nil
	}

	r.mu.RLock()
	types := make([]benefit.ConstraintType, 0, len(r.evaluators))
	types = slices.AppendSeq(types, maps.Keys(r.evaluators))
	r.mu.RUnlock()

	slices.Sort(types)
	return types
}

// Evaluate evaluates one constraint. Unknown types are explicitly unsatisfied.
// Evaluator errors are converted into ConstraintDecisionError without exposing
// their original error or diagnostic text.
func (r *Registry) Evaluate(
	ctx context.Context,
	constraint benefit.Constraint,
	input Input,
) benefit.ConstraintDecision {
	evaluator, recognized := r.Get(constraint.Type)
	if !recognized {
		reason := fmt.Sprintf("constraint type %q is not registered", constraint.Type)
		decision := benefit.ConstraintUnsatisfied(benefit.ConstraintDecisionUnrecognized, reason, nil)
		decision.Type = constraint.Type
		return decision
	}

	decision, err := evaluator.Evaluate(ctx, constraint, input)
	if err != nil {
		decision := benefit.ConstraintUnsatisfied(benefit.ConstraintDecisionError, "constraint evaluator failed", nil)
		decision.Type = constraint.Type
		return decision
	}

	code := decision.Code
	if reason := invalidDecisionCodeReason(code); reason != "" {
		decision := benefit.ConstraintUnsatisfied(benefit.ConstraintDecisionError, reason, nil)
		decision.Type = constraint.Type
		return decision
	}

	decision.Type = constraint.Type
	if decision.IsSatisfied() {
		decision.Diagnostic = benefit.Diagnostic{}
	} else if decision.Reason == "" {
		const msg = "constraint evaluator returned a negative decision without a reason"
		decision = benefit.ConstraintUnsatisfied(benefit.ConstraintDecisionError, msg, nil)
		decision.Type = constraint.Type
	}
	return decision
}

func invalidDecisionCodeReason(code benefit.ConstraintDecisionCode) string {
	switch code {
	case
		benefit.ConstraintDecisionSatisfied,
		benefit.ConstraintDecisionUnsatisfied,
		benefit.ConstraintDecisionInvalid,
		benefit.ConstraintDecisionError:
		return ""

	case "":
		return "constraint evaluator returned an empty result code"

	case benefit.ConstraintDecisionUnrecognized:
		return "registered constraint evaluator returned the reserved unrecognized result code"

	default:
		return fmt.Sprintf("constraint evaluator returned invalid result code %q", code)
	}
}

// EvaluateAll evaluates every constraint and returns all violations.
func (r *Registry) EvaluateAll(
	ctx context.Context,
	input Input,
	constraints benefit.Constraints,
) benefit.ConstraintReport {
	report := benefit.ConstraintReport{
		Status: benefit.ConstraintReportStatusSatisfied,
	}

	for _, constraint := range constraints {
		decision := r.Evaluate(ctx, constraint, input)
		if !decision.IsSatisfied() {
			report.Status = benefit.ConstraintReportStatusUnsatisfied
			report.Violations = append(report.Violations, decision)
		}
		if !decision.IsRecognized() {
			report.Unrecognized++
		}
	}

	return report
}

// DefaultRegistry is the package-level registry used by helpers.
var DefaultRegistry = NewDefaultRegistry()

// Register registers an evaluator in the package-level registry.
func Register(typ benefit.ConstraintType, evaluator Evaluator) error {
	return DefaultRegistry.Register(typ, evaluator)
}

// MustRegister registers an evaluator in the package-level registry
// and panics if registration fails.
func MustRegister(typ benefit.ConstraintType, evaluator Evaluator) {
	if err := Register(typ, evaluator); err != nil {
		panic(err)
	}
}

// Evaluate evaluates constraints with the package-level registry.
func Evaluate(
	ctx context.Context,
	input Input,
	constraints benefit.Constraints,
) benefit.ConstraintReport {
	return DefaultRegistry.EvaluateAll(ctx, input, constraints)
}
