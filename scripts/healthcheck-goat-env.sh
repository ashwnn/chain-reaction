#!/usr/bin/env bash
# healthcheck-goat-env.sh — verify the evaluation environment is ready.
#
# Usage:
#   ./scripts/healthcheck-goat-env.sh
#   make env-healthcheck
#
# Exits 0 if healthy, 1 if any check fails.
# All failures are printed to stderr with remediation hints.
#
# Environment variables:
#   GOAT_READINESS_TIMEOUT  — seconds to wait for Goat pods to become
#                              Ready (default: 60). Set to 0 to skip
#                              the readiness wait loop.

set -euo pipefail

SCRIPTS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source-path=SCRIPTDIR
source "${SCRIPTS_DIR}/goat-env-config.sh"

PASS=0
FAIL=0

EXPECTED_CONTEXT="kind-${KIND_CLUSTER_NAME}"
GOAT_READINESS_TIMEOUT="${GOAT_READINESS_TIMEOUT:-60}"

# check runs a shell command string and records pass/fail.
# Usage: check "description" "command string with pipes etc."
check() {
    local description="$1"
    local cmd="$2"
    if eval "${cmd}" &>/dev/null; then
        echo "  [PASS] ${description}"
        PASS=$((PASS + 1))
    else
        echo "  [FAIL] ${description}" >&2
        FAIL=$((FAIL + 1))
    fi
}

# check_warn is like check but only warns — does not count as a failure.
check_warn() {
    local description="$1"
    local cmd="$2"
    if eval "${cmd}" &>/dev/null; then
        echo "  [PASS] ${description}"
        PASS=$((PASS + 1))
    else
        echo "  [WARN] ${description}" >&2
        # Warns do not increment FAIL.
    fi
}

count_default_pods() {
    { kubectl -n default get pods --field-selector=status.phase!=Succeeded,status.phase!=Failed --no-headers 2>/dev/null || true; } \
        | awk 'END { print NR + 0 }'
}

count_ready_default_pods() {
    { kubectl -n default get pods --field-selector=status.phase!=Succeeded,status.phase!=Failed --no-headers 2>/dev/null || true; } \
        | awk '{split($2,a,"/"); if ($3 == "Running" && a[1] == a[2]) count++} END {print count+0}'
}

count_error_default_pods() {
    { kubectl -n default get pods --field-selector=status.phase!=Succeeded,status.phase!=Failed --no-headers 2>/dev/null || true; } \
        | awk '/CrashLoopBackOff|Error|ImagePullBackOff|ErrImagePull|Unknown/ {count++} END {print count+0}'
}

echo "=== Chain Reaction Evaluation Environment Health Check ==="
echo ""

# ---------------------------------------------------------------------------
# 1. Cluster existence and context
# ---------------------------------------------------------------------------

echo "[Cluster]"

check "kind cluster '${KIND_CLUSTER_NAME}' exists" \
    "kind get clusters 2>/dev/null | grep -q '^${KIND_CLUSTER_NAME}\$'"

check "kubectl context is '${EXPECTED_CONTEXT}'" \
    "[ \"\$(kubectl config current-context 2>/dev/null)\" = '${EXPECTED_CONTEXT}' ]"

check "Kubernetes API is reachable" \
    "kubectl cluster-info 2>/dev/null"

check "Kubernetes server version matches pin (v1.30)" \
    "kubectl version --output=json 2>/dev/null | grep -q '\"gitVersion\": \"v1\\.30\\.'"

echo ""

# ---------------------------------------------------------------------------
# 2. Core cluster health
# ---------------------------------------------------------------------------

echo "[Core Services]"

check "CoreDNS pods are Running" \
    "kubectl -n kube-system get pods -l k8s-app=kube-dns --no-headers 2>/dev/null | awk '{if (\$3 != \"Running\") exit 1} END {if (NR == 0) exit 1}'"

check "kube-proxy pods are Running" \
    "kubectl -n kube-system get pods -l k8s-app=kube-proxy --no-headers 2>/dev/null | awk '{if (\$3 != \"Running\") exit 1} END {if (NR == 0) exit 1}'"

echo ""

# ---------------------------------------------------------------------------
# 3. Kubernetes Goat readiness
# ---------------------------------------------------------------------------

echo "[Kubernetes Goat]"

# Count active pods in the default namespace where Goat scenarios deploy.
GOAT_TOTAL="$(count_default_pods)"

