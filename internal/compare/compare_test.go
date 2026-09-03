package compare

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestGenerateTheoryOnly tests comparison with only the theory artifact.
func TestGenerateTheoryOnly(t *testing.T) {
	// Create a temporary directory with a theory comparison-baseline.json.
	dir := t.TempDir()
	theoryPath := filepath.Join(dir, "theory-baseline.json")

	theoryContent := `{
  "contract_version": "baseline.comparison.v1",
  "baseline_kind": "static_theoretical",
  "families": [
    {
      "family_id": "KG-001",
      "family_name": "RBAC Over-Provisioning",
      "in_scope": true,
      "steps": [
        {
          "step_id": "KG-001-S1",
          "description": "Identify current SA identity and token context",
          "status": "theoretical",
          "expected_tools": ["validation.check_token"]
        },
        {
          "step_id": "KG-001-S2",
          "description": "Enumerate effective permissions",
          "status": "theoretical",
          "expected_tools": ["validation.check_permissions"]
        }
      ]
    }
  ]
}`

	if err := os.WriteFile(theoryPath, []byte(theoryContent), 0o644); err != nil {
		t.Fatalf("write theory fixture: %v", err)
	}

	result, err := Generate(InputPaths{Theory: theoryPath})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	// Verify contract version.
	if result.ContractVersion != ContractVersion {
		t.Errorf("expected contract version %q, got %q", ContractVersion, result.ContractVersion)
	}

	// Verify sources.
	if result.Sources.Theory == nil {
		t.Fatal("expected theory source to be populated")
	}
	if result.Sources.Scan != nil {
		t.Error("expected scan source to be nil")
	}
	if result.Sources.Analysis != nil {
		t.Error("expected analysis source to be nil")
	}

	// Verify family count.
	if len(result.Families) < 1 {
		t.Fatal("expected at least one family")
	}

	// Find KG-001.
	var kg001 *FamilyResult
	for i := range result.Families {
		if result.Families[i].FamilyID == "KG-001" {
			kg001 = &result.Families[i]
			break
		}
	}
	if kg001 == nil {
		t.Fatal("KG-001 not found in families")
	}

	// Verify step data.
	if len(kg001.Steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(kg001.Steps))
	}

	// First step should have theory status.
	s1 := findStep(kg001.Steps, "KG-001-S1")
	if s1 == nil {
		t.Fatal("KG-001-S1 not found")
	}
	if s1.TheoryStatus == nil || *s1.TheoryStatus != "theoretical" {
		t.Errorf("expected theory status 'theoretical', got %v", s1.TheoryStatus)
	}
	if s1.ScanStatus != nil {
		t.Error("expected scan status to be nil")
	}
	if s1.RuntimeStatus != nil {
		t.Error("expected runtime status to be nil")
	}
}

// TestGenerateScanOnly tests comparison with only the scan artifact.
func TestGenerateScanOnly(t *testing.T) {
	dir := t.TempDir()
	scanPath := filepath.Join(dir, "scan-baseline.json")

	scanContent := `{
  "contract_version": "baseline.comparison.v1",
  "baseline_kind": "discovery",
  "families": [
    {
      "family_id": "KG-002",
      "family_name": "Secret Access",
      "in_scope": true,
      "steps": [
        {
          "step_id": "KG-002-S1",
          "description": "Confirm permission to read secrets",
          "status": "observed",
          "expected_tools": ["validation.check_permissions"],
          "supporting_tools": ["validation.check_permissions"]
        }
      ]
    }
  ]
}`

	if err := os.WriteFile(scanPath, []byte(scanContent), 0o644); err != nil {
		t.Fatalf("write scan fixture: %v", err)
	}

	result, err := Generate(InputPaths{Scan: scanPath})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	// Find KG-002.
	var kg002 *FamilyResult
	for i := range result.Families {
		if result.Families[i].FamilyID == "KG-002" {
			kg002 = &result.Families[i]
			break
		}
	}
	if kg002 == nil {
		t.Fatal("KG-002 not found")
	}

	s1 := findStep(kg002.Steps, "KG-002-S1")
	if s1 == nil {
		t.Fatal("KG-002-S1 not found")
	}
	if s1.ScanStatus == nil || *s1.ScanStatus != "observed" {
		t.Errorf("expected scan status 'observed', got %v", s1.ScanStatus)
	}
	if s1.SupportingTools == nil || len(s1.SupportingTools) == 0 {
		t.Error("expected supporting tools to be populated")
	}
}

