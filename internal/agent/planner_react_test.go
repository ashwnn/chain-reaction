package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ashwnn/chain-reaction/internal/llm"
	"github.com/ashwnn/chain-reaction/internal/tools"
	"github.com/ashwnn/chain-reaction/internal/tools/validation"
)

func TestReactValidationPlanner(t *testing.T) {
	requested := false
	var requestBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested = true
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		resp := map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"role":    "assistant",
						"content": "",
						"tool_calls": []map[string]any{
							{
								"id":   "call_123",
								"type": "function",
								"function": map[string]any{
									"name":      "validation.read_secret",
									"arguments": `{"name":"db-creds"}`,
								},
							},
						},
					},
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     14,
				"completion_tokens": 6,
				"total_tokens":      20,
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider, err := llm.NewProvider(llm.ProviderConfig{
		Provider:    "openai",
		APIKey:      "fake",
		Model:       "gpt-4",
		BaseURL:     server.URL,
		HTTPTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	registry := tools.NewRegistry()
	readSecretSchema := tools.Schema{
		Type: "object",
		Properties: map[string]tools.Schema{
			"name": {
				Type:        "string",
				Description: "Secret name",
			},
			"namespace": {
				Type:        "string",
				Description: "Namespace to query",
				Default:     "default",
			},
		},
		Required: []string{"name"},
	}
	if err := registry.Register(&fakeTool{name: "validation.read_secret", schema: &readSecretSchema}); err != nil {
		t.Fatalf("register fake tool: %v", err)
	}

	planner := newReactValidationPlanner(provider, registry, "openai")

	action, err := planner.NextAction(context.Background(), newState(executionModeValidation, "test goal", time.Now().UTC()), []string{"validation.read_secret"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !requested {
		t.Fatal("expected LLM server to be called")
	}

	if action.ActionType != actionTypeExecute {
		t.Errorf("expected actionTypeExecute, got %q", action.ActionType)
	}
	if action.ToolName != "validation.read_secret" {
		t.Errorf("expected tool name validation.read_secret, got %q", action.ToolName)
	}
	if action.Parameters["name"] != "db-creds" {
		t.Errorf("expected parameter name=db-creds, got %v", action.Parameters)
	}
	if action.Usage == nil {
		t.Fatal("expected planner usage metadata to be preserved")
	}
	if action.Usage.Provider != "openai" || action.Usage.TotalTokens != 20 {
		t.Fatalf("unexpected planner usage metadata: %#v", action.Usage)
	}

	toolsPayload, ok := requestBody["tools"].([]any)
	if !ok || len(toolsPayload) != 1 {
		t.Fatalf("expected single tool payload, got %#v", requestBody["tools"])
	}

	toolPayload, ok := toolsPayload[0].(map[string]any)
	if !ok {
		t.Fatalf("expected tool payload object, got %#v", toolsPayload[0])
	}
	functionPayload, ok := toolPayload["function"].(map[string]any)
	if !ok {
		t.Fatalf("expected function payload object, got %#v", toolPayload["function"])
	}
	parametersPayload, ok := functionPayload["parameters"].(map[string]any)
	if !ok {
		t.Fatalf("expected parameters payload object, got %#v", functionPayload["parameters"])
	}
	if parametersPayload["type"] != "object" {
		t.Fatalf("expected object schema, got %#v", parametersPayload)
	}
	if parametersPayload["additionalProperties"] != false {
		t.Fatalf("expected closed object schema, got %#v", parametersPayload)
	}
	requiredPayload, ok := parametersPayload["required"].([]any)
	if !ok || len(requiredPayload) != 1 || requiredPayload[0] != "name" {
		t.Fatalf("expected required=[name], got %#v", parametersPayload["required"])
	}
	propertiesPayload, ok := parametersPayload["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected properties payload object, got %#v", parametersPayload["properties"])
	}
	nameProperty, ok := propertiesPayload["name"].(map[string]any)
	if !ok || nameProperty["type"] != "string" {
		t.Fatalf("expected name property schema, got %#v", propertiesPayload["name"])
	}
}

func TestReactValidationPlanner_ThoughtPropagates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(new(map[string]any)); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		resp := map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"role":    "assistant",
						"content": "The secret name follows a known pattern; read it directly.",
						"tool_calls": []map[string]any{
							{
								"id":   "call_456",
								"type": "function",
								"function": map[string]any{
									"name":      "validation.read_secret",
									"arguments": `{"name":"db-creds"}`,
								},
							},
						},
					},
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     14,
				"completion_tokens": 6,
				"total_tokens":      20,
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider, err := llm.NewProvider(llm.ProviderConfig{
		Provider:    "openai",
		APIKey:      "fake",
		Model:       "gpt-4",
		BaseURL:     server.URL,
		HTTPTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	registry := tools.NewRegistry()
	readSecretSchema := tools.Schema{
		Type: "object",
		Properties: map[string]tools.Schema{
			"name": {
				Type:        "string",
				Description: "Secret name",
			},
		},
		Required: []string{"name"},
	}
	if err := registry.Register(&fakeTool{name: "validation.read_secret", schema: &readSecretSchema}); err != nil {
		t.Fatalf("register fake tool: %v", err)
	}

	planner := newReactValidationPlanner(provider, registry, "openai")

	action, err := planner.NextAction(context.Background(), newState(executionModeValidation, "test goal", time.Now().UTC()), []string{"validation.read_secret"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if action.Thought == "" {
		t.Fatal("expected planner action thought to be populated from LLM response content")
	}
	if action.Thought != "The secret name follows a known pattern; read it directly." {
		t.Errorf("unexpected thought value: %q", action.Thought)
	}
}

func TestReactValidationPlanner_FinalAnswer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"role":    "assistant",
						"content": "I have finished validating.",
					},
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     9,
				"completion_tokens": 3,
				"total_tokens":      12,
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider, _ := llm.NewProvider(llm.ProviderConfig{
		Provider:    "openai",
		APIKey:      "fake",
		Model:       "gpt-4",
		BaseURL:     server.URL,
		HTTPTimeout: time.Second,
	})

	registry := tools.NewRegistry()
	planner := newReactValidationPlanner(provider, registry, "openai")

	action, err := planner.NextAction(context.Background(), newState(executionModeValidation, "test goal", time.Now().UTC()), []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if action.ActionType != actionTypeFinalAnswer {
		t.Errorf("expected actionTypeFinalAnswer, got %q", action.ActionType)
	}
	if action.FinalAnswer != "I have finished validating." {
		t.Errorf("expected final answer text, got %q", action.FinalAnswer)
	}
	if action.Usage == nil || action.Usage.TotalTokens != 12 {
		t.Fatalf("expected final answer usage metadata, got %#v", action.Usage)
	}
}

func TestReactValidationPlannerUsesPromptModule(t *testing.T) {
	var requestBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"message": map[string]any{
					"role":    "assistant",
					"content": "Prompt inspected.",
				},
			}},
		})
	}))
	defer server.Close()

	provider, err := llm.NewProvider(llm.ProviderConfig{
		Provider:    "openai",
		APIKey:      "fake",
		Model:       "gpt-4",
		BaseURL:     server.URL,
		HTTPTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	planner := newReactValidationPlanner(provider, tools.NewRegistry(), "groq")
	state := newState(executionModeValidation, "test goal", time.Now().UTC())
	state.Iteration = 2

	if _, err := planner.NextAction(context.Background(), state, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	messages := mustRequestMessages(t, requestBody)
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	if got := messages[0]["role"]; got != "system" {
		t.Fatalf("expected system role, got %#v", got)
	}
	systemContent, _ := messages[0]["content"].(string)
	if !strings.Contains(systemContent, "assumed-breach pod identity") {
		t.Fatalf("expected prompt-module system content, got:\n%s", systemContent)
	}
	if !strings.Contains(systemContent, "Groq overlay") {
		t.Fatalf("expected groq overlay content, got:\n%s", systemContent)
	}
	userContent, _ := messages[1]["content"].(string)
	if !strings.Contains(userContent, "Goal: test goal") || !strings.Contains(userContent, "Mode: validation") || !strings.Contains(userContent, "Iteration: 2") {
		t.Fatalf("expected prompt-module user content, got:\n%s", userContent)
	}
}

func TestReactValidationPlannerPromptContract_NoFinalAnswerTool(t *testing.T) {
	var requestBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"message": map[string]any{
					"role":    "assistant",
					"content": "done",
				},
			}},
		})
	}))
	defer server.Close()

	provider, err := llm.NewProvider(llm.ProviderConfig{
		Provider:    "openai",
		APIKey:      "fake",
		Model:       "gpt-4",
		BaseURL:     server.URL,
		HTTPTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	planner := newReactValidationPlanner(provider, tools.NewRegistry(), "openai")
	if _, err := planner.NextAction(context.Background(), newState(executionModeValidation, "test goal", time.Now().UTC()), nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	systemContent, _ := mustRequestMessages(t, requestBody)[0]["content"].(string)
	if strings.Contains(systemContent, "final_answer tool") || strings.Contains(systemContent, "final_answer tool/action") {
		t.Fatalf("system prompt should not mention final_answer tool:\n%s", systemContent)
	}
	if !strings.Contains(systemContent, "conclude concisely from the evidence") {
		t.Fatalf("system prompt should instruct plain-text conclusions:\n%s", systemContent)
	}
}

func TestRenderValidationPlannerHistory_WithThought(t *testing.T) {
	state := newState(executionModeValidation, "test goal", time.Now().UTC())
	state.appendHistory(historyEntry{
		Iteration: 1,
		ToolName:  "validation.read_secret",
		Input:     map[string]any{"name": "db-creds"},
		Output:    map[string]any{"status": "ok"},
		Thought:   "The secret name follows a known pattern.",
	})
	state.appendHistory(historyEntry{
		Iteration: 2,
		ToolName:  "discovery.list_pods",
		Input:     map[string]any{"namespace": "default"},
		Output:    map[string]any{"pods": []any{"pod-a"}},
		// Thought is empty — should not appear in rendered history.
	})

	rendered := renderValidationPlannerHistory(state.History)
	if !strings.Contains(rendered, "[thought: The secret name follows a known pattern.]") {
		t.Fatalf("expected thought in rendered history, got:\n%s", rendered)
	}
	// Second entry has no thought; ensure "[thought:" suffix from first entry
	// does not bleed into second.
	if strings.Contains(rendered, "[thought: The secret name") &&
		strings.Contains(rendered, "discovery.list_pods") &&
		strings.Contains(rendered, "Iteration 2") &&
		strings.Contains(rendered, "[thought:") {
		// Count occurrences of "[thought:" — should be exactly 1.
		count := strings.Count(rendered, "[thought:")
		if count != 1 {
			t.Errorf("expected exactly 1 [thought:] tag, got %d in:\n%s", count, rendered)
		}
	}
}

func TestReactValidationPlannerPromptHistoryFormatting(t *testing.T) {
	var requestBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"message": map[string]any{
					"role":    "assistant",
					"content": "done",
				},
			}},
		})
	}))
	defer server.Close()

	provider, err := llm.NewProvider(llm.ProviderConfig{
		Provider:    "openai",
		APIKey:      "fake",
		Model:       "gpt-4",
		BaseURL:     server.URL,
		HTTPTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	planner := newReactValidationPlanner(provider, tools.NewRegistry(), "openai")
	state := newState(executionModeValidation, "confirm evidence flow", time.Now().UTC())
	state.Iteration = 4
	state.appendHistory(historyEntry{
		Iteration: 1,
		ToolName:  "validation.check_permissions",
		Input: map[string]any{
			"verb":     "list",
			"resource": "secrets",
		},
		Output:    map[string]any{"allowed": false},
		Timestamp: time.Now().UTC(),
	})

	if _, err := planner.NextAction(context.Background(), state, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	userContent, _ := mustRequestMessages(t, requestBody)[1]["content"].(string)
	if !strings.Contains(userContent, "Goal: confirm evidence flow") {
		t.Fatalf("expected goal section, got:\n%s", userContent)
	}
	if !strings.Contains(userContent, "Mode: validation") {
		t.Fatalf("expected mode section, got:\n%s", userContent)
	}
	if !strings.Contains(userContent, "Iteration: 4") {
		t.Fatalf("expected iteration section, got:\n%s", userContent)
	}
	if !strings.Contains(userContent, "History of executed tools and their results:") {
		t.Fatalf("expected history header, got:\n%s", userContent)
	}
	if !strings.Contains(userContent, "- Iteration 1: validation.check_permissions({\"resource\":\"secrets\",\"verb\":\"list\"}) => {\"allowed\":false}") {
		t.Fatalf("expected stable history line, got:\n%s", userContent)
	}
}

func TestReactValidationPlannerPrefixStableAcrossCalls(t *testing.T) {
	requestBodies := make([]map[string]any, 0, 5)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var requestBody map[string]any
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		requestBodies = append(requestBodies, requestBody)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"message": map[string]any{
					"role":    "assistant",
					"content": "done",
				},
			}},
		})
	}))
	defer server.Close()

	provider, err := llm.NewProvider(llm.ProviderConfig{
		Provider:    "openai",
		APIKey:      "fake",
		Model:       "gpt-4",
		BaseURL:     server.URL,
		HTTPTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	registry := tools.NewRegistry()
	schema := tools.Schema{
		Type: "object",
		Properties: map[string]tools.Schema{
			"namespace": {Type: "string", Description: "Namespace to query"},
			"name":      {Type: "string", Description: "Object name"},
		},
		Required: []string{"name"},
	}
	if err := registry.Register(&fakeTool{name: "validation.read_secret", schema: &schema}); err != nil {
		t.Fatalf("register read_secret: %v", err)
	}
	if err := registry.Register(&fakeTool{name: "validation.check_permissions", schema: &schema}); err != nil {
		t.Fatalf("register check_permissions: %v", err)
	}

	planner := newReactValidationPlanner(provider, registry, "openai")

	state := newState(executionModeValidation, "test goal", time.Now().UTC())
	for i := 0; i < 5; i++ {
		state.Iteration = i + 1
		state.appendHistory(historyEntry{
			Iteration: i + 1,
			ToolName:  "validation.read_secret",
			Input:     map[string]any{"name": fmt.Sprintf("secret-%d", i)},
			Output:    map[string]any{"status": "failed"},
		})
		if _, err := planner.NextAction(context.Background(), state, []string{"validation.read_secret", "validation.check_permissions"}); err != nil {
			t.Fatalf("planner call %d returned error: %v", i+1, err)
		}
	}

	if len(requestBodies) != 5 {
		t.Fatalf("expected 5 requests, got %d", len(requestBodies))
	}

	firstMessages := mustRequestMessages(t, requestBodies[0])
	firstSystem, _ := firstMessages[0]["content"].(string)
	firstToolsJSON, err := json.Marshal(requestBodies[0]["tools"])
	if err != nil {
		t.Fatalf("marshal first tools: %v", err)
	}

	for i := 1; i < len(requestBodies); i++ {
		messages := mustRequestMessages(t, requestBodies[i])
		systemContent, _ := messages[0]["content"].(string)
		if systemContent != firstSystem {
			t.Fatalf("request %d system prompt changed across calls", i+1)
		}

		toolsJSON, err := json.Marshal(requestBodies[i]["tools"])
		if err != nil {
			t.Fatalf("marshal request %d tools: %v", i+1, err)
		}
		if string(toolsJSON) != string(firstToolsJSON) {
			t.Fatalf("request %d tool payload changed across calls", i+1)
		}
	}
}

func TestRenderValidationPlannerHistory_SlidingWindow(t *testing.T) {
	// Empty history: no truncation applied, returns empty string.
	rendered := renderValidationPlannerHistory(nil)
	if rendered != "" {
		t.Fatalf("empty history: expected empty string, got %q", rendered)
	}
	rendered = renderValidationPlannerHistory([]historyEntry{})
	if rendered != "" {
		t.Fatalf("empty slice: expected empty string, got %q", rendered)
	}

	// Short history (len < maxHistoryEntries): all entries preserved.
	short := make([]historyEntry, maxHistoryEntries-1)
	for i := range short {
		short[i] = historyEntry{Iteration: i + 1, ToolName: fmt.Sprintf("t/%d", i+1)}
	}
	rendered = renderValidationPlannerHistory(short)
	for i := range short {
		if !strings.Contains(rendered, fmt.Sprintf("t/%d", i+1)) {
			t.Errorf("short history: expected t/%d in output", i+1)
		}
	}

	// Exact boundary (len == maxHistoryEntries): all entries preserved.
	full := make([]historyEntry, maxHistoryEntries)
	for i := range full {
		full[i] = historyEntry{Iteration: i + 1, ToolName: fmt.Sprintf("t/%d", i+1)}
	}
	rendered = renderValidationPlannerHistory(full)
	for i := range full {
		if !strings.Contains(rendered, fmt.Sprintf("t/%d", i+1)) {
			t.Errorf("full history: expected t/%d in output", i+1)
		}
	}

	// Overflow (len > maxHistoryEntries): only last maxHistoryEntries entries preserved.
	// Use iteration-number prefix as the authoritative identifier to avoid substring
	// collisions: "Iteration 1:" is not a substring of "Iteration 10:", unlike naive
	// "tool.1" patterns where "tool.1" is a substring of "tool.10".
	overflow := make([]historyEntry, maxHistoryEntries+5)
	for i := range overflow {
		overflow[i] = historyEntry{Iteration: i + 1, ToolName: fmt.Sprintf("tool-%d", i+1)}
	}
	rendered = renderValidationPlannerHistory(overflow)
	// Entries 1-5 (the oldest) must be absent — check by iteration number.
	for i := 1; i <= 5; i++ {
		if strings.Contains(rendered, fmt.Sprintf("Iteration %d:", i)) {
			t.Errorf("overflow history: expected Iteration %d: to be absent, but it was present", i)
		}
	}
	// Entries 6 through maxHistoryEntries+5 (the last maxHistoryEntries) must be present.
	for i := 6; i <= maxHistoryEntries+5; i++ {
		if !strings.Contains(rendered, fmt.Sprintf("Iteration %d:", i)) {
			t.Errorf("overflow history: expected Iteration %d: to be present, but it was absent", i)
		}
	}
}

// TestReactValidationPlannerGoalModePrefixByteStable verifies the Goal+Mode prefix is
// byte-stable across repeated planner calls. This ensures OpenAI prompt caching can work
// for the stable prefix while only Iteration and History grow per-call ().
func TestReactValidationPlannerGoalModePrefixByteStable(t *testing.T) {
	requestBodies := make([]map[string]any, 0, 5)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var requestBody map[string]any
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		requestBodies = append(requestBodies, requestBody)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"message": map[string]any{
					"role":    "assistant",
					"content": "done",
				},
			}},
		})
	}))
	defer server.Close()

	provider, err := llm.NewProvider(llm.ProviderConfig{
		Provider:    "openai",
		APIKey:      "fake",
		Model:       "gpt-4",
		BaseURL:     server.URL,
		HTTPTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	planner := newReactValidationPlanner(provider, tools.NewRegistry(), "openai")
	state := newState(executionModeValidation, "stable goal for caching test", time.Now().UTC())

	// Simulate 5 iterations of the planner loop with growing history.
	for i := 0; i < 5; i++ {
		state.Iteration = i + 1
		state.appendHistory(historyEntry{
			Iteration: i + 1,
			ToolName:  "validation.read_secret",
			Input:     map[string]any{"name": fmt.Sprintf("secret-%d", i)},
			Output:    map[string]any{"status": "validated"},
		})
		if _, err := planner.NextAction(context.Background(), state, nil); err != nil {
			t.Fatalf("planner call %d returned error: %v", i+1, err)
		}
	}

	if len(requestBodies) != 5 {
		t.Fatalf("expected 5 requests, got %d", len(requestBodies))
	}

	// Extract the Goal+Mode prefix from user message content (before "Iteration:").
	extractGoalModePrefix := func(content string) string {
		lines := strings.SplitN(content, "Iteration:", 2)
		return lines[0]
	}

	firstMessages := mustRequestMessages(t, requestBodies[0])
	firstUserContent, ok := firstMessages[1]["content"].(string)
	if !ok {
		t.Fatalf("expected user message content, got %#v", firstMessages[1])
	}
	firstGoalModePrefix := extractGoalModePrefix(firstUserContent)

	// Verify Goal+Mode prefix is byte-identical across all 5 calls.
	for i := 1; i < len(requestBodies); i++ {
		messages := mustRequestMessages(t, requestBodies[i])
		userContent, ok := messages[1]["content"].(string)
		if !ok {
			t.Fatalf("request %d: expected user message content, got %#v", i+1, messages[1])
		}
		goalModePrefix := extractGoalModePrefix(userContent)
		if goalModePrefix != firstGoalModePrefix {
			t.Errorf("request %d: Goal+Mode prefix changed\nfirst:  %q\nactual: %q",
				i+1, firstGoalModePrefix, goalModePrefix)
		}
	}

	// Also verify the prefix contains the expected Goal and Mode.
	if !strings.Contains(firstGoalModePrefix, "Goal: stable goal for caching test") {
		t.Errorf("expected Goal line in prefix, got %q", firstGoalModePrefix)
	}
	if !strings.Contains(firstGoalModePrefix, "Mode: validation") {
		t.Errorf("expected Mode line in prefix, got %q", firstGoalModePrefix)
	}
}

func mustRequestMessages(t *testing.T, requestBody map[string]any) []map[string]any {
	t.Helper()
	rawMessages, ok := requestBody["messages"].([]any)
	if !ok {
		t.Fatalf("expected messages array, got %#v", requestBody["messages"])
	}
	messages := make([]map[string]any, len(rawMessages))
	for i, raw := range rawMessages {
		message, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("expected message object, got %#v", raw)
		}
		messages[i] = message
	}
	return messages
}

