package analysis

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// IngestOptions controls which runs are included in the analysis and how
// the RunSet is labeled.
type IngestOptions struct {
	// RootDir is the base scenario-runs directory (e.g., artifacts/scenario-runs/).
	// Required.
	RootDir string

	// RunSetID is an optional identifier for this run set. When empty,
	// defaults to the directory name of RootDir.
	RunSetID string

	// RunSetLabel is an optional human-readable label for this run set.
	// Shown in output but not used for computation.
	RunSetLabel string

	// RunIDFilter restricts analysis to the specified run IDs (directory names
	// directly under RootDir). When nil, all discovered flat runs are included.
	RunIDFilter []string

	// FamilyFilter restricts analysis to the specified families.
	// When nil, all observed families are included.
	FamilyFilter []string
}

// Ingest reads all validation-metrics.json files from the given RootDir and
// produces a RunSet with descriptive statistics. It enumerates flat run
// directories directly under RootDir (not the per-family KG-xxx/run-N/ symlink
// views) to avoid double-counting — each flat directory is one run covering
// all families simultaneously.
//
// The run set ID defaults to the base name of RootDir when RunSetID is empty.
//
// Returns an error when RootDir does not exist or no run directories contain
// a valid validation-metrics.json.
func Ingest(opts IngestOptions) (*RunSet, error) {
	if opts.RootDir == "" {
		return nil, &IngestError{Kind: ErrKindInvalidInput, Message: "RootDir is required"}
	}

	info, err := os.Stat(opts.RootDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &IngestError{Kind: ErrKindNotFound, Message: "RootDir does not exist: " + opts.RootDir}
		}
		return nil, &IngestError{Kind: ErrKindIO, Message: "stat RootDir: " + err.Error()}
	}
	if !info.IsDir() {
		return nil, &IngestError{Kind: ErrKindInvalidInput, Message: "RootDir is not a directory: " + opts.RootDir}
	}

	runSetID := opts.RunSetID
	if runSetID == "" {
		runSetID = filepath.Base(opts.RootDir)
	}

	// Enumerate flat run directories.
	entries, err := os.ReadDir(opts.RootDir)
	if err != nil {
		return nil, &IngestError{Kind: ErrKindIO, Message: "read RootDir: " + err.Error()}
	}

	runIDSet := make(map[string]bool, len(opts.RunIDFilter))
	for _, runID := range opts.RunIDFilter {
		runIDSet[runID] = true
	}

	var runMetrics []MetricsWithPath
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if len(runIDSet) > 0 && !runIDSet[entry.Name()] {
			continue
		}
		metricsPath := filepath.Join(opts.RootDir, entry.Name(), "validation-metrics.json")
		data, err := os.ReadFile(metricsPath)
		if err != nil {
			// Skip directories that don't have a metrics file yet.
			continue
		}

		var m MetricsForIngestion
		if err := json.Unmarshal(data, &m); err != nil {
			// Skip malformed metrics files.
			continue
		}

		runMetrics = append(runMetrics, MetricsWithPath{
			Path:    metricsPath,
			Metrics: m,
		})
	}

	if len(runMetrics) == 0 {
		msg := "no validation-metrics.json files found in " + opts.RootDir
		if len(runIDSet) > 0 {
			msg += " for the requested run IDs"
		}
		return nil, &IngestError{
			Kind:    ErrKindNotFound,
			Message: msg,
		}
	}

	// Sort by run ID for deterministic output.
	sort.Slice(runMetrics, func(i, j int) bool {
		return filepath.Base(filepath.Dir(runMetrics[i].Path)) <
			filepath.Base(filepath.Dir(runMetrics[j].Path))
	})

	return buildRunSet(opts, runSetID, runMetrics)
}

// MetricsForIngestion is the minimal subset of ValidationMetrics needed by the
// analysis pipeline. Embedding the full type would create a dependency cycle
// (analysis -> metrics -> baseline). We only read the fields we need.
type MetricsForIngestion struct {
	ScenarioCoverage *IngestScenarioCoverage `json:"scenario_coverage"`
	TimeToChain      *IngestTimeToChain      `json:"time_to_chain"`
}

