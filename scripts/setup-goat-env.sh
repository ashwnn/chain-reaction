#!/usr/bin/env bash
# setup-goat-env.sh — create the kind cluster, deploy Kubernetes Goat,
# and deploy Chain Reaction manifests.
#
# Usage:
#   ./scripts/setup-goat-env.sh
#   make env-setup
#
# Prerequisites: kind (v0.24.0+), kubectl, git, docker (for image load)
#
# This script FAILS LOUDLY on any error (set -euo pipefail).

set -euo pipefail

SCRIPTS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source-path=SCRIPTDIR
source "${SCRIPTS_DIR}/goat-env-config.sh"

EXPECTED_CONTEXT="kind-${KIND_CLUSTER_NAME}"

echo "=== Chain Reaction Evaluation Environment Setup ==="
echo ""

# ---------------------------------------------------------------------------
# 1. Check prerequisites
# ---------------------------------------------------------------------------

check_cmd() {
    if ! command -v "$1" &>/dev/null; then
        echo "ERROR: $1 is not installed. $2" >&2
        exit 1
    fi
}

check_cmd kind "Install from https://kind.sigs.k8s.io/ (v${KIND_VERSION_MIN}+)"
check_cmd kubectl "Install from https://kubernetes.io/docs/tasks/tools/"
check_cmd git "Install your platform's git package."

