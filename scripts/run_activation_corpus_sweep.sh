#!/usr/bin/env bash
#
# run_activation_corpus_sweep.sh — Best-effort corpus trace sweep runner
#
# Reads test/e2e/activation_corpus_traces.yaml, runs one trace per subprocess,
# applies per-trace timeout, writes JSONL results, produces Markdown summary.
# Continues after admission refusal, compile failure, e2e failure, or timeout.
#
# Usage:
#   scripts/run_activation_corpus_sweep.sh [OPTIONS]
#
# Options:
#   --admission-only     Run AdmitCut/AdmitPlan only (no Kind cluster needed)
#   --phases PHASES      Comma-separated phase filter (e.g., "1,3" or "all")
#   --timeout-per-trace  Per-trace timeout (default: 25m)
#   --manifest PATH      Path to manifest YAML (default: test/e2e/activation_corpus_traces.yaml)
#   --output-dir DIR     Output directory for results (default: .moab/runs/<timestamp>)
#
# Trace result statuses:
#   pass            — focused Kind e2e passes
#   admission-skip  — AdmitCut/AdmitPlan refused the trace
#   build-skip      — patched package or extracted service failed to compile
#   e2e-fail        — Kind e2e failed (deploy, workload, oracle, or transcript mismatch)
#   timeout-skip    — trace exceeded per-trace timeout
#   manifest-skip   — trace skipped by manifest annotation (not attempted)
#   infra-fail      — infrastructure failure (cluster, docker, network)

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPO_ROOT"

# ── Defaults ──────────────────────────────────────────────────────────────
MANIFEST="test/e2e/activation_corpus_traces.yaml"
ADMISSION_ONLY=false
PHASES="all"
TIMEOUT_PER_TRACE="25m"
OUTPUT_DIR=""

# ── Parse arguments ───────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
    case "$1" in
        --admission-only)
            ADMISSION_ONLY=true
            shift
            ;;
        --phases)
            PHASES="$2"
            shift 2
            ;;
        --timeout-per-trace)
            TIMEOUT_PER_TRACE="$2"
            shift 2
            ;;
        --manifest)
            MANIFEST="$2"
            shift 2
            ;;
        --output-dir)
            OUTPUT_DIR="$2"
            shift 2
            ;;
        *)
            echo "Unknown option: $1" >&2
            exit 1
            ;;
    esac
done

if [[ -z "$OUTPUT_DIR" ]]; then
    OUTPUT_DIR=".moab/runs/$(date +%s)"
fi
mkdir -p "$OUTPUT_DIR"

RESULTS_JSONL="$OUTPUT_DIR/results.jsonl"
SUMMARY_MD="$OUTPUT_DIR/summary.md"

# ── Ensure yq is available (for YAML parsing) ────────────────────────────
if ! command -v yq &>/dev/null; then
    echo "ERROR: yq is required but not installed. Install via: brew install yq" >&2
    exit 1
fi

# ── Read manifest ─────────────────────────────────────────────────────────
TRACE_COUNT=$(yq '.traces | length' "$MANIFEST")
echo "=== Corpus Sweep: $TRACE_COUNT traces, admission_only=$ADMISSION_ONLY, phases=$PHASES, timeout=$TIMEOUT_PER_TRACE ==="
echo "=== Output: $OUTPUT_DIR ==="
echo ""

# ── Counters ──────────────────────────────────────────────────────────────
declare -A STATUS_COUNTS
for status in pass admission-skip build-skip e2e-fail timeout-skip manifest-skip infra-fail; do
    STATUS_COUNTS[$status]=0
done

STARTED_AT=$(date +%s)