// TestBuildPlannerStateSummary_KG004PartialProgress verifies that a partial KG-004
// progress state (one validated probe, second probe not yet attempted) is rendered
// clearly with the second distinct probe still required.
func TestBuildPlannerStateSummary_KG004PartialProgress(t *testing.T) {
	state := newState(executionModeValidation, "test goal", time.Now().UTC())
	state.Iteration = 2

	// KG-004-S1: first probe_network validated against a cross-namespace target.
	state.appendHistory(historyEntry{
		Iteration: 1,
		ToolName:  "validation.probe_network",
		Input:     map[string]any{"target": "some-service.some-ns.svc.cluster.local", "port": 8080, "probe": "tcp"},
		Output:    map[string]any{"reachable": true},
		Outcome:   validation.StepValidated,
	})
	// KG-004-S2: not yet attempted (no history entry for second probe).
	// KG-005-S1: namespace discovery
	state.appendHistory(historyEntry{
		Iteration: 2,
		ToolName:  "discovery.list_namespaces",
		Input:     map[string]any{},
		Output:    map[string]any{"namespaces": []any{"default", "secure-middleware"}},
		Outcome:   validation.StepValidated,
	})

	summary := buildPlannerStateSummary(state)
	if summary == "" {
		t.Fatal("expected non-empty summary for partial progress state")
	}

	// Scenario progress must show KG-004 with exactly 1/2 steps validated.
	if !strings.Contains(summary, "KG-004") {
		t.Fatalf("expected KG-004 in scenario_progress, got:\n%s", summary)
	}
	if !strings.Contains(summary, "1/2") {
		t.Fatalf("expected KG-004 1/2 progress in summary, got:\n%s", summary)
	}

	// The next_required_actions must list KG-004-S2 as still required.
	if !strings.Contains(summary, "KG-004-S2") {
		t.Fatalf("expected KG-004-S2 in next_required_actions, got:\n%s", summary)
	}

	// validated_facts must include the first probe.
	if !strings.Contains(summary, "validation.probe_network") {
		t.Fatalf("expected validated probe_network in validated_facts, got:\n%s", summary)
	}
}

