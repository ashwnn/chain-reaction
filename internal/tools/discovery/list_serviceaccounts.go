package discovery

import (
	"context"

	"github.com/ashwnn/chain-reaction/internal/k8s"
	"github.com/ashwnn/chain-reaction/internal/tools"
)

type ListServiceAccountsTool struct {
	client *k8s.Client
}

func NewListServiceAccountsTool(client *k8s.Client) *ListServiceAccountsTool {
	return &ListServiceAccountsTool{client: client}
}

func (t *ListServiceAccountsTool) Name() string {
	return "discovery.list_serviceaccounts"
}

func (t *ListServiceAccountsTool) Description() string {
	return "Lists service accounts in a namespace for RBAC analysis"
}

func (t *ListServiceAccountsTool) ParameterSchema() tools.Schema {
	return namespaceParameterSchema()
}

func (t *ListServiceAccountsTool) Run(ctx context.Context, input map[string]any) (map[string]any, error) {
	namespace := namespaceFromInput(input)

	sas, err := t.client.ListServiceAccounts(ctx, namespace)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"count":            len(sas),
		"service_accounts": sas,
	}, nil
}
