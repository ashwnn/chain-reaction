package analysis

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"testing"
)

// -------------------------------------------------------------------------- //
// WILCOXON SIGNED-RANK TESTS
// -------------------------------------------------------------------------- //

func TestWilcoxonSignedRank_InsufficientData(t *testing.T) {
	tests := []struct {
		name   string
		values []float64
		n      int
	}{
		{"empty", nil, 0},
		{"nil slice", []float64(nil), 0},
		{"single value", []float64{0.8}, 1},
		{"four values (below threshold)", []float64{0.8, 0.8, 0.8, 0.8}, 4},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := wilcoxonSignedRank(tc.values, 0.80)
			if err == nil {
				t.Fatalf("expected errInsufficientData, got result=%+v", result)
			}
			if err != errInsufficientData {
				t.Fatalf("expected errInsufficientData, got %v", err)
			}
		})
	}
}

func TestWilcoxonSignedRank_ZeroDifferences(t *testing.T) {
	// All values exactly equal the hypothesized median → all differences are zero.
	result, err := wilcoxonSignedRank([]float64{0.80, 0.80, 0.80, 0.80, 0.80}, 0.80)
	if err == nil {
		t.Fatalf("expected errInsufficientData for all-zero differences, got result=%+v", result)
	}
}

func TestWilcoxonSignedRank_AllPositiveDifferences(t *testing.T) {
	// All values > median → all ranks go to positive side, W- = 0, W = 0.
	result, err := wilcoxonSignedRank([]float64{0.90, 0.95, 1.00, 1.00, 1.00}, 0.80)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.N != 5 {
		t.Fatalf("expected N=5, got %d", result.N)
	}
	// W should be 0 (all positive, W-=0)
	if result.Statistic != 0 {
		t.Fatalf("expected Statistic=0 (all positive), got %.1f", result.Statistic)
	}
	if result.PValue == nil {
		t.Fatal("expected non-nil PValue")
	}
	if result.SignificantAt005 == nil {
		t.Fatal("expected non-nil SignificantAt005")
	}
}

func TestWilcoxonSignedRank_AllNegativeDifferences(t *testing.T) {
	// All values < median → all ranks go to negative side, W+ = 0, W = 0.
	result, err := wilcoxonSignedRank([]float64{0.60, 0.65, 0.70, 0.75, 0.79}, 0.80)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Statistic != 0 {
		t.Fatalf("expected Statistic=0 (all negative), got %.1f", result.Statistic)
	}
}

func TestWilcoxonSignedRank_MixedDifferences(t *testing.T) {
	// Mixed positive and negative differences.
	// Values: 0.85, 0.82, 0.78, 0.90, 0.88 vs 0.80
	// Diffs:  0.05, 0.02, -0.02, 0.10, 0.08
	// At float64 precision, 0.82-0.80 and 0.80-0.78 are not identical,
	// so the 0.02s are NOT tied. Sorted abs: 0.02, 0.02, 0.05, 0.08, 0.10
	// → ranks: 1, 2, 3, 4, 5 (no ties)
	// Positive diffs (0.05, 0.02, 0.10, 0.08) → ranks [3, 1, 5, 4]: W+ = 13
	// Negative diffs (-0.02) → rank [2]: W- = 2
	// W = min(13, 2) = 2
	result, err := wilcoxonSignedRank([]float64{0.85, 0.82, 0.78, 0.90, 0.88}, 0.80)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.N != 5 {
		t.Fatalf("expected N=5, got %d", result.N)
	}
	// W = min(W+, W-) = min(13, 2) = 2
	if result.Statistic != 2.0 {
		t.Fatalf("expected Statistic=2.0, got %.1f", result.Statistic)
	}
}

