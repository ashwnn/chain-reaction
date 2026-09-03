// Package analysis provides cross-run statistical analysis of repeated validation
// runs. It ingests validation-metrics.json artifacts from scenario-runs directories
// and produces descriptive and inferential statistics suitable for the paper.
//
// The pipeline computes mean, SD, CV, min, and max for scenario coverage (SC) and
// catalog-step coverage across runs. It also performs:
//   - One-sample Wilcoxon signed-rank test comparing median SC against the 0.80 target
//   - Spearman rank correlation between SC and catalog-step coverage
package analysis

import "time"

// ContractVersion is the stable schema identifier for analysis output artifacts.
const ContractVersion = "analysis.v2"

// RunSet represents one logical group of repeated runs (e.g., one experimental
// condition or one invocation of run-reproducibility.sh).
type RunSet struct {
	ID         string    `json:"id"`
	Label      string    `json:"label,omitempty"`
	SourceDir  string    `json:"source_dir"`
	RunCount   int       `json:"run_count"`
	RunIDs     []string  `json:"run_ids,omitempty"`
	ComputedAt time.Time `json:"computed_at"`

	// Global aggregates — computed across all runs in the set.
	Global GlobalStats `json:"global"`

	// PerFamily results — one entry per family observed in at least one run.
	PerFamily []FamilyStats `json:"per_family,omitempty"`

	// Statistical tests: Wilcoxon signed-rank test and Spearman correlation.
	// These are populated when sufficient data is available.
	//
	// WilcoxonSignedRank is non-nil when there are ≥5 runs with SC values.
	// It performs a one-sample Wilcoxon signed-rank test against the paper's
	// 0.80 SC target, testing whether the median SC differs from 0.80.
	//
	// SpearmanCorrelation is non-nil when there are ≥3 runs with paired
	// non-null SC and catalog-step coverage observations. It measures the rank
	// correlation between family coverage and catalog-step coverage.
	WilcoxonSignedRank  *WilcoxonResult `json:"wilcoxon_signed_rank,omitempty"`
	SpearmanCorrelation *SpearmanResult `json:"spearman_correlation,omitempty"`
}

// GlobalStats holds aggregate statistics computed across all runs in a RunSet.
type GlobalStats struct {
	// Scenario coverage (SC) statistics.
	SC SampleStats `json:"scenario_coverage"`

	// Catalog-step coverage statistics.
	CatalogStepCoverage SampleStats `json:"catalog_step_coverage"`

	// Time-to-chain (TTC) statistics, present when ≥1 run had a non-null TTC.
	// Each run's TTC is in milliseconds.
	TTC *DescriptiveStats `json:"time_to_chain,omitempty"`
}

// DescriptiveStats holds basic descriptive statistics for a set of numeric values.
type DescriptiveStats struct {
	N    int     `json:"n"`
	Mean float64 `json:"mean"`
	SD   float64 `json:"sd"`
	CV   float64 `json:"cv"` // Coefficient of variation as a percentage (SD/mean*100). Zero when mean is zero.
	Min  float64 `json:"min"`
	Max  float64 `json:"max"`
	// VarianceFlag signals unusually high variance using the following scale
	// (mirrors the thresholds documented in compute-stability.sh):
	//   "low"      — CV < 10%, high reproducibility
	//   "moderate" — 10% ≤ CV ≤ 25%, acceptable variability for LLM-driven planner
	//   "high"     — CV > 25%, unusually high variability (investigate non-determinism)
	//   "" (empty) — N < 2, insufficient data to compute a reliable CV
	VarianceFlag string `json:"variance_flag,omitempty"`
}

// SampleStats is DescriptiveStats plus the raw values used to compute it.
// RawValues are included for downstream consumers that need the per-run values
// (e.g., to regenerate statistics with a different subset, or to use in a
// custom plot). Values are ordered by run ID.
type SampleStats struct {
	DescriptiveStats
	// RawValues holds the raw per-run values used to compute the statistics.
	// Null values from individual runs are omitted (N < run_count is expected
	// when some runs return null for a metric).
	RawValues []float64 `json:"raw_values"`
	// NullCount is the number of runs where this metric was null.
	NullCount int `json:"null_count"`
}

// FamilyStats holds per-family reliability and catalog-step coverage statistics
// across runs.
type FamilyStats struct {
	FamilyID string `json:"family_id"`

	// ChainValidated counts — how many runs had this family's chain fully validated.
	Validated int `json:"chain_validated_count"`
	Attempted int `json:"attempted_count"` // runs where this family was in-scope

	// ReliabilityFraction is Validated / Attempted, or nil when Attempted is 0.
	ReliabilityFraction *float64 `json:"reliability_fraction,omitempty"`

	// CatalogCoverageSample holds per-run catalog-step coverage values for this
	// family where available.
	// Null runs are omitted.
	CatalogCoverageSample DescriptiveStats `json:"catalog_step_coverage_sample,omitempty"`
}

// WilcoxonResult holds the result of a one-sample Wilcoxon signed-rank test.
// Wilcoxon tests whether the median of the sample differs from a hypothesized value.
// A non-nil pointer signals the test was run and sufficient data was available.
type WilcoxonResult struct {
	// N is the number of non-zero differences used in the test.
	N int `json:"n"`
	// Statistic is the test statistic (W for the Wilcoxon signed-rank test).
	Statistic float64 `json:"statistic"`
	// PValue is the two-sided p-value. Nil when not computed.
	PValue *float64 `json:"p_value,omitempty"`
	// SignificantAt005 is true when p_value < 0.05. Nil when not computed.
	SignificantAt005 *bool `json:"significant_at_0.05,omitempty"`
	// Interpretation is a human-readable summary of the result.
	Interpretation string `json:"interpretation,omitempty"`
}

// SpearmanResult holds the result of a Spearman rank correlation analysis.
// Spearman measures the monotonic relationship between two variables using ranks.
// A non-nil pointer signals the test was run and sufficient data was available.
type SpearmanResult struct {
	// N is the number of paired observations (both non-null) used in the test.
	N int `json:"n"`
	// Rho is the Spearman rank correlation coefficient, in [-1, 1].
	Rho float64 `json:"rho"`
	// PValue is the two-sided p-value for the null hypothesis that rho = 0.
	// Nil when not computed.
	PValue *float64 `json:"p_value,omitempty"`
	// SignificantAt005 is true when p_value < 0.05. Nil when not computed.
	SignificantAt005 *bool `json:"significant_at_0.05,omitempty"`
	// Interpretation is a human-readable summary of the result.
	Interpretation string `json:"interpretation,omitempty"`
}
