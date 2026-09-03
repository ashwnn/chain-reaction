// Package plot provides pure-Go SVG chart generation for analysis outputs.
// Chart renderers produce deterministic SVG markup from analysis.RunSet data.
package plot

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/ashwnn/chain-reaction/internal/analysis"
)

// -------------------------------------------------------------------------- //
// GLOBAL SC/CATALOG-COVERAGE BAR CHART
// -------------------------------------------------------------------------- //

// RenderGlobalSCCatalogCoverageBarChart renders a grouped bar chart showing mean ± SD for
// SC and catalog-step coverage. Produces a self-contained SVG string.
func RenderGlobalSCCatalogCoverageBarChart(rs *analysis.RunSet) string {
	f := NewFigure(AcademicWidth, AcademicHeight)
	f.Title = "Scenario Coverage & Catalog Step Coverage"
	f.Subtitle = "Mean ± 1 SD across repeated validation runs"
	f.YLabel = "Rate"
	f.XLabel = "Metric"
	f.Caption = fmt.Sprintf("N = %d runs  ·  dashed line is the paper target (SC ≥ 0.80)", rs.RunCount)
	f.Legend = []LegendItem{
		{Label: "SC", Color: ColorSC},
		{Label: "Catalog", Color: ColorCatalogCoverage},
	}

	var b strings.Builder
	b.WriteString(f.Open())

	yMin, yMax := 0.0, 1.05
	b.WriteString(f.DrawYAxis(yMin, yMax, []float64{0.0, 0.25, 0.50, 0.75, 1.0}, "%.2f"))
	b.WriteString(f.DrawHRefLine(0.80, yMin, yMax, "target (0.80)"))

	bars := []struct {
		label string
		mean  float64
		sd    float64
		color string
	}{{
		label: "SC",
		mean:  rs.Global.SC.Mean,
		sd:    rs.Global.SC.SD,
		color: ColorSC,
	}, {
		label: "Catalog",
		mean:  rs.Global.CatalogStepCoverage.Mean,
		sd:    rs.Global.CatalogStepCoverage.SD,
		color: ColorCatalogCoverage,
	}}

	barCount := len(bars)
	groupWidth := float64(f.PlotWidth) / float64(barCount+1)
	barWidth := groupWidth * 0.42

	for i, bar := range bars {
		meanClamped := clamp01(bar.mean)
		sdClamped := math.Min(bar.sd, math.Min(meanClamped, 1-meanClamped))

		groupCenter := float64(f.PlotX) + groupWidth + float64(i)*groupWidth
		barX := groupCenter - barWidth/2
		barY := f.DataToY(meanClamped, yMin, yMax)
		barHeight := float64(f.PlotY+f.PlotHeight) - barY

		b.WriteString(f.DrawRoundRect(barX, barY, barWidth, barHeight, 3, bar.color, "none", 0))

		if bar.sd > 0 && rs.Global.SC.N > 1 {
			errY := f.DataToY(meanClamped+sdClamped, yMin, yMax) - barY
			b.WriteString(f.DrawErrorBar(groupCenter, barY, errY, barWidth*0.45, ColorErrorBar))
		}

		b.WriteString(f.DrawText(groupCenter, float64(f.PlotY+f.PlotHeight)+16, bar.label, "tick-label"))

		meanLabel := fmt.Sprintf("%.2f", meanClamped)
		if bar.sd > 0 && rs.Global.SC.N > 1 {
			meanLabel = fmt.Sprintf("%.2f±%.2f", meanClamped, sdClamped)
		}
		labelY := barY - 12
		if labelY < float64(f.PlotY)+10 {
			labelY = float64(f.PlotY) + 12
		}
		b.WriteString(f.DrawText(groupCenter, labelY, meanLabel, "bar-label"))
	}

	b.WriteString(f.Close())
	return b.String()
}

// -------------------------------------------------------------------------- //
// PER-FAMILY RELIABILITY BAR CHART
// -------------------------------------------------------------------------- //

