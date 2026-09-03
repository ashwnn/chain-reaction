// Package plot provides pure-Go SVG chart generation for analysis outputs.
// All rendering is offline, deterministic, and produces self-contained SVG files
// suitable for inclusion in academic papers and reports.
package plot

import (
	"fmt"
	"math"
	"strings"
)

// Academic figure defaults. Sized for a two-column IEEE figure at print
// resolution while remaining readable on-screen.
const (
	AcademicWidth  = 720
	AcademicHeight = 460
	CompactHeight  = 320
)

// Okabe–Ito colorblind-safe palette plus ink/grid tones.
const (
	ColorSC              = "#0072B2" // blue — scenario coverage
	ColorCatalogCoverage = "#009E73" // bluish green — catalog-step coverage
	ColorErrorBar        = "#333333" // near-black — error bars
	ColorFamily          = "#CC79A7" // reddish purple
	ColorFamilyRel       = "#E69F00" // orange — partial reliability
	ColorGrid            = "#E4E6EA" // light gray — grid lines
	ColorAxis            = "#333333" // dark gray — axis lines
	ColorTargetLine      = "#D55E00" // vermillion — reference/target line
	ColorText            = "#1A1A1A" // near-black — title text
	ColorTickLabel       = "#333333" // dark gray — tick labels
	ColorLegend          = "#333333" // dark gray — legend text
	ColorDot             = "#0072B2" // blue — scatter dots
	ColorDotStroke       = "#004D7A" // dark blue — dot stroke
	ColorNilDot          = "#999999" // gray — null/missing/below-target
	ColorMetricBox       = "#F4F6F8" // light gray — metric display background
	ColorMetricText      = "#1A1A1A" // near-black — metric text
	ColorCanvas          = "#FFFFFF"
	ColorPlotFill        = "#FBFCFD"
	ColorPlotStroke      = "#D7DCE2"
	ColorMuted           = "#5C5C5C"

	ColorTheoretical = "#999999" // gray — theoretical-only steps
	ColorObserved    = "#E69F00" // orange — observed at discovery/scan
	ColorValidated   = "#009E73" // green — runtime-validated steps
	ColorBlocked     = "#D55E00" // vermillion — blocked/never-validated
	ColorPartial     = "#F0E442" // yellow — partially validated
	ColorSky         = "#56B4E9"
)

const fontStack = `"Liberation Sans", "DejaVu Sans", "Nimbus Sans L", Helvetica, Arial, sans-serif`

// Renderer holds SVG rendering configuration and produces SVG markup.
type Renderer struct {
	Width, Height                                    int
	MarginTop, MarginRight, MarginBottom, MarginLeft int

	// Derived plot area dimensions.
	PlotWidth  int
	PlotHeight int
	PlotX      int
	PlotY      int
}

// NewRenderer returns a Renderer configured with academic-plot margins.
// Small canvases scale margins down so the plot area stays usable.
func NewRenderer(width, height int) *Renderer {
	mt, mr, mb, ml := academicMargins(width, height)
	r := &Renderer{
		Width:        width,
		Height:       height,
		MarginTop:    mt,
		MarginRight:  mr,
		MarginBottom: mb,
		MarginLeft:   ml,
	}
	r.recomputePlotArea()
	return r
}

func academicMargins(width, height int) (top, right, bottom, left int) {
	top, right, bottom, left = 72, 36, 68, 76
	if width < 480 {
		left = maxInt(40, width/6)
		right = maxInt(16, width/16)
	}
	if height < 300 {
		top = maxInt(28, height/8)
		bottom = maxInt(32, height/7)
	}
	if width-left-right < 40 {
		left = width / 4
		right = width / 10
		if left+right >= width {
			left, right = width/5, width/10
		}
	}
	if height-top-bottom < 40 {
		top = height / 5
		bottom = height / 5
	}
	return top, right, bottom, left
}

func (r *Renderer) recomputePlotArea() {
	r.PlotWidth = r.Width - r.MarginLeft - r.MarginRight
	r.PlotHeight = r.Height - r.MarginTop - r.MarginBottom
	r.PlotX = r.MarginLeft
	r.PlotY = r.MarginTop
	if r.PlotWidth < 1 {
		r.PlotWidth = 1
	}
	if r.PlotHeight < 1 {
		r.PlotHeight = 1
	}
}

