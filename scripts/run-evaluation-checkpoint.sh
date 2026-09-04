#!/usr/bin/env bash
# run-evaluation-checkpoint.sh — execute the live evaluation checkpoint batch
# under pinned settings, validate the artifact contract, and summarize the gate.
#
# Usage:
#   ./scripts/run-evaluation-checkpoint.sh [OPTIONS]
#
# Options:
#   -o, --output DIR         Base output directory (default: artifacts/scenario-runs)
#   --checkpoint-runs N      Number of runs in the decision gate (default: 3)
#   --target-runs N          Total runs for the full batch (default: 5)
#   --continue-to-five       If the checkpoint is stable, continue to target-runs and run analyze
#   -b, --binary PATH        Path to chain-reaction binary for analyze (default: bin/chain-reaction)
#   --run-set-id ID          Explicit run-set identifier for reports/analyze
#   --label LABEL            Human-readable label for analyze output
#   --llm-provider NAME      LLM provider (default: openai)
#   --llm-model MODEL        LLM model (default: gpt-5.4-mini)
#   --max-steps N            Max validation steps (default: 20)
#   --time-budget DUR        Validation time budget (default: 5m)
#   --job-prefix PREFIX      Kubernetes Job name prefix (default: cr-checkpoint)
#   --skip-healthcheck       Skip the initial environment healthcheck
#   -h, --help               Show this help

set -euo pipefail

SCRIPTS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source-path=SCRIPTDIR
source "${SCRIPTS_DIR}/goat-env-config.sh"
# shellcheck source-path=SCRIPTDIR
source "${SCRIPTS_DIR}/scenario-run-lib.sh"

usage() {
    sed -n '3,/^$/p' "${BASH_SOURCE[0]}" | sed 's/^# \?//'
    exit 0
}

OUTPUT_BASE="${PROJECT_ROOT}/artifacts/scenario-runs"
CHECKPOINT_RUNS=3
TARGET_RUNS=5
CONTINUE_TO_FIVE=false
BINARY="${PROJECT_ROOT}/bin/chain-reaction"
LLM_PROVIDER="${LLM_PROVIDER:-openai}"
LLM_MODEL="${LLM_MODEL:-gpt-5.4-mini}"
MAX_STEPS="${MAX_STEPS:-20}"
TIME_BUDGET="${TIME_BUDGET:-5m}"
JOB_PREFIX="cr-checkpoint"
SKIP_HEALTHCHECK=false
RUN_SET_ID=""
RUN_SET_LABEL=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        -o|--output)
            OUTPUT_BASE="$2"
            shift 2
            ;;
        --checkpoint-runs)
            CHECKPOINT_RUNS="$2"
            shift 2
            ;;
        --target-runs)
            TARGET_RUNS="$2"
            shift 2
            ;;
        --continue-to-five)
            CONTINUE_TO_FIVE=true
            shift
            ;;
        -b|--binary)
            BINARY="$2"
            shift 2
            ;;
        --run-set-id)
            RUN_SET_ID="$2"
            shift 2
            ;;
        --label)
            RUN_SET_LABEL="$2"
            shift 2
            ;;
        --llm-provider)
            LLM_PROVIDER="$2"
            shift 2
            ;;
        --llm-model)
            LLM_MODEL="$2"
            shift 2
            ;;
        --max-steps)
            MAX_STEPS="$2"
            shift 2
            ;;
        --time-budget)
            TIME_BUDGET="$2"
            shift 2
            ;;
        --job-prefix)
            JOB_PREFIX="$2"
            shift 2
            ;;
        --skip-healthcheck)
            SKIP_HEALTHCHECK=true
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

if [[ "${TARGET_RUNS}" -lt "${CHECKPOINT_RUNS}" ]]; then
    echo "ERROR: --target-runs must be greater than or equal to --checkpoint-runs." >&2
    exit 1
fi

model_slug="$(scenario_run_slug "${LLM_MODEL}")"
provider_slug="$(scenario_run_slug "${LLM_PROVIDER}")"

if [[ -z "${RUN_SET_ID}" ]]; then
    RUN_SET_ID="$(date -u +%Y%m%dT%H%M%SZ)-${provider_slug}-${model_slug}"
