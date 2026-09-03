// Package plot provides pure-Go SVG chart generation for analysis and comparison outputs.
// All rendering is offline, deterministic, and produces self-contained SVG files
// suitable for inclusion in academic papers and reports.
package plot

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/ashwnn/chain-reaction/internal/compare"
)

// -------------------------------------------------------------------------- //
// THEORY-SCAN-RUNTIME GAP CHART
// -------------------------------------------------------------------------- //

// RenderTheoryScanRuntimeGapChart renders a grouped bar chart showing coverage at
// each artifact level (theory, scan, runtime) for each in-scope family.
func RenderTheoryScanRuntimeGapChart(result *compare.Result) string {
	f := NewFigure(AcademicWidth, AcademicHeight)
	f.Title = "Theory vs. Scan vs. Runtime Coverage"
	f.Subtitle = "Catalog steps confirmed at each artifact level"
	f.YLabel = "Steps"
	f.XLabel = "Scenario family"
	f.Legend = []LegendItem{
		{Label: "Theory", Color: ColorTheoretical},
		{Label: "Scan", Color: ColorObserved},
		{Label: "Runtime", Color: ColorValidated},
	}

	var b strings.Builder
	b.WriteString(f.Open())

	families := inScopeFamilies(result)
	if len(families) == 0 {
		b.WriteString(f.EmptyMessage("No in-scope family data"))
		b.WriteString(f.Close())
		return b.String()
	}

	maxSteps := 0
	counts := make([][3]int, len(families))
	for i, family := range families {
		t, s, r := stepCounts(family)
		counts[i] = [3]int{t, s, r}
		if t > maxSteps {
			maxSteps = t
		}
		if s > maxSteps {
			maxSteps = s
		}
		if r > maxSteps {
			maxSteps = r
		}
	}
	if maxSteps < 3 {
		maxSteps = 3
	}

	yMin, yMax := 0.0, float64(maxSteps)
	ticks := make([]float64, 0, maxSteps+1)
	for i := 0; i <= maxSteps; i++ {
		ticks = append(ticks, float64(i))
	}
	b.WriteString(f.DrawYAxis(yMin, yMax, ticks, "%.0f"))

	groupWidth := float64(f.PlotWidth) / float64(len(families))
	barCount := 3
	barWidth := groupWidth * 0.18
	inner := (groupWidth - barWidth*float64(barCount)) / float64(barCount+1)

	for i, family := range families {
		runtimeColor := ColorValidated
		if family.Runtime != nil && family.Runtime.AttemptedCount > 0 && family.Runtime.ReliabilityFraction != nil {
			frac := *family.Runtime.ReliabilityFraction
			if frac == 0 {
				runtimeColor = ColorBlocked
			} else if frac > 0 && frac < 1 {
				runtimeColor = ColorPartial
			}
		}
		familyColors := []string{ColorTheoretical, ColorObserved, runtimeColor}
		groupOrigin := float64(f.PlotX) + float64(i)*groupWidth

		for j := 0; j < barCount; j++ {
			count := counts[i][j]
			barX := groupOrigin + inner + float64(j)*(barWidth+inner)
			if count <= 0 {
				continue
			}
			barY := f.DataToY(float64(count), yMin, yMax)
			barHeight := float64(f.PlotY+f.PlotHeight) - barY
			if barHeight > 0 {
				b.WriteString(f.DrawRoundRect(barX, barY, barWidth, barHeight, 2, familyColors[j], "none", 0))
			}
			labelClass := "on-bar"
			if familyColors[j] == ColorPartial || familyColors[j] == ColorObserved || familyColors[j] == ColorTheoretical {
				labelClass = "tick-label"
			}
			if barHeight > 16 {
				b.WriteString(f.DrawText(barX+barWidth/2, barY+barHeight/2, fmt.Sprintf("%d", count), labelClass))
			} else {
				b.WriteString(f.DrawText(barX+barWidth/2, barY-10, fmt.Sprintf("%d", count), "tick-label"))
			}
		}

		b.WriteString(f.DrawText(groupOrigin+groupWidth/2, float64(f.PlotY+f.PlotHeight)+16, family.FamilyID, "tick-label"))
	}

	b.WriteString(f.Close())
	return b.String()
}

func stepCounts(family compare.FamilyResult) (theory, scan, runtime int) {
	for _, step := range family.Steps {
		if step.TheoryStatus != nil {
			theory++
		}
		if step.ScanStatus != nil && *step.ScanStatus == "observed" {
			scan++
		}
		if step.RuntimeStatus != nil && *step.RuntimeStatus == "validated" {
			runtime++
		}
	}
	return
}