// TestBuildPlannerStateSummary_KG004OneValidatedOneFailed verifies that when
// KG-004-S1 is validated and KG-004-S2 fails, the summary correctly shows
// the partial progress with the failed step.
func TestBuildPlannerStateSummary_KG004OneValidatedOneFailed(t *testing.T) {
	state := newState(executionModeValidation, "test goal", time.Now().UTC())
	state.Iteration = 3

	// KG-004-S1: first probe_network validated.
	state.appendHistory(historyEntry{
		Iteration: 1,
		ToolName:  "validation.probe_network",
		Input:     map[string]any{"target": "service-a.other-ns.svc.cluster.local", "port": 80, "probe": "tcp"},
		Output:    map[string]any{"reachable": true},
		Outcome:   validation.StepValidated,
	})
	// KG-004-S2: second probe failed (wrong shape — http probe against Redis target).
	state.appendHistory(historyEntry{
		Iteration:     2,
		ToolName:      "validation.probe_network",
		Input:         map[string]any{"target": "cache-store-service.secure-middleware.svc.cluster.local", "port": 6379, "probe": "http"},
		Output:        map[string]any{"reachable": false},
		Outcome:       validation.StepFailed,
		FailureReason: validation.FailureNetworkUnreachable,
	})
	// KG-004-S2: second probe with correct shape (tcp) — still pending, not yet attempted.

	summary := buildPlannerStateSummary(state)
	if summary == "" {
		t.Fatal("expected non-empty summary")
	}

	// KG-004 should show 1/2 validated and include KG-004-S2 in next_required_actions.
	if !strings.Contains(summary, "KG-004") {
		t.Fatalf("expected KG-004 in summary, got:\n%s", summary)
	}
	if !strings.Contains(summary, "1/2") {
		t.Fatalf("expected KG-004 1/2 in summary, got:\n%s", summary)
	}
	// The failed step should appear in failed_facts.
	if !strings.Contains(summary, "failed_facts:") {
		t.Fatalf("expected failed_facts section in summary, got:\n%s", summary)
	}
	if !strings.Contains(summary, "KG-004-S2") {
		t.Fatalf("expected KG-004-S2 in next_required_actions, got:\n%s", summary)
	}
}

