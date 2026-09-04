#!/usr/bin/env bash
# run-validate-in-cluster.sh — run `chain-reaction validate` inside the
# evaluation cluster using the chain-reaction ServiceAccount, then copy the
# produced artifacts back to the local scenario-runs directory.
#
# Usage:
#   ./scripts/run-validate-in-cluster.sh [OPTIONS]
#
# Options:
#   -o, --output DIR       Local output base (default: artifacts/scenario-runs)
#   -n, --job-name NAME    Kubernetes Job name (default: chain-reaction-validate)
#   --run-id ID            Override the generated run ID
#   --llm-provider NAME    LLM provider (default: openai)
#   --llm-model MODEL      LLM model (default: gpt-5.4-mini)
#   --llm-base-url URL     Optional custom LLM base URL
#   Local env / Secret keys accepted for LLM auth:
#     OPENAI_API_KEY / ANTHROPIC_API_KEY / GROQ_API_KEY
#     LLM_API_KEY (generic fallback; useful for self-hosted endpoints)
#   --max-steps N          Max validation steps (default: 20)
#   --time-budget DUR      Validation time budget (default: 5m)
#   --no-debug             Disable live validate debug logging in the pod
#   --skip-healthcheck     Skip the pre-run healthcheck
#   --keep-job             Keep the completed Job instead of deleting it first
#   -h, --help             Show this help

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
JOB_NAME="chain-reaction-validate"
RUN_ID=""
LLM_PROVIDER="${LLM_PROVIDER:-openai}"
LLM_MODEL="${LLM_MODEL:-gpt-5.4-mini}"
LLM_BASE_URL="${LLM_BASE_URL:-}"
MAX_STEPS="${MAX_STEPS:-20}"
TIME_BUDGET="${TIME_BUDGET:-5m}"
DEBUG=true
SKIP_HEALTHCHECK=false
KEEP_JOB=false
REMOTE_OUTPUT="/tmp/artifacts"
REMOTE_DONE="/tmp/chain-reaction.done"
REMOTE_EXITCODE="/tmp/chain-reaction.exitcode"
REMOTE_RELEASE="/tmp/chain-reaction.release"
REMOTE_STDOUT="/tmp/chain-reaction.stdout"
SECRET_NAME="chain-reaction-llm"
NAMESPACE="chain-reaction"
LOG_STREAM_PID=""
POD_NAME=""
RUN_STARTED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

stop_log_stream() {
    if [[ -n "${LOG_STREAM_PID:-}" ]]; then
        kill "${LOG_STREAM_PID}" >/dev/null 2>&1 || true
        wait "${LOG_STREAM_PID}" >/dev/null 2>&1 || true
        LOG_STREAM_PID=""
    fi
}

capture_job_status() {
    if [[ -n "${POD_NAME:-}" ]]; then
        kubectl -n "${NAMESPACE}" get pod "${POD_NAME}" -o wide > "${RUN_DIR}/pod-status.txt" 2>/dev/null || true
        kubectl -n "${NAMESPACE}" describe "pod/${POD_NAME}" > "${RUN_DIR}/pod-describe.txt" 2>/dev/null || true
    fi
    kubectl -n "${NAMESPACE}" describe "job/${JOB_NAME}" > "${RUN_DIR}/job-describe.txt" 2>/dev/null || true
}

start_log_stream() {
    : > "${RUN_DIR}/job.log"
    echo "=== Streaming pod logs ==="
    kubectl -n "${NAMESPACE}" logs -f "${POD_NAME}" 2>&1 | tee "${RUN_DIR}/job.log" &
    LOG_STREAM_PID=$!
}

trap stop_log_stream EXIT

while [[ $# -gt 0 ]]; do
    case "$1" in
        -o|--output)
            OUTPUT_BASE="$2"
            shift 2
            ;;
        -n|--job-name)
            JOB_NAME="$2"
            shift 2
            ;;
        --run-id)
            RUN_ID="$2"
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
        --llm-base-url)
            LLM_BASE_URL="$2"
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
        --no-debug)
            DEBUG=false
            shift
            ;;
        --skip-healthcheck)
            SKIP_HEALTHCHECK=true
            shift
            ;;
        --keep-job)
            KEEP_JOB=true
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

case "${LLM_PROVIDER}" in
    openai)
        PREFERRED_API_ENV_NAME="OPENAI_API_KEY"
        ;;
    anthropic)
        PREFERRED_API_ENV_NAME="ANTHROPIC_API_KEY"
        ;;
    groq)
        PREFERRED_API_ENV_NAME="GROQ_API_KEY"
        ;;
    *)
        echo "ERROR: unsupported llm provider: ${LLM_PROVIDER}" >&2
        exit 1
        ;;
esac

