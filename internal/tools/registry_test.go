package tools

import (
	"context"
	"reflect"
	"testing"
)

type registryTestTool struct {
	name        string
	description string
}

func (t registryTestTool) Name() string {
	return t.name
}

func (t registryTestTool) Description() string {
	return t.description
}

func (t registryTestTool) Run(context.Context, map[string]any) (map[string]any, error) {
	return map[string]any{"ok": true}, nil
}

type registrySchemaTool struct {
	registryTestTool
	schema Schema
}

func (t registrySchemaTool) ParameterSchema() Schema {
	return t.schema
}

func TestRegistryDefinitionsUsesExplicitSchemas(t *testing.T) {
	registry := NewRegistry()

	tool := registrySchemaTool{
		registryTestTool: registryTestTool{
			name:        "validation.read_secret",
			description: "Reads a specific Secret",
		},
		schema: Schema{
			Type: "object",
			Properties: map[string]Schema{
				"name": {
					Type:        "string",
					Description: "Secret name",
				},
				"namespace": {
					Type:        "string",
					Description: "Namespace to query",
					Default:     "default",
				},
			},
			Required: []string{"name"},
		},
	}
	if err := registry.Register(tool); err != nil {
		t.Fatalf("register tool: %v", err)
	}

	definitions, err := registry.Definitions([]string{tool.Name()})
	if err != nil {
		t.Fatalf("export definitions: %v", err)
	}

	if len(definitions) != 1 {
		t.Fatalf("expected 1 definition, got %d", len(definitions))
	}

	expected := tool.ParameterSchema().Map()
	if !reflect.DeepEqual(definitions[0].Parameters, expected) {
		t.Fatalf("expected explicit schema %#v, got %#v", expected, definitions[0].Parameters)
	}
}

func TestRegistryDefinitionsReadSecretSchema(t *testing.T) {
	registry := NewRegistry()

	tool := registrySchemaTool{
		registryTestTool: registryTestTool{
			name:        "validation.read_secret",
			description: "Reads a specific Secret",
		},
		schema: Schema{
			Type: "object",
			Properties: map[string]Schema{
				"name": {
					Type: "string",
				},
				"namespace": {
					Type:    "string",
					Default: "default",
				},
				"allow_namespaces": {
					Type: "array",
					Items: &Schema{
						Type: "string",
					},
				},
			},
			Required: []string{"name"},
		},
	}
	if err := registry.Register(tool); err != nil {
		t.Fatalf("register tool: %v", err)
	}

	definitions, err := registry.Definitions([]string{tool.Name()})
	if err != nil {
		t.Fatalf("export definitions: %v", err)
	}

	if len(definitions) != 1 {
		t.Fatalf("expected 1 definition, got %d", len(definitions))
	}

	expected := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"name": map[string]any{
				"type": "string",
			},
			"namespace": map[string]any{
				"type":    "string",
				"default": "default",
			},
			"allow_namespaces": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "string",
				},
			},
		},
		"required": []string{"name"},
	}
	if !reflect.DeepEqual(definitions[0].Parameters, expected) {
		t.Fatalf("expected read_secret schema %#v, got %#v", expected, definitions[0].Parameters)
	}
}

func TestRegistryDefinitionsFallsBackToEmptyObjectSchema(t *testing.T) {
	registry := NewRegistry()

	legacyTool := registryTestTool{
		name:        "discovery.list_namespaces",
		description: "Lists namespaces",
	}
	if err := registry.Register(legacyTool); err != nil {
		t.Fatalf("register tool: %v", err)
	}

	definitions, err := registry.Definitions([]string{legacyTool.name})
	if err != nil {
		t.Fatalf("export definitions: %v", err)
	}

	if len(definitions) != 1 {
		t.Fatalf("expected 1 definition, got %d", len(definitions))
	}

	expected := EmptyObjectSchema().Map()
	if !reflect.DeepEqual(definitions[0].Parameters, expected) {
		t.Fatalf("expected fallback schema %#v, got %#v", expected, definitions[0].Parameters)
	}
}

func TestRegistryDefinitionsPreserveRequestedOrder(t *testing.T) {
	registry := NewRegistry()

	first := registryTestTool{name: "tool.first", description: "first"}
	second := registryTestTool{name: "tool.second", description: "second"}

	if err := registry.Register(first); err != nil {
		t.Fatalf("register first tool: %v", err)
	}
	if err := registry.Register(second); err != nil {
		t.Fatalf("register second tool: %v", err)
	}

	definitions, err := registry.Definitions([]string{second.Name(), first.Name()})
	if err != nil {
		t.Fatalf("export definitions: %v", err)
	}

	if len(definitions) != 2 {
		t.Fatalf("expected 2 definitions, got %d", len(definitions))
	}
	if definitions[0].Name != second.Name() || definitions[1].Name != first.Name() {
		t.Fatalf("expected requested order [%q %q], got [%q %q]", second.Name(), first.Name(), definitions[0].Name, definitions[1].Name)
	}
}
