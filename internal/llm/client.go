package llm

import (
	"context"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
)

type Planner interface {
	SuggestNextTool(ctx context.Context, prompt string) (string, error)
}

type StaticPlanner struct{}

func (p *StaticPlanner) SuggestNextTool(_ context.Context, _ string) (string, error) {
	return "discovery.list_namespaces", nil
}

type OpenAIPlanner struct {
	client *openai.Client
	model  string
}

func NewOpenAIPlanner(apiKey, model string) *OpenAIPlanner {
	client := openai.NewClient(option.WithAPIKey(apiKey))
	return &OpenAIPlanner{client: &client, model: model}
}

func (p *OpenAIPlanner) SuggestNextTool(ctx context.Context, prompt string) (string, error) {
	resp, err := p.client.Responses.New(ctx, responses.ResponseNewParams{
		Model: openai.ChatModel(p.model),
		Input: responses.ResponseNewParamsInputUnion{
			OfString: openai.String("Return only one tool name. Available tool: discovery.list_namespaces. Context: " + prompt),
		},
	})
	if err != nil {
		return "", err
	}

	if text := resp.OutputText(); text != "" {
		return text, nil
	}

	return "discovery.list_namespaces", nil
}

func NewPlanner(apiKey, model string) Planner {
	if apiKey == "" {
		return &StaticPlanner{}
	}
	return NewOpenAIPlanner(apiKey, model)
}
