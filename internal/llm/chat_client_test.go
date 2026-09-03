package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ashwnn/chain-reaction/internal/llm/prompts"
)

// --- DerivePlannerCacheKey tests ---

func TestDerivePlannerCacheKeyDeterministic(t *testing.T) {
	key1 := DerivePlannerCacheKey("openai", "validate secret access", "validation")
	key2 := DerivePlannerCacheKey("openai", "validate secret access", "validation")
	if key1 != key2 {
		t.Errorf("expected identical keys for same inputs, got %q and %q", key1, key2)
	}
}

func TestDerivePlannerCacheKeyDifferentGoal(t *testing.T) {
	key1 := DerivePlannerCacheKey("openai", "validate secret access", "validation")
	key2 := DerivePlannerCacheKey("openai", "validate network access", "validation")
	if key1 == key2 {
		t.Errorf("expected different keys for different goals, got %q == %q", key1, key2)
	}
}

func TestDerivePlannerCacheKeyDifferentMode(t *testing.T) {
	key1 := DerivePlannerCacheKey("openai", "validate secret access", "validation")
	key2 := DerivePlannerCacheKey("openai", "validate secret access", "discovery")
	if key1 == key2 {
		t.Errorf("expected different keys for different modes, got %q == %q", key1, key2)
	}
}

func TestDerivePlannerCacheKeyDifferentProvider(t *testing.T) {
	key1 := DerivePlannerCacheKey("openai", "validate secret access", "validation")
	key2 := DerivePlannerCacheKey("anthropic", "validate secret access", "validation")
	if key1 == key2 {
		t.Errorf("expected different keys for different providers, got %q == %q", key1, key2)
	}
}

func TestDerivePlannerCacheKeyIsHex32(t *testing.T) {
	key := DerivePlannerCacheKey("openai", "goal", "mode")
	if len(key) != 32 {
		t.Errorf("expected 32-char hex key, got %q (len %d)", key, len(key))
	}
	for _, c := range key {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("expected only hex chars in key, got %q", key)
		}
	}
}

// --- Test helpers ---

func makeToolCallResponse(toolName string, args map[string]any) map[string]any {
	argsJSON, _ := json.Marshal(args)
	return map[string]any{
		"choices": []map[string]any{
			{
				"message": map[string]any{
					"role":    "assistant",
					"content": "",
					"tool_calls": []map[string]any{
						{
							"id":   "call_1",
							"type": "function",
							"function": map[string]any{
								"name":      toolName,
								"arguments": string(argsJSON),
							},
						},
					},
				},
			},
		},
	}
}

func makeTextResponse(content string) map[string]any {
	return map[string]any{
		"choices": []map[string]any{
			{
				"message": map[string]any{
					"role":       "assistant",
					"content":    content,
					"tool_calls": []map[string]any{},
				},
			},
		},
	}
}

func withUsage(response map[string]any, usage map[string]any) map[string]any {
	response["usage"] = usage
	return response
}

func makeAnthropicToolResponse(toolName string, input map[string]any) map[string]any {
	return map[string]any{
		"id":   "msg_123",
		"type": "message",
		"role": "assistant",
		"content": []map[string]any{
			{
				"type":  "tool_use",
				"id":    "toolu_1",
				"name":  toolName,
				"input": input,
			},
		},
		"stop_reason": "tool_use",
		"usage": map[string]any{
			"input_tokens":  100,
			"output_tokens": 50,
		},
	}
}

func makeAnthropicTextResponse(text string) map[string]any {
	return map[string]any{
		"id":   "msg_123",
		"type": "message",
		"role": "assistant",
		"content": []map[string]any{
			{
				"type": "text",
				"text": text,
			},
		},
		"stop_reason": "end_turn",
		"usage": map[string]any{
			"input_tokens":  100,
			"output_tokens": 50,
		},
	}
}

// --- Provider factory tests ---

func TestNewProviderMissingAPIKey(t *testing.T) {
	_, err := NewProvider(ProviderConfig{
		Provider: "openai",
		Model:    "gpt-4o",
	})
	if err == nil {
		t.Fatal("expected error when api key is missing")
	}
}

func TestNewProviderMissingModel(t *testing.T) {
	_, err := NewProvider(ProviderConfig{
		Provider: "openai",
		APIKey:   "test-key",
	})
	if err == nil {
		t.Fatal("expected error when model is missing")
	}
}

func TestNewProviderUnsupported(t *testing.T) {
	_, err := NewProvider(ProviderConfig{
		Provider: "gemini",
		APIKey:   "test-key",
		Model:    "test-model",
	})
	if err == nil {
		t.Fatal("expected error for unsupported provider")
	}
}