// TestBuildPlannerStateSummary_DuplicateWorkPattern verifies that repeated tool
// executions appear in the avoid_repeating section.
func TestBuildPlannerStateSummary_DuplicateWorkPattern(t *testing.T) {
	state := newState(executionModeValidation, "test goal", time.Now().UTC())
	state.Iteration = 3

	// Same tool executed three times.
	for i := 1; i <= 3; i++ {
		state.appendHistory(historyEntry{
			Iteration: i,
			ToolName:  "validation.probe_network",
			Input:     map[string]any{"target": fmt.Sprintf("service-%d.default.svc.cluster.local", i), "port": 80},
			Output:    map[string]any{"reachable": true},
			Outcome:   validation.StepValidated,
		})
	}

	summary := buildPlannerStateSummary(state)
	if summary == "" {
		t.Fatal("expected non-empty summary")
	}

	if !strings.Contains(summary, "avoid_repeating:") {
		t.Fatalf("expected avoid_repeating section, got:\n%s", summary)
	}
	if !strings.Contains(summary, "validation.probe_network") {
		t.Fatalf("expected validation.probe_network in avoid_repeating, got:\n%s", summary)
	}
	if !strings.Contains(summary, "3 times") {
		t.Fatalf("expected '3 times' count in avoid_repeating, got:\n%s", summary)
	}
}

