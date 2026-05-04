# SPRINT-0037 Report

## Phase 0 Reproduction

Toolchain: `GOTOOLCHAIN=go1.26.2+auto`.

Baseline caddy command:

```sh
/tmp/monolift-activation-path-go126 --packages ./cmd/caddy --target cmd/commandfuncs.go:172 --augmentations all --timeout 180s --verbose
```

Result: found 6 steps ending at `github.com/caddyserver/caddy/v2/cmd.cmdRun` through `github.com/spf13/cobra.*Command.execute --StructFieldFuncValue--> cmdRun`.

Outgoing-edge probe against the in-memory augmented graph found:

```text
cmdRun outgoing edges: 1
GoroutineLaunch -> github.com/caddyserver/caddy/v2/cmd.watchConfigFile (static function call)
```

Baseline caddy `Load` command:

```sh
/tmp/monolift-activation-path-go126 --packages ./cmd/caddy --target caddy.go:115 --augmentations all --timeout 180s --verbose
```

Result: `miss: target-unreachable` for `github.com/caddyserver/caddy/v2.Load`.

Note: the sprint plan said `caddy.Load` was present elsewhere in the RTA graph. In this checkout/toolchain, an exact graph-node probe did not find `github.com/caddyserver/caddy/v2.Load` before the fix. The observable blocker is still the same: augmentation reaches `cmdRun`, but `cmdRun`'s body is not explored, so direct callees such as `caddy.Load` are absent or unreachable.

## Fix Summary

Implemented re-rooted RTA exploration for augmentation-discovered functions:

- Added `Graph.FunctionSet()` and `Graph.NewFunctionsSince()` to snapshot graph functions and deterministically identify new augmentation roots.
- Added `ExploreCallees(graph, program, roots)` to run `rta.Analyze` from new roots only, then merge classified call-graph edges into the existing activation graph.
- Converted `Augment()` into an iterative convergence loop: run selected augmentation passes, explore newly added roots, then repeat until no new functions are added.
- Added a 10-round cap and convergence diagnostics. All projects converged in 2-3 rounds.
- Kept `ModeRTAOnly` as a pure baseline path with no augmentation or re-rooted exploration.
- Tightened generic struct-field read connection to read sites already present in the current graph. This avoids adding disconnected read callers from the whole loaded program.
- Filtered augmentation targets nested in generic function contexts before inserting them into the graph, because `x/tools/rta` cannot handle some of these closure roots without panicking. `ExploreCallees` also keeps a warning-based safety filter for any generic-context roots that reach it. The filtered roots are generic pool/init closures, not the command handlers targeted by this sprint.

## Phase 3 Smoke Results

### Caddy

`cmdRun` now has 1007 outgoing edges after augmentation and callee exploration, including `cmdRun --DirectCall--> github.com/caddyserver/caddy/v2.Load`.

`caddy.go:115` is reachable in 7 steps:

```text
main -> Main -> Execute -> ExecuteC -> execute -> cmdRun -> Load
```

Per-project smoke evaluation (`/tmp/s37-caddy.json`) reached all 6 caddy traces. The requested old-dead-end traces `M-1`, `M-2`, `M-3`, `M-5`, and `M-7` are all reachable; `M-4` is also reachable in this run.

### Mattermost

Per-project smoke evaluation (`/tmp/s37-mattermost.json`) reached 14/15 mattermost traces. The requested traces `M-3`, `M-5`, `M-6`, `M-11`, `M-13`, and `M-15` are all reachable after callee exploration. `M-11`, `M-5`, and `M-15` follow the expected command-handler branch (`bulkImportCmdF`, `bulkExportCmdF`, `slackImportCmdF`). `M-3`, `M-6`, and `M-13` are reachable but their shortest paths are broad/indirect rather than the exact expected `serverCmdF -> runServer` chain; these are included in the later Tier 2 path-quality review.

### Gitea

Per-project smoke evaluation (`/tmp/s37-gitea.json`) remained 16/18 reachable. `M-13` is still blocked at partial depth 1/7 with `struct-field-not-resolved` / `CallbackRegistration`; `M-16` is still blocked at partial depth 2/9 with `struct-field-not-resolved` / `Unsupported`. Their downstream handler bodies are not explored by this sprint's fix.

## Phase 4 Full Evaluation

Full augmented artifact: `docs/research/runs/SPRINT-0037-full.json`.

### Corpus Comparison

| Run | Reachable | Rate | Mean T2 exact | Mean T2 fuzzy |
|---|---:|---:|---:|---:|
| SPRINT-0035 RTA | 49/72 | 68.1% | 0.173 | 0.199 |
| SPRINT-0036 final | 49/72 | 68.1% | 0.173 | 0.199 |
| SPRINT-0037 full | 69/72 | 95.8% | 0.231 | 0.261 |

### Per Project

| Project | SPRINT-0035 | SPRINT-0036 | SPRINT-0037 |
|---|---:|---:|---:|
| caddy | 0/6 | 0/6 | 6/6 |
| gitea | 16/18 | 16/18 | 16/18 |
| listmonk | 10/10 | 10/10 | 10/10 |
| mattermost | 0/15 | 0/15 | 14/15 |
| miniflux | 12/12 | 12/12 | 12/12 |
| pocketbase | 11/11 | 11/11 | 11/11 |

### Convergence

| Project | Exploration rounds | Hit cap? |
|---|---:|---|
| miniflux | 2 | no |
| listmonk | 2 | no |
| pocketbase | 2 | no |
| caddy | 3 | no |
| gitea | 2 | no |
| mattermost | 3 | no |

### Original 22 Struct-Field-Blocked Traces