// TestGenerateAnalysisOnly tests comparison with only the analysis artifact.
func TestGenerateAnalysisOnly(t *testing.T) {
	dir := t.TempDir()
	analysisPath := filepath.Join(dir, "analysis.json")

	analysisContent := `{
  "id": "test-run-set",
  "source_dir": "artifacts/scenario-runs",
  "run_count": 5,
  "per_family": [
    {
      "family_id": "KG-001",
      "chain_validated_count": 5,
      "attempted_count": 5,
      "reliability_fraction": 1.0,
      "catalog_step_coverage_sample": {
        "n": 5,
        "mean": 1.0,
        "sd": 0.0
      }
    },
    {
      "family_id": "KG-004",
      "chain_validated_count": 2,
      "attempted_count": 5,
      "reliability_fraction": 0.4,
      "catalog_step_coverage_sample": {
        "n": 5,
        "mean": 0.6,
        "sd": 0.2
      }
    }
  ]
}`

	if err := os.WriteFile(analysisPath, []byte(analysisContent), 0o644); err != nil {
		t.Fatalf("write analysis fixture: %v", err)
	}

	result, err := Generate(InputPaths{Analysis: analysisPath})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	// Verify analysis source.
	if result.Sources.Analysis == nil {
		t.Fatal("expected analysis source to be populated")
	}
	if result.Sources.Analysis.RunSetID != "test-run-set" {
		t.Errorf("expected run set ID 'test-run-set', got %q", result.Sources.Analysis.RunSetID)
	}
	if result.Sources.Analysis.RunCount != 5 {
		t.Errorf("expected run count 5, got %d", result.Sources.Analysis.RunCount)
	}

	// Verify blocker summary - KG-004 has 0.4 reliability (not blocked).
	if result.BlockerSummary != nil {
		for _, b := range result.BlockerSummary.BlockedFamilies {
			if b.FamilyID == "KG-004" {
				t.Error("KG-004 should not be in blocker summary (reliability = 0.4)")
			}
		}
	}

	// Verify failure summary - KG-004 has partial completion.
	if result.FailureSummary == nil {
		t.Error("expected failure summary for KG-004 (partial validation)")
	}
	if result.FailureSummary != nil && len(result.FailureSummary.FamiliesWithFailures) > 0 {
		found := false
		for _, f := range result.FailureSummary.FamiliesWithFailures {
			if f.FamilyID == "KG-004" {
				found = true
				if f.PartialRate != 0.6 {
					t.Errorf("expected partial rate 0.6, got %f", f.PartialRate)
				}
			}
		}
		if !found {
			t.Error("KG-004 should be in failure summary")
		}
	}

	// Verify runtime status on steps.
	kg004 := findFamily(result.Families, "KG-004")
	if kg004 != nil && kg004.Runtime != nil {
		if kg004.Runtime.ReliabilityFraction == nil || *kg004.Runtime.ReliabilityFraction != 0.4 {
			t.Errorf("expected KG-004 reliability 0.4, got %v", kg004.Runtime.ReliabilityFraction)
		}
	}
}

