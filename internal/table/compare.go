// Package table provides Markdown table generation for analysis and comparison outputs.
// All rendering uses text/tabwriter for clean ASCII-aligned output.
package table

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/ashwnn/chain-reaction/internal/compare"
	"github.com/ashwnn/chain-reaction/internal/mdtable"
)

// -------------------------------------------------------------------------- //
// STATUS LABEL HELPERS
// -------------------------------------------------------------------------- //

// statusLabel returns a human-readable label for a step status value.
func statusLabel(s *string) string {
	if s == nil {
		return "—"
	}
	switch *s {
	case "theoretical":
		return "theoretical"
	case "observed":
		return "observed"
	case "not_attempted":
		return "not attempted"
	case "validated":
		return "validated"
	case "not_validated":
		return "not validated"
	case "failed":
		return "failed"
	case "failed_rbac":
		return "failed (RBAC)"
	default:
		return *s
	}
}

// coverageSummary computes truthful coverage counts for a family's steps.
// Theory counts any defined step. Scan counts only steps explicitly observed by
// the discovery baseline. Runtime counts only steps explicitly validated.
func coverageSummary(steps []compare.StepResult) (theoryCount, scanCount, runtimeCount, total int) {
	total = len(steps)
	for _, s := range steps {
		if s.TheoryStatus != nil {
			theoryCount++
		}
		if s.ScanStatus != nil && *s.ScanStatus == "observed" {
			scanCount++
		}
		if s.RuntimeStatus != nil && *s.RuntimeStatus == "validated" {
			runtimeCount++
		}
	}
	return
}

// chainStatusFromRuntime returns a human-readable chain status string.
func chainStatusFromRuntime(rs *compare.RuntimeSummary) string {
	if rs == nil {
		return "no runtime data"
	}
	if rs.AttemptedCount == 0 {
		return "no runs attempted"
	}
	if rs.ReliabilityFraction == nil {
		return fmt.Sprintf("%d/%d runs validated", rs.ChainValidatedCount, rs.AttemptedCount)
	}
	frac := *rs.ReliabilityFraction
	if frac >= 1.0 {
		return fmt.Sprintf("validated (%d/%d runs)", rs.ChainValidatedCount, rs.AttemptedCount)
	}
	if frac == 0 {
		return fmt.Sprintf("blocked (%d/%d runs — chain never fully validated)", rs.ChainValidatedCount, rs.AttemptedCount)
	}
	return fmt.Sprintf("partially validated (%d/%d runs, %.0f%% reliability)",
		rs.ChainValidatedCount, rs.AttemptedCount, frac*100)
}

// -------------------------------------------------------------------------- //
// COMPARISON NARRATIVE TABLE
// -------------------------------------------------------------------------- //

// WriteComparisonNarrativeTable writes a per-family narrative explanation table
// that documents the theory→scan→runtime progression for each in-scope family,
// including explicit blocker explanations where the chain was not fully validated.
func WriteComparisonNarrativeTable(w io.Writer, result *compare.Result) {
	fmt.Fprintln(w, "# Chain Reaction Comparison: Per-Family Narrative")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Generated: %s\n\n", result.GeneratedAt.Format("2006-01-02T15:04:05Z"))

	// Source attribution.
	if result.Sources.Theory != nil {
		fmt.Fprintf(w, "- **Theory baseline**: %s\n", result.Sources.Theory.Path)
	}
	if result.Sources.Scan != nil {
		fmt.Fprintf(w, "- **Scan baseline**: %s\n", result.Sources.Scan.Path)
	}
	if result.Sources.Analysis != nil {
		fmt.Fprintf(w, "- **Runtime analysis**: %s (%d runs)\n",
			result.Sources.Analysis.Path, result.Sources.Analysis.RunCount)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "---")
	fmt.Fprintln(w)

	// Sort families for deterministic output.
	families := make([]compare.FamilyResult, len(result.Families))
	copy(families, result.Families)
	sort.Slice(families, func(i, j int) bool {
		return families[i].FamilyID < families[j].FamilyID
	})

	for _, family := range families {
		if !family.InScope {
			continue
		}

		theoryTotal, _, _, _ := coverageSummary(family.Steps)
		chainStatus := chainStatusFromRuntime(family.Runtime)

		fmt.Fprintf(w, "## %s: %s\n", family.FamilyID, family.FamilyName)
		fmt.Fprintln(w)

		// Theory row.
		theoryDesc := describeTheoryCoverage(family.Steps, theoryTotal)
		fmt.Fprintf(w, "**Theory**: %s\n\n", theoryDesc)

		// Scan row.
		scanDesc := describeScanCoverage(family.Steps)
		fmt.Fprintf(w, "**Scan (discovery baseline)**: %s\n\n", scanDesc)

		// Runtime row.
		runtimeDesc := describeRuntimeCoverage(family.Runtime, family.Steps)
		fmt.Fprintf(w, "**Runtime (live validation)**: %s\n\n", runtimeDesc)

		// Chain status.
		fmt.Fprintf(w, "**Chain status**: %s\n\n", chainStatus)

		// Blocker explanation if applicable.
		if family.Runtime != nil && family.Runtime.ReliabilityFraction != nil {
			frac := *family.Runtime.ReliabilityFraction
			if frac < 1.0 {
				blockerNote := explainBlocker(family.FamilyID, family.Runtime, family.Steps)
				fmt.Fprintf(w, "**Blocker analysis**: %s\n\n", blockerNote)
			}
		}

		// Gap summary.
		gap := describeCoverageGap(family.Steps, family.Runtime)
		if gap != "" {
			fmt.Fprintf(w, "**Gap (theory → runtime)**: %s\n\n", gap)
		}

		fmt.Fprintln(w, "---")
		fmt.Fprintln(w)
	}
}

