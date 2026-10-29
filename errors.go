package benefit

import (
	"errors"
	"fmt"
)

var (
	// ErrValidation reports an invalid model, request, or constraint argument.
	// More specific validation categories below also wrap ErrValidation.
	ErrValidation = errors.New("benefit: validation failed")

	// ErrIdMismatch reports a caller or provider operation ID inconsistent with
	// the corresponding request. It does not imply that no remote effect occurred.
	ErrIdMismatch = fmt.Errorf("%w: operation ID does not match request", ErrValidation)

	// ErrInvalidDriverConfig reports malformed or rejected driver configuration.
	// Driver registries preserve the definition's original validation or
	// compilation error alongside this category.
	ErrInvalidDriverConfig = fmt.Errorf("%w: invalid driver config", ErrValidation)

	// ErrInvalidConfigSchema reports an invalid configuration schema envelope
	// or required root declaration.
	ErrInvalidConfigSchema = fmt.Errorf("%w: invalid config schema", ErrValidation)

	// ErrInvalidCurrency reports a non-empty currency with invalid ISO 4217 syntax.
	ErrInvalidCurrency = fmt.Errorf("%w: invalid ISO 4217 currency code", ErrValidation)

	// ErrInvalidConstraintParams reports missing or unencodable constraint
	// parameters, or parameters that cannot be decoded into the destination.
	ErrInvalidConstraintParams = fmt.Errorf("%w: invalid constraint params", ErrValidation)
)
