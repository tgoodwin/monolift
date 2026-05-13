#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_MD="${1:-$ROOT/docs/research/runs/SPRINT-0046-baseline.md}"
PROFILE_DIR="${2:-${OUT_MD%.md}-profiles}"
BIN="$(mktemp "${TMPDIR:-/tmp}/monolift-activation-path.XXXXXX")"
RUNS_TSV="$(mktemp "${TMPDIR:-/tmp}/monolift-benchmark-runs.XXXXXX")"

if [[ "$OUT_MD" != /* ]]; then
  OUT_MD="$ROOT/$OUT_MD"
fi
if [[ "$PROFILE_DIR" != /* ]]; then
  PROFILE_DIR="$ROOT/$PROFILE_DIR"
fi

if [[ -z "${GOCACHE:-}" || ! -w "${GOCACHE:-/}" ]]; then
  export GOCACHE="/tmp/monolift-go-cache"
fi
mkdir -p "$GOCACHE"

cleanup() {
  rm -f "$BIN" "$RUNS_TSV"
}
trap cleanup EXIT

mkdir -p "$(dirname "$OUT_MD")" "$PROFILE_DIR"

(
  cd "$ROOT"
  go build -o "$BIN" ./cmd/activation-path
)

{
  printf 'project\tdir\ttarget\tstatus\tprofile\tlog\n'
} > "$RUNS_TSV"

run_target() {
  local project="$1"
  local dir="$2"
  local target="$3"
  local timeout="$4"
  local profile="$PROFILE_DIR/$project.json"
  local log="$PROFILE_DIR/$project.log"
  local gowork=""

  if [[ "$project" == "mattermost" ]]; then
    gowork="$PROFILE_DIR/mattermost.go.work"
    printf 'go 1.25.8\n\nuse (\n\t%s\n\t%s\n)\n' \
      "$ROOT/evaluation/mattermost/server" \
      "$ROOT/evaluation/mattermost/server/public" > "$gowork"
  fi

  printf 'benchmarking %s...\n' "$project" >&2
  set +e
  (
    cd "$ROOT/$dir" || exit 1
    if [[ -n "$gowork" ]]; then
      export GOWORK="$gowork"
    fi
    "$BIN" \
      --packages ./... \
      --target "$target" \
      --timeout "$timeout" \
      --augmentations all \
      --reverse-import-scope \
      --profile \
      --profile-output "$profile" \
      --verbose \
      --format text
  ) >"$log" 2>&1
  local status=$?
  set -e
  printf '%s\t%s\t%s\t%s\t%s\t%s\n' "$project" "$dir" "$target" "$status" "$profile" "$log" >> "$RUNS_TSV"
}

run_target "caddy" "evaluation/caddy" "modules/caddyhttp/caddyhttp.go:279" "10m"
run_target "miniflux" "evaluation/miniflux" "internal/reader/sanitizer/sanitizer.go:217" "10m"
run_target "gitea" "evaluation/gitea" "modules/util/url.go:12" "20m"
run_target "listmonk" "evaluation/listmonk" "internal/utils/utils.go:41" "10m"
run_target "pocketbase" "evaluation/pocketbase" "tools/inflector/inflector.go:24" "10m"
run_target "mattermost" "evaluation/mattermost/server" "channels/app/file.go:588" "30m"

GO_VERSION="$(go version 2>/dev/null || true)"
GO_OS_ARCH="$(go env GOOS GOARCH 2>/dev/null | tr '\n' '/' | sed 's:/$::')"
GOCACHE="$(go env GOCACHE 2>/dev/null || true)"
GOMODCACHE="$(go env GOMODCACHE 2>/dev/null || true)"
DOCKER_VERSION="$(docker version --format '{{.Server.Version}}' 2>/dev/null || docker --version 2>/dev/null || true)"
if [[ -n "$GOMODCACHE" && -d "$GOMODCACHE" ]]; then
  MODULE_CACHE_WARMTH="$(find "$GOMODCACHE" -mindepth 1 -maxdepth 1 2>/dev/null | wc -l | tr -d ' ') top-level entries"
else
  MODULE_CACHE_WARMTH="module cache unavailable"
fi
if [[ -n "$GOCACHE" && -d "$GOCACHE" ]]; then
  BUILD_CACHE_STATE="present"
else
  BUILD_CACHE_STATE="missing"
fi

python3 - "$OUT_MD" "$RUNS_TSV" "$GO_VERSION" "$GO_OS_ARCH" "$DOCKER_VERSION" "$GOCACHE" "$GOMODCACHE" "$BUILD_CACHE_STATE" "$MODULE_CACHE_WARMTH" <<'PY'
import csv
import json
import sys
from pathlib import Path

out_md = Path(sys.argv[1])
runs_tsv = Path(sys.argv[2])
go_version = sys.argv[3]
go_os_arch = sys.argv[4]
docker_version = sys.argv[5]
gocache = sys.argv[6]
gomodcache = sys.argv[7]
build_cache_state = sys.argv[8]
module_cache_warmth = sys.argv[9]

def duration_ns(profile, phase):
    for timing in profile.get("phase_timings", []):
        if timing.get("phase") == phase:
            return int(timing.get("duration", 0))
    return 0

def fmt_duration(ns):
    if ns <= 0:
        return "-"
    seconds = ns / 1_000_000_000
    if seconds >= 60:
        return f"{seconds / 60:.2f}m"
    if seconds >= 1:
        return f"{seconds:.2f}s"
    return f"{seconds * 1000:.1f}ms"

def augment_iteration_rows(profile):
    phases = [
        "AugmentStructField",
        "ApplyPredicates",
        "AugmentGoroutine",
        "AugmentPackageVars",
        "AugmentFuncArgs",
        "AugmentMapFuncValues",
        "AugmentInterfaceFields",
        "ExploreCallees",
    ]
    rows = []
    current = {"iteration": 0, "total": 0, **{phase: 0 for phase in phases}}
    for timing in profile.get("augment_sub_timings", []):
        phase = timing.get("phase", "")
        duration = int(timing.get("duration", 0))
        if phase in current:
            current[phase] += duration
        current["total"] += duration
        if phase == "ExploreCallees":
            rows.append(current)
            current = {"iteration": current["iteration"] + 1, "total": 0, **{phase: 0 for phase in phases}}
    if current["total"] > 0:
        rows.append(current)
    return rows

rows = []
with runs_tsv.open(newline="") as fh:
    for row in csv.DictReader(fh, delimiter="\t"):
        profile_path = Path(row["profile"])
        profile = {}
        if profile_path.exists() and profile_path.stat().st_size > 0:
            try:
                profile = json.loads(profile_path.read_text())
            except json.JSONDecodeError:
                profile = {}
        phases = profile.get("phase_timings", [])
        total_ns = sum(int(t.get("duration", 0)) for t in phases)
        stats = profile.get("stats", {})
        rows.append({
            "project": row["project"],
            "target": row["target"],
            "status": row["status"],
            "found": str(profile.get("found", False)).lower() if profile else "no-profile",
            "total": fmt_duration(total_ns),
            "scope": fmt_duration(duration_ns(profile, "scope")),
            "load": fmt_duration(duration_ns(profile, "load")),
            "ssa": fmt_duration(duration_ns(profile, "ssa")),
            "rta": fmt_duration(duration_ns(profile, "rta")),
            "augment": fmt_duration(duration_ns(profile, "augment")),
            "bfs": fmt_duration(duration_ns(profile, "bfs")),
            "nodes": stats.get("nodes", "-"),
            "edges": stats.get("edges", "-"),
            "iterations": stats.get("augment_iterations", "-"),
            "path_length": profile.get("path_length", "-"),
            "cut_step": profile.get("recommended_cut_step", "-"),
            "profile": profile_path,
            "log": Path(row["log"]),
        })

lines = [
    "# SPRINT-0046 Baseline Benchmark",
    "",
    "## Machine And Cache State",
    "",
    f"- Go: `{go_version or 'unknown'}`",
    f"- OS/arch: `{go_os_arch or 'unknown'}`",
    f"- Docker: `{docker_version or 'unknown'}`",
    f"- GOCACHE: `{gocache or 'unknown'}` ({build_cache_state})",
    f"- GOMODCACHE: `{gomodcache or 'unknown'}` ({module_cache_warmth})",
    "",
    "## Summary",
    "",
    "| Project | Status | Found | Total | Scope | Load | SSA | RTA | Augment | BFS | Nodes | Edges | Iter | Path | Cut | Profile |",
    "|---|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|",
]
for row in rows:
    profile_rel = row["profile"]
    try:
        profile_rel = profile_rel.relative_to(out_md.parent)
    except ValueError:
        pass
    lines.append(
        f"| {row['project']} | {row['status']} | {row['found']} | {row['total']} | {row['scope']} | "
        f"{row['load']} | {row['ssa']} | {row['rta']} | {row['augment']} | {row['bfs']} | "
        f"{row['nodes']} | {row['edges']} | {row['iterations']} | {row['path_length']} | {row['cut_step']} | `{profile_rel}` |"
    )
lines.extend([
    "",
    "## Targets",
    "",
    "| Project | Target | Log |",
    "|---|---|---|",
])
for row in rows:
    log_rel = row["log"]
    try:
        log_rel = log_rel.relative_to(out_md.parent)
    except ValueError:
        pass
    lines.append(f"| {row['project']} | `{row['target']}` | `{log_rel}` |")

mattermost_profile = None
for row in rows:
    if row["project"] == "mattermost" and row["profile"].exists():
        try:
            mattermost_profile = json.loads(row["profile"].read_text())
        except json.JSONDecodeError:
            mattermost_profile = None
        break
if mattermost_profile:
    lines.extend([
        "",
        "## Mattermost Augment Iterations",
        "",
        "| Iter | Struct field | Predicates | Goroutine | Package vars | Func args | Map funcs | Interface fields | Explore callees | Total |",
        "|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|",
    ])
    for row in augment_iteration_rows(mattermost_profile):
        lines.append(
            f"| {row['iteration']} | {fmt_duration(row['AugmentStructField'])} | "
            f"{fmt_duration(row['ApplyPredicates'])} | {fmt_duration(row['AugmentGoroutine'])} | "
            f"{fmt_duration(row['AugmentPackageVars'])} | {fmt_duration(row['AugmentFuncArgs'])} | "
            f"{fmt_duration(row['AugmentMapFuncValues'])} | {fmt_duration(row['AugmentInterfaceFields'])} | "
            f"{fmt_duration(row['ExploreCallees'])} | {fmt_duration(row['total'])} |"
        )

out_md.write_text("\n".join(lines) + "\n")
PY

printf 'wrote %s\n' "$OUT_MD" >&2