if [ "${GOAT_TOTAL}" -eq 0 ]; then
    echo "  [FAIL] No active Goat pods found in default namespace" >&2
    echo "         Did setup complete? Run: make env-setup" >&2
    FAIL=$((FAIL + 1))
else
    echo "  [PASS] Active Goat pods found in default namespace (${GOAT_TOTAL} total)"
    PASS=$((PASS + 1))

    # Count pods that are Running AND Ready (READY column shows N/M).
    GOAT_READY="$(count_ready_default_pods)"

    if [ "${GOAT_READY}" -eq "${GOAT_TOTAL}" ]; then
        echo "  [PASS] All ${GOAT_READY} active Goat pods are Running and Ready"
        PASS=$((PASS + 1))
    elif [ "${GOAT_READINESS_TIMEOUT}" -gt 0 ] && [ "${GOAT_READY}" -lt "${GOAT_TOTAL}" ]; then
        # Not all ready yet — wait with a timeout.
        echo "  [WAIT] ${GOAT_READY}/${GOAT_TOTAL} active Goat pods Ready — waiting up to ${GOAT_READINESS_TIMEOUT}s..."
        ELAPSED=0
        while [ "${ELAPSED}" -lt "${GOAT_READINESS_TIMEOUT}" ]; do
            sleep 5
            ELAPSED=$((ELAPSED + 5))
            GOAT_READY_NOW="$(count_ready_default_pods)"
            if [ "${GOAT_READY_NOW}" -eq "${GOAT_TOTAL}" ]; then
                echo "  [PASS] All ${GOAT_READY_NOW} active Goat pods are Running and Ready (waited ${ELAPSED}s)"
                PASS=$((PASS + 1))
                break
            fi
            echo "         ${GOAT_READY_NOW}/${GOAT_TOTAL} Ready after ${ELAPSED}s..."
        done

        # Final check after timeout.
        GOAT_READY_FINAL="$(count_ready_default_pods)"
        if [ "${GOAT_READY_FINAL}" -lt "${GOAT_TOTAL}" ]; then
            echo "  [FAIL] Only ${GOAT_READY_FINAL}/${GOAT_TOTAL} active Goat pods Ready after ${GOAT_READINESS_TIMEOUT}s" >&2
            echo "         Check pod events: kubectl -n default describe pods" >&2
            FAIL=$((FAIL + 1))
        fi
    fi

    # Warn if any pods are in CrashLoopBackOff or other non-Running states.
    CRASHING="$(count_error_default_pods)"
    if [ "${CRASHING}" -gt 0 ]; then
        echo "  [WARN] ${CRASHING} Goat pod(s) in error state (CrashLoopBackOff/Error/ImagePullBackOff)" >&2
    fi
fi

echo ""

# ---------------------------------------------------------------------------
# 4. Chain Reaction namespace and RBAC
# ---------------------------------------------------------------------------

echo "[Chain Reaction]"

check "chain-reaction namespace exists" \
    "kubectl get namespace chain-reaction 2>/dev/null"

check "chain-reaction ServiceAccount exists" \
    "kubectl -n chain-reaction get serviceaccount chain-reaction 2>/dev/null"

check "chain-reaction ClusterRole exists" \
    "kubectl get clusterrole chain-reaction-read 2>/dev/null"

check "chain-reaction ClusterRoleBinding exists" \
    "kubectl get clusterrolebinding chain-reaction-read 2>/dev/null"

# Verify the Chain Reaction image is loaded into the kind node.
# This is a warning, not a hard failure — Goat is still usable without it.
# Guarded: docker may not be installed if the operator only runs healthcheck
# against a remote cluster or already has the image.
if command -v docker &>/dev/null; then
    check_warn "Chain Reaction image '${CHAIN_REACTION_IMAGE}' is loaded into kind" \
        "docker exec kind-${KIND_CLUSTER_NAME}-control-plane crictl images 2>/dev/null | grep -q 'chain-reaction'"
fi

echo ""

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------

echo "=== Results: ${PASS} passed, ${FAIL} failed ==="

if [ "${FAIL}" -gt 0 ]; then
    echo ""
    echo "Remediation:" >&2
    echo "  Full reset:    make env-teardown && make env-setup" >&2
    echo "  Image missing: make env-load-image" >&2
    exit 1
fi

exit 0
