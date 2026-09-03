package discovery

import (
	"context"

	"github.com/ashwnn/chain-reaction/internal/k8s"
	"github.com/ashwnn/chain-reaction/internal/tools"
)

type ListPodsTool struct {
	client *k8s.Client
}

func NewListPodsTool(client *k8s.Client) *ListPodsTool {
	return &ListPodsTool{client: client}
}

func (t *ListPodsTool) Name() string {
	return "discovery.list_pods"
}

func (t *ListPodsTool) Description() string {
	return "Lists pods in a namespace with metadata for attack chain discovery"
}

func (t *ListPodsTool) ParameterSchema() tools.Schema {
	return namespaceParameterSchema()
}

func (t *ListPodsTool) Run(ctx context.Context, input map[string]any) (map[string]any, error) {
	namespace := namespaceFromInput(input)

	pods, err := t.client.ListPods(ctx, namespace)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"count": len(pods),
		"pods":  pods,
	}, nil
}
