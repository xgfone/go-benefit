package benefit_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	benefit "github.com/xgfone/go-benefit"
)

const validDriverConfig = benefit.DriverConfig(`{"token":"secret"}`)

func TestDriverRegistry(t *testing.T) {
	registry := benefit.NewDriverRegistry()
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
		ReversalID:   "RV1",
		RedemptionID: "R1",
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

func TestConfigSchemaValidation(t *testing.T) {
	const dialect = `"$schema":"https://json-schema.org/draft/2020-12/schema"`
	tests := map[string]benefit.ConfigSchema{
		"empty revision":       {Schema: json.RawMessage(`{` + dialect + `,"type":"object"}`)},
		"empty document":       {Revision: "v1"},
		"invalid JSON":         {Revision: "v1", Schema: json.RawMessage(`{`)},
		"non-object document":  {Revision: "v1", Schema: json.RawMessage(`null`)},
		"missing dialect":      {Revision: "v1", Schema: json.RawMessage(`{"type":"object"}`)},
		"non-string dialect":   {Revision: "v1", Schema: json.RawMessage(`{"$schema":1,"type":"object"}`)},
		"unsupported dialect":  {Revision: "v1", Schema: json.RawMessage(`{"$schema":"other","type":"object"}`)},
		"missing root type":    {Revision: "v1", Schema: json.RawMessage(`{` + dialect + `}`)},
		"non-string root type": {Revision: "v1", Schema: json.RawMessage(`{` + dialect + `,"type":1}`)},
		"non-object root type": {Revision: "v1", Schema: json.RawMessage(`{` + dialect + `,"type":"array"}`)},
	}
	for name, schema := range tests {
		t.Run(name, func(t *testing.T) {
			if err := schema.Validate(); err == nil {
				t.Fatal("invalid config schema unexpectedly validated")
			}
		})
	}
}

func TestDriverRegistryRejectsMissingOptionalCapability(t *testing.T) {
	registry := benefit.NewDriverRegistry()
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
		if err := benefit.NewDriverRegistry().Register(definition); err == nil {
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
	tests := []struct {
		name   string
		mutate func(*benefit.DriverDescriptor)
	}{
		{"invalid type", func(d *benefit.DriverDescriptor) { d.Type = "coupon" }},
		{"empty name", func(d *benefit.DriverDescriptor) { d.Name = "" }},
		{"empty kind", func(d *benefit.DriverDescriptor) { d.Kind.Type = "" }},
		{"empty provider", func(d *benefit.DriverDescriptor) { d.Provider.Type = "" }},
		{"mismatched type", func(d *benefit.DriverDescriptor) { d.Type = "other.coupon" }},
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

func TestNilDriverRegistryAndFactory(t *testing.T) {
	var registry *benefit.DriverRegistry
	if err := registry.Register(nil); err == nil {
		t.Fatal("nil registry unexpectedly accepted a definition")
	}
	if registry.Unregister("test.coupon") || registry.Descriptors() != nil {
		t.Fatal("nil registry unexpectedly contained drivers")
	}
	if _, ok := registry.Get("test.coupon"); ok {
		t.Fatal("nil registry unexpectedly returned a definition")
	}
	if err := benefit.NewDriverRegistry().Register(nil); err == nil {
		t.Fatal("nil driver definition unexpectedly registered")
	}
	definition := newFakeDefinition("nil_factory", false, false)
	definition.nilFactory = true
	registry = benefit.NewDriverRegistry()
	if err := registry.Register(definition); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Bind("test.nil_factory", validDriverConfig); err == nil {
		t.Fatal("nil driver factory unexpectedly bound a driver")
	}

	definition = newFakeDefinition("nil_driver", false, false)
	definition.nilDriver = true
	registry = benefit.NewDriverRegistry()
	if err := registry.Register(definition); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Bind("test.nil_driver", validDriverConfig); err == nil {
		t.Fatal("factory returning a nil driver unexpectedly bound")
	}
}

func TestRequiredDriverConfigRejectsEmpty(t *testing.T) {
	registry := benefit.NewDriverRegistry()
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
	registry := benefit.NewDriverRegistry()
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

func TestDriverConfigValidation(t *testing.T) {
	valid := []benefit.DriverConfig{
		"",
		`{}`,
		`{"token":"secret"}`,
		"  {\n\t\"enabled\": true\n  }  ",
	}
	for _, config := range valid {
		if err := config.Validate(); err != nil {
			t.Fatalf("valid config %q failed: %v", config, err)
		}
	}
	if !benefit.DriverConfig("").IsZero() || benefit.DriverConfig(" \n").IsZero() {
		t.Fatal("driver config zero detection is incorrect")
	}

	invalidUTF8 := benefit.DriverConfig(string([]byte{'{', '"', 0xff, '"', ':', '1', '}'}))
	invalid := []benefit.DriverConfig{
		"  \n\t",
		"null",
		"[]",
		`"value"`,
		`{"token":`,
		`{} {}`,
		invalidUTF8,
	}
	for _, config := range invalid {
		if err := config.Validate(); err == nil {
			t.Fatalf("invalid config %q unexpectedly validated", config)
		}
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
			Schema: json.RawMessage(`{
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
		Provider: benefit.TypeDescriptor{
			Type: "test",
			Name: "Test",
		},
		Kind:       benefit.TypeDescriptor{Type: d.kind},
		Operations: operations,
	}
}

func (d *fakeDefinition) ConfigSchema() benefit.ConfigSchema {
	return d.schema.Clone()
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
