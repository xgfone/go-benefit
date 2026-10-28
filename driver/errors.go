package driver

import "errors"

var (
	// ErrNilRegistry reports registration through a nil registry.
	ErrNilRegistry = errors.New("benefit: driver registry is nil")

	// ErrInvalidDefinition reports a nil or invalid definition, or a definition
	// whose factory or driver violates the declared driver contract.
	ErrInvalidDefinition = errors.New("benefit: invalid driver definition")

	// ErrAlreadyRegistered reports registration of an existing driver type.
	ErrAlreadyRegistered = errors.New("benefit: driver type is already registered")

	// ErrNotRegistered reports configuration validation or binding of an
	// unregistered driver type.
	ErrNotRegistered = errors.New("benefit: driver type is not registered")
)
