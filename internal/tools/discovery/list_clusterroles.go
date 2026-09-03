package discovery

import (
	"context"

	"github.com/ashwnn/chain-reaction/internal/k8s"
	"github.com/ashwnn/chain-reaction/internal/tools"
)

type ListClusterRolesTool struct {
	client *k8s.Client
}

func NewListClusterRolesTool(client *k8s.Client) *ListClusterRolesTool {
	return &ListClusterRolesTool{client: client}
}

func (t *ListClusterRolesTool) Name() string {
	return "discovery.list_clusterroles"
}

func (t *ListClusterRolesTool) Description() string {
	return "Lists RBAC cluster roles for permission analysis across all namespaces"
}

func (t *ListClusterRolesTool) ParameterSchema() tools.Schema {
	return tools.EmptyObjectSchema()
}

func (t *ListClusterRolesTool) Run(ctx context.Context, _ map[string]any) (map[string]any, error) {
	roles, err := t.client.ListClusterRoles(ctx)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"count":        len(roles),
		"clusterroles": roles,
	}, nil
}
