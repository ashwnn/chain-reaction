package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/ashwnn/chain-reaction/internal/config"
	"github.com/ashwnn/chain-reaction/internal/evidence"
	"github.com/ashwnn/chain-reaction/internal/graph"
	"github.com/ashwnn/chain-reaction/internal/guardrails"
	"github.com/ashwnn/chain-reaction/internal/tools"
	"github.com/ashwnn/chain-reaction/internal/tools/validation"
)

type fakeStepOutcomeEvaluator struct {
	results []stepOutcomeEvaluation
	err     error
	calls   []stepOutcomeEvaluationInput
}

func (f *fakeStepOutcomeEvaluator) Evaluate(_ context.Context, input stepOutcomeEvaluationInput) (stepOutcomeEvaluation, error) {
	f.calls = append(f.calls, input)
	if f.err != nil {
		return stepOutcomeEvaluation{}, f.err
	}
	if len(f.results) == 0 {
		return stepOutcomeEvaluation{}, fmt.Errorf("no evaluator result scripted")
	}
	result := f.results[0]
	f.results = f.results[1:]
	return result, nil
}

func TestNewStepOutcomeEvaluatorReturnsNilWhenEvaluatorDisabled(t *testing.T) {
	// Even with valid LLM credentials, the evaluator is nil when StepOutcomeEvaluator
	// is false (cost-sensitive opt-in per ).
	evaluator := newStepOutcomeEvaluator(config.Config{
		LLMProvider:          "openai",
		LLMAPIKey:            "test-key",
		LLMModel:             "gpt-5.4-mini",
		StepOutcomeEvaluator: false,
	})
	if evaluator != nil {
		t.Fatal("expected nil evaluator when StepOutcomeEvaluator is false")
	}
}

func TestNewStepOutcomeEvaluatorReturnsNilWhenMissingCredentials(t *testing.T) {
	// Without credentials the evaluator is nil regardless of the flag.
	evaluator := newStepOutcomeEvaluator(config.Config{
		LLMProvider:          "openai",
		LLMAPIKey:            "",
		LLMModel:             "gpt-5.4-mini",
		StepOutcomeEvaluator: true,
	})
	if evaluator != nil {
		t.Fatal("expected nil evaluator when LLMAPIKey is empty")
	}

	evaluator = newStepOutcomeEvaluator(config.Config{
		LLMProvider:          "openai",
		LLMAPIKey:            "test-key",
		LLMModel:             "",
		StepOutcomeEvaluator: true,
	})
	if evaluator != nil {
		t.Fatal("expected nil evaluator when LLMModel is empty")
	}
}

func TestParsestepOutcomeEvaluation(t *testing.T) {
	evaluation, err := parsestepOutcomeEvaluation(`{"classification":"failed_network","rationale":"tcp connect timed out"}`)
	if err != nil {
		t.Fatalf("parsestepOutcomeEvaluation returned error: %v", err)
	}
	if evaluation.Classification != stepOutcomeFailedNetwork {
		t.Fatalf("expected failed_network, got %q", evaluation.Classification)
	}
	if evaluation.Rationale != "tcp connect timed out" {
		t.Fatalf("unexpected rationale %q", evaluation.Rationale)
	}
}

func TestParsestepOutcomeEvaluationRejectsUnknownClassification(t *testing.T) {
	if _, err := parsestepOutcomeEvaluation(`{"classification":"mystery","rationale":"nope"}`); err == nil {
		t.Fatal("expected unknown classification error")
	}
}