// Header returns the SVG header element with viewBox, namespace, and styles.
func (r *Renderer) Header() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" width="%d" height="%d" role="img">`,
		r.Width, r.Height, r.Width, r.Height))
	b.WriteString("\n")
	b.WriteString("  <style>\n")
	b.WriteString(fmt.Sprintf("    text { font-family: %s; }\n", fontStack))
	b.WriteString("    .title { font-size: 16px; font-weight: 700; fill: #1A1A1A; }\n")
	b.WriteString("    .subtitle { font-size: 11px; fill: #5C5C5C; }\n")
	b.WriteString("    .axis-label { font-size: 12px; font-weight: 600; fill: #333333; }\n")
	b.WriteString("    .tick-label { font-size: 11px; fill: #333333; }\n")
	b.WriteString("    .legend-text { font-size: 11px; fill: #333333; }\n")
	b.WriteString("    .caption { font-size: 10px; fill: #5C5C5C; }\n")
	b.WriteString("    .stat-value { font-size: 20px; font-weight: 700; fill: #1A1A1A; }\n")
	b.WriteString("    .stat-label { font-size: 10px; font-weight: 600; fill: #5C5C5C; letter-spacing: 0.04em; }\n")
	b.WriteString("    .interpretation { font-size: 11px; fill: #333333; }\n")
	b.WriteString("    .grid { stroke: #E4E6EA; stroke-width: 1; }\n")
	b.WriteString("    .axis { stroke: #333333; stroke-width: 1.2; }\n")
	b.WriteString("    .reference-line { stroke: #D55E00; stroke-width: 1.2; stroke-dasharray: 5,3; }\n")
	b.WriteString("    .reference-label { font-size: 10px; font-weight: 600; fill: #D55E00; }\n")
	b.WriteString("    .bar-label { font-size: 11px; font-weight: 600; fill: #1A1A1A; }\n")
	b.WriteString("    .on-bar { font-size: 11px; font-weight: 600; fill: #FFFFFF; }\n")
	b.WriteString("  </style>\n")
	b.WriteString(r.DrawRect(0, 0, float64(r.Width), float64(r.Height), ColorCanvas, "none", 0))
	return b.String()
}

// Footer returns the SVG closing tag.
func (r *Renderer) Footer() string {
	return "</svg>\n"
}

// DrawRect appends a rectangle element.
func (r *Renderer) DrawRect(x, y, w, h float64, fill, stroke string, strokeWidth float64) string {
	if w < 0 {
		x += w
		w = -w
	}
	if h < 0 {
		y += h
		h = -h
	}
	return fmt.Sprintf(`  <rect x="%.2f" y="%.2f" width="%.2f" height="%.2f" fill="%s" stroke="%s" stroke-width="%.1f"/>`+"\n",
		x, y, w, h, fill, stroke, strokeWidth)
}

// DrawRoundRect appends a rectangle with rounded corners.
func (r *Renderer) DrawRoundRect(x, y, w, h, rx float64, fill, stroke string, strokeWidth float64) string {
	if w < 0 {
		x += w
		w = -w
	}
	if h < 0 {
		y += h
		h = -h
	}
	return fmt.Sprintf(`  <rect x="%.2f" y="%.2f" width="%.2f" height="%.2f" rx="%.2f" fill="%s" stroke="%s" stroke-width="%.1f"/>`+"\n",
		x, y, w, h, rx, fill, stroke, strokeWidth)
}

// DrawText appends a text element centered at (x, y).
func (r *Renderer) DrawText(x, y float64, text, cssClass string) string {
	return r.drawAnchoredText(x, y, text, cssClass, "middle")
}

// DrawTextLeft appends a left-aligned text element with baseline at (x, y).
func (r *Renderer) DrawTextLeft(x, y float64, text, cssClass string) string {
	return r.drawAnchoredText(x, y, text, cssClass, "start")
}

