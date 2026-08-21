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
		context.Background(),
		"test.coupon",
		benefit.DriverConfig(`{}`),
	); err == nil {
		t.Fatal("invalid driver config unexpectedly validated")
	}
	if err := registry.ValidateConfig(
		context.Background(),
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
	if definition.compileCalls.Load() != 1 || definition.newDriverCalls.Load() != 1 {
		t.Fatalf(
			"unexpected compile or construction calls: compile=%d new=%d",
			definition.compileCalls.Load(),
			definition.newDriverCalls.Load(),
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
}

func TestDriverRegistryRejectsInvalidConfigSchema(t *testing.T) {
	definition := newFakeDefinition("invalid_schema", false, false)
	definition.schema = benefit.ConfigSchema{
		Revision: "v1",
		Schema:   json.RawMessage(`{"type":"object"}`),
	}

	if err := benefit.NewDriverRegistry().Register(definition); err == nil {
		t.Fatal("driver with an invalid config schema unexpectedly registered")
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

func TestRequiredDriverConfigRejectsEmpty(t *testing.T) {
	registry := benefit.NewDriverRegistry()
	definition := newFakeDefinition("required_config", false, false)
	if err := registry.Register(definition); err != nil {
		t.Fatal(err)
	}

	if err := registry.ValidateConfig(
		context.Background(),
		"test.required_config",
		"",
	); err == nil {
		t.Fatal("empty required configuration unexpectedly validated")
	}
	if _, err := registry.Bind("test.required_config", ""); err == nil {
		t.Fatal("empty required configuration unexpectedly bound")
	}
	if definition.validateCalls.Load() != 0 || definition.compileCalls.Load() != 0 {
		t.Fatal("empty required configuration reached the driver definition")
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
		context.Background(),
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
		definition.newDriverCalls.Load() != 1 {
		t.Fatalf(
			"unexpected optional config calls: validate=%d compile=%d new=%d",
			definition.validateCalls.Load(),
			definition.compileCalls.Load(),
			definition.newDriverCalls.Load(),
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
	validateCalls        atomic.Int64
	compileCalls         atomic.Int64
	newDriverCalls       atomic.Int64
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
	var operations benefit.OperationSupports
	if d.declareCoreOperation != "" {
		operations = append(operations, benefit.OperationSupport{
			Operation: d.declareCoreOperation,
			Supported: true,
		})
	}
	if d.declareReverse {
		operations = append(operations, benefit.OperationSupport{
			Operation: benefit.OperationReverse,
			Supported: true,
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

func (d *fakeDefinition) ValidateConfig(_ context.Context, config benefit.DriverConfig) error {
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

	descriptor := d.Descriptor()
	return benefit.DriverFactoryFunc(func() (benefit.Driver, error) {
		d.newDriverCalls.Add(1)
		core := fakeDriverCore{descriptor: descriptor}
		if d.driverHasReverse {
			return &fakeDriver{fakeDriverCore: core}, nil
		}
		return &core, nil
	}), nil
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
		Failure: &benefit.ReversalFailure{Type: benefit.ReversalFailureReversalUnsupported},
	}, nil
}