func TestNewStepOutcomeEvaluatorOpenAIDoesNotSendMaxTokens(t *testing.T) {
	var captured map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"role":    "assistant",
						"content": `{"classification":"validated","rationale":"raw output proved the step succeeded"}`,
					},
				},
			},
		})
	}))
	t.Cleanup(srv.Close)

	evaluator := newStepOutcomeEvaluator(config.Config{
		LLMProvider:          "openai",
		LLMAPIKey:            "test-key",
		LLMModel:             "gpt-5.4-mini",
		LLMBaseURL:           srv.URL,
		StepOutcomeEvaluator: true,
	})
	if evaluator == nil {
		t.Fatal("expected evaluator to be configured")
	}

	evaluation, err := evaluator.Evaluate(context.Background(), stepOutcomeEvaluationInput{
		ToolName:         "validation.check_token",
		Parameters:       map[string]any{"namespace": "chain-reaction"},
		Output:           map[string]any{"status": "validated"},
		CurrentGoal:      "validate current pod identity",
		CandidateStepIDs: []string{"KG-001-S1"},
	})
	if err != nil {
		t.Fatalf("Evaluate returned error: %v", err)
	}
	if evaluation.Classification != stepOutcomeValidated {
		t.Fatalf("expected validated classification, got %q", evaluation.Classification)
	}
	if _, ok := captured["max_tokens"]; ok {
		t.Fatalf("expected openai evaluator request to omit max_tokens, got %#v", captured["max_tokens"])
	}
}

func TestValidationLoopWithEvaluatorRecordsClassificationAndUsesItForGraph(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.Config{
		OutputPath: tmpDir,
		TimeBudget: time.Minute,
		MaxSteps:   3,
	}

	evidenceDir := filepath.Join(cfg.OutputPath, "evidence")
	collector, err := evidence.NewCollector(evidenceDir)
	if err != nil {
		t.Fatalf("create collector: %v", err)
	}
	t.Cleanup(func() {
		_ = collector.Close()
	})

	registry := tools.NewRegistry()
	if err := registry.Register(fakeTool{
		name: "discovery.list_namespaces",
		run: func(_ context.Context, _ map[string]any) (map[string]any, error) {
			return map[string]any{"count": 2, "namespaces": []string{"default", "big-monolith"}}, nil
		},
	}); err != nil {
		t.Fatalf("register discovery tool: %v", err)
	}
	if err := registry.Register(fakeTool{
		name: "validation.check_permissions",
		run: func(_ context.Context, _ map[string]any) (map[string]any, error) {
			return map[string]any{
				"allowed":   false,
				"denied":    true,
				"reason":    string(validation.FailureRBACDenied),
				"verb":      "get",
				"resource":  "secrets",
				"namespace": "default",
			}, nil
		},
		schema: &tools.Schema{
			Type: "object",
			Properties: map[string]tools.Schema{
				"namespace": {Type: "string"},
			},
		},
	}); err != nil {
		t.Fatalf("register permission tool: %v", err)
	}

	planner := &stubValidationPlanner{
		actions: []plannerAction{
			{ActionType: actionTypeExecute, ToolName: "discovery.list_namespaces"},
			{ActionType: actionTypeExecute, ToolName: "validation.check_permissions", Parameters: map[string]any{"namespace": "default"}},
			{ActionType: actionTypeFinalAnswer, FinalAnswer: "done"},
		},
	}

	evaluator := &fakeStepOutcomeEvaluator{
		results: []stepOutcomeEvaluation{
			{Classification: stepOutcomeTheoretical, Rationale: "enumeration only"},
			{Classification: stepOutcomeFailedRBAC, Rationale: "subject lacks secret-read rights"},
		},
	}

	result, err := runValidationLoopWithEvaluator(
		context.Background(),
		cfg,
		time.Now().UTC(),
		guardrails.New(nil, cfg.QPS, cfg.Burst),
		collector,
		evidence.NewSnapshotWriter(evidenceDir),
		registry,
		planner,
		evaluator,
		nil,
	)
	if err != nil {
		t.Fatalf("runValidationLoopWithEvaluator returned error: %v", err)
	}

	if len(evaluator.calls) != 2 {
		t.Fatalf("expected evaluator to be called twice, got %d", len(evaluator.calls))
	}
	if evaluator.calls[0].ToolName != "discovery.list_namespaces" || evaluator.calls[1].ToolName != "validation.check_permissions" {
		t.Fatalf("unexpected evaluator call order: %#v", evaluator.calls)
	}

	if len(result.Trace) != 2 {
		t.Fatalf("expected 2 trace entries, got %d", len(result.Trace))
	}
	if result.Trace[0].Outcome != validation.StepTheoretical {
		t.Fatalf("expected discovery outcome theoretical, got %q", result.Trace[0].Outcome)
	}
	if result.Trace[0].EvaluatorClassification != string(stepOutcomeTheoretical) {
		t.Fatalf("expected discovery evaluator classification theoretical, got %q", result.Trace[0].EvaluatorClassification)
	}
	if result.Trace[1].Outcome != validation.StepFailed || result.Trace[1].FailureReason != validation.FailureRBACDenied {
		t.Fatalf("expected permission outcome failed/rbac_denied, got %q / %q", result.Trace[1].Outcome, result.Trace[1].FailureReason)
	}

	records := readEvidenceRecords(t, result.EvidencePath)
	if len(records) < 2 {
		t.Fatalf("expected at least 2 evidence records, got %d", len(records))
	}
	stepRecord := records[0]
	if stepRecord.Data["evaluator_classification"] != "theoretical" {
		t.Fatalf("expected evaluator classification on first evidence record, got %#v", stepRecord.Data["evaluator_classification"])
	}
	if stepRecord.Data["evaluator_rationale"] != "enumeration only" {
		t.Fatalf("unexpected evaluator rationale %#v", stepRecord.Data["evaluator_rationale"])
	}

	ag := readAttackGraph(t, result.GraphPath)
	if len(ag.Edges) != 2 {
		t.Fatalf("expected 2 graph edges, got %d", len(ag.Edges))
	}
	if ag.Edges[0].Status != graph.EdgeTheoretical {
		t.Fatalf("expected first edge theoretical, got %q", ag.Edges[0].Status)
	}
	if ag.Edges[1].Status != graph.EdgeFailedRBAC {
		t.Fatalf("expected second edge failed_rbac, got %q", ag.Edges[1].Status)
	}
}