// DrawTextRight appends a right-aligned text element with baseline at (x, y).
func (r *Renderer) DrawTextRight(x, y float64, text, cssClass string) string {
	return r.drawAnchoredText(x, y, text, cssClass, "end")
}

func (r *Renderer) drawAnchoredText(x, y float64, text, cssClass, anchor string) string {
	classAttr := ""
	if cssClass != "" {
		classAttr = fmt.Sprintf(` class="%s"`, cssClass)
	}
	return fmt.Sprintf(`  <text x="%.2f" y="%.2f" text-anchor="%s" dominant-baseline="middle"%s>%s</text>`+"\n",
		x, y, anchor, classAttr, r.EscapeXML(text))
}

// DrawTextRotated appends text rotated about (x, y). Degrees is clockwise-positive
// in SVG; use -90 for a conventional left-side Y-axis label.
func (r *Renderer) DrawTextRotated(x, y float64, text, cssClass string, degrees float64) string {
	classAttr := ""
	if cssClass != "" {
		classAttr = fmt.Sprintf(` class="%s"`, cssClass)
	}
	return fmt.Sprintf(`  <text x="%.2f" y="%.2f" text-anchor="middle" dominant-baseline="middle" transform="rotate(%.1f %.2f %.2f)"%s>%s</text>`+"\n",
		x, y, degrees, x, y, classAttr, r.EscapeXML(text))
}

// DrawLine appends a line element.
func (r *Renderer) DrawLine(x1, y1, x2, y2 float64, stroke string, strokeWidth float64, dashArray string) string {
	dash := ""
	if dashArray != "" {
		dash = fmt.Sprintf(` stroke-dasharray="%s"`, dashArray)
	}
	return fmt.Sprintf(`  <line x1="%.2f" y1="%.2f" x2="%.2f" y2="%.2f" stroke="%s" stroke-width="%.1f" stroke-linecap="square"%s/>`+"\n",
		x1, y1, x2, y2, stroke, strokeWidth, dash)
}

// DrawCircle appends a circle element.
func (r *Renderer) DrawCircle(cx, cy, rCircle float64, fill, stroke string, strokeWidth float64) string {
	return fmt.Sprintf(`  <circle cx="%.2f" cy="%.2f" r="%.2f" fill="%s" stroke="%s" stroke-width="%.1f"/>`+"\n",
		cx, cy, rCircle, fill, stroke, strokeWidth)
}

// DrawErrorBar appends a vertical error bar (±err) centered at (cx, yCenter).
func (r *Renderer) DrawErrorBar(cx, yCenter, err, barWidth float64, stroke string) string {
	var b strings.Builder
	yTop := yCenter - err
	yBottom := yCenter + err
	b.WriteString(fmt.Sprintf(`  <line x1="%.2f" y1="%.2f" x2="%.2f" y2="%.2f" stroke="%s" stroke-width="1.2"/>`+"\n",
		cx, yTop, cx, yBottom, stroke))
	b.WriteString(fmt.Sprintf(`  <line x1="%.2f" y1="%.2f" x2="%.2f" y2="%.2f" stroke="%s" stroke-width="1.2"/>`+"\n",
		cx-barWidth/2, yTop, cx+barWidth/2, yTop, stroke))
	b.WriteString(fmt.Sprintf(`  <line x1="%.2f" y1="%.2f" x2="%.2f" y2="%.2f" stroke="%s" stroke-width="1.2"/>`+"\n",
		cx-barWidth/2, yBottom, cx+barWidth/2, yBottom, stroke))
	return b.String()
}

// EscapeXML escapes special XML characters in text content.
func (r *Renderer) EscapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}

// DataToY converts a data value in [minVal, maxVal] to an SVG Y coordinate
// within the plot area. yMin corresponds to the bottom of the plot area.
func (r *Renderer) DataToY(val, minVal, maxVal float64) float64 {
	if maxVal == minVal {
		return float64(r.PlotY) + float64(r.PlotHeight)/2
	}
	frac := (val - minVal) / (maxVal - minVal)
	return float64(r.PlotY) + float64(r.PlotHeight) - frac*float64(r.PlotHeight)
}

