#!/usr/bin/env bash
# run-comparison.sh — Offline benchmark-vs-baselines comparison workflow
#
# Produces the full comparison artifact set from saved runtime, theory, and scan
# artifacts. All work is fully offline; no live cluster, no LLM calls, no Kubernetes
# Goat reruns.
#
# Inputs (all optional, see EXAMPLES):
#   - analysis.json    — runtime analysis from repeated validate runs
#   - theory/comparison-baseline.json  — static theoretical baseline
#   - scan/comparison-baseline.json    — deterministic discovery baseline
#
# Outputs (written to --output or ./comparison/):
#   - comparison.json           — machine-readable joined result
#   - comparison.md             — basic step-level Markdown table
#   - comparison-gap-chart.svg   — theory vs scan vs runtime coverage bars
#   - comparison-step-heatmap.svg — per-family per-step status grid
#   - comparison-chain-status.svg — per-family chain validation status
#   - comparison-narrative.md    — per-family narrative with blocker explanations
#
# Usage:
#   ./scripts/run-comparison.sh [OPTIONS]
#
# Options:
#   -a, --analysis PATH    Path to analysis.json
#   -t, --theory PATH      Path to theory/comparison-baseline.json
#   -s, --scan PATH        Path to scan/comparison-baseline.json
#   -o, --output DIR       Output directory (default: ./comparison/)
#   -h, --help             Show this help message
#
# Examples:
#   # Full comparison with all three artifact types:
#   ./scripts/run-comparison.sh \
#     -a artifacts/scenario-runs/run-sets/openai-gpt-5.4-mini-2026-04-09-live/analysis.json \
#     -t artifacts/theory/comparison-baseline.json \
#     -s artifacts/scan/comparison-baseline.json
#
#   # Theory vs scan only (no runtime data):
#   ./scripts/run-comparison.sh \
#     -t artifacts/theory/comparison-baseline.json \
#     -s artifacts/scan/comparison-baseline.json
#
#   # Theory only (catalog-only comparison):
#   ./scripts/run-comparison.sh -t artifacts/theory/comparison-baseline.json
#
# Environment:
#   CHAIN_REACTION_BIN   Path to chain-reaction binary (default: ./bin/chain-reaction)
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
BIN="${CHAIN_REACTION_BIN:-${REPO_DIR}/bin/chain-reaction}"
OUTPUT_DIR="./comparison/"

# Parse arguments
ANALYSIS_PATH=""
THEORY_PATH=""
SCAN_PATH=""

usage() {
  grep "^#" "$0" | sed 's/^#//'
  exit 1
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    -a|--analysis)  ANALYSIS_PATH="$2"; shift 2 ;;
    -t|--theory)    THEORY_PATH="$2"; shift 2 ;;
    -s|--scan)      SCAN_PATH="$2"; shift 2 ;;
    -o|--output)    OUTPUT_DIR="$2"; shift 2 ;;
    -h|--help)      usage ;;
    *)              echo "Unknown option: $1"; usage ;;
  esac
done

# Build the chain-reaction binary if not present
if [[ ! -x "$BIN" ]]; then
  echo "chain-reaction binary not found at $BIN, building..."
  (cd "$REPO_DIR" && go build -o "$BIN" ./cmd/chain-reaction/)
fi

# Create output directory
mkdir -p "$OUTPUT_DIR"

echo "=== Chain Reaction Comparison Workflow ==="
echo "Output directory: $OUTPUT_DIR"
echo ""

# Build the compare command arguments
CMD_ARGS=("compare")

if [[ -n "$ANALYSIS_PATH" ]]; then
  if [[ ! -f "$ANALYSIS_PATH" ]]; then
    echo "ERROR: analysis artifact not found: $ANALYSIS_PATH" >&2
    exit 1
  fi
  CMD_ARGS+=("-a" "$ANALYSIS_PATH")
  echo "Analysis: $ANALYSIS_PATH"
fi

if [[ -n "$THEORY_PATH" ]]; then
  if [[ ! -f "$THEORY_PATH" ]]; then
    echo "ERROR: theory artifact not found: $THEORY_PATH" >&2
    exit 1
  fi
  CMD_ARGS+=("-t" "$THEORY_PATH")
  echo "Theory: $THEORY_PATH"
fi

if [[ -n "$SCAN_PATH" ]]; then
  if [[ ! -f "$SCAN_PATH" ]]; then
    echo "ERROR: scan artifact not found: $SCAN_PATH" >&2
    exit 1
  fi
  CMD_ARGS+=("-s" "$SCAN_PATH")
  echo "Scan: $SCAN_PATH"
fi

if [[ ${#CMD_ARGS[@]} -eq 1 ]]; then
  echo "ERROR: at least one artifact path is required (-a, -t, or -s)" >&2
  usage
fi

echo ""
echo "Running comparison..."
echo ""

# Run the comparison command
"$BIN" "${CMD_ARGS[@]}" \
  --output "$OUTPUT_DIR" \
  --plots \
  --tables

echo ""
echo "=== Comparison artifacts written to $OUTPUT_DIR ==="
echo "Files:"
ls -1 "$OUTPUT_DIR"
