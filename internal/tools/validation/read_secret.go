package validation

import (
	"context"
	"fmt"

	"github.com/ashwnn/chain-reaction/internal/k8s"
	"github.com/ashwnn/chain-reaction/internal/tools"
)

type ReadSecretTool struct {
	client *k8s.Client
}

func NewReadSecretTool(client *k8s.Client) *ReadSecretTool {
	return &ReadSecretTool{client: client}
}

func (t *ReadSecretTool) Name() string {
	return "validation.read_secret"
}

func (t *ReadSecretTool) Description() string {
	return "Attempts a bounded read of a specific Secret and returns redacted summary metadata only"
}

func (t *ReadSecretTool) ParameterSchema() tools.Schema {
	return tools.Schema{
		Type: "object",
		Properties: map[string]tools.Schema{
			"name": {
				Type:        "string",
				Description: "Secret name to read",
			},
			"namespace": {
				Type:        "string",
				Description: "Namespace containing the Secret",
				Default:     "default",
			},
			"allow_namespaces": {
				Type:        "array",
				Description: "Optional namespace allow-list guardrail",
				Items: &tools.Schema{
					Type: "string",
				},
			},
		},
		Required: []string{"name"},
	}
}

func (t *ReadSecretTool) Run(ctx context.Context, input map[string]any) (map[string]any, error) {
	namespace := "default"
	name := ""
	allowedNamespaces := map[string]struct{}{}

	if input != nil {
		if v, ok := input["namespace"].(string); ok && v != "" {
			namespace = v
		}
		if v, ok := input["name"].(string); ok && v != "" {
			name = v
		}
		if rawAllowed, ok := input["allow_namespaces"].([]string); ok {
			for _, ns := range rawAllowed {
				allowedNamespaces[ns] = struct{}{}
			}
		}
		if rawAllowed, ok := input["allow_namespaces"].([]any); ok {
			for _, item := range rawAllowed {
				if ns, ok := item.(string); ok && ns != "" {
					allowedNamespaces[ns] = struct{}{}
				}
			}
		}
	}

	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if len(allowedNamespaces) > 0 {
		if _, ok := allowedNamespaces[namespace]; !ok {
			return map[string]any{
				"namespace": namespace,
				"name":      name,
				"status":    string(StepFailed),
				"reason":    string(FailureGuardrailBlocked),
			}, nil
		}
	}

	secret, err := t.client.ReadSecretSummary(ctx, namespace, name)
	if err != nil {
		status := string(StepFailed)
		reason := FailureToolOrRuntimeError
		if k8s.IsSecretAccessForbidden(err) {
			reason = FailureRBACDenied
		} else if k8s.IsSecretMissing(err) {
			reason = FailureSecretNotFound
		}
		return map[string]any{
			"namespace": namespace,
			"name":      name,
			"status":    status,
			"reason":    string(reason),
			"error":     err.Error(),
		}, nil
	}

	return map[string]any{
		"namespace": namespace,
		"name":      name,
		"status":    string(StepValidated),
		"reason":    "secret_read_succeeded",
		"secret":    secret,
	}, nil
}