| Trace | SPRINT-0037 status | Notes |
|---|---|---|
| caddy/M-1 | reachable | moved past `cmdRun` |
| caddy/M-2 | reachable | moved past `cmdRun` |
| caddy/M-3 | reachable | moved past `cmdRun` |
| caddy/M-5 | reachable | moved past `cmdRun` |
| caddy/M-7 | reachable | moved past `cmdRun` |
| gitea/M-13 | blocked | partial 1/7, `CallbackRegistration` |
| gitea/M-16 | blocked | partial 2/9, `Unsupported` map-indexed dispatch |
| mattermost/M-1 | reachable | moved past `serverCmdF` |
| mattermost/M-2 | reachable | moved past `serverCmdF` |
| mattermost/M-3 | reachable | target reachable, shortest path is broad/indirect |
| mattermost/M-4 | target-not-found | enterprise package still not loaded |
| mattermost/M-5 | reachable | moved past command handler |
| mattermost/M-6 | reachable | target reachable, shortest path is broad/indirect |
| mattermost/M-7 | reachable | moved past `serverCmdF` |
| mattermost/M-8 | reachable | moved past `serverCmdF` |
| mattermost/M-9 | reachable | moved past `serverCmdF` |
| mattermost/M-10 | reachable | moved past `serverCmdF` |
| mattermost/M-11 | reachable | moved past `bulkImportCmdF` |
| mattermost/M-12 | reachable | moved past `serverCmdF` |
| mattermost/M-13 | reachable | target reachable, shortest path is broad/indirect |
| mattermost/M-14 | reachable | moved past `serverCmdF` |
| mattermost/M-15 | reachable | moved past `slackImportCmdF` |

Result: 19/22 original struct-field-blocked traces are now reachable.

### SPRINT-0036 Target-Unreachable Cases

| Trace | SPRINT-0037 status | Path length | T2 exact | T2 fuzzy |
|---|---|---:|---:|---:|
| caddy/M-2 | reachable | 11 | 0.176 | 0.250 |
| caddy/M-7 | reachable | 11 | 0.214 | 0.214 |
| mattermost/M-3 | reachable | 9 | 0.125 | 0.125 |
| mattermost/M-5 | reachable | 7 | 0.400 | 0.400 |
| mattermost/M-6 | reachable | 12 | 0.235 | 0.235 |
| mattermost/M-11 | reachable | 8 | 0.429 | 0.429 |
| mattermost/M-13 | reachable | 11 | 0.188 | 0.188 |
| mattermost/M-15 | reachable | 8 | 0.500 | 0.500 |

All 8 SPRINT-0036 `target-unreachable` cases are now reachable.

### Path Plausibility Spot Check

| Trace/probe | Plausible command-dispatch chain? | Notes |
|---|---|---|
| caddy `caddy.go:115` probe | yes | `main -> Main -> Execute -> ExecuteC -> execute -> cmdRun -> Load` |
| mattermost/M-5 | yes | `execute -> bulkExportCmdF -> App.BulkExport` |
| mattermost/M-11 | yes | `execute -> bulkImportCmdF -> App.BulkImportWithPath -> App.bulkImport` |
| mattermost/M-15 | yes | `execute -> slackImportCmdF -> App.SlackImport -> SlackImporter.SlackImport` |
| caddy/M-2 | no | target reachable, but shortest path enters HTTP/template code through broad non-command edges |
| mattermost/M-13 | partial | target reachable, but shortest path bypasses expected `serverCmdF -> runServer` chain |

Conclusion: Tier 1 reachability improved sharply, but broader RTA roots expose shortest-path quality issues for some server/runtime targets. This is reflected in Tier 2 scores and should be handled separately from the dead-end-node fix.

## Phase 5 New-Wall Analysis

### Remaining Blockers

| Rank | Blocker class | Count | Traces | Notes |
|---:|---|---:|---|---|
| 1 | Unsupported patterns | 2 | gitea/M-13, gitea/M-16 | callback registration and map-indexed function-value dispatch |
| 2 | Target not loaded | 1 | mattermost/M-4 | enterprise package target is not in the loaded package graph |
| 3 | HTTP registration | 0 |  | no remaining Tier 1 blocker after this fix |
| 3 | Closure capture | 0 |  | no remaining Tier 1 blocker after this fix |
| 3 | Channel flow | 0 |  | no remaining Tier 1 blocker after this fix |
| 3 | Target-unreachable with no identified blocker | 0 |  | no remaining unknown-unreachable miss |

Per-trace first blocker classification:

| Trace | Category | First blocker | Partial depth | Gap |
|---|---|---|---:|---|
| gitea/M-13 | `unsupported-edge-kind` | callback registration | 1/7 | `struct-field-not-resolved` |
| gitea/M-16 | `unsupported-edge-kind` | map-indexed function-value dispatch | 2/9 | `struct-field-not-resolved` |
| mattermost/M-4 | `target-not-found` | target package not loaded | 2/12 | `target-not-loaded` |

Tier 2 regression check: among the 49 traces that were reachable in SPRINT-0035 and remain reachable in SPRINT-0037, no trace has a lower Tier 2 exact or fuzzy score.

### Next Priority Ranking

| Priority | Blocker class | Count | Feasibility | Cross-project impact | Recommendation |
|---:|---|---:|---|---|---|
| 1 | Callback registration | 1 | medium | medium | Generalize callback/function registration edges beyond struct fields; starts with gitea/M-13. |
| 2 | Map-indexed function-value dispatch | 1 | medium | medium | Track function values stored in string/keyed maps and resolve map-indexed calls; starts with gitea/M-16. |
| 3 | Target/package loading | 1 | low for activation graph | low | Mattermost/M-4 needs package/build-pattern work, not another graph edge. |

HTTP registration, closure capture, and channel-flow edges are still important for path fidelity and future corpora, but they are not the remaining Tier 1 wall in this SPRINT-0037 run.
