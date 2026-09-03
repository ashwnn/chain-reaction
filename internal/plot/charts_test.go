package plot

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ashwnn/chain-reaction/internal/analysis"
)

// -------------------------------------------------------------------------- //
// DETERMINISTIC FIXTURE DATA
// -------------------------------------------------------------------------- //

// makeTestRunSet returns a deterministic RunSet matching the openai-gpt-5.4-mini
// 5-run live batch shape for golden testing.
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

	// Compute statistics
	rs.Global.SC = analysis.ComputeSampleStats(scValues)
	rs.Global.CatalogStepCoverage = analysis.ComputeSampleStats(svrValues)
	ttcStats := analysis.ComputeDescriptiveStats(ttcValues)
	ttcStats.VarianceFlag = analysis.ComputeVarianceFlag(ttcStats.CV, ttcStats.N)
	rs.Global.TTC = &ttcStats

	// Set reliability fractions for each family
	for i := range rs.PerFamily {
		f := &rs.PerFamily[i]
		if f.Attempted > 0 {
			frac := float64(f.Validated) / float64(f.Attempted)
			f.ReliabilityFraction = &frac
		}
		f.CatalogCoverageSample.VarianceFlag = analysis.ComputeVarianceFlag(f.CatalogCoverageSample.CV, f.CatalogCoverageSample.N)
	}

	// Wilcoxon test (N=5, mixed positive/negative differences vs 0.80)
	// Values: 0.80, 1.00, 1.00, 0.80, 0.80 → diffs: 0, 0.20, 0.20, 0, 0 → non-zero: 3
	wilcoxonResult := &analysis.WilcoxonResult{
		N:                2,
		Statistic:        0.0,
		PValue:           floatPtr(0.05),
		SignificantAt005: boolPtr(true),
		Interpretation:   "The median SC differs significantly from 0.80 (two-sided p=0.0500).",
	}
	rs.WilcoxonSignedRank = wilcoxonResult

	// Spearman correlation
	spearmanResult := &analysis.SpearmanResult{
		N:                5,
		Rho:              0.90,
		PValue:           floatPtr(0.03),
		SignificantAt005: boolPtr(true),
		Interpretation:   "Strong positive monotonic relationship between SC and Catalog Step Coverage (rho=0.900).",
	}
	rs.SpearmanCorrelation = spearmanResult

	return rs
}

// makeSparseRunSet returns a RunSet with minimal data (below test thresholds).
func makeSparseRunSet() *analysis.RunSet {
	return &analysis.RunSet{
		ID:        "sparse-001",
		SourceDir: "artifacts/scenario-runs",
		RunCount:  2,
		RunIDs:    []string{"r01", "r02"},
		Global: analysis.GlobalStats{
			SC:                  analysis.ComputeSampleStats([]float64{0.80, 0.60}),
			CatalogStepCoverage: analysis.ComputeSampleStats([]float64{0.75, 0.55}),
		},
		PerFamily: []analysis.FamilyStats{
			{FamilyID: "KG-001", Validated: 1, Attempted: 2},
		},
	}
}

// -------------------------------------------------------------------------- //
// GOLDEN FILE HELPERS
// -------------------------------------------------------------------------- //

// updateGolden is set by the UPDATE_GOLDEN env var to regenerate golden files.
const updateGoldenEnv = "UPDATE_GOLDEN_PLOT"

func updateGolden() bool {
	return os.Getenv(updateGoldenEnv) == "1"
}

// loadGolden returns the content of a golden file, or empty string if it doesn't exist.
func loadGolden(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ""
		}
		t.Fatalf("read golden file %s: %v", path, err)
	}
	return string(data)
}

// saveGolden writes content to a golden file.
func saveGolden(t *testing.T, name string, content string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write golden file %s: %v", path, err)
	}
}

// assertGoldenMatch verifies that got matches the golden file content.
// If UPDATE_GOLDEN_PLOT=1, updates the golden file instead of failing.
func assertGoldenMatch(t *testing.T, name string, got string) {
	t.Helper()
	want := loadGolden(t, name)
	if updateGolden() {
		saveGolden(t, name, got)
		return
	}
	if want == "" {
		t.Fatalf("golden file testdata/%s.golden does not exist; set %s=1 to generate it", name, updateGoldenEnv)
	}
	if got != want {
		t.Errorf("golden mismatch for %s:\n--- GOT ---\n%s\n--- WANT ---\n%s", name, got, want)
	}
}

