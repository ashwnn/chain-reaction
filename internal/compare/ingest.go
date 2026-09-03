package compare

import (
	"encoding/json"
	"fmt"
	"os"
)

// Theory baseline artifact structure (baseline.comparison.v1).
type theoryBaselineArtifact struct {
	ContractVersion string                 `json:"contract_version"`
	BaselineKind    string                 `json:"baseline_kind"`
	SourceArtifacts []string               `json:"source_artifacts"`
	Families        []theoryBaselineFamily `json:"families"`
}

// Scan baseline artifact structure (baseline.comparison.v1).
type scanBaselineArtifact struct {
	ContractVersion string               `json:"contract_version"`
	BaselineKind    string               `json:"baseline_kind"`
	SourceArtifacts []string             `json:"source_artifacts"`
	Families        []scanBaselineFamily `json:"families"`
}

type theoryBaselineFamily struct {
	FamilyID   string               `json:"family_id"`
	FamilyName string               `json:"family_name"`
	InScope    bool                 `json:"in_scope"`
	Steps      []theoryBaselineStep `json:"steps"`
}

type theoryBaselineStep struct {
	StepID        string   `json:"step_id"`
	Description   string   `json:"description"`
	Status        string   `json:"status"`
	ExpectedTools []string `json:"expected_tools,omitempty"`
}

type scanBaselineFamily struct {
	FamilyID   string             `json:"family_id"`
	FamilyName string             `json:"family_name"`
	InScope    bool               `json:"in_scope"`
	Steps      []scanBaselineStep `json:"steps"`
}

type scanBaselineStep struct {
	StepID          string   `json:"step_id"`
	Description     string   `json:"description"`
	Status          string   `json:"status"`
	ExpectedTools   []string `json:"expected_tools,omitempty"`
	SupportingTools []string `json:"supporting_tools,omitempty"`
}

// Analysis artifact structure (analysis.v2).
type analysisArtifact struct {
	ID        string `json:"id"`
	Label     string `json:"label,omitempty"`
	SourceDir string `json:"source_dir"`
	RunCount  int    `json:"run_count"`
	PerFamily []struct {
		FamilyID              string   `json:"family_id"`
		Validated             int      `json:"chain_validated_count"`
		Attempted             int      `json:"attempted_count"`
		ReliabilityFraction   *float64 `json:"reliability_fraction,omitempty"`
		CatalogCoverageSample struct {
			N    int     `json:"n"`
			Mean float64 `json:"mean"`
			SD   float64 `json:"sd"`
		} `json:"catalog_step_coverage_sample,omitempty"`
	} `json:"per_family,omitempty"`
}

// ingestTheory reads and parses a theory comparison-baseline.json.
func ingestTheory(path string) (map[string]comparisonFamily, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read theory artifact: %w", err)
	}

	var artifact theoryBaselineArtifact
	if err := json.Unmarshal(data, &artifact); err != nil {
		return nil, fmt.Errorf("parse theory artifact: %w", err)
	}

	result := make(map[string]comparisonFamily, len(artifact.Families))
	for _, f := range artifact.Families {
		steps := make([]comparisonStep, 0, len(f.Steps))
		for _, s := range f.Steps {
			steps = append(steps, comparisonStep{
				StepID:          s.StepID,
				Description:     s.Description,
				Status:          s.Status,
				ExpectedTools:   s.ExpectedTools,
				SupportingTools: nil, // theory doesn't have supporting tools
			})
		}
		result[f.FamilyID] = comparisonFamily{
			FamilyID:   f.FamilyID,
			FamilyName: f.FamilyName,
			InScope:    f.InScope,
			Steps:      steps,
		}
	}

	return result, nil
}

// ingestScan reads and parses a scan comparison-baseline.json.
func ingestScan(path string) (map[string]comparisonFamily, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read scan artifact: %w", err)
	}

	var artifact scanBaselineArtifact
	if err := json.Unmarshal(data, &artifact); err != nil {
		return nil, fmt.Errorf("parse scan artifact: %w", err)
	}

	result := make(map[string]comparisonFamily, len(artifact.Families))
	for _, f := range artifact.Families {
		steps := make([]comparisonStep, 0, len(f.Steps))
		for _, s := range f.Steps {
			steps = append(steps, comparisonStep{
				StepID:          s.StepID,
				Description:     s.Description,
				Status:          s.Status,
				ExpectedTools:   s.ExpectedTools,
				SupportingTools: s.SupportingTools,
			})
		}
		result[f.FamilyID] = comparisonFamily{
			FamilyID:   f.FamilyID,
			FamilyName: f.FamilyName,
			InScope:    f.InScope,
			Steps:      steps,
		}
	}

	return result, nil
}

