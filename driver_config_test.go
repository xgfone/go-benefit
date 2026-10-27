package benefit_test

import (
	"encoding/json/jsontext"
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
			if err := schema.Validate(); err == nil {
				t.Fatal("invalid config schema unexpectedly validated")
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
		if err := config.Validate(); err == nil {
			t.Fatalf("invalid config %q unexpectedly validated", config)
		}
	}
}
