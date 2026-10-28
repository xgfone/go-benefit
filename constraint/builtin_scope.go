package constraint

import (
	"context"
	"fmt"
	"strings"

	"github.com/xgfone/go-benefit"
)

// ScopeMatch controls how extracted values are matched against an allow list.
type ScopeMatch string

const (
	ScopeMatchAny ScopeMatch = "any"
	ScopeMatchAll ScopeMatch = "all"
)

// ScopeParams configures an allow-list scope constraint.
//
// Values must be non-sensitive because failed decisions may expose scope values
// in Diagnostic.Details for troubleshooting.
type ScopeParams struct {
	Values []string
	Match  ScopeMatch `json:",omitempty"`
}

// ScopeValuesExtractor extracts application-defined values for scope matching.
// Returned values must be non-sensitive because failed decisions include them
// in Diagnostic.Details for troubleshooting.
type ScopeValuesExtractor func(Input) ([]string, error)

type scopeConstraintEvaluator struct {
	extract ScopeValuesExtractor
}

// NewScopeEvaluator returns an evaluator that matches values from
// extract against the allow list stored in ScopeParams.
func NewScopeEvaluator(extract ScopeValuesExtractor) Evaluator {
	return scopeConstraintEvaluator{extract: extract}
}

func (e scopeConstraintEvaluator) Evaluate(
	ctx context.Context,
	constraint benefit.Constraint,
	input Input,
) (benefit.ConstraintDecision, error) {
	if err := ctx.Err(); err != nil {
		return benefit.ConstraintDecision{}, err
	}
	if e.extract == nil {
		return benefit.ConstraintDecision{}, fmt.Errorf("%w: scope values extractor is nil", ErrInvalidEvaluator)
	}

	var params ScopeParams
	if err := constraint.UnmarshalParams(&params); err != nil {
		return invalidConstraint(err.Error()), nil
	}
	if len(params.Values) == 0 {
		return invalidConstraint("scope values must not be empty"), nil
	}
	if params.Match == "" {
		params.Match = ScopeMatchAny
	}
	if params.Match != ScopeMatchAny && params.Match != ScopeMatchAll {
		return invalidConstraint(fmt.Sprintf("unsupported scope match %q", params.Match)), nil
	}

	allowed := make(map[string]struct{}, len(params.Values))
	for _, value := range params.Values {
		if value = strings.TrimSpace(value); value != "" {
			allowed[value] = struct{}{}
		}
	}
	if len(allowed) == 0 {
		return invalidConstraint("scope values must contain a non-empty value"), nil
	}

	values, err := e.extract(input)
	if err != nil {
		return benefit.ConstraintDecision{}, err
	}

	satisfied := matchScope(values, allowed, params.Match)
	return constraintDecision(
		satisfied,
		"operation is outside the allowed scope",
		map[string]any{"match": params.Match, "values": values},
	), nil
}

func matchScope(values []string, allowed map[string]struct{}, match ScopeMatch) bool {
	if len(values) == 0 {
		return false
	}

	if match == ScopeMatchAll {
		for _, value := range values {
			if _, ok := allowed[value]; !ok || value == "" {
				return false
			}
		}
		return true
	}

	for _, value := range values {
		if _, ok := allowed[value]; ok && value != "" {
			return true
		}
	}

	return false
}
