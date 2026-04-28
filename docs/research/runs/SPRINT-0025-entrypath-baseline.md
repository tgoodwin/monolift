# SPRINT-0025 EntryPath Baseline

Date: 2026-04-27

## Purpose

This note preserves the full Mattermost EntryPath diagnostic baseline before
SPRINT-0025 search experiments. The run intentionally uses the same target,
region roots, and required Mattermost workspace as the SPRINT-0024 diagnostic.

## Host

- OS: macOS 14.7.6 (Darwin 23.6.0, arm64)
- Hardware: MacBook Pro (MacBookPro18,3), Apple M1 Pro, 10 cores, 16 GB RAM
- Go: `go version go1.25.4 darwin/arm64`
- Shell timestamp captured: `2026-04-27 14:20:17 PDT`

## Required Workspace

The Mattermost probe must run with:

```sh
GOWORK=/Users/tgoodwin/projects/monolift/.tmp/sprint-0021-a1-go.work
```

Without this workspace, package loading may resolve `server/public` from the
module cache instead of the local evaluation checkout.

## Region Roots

- `(*Hub).Start`
- `(*WebConn).Pump`

## Baseline Command

Build:

```sh
go build -o /tmp/monolift-sprint-0025-entrypath-probe ./cmd/entrypath-probe
```

Full Mattermost diagnostic run:

```sh
/usr/bin/time -l timeout 300 env GOWORK=/Users/tgoodwin/projects/monolift/.tmp/sprint-0021-a1-go.work \
  /tmp/monolift-sprint-0025-entrypath-probe \
    --diagnostic-timings \
    --region-root '(*Hub).Start' \
    --region-root '(*WebConn).Pump' \
    /Users/tgoodwin/projects/monolift/evaluation/mattermost/server/cmd/mattermost \
    > docs/research/runs/SPRINT-0025-entrypath-baseline.json \
    2> docs/research/runs/SPRINT-0025-entrypath-baseline.stderr
```

Timeout: 300 seconds.

Stdout JSON path: `docs/research/runs/SPRINT-0025-entrypath-baseline.json`

Stderr/timing path: `docs/research/runs/SPRINT-0025-entrypath-baseline.stderr`

## SPRINT-0024 Reference Numbers

The SPRINT-0024 five-minute diagnostic timed out in `function_ref_index` before
emitting JSON. Stdout was 0 bytes.

| Phase | Wall clock | Reported memory |
|---|---:|---:|
| `package_load` | ~4.914s | 2.73 GB |
| `ssa_build` | ~4.788s | 4.49 GB |
| `root_resolution` | ~16.270s | 6.43 GB |
| `callgraph` | ~39.702s | 6.68 GB |
| `reverse_bfs` | ~0.354s | 6.68 GB |
| `function_ref_index` | >233s, timed out | >=6.68 GB |

Baseline comparison rule for SPRINT-0025: if any completed phase differs by
more than 2x from these SPRINT-0024 values, stop and write a baseline-divergence
note before running search experiments.

## SPRINT-0025 Baseline Run

Command executed exactly as listed above.

- Raw JSON artifact:
  [`SPRINT-0025-entrypath-baseline.json`](SPRINT-0025-entrypath-baseline.json)
- Stderr/timing artifact:
  [`SPRINT-0025-entrypath-baseline.stderr`](SPRINT-0025-entrypath-baseline.stderr)
- Wrapper exit status: 1
- Probe output: complete JSON was emitted
- Exit note: `/usr/bin/time -l` printed wall time but failed to read
  `kern.clockrate` under the sandbox, so it did not emit its usual resource
  footer. The probe's own phase RSS readings are used for peak memory.
- Wall time from `/usr/bin/time -l`: 246.28s real, 165.73s user, 228.47s sys
- Stdout JSON size: 286,798,022 bytes
- Peak RSS from probe phase readings: 10,865,488,224 bytes
- Probe stats: wallClockMillis=220,727; functionCount=140,801;
  staticEdgeCount=375,666; dynamicEdgeCount=290,044;
  unresolvedDynamicSiteCount=95,843; callgraphAlgorithm=`rta+vta`

