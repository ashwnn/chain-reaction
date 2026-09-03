package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ashwnn/chain-reaction/internal/config"
)

func TestExportTheoreticalBaselineWritesStableArtifact(t *testing.T) {
	tmpDir := t.TempDir()
	result, err := ExportTheoreticalBaseline(config.Config{OutputPath: tmpDir})
	if err != nil {
		t.Fatalf("export theoretical baseline: %v", err)
	}

	expectedPath := filepath.Join(tmpDir, "theoretical-baseline.json")
	if result.Path != expectedPath {
		t.Fatalf("expected theoretical baseline path %q, got %q", expectedPath, result.Path)
	}
	expectedComparisonPath := filepath.Join(tmpDir, "comparison-baseline.json")
	if result.ComparisonPath != expectedComparisonPath {
		t.Fatalf("expected comparison baseline path %q, got %q", expectedComparisonPath, result.ComparisonPath)
	}
	if result.RunMode != "baseline.static_theoretical_catalog" {
		t.Fatalf("expected static theoretical run mode, got %q", result.RunMode)
	}
	info, err := os.Stat(result.Path)
	if err != nil {
		t.Fatalf("expected theoretical baseline artifact at %q: %v", result.Path, err)
	}

	if info.Mode().Perm() != 0o600 && info.Mode().Perm() != 0o666 {
		t.Errorf("expected file permissions 0600 (or 0666 on Windows), got %o", info.Mode().Perm())
	}
}

func TestExportTheoreticalBaselineWritesComparisonContract(t *testing.T) {
	tmpDir := t.TempDir()
	result, err := ExportTheoreticalBaseline(config.Config{OutputPath: tmpDir})
	if err != nil {
		t.Fatalf("export theoretical baseline: %v", err)
	}

	contents, err := os.ReadFile(result.ComparisonPath)
	if err != nil {
		t.Fatalf("read comparison baseline: %v", err)
	}

	var artifact comparisonBaselineArtifact
	if err := json.Unmarshal(contents, &artifact); err != nil {
		t.Fatalf("unmarshal comparison baseline: %v", err)
	}
	if artifact.ContractVersion != "baseline.comparison.v1" {
		t.Fatalf("expected comparison contract version, got %q", artifact.ContractVersion)
	}
	if artifact.BaselineKind != "static_theoretical" {
		t.Fatalf("expected static_theoretical baseline kind, got %q", artifact.BaselineKind)
	}
	for _, family := range artifact.Families {
		for _, step := range family.Steps {
			if step.Status != "theoretical" {
				t.Fatalf("expected theoretical status in normalized artifact, got %q", step.Status)
			}
		}
	}
}

func TestExportTheoreticalBaselineEmitsTheoreticalSteps(t *testing.T) {
	tmpDir := t.TempDir()
	result, err := ExportTheoreticalBaseline(config.Config{OutputPath: tmpDir})
	if err != nil {
		t.Fatalf("export theoretical baseline: %v", err)
	}

	contents, err := os.ReadFile(result.Path)
	if err != nil {
		t.Fatalf("read theoretical baseline: %v", err)
	}

	var artifact theoreticalBaselineArtifact
	if err := json.Unmarshal(contents, &artifact); err != nil {
		t.Fatalf("unmarshal theoretical baseline: %v", err)
	}
	if artifact.ContractVersion != "baseline.static_theoretical.v1" {
		t.Fatalf("expected contract version, got %q", artifact.ContractVersion)
	}
	if len(artifact.Scenarios) == 0 {
		t.Fatal("expected theoretical scenarios in artifact")
	}

	// Verify all step statuses are theoretical.
	for _, scenario := range artifact.Scenarios {
		for _, step := range scenario.Steps {
			if step.Status != "theoretical" {
				t.Fatalf("expected only theoretical step statuses, got %q in %s", step.Status, step.ID)
			}
		}
	}

	// Verify step IDs match the step-chain catalog format (KG-xxx-Sy).
	for _, scenario := range artifact.Scenarios {
		for _, step := range scenario.Steps {
			if len(step.ID) < 8 {
				t.Fatalf("step ID %q too short, expected format KG-xxx-Sy", step.ID)
			}
			if step.ID[6] != '-' || step.ID[7] != 'S' {
				t.Fatalf("step ID %q does not match catalog format KG-xxx-Sy", step.ID)
			}
		}
	}

	// Verify correct scenario count: 5 in-scope + 1 out-of-scope (KG-006).
	if len(artifact.Scenarios) != 6 {
		t.Fatalf("expected 6 scenarios (5 in-scope + KG-006), got %d", len(artifact.Scenarios))
	}

	// Verify in-scope scenarios have correct step counts from the catalog.
	expectedStepCounts := map[string]int{
		"KG-001": 3,
		"KG-002": 2,
		"KG-003": 2,
		"KG-004": 2,
		"KG-005": 3,
		"KG-006": 1,
	}
	for _, scenario := range artifact.Scenarios {
		expected, ok := expectedStepCounts[scenario.CatalogID]
		if !ok {
			t.Fatalf("unexpected catalog ID %q", scenario.CatalogID)
		}
		if len(scenario.Steps) != expected {
			t.Fatalf("expected %d steps for %s, got %d", expected, scenario.CatalogID, len(scenario.Steps))
		}
	}
}
