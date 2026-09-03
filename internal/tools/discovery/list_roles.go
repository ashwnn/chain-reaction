package discovery

import (
	"context"

	"github.com/ashwnn/chain-reaction/internal/k8s"
	"github.com/ashwnn/chain-reaction/internal/tools"
)

type ListRolesTool struct {
	client *k8s.Client
}

func NewListRolesTool(client *k8s.Client) *ListRolesTool {
	return &ListRolesTool{client: client}
}

func (t *ListRolesTool) Name() string {
	return "discovery.list_roles"
}

func (t *ListRolesTool) Description() string {
	return "Lists RBAC roles in a namespace for permission analysis"
}

func (t *ListRolesTool) ParameterSchema() tools.Schema {
	return namespaceParameterSchema()
}

func (t *ListRolesTool) Run(ctx context.Context, input map[string]any) (map[string]any, error) {
	namespace := namespaceFromInput(input)

	roles, err := t.client.ListRoles(ctx, namespace)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"count": len(roles),
		"roles": roles,
	}, nil
}