// -------------------------------------------------------------------------- //
// GLOBAL SC/Catalog Step Coverage BAR CHART TESTS
// -------------------------------------------------------------------------- //

func TestRenderGlobalSCCatalogCoverageBarChart_Deterministic(t *testing.T) {
	rs := makeTestRunSet()
	got := RenderGlobalSCCatalogCoverageBarChart(rs)

	// Verify it produces valid SVG
	if !strings.Contains(got, `<svg xmlns="http://www.w3.org/2000/svg"`) {
		t.Error("output does not contain SVG header")
	}
	if !strings.Contains(got, `</svg>`) {
		t.Error("output does not contain SVG footer")
	}

	// Verify key elements are present
	if !strings.Contains(got, "Scenario Coverage") {
		t.Error("output missing title")
	}
	if !strings.Contains(got, "SC") {
		t.Error("output missing SC bar label")
	}
	if !strings.Contains(got, "Catalog Step Coverage") {
		t.Error("output missing Catalog Step Coverage bar label")
	}
	if !strings.Contains(got, "target (0.80)") {
		t.Error("output missing target reference line")
	}

	// Verify it references the run count
	if !strings.Contains(got, "N = 5") {
		t.Error("output missing N annotation")
	}

	assertGoldenMatch(t, "sc-catalog-step-coverage-bar", got)
}

func TestRenderGlobalSCCatalogCoverageBarChart_Sparse(t *testing.T) {
	rs := makeSparseRunSet()
	got := RenderGlobalSCCatalogCoverageBarChart(rs)

	if !strings.Contains(got, `<svg xmlns="http://www.w3.org/2000/svg"`) {
		t.Error("sparse data should still produce valid SVG")
	}
}

// -------------------------------------------------------------------------- //
// FAMILY RELIABILITY BAR CHART TESTS
// -------------------------------------------------------------------------- //

func TestRenderFamilyReliabilityBarChart_Deterministic(t *testing.T) {
	rs := makeTestRunSet()
	got := RenderFamilyReliabilityBarChart(rs)

	if !strings.Contains(got, `<svg xmlns="http://www.w3.org/2000/svg"`) {
		t.Error("output does not contain SVG header")
	}
	if !strings.Contains(got, `</svg>`) {
		t.Error("output does not contain SVG footer")
	}

	if !strings.Contains(got, "Per-Family Chain Reliability") {
		t.Error("output missing title")
	}

	// Verify family labels are present
	for _, fam := range []string{"KG-001", "KG-002", "KG-003", "KG-004", "KG-005"} {
		if !strings.Contains(got, fam) {
			t.Errorf("output missing family label %s", fam)
		}
	}

	// Verify percentage labels
	if !strings.Contains(got, "100%") {
		t.Error("output missing 100% reliability label")
	}
	if !strings.Contains(got, "40%") {
		t.Error("output missing 40% reliability label for KG-004")
	}

	assertGoldenMatch(t, "family-reliability-bar", got)
}

func TestRenderFamilyReliabilityBarChart_Empty(t *testing.T) {
	rs := &analysis.RunSet{
		ID:        "empty-001",
		RunCount:  0,
		RunIDs:    nil,
		PerFamily: nil,
	}
	got := RenderFamilyReliabilityBarChart(rs)

	if !strings.Contains(got, "No family data available") {
		t.Error("empty data should show 'No family data available' message")
	}
}

// -------------------------------------------------------------------------- //
// RAW SC VALUES DOT PLOT TESTS
// -------------------------------------------------------------------------- //

