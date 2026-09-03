package metrics

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ashwnn/chain-reaction/internal/baseline"
	"github.com/ashwnn/chain-reaction/internal/graph"
	"github.com/ashwnn/chain-reaction/internal/llm"
)

func TestComputeValidationMetrics(t *testing.T) {
	now := time.Now().UTC()
	result := ComputeValidationMetrics(ValidationMetricsInput{
		PlannerType:       "react_llm",
		StartedAt:         now.Add(-5 * time.Second),
		FinishedAt:        now,
		Steps:             3,
		TerminationReason: "goal_achieved",
		ToolsUsed:         []string{"validation.check_permissions"},
		NamespacesTouched: []string{"lab"},
		SnapshotCount:     2,
		GraphNodes: []graph.Node{
			{ID: "pod:current", Phase: "foothold", Kind: "pod"},
			{ID: "n1", Phase: "validation", Kind: "api_call"},
		},
		GraphEdges: []graph.Edge{
			{From: "pod:current", To: "n1", Status: graph.EdgeValidated},
		},
		FinalAnswerPresent: true,
		GraphPath:          "/tmp/test/graph/attack-graph.json",
		EvidencePath:       "/tmp/test/evidence",
		OutputDir:          "/tmp/test",
	})

	if result.ContractVersion != "validation.metrics.v4" {
		t.Errorf("expected contract version validation.metrics.v4, got %q", result.ContractVersion)
	}
	if result.PlannerType != "react_llm" {
		t.Errorf("expected planner_type react_llm, got %q", result.PlannerType)
	}
	if result.DurationMS != 5000 {
		t.Errorf("expected duration_ms 5000, got %d", result.DurationMS)
	}
	if result.Termination.Reason != "goal_achieved" {
		t.Errorf("expected termination reason goal_achieved, got %q", result.Termination.Reason)
	}
	if result.Termination.Steps != 3 {
		t.Errorf("expected termination steps 3, got %d", result.Termination.Steps)
	}
	if len(result.Coverage.ToolsUsed) != 1 || result.Coverage.ToolsUsed[0] != "validation.check_permissions" {
		t.Errorf("expected tools_used [validation.check_permissions], got %v", result.Coverage.ToolsUsed)
	}
	if len(result.Coverage.NamespacesTouched) != 1 || result.Coverage.NamespacesTouched[0] != "lab" {
		t.Errorf("expected namespaces_touched [lab], got %v", result.Coverage.NamespacesTouched)
	}
	if result.Coverage.SnapshotCount != 2 {
		t.Errorf("expected snapshot_count 2, got %d", result.Coverage.SnapshotCount)
	}
	if result.GraphSummary.TotalNodes != 2 {
		t.Errorf("expected total_nodes 2, got %d", result.GraphSummary.TotalNodes)
	}
	if result.GraphSummary.TotalEdges != 1 {
		t.Errorf("expected total_edges 1, got %d", result.GraphSummary.TotalEdges)
	}
	if result.GraphSummary.EdgesByStatus["validated"] != 1 {
		t.Errorf("expected edges_by_status.validated 1, got %d", result.GraphSummary.EdgesByStatus["validated"])
	}
	if !result.FinalAnswerPresent {
		t.Error("expected final_answer_present true")
	}
	if result.ScenarioCoverage != nil {
		t.Errorf("expected scenario_coverage nil, got %v", result.ScenarioCoverage)
	}
	if result.TimeToChain != nil {
		t.Errorf("expected time_to_chain nil, got %v", result.TimeToChain)
	}
	if result.Guardrails.ActionBlocks != 0 {
		t.Errorf("expected guardrails.action_blocks 0, got %d", result.Guardrails.ActionBlocks)
	}
	// Edge validation rate: 1 edge, 1 validated → rate 1.0
	if result.EdgeValidationRate == nil {
		t.Fatal("expected edge_validation_rate non-nil with 1 edge")
	}
	if result.EdgeValidationRate.ValidatedEdges != 1 {
		t.Errorf("expected validated_edges 1, got %d", result.EdgeValidationRate.ValidatedEdges)
	}
	if result.EdgeValidationRate.TotalEdges != 1 {
		t.Errorf("expected total_edges 1, got %d", result.EdgeValidationRate.TotalEdges)
	}
	if result.EdgeValidationRate.Rate == nil || *result.EdgeValidationRate.Rate != 1.0 {
		t.Errorf("expected rate 1.0, got %v", result.EdgeValidationRate.Rate)
	}
	wantMetrics := filepath.Join("/tmp/test", "validation-metrics.json")
	if result.Artifacts.MetricsPath != wantMetrics {
		t.Errorf("expected metrics_path %q, got %q", wantMetrics, result.Artifacts.MetricsPath)
	}
	if result.Artifacts.GraphPath != "/tmp/test/graph/attack-graph.json" {
		t.Errorf("expected graph_path /tmp/test/graph/attack-graph.json, got %q", result.Artifacts.GraphPath)
	}
}