// describeTheoryCoverage returns a sentence describing theory coverage.
func describeTheoryCoverage(steps []compare.StepResult, theoryCount int) string {
	if theoryCount == 0 {
		return "no step data in theory artifact"
	}
	stepList := listStepIDs(steps, func(s compare.StepResult) bool { return s.TheoryStatus != nil })
	if theoryCount == len(steps) {
		return fmt.Sprintf("all %d steps are theoretical (%s). These steps are defined by the step-chain catalog but have not been confirmed by discovery or runtime.", theoryCount, stepList)
	}
	return fmt.Sprintf("%d/%d steps are theoretical (%s).", theoryCount, len(steps), stepList)
}

// describeScanCoverage returns a sentence describing scan/discovery coverage.
func describeScanCoverage(steps []compare.StepResult) string {
	var observed, notAttempted []string
	for _, s := range steps {
		if s.ScanStatus != nil {
			switch *s.ScanStatus {
			case "observed":
				observed = append(observed, s.StepID)
			case "not_attempted":
				notAttempted = append(notAttempted, s.StepID)
			default:
				observed = append(observed, s.StepID)
			}
		}
	}
	if len(observed) == 0 && len(notAttempted) == 0 {
		return "no step data in scan artifact"
	}
	var parts []string
	if len(observed) > 0 {
		parts = append(parts, fmt.Sprintf("%d steps observed at discovery: %s", len(observed), strings.Join(observed, ", ")))
	}
	if len(notAttempted) > 0 {
		parts = append(parts, fmt.Sprintf("%d steps not attempted at discovery: %s", len(notAttempted), strings.Join(notAttempted, ", ")))
	}
	return strings.Join(parts, ". ") + "."
}

// describeRuntimeCoverage returns a sentence describing runtime validation coverage.
func describeRuntimeCoverage(rs *compare.RuntimeSummary, steps []compare.StepResult) string {
	if rs == nil {
		return "no runtime data available"
	}
	if rs.AttemptedCount == 0 {
		return "family was not attempted in any validation run"
	}

	var validated, notValidated []string
	for _, s := range steps {
		if s.RuntimeStatus != nil {
			switch *s.RuntimeStatus {
			case "validated":
				validated = append(validated, s.StepID)
			case "not_validated":
				notValidated = append(notValidated, s.StepID)
			}
		}
	}

	if len(validated) > 0 {
		return fmt.Sprintf("runtime validation confirmed: %s. Chain validated in %d/%d runs (%.0f%% reliability).",
			strings.Join(validated, "→"), rs.ChainValidatedCount, rs.AttemptedCount,
			ptrFloat(rs.ReliabilityFraction)*100)
	}
	if len(notValidated) > 0 {
		return fmt.Sprintf("runtime validation did not confirm: %s. Chain validated in %d/%d runs (%.0f%% reliability).",
			strings.Join(notValidated, ", "), rs.ChainValidatedCount, rs.AttemptedCount,
			ptrFloat(rs.ReliabilityFraction)*100)
	}
	return chainStatusFromRuntime(rs)
}

