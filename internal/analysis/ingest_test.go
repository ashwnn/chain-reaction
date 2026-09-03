package analysis

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestIngestFiltersByRunID(t *testing.T) {
	rootDir := t.TempDir()

	writeMetricsFixture(t, rootDir, "2026-04-10T140000Z-react-001", 0.8, []IngestFamilyResult{
		{FamilyID: "KG-001", ChainValidated: true, TotalSteps: 3, ValidatedSteps: 3},
		{FamilyID: "KG-005", ChainValidated: false, TotalSteps: 3, ValidatedSteps: 1},
	})
	writeMetricsFixture(t, rootDir, "2026-04-10T143000Z-react-002", 0.6, []IngestFamilyResult{
		{FamilyID: "KG-001", ChainValidated: true, TotalSteps: 3, ValidatedSteps: 3},
		{FamilyID: "KG-005", ChainValidated: false, TotalSteps: 3, ValidatedSteps: 0},
	})

	rs, err := Ingest(IngestOptions{
		RootDir:     rootDir,
		RunSetID:    "checkpoint",
		RunIDFilter: []string{"2026-04-10T143000Z-react-002"},
	})
	if err != nil {
		t.Fatalf("Ingest returned error: %v", err)
	}

	if rs.RunCount != 1 {
		t.Fatalf("expected one filtered run, got %d", rs.RunCount)
	}
	if len(rs.RunIDs) != 1 || rs.RunIDs[0] != "2026-04-10T143000Z-react-002" {
		t.Fatalf("unexpected run IDs: %#v", rs.RunIDs)
	}
	if rs.Global.SC.N != 1 || rs.Global.SC.Mean != 0.6 {
		t.Fatalf("unexpected SC stats: %#v", rs.Global.SC)
	}
}

func TestIngestReturnsNotFoundForMissingFilteredRuns(t *testing.T) {
	rootDir := t.TempDir()
	writeMetricsFixture(t, rootDir, "2026-04-10T140000Z-react-001", 0.8, []IngestFamilyResult{
		{FamilyID: "KG-001", ChainValidated: true, TotalSteps: 3, ValidatedSteps: 3},
	})

	_, err := Ingest(IngestOptions{
		RootDir:     rootDir,
		RunIDFilter: []string{"2026-04-10T143000Z-react-002"},
	})
	if err == nil {
		t.Fatal("expected filtered ingest to fail when no requested run IDs are present")
	}

	ingestErr, ok := err.(*IngestError)
	if !ok {
		t.Fatalf("expected IngestError, got %T", err)
	}
	if ingestErr.Kind != ErrKindNotFound {
		t.Fatalf("expected not found error, got %q", ingestErr.Kind)
	}
}

func writeMetricsFixture(t *testing.T, rootDir, runID string, scenarioRate float64, families []IngestFamilyResult) {
	t.Helper()

	runDir := filepath.Join(rootDir, runID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	payload := MetricsForIngestion{
		ScenarioCoverage: &IngestScenarioCoverage{
			ScenarioRate:        &scenarioRate,
			CatalogStepCoverage: &scenarioRate, // same as SC for deterministic fixture data
			Families:            families,
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal metrics fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "validation-metrics.json"), data, 0o644); err != nil {
		t.Fatalf("write metrics fixture: %v", err)
	}
}