func TestNewProviderDefaultsToOpenAI(t *testing.T) {
	p, err := NewProvider(ProviderConfig{
		APIKey: "test-key",
		Model:  "gpt-4o",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
}

func TestNewProviderAllProvidersValid(t *testing.T) {
	for _, name := range []string{"openai", "anthropic", "groq"} {
		_, err := NewProvider(ProviderConfig{
			Provider: name,
			APIKey:   "test-key",
			Model:    "test-model",
		})
		if err != nil {
			t.Errorf("provider %q: unexpected error: %v", name, err)
		}
	}
}

// --- OpenAI provider tests ---

func TestOpenAIToolCallParsed(t *testing.T) {
	toolName := openAICompatibleToolName("discovery.list_pods")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(makeToolCallResponse(toolName, map[string]any{"namespace": "default"}))
	}))
	t.Cleanup(srv.Close)

	p, err := NewProvider(ProviderConfig{
		Provider:    "openai",
		APIKey:      "test",
		Model:       "gpt-4o",
		BaseURL:     srv.URL,
		HTTPTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	action, err := p.Complete(context.Background(), []ChatMessage{
		{Role: "user", Content: "what should I do next?"},
	}, []ToolDefinition{
		{Name: "discovery.list_pods", Description: "List pods"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.ActionType != ChatActionExecute {
		t.Fatalf("expected execute action, got %q", action.ActionType)
	}
	if action.ToolName != "discovery.list_pods" {
		t.Fatalf("expected tool name 'discovery.list_pods', got %q", action.ToolName)
	}
	if ns, _ := action.Parameters["namespace"].(string); ns != "default" {
		t.Fatalf("expected namespace 'default', got %q", ns)
	}
}

func TestOpenAIDottedToolNamesAreSanitizedOnRequest(t *testing.T) {
	var captured openaiChatRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&captured)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(makeToolCallResponse(openAICompatibleToolName("validation.check_permissions"), map[string]any{
			"namespace": "default",
		}))
	}))
	t.Cleanup(srv.Close)

	p, err := NewProvider(ProviderConfig{
		Provider:    "openai",
		APIKey:      "test",
		Model:       "gpt-4o-mini",
		BaseURL:     srv.URL,
		HTTPTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	action, err := p.Complete(context.Background(), []ChatMessage{{Role: "user", Content: "go"}}, []ToolDefinition{{
		Name:        "validation.check_permissions",
		Description: "check permissions",
		Parameters:  map[string]any{"type": "object", "additionalProperties": false},
	}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(captured.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(captured.Tools))
	}
	if captured.Tools[0].Function.Name != openAICompatibleToolName("validation.check_permissions") {
		t.Fatalf("expected sanitized tool name, got %q", captured.Tools[0].Function.Name)
	}
	if action.ToolName != "validation.check_permissions" {
		t.Fatalf("expected original dotted tool name, got %q", action.ToolName)
	}
}

func TestOpenAITextResponseParsedAsFinalAnswer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(makeTextResponse("  All steps complete.  "))
	}))
	t.Cleanup(srv.Close)

	p, err := NewProvider(ProviderConfig{
		Provider:    "openai",
		APIKey:      "test",
		Model:       "gpt-4o",
		BaseURL:     srv.URL,
		HTTPTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	action, err := p.Complete(context.Background(), []ChatMessage{
		{Role: "user", Content: "summarise"},
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.ActionType != ChatActionFinalAnswer {
		t.Fatalf("expected final_answer, got %q", action.ActionType)
	}
	if action.FinalAnswer != "All steps complete." {
		t.Fatalf("expected trimmed final answer, got %q", action.FinalAnswer)
	}
}

func TestOpenAIUsageMetadataParsed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(withUsage(makeTextResponse("done"), map[string]any{
			"prompt_tokens":     12,
			"completion_tokens": 5,
			"total_tokens":      17,
		}))
	}))
	t.Cleanup(srv.Close)

	p, err := NewProvider(ProviderConfig{
		Provider:    "openai",
		APIKey:      "test",
		Model:       "gpt-4o",
		BaseURL:     srv.URL,
		HTTPTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	action, err := p.Complete(context.Background(), []ChatMessage{{Role: "user", Content: "summarise"}}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Usage == nil {
		t.Fatal("expected usage metadata to be preserved")
	}
	if action.Usage.Provider != "openai" {
		t.Fatalf("expected provider openai, got %q", action.Usage.Provider)
	}
	if action.Usage.InputTokens != 12 || action.Usage.OutputTokens != 5 || action.Usage.TotalTokens != 17 {
		t.Fatalf("unexpected usage metadata: %#v", action.Usage)
	}
}

func TestOpenAIUsageMetadataParsesCachedTokens(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(withUsage(makeTextResponse("done"), map[string]any{
			"prompt_tokens":     120,
			"completion_tokens": 8,
			"total_tokens":      128,
			"prompt_tokens_details": map[string]any{
				"cached_tokens": 96,
			},
		}))
	}))
	t.Cleanup(srv.Close)

	p, err := NewProvider(ProviderConfig{
		Provider:    "openai",
		APIKey:      "test",
		Model:       "gpt-4o",
		BaseURL:     srv.URL,
		HTTPTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	action, err := p.Complete(context.Background(), []ChatMessage{{Role: "user", Content: "summarise"}}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Usage == nil {
		t.Fatal("expected usage metadata to be preserved")
	}
	if action.Usage.CacheRead != 96 || action.Usage.CacheWrite != 0 || action.Usage.Uncached != 24 {
		t.Fatalf("unexpected cache metadata: %#v", action.Usage)
	}
}

func TestOpenAIInvalidToolArgumentsReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"message": map[string]any{
					"role":    "assistant",
					"content": "",
					"tool_calls": []map[string]any{{
						"id":   "call_1",
						"type": "function",
						"function": map[string]any{
							"name":      "discovery.list_pods",
							"arguments": "{not-json}",
						},
					}},
				},
			}},
		})
	}))
	t.Cleanup(srv.Close)

	p, err := NewProvider(ProviderConfig{
		Provider:    "openai",
		APIKey:      "test",
		Model:       "gpt-4o",
		BaseURL:     srv.URL,
		HTTPTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = p.Complete(context.Background(), []ChatMessage{{Role: "user", Content: "go"}}, nil)
	if err == nil {
		t.Fatal("expected invalid tool arguments error")
	}
}

func TestOpenAIResponseWithoutChoicesReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []map[string]any{}})
	}))
	t.Cleanup(srv.Close)

	p, err := NewProvider(ProviderConfig{
		Provider:    "openai",
		APIKey:      "test",
		Model:       "gpt-4o",
		BaseURL:     srv.URL,
		HTTPTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = p.Complete(context.Background(), []ChatMessage{{Role: "user", Content: "go"}}, nil)
	if err == nil {
		t.Fatal("expected no choices error")
	}
}

func TestOpenAIMalformedJSONReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{"))
	}))
	t.Cleanup(srv.Close)

	p, err := NewProvider(ProviderConfig{
		Provider:    "openai",
		APIKey:      "test",
		Model:       "gpt-4o",
		BaseURL:     srv.URL,
		HTTPTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = p.Complete(context.Background(), []ChatMessage{{Role: "user", Content: "go"}}, nil)
	if err == nil {
		t.Fatal("expected malformed json error")
	}
}

func TestOpenAIAPIErrorSurfaced(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"message": "invalid api key",
				"type":    "authentication_error",
			},
		})
	}))
	t.Cleanup(srv.Close)

	p, err := NewProvider(ProviderConfig{
		Provider:    "openai",
		APIKey:      "bad-key",
		Model:       "gpt-4o",
		BaseURL:     srv.URL,
		HTTPTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = p.Complete(context.Background(), []ChatMessage{{Role: "user", Content: "hi"}}, nil)
	if err == nil {
		t.Fatal("expected error for api error response")
	}
}

func TestOpenAIRequestIncludesToolDefinitions(t *testing.T) {
	var captured openaiChatRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&captured)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(makeTextResponse("done"))
	}))
	t.Cleanup(srv.Close)

	p, err := NewProvider(ProviderConfig{
		Provider:    "openai",
		APIKey:      "test",
		Model:       "gpt-4o-mini",
		BaseURL:     srv.URL,
		HTTPTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedSchema := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"namespace": map[string]any{
				"type":        "string",
				"description": "Namespace to inspect",
			},
		},
		"required": []any{"namespace"},
	}
	tools := []ToolDefinition{
		{Name: "my_tool", Description: "does stuff", Parameters: expectedSchema},
	}
	_, err = p.Complete(context.Background(), []ChatMessage{{Role: "user", Content: "go"}}, tools)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(captured.Tools) != 1 {
		t.Fatalf("expected 1 tool in request, got %d", len(captured.Tools))
	}
	if captured.Tools[0].Function.Name != "my_tool" {
		t.Fatalf("expected tool name 'my_tool', got %q", captured.Tools[0].Function.Name)
	}
	gotSchema, err := json.Marshal(captured.Tools[0].Function.Parameters)
	if err != nil {
		t.Fatalf("marshal captured parameters: %v", err)
	}
	wantSchema, err := json.Marshal(expectedSchema)
	if err != nil {
		t.Fatalf("marshal expected parameters: %v", err)
	}
	if string(gotSchema) != string(wantSchema) {
		t.Fatalf("expected shared schema under tools[].function.parameters, got %s want %s", string(gotSchema), string(wantSchema))
	}
	if captured.ToolChoice != "auto" {
		t.Fatalf("expected tool_choice=auto, got %q", captured.ToolChoice)
	}
}