fi
if [[ -z "${RUN_SET_LABEL}" ]]; then
    RUN_SET_LABEL="${LLM_PROVIDER}/${LLM_MODEL} checkpoint $(date -u +%Y-%m-%d)"
fi

RUN_SET_DIR="${OUTPUT_BASE}/run-sets/${RUN_SET_ID}"
SUMMARY_PATH="${RUN_SET_DIR}/checkpoint-summary.md"
RUN_IDS_PATH="${RUN_SET_DIR}/run-ids.txt"

mkdir -p "${RUN_SET_DIR}"
: > "${RUN_IDS_PATH}"

cat > "${SUMMARY_PATH}" <<EOF
# Evaluation Checkpoint

- Run set ID: \`${RUN_SET_ID}\`
- Model: \`${LLM_PROVIDER}/${LLM_MODEL}\`
- Max steps: \`${MAX_STEPS}\`
- Time budget: \`${TIME_BUDGET}\`
- Checkpoint runs: \`${CHECKPOINT_RUNS}\`
- Continue to five: \`${CONTINUE_TO_FIVE}\`

## Runs
EOF

compare_ge() {
    local left="$1"
    local right="$2"
    awk -v left="${left}" -v right="${right}" 'BEGIN { exit !(left + 0 >= right + 0) }'
}

append_run_summary() {
    local run_index="$1"
    local run_id="$2"
    local scenario_rate="$3"
    local family_statuses="$4"
    local missing_artifacts="$5"
    local run_valid="$6"

    {
        echo ""
        echo "### Run ${run_index}"
        echo ""
        echo "- Run ID: \`${run_id}\`"
        echo "- Model: \`${LLM_PROVIDER}/${LLM_MODEL}\`"
        echo "- Scenario coverage: \`${scenario_rate}\`"
        echo "- Families: ${family_statuses}"
        echo "- Missing artifacts: ${missing_artifacts}"
        echo "- Valid checkpoint run: \`${run_valid}\`"
    } >> "${SUMMARY_PATH}"
}

run_ids=()
complete_runs=0
passing_runs=0
kg005_miss_runs=0
non_kg005_miss_runs=0
invalid_runs=0

run_batch() {
    local start_index="$1"
    local end_index="$2"
    local skip_healthcheck_flag="${3}"
    local idx=""
    local -a skip_healthcheck_args=()

    if [[ -n "${skip_healthcheck_flag}" ]]; then
        skip_healthcheck_args=("${skip_healthcheck_flag}")
    fi

    for idx in $(seq "${start_index}" "${end_index}"); do
        local timestamp=""
        local run_id=""
        local run_dir=""
        local job_name=""
        local runner_rc=0
        local missing_artifacts=""
        local missing_summary="none"
        local scenario_rate=""
        local family_statuses=""
        local false_families=""
        local run_valid="false"

        timestamp="$(date -u +%Y-%m-%dT%H%M%SZ)"
        run_id="${timestamp}-react-incluster-${model_slug}-r$(printf '%02d' "${idx}")"
        run_dir="${OUTPUT_BASE}/${run_id}"
        job_name="${JOB_PREFIX}-$(printf '%02d' "${idx}")"

        echo "=== Checkpoint run ${idx}: ${run_id} ==="
        if "${SCRIPTS_DIR}/run-validate-in-cluster.sh" \
            --output "${OUTPUT_BASE}" \
            --run-id "${run_id}" \
            --job-name "${job_name}" \
            --llm-provider "${LLM_PROVIDER}" \
            --llm-model "${LLM_MODEL}" \
            --max-steps "${MAX_STEPS}" \
            --time-budget "${TIME_BUDGET}" \
            "${skip_healthcheck_args[@]}"; then
            runner_rc=0
        else
            runner_rc=$?
        fi
        skip_healthcheck_flag="--skip-healthcheck"

        run_ids+=("${run_id}")
        printf '%s\n' "${run_id}" >> "${RUN_IDS_PATH}"

        missing_artifacts="$(scenario_run_list_missing_artifacts "${run_dir}" || true)"
        if [[ -n "${missing_artifacts}" ]]; then
            missing_summary="$(printf '%s\n' "${missing_artifacts}" | paste -sd ',' - | sed 's/,/, /g')"
        fi

        scenario_rate="$(scenario_run_scenario_rate "${run_dir}")"
        family_statuses="$(scenario_run_family_statuses "${run_dir}")"
        false_families="$(scenario_run_false_families "${run_dir}" || true)"

        if [[ ${runner_rc} -eq 0 && -z "${missing_artifacts}" ]]; then
            run_valid="true"
            complete_runs=$((complete_runs + 1))
            if [[ "${scenario_rate}" != "null" ]] && compare_ge "${scenario_rate}" "0.80"; then
                passing_runs=$((passing_runs + 1))
            fi
        else
            invalid_runs=$((invalid_runs + 1))
        fi

        if printf '%s\n' "${false_families}" | grep -q '^KG-005$'; then
            kg005_miss_runs=$((kg005_miss_runs + 1))
        fi
        if printf '%s\n' "${false_families}" | grep -qv '^KG-005$'; then
            non_kg005_miss_runs=$((non_kg005_miss_runs + 1))
        fi

        append_run_summary "${idx}" "${run_id}" "${scenario_rate}" "${family_statuses}" "${missing_summary}" "${run_valid}"
        echo ""
    done
}

initial_skip_flag=""
if [[ "${SKIP_HEALTHCHECK}" == "true" ]]; then
    initial_skip_flag="--skip-healthcheck"
fi
run_batch 1 "${CHECKPOINT_RUNS}" "${initial_skip_flag}"

decision="stop_and_review"
decision_reason="checkpoint did not yet meet the proceed criteria"

if [[ "${complete_runs}" -eq "${CHECKPOINT_RUNS}" && "${passing_runs}" -ge 2 ]]; then
    decision="proceed_to_reproducibility"
    decision_reason="all checkpoint runs completed with full bundles and at least two runs reached SC >= 0.80"
elif [[ "${invalid_runs}" -gt 0 ]]; then
    decision="stop_on_artifact_surface"
    decision_reason="one or more runs failed the required artifact bundle contract"
elif [[ "${kg005_miss_runs}" -ge 2 && "${non_kg005_miss_runs}" -eq 0 ]]; then
    decision="bounded_kg005_hardening_candidate"
    decision_reason="the repeated remaining miss appears to be isolated to KG-005"
fi

{
    echo ""
    echo "## Decision Gate"
    echo ""
    echo "- Complete runs: \`${complete_runs}/${CHECKPOINT_RUNS}\`"
    echo "- Runs with SC >= 0.80: \`${passing_runs}/${CHECKPOINT_RUNS}\`"
    echo "- Runs with KG-005 miss: \`${kg005_miss_runs}\`"
    echo "- Runs with non-KG-005 misses: \`${non_kg005_miss_runs}\`"
    echo "- Decision: \`${decision}\`"
    echo "- Reason: ${decision_reason}"
} >> "${SUMMARY_PATH}"

if [[ "${CONTINUE_TO_FIVE}" == "true" && "${decision}" == "proceed_to_reproducibility" ]]; then
    run_batch "$((CHECKPOINT_RUNS + 1))" "${TARGET_RUNS}" "--skip-healthcheck"

    if [[ ! -x "${BINARY}" ]]; then
        echo "ERROR: binary not found or not executable for analyze: ${BINARY}" >&2
        echo "Checkpoint summary written to ${SUMMARY_PATH}" >&2
        exit 1
    fi

    analyze_args=(
        analyze
        -i "${OUTPUT_BASE}"
        -o "${RUN_SET_DIR}/analysis.json"
        --run-set-id "${RUN_SET_ID}"
        --label "${RUN_SET_LABEL}"
    )
    for run_id in "${run_ids[@]}"; do
        analyze_args+=(--run-id "${run_id}")
    done

    "${BINARY}" "${analyze_args[@]}" | tee "${RUN_SET_DIR}/analysis-summary.txt"
fi

echo "Checkpoint summary written to ${SUMMARY_PATH}"
