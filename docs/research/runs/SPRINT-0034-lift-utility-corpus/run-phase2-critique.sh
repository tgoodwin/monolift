#!/usr/bin/env bash
# Phase 2a runner: invoke <model> as critic for <project>, reviewing the other two models' drafts.
#
# Usage: ./run-phase2-critique.sh <claude|codex|gemini> <caddy|gitea|listmonk|mattermost|miniflux|pocketbase>
#
# The agent reads the rubric, its own Phase 1 draft (for self-anchoring), and the two foreign drafts,
# then writes a structured critique to projects/<project>/critique-by-<model>.md.

set -uo pipefail

MODEL="${1:?usage: $0 <model> <project>}"
PROJECT="${2:?usage: $0 <model> <project>}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../../.." && pwd)"
SPRINT_ROOT="${SCRIPT_DIR}"
TEMPLATE_PATH="${SPRINT_ROOT}/prompt-phase2-critique.md"
OUTPUT_PATH="${SPRINT_ROOT}/projects/${PROJECT}/critique-by-${MODEL}.md"
LOG_DIR="/tmp/sprint-0034-logs"
PROMPT_FILE="${LOG_DIR}/${MODEL}-${PROJECT}-critique.prompt"
LOG_PATH="${LOG_DIR}/${MODEL}-${PROJECT}-critique.log"

mkdir -p "$LOG_DIR"

# Determine the two other models this critic reviews.
ALL_MODELS=(claude codex gemini)
OTHERS=()
for m in "${ALL_MODELS[@]}"; do
    if [[ "$m" != "$MODEL" ]]; then
        OTHERS+=("$m")
    fi
done

sed -e "s|\${PROJECT_NAME}|${PROJECT}|g" \
    -e "s|\${REVIEWER}|${MODEL}|g" \
    -e "s|\${OTHER1}|${OTHERS[0]}|g" \
    -e "s|\${OTHER2}|${OTHERS[1]}|g" \
    -e "s|\${OUTPUT_PATH}|${OUTPUT_PATH}|g" \
    "$TEMPLATE_PATH" > "$PROMPT_FILE"

cd "$REPO_ROOT"

echo "[$(date -u +%FT%TZ)] starting critique: ${MODEL} on ${PROJECT}" > "$LOG_PATH"

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

echo "[$(date -u +%FT%TZ)] finished critique: ${MODEL} on ${PROJECT} (exit ${EXIT_CODE})" >> "$LOG_PATH"
exit "$EXIT_CODE"