func TestValidationLoopWithEvaluatorFallsBackToRawTaxonomyOnEvaluatorError(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.Config{
		OutputPath: tmpDir,
		TimeBudget: time.Minute,
		MaxSteps:   2,
	}

	evidenceDir := filepath.Join(cfg.OutputPath, "evidence")
	collector, err := evidence.NewCollector(evidenceDir)
	if err != nil {
		t.Fatalf("create collector: %v", err)
	}
	t.Cleanup(func() {
		_ = collector.Close()
	})

	registry := tools.NewRegistry()
	if err := registry.Register(fakeTool{
		name: "validation.check_permissions",
		run: func(_ context.Context, _ map[string]any) (map[string]any, error) {
			return map[string]any{
				"allowed":   false,
				"denied":    true,
				"reason":    string(validation.FailureRBACDenied),
				"verb":      "get",
				"resource":  "secrets",
				"namespace": "default",
			}, nil
		},
	}); err != nil {
		t.Fatalf("register tool: %v", err)
	}

	planner := &stubValidationPlanner{
		actions: []plannerAction{
			{ActionType: actionTypeExecute, ToolName: "validation.check_permissions"},
			{ActionType: actionTypeFinalAnswer, FinalAnswer: "done"},
		},
	}

	evaluator := &fakeStepOutcomeEvaluator{err: fmt.Errorf("evaluator unavailable")}

	result, err := runValidationLoopWithEvaluator(
		context.Background(),
		cfg,
		time.Now().UTC(),
		guardrails.New(nil, cfg.QPS, cfg.Burst),
		collector,
		evidence.NewSnapshotWriter(evidenceDir),
		registry,
		planner,
		evaluator,
		nil,
	)
	if err != nil {
		t.Fatalf("runValidationLoopWithEvaluator returned error: %v", err)
	}

	if len(result.Trace) != 1 {
		t.Fatalf("expected 1 trace entry, got %d", len(result.Trace))
	}
	if result.Trace[0].Outcome != validation.StepFailed || result.Trace[0].FailureReason != validation.FailureRBACDenied {
		t.Fatalf("expected raw failed/rbac_denied fallback, got %q / %q", result.Trace[0].Outcome, result.Trace[0].FailureReason)
	}
	if result.Trace[0].EvaluatorError == "" {
		t.Fatal("expected evaluator_error to be recorded")
	}
}
