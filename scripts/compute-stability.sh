#!/usr/bin/env bash
# compute-stability.sh — compute run-to-run stability (coefficient of variation)
# for scenario coverage (SC) and step validation rate (SVR) across repeated runs.
#
# Usage:
#   ./scripts/compute-stability.sh [OPTIONS]
#
# Options:
#   -o, --output DIR    Base output directory containing scenario-runs (default: artifacts/scenario-runs)
#   -f, --family ID     Compute for a specific family only (e.g., KG-001). Default: all families.
#   -h, --help          Show this help
#
# Reads validation-metrics.json from each KG-xxx/run-N/ directory (or flat run
# directories if no per-family view exists) and outputs a stability summary.
#
# scope: this script computes the descriptive statistics (mean, SD, CV)
# that demonstrate run-to-run reproducibility. Wilcoxon signed-rank and Spearman
# correlation are implemented in the Go analysis pipeline (`chain-reaction analyze`).

set -euo pipefail

SCRIPTS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPTS_DIR}/.." && pwd)"
OUTPUT_BASE="${PROJECT_ROOT}/artifacts/scenario-runs"
TARGET_FAMILY=""

# ---------------------------------------------------------------------------
# Argument parsing
# ---------------------------------------------------------------------------

usage() {
    sed -n '3,$p' "${BASH_SOURCE[0]}" 2>/dev/null | sed 's/^# \?//' || true
    exit 0
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        -o|--output)
            OUTPUT_BASE="$2"
            shift 2
            ;;
        -f|--family)
            TARGET_FAMILY="$2"
            shift 2
            ;;
        -h|--help)
            usage
            ;;
        *)
            echo "ERROR: unknown option: $1" >&2
            exit 1
            ;;
    esac
done

# ---------------------------------------------------------------------------
# jq availability check
# ---------------------------------------------------------------------------

if ! command -v jq &>/dev/null; then
    echo "ERROR: jq is required for JSON parsing. Install it first." >&2
    exit 1
fi

# ---------------------------------------------------------------------------
# Helper: extract numeric value from validation-metrics.json
# ---------------------------------------------------------------------------

# Extract scenario_rate (SC) from a metrics file. Returns "null" if absent.
extract_sc() {
    local metrics_file="$1"
    jq -r '.scenario_coverage.scenario_rate // "null"' "${metrics_file}" 2>/dev/null || echo "null"
}

# Extract step_validation_rate (SVR) from a metrics file. Returns "null" if absent.
extract_svr() {
    local metrics_file="$1"
    jq -r '.scenario_coverage.step_validation_rate // "null"' "${metrics_file}" 2>/dev/null || echo "null"
}

# Extract per-family chain_validated boolean.
extract_family_validated() {
    local metrics_file="$1"
    local family_id="$2"
    jq -r --arg fid "${family_id}" '
        .scenario_coverage.families[] | select(.family_id == $fid) | .chain_validated // false
    ' "${metrics_file}" 2>/dev/null || echo "false"
}

# ---------------------------------------------------------------------------
# Compute mean, standard deviation, and coefficient of variation
# from a space-separated list of values.
# ---------------------------------------------------------------------------

compute_stats() {
    local values=("$@")
    local count="${#values[@]}"

    if [[ "${count}" -eq 0 ]]; then
        echo "0 null null null"
        return
    fi

    # Compute sum.
    sum=0
    valid=0
    for v in "${values[@]}"; do
        if [[ "${v}" != "null" ]]; then
            # Use awk for floating-point arithmetic.
            sum="$(echo "${sum} ${v}" | awk '{print $1 + $2}')"
            valid=$((valid + 1))
        fi
    done

    if [[ "${valid}" -eq 0 ]]; then
        echo "0 null null null"
        return
    fi

    mean="$(echo "${sum} ${valid}" | awk '{printf "%.6f", $1 / $2}')"

    # Compute variance.
    sum_sq_diff=0
    for v in "${values[@]}"; do
        if [[ "${v}" != "null" ]]; then
            sum_sq_diff="$(echo "${sum_sq_diff} ${v} ${mean}" | awk '{printf "%.10f", $1 + ($2 - $3)^2}')"
        fi
    done

    # Sample standard deviation (N-1 denominator for stability across small N).
    if [[ "${valid}" -gt 1 ]]; then
        sd="$(echo "${sum_sq_diff} ${valid}" | awk '{printf "%.6f", sqrt($1 / ($2 - 1))}')"
    else
        sd="0.000000"
    fi

    # Coefficient of variation (SD / mean * 100). Zero if mean is zero.
    cv="$(echo "${sd} ${mean}" | awk '{if ($2 == 0) print "0.00"; else printf "%.2f", ($1 / $2) * 100}')"

    echo "${valid} ${mean} ${sd} ${cv}"
}

# ---------------------------------------------------------------------------
# Collect runs
# ---------------------------------------------------------------------------

FAMILIES=("KG-001" "KG-002" "KG-003" "KG-004" "KG-005")

if [[ -n "${TARGET_FAMILY}" ]]; then
    FAMILIES=("${TARGET_FAMILY}")
fi

