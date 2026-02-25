package discovery

import (
	"context"

	"github.com/ashwnn/chain-reaction/internal/k8s"
)

type ListNamespacesTool struct {
	client *k8s.Client
}

func NewListNamespacesTool(client *k8s.Client) *ListNamespacesTool {
	return &ListNamespacesTool{client: client}
}

func (t *ListNamespacesTool) Name() string {
	return "discovery.list_namespaces"
}

func (t *ListNamespacesTool) Description() string {
	return "Lists namespaces visible to current identity"
}

func (t *ListNamespacesTool) Run(ctx context.Context, _ map[string]any) (map[string]any, error) {
	namespaces, err := t.client.ListNamespaces(ctx)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"count":      len(namespaces),
		"namespaces": namespaces,
	}, nil
}
