// Package metrics provides stable, machine-readable metrics summary artifacts
// for Chain Reaction runtime paths. The schema uses existing truthful signals
// only — fields that cannot be computed honestly today are emitted as null.
package metrics

import "github.com/ashwnn/chain-reaction/internal/llm"

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ashwnn/chain-reaction/internal/baseline"
	"github.com/ashwnn/chain-reaction/internal/graph"
)

// ContractVersion is the stable schema identifier for validation metrics artifacts.
const contractVersion = "validation.metrics.v4"

// Planner type labels — kept as constants to avoid parallel vocabularies.
const (
	plannerTypeReactLLM              = "react_llm"
	plannerTypeDeterministicSkeleton = "deterministic_skeleton"
)

// ValidationMetrics is the stable machine-readable metrics summary emitted after
// a validation run. Fields that cannot be computed honestly are set to null.
type ValidationMetrics struct {
	ContractVersion     string                 `json:"contract_version"`
	PlannerType         string                 `json:"planner_type"`
	PlannerMode         string                 `json:"planner_mode"`
	ObservationContract string                 `json:"observation_contract"`
	PromptIntegrity     MetricsPromptIntegrity `json:"prompt_integrity"`
	ToolSetHash         string                 `json:"tool_set_hash"`
	PolicyHash          string                 `json:"policy_hash"`
	StartedAt           time.Time              `json:"started_at"`
	FinishedAt          time.Time              `json:"finished_at"`
	DurationMS          int64                  `json:"duration_ms"`
	Termination         MetricsTermination     `json:"termination"`
	Coverage            MetricsCoverage        `json:"coverage"`
	GraphSummary        MetricsGraphSummary    `json:"graph_summary"`
	FinalAnswerPresent  bool                   `json:"final_answer_present"`
	// EdgeValidationRate is the fraction of validation graph edges that are
	// validated. This is a graph-level statistic, not catalog-step coverage.
	// Null when no edges exist.
	EdgeValidationRate *MetricsEdgeValidationRate `json:"edge_validation_rate"`
	// AttackTypeCoverage measures breadth of attack surface coverage across
	// the validation tool taxonomy (edge types). This is a graph-level
	// statistic, not API call volume.
	// Null when no edges exist.
	AttackTypeCoverage *MetricsAttackTypeCoverage `json:"attack_type_coverage"`
	// ScenarioCoverage is null when no scenario matcher result is provided.
	// When non-nil, it contains the post-hoc scenario matching results computed
	// by the baseline step-chain matcher (internal/baseline). Denominator
	// contract: 5 in-scope families (KG-001..KG-005).
	// See built-in catalog for step-chain definitions.
	ScenarioCoverage *MetricsScenarioCoverage `json:"scenario_coverage"`
	// TimeToChain is null when no scenario matcher result is provided or when
	// no chains validated. When non-nil, it is the duration from run start to
	// the first fully-validated chain completion.
	TimeToChain *MetricsTimeToChain `json:"time_to_chain"`
	// LLMUsage aggregates token usage and estimated cost across all planner and
	// evaluator LLM calls during the run. Nil when no LLM calls were made
	// (deterministic skeleton mode). Estimated cost is null-safe — omitted when
	// no calls had known pricing in knownModelPricing.
	LLMUsage   *MetricsLLMUsage  `json:"llm_usage"`
	Guardrails MetricsGuardrails `json:"guardrails"`
	Artifacts  MetricsArtifacts  `json:"artifacts"`
}

type MetricsPromptIntegrity struct {
	Status string `json:"status"`
}
type MetricsTermination struct {
	Reason string `json:"reason"`
	Steps  int    `json:"steps"`
}

type MetricsCoverage struct {
	ToolsUsed         []string `json:"tools_used"`
	NamespacesTouched []string `json:"namespaces_touched"`
	SnapshotCount     int      `json:"snapshot_count"`
}