// DataToX converts a data value in [minVal, maxVal] to an SVG X coordinate
// within the plot area. xMin corresponds to the left edge.
func (r *Renderer) DataToX(val, minVal, maxVal float64) float64 {
	if maxVal == minVal {
		return float64(r.PlotX) + float64(r.PlotWidth)/2
	}
	frac := (val - minVal) / (maxVal - minVal)
	return float64(r.PlotX) + frac*float64(r.PlotWidth)
}

// PlotArea returns the bounding box of the plot area as (x, y, width, height).
func (r *Renderer) PlotArea() (x, y, w, h int) {
	return r.PlotX, r.PlotY, r.PlotWidth, r.PlotHeight
}

// LegendItem is one entry in a figure legend.
type LegendItem struct {
	Label string
	Color string
	Shape string // "rect" (default) or "circle"
}

// Figure is a complete academic plot: canvas, title, axes, legend, caption.
type Figure struct {
	*Renderer
	Title    string
	Subtitle string
	XLabel   string
	YLabel   string
	Caption  string
	Legend   []LegendItem
}

// NewFigure returns an academic-sized figure renderer.
func NewFigure(width, height int) *Figure {
	return &Figure{Renderer: NewRenderer(width, height)}
}

// Open writes the SVG header, canvas, title block, optional legend, and plot frame.
func (f *Figure) Open() string {
	if len(f.Legend) > 0 && f.MarginTop < 96 {
		f.MarginTop = 96
		f.recomputePlotArea()
	}
	var b strings.Builder
	b.WriteString(f.Header())
	b.WriteString(f.drawChrome())
	return b.String()
}

func (f *Figure) drawChrome() string {
	var b strings.Builder
	cx := float64(f.Width) / 2

	titleY := 22.0
	if f.Height < 280 {
		titleY = 16.0
	}
	if f.Title != "" {
		b.WriteString(f.DrawText(cx, titleY, f.Title, "title"))
	}
	if f.Subtitle != "" {
		b.WriteString(f.DrawText(cx, titleY+16, f.Subtitle, "subtitle"))
	}

	legendY := float64(f.PlotY) - 24
	if len(f.Legend) > 0 && legendY > titleY+8 {
		b.WriteString(f.drawLegendRow(legendY))
	}

	// Plot area fill and border sit behind series.
	b.WriteString(f.DrawRect(float64(f.PlotX), float64(f.PlotY), float64(f.PlotWidth), float64(f.PlotHeight), ColorPlotFill, ColorPlotStroke, 1))

	if f.YLabel != "" {
		b.WriteString(f.DrawTextRotated(16, float64(f.PlotY)+float64(f.PlotHeight)/2, f.YLabel, "axis-label", -90))
	}
	if f.XLabel != "" {
		xLabelY := float64(f.Height) - 22
		if f.Caption != "" {
			xLabelY = float64(f.Height) - 30
		}
		b.WriteString(f.DrawText(cx, xLabelY, f.XLabel, "axis-label"))
	}
	if f.Caption != "" {
		b.WriteString(f.DrawText(cx, float64(f.Height)-12, f.Caption, "caption"))
	}
	return b.String()
}

func (f *Figure) drawLegendRow(y float64) string {
	const itemWidth = 92.0
	const swatch = 10.0
	total := float64(len(f.Legend)) * itemWidth
	startX := float64(f.PlotX) + (float64(f.PlotWidth)-total)/2
	if startX < float64(f.PlotX) {
		startX = float64(f.PlotX)
	}
	var b strings.Builder
	for i, item := range f.Legend {
		x := startX + float64(i)*itemWidth
		shape := item.Shape
		if shape == "" {
			shape = "rect"
		}
		if shape == "circle" {
			b.WriteString(f.DrawCircle(x+5, y, 4.5, item.Color, ColorAxis, 0.8))
		} else {
			b.WriteString(f.DrawRoundRect(x, y-5, swatch, swatch, 1.5, item.Color, "none", 0))
		}
		b.WriteString(f.DrawTextLeft(x+swatch+6, y, item.Label, "legend-text"))
	}
	return b.String()
}

