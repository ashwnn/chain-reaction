package compare

import (
	"fmt"
	"io"

	"github.com/ashwnn/chain-reaction/internal/mdtable"
)

type MarkdownOptions struct {
	IncludeRuntime     bool
	IncludeReliability bool
}

func WriteMarkdown(w io.Writer, result *Result, opts MarkdownOptions) {
	fmt.Fprintln(w, "# Chain Reaction Comparison Report")
	fmt.Fprintf(w, "\nGenerated: %s\n", result.GeneratedAt.Format("2006-01-02T15:04:05Z"))
	fmt.Fprintln(w)

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

	if result.BlockerSummary != nil {
		fmt.Fprintln(w, "## Blocker Summary")
		fmt.Fprintln(w)
		fmt.Fprintf(w, "Total chains attempted: %d | Validated: %d | Blocked: %d\n\n",
			result.BlockerSummary.TotalChainsAttempted,
			result.BlockerSummary.TotalChainsValidated,
			len(result.BlockerSummary.BlockedFamilies))
		if len(result.BlockerSummary.BlockedFamilies) > 0 {
			rows := make([][]string, 0, len(result.BlockerSummary.BlockedFamilies))
			for _, b := range result.BlockerSummary.BlockedFamilies {
				rows = append(rows, []string{
					b.FamilyID,
					fmt.Sprintf("%d", b.AttemptedCount),
					fmt.Sprintf("%d", b.ValidatedCount),
					fmt.Sprintf("%.0f%%", b.ReliabilityFraction*100),
				})
			}
			mdtable.Write(w, []string{"Family", "Attempted", "Validated", "Reliability"}, rows)
		}
		fmt.Fprintln(w)
	}

	if result.FailureSummary != nil && len(result.FailureSummary.FamiliesWithFailures) > 0 {
		fmt.Fprintln(w, "## Failure Summary")
		fmt.Fprintln(w)
		rows := make([][]string, 0, len(result.FailureSummary.FamiliesWithFailures))
		for _, f := range result.FailureSummary.FamiliesWithFailures {
			rows = append(rows, []string{
				f.FamilyID,
				fmt.Sprintf("%d", f.AttemptedCount),
				fmt.Sprintf("%d", f.NeverValidated),
				fmt.Sprintf("%.0f%%", f.PartialRate*100),
			})
		}
		mdtable.Write(w, []string{"Family", "Attempted", "Never Validated", "Partial Rate"}, rows)
		fmt.Fprintln(w)
	}

	for _, family := range result.Families {
		fmt.Fprintf(w, "## %s: %s\n", family.FamilyID, family.FamilyName)
		if !family.InScope {
			fmt.Fprintln(w, "*Out of scope*")
			fmt.Fprintln(w)
			continue
		}
		fmt.Fprintln(w)

		if family.Runtime != nil {
			relStr := "N/A"
			if family.Runtime.ReliabilityFraction != nil {
				relStr = fmt.Sprintf("%.0f%%", *family.Runtime.ReliabilityFraction*100)
			}
			svrStr := "N/A"
			if family.Runtime.CatalogCoverageSample != nil {
				svrStr = fmt.Sprintf("%.2f (±%.2f)", family.Runtime.CatalogCoverageSample.Mean, family.Runtime.CatalogCoverageSample.SD)
			}
			fmt.Fprintf(w, "**Runtime**: %d/%d chains validated | Reliability: %s | Catalog step coverage: %s\n\n",
				family.Runtime.ChainValidatedCount,
				family.Runtime.AttemptedCount,
				relStr,
				svrStr)
		}

		writeStepTable(w, family.Steps, opts)
		fmt.Fprintln(w)
	}
}

func writeStepTable(w io.Writer, steps []StepResult, opts MarkdownOptions) {
	headers := []string{"Step", "Description", "Theory", "Scan"}
	if opts.IncludeRuntime {
		headers = append(headers, "Runtime")
	}
	rows := make([][]string, 0, len(steps))
	for _, s := range steps {
		row := []string{
			s.StepID,
			truncate(s.Description, 40),
			ptrToStr(s.TheoryStatus, "—"),
			ptrToStr(s.ScanStatus, "—"),
		}
		if opts.IncludeRuntime {
			row = append(row, ptrToStr(s.RuntimeStatus, "—"))
		}
		rows = append(rows, row)
	}
	mdtable.Write(w, headers, rows)
}

func ptrToStr(p *string, empty string) string {
	if p == nil {
		return empty
	}
	return *p
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
