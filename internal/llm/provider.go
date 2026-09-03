package llm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// defaultHTTPTimeout is applied to each individual HTTP call.
const defaultHTTPTimeout = 60 * time.Second

// ChatActionType indicates what the model decided to do.
type ChatActionType string

const (
	ChatActionExecute     ChatActionType = "execute"
	ChatActionFinalAnswer ChatActionType = "final_answer"
)

// ChatAction is the parsed result of a completion call.
// It mirrors agent.PlannerAction so that agent can convert from ChatAction
// without creating a circular import between llm and agent.
type ChatAction struct {
	ActionType  ChatActionType `json:"action_type"`
	ToolName    string         `json:"tool_name,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
	FinalAnswer string         `json:"final_answer,omitempty"`
	Usage       *UsageMetadata `json:"usage,omitempty"`
	// Thought captures the model's pre-action reasoning. Populated by providers
	// from text emitted before tool calls. Empty when the model produces no
	// intermediate reasoning.
	Thought string `json:"thought,omitempty"`
}

// modelPricing holds per-token pricing for a specific model version.
// Prices are in USD per million tokens. Versioned to allow updates without
// breaking existing calculations.
type modelPricing struct {
	Version    string  // e.g. "2026-04-01"
	InputPerM  float64 // USD per million input tokens
	OutputPerM float64 // USD per million output tokens
}

// knownModelPricing maps provider+model identifiers to their pricing.
// Keys are "provider/model" in lowercase.
var knownModelPricing = map[string]modelPricing{
	"openai/gpt-4o":                                  {Version: "2026-04-01", InputPerM: 2.50, OutputPerM: 10.00},
	"openai/gpt-4o-mini":                             {Version: "2026-04-01", InputPerM: 0.15, OutputPerM: 0.60},
	"anthropic/claude-3-5-sonnet":                    {Version: "2026-04-01", InputPerM: 3.00, OutputPerM: 15.00},
	"anthropic/claude-3-5-haiku":                     {Version: "2026-04-01", InputPerM: 0.80, OutputPerM: 4.00},
	"groq/meta-llama-llama-4-scout-17b-16e-instruct": {Version: "2026-04-12", InputPerM: 0.11, OutputPerM: 0.34},
}

// UsageMetadata preserves provider-reported token accounting for a completion.
// Normalized counts stay stable across providers while Raw retains provider-
// specific fields for later evidence capture. EstimatedCostUSD is populated
// by estimateCost when pricing is available for the provider+model; it is nil
// when pricing is unknown (safe degradation — usage is surfaced, cost is not invented).
type UsageMetadata struct {
	Provider         string         `json:"provider,omitempty"`
	InputTokens      int            `json:"input_tokens,omitempty"`
	OutputTokens     int            `json:"output_tokens,omitempty"`
	TotalTokens      int            `json:"total_tokens,omitempty"`
	CacheRead        int            `json:"cache_read_tokens,omitempty"`
	CacheWrite       int            `json:"cache_write_tokens,omitempty"`
	Uncached         int            `json:"uncached_tokens,omitempty"`
	Raw              map[string]any `json:"raw,omitempty"`
	EstimatedCostUSD *float64       `json:"estimated_cost_usd,omitempty"`
}

// ToolDefinition describes a tool the LLM may call.
type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// ChatMessage is a single entry in the conversation history.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Provider is the interface for LLM chat completion with tool calling.
// The interface is designed to be configurable; built-in adapters include
// OpenAI-compatible, Anthropic, and Groq, and additional providers can
// be added by implementing this interface.
type Provider interface {
	Complete(ctx context.Context, messages []ChatMessage, tools []ToolDefinition) (ChatAction, error)
}

// ProviderConfig holds all configurable fields for creating a Provider.
type ProviderConfig struct {
	Provider    string        // current adapters: "openai", "anthropic", "groq"
	BaseURL     string        // provider-specific default if empty
	APIKey      string        // required
	Model       string        // required
	Temperature *float64      // optional
	MaxTokens   *int          // optional
	HTTPTimeout time.Duration // defaults to 60s; overridden by BudgetTimeout if set
	// BudgetTimeout is the remaining run budget at the time the provider is
	// created. When set, the HTTP client timeout is capped to this value so
	// that individual provider calls cannot outlive the remaining time budget.
	BudgetTimeout time.Duration
	// PromptCacheKey is an optional cache key for OpenAI's prompt caching feature.
	// When non-empty, it is sent as prompt_cache_key in the chat completions request.
	// This enables cache hits on repeated planner calls with the same prefix inputs.
	// Safe for OpenAI providers only; other providers ignore this field.
	PromptCacheKey *string
}

// NewProvider constructs a Provider from the given config, routed by Provider name.
// The returned provider is wrapped with retry middleware for transient failures.
func NewProvider(cfg ProviderConfig) (Provider, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("llm api key is required")
	}
	if cfg.Model == "" {
		return nil, fmt.Errorf("llm model is required")
	}

	timeout := cfg.HTTPTimeout
	if timeout <= 0 {
		timeout = defaultHTTPTimeout
	}
	// When a remaining run budget is provided, cap the HTTP timeout to it so that
	// individual provider calls cannot extend a run past the declared budget.
	if cfg.BudgetTimeout > 0 && cfg.BudgetTimeout < timeout {
		timeout = cfg.BudgetTimeout
	}

	var base Provider
	var err error

	switch normalizeProviderName(cfg.Provider) {
	case "openai", "":
		base, err = newOpenAIProvider(cfg, timeout)
	case "anthropic":
		base, err = newAnthropicProvider(cfg, timeout)
	case "groq":
		base, err = newGroqProvider(cfg, timeout)
	default:
		return nil, fmt.Errorf("unsupported llm provider %q (supported: openai, anthropic, groq)", cfg.Provider)
	}
	if err != nil {
		return nil, err
	}

	return &retryProvider{base: base, maxRetries: 3}, nil
}

func normalizeProviderName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func newUsageMetadata(provider string, inputTokens, outputTokens, totalTokens int, raw map[string]any, cacheRead, cacheWrite, uncached int) *UsageMetadata {
	if totalTokens == 0 && (inputTokens > 0 || outputTokens > 0) {
		totalTokens = inputTokens + outputTokens
	}
	if uncached == 0 && inputTokens > 0 {
		uncached = inputTokens - cacheRead - cacheWrite
		if uncached < 0 {
			uncached = 0
		}
	}
	if inputTokens == 0 && outputTokens == 0 && totalTokens == 0 && cacheRead == 0 && cacheWrite == 0 && uncached == 0 && len(raw) == 0 {
		return nil
	}

	um := &UsageMetadata{
		Provider:     normalizeProviderName(provider),
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		TotalTokens:  totalTokens,
		CacheRead:    cacheRead,
		CacheWrite:   cacheWrite,
		Uncached:     uncached,
		Raw:          raw,
	}
	return um
}

// estimateCost computes the estimated USD cost for a usage record given a
// provider name and model identifier. It performs a lookup in knownModelPricing
// using the "provider/model" key (case-insensitive). Returns nil when the
// provider/model combination is not in the pricing table — callers must check
// for nil before using the value. This ensures safe degradation: missing pricing
// data surfaces usage without inventing a dollar value.
//
// Cache-aware pricing: when uncached tokens are reported, those are used for the
// input cost calculation. Cache-read tokens are free on most providers and are
// excluded from the input cost when uncached tokens are explicitly reported.
func estimateCost(um *UsageMetadata, provider, model string) *float64 {
	if um == nil {
		return nil
	}
	key := strings.ToLower(strings.TrimSpace(provider)) + "/" + strings.ToLower(strings.TrimSpace(model))
	pricing, ok := knownModelPricing[key]
	if !ok {
		return nil
	}

	// Use uncached tokens for input cost when available, otherwise fall back to
	// total input tokens. This is the conservative estimate for cache-aware models.
	inputTokens := um.InputTokens
	if um.Uncached > 0 && um.Uncached <= um.InputTokens {
		inputTokens = um.Uncached
	}

	inputCost := float64(inputTokens) * pricing.InputPerM / 1_000_000
	outputCost := float64(um.OutputTokens) * pricing.OutputPerM / 1_000_000
	total := inputCost + outputCost
	return &total
}

// DerivePlannerCacheKey computes a stable cache key from the planner's prefix
// inputs: provider name, goal string, and mode string. The result is a short
// hex-encoded SHA256 prefix that uniquely identifies this combination.
//
// This function is safe to call for any provider. Callers should pass the
// result to NewProvider via ProviderConfig.PromptCacheKey only when using
// OpenAI (the field is ignored for other providers). The key is opaque to the
// provider; OpenAI uses it to match the request against cached prompt prefixes.
func DerivePlannerCacheKey(provider, goal, mode string) string {
	h := sha256.New()
	// Include provider so that different provider overlays produce different keys
	// even when goal and mode are identical.
	_, _ = h.Write([]byte(strings.ToLower(provider)))
	_, _ = h.Write([]byte("|"))
	_, _ = h.Write([]byte(goal))
	_, _ = h.Write([]byte("|"))
	_, _ = h.Write([]byte(mode))
	return hex.EncodeToString(h.Sum(nil))[:32]
}