func TestWilcoxonSignedRank_NaNFiltering(t *testing.T) {
	// NaN values should be filtered out. 5 values with 1 NaN = 4 non-NaN < threshold.
	result, err := wilcoxonSignedRank([]float64{0.8, 0.9, math.NaN(), 0.7, 0.85}, 0.80)
	if err != errInsufficientData {
		t.Fatalf("expected errInsufficientData with 4 non-NaN values, got err=%v", err)
	}

	// 6 values with 1 NaN = 5 non-NaN → should succeed.
	result, err = wilcoxonSignedRank([]float64{0.8, 0.9, math.NaN(), 0.7, 0.85, 0.88}, 0.80)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Note: values [0.8, 0.9, 0.7, 0.85, 0.88] vs 0.80:
	// diffs: [0, 0.1, -0.1, 0.05, 0.08] → non-zero: [0.1, -0.1, 0.05, 0.08] → N=4
	if result.N != 4 {
		t.Fatalf("expected N=4 (one zero diff skipped), got %d", result.N)
	}
}

func TestWilcoxonSignedRank_NormalApproximation(t *testing.T) {
	// With n >= 10, normal approximation should be used.
	// Construct 10 values that are all > 0.80 so W = 0.
	values := make([]float64, 10)
	for i := range values {
		values[i] = 0.80 + 0.01 + float64(i)*0.01
	}
	result, err := wilcoxonSignedRank(values, 0.80)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.N != 10 {
		t.Fatalf("expected N=10, got %d", result.N)
	}
	// Statistic should be 0 (all positive)
	if result.Statistic != 0 {
		t.Fatalf("expected Statistic=0, got %.1f", result.Statistic)
	}
	// With n=10 and all positive, p should be very small.
	if result.PValue == nil || *result.PValue >= 0.05 {
		t.Fatalf("expected significant result (p < 0.05), got p=%v", result.PValue)
	}
	if result.Interpretation == "" {
		t.Fatal("expected non-empty interpretation")
	}
}

func TestWilcoxonSignedRank_TieWithMedian(t *testing.T) {
	// Values where some equal the hypothesized median (zero diff, skipped).
	// 0.80 values produce zero diffs; 0.90 values produce positive diffs.
	// Rank only the non-zero diffs.
	result, err := wilcoxonSignedRank([]float64{0.80, 0.80, 0.90, 0.95, 1.00}, 0.80)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should have 3 non-zero positive differences (0.10, 0.15, 0.20)
	if result.N != 3 {
		t.Fatalf("expected N=3 (zero diffs skipped), got %d", result.N)
	}
}

// -------------------------------------------------------------------------- //
// SPEARMAN CORRELATION TESTS
// -------------------------------------------------------------------------- //

func TestSpearmanCorrelation_MismatchedLengths(t *testing.T) {
	_, err := spearmanCorrelation([]float64{1, 2, 3}, []float64{1, 2})
	if err == nil {
		t.Fatal("expected error for mismatched lengths")
	}
}

func TestSpearmanCorrelation_InsufficientData(t *testing.T) {
	tests := []struct {
		name string
		x    []float64
		y    []float64
	}{
		{"empty", nil, nil},
		{"nil", []float64(nil), []float64(nil)},
		{"single", []float64{1.0}, []float64{2.0}},
		{"two values", []float64{1.0, 2.0}, []float64{2.0, 3.0}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := spearmanCorrelation(tc.x, tc.y)
			if err == nil {
				t.Fatalf("expected errInsufficientData, got result=%+v", result)
			}
			if err != errInsufficientData {
				t.Fatalf("expected errInsufficientData, got %v", err)
			}
		})
	}
}

func TestSpearmanCorrelation_PerfectPositive(t *testing.T) {
	// Perfect positive monotonic: larger X always means larger Y.
	x := []float64{1.0, 2.0, 3.0, 4.0, 5.0}
	y := []float64{2.0, 4.0, 6.0, 8.0, 10.0}
	result, err := spearmanCorrelation(x, y)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if math.Abs(result.Rho-1.0) > 0.0001 {
		t.Fatalf("expected rho≈1.0 for perfect positive correlation, got %.4f", result.Rho)
	}
	if result.N != 5 {
		t.Fatalf("expected N=5, got %d", result.N)
	}
}

