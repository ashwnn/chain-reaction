package agent

import (
	"fmt"
	"time"

	"github.com/ashwnn/chain-reaction/internal/llm"
	"github.com/ashwnn/chain-reaction/internal/tools/validation"
)

type actionType string

const (
	actionTypeExecute     actionType = "execute"
	actionTypeFinalAnswer actionType = "final_answer"
)

type plannerAction struct {
	Thought     string             `json:"thought,omitempty"`
	ToolName    string             `json:"tool_name,omitempty"`
	Parameters  map[string]any     `json:"parameters,omitempty"`
	ActionType  actionType         `json:"action_type"`
	FinalAnswer string             `json:"final_answer,omitempty"`
	Usage       *llm.UsageMetadata `json:"usage,omitempty"`
}

type toolExecutionResult struct {
	ActionID                string                   `json:"action_id"`
	ActionSequence          int                      `json:"action_sequence"`
	ToolName                string                   `json:"tool_name"`
	Input                   map[string]any           `json:"input,omitempty"`
	Output                  map[string]any           `json:"output,omitempty"`
	Success                 bool                     `json:"success"`
	Error                   string                   `json:"error,omitempty"`
	Outcome                 validation.StepResult    `json:"outcome,omitempty"`
	FailureReason           validation.FailureReason `json:"failure_reason,omitempty"`
	EvaluatorClassification string                   `json:"evaluator_classification,omitempty"`
	EvaluatorRationale      string                   `json:"evaluator_rationale,omitempty"`
	EvaluatorError          string                   `json:"evaluator_error,omitempty"`
	Timestamp               time.Time                `json:"timestamp"`
	DurationMS              int64                    `json:"duration_ms"`
	PlannerUsage            *llm.UsageMetadata       `json:"planner_usage,omitempty"`
	EvaluatorUsage          *llm.UsageMetadata       `json:"evaluator_usage,omitempty"`
	// CandidateStepIDs lists the KG step IDs from
	// built-in catalog that this tool execution could
	// contribute to. The mapping is tool-name-derived only; it does NOT account
	// for prerequisite ordering, outcome status, or chain state. An empty slice
	// means the tool is not part of any catalog step chain.
	CandidateStepIDs []string `json:"candidate_step_ids,omitempty"`
}

func (a plannerAction) Validate(availableTools []string) error {
	switch a.ActionType {
	case actionTypeExecute:
		if a.ToolName == "" {
			return fmt.Errorf("execute action requires tool name")
		}
		if !containsTool(availableTools, a.ToolName) {
			return fmt.Errorf("unknown tool %q", a.ToolName)
		}
		return nil
	case actionTypeFinalAnswer:
		if a.FinalAnswer == "" {
			return fmt.Errorf("final_answer action requires final answer text")
		}
		return nil
	default:
		return fmt.Errorf("invalid action type %q", a.ActionType)
	}
}

func containsTool(availableTools []string, toolName string) bool {
	for _, available := range availableTools {
		if available == toolName {
			return true
		}
	}
	return false
}