# Verify kind version meets minimum.
KIND_VERSION="$(kind version --output json 2>/dev/null | grep -o '"kindVersion":"[^"]*"' | head -1 | cut -d'"' -f4 || true)"
if [ -z "${KIND_VERSION}" ]; then
    KIND_VERSION="$(kind version 2>/dev/null | awk '{print $2}' || true)"
fi
if [ -n "${KIND_VERSION}" ]; then
    # kind version strings look like "0.24.0" or "v0.24.0".
    KIND_VERSION_CLEAN="${KIND_VERSION#v}"
    KIND_VERSION_MAJOR="$(echo "${KIND_VERSION_CLEAN}" | cut -d. -f1)"
    KIND_VERSION_MINOR="$(echo "${KIND_VERSION_CLEAN}" | cut -d. -f2)"
    MIN_MAJOR="$(echo "${KIND_VERSION_MIN}" | cut -d. -f1)"
    MIN_MINOR="$(echo "${KIND_VERSION_MIN}" | cut -d. -f2)"
    if [ "${KIND_VERSION_MAJOR}" -lt "${MIN_MAJOR}" ] \
        || { [ "${KIND_VERSION_MAJOR}" -eq "${MIN_MAJOR}" ] && [ "${KIND_VERSION_MINOR}" -lt "${MIN_MINOR}" ]; }; then
        echo "ERROR: kind version ${KIND_VERSION} is below minimum ${KIND_VERSION_MIN}." >&2
        echo "       Upgrade kind: https://kind.sigs.k8s.io/docs/user/quick-start/#upgrading" >&2
        exit 1
    fi
    echo "[1/6] Prerequisites OK (kind v${KIND_VERSION_CLEAN})."
else
    echo "[1/6] Prerequisites OK (kind version could not be parsed — continuing)."
fi

# ---------------------------------------------------------------------------
# 2. Create kind cluster
# ---------------------------------------------------------------------------

if kind get clusters 2>/dev/null | grep -q "^${KIND_CLUSTER_NAME}$"; then
    echo "[2/6] Kind cluster '${KIND_CLUSTER_NAME}' already exists — skipping creation."
else
    echo "[2/6] Creating kind cluster '${KIND_CLUSTER_NAME}' (K8s ${KIND_NODE_IMAGE})..."
    kind create cluster \
        --name "${KIND_CLUSTER_NAME}" \
        --config "${KIND_CONFIG}" \
        --wait 120s
    echo "      Cluster ready."
fi

# ---------------------------------------------------------------------------
# 3. Verify kubectl context
# ---------------------------------------------------------------------------

CURRENT_CONTEXT="$(kubectl config current-context 2>/dev/null || echo "")"

if [ "${CURRENT_CONTEXT}" = "${EXPECTED_CONTEXT}" ]; then
    echo "[3/6] kubectl context already set to '${EXPECTED_CONTEXT}'."
else
    echo "[3/6] Switching kubectl context from '${CURRENT_CONTEXT}' to '${EXPECTED_CONTEXT}'..."
    kubectl config use-context "${EXPECTED_CONTEXT}" >/dev/null
fi

# Verify the switch actually took effect.
ACTUAL_CONTEXT="$(kubectl config current-context 2>/dev/null || echo "")"
if [ "${ACTUAL_CONTEXT}" != "${EXPECTED_CONTEXT}" ]; then
    echo "ERROR: kubectl context is '${ACTUAL_CONTEXT}' but expected '${EXPECTED_CONTEXT}'." >&2
    echo "       The cluster may not be running or the context was not created." >&2
    exit 1
fi

# ---------------------------------------------------------------------------
# 4. Clone Kubernetes Goat at pinned ref
# ---------------------------------------------------------------------------

GOAT_CLONE_DIR="$(mktemp -d)"
trap 'rm -rf "${GOAT_CLONE_DIR}"' EXIT

echo "[4/6] Cloning Kubernetes Goat at ref '${GOAT_REF}'..."
if ! git clone --depth 1 --branch "${GOAT_REF}" "${GOAT_REPO}" "${GOAT_CLONE_DIR}" 2>&1; then
    echo "ERROR: Failed to clone Kubernetes Goat at ref '${GOAT_REF}'." >&2
    echo "       Verify the tag exists: git ls-remote --tags ${GOAT_REPO}" >&2
    echo "       Update GOAT_REF in scripts/goat-env-config.sh if needed." >&2
    exit 1
fi

# ---------------------------------------------------------------------------
# 5. Deploy Kubernetes Goat
# ---------------------------------------------------------------------------

echo "[5/6] Deploying Kubernetes Goat manifests..."

# Trust boundary: we execute the upstream setup-kubernetes-goat.sh at the
# pinned ref cloned above.  The script creates deliberately-vulnerable
# resources in the cluster.  The pinned ref constrains what runs, but the
# operator should still review upstream changes when bumping GOAT_REF.
if [ -f "${GOAT_CLONE_DIR}/setup-kubernetes-goat.sh" ]; then
    cd "${GOAT_CLONE_DIR}"
    bash setup-kubernetes-goat.sh
    cd - >/dev/null
else
    echo "       WARNING: No upstream setup-kubernetes-goat.sh found at ref '${GOAT_REF}'." >&2
    echo "       The upstream layout may have changed. Deploying may require manual steps." >&2
fi

echo "      Goat manifests applied."

# ---------------------------------------------------------------------------
# 6. Load Chain Reaction image + deploy manifests
# ---------------------------------------------------------------------------

echo "[6/6] Deploying Chain Reaction..."

# Attempt to load the locally-built image into the kind cluster.
# This is optional — if the image is not built yet, warn but continue
# so the Goat environment is still usable for pre-deployment testing.
if docker image inspect "${CHAIN_REACTION_IMAGE}" &>/dev/null; then
    echo "      Loading image '${CHAIN_REACTION_IMAGE}' into kind..."
    kind load docker-image \
        --name "${KIND_CLUSTER_NAME}" \
        "${CHAIN_REACTION_IMAGE}"
    echo "      Image loaded."
else
    echo "      WARNING: Image '${CHAIN_REACTION_IMAGE}' not found locally." >&2
    echo "       Build and load it with:  make env-load-image" >&2
    echo "       The Chain Reaction Job will fail with ImagePullBackOff until loaded." >&2
fi

# Apply namespace, SA, and RBAC. The Job is applied on-demand during a scan.
kubectl apply -f "${DEPLOY_DIR}/namespace.yaml"
kubectl apply -f "${DEPLOY_DIR}/serviceaccount.yaml"
kubectl apply -f "${DEPLOY_DIR}/rbac.yaml"
echo "      Chain Reaction namespace and RBAC ready."

# ---------------------------------------------------------------------------
# Done
# ---------------------------------------------------------------------------

echo ""
echo "=== Setup complete ==="
echo "Cluster : ${KIND_CLUSTER_NAME}"
echo "K8s     : ${KIND_NODE_IMAGE}"
echo "Goat ref: ${GOAT_REF}"
echo ""
echo "Next steps:"
echo "  1. Run 'make env-healthcheck' to verify readiness."
echo "  2. If Chain Reaction image was not loaded, run: make env-load-image"
