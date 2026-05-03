#!/usr/bin/env bash
# Phase 3 runner: invoke <model> to produce an activation-path trace for a single candidate.
#
# Usage: ./run-phase3-trace.sh <claude|codex|gemini> <project> <M-number>
#
# Example: ./run-phase3-trace.sh claude miniflux M-4
#
# The candidate's name and region root are extracted from CANDIDATE-MANIFEST.md.
# Output goes to projects/<project>/traces/<M-number>.<model>.md

set -uo pipefail

MODEL="${1:?usage: $0 <model> <project> <M-number>}"
PROJECT="${2:?usage: $0 <model> <project> <M-number>}"
CANDIDATE_M="${3:?usage: $0 <model> <project> <M-number>}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../../.." && pwd)"
SPRINT_ROOT="${SCRIPT_DIR}"
TEMPLATE_PATH="${SPRINT_ROOT}/prompt-phase3-trace.md"
MANIFEST_PATH="${SPRINT_ROOT}/CANDIDATE-MANIFEST.md"

CANDIDATE_ID="${PROJECT}/${CANDIDATE_M}"
TRACES_DIR="${SPRINT_ROOT}/projects/${PROJECT}/traces"
OUTPUT_PATH="${TRACES_DIR}/${CANDIDATE_M}.${MODEL}.md"
LOG_DIR="/tmp/sprint-0034-logs"
PROMPT_FILE="${LOG_DIR}/${MODEL}-${PROJECT}-${CANDIDATE_M}-trace.prompt"
LOG_PATH="${LOG_DIR}/${MODEL}-${PROJECT}-${CANDIDATE_M}-trace.log"

mkdir -p "$TRACES_DIR" "$LOG_DIR"

# Extract candidate name from manifest (the row matching this ID).
# Table format: | caddy/M-1 | Name | [`file:line`](link) | ...
ROW=$(grep "| ${CANDIDATE_ID} |" "$MANIFEST_PATH" || true)
if [[ -z "$ROW" ]]; then
    echo "ERROR: candidate ${CANDIDATE_ID} not found in manifest" >&2
    exit 1
fi

# Extract name (2nd column) and region root file:line (from the link text in 3rd column).
CANDIDATE_NAME=$(echo "$ROW" | awk -F'|' '{print $3}' | sed 's/^ *//;s/ *$//')
# The region root link text looks like [`filename.go:NNN`](path) — extract the display text.
REGION_ROOT_DISPLAY=$(echo "$ROW" | awk -F'|' '{print $4}' | grep -o '\[`[^`]*`\]' | tr -d '[]`')

# The full region root path is relative to evaluation/<project>/.
# Extract it from the link target: (../../../../evaluation/<project>/path/to/file.go)
REGION_ROOT_FILE=$(echo "$ROW" | awk -F'|' '{print $4}' | grep -o '(../../../../evaluation/[^)]*' | sed 's/(.*evaluation\/[^/]*\///')
REGION_ROOT_LINE=$(echo "$REGION_ROOT_DISPLAY" | grep -o ':[0-9]*$' | tr -d ':')
REGION_ROOT="${REGION_ROOT_FILE}:${REGION_ROOT_LINE}"

echo "Candidate: ${CANDIDATE_ID}"
echo "Name: ${CANDIDATE_NAME}"
echo "Region root: ${REGION_ROOT}"

sed -e "s|\${PROJECT_NAME}|${PROJECT}|g" \
    -e "s|\${CANDIDATE_ID}|${CANDIDATE_ID}|g" \
    -e "s|\${CANDIDATE_NAME}|${CANDIDATE_NAME}|g" \
    -e "s|\${REGION_ROOT}|${REGION_ROOT}|g" \
    -e "s|\${OUTPUT_PATH}|${OUTPUT_PATH}|g" \
    "$TEMPLATE_PATH" > "$PROMPT_FILE"

cd "$REPO_ROOT"

echo "[$(date -u +%FT%TZ)] starting trace: ${MODEL} on ${CANDIDATE_ID}" > "$LOG_PATH"

case "$MODEL" in
    claude)
        cat "$PROMPT_FILE" | claude --print \
            --permission-mode bypassPermissions \
            --model opus \
            >> "$LOG_PATH" 2>&1
        EXIT_CODE=$?
        ;;
    codex)
        cat "$PROMPT_FILE" | codex exec \
            --ignore-user-config \
            --full-auto \
            --skip-git-repo-check \
            >> "$LOG_PATH" 2>&1
        EXIT_CODE=$?
        ;;
    gemini)
        cat "$PROMPT_FILE" | gemini \
            --approval-mode yolo \
            --prompt "Read the instructions in the message that follows and follow them precisely. Use your file-write tool to write your final markdown to the output path mentioned in the instructions." \
            >> "$LOG_PATH" 2>&1
        EXIT_CODE=$?
        ;;
    *)
        echo "Unknown model: $MODEL" >&2
        exit 1
        ;;
esac

echo "[$(date -u +%FT%TZ)] finished trace: ${MODEL} on ${CANDIDATE_ID} (exit ${EXIT_CODE})" >> "$LOG_PATH"
exit "$EXIT_CODE"
