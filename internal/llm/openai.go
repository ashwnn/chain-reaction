package llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultOpenAIBaseURL = "https://api.openai.com/v1"

// openaiProvider sends chat completion requests to OpenAI-compatible endpoints.
type openaiProvider struct {
	httpClient     *http.Client
	provider       string
	baseURL        string
	apiKey         string
	model          string
	temperature    *float64
	maxTokens      *int
	promptCacheKey *string // optional; nil means do not send the field
}

func newOpenAIProvider(cfg ProviderConfig, timeout time.Duration) (*openaiProvider, error) {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = defaultOpenAIBaseURL
	}
	provider := normalizeProviderName(cfg.Provider)
	if provider == "" {
		provider = "openai"
	}
	return &openaiProvider{
		httpClient:     &http.Client{Timeout: timeout},
		provider:       provider,
		baseURL:        baseURL,
		apiKey:         cfg.APIKey,
		model:          cfg.Model,
		temperature:    cfg.Temperature,
		maxTokens:      cfg.MaxTokens,
		promptCacheKey: cfg.PromptCacheKey,
	}, nil
}

// openaiChatRequest is the payload sent to /v1/chat/completions.
type openaiChatRequest struct {
	Model          string        `json:"model"`
	Messages       []ChatMessage `json:"messages"`
	Tools          []openaiTool  `json:"tools,omitempty"`
	Temperature    *float64      `json:"temperature,omitempty"`
	MaxTokens      *int          `json:"max_tokens,omitempty"`
	ToolChoice     string        `json:"tool_choice,omitempty"`
	PromptCacheKey *string       `json:"prompt_cache_key,omitempty"`
}

type openaiTool struct {
	Type     string         `json:"type"`
	Function ToolDefinition `json:"function"`
}

type openaiChatResponse struct {
	Choices []openaiChoice  `json:"choices"`
	Usage   *openAIUsage    `json:"usage,omitempty"`
	Error   *openaiAPIError `json:"error,omitempty"`
}

type openAIUsage struct {
	PromptTokens          int                       `json:"prompt_tokens,omitempty"`
	CompletionTokens      int                       `json:"completion_tokens,omitempty"`
	TotalTokens           int                       `json:"total_tokens,omitempty"`
	PromptTokensDetails   *openAIPromptTokenDetails `json:"prompt_tokens_details,omitempty"`
	PromptCacheHitTokens  int                       `json:"prompt_cache_hit_tokens,omitempty"`
	PromptCacheMissTokens int                       `json:"prompt_cache_miss_tokens,omitempty"`
}

type openAIPromptTokenDetails struct {
	CachedTokens int `json:"cached_tokens,omitempty"`
}

type openaiChoice struct {
	Message openaiChoiceMessage `json:"message"`
}

type openaiChoiceMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	ToolCalls []openaiToolCall `json:"tool_calls"`
}

type openaiToolCall struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Function openaiToolCallFunction `json:"function"`
}

type openaiToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openaiAPIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

func (p *openaiProvider) Complete(ctx context.Context, messages []ChatMessage, tools []ToolDefinition) (ChatAction, error) {
	req := openaiChatRequest{
		Model:    p.model,
		Messages: messages,
	}
	// Attach prompt_cache_key when configured. This is OpenAI-specific; other
	// providers ignore the PromptCacheKey config field. The field is omitted
	// from the JSON payload when nil (omitempty), preserving compatibility with
	// servers that do not support prompt caching.
	if p.promptCacheKey != nil && *p.promptCacheKey != "" {
		req.PromptCacheKey = p.promptCacheKey
	}
	toolAliases := map[string]string{}
	if p.temperature != nil {
		req.Temperature = p.temperature
	}
	if p.maxTokens != nil {
		req.MaxTokens = p.maxTokens
	}
	if len(tools) > 0 {
		chatTools := make([]openaiTool, len(tools))
		for i, t := range tools {
			toolName := openAICompatibleToolName(t.Name)
			toolAliases[toolName] = t.Name
			definition := t
			definition.Name = toolName
			chatTools[i] = openaiTool{Type: "function", Function: definition}
		}
		req.Tools = chatTools
		req.ToolChoice = "auto"
	}

	body, err := json.Marshal(req)
	if err != nil {
		return ChatAction{}, fmt.Errorf("marshal openai request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return ChatAction{}, fmt.Errorf("build http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return ChatAction{}, fmt.Errorf("openai chat completion request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return ChatAction{}, fmt.Errorf("read openai response: %w", err)
	}

	// Check for retryable HTTP status codes before parsing.
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return ChatAction{}, &retryableError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("openai returned status %d: %s", resp.StatusCode, string(respBody)),
		}
	}

	var ccResp openaiChatResponse
	if err := json.Unmarshal(respBody, &ccResp); err != nil {
		return ChatAction{}, fmt.Errorf("unmarshal openai response: %w", err)
	}

	if ccResp.Error != nil {
		return ChatAction{}, fmt.Errorf("openai api error (%s): %s", ccResp.Error.Type, ccResp.Error.Message)
	}

	if resp.StatusCode != http.StatusOK {
		return ChatAction{}, fmt.Errorf("unexpected status %d from openai", resp.StatusCode)
	}

	if len(ccResp.Choices) == 0 {
		return ChatAction{}, fmt.Errorf("openai returned no choices")
	}

	msg := ccResp.Choices[0].Message
	usage := usageMetadataFromOpenAI(p.provider, p.model, ccResp.Usage)

	if len(msg.ToolCalls) > 0 {
		tc := msg.ToolCalls[0]
		params := map[string]any{}
		if tc.Function.Arguments != "" {
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &params); err != nil {
				return ChatAction{}, fmt.Errorf("parse tool_call arguments for %q: %w", tc.Function.Name, err)
			}
		}
		toolName := tc.Function.Name
		if originalName, ok := toolAliases[toolName]; ok {
			toolName = originalName
		}
		return ChatAction{
			ActionType: ChatActionExecute,
			ToolName:   toolName,
			Parameters: params,
			Usage:      usage,
			Thought:    strings.TrimSpace(msg.Content),
		}, nil
	}

	return ChatAction{
		ActionType:  ChatActionFinalAnswer,
		FinalAnswer: strings.TrimSpace(msg.Content),
		Usage:       usage,
	}, nil
}

func usageMetadataFromOpenAI(provider, model string, usage *openAIUsage) *UsageMetadata {
	if usage == nil {
		return nil
	}

	raw := map[string]any{
		"prompt_tokens":     usage.PromptTokens,
		"completion_tokens": usage.CompletionTokens,
		"total_tokens":      usage.TotalTokens,
	}
	cacheRead := 0
	cacheWrite := 0
	uncached := usage.PromptTokens

	if usage.PromptTokensDetails != nil {
		raw["prompt_tokens_details"] = map[string]any{
			"cached_tokens": usage.PromptTokensDetails.CachedTokens,
		}
		cacheRead = usage.PromptTokensDetails.CachedTokens
		uncached = usage.PromptTokens - cacheRead
		if uncached < 0 {
			uncached = 0
		}
	}

	um := newUsageMetadata(provider, usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens, raw, cacheRead, cacheWrite, uncached)
	if um == nil {
		return nil
	}
	// Populate estimated cost if pricing is available for this provider/model.
	um.EstimatedCostUSD = estimateCost(um, provider, model)
	return um
}

func openAICompatibleToolName(name string) string {
	if isOpenAICompatibleToolName(name) {
		return name
	}
	return "cr_alias_" + base64.RawURLEncoding.EncodeToString([]byte(name))
}

func isOpenAICompatibleToolName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}
