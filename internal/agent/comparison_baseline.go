package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/ashwnn/chain-reaction/internal/baseline"
)

type comparisonBaselineArtifact struct {
	ContractVersion string                     `json:"contract_version"`
	BaselineKind    string                     `json:"baseline_kind"`
	RunMode         string                     `json:"run_mode"`
	GeneratedAt     time.Time                  `json:"generated_at"`
	SourceContract  string                     `json:"source_contract"`
	SourceArtifacts []string                   `json:"source_artifacts"`
	ObservedTools   []string                   `json:"observed_tools,omitempty"`
	Families        []comparisonBaselineFamily `json:"families"`
	Notes           []string                   `json:"notes"`
}

type comparisonBaselineFamily struct {
	FamilyID   string                   `json:"family_id"`
	FamilyName string                   `json:"family_name"`
	InScope    bool                     `json:"in_scope"`
	Steps      []comparisonBaselineStep `json:"steps"`
	Notes      []string                 `json:"notes,omitempty"`
}

type comparisonBaselineStep struct {
	StepID          string   `json:"step_id"`
	Description     string   `json:"description"`
	Status          string   `json:"status"`
	ExpectedTools   []string `json:"expected_tools,omitempty"`
	SupportingTools []string `json:"supporting_tools,omitempty"`
	ArtifactRefs    []string `json:"artifact_refs,omitempty"`
}

func writeDiscoveryComparisonBaseline(outputDir, summaryPath, graphPath, evidenceDir string, toolsExecuted []string, generatedAt time.Time) (string, error) {
	artifact := comparisonBaselineArtifact{
		ContractVersion: "baseline.comparison.v1",
		BaselineKind:    "discovery",
		RunMode:         "baseline.discovery_full_pass",
		GeneratedAt:     generatedAt.UTC(),
		SourceContract:  "built-in catalog",
		SourceArtifacts: []string{
			summaryPath,
			graphPath,
			filepath.Join(evidenceDir, "evidence.jsonl"),
			filepath.Join(evidenceDir, "index.json"),
		},
		ObservedTools: cloneStrings(toolsExecuted),
		Notes: []string{
			"Observed steps are limited to catalog steps whose expected tools were executed by the deterministic discovery baseline.",
			"Steps marked not_attempted were outside the discovery baseline's tool plan and remain available for later runtime comparison.",
		},
	}

	observed := make(map[string]struct{}, len(toolsExecuted))
	for _, tool := range toolsExecuted {
		observed[tool] = struct{}{}
	}

	catalog := baseline.DefaultCatalog()
	for _, family := range catalog.Families {
		entry := comparisonBaselineFamily{
			FamilyID:   family.ID,
			FamilyName: family.Name,
			InScope:    true,
			Steps:      make([]comparisonBaselineStep, 0, len(family.Steps)),
		}
		for _, step := range family.Steps {
			supportingTools := intersectExpectedTools(step.ExpectedTools, observed)
			status := "not_attempted"
			var artifactRefs []string
			if len(supportingTools) > 0 {
				status = "observed"
				artifactRefs = []string{summaryPath, filepath.Join(evidenceDir, "evidence.jsonl")}
			}
			entry.Steps = append(entry.Steps, comparisonBaselineStep{
				StepID:          step.StepID,
				Description:     step.Description,
				Status:          status,
				ExpectedTools:   cloneStrings(step.ExpectedTools),
				SupportingTools: supportingTools,
				ArtifactRefs:    artifactRefs,
			})
		}
		artifact.Families = append(artifact.Families, entry)
	}

	return writeComparisonBaselineArtifact(outputDir, artifact)
}

func writeTheoreticalComparisonBaseline(outputDir, artifactPath string, generatedAt time.Time) (string, error) {
	artifact := comparisonBaselineArtifact{
		ContractVersion: "baseline.comparison.v1",
		BaselineKind:    "static_theoretical",
		RunMode:         "baseline.static_theoretical_catalog",
		GeneratedAt:     generatedAt.UTC(),
		SourceContract:  "built-in catalog",
		SourceArtifacts: []string{artifactPath},
		Notes: []string{
			"All steps remain theoretical because this baseline does not inspect a live cluster or execute probes.",
			"The normalized artifact preserves the frozen step-chain catalog so later comparison tooling can diff theory against discovery and validation outputs consistently.",
		},
	}

	catalog := baseline.DefaultCatalog()
	for _, family := range catalog.Families {
		entry := comparisonBaselineFamily{
			FamilyID:   family.ID,
			FamilyName: family.Name,
			InScope:    true,
			Steps:      make([]comparisonBaselineStep, 0, len(family.Steps)),
		}
		for _, step := range family.Steps {
			entry.Steps = append(entry.Steps, comparisonBaselineStep{
				StepID:        step.StepID,
				Description:   step.Description,
				Status:        "theoretical",
				ExpectedTools: cloneStrings(step.ExpectedTools),
				ArtifactRefs:  []string{artifactPath},
			})
		}
		artifact.Families = append(artifact.Families, entry)
	}

	artifact.Families = append(artifact.Families, comparisonBaselineFamily{
		FamilyID:   "KG-006",
		FamilyName: "Container escape or node-compromise dependent scenario",
		InScope:    false,
		Steps: []comparisonBaselineStep{
			{
				StepID:       "KG-006-S1",
				Description:  "Model a host or kernel-level breakout dependency outside the assumed-breach Pod contract",
				Status:       "theoretical",
				ArtifactRefs: []string{artifactPath},
			},
		},
		Notes: []string{"This scenario family remains out of scope for validated-success metrics under the current assumed-breach model."},
	})

	return writeComparisonBaselineArtifact(outputDir, artifact)
}

func writeComparisonBaselineArtifact(outputDir string, artifact comparisonBaselineArtifact) (string, error) {
	path := filepath.Join(outputDir, "comparison-baseline.json")
	body, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal comparison baseline: %w", err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return "", fmt.Errorf("write comparison baseline: %w", err)
	}
	return path, nil
}

func intersectExpectedTools(expected []string, observed map[string]struct{}) []string {
	if len(expected) == 0 || len(observed) == 0 {
		return nil
	}

	matched := make([]string, 0, len(expected))
	for _, tool := range expected {
		if _, ok := observed[tool]; ok {
			matched = append(matched, tool)
		}
	}
	if len(matched) == 0 {
		return nil
	}
	sort.Strings(matched)
	return matched
}
