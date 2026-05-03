#!/usr/bin/env bash
# Clone (or sync) every evaluation target listed in evaluation/MANIFEST.yaml
# at its pinned SHA, into evaluation/<name>/.
#
# Idempotent:
#   - Skips a target if its working tree is already checked out at the pinned SHA.
#   - Otherwise fetches the SHA into the existing clone and checks it out.
#   - Does a fresh shallow clone if the target dir does not exist.
#
# Usage:  ./clone-evaluation-targets.sh [name ...]
#   With no args, processes every target in the manifest.
#   With one or more names, processes only the named targets.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../../.." && pwd)"
MANIFEST="${REPO_ROOT}/evaluation/MANIFEST.yaml"
EVAL_DIR="${REPO_ROOT}/evaluation"

if [[ ! -f "${MANIFEST}" ]]; then
    echo "ERROR: manifest not found at ${MANIFEST}" >&2
    exit 1
fi

# Parse the manifest with a tiny awk script. The format is:
#   - name: foo
#     upstream: https://...
#     sha: <40-hex>
#     go_files: <int>
parse_manifest() {
    awk '
        /^[[:space:]]*-[[:space:]]*name:/   { name=$3; next }
        /^[[:space:]]*upstream:/             { upstream=$2; next }
        /^[[:space:]]*sha:/                  { sha=$2 }
        /^[[:space:]]*go_files:/             { print name "\t" upstream "\t" sha; name=""; upstream=""; sha="" }
    ' "${MANIFEST}"
}

filter='*'
if [[ $# -gt 0 ]]; then
    filter=$(printf '|%s' "$@")
    filter="^(${filter:1})\$"
fi

while IFS=$'\t' read -r name upstream sha; do
    if [[ "${filter}" != '*' ]] && ! [[ "${name}" =~ ${filter} ]]; then
        continue
    fi

    target_dir="${EVAL_DIR}/${name}"
    echo "==> ${name} (pinned ${sha})"

    if [[ -d "${target_dir}/.git" ]]; then
        current=$(git -C "${target_dir}" rev-parse HEAD 2>/dev/null || echo "")
        if [[ "${current}" == "${sha}" ]]; then
            echo "    already at pinned SHA, skipping"
            continue
        fi
        echo "    syncing existing clone (currently ${current:0:12})"
        # Fetch the specific SHA. Some servers (GitHub) reject fetch-by-sha
        # without uploadpack.allowReachableSHA1InWant; if that happens, fall
        # back to a shallow refetch of the default branch.
        if ! git -C "${target_dir}" fetch --depth 1 origin "${sha}" 2>/dev/null; then
            echo "    fetch-by-SHA refused; deepening from default branch"
            git -C "${target_dir}" fetch --unshallow origin 2>/dev/null || \
                git -C "${target_dir}" fetch origin
        fi
        git -C "${target_dir}" checkout --detach "${sha}"
    else
        echo "    fresh clone from ${upstream}"
        # Shallow clone of the default branch first; then deepen until the SHA is reachable.
        git clone --filter=blob:none "${upstream}" "${target_dir}"
        if ! git -C "${target_dir}" cat-file -e "${sha}^{commit}" 2>/dev/null; then
            git -C "${target_dir}" fetch origin "${sha}" || \
                git -C "${target_dir}" fetch --unshallow origin
        fi
        git -C "${target_dir}" checkout --detach "${sha}"
    fi
done < <(parse_manifest)

echo
echo "Done. Verify with:"
echo "  for d in ${EVAL_DIR}/*/; do printf '%-12s %s\\n' \"\$(basename \$d)\" \"\$(git -C \"\$d\" rev-parse HEAD)\"; done"
