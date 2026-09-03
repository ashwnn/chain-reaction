package agent

import (
	"time"

	"github.com/ashwnn/chain-reaction/internal/config"
	"github.com/ashwnn/chain-reaction/internal/tools/validation"
)

type executionMode string

const (
	executionModeValidation executionMode = "validation"
)

type state struct {
	Mode      executionMode
	Iteration int
	Goal      string
	History   []historyEntry
	Context   map[string]any
	StartedAt time.Time
}

type historyEntry struct {
	Iteration int
	ToolName  string
	Input     map[string]any
	Output    map[string]any
	Timestamp time.Time
	// Thought captures the LLM planner's reasoning for this step, enabling
	// the ReAct observe→plan feedback loop. Empty when the model produced no
	// intermediate reasoning.
	Thought string
	// Outcome records the step result classification from the validation taxonomy.
	// Set by the validation loop when appending history. Zero value means
	// the outcome was not recorded (e.g., for pre-loop entries).
	Outcome validation.StepResult
	// FailureReason records why a step failed, when Outcome is StepFailed.
	// Zero value means no failure was recorded.
	FailureReason validation.FailureReason
}

func newState(mode executionMode, goal string, startedAt time.Time) *state {
	return &state{
		Mode:      mode,
		Goal:      goal,
		History:   make([]historyEntry, 0),
		Context:   make(map[string]any),
		StartedAt: startedAt.UTC(),
	}
}

func (s *state) appendHistory(entry historyEntry) {
	if s == nil {
		return
	}
	s.History = append(s.History, entry)
}

func plannerModeFromState(s *state) config.PlannerMode {
	if s != nil && s.Context != nil {
		if mode, ok := s.Context["planner_mode"].(config.PlannerMode); ok {
			return mode
		}
	}
	return config.PlannerModeGoatHinted
}