API_ENV_NAME="${PREFERRED_API_ENV_NAME}"
API_KEY="${!PREFERRED_API_ENV_NAME:-}"
if [[ -z "${API_KEY}" && -n "${LLM_API_KEY:-}" ]]; then
    API_KEY="${LLM_API_KEY}"
fi

REUSE_EXISTING_SECRET=false
if [[ -z "${API_KEY}" ]]; then
    existing_secret_key="$(kubectl -n "${NAMESPACE}" get secret "${SECRET_NAME}" -o "jsonpath={.data.${API_ENV_NAME}}" 2>/dev/null || true)"
    if [[ -n "${existing_secret_key}" ]]; then
        REUSE_EXISTING_SECRET=true
    else
        generic_secret_key="$(kubectl -n "${NAMESPACE}" get secret "${SECRET_NAME}" -o "jsonpath={.data.LLM_API_KEY}" 2>/dev/null || true)"
        if [[ -n "${generic_secret_key}" ]]; then
            API_ENV_NAME="LLM_API_KEY"
            REUSE_EXISTING_SECRET=true
        else
            echo "ERROR: ${PREFERRED_API_ENV_NAME} or LLM_API_KEY must be set in the local environment or already present in secret ${NAMESPACE}/${SECRET_NAME}." >&2
            exit 1
        fi
    fi
fi

if [[ "${SKIP_HEALTHCHECK}" == "false" ]]; then
    bash "${SCRIPTS_DIR}/healthcheck-goat-env.sh"
    echo ""
fi

if [[ -z "${RUN_ID}" ]]; then
    RUN_ID="$(date -u +%Y-%m-%dT%H%M%SZ)-react-incluster-$(scenario_run_slug "${LLM_MODEL}")"
fi

RUN_DIR="${OUTPUT_BASE}/${RUN_ID}"
mkdir -p "${RUN_DIR}"