Completed phase lines:

```text
entrypath-probe phase=package_load status=start elapsed_ms=0 rss_bytes=8407304
entrypath-probe phase=package_load status=end elapsed_ms=4417 rss_bytes=2544319576
entrypath-probe phase=ssa_build status=start elapsed_ms=0 rss_bytes=2544319576
entrypath-probe phase=ssa_build status=end elapsed_ms=3829 rss_bytes=4277786408
entrypath-probe phase=root_resolution status=start elapsed_ms=0 rss_bytes=4277786408
entrypath-probe phase=root_resolution status=end elapsed_ms=10054 rss_bytes=6106533352
entrypath-probe phase=callgraph status=start elapsed_ms=0 rss_bytes=6106533352
entrypath-probe phase=callgraph status=end elapsed_ms=25333 rss_bytes=6757604984
entrypath-probe phase=reverse_bfs status=start elapsed_ms=0 rss_bytes=6757604984
entrypath-probe phase=reverse_bfs status=end elapsed_ms=230 rss_bytes=6757736056
entrypath-probe phase=function_ref_index status=start elapsed_ms=0 rss_bytes=6757736056
entrypath-probe phase=function_ref_index status=end elapsed_ms=168071 rss_bytes=10865488224
entrypath-probe phase=function_value_flow status=start elapsed_ms=0 rss_bytes=10865488224
entrypath-probe phase=function_value_flow status=end elapsed_ms=18884 rss_bytes=10865488224
```

## Divergence Check

No SPRINT-0024 completed phase differed by more than 2x in the SPRINT-0025
baseline:

| Phase | SPRINT-0024 | SPRINT-0025 | Ratio |
|---|---:|---:|---:|
| `package_load` | 4.914s | 4.417s | 0.90x |
| `ssa_build` | 4.788s | 3.829s | 0.80x |
| `root_resolution` | 16.270s | 10.054s | 0.62x |
| `callgraph` | 39.702s | 25.333s | 0.64x |
| `reverse_bfs` | 0.354s | 0.230s | 0.65x |

`function_ref_index` was not a completed SPRINT-0024 phase; it timed out after
more than 233s. In this run it completed in 168.071s and was followed by
18.884s of `function_value_flow`.

## Subsystem Sanity Probe

Target package:
`/Users/tgoodwin/projects/monolift/evaluation/mattermost/server/channels/app/platform`

Command:

```sh
timeout 120 env GOWORK=/Users/tgoodwin/projects/monolift/.tmp/sprint-0021-a1-go.work \
  /tmp/monolift-sprint-0025-entrypath-probe \
    --diagnostic-timings \
    --region-root '(*Hub).Start' \
    --region-root '(*WebConn).Pump' \
    /Users/tgoodwin/projects/monolift/evaluation/mattermost/server/channels/app/platform \
    > docs/research/runs/SPRINT-0025-entrypath-subsystem-platform.json \
    2> docs/research/runs/SPRINT-0025-entrypath-subsystem-platform.stderr
```

Artifacts:

- Raw JSON:
  [`SPRINT-0025-entrypath-subsystem-platform.json`](SPRINT-0025-entrypath-subsystem-platform.json)
- Stderr/timing:
  [`SPRINT-0025-entrypath-subsystem-platform.stderr`](SPRINT-0025-entrypath-subsystem-platform.stderr)

Result: completed at reduced scale with exit status 0.

- Stdout JSON size: 196,687,968 bytes
- Probe stats: wallClockMillis=97,303; peakRSSBytes=6,434,623,016;
  functionCount=113,508; staticEdgeCount=278,917; dynamicEdgeCount=35,866;
  unresolvedDynamicSiteCount=87,805; callgraphAlgorithm=`rta+vta`

Completed phase lines:

