#!/usr/bin/env bash
# Phase 1 runner: invoke <model> on <project> and write the candidate draft to projects/<project>/<model>.md
#
# Usage: ./run-phase1.sh <claude|codex|gemini> <caddy|gitea|listmonk|mattermost|miniflux|pocketbase>
#
# The agent is responsible for writing its markdown response to the output path
# specified inside the prompt (via its file-write tool). CLI stdout/stderr is
# captured to /tmp/sprint-0034-logs/ for debugging.

set -uo pipefail

MODEL="${1:?usage: $0 <model> <project>}"
PROJECT="${2:?usage: $0 <model> <project>}"

# Derive repo root from this script's location so the runner is portable across machines.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../../.." && pwd)"
PROJECT_PATH="${REPO_ROOT}/evaluation/${PROJECT}"
SPRINT_ROOT="${SCRIPT_DIR}"
OUTPUT_PATH="${SPRINT_ROOT}/projects/${PROJECT}/${MODEL}.md"
TEMPLATE_PATH="${SPRINT_ROOT}/prompt-template.md"
LOG_DIR="/tmp/sprint-0034-logs"
PROMPT_FILE="${LOG_DIR}/${MODEL}-${PROJECT}.prompt"
LOG_PATH="${LOG_DIR}/${MODEL}-${PROJECT}.log"

mkdir -p "$LOG_DIR"

# Substitute project-specific variables into the prompt template.
sed -e "s|\${PROJECT_NAME}|${PROJECT}|g" \
    -e "s|\${PROJECT_PATH}|${PROJECT_PATH}|g" \
    -e "s|\${OUTPUT_PATH}|${OUTPUT_PATH}|g" \
    "$TEMPLATE_PATH" > "$PROMPT_FILE"

cd "$REPO_ROOT"

echo "[$(date -u +%FT%TZ)] starting ${MODEL} on ${PROJECT}" > "$LOG_PATH"

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
        # gemini appends stdin to --prompt; use a tiny --prompt and the full prompt via stdin.
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

echo "[$(date -u +%FT%TZ)] finished ${MODEL} on ${PROJECT} (exit ${EXIT_CODE})" >> "$LOG_PATH"
exit "$EXIT_CODE"
