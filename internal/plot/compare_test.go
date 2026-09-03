package plot

import (
	"strings"
	"testing"

	"github.com/ashwnn/chain-reaction/internal/compare"
)

// -------------------------------------------------------------------------- //
// DETERMINISTIC COMPARISON FIXTURE DATA
// -------------------------------------------------------------------------- //

// makeTestComparisonResult returns a deterministic compare.Result for golden testing.
// It includes all five KG families with varying coverage levels to exercise all code paths.
func makeTestComparisonResult() *compare.Result {
	// Helper for pointer strings
	strPtr := func(s string) *string { return &s }
	floatPtr := func(f float64) *float64 { return &f }

	return &compare.Result{
		ContractVersion: "comparison.v1",
		Sources: compare.Sources{
			Analysis: &compare.AnalysisSource{
				Path:     "artifacts/scenario-runs/analysis.json",
				RunSetID: "openai-gpt-5.4-mini-001",
				RunCount: 5,
				Label:    "openai/gpt-5.4-mini",
			},
			Theory: &compare.TheorySource{
				Path:         "theory/comparison-baseline.json",
				BaselineKind: "static_theoretical",
			},
			Scan: &compare.ScanSource{
				Path:         "scan/comparison-baseline.json",
				BaselineKind: "discovery",
			},
		},
		Families: []compare.FamilyResult{
			{
				FamilyID:   "KG-001",
				FamilyName: "Sensitive keys in codebases",
				InScope:    true,
				Runtime: &compare.RuntimeSummary{
					ChainValidatedCount: 5,
					AttemptedCount:      5,
					ReliabilityFraction: floatPtr(1.0),
					CatalogCoverageSample: &compare.CatalogCoverageStats{
						N:    5,
						Mean: 0.95,
						SD:   0.05,
					},
				},
				Steps: []compare.StepResult{
					{StepID: "KG-001-01", Description: "Discover source code repository", TheoryStatus: strPtr("defined"), ScanStatus: strPtr("observed"), RuntimeStatus: strPtr("validated")},
					{StepID: "KG-001-02", Description: "Locate sensitive key in source", TheoryStatus: strPtr("defined"), ScanStatus: strPtr("observed"), RuntimeStatus: strPtr("validated")},
				},
			},
			{
				FamilyID:   "KG-002",
				FamilyName: "Access container registries",
				InScope:    true,
				Runtime: &compare.RuntimeSummary{
					ChainValidatedCount: 5,
					AttemptedCount:      5,
					ReliabilityFraction: floatPtr(1.0),
					CatalogCoverageSample: &compare.CatalogCoverageStats{
						N:    5,
						Mean: 0.90,
						SD:   0.08,
					},
				},
				Steps: []compare.StepResult{
					{StepID: "KG-002-01", Description: "Enumerate accessible registries", TheoryStatus: strPtr("defined"), ScanStatus: strPtr("observed"), RuntimeStatus: strPtr("validated")},
					{StepID: "KG-002-02", Description: "Pull container image", TheoryStatus: strPtr("defined"), ScanStatus: strPtr("observed"), RuntimeStatus: strPtr("validated")},
				},
			},
			{
				FamilyID:   "KG-003",
				FamilyName: "Container escape via privileged",
				InScope:    true,
				Runtime: &compare.RuntimeSummary{
					ChainValidatedCount: 5,
					AttemptedCount:      5,
					ReliabilityFraction: floatPtr(1.0),
					CatalogCoverageSample: &compare.CatalogCoverageStats{
						N:    5,
						Mean: 0.88,
						SD:   0.10,
					},
				},
				Steps: []compare.StepResult{
					{StepID: "KG-003-01", Description: "Identify privileged container", TheoryStatus: strPtr("defined"), ScanStatus: strPtr("observed"), RuntimeStatus: strPtr("validated")},
					{StepID: "KG-003-02", Description: "Escape to host", TheoryStatus: strPtr("defined"), ScanStatus: strPtr("observed"), RuntimeStatus: strPtr("validated")},
				},
			},
			{
				FamilyID:   "KG-004",
				FamilyName: "Access Kubernetes dashboard",
				InScope:    true,
				Runtime: &compare.RuntimeSummary{
					ChainValidatedCount: 2,
					AttemptedCount:      5,
					ReliabilityFraction: floatPtr(0.4),
					CatalogCoverageSample: &compare.CatalogCoverageStats{
						N:    5,
						Mean: 0.55,
						SD:   0.20,
					},
				},
				Steps: []compare.StepResult{
					{StepID: "KG-004-01", Description: "Discover dashboard endpoint", TheoryStatus: strPtr("defined"), ScanStatus: strPtr("observed"), RuntimeStatus: strPtr("validated")},
					{StepID: "KG-004-02", Description: "Authenticate to dashboard", TheoryStatus: strPtr("defined"), ScanStatus: strPtr("observed"), RuntimeStatus: strPtr("not_validated")},
				},
			},
			{
				FamilyID:   "KG-005",
				FamilyName: "Access exposed sensitive resources",
				InScope:    true,
				Runtime: &compare.RuntimeSummary{
					ChainValidatedCount: 5,
					AttemptedCount:      5,
					ReliabilityFraction: floatPtr(1.0),
					CatalogCoverageSample: &compare.CatalogCoverageStats{
						N:    5,
						Mean: 0.92,
						SD:   0.06,
					},
				},
				Steps: []compare.StepResult{
					{StepID: "KG-005-01", Description: "Enumerate exposed services", TheoryStatus: strPtr("defined"), ScanStatus: strPtr("observed"), RuntimeStatus: strPtr("validated")},
					{StepID: "KG-005-02", Description: "Access sensitive data endpoint", TheoryStatus: strPtr("defined"), ScanStatus: strPtr("observed"), RuntimeStatus: strPtr("validated")},
				},
			},
		},
		BlockerSummary: &compare.BlockerSummary{
			BlockedFamilies: []compare.FamilyBlocker{
				{FamilyID: "KG-004", AttemptedCount: 5, ValidatedCount: 2, ReliabilityFraction: 0.4},
			},
			TotalChainsAttempted: 25,
			TotalChainsValidated: 22,
		},
		FailureSummary: &compare.FailureSummary{
			FamiliesWithFailures: []compare.FamilyFailure{
				{FamilyID: "KG-004", AttemptedCount: 5, NeverValidated: 3, PartiallyValidated: 0, PartialRate: 0.6},
			},
		},
	}
}

