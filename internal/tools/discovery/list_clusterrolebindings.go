package discovery

import (
	"context"

	"github.com/ashwnn/chain-reaction/internal/k8s"
	"github.com/ashwnn/chain-reaction/internal/tools"
)

type ListClusterRoleBindingsTool struct {
	client *k8s.Client
}

func NewListClusterRoleBindingsTool(client *k8s.Client) *ListClusterRoleBindingsTool {
	return &ListClusterRoleBindingsTool{client: client}
}

func (t *ListClusterRoleBindingsTool) Name() string {
	return "discovery.list_clusterrolebindings"
}

func (t *ListClusterRoleBindingsTool) Description() string {
	return "Lists RBAC cluster role bindings for permission analysis across all namespaces"
}

func (t *ListClusterRoleBindingsTool) ParameterSchema() tools.Schema {
	return tools.EmptyObjectSchema()
}

func (t *ListClusterRoleBindingsTool) Run(ctx context.Context, _ map[string]any) (map[string]any, error) {
	bindings, err := t.client.ListClusterRoleBindings(ctx)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"count":                len(bindings),
		"clusterrole_bindings": bindings,
	}, nil
}
