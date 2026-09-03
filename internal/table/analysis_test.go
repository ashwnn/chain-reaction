package table

import (
	"math"
	"strings"
	"testing"

	"github.com/ashwnn/chain-reaction/internal/analysis"
)

// -------------------------------------------------------------------------- //
// DETERMINISTIC FIXTURE DATA (same as plot package for consistency)
// -------------------------------------------------------------------------- //

func makeTestRunSet() *analysis.RunSet {
	scValues := []float64{0.80, 1.00, 1.00, 0.80, 0.80}
	svrValues := []float64{0.85, 0.95, 0.95, 0.80, 0.85}
	ttcValues := []float64{1250.5, 980.2, 1100.0, 1300.8, 1050.0}

	rs := &analysis.RunSet{
		ID:        "openai-gpt-5.4-mini-001",
		Label:     "openai/gpt-5.4-mini",
		SourceDir: "artifacts/scenario-runs/run-sets/openai-gpt-5.4-mini-001",
		RunCount:  5,
		RunIDs:    []string{"r01", "r02", "r03", "r04", "r05"},
		PerFamily: []analysis.FamilyStats{
			{FamilyID: "KG-001", Validated: 5, Attempted: 5, CatalogCoverageSample: analysis.DescriptiveStats{N: 5, Mean: 0.95, SD: 0.05, Min: 0.90, Max: 1.00}},
			{FamilyID: "KG-002", Validated: 5, Attempted: 5, CatalogCoverageSample: analysis.DescriptiveStats{N: 5, Mean: 0.90, SD: 0.08, Min: 0.80, Max: 1.00}},
			{FamilyID: "KG-003", Validated: 5, Attempted: 5, CatalogCoverageSample: analysis.DescriptiveStats{N: 5, Mean: 0.88, SD: 0.10, Min: 0.75, Max: 1.00}},
			{FamilyID: "KG-004", Validated: 2, Attempted: 5, CatalogCoverageSample: analysis.DescriptiveStats{N: 5, Mean: 0.55, SD: 0.20, Min: 0.30, Max: 0.80}},
			{FamilyID: "KG-005", Validated: 5, Attempted: 5, CatalogCoverageSample: analysis.DescriptiveStats{N: 5, Mean: 0.92, SD: 0.06, Min: 0.85, Max: 1.00}},
		},
	}

	rs.Global.SC = analysis.ComputeSampleStats(scValues)
	rs.Global.CatalogStepCoverage = analysis.ComputeSampleStats(svrValues)
	ttcStats := analysis.ComputeDescriptiveStats(ttcValues)
	ttcStats.VarianceFlag = analysis.ComputeVarianceFlag(ttcStats.CV, ttcStats.N)
	rs.Global.TTC = &ttcStats

	for i := range rs.PerFamily {
		f := &rs.PerFamily[i]
		if f.Attempted > 0 {
			frac := float64(f.Validated) / float64(f.Attempted)
			f.ReliabilityFraction = &frac
		}
		f.CatalogCoverageSample.VarianceFlag = analysis.ComputeVarianceFlag(f.CatalogCoverageSample.CV, f.CatalogCoverageSample.N)
	}

	rs.WilcoxonSignedRank = &analysis.WilcoxonResult{
		N:                2,
		Statistic:        0.0,
		PValue:           floatPtr(0.05),
		SignificantAt005: boolPtr(true),
		Interpretation:   "The median SC differs significantly from 0.80 (two-sided p=0.0500).",
	}
	rs.SpearmanCorrelation = &analysis.SpearmanResult{
		N:                5,
		Rho:              0.90,
		PValue:           floatPtr(0.03),
		SignificantAt005: boolPtr(true),
		Interpretation:   "Strong positive monotonic relationship between SC and Catalog Step Coverage (rho=0.900).",
	}

	return rs
}

// -------------------------------------------------------------------------- //
// GLOBAL STATS TABLE TESTS
// -------------------------------------------------------------------------- //

