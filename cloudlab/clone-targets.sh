#!/bin/bash
set -euo pipefail

# Clone evaluation targets pinned in evaluation/MANIFEST.yaml.
# Shallow clones at the pinned SHA. Idempotent: re-running on an
# already-cloned target verifies the SHA and resets if drifted.

REPO_DIR="${REPO_DIR:-/local/repository}"
MANIFEST="${REPO_DIR}/evaluation/MANIFEST.yaml"
EVAL_DIR="${REPO_DIR}/evaluation"

if [ ! -f "$MANIFEST" ]; then
    echo "error: $MANIFEST not found" >&2
    exit 1
fi

mkdir -p "$EVAL_DIR"

# Flat parse of MANIFEST.yaml — emits "name upstream sha" per target.
parse_manifest() {
    awk '
        /^  - name:/     { if (name) print name, upstream, sha; name=$3; upstream=""; sha="" }
        /^    upstream:/ { upstream=$2 }
        /^    sha:/      { sha=$2 }
        END              { if (name) print name, upstream, sha }
    ' "$MANIFEST"
}

clone_pinned() {
    local name="$1" upstream="$2" sha="$3"
    local dir="${EVAL_DIR}/${name}"

    if [ -d "${dir}/.git" ]; then
        local current
        current=$(git -C "$dir" rev-parse HEAD 2>/dev/null || echo none)
        if [ "$current" = "$sha" ]; then
            echo "[${name}] already at ${sha:0:12} — skip"
            return
        fi
        echo "[${name}] HEAD=${current:0:12}, manifest=${sha:0:12} — fetching"
        git -C "$dir" fetch --depth 1 origin "$sha"
        git -C "$dir" checkout -q --detach "$sha"
        return
    fi

    echo "[${name}] cloning ${upstream} @ ${sha:0:12}"
    git init --quiet "$dir"
    git -C "$dir" remote add origin "$upstream"
    # GitHub allows fetch-by-SHA when uploadpack.allowReachableSHA1InWant is set
    # (true for github.com); falls back to a full fetch if the server rejects.
    if ! git -C "$dir" fetch --depth 1 origin "$sha" 2>/dev/null; then
        echo "[${name}] shallow fetch-by-SHA rejected; falling back to full fetch"
        git -C "$dir" fetch origin
    fi
    git -C "$dir" checkout -q --detach "$sha"
}

parse_manifest | while read -r name upstream sha; do
    [ -z "$name" ] && continue
    clone_pinned "$name" "$upstream" "$sha"
done

echo "All evaluation targets present in ${EVAL_DIR}."
