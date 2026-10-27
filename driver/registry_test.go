package driver_test

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/xgfone/go-benefit"
	"github.com/xgfone/go-benefit/driver"
)

const validDriverConfig = benefit.DriverConfig(`{"token":"secret"}`)

func TestDefaultRegistryHelpers(t *testing.T) {
	definition := newFakeDefinition("default_registry", false, false)
	typ := definition.Descriptor().Type
	if err := driver.Register(definition); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { driver.DefaultRegistry().Unregister(typ) })
	if got, ok := driver.DefaultRegistry().Get(typ); !ok || got != definition {
		t.Fatal("package helpers and default registry disagree on registrations")
	}
	if err := driver.ValidateConfig(typ, validDriverConfig); err != nil {
		t.Fatal(err)
	}
	bound, err := driver.Bind(typ, validDriverConfig)
	if err != nil {
		t.Fatal(err)
	}
	if bound.Descriptor().Type != typ {
		t.Fatalf("unexpected bound driver type: %q", bound.Descriptor().Type)
	}
	if _, ok := driver.NewRegistry().Get(typ); ok {
		t.Fatal("independent registry inherited the default registrations")
	}
}

func TestDriverRegistry(t *testing.T) {
	registry := driver.NewRegistry()
	definition := newFakeDefinition("coupon", true, true)
	if err := registry.Register(definition); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(definition); err == nil {
		t.Fatal("duplicate driver registration unexpectedly succeeded")
	}

	descriptors := registry.Descriptors()
	if len(descriptors) != 1 || descriptors[0].Type != "test.coupon" {
		t.Fatalf("unexpected descriptors: %#v", descriptors)
	}

	if err := registry.ValidateConfig(
		"test.coupon",
		benefit.DriverConfig(`{}`),
	); err == nil {
		t.Fatal("invalid driver config unexpectedly validated")
	}
	if err := registry.ValidateConfig(
		"test.coupon",
		validDriverConfig,
	); err != nil {
		t.Fatal(err)
	}
	validateCalls := definition.validateCalls.Load()

	driver, err := registry.Bind("test.coupon", validDriverConfig)
	if err != nil {
		t.Fatal(err)
	}
	if driver == nil {
		t.Fatal("registry returned nil driver")
	}
	if descriptor := driver.Descriptor(); descriptor.Type != "test.coupon" {
		t.Fatalf("unexpected bound driver descriptor: %#v", descriptor)
	}
	if definition.validateCalls.Load() != validateCalls {
		t.Fatal("binding unexpectedly performed management configuration validation")
	}
	if definition.compileCalls.Load() != 1 || definition.factoryCalls.Load() != 1 {
		t.Fatalf(
			"unexpected compile or factory calls: compile=%d factory=%d",
			definition.compileCalls.Load(),
			definition.factoryCalls.Load(),
		)
	}

	reverser, ok := driver.(benefit.Reverser)
	if !ok {
		t.Fatal("driver does not implement its declared reverse capability")
	}
	result, err := reverser.Reverse(context.Background(), benefit.ReverseRequest{
		ReversalId:   "RV1",
		RedemptionId: "R1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != benefit.ResultStatusFailure {
		t.Fatalf("unexpected reverse result: %#v", result)
	}
	if got, ok := registry.Get("test.coupon"); !ok || got != definition {
		t.Fatal("registered driver definition was not returned")
	}
	if !registry.Unregister("test.coupon") || registry.Unregister("test.coupon") {
		t.Fatal("driver unregister result was incorrect")
	}
	if _, ok := registry.Get("test.coupon"); ok {
		t.Fatal("unregistered driver definition was still returned")
	}
}

func TestDriverRegistryRejectsMissingOptionalCapability(t *testing.T) {
	registry := driver.NewRegistry()
	definition := newFakeDefinition("missing_reverser", true, false)
	if err := registry.Register(definition); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Bind("test.missing_reverser", validDriverConfig); err == nil {
		t.Fatal("driver without its declared optional capability unexpectedly constructed")
	}
}

func TestDriverRegistryRejectsDeclaredCoreOperation(t *testing.T) {
	for _, operation := range []benefit.Operation{
		benefit.OperationInspect,
		benefit.OperationEvaluate,
		benefit.OperationRedeem,
	} {
		definition := newFakeDefinition("declared_"+strings.ToLower(string(operation)), false, false)
		definition.declareCoreOperation = operation
		if err := driver.NewRegistry().Register(definition); err == nil {
			t.Fatalf("driver with declared core operation %q unexpectedly registered", operation)
		}
	}
}

func TestDriverDescriptorTypeDescriptor(t *testing.T) {
	descriptor := benefit.DriverDescriptor{
		Type: "provider.coupon",
		Name: "Provider coupon",
		Provider: benefit.TypeDescriptor{
			Type: "provider",
			Name: "Provider name",
			Icon: "provider-icon",
		},
		Kind: benefit.TypeDescriptor{
			Type: "coupon",
			Name: "Coupon",
		},
	}
	merged := descriptor.Descriptor()
	if merged.Type != "provider.coupon" ||
		merged.Name != "Provider coupon" ||
		merged.Icon != "provider-icon" {
		t.Fatalf("unexpected merged descriptor: %#v", merged)
	}
}

func TestDriverDescriptorValidation(t *testing.T) {
	valid := newFakeDefinition("coupon", false, false).Descriptor()
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		mutate func(*benefit.DriverDescriptor)
	}{
		{"invalid type", func(d *benefit.DriverDescriptor) { d.Type = "coupon" }},
		{"empty name", func(d *benefit.DriverDescriptor) { d.Name = "" }},
		{"empty kind", func(d *benefit.DriverDescriptor) { d.Kind.Type = "" }},
		{"empty provider", func(d *benefit.DriverDescriptor) { d.Provider.Type = "" }},
		{"mismatched type", func(d *benefit.DriverDescriptor) { d.Type = "other.coupon" }},
		{"missing config schema", func(d *benefit.DriverDescriptor) { d.ConfigSchema = benefit.ConfigSchema{} }},
		{"invalid config schema", func(d *benefit.DriverDescriptor) { d.ConfigSchema.Revision = "" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			descriptor := valid
			test.mutate(&descriptor)
			if err := descriptor.Validate(); err == nil {
				t.Fatal("invalid descriptor unexpectedly validated")
			}
		})
	}
}

