package benefit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// JSONSchemaDraft202012 identifies the supported JSON Schema dialect.
const JSONSchemaDraft202012 = "https://json-schema.org/draft/2020-12/schema"

// DriverConfig contains a UTF-8 JSON object with provider-specific settings.
// An empty value means that the driver instance has no configuration.
type DriverConfig string

// IsZero reports whether the configuration is the empty string.
func (c DriverConfig) IsZero() bool {
	return c == ""
}

// Validate verifies that a non-empty configuration is a UTF-8 JSON object.
func (c DriverConfig) Validate() error {
	if c == "" {
		return nil
	}

	value := string(c)
	if !utf8.ValidString(value) {
		return errors.New("benefit: driver config is not valid UTF-8")
	}

	value = strings.TrimSpace(value)
	if len(value) < 2 || value[0] != '{' || value[len(value)-1] != '}' {
		return errors.New("benefit: driver config is not a JSON object")
	}

	var object map[string]json.RawMessage
	if err := json.Unmarshal([]byte(value), &object); err != nil {
		return fmt.Errorf("benefit: driver config is not a JSON object: %w", err)
	}

	return nil
}

// ConfigSchema describes versioned configuration form and validation metadata.
// DriverDefinition validation and compilation remain authoritative.
type ConfigSchema struct {
	Revision string          `json:"revision"`
	Optional bool            `json:"optional,omitempty"`
	Schema   json.RawMessage `json:"schema"`
}

// Clone returns a copy whose JSON Schema document does not alias the original.
func (s ConfigSchema) Clone() ConfigSchema {
	s.Schema = bytes.Clone(s.Schema)
	return s
}

// Validate verifies the schema envelope and its required root declarations.
func (s ConfigSchema) Validate() error {
	if strings.TrimSpace(s.Revision) == "" {
		return errors.New("benefit: config schema revision is empty")
	}
	if len(s.Schema) == 0 {
		return errors.New("benefit: config schema document is empty")
	}

	var root map[string]json.RawMessage
	if err := json.Unmarshal(s.Schema, &root); err != nil {
		return fmt.Errorf("benefit: config schema document is invalid: %w", err)
	}
	if root == nil {
		return errors.New("benefit: config schema document is not an object")
	}

	var dialect string
	if raw := root["$schema"]; len(raw) == 0 {
		return errors.New("benefit: config schema has no $schema declaration")
	} else if err := json.Unmarshal(raw, &dialect); err != nil {
		return errors.New("benefit: config schema $schema declaration is not a string")
	}
	if dialect != JSONSchemaDraft202012 {
		return fmt.Errorf("benefit: unsupported config schema dialect %q", dialect)
	}

	var rootType string
	if raw := root["type"]; len(raw) == 0 {
		return errors.New("benefit: config schema has no root type")
	} else if err := json.Unmarshal(raw, &rootType); err != nil {
		return errors.New("benefit: config schema root type is not a string")
	}
	if rootType != "object" {
		return fmt.Errorf("benefit: config schema root type is %q, want object", rootType)
	}
	return nil
}

// DriverFactory returns a driver from one compiled configuration. It must be
// safe for concurrent calls and may return the same or different concurrently
// safe Driver instances across calls.
type DriverFactory func() Driver
