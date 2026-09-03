package table

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/ashwnn/chain-reaction/internal/analysis"
	"github.com/ashwnn/chain-reaction/internal/mdtable"
)

func WriteGlobalStatsTable(w io.Writer, rs *analysis.RunSet) {
	fmt.Fprintln(w, "# Analysis: Global Descriptive Statistics")
	fmt.Fprintln(w)
	title := fmt.Sprintf("Run Set: %s", rs.ID)
	if rs.Label != "" {
		title += fmt.Sprintf(" (%s)", rs.Label)
	}
	fmt.Fprintln(w, title)
	fmt.Fprintf(w, "Runs: %d | Source: %s\n\n", rs.RunCount, rs.SourceDir)

	rows := [][]string{
		{
			"Scenario Coverage (SC)",
			fmt.Sprintf("%d", rs.Global.SC.N),
			formatMeanSD(rs.Global.SC.Mean, rs.Global.SC.SD),
			fmt.Sprintf("%.2f", rs.Global.SC.CV),
			fmt.Sprintf("%.4f", rs.Global.SC.Min),
			fmt.Sprintf("%.4f", rs.Global.SC.Max),
			ptrStr(&rs.Global.SC.VarianceFlag),
		},
		{
			"Catalog Step Coverage",
			fmt.Sprintf("%d", rs.Global.CatalogStepCoverage.N),
			formatMeanSD(rs.Global.CatalogStepCoverage.Mean, rs.Global.CatalogStepCoverage.SD),
			fmt.Sprintf("%.2f", rs.Global.CatalogStepCoverage.CV),
			fmt.Sprintf("%.4f", rs.Global.CatalogStepCoverage.Min),
			fmt.Sprintf("%.4f", rs.Global.CatalogStepCoverage.Max),
			ptrStr(&rs.Global.CatalogStepCoverage.VarianceFlag),
		},
	}
	if rs.Global.TTC != nil && rs.Global.TTC.N > 0 {
		rows = append(rows, []string{
			"Time-to-Chain (TTC, s)",
			fmt.Sprintf("%d", rs.Global.TTC.N),
			fmt.Sprintf("%.2f±%.2f", rs.Global.TTC.Mean/1000, rs.Global.TTC.SD/1000),
			fmt.Sprintf("%.2f", rs.Global.TTC.CV),
			fmt.Sprintf("%.2f", rs.Global.TTC.Min/1000),
			fmt.Sprintf("%.2f", rs.Global.TTC.Max/1000),
			ptrStr(&rs.Global.TTC.VarianceFlag),
		})
	}
	mdtable.Write(w, []string{"Metric", "N", "Mean±SD", "CV (%)", "Min", "Max", "Variance Flag"}, rows)
	fmt.Fprintln(w)
}

func WriteFamilyReliabilityTable(w io.Writer, rs *analysis.RunSet) {
	fmt.Fprintln(w, "# Analysis: Per-Family Chain Reliability")
	fmt.Fprintln(w)

	families := make([]analysis.FamilyStats, len(rs.PerFamily))
	copy(families, rs.PerFamily)
	sort.Slice(families, func(i, j int) bool {
		return families[i].FamilyID < families[j].FamilyID
	})

	rows := make([][]string, 0, len(families))
	for _, fam := range families {
		relStr := "N/A"
		if fam.ReliabilityFraction != nil {
			relStr = fmt.Sprintf("%.0f%%", *fam.ReliabilityFraction*100)
		}
		svrMeanSD := "N/A"
		if fam.CatalogCoverageSample.N > 0 {
			svrMeanSD = formatMeanSD(fam.CatalogCoverageSample.Mean, fam.CatalogCoverageSample.SD)
		}
		varianceFlag := ptrStr(&fam.CatalogCoverageSample.VarianceFlag)
		if fam.CatalogCoverageSample.N == 0 {
			varianceFlag = "N/A"
		}
		rows = append(rows, []string{
			fam.FamilyID,
			fmt.Sprintf("%d", fam.Validated),
			fmt.Sprintf("%d", fam.Attempted),
			relStr,
			svrMeanSD,
			fmt.Sprintf("%.2f", fam.CatalogCoverageSample.CV),
			varianceFlag,
		})
	}
	mdtable.Write(w, []string{"Family", "Validated", "Attempted", "Reliability", "Catalog Coverage Mean±SD", "Catalog Coverage CV (%)", "Variance Flag"}, rows)
	fmt.Fprintln(w)
}