func TestDriverRegistryDescriptorSnapshot(t *testing.T) {
	registry := driver.NewRegistry()
	definition := newFakeDefinition("coupon", true, true)
	wantSchema := string(definition.schema.Schema)
	if err := registry.Register(definition); err != nil {
		t.Fatal(err)
	}

	// The registered snapshot must not alias the definition's schema.
	definition.schema.Schema[0] = '['
	descriptors := registry.Descriptors()
	if len(descriptors) != 1 {
		t.Fatalf("unexpected descriptors: %#v", descriptors)
	}
	if got := descriptors[0].ConfigSchema; got.Revision != "v1" || string(got.Schema) != wantSchema {
		t.Fatalf("unexpected descriptor schema: %#v", got)
	}

	// Mutating a returned complete descriptor must not change either query.
	descriptors[0].ConfigSchema.Schema[0] = '['
	descriptors[0].Operations[0].Modes[0] = "changed"
	fresh := registry.Descriptors()[0]
	if string(fresh.ConfigSchema.Schema) != wantSchema ||
		fresh.Operations[0].Modes[0] != benefit.OperationModeReverseFull {
		t.Fatal("registered metadata was mutated through a returned descriptor")
	}
	schema, ok := registry.ConfigSchema(fresh.Type)
	if !ok || string(schema.Schema) != wantSchema {
		t.Fatal("schema query disagrees with the registered descriptor")
	}
	schema.Schema[0] = '['
	if string(registry.Descriptors()[0].ConfigSchema.Schema) != wantSchema {
		t.Fatal("registered descriptor was mutated through a returned schema")
	}

	// Replacing a registration must not affect previously returned metadata.
	registry.Unregister(fresh.Type)
	replacement := newFakeDefinition("coupon", false, false)
	replacement.schema.Revision = "v2"
	if err := registry.Register(replacement); err != nil {
		t.Fatal(err)
	}
	if fresh.ConfigSchema.Revision != "v1" || string(fresh.ConfigSchema.Schema) != wantSchema {
		t.Fatal("previous descriptor snapshot changed after replacement")
	}
	if got := registry.Descriptors()[0].ConfigSchema.Revision; got != "v2" {
		t.Fatalf("unexpected replacement schema revision: %q", got)
	}
}