```text
entrypath-probe phase=package_load status=start elapsed_ms=0 rss_bytes=8145160
entrypath-probe phase=package_load status=end elapsed_ms=2259 rss_bytes=1741418648
entrypath-probe phase=ssa_build status=start elapsed_ms=0 rss_bytes=1741418648
entrypath-probe phase=ssa_build status=end elapsed_ms=1369 rss_bytes=2904972584
entrypath-probe phase=root_resolution status=start elapsed_ms=0 rss_bytes=2904972584
entrypath-probe phase=root_resolution status=end elapsed_ms=5887 rss_bytes=4723838120
entrypath-probe phase=callgraph status=start elapsed_ms=0 rss_bytes=4723838120
entrypath-probe phase=callgraph status=end elapsed_ms=9214 rss_bytes=4760013992
entrypath-probe phase=reverse_bfs status=start elapsed_ms=0 rss_bytes=4760013992
entrypath-probe phase=reverse_bfs status=end elapsed_ms=0 rss_bytes=4760013992
entrypath-probe phase=function_ref_index status=start elapsed_ms=0 rss_bytes=4760013992
entrypath-probe phase=function_ref_index status=end elapsed_ms=72753 rss_bytes=6434623016
entrypath-probe phase=function_value_flow status=start elapsed_ms=0 rss_bytes=6434623016
entrypath-probe phase=function_value_flow status=end elapsed_ms=10495 rss_bytes=6434623016
```

## 120s Function-Index Budget Run

Command:

```sh
timeout 260 env GOWORK=/Users/tgoodwin/projects/monolift/.tmp/sprint-0021-a1-go.work \
  /tmp/monolift-sprint-0025-entrypath-probe \
    --diagnostic-timings \
    --function-index-budget=120s \
    --function-index-progress-interval=100000 \
    --region-root '(*Hub).Start' \
    --region-root '(*WebConn).Pump' \
    /Users/tgoodwin/projects/monolift/evaluation/mattermost/server/cmd/mattermost \
    > docs/research/runs/SPRINT-0025-entrypath-index-budget-120s-v3.json \
    2> docs/research/runs/SPRINT-0025-entrypath-index-budget-120s-v3.stderr
```

Artifacts:

- Raw JSON:
  [`SPRINT-0025-entrypath-index-budget-120s-v3.json`](SPRINT-0025-entrypath-index-budget-120s-v3.json)
- Stderr/progress:
  [`SPRINT-0025-entrypath-index-budget-120s-v3.stderr`](SPRINT-0025-entrypath-index-budget-120s-v3.stderr)
- Earlier failed attempts preserved:
  [`SPRINT-0025-entrypath-index-budget-120s.stderr`](SPRINT-0025-entrypath-index-budget-120s.stderr),
  [`SPRINT-0025-entrypath-index-budget-120s-v2.stderr`](SPRINT-0025-entrypath-index-budget-120s-v2.stderr)

Result: completed before the outer timeout with exit status 0. The index scanned
all functions but hit the 120s budget during index finalization/sorting, so the
JSON includes `Diagnostic{Kind: "function_ref_index_budget_exceeded"}` and
downstream flow ran on partial sorted index state.

- Stdout JSON size: 286,667,870 bytes
- Probe wallClockMillis: 206,689
- Probe peakRSSBytes: 12,470,170,336
- FunctionRefIndex: scannedFunctions=140,801; scannedBlocks=575,076;
  scannedInstructions=5,625,718; skippedFunctions=0; elapsedMillis=120,036;
  peakRSSBytes=12,458,504,928
- FunctionRefIndex refs: discoveredFunctionSources=140,801;
  closureSources=7,442; operandRefs=4,907,451; callArgRefs=894,142;
  storeRefs=1,327,094; returnRefs=249,446

Completed budget-run phase tail:

