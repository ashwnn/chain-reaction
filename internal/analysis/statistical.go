package analysis

import (
	"errors"
	"math"
	"sort"
	"strings"
)

// errInsufficientData is returned when there are not enough observations
// to perform a statistical test.
var errInsufficientData = errors.New("insufficient data for statistical test")

// -------------------------------------------------------------------------- //
// WILCOXON SIGNED-RANK TEST (one-sample, testing against hypothesized median)
// -------------------------------------------------------------------------- //

// WilcoxonSignedRank performs a one-sample Wilcoxon signed-rank test to determine
// whether the median of the sample differs significantly from the hypothesized
// median. This is the non-parametric alternative to the one-sample t-test.
//
// It returns a WilcoxonResult populated with the test statistic, p-value,
// significance flag, and a human-readable interpretation.
//
// Returns errInsufficientData when len(values) < 5, or when all values equal
// the hypothesized median (zero differences).
//
// The test uses the normal approximation with continuity correction when n ≥ 10,
// and exact enumeration when n < 10.
func WilcoxonAgainstTarget(values []float64, hypothesizedMedian float64) (*WilcoxonResult, error) {
	return wilcoxonSignedRank(values, hypothesizedMedian)
}

func wilcoxonSignedRank(values []float64, hypothesizedMedian float64) (*WilcoxonResult, error) {
	// Filter NaN and collect non-zero differences.
	var diffs []float64
	for _, v := range values {
		if !math.IsNaN(v) {
			d := v - hypothesizedMedian
			diffs = append(diffs, d)
		}
	}

	n := len(diffs)
	if n < 5 {
		return nil, errInsufficientData
	}

	// Rank absolute differences, preserving sign. We pair each diff's absolute
	// value with its original index so ranks are assigned in absolute-value order,
	// and store the rank with sign (positive rank for positive diffs, negative for
	// negative diffs). This avoids the floating-point tie problem where two
	// values that should be equal (e.g. 0.82-0.80 and 0.80-0.78 ≈ 0.02) can
	// have slightly different float64 representations.
	type rankedDiff struct {
		idx        int
		absVal     float64
		signedRank float64 // positive for positive diffs, negative for negative diffs
	}

	// Build list of non-zero diffs with their signs and absolute values.
	var ranked []rankedDiff
	for i, d := range diffs {
		if d == 0 {
			continue
		}
		ranked = append(ranked, rankedDiff{idx: i, absVal: math.Abs(d), signedRank: 0})
	}

	if len(ranked) == 0 {
		return nil, errInsufficientData
	}

	// Sort by absolute value for ranking.
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].absVal < ranked[j].absVal
	})

	// Assign average ranks (handling ties) and restore sign.
	i := 0
	for i < len(ranked) {
		j := i + 1
		for j < len(ranked) && ranked[j].absVal == ranked[i].absVal {
			j++
		}
		avgRank := float64(i+1+j) / 2.0
		// Assign signed rank: positive if original diff was positive, negative otherwise.
		for k := i; k < j; k++ {
			if diffs[ranked[k].idx] > 0 {
				ranked[k].signedRank = avgRank
			} else {
				ranked[k].signedRank = -avgRank
			}
		}
		i = j
	}

	// Sum positive and negative ranks.
	var wPos, wNeg float64
	for _, rd := range ranked {
		if rd.signedRank > 0 {
			wPos += rd.signedRank
		} else {
			wNeg += -rd.signedRank
		}
	}

	// Test statistic: the smaller of W+ and W-.
	wStat := math.Min(wPos, wNeg)
	nNonZero := len(ranked)

	var pValue float64
	var significant bool

	if nNonZero < 10 {
		// Exact test: compute two-sided p-value by enumeration.
		pValue = wilcoxonExactPValue(wStat, nNonZero)
		significant = pValue < 0.05
	} else {
		// Normal approximation with continuity correction.
		// Under H0: E[W] = n(n+1)/4, Var(W) = n(n+1)(2n+1)/24
		meanW := float64(nNonZero) * float64(nNonZero+1) / 4.0
		varSD := math.Sqrt(float64(nNonZero) * float64(nNonZero+1) * float64(2*nNonZero+1) / 24.0)
		// Continuity correction: subtract 0.5 in the direction of the true mean.
		z := wStat - meanW
		if wStat > meanW {
			z -= 0.5
		} else {
			z += 0.5
		}
		z /= varSD
		// Two-sided p-value.
		pValue = 2 * normalCDF(-math.Abs(z))
		significant = pValue < 0.05
	}

	return &WilcoxonResult{
		N:                nNonZero,
		Statistic:        wStat,
		PValue:           floatPtr(pValue),
		SignificantAt005: boolPtr(significant),
		Interpretation:   wilcoxonInterp(significant, pValue, hypothesizedMedian),
	}, nil
}