func inScopeFamilies(result *compare.Result) []compare.FamilyResult {
	var families []compare.FamilyResult
	for _, f := range result.Families {
		if f.InScope {
			families = append(families, f)
		}
	}
	sort.Slice(families, func(i, j int) bool {
		return families[i].FamilyID < families[j].FamilyID
	})
	return families
}

// -------------------------------------------------------------------------- //
// STEP-LEVEL STATUS HEATMAP
// -------------------------------------------------------------------------- //

// RenderStepLevelHeatmap renders a compact heatmap/grid showing the status
// of each step at each artifact level (theory, scan, runtime) for each
// in-scope family. Each cell is color-coded by status.
func RenderStepLevelHeatmap(result *compare.Result) string {
	families := inScopeFamilies(result)
	maxSteps := 1
	for _, fam := range families {
		if len(fam.Steps) > maxSteps {
			maxSteps = len(fam.Steps)
		}
	}

	rowH := 32.0
	headerH := 28.0
	legendH := 36.0
	height := 130 + int(headerH+rowH*float64(maxInt(len(families), 1))+legendH)
	if height < 380 {
		height = 380
	}

	f := NewFigure(AcademicWidth, height)
	f.Title = "Step-Level Status Heatmap"
	f.Subtitle = "Runtime status when present; otherwise scan; otherwise theory"
	f.Legend = []LegendItem{
		{Label: "T = Theoretical", Color: ColorTheoretical},
		{Label: "O = Observed", Color: ColorObserved},
		{Label: "V = Validated", Color: ColorValidated},
		{Label: "X = Not validated", Color: ColorBlocked},
	}

	var b strings.Builder
	b.WriteString(f.Open())

	if len(families) == 0 {
		b.WriteString(f.EmptyMessage("No in-scope family data"))
		b.WriteString(f.Close())
		return b.String()
	}

	tableX := float64(f.PlotX)
	tableY := float64(f.PlotY) + 4
	tableW := float64(f.PlotWidth)
	colFamily := 72.0
	colLevel := 58.0
	remain := tableW - colFamily - colLevel*3
	colW := remain / float64(maxSteps)
	if colW < 32 {
		colW = 32
	}

	headerBg := "#EEF1F4"
	b.WriteString(f.DrawRect(tableX, tableY, colFamily, headerH, headerBg, ColorAxis, 0.8))
	b.WriteString(f.DrawText(tableX+colFamily/2, tableY+headerH/2, "Family", "tick-label"))
	levelX := tableX + colFamily
	for i, level := range []string{"Theory", "Scan", "Runtime"} {
		lx := levelX + float64(i)*colLevel
		b.WriteString(f.DrawRect(lx, tableY, colLevel, headerH, headerBg, ColorAxis, 0.8))
		b.WriteString(f.DrawText(lx+colLevel/2, tableY+headerH/2, level, "tick-label"))
	}
	stepHeaderX := levelX + colLevel*3
	for s := 0; s < maxSteps; s++ {
		sx := stepHeaderX + float64(s)*colW
		b.WriteString(f.DrawRect(sx, tableY, colW, headerH, headerBg, ColorAxis, 0.8))
		b.WriteString(f.DrawText(sx+colW/2, tableY+headerH/2, fmt.Sprintf("S%d", s+1), "tick-label"))
	}

	for row, family := range families {
		rowY := tableY + headerH + float64(row)*rowH
		bg := ColorCanvas
		if row%2 == 1 {
			bg = "#F7F8FA"
		}
		b.WriteString(f.DrawRect(tableX, rowY, tableW, rowH, bg, "none", 0))
		b.WriteString(f.DrawRect(tableX, rowY, colFamily, rowH, bg, ColorAxis, 0.6))
		b.WriteString(f.DrawTextLeft(tableX+6, rowY+rowH/2, family.FamilyID, "tick-label"))

		tCount, sCount, rCount := stepCounts(family)
		levelCounts := []int{tCount, sCount, rCount}
		levelColors := []string{ColorTheoretical, ColorObserved, ColorValidated}
		for i, count := range levelCounts {
			lx := levelX + float64(i)*colLevel
			cellColor := levelColors[i]
			if count == 0 {
				cellColor = "#F2F3F5"
			}
			b.WriteString(f.DrawRect(lx, rowY, colLevel, rowH, cellColor, ColorAxis, 0.6))
			if count > 0 {
				cls := "tick-label"
				if i == 2 && cellColor != ColorPartial {
					cls = "on-bar"
				}
				b.WriteString(f.DrawText(lx+colLevel/2, rowY+rowH/2, fmt.Sprintf("%d/%d", count, len(family.Steps)), cls))
			}
		}

		for s := 0; s < maxSteps; s++ {
			sx := stepHeaderX + float64(s)*colW
			cellColor := "#F2F3F5"
			statusText := ""
			if s < len(family.Steps) {
				cellColor, statusText = heatmapCell(family.Steps[s])
			}
			b.WriteString(f.DrawRect(sx, rowY, colW, rowH, cellColor, ColorAxis, 0.6))
			if statusText != "" {
				cls := "tick-label"
				if statusText == "V" || statusText == "X" {
					cls = "on-bar"
				}
				b.WriteString(f.DrawText(sx+colW/2, rowY+rowH/2, statusText, cls))
			}
		}
	}

	b.WriteString(f.Close())
	return b.String()
}