func TestSpearmanCorrelation_PerfectNegative(t *testing.T) {
	// Perfect negative monotonic: larger X always means smaller Y.
	x := []float64{1.0, 2.0, 3.0, 4.0, 5.0}
	y := []float64{10.0, 8.0, 6.0, 4.0, 2.0}
	result, err := spearmanCorrelation(x, y)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if math.Abs(result.Rho+1.0) > 0.0001 {
		t.Fatalf("expected rho≈-1.0 for perfect negative correlation, got %.4f", result.Rho)
	}
}

func TestSpearmanCorrelation_ConstantSeries(t *testing.T) {
	x := []float64{1.0, 2.0, 3.0, 4.0, 5.0}
	y := []float64{5.0, 5.0, 5.0, 5.0, 5.0}

	result, err := spearmanCorrelation(x, y)
	if err != errInsufficientData {
		t.Fatalf("expected errInsufficientData, got result=%+v err=%v", result, err)
	}
}

func TestSpearmanCorrelation_WithTies(t *testing.T) {
	// Ties in X: [1, 1, 2, 3] vs [1, 2, 3, 4]
	// Ranks for X: [1, 1, 3, 4] (ties get average rank 1.5)
	// Ranks for Y: [1, 2, 3, 4]
	x := []float64{1.0, 1.0, 2.0, 3.0}
	y := []float64{1.0, 2.0, 3.0, 4.0}
	result, err := spearmanCorrelation(x, y)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Rho <= 0 || result.Rho >= 1 {
		t.Fatalf("expected positive rho between 0 and 1, got %.4f", result.Rho)
	}
}

func TestSpearmanCorrelation_NaNFiltering(t *testing.T) {
	// Paired NaN values should be filtered out.
	// y has no NaN values, so filtering leaves: (1,2), (NaN,4), (3,NaN), (4,8), (5,10)
	// → valid pairs: (1,2), (4,8), (5,10) → N=3 (NOT 2).
	x := []float64{1.0, math.NaN(), 3.0, 4.0, 5.0}
	y := []float64{2.0, 4.0, math.NaN(), 8.0, 10.0}
	result, err := spearmanCorrelation(x, y)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 3 valid pairs → n >= 3, succeeds with N=3.
	if result.N != 3 {
		t.Fatalf("expected N=3, got %d", result.N)
	}

	// (5,10) is already included above. The NaN filtering test verifies
	// that NaN pairs are correctly excluded from the paired observation count.
	x = []float64{1.0, math.NaN(), 3.0, 4.0, 5.0}
	y = []float64{2.0, 4.0, 6.0, math.NaN(), 10.0}
	result, err = spearmanCorrelation(x, y)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.N != 3 {
		t.Fatalf("expected N=3 after NaN filter, got %d", result.N)
	}
}

func TestSpearmanCorrelation_ThreeValues(t *testing.T) {
	// Minimum valid N=3.
	x := []float64{0.80, 1.00, 0.60}
	y := []float64{0.80, 0.90, 0.50}
	result, err := spearmanCorrelation(x, y)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.N != 3 {
		t.Fatalf("expected N=3, got %d", result.N)
	}
}

func TestSpearmanCorrelation_ConstantPairedSeries(t *testing.T) {
	x := []float64{0.80, 0.80, 0.80, 0.80, 0.80}
	y := []float64{0.80, 0.80, 0.80, 0.80, 0.80}

	result, err := spearmanCorrelation(x, y)
	if err != errInsufficientData {
		t.Fatalf("expected errInsufficientData, got result=%+v err=%v", result, err)
	}
}

// -------------------------------------------------------------------------- //
// RANK FUNCTION TESTS
// -------------------------------------------------------------------------- //

func TestRankFloat64_Basic(t *testing.T) {
	// [3.0, 1.0, 2.0] → ranks [3, 1, 2]
	x := []float64{3.0, 1.0, 2.0}
	ranks := rankFloat64(x)
	expected := []float64{3, 1, 2}
	for i := range expected {
		if ranks[i] != expected[i] {
			t.Fatalf("rankFloat64[%d]: got %.1f, want %.1f", i, ranks[i], expected[i])
		}
	}
}