// wilcoxonInterp generates a human-readable interpretation string.
func wilcoxonInterp(sig bool, pValue, median float64) string {
	sigStr := "No significant difference"
	if sig {
		sigStr = "The median SC differs significantly from"
	}
	return sprintf("%s %.2f (two-sided p=%.4f).", sigStr, median, pValue)
}

// rankWithTies assigns ranks to the absolute values of non-zero differences.
// Values must be in the same order as the original data (non-zero only).
// Returns a parallel slice of ranks (1-indexed, average for ties).
func rankWithTies(values []float64) []float64 {
	if len(values) == 0 {
		return nil
	}

	type pair struct {
		origIdx int
		value   float64
	}
	pairs := make([]pair, len(values))
	for i, v := range values {
		pairs[i] = pair{origIdx: i, value: v}
	}

	// Sort by value ascending.
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].value < pairs[j].value
	})

	ranks := make([]float64, len(values))
	i := 0
	for i < len(pairs) {
		j := i + 1
		for j < len(pairs) && pairs[j].value == pairs[i].value {
			j++
		}
		// Average rank for ties: (i+1 + j) / 2.
		avgRank := float64(i+1+j) / 2.0
		for k := i; k < j; k++ {
			ranks[pairs[k].origIdx] = avgRank
		}
		i = j
	}

	return ranks
}

// wilcoxonExactPValue computes the two-sided exact p-value for the Wilcoxon
// signed-rank test given the observed W statistic and sample size n.
// It enumerates all 2^n possible sign assignments and computes the probability
// of observing W or smaller under the null (two-sided).
func wilcoxonExactPValue(observedW float64, n int) float64 {
	totalSum := float64(n * (n + 1) / 2)
	threshold := observedW

	count := 1 << n // 2^n
	leCount := 0

	for mask := 0; mask < count; mask++ {
		var wPlus float64
		for i := 0; i < n; i++ {
			if mask&(1<<i) != 0 {
				wPlus += float64(i + 1) // rank is 1-indexed
			}
		}
		w := math.Min(wPlus, totalSum-wPlus)
		if w <= threshold {
			leCount++
		}
	}

	return float64(leCount) / float64(count)
}

// -------------------------------------------------------------------------- //
// SPEARMAN RANK CORRELATION
// -------------------------------------------------------------------------- //