K8S_VERSION="$(kubectl version --output=json 2>/dev/null | grep -o '"gitVersion": "[^"]*"' | head -1 | cut -d'"' -f4 || echo "unknown")"

write_run_metadata() {
    local run_status="$1"
    local pod_name="$2"
    local failure_reason="${3:-}"
    local completed_at="${4:-}"

    cat > "${RUN_DIR}/run-metadata.json" <<EOF
{
  "run_id": "${RUN_ID}",
  "planner_type": "react_llm_incluster",
  "llm_model": "${LLM_MODEL}",
  "llm_provider": "${LLM_PROVIDER}",
  "max_steps": ${MAX_STEPS},
  "time_budget": "${TIME_BUDGET}",
  "debug": ${DEBUG},
  "started_at": "${RUN_STARTED_AT}",
  "completed_at": "${completed_at}",
  "cluster_version": "${K8S_VERSION}",
  "goat_version": "${GOAT_REF}",
  "job_name": "${JOB_NAME}",
  "pod_name": "${pod_name}",
  "status": "${run_status}",
  "failure_reason": "${failure_reason}"
}
EOF
}

write_run_metadata "preparing" "" ""

echo "=== Preparing in-cluster validate run ==="
echo "Run ID  : ${RUN_ID}"
echo "Job     : ${JOB_NAME}"
echo "Model   : ${LLM_PROVIDER}/${LLM_MODEL}"
echo "LLM key : ${API_ENV_NAME}"
echo "Debug   : ${DEBUG}"
echo "Image   : ${CHAIN_REACTION_IMAGE}"
echo "Output  : ${RUN_DIR}"
echo ""

if [[ "${REUSE_EXISTING_SECRET}" == "true" ]]; then
    echo "Reusing existing secret ${NAMESPACE}/${SECRET_NAME} for ${API_ENV_NAME}."
else
    kubectl -n "${NAMESPACE}" create secret generic "${SECRET_NAME}" \
        --from-literal="${API_ENV_NAME}=${API_KEY}" \
        --dry-run=client \
        -o yaml | kubectl apply -f -
fi

if [[ "${KEEP_JOB}" == "false" ]]; then
    kubectl -n "${NAMESPACE}" delete job "${JOB_NAME}" --ignore-not-found >/dev/null
    kubectl -n "${NAMESPACE}" wait --for=delete --timeout=60s "job/${JOB_NAME}" >/dev/null 2>&1 || true
fi

validate_cmd="chain-reaction validate --output '${REMOTE_OUTPUT}' --time-budget '${TIME_BUDGET}' --max-steps '${MAX_STEPS}' --llm-provider '${LLM_PROVIDER}' --llm-model '${LLM_MODEL}'"
if [[ -n "${LLM_BASE_URL}" ]]; then
    validate_cmd="${validate_cmd} --llm-base-url '${LLM_BASE_URL}'"
fi
if [[ "${DEBUG}" == "true" ]]; then
    validate_cmd="${validate_cmd} --debug"
fi

# Export template variables so envsubst can substitute them.
export CHAIN_REACTION_IMAGE
export JOB_NAME
export NAMESPACE
export API_ENV_NAME
export SECRET_NAME
export REMOTE_OUTPUT
export REMOTE_STDOUT
export REMOTE_EXITCODE
export REMOTE_DONE
export REMOTE_RELEASE
export VALIDATE_CMD="${validate_cmd}"

# Apply the committed validate Job manifest (envsubst template form).
# The manifest in deploy/validate-job.yaml uses ${VARIABLE} references
# that are substituted here before submission to the API server.
# envsubst is restricted to the explicit deployment-time variable allowlist
# so that shell runtime variables (validate_pid, tail_pid, status, etc.)
# inside the wrapper script are preserved unchanged.
# shellcheck disable=SC2016 # envsubst receives literal variable references.
envsubst '${CHAIN_REACTION_IMAGE} ${JOB_NAME} ${NAMESPACE} ${API_ENV_NAME} ${SECRET_NAME} ${REMOTE_OUTPUT} ${REMOTE_STDOUT} ${REMOTE_EXITCODE} ${REMOTE_DONE} ${REMOTE_RELEASE} ${VALIDATE_CMD}' < "${DEPLOY_DIR}/validate-job.yaml" | kubectl apply -f -

echo "=== Waiting for pod startup ==="
for _ in $(seq 1 60); do
    POD_NAME="$(kubectl -n "${NAMESPACE}" get pods -l "job-name=${JOB_NAME}" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)"
    if [[ -n "${POD_NAME}" ]]; then
        break
    fi
    sleep 2
done

if [[ -z "${POD_NAME}" ]]; then
    echo "Validation pod was not created within the wait window." >&2
    capture_job_status
    write_run_metadata "failed" "" "pod_not_created" "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    exit 1
fi

kubectl -n "${NAMESPACE}" wait --for=condition=Ready --timeout=2m "pod/${POD_NAME}" >/dev/null
start_log_stream

echo "=== Waiting for validate process to finish ==="
for _ in $(seq 1 450); do
    if kubectl -n "${NAMESPACE}" exec "${POD_NAME}" -- test -f "${REMOTE_DONE}" >/dev/null 2>&1; then
        break
    fi
    sleep 2
done

if ! kubectl -n "${NAMESPACE}" exec "${POD_NAME}" -- test -f "${REMOTE_DONE}" >/dev/null 2>&1; then
    echo "Validation process did not signal completion within the wait window." >&2
    capture_job_status
    kubectl -n "${NAMESPACE}" exec "${POD_NAME}" -- touch "${REMOTE_RELEASE}" >/dev/null 2>&1 || true
    write_run_metadata "failed" "${POD_NAME}" "job_did_not_signal_completion" "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    exit 1
fi

EXIT_CODE="$(kubectl -n "${NAMESPACE}" exec "${POD_NAME}" -- cat "${REMOTE_EXITCODE}" 2>/dev/null || echo 1)"
stop_log_stream

echo ""
echo "=== Copying artifacts ==="
kubectl -n "${NAMESPACE}" cp "${POD_NAME}:${REMOTE_OUTPUT}/." "${RUN_DIR}" >/dev/null 2>&1 || true
kubectl -n "${NAMESPACE}" cp "${POD_NAME}:${REMOTE_STDOUT}" "${RUN_DIR}/job.log" >/dev/null 2>&1 || true
kubectl -n "${NAMESPACE}" exec "${POD_NAME}" -- touch "${REMOTE_RELEASE}" >/dev/null 2>&1 || true

if [[ "${EXIT_CODE}" != "0" ]]; then
    capture_job_status
    write_run_metadata "failed" "${POD_NAME}" "validate_exit_${EXIT_CODE}" "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    exit 1
fi

missing_artifacts="$(scenario_run_list_missing_artifacts "${RUN_DIR}")"
if [[ -n "${missing_artifacts}" ]]; then
    echo "Artifact bundle is incomplete for ${RUN_ID}." >&2
    printf '%s\n' "${missing_artifacts}" | sed 's/^/  missing: /' >&2
    capture_job_status
    write_run_metadata "failed" "${POD_NAME}" "missing_artifacts" "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    exit 1
fi

kubectl -n "${NAMESPACE}" wait --for=condition=complete --timeout=60s "job/${JOB_NAME}" >/dev/null 2>&1 || true
write_run_metadata "completed" "${POD_NAME}" "" "$(date -u +%Y-%m-%dT%H:%M:%SZ)"

echo "Artifacts copied to ${RUN_DIR}"