// ingestAnalysis reads and parses an analysis.json and returns runtime family summaries
// plus the analysis source metadata.
func ingestAnalysis(path string) (map[string]*RuntimeSummary, *AnalysisSource, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("read analysis artifact: %w", err)
	}

	var artifact analysisArtifact
	if err := json.Unmarshal(data, &artifact); err != nil {
		return nil, nil, fmt.Errorf("parse analysis artifact: %w", err)
	}

	result := make(map[string]*RuntimeSummary, len(artifact.PerFamily))
	for _, f := range artifact.PerFamily {
		rs := &RuntimeSummary{
			ChainValidatedCount: f.Validated,
			AttemptedCount:      f.Attempted,
			ReliabilityFraction: f.ReliabilityFraction,
		}
		if f.CatalogCoverageSample.N > 0 {
			rs.CatalogCoverageSample = &CatalogCoverageStats{
				N:    f.CatalogCoverageSample.N,
				Mean: f.CatalogCoverageSample.Mean,
				SD:   f.CatalogCoverageSample.SD,
			}
		}
		result[f.FamilyID] = rs
	}

	source := &AnalysisSource{
		Path:     path,
		RunSetID: artifact.ID,
		RunCount: artifact.RunCount,
		Label:    artifact.Label,
	}

	return result, source, nil
}

// buildFamilyResults constructs the unified family list by joining theory, scan,
// and runtime data. The catalog provides the canonical family/step structure;
// theory and scan may contribute additional families (e.g., KG-006 out of scope).
func buildFamilyResults(
	theory map[string]comparisonFamily,
	scan map[string]comparisonFamily,
	runtime map[string]*RuntimeSummary,
) []FamilyResult {
	// Collect all family IDs across all sources.
	familyIDs := make(map[string]struct{})
	for id := range theory {
		familyIDs[id] = struct{}{}
	}
	for id := range scan {
		familyIDs[id] = struct{}{}
	}
	for id := range runtime {
		familyIDs[id] = struct{}{}
	}

	// Also include all families from the step-chain catalog to ensure we have
	// the canonical 5 in-scope families even if not present in artifacts.
	catalogFamilies := defaultCatalogFamilyIDs()
	for _, id := range catalogFamilies {
		familyIDs[id] = struct{}{}
	}

	var results []FamilyResult
	for familyID := range familyIDs {
		fr := FamilyResult{
			FamilyID: familyID,
			Steps:    make([]StepResult, 0),
		}

		// Determine family name and in-scope from available sources.
		if tf, ok := theory[familyID]; ok {
			fr.FamilyName = tf.FamilyName
			fr.InScope = tf.InScope
		} else if sf, ok := scan[familyID]; ok {
			fr.FamilyName = sf.FamilyName
			fr.InScope = sf.InScope
		} else {
			fr.FamilyName = familyID // fallback
			fr.InScope = isInScopeFamily(familyID)
		}

		// Runtime summary.
		if rs, ok := runtime[familyID]; ok {
			fr.Runtime = rs
		}

		// Collect all step IDs for this family.
		stepIDs := make(map[string]struct{})
		if tf, ok := theory[familyID]; ok {
			for _, s := range tf.Steps {
				stepIDs[s.StepID] = struct{}{}
			}
		}
		if sf, ok := scan[familyID]; ok {
			for _, s := range sf.Steps {
				stepIDs[s.StepID] = struct{}{}
			}
		}

		// If no step data, add from catalog.
		if len(stepIDs) == 0 {
			catalogSteps := defaultCatalogSteps(familyID)
			for _, s := range catalogSteps {
				stepIDs[s.StepID] = struct{}{}
			}
		}

		// Build step results.
		for stepID := range stepIDs {
			sr := StepResult{StepID: stepID}

			// Theory data.
			if tf, ok := theory[familyID]; ok {
				for _, s := range tf.Steps {
					if s.StepID == stepID {
						sr.Description = s.Description
						sr.TheoryStatus = &s.Status
						sr.ExpectedTools = s.ExpectedTools
						break
					}
				}
			}

			// Scan data.
			if sf, ok := scan[familyID]; ok {
				for _, s := range sf.Steps {
					if s.StepID == stepID {
						if sr.Description == "" {
							sr.Description = s.Description
						}
						sr.ScanStatus = &s.Status
						if len(s.SupportingTools) > 0 {
							sr.SupportingTools = s.SupportingTools
						}
						if len(s.ExpectedTools) > 0 && len(sr.ExpectedTools) == 0 {
							sr.ExpectedTools = s.ExpectedTools
						}
						break
					}
				}
			}

			// Runtime status: derived from runtime summary.
			// If runtime says chain was validated for this family, we can infer
			// the steps were validated. But we don't have per-step runtime data
			// in analysis.json, so we only set runtime status when the family
			// had consistent validation across all attempted runs.
			if rs, ok := runtime[familyID]; ok && rs.AttemptedCount > 0 {
				if rs.ReliabilityFraction != nil && *rs.ReliabilityFraction >= 1.0 {
					status := "validated"
					sr.RuntimeStatus = &status
				} else if rs.ReliabilityFraction != nil && *rs.ReliabilityFraction == 0 {
					status := "not_validated"
					sr.RuntimeStatus = &status
				}
			}

			// Fill in description from catalog if still empty.
			if sr.Description == "" {
				sr.Description = catalogStepDescription(stepID)
			}

			fr.Steps = append(fr.Steps, sr)
		}

		results = append(results, fr)
	}

	return results
}

