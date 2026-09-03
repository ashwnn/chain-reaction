#!/usr/bin/env bash
# scenario-run-lib.sh — shared helpers for validating and summarizing
# scenario-run artifact bundles.

set -euo pipefail

scenario_run_slug() {
    local value="${1:-}"
    printf '%s' "${value}" \
        | tr '[:upper:]' '[:lower:]' \
        | sed 's/[^a-z0-9][^a-z0-9]*/-/g; s/^-//; s/-$//'
}

scenario_run_required_artifacts() {
    cat <<'EOF'
validation-metrics.json|file
graph/attack-graph.json|file
graph/attack-graph.dot|file
evidence/evidence.jsonl|file
evidence/index.json|file
evidence/snapshots|dir
validate-debug.log|file
run-metadata.json|file
job.log|file
EOF
}

scenario_run_list_missing_artifacts() {
    local run_dir="$1"
    local path=""
    local kind=""

    while IFS='|' read -r path kind; do
        [[ -z "${path}" ]] && continue
        case "${kind}" in
            file)
                if [[ ! -f "${run_dir}/${path}" ]]; then
                    printf '%s\n' "${path}"
                fi
                ;;
            dir)
                if [[ ! -d "${run_dir}/${path}" ]]; then
                    printf '%s/\n' "${path}"
                    continue
                fi
                if ! find "${run_dir}/${path}" -mindepth 1 -maxdepth 1 -print -quit | grep -q .; then
                    printf '%s/\n' "${path}"
                fi
                ;;
        esac
    done < <(scenario_run_required_artifacts)
}

scenario_run_is_complete() {
    local run_dir="$1"
    if [[ -n "$(scenario_run_list_missing_artifacts "${run_dir}")" ]]; then
        return 1
    fi
    return 0
}

scenario_run_scenario_rate() {
    local run_dir="$1"

    if [[ ! -f "${run_dir}/validation-metrics.json" ]]; then
        printf 'null\n'
        return 0
    fi

    jq -r '.scenario_coverage.scenario_rate // "null"' "${run_dir}/validation-metrics.json"
}

scenario_run_family_statuses() {
    local run_dir="$1"

    if [[ ! -f "${run_dir}/validation-metrics.json" ]]; then
        printf 'unavailable\n'
        return 0
    fi

    jq -r '
        (.scenario_coverage.families // [])
        | sort_by(.family_id)
        | map("\(.family_id)=\(.chain_validated)")
        | if length == 0 then "unavailable" else join(", ") end
    ' "${run_dir}/validation-metrics.json"
}

scenario_run_false_families() {
    local run_dir="$1"

    if [[ ! -f "${run_dir}/validation-metrics.json" ]]; then
        return 0
    fi

    jq -r '
        (.scenario_coverage.families // [])
        | map(select(.chain_validated == false).family_id)
        | .[]
    ' "${run_dir}/validation-metrics.json"
}
