package benefit_test

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"testing"

	"github.com/xgfone/go-benefit"
)

func TestConfigSchemaValidation(t *testing.T) {
	const dialect = `"$schema":"https://json-schema.org/draft/2020-12/schema"`
	tests := map[string]benefit.ConfigSchema{
		"empty revision":       {Schema: jsontext.Value(`{` + dialect + `,"type":"object"}`)},
		"empty document":       {Revision: "v1"},
		"invalid JSON":         {Revision: "v1", Schema: jsontext.Value(`{`)},
		"non-object document":  {Revision: "v1", Schema: jsontext.Value(`null`)},
		"missing dialect":      {Revision: "v1", Schema: jsontext.Value(`{"type":"object"}`)},
		"non-string dialect":   {Revision: "v1", Schema: jsontext.Value(`{"$schema":1,"type":"object"}`)},
		"unsupported dialect":  {Revision: "v1", Schema: jsontext.Value(`{"$schema":"other","type":"object"}`)},
		"missing root type":    {Revision: "v1", Schema: jsontext.Value(`{` + dialect + `}`)},
		"non-string root type": {Revision: "v1", Schema: jsontext.Value(`{` + dialect + `,"type":1}`)},
		"non-object root type": {Revision: "v1", Schema: jsontext.Value(`{` + dialect + `,"type":"array"}`)},
	}
	for name, schema := range tests {
		t.Run(name, func(t *testing.T) {
			if err := schema.Validate(); !errors.Is(err, benefit.ErrInvalidConfigSchema) {
				t.Fatalf("expected invalid config schema category, got %v", err)
			}
		})
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

	invalidUtf8 := benefit.DriverConfig(string([]byte{'{', '"', 0xff, '"', ':', '1', '}'}))
	invalid := []benefit.DriverConfig{
		`{"key":1,"key":2}`,
		`{"nested":{"key":1,"key":2}}`,
		"  \n\t",
		"null",
		"[]",
		`"value"`,
		`{"token":`,
		`{} {}`,
		invalidUtf8,
	}
	for _, config := range invalid {
		if err := config.Validate(); !errors.Is(err, benefit.ErrInvalidDriverConfig) {
			t.Fatalf("expected invalid driver config category for %q, got %v", config, err)
		}
	}
}

func TestConfigErrorsPreserveJsonCause(t *testing.T) {
	const dialect = `"$schema":"https://json-schema.org/draft/2020-12/schema"`
	tests := []struct {
		name     string
		err      error
		category error
		semantic bool
	}{
		{"config syntax", benefit.DriverConfig(`{"key":}`).Validate(), benefit.ErrInvalidDriverConfig, false},
		{"schema syntax", (benefit.ConfigSchema{Revision: "v1", Schema: []byte(`{`)}).Validate(), benefit.ErrInvalidConfigSchema, false},
		{"schema object", (benefit.ConfigSchema{Revision: "v1", Schema: []byte(`[]`)}).Validate(), benefit.ErrInvalidConfigSchema, true},
		{"schema dialect", (benefit.ConfigSchema{Revision: "v1", Schema: []byte(`{"$schema":1,"type":"object"}`)}).Validate(), benefit.ErrInvalidConfigSchema, true},
		{"schema type", (benefit.ConfigSchema{Revision: "v1", Schema: []byte(`{` + dialect + `,"type":1}`)}).Validate(), benefit.ErrInvalidConfigSchema, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := fmt.Errorf("application: %w", test.err)
			if !errors.Is(err, test.category) || !errors.Is(err, benefit.ErrValidation) {
				t.Fatalf("validation categories were lost: %v", err)
			}
			if test.semantic {
				var cause *json.SemanticError
				if !errors.As(err, &cause) {
					t.Fatalf("semantic cause was lost: %v", err)
				}
			} else {
				var cause *jsontext.SyntacticError
				if !errors.As(err, &cause) {
					t.Fatalf("syntactic cause was lost: %v", err)
				}
			}
		})
	}
}