type MetricsGraphSummary struct {
	TotalNodes    int            `json:"total_nodes"`
	TotalEdges    int            `json:"total_edges"`
	EdgesByStatus map[string]int `json:"edges_by_status"`
}

type MetricsArtifacts struct {
	GraphPath    string `json:"graph_path"`
	EvidencePath string `json:"evidence_path"`
	MetricsPath  string `json:"metrics_path"`
}

// MetricsGuardrails captures guardrail enforcement statistics for a single run.
// These are per-run counts, not cross-run rates. Fields that cannot be computed
// honestly from available runtime signals are omitted until those signals exist.
//
// Namespace-allow-list violations that cause immediate run termination are NOT
// counted here — they prevent the metrics artifact from being written at all.
// ActionBlocks counts only the tool-level guardrail denials that are recorded
// in the trace (e.g., read_secret returning failed status with guardrail_blocked reason).
type MetricsGuardrails struct {
	// ActionBlocks is the count of tool invocations where a guardrail blocked
	// the action during this run. Zero means no guardrail blocks occurred.
	ActionBlocks int `json:"action_blocks"`
}

// MetricsEdgeValidationRate is the fraction of validation graph edges that
// are validated. Null when no edges exist (denominator is zero).
// This is a graph-level statistic, not catalog-step coverage.
type MetricsEdgeValidationRate struct {
	ValidatedEdges int      `json:"validated_edges"`
	TotalEdges     int      `json:"total_edges"`
	Rate           *float64 `json:"rate"`
}

// MetricsAttackTypeCoverage measures breadth of attack surface coverage across
// the validation tool taxonomy (edge types). This is a graph-level statistic,
// not API call volume. Each validation-relevant EdgeType (secret_access,
// permission_check, network_probe, token_review) is tracked for attempted
// and validated counts.
type MetricsAttackTypeCoverage struct {
	EdgeTypesValidated []string                          `json:"edge_types_validated"`
	EdgeTypesAttempted []string                          `json:"edge_types_attempted"`
	EdgeTypesAvailable int                               `json:"edge_types_available"`
	Breakdown          map[string]MetricsEdgeTypeSummary `json:"breakdown"`
}

// MetricsEdgeTypeSummary counts attempted and validated edges for one EdgeType.
type MetricsEdgeTypeSummary struct {
	Attempted int `json:"attempted"`
	Validated int `json:"validated"`
}

// MetricsScenarioCoverage contains post-hoc scenario matching results computed
// by the baseline step-chain matcher. When a scenario matcher result is provided
// to ComputeValidationMetrics, these fields are populated from the matcher output.
// When no result is provided, this field is nil (null in JSON).
type MetricsScenarioCoverage struct {
	ValidatedChainCount      int                  `json:"validated_chain_count"`
	TotalFamilies            int                  `json:"total_families"`
	ScenarioRate             *float64             `json:"scenario_rate"`
	CatalogStepCoverage      *float64             `json:"catalog_step_coverage"`
	ValidatedSteps           int                  `json:"validated_steps"`
	TotalSteps               int                  `json:"total_steps"`
	AttemptedSteps           int                  `json:"attempted_steps"`
	AttemptedStepSuccessRate *float64             `json:"attempted_step_success_rate"`
	Families                 []MetricsChainResult `json:"families"`
}

// MetricsChainResult is the per-family summary included in scenario coverage.
type MetricsChainResult struct {
	FamilyID       string `json:"family_id"`
	ChainValidated bool   `json:"chain_validated"`
	TotalSteps     int    `json:"total_steps"`
	ValidatedSteps int    `json:"validated_steps"`
}

// MetricsTimeToChain is the duration from run start to the first fully-validated
// chain completion. Nil when no chains validated.
type MetricsTimeToChain struct {
	DurationMS int64 `json:"duration_ms"`
}