func TestDriverRegistryRejectsInvalidConfigSchema(t *testing.T) {
	definition := newFakeDefinition("coupon", false, false)
	definition.schema.Revision = ""
	registry := driver.NewRegistry()
	if err := registry.Register(definition); err == nil {
		t.Fatal("driver with invalid config schema unexpectedly registered")
	}
	if len(registry.Descriptors()) != 0 {
		t.Fatal("invalid driver was retained in the registry")
	}
}

func TestNilDriverRegistryAndFactory(t *testing.T) {
	var registry *driver.Registry
	if err := registry.Register(nil); err == nil {
		t.Fatal("nil registry unexpectedly accepted a definition")
	}
	if registry.Unregister("test.coupon") || registry.Descriptors() != nil {
		t.Fatal("nil registry unexpectedly contained drivers")
	}
	if _, ok := registry.Get("test.coupon"); ok {
		t.Fatal("nil registry unexpectedly returned a definition")
	}
	if err := driver.NewRegistry().Register(nil); err == nil {
		t.Fatal("nil driver definition unexpectedly registered")
	}
	definition := newFakeDefinition("nil_factory", false, false)
	definition.nilFactory = true
	registry = driver.NewRegistry()
	if err := registry.Register(definition); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Bind("test.nil_factory", validDriverConfig); err == nil {
		t.Fatal("nil driver factory unexpectedly bound a driver")
	}

	definition = newFakeDefinition("nil_driver", false, false)
	definition.nilDriver = true
	registry = driver.NewRegistry()
	if err := registry.Register(definition); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Bind("test.nil_driver", validDriverConfig); err == nil {
		t.Fatal("factory returning a nil driver unexpectedly bound")
	}
}

func TestRequiredDriverConfigRejectsEmpty(t *testing.T) {
	registry := driver.NewRegistry()
	definition := newFakeDefinition("required_config", false, false)
	if err := registry.Register(definition); err != nil {
		t.Fatal(err)
	}

	if err := registry.ValidateConfig(
		"test.required_config",
		"",
	); err == nil {
		t.Fatal("empty required configuration unexpectedly validated")
	}
	if _, err := registry.Bind("test.required_config", ""); err == nil {
		t.Fatal("empty required configuration unexpectedly bound")
	}
	if definition.validateCalls.Load() != 1 || definition.compileCalls.Load() != 1 {
		t.Fatal("driver definition did not authoritatively reject empty configuration")
	}
}

func TestOptionalDriverConfig(t *testing.T) {
	registry := driver.NewRegistry()
	definition := newFakeDefinition("optional_config", false, false)
	definition.schema.Optional = true
	if err := registry.Register(definition); err != nil {
		t.Fatal(err)
	}
	schema, ok := registry.ConfigSchema("test.optional_config")
	if !ok || !schema.Optional {
		t.Fatalf("unexpected registered schema: %#v", schema)
	}
	schema.Schema[0] = '['
	schema, _ = registry.ConfigSchema("test.optional_config")
	if len(schema.Schema) == 0 || schema.Schema[0] != '{' {
		t.Fatal("registered config schema was mutated through a returned snapshot")
	}

	if err := registry.ValidateConfig(
		"test.optional_config",
		"",
	); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Bind("test.optional_config", ""); err != nil {
		t.Fatal(err)
	}
	if definition.validateCalls.Load() != 1 ||
		definition.compileCalls.Load() != 1 ||
		definition.factoryCalls.Load() != 1 {
		t.Fatalf(
			"unexpected optional config calls: validate=%d compile=%d factory=%d",
			definition.validateCalls.Load(),
			definition.compileCalls.Load(),
			definition.factoryCalls.Load(),
		)
	}
}

type fakeDefinition struct {
	kind                 string
	declareReverse       bool
	driverHasReverse     bool
	declareCoreOperation benefit.Operation
	schema               benefit.ConfigSchema
	nilFactory           bool
	nilDriver            bool
	validateCalls        atomic.Int64
	compileCalls         atomic.Int64
	factoryCalls         atomic.Int64
}