type IngestScenarioCoverage struct {
	ScenarioRate        *float64             `json:"scenario_rate"`
	CatalogStepCoverage *float64             `json:"catalog_step_coverage"`
	Families            []IngestFamilyResult `json:"families"`
}

type IngestFamilyResult struct {
	FamilyID       string `json:"family_id"`
	ChainValidated bool   `json:"chain_validated"`
	TotalSteps     int    `json:"total_steps"`
	ValidatedSteps int    `json:"validated_steps"`
}

type IngestTimeToChain struct {
	DurationMS int64 `json:"duration_ms"`
}

// MetricsWithPath pairs a metrics file path with its parsed contents.
type MetricsWithPath struct {
	Path    string
	Metrics MetricsForIngestion
}

// buildRunSet constructs a RunSet from the sorted list of metrics files.
func buildRunSet(opts IngestOptions, runSetID string, runs []MetricsWithPath) (*RunSet, error) {
	rs := &RunSet{
		ID:         runSetID,
		Label:      opts.RunSetLabel,
		SourceDir:  opts.RootDir,
		RunCount:   len(runs),
		RunIDs:     make([]string, 0, len(runs)),
		ComputedAt: time.Now().UTC(),
		PerFamily:  make([]FamilyStats, 0), // always a non-nil slice in output
	}

	for _, run := range runs {
		rs.RunIDs = append(rs.RunIDs, filepath.Base(filepath.Dir(run.Path)))
	}

	// Build family filter lookup set.
	familySet := make(map[string]bool)
	for _, f := range opts.FamilyFilter {
		familySet[f] = true
	}

	// Collect per-run scalar values for global stats.
	scValues := make([]float64, len(runs))
	catalogCoverageValues := make([]float64, len(runs))
	ttcValues := make([]float64, len(runs))

	for i, r := range runs {
		scValues[i] = math.NaN()
		catalogCoverageValues[i] = math.NaN()
		ttcValues[i] = math.NaN()

		if r.Metrics.ScenarioCoverage != nil {
			if r.Metrics.ScenarioCoverage.ScenarioRate != nil {
				scValues[i] = *r.Metrics.ScenarioCoverage.ScenarioRate
			}
			if r.Metrics.ScenarioCoverage.CatalogStepCoverage != nil {
				catalogCoverageValues[i] = *r.Metrics.ScenarioCoverage.CatalogStepCoverage
			}
		}
		if r.Metrics.TimeToChain != nil {
			ttcValues[i] = float64(r.Metrics.TimeToChain.DurationMS)
		}
	}

	rs.Global.SC = ComputeSampleStats(scValues)
	rs.Global.SC.VarianceFlag = ComputeVarianceFlag(rs.Global.SC.CV, rs.Global.SC.N)

	rs.Global.CatalogStepCoverage = ComputeSampleStats(catalogCoverageValues)
	rs.Global.CatalogStepCoverage.VarianceFlag = ComputeVarianceFlag(rs.Global.CatalogStepCoverage.CV, rs.Global.CatalogStepCoverage.N)

	ttcStats := ComputeDescriptiveStats(ttcValues)
	if ttcStats.N > 0 {
		ttcStats.VarianceFlag = ComputeVarianceFlag(ttcStats.CV, ttcStats.N)
		rs.Global.TTC = &ttcStats
	}

	// Per-family stats: collect chain validation and catalog-step coverage across runs.
	familyData := gatherFamilyData(runs, familySet)
	for _, fd := range familyData {
		fs := FamilyStats{
			FamilyID:              fd.FamilyID,
			Validated:             fd.Validated,
			Attempted:             fd.Attempted,
			CatalogCoverageSample: fd.CatalogCoverageStats,
		}
		fs.CatalogCoverageSample.VarianceFlag = ComputeVarianceFlag(fs.CatalogCoverageSample.CV, fs.CatalogCoverageSample.N)
		if fd.Attempted > 0 {
			frac := float64(fd.Validated) / float64(fd.Attempted)
			fs.ReliabilityFraction = &frac
		}
		rs.PerFamily = append(rs.PerFamily, fs)
	}

	// Sort per-family by family ID for deterministic output.
	sort.Slice(rs.PerFamily, func(i, j int) bool {
		return rs.PerFamily[i].FamilyID < rs.PerFamily[j].FamilyID
	})

	// Wilcoxon signed-rank test: tests whether the median SC differs from 0.80
	// (the paper's target threshold). Requires ≥5 non-null SC observations.
	if rs.Global.SC.N >= 5 {
		result, err := wilcoxonSignedRank(rs.Global.SC.RawValues, 0.80)
		if err == nil {
			rs.WilcoxonSignedRank = result
		}
		// On error (degenerate data), leave nil — caller already has descriptive stats.
	}

	// Spearman correlation: measures the monotonic relationship between SC and
	// catalog-step coverage across runs. Requires ≥3 paired non-null observations.
	var pairedSC, pairedCatalogCoverage []float64
	for i := range scValues {
		if !math.IsNaN(scValues[i]) && !math.IsNaN(catalogCoverageValues[i]) {
			pairedSC = append(pairedSC, scValues[i])
			pairedCatalogCoverage = append(pairedCatalogCoverage, catalogCoverageValues[i])
		}
	}
	if len(pairedSC) >= 3 {
		result, err := spearmanCorrelation(pairedSC, pairedCatalogCoverage)
		if err == nil {
			rs.SpearmanCorrelation = result
		}
	}

	return rs, nil
}

