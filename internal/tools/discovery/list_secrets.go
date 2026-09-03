package discovery

import (
	"context"

	"github.com/ashwnn/chain-reaction/internal/k8s"
	"github.com/ashwnn/chain-reaction/internal/tools"
)

type ListSecretsTool struct {
	client *k8s.Client
}

func NewListSecretsTool(client *k8s.Client) *ListSecretsTool {
	return &ListSecretsTool{client: client}
}

func (t *ListSecretsTool) Name() string {
	return "discovery.list_secrets"
}

func (t *ListSecretsTool) Description() string {
	return "Lists secrets in a namespace (metadata only - shows key names, not values)"
}

func (t *ListSecretsTool) ParameterSchema() tools.Schema {
	return namespaceParameterSchema()
}

func (t *ListSecretsTool) Run(ctx context.Context, input map[string]any) (map[string]any, error) {
	namespace := namespaceFromInput(input)

	secrets, err := t.client.ListSecrets(ctx, namespace)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"count":   len(secrets),
		"secrets": secrets,
	}, nil
}