echo "=== Stability Analysis ==="
echo "Output: ${OUTPUT_BASE}"
echo ""

# ---------------------------------------------------------------------------
# Global stability (across all runs)
# ---------------------------------------------------------------------------

echo "[Global]"

# Collect metrics from flat run directories.
SC_VALUES=()
SVR_VALUES=()

while IFS= read -r dir; do
    rid="$(basename "${dir}")"
    metrics="${dir}/validation-metrics.json"
    if [[ -f "${metrics}" ]]; then
        sc="$(extract_sc "${metrics}")"
        svr="$(extract_svr "${metrics}")"
        SC_VALUES+=("${sc}")
        SVR_VALUES+=("${svr}")
    else
        echo "  WARNING: No validation-metrics.json in ${rid}" >&2
    fi
done < <(find "${OUTPUT_BASE}" -maxdepth 1 -mindepth 1 -type d | sort)

if [[ "${#SC_VALUES[@]}" -eq 0 ]]; then
    echo "  No runs found. Execute runs first: ./scripts/run-reproducibility.sh" >&2
    exit 1
fi

read -r n_sc mean_sc sd_sc cv_sc <<< "$(compute_stats "${SC_VALUES[@]}")"
read -r n_svr mean_svr sd_svr cv_svr <<< "$(compute_stats "${SVR_VALUES[@]}")"

echo "  Runs analyzed: ${#SC_VALUES[@]}"
echo "  SC  (scenario_rate)      : n=${n_sc}, mean=${mean_sc}, SD=${sd_sc}, CV=${cv_sc}%"
echo "  SVR (step_validation_rate): n=${n_svr}, mean=${mean_svr}, SD=${sd_svr}, CV=${cv_svr}%"
echo ""

# warn when CV exceeds 25% (high variance flag threshold).
high_cv_sc="false"
high_cv_svr="false"

# Use awk comparison on cv values to detect high variance.
if echo "${cv_sc}" | awk '{exit !($1 > 25)}'; then
    high_cv_sc="true"
fi
if echo "${cv_svr}" | awk '{exit !($1 > 25)}'; then
    high_cv_svr="true"
fi

if [[ "${high_cv_sc}" == "true" ]] || [[ "${high_cv_svr}" == "true" ]]; then
    echo "  ⚠ WARNING: Unusually high variance detected (CV > 25%)."
    [[ "${high_cv_sc}" == "true" ]]  && echo "    SC  CV=${cv_sc}% exceeds threshold — investigate planner non-determinism."
    [[ "${high_cv_svr}" == "true" ]] && echo "    SVR CV=${cv_svr}% exceeds threshold — investigate planner non-determinism."
    echo ""
fi

# ---------------------------------------------------------------------------
# Per-family chain reliability
# ---------------------------------------------------------------------------

# produces a raw pass-count across runs. will compute
# per-family CV of SVR and formal statistical tests. This section reports
# the binary chain-validation outcome (chain validated / not validated)
# for each run so can consume it later.

echo "[Per-Family Chain Reliability]"

for family in "${FAMILIES[@]}"; do
    FAMILY_DIR="${OUTPUT_BASE}/${family}"

    if [[ ! -d "${FAMILY_DIR}" ]]; then
        echo "  ${family}: no per-family directory found, skipping"
        continue
    fi

    validated_count=0
    attempted_count=0

    for run_dir in "${FAMILY_DIR}"/run-*; do
        [[ -e "${run_dir}" ]] || continue

        # Resolve symlink to find the actual flat run directory.
        # If it is not a symlink, run_dir itself is the metrics directory.
        if [[ -L "${run_dir}" ]]; then
            # realpath resolves the symlink and normalizes the path.
            resolved_dir="$(realpath "${run_dir}" 2>/dev/null)" || continue
            metrics="${resolved_dir}/validation-metrics.json"
        else
            metrics="${run_dir}/validation-metrics.json"
        fi

        if [[ -f "${metrics}" ]]; then
            attempted_count=$((attempted_count + 1))
            chain_ok="$(extract_family_validated "${metrics}" "${family}")"
            if [[ "${chain_ok}" == "true" ]]; then
                validated_count=$((validated_count + 1))
            fi
        fi
    done

    reliability="N/A"
    if [[ "${attempted_count}" -gt 0 ]]; then
        reliability="$(echo "${validated_count} ${attempted_count}" | awk '{printf "%d/%d (%.0f%%)", $1, $2, ($1/$2)*100}')"
    fi

    echo "  ${family}: chain validated in ${reliability} runs"
done

echo ""
echo "  Note: per-family CV of SVR and statistical tests are scope."
echo ""

# ---------------------------------------------------------------------------
# Summary output
# ---------------------------------------------------------------------------

echo "=== Interpretation ==="
echo "  CV < 10%  : High reproducibility (stable across runs)"
echo "  CV 10-25% : Moderate variability (acceptable for LLM-driven planner)"
echo "  CV > 25%  : High variability (investigate planner non-determinism)"
echo ""
echo "  For the paper: report SC mean ± SD with CV across ${n_sc} runs."
echo "  Per-family CV of SVR and formal statistical tests are scope."