// SpearmanCorrelation computes the Spearman rank correlation coefficient (rho)
// between two variables, measuring the strength and direction of their monotonic
// relationship. Unlike Pearson correlation, Spearman uses ranks rather than
// raw values, making it robust to outliers and non-linear monotonic relationships.
//
// It returns a SpearmanResult populated with rho, p-value, significance flag,
// and interpretation.
//
// Returns errInsufficientData when len(X) ≠ len(Y) or when len(X) < 3,
// or when all values in either variable are identical (zero variance).
func spearmanCorrelation(x, y []float64) (*SpearmanResult, error) {
	n := len(x)
	if n != len(y) || n < 3 {
		return nil, errInsufficientData
	}

	// Filter paired observations where both are non-NaN.
	var xF, yF []float64
	for i := range x {
		if !math.IsNaN(x[i]) && !math.IsNaN(y[i]) {
			xF = append(xF, x[i])
			yF = append(yF, y[i])
		}
	}

	n = len(xF)
	if n < 3 {
		return nil, errInsufficientData
	}
	if allFloat64Equal(xF) || allFloat64Equal(yF) {
		return nil, errInsufficientData
	}

	// Rank X and Y separately.
	xRanks := rankFloat64(xF)
	yRanks := rankFloat64(yF)

	// Compute squared differences of ranks.
	var sumSqDiff float64
	for i := 0; i < n; i++ {
		d := xRanks[i] - yRanks[i]
		sumSqDiff += d * d
	}

	// Spearman rho = 1 - (6 * Σd²) / (n(n² - 1))
	denom := float64(n*n*n - n)
	if denom == 0 {
		return nil, errInsufficientData
	}
	rho := 1.0 - (6.0*sumSqDiff)/denom

	// Clamp rho to [-1, 1] to handle floating-point rounding.
	rho = math.Max(-1, math.Min(1, rho))

	// Compute p-value using t-distribution approximation.
	// t = rho * sqrt((n-2) / (1 - rho²)), df = n - 2
	var pValue float64
	var significant bool
	if n >= 5 && math.Abs(rho) < 1 {
		df := float64(n - 2)
		tStat := rho * math.Sqrt(df/(1.0-rho*rho))
		pValue = 2 * tDistCDF(-math.Abs(tStat), df)
		significant = pValue < 0.05
	} else {
		// For n < 5, we don't have enough data for a reliable p-value.
		pValue = 1.0
		significant = false
	}
	if math.IsNaN(pValue) || math.IsInf(pValue, 0) {
		return nil, errInsufficientData
	}

	return &SpearmanResult{
		N:                n,
		Rho:              rho,
		PValue:           floatPtr(pValue),
		SignificantAt005: boolPtr(significant),
		Interpretation:   spearmanInterp(rho, n, pValue, significant),
	}, nil
}

func allFloat64Equal(values []float64) bool {
	if len(values) < 2 {
		return true
	}
	first := values[0]
	for _, v := range values[1:] {
		if v != first {
			return false
		}
	}
	return true
}

// spearmanInterp generates a human-readable interpretation string.
func spearmanInterp(rho float64, n int, pValue float64, sig bool) string {
	// Determine strength label.
	var strength string
	absRho := math.Abs(rho)
	switch {
	case absRho >= 0.7:
		strength = "Strong"
	case absRho >= 0.4:
		strength = "Moderate"
	case absRho >= 0.2:
		strength = "Weak"
	default:
		strength = "Negligible"
	}

	direction := "positive"
	if rho < 0 {
		direction = "negative"
	}

	interp := sprintf("%s %s monotonic relationship between SC and catalog-step coverage (rho=%.3f).", strength, direction, rho)
	if n < 5 {
		interp += " (p-value not computed for n < 5; insufficient data.)"
	}
	return interp
}

// rankFloat64 ranks a slice of float64 values. Returns a new slice of ranks
// (1-indexed, average for ties). Example: [1.0, 2.0, 2.0, 3.0] → [1, 2.5, 2.5, 4]
func rankFloat64(values []float64) []float64 {
	if len(values) == 0 {
		return nil
	}

	type idxVal struct {
		idx   int
		value float64
	}
	pairs := make([]idxVal, len(values))
	for i, v := range values {
		pairs[i] = idxVal{idx: i, value: v}
	}

	// Sort by value ascending.
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].value < pairs[j].value
	})

	ranks := make([]float64, len(values))
	i := 0
	for i < len(pairs) {
		j := i + 1
		for j < len(pairs) && pairs[j].value == pairs[i].value {
			j++
		}
		// Average rank for ties.
		avgRank := float64(i+1+j) / 2.0
		for k := i; k < j; k++ {
			ranks[pairs[k].idx] = avgRank
		}
		i = j
	}

	return ranks
}

// -------------------------------------------------------------------------- //
// DISTRIBUTION FUNCTIONS (no external dependencies — pure Go math)
// -------------------------------------------------------------------------- //

// normalCDF returns the cumulative distribution function of the standard
// normal distribution at x. Uses the error function approximation.
func normalCDF(x float64) float64 {
	return 0.5 * (1 + math.Erf(x/math.Sqrt2))
}