// makeTestComparisonResultNoRuntime returns a result with only theory and scan data.
func makeTestComparisonResultNoRuntime() *compare.Result {
	strPtr := func(s string) *string { return &s }

	return &compare.Result{
		ContractVersion: "comparison.v1",
		Sources: compare.Sources{
			Theory: &compare.TheorySource{
				Path:         "theory/comparison-baseline.json",
				BaselineKind: "static_theoretical",
			},
			Scan: &compare.ScanSource{
				Path:         "scan/comparison-baseline.json",
				BaselineKind: "discovery",
			},
		},
		Families: []compare.FamilyResult{
			{
				FamilyID:   "KG-001",
				FamilyName: "Sensitive keys in codebases",
				InScope:    true,
				Steps: []compare.StepResult{
					{StepID: "KG-001-01", Description: "Discover source code repository", TheoryStatus: strPtr("defined"), ScanStatus: strPtr("observed")},
					{StepID: "KG-001-02", Description: "Locate sensitive key in source", TheoryStatus: strPtr("defined"), ScanStatus: strPtr("observed")},
				},
			},
		},
	}
}

// makeTestComparisonResultEmpty returns a result with no in-scope families.
func makeTestComparisonResultEmpty() *compare.Result {
	return &compare.Result{
		ContractVersion: "comparison.v1",
		Families:        []compare.FamilyResult{},
	}
}