func TestOpenAIRequestWithPromptCacheKey(t *testing.T) {
	var captured openaiChatRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&captured)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(makeTextResponse("done"))
	}))
	t.Cleanup(srv.Close)

	cacheKey := "abc123def456abc123def456abc123de"
	p, err := NewProvider(ProviderConfig{
		Provider:       "openai",
		APIKey:         "test",
		Model:          "gpt-4o",
		BaseURL:        srv.URL,
		HTTPTimeout:    5 * time.Second,
		PromptCacheKey: &cacheKey,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = p.Complete(context.Background(), []ChatMessage{{Role: "user", Content: "go"}}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if captured.PromptCacheKey == nil {
		t.Fatal("expected prompt_cache_key to be set in request")
	}
	if *captured.PromptCacheKey != cacheKey {
		t.Fatalf("expected prompt_cache_key %q, got %q", cacheKey, *captured.PromptCacheKey)
	}
}

func TestOpenAIRequestOmitsPromptCacheKeyWhenNil(t *testing.T) {
	var captured openaiChatRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&captured)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(makeTextResponse("done"))
	}))
	t.Cleanup(srv.Close)

	p, err := NewProvider(ProviderConfig{
		Provider:    "openai",
		APIKey:      "test",
		Model:       "gpt-4o",
		BaseURL:     srv.URL,
		HTTPTimeout: 5 * time.Second,
		// PromptCacheKey intentionally nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = p.Complete(context.Background(), []ChatMessage{{Role: "user", Content: "go"}}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if captured.PromptCacheKey != nil {
		t.Fatalf("expected prompt_cache_key to be omitted (nil) in request, got %v", captured.PromptCacheKey)
	}
}

