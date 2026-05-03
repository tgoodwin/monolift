#!/usr/bin/env bash
# Phase 3 trace critique runner: invoke <model> to critique the other two traces for a single candidate.
#
# Usage: ./run-phase3-trace-critique.sh <claude|codex|gemini> <project> <M-number>

set -uo pipefail

MODEL="${1:?usage: $0 <model> <project> <M-number>}"
PROJECT="${2:?usage: $0 <model> <project> <M-number>}"
CANDIDATE_M="${3:?usage: $0 <model> <project> <M-number>}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../../.." && pwd)"
SPRINT_ROOT="docs/research/runs/SPRINT-0034-lift-utility-corpus"
TEMPLATE_PATH="${SCRIPT_DIR}/prompt-phase3-trace-critique.md"
MANIFEST_PATH="${SCRIPT_DIR}/CANDIDATE-MANIFEST.md"

CANDIDATE_ID="${PROJECT}/${CANDIDATE_M}"
OUTPUT_PATH="${SPRINT_ROOT}/projects/${PROJECT}/traces/${CANDIDATE_M}.critique-by-${MODEL}.md"
LOG_DIR="/tmp/sprint-0034-logs"
PROMPT_FILE="${LOG_DIR}/${MODEL}-${PROJECT}-${CANDIDATE_M}-trace-critique.prompt"
LOG_PATH="${LOG_DIR}/${MODEL}-${PROJECT}-${CANDIDATE_M}-trace-critique.log"

mkdir -p "$LOG_DIR"

# Determine the two other models.
ALL_MODELS=(claude codex gemini)
OTHERS=()
for m in "${ALL_MODELS[@]}"; do
    [[ "$m" != "$MODEL" ]] && OTHERS+=("$m")
done

# Extract candidate info from manifest.
ROW=$(grep "| ${CANDIDATE_ID} |" "$MANIFEST_PATH" || true)
if [[ -z "$ROW" ]]; then
    echo "ERROR: candidate ${CANDIDATE_ID} not found in manifest" >&2
    exit 1
fi
CANDIDATE_NAME=$(echo "$ROW" | awk -F'|' '{print $3}' | sed 's/^ *//;s/ *$//')
REGION_ROOT_DISPLAY=$(echo "$ROW" | awk -F'|' '{print $4}' | grep -o '\[`[^`]*`\]' | tr -d '[]`')
REGION_ROOT_FILE=$(echo "$ROW" | awk -F'|' '{print $4}' | grep -o '(../../../../evaluation/[^)]*' | sed 's|(.*evaluation/[^/]*/||')
REGION_ROOT_LINE=$(echo "$REGION_ROOT_DISPLAY" | grep -o ':[0-9]*$' | tr -d ':')
REGION_ROOT="${REGION_ROOT_FILE}:${REGION_ROOT_LINE}"

sed -e "s|\${PROJECT_NAME}|${PROJECT}|g" \
    -e "s|\${CANDIDATE_ID}|${CANDIDATE_ID}|g" \
    -e "s|\${CANDIDATE_NAME}|${CANDIDATE_NAME}|g" \
    -e "s|\${CANDIDATE_M}|${CANDIDATE_M}|g" \
    -e "s|\${REGION_ROOT}|${REGION_ROOT}|g" \
    -e "s|\${REVIEWER}|${MODEL}|g" \
    -e "s|\${OTHER1}|${OTHERS[0]}|g" \
    -e "s|\${OTHER2}|${OTHERS[1]}|g" \
    -e "s|\${SPRINT_ROOT}|${SPRINT_ROOT}|g" \
    -e "s|\${OUTPUT_PATH}|${OUTPUT_PATH}|g" \
    "$TEMPLATE_PATH" > "$PROMPT_FILE"

cd "$REPO_ROOT"

echo "[$(date -u +%FT%TZ)] starting trace critique: ${MODEL} on ${CANDIDATE_ID}" > "$LOG_PATH"

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

echo "[$(date -u +%FT%TZ)] finished trace critique: ${MODEL} on ${CANDIDATE_ID} (exit ${EXIT_CODE})" >> "$LOG_PATH"
exit "$EXIT_CODE"
