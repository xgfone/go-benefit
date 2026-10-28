package benefit

import (
	"bytes"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"strings"
	"unicode/utf8"
)

// JsonSchemaDraft202012 identifies the supported JSON Schema dialect.
const JsonSchemaDraft202012 = "https://json-schema.org/draft/2020-12/schema"

// DriverConfig contains a UTF-8 JSON object with provider-specific settings.
// An empty value means that the driver instance has no configuration.
type DriverConfig string

// IsZero reports whether the configuration is the empty string.
func (c DriverConfig) IsZero() bool {
	return c == ""
}

// Validate verifies that a non-empty configuration is a UTF-8 JSON object.
// Errors wrap ErrInvalidDriverConfig and preserve any JSON decoding error.
func (c DriverConfig) Validate() error {
	if c == "" {
		return nil
	}

	value := string(c)
	if !utf8.ValidString(value) {
		return fmt.Errorf("%w: not valid UTF-8", ErrInvalidDriverConfig)
	}

	value = strings.TrimSpace(value)
	if len(value) < 2 || value[0] != '{' || value[len(value)-1] != '}' {
		return fmt.Errorf("%w: not a JSON object", ErrInvalidDriverConfig)
	}

	var object map[string]jsontext.Value
	if err := json.Unmarshal([]byte(value), &object); err != nil {
		return fmt.Errorf("%w: not a JSON object: %w", ErrInvalidDriverConfig, err)
	}

	return nil
}

// ConfigSchema describes versioned configuration form and validation metadata.
// DriverDefinition validation and compilation remain authoritative.
type ConfigSchema struct {
	Optional bool `json:",omitzero"`
	Revision string
	Schema   jsontext.Value
}

// Clone returns a copy whose JSON Schema document does not alias the original.
func (s ConfigSchema) Clone() ConfigSchema {
	s.Schema = bytes.Clone(s.Schema)
	return s
}

// Validate verifies the schema envelope and its required root declarations.
// Errors wrap ErrInvalidConfigSchema and preserve any JSON decoding error.
func (s ConfigSchema) Validate() error {
	if strings.TrimSpace(s.Revision) == "" {
		return fmt.Errorf("%w: revision is empty", ErrInvalidConfigSchema)
	}
	if len(s.Schema) == 0 {
		return fmt.Errorf("%w: document is empty", ErrInvalidConfigSchema)
	}

	var root map[string]jsontext.Value
	if err := json.Unmarshal(s.Schema, &root); err != nil {
		return fmt.Errorf("%w: document is invalid: %w", ErrInvalidConfigSchema, err)
	}
	if root == nil {
		return fmt.Errorf("%w: document is not an object", ErrInvalidConfigSchema)
	}

	var dialect string
	if raw := root["$schema"]; len(raw) == 0 {
		return fmt.Errorf("%w: no $schema declaration", ErrInvalidConfigSchema)
	} else if err := json.Unmarshal(raw, &dialect); err != nil {
		return fmt.Errorf("%w: $schema declaration is not a string: %w", ErrInvalidConfigSchema, err)
	}
	if dialect != JsonSchemaDraft202012 {
		return fmt.Errorf("%w: unsupported dialect %q", ErrInvalidConfigSchema, dialect)
	}

	var rootType string
	if raw := root["type"]; len(raw) == 0 {
		return fmt.Errorf("%w: no root type", ErrInvalidConfigSchema)
	} else if err := json.Unmarshal(raw, &rootType); err != nil {
		return fmt.Errorf("%w: root type is not a string: %w", ErrInvalidConfigSchema, err)
	}
	if rootType != "object" {
		return fmt.Errorf("%w: root type is %q, want object", ErrInvalidConfigSchema, rootType)
	}
	return nil
}

// DriverFactory returns a driver from one compiled configuration. It must be
// safe for concurrent calls and may return the same or different concurrently
// safe Driver instances across calls.
type DriverFactory func() Driver