// -------------------------------------------------------------------------- //
// THEORY-SCAN-RUNTIME GAP CHART TESTS
// -------------------------------------------------------------------------- //

func TestRenderTheoryScanRuntimeGapChart_Deterministic(t *testing.T) {
	result := makeTestComparisonResult()
	got := RenderTheoryScanRuntimeGapChart(result)

	// Verify valid SVG structure
	if !strings.Contains(got, `<svg xmlns="http://www.w3.org/2000/svg"`) {
		t.Error("output does not contain SVG header")
	}
	if !strings.Contains(got, `</svg>`) {
		t.Error("output does not contain SVG footer")
	}

	// Verify title
	if !strings.Contains(got, "Theory vs. Scan vs. Runtime Coverage") {
		t.Error("output missing title")
	}

	// Verify family labels are present
	for _, fam := range []string{"KG-001", "KG-002", "KG-003", "KG-004", "KG-005"} {
		if !strings.Contains(got, fam) {
			t.Errorf("output missing family label %s", fam)
		}
	}

	// Verify legend entries
	if !strings.Contains(got, "Theory") {
		t.Error("output missing Theory legend")
	}
	if !strings.Contains(got, "Scan") {
		t.Error("output missing Scan legend")
	}
	if !strings.Contains(got, "Runtime") {
		t.Error("output missing Runtime legend")
	}

	// The chart should render actual data bars, not just the legend.
	if strings.Count(got, `fill="#999999"`) < 1 {
		t.Error("output missing theory data bars")
	}
	if strings.Count(got, `fill="#E69F00"`) < 1 {
		t.Error("output missing scan data bars")
	}
	if strings.Count(got, `fill="#009E73"`) < 1 {
		t.Error("output missing runtime validated data bars")
	}
	if !strings.Contains(got, `fill="#F0E442"`) {
		t.Error("output missing partial runtime bar color")
	}

	assertGoldenMatch(t, "comparison-gap-chart", got)
}

func TestRenderTheoryScanRuntimeGapChart_NoRuntime(t *testing.T) {
	result := makeTestComparisonResultNoRuntime()
	got := RenderTheoryScanRuntimeGapChart(result)

	if !strings.Contains(got, `<svg xmlns="http://www.w3.org/2000/svg"`) {
		t.Error("output should still produce valid SVG without runtime data")
	}

	// Should still show family data
	if !strings.Contains(got, "KG-001") {
		t.Error("output missing family label KG-001")
	}
}

func TestRenderTheoryScanRuntimeGapChart_Empty(t *testing.T) {
	result := makeTestComparisonResultEmpty()
	got := RenderTheoryScanRuntimeGapChart(result)

	if !strings.Contains(got, "No in-scope family data") {
		t.Error("empty data should show 'No in-scope family data' message")
	}
}

// -------------------------------------------------------------------------- //
// STEP-LEVEL HEATMAP TESTS
// -------------------------------------------------------------------------- //

func TestRenderStepLevelHeatmap_Deterministic(t *testing.T) {
	result := makeTestComparisonResult()
	got := RenderStepLevelHeatmap(result)

	// Verify valid SVG structure
	if !strings.Contains(got, `<svg xmlns="http://www.w3.org/2000/svg"`) {
		t.Error("output does not contain SVG header")
	}
	if !strings.Contains(got, `</svg>`) {
		t.Error("output does not contain SVG footer")
	}

	// Verify title
	if !strings.Contains(got, "Step-Level Status Heatmap") {
		t.Error("output missing title")
	}

	// Verify family labels
	for _, fam := range []string{"KG-001", "KG-002", "KG-004"} {
		if !strings.Contains(got, fam) {
			t.Errorf("output missing family label %s", fam)
		}
	}

	// Verify column headers
	if !strings.Contains(got, "Theory") {
		t.Error("output missing Theory column header")
	}
	if !strings.Contains(got, "Scan") {
		t.Error("output missing Scan column header")
	}
	if !strings.Contains(got, "Runtime") {
		t.Error("output missing Runtime column header")
	}

	// Verify legend entries
	if !strings.Contains(got, "T = Theoretical") {
		t.Error("output missing theoretical legend")
	}
	if !strings.Contains(got, "O = Observed") {
		t.Error("output missing observed legend")
	}
	if !strings.Contains(got, "V = Validated") {
		t.Error("output missing validated legend")
	}
	if !strings.Contains(got, "X = Not validated") {
		t.Error("output missing not-validated legend")
	}

	assertGoldenMatch(t, "comparison-step-heatmap", got)
}

