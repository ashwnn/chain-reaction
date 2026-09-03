// Package compare provides an offline comparison generator that joins saved
// runtime (analysis.json), theory (comparison-baseline.json), and scan
// (comparison-baseline.json) artifacts into stable JSON and Markdown outputs.
//
// The comparison is organized by family and step, not only top-level averages,
// and surfaces runtime reliability and blocker/failure summaries where the
// artifacts support it.
package compare

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// ContractVersion is the stable schema identifier for comparison output artifacts.
const ContractVersion = "comparison.v1"

// InputPaths holds the paths to the three source artifacts.
type InputPaths struct {
	Analysis string // path to analysis.json (optional)
	Theory   string // path to theory/comparison-baseline.json (optional)
	Scan     string // path to scan/comparison-baseline.json (optional)
}

// Result is the joined comparison output. It is the canonical output of the
// comparison generator and is serialized as the machine-readable JSON artifact.
type Result struct {
	ContractVersion string          `json:"contract_version"`
	GeneratedAt     time.Time       `json:"generated_at"`
	Sources         Sources         `json:"sources"`
	Families        []FamilyResult  `json:"families"`
	BlockerSummary  *BlockerSummary `json:"blocker_summary,omitempty"`
	FailureSummary  *FailureSummary `json:"failure_summary,omitempty"`
}

// Sources documents which artifacts were joined.
type Sources struct {
	Analysis *AnalysisSource `json:"analysis,omitempty"`
	Theory   *TheorySource   `json:"theory,omitempty"`
	Scan     *ScanSource     `json:"scan,omitempty"`
}

// AnalysisSource describes the analysis.json artifact used.
type AnalysisSource struct {
	Path     string `json:"path"`
	RunSetID string `json:"run_set_id"`
	RunCount int    `json:"run_count"`
	Label    string `json:"label,omitempty"`
}

// TheorySource describes the theory comparison-baseline.json artifact used.
type TheorySource struct {
	Path         string `json:"path"`
	BaselineKind string `json:"baseline_kind"`
}

// ScanSource describes the scan comparison-baseline.json artifact used.
type ScanSource struct {
	Path         string `json:"path"`
	BaselineKind string `json:"baseline_kind"`
}

// FamilyResult holds the joined comparison data for one scenario family.
type FamilyResult struct {
	FamilyID   string          `json:"family_id"`
	FamilyName string          `json:"family_name"`
	InScope    bool            `json:"in_scope"`
	Runtime    *RuntimeSummary `json:"runtime,omitempty"`
	Steps      []StepResult    `json:"steps"`
}

// RuntimeSummary holds runtime aggregates for a family from analysis.json.
type RuntimeSummary struct {
	ChainValidatedCount   int                   `json:"chain_validated_count"`
	AttemptedCount        int                   `json:"attempted_count"`
	ReliabilityFraction   *float64              `json:"reliability_fraction,omitempty"`
	CatalogCoverageSample *CatalogCoverageStats `json:"catalog_step_coverage_sample,omitempty"`
}

// CatalogCoverageStats holds catalog-step coverage statistics for a family.
type CatalogCoverageStats struct {
	N    int     `json:"n"`
	Mean float64 `json:"mean"`
	SD   float64 `json:"sd"`
}

// StepResult holds the joined comparison data for one step.
type StepResult struct {
	StepID          string   `json:"step_id"`
	Description     string   `json:"description"`
	TheoryStatus    *string  `json:"theory_status,omitempty"`  // nil when theory artifact not provided
	ScanStatus      *string  `json:"scan_status,omitempty"`    // nil when scan artifact not provided
	RuntimeStatus   *string  `json:"runtime_status,omitempty"` // nil when no runtime data or not validated
	ExpectedTools   []string `json:"expected_tools,omitempty"`
	SupportingTools []string `json:"supporting_tools,omitempty"` // from scan baseline
}

// BlockerSummary captures family-level blocker data from runtime analysis.
type BlockerSummary struct {
	// BlockedFamilies lists families that had zero successful chain validations.
	BlockedFamilies []FamilyBlocker `json:"blocked_families,omitempty"`
	// TotalChainsAttempted is the total across all families.
	TotalChainsAttempted int `json:"total_chains_attempted"`
	// TotalChainsValidated is the total across all families.
	TotalChainsValidated int `json:"total_chains_validated"`
}