func TestComputeValidationMetricsEmptyGraph(t *testing.T) {
	now := time.Now().UTC()
	result := ComputeValidationMetrics(ValidationMetricsInput{
		PlannerType:        "deterministic_skeleton",
		StartedAt:          now,
		FinishedAt:         now,
		Steps:              0,
		TerminationReason:  "timeout",
		ToolsUsed:          nil,
		NamespacesTouched:  nil,
		SnapshotCount:      0,
		GraphNodes:         nil,
		GraphEdges:         nil,
		FinalAnswerPresent: false,
		GraphPath:          "/out/graph/attack-graph.json",
		EvidencePath:       "/out/evidence",
		OutputDir:          "/out",
	})

	if result.GraphSummary.TotalNodes != 0 {
		t.Errorf("expected total_nodes 0, got %d", result.GraphSummary.TotalNodes)
	}
	if result.GraphSummary.TotalEdges != 0 {
		t.Errorf("expected total_edges 0, got %d", result.GraphSummary.TotalEdges)
	}
	if len(result.GraphSummary.EdgesByStatus) != 0 {
		t.Errorf("expected empty edges_by_status, got %v", result.GraphSummary.EdgesByStatus)
	}
	if result.Coverage.ToolsUsed != nil {
		t.Errorf("expected nil tools_used, got %v", result.Coverage.ToolsUsed)
	}
	if result.ScenarioCoverage != nil {
		t.Errorf("expected scenario_coverage nil, got %v", result.ScenarioCoverage)
	}
	if result.TimeToChain != nil {
		t.Errorf("expected time_to_chain nil, got %v", result.TimeToChain)
	}
	if result.EdgeValidationRate != nil {
		t.Errorf("expected edge_validation_rate nil for empty graph, got %+v", result.EdgeValidationRate)
	}
	if result.AttackTypeCoverage != nil {
		t.Errorf("expected attack_type_coverage nil for empty graph, got %+v", result.AttackTypeCoverage)
	}
	if result.Guardrails.ActionBlocks != 0 {
		t.Errorf("expected guardrails.action_blocks 0 for empty run, got %d", result.Guardrails.ActionBlocks)
	}
}