func TestRankFloat64_Ties(t *testing.T) {
	// [2.0, 1.0, 1.0, 3.0] → sorted: 1.0, 1.0, 2.0, 3.0
	// Two 1.0s → avg rank (1+2)/2 = 1.5 each; 2.0 is 3rd smallest → rank 3; 3.0 is 4th → rank 4.
	// Output in original order: ranks[0]=3 (value 2.0), ranks[1]=1.5, ranks[2]=1.5, ranks[3]=4.
	x := []float64{2.0, 1.0, 1.0, 3.0}
	ranks := rankFloat64(x)
	expected := []float64{3, 1.5, 1.5, 4}
	for i := range expected {
		if math.Abs(ranks[i]-expected[i]) > 0.001 {
			t.Fatalf("rankFloat64[%d]: got %.2f, want %.2f", i, ranks[i], expected[i])
		}
	}
}

func TestRankFloat64_Empty(t *testing.T) {
	ranks := rankFloat64(nil)
	if ranks != nil {
		t.Fatalf("expected nil for empty input, got %v", ranks)
	}
}

// -------------------------------------------------------------------------- //
// DISTRIBUTION FUNCTION TESTS
// -------------------------------------------------------------------------- //

func TestNormalCDF(t *testing.T) {
	tests := []struct {
		x      float64
		approx float64
		tol    float64
	}{
		{0.0, 0.5, 0.001},
		{1.96, 0.975, 0.001}, // 95% CI boundary
		{-1.96, 0.025, 0.001},
		{3.0, 0.99865, 0.001},
	}

	for _, tc := range tests {
		got := normalCDF(tc.x)
		if math.Abs(got-tc.approx) > tc.tol {
			t.Fatalf("normalCDF(%.2f): got %.5f, want ~%.5f", tc.x, got, tc.approx)
		}
	}
}

func TestTDistCDF(t *testing.T) {
	// For df >= large, t converges to normal.
	for df := 31; df <= 50; df++ {
		got := tDistCDF(1.96, float64(df))
		want := normalCDF(1.96)
		if math.Abs(got-want) > 0.01 {
			t.Fatalf("tDistCDF(1.96, df=%d): got %.5f, want ~%.5f (normal)", df, got, want)
		}
	}

	// tDistCDF(df=1) at x=0 should be 0.5.
	got := tDistCDF(0.0, 1.0)
	if math.Abs(got-0.5) > 0.01 {
		t.Fatalf("tDistCDF(0, df=1): got %.4f, want 0.5", got)
	}
}

// -------------------------------------------------------------------------- //
// INTEGRATION TEST (via Ingest)
// -------------------------------------------------------------------------- //

func TestIngest_PopulatesWilcoxon(t *testing.T) {
	rootDir := t.TempDir()

	// Write 5 runs with known SC values.
	scValues := []float64{0.80, 0.80, 1.00, 1.00, 0.80}
	for i, sc := range scValues {
		writeMetricsFixture(t, rootDir, runID(i), sc, []IngestFamilyResult{
			{FamilyID: "KG-001", ChainValidated: true, TotalSteps: 3, ValidatedSteps: 3},
		})
	}

	rs, err := Ingest(IngestOptions{RootDir: rootDir, RunSetID: "test"})
	if err != nil {
		t.Fatalf("Ingest returned error: %v", err)
	}

	// Should have Wilcoxon (5 runs with SC).
	if rs.WilcoxonSignedRank == nil {
		t.Fatal("expected Wilcoxon result to be non-nil with 5 runs")
	}
	if rs.WilcoxonSignedRank.N != 2 {
		// 0.80 - 0.80 = 0 (zero diffs skipped) → only 2 non-zero positive diffs
		t.Fatalf("expected Wilcoxon N=2 (zero diffs skipped), got %d", rs.WilcoxonSignedRank.N)
	}
}