// DrawYAxis draws a numeric Y axis with gridlines and tick labels.
func (f *Figure) DrawYAxis(minVal, maxVal float64, ticks []float64, format string) string {
	if format == "" {
		format = "%.2f"
	}
	var b strings.Builder
	for _, val := range ticks {
		y := f.DataToY(val, minVal, maxVal)
		b.WriteString(f.DrawLine(float64(f.PlotX), y, float64(f.PlotX+f.PlotWidth), y, ColorGrid, 1, ""))
		b.WriteString(f.DrawTextRight(float64(f.PlotX)-8, y, fmt.Sprintf(format, val), "tick-label"))
	}
	b.WriteString(f.DrawLine(float64(f.PlotX), float64(f.PlotY), float64(f.PlotX), float64(f.PlotY+f.PlotHeight), ColorAxis, 1.2, ""))
	b.WriteString(f.DrawLine(float64(f.PlotX), float64(f.PlotY+f.PlotHeight), float64(f.PlotX+f.PlotWidth), float64(f.PlotY+f.PlotHeight), ColorAxis, 1.2, ""))
	return b.String()
}

// DrawXAxis draws a numeric X axis with gridlines and tick labels.
func (f *Figure) DrawXAxis(minVal, maxVal float64, ticks []float64, format string) string {
	if format == "" {
		format = "%.2f"
	}
	var b strings.Builder
	for _, val := range ticks {
		x := f.DataToX(val, minVal, maxVal)
		b.WriteString(f.DrawLine(x, float64(f.PlotY), x, float64(f.PlotY+f.PlotHeight), ColorGrid, 1, ""))
		b.WriteString(f.DrawText(x, float64(f.PlotY+f.PlotHeight)+16, fmt.Sprintf(format, val), "tick-label"))
	}
	b.WriteString(f.DrawLine(float64(f.PlotX), float64(f.PlotY), float64(f.PlotX), float64(f.PlotY+f.PlotHeight), ColorAxis, 1.2, ""))
	b.WriteString(f.DrawLine(float64(f.PlotX), float64(f.PlotY+f.PlotHeight), float64(f.PlotX+f.PlotWidth), float64(f.PlotY+f.PlotHeight), ColorAxis, 1.2, ""))
	return b.String()
}

// DrawHRefLine draws a horizontal reference line with a right-aligned label.
func (f *Figure) DrawHRefLine(val, minVal, maxVal float64, label string) string {
	y := f.DataToY(val, minVal, maxVal)
	var b strings.Builder
	b.WriteString(f.DrawLine(float64(f.PlotX), y, float64(f.PlotX+f.PlotWidth), y, ColorTargetLine, 1.2, "5,3"))
	if label != "" {
		b.WriteString(f.DrawTextRight(float64(f.PlotX+f.PlotWidth)-4, y-10, label, "reference-label"))
	}
	return b.String()
}

// DrawVRefLine draws a vertical reference line with a label above the plot.
func (f *Figure) DrawVRefLine(val, minVal, maxVal float64, label string) string {
	x := f.DataToX(val, minVal, maxVal)
	var b strings.Builder
	b.WriteString(f.DrawLine(x, float64(f.PlotY), x, float64(f.PlotY+f.PlotHeight), ColorTargetLine, 1.2, "5,3"))
	if label != "" {
		b.WriteString(f.DrawText(x, float64(f.PlotY)-10, label, "reference-label"))
	}
	return b.String()
}

// EmptyMessage writes a centered placeholder in the plot area.
func (f *Figure) EmptyMessage(msg string) string {
	return f.DrawText(float64(f.PlotX)+float64(f.PlotWidth)/2, float64(f.PlotY)+float64(f.PlotHeight)/2, msg, "axis-label")
}

// Close writes the SVG footer.
func (f *Figure) Close() string {
	return f.Footer()
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func clamp01(v float64) float64 {
	return math.Max(0, math.Min(1, v))
}

func formatSeconds(ms float64) string {
	sec := ms / 1000.0
	if math.Abs(sec) >= 10 {
		return fmt.Sprintf("%.1fs", sec)
	}
	return fmt.Sprintf("%.2fs", sec)
}