```text
entrypath-probe phase=function_ref_index status=progress elapsed_ms=36191 rss_bytes=7051620040 scanned_functions=140767 scanned_blocks=574876 scanned_instructions=5600000 current_package_path=vendor/golang.org/x/text/unicode/norm
entrypath-probe phase=function_ref_index status=end elapsed_ms=120038 rss_bytes=12458504928
entrypath-probe phase=function_value_flow status=start elapsed_ms=0 rss_bytes=12458504928
entrypath-probe phase=function_value_flow status=end elapsed_ms=23954 rss_bytes=12470170336
```

Progress hotspots from 100k-instruction events:

- Repeated package owners: `github.com/andybalholm/brotli` (7 intervals),
  `modernc.org/sqlite/lib` (4), `github.com/bits-and-blooms/bitset` (3),
  `modernc.org/mathutil` (2), `golang.org/x/text/encoding/charmap` (2),
  `github.com/redis/rueidis/internal/cmds` (2), and
  `github.com/mattermost/mattermost/server/v8/channels/store/storetest` (2).
- Largest progress gaps were between 900k and 1.3M scanned instructions:
  +3005ms at the 1.0M event, +2417ms at `go/build`, +1806ms at
  `google.golang.org/protobuf/internal/descfmt`, and +1662ms at `net/http`.
- Mattermost-owned packages appeared early and mid-run:
  `server/public/model` around 400k instructions, `channels/app` around 500k,
  `channels/app/platform` around 600k, `channels/api4` around 3.4M, and
  `channels/store/storetest` around 3.5M-3.6M.

## Heap Profile Decision

Heap profile capture was cut for this phase. `cmd/entrypath-probe` does not yet
have a heap-profile trigger at the function-index budget boundary, and the
budgeted Mattermost run already reached ~12.5 GB RSS and ~206s wall time. Adding
ad hoc profiling or another full run here would risk destabilizing the
diagnostic path without changing the immediate search-mode question. The
phase-level RSS and function-index progress artifacts above are the preserved
memory evidence for this sprint phase.

## Root Resolution Exact-Spec Comparison

Both comparison runs used the full Mattermost target and
`--function-index-max-functions=1` to keep post-callgraph index work bounded.

Bare-root fallback command used:

```sh
timeout 180 env GOWORK=/Users/tgoodwin/projects/monolift/.tmp/sprint-0021-a1-go.work \
  /tmp/monolift-sprint-0025-entrypath-probe \
    --diagnostic-timings \
    --function-index-max-functions=1 \
    --region-root '(*Hub).Start' \
    --region-root '(*WebConn).Pump' \
    /Users/tgoodwin/projects/monolift/evaluation/mattermost/server/cmd/mattermost \
    > docs/research/runs/SPRINT-0025-root-resolution-bare.json \
    2> docs/research/runs/SPRINT-0025-root-resolution-bare.stderr
```

Exact-root fast-path command used:

```sh
timeout 180 env GOWORK=/Users/tgoodwin/projects/monolift/.tmp/sprint-0021-a1-go.work \
  /tmp/monolift-sprint-0025-entrypath-probe \
    --diagnostic-timings \
    --function-index-max-functions=1 \
    --region-root 'github.com/mattermost/mattermost/server/v8/channels/app/platform.(*Hub).Start' \
    --region-root 'github.com/mattermost/mattermost/server/v8/channels/app/platform.(*WebConn).Pump' \
    /Users/tgoodwin/projects/monolift/evaluation/mattermost/server/cmd/mattermost \
    > docs/research/runs/SPRINT-0025-root-resolution-exact.json \
    2> docs/research/runs/SPRINT-0025-root-resolution-exact.stderr
```

Artifacts:

- Bare JSON:
  [`SPRINT-0025-root-resolution-bare.json`](SPRINT-0025-root-resolution-bare.json)
- Bare stderr:
  [`SPRINT-0025-root-resolution-bare.stderr`](SPRINT-0025-root-resolution-bare.stderr)
- Exact JSON:
  [`SPRINT-0025-root-resolution-exact.json`](SPRINT-0025-root-resolution-exact.json)