// explainBlocker provides an explicit explanation of why a chain was blocked or
// only partially validated, grounded in the available artifact data.
func explainBlocker(familyID string, rs *compare.RuntimeSummary, steps []compare.StepResult) string {
	if rs == nil || rs.AttemptedCount == 0 {
		return "insufficient runtime data to determine blocker"
	}

	if rs.ReliabilityFraction != nil && *rs.ReliabilityFraction == 0 {
		// Fully blocked: chain never validated in any run.
		stepList := listStepIDs(steps, func(s compare.StepResult) bool { return true })
		reason := blockerReasonFromFamily(familyID, steps)
		return fmt.Sprintf("chain was never fully validated in %d/%d runs. "+
			"All %d steps are theoretical but none were confirmed at runtime. "+
			"Known failure points: %s. %s",
			rs.ChainValidatedCount, rs.AttemptedCount, len(steps), stepList, reason)
	}

	// Partially validated: some runs succeeded.
	if rs.ReliabilityFraction != nil && *rs.ReliabilityFraction > 0 && *rs.ReliabilityFraction < 1 {
		neverValidated := rs.AttemptedCount - rs.ChainValidatedCount
		stepList := listStepIDs(steps, func(s compare.StepResult) bool {
			return s.RuntimeStatus == nil || s.RuntimeStatus != nil && *s.RuntimeStatus != "validated"
		})
		reason := partialBlockerReasonFromFamily(familyID)
		return fmt.Sprintf("chain validated in %d/%d runs (%.0f%%), but %d runs failed to complete the chain. "+
			"Partial failure steps: %s. %s",
			rs.ChainValidatedCount, rs.AttemptedCount, *rs.ReliabilityFraction*100,
			neverValidated, stepList, reason)
	}

	return "runtime validation behavior is consistent across runs"
}

// blockerReasonFromFamily returns a family-specific explanation of common failure modes.
func blockerReasonFromFamily(familyID string, steps []compare.StepResult) string {
	switch familyID {
	case "KG-001":
		return "KG-001 requires permission enumeration (check_permissions) and exploitation proof (read_secret). Failure suggests RBAC denies sensitive actions or no exploitable permissions were present."
	case "KG-002":
		return "KG-002 requires permission to list secrets and successful secret read. Failure suggests RBAC denies secret access or the target secret was not present."
	case "KG-003":
		return "KG-003 requires token inspection (check_token) and permission confirmation. Failure suggests the mounted ServiceAccount token was inaccessible or lacked readable metadata."
	case "KG-004":
		return "KG-004 requires network probe to a service endpoint and a secondary target. Failure suggests network policies block egress, DNS resolution fails for service FQDNs, or the probe target is unreachable."
	case "KG-005":
		return "KG-005 requires namespace enumeration and cross-namespace access (API or network). Failure suggests RBAC denies cross-namespace permissions or network policies block cross-namespace traffic."
	default:
		return "blocker reason is family-specific; consult the step-chain catalog for expected tools and validation criteria."
	}
}

// partialBlockerReasonFromFamily returns a family-specific explanation for partial chain completion.
func partialBlockerReasonFromFamily(familyID string) string {
	switch familyID {
	case "KG-001":
		return "KG-001's S3 (permission exploitation) may be sensitive to which specific secret or resource the planner selects to prove exploitation."
	case "KG-002":
		return "KG-002's S2 (secret read) may be sensitive to which secret the planner targets; some secrets may be inaccessible while others are readable."
	case "KG-003":
		return "KG-003's S2 (permission confirmation) may be sensitive to the planner's choice of target resource for the permission check."
	case "KG-004":
		return "KG-004's S2 (secondary network target) is the most unreliable step; cross-namespace or cross-service probe targets may be intermittently unreachable due to DNS, network policy, or service availability."
	case "KG-005":
		return "KG-005's S3 (cross-namespace API access) requires both network reachability and RBAC permission to succeed; failure in either dimension blocks the chain."
	default:
		return "partial validation suggests the chain is sensitive to planner choices or runtime conditions."
	}
}

// describeCoverageGap returns a sentence describing the theory→runtime coverage gap.
func describeCoverageGap(steps []compare.StepResult, rs *compare.RuntimeSummary) string {
	theoryCount := 0
	for _, s := range steps {
		if s.TheoryStatus != nil {
			theoryCount++
		}
	}
	if theoryCount == 0 {
		return ""
	}

	if rs != nil && rs.AttemptedCount > 0 && rs.ReliabilityFraction != nil {
		if *rs.ReliabilityFraction == 1.0 {
			return fmt.Sprintf("all %d theoretical steps survived to runtime validation. The planner successfully exercised the theoretical attack path in every run.", theoryCount)
		}
		if *rs.ReliabilityFraction == 0 {
			return fmt.Sprintf("theoretical path defined (%d steps) but no runtime chain completed. Gap: theory assumes the path is viable; runtime shows it was blocked.", theoryCount)
		}
		return fmt.Sprintf("theoretical path partially confirmed: %d theoretical steps, runtime validated %.0f%% of runs.", theoryCount, *rs.ReliabilityFraction*100)
	}

	if theoryCount > 0 && (rs == nil || rs.AttemptedCount == 0) {
		return fmt.Sprintf("theoretical path defined (%d steps) but no runtime data available to confirm viability.", theoryCount)
	}
	return ""
}