func TestIngest_PopulatesSpearman(t *testing.T) {
	rootDir := t.TempDir()

	// Write 3 runs with known SC and catalog-step coverage values.
	for i := 0; i < 3; i++ {
		sc := 0.6 + float64(i)*0.2
		coverage := 0.5 + float64(i)*0.2
		writeMetricsWithSCCatalogCoverage(t, rootDir, runID(i), sc, coverage)
	}

	rs, err := Ingest(IngestOptions{RootDir: rootDir, RunSetID: "test"})
	if err != nil {
		t.Fatalf("Ingest returned error: %v", err)
	}

	// Should have Spearman (3 paired non-null observations).
	if rs.SpearmanCorrelation == nil {
		t.Fatal("expected Spearman result to be non-nil with 3 runs")
	}
	if rs.SpearmanCorrelation.N != 3 {
		t.Fatalf("expected Spearman N=3, got %d", rs.SpearmanCorrelation.N)
	}
	if rs.SpearmanCorrelation.Rho < 0.99 {
		t.Fatalf("expected rho≈1.0 for perfectly increasing SC/catalog-step coverage, got %.4f", rs.SpearmanCorrelation.Rho)
	}
}

func TestIngest_OmitsSpearmanForConstantCatalogCoverage(t *testing.T) {
	rootDir := t.TempDir()

	scValues := []float64{0.8, 1.0, 1.0, 0.8, 0.8}
	for i, sc := range scValues {
		writeMetricsWithSCCatalogCoverage(t, rootDir, runID(i), sc, 1.0)
	}

	rs, err := Ingest(IngestOptions{RootDir: rootDir, RunSetID: "constant-catalog-coverage"})
	if err != nil {
		t.Fatalf("Ingest returned error: %v", err)
	}
	if rs.SpearmanCorrelation != nil {
		t.Fatalf("expected Spearman to be nil for constant catalog-step coverage input, got %#v", rs.SpearmanCorrelation)
	}
	if _, err := json.Marshal(rs); err != nil {
		t.Fatalf("expected RunSet to marshal without NaN values, got %v", err)
	}
}

func TestIngest_WilcoxonNotPopulatedBelowThreshold(t *testing.T) {
	rootDir := t.TempDir()

	// Write only 3 runs — below Wilcoxon threshold of 5.
	for i := 0; i < 3; i++ {
		writeMetricsFixture(t, rootDir, runID(i), 0.8, []IngestFamilyResult{
			{FamilyID: "KG-001", ChainValidated: true, TotalSteps: 3, ValidatedSteps: 3},
		})
	}

	rs, err := Ingest(IngestOptions{RootDir: rootDir, RunSetID: "test"})
	if err != nil {
		t.Fatalf("Ingest returned error: %v", err)
	}

	// Spearman should also be nil because SC/catalog-step coverage are constant
	// across the three runs.
	if rs.SpearmanCorrelation != nil {
		t.Fatalf("expected Spearman to be nil with constant SC/catalog-step coverage, got %#v", rs.SpearmanCorrelation)
	}
	if rs.WilcoxonSignedRank != nil {
		t.Fatal("expected Wilcoxon to be nil with only 3 runs")
	}
}

// -------------------------------------------------------------------------- //
// HELPERS
// -------------------------------------------------------------------------- //

func runID(i int) string {
	return fmt.Sprintf("2026-04-10T100000Z-test-r%02d", i+1)
}

// writeMetricsWithSCCatalogCoverage writes a validation-metrics.json fixture with
// explicit SC and catalog-step coverage.
func writeMetricsWithSCCatalogCoverage(t *testing.T, rootDir string, runID string, sc, coverage float64) {
	t.Helper()
	runDir := rootDir + "/" + runID
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	payload := MetricsForIngestion{
		ScenarioCoverage: &IngestScenarioCoverage{
			ScenarioRate:        &sc,
			CatalogStepCoverage: &coverage,
			Families: []IngestFamilyResult{
				{FamilyID: "KG-001", ChainValidated: true, TotalSteps: 3, ValidatedSteps: 3},
			},
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(runDir+"/validation-metrics.json", data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}