func TestWriteGlobalStatsTable_Deterministic(t *testing.T) {
	rs := makeTestRunSet()
	var b strings.Builder
	WriteGlobalStatsTable(&b, rs)
	got := b.String()

	// Verify structure
	if !strings.Contains(got, "# Analysis: Global Descriptive Statistics") {
		t.Error("missing table title")
	}
	if !strings.Contains(got, "Scenario Coverage (SC)") {
		t.Error("missing SC row")
	}
	if !strings.Contains(got, "Catalog Step Coverage") {
		t.Error("missing catalog-step coverage row")
	}
	if !strings.Contains(got, "Time-to-Chain (TTC, s)") {
		t.Error("missing TTC row")
	}
	if !strings.Contains(got, "Run Set: openai-gpt-5.4-mini-001") {
		t.Error("missing run set identifier")
	}
	if !strings.Contains(got, "Runs: 5") {
		t.Error("missing run count")
	}

	// Verify tabwriter output — tabwriter expands \t to spaces in output,
	// so check for key content presence instead of literal tab bytes.
	if !strings.Contains(got, "Metric") || !strings.Contains(got, "Scenario Coverage") {
		t.Error("output missing table content")
	}
}

func TestWriteGlobalStatsTable_FormatsMeanSD(t *testing.T) {
	rs := &analysis.RunSet{
		ID:        "test-001",
		RunCount:  1,
		SourceDir: "test",
		Global: analysis.GlobalStats{
			SC:                  analysis.ComputeSampleStats([]float64{0.80}),
			CatalogStepCoverage: analysis.ComputeSampleStats([]float64{0.90}),
		},
	}
	var b strings.Builder
	WriteGlobalStatsTable(&b, rs)
	got := b.String()

	// With N=1, SD should be 0, so format should be "0.8000" not "0.8000±0.0000"
	if !strings.Contains(got, "0.8000") {
		t.Errorf("expected exact mean without ± when SD=0\n%s", got)
	}
	if strings.Contains(got, "0.8000±") {
		t.Errorf("should not contain ± format when SD=0\n%s", got)
	}
}

func TestWriteGlobalStatsTable_WithVarianceFlags(t *testing.T) {
	rs := makeTestRunSet()
	var b strings.Builder
	WriteGlobalStatsTable(&b, rs)
	got := b.String()

	// Variance flags should be populated for the test data
	if !strings.Contains(got, "moderate") && !strings.Contains(got, "low") {
		t.Error("expected variance flags to be present in output")
	}
}

// -------------------------------------------------------------------------- //
// FAMILY RELIABILITY TABLE TESTS
// -------------------------------------------------------------------------- //

func TestWriteFamilyReliabilityTable_Deterministic(t *testing.T) {
	rs := makeTestRunSet()
	var b strings.Builder
	WriteFamilyReliabilityTable(&b, rs)
	got := b.String()

	if !strings.Contains(got, "# Analysis: Per-Family Chain Reliability") {
		t.Error("missing table title")
	}
	// Tabwriter expands \t to spaces, so check for column header presence.
	if !strings.Contains(got, "Family") || !strings.Contains(got, "Validated") || !strings.Contains(got, "Attempted") || !strings.Contains(got, "Reliability") {
		t.Error("missing header columns")
	}

	// Verify all families appear
	for _, fam := range []string{"KG-001", "KG-002", "KG-003", "KG-004", "KG-005"} {
		if !strings.Contains(got, fam) {
			t.Errorf("missing family %s", fam)
		}
	}

	// Verify 100% reliability for families with all 5 validated
	if !strings.Contains(got, "100%") {
		t.Error("missing 100% reliability values")
	}

	// Verify KG-004 partial reliability (2/5 = 40%)
	if !strings.Contains(got, "40%") {
		t.Error("missing 40% reliability for KG-004")
	}

	// Verify catalog coverage mean±SD is present
	if !strings.Contains(got, "±") {
		t.Error("missing catalog coverage mean±SD values")
	}
}

func TestWriteFamilyReliabilityTable_SortedByFamilyID(t *testing.T) {
	rs := &analysis.RunSet{
		ID:        "test-002",
		RunCount:  3,
		SourceDir: "test",
		PerFamily: []analysis.FamilyStats{
			{FamilyID: "KG-003", Validated: 1, Attempted: 2},
			{FamilyID: "KG-001", Validated: 2, Attempted: 2},
			{FamilyID: "KG-005", Validated: 0, Attempted: 2},
		},
	}
	var b strings.Builder
	WriteFamilyReliabilityTable(&b, rs)
	got := b.String()

	// Find positions of family IDs
	posKG001 := strings.Index(got, "KG-001")
	posKG003 := strings.Index(got, "KG-003")
	posKG005 := strings.Index(got, "KG-005")

	if posKG001 == -1 || posKG003 == -1 || posKG005 == -1 {
		t.Fatal("one or more families not found")
	}

	if posKG001 > posKG003 || posKG003 > posKG005 {
		t.Errorf("families should be sorted KG-001 < KG-003 < KG-005:\n%s", got)
	}
}

