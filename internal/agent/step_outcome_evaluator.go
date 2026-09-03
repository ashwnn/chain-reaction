package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ashwnn/chain-reaction/internal/config"
	"github.com/ashwnn/chain-reaction/internal/graph"
	"github.com/ashwnn/chain-reaction/internal/llm"
	"github.com/ashwnn/chain-reaction/internal/llm/prompts"
	"github.com/ashwnn/chain-reaction/internal/tools/validation"
)

type stepOutcomeClassification string

const (
	stepOutcomeValidated     stepOutcomeClassification = "validated"
	stepOutcomeFailedRBAC    stepOutcomeClassification = "failed_rbac"
	stepOutcomeFailedNetwork stepOutcomeClassification = "failed_network"
	stepOutcomeFailedPrereq  stepOutcomeClassification = "failed_prereq"
	stepOutcomeTheoretical   stepOutcomeClassification = "theoretical"
)

type stepOutcomeEvaluation struct {
	Classification stepOutcomeClassification `json:"classification"`
	Rationale      string                    `json:"rationale,omitempty"`
	Usage          *llm.UsageMetadata        `json:"usage,omitempty"`
}

type stepOutcomeEvaluationInput struct {
	ToolName         string
	Parameters       map[string]any
	Output           map[string]any
	CandidateStepIDs []string
	CurrentGoal      string
}

type stepOutcomeEvaluator interface {
	Evaluate(context.Context, stepOutcomeEvaluationInput) (stepOutcomeEvaluation, error)
}

type llmStepOutcomeEvaluator struct {
	client llm.Provider
}

var newStepOutcomeEvaluator = func(cfg config.Config) stepOutcomeEvaluator {
	// StepOutcomeEvaluator is an opt-in cost-sensitive feature. Even when LLM
	// credentials are present, the evaluator is disabled unless explicitly enabled
	// via --step-outcome-evaluator or step_outcome_evaluator: true in the config.
	if cfg.LLMAPIKey == "" || cfg.LLMModel == "" || !cfg.StepOutcomeEvaluator {
		return nil
	}

	temperature := 0.0
	providerCfg := llm.ProviderConfig{
		Provider:    cfg.LLMProvider,
		APIKey:      cfg.LLMAPIKey,
		Model:       cfg.LLMModel,
		BaseURL:     cfg.LLMBaseURL,
		Temperature: &temperature,
	}

	provider, err := llm.NewProvider(providerCfg)
	if err != nil {
		return nil
	}
	return &llmStepOutcomeEvaluator{client: provider}
}

func (e *llmStepOutcomeEvaluator) Evaluate(ctx context.Context, input stepOutcomeEvaluationInput) (stepOutcomeEvaluation, error) {
	if e == nil || e.client == nil {
		return stepOutcomeEvaluation{}, fmt.Errorf("step outcome evaluator is not configured")
	}

	messages := []llm.ChatMessage{
		{Role: "system", Content: prompts.RenderStepOutcomeEvaluatorSystemPrompt()},
		{
			Role: "user",
			Content: prompts.RenderStepOutcomeEvaluatorUserPrompt(prompts.StepOutcomeEvaluatorInput{
				ToolName:         input.ToolName,
				Parameters:       input.Parameters,
				Output:           input.Output,
				CandidateStepIDs: input.CandidateStepIDs,
				ExpectedSuccess:  expectedSuccessCondition(input.ToolName),
				CurrentGoal:      input.CurrentGoal,
			}),
		},
	}

	action, err := e.client.Complete(ctx, messages, nil)
	if err != nil {
		return stepOutcomeEvaluation{}, fmt.Errorf("evaluate step outcome: %w", err)
	}

	evaluation, err := parsestepOutcomeEvaluation(action.FinalAnswer)
	if err != nil {
		return stepOutcomeEvaluation{}, fmt.Errorf("parse evaluator response: %w", err)
	}
	evaluation.Usage = action.Usage
	return evaluation, nil
}

func parsestepOutcomeEvaluation(raw string) (stepOutcomeEvaluation, error) {
	type payload struct {
		Classification stepOutcomeClassification `json:"classification"`
		Rationale      string                    `json:"rationale"`
	}

	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end < start {
		return stepOutcomeEvaluation{}, fmt.Errorf("response did not contain a JSON object: %q", raw)
	}

	var decoded payload
	if err := json.Unmarshal([]byte(raw[start:end+1]), &decoded); err != nil {
		return stepOutcomeEvaluation{}, err
	}

	switch decoded.Classification {
	case stepOutcomeValidated, stepOutcomeFailedRBAC, stepOutcomeFailedNetwork, stepOutcomeFailedPrereq, stepOutcomeTheoretical:
	default:
		return stepOutcomeEvaluation{}, fmt.Errorf("unsupported classification %q", decoded.Classification)
	}

	return stepOutcomeEvaluation{
		Classification: decoded.Classification,
		Rationale:      decoded.Rationale,
	}, nil
}

