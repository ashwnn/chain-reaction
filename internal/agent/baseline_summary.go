package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ashwnn/chain-reaction/internal/config"
)

type baselineSummary struct {
	ContractVersion string                     `json:"contract_version"`
	RunMode         string                     `json:"run_mode"`
	ExecutionModel  string                     `json:"execution_model"`
	StartedAt       time.Time                  `json:"started_at"`
	FinishedAt      time.Time                  `json:"finished_at"`
	Duration        string                     `json:"duration"`
	Scope           baselineSummaryScope       `json:"scope"`
	Constraints     baselineSummaryConstraints `json:"constraints"`
	Discovery       baselineSummaryDiscovery   `json:"discovery"`
	Artifacts       baselineSummaryArtifacts   `json:"artifacts"`
	Notes           []string                   `json:"notes"`
}

type baselineSummaryScope struct {
	RequestedNamespace string   `json:"requested_namespace,omitempty"`
	AllowedNamespaces  []string `json:"allowed_namespaces,omitempty"`
}

type baselineSummaryConstraints struct {
	TimeBudget string  `json:"time_budget"`
	K8sQPS     float32 `json:"k8s_qps"`
	K8sBurst   int     `json:"k8s_burst"`
}

type baselineSummaryDiscovery struct {
	Namespaces          []string `json:"namespaces"`
	ToolCalls           int      `json:"tool_calls"`
	ToolsExecuted       []string `json:"tools_executed"`
	NamespacedTools     []string `json:"namespaced_tools"`
	ClusterScopedTools  []string `json:"cluster_scoped_tools"`
	ValidationSemantics string   `json:"validation_semantics"`
}

type baselineSummaryArtifacts struct {
	GraphPath       string `json:"graph_path"`
	EvidenceDir     string `json:"evidence_dir"`
	EvidenceLogPath string `json:"evidence_log_path"`
	SnapshotIndex   string `json:"snapshot_index_path"`
	OutputFormat    string `json:"output_format"`
	SummaryPath     string `json:"summary_path"`
}

func writeBaselineSummary(
	outputDir string,
	cfg config.Config,
	start time.Time,
	finished time.Time,
	namespaces []string,
	toolsExecuted []string,
	toolCalls int,
	graphPath string,
	evidenceDir string,
) (string, error) {
	summaryPath := filepath.Join(outputDir, "baseline-summary.json")
	summary := baselineSummary{
		ContractVersion: "baseline.discovery.v1",
		RunMode:         "baseline.discovery_full_pass",
		ExecutionModel:  "deterministic_discovery_only",
		StartedAt:       start.UTC(),
		FinishedAt:      finished.UTC(),
		Duration:        finished.UTC().Sub(start.UTC()).String(),
		Scope: baselineSummaryScope{
			RequestedNamespace: cfg.Namespace,
			AllowedNamespaces:  cloneStrings(cfg.AllowListNamespaces),
		},
		Constraints: baselineSummaryConstraints{
			TimeBudget: cfg.TimeBudget.String(),
			K8sQPS:     cfg.QPS,
			K8sBurst:   cfg.Burst,
		},
		Discovery: baselineSummaryDiscovery{
			Namespaces:          cloneStrings(namespaces),
			ToolCalls:           toolCalls,
			ToolsExecuted:       cloneStrings(toolsExecuted),
			NamespacedTools:     cloneStrings(baselineNamespacedTools),
			ClusterScopedTools:  cloneStrings(baselineClusterScopedTools),
			ValidationSemantics: "not_applied",
		},
		Artifacts: baselineSummaryArtifacts{
			GraphPath:       graphPath,
			EvidenceDir:     evidenceDir,
			EvidenceLogPath: filepath.Join(evidenceDir, "evidence.jsonl"),
			SnapshotIndex:   filepath.Join(evidenceDir, "index.json"),
			OutputFormat:    cfg.OutputFormat,
			SummaryPath:     summaryPath,
		},
		Notes: []string{
			"This summary describes the current scan baseline only.",
			"The baseline does not perform planner-driven chaining or failed-step validation semantics.",
			"Validation and introspection tools may be registered elsewhere in the repo but are not part of the deterministic discovery execution contract.",
		},
	}

	body, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal baseline summary: %w", err)
	}
	if err := os.WriteFile(summaryPath, body, 0o600); err != nil {
		return "", fmt.Errorf("write baseline summary: %w", err)
	}
	return summaryPath, nil
}

func cloneStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	cloned := make([]string, len(values))
	copy(cloned, values)
	return cloned
}