# ── Per-trace loop ────────────────────────────────────────────────────────
for i in $(seq 0 $((TRACE_COUNT - 1))); do
    TRACE_ID=$(yq ".traces[$i].trace_id" "$MANIFEST")
    PROJECT=$(yq ".traces[$i].project" "$MANIFEST")
    FUNCTION=$(yq ".traces[$i].function" "$MANIFEST")
    FILE_LINE=$(yq ".traces[$i].file_line" "$MANIFEST")
    STATUS=$(yq ".traces[$i].status" "$MANIFEST")
    PHASE=$(yq ".traces[$i].phase" "$MANIFEST")
    SKIP_REASON=$(yq ".traces[$i].skip_reason" "$MANIFEST")
    E2E_PKG=$(yq ".traces[$i].e2e_package" "$MANIFEST")

    # ── Phase filter ──────────────────────────────────────────────────
    if [[ "$PHASES" != "all" ]]; then
        MATCH=false
        IFS=',' read -ra PHASE_LIST <<< "$PHASES"
        for p in "${PHASE_LIST[@]}"; do
            if [[ "$PHASE" == "$p" ]]; then
                MATCH=true
                break
            fi
        done
        if [[ "$MATCH" == "false" ]]; then
            continue
        fi
    fi

    # ── Manifest-skip ─────────────────────────────────────────────────
    if [[ "$STATUS" == "manifest-skip" && -n "$SKIP_REASON" ]]; then
        TRACE_START=$(date +%s)
        echo "[$((i+1))/$TRACE_COUNT] $TRACE_ID: manifest-skip ($SKIP_REASON)"
        TRACE_END=$(date +%s)
        DURATION=$((TRACE_END - TRACE_START))
        echo "{\"trace_id\":\"$TRACE_ID\",\"project\":\"$PROJECT\",\"function\":\"$FUNCTION\",\"status\":\"manifest-skip\",\"stage\":\"manifest\",\"duration_s\":$DURATION,\"error\":\"$SKIP_REASON\"}" >> "$RESULTS_JSONL"
        STATUS_COUNTS[manifest-skip]=$((${STATUS_COUNTS[manifest-skip]} + 1))
        continue
    fi

    # ── Already passing ───────────────────────────────────────────────
    if [[ "$STATUS" == "pass" ]]; then
        echo "[$((i+1))/$TRACE_COUNT] $TRACE_ID: pass (already verified)"
        echo "{\"trace_id\":\"$TRACE_ID\",\"project\":\"$PROJECT\",\"function\":\"$FUNCTION\",\"status\":\"pass\",\"stage\":\"verified\",\"duration_s\":0,\"error\":\"\"}" >> "$RESULTS_JSONL"
        STATUS_COUNTS[pass]=$((${STATUS_COUNTS[pass]} + 1))
        continue
    fi

    # ── Admission-only mode ───────────────────────────────────────────
    if [[ "$ADMISSION_ONLY" == "true" ]]; then
        echo "[$((i+1))/$TRACE_COUNT] $TRACE_ID: running admission check..."
        TRACE_START=$(date +%s)

        ADMIT_OUTPUT=""
        ADMIT_STATUS="admission-skip"
        ADMIT_ERROR=""

        if [[ "$FILE_LINE" == "N/A" || "$FILE_LINE" == *"FieldNotInCorpus"* ]]; then
            ADMIT_ERROR="no file:line available for admission"
            ADMIT_STATUS="manifest-skip"
        else
            # Run admission check via go test with a short timeout
            set +e
            ADMIT_OUTPUT=$(timeout 120 go test ./pkg/codegen/... -run "TestAdmission" -v \
                -args -trace-target="$FILE_LINE" -source-dir="evaluation/$PROJECT" 2>&1)
            EXIT_CODE=$?
            set -e

            if [[ $EXIT_CODE -eq 0 ]]; then
                ADMIT_STATUS="pass"
                ADMIT_ERROR=""
            elif [[ $EXIT_CODE -eq 124 ]]; then
                ADMIT_STATUS="timeout-skip"
                ADMIT_ERROR="admission check timed out after 120s"
            else
                ADMIT_STATUS="admission-skip"
                # Extract refusal reason from output
                ADMIT_ERROR=$(echo "$ADMIT_OUTPUT" | grep -oP 'refusal:\s*\K.*' | head -1)
                if [[ -z "$ADMIT_ERROR" ]]; then
                    ADMIT_ERROR="admission refused (exit code $EXIT_CODE)"
                fi
            fi
        fi

        TRACE_END=$(date +%s)
        DURATION=$((TRACE_END - TRACE_START))
        echo "  → $ADMIT_STATUS ($DURATION s): $ADMIT_ERROR"
        echo "{\"trace_id\":\"$TRACE_ID\",\"project\":\"$PROJECT\",\"function\":\"$FUNCTION\",\"status\":\"$ADMIT_STATUS\",\"stage\":\"admission\",\"duration_s\":$DURATION,\"error\":\"$ADMIT_ERROR\"}" >> "$RESULTS_JSONL"
        STATUS_COUNTS[$ADMIT_STATUS]=$((${STATUS_COUNTS[$ADMIT_STATUS]} + 1))
        continue
    fi

    # ── Full e2e run ──────────────────────────────────────────────────
    if [[ -z "$E2E_PKG" ]]; then
        echo "[$((i+1))/$TRACE_COUNT] $TRACE_ID: manifest-skip (no e2e package)"
        echo "{\"trace_id\":\"$TRACE_ID\",\"project\":\"$PROJECT\",\"function\":\"$FUNCTION\",\"status\":\"manifest-skip\",\"stage\":\"no-e2e-package\",\"duration_s\":0,\"error\":\"no e2e package implemented\"}" >> "$RESULTS_JSONL"
        STATUS_COUNTS[manifest-skip]=$((${STATUS_COUNTS[manifest-skip]} + 1))
        continue
    fi

    echo "[$((i+1))/$TRACE_COUNT] $TRACE_ID: running e2e ($E2E_PKG)..."
    TRACE_START=$(date +%s)

    TRACE_STATUS="e2e-fail"
    TRACE_ERROR=""
    TRACE_STAGE="e2e"

    set +e
    E2E_OUTPUT=$(timeout "$TIMEOUT_PER_TRACE" bash -c "
        MONOLIFT_E2E=1 go test -tags=e2e -v -run 'TestE2E/$E2E_PKG' ./test/e2e/... -timeout=30m 2>&1
    ")
    EXIT_CODE=$?
    set -e

    if [[ $EXIT_CODE -eq 0 ]]; then
        TRACE_STATUS="pass"
        TRACE_ERROR=""
    elif [[ $EXIT_CODE -eq 124 ]]; then
        TRACE_STATUS="timeout-skip"
        TRACE_ERROR="trace exceeded ${TIMEOUT_PER_TRACE} timeout"
        TRACE_STAGE="timeout"
    else
        # Try to extract the failing stage
        FAILING_STAGE=$(echo "$E2E_OUTPUT" | grep -oP 'stage\[\K[0-9]+' | tail -1)
        if [[ -n "$FAILING_STAGE" ]]; then
            TRACE_STAGE="stage-$FAILING_STAGE"
        fi

        # Check for specific failure types
        if echo "$E2E_OUTPUT" | grep -q "admission.*refus\|AdmitCut.*refused\|AdmitPlan.*refused"; then
            TRACE_STATUS="admission-skip"
            TRACE_STAGE="admission"
        elif echo "$E2E_OUTPUT" | grep -q "compile.*fail\|build.*fail\|patched.*fail"; then
            TRACE_STATUS="build-skip"
            TRACE_STAGE="compile"
        elif echo "$E2E_OUTPUT" | grep -q "cluster ensure failed\|docker.*fail\|kind.*fail"; then
            TRACE_STATUS="infra-fail"
            TRACE_STAGE="infra"
        fi

        TRACE_ERROR=$(echo "$E2E_OUTPUT" | grep -m1 'FAIL\|Fatal\|Error' | head -c 200)
        if [[ -z "$TRACE_ERROR" ]]; then
            TRACE_ERROR="exit code $EXIT_CODE"
        fi
    fi

    TRACE_END=$(date +%s)
    DURATION=$((TRACE_END - TRACE_START))

    echo "  → $TRACE_STATUS ($DURATION s, $TRACE_STAGE): $TRACE_ERROR"

    # Escape quotes in error for JSON
    TRACE_ERROR_ESCAPED=$(echo "$TRACE_ERROR" | sed 's/"/\\"/g' | tr '\n' ' ')
    echo "{\"trace_id\":\"$TRACE_ID\",\"project\":\"$PROJECT\",\"function\":\"$FUNCTION\",\"status\":\"$TRACE_STATUS\",\"stage\":\"$TRACE_STAGE\",\"duration_s\":$DURATION,\"error\":\"$TRACE_ERROR_ESCAPED\"}" >> "$RESULTS_JSONL"
    STATUS_COUNTS[$TRACE_STATUS]=$((${STATUS_COUNTS[$TRACE_STATUS]} + 1))

    # Save per-trace log
    echo "$E2E_OUTPUT" > "$OUTPUT_DIR/${TRACE_ID//\//_}.log"
done

ENDED_AT=$(date +%s)
TOTAL_DURATION=$((ENDED_AT - STARTED_AT))

# ── Summary table ─────────────────────────────────────────────────────────
echo ""
echo "=== Sweep Complete ($TOTAL_DURATION s) ==="
echo ""

cat > "$SUMMARY_MD" << HEADER
# Corpus Sweep Summary

**Date:** $(date -u +"%Y-%m-%dT%H:%M:%SZ")
**Mode:** $(if [[ "$ADMISSION_ONLY" == "true" ]]; then echo "admission-only"; else echo "full e2e"; fi)
**Phases:** $PHASES
**Per-trace timeout:** $TIMEOUT_PER_TRACE
**Total duration:** ${TOTAL_DURATION}s

## Status Counts

| Status | Count |
|---|---:|
HEADER

for status in pass admission-skip build-skip e2e-fail timeout-skip manifest-skip infra-fail; do
    COUNT=${STATUS_COUNTS[$status]}
    echo "| $status | $COUNT |" >> "$SUMMARY_MD"
    echo "  $status: $COUNT"
done

echo "" >> "$SUMMARY_MD"
echo "## Per-Trace Results" >> "$SUMMARY_MD"
echo "" >> "$SUMMARY_MD"
echo "| Trace ID | Function | Status | Stage | Duration | Error |" >> "$SUMMARY_MD"
echo "|---|---|---|---|---:|---|" >> "$SUMMARY_MD"

if [[ -f "$RESULTS_JSONL" ]]; then
    while IFS= read -r line; do
        TID=$(echo "$line" | jq -r '.trace_id')
        FUNC=$(echo "$line" | jq -r '.function')
        STAT=$(echo "$line" | jq -r '.status')
        STG=$(echo "$line" | jq -r '.stage')
        DUR=$(echo "$line" | jq -r '.duration_s')
        ERR=$(echo "$line" | jq -r '.error' | head -c 80)
        echo "| \`$TID\` | \`$FUNC\` | $STAT | $STG | ${DUR}s | $ERR |" >> "$SUMMARY_MD"
    done < "$RESULTS_JSONL"
fi

echo ""
echo "Results: $RESULTS_JSONL"
echo "Summary: $SUMMARY_MD"
