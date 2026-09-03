package introspection

import (
	"context"

	"github.com/ashwnn/chain-reaction/internal/k8s"
	"github.com/ashwnn/chain-reaction/internal/tools"
)

type GetEffectivePermissionsTool struct {
	client *k8s.Client
}

func NewGetEffectivePermissionsTool(client *k8s.Client) *GetEffectivePermissionsTool {
	return &GetEffectivePermissionsTool{client: client}
}

func (t *GetEffectivePermissionsTool) Name() string {
	return "introspection.get_effective_permissions"
}

func (t *GetEffectivePermissionsTool) Description() string {
	return "Summarizes effective RBAC permissions for the current identity in a namespace"
}

func (t *GetEffectivePermissionsTool) ParameterSchema() tools.Schema {
	return tools.Schema{
		Type: "object",
		Properties: map[string]tools.Schema{
			"namespace": {
				Type:        "string",
				Description: "Kubernetes namespace to evaluate",
				Default:     "default",
			},
		},
	}
}

func (t *GetEffectivePermissionsTool) Run(ctx context.Context, input map[string]any) (map[string]any, error) {
	namespace := "default"
	if input != nil {
		if ns, ok := input["namespace"].(string); ok && ns != "" {
			namespace = ns
		}
	}

	permissions, err := t.client.GetEffectivePermissions(ctx, namespace)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"namespace":           permissions.Namespace,
		"resource_rule_count": len(permissions.ResourceRules),
		"resource_rules":      permissions.ResourceRules,
		"non_resource_rules":  permissions.NonResourceRules,
		"incomplete":          permissions.Incomplete,
		"evaluation_error":    permissions.EvaluationError,
	}, nil
}
