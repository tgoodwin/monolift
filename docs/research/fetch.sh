#!/usr/bin/env bash
# fetch.sh — download items from manifest.yaml into inspiration/
#
# Usage:
#   ./fetch.sh                      # download everything
#   ./fetch.sh --batch 3            # download batch 3 only (10 workers: --batch 1..10)
#   ./fetch.sh --type pdf           # pdf | arxiv | repo | html
#   ./fetch.sh --id 001,005,042     # specific item IDs
#   ./fetch.sh --dry-run            # print commands without executing
#   ./fetch.sh --jobs 4             # parallel curl jobs (default: 4)
#
# Requires: yq (brew install yq), curl, git

set -euo pipefail

MANIFEST="$(dirname "$0")/manifest.yaml"
REPO_ROOT="$(git -C "$(dirname "$0")" rev-parse --show-toplevel)"
DEST_ROOT="$REPO_ROOT/inspiration"

BATCH=""
TYPE_FILTER=""
IDS=""
DRY_RUN=false
JOBS=4

while [[ $# -gt 0 ]]; do
  case "$1" in
    --batch)   BATCH="$2";       shift 2 ;;
    --type)    TYPE_FILTER="$2"; shift 2 ;;
    --id)      IDS="$2";         shift 2 ;;
    --dry-run) DRY_RUN=true;     shift   ;;
    --jobs)    JOBS="$2";        shift 2 ;;
    *) echo "Unknown flag: $1"; exit 1   ;;
  esac
done

command -v yq  >/dev/null || { echo "yq required: brew install yq"; exit 1; }
command -v curl >/dev/null || { echo "curl required"; exit 1; }

mkdir -p "$DEST_ROOT"/{papers,repos,html}

# Build the yq filter
YQ_FILTER=".items[]"
[[ -n "$BATCH" ]]       && YQ_FILTER+=" | select(.batch == $BATCH)"
[[ -n "$TYPE_FILTER" ]] && YQ_FILTER+=" | select(.type == \"$TYPE_FILTER\")"
if [[ -n "$IDS" ]]; then
  IDS_RE=$(echo "$IDS" | tr ',' '|')
  YQ_FILTER+=" | select(.id | test(\"^($IDS_RE)$\"))"
fi

download_pdf() {
  local id="$1" url="$2" dest="$3" title="$4"
  local full_dest="$DEST_ROOT/${dest#inspiration/}"
  mkdir -p "$(dirname "$full_dest")"
  if [[ -f "$full_dest" ]]; then
    echo "  SKIP [$id] already exists: $full_dest"
    return
  fi
  echo "  GET  [$id] $title"
  if $DRY_RUN; then
    echo "       curl -fsSL -o \"$full_dest\" \"$url\""
  else
    curl -fsSL --retry 3 --retry-delay 2 -o "$full_dest" "$url" \
      && echo "  OK   [$id] -> $full_dest" \
      || echo "  FAIL [$id] $url"
  fi
}

download_repo() {
  local id="$1" url="$2" dest="$3" title="$4"
  local full_dest="$DEST_ROOT/${dest#inspiration/}"
  if [[ -d "$full_dest" ]]; then
    echo "  SKIP [$id] already cloned: $full_dest"
    return
  fi
  echo "  CLONE [$id] $title"
  if $DRY_RUN; then
    echo "        git clone --depth=1 \"$url\" \"$full_dest\""
  else
    git clone --depth=1 "$url" "$full_dest" \
      && echo "  OK   [$id] -> $full_dest" \
      || echo "  FAIL [$id] $url"
  fi
}

download_html() {
  local id="$1" url="$2" dest="$3" title="$4"
  local full_dest="$DEST_ROOT/${dest#inspiration/}"
  mkdir -p "$(dirname "$full_dest")"
  if [[ -f "$full_dest" ]]; then
    echo "  SKIP [$id] already exists: $full_dest"
    return
  fi
  echo "  WGET [$id] $title"
  if $DRY_RUN; then
    echo "       curl -fsSL -o \"$full_dest\" \"$url\""
  else
    curl -fsSL --retry 2 -A "Mozilla/5.0" -o "$full_dest" "$url" \
      && echo "  OK   [$id] -> $full_dest" \
      || echo "  FAIL [$id] $url"
  fi
}

# Process each item — collect into temp file for xargs-based parallelism
TMPDIR_CMDS=$(mktemp -d)
trap 'rm -rf "$TMPDIR_CMDS"' EXIT

IDX=0
while IFS= read -r item; do
  id=$(echo "$item" | yq '.id')
  type=$(echo "$item" | yq '.type')
  url=$(echo "$item" | yq '.url')
  dest=$(echo "$item" | yq '.dest')
  title=$(echo "$item" | yq '.title')

  TMPFILE="$TMPDIR_CMDS/$(printf "%04d" $IDX)"
  IDX=$((IDX+1))

  case "$type" in
    pdf|arxiv) echo "download_pdf  \"$id\" \"$url\" \"$dest\" \"$title\"" > "$TMPFILE" ;;
    repo)      echo "download_repo \"$id\" \"$url\" \"$dest\" \"$title\"" > "$TMPFILE" ;;
    html)      echo "download_html \"$id\" \"$url\" \"$dest\" \"$title\"" > "$TMPFILE" ;;
  esac
done < <(yq -o=json -I0 "$YQ_FILTER" "$MANIFEST")

# Export functions and run in parallel
export -f download_pdf download_repo download_html
export DEST_ROOT DRY_RUN

echo "==> Downloading $(ls "$TMPDIR_CMDS" | wc -l | tr -d ' ') items (jobs=$JOBS) to $DEST_ROOT"
echo ""

ls "$TMPDIR_CMDS" | sort | xargs -P "$JOBS" -I{} bash "$TMPDIR_CMDS/{}" 2>&1

echo ""
echo "==> Done. Check $DEST_ROOT/"