- Exact stderr:
  [`SPRINT-0025-root-resolution-exact.stderr`](SPRINT-0025-root-resolution-exact.stderr)

| Mode | root_resolution phase | functions inspected | matched specs | fast-path hits | fallback hits | RSS delta |
|---|---:|---:|---:|---:|---:|---:|
| bare fallback | 13.704s | 281,602 | 2 | 0 | 2 | +1,846,048,448 bytes |
| exact fast path | 6.912s | 161,009 | 2 | 2 | 0 | +84,611,088 bytes |

Verdict: the exact-spec resolver moved the observed root-resolution cost down
by roughly half in this bounded run and avoided the large fallback memory jump.
It does not eliminate all root-resolution cost because the current fast path
still scans unsorted SSA functions to find exact fully qualified specs.

Root-resolution branch cut decision: not cut. The branch completed with focused
tests and the bounded Mattermost comparison above. Remaining known unknown:
whether an SSA package/member lookup can replace the unsorted exact scan and
move the exact-root path below the remaining ~7s observed here.

## Reverse-Path Mode Run

Command:

```sh
timeout 220 env GOWORK=/Users/tgoodwin/projects/monolift/.tmp/sprint-0021-a1-go.work \
  /tmp/monolift-sprint-0025-entrypath-probe \
    --diagnostic-timings \
    --function-index-mode=reverse-path \
    --function-index-budget=60s \
    --function-index-progress-interval=50000 \
    --region-root 'github.com/mattermost/mattermost/server/v8/channels/app/platform.(*Hub).Start' \
    --region-root 'github.com/mattermost/mattermost/server/v8/channels/app/platform.(*WebConn).Pump' \
    /Users/tgoodwin/projects/monolift/evaluation/mattermost/server/cmd/mattermost \
    > docs/research/runs/SPRINT-0025-entrypath-reverse-path.json \
    2> docs/research/runs/SPRINT-0025-entrypath-reverse-path.stderr
```

Artifacts:

- Raw JSON:
  [`SPRINT-0025-entrypath-reverse-path.json`](SPRINT-0025-entrypath-reverse-path.json)
- Stderr/progress:
  [`SPRINT-0025-entrypath-reverse-path.stderr`](SPRINT-0025-entrypath-reverse-path.stderr)

Result: completed with exit status 0.

- Probe wallClockMillis: 62,202
- Probe peakRSSBytes: 6,613,876,312
- FunctionRefIndex: scannedFunctions=11,115; scannedBlocks=93,711;
  scannedInstructions=538,869; skippedFunctions=0; elapsedMillis=5,915;
  phase line elapsedMillis=7,081; peakRSSBytes=6,193,044,984
- Output counts: externalSurfaces=126; registrationSites=252;
  wrapperChains=12,211
- `connectWebSocket` recovered: no (`0` external surfaces, `0`
  registration handlers, `0` wrapper-chain external surfaces)
- Diagnostics summary: `vta_fallback_used`=1, `reverse_bfs_bound_exceeded`=1,
  `funcvalue_terminated_at_unknown_sink`=467

Progress packages included `server/public/model`, `channels/app`,
`channels/app/platform`, `channels/api4`, `golang.org/x/net/http2`, and
`github.com/gorilla/websocket`. Reverse-path mode was substantially cheaper
than `all` for function-index work, but it did not recover the target
Mattermost websocket surface.

## HTTP-Sinks Mode Run

Command:

```sh
timeout 220 env GOWORK=/Users/tgoodwin/projects/monolift/.tmp/sprint-0021-a1-go.work \
  /tmp/monolift-sprint-0025-entrypath-probe \
    --diagnostic-timings \
    --function-index-mode=http-sinks \
    --function-index-budget=60s \
    --function-index-progress-interval=50000 \
    --region-root 'github.com/mattermost/mattermost/server/v8/channels/app/platform.(*Hub).Start' \
    --region-root 'github.com/mattermost/mattermost/server/v8/channels/app/platform.(*WebConn).Pump' \
    /Users/tgoodwin/projects/monolift/evaluation/mattermost/server/cmd/mattermost \
    > docs/research/runs/SPRINT-0025-entrypath-http-sinks-v3.json \
    2> docs/research/runs/SPRINT-0025-entrypath-http-sinks-v3.stderr
```

