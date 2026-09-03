package report

import (
	"strings"
	"testing"

	"github.com/ashwnn/chain-reaction/internal/analysis"
	"github.com/ashwnn/chain-reaction/internal/compare"
)

func TestWriteAnalysisHTML(t *testing.T) {
	rs := &analysis.RunSet{
		ID:        "html-001",
		Label:     "test",
		SourceDir: "artifacts/scenario-runs",
		RunCount:  5,
		RunIDs:    []string{"r01", "r02", "r03", "r04", "r05"},
		Global: analysis.GlobalStats{
			SC:                  analysis.ComputeSampleStats([]float64{0.80, 1.00, 1.00, 0.80, 0.80}),
			CatalogStepCoverage: analysis.ComputeSampleStats([]float64{0.85, 0.95, 0.95, 0.80, 0.85}),
		},
	}
	var b strings.Builder
	if err := WriteAnalysisHTML(&b, rs); err != nil {
		t.Fatal(err)
	}
	got := b.String()
	for _, want := range []string{"<!DOCTYPE html>", "Chain Reaction Analysis Report", "<svg", "Scenario Coverage", "</html>"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q", want)
		}
	}
}

func TestWriteComparisonHTML(t *testing.T) {
	strPtr := func(s string) *string { return &s }
	result := &compare.Result{
		ContractVersion: "comparison.v1",
		Families: []compare.FamilyResult{{
			FamilyID:   "KG-001",
			FamilyName: "RBAC Over-Provisioning",
			InScope:    true,
			Steps: []compare.StepResult{
				{StepID: "KG-001-S1", Description: "Identify SA", TheoryStatus: strPtr("theoretical"), ScanStatus: strPtr("observed"), RuntimeStatus: strPtr("validated")},
			},
		}},
	}
	var b strings.Builder
	if err := WriteComparisonHTML(&b, result); err != nil {
		t.Fatal(err)
	}
	got := b.String()
	for _, want := range []string{"<!DOCTYPE html>", "Chain Reaction Comparison Report", "KG-001", "<svg"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q", want)
		}
	}
}
