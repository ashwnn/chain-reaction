#!/usr/bin/env bash
# run-reproducibility.sh — execute N repeated validate runs and organize artifacts
# per the scenario-runs runbook contract.
#
# Usage:
#   ./scripts/run-reproducibility.sh [OPTIONS]
#
# Options:
#   -n, --runs N          Number of runs to execute (default: 5)
#   -o, --output DIR      Base output directory (default: artifacts/scenario-runs)
#   -b, --binary PATH     Path to chain-reaction binary (default: bin/chain-reaction)
#   --skip-healthcheck    Skip the pre-run healthcheck
#   --organize-only       Re-organize existing runs without executing new ones
#   -h, --help            Show this help
#
# Data model — all-families-per-run:
#   Each `chain-reaction validate` invocation exercises all five KG-001..KG-005
#   families simultaneously. The post-hoc scenario matcher assigns per-family
#   results within a single `validation-metrics.json`. The per-family KG-xxx/run-N/
#   directories are therefore views over the same flat run set — every symlink
#   under KG-001/run-N/ points to the identical directory as KG-002/run-N/.
#   This is intentional and matches the runbook contract; do not run once per family.
#
# Prerequisites:
#   - Evaluation environment running (run make env-setup && make env-healthcheck)
#   - chain-reaction binary built with LLM support
#   - provider API-key environment variable set
#
# This script FAILS LOUDLY on any error (set -euo pipefail).

set -euo pipefail

SCRIPTS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source-path=SCRIPTDIR
source "${SCRIPTS_DIR}/goat-env-config.sh"

# ---------------------------------------------------------------------------
# Defaults
# ---------------------------------------------------------------------------

NUM_RUNS=5
OUTPUT_BASE="${PROJECT_ROOT}/artifacts/scenario-runs"
BINARY="${PROJECT_ROOT}/bin/chain-reaction"
SKIP_HEALTHCHECK=false
ORGANIZE_ONLY=false

# Planner configuration — passed through to each validate invocation.
# These are intentionally not configurable per-run to maintain controlled conditions.
LLM_API_KEY="${LLM_API_KEY:-}"
LLM_MODEL="${LLM_MODEL:-}"
LLM_PROVIDER="${LLM_PROVIDER:-openai}"
LLM_BASE_URL="${LLM_BASE_URL:-}"
LLM_TEMPERATURE="${LLM_TEMPERATURE:--1.0}"
LLM_MAX_TOKENS="${LLM_MAX_TOKENS:-0}"
MAX_STEPS="${MAX_STEPS:-20}"
TIME_BUDGET="${TIME_BUDGET:-5m}"

# ---------------------------------------------------------------------------
# Argument parsing
# ---------------------------------------------------------------------------

usage() {
    sed -n '3,/^$/p' "${BASH_SOURCE[0]}" | sed 's/^# \?//'
    exit 0
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        -n|--runs)
            NUM_RUNS="$2"
            shift 2
            ;;
        -o|--output)
            OUTPUT_BASE="$2"
            shift 2
            ;;
        -b|--binary)
            BINARY="$2"
            shift 2
            ;;
        --skip-healthcheck)
            SKIP_HEALTHCHECK=true
            shift
            ;;
        --organize-only)
            ORGANIZE_ONLY=true
            shift
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
# Validation
# ---------------------------------------------------------------------------

if [[ "${ORGANIZE_ONLY}" == "false" ]]; then
    if [[ ! -x "${BINARY}" ]]; then
        echo "ERROR: Binary not found or not executable: ${BINARY}" >&2
        echo "       Build it with: make build" >&2
        exit 1
    fi

    if [[ -z "${LLM_API_KEY:-}" && -z "${OPENAI_API_KEY:-}" && -z "${ANTHROPIC_API_KEY:-}" && -z "${GROQ_API_KEY:-}" ]]; then
        echo "ERROR: no supported provider API key is set in the environment." >&2
        exit 1
    fi

    if [[ -z "${LLM_MODEL}" ]]; then
        echo "ERROR: LLM_MODEL is not set. Set it via environment variable." >&2
        exit 1
    fi
fi

# ---------------------------------------------------------------------------
# Healthcheck
# ---------------------------------------------------------------------------

if [[ "${ORGANIZE_ONLY}" == "false" && "${SKIP_HEALTHCHECK}" == "false" ]]; then
    echo "=== Pre-run healthcheck ==="
    if ! bash "${SCRIPTS_DIR}/healthcheck-goat-env.sh"; then
        echo "ERROR: Healthcheck failed. Fix the environment before running." >&2
        echo "       Run: make env-healthcheck" >&2
        exit 1
    fi
    echo ""
fi

# ---------------------------------------------------------------------------
# Detect controlled-condition metadata
# ---------------------------------------------------------------------------