Artifacts:

- Raw JSON:
  [`SPRINT-0025-entrypath-http-sinks-v3.json`](SPRINT-0025-entrypath-http-sinks-v3.json)
- Stderr:
  [`SPRINT-0025-entrypath-http-sinks-v3.stderr`](SPRINT-0025-entrypath-http-sinks-v3.stderr)
- Earlier timeout attempts preserved:
  [`SPRINT-0025-entrypath-http-sinks.stderr`](SPRINT-0025-entrypath-http-sinks.stderr),
  [`SPRINT-0025-entrypath-http-sinks-v2.stderr`](SPRINT-0025-entrypath-http-sinks-v2.stderr)

Result: completed with exit status 0 after making HTTP seed discovery
budget-aware. The 60s function-index budget was spent during seed discovery,
before seeded owner scanning began.

- Probe wallClockMillis: 125,635
- Probe peakRSSBytes: 7,867,346,744
- Seed counts: ownerCount=117; httpSinkOwners=117;
  rejectedNonHTTPInterfaceOwners=7,842
- FunctionRefIndex: scannedFunctions=0; scannedInstructions=0;
  skippedFunctions=117; elapsedMillis=1; peakRSSBytes=7,867,346,744
- Output counts: externalSurfaces=0; registrationSites=0; wrapperChains=0
- `connectWebSocket` recovered: no
- Diagnostics summary: `function_ref_index_budget_exceeded`=2,
  `vta_fallback_used`=1, `reverse_bfs_bound_exceeded`=1

Verdict: the structural HTTP-sink seed finder is semantically useful but too
expensive in its current whole-program scan form. With a 60s budget it produced
seed counts and rejected-interface counts, but no indexed flow evidence.

## Targeted Mode Runs

Default targeted command:

```sh
timeout 220 env GOWORK=/Users/tgoodwin/projects/monolift/.tmp/sprint-0021-a1-go.work \
  /tmp/monolift-sprint-0025-entrypath-probe \
    --diagnostic-timings \
    --function-index-mode=targeted \
    --function-index-budget=60s \
    --function-index-progress-interval=50000 \
    --region-root 'github.com/mattermost/mattermost/server/v8/channels/app/platform.(*Hub).Start' \
    --region-root 'github.com/mattermost/mattermost/server/v8/channels/app/platform.(*WebConn).Pump' \
    /Users/tgoodwin/projects/monolift/evaluation/mattermost/server/cmd/mattermost \
    > docs/research/runs/SPRINT-0025-entrypath-targeted-default.json \
    2> docs/research/runs/SPRINT-0025-entrypath-targeted-default.stderr
```

Expanded targeted command:

```sh
timeout 360 env GOWORK=/Users/tgoodwin/projects/monolift/.tmp/sprint-0021-a1-go.work \
  /tmp/monolift-sprint-0025-entrypath-probe \
    --diagnostic-timings \
    --function-index-mode=targeted \
    --function-index-budget=120s \
    --function-index-progress-interval=50000 \
    --targeted-max-depth=2 \
    --targeted-max-duration=90s \
    --targeted-max-functions=50000 \
    --targeted-max-queue=500000 \
    --region-root 'github.com/mattermost/mattermost/server/v8/channels/app/platform.(*Hub).Start' \
    --region-root 'github.com/mattermost/mattermost/server/v8/channels/app/platform.(*WebConn).Pump' \
    /Users/tgoodwin/projects/monolift/evaluation/mattermost/server/cmd/mattermost \
    > docs/research/runs/SPRINT-0025-entrypath-targeted-expanded.json \
    2> docs/research/runs/SPRINT-0025-entrypath-targeted-expanded.stderr
```

