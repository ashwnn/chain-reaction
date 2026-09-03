package report

import (
	"fmt"
	"html"
	"io"
	"strings"
	"time"

	"github.com/ashwnn/chain-reaction/internal/analysis"
	"github.com/ashwnn/chain-reaction/internal/compare"
	"github.com/ashwnn/chain-reaction/internal/plot"
	"github.com/ashwnn/chain-reaction/internal/table"
)

func WriteAnalysisHTML(w io.Writer, rs *analysis.RunSet) error {
	var tables strings.Builder
	table.WriteAllAnalysisTables(&tables, rs)

	figures := []struct {
		title string
		svg   string
	}{
		{"Scenario coverage and catalog-step coverage", plot.RenderGlobalSCCatalogCoverageBarChart(rs)},
		{"Coverage distributions", plot.RenderCoverageBoxPlot(rs)},
		{"Per-family chain reliability", plot.RenderFamilyReliabilityBarChart(rs)},
		{"Scenario coverage per run", plot.RenderRawSCValuesDotPlot(rs)},
		{"SC vs catalog-step coverage", plot.RenderSpearmanScatterPlot(rs)},
		{"Wilcoxon signed-rank test", plot.RenderWilcoxonMetricDisplay(rs)},
		{"Spearman correlation", plot.RenderSpearmanMetricDisplay(rs)},
	}
	if ttc := plot.RenderTTCBarchart(rs); ttc != "" {
		figures = append(figures, struct {
			title string
			svg   string
		}{"Time-to-chain", ttc})
	}

	meta := [][2]string{
		{"Run set", rs.ID},
		{"Label", rs.Label},
		{"Source", rs.SourceDir},
		{"Runs", fmt.Sprintf("%d", rs.RunCount)},
		{"Computed", rs.ComputedAt.UTC().Format(time.RFC3339)},
	}
	return writeHTML(w, "Chain Reaction Analysis Report", meta, tables.String(), figures)
}

func WriteComparisonHTML(w io.Writer, result *compare.Result) error {
	var tables strings.Builder
	table.WriteAllComparisonTables(&tables, result)

	figures := []struct {
		title string
		svg   string
	}{
		{"Theory vs scan vs runtime coverage", plot.RenderTheoryScanRuntimeGapChart(result)},
		{"Step-level status heatmap", plot.RenderStepLevelHeatmap(result)},
		{"Per-family chain validation status", plot.RenderFamilyChainStatusChart(result)},
	}
	meta := [][2]string{
		{"Generated", result.GeneratedAt.UTC().Format(time.RFC3339)},
		{"Contract", result.ContractVersion},
	}
	if result.Sources.Theory != nil {
		meta = append(meta, [2]string{"Theory", result.Sources.Theory.Path})
	}
	if result.Sources.Scan != nil {
		meta = append(meta, [2]string{"Scan", result.Sources.Scan.Path})
	}
	if result.Sources.Analysis != nil {
		meta = append(meta, [2]string{"Runtime", fmt.Sprintf("%s (%d runs)", result.Sources.Analysis.Path, result.Sources.Analysis.RunCount)})
	}
	return writeHTML(w, "Chain Reaction Comparison Report", meta, tables.String(), figures)
}

