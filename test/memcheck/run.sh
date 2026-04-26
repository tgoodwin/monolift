#!/usr/bin/env bash
set -euo pipefail

label="${MEMCHECK_LABEL:-}"
output="${MEMCHECK_OUTPUT:-}"
baseline="${MEMCHECK_BASELINE:-}"
target_reduction_pct="${MEMCHECK_TARGET_REDUCTION_PCT:-0}"
absolute_peak_limit_mb="${MEMCHECK_ABSOLUTE_PEAK_LIMIT_MB:-0}"
stability_limit_pct="${MEMCHECK_STABILITY_LIMIT_PCT:-10}"
record_only="${MEMCHECK_RECORD_ONLY:-0}"
rss_limit_mb="${MEMCHECK_RSS_LIMIT_MB:-4096}"
wall_limit_sec="${MEMCHECK_WALL_LIMIT_SEC:-900}"
sample_ms="${MEMCHECK_POLL_INTERVAL_MS:-250}"
env_vars=()
cmd=()

while (($#)); do
  case "$1" in
    --label)
      label="$2"
      shift 2
      ;;
    --output)
      output="$2"
      shift 2
      ;;
    --baseline)
      baseline="$2"
      shift 2
      ;;
    --target-reduction-pct)
      target_reduction_pct="$2"
      shift 2
      ;;
    --absolute-peak-limit-mb)
      absolute_peak_limit_mb="$2"
      shift 2
      ;;
    --stability-limit-pct)
      stability_limit_pct="$2"
      shift 2
      ;;
    --rss-limit-mb)
      rss_limit_mb="$2"
      shift 2
      ;;
    --wall-limit-sec)
      wall_limit_sec="$2"
      shift 2
      ;;
    --sample-ms)
      sample_ms="$2"
      shift 2
      ;;
    --record-only)
      record_only="1"
      shift
      ;;
    --env)
      env_vars+=("$2")
      shift 2
      ;;
    --)
      shift
      cmd=("$@")
      break
      ;;
    *)
      echo "unknown argument: $1" >&2
      exit 1
      ;;
  esac
done

if [[ -z "$label" ]]; then
  echo "--label is required" >&2
  exit 1
fi
if [[ -z "$output" ]]; then
  echo "--output is required" >&2
  exit 1
fi
if [[ ${#cmd[@]} -eq 0 ]]; then
  echo "measured command is required after --" >&2
  exit 1
fi

IFS=',' read -r -a seeds <<< "${MEMCHECK_SEEDS:-101,202,303}"
tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

run_files=()
for seed in "${seeds[@]}"; do
  go clean -cache -testcache

  seeded_cmd=()
  for arg in "${cmd[@]}"; do
    seeded_cmd+=("${arg//__SEED__/${seed}}")
  done

  run_file="${tmpdir}/run-${seed}.json"
  env "${env_vars[@]}" \
    go run ./cmd/memcheck run \
      -label "$label" \
      -output "$run_file" \
      -sample-ms "$sample_ms" \
      -rss-limit-mb "$rss_limit_mb" \
      -wall-limit-sec "$wall_limit_sec" \
      -seed "$seed" \
      -- "${seeded_cmd[@]}"
  run_files+=("$run_file")
done

aggregate_args=(
  aggregate
  -label "$label"
  -output "$output"
)
if [[ -n "$baseline" ]]; then
  aggregate_args+=(-baseline "$baseline")
fi
if [[ "$record_only" == "1" ]]; then
  aggregate_args+=(-record-only)
else
  aggregate_args+=(-target-reduction-pct "$target_reduction_pct")
fi
if [[ "$absolute_peak_limit_mb" != "0" ]]; then
  aggregate_args+=(-absolute-peak-limit-mb "$absolute_peak_limit_mb")
fi
aggregate_args+=(-stability-limit-pct "$stability_limit_pct")

go run ./cmd/memcheck "${aggregate_args[@]}" -- "${run_files[@]}"