// computeBlockerSummary identifies families that had zero successful chain validations.
func computeBlockerSummary(runtime map[string]*RuntimeSummary) *BlockerSummary {
	if len(runtime) == 0 {
		return nil
	}

	var summary BlockerSummary
	for familyID, rs := range runtime {
		if rs.AttemptedCount > 0 && (rs.ReliabilityFraction == nil || *rs.ReliabilityFraction == 0) {
			frac := 0.0
			if rs.ReliabilityFraction != nil {
				frac = *rs.ReliabilityFraction
			}
			summary.BlockedFamilies = append(summary.BlockedFamilies, FamilyBlocker{
				FamilyID:            familyID,
				AttemptedCount:      rs.AttemptedCount,
				ValidatedCount:      rs.ChainValidatedCount,
				ReliabilityFraction: frac,
			})
		}
		summary.TotalChainsAttempted += rs.AttemptedCount
		summary.TotalChainsValidated += rs.ChainValidatedCount
	}

	if len(summary.BlockedFamilies) == 0 {
		return nil // no blockers
	}
	return &summary
}

// computeFailureSummary identifies families with partial chain completion.
func computeFailureSummary(runtime map[string]*RuntimeSummary) *FailureSummary {
	if len(runtime) == 0 {
		return nil
	}

	var summary FailureSummary
	for familyID, rs := range runtime {
		if rs.AttemptedCount > 0 && rs.ReliabilityFraction != nil {
			// A family "never validated" means the chain was never fully validated.
			// Partial means some runs validated but not all.
			if *rs.ReliabilityFraction > 0 && *rs.ReliabilityFraction < 1 {
				neverValidated := rs.AttemptedCount - rs.ChainValidatedCount
				summary.FamiliesWithFailures = append(summary.FamiliesWithFailures, FamilyFailure{
					FamilyID:           familyID,
					AttemptedCount:     rs.AttemptedCount,
					NeverValidated:     neverValidated,
					PartiallyValidated: rs.ChainValidatedCount,
					PartialRate:        float64(neverValidated) / float64(rs.AttemptedCount),
				})
			}
		}
	}

	if len(summary.FamiliesWithFailures) == 0 {
		return nil // no partial failures
	}
	return &summary
}

// defaultCatalogFamilyIDs returns the canonical in-scope family IDs.
func defaultCatalogFamilyIDs() []string {
	return []string{"KG-001", "KG-002", "KG-003", "KG-004", "KG-005"}
}

// isInScopeFamily returns true for the 5 in-scope families.
func isInScopeFamily(familyID string) bool {
	switch familyID {
	case "KG-001", "KG-002", "KG-003", "KG-004", "KG-005":
		return true
	default:
		return false
	}
}

// catalogSteps provides step IDs and descriptions for known families.
// This mirrors internal/baseline/catalog.go for use in comparison generation.
func defaultCatalogSteps(familyID string) []struct{ StepID, Description string } {
	switch familyID {
	case "KG-001":
		return []struct{ StepID, Description string }{
			{"KG-001-S1", "Identify current SA identity and token context"},
			{"KG-001-S2", "Enumerate effective permissions; confirm sensitive access"},
			{"KG-001-S3", "Exercise the over-provisioned permission to prove exploitability"},
		}
	case "KG-002":
		return []struct{ StepID, Description string }{
			{"KG-002-S1", "Confirm permission to read secrets in target namespace"},
			{"KG-002-S2", "Read the target secret object"},
		}
	case "KG-003":
		return []struct{ StepID, Description string }{
			{"KG-003-S1", "Inspect mounted token and SA identity context"},
			{"KG-003-S2", "Confirm permissions enabled by the token's SA identity"},
		}
	case "KG-004":
		return []struct{ StepID, Description string }{
			{"KG-004-S1", "Resolve and confirm reachability to a service endpoint"},
			{"KG-004-S2", "Confirm connectivity to a secondary target or cross-namespace endpoint"},
		}
	case "KG-005":
		return []struct{ StepID, Description string }{
			{"KG-005-S1", "Enumerate namespaces beyond the pod's own"},
			{"KG-005-S2", "Confirm cross-namespace service reachability"},
			{"KG-005-S3", "Confirm cross-namespace API access to a sensitive resource"},
		}
	default:
		return nil
	}
}

// catalogStepDescription returns the description for a known step ID.
func catalogStepDescription(stepID string) string {
	for _, familyID := range defaultCatalogFamilyIDs() {
		for _, step := range defaultCatalogSteps(familyID) {
			if step.StepID == stepID {
				return step.Description
			}
		}
	}
	return ""
}