K8S_VERSION="$(kubectl version --output=json 2>/dev/null | grep -o '"gitVersion": "[^"]*"' | head -1 | cut -d'"' -f4 || echo "unknown")"

# ---------------------------------------------------------------------------
# Execute runs
# ---------------------------------------------------------------------------

mkdir -p "${OUTPUT_BASE}"

# Collect run IDs for the organize step.
RUN_IDS=()

if [[ "${ORGANIZE_ONLY}" == "false" ]]; then
    echo "=== Reproducibility Runs: ${NUM_RUNS} runs ==="
    echo "Output: ${OUTPUT_BASE}"
    echo "Binary: ${BINARY}"
    echo "Model : ${LLM_MODEL}"
    echo ""

    for i in $(seq 1 "${NUM_RUNS}"); do
        RUN_TIMESTAMP="$(date -u +%Y-%m-%dT%H%M%SZ)"
        RUN_ID="${RUN_TIMESTAMP}-react-$(printf '%03d' "${i}")"
        RUN_DIR="${OUTPUT_BASE}/${RUN_ID}"

        echo "--- Run ${i}/${NUM_RUNS}: ${RUN_ID} ---"

        mkdir -p "${RUN_DIR}"

        # Build the validate command arguments.
        VALIDATE_ARGS=(
            validate
            --output "${RUN_DIR}"
            --max-steps "${MAX_STEPS}"
            --time-budget "${TIME_BUDGET}"
            --llm-provider "${LLM_PROVIDER}"
            --llm-model "${LLM_MODEL}"
        )

        if [[ -n "${LLM_BASE_URL}" ]]; then
            VALIDATE_ARGS+=(--llm-base-url "${LLM_BASE_URL}")
        fi

        if [[ "${LLM_TEMPERATURE}" != "-1.0" ]]; then
            VALIDATE_ARGS+=(--llm-temperature "${LLM_TEMPERATURE}")
        fi

        if [[ "${LLM_MAX_TOKENS}" -gt 0 ]]; then
            VALIDATE_ARGS+=(--llm-max-tokens "${LLM_MAX_TOKENS}")
        fi

        # Execute the validate run.
        RUN_START_EPOCH="$(date -u +%s)"
        if ! "${BINARY}" "${VALIDATE_ARGS[@]}"; then
            echo "WARNING: Run ${RUN_ID} exited with non-zero status." >&2
            echo "         Artifacts may be incomplete. Continuing to next run." >&2
        fi
        RUN_END_EPOCH="$(date -u +%s)"

        # Write run-metadata.json for this run.
        # This allows to verify controlled conditions and group runs.
        cat > "${RUN_DIR}/run-metadata.json" <<META_EOF
{
  "run_id": "${RUN_ID}",
  "run_index": ${i},
  "planner_type": "react_llm",
  "llm_model": "${LLM_MODEL}",
  "llm_provider": "${LLM_PROVIDER}",
  "max_steps": ${MAX_STEPS},
  "time_budget": "${TIME_BUDGET}",
  "started_at": "$(date -u -d @"${RUN_START_EPOCH}" +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -r "${RUN_START_EPOCH}" +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || echo unknown)",
  "cluster_version": "${K8S_VERSION}",
  "goat_version": "${GOAT_REF}",
  "notes": "Reproducibility run ${i}/${NUM_RUNS}"
}
META_EOF

        RUN_IDS+=("${RUN_ID}")
        echo ""
    done

    echo "=== Runs complete: ${NUM_RUNS}/${NUM_RUNS} ==="
    echo ""
else
    # Organize-only: discover existing run directories.
    while IFS= read -r dir; do
        basename_dir="$(basename "${dir}")"
        # Match the run ID pattern: timestamp-react-NNN
        if [[ "${basename_dir}" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{6}Z-react-[0-9]{3}$ ]]; then
            RUN_IDS+=("${basename_dir}")
        fi
    done < <(find "${OUTPUT_BASE}" -maxdepth 1 -mindepth 1 -type d | sort)
    echo "Discovered ${#RUN_IDS[@]} existing runs in ${OUTPUT_BASE}"
fi

# ---------------------------------------------------------------------------
# Organize: create per-family KG-xxx/run-N/ view
# ---------------------------------------------------------------------------

echo "=== Organizing per-family views ==="
echo "  Note: one run covers all families (KG-001..KG-005) simultaneously."
echo "        Per-family directories are symlink views over the same flat run set."

FAMILIES=("KG-001" "KG-002" "KG-003" "KG-004" "KG-005")

for family in "${FAMILIES[@]}"; do
    FAMILY_DIR="${OUTPUT_BASE}/${family}"
    mkdir -p "${FAMILY_DIR}"

    run_n=1
    for rid in "${RUN_IDS[@]}"; do
        DST="${FAMILY_DIR}/run-${run_n}"

        if [[ -e "${DST}" ]]; then
            rm -f "${DST}"
        fi

        # Relative symlink: KG-xxx/run-N/ → ../<run-id>/
        # All five family views point to the same flat run directory.
        ln -s "../${rid}" "${DST}"
        run_n=$((run_n + 1))
    done

    echo "  ${family}/ → ${#RUN_IDS[@]} runs (symlinks to flat run directories)"
done

echo ""
echo "=== Done ==="
echo "Flat runs : ${OUTPUT_BASE}/<run-id>/"
echo "Per-family: ${OUTPUT_BASE}/KG-xxx/run-N/ (symlinks to flat runs)"
echo ""
echo "Next steps:"
echo "  1. Review artifacts: ls ${OUTPUT_BASE}/"
echo "  2. Compute stability: ./scripts/compute-stability.sh -o ${OUTPUT_BASE}"
