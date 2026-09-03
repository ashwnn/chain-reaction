package discovery

import "github.com/ashwnn/chain-reaction/internal/tools"

func namespaceParameterSchema() tools.Schema {
	return tools.Schema{
		Type: "object",
		Properties: map[string]tools.Schema{
			"namespace": {
				Type:        "string",
				Description: "Kubernetes namespace to query",
				Default:     "default",
			},
		},
	}
}

func namespaceFromInput(input map[string]any) string {
	if input == nil {
		return "default"
	}

	namespace, ok := input["namespace"].(string)
	if !ok || namespace == "" {
		return "default"
	}

	return namespace
}