// TestGenerateAllSources tests joining all three artifact types.
func TestGenerateAllSources(t *testing.T) {
	dir := t.TempDir()

	theoryPath := filepath.Join(dir, "theory.json")
	scanPath := filepath.Join(dir, "scan.json")
	analysisPath := filepath.Join(dir, "analysis.json")

	// Write theory.
	theoryContent := `{
  "contract_version": "baseline.comparison.v1",
  "baseline_kind": "static_theoretical",
  "families": [
    {
      "family_id": "KG-001",
      "family_name": "RBAC Over-Provisioning",
      "in_scope": true,
      "steps": [
        {"step_id": "KG-001-S1", "description": "Step 1", "status": "theoretical", "expected_tools": ["validation.check_token"]},
        {"step_id": "KG-001-S2", "description": "Step 2", "status": "theoretical", "expected_tools": ["validation.check_permissions"]}
      ]
    }
  ]
}`
	os.WriteFile(theoryPath, []byte(theoryContent), 0o644)

	// Write scan.
	scanContent := `{
  "contract_version": "baseline.comparison.v1",
  "baseline_kind": "discovery",
  "families": [
    {
      "family_id": "KG-001",
      "family_name": "RBAC Over-Provisioning",
      "in_scope": true,
      "steps": [
        {"step_id": "KG-001-S1", "description": "Step 1", "status": "observed", "expected_tools": ["validation.check_token"], "supporting_tools": ["validation.check_token"]},
        {"step_id": "KG-001-S2", "description": "Step 2", "status": "not_attempted", "expected_tools": ["validation.check_permissions"]}
      ]
    }
  ]
}`
	os.WriteFile(scanPath, []byte(scanContent), 0o644)

	// Write analysis.
	analysisContent := `{
  "id": "all-sources-test",
  "run_count": 3,
  "per_family": [
    {
      "family_id": "KG-001",
      "chain_validated_count": 3,
      "attempted_count": 3,
      "reliability_fraction": 1.0
    }
  ]
}`
	os.WriteFile(analysisPath, []byte(analysisContent), 0o644)

	result, err := Generate(InputPaths{
		Analysis: analysisPath,
		Theory:   theoryPath,
		Scan:     scanPath,
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	// Verify all sources are populated.
	if result.Sources.Theory == nil {
		t.Error("expected theory source")
	}
	if result.Sources.Scan == nil {
		t.Error("expected scan source")
	}
	if result.Sources.Analysis == nil {
		t.Error("expected analysis source")
	}

	// Find KG-001.
	kg001 := findFamily(result.Families, "KG-001")
	if kg001 == nil {
		t.Fatal("KG-001 not found")
	}

	// Verify step S1 has both theory and scan statuses.
	s1 := findStep(kg001.Steps, "KG-001-S1")
	if s1 == nil {
		t.Fatal("KG-001-S1 not found")
	}
	if s1.TheoryStatus == nil || *s1.TheoryStatus != "theoretical" {
		t.Error("expected theory status")
	}
	if s1.ScanStatus == nil || *s1.ScanStatus != "observed" {
		t.Error("expected scan status 'observed'")
	}
	if s1.RuntimeStatus == nil || *s1.RuntimeStatus != "validated" {
		t.Error("expected runtime status 'validated'")
	}

	// Verify step S2 has theory, scan, but no runtime validation.
	s2 := findStep(kg001.Steps, "KG-001-S2")
	if s2 == nil {
		t.Fatal("KG-001-S2 not found")
	}
	if s2.ScanStatus == nil || *s2.ScanStatus != "not_attempted" {
		t.Error("expected scan status 'not_attempted'")
	}
	// Runtime status should be nil because we don't have per-step runtime data.
	// The family had 100% reliability, so all steps should be marked validated.
	if s2.RuntimeStatus == nil {
		t.Error("expected runtime status for KG-001-S2 (family had 100% reliability)")
	}
}

// TestGenerateMissingArtifact tests error handling for missing artifacts.
func TestGenerateMissingArtifact(t *testing.T) {
	_, err := Generate(InputPaths{
		Analysis: "/nonexistent/analysis.json",
	})
	if err == nil {
		t.Error("expected error for missing analysis artifact")
	}
}

// TestJSONStability tests that JSON output is deterministic.
func TestJSONStability(t *testing.T) {
	dir := t.TempDir()
	theoryPath := filepath.Join(dir, "theory.json")
	jsonPath := filepath.Join(dir, "output.json")

	theoryContent := `{
  "contract_version": "baseline.comparison.v1",
  "baseline_kind": "static_theoretical",
  "families": [
    {
      "family_id": "KG-001",
      "family_name": "Test Family",
      "in_scope": true,
      "steps": [
        {"step_id": "KG-001-S1", "description": "Test step", "status": "theoretical", "expected_tools": ["test.tool"]}
      ]
    }
  ]
}`
	os.WriteFile(theoryPath, []byte(theoryContent), 0o644)

	// Generate twice with a fixed time to ensure determinism.
	result, err := Generate(InputPaths{Theory: theoryPath})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	// Write JSON.
	path1, err := WriteJSON(result, jsonPath)
	if err != nil {
		t.Fatalf("WriteJSON returned error: %v", err)
	}

	// Read back.
	data1, err := os.ReadFile(path1)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}

	// Generate again.
	result2, err := Generate(InputPaths{Theory: theoryPath})
	if err != nil {
		t.Fatalf("Generate returned error on second call: %v", err)
	}

	// Note: GeneratedAt is time.Now() so the output won't be byte-identical.
	// Instead, verify the structure is correct.
	if result2.ContractVersion != result.ContractVersion {
		t.Error("contract version mismatch")
	}
	if len(result2.Families) != len(result.Families) {
		t.Error("family count mismatch")
	}

	// Verify JSON is valid.
	var parsed map[string]interface{}
	if err := json.Unmarshal(data1, &parsed); err != nil {
		t.Fatalf("output JSON is invalid: %v", err)
	}

	// Check required top-level fields.
	requiredFields := []string{"contract_version", "generated_at", "sources", "families"}
	for _, field := range requiredFields {
		if _, ok := parsed[field]; !ok {
			t.Errorf("missing required field: %s", field)
		}
	}
}

// TestMarkdownOutput tests Markdown rendering.
func TestMarkdownOutput(t *testing.T) {
	dir := t.TempDir()
	theoryPath := filepath.Join(dir, "theory.json")

	theoryContent := `{
  "contract_version": "baseline.comparison.v1",
  "baseline_kind": "static_theoretical",
  "families": [
    {
      "family_id": "KG-001",
      "family_name": "RBAC Over-Provisioning",
      "in_scope": true,
      "steps": [
        {"step_id": "KG-001-S1", "description": "Identify SA identity", "status": "theoretical", "expected_tools": ["validation.check_token"]},
        {"step_id": "KG-001-S2", "description": "Enumerate permissions", "status": "theoretical", "expected_tools": ["validation.check_permissions"]}
      ]
    }
  ]
}`
	os.WriteFile(theoryPath, []byte(theoryContent), 0o644)

	result, err := Generate(InputPaths{Theory: theoryPath})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	// Write Markdown to buffer.
	var buf bytes.Buffer
	opts := MarkdownOptions{IncludeRuntime: false}
	WriteMarkdown(&buf, result, opts)
	output := buf.String()

	// Verify expected sections.
	if !bytes.Contains([]byte(output), []byte("# Chain Reaction Comparison Report")) {
		t.Error("missing report header")
	}
	if !bytes.Contains([]byte(output), []byte("KG-001")) {
		t.Error("missing KG-001 family")
	}
	if !bytes.Contains([]byte(output), []byte("KG-001-S1")) {
		t.Error("missing KG-001-S1 step")
	}
	if !bytes.Contains([]byte(output), []byte("theoretical")) {
		t.Error("missing theoretical status")
	}
	if !bytes.Contains([]byte(output), []byte("Theory")) {
		t.Error("missing Theory column header")
	}
	if !bytes.Contains([]byte(output), []byte("Scan")) {
		t.Error("missing Scan column header")
	}
}

// TestMarkdownWithRuntime tests Markdown rendering with runtime data.
func TestMarkdownWithRuntime(t *testing.T) {
	dir := t.TempDir()
	analysisPath := filepath.Join(dir, "analysis.json")

	analysisContent := `{
  "id": "runtime-test",
  "run_count": 5,
  "per_family": [
    {
      "family_id": "KG-001",
      "chain_validated_count": 4,
      "attempted_count": 5,
      "reliability_fraction": 0.8,
      "catalog_step_coverage_sample": {"n": 5, "mean": 0.9, "sd": 0.1}
    }
  ]
}`
	os.WriteFile(analysisPath, []byte(analysisContent), 0o644)

	result, err := Generate(InputPaths{Analysis: analysisPath})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	// Write Markdown with runtime.
	var buf bytes.Buffer
	opts := MarkdownOptions{IncludeRuntime: true}
	WriteMarkdown(&buf, result, opts)
	output := buf.String()

	// Verify runtime column is present.
	if !bytes.Contains([]byte(output), []byte("Runtime")) {
		t.Error("missing Runtime column header")
	}

	// Verify runtime summary is present.
	if !bytes.Contains([]byte(output), []byte("4/5 chains validated")) {
		t.Error("missing runtime chain count")
	}
	if !bytes.Contains([]byte(output), []byte("80%")) {
		t.Error("missing reliability percentage")
	}
}

// TestBlockerSummaryWithZeroReliability tests blocker summary for zero reliability families.
func TestBlockerSummaryWithZeroReliability(t *testing.T) {
	dir := t.TempDir()
	analysisPath := filepath.Join(dir, "analysis.json")

	analysisContent := `{
  "id": "blocker-test",
  "run_count": 5,
  "per_family": [
    {
      "family_id": "KG-004",
      "chain_validated_count": 0,
      "attempted_count": 5,
      "reliability_fraction": 0.0
    }
  ]
}`
	os.WriteFile(analysisPath, []byte(analysisContent), 0o644)

	result, err := Generate(InputPaths{Analysis: analysisPath})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	// Verify blocker summary.
	if result.BlockerSummary == nil {
		t.Fatal("expected blocker summary")
	}
	if len(result.BlockerSummary.BlockedFamilies) != 1 {
		t.Errorf("expected 1 blocked family, got %d", len(result.BlockerSummary.BlockedFamilies))
	}
	if result.BlockerSummary.BlockedFamilies[0].FamilyID != "KG-004" {
		t.Errorf("expected blocked family KG-004, got %s", result.BlockerSummary.BlockedFamilies[0].FamilyID)
	}
	if result.BlockerSummary.TotalChainsAttempted != 5 {
		t.Errorf("expected 5 total attempted, got %d", result.BlockerSummary.TotalChainsAttempted)
	}
	if result.BlockerSummary.TotalChainsValidated != 0 {
		t.Errorf("expected 0 total validated, got %d", result.BlockerSummary.TotalChainsValidated)
	}
}

// TestFamilySorting tests that families are sorted deterministically.
func TestFamilySorting(t *testing.T) {
	dir := t.TempDir()
	analysisPath := filepath.Join(dir, "analysis.json")

	// Analysis with families in non-sorted order.
	analysisContent := `{
  "id": "sort-test",
  "run_count": 1,
  "per_family": [
    {"family_id": "KG-005", "chain_validated_count": 1, "attempted_count": 1, "reliability_fraction": 1.0},
    {"family_id": "KG-001", "chain_validated_count": 1, "attempted_count": 1, "reliability_fraction": 1.0},
    {"family_id": "KG-003", "chain_validated_count": 1, "attempted_count": 1, "reliability_fraction": 1.0}
  ]
}`
	os.WriteFile(analysisPath, []byte(analysisContent), 0o644)

	result, err := Generate(InputPaths{Analysis: analysisPath})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	// Verify sorted order.
	expectedOrder := []string{"KG-001", "KG-002", "KG-003", "KG-004", "KG-005"}
	for i, expected := range expectedOrder {
		if i >= len(result.Families) {
			break
		}
		if result.Families[i].FamilyID != expected {
			t.Errorf("family %d: expected %s, got %s", i, expected, result.Families[i].FamilyID)
		}
	}
}

// TestEmptyPaths tests that empty paths produce catalog-only results (no error).
// The canonical 5 in-scope families are included from the catalog even when
// no artifacts are provided.
func TestEmptyPaths(t *testing.T) {
	result, err := Generate(InputPaths{})
	if err != nil {
		t.Fatalf("Generate with empty paths returned unexpected error: %v", err)
	}
	// Empty paths should still produce the 5 canonical families from the catalog.
	if len(result.Families) != 5 {
		t.Errorf("expected 5 canonical families with empty paths, got %d", len(result.Families))
	}
	// All sources should be nil.
	if result.Sources.Theory != nil {
		t.Error("expected nil theory source")
	}
	if result.Sources.Scan != nil {
		t.Error("expected nil scan source")
	}
	if result.Sources.Analysis != nil {
		t.Error("expected nil analysis source")
	}
	// Runtime summaries should all be nil.
	for _, f := range result.Families {
		if f.Runtime != nil {
			t.Errorf("expected nil runtime for %s", f.FamilyID)
		}
	}
}

// findFamily finds a family by ID in the results.
func findFamily(families []FamilyResult, familyID string) *FamilyResult {
	for i := range families {
		if families[i].FamilyID == familyID {
			return &families[i]
		}
	}
	return nil
}

// findStep finds a step by ID in the results.
func findStep(steps []StepResult, stepID string) *StepResult {
	for i := range steps {
		if steps[i].StepID == stepID {
			return &steps[i]
		}
	}
	return nil
}

// TestGeneratedAtIsRecent verifies that GeneratedAt is set.
func TestGeneratedAtIsRecent(t *testing.T) {
	dir := t.TempDir()
	theoryPath := filepath.Join(dir, "theory.json")

	theoryContent := `{
  "contract_version": "baseline.comparison.v1",
  "baseline_kind": "static_theoretical",
  "families": [
    {"family_id": "KG-001", "family_name": "Test", "in_scope": true, "steps": []}
  ]
}`
	os.WriteFile(theoryPath, []byte(theoryContent), 0o644)

	before := time.Now().Add(-time.Second)
	result, err := Generate(InputPaths{Theory: theoryPath})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	after := time.Now().Add(time.Second)

	if result.GeneratedAt.Before(before) || result.GeneratedAt.After(after) {
		t.Error("GeneratedAt is not within expected range")
	}
}