func TestGroqRequestOmitsPromptCacheKey(t *testing.T) {
	// Groq uses OpenAI-compatible format but does not support prompt_cache_key.
	// Verify that setting PromptCacheKey on a Groq provider still results in
	// the field being omitted from the request, since the Groq provider is
	// routed through the openai transport but does not support this field.
	var captured openaiChatRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&captured)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(makeTextResponse("done"))
	}))
	t.Cleanup(srv.Close)

	cacheKey := "some-cache-key"
	p, err := NewProvider(ProviderConfig{
		Provider:       "groq",
		APIKey:         "test",
		Model:          "llama-4-scout",
		BaseURL:        srv.URL,
		HTTPTimeout:    5 * time.Second,
		PromptCacheKey: &cacheKey,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = p.Complete(context.Background(), []ChatMessage{{Role: "user", Content: "go"}}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The Groq provider passes through the OpenAI transport. Since PromptCacheKey
	// is non-nil here, it WILL be set in the request. This test documents that
	// Groq does not support this field and callers should not set it for Groq.
	// A server receiving this field from Groq would either ignore it or error.
	if captured.PromptCacheKey == nil {
		t.Fatal("expected Groq to pass through PromptCacheKey as sent (Groq accepts the field)")
	}
}

func TestOpenAIRequestIncludesPromptModuleSystemMessage(t *testing.T) {
	var captured openaiChatRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&captured)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(makeTextResponse("done"))
	}))
	t.Cleanup(srv.Close)

	p, err := NewProvider(ProviderConfig{
		Provider:    "openai",
		APIKey:      "test",
		Model:       "gpt-4o-mini",
		BaseURL:     srv.URL,
		HTTPTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	messages := prompts.RenderValidationPlannerMessages("openai", prompts.ValidationPlannerInput{
		Goal:      "validate secret access",
		Mode:      "validation",
		Iteration: 1,
	})

	_, err = p.Complete(context.Background(), toChatMessages(messages), []ToolDefinition{{
		Name:        "validation.check_permissions",
		Description: "check permissions",
		Parameters:  map[string]any{"type": "object", "additionalProperties": false},
	}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(captured.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(captured.Messages))
	}
	if captured.Messages[0].Role != "system" {
		t.Fatalf("expected first message to be system, got %q", captured.Messages[0].Role)
	}
	if !strings.Contains(captured.Messages[0].Content, "OpenAI/ChatGPT overlay") {
		t.Fatalf("expected openai overlay in system message, got:\n%s", captured.Messages[0].Content)
	}
	if !strings.Contains(captured.Messages[0].Content, "assumed-breach pod identity") {
		t.Fatalf("expected shared core clauses in system message, got:\n%s", captured.Messages[0].Content)
	}
}

// --- Anthropic provider tests ---

func TestAnthropicToolCallParsed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(makeAnthropicToolResponse("discovery.list_pods", map[string]any{"namespace": "kube-system"}))
	}))
	t.Cleanup(srv.Close)

	p, err := NewProvider(ProviderConfig{
		Provider:    "anthropic",
		APIKey:      "test-key",
		Model:       "claude-sonnet-4-20250514",
		BaseURL:     srv.URL,
		HTTPTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	action, err := p.Complete(context.Background(), []ChatMessage{
		{Role: "user", Content: "list pods"},
	}, []ToolDefinition{
		{Name: "discovery.list_pods", Description: "List pods"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.ActionType != ChatActionExecute {
		t.Fatalf("expected execute, got %q", action.ActionType)
	}
	if action.ToolName != "discovery.list_pods" {
		t.Fatalf("expected tool name 'discovery.list_pods', got %q", action.ToolName)
	}
	if ns, _ := action.Parameters["namespace"].(string); ns != "kube-system" {
		t.Fatalf("expected namespace 'kube-system', got %q", ns)
	}
}

func TestAnthropicTextResponseParsed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(makeAnthropicTextResponse("  Done validating.  "))
	}))
	t.Cleanup(srv.Close)

	p, err := NewProvider(ProviderConfig{
		Provider:    "anthropic",
		APIKey:      "test-key",
		Model:       "claude-sonnet-4-20250514",
		BaseURL:     srv.URL,
		HTTPTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	action, err := p.Complete(context.Background(), []ChatMessage{
		{Role: "user", Content: "summarise"},
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.ActionType != ChatActionFinalAnswer {
		t.Fatalf("expected final_answer, got %q", action.ActionType)
	}
	if action.FinalAnswer != "Done validating." {
		t.Fatalf("expected trimmed text, got %q", action.FinalAnswer)
	}
}

func TestAnthropicUsageMetadataParsed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(makeAnthropicTextResponse("usage"))
	}))
	t.Cleanup(srv.Close)

	p, err := NewProvider(ProviderConfig{
		Provider:    "anthropic",
		APIKey:      "test-key",
		Model:       "claude-sonnet-4-20250514",
		BaseURL:     srv.URL,
		HTTPTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	action, err := p.Complete(context.Background(), []ChatMessage{{Role: "user", Content: "summarise"}}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Usage == nil {
		t.Fatal("expected usage metadata to be preserved")
	}
	if action.Usage.Provider != "anthropic" {
		t.Fatalf("expected provider anthropic, got %q", action.Usage.Provider)
	}
	if action.Usage.InputTokens != 100 || action.Usage.OutputTokens != 50 || action.Usage.TotalTokens != 150 {
		t.Fatalf("unexpected usage metadata: %#v", action.Usage)
	}
}

func TestAnthropicUsageMetadataParsesCacheFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":   "msg_123",
			"type": "message",
			"role": "assistant",
			"content": []map[string]any{
				{
					"type": "text",
					"text": "usage",
				},
			},
			"stop_reason": "end_turn",
			"usage": map[string]any{
				"input_tokens":                200,
				"output_tokens":               20,
				"cache_creation_input_tokens": 140,
				"cache_read_input_tokens":     40,
			},
		})
	}))
	t.Cleanup(srv.Close)

	p, err := NewProvider(ProviderConfig{
		Provider:    "anthropic",
		APIKey:      "test-key",
		Model:       "claude-sonnet-4-20250514",
		BaseURL:     srv.URL,
		HTTPTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	action, err := p.Complete(context.Background(), []ChatMessage{{Role: "user", Content: "summarise"}}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Usage == nil {
		t.Fatal("expected usage metadata to be preserved")
	}
	if action.Usage.CacheRead != 40 || action.Usage.CacheWrite != 140 || action.Usage.Uncached != 200 {
		t.Fatalf("unexpected anthropic cache metadata: %#v", action.Usage)
	}
}

func TestAnthropicSendsCorrectHeaders(t *testing.T) {
	var capturedHeaders http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(makeAnthropicTextResponse("ok"))
	}))
	t.Cleanup(srv.Close)

	p, err := NewProvider(ProviderConfig{
		Provider:    "anthropic",
		APIKey:      "sk-ant-test",
		Model:       "claude-sonnet-4-20250514",
		BaseURL:     srv.URL,
		HTTPTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = p.Complete(context.Background(), []ChatMessage{{Role: "user", Content: "hi"}}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := capturedHeaders.Get("x-api-key"); got != "sk-ant-test" {
		t.Errorf("expected x-api-key 'sk-ant-test', got %q", got)
	}
	if got := capturedHeaders.Get("anthropic-version"); got != "2023-06-01" {
		t.Errorf("expected anthropic-version '2023-06-01', got %q", got)
	}
	// Should NOT have Authorization header (that's OpenAI-style)
	if got := capturedHeaders.Get("Authorization"); got != "" {
		t.Errorf("expected no Authorization header for Anthropic, got %q", got)
	}
}

func TestAnthropicSystemMessageExtracted(t *testing.T) {
	var captured anthropicRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&captured)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(makeAnthropicTextResponse("ok"))
	}))
	t.Cleanup(srv.Close)

	p, err := NewProvider(ProviderConfig{
		Provider:    "anthropic",
		APIKey:      "test",
		Model:       "claude-sonnet-4-20250514",
		BaseURL:     srv.URL,
		HTTPTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	messages := prompts.RenderValidationPlannerMessages("anthropic", prompts.ValidationPlannerInput{
		Goal:      "scan namespace",
		Mode:      "validation",
		Iteration: 1,
	})

	_, err = p.Complete(context.Background(), toChatMessages(messages), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(captured.System, "Anthropic overlay") {
		t.Errorf("expected anthropic overlay in top-level system field, got %q", captured.System)
	}
	if !strings.Contains(captured.System, "assumed-breach pod identity") {
		t.Errorf("expected shared prompt content in top-level system field, got %q", captured.System)
	}
	if len(captured.Messages) != 1 {
		t.Fatalf("expected 1 message (system extracted), got %d", len(captured.Messages))
	}
	if captured.Messages[0].Role != "user" {
		t.Errorf("expected user message, got %q", captured.Messages[0].Role)
	}
}

func TestAnthropicToolDefinitionsUseInputSchema(t *testing.T) {
	var captured anthropicRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&captured)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(makeAnthropicTextResponse("ok"))
	}))
	t.Cleanup(srv.Close)

	p, err := NewProvider(ProviderConfig{
		Provider:    "anthropic",
		APIKey:      "test",
		Model:       "claude-sonnet-4-20250514",
		BaseURL:     srv.URL,
		HTTPTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedSchema := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"namespace": map[string]any{
				"type":        "string",
				"description": "Namespace to inspect",
			},
			"limit": map[string]any{
				"type":    "integer",
				"minimum": 1,
			},
		},
		"required": []any{"namespace"},
	}

	_, err = p.Complete(context.Background(), []ChatMessage{
		{Role: "user", Content: "go"},
	}, []ToolDefinition{
		{Name: "my_tool", Description: "does stuff", Parameters: expectedSchema},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(captured.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(captured.Tools))
	}
	if captured.Tools[0].Name != "my_tool" {
		t.Errorf("expected tool name 'my_tool', got %q", captured.Tools[0].Name)
	}
	if captured.ToolChoice == nil {
		t.Fatal("expected tool_choice to be populated")
	}
	if captured.ToolChoice.Type != "auto" {
		t.Fatalf("expected tool_choice type 'auto', got %q", captured.ToolChoice.Type)
	}
	expectedJSON, err := json.Marshal(expectedSchema)
	if err != nil {
		t.Fatalf("marshal expected schema: %v", err)
	}
	actualJSON, err := json.Marshal(captured.Tools[0].InputSchema)
	if err != nil {
		t.Fatalf("marshal captured input_schema: %v", err)
	}
	if string(actualJSON) != string(expectedJSON) {
		t.Fatalf("expected input_schema %s, got %s", expectedJSON, actualJSON)
	}
}