// tDistCDF returns the cumulative distribution function of the Student's
// t-distribution with df degrees of freedom at x.
//
// Uses the regularized incomplete beta function relationship:
// F(t; v) = 0.5 + sign(t) * [1 - I_{v/(v+t²)}(v/2, 0.5)] / 2
//
// Falls back to normalCDF when df > 30.
func tDistCDF(x float64, df float64) float64 {
	if df <= 0 {
		return 0.5
	}

	// For large df, t converges to normal.
	if df > 30 {
		return normalCDF(x)
	}

	// t-distribution is symmetric around 0: CDF(0) = 0.5 for all df.
	if x == 0 {
		return 0.5
	}

	// Use the incomplete beta function relationship.
	neg := x < 0
	x = math.Abs(x)

	z := df / (df + x*x)
	betaInc := incompleteBetaReg(z, df/2.0, 0.5)

	result := 0.5 * (1.0 - betaInc)
	if neg {
		result = 0.5 * (1.0 - result)
	}

	// Clamp to [0, 1].
	if result < 0 {
		result = 0
	}
	if result > 1 {
		result = 1
	}

	return result
}

// incompleteBetaReg computes the regularized incomplete beta function I_z(a, b).
// Uses the continued fraction representation with Lentz's method for evaluation.
func incompleteBetaReg(z, a, b float64) float64 {
	if z < 0 || z > 1 {
		return 0
	}
	if z == 0 {
		return 0
	}
	if z == 1 {
		return 1
	}

	// For z <= (a+1)/(a+b+2), compute directly.
	// Otherwise use the reflection: I_z(a,b) = 1 - I_{1-z}(b,a)
	useReflect := z > (a+1.0)/(a+b+2.0)
	if useReflect {
		z = 1 - z
		a, b = b, a
	}

	// Compute the continued fraction using Lentz's method.
	// The regularized incomplete beta CF:
	// I_z(a,b) = z^a * (1-z)^b * CF / (a * B(a,b))
	//
	// CF = 1 + c1/(1 + c2/(1 + c3/(1 + ...)))
	// where:
	//   c_{2m}   = (a+m-1)(a+m)(1-z) / ((a+2m-2)(a+2m-1))
	//   c_{2m+1} = -m(b-m)z / ((a+2m-1)(a+2m))

	cf := betaContinuedFraction(z, a, b)

	// Compute log(I_z) = a*log(z) + b*log(1-z) + log(CF) - log(a) - log(B(a,b))
	// log(B(a,b)) = logGamma(a) + logGamma(b) - logGamma(a+b)
	logBeta := lgamma(a) + lgamma(b) - lgamma(a+b)
	logIz := a*math.Log(z) + b*math.Log(1-z) + math.Log(cf) - math.Log(a) - logBeta
	result := math.Exp(logIz)

	if useReflect {
		result = 1 - result
	}

	// Clamp.
	if result < 0 {
		result = 0
	}
	if result > 1 {
		result = 1
	}

	return result
}

// betaContinuedFraction evaluates the continued fraction for the regularized
// incomplete beta function using Lentz's method.
func betaContinuedFraction(z, a, b float64) float64 {
	const tiny = 1e-30
	const maxIter = 200

	// Initialize Lentz's method.
	f := tiny
	c := tiny
	d := 0.0

	for n := 1; n <= maxIter; n++ {
		var an float64

		if n%2 == 1 {
			// Odd term (a_{2m-1} = (a+m-1)(a+m)(1-z) / ((a+2m-2)(a+2m-1)))
			m := float64((n - 1) / 2)
			an = (a + m - 1) * (a + m) * (1 - z) / ((a + 2*m - 2) * (a + 2*m - 1))
		} else {
			// Even term (a_{2m} = -m(b-m)z / ((a+2m-1)(a+2m)))
			m := float64(n / 2)
			an = -m * (b - m) * z / ((a + 2*m - 1) * (a + 2*m))
		}

		bn := 1.0
		d = bn + an*d
		if math.Abs(d) < tiny {
			d = tiny
		}
		c = bn + an/c
		if math.Abs(c) < tiny {
			c = tiny
		}
		d = 1.0 / d
		delta := c * d
		f *= delta

		if math.Abs(delta-1) < 1e-14 {
			break
		}
	}

	return f
}

