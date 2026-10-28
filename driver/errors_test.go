package driver_test

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"testing"

	"github.com/xgfone/go-benefit"
	"github.com/xgfone/go-benefit/driver"
)

func TestUnregisteredDriverErrorCategory(t *testing.T) {
	registry := driver.NewRegistry()
	_, bindErr := registry.Bind("test.missing", validDriverConfig)
	for _, err := range []error{
		registry.ValidateConfig("test.missing", validDriverConfig), bindErr,
	} {
		err = fmt.Errorf("application: %w", err)
		if !errors.Is(err, driver.ErrNotRegistered) || errors.Is(err, benefit.ErrBenefitNotFound) ||
			errors.Is(err, benefit.ErrInvalidDriverConfig) {
			t.Fatalf("incorrect unregistered driver category: %v", err)
		}
	}
}

func TestDriverConfigErrorCauses(t *testing.T) {
	cause := errors.New("provider config rejected")
	original := &json.SemanticError{Err: cause}
	definition := &errorDefinition{
		fakeDefinition: newFakeDefinition("errors", false, false),
		err:            fmt.Errorf("provider validation: %w", original),
	}
	registry := driver.NewRegistry()
	if err := registry.Register(definition); err != nil {
		t.Fatal(err)
	}
	validate := func(config benefit.DriverConfig) error { return registry.ValidateConfig("test.errors", config) }
	bind := func(config benefit.DriverConfig) error {
		_, err := registry.Bind("test.errors", config)
		return err
	}
	for name, call := range map[string]func(benefit.DriverConfig) error{"validate": validate, "bind": bind} {
		t.Run(name, func(t *testing.T) {
			err := fmt.Errorf("application: %w", call(validDriverConfig))
			var typed *json.SemanticError
			if !errors.Is(err, benefit.ErrInvalidDriverConfig) || !errors.Is(err, benefit.ErrValidation) ||
				!errors.Is(err, cause) || !errors.As(err, &typed) || typed != original {
				t.Fatalf("driver config lost category, cause, or original error type: %v", err)
			}
			if err := call(`[]`); !errors.Is(err, benefit.ErrInvalidDriverConfig) || errors.Is(err, cause) {
				t.Fatalf("malformed config did not stop before definition invocation: %v", err)
			}
		})
	}
}

func TestDriverDescriptorErrorPreservesJsonCause(t *testing.T) {
	definition := newFakeDefinition("schema_error", false, false)
	definition.schema.Schema = []byte(`{"$schema":1,"type":"object"}`)
	err := driver.NewRegistry().Register(definition)
	var cause *json.SemanticError
	if !errors.Is(err, driver.ErrInvalidDefinition) || !errors.Is(err, benefit.ErrInvalidConfigSchema) ||
		!errors.Is(err, benefit.ErrValidation) || !errors.As(err, &cause) {
		t.Fatalf("descriptor validation lost categories or JSON cause: %v", err)
	}
}

type errorDefinition struct {
	*fakeDefinition
	err error
}

func (d *errorDefinition) ValidateConfig(benefit.DriverConfig) error {
	return d.err
}

func (d *errorDefinition) CompileConfig(benefit.DriverConfig) (benefit.DriverFactory, error) {
	return nil, d.err
}