func writeHTML(w io.Writer, title string, meta [][2]string, markdownTables string, figures []struct {
	title string
	svg   string
}) error {
	var b strings.Builder
	b.WriteString("<!DOCTYPE html>\n<html lang=\"en\">\n<head>\n<meta charset=\"utf-8\">\n")
	b.WriteString(fmt.Sprintf("<title>%s</title>\n", html.EscapeString(title)))
	b.WriteString(`<style>
:root { --ink:#1a1a1a; --muted:#5c5c5c; --line:#d7dce2; --bg:#f7f8fa; --card:#fff; --accent:#0072b2; }
* { box-sizing: border-box; }
html, body { margin: 0; padding: 0; background: var(--bg); color: var(--ink);
  font-family: "Liberation Serif", "Times New Roman", Times, serif; }
header { background: var(--card); border-bottom: 1px solid var(--line); padding: 28px 40px 22px; }
header h1 { margin: 0 0 6px; font-size: 26px; letter-spacing: 0.01em; }
header p { margin: 0; color: var(--muted); font-size: 14px; }
main { max-width: 980px; margin: 0 auto; padding: 28px 24px 64px; }
.meta { display: grid; grid-template-columns: 140px 1fr; gap: 6px 16px; background: var(--card);
  border: 1px solid var(--line); padding: 16px 20px; margin: 0 0 28px; font-family: "Liberation Sans", Helvetica, Arial, sans-serif; font-size: 13px; }
.meta dt { color: var(--muted); font-weight: 600; }
.meta dd { margin: 0; }
section { background: var(--card); border: 1px solid var(--line); padding: 20px 22px 8px; margin: 0 0 22px; }
section h2 { margin: 0 0 12px; font-size: 18px; border-bottom: 1px solid var(--line); padding-bottom: 8px; }
figure { margin: 0 0 22px; }
figure svg { width: 100%; height: auto; display: block; background: #fff; border: 1px solid var(--line); }
figcaption { font-family: "Liberation Sans", Helvetica, Arial, sans-serif; font-size: 12px; color: var(--muted); margin-top: 8px; }
.prose { font-family: "Liberation Sans", Helvetica, Arial, sans-serif; font-size: 14px; line-height: 1.5; }
.prose table { border-collapse: collapse; width: 100%; margin: 10px 0 18px; font-size: 13px; }
.prose th, .prose td { border: 1px solid var(--line); padding: 6px 8px; text-align: left; vertical-align: top; }
.prose th { background: #eef1f4; }
.prose h1 { font-size: 16px; margin: 18px 0 8px; }
.prose h2 { font-size: 15px; margin: 16px 0 8px; border: 0; padding: 0; }
@media print { body { background: #fff; } header, section, .meta { border-color: #bbb; } }
</style>
</head>
<body>
`)
	b.WriteString("<header><h1>")
	b.WriteString(html.EscapeString(title))
	b.WriteString("</h1><p>Evidence-backed Kubernetes attack-chain validation</p></header>\n<main>\n")
	b.WriteString("<dl class=\"meta\">\n")
	for _, kv := range meta {
		if strings.TrimSpace(kv[1]) == "" {
			continue
		}
		b.WriteString("<dt>")
		b.WriteString(html.EscapeString(kv[0]))
		b.WriteString("</dt><dd>")
		b.WriteString(html.EscapeString(kv[1]))
		b.WriteString("</dd>\n")
	}
	b.WriteString("</dl>\n")

	if len(figures) > 0 {
		b.WriteString("<section><h2>Figures</h2>\n")
		for i, fig := range figures {
			if fig.svg == "" {
				continue
			}
			b.WriteString("<figure>\n")
			b.WriteString(fig.svg)
			b.WriteString(fmt.Sprintf("<figcaption>Figure %d. %s</figcaption>\n", i+1, html.EscapeString(fig.title)))
			b.WriteString("</figure>\n")
		}
		b.WriteString("</section>\n")
	}

	b.WriteString("<section><h2>Tables</h2>\n<div class=\"prose\">\n")
	b.WriteString(markdownToHTML(markdownTables))
	b.WriteString("</div></section>\n</main>\n</body>\n</html>\n")
	_, err := io.WriteString(w, b.String())
	return err
}

func markdownToHTML(md string) string {
	lines := strings.Split(md, "\n")
	var b strings.Builder
	inTable := false
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "|") {
			if isTableSep(trim) {
				continue
			}
			cells := splitMDRow(trim)
			if !inTable {
				b.WriteString("<table>\n<thead><tr>")
				for _, c := range cells {
					b.WriteString("<th>")
					b.WriteString(html.EscapeString(c))
					b.WriteString("</th>")
				}
				b.WriteString("</tr></thead>\n<tbody>\n")
				inTable = true
				continue
			}
			b.WriteString("<tr>")
			for _, c := range cells {
				b.WriteString("<td>")
				b.WriteString(html.EscapeString(c))
				b.WriteString("</td>")
			}
			b.WriteString("</tr>\n")
			continue
		}
		if inTable {
			b.WriteString("</tbody></table>\n")
			inTable = false
		}
		switch {
		case trim == "":
			continue
		case strings.HasPrefix(trim, "# "):
			b.WriteString("<h1>")
			b.WriteString(html.EscapeString(strings.TrimPrefix(trim, "# ")))
			b.WriteString("</h1>\n")
		case strings.HasPrefix(trim, "## "):
			b.WriteString("<h2>")
			b.WriteString(html.EscapeString(strings.TrimPrefix(trim, "## ")))
			b.WriteString("</h2>\n")
		case strings.HasPrefix(trim, "- "):
			b.WriteString("<p>")
			b.WriteString(inlineMD(strings.TrimPrefix(trim, "- ")))
			b.WriteString("</p>\n")
		default:
			b.WriteString("<p>")
			b.WriteString(inlineMD(trim))
			b.WriteString("</p>\n")
		}
	}
	if inTable {
		b.WriteString("</tbody></table>\n")
	}
	return b.String()
}

func isTableSep(line string) bool {
	s := strings.ReplaceAll(line, "|", "")
	s = strings.ReplaceAll(s, ":", "")
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, " ", "")
	return s == ""
}

func splitMDRow(line string) []string {
	line = strings.Trim(line, "|")
	parts := strings.Split(line, "|")
	out := make([]string, len(parts))
	for i, p := range parts {
		out[i] = strings.TrimSpace(p)
	}
	return out
}

func inlineMD(s string) string {
	s = html.EscapeString(s)
	s = strings.ReplaceAll(s, "**", "")
	return s
}
