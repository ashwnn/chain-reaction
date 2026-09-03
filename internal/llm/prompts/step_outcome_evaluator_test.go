package prompts

import (
	"strings"
	"testing"
)

func TestRenderStepOutcomeEvaluatorSystemPrompt(t *testing.T) {
	prompt := RenderStepOutcomeEvaluatorSystemPrompt()

	required := []string{
		"You independently classify a single Chain Reaction validation-loop step.",
		"You are not the planner.",
		`"classification":"validated|failed_rbac|failed_network|failed_prereq|theoretical"`,
		`Use "theoretical" when the step gathered context`,
		"Output JSON only.",
	}

	for _, clause := range required {
		if !strings.Contains(prompt, clause) {
			t.Fatalf("system prompt missing clause %q:\n%s", clause, prompt)
		}
	}
}

func TestRenderStepOutcomeEvaluatorUserPrompt(t *testing.T) {
	prompt := RenderStepOutcomeEvaluatorUserPrompt(StepOutcomeEvaluatorInput{
		ToolName:         "validation.check_permissions",
		Parameters:       map[string]any{"namespace": "default", "verb": "get", "resource": "secrets"},
		Output:           map[string]any{"allowed": false, "reason": "RBAC denied"},
		CandidateStepIDs: []string{"KG-001-S2", "KG-002-S1"},
		ExpectedSuccess:  "The step validates only if allowed=true for the requested permission.",
		CurrentGoal:      "Validate KG-001 through KG-005 from the current pod identity.",
	})

	required := []string{
		"Current validation goal: Validate KG-001 through KG-005 from the current pod identity.",
		"Tool: validation.check_permissions",
		"Candidate step IDs: KG-001-S2, KG-002-S1",
		"Expected success condition: The step validates only if allowed=true for the requested permission.",
		`"namespace":"default"`,
		`"allowed":false`,
	}

	for _, clause := range required {
		if !strings.Contains(prompt, clause) {
			t.Fatalf("user prompt missing clause %q:\n%s", clause, prompt)
		}
	}
}