func TestWriteFamilyReliabilityTable_Empty(t *testing.T) {
	rs := &analysis.RunSet{
		ID:        "empty-001",
		RunCount:  0,
		SourceDir: "test",
		PerFamily: nil,
	}
	var b strings.Builder
	WriteFamilyReliabilityTable(&b, rs)
	got := b.String()

	// Should still produce the header
	if !strings.Contains(got, "# Analysis: Per-Family Chain Reliability") {
		t.Error("empty run set should still produce table with header")
	}
}

// -------------------------------------------------------------------------- //
// STATISTICAL TESTS TABLE TESTS
// -------------------------------------------------------------------------- //

func TestWriteStatisticalTestsTable_BothTestsPresent(t *testing.T) {
	rs := makeTestRunSet()
	var b strings.Builder
	WriteStatisticalTestsTable(&b, rs)
	got := b.String()

	if !strings.Contains(got, "# Analysis: Statistical Tests") {
		t.Error("missing table title")
	}
	if !strings.Contains(got, "Wilcoxon Signed-Rank") {
		t.Error("missing Wilcoxon row")
	}
	if !strings.Contains(got, "Spearman Correlation") {
		t.Error("missing Spearman row")
	}
	if !strings.Contains(got, "H₀: median SC = 0.80") {
		t.Error("missing Wilcoxon hypothesis")
	}
	if !strings.Contains(got, "SC vs catalog-step coverage") {
		t.Error("missing Spearman subtitle")
	}
}

func TestWriteStatisticalTestsTable_OnlyWilcoxon(t *testing.T) {
	rs := &analysis.RunSet{
		ID:        "wilcoxon-only",
		RunCount:  5,
		SourceDir: "test",
		WilcoxonSignedRank: &analysis.WilcoxonResult{
			N:                3,
			Statistic:        2.0,
			PValue:           floatPtr(0.1),
			SignificantAt005: boolPtr(false),
			Interpretation:   "No significant difference.",
		},
		// SpearmanCorrelation is nil
	}
	var b strings.Builder
	WriteStatisticalTestsTable(&b, rs)
	got := b.String()

	if !strings.Contains(got, "Wilcoxon Signed-Rank") {
		t.Error("should contain Wilcoxon row")
	}
	// The implementation intentionally writes an "insufficient data" Spearman row
	// so users know WHY the test wasn't computed.
	if !strings.Contains(got, "insufficient data") {
		t.Error("Spearman section should explain insufficient data")
	}
}

func TestWriteStatisticalTestsTable_NoTests(t *testing.T) {
	rs := &analysis.RunSet{
		ID:        "no-tests",
		RunCount:  2,
		SourceDir: "test",
		// Both test results are nil
	}
	var buf strings.Builder
	WriteStatisticalTestsTable(&buf, rs)
	got := buf.String()

	if !strings.Contains(got, "No statistical tests computed") {
		t.Error("should show 'No statistical tests computed' message")
	}
	if !strings.Contains(got, "Wilcoxon signed-rank requires") {
		t.Error("should explain Wilcoxon threshold")
	}
}

func TestWriteStatisticalTestsTable_PValueFormat(t *testing.T) {
	rs := &analysis.RunSet{
		ID:        "pval-test",
		RunCount:  5,
		SourceDir: "test",
		WilcoxonSignedRank: &analysis.WilcoxonResult{
			N:                4,
			Statistic:        1.5,
			PValue:           floatPtr(0.0234),
			SignificantAt005: boolPtr(true),
		},
	}
	var b strings.Builder
	WriteStatisticalTestsTable(&b, rs)
	got := b.String()

	// P-value should be formatted to 4 decimal places
	if !strings.Contains(got, "0.0234") {
		t.Error("p-value should be formatted to 4 decimal places")
	}
}

// -------------------------------------------------------------------------- //
// RAW VALUES TABLE TESTS
// -------------------------------------------------------------------------- //

