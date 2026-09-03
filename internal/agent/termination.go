package agent

import (
	"encoding/json"
	"time"
)

type stopReason string

const (
	stopReasonNone                 stopReason = ""
	stopReasonGoalAchieved         stopReason = "goal_achieved"
	stopReasonAllFamiliesValidated stopReason = "all_families_validated"
	stopReasonMaxIterationsReached stopReason = "max_iterations_reached"
	stopReasonTimeout              stopReason = "timeout"
	stopReasonNoProgress           stopReason = "no_progress"
	stopReasonGuardrailStop        stopReason = "guardrail_stop"
)

type terminationChecker struct {
	MaxIterations       int
	Timeout             time.Duration
	repeatedActionLimit int
}

func (tc terminationChecker) shouldStop(now time.Time, state *state) (bool, stopReason) {
	if state == nil {
		return false, stopReasonNone
	}
	if flagFromContext(state, "goal_achieved") {
		return true, stopReasonGoalAchieved
	}
	if flagFromContext(state, "guardrail_stop") {
		return true, stopReasonGuardrailStop
	}
	if tc.MaxIterations > 0 && state.Iteration >= tc.MaxIterations {
		return true, stopReasonMaxIterationsReached
	}
	if tc.MaxIterations > 0 && intFromContext(state, "rejected_final_answer_count") >= tc.MaxIterations {
		return true, stopReasonMaxIterationsReached
	}
	if tc.Timeout > 0 && !state.StartedAt.IsZero() && now.UTC().Sub(state.StartedAt) >= tc.Timeout {
		return true, stopReasonTimeout
	}
	if tc.repeatedActionLimit > 0 && hasNoProgress(state.History, tc.repeatedActionLimit) {
		return true, stopReasonNoProgress
	}
	return false, stopReasonNone
}

func intFromContext(state *state, key string) int {
	if state == nil || state.Context == nil {
		return 0
	}
	value, ok := state.Context[key].(int)
	if !ok {
		return 0
	}
	return value
}

func flagFromContext(state *state, key string) bool {
	if state == nil || state.Context == nil {
		return false
	}
	value, ok := state.Context[key].(bool)
	return ok && value
}

func hasNoProgress(history []historyEntry, limit int) bool {
	if len(history) < limit || limit <= 1 {
		return false
	}
	start := len(history) - limit
	reference := historySignature(history[start])
	for i := start + 1; i < len(history); i++ {
		if historySignature(history[i]) != reference {
			return false
		}
	}
	return true
}

func historySignature(entry historyEntry) string {
	payload, err := json.Marshal(entry.Input)
	if err != nil {
		return entry.ToolName
	}
	return entry.ToolName + ":" + string(payload)
}