// familyDatum accumulates per-family data across all runs.
type familyDatum struct {
	FamilyID              string
	Validated             int
	Attempted             int
	CatalogCoverageValues []float64
	CatalogCoverageStats  DescriptiveStats
}

// gatherFamilyData collects chain-validation and catalog-step coverage data per
// family.
func gatherFamilyData(runs []MetricsWithPath, familyFilter map[string]bool) []familyDatum {
	// Map from family ID to accumulated data.
	families := make(map[string]*familyDatum)

	for _, r := range runs {
		if r.Metrics.ScenarioCoverage == nil {
			continue
		}
		for _, f := range r.Metrics.ScenarioCoverage.Families {
			if len(familyFilter) > 0 && !familyFilter[f.FamilyID] {
				continue
			}

			d, ok := families[f.FamilyID]
			if !ok {
				d = &familyDatum{FamilyID: f.FamilyID}
				families[f.FamilyID] = d
			}

			d.Attempted++
			if f.ChainValidated {
				d.Validated++
			}

			// Compute per-family catalog-step coverage if total_steps > 0.
			if f.TotalSteps > 0 {
				coverage := float64(f.ValidatedSteps) / float64(f.TotalSteps)
				d.CatalogCoverageValues = append(d.CatalogCoverageValues, coverage)
			}
		}
	}

	// Compute catalog-step coverage stats per family.
	var result []familyDatum
	for _, d := range families {
		d.CatalogCoverageStats = ComputeDescriptiveStats(d.CatalogCoverageValues)
		result = append(result, *d)
	}
	return result
}

// IngestError represents a failure during ingestion.
type IngestError struct {
	Kind    IngestErrorKind
	Message string
}

func (e *IngestError) Error() string {
	return e.Kind.String() + ": " + e.Message
}

// IngestErrorKind classifies the category of ingestion failure.
type IngestErrorKind int

const (
	ErrKindInvalidInput IngestErrorKind = iota
	ErrKindNotFound
	ErrKindIO
)

func (k IngestErrorKind) String() string {
	switch k {
	case ErrKindInvalidInput:
		return "invalid input"
	case ErrKindNotFound:
		return "not found"
	case ErrKindIO:
		return "I/O error"
	default:
		return "unknown"
	}
}