// RenderFamilyReliabilityBarChart renders a horizontal bar chart showing
// per-family chain reliability (validated/attempted) with family labels.
func RenderFamilyReliabilityBarChart(rs *analysis.RunSet) string {
	f := NewFigure(AcademicWidth, AcademicHeight)
	f.Title = "Per-Family Chain Reliability"
	f.Subtitle = "Fraction of repeated runs in which the full family chain validated"
	f.XLabel = "Reliability (validated / attempted)"
	f.Caption = fmt.Sprintf("N = %d runs", rs.RunCount)
	f.Legend = []LegendItem{
		{Label: "Full chain", Color: ColorValidated},
		{Label: "Partial", Color: ColorFamilyRel},
		{Label: "Never validated", Color: ColorNilDot},
	}

	var b strings.Builder
	b.WriteString(f.Open())

	xMin, xMax := 0.0, 1.0
	b.WriteString(f.DrawXAxis(xMin, xMax, []float64{0.0, 0.25, 0.50, 0.75, 1.0}, "%.2f"))

	families := make([]analysis.FamilyStats, len(rs.PerFamily))
	copy(families, rs.PerFamily)
	sort.Slice(families, func(i, j int) bool {
		return families[i].FamilyID < families[j].FamilyID
	})

	if len(families) == 0 {
		b.WriteString(f.EmptyMessage("No family data available"))
		b.WriteString(f.Close())
		return b.String()
	}

	n := float64(len(families))
	slot := float64(f.PlotHeight) / n
	barHeight := math.Min(slot*0.55, 28)

	for i, fam := range families {
		reliability := 0.0
		if fam.ReliabilityFraction != nil {
			reliability = *fam.ReliabilityFraction
		}
		reliability = clamp01(reliability)

		barY := float64(f.PlotY) + slot*float64(i) + (slot-barHeight)/2
		barX := float64(f.PlotX)
		barWidthPx := float64(f.PlotWidth) * reliability

		barColor := ColorFamilyRel
		if reliability >= 1.0 {
			barColor = ColorValidated
		} else if reliability == 0 {
			barColor = ColorNilDot
		}

		if barWidthPx > 0 {
			b.WriteString(f.DrawRoundRect(barX, barY, barWidthPx, barHeight, 3, barColor, "none", 0))
		}

		labelY := barY + barHeight/2
		b.WriteString(f.DrawTextRight(float64(f.PlotX)-8, labelY, fam.FamilyID, "tick-label"))

		relLabel := fmt.Sprintf("%.0f%%  (%d/%d)", reliability*100, fam.Validated, fam.Attempted)
		if barWidthPx > 110 {
			b.WriteString(f.DrawText(barX+barWidthPx/2, labelY, relLabel, "on-bar"))
		} else {
			b.WriteString(f.DrawTextLeft(barX+barWidthPx+8, labelY, relLabel, "tick-label"))
		}
	}

	b.WriteString(f.Close())
	return b.String()
}

// -------------------------------------------------------------------------- //
// RAW SC VALUES DOT PLOT
// -------------------------------------------------------------------------- //

