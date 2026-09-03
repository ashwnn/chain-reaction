package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/ashwnn/chain-reaction/internal/analysis"
	"github.com/ashwnn/chain-reaction/internal/plot"
	"github.com/ashwnn/chain-reaction/internal/report"
	"github.com/ashwnn/chain-reaction/internal/table"
)

func newAnalyzeCmd(state *appState) *cobra.Command {
	var (
		inputPath      string
		outputPath     string
		familyIDs      []string
		runIDs         []string
		runSetID       string
		runSetLabel    string
		generatePlots  bool
		generateTables bool
		generateHTML   bool
	)

	cmd := &cobra.Command{
		Use:   "analyze",
		Short: "Analyze repeated validation runs and produce descriptive statistics",
		Long: `Ingest validation-metrics.json artifacts from a scenario-runs directory
and compute descriptive statistics across runs.

Reads flat run directories under --input (each containing a validation-metrics.json)
and computes mean, sample SD, CV, min, and max for:

- SC  (scenario_rate): fraction of families whose chains fully validated
- catalog_step_coverage: fraction of frozen catalog steps validated across all families

Also produces per-family chain reliability (pass count / N runs) and per-family
catalog-step coverage statistics, plus Wilcoxon signed-rank and Spearman
correlation tests when sufficient data is available (≥5 runs for Wilcoxon, ≥3
paired observations with variation in both series for Spearman).

Output is written as JSON (machine-readable) to --output, and a
human-readable summary is printed to stdout. When --output is a directory,
analysis.json is written inside it.

Use --plots to generate SVG charts (sc-catalog-step-coverage-bar.svg, family-reliability-bar.svg,
sc-raw-values.svg, wilcoxon-result.svg, spearman-result.svg, ttc-barchart.svg if TTC data exists).

Use --tables to generate a Markdown table report (analysis-tables.md) with global stats,
per-family reliability, statistical tests, and per-run raw values.

Examples:
  chain-reaction analyze -i artifacts/scenario-runs -o analysis-output/analysis.json
  chain-reaction analyze -i artifacts/scenario-runs --run-set-id "openai-gpt-4o-001"
  chain-reaction analyze -i artifacts/scenario-runs --run-id 2026-04-10T140000Z-react-001 --run-id 2026-04-10T143000Z-react-002
  chain-reaction analyze -i artifacts/scenario-runs -f KG-001 -f KG-002
  chain-reaction analyze -i artifacts/scenario-runs --plots --tables
`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Resolve input directory.
			runDir := inputPath
			if runDir == "" {
				runDir = "artifacts/scenario-runs"
			}

			outPath := outputPath
			if outPath == "" {
				outPath = runDir
			}
			if !strings.HasSuffix(strings.ToLower(outPath), ".json") {
				if err := os.MkdirAll(outPath, 0o755); err != nil {
					return fmt.Errorf("create output directory: %w", err)
				}
				outPath = filepath.Join(outPath, "analysis.json")
			}

			// Resolve run-set ID from output file path when not explicitly provided.
			resolvedRunSetID := runSetID
			if resolvedRunSetID == "" {
				resolvedRunSetID = filepath.Base(filepath.Dir(outPath))
				if resolvedRunSetID == "" || resolvedRunSetID == "." || resolvedRunSetID == "/" {
					resolvedRunSetID = "run-set"
				}
			}

			opts := analysis.IngestOptions{
				RootDir:      runDir,
				RunSetID:     resolvedRunSetID,
				RunSetLabel:  runSetLabel,
				RunIDFilter:  runIDs,
				FamilyFilter: familyIDs,
			}

			rs, err := analysis.Ingest(opts)
			if err != nil {
				return fmt.Errorf("analyze: %w", err)
			}

			// Ensure output directory exists.
			if dir := filepath.Dir(outPath); dir != "" && dir != "." {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					return fmt.Errorf("create output directory: %w", err)
				}
			}

			f, err := os.Create(outPath)
			if err != nil {
				return fmt.Errorf("create output file: %w", err)
			}

			enc := json.NewEncoder(f)
			enc.SetIndent("", "  ")
			if err := enc.Encode(rs); err != nil {
				f.Close()
				return fmt.Errorf("write analysis artifact: %w", err)
			}
			f.Close()

			fmt.Fprintf(cmd.OutOrStdout(), "Analysis artifact written to: %s\n\n", outPath)

			// Determine output directory for plots and tables.
			outDir := filepath.Dir(outPath)
			if outDir == "" || outDir == "." {
				outDir = "."
			}

			// Generate SVG plots if requested.
			if generatePlots {
				plotFiles, err := writePlots(rs, outDir)
				if err != nil {
					return fmt.Errorf("write plots: %w", err)
				}
				for _, pf := range plotFiles {
					fmt.Fprintf(cmd.OutOrStdout(), "Plot written to: %s\n", pf)
				}
				if len(plotFiles) > 0 {
					fmt.Fprintln(cmd.OutOrStdout())
				}
			}

			if generateTables {
				tablePath, err := writeTables(rs, outDir)
				if err != nil {
					return fmt.Errorf("write tables: %w", err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Tables written to: %s\n\n", tablePath)
			}

			if generateHTML {
				htmlPath, err := writeAnalysisHTML(rs, outDir)
				if err != nil {
					return fmt.Errorf("write html report: %w", err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "HTML report written to: %s\n\n", htmlPath)
			}

			// Human-readable console summary.
			printSummary(cmd.OutOrStdout(), rs)

			return nil
		},
	}

	cmd.Flags().StringVarP(&inputPath, "input", "i", "", "Directory containing flat run directories with validation-metrics.json (default: artifacts/scenario-runs)")
	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "Output path for analysis.json. Can be a file or directory (analysis.json is appended if directory)")
	cmd.Flags().StringSliceVar(&runIDs, "run-id", nil, "Restrict analysis to specific run IDs (repeatable). All discovered runs when omitted.")
	cmd.Flags().StringSliceVarP(&familyIDs, "family", "f", nil, "Restrict analysis to specific families (e.g., -f KG-001 -f KG-002). All families when omitted.")
	cmd.Flags().StringVar(&runSetID, "run-set-id", "", "Identifier for this run set (default: name of output directory)")
	cmd.Flags().StringVar(&runSetLabel, "label", "", "Human-readable label for this run set")
	cmd.Flags().BoolVar(&generatePlots, "plots", false, "Generate SVG plots")
	cmd.Flags().BoolVar(&generateTables, "tables", false, "Generate Markdown table report (analysis-tables.md)")
	cmd.Flags().BoolVar(&generateHTML, "html", false, "Generate a self-contained HTML report (analysis-report.html)")

	return cmd
}