// FamilyBlocker documents why a family was blocked.
type FamilyBlocker struct {
	FamilyID            string  `json:"family_id"`
	AttemptedCount      int     `json:"attempted_count"`
	ValidatedCount      int     `json:"validated_count"`
	ReliabilityFraction float64 `json:"reliability_fraction"`
}

// FailureSummary captures per-family failure breakdown from runtime analysis.
type FailureSummary struct {
	// FamiliesWithFailures documents families with partial chain completion.
	FamiliesWithFailures []FamilyFailure `json:"families_with_failures,omitempty"`
}

// FamilyFailure documents partial chain completion for a family.
type FamilyFailure struct {
	FamilyID           string  `json:"family_id"`
	AttemptedCount     int     `json:"attempted_count"`
	NeverValidated     int     `json:"never_validated_count"`     // runs where chain was never validated
	PartiallyValidated int     `json:"partially_validated_count"` // runs with some steps validated but not full chain
	PartialRate        float64 `json:"partial_rate,omitempty"`    // NeverValidated / Attempted
}

// Generate produces a joined comparison Result from the given input artifacts.
// Empty InputPaths fields are treated as absent and will not contribute data.
// Returns an error when a specified artifact cannot be read or parsed.
func Generate(paths InputPaths) (*Result, error) {
	result := &Result{
		ContractVersion: ContractVersion,
		GeneratedAt:     time.Now().UTC(),
		Sources:         Sources{},
		Families:        make([]FamilyResult, 0),
	}

	// Ingest artifacts in order. Each returns a map keyed by family ID.
	var theoryFamilies map[string]comparisonFamily
	var scanFamilies map[string]comparisonFamily
	var runtimeFamilies map[string]*RuntimeSummary

	if paths.Theory != "" {
		tf, err := ingestTheory(paths.Theory)
		if err != nil {
			return nil, fmt.Errorf("ingest theory: %w", err)
		}
		theoryFamilies = tf
		result.Sources.Theory = &TheorySource{
			Path:         paths.Theory,
			BaselineKind: "static_theoretical",
		}
	}

	if paths.Scan != "" {
		sf, err := ingestScan(paths.Scan)
		if err != nil {
			return nil, fmt.Errorf("ingest scan: %w", err)
		}
		scanFamilies = sf
		result.Sources.Scan = &ScanSource{
			Path:         paths.Scan,
			BaselineKind: "discovery",
		}
	}

	if paths.Analysis != "" {
		rf, as, err := ingestAnalysis(paths.Analysis)
		if err != nil {
			return nil, fmt.Errorf("ingest analysis: %w", err)
		}
		runtimeFamilies = rf
		result.Sources.Analysis = as
	}

	// Build the unified family list. We use the catalog as the canonical source
	// for family/step structure. Theory and scan may add KG-006 (out of scope).
	families := buildFamilyResults(theoryFamilies, scanFamilies, runtimeFamilies)

	result.Families = families
	result.BlockerSummary = computeBlockerSummary(runtimeFamilies)
	result.FailureSummary = computeFailureSummary(runtimeFamilies)

	// Sort for deterministic output.
	sort.Slice(result.Families, func(i, j int) bool {
		return result.Families[i].FamilyID < result.Families[j].FamilyID
	})
	for i := range result.Families {
		sort.Slice(result.Families[i].Steps, func(x, y int) bool {
			return result.Families[i].Steps[x].StepID < result.Families[i].Steps[y].StepID
		})
	}

	return result, nil
}

// WriteJSON marshals and writes the Result to the given output path.
// Returns the written path.
func WriteJSON(result *Result, outputPath string) (string, error) {
	dir := filepath.Dir(outputPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", fmt.Errorf("create output directory: %w", err)
		}
	}

	body, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal comparison result: %w", err)
	}
	if err := os.WriteFile(outputPath, body, 0o644); err != nil {
		return "", fmt.Errorf("write comparison result: %w", err)
	}
	return outputPath, nil
}

// comparisonFamily mirrors the internal structure of comparison-baseline.json.
type comparisonFamily struct {
	FamilyID   string
	FamilyName string
	InScope    bool
	Steps      []comparisonStep
}

type comparisonStep struct {
	StepID          string
	Description     string
	Status          string
	ExpectedTools   []string
	SupportingTools []string
}
