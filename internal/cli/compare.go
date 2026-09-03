package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ashwnn/chain-reaction/internal/compare"
	"github.com/ashwnn/chain-reaction/internal/plot"
	"github.com/ashwnn/chain-reaction/internal/report"
	"github.com/ashwnn/chain-reaction/internal/table"
)

func newCompareCmd(state *appState) *cobra.Command {
	var (
		analysisPath   string
		theoryPath     string
		scanPath       string
		outputPath     string
		markdownPath   string
		noMarkdown     bool
		includeRuntime bool
		generatePlots  bool
		generateTables bool
		generateHTML   bool
	)

	cmd := &cobra.Command{
		Use:   "compare",
		Short: "Compare runtime, theory, and scan artifacts into JSON, Markdown, and SVG outputs",
		Long: `Join saved runtime, theory, and scan artifacts into stable comparison outputs.

This command reads one or more of the following artifact types:
  - analysis.json: runtime analysis from repeated validate runs
  - theory/comparison-baseline.json: static theoretical baseline
  - scan/comparison-baseline.json: deterministic discovery baseline

And produces a joined comparison organized by family and step, including:
  - Per-family runtime reliability summaries
  - Blocker/failure summaries where the artifacts support it
  - Deterministic JSON and optional Markdown table outputs
  - Comparison-specific SVG charts (with --plots)
  - Comparison-specific narrative tables (with --tables)

All three input artifacts are optional. The comparison is computed from whatever
artifacts are provided. Providing only theory and scan produces a baseline
comparison; adding analysis.json enriches it with runtime reliability data.

Examples:
  chain-reaction compare -a analysis.json -t theory/comparison-baseline.json -s scan/comparison-baseline.json
  chain-reaction compare -a analysis.json -o comparison-output/
  chain-reaction compare -a analysis.json -t theory/comparison-baseline.json -s scan/comparison-baseline.json --plots --tables -o comparison/
`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			paths := compare.InputPaths{
				Analysis: analysisPath,
				Theory:   theoryPath,
				Scan:     scanPath,
			}

			// Validate that at least one artifact is specified.
			if paths.Analysis == "" && paths.Theory == "" && paths.Scan == "" {
				return fmt.Errorf("at least one artifact path is required: --analysis, --theory, or --scan")
			}

			// Validate specified paths exist.
			if paths.Analysis != "" {
				if err := validatePath(paths.Analysis, "analysis"); err != nil {
					return err
				}
			}
			if paths.Theory != "" {
				if err := validatePath(paths.Theory, "theory"); err != nil {
					return err
				}
			}
			if paths.Scan != "" {
				if err := validatePath(paths.Scan, "scan"); err != nil {
					return err
				}
			}

			result, err := compare.Generate(paths)
			if err != nil {
				return fmt.Errorf("compare.Generate: %w", err)
			}

			outDir, jsonOutPath, err := resolveOutputBundle(outputPath, "comparison.json")
			if err != nil {
				return err
			}
			jsonWritten, err := compare.WriteJSON(result, jsonOutPath)
			if err != nil {
				return fmt.Errorf("write JSON: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Comparison JSON: %s\n", jsonWritten)

			// Write Markdown output if requested.
			if !noMarkdown {
				var mdOutPath string
				if markdownPath != "" {
					mdOutPath = markdownPath
				} else if outputPath != "" {
					info, err := os.Stat(outputPath)
					if err == nil && info.IsDir() {
						mdOutPath = filepath.Join(outputPath, "comparison.md")
					}
				}

				if mdOutPath != "" {
					if err := writeMarkdownOutput(result, mdOutPath, includeRuntime); err != nil {
						return fmt.Errorf("write Markdown: %w", err)
					}
					fmt.Fprintf(cmd.OutOrStdout(), "Comparison Markdown: %s\n", mdOutPath)
				} else {
					// Write to stdout.
					opts := compare.MarkdownOptions{IncludeRuntime: includeRuntime}
					compare.WriteMarkdown(cmd.OutOrStdout(), result, opts)
				}
			}

			// Generate comparison-specific SVG plots if requested.
			if generatePlots {
				plotFiles, err := writeComparisonPlots(result, outDir)
				if err != nil {
					return fmt.Errorf("write comparison plots: %w", err)
				}
				for _, pf := range plotFiles {
					fmt.Fprintf(cmd.OutOrStdout(), "Plot written: %s\n", pf)
				}
				if len(plotFiles) > 0 {
					fmt.Fprintln(cmd.OutOrStdout())
				}
			}

			if generateTables {
				tablePath, err := writeComparisonTables(result, outDir)
				if err != nil {
					return fmt.Errorf("write comparison tables: %w", err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Comparison tables written: %s\n\n", tablePath)
			}

			if generateHTML {
				htmlPath, err := writeComparisonHTML(result, outDir)
				if err != nil {
					return fmt.Errorf("write html report: %w", err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "HTML report written: %s\n\n", htmlPath)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&analysisPath, "analysis", "a", "", "Path to analysis.json (runtime aggregates from repeated validate runs)")
	cmd.Flags().StringVarP(&theoryPath, "theory", "t", "", "Path to theory/comparison-baseline.json (static theoretical baseline)")
	cmd.Flags().StringVarP(&scanPath, "scan", "s", "", "Path to scan/comparison-baseline.json (deterministic discovery baseline)")
	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "Output path for comparison JSON, or directory if ending in /")
	cmd.Flags().StringVar(&markdownPath, "markdown", "", "Output path for Markdown table (prints to stdout if omitted)")
	cmd.Flags().BoolVar(&noMarkdown, "no-markdown", false, "Skip basic Markdown output")
	cmd.Flags().BoolVar(&includeRuntime, "include-runtime", true, "Include runtime column in basic Markdown tables")
	cmd.Flags().BoolVar(&generatePlots, "plots", false, "Generate comparison SVG charts (gap chart, heatmap, chain status)")
	cmd.Flags().BoolVar(&generateTables, "tables", false, "Generate comparison Markdown tables (gap table, narrative)")
	cmd.Flags().BoolVar(&generateHTML, "html", false, "Generate a self-contained HTML report (comparison-report.html)")

	return cmd
}

// validatePath checks that the given path exists and is a file.
func resolveOutputBundle(path, filename string) (dir, file string, err error) {
	if path == "" {
		return ".", filename, nil
	}
	if strings.HasSuffix(strings.ToLower(path), ".json") {
		return filepath.Dir(path), path, nil
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return "", "", fmt.Errorf("create output directory: %w", err)
	}
	return path, filepath.Join(path, filename), nil
}

func validatePath(path, name string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s artifact does not exist: %s", name, path)
		}
		return fmt.Errorf("stat %s artifact: %w", name, err)
	}
	if info.IsDir() {
		return fmt.Errorf("%s artifact path must be a file, not a directory: %s", name, path)
	}
	return nil
}

// writeMarkdownOutput writes the Markdown comparison to the specified path.
func writeMarkdownOutput(result *compare.Result, path string, includeRuntime bool) error {
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create output directory: %w", err)
		}
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create markdown file: %w", err)
	}
	defer f.Close()

	opts := compare.MarkdownOptions{IncludeRuntime: includeRuntime}
	compare.WriteMarkdown(f, result, opts)
	return nil
}

