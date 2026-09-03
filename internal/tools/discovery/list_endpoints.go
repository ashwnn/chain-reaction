package discovery

import (
	"context"

	"github.com/ashwnn/chain-reaction/internal/k8s"
	"github.com/ashwnn/chain-reaction/internal/tools"
)

type ListEndpointsTool struct {
	client *k8s.Client
}

func NewListEndpointsTool(client *k8s.Client) *ListEndpointsTool {
	return &ListEndpointsTool{client: client}
}

func (t *ListEndpointsTool) Name() string {
	return "discovery.list_endpoints"
}

func (t *ListEndpointsTool) Description() string {
	return "Lists endpoints in a namespace for network reachability analysis"
}

func (t *ListEndpointsTool) ParameterSchema() tools.Schema {
	return namespaceParameterSchema()
}

func (t *ListEndpointsTool) Run(ctx context.Context, input map[string]any) (map[string]any, error) {
	namespace := namespaceFromInput(input)

	endpoints, err := t.client.ListEndpoints(ctx, namespace)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"count":     len(endpoints),
		"endpoints": endpoints,
	}, nil
}
