package driver

import (
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/xgfone/go-benefit"
)

// Registry is a concurrency-safe registry of driver definitions.
type Registry struct {
	mu          sync.RWMutex
	definitions map[benefit.DriverType]registeredDriverDefinition
}

type registeredDriverDefinition struct {
	definition benefit.DriverDefinition
	descriptor benefit.DriverDescriptor
}

// NewRegistry returns an empty driver registry.
func NewRegistry() *Registry {
	return &Registry{
		definitions: make(map[benefit.DriverType]registeredDriverDefinition, 8),
	}
}

// Register adds a driver definition and rejects duplicate driver types.
func (r *Registry) Register(definition benefit.DriverDefinition) error {
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

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.definitions[descriptor.Type]; exists {
		return fmt.Errorf("benefit: driver type %q is already registered", descriptor.Type)
	}

	r.definitions[descriptor.Type] = registeredDriverDefinition{
		definition: definition,
		descriptor: cloneDriverDescriptor(descriptor),
	}
	return nil
}

// Unregister removes a definition and reports whether it existed.
func (r *Registry) Unregister(driverType benefit.DriverType) bool {
	if r == nil {
		return false
	}

	r.mu.Lock()
	_, exists := r.definitions[driverType]
	delete(r.definitions, driverType)
	r.mu.Unlock()

	return exists
}

// Get returns a registered definition.
func (r *Registry) Get(driverType benefit.DriverType) (benefit.DriverDefinition, bool) {
	registered, ok := r.get(driverType)
	return registered.definition, ok
}

// ConfigSchema returns the registered configuration schema snapshot.
func (r *Registry) ConfigSchema(driverType benefit.DriverType) (benefit.ConfigSchema, bool) {
	if registered, ok := r.get(driverType); ok {
		return registered.descriptor.ConfigSchema.Clone(), true
	}
	return benefit.ConfigSchema{}, false
}

func (r *Registry) get(driverType benefit.DriverType) (registeredDriverDefinition, bool) {
	if r == nil {
		return registeredDriverDefinition{}, false
	}

	r.mu.RLock()
	registered, ok := r.definitions[driverType]
	r.mu.RUnlock()

	return registered, ok
}

// Descriptors returns a consistent snapshot of all complete descriptors,
// sorted by driver type. Returned descriptors do not share mutable data
// with the registry.
func (r *Registry) Descriptors() []benefit.DriverDescriptor {
	if r == nil {
		return nil
	}

	descriptors := r.getDescriptors()
	sort.Slice(descriptors, func(i, j int) bool {
		return descriptors[i].Type < descriptors[j].Type
	})

	return descriptors
}

func (r *Registry) getDescriptors() []benefit.DriverDescriptor {
	r.mu.RLock()
	defer r.mu.RUnlock()

	descriptors := make([]benefit.DriverDescriptor, 0, len(r.definitions))
	for _, definition := range r.definitions {
		descriptors = append(descriptors, cloneDriverDescriptor(definition.descriptor))
	}

	return descriptors
}

// ValidateConfig validates configuration before a driver instance is persisted.
func (r *Registry) ValidateConfig(driverType benefit.DriverType, config benefit.DriverConfig) error {
	registered, ok := r.get(driverType)
	if !ok {
		return fmt.Errorf("benefit: driver type %q is not registered", driverType)
	}

	if err := config.Validate(); err != nil {
		return err
	}
	if err := registered.definition.ValidateConfig(config); err != nil {
		return fmt.Errorf("benefit: validate driver %q config: %w", driverType, err)
	}

	return nil
}

// Bind compiles configuration and constructs a lightweight provider-bound driver.
func (r *Registry) Bind(driverType benefit.DriverType, config benefit.DriverConfig) (benefit.Driver, error) {
	registered, ok := r.get(driverType)
	if !ok {
		return nil, fmt.Errorf("benefit: driver type %q is not registered", driverType)
	}

	if err := config.Validate(); err != nil {
		return nil, err
	}

	factory, err := registered.definition.CompileConfig(config)
	if err != nil {
		return nil, fmt.Errorf("benefit: compile driver %q config: %w", driverType, err)
	}
	if factory == nil {
		return nil, fmt.Errorf("benefit: driver definition %q returned a nil factory", driverType)
	}

	driver := factory()
	if driver == nil {
		return nil, fmt.Errorf("benefit: driver factory %q returned nil", driverType)
	}

	descriptor := driver.Descriptor()
	if descriptor.Type != registered.descriptor.Type {
		const msg = "benefit: driver %q returned descriptor for type %q"
		return nil, fmt.Errorf(msg, driverType, descriptor.Type)
	}

	if err := validateDriverOperation[benefit.Reverser](driver, benefit.OperationReverse); err != nil {
		return nil, err
	}

	return driver, nil
}

func validateDriverOperation[T any](driver benefit.Driver, op benefit.Operation) error {
	_, ok := driver.Descriptor().Operations.Get(op)
	if !ok {
		return nil
	}

	if _, ok := driver.(T); ok {
		return nil
	}

	const msg = "benefit: driver %q declares %q but does not implement Reverser"
	return fmt.Errorf(msg, driver.Descriptor().Type, benefit.OperationReverse)
}

func cloneDriverDescriptor(descriptor benefit.DriverDescriptor) benefit.DriverDescriptor {
	descriptor.Operations = cloneOperationCapabilities(descriptor.Operations)
	descriptor.ConfigSchema = descriptor.ConfigSchema.Clone()
	return descriptor
}

var defaultDriverRegistry = NewRegistry()

// DefaultRegistry returns the package-level default driver registry.
func DefaultRegistry() *Registry {
	return defaultDriverRegistry
}

// Register registers a definition in the package-level registry.
func Register(definition benefit.DriverDefinition) error {
	return defaultDriverRegistry.Register(definition)
}

// MustRegister registers a definition in the package-level registry
// and panics if registration fails.
func MustRegister(definition benefit.DriverDefinition) {
	if err := Register(definition); err != nil {
		panic(err)
	}
}

// ValidateConfig validates configuration with the package-level registry.
func ValidateConfig(driverType benefit.DriverType, config benefit.DriverConfig) error {
	return defaultDriverRegistry.ValidateConfig(driverType, config)
}

// Bind constructs a driver from the package-level registry.
func Bind(driverType benefit.DriverType, config benefit.DriverConfig) (benefit.Driver, error) {
	return defaultDriverRegistry.Bind(driverType, config)
}

func cloneOperationCapabilities(capabilities benefit.OperationCapabilities) benefit.OperationCapabilities {
	cloned := make(benefit.OperationCapabilities, len(capabilities))
	for i, capability := range capabilities {
		cloned[i] = capability
		cloned[i].Modes = append([]benefit.OperationMode(nil), capability.Modes...)
	}
	return cloned
}