// TestBuildPlannerStateSummary_Deterministic verifies that the summary output
// is identical for identical state across multiple calls.
func TestBuildPlannerStateSummary_Deterministic(t *testing.T) {
	state := newState(executionModeValidation, "test goal", time.Now().UTC())
	state.Iteration = 2
	state.appendHistory(historyEntry{
		Iteration: 1,
		ToolName:  "validation.check_token",
		Output:    map[string]any{"namespace": "test-ns"},
		Outcome:   validation.StepValidated,
	})
	state.appendHistory(historyEntry{
		Iteration:     2,
		ToolName:      "validation.check_permissions",
		Input:         map[string]any{"verb": "list", "resource": "secrets"},
		Outcome:       validation.StepFailed,
		FailureReason: validation.FailureRBACDenied,
	})

	first := buildPlannerStateSummary(state)
	second := buildPlannerStateSummary(state)
	third := buildPlannerStateSummary(state)

	if first != second || second != third {
		t.Errorf("summary is not deterministic:\nfirst:  %q\nsecond: %q\nthird:  %q", first, second, third)
	}
}

// TestBuildPlannerStateSummary_Bounded verifies that the summary output is
// bounded regardless of how many history entries exist.
func TestBuildPlannerStateSummary_Bounded(t *testing.T) {
	state := newState(executionModeValidation, "test goal", time.Now().UTC())
	state.Iteration = 50

	// Append far more than maxSummaryFacts entries.
	for i := 1; i <= 50; i++ {
		state.appendHistory(historyEntry{
			Iteration: i,
			ToolName:  "validation.probe_network",
			Input:     map[string]any{"target": fmt.Sprintf("service-%d.default.svc.cluster.local", i)},
			Output:    map[string]any{"reachable": true},
			Outcome:   validation.StepValidated,
		})
	}

	summary := buildPlannerStateSummary(state)
	if summary == "" {
		t.Fatal("expected non-empty summary")
	}

	// validated_facts section should have at most maxSummaryFacts entries.
	// Extract only the validated_facts section by finding the section header
	// and consuming lines until the next section header or end of summary.
	validatedFactsSection := ""
	if idx := strings.Index(summary, "validated_facts:"); idx >= 0 {
		end := len(summary)
		if nextIdx := strings.Index(summary[idx+len("validated_facts:"):], "  failed_facts:"); nextIdx > 0 {
			end = idx + len("validated_facts:") + nextIdx
		} else if nextIdx := strings.Index(summary[idx+len("validated_facts:"):], "  scenario_progress:"); nextIdx > 0 {
			end = idx + len("validated_facts:") + nextIdx
		} else if nextIdx := strings.Index(summary[idx+len("validated_facts:"):], "  avoid_repeating:"); nextIdx > 0 {
			end = idx + len("validated_facts:") + nextIdx
		} else if nextIdx := strings.Index(summary[idx+len("validated_facts:"):], "  next_required_actions:"); nextIdx > 0 {
			end = idx + len("validated_facts:") + nextIdx
		}
		validatedFactsSection = summary[idx:end]
	}
	validatedFactCount := strings.Count(validatedFactsSection, "    - validation.probe_network")
	if validatedFactCount > maxSummaryFacts {
		t.Errorf("validated_facts exceeds bound: got %d, max %d", validatedFactCount, maxSummaryFacts)
	}

	// avoid_repeating should list probe_network once (since it's one tool, sorted).
	if !strings.Contains(summary, "avoid_repeating:") {
		t.Fatalf("expected avoid_repeating section, got:\n%s", summary)
	}

	// scenario_progress should list all 5 families (capped at maxSummaryFacts=5).
	progressLines := 0
	for _, line := range strings.Split(summary, "\n") {
		if strings.Contains(line, "KG-001") || strings.Contains(line, "KG-002") ||
			strings.Contains(line, "KG-003") || strings.Contains(line, "KG-004") ||
			strings.Contains(line, "KG-005") {
			if strings.Contains(line, "scenario_progress:") || strings.HasPrefix(strings.TrimSpace(line), "KG-") {
				progressLines++
			}
		}
	}
	// 5 families, all showing as 2/2 or similar — but bounded to maxSummaryFacts=5 lines
	if progressLines > maxSummaryFacts+1 { // +1 for the header line
		t.Errorf("scenario_progress exceeds bound: counted %d lines, max expected %d", progressLines, maxSummaryFacts+1)
	}
}

