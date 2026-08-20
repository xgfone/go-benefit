package benefit

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
)

// DriverRegistry is a concurrency-safe registry of driver definitions.
type DriverRegistry struct {
	mu          sync.RWMutex
	aliases     map[DriverType]DriverType
	definitions map[DriverType]registeredDriverDefinition
}

type registeredDriverDefinition struct {
	definition DriverDefinition
	descriptor DriverDescriptor
	schema     ConfigSchema
}

// NewDriverRegistry returns an empty driver registry.
func NewDriverRegistry() *DriverRegistry {
	return &DriverRegistry{
		aliases:     make(map[DriverType]DriverType, 4),
		definitions: make(map[DriverType]registeredDriverDefinition, 8),
	}
}

// Register adds a driver definition and rejects duplicate driver types.
func (r *DriverRegistry) Register(definition DriverDefinition) error {
	if r == nil {
		return errors.New("benefit: driver registry is nil")
	}
	if definition == nil {
		return errors.New("benefit: driver definition is nil")
	}

	descriptor := definition.Descriptor()
	if err := descriptor.Validate(); err != nil {
		return err
	}

	schema := definition.ConfigSchema()
	if err := schema.Validate(); err != nil {
		return fmt.Errorf("benefit: invalid driver %q config schema: %w", descriptor.Type, err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.definitions[descriptor.Type]; exists {
		return fmt.Errorf("benefit: driver type %q is already registered", descriptor.Type)
	}
	if _, exists := r.aliases[descriptor.Type]; exists {
		return fmt.Errorf("benefit: driver type %q is already registered as an alias", descriptor.Type)
	}

	r.definitions[descriptor.Type] = registeredDriverDefinition{
		definition: definition,
		descriptor: cloneDriverDescriptor(descriptor),
		schema:     schema.Clone(),
	}
	return nil
}

// MustRegister adds a definition and panics on failure.
func (r *DriverRegistry) MustRegister(definition DriverDefinition) {
	if err := r.Register(definition); err != nil {
		panic(err)
	}
}

// RegisterAlias registers aliasType as a deprecated alias for an already
// registered canonicalType. Alias targets must be canonical definitions rather
// than other aliases.
func (r *DriverRegistry) RegisterAlias(aliasType, canonicalType DriverType) error {
	if r == nil {
		return errors.New("benefit: driver registry is nil")
	}
	if err := aliasType.Validate(); err != nil {
		return err
	}
	if err := canonicalType.Validate(); err != nil {
		return err
	}
	if aliasType == canonicalType {
		return fmt.Errorf("benefit: driver alias %q must differ from its canonical type", aliasType)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.definitions[canonicalType]; !exists {
		if target, alias := r.aliases[canonicalType]; alias {
			const msg = "benefit: driver alias target %q is an alias for %q"
			return fmt.Errorf(msg, canonicalType, target)
		}
		return fmt.Errorf("benefit: driver alias target %q is not registered", canonicalType)
	}
	if _, exists := r.definitions[aliasType]; exists {
		return fmt.Errorf("benefit: driver alias %q is already registered as a definition", aliasType)
	}
	if target, exists := r.aliases[aliasType]; exists {
		return fmt.Errorf("benefit: driver alias %q is already registered for %q", aliasType, target)
	}

	r.aliases[aliasType] = canonicalType
	return nil
}

// Unregister removes a definition or alias and reports whether it existed.
// Removing a canonical definition also removes all aliases that target it.
func (r *DriverRegistry) Unregister(driverType DriverType) bool {
	if r == nil {
		return false
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.aliases[driverType]; exists {
		delete(r.aliases, driverType)
		return true
	}

	if _, exists := r.definitions[driverType]; !exists {
		return false
	}

	delete(r.definitions, driverType)
	for aliasType, canonicalType := range r.aliases {
		if canonicalType == driverType {
			delete(r.aliases, aliasType)
		}
	}

	return true
}

// Get returns a registered definition.
//
// NOTE: Aliases resolve to their canonical definitions.
func (r *DriverRegistry) Get(driverType DriverType) (DriverDefinition, bool) {
	registered, ok := r.get(driverType)
	return registered.definition, ok
}

// ConfigSchema returns the registered configuration schema snapshot.
func (r *DriverRegistry) ConfigSchema(driverType DriverType) (ConfigSchema, bool) {
	if registered, ok := r.get(driverType); ok {
		return registered.schema.Clone(), true
	}
	return ConfigSchema{}, false
}

func (r *DriverRegistry) get(driverType DriverType) (registeredDriverDefinition, bool) {
	if r == nil {
		return registeredDriverDefinition{}, false
	}

	r.mu.RLock()
	if canonicalType, exists := r.aliases[driverType]; exists {
		driverType = canonicalType
	}
	registered, ok := r.definitions[driverType]
	r.mu.RUnlock()

	return registered, ok
}

// Descriptors returns all canonical descriptors sorted by driver type.
//
// NOTE: Aliases are intentionally omitted so deprecated types are not offered
// for new use.
func (r *DriverRegistry) Descriptors() []DriverDescriptor {
	if r == nil {
		return nil
	}

	descriptors := r.getDescriptors()
	sort.Slice(descriptors, func(i, j int) bool {
		return descriptors[i].Type < descriptors[j].Type
	})

	return descriptors
}

func (r *DriverRegistry) getDescriptors() []DriverDescriptor {
	r.mu.RLock()
	defer r.mu.RUnlock()

	descriptors := make([]DriverDescriptor, 0, len(r.definitions))
	for _, definition := range r.definitions {
		descriptors = append(descriptors, cloneDriverDescriptor(definition.descriptor))
	}

	return descriptors
}

// ValidateConfig validates configuration before a driver instance is persisted.
func (r *DriverRegistry) ValidateConfig(ctx context.Context, driverType DriverType, config DriverConfig) error {
	registered, ok := r.get(driverType)
	if !ok {
		return fmt.Errorf("benefit: driver type %q is not registered", driverType)
	}

	if err := validateDriverConfigPresence(registered.schema, config); err != nil {
		return err
	}
	if err := config.Validate(); err != nil {
		return err
	}
	if err := registered.definition.ValidateConfig(ctx, config); err != nil {
		return fmt.Errorf("benefit: validate driver %q config: %w", driverType, err)
	}

	return nil
}

// Bind compiles configuration and constructs a lightweight provider-bound driver.
func (r *DriverRegistry) Bind(driverType DriverType, config DriverConfig) (Driver, error) {
	registered, ok := r.get(driverType)
	if !ok {
		return nil, fmt.Errorf("benefit: driver type %q is not registered", driverType)
	}

	if err := validateDriverConfigPresence(registered.schema, config); err != nil {
		return nil, err
	}

	factory, err := registered.definition.CompileConfig(config)
	if err != nil {
		return nil, fmt.Errorf("benefit: compile driver %q config: %w", driverType, err)
	}
	if factory == nil {
		return nil, fmt.Errorf("benefit: driver definition %q returned a nil factory", driverType)
	}

	driver, err := factory.NewDriver()
	if err != nil {
		return nil, fmt.Errorf("benefit: bind driver %q: %w", driverType, err)
	}

	if driver == nil {
		return nil, fmt.Errorf("benefit: driver factory %q returned nil", driverType)
	}

	descriptor := driver.Descriptor()
	if descriptor.Type != registered.descriptor.Type {
		const msg = "benefit: driver %q returned descriptor for type %q"
		return nil, fmt.Errorf(msg, driverType, descriptor.Type)
	}

	if err := validateDriverOperation[Reverser](driver, OperationReverse); err != nil {
		return nil, err
	}

	return driver, nil
}

func validateDriverOperation[T any](driver Driver, op Operation) error {
	support, ok := driver.Descriptor().Operations.Get(op)
	if !ok || !support.Supported {
		return nil
	}

	if _, ok := driver.(T); ok {
		return nil
	}

	const msg = "benefit: driver %q declares %q but does not implement Reverser"
	return fmt.Errorf(msg, driver.Descriptor().Type, OperationReverse)
}

func validateDriverConfigPresence(schema ConfigSchema, config DriverConfig) error {
	if config.IsZero() && !schema.Optional {
		return errors.New("benefit: driver config is required")
	}
	return nil
}

func cloneDriverDescriptor(descriptor DriverDescriptor) DriverDescriptor {
	descriptor.Operations = cloneOperationSupports(descriptor.Operations)
	return descriptor
}

// DefaultDriverRegistry is the package-level driver registry.
var DefaultDriverRegistry = NewDriverRegistry()

// RegisterDriver registers a definition in the package-level registry.
func RegisterDriver(definition DriverDefinition) error {
	return DefaultDriverRegistry.Register(definition)
}

// RegisterDriverAlias registers a deprecated alias in the package-level registry.
//
// The canonical type must already be registered.
func RegisterDriverAlias(aliasType, canonicalType DriverType) error {
	return DefaultDriverRegistry.RegisterAlias(aliasType, canonicalType)
}

// ValidateDriverConfig validates configuration with the package-level registry.
func ValidateDriverConfig(ctx context.Context, driverType DriverType, config DriverConfig) error {
	return DefaultDriverRegistry.ValidateConfig(ctx, driverType, config)
}

// BindDriver constructs a driver from the package-level registry.
func BindDriver(driverType DriverType, config DriverConfig) (Driver, error) {
	return DefaultDriverRegistry.Bind(driverType, config)
}
