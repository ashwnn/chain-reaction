package discovery

import (
	"context"

	"github.com/ashwnn/chain-reaction/internal/k8s"
	"github.com/ashwnn/chain-reaction/internal/tools"
)

type ListNetworkPoliciesTool struct {
	client *k8s.Client
}

func NewListNetworkPoliciesTool(client *k8s.Client) *ListNetworkPoliciesTool {
	return &ListNetworkPoliciesTool{client: client}
}

func (t *ListNetworkPoliciesTool) Name() string {
	return "discovery.list_networkpolicies"
}

func (t *ListNetworkPoliciesTool) Description() string {
	return "Lists network policies in a namespace for network segmentation analysis"
}

func (t *ListNetworkPoliciesTool) ParameterSchema() tools.Schema {
	return namespaceParameterSchema()
}

func (t *ListNetworkPoliciesTool) Run(ctx context.Context, input map[string]any) (map[string]any, error) {
	namespace := namespaceFromInput(input)

	policies, err := t.client.ListNetworkPolicies(ctx, namespace)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"count":            len(policies),
		"network_policies": policies,
	}, nil
}