// writePlots writes SVG plot artifacts to the output directory.
// Returns a slice of written file paths.
func writePlots(rs *analysis.RunSet, outDir string) ([]string, error) {
	var written []string

	// Global SC/catalog-step coverage bar chart
	scSvrSVG := plot.RenderGlobalSCCatalogCoverageBarChart(rs)
	scSvrPath := filepath.Join(outDir, "sc-catalog-step-coverage-bar.svg")
	if err := os.WriteFile(scSvrPath, []byte(scSvrSVG), 0o644); err != nil {
		return written, fmt.Errorf("write sc-catalog-step-coverage-bar.svg: %w", err)
	}
	written = append(written, scSvrPath)

	// Per-family reliability bar chart
	reliabilitySVG := plot.RenderFamilyReliabilityBarChart(rs)
	reliabilityPath := filepath.Join(outDir, "family-reliability-bar.svg")
	if err := os.WriteFile(reliabilityPath, []byte(reliabilitySVG), 0o644); err != nil {
		return written, fmt.Errorf("write family-reliability-bar.svg: %w", err)
	}
	written = append(written, reliabilityPath)

	scRawSVG := plot.RenderRawSCValuesDotPlot(rs)
	scRawPath := filepath.Join(outDir, "sc-raw-values.svg")
	if err := os.WriteFile(scRawPath, []byte(scRawSVG), 0o644); err != nil {
		return written, fmt.Errorf("write sc-raw-values.svg: %w", err)
	}
	written = append(written, scRawPath)

	boxSVG := plot.RenderCoverageBoxPlot(rs)
	boxPath := filepath.Join(outDir, "sc-catalog-coverage-box.svg")
	if err := os.WriteFile(boxPath, []byte(boxSVG), 0o644); err != nil {
		return written, fmt.Errorf("write sc-catalog-coverage-box.svg: %w", err)
	}
	written = append(written, boxPath)

	scatterSVG := plot.RenderSpearmanScatterPlot(rs)
	scatterPath := filepath.Join(outDir, "sc-catalog-scatter.svg")
	if err := os.WriteFile(scatterPath, []byte(scatterSVG), 0o644); err != nil {
		return written, fmt.Errorf("write sc-catalog-scatter.svg: %w", err)
	}
	written = append(written, scatterPath)

	// Wilcoxon result metric display
	wilcoxonSVG := plot.RenderWilcoxonMetricDisplay(rs)
	wilcoxonPath := filepath.Join(outDir, "wilcoxon-result.svg")
	if err := os.WriteFile(wilcoxonPath, []byte(wilcoxonSVG), 0o644); err != nil {
		return written, fmt.Errorf("write wilcoxon-result.svg: %w", err)
	}
	written = append(written, wilcoxonPath)

	// Spearman result metric display
	spearmanSVG := plot.RenderSpearmanMetricDisplay(rs)
	spearmanPath := filepath.Join(outDir, "spearman-result.svg")
	if err := os.WriteFile(spearmanPath, []byte(spearmanSVG), 0o644); err != nil {
		return written, fmt.Errorf("write spearman-result.svg: %w", err)
	}
	written = append(written, spearmanPath)

	// TTC barchart (only if TTC data exists)
	ttcSVG := plot.RenderTTCBarchart(rs)
	if ttcSVG != "" {
		ttcPath := filepath.Join(outDir, "ttc-barchart.svg")
		if err := os.WriteFile(ttcPath, []byte(ttcSVG), 0o644); err != nil {
			return written, fmt.Errorf("write ttc-barchart.svg: %w", err)
		}
		written = append(written, ttcPath)
	}

	return written, nil
}