func TestRenderRawSCValuesDotPlot_Deterministic(t *testing.T) {
	rs := makeTestRunSet()
	got := RenderRawSCValuesDotPlot(rs)

	if !strings.Contains(got, `<svg xmlns="http://www.w3.org/2000/svg"`) {
		t.Error("output does not contain SVG header")
	}
	if !strings.Contains(got, `</svg>`) {
		t.Error("output does not contain SVG footer")
	}

	if !strings.Contains(got, "Scenario Coverage per Run") {
		t.Error("output missing title")
	}

	// Verify target line
	if !strings.Contains(got, "target (0.80)") {
		t.Error("output missing target reference line")
	}

	// Verify mean line label
	if !strings.Contains(got, "mean=") {
		t.Error("output missing mean reference line")
	}

	// Verify legend entries (SVG text is XML-escaped)
	if !strings.Contains(got, "≥ target") {
		t.Error("output missing legend for at-target runs")
	}
	if !strings.Contains(got, "&lt; target") {
		t.Error("output missing legend for below-target runs")
	}

	assertGoldenMatch(t, "sc-raw-values", got)
}

func TestRenderRawSCValuesDotPlot_Empty(t *testing.T) {
	rs := &analysis.RunSet{
		ID:       "empty-002",
		RunCount: 0,
		Global:   analysis.GlobalStats{},
	}
	got := RenderRawSCValuesDotPlot(rs)

	if !strings.Contains(got, "No run data available") {
		t.Error("empty data should show 'No run data available' message")
	}
}

// -------------------------------------------------------------------------- //
// WILCOXON METRIC DISPLAY TESTS
// -------------------------------------------------------------------------- //

func TestRenderWilcoxonMetricDisplay_HasResult(t *testing.T) {
	rs := makeTestRunSet()
	got := RenderWilcoxonMetricDisplay(rs)

	if !strings.Contains(got, `<svg xmlns="http://www.w3.org/2000/svg"`) {
		t.Error("output does not contain SVG header")
	}
	if !strings.Contains(got, "Wilcoxon Signed-Rank Test") {
		t.Error("output missing title")
	}
	if !strings.Contains(got, "H₀: median SC = 0.80") {
		t.Error("output missing hypothesis label")
	}
	if !strings.Contains(got, "N (non-zero diffs)") {
		t.Error("output missing N label")
	}
	if !strings.Contains(got, "W statistic") {
		t.Error("output missing W statistic label")
	}

	assertGoldenMatch(t, "wilcoxon-result", got)
}

func TestRenderWilcoxonMetricDisplay_NoResult(t *testing.T) {
	rs := makeSparseRunSet()
	// Sparse has 2 runs — Wilcoxon requires >= 5, so it should be nil
	got := RenderWilcoxonMetricDisplay(rs)

	if !strings.Contains(got, "Test not computed (N &lt; 5)") {
		t.Error("nil Wilcoxon result should show 'Test not computed' message")
	}
}

// -------------------------------------------------------------------------- //
// SPEARMAN METRIC DISPLAY TESTS
// -------------------------------------------------------------------------- //

func TestRenderSpearmanMetricDisplay_HasResult(t *testing.T) {
	rs := makeTestRunSet()
	got := RenderSpearmanMetricDisplay(rs)

	if !strings.Contains(got, `<svg xmlns="http://www.w3.org/2000/svg"`) {
		t.Error("output does not contain SVG header")
	}
	if !strings.Contains(got, "Spearman Correlation") {
		t.Error("output missing title")
	}
	if !strings.Contains(got, "SC vs catalog-step coverage") {
		t.Error("output missing subtitle")
	}
	if !strings.Contains(got, "ρ (rho)") {
		t.Error("output missing rho label")
	}

	assertGoldenMatch(t, "spearman-result", got)
}

func TestRenderSpearmanMetricDisplay_NoResult(t *testing.T) {
	rs := makeSparseRunSet()
	// Sparse has 2 runs — Spearman requires >= 3, so it should be nil
	got := RenderSpearmanMetricDisplay(rs)

	if !strings.Contains(got, "Test not computed (N &lt; 3)") {
		t.Error("nil Spearman result should show 'Test not computed' message")
	}
}

// -------------------------------------------------------------------------- //
// TTC BAR CHART TESTS
// -------------------------------------------------------------------------- //

