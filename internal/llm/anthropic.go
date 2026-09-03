package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultAnthropicBaseURL = "https://api.anthropic.com"
	anthropicAPIVersion     = "2023-06-01"
	defaultAnthropicMaxTok  = 4096
)

// anthropicProvider sends chat completion requests to the Anthropic Messages API.
type anthropicProvider struct {
	httpClient  *http.Client
	baseURL     string
	apiKey      string
	provider    string
	model       string
	temperature *float64
	maxTokens   int
}

func newAnthropicProvider(cfg ProviderConfig, timeout time.Duration) (*anthropicProvider, error) {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = defaultAnthropicBaseURL
	}
	maxTokens := defaultAnthropicMaxTok
	if cfg.MaxTokens != nil && *cfg.MaxTokens > 0 {
		maxTokens = *cfg.MaxTokens
	}
	return &anthropicProvider{
		httpClient:  &http.Client{Timeout: timeout},
		baseURL:     baseURL,
		apiKey:      cfg.APIKey,
		provider:    "anthropic",
		model:       cfg.Model,
		temperature: cfg.Temperature,
		maxTokens:   maxTokens,
	}, nil
}

// --- Anthropic request types ---

type anthropicRequest struct {
	Model       string               `json:"model"`
	MaxTokens   int                  `json:"max_tokens"`
	System      string               `json:"system,omitempty"`
	Messages    []anthropicMessage   `json:"messages"`
	Tools       []anthropicTool      `json:"tools,omitempty"`
	ToolChoice  *anthropicToolChoice `json:"tool_choice,omitempty"`
	Temperature *float64             `json:"temperature,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"input_schema"`
}

type anthropicToolChoice struct {
	Type string `json:"type"` // "auto", "any", "tool", "none"
}

// --- Anthropic response types ---

type anthropicResponse struct {
	ID         string                  `json:"id"`
	Type       string                  `json:"type"`
	Role       string                  `json:"role"`
	Content    []anthropicContentBlock `json:"content"`
	StopReason string                  `json:"stop_reason"`
	Usage      *anthropicUsage         `json:"usage,omitempty"`
	Error      *anthropicError         `json:"error,omitempty"`
}

type anthropicContentBlock struct {
	Type  string         `json:"type"` // "text" or "tool_use"
	Text  string         `json:"text,omitempty"`
	ID    string         `json:"id,omitempty"`
	Name  string         `json:"name,omitempty"`
	Input map[string]any `json:"input,omitempty"`
}

type anthropicUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
}

type anthropicError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// --- Anthropic error response (top-level) ---

type anthropicErrorResponse struct {
	Type  string          `json:"type"`
	Error *anthropicError `json:"error,omitempty"`
}

func (p *anthropicProvider) Complete(ctx context.Context, messages []ChatMessage, tools []ToolDefinition) (ChatAction, error) {
	// Separate system messages from conversation messages.
	// Anthropic uses a top-level "system" field instead of a system role in messages.
	var systemPrompt string
	var convMessages []anthropicMessage
	for _, m := range messages {
		if m.Role == "system" {
			if systemPrompt != "" {
				systemPrompt += "\n\n"
			}
			systemPrompt += m.Content
		} else {
			convMessages = append(convMessages, anthropicMessage{
				Role:    m.Role,
				Content: m.Content,
			})
		}
	}

	req := anthropicRequest{
		Model:     p.model,
		MaxTokens: p.maxTokens,
		System:    systemPrompt,
		Messages:  convMessages,
	}
	if p.temperature != nil {
		req.Temperature = p.temperature
	}

	if len(tools) > 0 {
		anthropicTools := make([]anthropicTool, len(tools))
		for i, t := range tools {
			// Anthropic uses "input_schema" instead of "parameters"
			inputSchema := t.Parameters
			if inputSchema == nil {
				inputSchema = map[string]any{"type": "object"}
			}
			anthropicTools[i] = anthropicTool{
				Name:        t.Name,
				Description: t.Description,
				InputSchema: inputSchema,
			}
		}
		req.Tools = anthropicTools
		req.ToolChoice = &anthropicToolChoice{Type: "auto"}
	}

	body, err := json.Marshal(req)
	if err != nil {
		return ChatAction{}, fmt.Errorf("marshal anthropic request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return ChatAction{}, fmt.Errorf("build anthropic http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicAPIVersion)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return ChatAction{}, fmt.Errorf("anthropic request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return ChatAction{}, fmt.Errorf("read anthropic response: %w", err)
	}

	// Check for retryable HTTP status codes before parsing.
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return ChatAction{}, &retryableError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("anthropic returned status %d: %s", resp.StatusCode, string(respBody)),
		}
	}

	// Handle error responses (4xx).
	if resp.StatusCode != http.StatusOK {
		var errResp anthropicErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Error != nil {
			return ChatAction{}, fmt.Errorf("anthropic api error (%s): %s", errResp.Error.Type, errResp.Error.Message)
		}
		return ChatAction{}, fmt.Errorf("unexpected status %d from anthropic: %s", resp.StatusCode, string(respBody))
	}

	var arResp anthropicResponse
	if err := json.Unmarshal(respBody, &arResp); err != nil {
		return ChatAction{}, fmt.Errorf("unmarshal anthropic response: %w", err)
	}

	if arResp.Error != nil {
		return ChatAction{}, fmt.Errorf("anthropic api error (%s): %s", arResp.Error.Type, arResp.Error.Message)
	}

	// Look for tool_use content blocks first.
	var thought string
	for _, block := range arResp.Content {
		if block.Type == "tool_use" {
			return ChatAction{
				ActionType: ChatActionExecute,
				ToolName:   block.Name,
				Parameters: block.Input,
				Usage:      usageMetadataFromAnthropic(p.provider, p.model, arResp.Usage),
				Thought:    strings.TrimSpace(thought),
			}, nil
		}
		if block.Type == "text" && block.Text != "" && thought == "" {
			// Capture the first text block as thought; subsequent text blocks
			// accumulate into the final answer if no tool_use appears.
			thought = block.Text
		}
	}

	// Collect all text blocks as the final answer.
	var textParts []string
	for _, block := range arResp.Content {
		if block.Type == "text" && block.Text != "" {
			textParts = append(textParts, block.Text)
		}
	}

	return ChatAction{
		ActionType:  ChatActionFinalAnswer,
		FinalAnswer: strings.TrimSpace(strings.Join(textParts, "\n")),
		Usage:       usageMetadataFromAnthropic(p.provider, p.model, arResp.Usage),
	}, nil
}

func usageMetadataFromAnthropic(provider, model string, usage *anthropicUsage) *UsageMetadata {
	if usage == nil {
		return nil
	}

	raw := map[string]any{
		"input_tokens":  usage.InputTokens,
		"output_tokens": usage.OutputTokens,
	}
	if usage.CacheCreationInputTokens > 0 {
		raw["cache_creation_input_tokens"] = usage.CacheCreationInputTokens
	}
	if usage.CacheReadInputTokens > 0 {
		raw["cache_read_input_tokens"] = usage.CacheReadInputTokens
	}

	um := newUsageMetadata(
		provider,
		usage.InputTokens,
		usage.OutputTokens,
		0,
		raw,
		usage.CacheReadInputTokens,
		usage.CacheCreationInputTokens,
		usage.InputTokens,
	)
	if um == nil {
		return nil
	}
	um.EstimatedCostUSD = estimateCost(um, provider, model)
	return um
}