// RenderRawSCValuesDotPlot renders a scatter/dot plot showing per-run SC values
// with a reference line at the 0.80 target.
func RenderRawSCValuesDotPlot(rs *analysis.RunSet) string {
	f := NewFigure(AcademicWidth, AcademicHeight)
	f.Title = "Scenario Coverage per Run"
	f.Subtitle = "Each point is one independent validation run"
	f.YLabel = "Scenario coverage (SC)"
	f.XLabel = "Run"
	f.Caption = fmt.Sprintf("N = %d runs  ·  solid line is the sample mean", rs.RunCount)
	f.Legend = []LegendItem{
		{Label: "≥ target", Color: ColorDot, Shape: "circle"},
		{Label: "< target", Color: ColorNilDot, Shape: "circle"},
	}

	var b strings.Builder
	b.WriteString(f.Open())

	yMin, yMax := 0.0, 1.05
	b.WriteString(f.DrawYAxis(yMin, yMax, []float64{0.0, 0.25, 0.50, 0.75, 1.0}, "%.2f"))
	b.WriteString(f.DrawHRefLine(0.80, yMin, yMax, "target (0.80)"))

	values := rs.Global.SC.RawValues
	runIDs := rs.RunIDs
	n := len(values)
	if n == 0 {
		b.WriteString(f.EmptyMessage("No run data available"))
		b.WriteString(f.Close())
		return b.String()
	}

	xStep := float64(f.PlotWidth) / float64(n+1)
	const dotRadius = 5.5

	if rs.Global.SC.N > 0 {
		meanY := f.DataToY(rs.Global.SC.Mean, yMin, yMax)
		b.WriteString(f.DrawLine(float64(f.PlotX), meanY, float64(f.PlotX+f.PlotWidth), meanY, ColorCatalogCoverage, 1.6, ""))
		b.WriteString(f.DrawTextLeft(float64(f.PlotX)+6, meanY-10, fmt.Sprintf("mean=%.2f", rs.Global.SC.Mean), "reference-label"))
	}

	for i, val := range values {
		cx := float64(f.PlotX) + xStep*float64(i+1)
		cy := f.DataToY(val, yMin, yMax)

		dotColor := ColorDot
		dotStroke := ColorDotStroke
		if val < 0.80 {
			dotColor = ColorNilDot
			dotStroke = "#666666"
		}
		b.WriteString(f.DrawCircle(cx, cy, dotRadius, dotColor, dotStroke, 1.1))

		runLabel := fmt.Sprintf("R%d", i+1)
		if len(runIDs) == n && runIDs[i] != "" {
			shortID := runIDs[i]
			if len(shortID) > 8 {
				runLabel = shortID[len(shortID)-8:]
			} else {
				runLabel = shortID
			}
		}
		b.WriteString(f.DrawText(cx, float64(f.PlotY+f.PlotHeight)+16, runLabel, "tick-label"))
	}

	b.WriteString(f.Close())
	return b.String()
}

// -------------------------------------------------------------------------- //
// BOX PLOT — SC vs catalog-step coverage
// -------------------------------------------------------------------------- //

// RenderCoverageBoxPlot renders side-by-side Tukey box plots for SC and
// catalog-step coverage. This is the academic distribution figure that the
// mean±SD bar chart cannot show (median, IQR, whiskers, outliers).
func RenderCoverageBoxPlot(rs *analysis.RunSet) string {
	f := NewFigure(AcademicWidth, AcademicHeight)
	f.Title = "Coverage Distributions"
	f.Subtitle = "Tukey box plots (median, IQR, 1.5×IQR whiskers)"
	f.YLabel = "Rate"
	f.XLabel = "Metric"
	f.Caption = fmt.Sprintf("N = %d runs  ·  dashed line is the paper target (SC ≥ 0.80)", rs.RunCount)
	f.Legend = []LegendItem{
		{Label: "SC", Color: ColorSC},
		{Label: "Catalog", Color: ColorCatalogCoverage},
	}

	var b strings.Builder
	b.WriteString(f.Open())

	yMin, yMax := 0.0, 1.05
	b.WriteString(f.DrawYAxis(yMin, yMax, []float64{0.0, 0.25, 0.50, 0.75, 1.0}, "%.2f"))
	b.WriteString(f.DrawHRefLine(0.80, yMin, yMax, "target (0.80)"))

	series := []struct {
		label  string
		values []float64
		color  string
	}{
		{"SC", rs.Global.SC.RawValues, ColorSC},
		{"Catalog", rs.Global.CatalogStepCoverage.RawValues, ColorCatalogCoverage},
	}

	if len(rs.Global.SC.RawValues) == 0 && len(rs.Global.CatalogStepCoverage.RawValues) == 0 {
		b.WriteString(f.EmptyMessage("No run data available"))
		b.WriteString(f.Close())
		return b.String()
	}

	groupWidth := float64(f.PlotWidth) / float64(len(series)+1)
	boxWidth := groupWidth * 0.38

	for i, s := range series {
		cx := float64(f.PlotX) + groupWidth + float64(i)*groupWidth
		b.WriteString(drawBoxPlot(&b, f, cx, boxWidth, s.values, yMin, yMax, s.color))
		b.WriteString(f.DrawText(cx, float64(f.PlotY+f.PlotHeight)+16, s.label, "tick-label"))
	}

	b.WriteString(f.Close())
	return b.String()
}

