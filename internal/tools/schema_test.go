package tools

import (
	"errors"
	"testing"
)

// boolPtr is a helper to create a *bool pointer to a bool literal, needed for
// struct literals with *bool fields (AdditionalProperties, etc.).
func boolPtr(b bool) *bool { return &b }

func TestEmptyObjectSchemaExportsAdditionalPropertiesTrue(t *testing.T) {
	schema := EmptyObjectSchema()
	m := schema.Map()

	// EmptyObjectSchema explicitly sets AdditionalProperties=true so that
	// ValidateParameters is permissive and the exported schema signals permissiveness
	// to any JSON-Schema-aware consumer.
	if m["additionalProperties"] != true {
		t.Fatalf("EmptyObjectSchema().Map() must have additionalProperties=true, got %v", m["additionalProperties"])
	}

	// Verify the schema is still structurally an empty object.
	if m["type"] != "object" {
		t.Fatalf("expected type=object, got %v", m["type"])
	}
	props, ok := m["properties"].(map[string]any)
	if !ok || len(props) != 0 {
		t.Fatalf("expected empty properties map, got %v", m["properties"])
	}
}

func TestEmptyObjectSchemaAcceptsUnknownParams(t *testing.T) {
	schema := EmptyObjectSchema()

	// Simulate GPT-4o-mini emitting a spurious namespace param for discovery.list_namespaces.
	// This must NOT error — the whole point of is that the validation loop
	// should not abort before metrics/graph emission.
	input := map[string]any{
		"namespace": "default", // unknown for an empty-object tool
	}
	if err := ValidateParameters(input, schema); err != nil {
		t.Fatalf("EmptyObjectSchema must accept unknown params (was: %v)", err)
	}
}

func TestEmptyObjectSchemaAcceptsNilInput(t *testing.T) {
	schema := EmptyObjectSchema()
	if err := ValidateParameters(nil, schema); err != nil {
		t.Fatalf("EmptyObjectSchema must accept nil input (was: %v)", err)
	}
}

func TestSchemaWithAdditionalPropertiesTrueAcceptsUnknownParams(t *testing.T) {
	schema := Schema{
		Type: "object",
		Properties: map[string]Schema{
			"name": {Type: "string"},
		},
		AdditionalProperties: boolPtr(true),
	}

	input := map[string]any{
		"name": "my-secret",
		"foo":  "unknown-param",
	}
	if err := ValidateParameters(input, schema); err != nil {
		t.Fatalf("AdditionalProperties=true must accept unknown params (was: %v)", err)
	}
}

func TestSchemaWithAdditionalPropertiesFalseRejectsUnknownParams(t *testing.T) {
	additionalProperties := false
	schema := Schema{
		Type: "object",
		Properties: map[string]Schema{
			"name": {Type: "string"},
		},
		AdditionalProperties: &additionalProperties,
	}

	input := map[string]any{
		"name": "my-secret",
		"foo":  "unknown-param",
	}
	err := ValidateParameters(input, schema)
	if err == nil {
		t.Fatal("expected error for unknown param with AdditionalProperties=false")
	}
	var paramErr *ParameterValidationError
	if errors.As(err, &paramErr) && paramErr.Parameter != "foo" {
		t.Fatalf("expected error for param 'foo', got %q", paramErr.Parameter)
	}
}

func TestSchemaMapNonEmptyWithoutAdditionalPropertiesExportsFalse(t *testing.T) {
	// Verify that a non-empty schema built inline (nil AdditionalProperties) still
	// exports additionalProperties=false in its Map() output. This preserves the
	// pre-behavior for non-EmptyObjectSchema users.
	schema := Schema{
		Type: "object",
		Properties: map[string]Schema{
			"name": {Type: "string"},
		},
		// AdditionalProperties intentionally nil — Map() auto-defaults to false
	}
	m := schema.Map()

	if m["additionalProperties"] != false {
		t.Fatalf("expected additionalProperties=false for non-empty schema with nil AdditionalProperties, got %v", m["additionalProperties"])
	}
}

func TestSchemaMapPreservesExplicitAdditionalProperties(t *testing.T) {
	additionalProperties := false
	schema := Schema{
		Type: "object",
		Properties: map[string]Schema{
			"name": {Type: "string"},
		},
		AdditionalProperties: &additionalProperties,
	}
	m := schema.Map()

	if m["additionalProperties"] != false {
		t.Fatalf("expected additionalProperties=false, got %v", m["additionalProperties"])
	}
}