func TestComputeValidationMetricsMixedEdgeStatuses(t *testing.T) {
	now := time.Now().UTC()
	result := ComputeValidationMetrics(ValidationMetricsInput{
		PlannerType:       "react_llm",
		StartedAt:         now,
		FinishedAt:        now,
		Steps:             4,
		TerminationReason: "goal_achieved",
		ToolsUsed:         []string{"validation.check_permissions", "validation.read_secret"},
		NamespacesTouched: []string{"default", "lab"},
		SnapshotCount:     4,
		GraphEdges: []graph.Edge{
			{Status: graph.EdgeValidated},
			{Status: graph.EdgeValidated},
			{Status: graph.EdgeFailedRBAC},
			{Status: graph.EdgeFailed},
		},
		GraphNodes: []graph.Node{
			{ID: "pod:current"},
			{ID: "n1"},
			{ID: "n2"},
			{ID: "n3"},
			{ID: "n4"},
		},
		FinalAnswerPresent: true,
		GraphPath:          "/tmp/g.json",
		EvidencePath:       "/tmp/e",
		OutputDir:          "/tmp",
	})

	if result.GraphSummary.EdgesByStatus["validated"] != 2 {
		t.Errorf("expected validated=2, got %d", result.GraphSummary.EdgesByStatus["validated"])
	}
	if result.GraphSummary.EdgesByStatus["failed_rbac"] != 1 {
		t.Errorf("expected failed_rbac=1, got %d", result.GraphSummary.EdgesByStatus["failed_rbac"])
	}
	if result.GraphSummary.EdgesByStatus["failed"] != 1 {
		t.Errorf("expected failed=1, got %d", result.GraphSummary.EdgesByStatus["failed"])
	}
	if result.GraphSummary.EdgesByStatus["theoretical"] != 0 {
		t.Errorf("expected theoretical=0 (absent), got %d", result.GraphSummary.EdgesByStatus["theoretical"])
	}
	// Edge validation rate: 2 validated out of 4 edges → rate 0.5
	if result.EdgeValidationRate == nil {
		t.Fatal("expected edge_validation_rate non-nil with 4 edges")
	}
	if result.EdgeValidationRate.ValidatedEdges != 2 {
		t.Errorf("expected validated_edges 2, got %d", result.EdgeValidationRate.ValidatedEdges)
	}
	if result.EdgeValidationRate.TotalEdges != 4 {
		t.Errorf("expected total_edges 4, got %d", result.EdgeValidationRate.TotalEdges)
	}
	if result.EdgeValidationRate.Rate == nil || *result.EdgeValidationRate.Rate != 0.5 {
		t.Errorf("expected rate 0.5, got %v", result.EdgeValidationRate.Rate)
	}
}

