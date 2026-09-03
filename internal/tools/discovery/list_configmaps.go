package discovery

import (
	"context"

	"github.com/ashwnn/chain-reaction/internal/k8s"
	"github.com/ashwnn/chain-reaction/internal/tools"
)

type ListConfigMapsTool struct {
	client *k8s.Client
}

func NewListConfigMapsTool(client *k8s.Client) *ListConfigMapsTool {
	return &ListConfigMapsTool{client: client}
}

func (t *ListConfigMapsTool) Name() string {
	return "discovery.list_configmaps"
}

func (t *ListConfigMapsTool) Description() string {
	return "Lists ConfigMaps in a namespace with metadata and key names for attack chain discovery"
}

func (t *ListConfigMapsTool) ParameterSchema() tools.Schema {
	return namespaceParameterSchema()
}

func (t *ListConfigMapsTool) Run(ctx context.Context, input map[string]any) (map[string]any, error) {
	namespace := namespaceFromInput(input)

	configMaps, err := t.client.ListConfigMaps(ctx, namespace)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"count":      len(configMaps),
		"configmaps": configMaps,
	}, nil
}