func TestRenderTTCBarchart_HasData(t *testing.T) {
	rs := makeTestRunSet()
	got := RenderTTCBarchart(rs)

	if got == "" {
		t.Fatal("TTC data should produce non-empty SVG")
	}
	if !strings.Contains(got, `<svg xmlns="http://www.w3.org/2000/svg"`) {
		t.Error("output does not contain SVG header")
	}
	if !strings.Contains(got, "Time-to-Chain Statistics") {
		t.Error("output missing title")
	}
	if !strings.Contains(got, "Mean") {
		t.Error("output missing Mean bar label")
	}
	if !strings.Contains(got, "Min") {
		t.Error("output missing Min bar label")
	}
	if !strings.Contains(got, "Max") {
		t.Error("output missing Max bar label")
	}
	if !strings.Contains(got, "N = 5") {
		t.Error("output missing N annotation")
	}

	assertGoldenMatch(t, "ttc-barchart", got)
}

func TestRenderCoverageBoxPlot_Deterministic(t *testing.T) {
	rs := makeTestRunSet()
	got := RenderCoverageBoxPlot(rs)
	if !strings.Contains(got, "Coverage Distributions") {
		t.Error("output missing title")
	}
	if !strings.Contains(got, "SC") || !strings.Contains(got, "Catalog") {
		t.Error("output missing series labels")
	}
	assertGoldenMatch(t, "sc-catalog-coverage-box", got)
}

func TestRenderSpearmanScatterPlot_Deterministic(t *testing.T) {
	rs := makeTestRunSet()
	got := RenderSpearmanScatterPlot(rs)
	if !strings.Contains(got, "SC vs Catalog Step Coverage") {
		t.Error("output missing title")
	}
	if !strings.Contains(got, "ρ") && !strings.Contains(got, "rho") {
		t.Error("output missing rho annotation")
	}
	assertGoldenMatch(t, "sc-catalog-scatter", got)
}

func TestRenderTTCBarchart_NoData(t *testing.T) {
	rs := &analysis.RunSet{
		ID:       "no-ttc",
		RunCount: 3,
		Global:   analysis.GlobalStats{},
		// TTC is nil
	}
	got := RenderTTCBarchart(rs)

	if got != "" {
		t.Error("nil TTC should produce empty string, got:", got[:min(len(got), 50)])
	}
}

// -------------------------------------------------------------------------- //
// RENDERER SVG PRIMITIVES TESTS
// -------------------------------------------------------------------------- //

func TestRenderer_DrawRect(t *testing.T) {
	r := NewRenderer(100, 100)
	got := r.DrawRect(10, 20, 30, 40, "#fff", "#000", 1)
	if !strings.Contains(got, `x="10.00"`) {
		t.Error("rect should have x=10.00")
	}
	if !strings.Contains(got, `y="20.00"`) {
		t.Error("rect should have y=20.00")
	}
	if !strings.Contains(got, `width="30.00"`) {
		t.Error("rect should have width=30.00")
	}
	if !strings.Contains(got, `height="40.00"`) {
		t.Error("rect should have height=40.00")
	}
	if !strings.Contains(got, `fill="#fff"`) {
		t.Error("rect should have fill=#fff")
	}
}

func TestRenderer_DrawText(t *testing.T) {
	r := NewRenderer(100, 100)
	got := r.DrawText(50, 50, "Hello", "title")
	if !strings.Contains(got, `x="50.00"`) {
		t.Error("text should have x=50.00")
	}
	if !strings.Contains(got, `y="50.00"`) {
		t.Error("text should have y=50.00")
	}
	if !strings.Contains(got, `Hello`) {
		t.Error("text should contain 'Hello'")
	}
	if !strings.Contains(got, `text-anchor="middle"`) {
		t.Error("text should be middle-anchored")
	}
}

func TestRenderer_DrawCircle(t *testing.T) {
	r := NewRenderer(100, 100)
	got := r.DrawCircle(50, 50, 5, "#00f", "#000", 1)
	if !strings.Contains(got, `cx="50.00"`) {
		t.Error("circle should have cx=50.00")
	}
	if !strings.Contains(got, `cy="50.00"`) {
		t.Error("circle should have cy=50.00")
	}
	if !strings.Contains(got, `r="5.00"`) {
		t.Error("circle should have r=5.00")
	}
}