func newFakeDefinition(kind string, declareReverse, driverHasReverse bool) *fakeDefinition {
	return &fakeDefinition{
		kind:             kind,
		declareReverse:   declareReverse,
		driverHasReverse: driverHasReverse,
		schema: benefit.ConfigSchema{
			Revision: "v1",
			Schema: jsontext.Value(`{
                "$schema":"https://json-schema.org/draft/2020-12/schema",
                "type":"object",
                "additionalProperties":false,
                "required":["token"],
                "properties":{"token":{"type":"string","minLength":1,"x-secret":true}}
            }`),
		},
	}
}

func (d *fakeDefinition) Descriptor() benefit.DriverDescriptor {
	var operations benefit.OperationCapabilities
	if d.declareCoreOperation != "" {
		operations = append(operations, benefit.OperationCapability{
			Operation: d.declareCoreOperation,
		})
	}
	if d.declareReverse {
		operations = append(operations, benefit.OperationCapability{
			Operation: benefit.OperationReverse,
			Modes:     []benefit.OperationMode{benefit.OperationModeReverseFull},
		})
	}
	return benefit.DriverDescriptor{
		Type: benefit.DriverType("test." + d.kind),
		Name: "Test " + d.kind,

		Kind:     benefit.TypeDescriptor{Type: d.kind},
		Provider: benefit.TypeDescriptor{Type: "test", Name: "Test"},

		Operations:   operations,
		ConfigSchema: d.schema,
	}
}

func (d *fakeDefinition) ValidateConfig(config benefit.DriverConfig) error {
	d.validateCalls.Add(1)
	if config.IsZero() && d.schema.Optional {
		return nil
	}
	_, err := parseFakeConfig(config)
	return err
}

func (d *fakeDefinition) CompileConfig(config benefit.DriverConfig) (benefit.DriverFactory, error) {
	d.compileCalls.Add(1)
	if !config.IsZero() || !d.schema.Optional {
		if _, err := parseFakeConfig(config); err != nil {
			return nil, err
		}
	}
	if d.nilFactory {
		return nil, nil
	}

	descriptor := d.Descriptor()
	return func() benefit.Driver {
		d.factoryCalls.Add(1)
		if d.nilDriver {
			return nil
		}
		core := fakeDriverCore{descriptor: descriptor}
		if d.driverHasReverse {
			return &fakeDriver{fakeDriverCore: core}
		}
		return &core
	}, nil
}

type fakeConfig struct {
	Token string `json:"token"`
}

func parseFakeConfig(config benefit.DriverConfig) (fakeConfig, error) {
	var parsed fakeConfig
	if err := config.Validate(); err != nil {
		return parsed, err
	}
	if err := json.Unmarshal([]byte(config), &parsed); err != nil {
		return parsed, err
	}
	if parsed.Token == "" {
		return parsed, errors.New("token is required")
	}
	return parsed, nil
}

type fakeDriver struct {
	fakeDriverCore
}

type fakeDriverCore struct {
	descriptor benefit.DriverDescriptor
}

func (d fakeDriverCore) Descriptor() benefit.DriverDescriptor {
	return d.descriptor
}

func (d fakeDriverCore) Inspect(context.Context, benefit.InspectRequest) (benefit.BenefitInfo, error) {
	return benefit.BenefitInfo{
		DriverType: d.descriptor.Type,
		Status:     benefit.StatusActive,
	}, nil
}

func (fakeDriverCore) Evaluate(context.Context, benefit.EvaluateRequest) (benefit.EvaluationResult, error) {
	return benefit.EvaluationResult{Eligible: true}, nil
}

func (fakeDriverCore) Redeem(context.Context, benefit.RedeemRequest) (benefit.RedeemResult, error) {
	return benefit.RedeemResult{Status: benefit.ResultStatusUnknown}, nil
}

func (fakeDriver) Reverse(context.Context, benefit.ReverseRequest) (benefit.ReverseResult, error) {
	return benefit.ReverseResult{
		Status:  benefit.ResultStatusFailure,
		Failure: &benefit.ReversalFailure{Code: benefit.ReversalFailureReversalUnsupported},
	}, nil
}
