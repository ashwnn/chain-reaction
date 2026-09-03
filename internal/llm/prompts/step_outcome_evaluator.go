package prompts

import (
	"encoding/json"
	"fmt"
	"strings"
)

// StepOutcomeEvaluatorInput contains the fields needed to classify a single step outcome.
type StepOutcomeEvaluatorInput struct {
	ToolName         string
	Parameters       map[string]any
	Output           map[string]any
	CandidateStepIDs []string
	ExpectedSuccess  string
	CurrentGoal      string
}

// RenderStepOutcomeEvaluatorSystemPrompt renders the system prompt for the independent
// step-outcome evaluator. This prompt is intentionally separate from the planner prompt.
func RenderStepOutcomeEvaluatorSystemPrompt() string {
	return `You independently classify a single Chain Reaction validation-loop step.
You are not the planner.
You do not choose the next action.
Your only job is to classify the completed step outcome from the evidence provided.

Return JSON only with this exact shape:
{"classification":"validated|failed_rbac|failed_network|failed_prereq|theoretical","rationale":"short explanation"}

Classification rules:
- Use "validated" only when the completed step directly proves the intended security property.
- Use "failed_rbac" when the step failed because access was denied by Kubernetes authorization.
- Use "failed_network" when the step failed because the target could not be reached over the network.
- Use "failed_prereq" when the step failed because a required target, token, secret, namespace, parameter, or other prerequisite was missing or invalid.
- Use "theoretical" when the step gathered context but did not directly prove the intended property.

Keep the rationale short and evidence-backed. Output JSON only.`
}

// RenderStepOutcomeEvaluatorUserPrompt renders the user prompt for the evaluator call.
func RenderStepOutcomeEvaluatorUserPrompt(input StepOutcomeEvaluatorInput) string {
	var b strings.Builder
	paramsJSON, _ := json.Marshal(input.Parameters)
	outputJSON, _ := json.Marshal(input.Output)

	_, _ = fmt.Fprintf(&b, "Current validation goal: %s\n", strings.TrimSpace(input.CurrentGoal))
	_, _ = fmt.Fprintf(&b, "Tool: %s\n", input.ToolName)
	_, _ = fmt.Fprintf(&b, "Candidate step IDs: %s\n", strings.Join(input.CandidateStepIDs, ", "))
	_, _ = fmt.Fprintf(&b, "Expected success condition: %s\n", input.ExpectedSuccess)
	_, _ = fmt.Fprintf(&b, "Input parameters JSON: %s\n", string(paramsJSON))
	_, _ = fmt.Fprintf(&b, "Raw output JSON: %s\n", string(outputJSON))

	return strings.TrimSpace(b.String())
}
