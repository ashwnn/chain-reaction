package agent

import (
	"testing"
	"time"
)

func TestTerminationReasons(t *testing.T) {
	now := time.Date(2026, time.March, 17, 18, 0, 0, 0, time.UTC)
	checker := terminationChecker{MaxIterations: 5, Timeout: 2 * time.Minute, repeatedActionLimit: 3}

	tests := []struct {
		name   string
		state  *state
		reason stopReason
	}{
		{
			name:   "goal achieved",
			state:  stateWithContext(map[string]any{"goal_achieved": true}),
			reason: stopReasonGoalAchieved,
		},
		{
			name:   "guardrail stop",
			state:  stateWithContext(map[string]any{"guardrail_stop": true}),
			reason: stopReasonGuardrailStop,
		},
		{
			name:   "max iterations reached",
			state:  &state{Iteration: 5, StartedAt: now.Add(-time.Minute), Context: map[string]any{}},
			reason: stopReasonMaxIterationsReached,
		},
		{
			name:   "timeout",
			state:  &state{Iteration: 1, StartedAt: now.Add(-3 * time.Minute), Context: map[string]any{}},
			reason: stopReasonTimeout,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shouldStop, reason := checker.shouldStop(now, tt.state)
			if !shouldStop {
				t.Fatal("expected termination")
			}
			if reason != tt.reason {
				t.Fatalf("expected reason %q, got %q", tt.reason, reason)
			}
		})
	}
}

func TestNoProgressTermination(t *testing.T) {
	now := time.Now().UTC()
	state := newState(executionModeValidation, "validate chain", now)
	state.appendHistory(historyEntry{Iteration: 1, ToolName: "discovery.list_pods", Input: map[string]any{"namespace": "team-a"}})
	state.appendHistory(historyEntry{Iteration: 2, ToolName: "discovery.list_pods", Input: map[string]any{"namespace": "team-a"}})
	state.appendHistory(historyEntry{Iteration: 3, ToolName: "discovery.list_pods", Input: map[string]any{"namespace": "team-a"}})

	checker := terminationChecker{repeatedActionLimit: 3}
	shouldStop, reason := checker.shouldStop(now, state)
	if !shouldStop {
		t.Fatal("expected repeated actions to trigger termination")
	}
	if reason != stopReasonNoProgress {
		t.Fatalf("expected no progress reason, got %q", reason)
	}
}

func TestTimeoutTermination(t *testing.T) {
	now := time.Date(2026, time.March, 17, 18, 5, 0, 0, time.UTC)
	state := newState(executionModeValidation, "validate chain", now.Add(-10*time.Minute))
	checker := terminationChecker{Timeout: 5 * time.Minute}

	shouldStop, reason := checker.shouldStop(now, state)
	if !shouldStop {
		t.Fatal("expected timeout termination")
	}
	if reason != stopReasonTimeout {
		t.Fatalf("expected timeout reason, got %q", reason)
	}
}

func stateWithContext(values map[string]any) *state {
	state := newState(executionModeValidation, "validate chain", time.Now().UTC())
	for key, value := range values {
		state.Context[key] = value
	}
	return state
}