func drawBoxPlot(b *strings.Builder, f *Figure, cx, boxWidth float64, values []float64, yMin, yMax float64, color string) string {
	if len(values) == 0 {
		return f.DrawText(cx, float64(f.PlotY)+float64(f.PlotHeight)/2, "n/a", "tick-label")
	}
	summ := tukey(values)
	q1Y := f.DataToY(summ.q1, yMin, yMax)
	medY := f.DataToY(summ.median, yMin, yMax)
	q3Y := f.DataToY(summ.q3, yMin, yMax)
	loY := f.DataToY(summ.lo, yMin, yMax)
	hiY := f.DataToY(summ.hi, yMin, yMax)

	// Whiskers
	b.WriteString(f.DrawLine(cx, hiY, cx, q3Y, ColorAxis, 1.2, ""))
	b.WriteString(f.DrawLine(cx, q1Y, cx, loY, ColorAxis, 1.2, ""))
	cap := boxWidth * 0.35
	b.WriteString(f.DrawLine(cx-cap, hiY, cx+cap, hiY, ColorAxis, 1.2, ""))
	b.WriteString(f.DrawLine(cx-cap, loY, cx+cap, loY, ColorAxis, 1.2, ""))

	// IQR box
	boxTop := math.Min(q1Y, q3Y)
	boxH := math.Abs(q1Y - q3Y)
	if boxH < 2 {
		boxH = 2
	}
	b.WriteString(f.DrawRoundRect(cx-boxWidth/2, boxTop, boxWidth, boxH, 2, color, ColorAxis, 1.1))

	// Median
	b.WriteString(f.DrawLine(cx-boxWidth/2, medY, cx+boxWidth/2, medY, "#FFFFFF", 2.2, ""))
	b.WriteString(f.DrawLine(cx-boxWidth/2, medY, cx+boxWidth/2, medY, ColorAxis, 1.4, ""))

	for i, v := range values {
		jitter := ((float64(i) / math.Max(1, float64(len(values)-1))) - 0.5) * boxWidth * 0.45
		b.WriteString(f.DrawCircle(cx+jitter, f.DataToY(v, yMin, yMax), 3.0, ColorCanvas, ColorAxis, 1.1))
	}
	for _, o := range summ.outliers {
		oy := f.DataToY(o, yMin, yMax)
		b.WriteString(f.DrawCircle(cx, oy, 3.2, ColorCanvas, color, 1.3))
	}

	_ = b
	return ""
}

type tukeySummary struct {
	q1, median, q3, lo, hi float64
	outliers               []float64
}

func tukey(values []float64) tukeySummary {
	xs := append([]float64(nil), values...)
	sort.Float64s(xs)
	q1 := percentile(xs, 0.25)
	med := percentile(xs, 0.50)
	q3 := percentile(xs, 0.75)
	iqr := q3 - q1
	fenceLo := q1 - 1.5*iqr
	fenceHi := q3 + 1.5*iqr

	lo, hi := xs[0], xs[len(xs)-1]
	var outliers []float64
	for _, v := range xs {
		if v < fenceLo || v > fenceHi {
			outliers = append(outliers, v)
			continue
		}
		if v < lo || lo < fenceLo {
			lo = v
		}
		if v > hi || hi > fenceHi {
			hi = v
		}
	}
	// Recompute whiskers as most extreme in-fence values.
	lo, hi = xs[len(xs)-1], xs[0]
	for _, v := range xs {
		if v >= fenceLo && v <= fenceHi {
			if v < lo {
				lo = v
			}
			if v > hi {
				hi = v
			}
		}
	}
	if lo > hi {
		lo, hi = xs[0], xs[len(xs)-1]
	}
	return tukeySummary{q1: q1, median: med, q3: q3, lo: lo, hi: hi, outliers: outliers}
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	idx := p * float64(len(sorted)-1)
	lo := int(math.Floor(idx))
	hi := int(math.Ceil(idx))
	if lo == hi {
		return sorted[lo]
	}
	w := idx - float64(lo)
	return sorted[lo]*(1-w) + sorted[hi]*w
}