func WriteStatisticalTestsTable(w io.Writer, rs *analysis.RunSet) {
	fmt.Fprintln(w, "# Analysis: Statistical Tests")
	fmt.Fprintln(w)

	if rs.WilcoxonSignedRank == nil && rs.SpearmanCorrelation == nil {
		fmt.Fprintln(w, "No statistical tests computed.")
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Note: Wilcoxon signed-rank requires ≥5 runs with SC values. Spearman correlation requires ≥3 paired SC/catalog-step coverage observations with variation in both series.")
		return
	}

	rows := make([][]string, 0, 2)
	if rs.WilcoxonSignedRank != nil {
		wr := rs.WilcoxonSignedRank
		sigStr := "No"
		if wr.SignificantAt005 != nil && *wr.SignificantAt005 {
			sigStr = "Yes"
		}
		pValStr := "N/A"
		if wr.PValue != nil {
			pValStr = fmt.Sprintf("%.4f", *wr.PValue)
		}
		interp := wr.Interpretation
		if interp == "" {
			interp = "—"
		}
		rows = append(rows, []string{
			"Wilcoxon Signed-Rank",
			"H₀: median SC = 0.80",
			fmt.Sprintf("%d", wr.N),
			fmt.Sprintf("W=%.1f", wr.Statistic),
			pValStr,
			sigStr,
			interp,
		})
	} else {
		rows = append(rows, []string{"Wilcoxon Signed-Rank", "H₀: median SC = 0.80", "<5 runs (insufficient data)", "—", "—", "—", "Requires ≥5 runs with SC values"})
	}

	if rs.SpearmanCorrelation != nil {
		sr := rs.SpearmanCorrelation
		sigStr := "No"
		if sr.SignificantAt005 != nil && *sr.SignificantAt005 {
			sigStr = "Yes"
		}
		pValStr := "N/A"
		if sr.PValue != nil {
			pValStr = fmt.Sprintf("%.4f", *sr.PValue)
		}
		interp := sr.Interpretation
		if interp == "" {
			interp = "—"
		}
		rows = append(rows, []string{
			"Spearman Correlation",
			"SC vs catalog-step coverage",
			fmt.Sprintf("%d", sr.N),
			fmt.Sprintf("rho=%.4f", sr.Rho),
			pValStr,
			sigStr,
			interp,
		})
	} else {
		rows = append(rows, []string{"Spearman Correlation", "SC vs catalog-step coverage", "insufficient data", "—", "—", "—", "Requires ≥3 paired (SC, catalog-step coverage) observations with variation"})
	}

	mdtable.Write(w, []string{"Test", "Hypotheses", "N", "Statistic", "P-value", "Significant (p<0.05)", "Interpretation"}, rows)
	fmt.Fprintln(w)
}

func WriteRawValuesTable(w io.Writer, rs *analysis.RunSet) {
	fmt.Fprintln(w, "# Analysis: Per-Run Raw Values")
	fmt.Fprintln(w)

	scValues := rs.Global.SC.RawValues
	svrValues := rs.Global.CatalogStepCoverage.RawValues
	runIDs := rs.RunIDs
	n := len(scValues)
	if len(svrValues) > n {
		n = len(svrValues)
	}

	rows := make([][]string, 0, n)
	for i := 0; i < n; i++ {
		runLabel := fmt.Sprintf("R%d", i+1)
		if len(runIDs) == n && runIDs[i] != "" {
			runLabel = runIDs[i]
		}
		scStr := "null"
		if i < len(scValues) && !isNaN(scValues[i]) {
			scStr = fmt.Sprintf("%.4f", scValues[i])
		}
		svrStr := "null"
		if i < len(svrValues) && !isNaN(svrValues[i]) {
			svrStr = fmt.Sprintf("%.4f", svrValues[i])
		}
		rows = append(rows, []string{runLabel, scStr, svrStr, "N/A (TTC raw values not persisted)"})
	}
	mdtable.Write(w, []string{"Run", "SC", "Catalog Step Coverage", "TTC (s)"}, rows)
	fmt.Fprintln(w)
}

func WriteAllAnalysisTables(w io.Writer, rs *analysis.RunSet) {
	WriteGlobalStatsTable(w, rs)
	WriteFamilyReliabilityTable(w, rs)
	WriteStatisticalTestsTable(w, rs)
	WriteRawValuesTable(w, rs)
}

func formatMeanSD(mean, sd float64) string {
	if sd == 0 {
		return fmt.Sprintf("%.4f", mean)
	}
	return fmt.Sprintf("%.4f±%.4f", mean, sd)
}

func ptrStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func isNaN(v float64) bool {
	return v != v
}

func sanitizeForMarkdown(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}