// writeTables writes the Markdown table report to the output directory.
// Returns the path to the written file.
func writeTables(rs *analysis.RunSet, outDir string) (string, error) {
	tablePath := filepath.Join(outDir, "analysis-tables.md")
	f, err := os.Create(tablePath)
	if err != nil {
		return "", fmt.Errorf("create analysis-tables.md: %w", err)
	}
	defer f.Close()

	table.WriteAllAnalysisTables(f, rs)
	return tablePath, nil
}

func writeAnalysisHTML(rs *analysis.RunSet, outDir string) (string, error) {
	htmlPath := filepath.Join(outDir, "analysis-report.html")
	f, err := os.Create(htmlPath)
	if err != nil {
		return "", fmt.Errorf("create analysis-report.html: %w", err)
	}
	defer f.Close()
	if err := report.WriteAnalysisHTML(f, rs); err != nil {
		return "", err
	}
	return htmlPath, nil
}

func printSummary(w io.Writer, rs *analysis.RunSet) {
	fmt.Fprintf(w, "=== Analysis: %s ===\n", rs.ID)
	if rs.Label != "" {
		fmt.Fprintf(w, "Label: %s\n", rs.Label)
	}
	fmt.Fprintf(w, "Source: %s\n", rs.SourceDir)
	fmt.Fprintf(w, "Runs: %d\n", rs.RunCount)
	fmt.Fprintf(w, "Computed: %s\n\n", rs.ComputedAt.Format("2006-01-02T15:04:05Z"))

	// Global SC and catalog-step coverage.
	fmt.Fprintln(w, "--- Global Statistics ---")
	printSampleStats(w, "SC  (scenario_rate)", rs.Global.SC)
	printSampleStats(w, "catalog_step_coverage", rs.Global.CatalogStepCoverage)

	if rs.Global.TTC != nil {
		fmt.Fprintf(w, "TTC (time_to_chain)       : N=%d, mean=%.1fms, SD=%.1fms, min=%.1fms, max=%.1fms\n",
			rs.Global.TTC.N, rs.Global.TTC.Mean, rs.Global.TTC.SD,
			rs.Global.TTC.Min, rs.Global.TTC.Max)
	}

	fmt.Fprintln(w)

	// Per-family results.
	if len(rs.PerFamily) > 0 {
		fmt.Fprintln(w, "--- Per-Family Chain Reliability ---")

		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "Family\tValidated\tAttempted\tReliability\tCatalog Coverage Mean\tCatalog Coverage SD")
		for _, f := range rs.PerFamily {
			rel := "N/A"
			if f.ReliabilityFraction != nil {
				rel = fmt.Sprintf("%.0f%%", *f.ReliabilityFraction*100)
			}
			coverageMean := "N/A"
			coverageSD := "N/A"
			if f.CatalogCoverageSample.N > 0 {
				coverageMean = fmt.Sprintf("%.4f", f.CatalogCoverageSample.Mean)
				if f.CatalogCoverageSample.N > 1 {
					coverageSD = fmt.Sprintf("%.4f", f.CatalogCoverageSample.SD)
				}
			}
			fmt.Fprintf(tw, "%s\t%d\t%d\t%s\t%s\t%s\n",
				f.FamilyID, f.Validated, f.Attempted, rel, coverageMean, coverageSD)
		}
		tw.Flush()
		fmt.Fprintln(w)
	}

	// Wilcoxon signed-rank test result.
	if rs.WilcoxonSignedRank != nil {
		wr := rs.WilcoxonSignedRank
		sig := "no"
		if wr.SignificantAt005 != nil && *wr.SignificantAt005 {
			sig = "yes"
		}
		fmt.Fprintf(w, "Wilcoxon signed-rank (H0: median SC = 0.80): N=%d, W=%.1f, p=%s, significant(0.05)=%s\n",
			wr.N, wr.Statistic, ptrFloat(wr.PValue), sig)
		if wr.Interpretation != "" {
			fmt.Fprintf(w, "  Interpretation: %s\n", wr.Interpretation)
		}
	}

	// Spearman correlation result.
	if rs.SpearmanCorrelation != nil {
		sr := rs.SpearmanCorrelation
		sig := "no"
		if sr.SignificantAt005 != nil && *sr.SignificantAt005 {
			sig = "yes"
		}
		fmt.Fprintf(w, "Spearman correlation (SC vs catalog-step coverage): N=%d, rho=%.4f, p=%s, significant(0.05)=%s\n",
			sr.N, sr.Rho, ptrFloat(sr.PValue), sig)
		if sr.Interpretation != "" {
			fmt.Fprintf(w, "  Interpretation: %s\n", sr.Interpretation)
		}
	}
}

func printSampleStats(w io.Writer, label string, s analysis.SampleStats) {
	fmt.Fprintf(w, "%s: N=%d (nulls=%d), mean=%.4f, SD=%.4f, CV=%.2f%%, min=%.4f, max=%.4f\n",
		padRight(label, 28), s.N, s.NullCount, s.Mean, s.SD, s.CV, s.Min, s.Max)
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

func ptrFloat(p *float64) string {
	if p == nil {
		return "N/A"
	}
	return fmt.Sprintf("%.4f", *p)
}