// -------------------------------------------------------------------------- //
// SPEARMAN SCATTER
// -------------------------------------------------------------------------- //

// RenderSpearmanScatterPlot renders paired SC vs catalog-step coverage points
// so the Spearman result is a real figure, not only a metric card.
func RenderSpearmanScatterPlot(rs *analysis.RunSet) string {
	f := NewFigure(AcademicWidth, AcademicHeight)
	f.Title = "SC vs Catalog Step Coverage"
	f.Subtitle = "Paired per-run observations used by Spearman’s ρ"
	f.XLabel = "Scenario coverage (SC)"
	f.YLabel = "Catalog step coverage"
	f.Caption = spearmanCaption(rs)
	f.Legend = []LegendItem{
		{Label: "Run", Color: ColorDot, Shape: "circle"},
	}

	var b strings.Builder
	b.WriteString(f.Open())

	minV, maxV := 0.0, 1.05
	b.WriteString(f.DrawYAxis(minV, maxV, []float64{0.0, 0.25, 0.50, 0.75, 1.0}, "%.2f"))
	// X ticks without redrawing Y axis
	for _, val := range []float64{0.0, 0.25, 0.50, 0.75, 1.0} {
		x := f.DataToX(val, minV, maxV)
		b.WriteString(f.DrawLine(x, float64(f.PlotY), x, float64(f.PlotY+f.PlotHeight), ColorGrid, 1, ""))
		b.WriteString(f.DrawText(x, float64(f.PlotY+f.PlotHeight)+16, fmt.Sprintf("%.2f", val), "tick-label"))
	}
	b.WriteString(f.DrawLine(float64(f.PlotX), float64(f.PlotY), float64(f.PlotX), float64(f.PlotY+f.PlotHeight), ColorAxis, 1.2, ""))
	b.WriteString(f.DrawLine(float64(f.PlotX), float64(f.PlotY+f.PlotHeight), float64(f.PlotX+f.PlotWidth), float64(f.PlotY+f.PlotHeight), ColorAxis, 1.2, ""))
	b.WriteString(f.DrawHRefLine(0.80, minV, maxV, "SC target"))

	// Identity line y = x
	x0, y0 := f.DataToX(0, minV, maxV), f.DataToY(0, minV, maxV)
	x1, y1 := f.DataToX(1, minV, maxV), f.DataToY(1, minV, maxV)
	b.WriteString(f.DrawLine(x0, y0, x1, y1, ColorMuted, 1, "3,3"))

	xs := rs.Global.SC.RawValues
	ys := rs.Global.CatalogStepCoverage.RawValues
	n := len(xs)
	if len(ys) < n {
		n = len(ys)
	}
	if n == 0 {
		b.WriteString(f.EmptyMessage("No paired observations"))
		b.WriteString(f.Close())
		return b.String()
	}

	for i := 0; i < n; i++ {
		cx := f.DataToX(xs[i], minV, maxV)
		cy := f.DataToY(ys[i], minV, maxV)
		b.WriteString(f.DrawCircle(cx, cy, 5.5, ColorDot, ColorDotStroke, 1.1))
		label := fmt.Sprintf("R%d", i+1)
		if i < len(rs.RunIDs) && rs.RunIDs[i] != "" && len(rs.RunIDs[i]) <= 8 {
			label = rs.RunIDs[i]
		}
		b.WriteString(f.DrawText(cx+10, cy-10, label, "caption"))
	}

	b.WriteString(f.Close())
	return b.String()
}