func TestRenderStepLevelHeatmap_NoRuntime(t *testing.T) {
	result := makeTestComparisonResultNoRuntime()
	got := RenderStepLevelHeatmap(result)

	if !strings.Contains(got, `<svg xmlns="http://www.w3.org/2000/svg"`) {
		t.Error("output should still produce valid SVG without runtime data")
	}

	// Should show step-level data for theory/scan only
	if !strings.Contains(got, "KG-001") {
		t.Error("output missing family label KG-001")
	}
}

func TestRenderStepLevelHeatmap_Empty(t *testing.T) {
	result := makeTestComparisonResultEmpty()
	got := RenderStepLevelHeatmap(result)

	if !strings.Contains(got, "No in-scope family data") {
		t.Error("empty data should show 'No in-scope family data' message")
	}
}

// -------------------------------------------------------------------------- //
// FAMILY CHAIN STATUS CHART TESTS
// -------------------------------------------------------------------------- //

func TestRenderFamilyChainStatusChart_Deterministic(t *testing.T) {
	result := makeTestComparisonResult()
	got := RenderFamilyChainStatusChart(result)

	// Verify valid SVG structure
	if !strings.Contains(got, `<svg xmlns="http://www.w3.org/2000/svg"`) {
		t.Error("output does not contain SVG header")
	}
	if !strings.Contains(got, `</svg>`) {
		t.Error("output does not contain SVG footer")
	}

	// Verify title
	if !strings.Contains(got, "Per-Family Chain Validation Status") {
		t.Error("output missing title")
	}

	// Verify family labels
	for _, fam := range []string{"KG-001", "KG-002", "KG-003", "KG-004", "KG-005"} {
		if !strings.Contains(got, fam) {
			t.Errorf("output missing family label %s", fam)
		}
	}

	// Verify reliability indicators (percentage labels)
	if !strings.Contains(got, "100%") {
		t.Error("output missing 100% reliability label for validated families")
	}
	if !strings.Contains(got, "40%") {
		t.Error("output missing 40% reliability label for KG-004")
	}

	// Verify legend entries
	if !strings.Contains(got, "Validated") {
		t.Error("output missing Validated legend")
	}
	if !strings.Contains(got, "Partial") {
		t.Error("output missing Partial legend")
	}
	if !strings.Contains(got, "Blocked") {
		t.Error("output missing Blocked legend")
	}

	assertGoldenMatch(t, "comparison-chain-status", got)
}

func TestRenderFamilyChainStatusChart_NoRuntime(t *testing.T) {
	result := makeTestComparisonResultNoRuntime()
	got := RenderFamilyChainStatusChart(result)

	if !strings.Contains(got, `<svg xmlns="http://www.w3.org/2000/svg"`) {
		t.Error("output should still produce valid SVG without runtime data")
	}

	// Should show families with "no runtime" status
	if !strings.Contains(got, "KG-001") {
		t.Error("output missing family label KG-001")
	}
	if !strings.Contains(got, "no runtime") {
		t.Error("output should indicate 'no runtime' for families without runtime data")
	}
}

func TestRenderFamilyChainStatusChart_Empty(t *testing.T) {
	result := makeTestComparisonResultEmpty()
	got := RenderFamilyChainStatusChart(result)

	if !strings.Contains(got, "No in-scope family data") {
		t.Error("empty data should show 'No in-scope family data' message")
	}
}