func TestPlannerTypeFromConfig(t *testing.T) {
	tests := []struct {
		name   string
		apiKey string
		model  string
		want   string
	}{
		{"both set", "sk-123", "gpt-4", "react_llm"},
		{"api key only", "sk-123", "", "deterministic_skeleton"},
		{"model only", "", "gpt-4", "deterministic_skeleton"},
		{"neither set", "", "", "deterministic_skeleton"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := PlannerTypeFromConfig(tc.apiKey, tc.model)
			if got != tc.want {
				t.Errorf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestWriteValidationMetricsSummary(t *testing.T) {
	tmpDir := t.TempDir()

	input := ValidationMetricsInput{
		PlannerType:       "deterministic_skeleton",
		StartedAt:         time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC),
		FinishedAt:        time.Date(2026, 4, 2, 12, 0, 5, 0, time.UTC),
		Steps:             3,
		TerminationReason: "goal_achieved",
		ToolsUsed:         []string{"discovery.list_namespaces"},
		NamespacesTouched: []string{"default"},
		SnapshotCount:     5,
		GraphNodes: []graph.Node{
			{ID: "pod:current", Phase: "foothold", Kind: "pod"},
			{ID: "n1", Phase: "validation", Kind: "api_call"},
			{ID: "n2", Phase: "validation", Kind: "api_call"},
		},
		GraphEdges: []graph.Edge{
			{From: "pod:current", To: "n1", Status: graph.EdgeValidated},
			{From: "pod:current", To: "n2", Status: graph.EdgeFailedRBAC},
		},
		FinalAnswerPresent: true,
		GraphPath:          "g.json",
		EvidencePath:       "evidence",
		OutputDir:          tmpDir,
	}

	metrics := ComputeValidationMetrics(input)
	path, err := WriteValidationMetricsSummary(metrics)
	if err != nil {
		t.Fatalf("WriteValidationMetricsSummary returned error: %v", err)
	}

	expectedPath := filepath.Join(tmpDir, "validation-metrics.json")
	if path != expectedPath {
		t.Fatalf("expected path %q, got %q", expectedPath, path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read metrics file: %v", err)
	}

	var parsed ValidationMetrics
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal metrics: %v", err)
	}

	// Verify round-trip fidelity for key fields.
	if parsed.ContractVersion != "validation.metrics.v4" {
		t.Errorf("expected contract_version %q, got %q", "validation.metrics.v4", parsed.ContractVersion)
	}
	if parsed.DurationMS != 5000 {
		t.Errorf("expected duration_ms 5000, got %d", parsed.DurationMS)
	}
	if parsed.Termination.Steps != 3 {
		t.Errorf("expected termination.steps 3, got %d", parsed.Termination.Steps)
	}
	if parsed.GraphSummary.EdgesByStatus["validated"] != 1 {
		t.Errorf("expected edges_by_status.validated 1, got %d", parsed.GraphSummary.EdgesByStatus["validated"])
	}
	if parsed.GraphSummary.EdgesByStatus["failed_rbac"] != 1 {
		t.Errorf("expected edges_by_status.failed_rbac 1, got %d", parsed.GraphSummary.EdgesByStatus["failed_rbac"])
	}
	if parsed.ScenarioCoverage != nil {
		t.Errorf("expected scenario_coverage null, got %v", parsed.ScenarioCoverage)
	}
	if parsed.Artifacts.MetricsPath != expectedPath {
		t.Errorf("expected artifacts.metrics_path %q, got %q", expectedPath, parsed.Artifacts.MetricsPath)
	}
}

func TestComputeValidationMetricsGuardrailBlocks(t *testing.T) {
	now := time.Now().UTC()

	t.Run("non-zero guardrail blocks", func(t *testing.T) {
		result := ComputeValidationMetrics(ValidationMetricsInput{
			PlannerType:        "react_llm",
			StartedAt:          now,
			FinishedAt:         now,
			Steps:              5,
			TerminationReason:  "goal_achieved",
			ToolsUsed:          []string{"validation.read_secret"},
			NamespacesTouched:  []string{"lab"},
			SnapshotCount:      3,
			GraphNodes:         []graph.Node{{ID: "pod:current"}},
			GraphEdges:         nil,
			FinalAnswerPresent: true,
			GraphPath:          "/tmp/g.json",
			EvidencePath:       "/tmp/e",
			OutputDir:          "/tmp",
			GuardrailBlocks:    2,
		})

		if result.Guardrails.ActionBlocks != 2 {
			t.Errorf("expected guardrails.action_blocks 2, got %d", result.Guardrails.ActionBlocks)
		}
	})

	t.Run("zero guardrail blocks", func(t *testing.T) {
		result := ComputeValidationMetrics(ValidationMetricsInput{
			PlannerType:        "react_llm",
			StartedAt:          now,
			FinishedAt:         now,
			Steps:              3,
			TerminationReason:  "goal_achieved",
			ToolsUsed:          []string{"validation.check_permissions"},
			NamespacesTouched:  nil,
			SnapshotCount:      0,
			GraphNodes:         nil,
			GraphEdges:         nil,
			FinalAnswerPresent: true,
			GraphPath:          "/tmp/g.json",
			EvidencePath:       "/tmp/e",
			OutputDir:          "/tmp",
			GuardrailBlocks:    0,
		})

		if result.Guardrails.ActionBlocks != 0 {
			t.Errorf("expected guardrails.action_blocks 0, got %d", result.Guardrails.ActionBlocks)
		}
	})
}

func TestComputeValidationMetricsWithScenarioResult(t *testing.T) {
	now := time.Now().UTC()

	// Build a matcher output that simulates 1 chain validated (KG-002).
	scenarioRate := 0.2
	catalogCoverage := 0.6
	attemptedSuccess := 0.5
	ttc := 200 * time.Millisecond

	matcherOutput := &baseline.MatcherOutput{
		Families: []baseline.ChainResult{
			{
				FamilyID:       "KG-001",
				ChainValidated: false,
				TotalSteps:     3,
				ValidatedSteps: 1,
			},
			{
				FamilyID:       "KG-002",
				ChainValidated: true,
				TotalSteps:     2,
				ValidatedSteps: 2,
			},
		},
		ValidatedChainCount:      1,
		TotalFamilies:            5,
		ScenarioRate:             &scenarioRate,
		ValidatedSteps:           3,
		TotalSteps:               5,
		CatalogStepCoverage:      &catalogCoverage,
		AttemptedSteps:           6,
		AttemptedStepSuccessRate: &attemptedSuccess,
		TimeToFirstChain:         &ttc,
	}

	result := ComputeValidationMetrics(ValidationMetricsInput{
		PlannerType:        "react_llm",
		StartedAt:          now.Add(-5 * time.Second),
		FinishedAt:         now,
		Steps:              4,
		TerminationReason:  "goal_achieved",
		ToolsUsed:          []string{"validation.check_permissions", "validation.read_secret"},
		NamespacesTouched:  []string{"lab"},
		SnapshotCount:      2,
		GraphNodes:         []graph.Node{{ID: "pod:current"}},
		GraphEdges:         nil,
		FinalAnswerPresent: true,
		GraphPath:          "/tmp/g.json",
		EvidencePath:       "/tmp/e",
		OutputDir:          "/tmp",
		ScenarioResult:     matcherOutput,
	})

	// ScenarioCoverage should be populated.
	if result.ScenarioCoverage == nil {
		t.Fatal("expected scenario_coverage non-nil when ScenarioResult provided")
	}
	sc := result.ScenarioCoverage
	if sc.ValidatedChainCount != 1 {
		t.Errorf("ValidatedChainCount: got %d, want 1", sc.ValidatedChainCount)
	}
	if sc.TotalFamilies != 5 {
		t.Errorf("TotalFamilies: got %d, want 5", sc.TotalFamilies)
	}
	if sc.ScenarioRate == nil || *sc.ScenarioRate != 0.2 {
		t.Errorf("ScenarioRate: got %v, want 0.2", sc.ScenarioRate)
	}
	if sc.CatalogStepCoverage == nil || *sc.CatalogStepCoverage != 0.6 {
		t.Errorf("CatalogStepCoverage: got %v, want 0.6", sc.CatalogStepCoverage)
	}
	if sc.ValidatedSteps != 3 {
		t.Errorf("ValidatedSteps: got %d, want 3", sc.ValidatedSteps)
	}
	if sc.TotalSteps != 5 {
		t.Errorf("TotalSteps: got %d, want 5", sc.TotalSteps)
	}
	if sc.AttemptedSteps != 6 {
		t.Errorf("AttemptedSteps: got %d, want 6", sc.AttemptedSteps)
	}
	if sc.AttemptedStepSuccessRate == nil || *sc.AttemptedStepSuccessRate != 0.5 {
		t.Errorf("AttemptedStepSuccessRate: got %v, want 0.5", sc.AttemptedStepSuccessRate)
	}
	if len(sc.Families) != 2 {
		t.Fatalf("Families: got %d entries, want 2", len(sc.Families))
	}
	if sc.Families[0].FamilyID != "KG-001" {
		t.Errorf("Families[0].FamilyID: got %q, want KG-001", sc.Families[0].FamilyID)
	}
	if sc.Families[0].ChainValidated {
		t.Error("Families[0].ChainValidated: got true, want false")
	}
	if !sc.Families[1].ChainValidated {
		t.Error("Families[1].ChainValidated: got false, want true")
	}

	// TimeToChain should be populated with the matcher's TTC.
	if result.TimeToChain == nil {
		t.Fatal("expected time_to_chain non-nil when TTC available")
	}
	if result.TimeToChain.DurationMS != 200 {
		t.Errorf("TimeToChain.DurationMS: got %d, want 200", result.TimeToChain.DurationMS)
	}
}

func TestComputeValidationMetricsNilScenarioResult(t *testing.T) {
	now := time.Now().UTC()
	result := ComputeValidationMetrics(ValidationMetricsInput{
		PlannerType:       "react_llm",
		StartedAt:         now,
		FinishedAt:        now,
		Steps:             1,
		TerminationReason: "goal_achieved",
		GraphPath:         "/tmp/g.json",
		EvidencePath:      "/tmp/e",
		OutputDir:         "/tmp",
		ScenarioResult:    nil,
	})

	if result.ScenarioCoverage != nil {
		t.Errorf("expected scenario_coverage nil when ScenarioResult nil, got %v", result.ScenarioCoverage)
	}
	if result.TimeToChain != nil {
		t.Errorf("expected time_to_chain nil when ScenarioResult nil, got %v", result.TimeToChain)
	}
}

func TestNewMetricsLLMUsageCacheEfficiency(t *testing.T) {
	t.Run("cache efficiency computed when cache reads present", func(t *testing.T) {
		records := []*llm.UsageMetadata{
			{InputTokens: 100, OutputTokens: 20, TotalTokens: 120, CacheRead: 80},
			{InputTokens: 100, OutputTokens: 15, TotalTokens: 115, CacheRead: 80},
		}
		m := NewMetricsLLMUsage(records)
		if m.InputTokens != 200 {
			t.Errorf("InputTokens: got %d, want 200", m.InputTokens)
		}
		if m.CacheReadTokens != 160 {
			t.Errorf("CacheReadTokens: got %d, want 160", m.CacheReadTokens)
		}
		if m.CacheEfficiency == nil {
			t.Fatal("CacheEfficiency: got nil, want non-nil")
		}
		wantRatio := 160.0 / 200.0
		if *m.CacheEfficiency != wantRatio {
			t.Errorf("CacheEfficiency: got %v, want %v", *m.CacheEfficiency, wantRatio)
		}
	})

	t.Run("cache efficiency nil when no cache reads", func(t *testing.T) {
		records := []*llm.UsageMetadata{
			{InputTokens: 100, OutputTokens: 20, TotalTokens: 120, CacheRead: 0},
		}
		m := NewMetricsLLMUsage(records)
		if m.CacheEfficiency != nil {
			t.Errorf("CacheEfficiency: got %v, want nil when cache reads are 0", *m.CacheEfficiency)
		}
	})

	t.Run("cache efficiency nil when no input tokens", func(t *testing.T) {
		records := []*llm.UsageMetadata{
			{InputTokens: 0, OutputTokens: 20, TotalTokens: 20, CacheRead: 0},
		}
		m := NewMetricsLLMUsage(records)
		if m.CacheEfficiency != nil {
			t.Errorf("CacheEfficiency: got %v, want nil when input tokens are 0", *m.CacheEfficiency)
		}
	})

	t.Run("cache efficiency nil for empty records", func(t *testing.T) {
		m := NewMetricsLLMUsage(nil)
		if m.CacheEfficiency != nil {
			t.Errorf("CacheEfficiency: got %v, want nil for nil records", *m.CacheEfficiency)
		}
		m = NewMetricsLLMUsage([]*llm.UsageMetadata{})
		if m.CacheEfficiency != nil {
			t.Errorf("CacheEfficiency: got %v, want nil for empty records", *m.CacheEfficiency)
		}
	})
}

func TestComputeValidationMetricsWithCacheEfficiency(t *testing.T) {
	now := time.Now().UTC()
	records := []*llm.UsageMetadata{
		{InputTokens: 150, OutputTokens: 30, TotalTokens: 180, CacheRead: 120},
		{InputTokens: 150, OutputTokens: 25, TotalTokens: 175, CacheRead: 120},
	}
	input := ValidationMetricsInput{
		PlannerType:       "react_llm",
		StartedAt:         now,
		FinishedAt:        now,
		Steps:             2,
		TerminationReason: "goal_achieved",
		LLMUsageRecords:   records,
		GraphPath:         "/tmp/g.json",
		EvidencePath:      "/tmp/e",
		OutputDir:         "/tmp",
	}

	result := ComputeValidationMetrics(input)
	if result.LLMUsage == nil {
		t.Fatal("LLMUsage: got nil, want non-nil")
	}
	if result.LLMUsage.CacheReadTokens != 240 {
		t.Errorf("CacheReadTokens: got %d, want 240", result.LLMUsage.CacheReadTokens)
	}
	if result.LLMUsage.CacheEfficiency == nil {
		t.Fatal("CacheEfficiency: got nil, want non-nil")
	}
	// 240 cache reads / 300 total input = 0.8
	wantRatio := 240.0 / 300.0
	if *result.LLMUsage.CacheEfficiency != wantRatio {
		t.Errorf("CacheEfficiency: got %v, want %v", *result.LLMUsage.CacheEfficiency, wantRatio)
	}
}

func TestComputeValidationMetricsCacheEfficiencyNilWithoutLLM(t *testing.T) {
	now := time.Now().UTC()
	input := ValidationMetricsInput{
		PlannerType:       "deterministic_skeleton",
		StartedAt:         now,
		FinishedAt:        now,
		Steps:             0,
		TerminationReason: "timeout",
		LLMUsageRecords:   nil,
		GraphPath:         "/tmp/g.json",
		EvidencePath:      "/tmp/e",
		OutputDir:         "/tmp",
	}
	result := ComputeValidationMetrics(input)
	// LLMUsage is nil for deterministic skeleton
	if result.LLMUsage != nil {
		t.Errorf("LLMUsage: got %v, want nil for deterministic skeleton", result.LLMUsage)
	}
}
