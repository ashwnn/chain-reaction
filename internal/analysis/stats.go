package analysis

import (
	"math"
	"sort"
)

// ComputeDescriptiveStats derives mean, sample SD, CV, min, and max from the
// input values. NaN values are filtered out before computation. Returns a zero
// DescriptiveStats when the input has fewer than 2 non-NaN values.
// Sample SD uses the N-1 denominator (unbiased estimator).
//
// ComputeDescriptiveStats is a pure function — no I/O, no mutation of inputs.
func ComputeDescriptiveStats(values []float64) DescriptiveStats {
	filtered := filterNaN(values)

	n := len(filtered)
	if n == 0 {
		return DescriptiveStats{}
	}

	sorted := make([]float64, n)
	copy(sorted, filtered)
	sort.Float64s(sorted)

	sum := 0.0
	min := sorted[0]
	max := sorted[n-1]
	for _, v := range sorted {
		sum += v
	}
	mean := sum / float64(n)

	// Sample standard deviation (N-1 denominator).
	var sd float64
	if n > 1 {
		sumSqDiff := 0.0
		for _, v := range sorted {
			diff := v - mean
			sumSqDiff += diff * diff
		}
		sd = math.Sqrt(sumSqDiff / float64(n-1))
	}

	// CV as percentage. Zero when mean is zero (avoids division by zero).
	var cv float64
	if mean != 0 {
		cv = (sd / mean) * 100
	}

	return DescriptiveStats{
		N:    n,
		Mean: mean,
		SD:   sd,
		CV:   cv,
		Min:  min,
		Max:  max,
	}
}

// ComputeSampleStats is like ComputeDescriptiveStats but also populates
// RawValues and NullCount from the original input. The returned RawValues
// contains only the non-NaN values in input order.
//
// ComputeSampleStats is a pure function.
func ComputeSampleStats(values []float64) SampleStats {
	filtered := filterNaN(values)
	nullCount := countNaNs(values)

	ds := ComputeDescriptiveStats(filtered)
	return SampleStats{
		DescriptiveStats: ds,
		RawValues:        filtered,
		NullCount:        nullCount,
	}
}

// filterNaN returns a new slice with all NaN values removed.
func filterNaN(values []float64) []float64 {
	if len(values) == 0 {
		return nil
	}
	out := make([]float64, 0, len(values))
	for _, v := range values {
		if !math.IsNaN(v) {
			out = append(out, v)
		}
	}
	return out
}

// countNaNs returns the number of NaN values in the input.
func countNaNs(values []float64) int {
	n := 0
	for _, v := range values {
		if math.IsNaN(v) {
			n++
		}
	}
	return n
}

// ComputeVarianceFlag derives a variance flag from a coefficient of variation
// (CV, expressed as a percentage). The thresholds match the interpretation
// documented in compute-stability.sh:
//
//	"low"      — CV < 10%, high reproducibility
//	"moderate" — 10% ≤ CV ≤ 25%, acceptable variability for LLM-driven planner
//	"high"     — CV > 25%, unusually high variability (investigate non-determinism)
//
// Returns an empty string when n < 2 (insufficient data for a reliable CV).
//
// ComputeVarianceFlag is a pure function.
func ComputeVarianceFlag(cv float64, n int) string {
	if n < 2 {
		return ""
	}
	switch {
	case cv < 10:
		return "low"
	case cv <= 25:
		return "moderate"
	default:
		return "high"
	}
}