// lgamma computes the natural logarithm of the gamma function using
// Lanczos approximation. Valid for x > 0.
func lgamma(x float64) float64 {
	if x <= 0 {
		// Use reflection: Gamma(x) = pi / (sin(pi*x) * Gamma(1-x))
		// log|Gamma(x)| = log(pi) - |sin(pi*x)| - lgamma(1-x)
		return math.Log(math.Pi) - math.Log(math.Abs(math.Sin(math.Pi*x))) - lgamma(1-x)
	}

	// Lanczos approximation with g=5.
	g := 5.0
	c := []float64{
		1.000000000190015,
		76.18009172947146,
		-86.50532032941677,
		24.01409824083091,
		-1.231739572450155,
		0.1208650973866179e-2,
		-0.5395239384953e-5,
	}

	xm1 := x - 1
	tmp := xm1 + g + 0.5
	ser := c[0]
	for i := 1; i < len(c); i++ {
		ser += c[i] / (xm1 + float64(i))
	}

	return 0.5*math.Log(2*math.Pi) + (xm1+0.5)*math.Log(tmp) - tmp + math.Log(ser)
}

// -------------------------------------------------------------------------- //
// POINTER HELPERS
// -------------------------------------------------------------------------- //

func floatPtr(v float64) *float64 {
	return &v
}

func boolPtr(v bool) *bool {
	return &v
}

// -------------------------------------------------------------------------- //
// STRING FORMATTING (minimal, no external deps beyond math)
// -------------------------------------------------------------------------- //

// sprintf provides a minimal fmt.Sprintf-like substitution for static patterns.
// It handles %s, %d, %f with precision, and %.Nf formats.
func sprintf(format string, args ...interface{}) string {
	var out strings.Builder
	argIdx := 0
	i := 0
	for i < len(format) {
		if format[i] == '%' && i+1 < len(format) {
			c := format[i+1]
			if c == 's' && argIdx < len(args) {
				switch v := args[argIdx].(type) {
				case string:
					out.WriteString(v)
				}
				argIdx++
				i += 2
				continue
			}
			if c == 'd' && argIdx < len(args) {
				switch v := args[argIdx].(type) {
				case int:
					out.WriteString(intToStr(v))
				case float64:
					out.WriteString(intToStr(int(v)))
				}
				argIdx++
				i += 2
				continue
			}
			if c == 'f' || c == '.' {
				// Handle %f or %.Nf
				prec := 6
				start := i + 1
				if c == '.' {
					// Find precision.
					j := i + 2
					pStart := j
					for j < len(format) && format[j] >= '0' && format[j] <= '9' {
						j++
					}
					if j > pStart {
						prec = atoi(format[pStart:j])
						start = j
					}
					c = format[start]
					if c != 'f' {
						// Not a float format.
						out.WriteByte(format[i])
						i++
						continue
					}
				}
				if argIdx < len(args) {
					switch v := args[argIdx].(type) {
					case float64:
						out.WriteString(floatToStr(v, prec))
					case int:
						out.WriteString(floatToStr(float64(v), prec))
					}
					argIdx++
					i = start + 1
					continue
				}
			}
		}
		out.WriteByte(format[i])
		i++
	}
	return out.String()
}

func floatToStr(v float64, prec int) string {
	neg := v < 0
	if neg {
		v = -v
	}

	scale := math.Pow10(prec)
	intPart := math.Floor(v)
	fracPart := v - intPart
	frac := math.Floor(fracPart*scale + 0.5)
	if frac >= scale {
		intPart++
		frac = 0
	}

	var res strings.Builder
	if neg {
		res.WriteByte('-')
	}

	if intPart == 0 {
		res.WriteByte('0')
	} else {
		var buf [32]byte
		i := len(buf)
		tmp := intPart
		for tmp > 0 {
			i--
			buf[i] = byte('0' + int(math.Mod(tmp, 10)))
			tmp = math.Floor(tmp / 10)
		}
		res.Write(buf[i:])
	}

	res.WriteByte('.')
	res.WriteString(intToStrPad(int(frac), prec))

	return res.String()
}

func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func intToStrPad(n, width int) string {
	s := intToStr(n)
	if len(s) >= width {
		return s
	}
	res := make([]byte, width)
	pad := width - len(s)
	for i := 0; i < pad; i++ {
		res[i] = '0'
	}
	copy(res[pad:], s)
	return string(res)
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		n = n*10 + int(c-'0')
	}
	return n
}