func expectedSuccessCondition(toolName string) string {
	switch toolName {
	case "validation.check_token":
		return "Validated only when the step inspects the current or specified ServiceAccount with a readable mounted token and produces direct identity evidence."
	case "validation.check_permissions":
		return "Validated only when the step proves the requested permission is allowed for the current identity."
	case "validation.read_secret":
		return "Validated only when the step proves bounded Secret access succeeded for the requested target."
	case "validation.probe_network":
		return "Validated only when the step proves bounded network reachability to the target."
	case "discovery.list_namespaces":
		return "This step is usually theoretical unless it directly establishes cross-namespace context needed by a scenario step."
	default:
		return "Validated only when the step directly proves the intended security property rather than merely gathering context."
	}
}

func classifyFromRawToolOutput(toolName string, output map[string]any) (validation.StepResult, validation.FailureReason) {
	if output == nil {
		return validation.StepResult(""), validation.FailureReason("")
	}

	if v, ok := output["result"].(string); ok {
		result := validation.StepResult(v)
		if result == validation.StepFailed {
			return result, rawFailureReason(output)
		}
		return result, validation.FailureReason("")
	}
	if v, ok := output["status"].(string); ok {
		result := validation.StepResult(v)
		if result == validation.StepFailed {
			return result, rawFailureReason(output)
		}
		return result, validation.FailureReason("")
	}

	if strings.HasPrefix(toolName, "discovery.") {
		return validation.StepResult(""), validation.FailureReason("")
	}

	switch toolName {
	case "validation.check_permissions":
		if allowed, ok := output["allowed"].(bool); ok {
			if allowed {
				return validation.StepValidated, validation.FailureReason("")
			}
			return validation.StepFailed, validation.FailureRBACDenied
		}
	case "validation.probe_network":
		if reachable, ok := output["reachable"].(bool); ok {
			if reachable {
				return validation.StepValidated, validation.FailureReason("")
			}
			return validation.StepFailed, validation.FailureNetworkUnreachable
		}
	case "validation.read_secret", "validation.check_token":
		if reason := rawFailureReason(output); reason != "" {
			return validation.StepFailed, reason
		}
		if _, ok := output["name"].(string); ok {
			return validation.StepValidated, validation.FailureReason("")
		}
	}

	return validation.StepResult(""), validation.FailureReason("")
}

func rawFailureReason(output map[string]any) validation.FailureReason {
	if v, ok := output["failure_reason"].(string); ok {
		return validation.FailureReason(v)
	}
	if v, ok := output["reason"].(string); ok {
		return validation.FailureReason(v)
	}
	return validation.FailureReason("")
}

func applyEvaluation(execResult *toolExecutionResult, rawFailure validation.FailureReason, evaluation stepOutcomeEvaluation) {
	execResult.EvaluatorClassification = string(evaluation.Classification)
	execResult.EvaluatorRationale = evaluation.Rationale
	execResult.EvaluatorUsage = evaluation.Usage

	switch evaluation.Classification {
	case stepOutcomeValidated:
		execResult.Outcome = validation.StepValidated
		execResult.FailureReason = validation.FailureReason("")
	case stepOutcomeTheoretical:
		execResult.Outcome = validation.StepTheoretical
		execResult.FailureReason = validation.FailureReason("")
	case stepOutcomeFailedRBAC:
		execResult.Outcome = validation.StepFailed
		if rawFailure != "" {
			execResult.FailureReason = rawFailure
		} else {
			execResult.FailureReason = validation.FailureRBACDenied
		}
	case stepOutcomeFailedNetwork:
		execResult.Outcome = validation.StepFailed
		if rawFailure != "" {
			execResult.FailureReason = rawFailure
		} else {
			execResult.FailureReason = validation.FailureNetworkUnreachable
		}
	case stepOutcomeFailedPrereq:
		execResult.Outcome = validation.StepFailed
		if rawFailure != "" {
			execResult.FailureReason = rawFailure
		} else {
			execResult.FailureReason = validation.FailureMissingPrerequisite
		}
	}
}

func edgeStatusFromExecutionResult(result toolExecutionResult) graph.EdgeStatus {
	switch result.Outcome {
	case validation.StepValidated:
		return graph.EdgeValidated
	case validation.StepTheoretical:
		return graph.EdgeTheoretical
	case validation.StepFailed:
		if result.FailureReason == validation.FailureRBACDenied {
			return graph.EdgeFailedRBAC
		}
		return graph.EdgeFailed
	default:
		return ""
	}
}
