// Package constraint evaluates benefit constraints using registered and
// built-in evaluators. Definitions, decisions, and aggregate reports belong to
// the core benefit package.
//
// NewRegistry returns an empty registry. RegisterBuiltins adds only the three
// evaluators that do not require application-defined extractors: TimeRange,
// Weekday, and RedemptionLimit. NewDefaultRegistry and DefaultRegistry include
// the same three evaluators. Amount and scope evaluators require extractors and
// explicit registration.
package constraint
