package constraint

import "errors"

var (
	// ErrNilRegistry reports an operation that requires a non-nil registry.
	ErrNilRegistry = errors.New("benefit: constraint registry is nil")

	// ErrAlreadyRegistered reports registration of an existing constraint type.
	ErrAlreadyRegistered = errors.New("benefit: constraint is already registered")

	// ErrInvalidEvaluator reports a nil evaluator or a built-in evaluator
	// configured without its required extractor. Evaluator errors are observable
	// directly; Registry.Evaluate converts them into ConstraintDecisionError.
	ErrInvalidEvaluator = errors.New("benefit: invalid constraint evaluator")
)
