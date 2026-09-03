package discovery

import (
	"context"

	"github.com/ashwnn/chain-reaction/internal/k8s"
	"github.com/ashwnn/chain-reaction/internal/tools"
)

type ListRoleBindingsTool struct {
	client *k8s.Client
}

func NewListRoleBindingsTool(client *k8s.Client) *ListRoleBindingsTool {
	return &ListRoleBindingsTool{client: client}
}

func (t *ListRoleBindingsTool) Name() string {
	return "discovery.list_rolebindings"
}

func (t *ListRoleBindingsTool) Description() string {
	return "Lists RBAC role bindings in a namespace for permission analysis"
}

func (t *ListRoleBindingsTool) ParameterSchema() tools.Schema {
	return namespaceParameterSchema()
}

func (t *ListRoleBindingsTool) Run(ctx context.Context, input map[string]any) (map[string]any, error) {
	namespace := namespaceFromInput(input)

	bindings, err := t.client.ListRoleBindings(ctx, namespace)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"count":         len(bindings),
		"role_bindings": bindings,
	}, nil
}