func TestWriteRawValuesTable_Deterministic(t *testing.T) {
	rs := makeTestRunSet()
	var b strings.Builder
	WriteRawValuesTable(&b, rs)
	got := b.String()

	if !strings.Contains(got, "# Analysis: Per-Run Raw Values") {
		t.Error("missing table title")
	}
	// Tabwriter expands \t to spaces, so check for column presence.
	if !strings.Contains(got, "Run") || !strings.Contains(got, "SC") || !strings.Contains(got, "Catalog Step Coverage") {
		t.Error("missing column headers")
	}

	// Should have 5 run rows
	count := strings.Count(got, "r0")
	if count < 5 {
		t.Errorf("expected at least 5 run rows, got %d", count)
	}

	// Should contain actual SC values
	for _, v := range []string{"0.8000", "1.0000"} {
		if !strings.Contains(got, v) {
			t.Errorf("expected SC value %s in output", v)
		}
	}
}

func TestWriteRawValuesTable_WithNaN(t *testing.T) {
	rs := &analysis.RunSet{
		ID:        "nan-test",
		RunCount:  3,
		SourceDir: "test",
		RunIDs:    []string{"r01", "r02", "r03"},
		Global: analysis.GlobalStats{
			SC:                  analysis.ComputeSampleStats([]float64{0.80, 1.00, 0.60}),
			CatalogStepCoverage: analysis.ComputeSampleStats([]float64{0.75, 0.85, 0.70}),
		},
	}
	var b strings.Builder
	WriteRawValuesTable(&b, rs)
	got := b.String()

	// Should not contain NaN representation
	if strings.Contains(got, "NaN") {
		t.Error("output should not contain raw NaN")
	}
}

// -------------------------------------------------------------------------- //
// COMBINED OUTPUT TESTS
// -------------------------------------------------------------------------- //

func TestWriteAllAnalysisTables_ContainsAllSections(t *testing.T) {
	rs := makeTestRunSet()
	var b strings.Builder
	WriteAllAnalysisTables(&b, rs)
	got := b.String()

	sections := []string{
		"# Analysis: Global Descriptive Statistics",
		"# Analysis: Per-Family Chain Reliability",
		"# Analysis: Statistical Tests",
		"# Analysis: Per-Run Raw Values",
	}

	for _, section := range sections {
		if !strings.Contains(got, section) {
			t.Errorf("combined output missing section: %s", section)
		}
	}
}

// -------------------------------------------------------------------------- //
// HELPER TESTS
// -------------------------------------------------------------------------- //

func TestFormatMeanSD_WithSD(t *testing.T) {
	got := formatMeanSD(0.85, 0.05)
	want := "0.8500±0.0500"
	if got != want {
		t.Errorf("formatMeanSD(0.85, 0.05): got %q, want %q", got, want)
	}
}

func TestFormatMeanSD_ZeroSD(t *testing.T) {
	got := formatMeanSD(0.85, 0.0)
	want := "0.8500"
	if got != want {
		t.Errorf("formatMeanSD(0.85, 0.0): got %q, want %q", got, want)
	}
}

func TestPtrStr(t *testing.T) {
	if ptrStr(nil) != "" {
		t.Error("ptrStr(nil) should return empty string")
	}
	s := "hello"
	if ptrStr(&s) != "hello" {
		t.Error("ptrStr(&s) should return 'hello'")
	}
}

func TestSanitizeForMarkdown(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"normal text", "normal text"},
		{"a | b", "a \\| b"},
		{"line1\nline2", "line1 line2"},
		{"a | b\nc | d", "a \\| b c \\| d"},
	}

	for _, tc := range tests {
		got := sanitizeForMarkdown(tc.input)
		if got != tc.expected {
			t.Errorf("sanitizeForMarkdown(%q): got %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestIsNaN(t *testing.T) {
	if !isNaN(math.NaN()) {
		t.Error("isNaN(NaN) should be true")
	}
	if isNaN(1.0) {
		t.Error("isNaN(1.0) should be false")
	}
	if isNaN(0.0) {
		t.Error("isNaN(0.0) should be false")
	}
}

// -------------------------------------------------------------------------- //
// HELPER
// -------------------------------------------------------------------------- //

func floatPtr(v float64) *float64 { return &v }
func boolPtr(v bool) *bool        { return &v }
