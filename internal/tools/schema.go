package tools

import "fmt"

type Definition struct {
	Name        string
	Description string
	Parameters  map[string]any
}

type Schema struct {
	Type                 string
	Description          string
	Properties           map[string]Schema
	Required             []string
	Enum                 []string
	Items                *Schema
	Default              any
	AdditionalProperties *bool
}

func EmptyObjectSchema() Schema {
	additionalProperties := true
	return Schema{
		Type:                 "object",
		Properties:           map[string]Schema{},
		AdditionalProperties: &additionalProperties,
	}
}

// ValidateParameters checks that input satisfies the constraints defined in schema.
// It covers the minimal surface needed for planner-action parameter validation:
// required fields, type checking (string, array), enum constraints, and
// unknown-property rejection. It intentionally does not cover nested objects,
// patterns, formats, or other features not present in the current tool schemas.
func ValidateParameters(input map[string]any, schema Schema) error {
	if input == nil {
		input = make(map[string]any)
	}

	// Build a set of known property keys for unknown-property checking.
	knownProps := make(map[string]struct{}, len(schema.Properties))
	for name := range schema.Properties {
		knownProps[name] = struct{}{}
	}

	// Unknown-property guard: reject keys not declared in the schema.
	if schema.AdditionalProperties != nil && !*schema.AdditionalProperties {
		for key := range input {
			if _, ok := knownProps[key]; !ok {
				return &ParameterValidationError{
					ToolName:  "",
					Parameter: key,
					Message:   "unknown parameter",
				}
			}
		}
	}

	// Required-field and type checking.
	for name, propSchema := range schema.Properties {
		val, present := input[name]
		if !present || val == nil {
			// Missing or nil: check if required.
			for _, req := range schema.Required {
				if req == name {
					return &ParameterValidationError{
						ToolName:  "",
						Parameter: name,
						Message:   "missing required parameter",
					}
				}
			}
			continue
		}

		// Type check.
		switch propSchema.Type {
		case "string":
			if _, ok := val.(string); !ok {
				return &ParameterValidationError{
					ToolName:  "",
					Parameter: name,
					Message:   "must be string",
					Actual:    fmt.Sprintf("%T", val),
				}
			}
		case "integer":
			switch v := val.(type) {
			case int, int8, int16, int32, int64:
				// valid
			case float64:
				// JSON unmarshals numbers as float64; allow integer-valued float64s
				if v != float64(int64(v)) {
					return &ParameterValidationError{
						ToolName:  "",
						Parameter: name,
						Message:   "must be integer",
						Actual:    fmt.Sprintf("%T", val),
					}
				}
			default:
				return &ParameterValidationError{
					ToolName:  "",
					Parameter: name,
					Message:   "must be integer",
					Actual:    fmt.Sprintf("%T", val),
				}
			}
		case "array":
			switch val.(type) {
			case []any, []string:
				// Acceptable.
			default:
				return &ParameterValidationError{
					ToolName:  "",
					Parameter: name,
					Message:   "must be array",
					Actual:    fmt.Sprintf("%T", val),
				}
			}
		case "object":
			// Nested objects not yet needed for the current tool surface; skip.
		}

		// Enum check.
		if len(propSchema.Enum) > 0 {
			valStr, isStr := val.(string)
			if !isStr {
				return &ParameterValidationError{
					ToolName:  "",
					Parameter: name,
					Message:   "must be one of the allowed values",
					Actual:    fmt.Sprintf("%T", val),
				}
			}
			found := false
			for _, e := range propSchema.Enum {
				if e == valStr {
					found = true
					break
				}
			}
			if !found {
				return &ParameterValidationError{
					ToolName:  "",
					Parameter: name,
					Message:   "value not in allowed enum",
					Actual:    valStr,
				}
			}
		}
	}

	return nil
}

// ParameterValidationError describes a single parameter validation failure.
type ParameterValidationError struct {
	ToolName  string
	Parameter string
	Message   string
	Actual    string
}

func (e *ParameterValidationError) Error() string {
	if e.Parameter == "" {
		return e.Message
	}
	msg := fmt.Sprintf("parameter %q %s", e.Parameter, e.Message)
	if e.Actual != "" {
		msg += fmt.Sprintf(", got %s", e.Actual)
	}
	return msg
}

func (s Schema) Map() map[string]any {
	if s.Type == "" {
		s = EmptyObjectSchema()
	}

	out := map[string]any{
		"type": s.Type,
	}
	if s.Description != "" {
		out["description"] = s.Description
	}
	if s.Properties != nil {
		properties := make(map[string]any, len(s.Properties))
		for name, property := range s.Properties {
			properties[name] = property.Map()
		}
		out["properties"] = properties
	}
	if len(s.Required) > 0 {
		required := make([]string, len(s.Required))
		copy(required, s.Required)
		out["required"] = required
	}
	if len(s.Enum) > 0 {
		enum := make([]string, len(s.Enum))
		copy(enum, s.Enum)
		out["enum"] = enum
	}
	if s.Items != nil {
		out["items"] = s.Items.Map()
	}
	if s.Default != nil {
		out["default"] = s.Default
	}
	if s.Type == "object" && s.AdditionalProperties == nil {
		out["additionalProperties"] = false
	}
	if s.AdditionalProperties != nil {
		out["additionalProperties"] = *s.AdditionalProperties
	}

	return out
}
