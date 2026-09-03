package agent

import (
	"testing"
	"time"
)

func TestExecutionStateInitialization(t *testing.T) {
	startedAt := time.Date(2026, time.March, 17, 17, 30, 0, 0, time.UTC)
	state := newState(executionModeValidation, "validate chain", startedAt)

	if state == nil {
		t.Fatal("expected state to be created")
	}
	if state.Mode != executionModeValidation {
		t.Fatalf("expected validation mode, got %q", state.Mode)
	}
	if state.Iteration != 0 {
		t.Fatalf("expected zero iterations, got %d", state.Iteration)
	}
	if state.Goal != "validate chain" {
		t.Fatalf("unexpected goal %q", state.Goal)
	}
	if state.StartedAt != startedAt {
		t.Fatalf("expected start time %v, got %v", startedAt, state.StartedAt)
	}
	if state.Context == nil {
		t.Fatal("expected context map to be initialized")
	}
	if len(state.History) != 0 {
		t.Fatalf("expected empty history, got %d entries", len(state.History))
	}
}

func TestStateHistoryAppend(t *testing.T) {
	state := newState(executionModeValidation, "validate chain", time.Now())
	entry := historyEntry{
		Iteration: 1,
		ToolName:  "discovery.list_namespaces",
		Input:     map[string]any{"namespace": "team-a"},
		Output:    map[string]any{"count": 1},
		Timestamp: time.Now().UTC(),
	}

	state.appendHistory(entry)

	if len(state.History) != 1 {
		t.Fatalf("expected one history entry, got %d", len(state.History))
	}
	if state.History[0].ToolName != entry.ToolName {
		t.Fatalf("expected tool name %q, got %q", entry.ToolName, state.History[0].ToolName)
	}
	if state.History[0].Iteration != 1 {
		t.Fatalf("expected history iteration 1, got %d", state.History[0].Iteration)
	}
	if state.Iteration != 0 {
		t.Fatalf("expected append not to mutate state iteration, got %d", state.Iteration)
	}
}
