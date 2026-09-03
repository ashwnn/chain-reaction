#!/usr/bin/env bash
# goat-env-config.sh — shared version pins and paths for the evaluation environment.
#
# Sourced by setup, teardown, and healthcheck scripts.
# This is the single source of truth for pinned versions.
#
# If you need to change versions, change them HERE.

set -euo pipefail

# ---------------------------------------------------------------------------
# Cluster
# ---------------------------------------------------------------------------

KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME:-chain-reaction-goat}"

# Hard requirement — the scripts will refuse to run with older kind.
KIND_VERSION_MIN="0.24.0"

# Pinned node image with digest. Matches the environment assumptions in
# built-in catalog
KIND_NODE_IMAGE="kindest/node:v1.30.4@sha256:976ea815844d5fa93be213437e3ff5754cd599b040946b5cca43ca45c2047114"

# ---------------------------------------------------------------------------
# Chain Reaction container image
# ---------------------------------------------------------------------------

# Must match the image reference in deploy/job.yaml.
# Loaded into kind via: make env-load-image
CHAIN_REACTION_IMAGE="${CHAIN_REACTION_IMAGE:-ghcr.io/ashwnn/chain-reaction:dev}"

# ---------------------------------------------------------------------------
# Kubernetes Goat upstream
# ---------------------------------------------------------------------------

GOAT_REPO="https://github.com/madhuakula/kubernetes-goat.git"

# Researched pin (commit aa72b61). v2.3.0 fixes kubectl --short breakage
# on K8s 1.30+. Verify with: git ls-remote --tags "${GOAT_REPO}"
GOAT_REF="v2.3.0"

# ---------------------------------------------------------------------------
# Paths (derived, do not override)
# ---------------------------------------------------------------------------

SCRIPTS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPTS_DIR}/.." && pwd)"

KIND_CONFIG="${PROJECT_ROOT}/deploy/kind-config.yaml"
GOAT_VALUES="${PROJECT_ROOT}/deploy/goat-values.yaml"
DEPLOY_DIR="${PROJECT_ROOT}/deploy"