func spearmanCaption(rs *analysis.RunSet) string {
	if rs.SpearmanCorrelation == nil {
		return fmt.Sprintf("N = %d paired observations  ·  Spearman not computed", rs.RunCount)
	}
	sr := rs.SpearmanCorrelation
	p := "n/a"
	if sr.PValue != nil {
		p = fmt.Sprintf("%.4f", *sr.PValue)
	}
	return fmt.Sprintf("N = %d  ·  ρ = %.3f  ·  p = %s", sr.N, sr.Rho, p)
}

// -------------------------------------------------------------------------- //
// WILCOXON RESULT METRIC DISPLAY
// -------------------------------------------------------------------------- //

// RenderWilcoxonMetricDisplay renders a compact metric display for the Wilcoxon
// signed-rank test result, showing N, W statistic, p-value, and significance.
func RenderWilcoxonMetricDisplay(rs *analysis.RunSet) string {
	return renderStatCard(
		"Wilcoxon Signed-Rank Test",
		"H₀: median SC = 0.80",
		wilcoxonStats(rs),
		wilcoxonNote(rs),
	)
}

func wilcoxonStats(rs *analysis.RunSet) []statCell {
	if rs.WilcoxonSignedRank == nil {
		return nil
	}
	wr := rs.WilcoxonSignedRank
	cells := []statCell{
		{"N (non-zero diffs)", fmt.Sprintf("%d", wr.N)},
		{"W statistic", fmt.Sprintf("%.1f", wr.Statistic)},
	}
	if wr.PValue != nil {
		cells = append(cells, statCell{"p-value (two-sided)", fmt.Sprintf("%.4f", *wr.PValue)})
	}
	if wr.SignificantAt005 != nil {
		sig := "No"
		if *wr.SignificantAt005 {
			sig = "Yes"
		}
		cells = append(cells, statCell{"Significant (p < 0.05)", sig})
	}
	return cells
}

func wilcoxonNote(rs *analysis.RunSet) string {
	if rs.WilcoxonSignedRank == nil {
		return "Test not computed (N < 5)"
	}
	return rs.WilcoxonSignedRank.Interpretation
}

// -------------------------------------------------------------------------- //
// SPEARMAN RESULT METRIC DISPLAY
// -------------------------------------------------------------------------- //

// RenderSpearmanMetricDisplay renders a compact metric display for the Spearman
// correlation result.
func RenderSpearmanMetricDisplay(rs *analysis.RunSet) string {
	return renderStatCard(
		"Spearman Correlation",
		"SC vs catalog-step coverage",
		spearmanStats(rs),
		spearmanNote(rs),
	)
}

func spearmanStats(rs *analysis.RunSet) []statCell {
	if rs.SpearmanCorrelation == nil {
		return nil
	}
	sr := rs.SpearmanCorrelation
	cells := []statCell{
		{"N (paired observations)", fmt.Sprintf("%d", sr.N)},
		{"ρ (rho)", fmt.Sprintf("%.4f", sr.Rho)},
	}
	if sr.PValue != nil {
		cells = append(cells, statCell{"p-value (two-sided)", fmt.Sprintf("%.4f", *sr.PValue)})
	}
	if sr.SignificantAt005 != nil {
		sig := "No"
		if *sr.SignificantAt005 {
			sig = "Yes"
		}
		cells = append(cells, statCell{"Significant (p < 0.05)", sig})
	}
	return cells
}

func spearmanNote(rs *analysis.RunSet) string {
	if rs.SpearmanCorrelation == nil {
		return "Test not computed (N < 3)"
	}
	return rs.SpearmanCorrelation.Interpretation
}

type statCell struct {
	label string
	value string
}