func TestRenderer_DrawErrorBar(t *testing.T) {
	r := NewRenderer(100, 100)
	got := r.DrawErrorBar(50, 80, 10, 8, "#000")

	// Error bar has 3 lines: vertical + 2 caps
	count := strings.Count(got, "<line")
	if count != 3 {
		t.Errorf("error bar should have 3 line elements, got %d", count)
	}

	if !strings.Contains(got, `x1="50.00"`) {
		t.Error("error bar should reference x=50.00")
	}
}

func TestRenderer_DataToY(t *testing.T) {
	r := NewRenderer(720, 460)
	x, y, w, h := r.PlotArea()
	tests := []struct {
		val        float64
		minVal     float64
		maxVal     float64
		expectNear float64
	}{
		{0.0, 0.0, 1.0, float64(y + h)},
		{1.0, 0.0, 1.0, float64(y)},
		{0.5, 0.0, 1.0, float64(y) + float64(h)/2},
	}
	_ = x
	_ = w

	for _, tc := range tests {
		got := r.DataToY(tc.val, tc.minVal, tc.maxVal)
		if math.Abs(got-tc.expectNear) > 1.0 {
			t.Errorf("DataToY(%.2f, %.2f, %.2f): got %.2f, want ~%.2f",
				tc.val, tc.minVal, tc.maxVal, got, tc.expectNear)
		}
	}
}

func TestRenderer_DataToX(t *testing.T) {
	r := NewRenderer(720, 460)
	x, _, w, _ := r.PlotArea()

	tests := []struct {
		val    float64
		minVal float64
		maxVal float64
		expect float64
	}{
		{0.0, 0.0, 1.0, float64(x)},
		{1.0, 0.0, 1.0, float64(x + w)},
		{0.5, 0.0, 1.0, float64(x) + float64(w)/2},
	}

	for _, tc := range tests {
		got := r.DataToX(tc.val, tc.minVal, tc.maxVal)
		if math.Abs(got-tc.expect) > 1.0 {
			t.Errorf("DataToX(%.2f, %.2f, %.2f): got %.2f, want ~%.2f",
				tc.val, tc.minVal, tc.maxVal, got, tc.expect)
		}
	}
}

func TestRenderer_EscapeXML(t *testing.T) {
	r := NewRenderer(100, 100)
	tests := []struct {
		input    string
		expected string
	}{
		{"<test>", "&lt;test&gt;"},
		{"a & b", "a &amp; b"},
		{`quote "test"`, "quote &quot;test&quot;"},
		{"normal text", "normal text"},
	}

	for _, tc := range tests {
		got := r.EscapeXML(tc.input)
		if got != tc.expected {
			t.Errorf("EscapeXML(%q): got %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestRenderer_Header(t *testing.T) {
	r := NewRenderer(400, 300)
	h := r.Header()

	if !strings.Contains(h, `<svg xmlns="http://www.w3.org/2000/svg"`) {
		t.Error("header missing SVG namespace")
	}
	if !strings.Contains(h, `viewBox="0 0 400 300"`) {
		t.Error("header missing correct viewBox")
	}
	if !strings.Contains(h, `width="400"`) {
		t.Error("header missing width attribute")
	}
	if !strings.Contains(h, `<style>`) {
		t.Error("header missing embedded styles")
	}
}

func TestRenderer_PlotArea(t *testing.T) {
	r := NewRenderer(720, 460)
	x, y, w, h := r.PlotArea()
	if x <= 0 || y <= 0 || w <= 0 || h <= 0 {
		t.Errorf("PlotArea should be positive, got (%d,%d,%d,%d)", x, y, w, h)
	}
	if x+w >= 720 || y+h >= 460 {
		t.Errorf("PlotArea overflows canvas: (%d,%d,%d,%d)", x, y, w, h)
	}
}

// -------------------------------------------------------------------------- //
// HELPER
// -------------------------------------------------------------------------- //

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func floatPtr(v float64) *float64 { return &v }
func boolPtr(v bool) *bool        { return &v }
