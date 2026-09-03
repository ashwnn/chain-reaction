package discovery

import (
	"context"

	"github.com/ashwnn/chain-reaction/internal/k8s"
	"github.com/ashwnn/chain-reaction/internal/tools"
)

type ListServicesTool struct {
	client *k8s.Client
}

func NewListServicesTool(client *k8s.Client) *ListServicesTool {
	return &ListServicesTool{client: client}
}

func (t *ListServicesTool) Name() string {
	return "discovery.list_services"
}

func (t *ListServicesTool) Description() string {
	return "Lists services in a namespace with metadata for attack chain discovery"
}

func (t *ListServicesTool) ParameterSchema() tools.Schema {
	return namespaceParameterSchema()
}

func (t *ListServicesTool) Run(ctx context.Context, input map[string]any) (map[string]any, error) {
	namespace := namespaceFromInput(input)

	services, err := t.client.ListServices(ctx, namespace)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"count":    len(services),
		"services": services,
	}, nil
}