func renderStatCard(title, subtitle string, cells []statCell, note string) string {
	f := NewFigure(AcademicWidth, CompactHeight)
	f.Title = title
	f.Subtitle = subtitle

	var b strings.Builder
	b.WriteString(f.Open())

	cardX := float64(f.PlotX)
	cardY := float64(f.PlotY)
	cardW := float64(f.PlotWidth)
	cardH := float64(f.PlotHeight)
	b.WriteString(f.DrawRoundRect(cardX, cardY, cardW, cardH, 8, ColorMetricBox, ColorPlotStroke, 1))
	b.WriteString(f.DrawRect(cardX, cardY, 6, cardH, ColorSC, "none", 0))

	if len(cells) == 0 {
		b.WriteString(f.DrawText(cardX+cardW/2, cardY+cardH/2, note, "axis-label"))
		b.WriteString(f.Close())
		return b.String()
	}

	cols := len(cells)
	if cols > 4 {
		cols = 4
	}
	cellW := (cardW - 36) / float64(cols)
	for i, cell := range cells {
		if i >= cols {
			break
		}
		cx := cardX + 28 + cellW*float64(i) + cellW/2
		b.WriteString(f.DrawText(cx, cardY+cardH*0.38, cell.value, "stat-value"))
		b.WriteString(f.DrawText(cx, cardY+cardH*0.55, cell.label, "stat-label"))
	}

	if note != "" {
		// Keep interpretation on one line; the markdown table carries the full text.
		if len(note) > 110 {
			note = note[:107] + "..."
		}
		b.WriteString(f.DrawText(cardX+cardW/2, cardY+cardH*0.82, note, "interpretation"))
	}

	b.WriteString(f.Close())
	return b.String()
}

// -------------------------------------------------------------------------- //
// TTC (TIME-TO-CHAIN) BAR CHART
// -------------------------------------------------------------------------- //

// RenderTTCBarchart renders a bar chart for time-to-chain statistics when available.
func RenderTTCBarchart(rs *analysis.RunSet) string {
	if rs.Global.TTC == nil || rs.Global.TTC.N == 0 {
		return ""
	}

	f := NewFigure(AcademicWidth, AcademicHeight)
	f.Title = "Time-to-Chain Statistics"
	f.Subtitle = "Wall-clock time from run start to the first fully validated chain"
	f.YLabel = "Seconds"
	f.XLabel = "Statistic"
	f.Caption = fmt.Sprintf("N = %d runs with a validated chain", rs.Global.TTC.N)

	var b strings.Builder
	b.WriteString(f.Open())

	maxVal := (rs.Global.TTC.Max / 1000.0) * 1.15
	if maxVal == 0 {
		maxVal = 1
	}
	yMin, yMax := 0.0, maxVal

	ticks := make([]float64, 0, 6)
	for i := 0; i <= 4; i++ {
		ticks = append(ticks, maxVal*float64(i)/4)
	}
	b.WriteString(f.DrawYAxis(yMin, yMax, ticks, "%.2f"))

	bars := []struct {
		label string
		val   float64
		color string
	}{
		{"Mean", rs.Global.TTC.Mean / 1000.0, ColorSC},
		{"Min", rs.Global.TTC.Min / 1000.0, ColorCatalogCoverage},
		{"Max", rs.Global.TTC.Max / 1000.0, ColorFamily},
	}

	groupWidth := float64(f.PlotWidth) / float64(len(bars)+1)
	barWidth := groupWidth * 0.42

	for i, bar := range bars {
		groupCenter := float64(f.PlotX) + groupWidth + float64(i)*groupWidth
		barX := groupCenter - barWidth/2
		barY := f.DataToY(bar.val, yMin, yMax)
		barHeight := float64(f.PlotY+f.PlotHeight) - barY
		b.WriteString(f.DrawRoundRect(barX, barY, barWidth, barHeight, 3, bar.color, "none", 0))
		b.WriteString(f.DrawText(groupCenter, float64(f.PlotY+f.PlotHeight)+16, bar.label, "tick-label"))
		b.WriteString(f.DrawText(groupCenter, barY-12, formatSeconds(bar.val*1000), "bar-label"))
	}

	b.WriteString(f.Close())
	return b.String()
}