func heatmapCell(step compare.StepResult) (color, text string) {
	if step.RuntimeStatus != nil {
		switch *step.RuntimeStatus {
		case "validated":
			return ColorValidated, "V"
		case "not_validated":
			return ColorBlocked, "X"
		default:
			return ColorPartial, "?"
		}
	}
	if step.ScanStatus != nil {
		if *step.ScanStatus == "observed" {
			return ColorObserved, "O"
		}
		return "#E0E0E0", "-"
	}
	if step.TheoryStatus != nil {
		return ColorTheoretical, "T"
	}
	return "#F5F5F5", "-"
}

// -------------------------------------------------------------------------- //
// FAMILY CHAIN STATUS SUMMARY CHART
// -------------------------------------------------------------------------- //

// RenderFamilyChainStatusChart renders a horizontal bar chart showing the
// chain validation status for each in-scope family: validated, partial, or blocked.
func RenderFamilyChainStatusChart(result *compare.Result) string {
	f := NewFigure(AcademicWidth, AcademicHeight)
	f.Title = "Per-Family Chain Validation Status"
	f.Subtitle = "Runtime reliability of the full catalog chain"
	f.XLabel = "Reliability (validated / attempted)"
	f.Legend = []LegendItem{
		{Label: "Validated", Color: ColorValidated},
		{Label: "Partial", Color: ColorPartial},
		{Label: "Blocked", Color: ColorBlocked},
		{Label: "No data", Color: ColorTheoretical},
	}

	var b strings.Builder
	b.WriteString(f.Open())

	xMin, xMax := 0.0, 1.0
	b.WriteString(f.DrawXAxis(xMin, xMax, []float64{0.0, 0.25, 0.50, 0.75, 1.0}, "%.2f"))

	families := inScopeFamilies(result)
	if len(families) == 0 {
		b.WriteString(f.EmptyMessage("No in-scope family data"))
		b.WriteString(f.Close())
		return b.String()
	}

	n := float64(len(families))
	slot := float64(f.PlotHeight) / n
	barHeight := math.Min(slot*0.55, 28)

	for i, family := range families {
		reliability := 0.0
		if family.Runtime != nil && family.Runtime.ReliabilityFraction != nil {
			reliability = *family.Runtime.ReliabilityFraction
		}
		reliability = clamp01(reliability)

		barY := float64(f.PlotY) + slot*float64(i) + (slot-barHeight)/2
		barX := float64(f.PlotX)
		barWidthPx := float64(f.PlotWidth) * reliability

		barColor := ColorBlocked
		statusText := "blocked"
		if family.Runtime == nil || family.Runtime.AttemptedCount == 0 {
			barColor = ColorTheoretical
			statusText = "no runtime"
		} else if reliability >= 1.0 {
			barColor = ColorValidated
			statusText = "100%"
		} else if reliability > 0 {
			barColor = ColorPartial
			statusText = fmt.Sprintf("%.0f%%", reliability*100)
		}

		if barWidthPx > 0 {
			b.WriteString(f.DrawRoundRect(barX, barY, barWidthPx, barHeight, 3, barColor, "none", 0))
		}

		labelY := barY + barHeight/2
		b.WriteString(f.DrawTextRight(float64(f.PlotX)-8, labelY, family.FamilyID, "tick-label"))
		if barWidthPx > 70 {
			cls := "on-bar"
			if barColor == ColorPartial || barColor == ColorTheoretical {
				cls = "tick-label"
			}
			b.WriteString(f.DrawText(barX+barWidthPx/2, labelY, statusText, cls))
		} else {
			b.WriteString(f.DrawTextLeft(barX+barWidthPx+8, labelY, statusText, "tick-label"))
		}
	}

	b.WriteString(f.Close())
	return b.String()
}
