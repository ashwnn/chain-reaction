package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ashwnn/chain-reaction/internal/baseline"
	"github.com/ashwnn/chain-reaction/internal/config"
)

type TheoreticalBaselineResult struct {
	Path           string
	ComparisonPath string
	RunMode        string
	Duration       time.Duration
}

type theoreticalBaselineArtifact struct {
	ContractVersion string                        `json:"contract_version"`
	RunMode         string                        `json:"run_mode"`
	ExecutionModel  string                        `json:"execution_model"`
	GeneratedAt     time.Time                     `json:"generated_at"`
	SourceBaseline  string                        `json:"source_baseline"`
	Scenarios       []theoreticalScenarioArtifact `json:"scenarios"`
	Notes           []string                      `json:"notes"`
}

type theoreticalScenarioArtifact struct {
	CatalogID       string                    `json:"catalog_id"`
	ScenarioFamily  string                    `json:"scenario_family"`
	InScope         bool                      `json:"in_scope"`
	ExpectedOutcome string                    `json:"expected_outcome"`
	Steps           []theoreticalStepArtifact `json:"steps"`
	Notes           []string                  `json:"notes,omitempty"`
}

type theoreticalStepArtifact struct {
	ID            string   `json:"id"`
	Description   string   `json:"description"`
	Status        string   `json:"status"`
	Prerequisites []string `json:"prerequisites,omitempty"`
}

func ExportTheoreticalBaseline(cfg config.Config) (TheoreticalBaselineResult, error) {
	start := time.Now().UTC()
	artifactPath := filepath.Join(cfg.OutputPath, "theoretical-baseline.json")
	artifact := theoreticalBaselineArtifact{
		ContractVersion: "baseline.static_theoretical.v1",
		RunMode:         "baseline.static_theoretical_catalog",
		ExecutionModel:  "catalog_backed_no_probes",
		GeneratedAt:     start,
		SourceBaseline:  "built-in catalog",
		Scenarios:       staticTheoreticalBaselineCatalog(),
		Notes: []string{
			"This artifact is derived from the frozen manual baseline catalog.",
			"It does not inspect a live cluster, invoke tools, or prove exploitability.",
			"All emitted step statuses are theoretical unless a future runtime validation path proves them otherwise.",
		},
	}

	body, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return TheoreticalBaselineResult{}, fmt.Errorf("marshal theoretical baseline: %w", err)
	}
	if err := os.MkdirAll(cfg.OutputPath, 0o755); err != nil {
		return TheoreticalBaselineResult{}, fmt.Errorf("create output directory: %w", err)
	}
	if err := os.WriteFile(artifactPath, body, 0o600); err != nil {
		return TheoreticalBaselineResult{}, fmt.Errorf("write theoretical baseline: %w", err)
	}

	comparisonPath, err := writeTheoreticalComparisonBaseline(cfg.OutputPath, artifactPath, start)
	if err != nil {
		return TheoreticalBaselineResult{}, fmt.Errorf("write comparison baseline: %w", err)
	}

	return TheoreticalBaselineResult{
		Path:           artifactPath,
		ComparisonPath: comparisonPath,
		RunMode:        artifact.RunMode,
		Duration:       time.Since(start),
	}, nil
}

// staticTheoreticalBaselineCatalog derives the theoretical baseline from the
// step-chain catalog (internal/baseline). KG-001 through KG-005 are populated
// from the catalog with matching step IDs, descriptions, and prerequisite
// ordering. KG-006 is appended separately as an out-of-scope entry because it
// is excluded from the step-chain catalog per the frozen baseline.
func staticTheoreticalBaselineCatalog() []theoreticalScenarioArtifact {
	catalog := baseline.DefaultCatalog()
	scenarios := make([]theoreticalScenarioArtifact, 0, len(catalog.Families)+1)

	for _, family := range catalog.Families {
		steps := make([]theoreticalStepArtifact, 0, len(family.Steps))
		for _, step := range family.Steps {
			steps = append(steps, theoreticalStepArtifact{
				ID:            step.StepID,
				Description:   step.Description,
				Status:        "theoretical",
				Prerequisites: step.Prerequisites,
			})
		}
		scenarios = append(scenarios, theoreticalScenarioArtifact{
			CatalogID:       family.ID,
			ScenarioFamily:  family.Name,
			InScope:         true,
			ExpectedOutcome: "theoretical",
			Steps:           steps,
		})
	}

	// KG-006 is out-of-scope per the frozen baseline and not included in the
	// step-chain catalog. Preserve it as a static entry so the theoretical
	// artifact still documents its exclusion.
	scenarios = append(scenarios, theoreticalScenarioArtifact{
		CatalogID:       "KG-006",
		ScenarioFamily:  "Container escape or node-compromise dependent scenario",
		InScope:         false,
		ExpectedOutcome: "out_of_scope_for_validated_success",
		Steps: []theoreticalStepArtifact{
			{ID: "KG-006-S1", Description: "Model a host or kernel-level breakout dependency outside the assumed-breach Pod contract", Status: "theoretical"},
		},
		Notes: []string{"This scenario family remains outside validated-success metrics under the current assumed-breach model."},
	})

	return scenarios
}
