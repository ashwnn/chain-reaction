package agent

import (
	"os"
	"strings"
	"testing"

	"github.com/ashwnn/chain-reaction/internal/llm"
)

func TestRunValidationLoopWritesDebugLogArtifact(t *testing.T) {
	planner := &stubValidationPlanner{
		actions: []plannerAction{
			{
				ActionType: actionTypeExecute,
				ToolName:   "discovery.list_namespaces",
				Thought:    "Enumerate visible namespaces before selecting a validation target.",
			},
			{ActionType: actionTypeFinalAnswer, FinalAnswer: "done"},
		},
	}

	result, err := runValidationLoopForTest(t, planner)
	if err != nil {
		t.Fatalf("runValidationLoopForTest returned error: %v", err)
	}
	if result.DebugLogPath == "" {
		t.Fatal("expected debug log path to be populated")
	}

	payload, err := os.ReadFile(result.DebugLogPath)
	if err != nil {
		t.Fatalf("read debug log: %v", err)
	}
	logText := string(payload)

	for _, needle := range []string{
		"event=validation_loop_start",
		"event=planner_request_start",
		"event=planner_request_complete",
		"event=tool_start",
		"event=tool_complete",
		"event=termination",
		"last_completed_tool=discovery.list_namespaces",
		"last_attempted_tool=discovery.list_namespaces",
		`planner_rationale="Enumerate visible namespaces before selecting a validation target."`,
		"event=final_answer_rejected_unmet_catalog_steps",
		`reason="max_iterations_reached"`,
	} {
		if !strings.Contains(logText, needle) {
			t.Fatalf("expected debug log to contain %q, got:\n%s", needle, logText)
		}
	}
}

func TestCompactValidationDebugText(t *testing.T) {
	got := compactValidationDebugText("first\n\nsecond\tthird", 100)
	if got != "first second third" {
		t.Fatalf("unexpected compacted text: %q", got)
	}

	got = compactValidationDebugText("abcdefghijklmnopqrstuvwxyz", 10)
	if got != "abcdefg..." {
		t.Fatalf("unexpected truncated text: %q", got)
	}
}

func TestFormatLLMUsageFields_CostUnavailable(t *testing.T) {
	usage := &llm.UsageMetadata{
		Provider:         "openai",
		InputTokens:      100,
		OutputTokens:     50,
		TotalTokens:      150,
		CacheRead:        20,
		CacheWrite:       10,
		EstimatedCostUSD: nil,
	}
	fields := formatLLMUsageFields(usage, "llm.")

	if fields == nil {
		t.Fatal("expected non-nil fields map")
	}
	if _, ok := fields["llm.estimated_cost_usd"]; ok {
		t.Fatalf("estimated_cost_usd should not be in fields when EstimatedCostUSD is nil, got: %v", fields)
	}
	if fields["llm.input_tokens"] != 100 {
		t.Fatalf("expected input_tokens=100, got: %v", fields["llm.input_tokens"])
	}
}

func TestFormatLLMUsageFields_CostAvailable(t *testing.T) {
	cost := 0.0015
	usage := &llm.UsageMetadata{
		Provider:         "openai",
		InputTokens:      100,
		OutputTokens:     50,
		TotalTokens:      150,
		EstimatedCostUSD: &cost,
	}
	fields := formatLLMUsageFields(usage, "llm.")

	if fields == nil {
		t.Fatal("expected non-nil fields map")
	}
	if _, ok := fields["llm.estimated_cost_usd"]; !ok {
		t.Fatalf("estimated_cost_usd should be present when EstimatedCostUSD is set, got: %v", fields)
	}
}

func TestFormatLLMUsageFields_NilUsage(t *testing.T) {
	fields := formatLLMUsageFields(nil, "llm.")
	if fields != nil {
		t.Fatalf("expected nil fields for nil usage, got: %v", fields)
	}
}

func TestFormatLLMUsageFields_CacheFieldsOmittedWhenZero(t *testing.T) {
	usage := &llm.UsageMetadata{
		Provider:     "openai",
		InputTokens:  100,
		OutputTokens: 50,
		TotalTokens:  150,
		CacheRead:    0,
		CacheWrite:   0,
	}
	fields := formatLLMUsageFields(usage, "llm.")

	if fields == nil {
		t.Fatal("expected non-nil fields map")
	}
	if _, ok := fields["llm.cache_read_tokens"]; ok {
		t.Fatalf("cache_read_tokens should be omitted when zero, got: %v", fields)
	}
	if _, ok := fields["llm.cache_write_tokens"]; ok {
		t.Fatalf("cache_write_tokens should be omitted when zero, got: %v", fields)
	}
}