// writeComparisonPlots writes comparison-specific SVG chart artifacts to the
// output directory. Returns a slice of written file paths.
func writeComparisonPlots(result *compare.Result, outDir string) ([]string, error) {
	var written []string

	// Theory-Scan-Runtime gap chart
	gapSVG := plot.RenderTheoryScanRuntimeGapChart(result)
	gapPath := filepath.Join(outDir, "comparison-gap-chart.svg")
	if err := os.WriteFile(gapPath, []byte(gapSVG), 0o644); err != nil {
		return written, fmt.Errorf("write comparison-gap-chart.svg: %w", err)
	}
	written = append(written, gapPath)

	// Step-level heatmap
	heatmapSVG := plot.RenderStepLevelHeatmap(result)
	heatmapPath := filepath.Join(outDir, "comparison-step-heatmap.svg")
	if err := os.WriteFile(heatmapPath, []byte(heatmapSVG), 0o644); err != nil {
		return written, fmt.Errorf("write comparison-step-heatmap.svg: %w", err)
	}
	written = append(written, heatmapPath)

	// Family chain status chart
	statusSVG := plot.RenderFamilyChainStatusChart(result)
	statusPath := filepath.Join(outDir, "comparison-chain-status.svg")
	if err := os.WriteFile(statusPath, []byte(statusSVG), 0o644); err != nil {
		return written, fmt.Errorf("write comparison-chain-status.svg: %w", err)
	}
	written = append(written, statusPath)

	return written, nil
}

// writeComparisonTables writes comparison-specific Markdown table artifacts
// to the output directory. Returns the path to the written file.
func writeComparisonTables(result *compare.Result, outDir string) (string, error) {
	tablePath := filepath.Join(outDir, "comparison-narrative.md")
	f, err := os.Create(tablePath)
	if err != nil {
		return "", fmt.Errorf("create comparison-narrative.md: %w", err)
	}
	defer f.Close()

	table.WriteAllComparisonTables(f, result)
	return tablePath, nil
}

func writeComparisonHTML(result *compare.Result, outDir string) (string, error) {
	htmlPath := filepath.Join(outDir, "comparison-report.html")
	f, err := os.Create(htmlPath)
	if err != nil {
		return "", fmt.Errorf("create comparison-report.html: %w", err)
	}
	defer f.Close()
	if err := report.WriteComparisonHTML(f, result); err != nil {
		return "", err
	}
	return htmlPath, nil
}