// TestBuildPlannerStateSummary_EmptyHistory verifies that an empty or nil history
// produces an empty summary string.
func TestBuildPlannerStateSummary_EmptyHistory(t *testing.T) {
	emptyState := newState(executionModeValidation, "test goal", time.Now().UTC())
	if summary := buildPlannerStateSummary(emptyState); summary != "" {
		t.Errorf("expected empty summary for empty history, got: %q", summary)
	}

	nilState := (*state)(nil)
	if summary := buildPlannerStateSummary(nilState); summary != "" {
		t.Errorf("expected empty summary for nil state, got: %q", summary)
	}
}

// TestBuildPlannerStateSummary_NextActionsExcludesCompleteFamilies verifies that
// families with all steps validated do not appear in next_required_actions.
func TestBuildPlannerStateSummary_NextActionsExcludesCompleteFamilies(t *testing.T) {
	state := newState(executionModeValidation, "test goal", time.Now().UTC())
	state.Iteration = 3

	// KG-001: all 3 steps validated.
	state.appendHistory(historyEntry{
		Iteration: 1,
		ToolName:  "validation.check_token",
		Output:    map[string]any{"namespace": "test-ns"},
		Outcome:   validation.StepValidated,
	})
	state.appendHistory(historyEntry{
		Iteration: 2,
		ToolName:  "validation.check_permissions",
		Input:     map[string]any{"verb": "list", "resource": "secrets"},
		Outcome:   validation.StepValidated,
	})
	state.appendHistory(historyEntry{
		Iteration: 3,
		ToolName:  "validation.read_secret",
		Input:     map[string]any{"name": "db-creds"},
		Outcome:   validation.StepValidated,
	})

	summary := buildPlannerStateSummary(state)

	// KG-001 should show 3/3 validated.
	if !strings.Contains(summary, "KG-001") {
		t.Fatalf("expected KG-001 in summary, got:\n%s", summary)
	}
	if !strings.Contains(summary, "3/3") {
		t.Fatalf("expected KG-001 3/3 in summary, got:\n%s", summary)
	}

	// KG-001 should NOT appear in next_required_actions.
	if strings.Contains(summary, "KG-001-S1") || strings.Contains(summary, "KG-001-S2") || strings.Contains(summary, "KG-001-S3") {
		if strings.Contains(summary, "next_required_actions:") {
			// Extract the next_required_actions section
			idx := strings.Index(summary, "next_required_actions:")
			if idx >= 0 {
				section := summary[idx:]
				if strings.Contains(section, "KG-001") {
					t.Fatalf("KG-001 should not appear in next_required_actions when fully validated, got:\n%s", section)
				}
			}
		}
	}
}