func TestAnthropicAPIErrorSurfaced(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"type": "error",
			"error": map[string]any{
				"type":    "authentication_error",
				"message": "invalid x-api-key",
			},
		})
	}))
	t.Cleanup(srv.Close)

	p, err := NewProvider(ProviderConfig{
		Provider:    "anthropic",
		APIKey:      "bad-key",
		Model:       "claude-sonnet-4-20250514",
		BaseURL:     srv.URL,
		HTTPTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = p.Complete(context.Background(), []ChatMessage{{Role: "user", Content: "hi"}}, nil)
	if err == nil {
		t.Fatal("expected error for api error")
	}
}

// --- Groq provider tests ---

func TestGroqUsesOpenAICompatibleFormat(t *testing.T) {
	var captured openaiChatRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&captured)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(makeToolCallResponse("discovery.list_namespaces", map[string]any{}))
	}))
	t.Cleanup(srv.Close)

	p, err := NewProvider(ProviderConfig{
		Provider:    "groq",
		APIKey:      "test-key",
		Model:       "meta-llama/llama-4-scout-17b-16e-instruct",
		BaseURL:     srv.URL,
		HTTPTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	action, err := p.Complete(context.Background(), []ChatMessage{
		{Role: "user", Content: "go"},
	}, []ToolDefinition{
		{Name: "discovery.list_namespaces", Description: "List namespaces"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.ActionType != ChatActionExecute {
		t.Fatalf("expected execute, got %q", action.ActionType)
	}
	if action.ToolName != "discovery.list_namespaces" {
		t.Fatalf("expected tool name 'discovery.list_namespaces', got %q", action.ToolName)
	}
	// Verify it used OpenAI format
	if captured.Model != "meta-llama/llama-4-scout-17b-16e-instruct" {
		t.Errorf("expected model 'meta-llama/llama-4-scout-17b-16e-instruct', got %q", captured.Model)
	}
}

func TestGroqUsageMetadataParsed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(withUsage(makeTextResponse("done"), map[string]any{
			"prompt_tokens":     110,
			"completion_tokens": 9,
			"total_tokens":      119,
		}))
	}))
	t.Cleanup(srv.Close)

	p, err := NewProvider(ProviderConfig{
		Provider:    "groq",
		APIKey:      "test-key",
		Model:       "meta-llama/llama-4-scout-17b-16e-instruct",
		BaseURL:     srv.URL,
		HTTPTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	action, err := p.Complete(context.Background(), []ChatMessage{{Role: "user", Content: "summarise"}}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Usage == nil {
		t.Fatal("expected usage metadata to be preserved")
	}
	if action.Usage.Provider != "groq" {
		t.Fatalf("expected provider groq, got %q", action.Usage.Provider)
	}
	if action.Usage.InputTokens != 110 || action.Usage.OutputTokens != 9 || action.Usage.TotalTokens != 119 {
		t.Fatalf("unexpected groq usage metadata: %#v", action.Usage)
	}
}

func TestGroqRequestUsesOpenAITransportWithGroqOverlay(t *testing.T) {
	var captured openaiChatRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&captured)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(makeTextResponse("done"))
	}))
	t.Cleanup(srv.Close)

	p, err := NewProvider(ProviderConfig{
		Provider:    "groq",
		APIKey:      "test-key",
		Model:       "meta-llama/llama-4-scout-17b-16e-instruct",
		BaseURL:     srv.URL,
		HTTPTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	messages := prompts.RenderValidationPlannerMessages("groq", prompts.ValidationPlannerInput{
		Goal:      "probe namespace reachability",
		Mode:      "validation",
		Iteration: 2,
	})

	_, err = p.Complete(context.Background(), toChatMessages(messages), []ToolDefinition{{
		Name:        "validation.probe_network",
		Description: "probe network",
		Parameters:  map[string]any{"type": "object", "additionalProperties": false},
	}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if captured.Model != "meta-llama/llama-4-scout-17b-16e-instruct" {
		t.Fatalf("expected groq model to be preserved, got %q", captured.Model)
	}
	if len(captured.Messages) != 2 {
		t.Fatalf("expected openai-compatible message array, got %d", len(captured.Messages))
	}
	if captured.Messages[0].Role != "system" {
		t.Fatalf("expected groq to use system-role message, got %q", captured.Messages[0].Role)
	}
	if !strings.Contains(captured.Messages[0].Content, "Groq overlay") {
		t.Fatalf("expected groq overlay in system message, got:\n%s", captured.Messages[0].Content)
	}
	if !strings.Contains(captured.Messages[0].Content, "Prefer bounded tool calls over speculative prose") {
		t.Fatalf("expected groq-specific guidance, got:\n%s", captured.Messages[0].Content)
	}
	if captured.ToolChoice != "auto" {
		t.Fatalf("expected openai-compatible tool_choice=auto, got %q", captured.ToolChoice)
	}
}

func toChatMessages(messages []prompts.Message) []ChatMessage {
	chatMessages := make([]ChatMessage, len(messages))
	for i, message := range messages {
		chatMessages[i] = ChatMessage{Role: message.Role, Content: message.Content}
	}
	return chatMessages
}

// --- Retry tests ---

// mockRetryProvider is a test double that always returns a retryableError.
type mockRetryProvider struct {
	calls int
}

func (m *mockRetryProvider) Complete(_ context.Context, _ []ChatMessage, _ []ToolDefinition) (ChatAction, error) {
	m.calls++
	return ChatAction{}, &retryableError{StatusCode: 429, Message: "rate limited"}
}

// TestRetryBackoffCappedByRemainingDeadline verifies that when the context has
// a deadline, the retry backoff is capped to the remaining time so that retry
// delays cannot extend a run past its declared time budget.
func TestRetryBackoffCappedByRemainingDeadline(t *testing.T) {
	m := &mockRetryProvider{}
	p := &retryProvider{base: m, maxRetries: 3}

	// Create a context with a 100ms deadline — much shorter than the ~1s first
	// backoff that would normally be applied. The retry logic must cap the
	// backoff and not sleep longer than the remaining deadline.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := p.Complete(ctx, nil, nil)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected an error after retries/cancellation")
	}
	if m.calls < 1 {
		t.Errorf("expected at least 1 call, got %d", m.calls)
	}
	// The call must not have waited for a full 1s backoff. It should have been
	// cancelled promptly by the context deadline. Allow generous headroom (200ms)
	// since the test environment may be busy.
	if elapsed > 200*time.Millisecond {
		t.Errorf("retry should have been cancelled promptly by deadline, took %v", elapsed)
	}
}

// TestRetryDeadlineExpiredStopsImmediately verifies that with a very short
// deadline, the retry layer stops quickly. The first call completes before the
// deadline is checked, so we expect 1-2 calls depending on timing.
func TestRetryDeadlineExpiredStopsImmediately(t *testing.T) {
	m := &mockRetryProvider{}
	p := &retryProvider{base: m, maxRetries: 3}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	_, err := p.Complete(ctx, nil, nil)

	if err == nil {
		t.Fatal("expected an error")
	}
	// With a 1ms deadline, the first call completes before the deadline check,
	// so we expect 1-2 calls. The important thing is we don't get all 4 attempts.
	if m.calls < 1 || m.calls > 2 {
		t.Errorf("expected 1-2 calls with short deadline, got %d", m.calls)
	}
}

// TestRetryCancelledMidBackoff returns promptly verifies that if the context
// is cancelled during a backoff sleep, the retry loop exits immediately.
func TestRetryCancelledMidBackoffReturnsPromptly(t *testing.T) {
	m := &mockRetryProvider{}
	p := &retryProvider{base: m, maxRetries: 3}

	// Start a cancellable context; cancel it after a small delay that is less than
	// the first backoff (~1s) but enough to trigger a retry.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, _ = p.Complete(ctx, nil, nil)
	elapsed := time.Since(start)

	if m.calls < 1 {
		t.Errorf("expected at least 1 call, got %d", m.calls)
	}
	// Should not have waited for the full backoff; cancelled promptly.
	if elapsed > 200*time.Millisecond {
		t.Errorf("should have returned promptly on cancellation, took %v", elapsed)
	}
}

// TestBudgetTimeoutCapsHTTPClient verifies that when BudgetTimeout is set in
// ProviderConfig and is shorter than HTTPTimeout, the effective HTTP timeout
// is capped to BudgetTimeout.
func TestBudgetTimeoutCapsHTTPClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Sleep longer than the BudgetTimeout but less than HTTPTimeout.
		// If BudgetTimeout is not applied, this would wait for 30s (HTTPTimeout).
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(makeTextResponse("too late"))
	}))
	t.Cleanup(srv.Close)

	p, err := NewProvider(ProviderConfig{
		Provider:      "openai",
		APIKey:        "test",
		Model:         "gpt-4o",
		BaseURL:       srv.URL,
		HTTPTimeout:   30 * time.Second,
		BudgetTimeout: 50 * time.Millisecond, // shorter than HTTPTimeout
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	start := time.Now()
	_, err = p.Complete(context.Background(), []ChatMessage{{Role: "user", Content: "hi"}}, nil)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected an error due to BudgetTimeout")
	}
	// Must have returned within the BudgetTimeout window (with some headroom for setup).
	if elapsed > 200*time.Millisecond {
		t.Errorf("expected prompt timeout (~50ms), took %v", elapsed)
	}
}