// MetricsLLMUsage aggregates token usage across all planner and evaluator calls
// during a validation run. Fields that cannot be computed honestly are omitted.
type MetricsLLMUsage struct {
	InputTokens     int `json:"input_tokens"`
	OutputTokens    int `json:"output_tokens"`
	TotalTokens     int `json:"total_tokens"`
	CacheReadTokens int `json:"cache_read_tokens"`
	// CacheWriteTokens is the sum of cache_write_tokens across all calls.
	// Null when no cache writes occurred — this reflects the truthful absence
	// of cache write data, not a zero-value default.
	CacheWriteTokens *int `json:"cache_write_tokens,omitempty"`
	// CacheEfficiency is the ratio of cache_read_tokens to input_tokens (0.0–1.0).
	// Null when no input tokens were recorded or no cache reads occurred — this
	// reflects the truthful absence of cache data, not a zero-value default.
	CacheEfficiency *float64 `json:"cache_efficiency,omitempty"`
	// EstimatedCostUSD is the sum of EstimatedCostUSD values from all calls that
	// have known pricing. Nil when no calls had known pricing (safe degradation).
	EstimatedCostUSD *float64 `json:"estimated_cost_usd,omitempty"`
}

// NewMetricsLLMUsage builds a MetricsLLMUsage from a slice of UsageMetadata
// entries. Empty or nil input produces a zero value (nil pointers throughout).
func NewMetricsLLMUsage(usageRecords []*llm.UsageMetadata) MetricsLLMUsage {
	if len(usageRecords) == 0 {
		return MetricsLLMUsage{}
	}
	var in, out, total, cacheRead, cacheWrite int
	var costSum float64
	var hasCost, hasCacheWrite bool

	for _, u := range usageRecords {
		if u == nil {
			continue
		}
		in += u.InputTokens
		out += u.OutputTokens
		total += u.TotalTokens
		cacheRead += u.CacheRead
		if u.CacheWrite > 0 {
			cacheWrite += u.CacheWrite
			hasCacheWrite = true
		}
		if u.EstimatedCostUSD != nil {
			costSum += *u.EstimatedCostUSD
			hasCost = true
		}
	}

	m := MetricsLLMUsage{
		InputTokens:     in,
		OutputTokens:    out,
		TotalTokens:     total,
		CacheReadTokens: cacheRead,
	}
	if hasCacheWrite {
		m.CacheWriteTokens = &cacheWrite
	}
	if in > 0 && cacheRead > 0 {
		ratio := float64(cacheRead) / float64(in)
		m.CacheEfficiency = &ratio
	}
	if hasCost {
		m.EstimatedCostUSD = &costSum
	}
	return m
}

// ValidationMetricsInput carries all raw data needed to compute a metrics summary.
// Callers populate this from their runtime state; ComputeValidationMetrics does
// the aggregation without side effects.
type ValidationMetricsInput struct {
	PlannerType           string
	PlannerMode           string
	ObservationContract   string
	PromptIntegrityStatus string
	ToolSetHash           string
	PolicyHash            string
	StartedAt             time.Time
	FinishedAt            time.Time
	Steps                 int
	TerminationReason     string
	ToolsUsed             []string // must be pre-sorted by caller
	NamespacesTouched     []string // must be pre-sorted by caller
	SnapshotCount         int
	GraphNodes            []graph.Node
	GraphEdges            []graph.Edge
	FinalAnswerPresent    bool
	GraphPath             string
	EvidencePath          string
	OutputDir             string
	GuardrailBlocks       int // pre-computed count of guardrail-blocked actions from trace
	// ScenarioResult is the post-hoc scenario matcher output. When nil,
	// ScenarioCoverage and TimeToChain remain null in the output.
	ScenarioResult *baseline.MatcherOutput
	// LLMUsageRecords collects all UsageMetadata from planner and evaluator
	// calls during the run. May be nil or empty (deterministic skeleton mode).
	LLMUsageRecords []*llm.UsageMetadata
}

