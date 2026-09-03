#!/usr/bin/env bash
# teardown-goat-env.sh — delete the evaluation kind cluster.
#
# Usage:
#   ./scripts/teardown-goat-env.sh
#   make env-teardown
#
# This destroys the entire kind cluster. All in-cluster artifacts
# (evidence, logs, pod state) are lost.
#
# Set SKIP_CONFIRM=1 to skip the confirmation prompt (useful in CI).

set -euo pipefail

SCRIPTS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source-path=SCRIPTDIR
source "${SCRIPTS_DIR}/goat-env-config.sh"

echo "=== Chain Reaction Evaluation Environment Teardown ==="
echo ""

if ! kind get clusters 2>/dev/null | grep -q "^${KIND_CLUSTER_NAME}$"; then
    echo "Cluster '${KIND_CLUSTER_NAME}' does not exist — nothing to tear down."
    exit 0
fi

# Show what will be destroyed so the operator can make an informed decision.
echo "WARNING: This will DELETE the entire kind cluster '${KIND_CLUSTER_NAME}'."
echo "         All pods, namespaces, evidence, and in-cluster state will be lost."
echo ""

# List active namespaces and pod counts for visibility.
echo "Current cluster contents:"
kubectl --context "kind-${KIND_CLUSTER_NAME}" get namespaces --no-headers 2>/dev/null \
    | awk '{printf "  namespace/%s (%s)\n", $1, $2}' || echo "  (could not list namespaces)"
echo ""

# Count pods across all namespaces for a quick summary.
POD_COUNT="$(kubectl --context "kind-${KIND_CLUSTER_NAME}" get pods --all-namespaces --no-headers 2>/dev/null | wc -l || echo "0")"
echo "  Total pods across all namespaces: ${POD_COUNT}"
echo ""

# Confirmation prompt (unless skipped).
if [ "${SKIP_CONFIRM:-0}" != "1" ]; then
    echo -n "Type 'yes' to delete cluster '${KIND_CLUSTER_NAME}': "
    read -r CONFIRM
    if [ "${CONFIRM}" != "yes" ]; then
        echo "Aborted. Cluster '${KIND_CLUSTER_NAME}' was NOT deleted."
        exit 0
    fi
fi

echo "Deleting kind cluster '${KIND_CLUSTER_NAME}'..."
kind delete cluster --name "${KIND_CLUSTER_NAME}"

echo "Done. Cluster '${KIND_CLUSTER_NAME}' removed."