// TestBuildPlannerStateSummary_AppearsInUserMessage verifies that the summary is
// injected into the planner user message when history is non-empty.
func TestBuildPlannerStateSummary_AppearsInUserMessage(t *testing.T) {
	var requestBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"message": map[string]any{
					"role":    "assistant",
					"content": "done",
				},
			}},
		})
	}))
	defer server.Close()

	provider, err := llm.NewProvider(llm.ProviderConfig{
		Provider:    "openai",
		APIKey:      "fake",
		Model:       "gpt-4",
		BaseURL:     server.URL,
		HTTPTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	registry := tools.NewRegistry()
	if err := registry.Register(&fakeTool{name: "validation.probe_network", schema: (*tools.Schema)(nil)}); err != nil {
		t.Fatalf("register probe_network: %v", err)
	}

	planner := newReactValidationPlanner(provider, registry, "openai")
	state := newState(executionModeValidation, "test goal", time.Now().UTC())
	state.Iteration = 1
	state.appendHistory(historyEntry{
		Iteration: 1,
		ToolName:  "validation.probe_network",
		Input:     map[string]any{"target": "service.default.svc.cluster.local", "port": 80},
		Output:    map[string]any{"reachable": true},
		Outcome:   validation.StepValidated,
	})

	if _, err := planner.NextAction(context.Background(), state, []string{"validation.probe_network"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	messages := mustRequestMessages(t, requestBody)
	userContent, ok := messages[1]["content"].(string)
	if !ok {
		t.Fatalf("expected user message content, got %#v", messages[1])
	}

	// The PlannerStateSummary header must appear in the user message.
	if !strings.Contains(userContent, "PlannerStateSummary:") {
		t.Fatalf("expected PlannerStateSummary in user message, got:\n%s", userContent)
	}
	if !strings.Contains(userContent, "validated_facts:") {
		t.Fatalf("expected validated_facts in user message, got:\n%s", userContent)
	}
	if !strings.Contains(userContent, "scenario_progress:") {
		t.Fatalf("expected scenario_progress in user message, got:\n%s", userContent)
	}
	// History must still appear after the summary.
	if !strings.Contains(userContent, "History of executed tools and their results:") {
		t.Fatalf("expected history section in user message, got:\n%s", userContent)
	}
}