// validationEdgeTypes is the set of EdgeType values that represent validation
// tool output (not baseline discovery). These are the edge types that count
// toward the attack-type-coverage breakdown.
var validationEdgeTypes = []string{
	string(graph.EdgeTypeSecretAccess),
	string(graph.EdgeTypePermissionCheck),
	string(graph.EdgeTypeNetworkProbe),
	string(graph.EdgeTypeTokenReview),
}

// ComputeValidationMetrics aggregates the input into a ValidationMetrics struct.
// It is a pure function — no I/O, no mutation of inputs.
func ComputeValidationMetrics(input ValidationMetricsInput) ValidationMetrics {
	edgesByStatus := make(map[string]int, len(input.GraphEdges))
	for _, edge := range input.GraphEdges {
		edgesByStatus[string(edge.Status)]++
	}

	return ValidationMetrics{
		ContractVersion:     contractVersion,
		PlannerType:         input.PlannerType,
		PlannerMode:         input.PlannerMode,
		ObservationContract: input.ObservationContract,
		PromptIntegrity:     MetricsPromptIntegrity{Status: input.PromptIntegrityStatus},
		ToolSetHash:         input.ToolSetHash,
		PolicyHash:          input.PolicyHash,
		StartedAt:           input.StartedAt.UTC(),
		FinishedAt:          input.FinishedAt.UTC(),
		DurationMS:          input.FinishedAt.Sub(input.StartedAt).Milliseconds(),
		Termination: MetricsTermination{
			Reason: input.TerminationReason,
			Steps:  input.Steps,
		},
		Coverage: MetricsCoverage{
			ToolsUsed:         input.ToolsUsed,
			NamespacesTouched: input.NamespacesTouched,
			SnapshotCount:     input.SnapshotCount,
		},
		GraphSummary: MetricsGraphSummary{
			TotalNodes:    len(input.GraphNodes),
			TotalEdges:    len(input.GraphEdges),
			EdgesByStatus: edgesByStatus,
		},
		FinalAnswerPresent: input.FinalAnswerPresent,
		EdgeValidationRate: computeEdgeValidationRate(input.GraphEdges),
		AttackTypeCoverage: computeAttackTypeCoverage(input.GraphEdges),
		ScenarioCoverage:   computeScenarioCoverage(input.ScenarioResult),
		TimeToChain:        computeTimeToChain(input.ScenarioResult),
		LLMUsage:           computeLLMUsage(input.LLMUsageRecords),
		Guardrails: MetricsGuardrails{
			ActionBlocks: input.GuardrailBlocks,
		},
		Artifacts: MetricsArtifacts{
			GraphPath:    input.GraphPath,
			EvidencePath: input.EvidencePath,
			MetricsPath:  filepath.Join(input.OutputDir, "validation-metrics.json"),
		},
	}
}

// WriteValidationMetricsSummary marshals and writes the metrics artifact to the
// path specified in metrics.Artifacts.MetricsPath. Returns the written path.
func WriteValidationMetricsSummary(metrics ValidationMetrics) (string, error) {
	path := metrics.Artifacts.MetricsPath
	body, err := json.MarshalIndent(metrics, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal validation metrics: %w", err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		return "", fmt.Errorf("write validation metrics: %w", err)
	}
	return path, nil
}

// PlannerTypeFromConfig derives the planner type label using the same logic
// as agent.newValidationPlanner — react_llm when both API key and model are
// configured, deterministic_skeleton otherwise.
func PlannerTypeFromConfig(llmAPIKey, llmModel string) string {
	if llmAPIKey != "" && llmModel != "" {
		return "react_llm"
	}
	return "deterministic_skeleton"
}

// computeEdgeValidationRate derives the edge validation rate from graph edges.
// Returns nil when there are no edges (denominator is zero).
func computeEdgeValidationRate(edges []graph.Edge) *MetricsEdgeValidationRate {
	if len(edges) == 0 {
		return nil
	}
	validated := 0
	for _, edge := range edges {
		if edge.Status == graph.EdgeValidated {
			validated++
		}
	}
	rate := float64(validated) / float64(len(edges))
	return &MetricsEdgeValidationRate{
		ValidatedEdges: validated,
		TotalEdges:     len(edges),
		Rate:           &rate,
	}
}