// listStepIDs returns a comma-separated list of step IDs matching the predicate.
func listStepIDs(steps []compare.StepResult, pred func(compare.StepResult) bool) string {
	var ids []string
	for _, s := range steps {
		if pred(s) {
			ids = append(ids, s.StepID)
		}
	}
	return strings.Join(ids, ", ")
}

// -------------------------------------------------------------------------- //
// COMPARISON GAP TABLE
// -------------------------------------------------------------------------- //

// WriteComparisonGapTable writes a Markdown table showing coverage at each
// artifact level (theory, scan, runtime) for each in-scope family.
func WriteComparisonGapTable(w io.Writer, result *compare.Result) {
	fmt.Fprintln(w, "# Chain Reaction Comparison: Coverage Gap Analysis")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Generated: %s\n\n", result.GeneratedAt.Format("2006-01-02T15:04:05Z"))
	fmt.Fprintln(w, "Coverage at each artifact level — theory, scan (discovery baseline), and runtime (live validation).")
	fmt.Fprintln(w)

	families := make([]compare.FamilyResult, len(result.Families))
	copy(families, result.Families)
	sort.Slice(families, func(i, j int) bool {
		return families[i].FamilyID < families[j].FamilyID
	})

	rows := make([][]string, 0, len(families))
	for _, family := range families {
		if !family.InScope {
			continue
		}

		theoryCount, scanObserved, runtimeValidated, total := coverageSummary(family.Steps)

		chainStatus := "no runtime"
		notes := ""
		if family.Runtime != nil && family.Runtime.AttemptedCount > 0 {
			if family.Runtime.ReliabilityFraction != nil {
				if *family.Runtime.ReliabilityFraction >= 1.0 {
					chainStatus = "validated"
				} else if *family.Runtime.ReliabilityFraction == 0 {
					chainStatus = "blocked"
				} else {
					chainStatus = "partial"
				}
			}
			if family.Runtime.ChainValidatedCount == 0 && family.Runtime.AttemptedCount > 0 {
				chainStatus = "blocked"
			}
		}

		if chainStatus == "blocked" {
			notes = "chain never fully validated"
		} else if chainStatus == "partial" && family.Runtime != nil {
			notes = fmt.Sprintf("%d/%d runs validated",
				family.Runtime.ChainValidatedCount, family.Runtime.AttemptedCount)
		} else if chainStatus == "validated" {
			notes = "full chain in all runs"
		}

		gapNote := ""
		if theoryCount > scanObserved {
			gapNote = fmt.Sprintf("scan gap: %d/%d observed", scanObserved, theoryCount)
		}
		if scanObserved > runtimeValidated {
			if gapNote != "" {
				gapNote += "; "
			}
			gapNote += fmt.Sprintf("runtime gap: %d/%d validated", runtimeValidated, scanObserved)
		}
		if gapNote == "" && chainStatus == "validated" {
			gapNote = "no gap"
		}
		if gapNote != "" {
			notes = gapNote
		}

		rows = append(rows, []string{
			family.FamilyID,
			fmt.Sprintf("%d/%d", theoryCount, total),
			fmt.Sprintf("%d/%d", scanObserved, total),
			fmt.Sprintf("%d/%d", runtimeValidated, total),
			chainStatus,
			notes,
		})
	}
	mdtable.Write(w, []string{"Family", "Theory Steps", "Scan Steps Observed", "Runtime Steps Validated", "Chain Status", "Notes"}, rows)
	fmt.Fprintln(w)
}

// -------------------------------------------------------------------------- //
// COMBINED COMPARISON TABLES
// -------------------------------------------------------------------------- //

// WriteAllComparisonTables writes a complete Markdown report containing all
// comparison tables in a structured order.
func WriteAllComparisonTables(w io.Writer, result *compare.Result) {
	WriteComparisonGapTable(w, result)
	fmt.Fprintln(w)
	WriteComparisonNarrativeTable(w, result)
}

// -------------------------------------------------------------------------- //
// HELPERS
// -------------------------------------------------------------------------- //

func ptrFloat(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}