Artifacts:

- Default JSON:
  [`SPRINT-0025-entrypath-targeted-default.json`](SPRINT-0025-entrypath-targeted-default.json)
- Default stderr:
  [`SPRINT-0025-entrypath-targeted-default.stderr`](SPRINT-0025-entrypath-targeted-default.stderr)
- Expanded JSON:
  [`SPRINT-0025-entrypath-targeted-expanded.json`](SPRINT-0025-entrypath-targeted-expanded.json)
- Expanded stderr:
  [`SPRINT-0025-entrypath-targeted-expanded.stderr`](SPRINT-0025-entrypath-targeted-expanded.stderr)

| Mode | wall ms | peak RSS | seed owners | reverse-path owners | HTTP-sink owners | rejected non-HTTP interface owners | scanned functions | scanned instructions | stop diagnostics |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| targeted-default | 108,230 | 7,971,815,240 | 11,131 | 10,983 | 182 | 11,048 | 0 | 0 | `targeted_index_budget_exceeded`, `function_ref_index_budget_exceeded` |
| targeted-expanded | 173,393 | 8,109,514,600 | 11,379 | 11,141 | 292 | 19,208 | 0 | 0 | `targeted_index_budget_exceeded`, `function_ref_index_budget_exceeded` |

Recovery checks for both targeted runs:

- `connectWebSocket`: no
- `APIHandlerTrustRequester`: no
- Any `http.Handler` registration sink reached: no

Verdict: targeted mode currently spends the budget in seed discovery/expansion
before final seeded owner scanning. The larger budget increases seed counts but
still does not reach indexed flow evidence on Mattermost.

## Reverse-Path Root Scaling Curve

Mode: `reverse-path`.

Additional one-root Mattermost command:

```sh
timeout 180 env GOWORK=/Users/tgoodwin/projects/monolift/.tmp/sprint-0021-a1-go.work \
  /tmp/monolift-sprint-0025-entrypath-probe \
    --diagnostic-timings \
    --function-index-mode=reverse-path \
    --function-index-budget=60s \
    --function-index-progress-interval=50000 \
    --region-root 'github.com/mattermost/mattermost/server/v8/channels/app/platform.(*Hub).Start' \
    /Users/tgoodwin/projects/monolift/evaluation/mattermost/server/cmd/mattermost \
    > docs/research/runs/SPRINT-0025-entrypath-scaling-reverse-path-one-root.json \
    2> docs/research/runs/SPRINT-0025-entrypath-scaling-reverse-path-one-root.stderr
```

Cheap synthetic fixture command:

```sh
/tmp/monolift-sprint-0025-entrypath-probe \
  --diagnostic-timings \
  --function-index-mode=reverse-path \
  --region-root root \
  /Users/tgoodwin/projects/monolift/pkg/compiler/entrypath/testdata/reverse_path_seed \
  > docs/research/runs/SPRINT-0025-entrypath-scaling-reverse-path-fixture.json \
  2> docs/research/runs/SPRINT-0025-entrypath-scaling-reverse-path-fixture.stderr
```

| Run | wall ms | peak RSS | function-index phase | scanned functions | scanned instructions | external surfaces | registration sites | wrapper chains |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| synthetic fixture | 503 | 764,431,784 | 0ms | 4 | 9 | 1 | 1 | 3 |
| Mattermost one root | 44,453 | 5,680,131,464 | 294ms | 244 | 13,898 | 16 | 21 | 769 |
| Mattermost two roots | 62,202 | 6,613,876,312 | 7,081ms | 11,115 | 538,869 | 126 | 252 | 12,211 |

Verdict: reverse-path cost scales with the number and breadth of seeded owner
functions, not merely with root count. The second Mattermost root greatly
increases reverse-reachable owner breadth and scanned instructions.