// computeAttackTypeCoverage derives the attack-type coverage breakdown from
// graph edges. Returns nil when there are no edges. Only validation-relevant
// edge types (not discovery) are included in the breakdown.
func computeAttackTypeCoverage(edges []graph.Edge) *MetricsAttackTypeCoverage {
	if len(edges) == 0 {
		return nil
	}

	breakdown := make(map[string]MetricsEdgeTypeSummary, len(validationEdgeTypes))
	for _, et := range validationEdgeTypes {
		breakdown[et] = MetricsEdgeTypeSummary{}
	}

	for _, edge := range edges {
		typeKey := string(edge.Type)
		if _, ok := breakdown[typeKey]; !ok {
			continue // not a validation-relevant type
		}
		entry := breakdown[typeKey]
		entry.Attempted++
		if edge.Status == graph.EdgeValidated {
			entry.Validated++
		}
		breakdown[typeKey] = entry
	}

	// Build sorted lists for deterministic output.
	var attempted, validated []string
	for _, et := range validationEdgeTypes {
		if breakdown[et].Attempted > 0 {
			attempted = append(attempted, et)
		}
		if breakdown[et].Validated > 0 {
			validated = append(validated, et)
		}
	}

	return &MetricsAttackTypeCoverage{
		EdgeTypesValidated: validated,
		EdgeTypesAttempted: attempted,
		EdgeTypesAvailable: len(validationEdgeTypes),
		Breakdown:          breakdown,
	}
}

// computeScenarioCoverage converts the baseline matcher output into the metrics
// scenario coverage struct. Returns nil when the matcher result is nil (no
// scenario matching was performed).
func computeScenarioCoverage(result *baseline.MatcherOutput) *MetricsScenarioCoverage {
	if result == nil {
		return nil
	}
	families := make([]MetricsChainResult, len(result.Families))
	for i, f := range result.Families {
		families[i] = MetricsChainResult{
			FamilyID:       f.FamilyID,
			ChainValidated: f.ChainValidated,
			TotalSteps:     f.TotalSteps,
			ValidatedSteps: f.ValidatedSteps,
		}
	}
	return &MetricsScenarioCoverage{
		ValidatedChainCount:      result.ValidatedChainCount,
		TotalFamilies:            result.TotalFamilies,
		ScenarioRate:             result.ScenarioRate,
		CatalogStepCoverage:      result.CatalogStepCoverage,
		ValidatedSteps:           result.ValidatedSteps,
		TotalSteps:               result.TotalSteps,
		AttemptedSteps:           result.AttemptedSteps,
		AttemptedStepSuccessRate: result.AttemptedStepSuccessRate,
		Families:                 families,
	}
}

// computeTimeToChain extracts the time-to-first-chain duration from the matcher
// output. Returns nil when the matcher result is nil or no chains validated.
func computeTimeToChain(result *baseline.MatcherOutput) *MetricsTimeToChain {
	if result == nil || result.TimeToFirstChain == nil {
		return nil
	}
	return &MetricsTimeToChain{
		DurationMS: result.TimeToFirstChain.Milliseconds(),
	}
}

// computeLLMUsage aggregates LLM usage records into a MetricsLLMUsage pointer.
// Returns nil when the input is nil or empty (deterministic skeleton mode with no
// LLM calls). This preserves the null-safe contract: no tokens means no LLM usage
// to report.
func computeLLMUsage(records []*llm.UsageMetadata) *MetricsLLMUsage {
	if len(records) == 0 {
		return nil
	}
	m := NewMetricsLLMUsage(records)
	// Return nil for zero-value (no tokens) to keep the artifact clean.
	if m.InputTokens == 0 && m.OutputTokens == 0 && m.TotalTokens == 0 {
		return nil
	}
	return &m
}