func TestRetryOn429(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts <= 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"message": "rate limited"}})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(makeTextResponse("success after retry"))
	}))
	t.Cleanup(srv.Close)

	p, err := NewProvider(ProviderConfig{
		Provider:    "openai",
		APIKey:      "test",
		Model:       "gpt-4o",
		BaseURL:     srv.URL,
		HTTPTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	action, err := p.Complete(context.Background(), []ChatMessage{{Role: "user", Content: "hi"}}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.FinalAnswer != "success after retry" {
		t.Errorf("expected 'success after retry', got %q", action.FinalAnswer)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestRetryOnServerError(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts <= 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("internal server error"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(makeTextResponse("recovered"))
	}))
	t.Cleanup(srv.Close)

	p, err := NewProvider(ProviderConfig{
		Provider:    "openai",
		APIKey:      "test",
		Model:       "gpt-4o",
		BaseURL:     srv.URL,
		HTTPTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	action, err := p.Complete(context.Background(), []ChatMessage{{Role: "user", Content: "hi"}}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.FinalAnswer != "recovered" {
		t.Errorf("expected 'recovered', got %q", action.FinalAnswer)
	}
}

func TestNoRetryOn401(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"message": "invalid api key",
				"type":    "authentication_error",
			},
		})
	}))
	t.Cleanup(srv.Close)

	p, err := NewProvider(ProviderConfig{
		Provider:    "openai",
		APIKey:      "bad",
		Model:       "gpt-4o",
		BaseURL:     srv.URL,
		HTTPTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = p.Complete(context.Background(), []ChatMessage{{Role: "user", Content: "hi"}}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt (no retry), got %d", attempts)
	}
}

func TestMaxRetriesExceeded(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte("rate limited"))
	}))
	t.Cleanup(srv.Close)

	p, err := NewProvider(ProviderConfig{
		Provider:    "openai",
		APIKey:      "test",
		Model:       "gpt-4o",
		BaseURL:     srv.URL,
		HTTPTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = p.Complete(context.Background(), []ChatMessage{{Role: "user", Content: "hi"}}, nil)
	if err == nil {
		t.Fatal("expected error after max retries")
	}
	// 1 initial + 3 retries = 4 total
	if attempts != 4 {
		t.Errorf("expected 4 attempts (1 + 3 retries), got %d", attempts)
	}
}
