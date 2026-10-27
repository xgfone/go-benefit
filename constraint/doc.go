// Package constraint evaluates benefit constraints using registered and
// built-in evaluators. Definitions, decisions, and aggregate reports belong to
// the core benefit package.
//
// NewRegistry returns an empty registry. NewDefaultRegistry and DefaultRegistry
// include time-range, weekday, and redemption-limit evaluators. Amount and scope
// evaluators require application-defined extractors and explicit registration.
package constraint
