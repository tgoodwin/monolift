# SPRINT-0035 — Activation-path algorithm: skeleton + RTA baseline

**Status:** completed
**Executor:** Codex
**Predecessor:** SPRINT-0034 (72 synthesis traces across 6 codebases); SPRINT-0033 (retired — bridge/frontier approach did not generalize)

## Intent

Build a standalone Go binary (`cmd/activation-path/`) backed by a new package (`pkg/activation/`) that answers: given a Go binary and a target function at `file:line`, what is the shortest static path from a `main` entrypoint to that target?

This sprint delivers the **skeleton, RTA baseline, and evaluation harness**. The tool loads a Go program, builds an RTA call graph, runs BFS from entrypoints to the target, and reports a structured path or miss. The evaluation harness reads the 72 ground-truth traces from SPRINT-0034 (structured JSON at `docs/research/activation-paths/traces/`) and scores the analyzer with tiered metrics.

A follow-up sprint will add augmentation passes (struct-field function-value tracking, goroutine/channel edges, closure capture, registration patterns) guided by the baseline miss data this sprint produces.

This is a clean-slate implementation. `pkg/compiler/entrypath/` and `cmd/entrypath-probe/` exist for reference but are not imported.

## Goals

- [x] **G1** Ship a working `cmd/activation-path` binary that accepts `--packages <pattern>` and `--target <file:line>` and emits an activation path or a structured miss.
- [x] **G2** Implement `pkg/activation/` with zero imports from `pkg/compiler/entrypath/`.
- [x] **G3** Establish an RTA baseline: Tier 1 reachability and Tier 2 path similarity scores across all 72 traces.
- [x] **G4** Classify every miss by cause (unsupported edge type, timeout, load failure, target not found) to inform the follow-up sprint's augmentation priority.
- [x] **G5** Validate feasibility on large codebases (gitea, mattermost) — can we even load and run RTA within reasonable time/memory?

## Scope boundaries

**In scope:**
- RTA-based call graph with edge-type classification (direct call, concrete method call, interface dispatch)
- BFS shortest-path search with deterministic tie-breaking
- Evaluation harness reading the 72 structured JSON traces
- Tiered scoring: Tier 1 (reachability), Tier 2 (function-identity overlap), Tier 3 (file:line match, aspirational)
- Edge-type taxonomy as Go constants
- Per-target timeout and phase timing diagnostics

**Out of scope:**
- Augmentation passes (struct-field tracking, goroutine edges, channel flow, closure capture, registration patterns) — deferred to follow-up sprint
- Importing or extending `pkg/compiler/entrypath/`, `cmd/entrypath-probe/`
- Changes to downstream compiler packages
- Dynamic/runtime tracing
- Reflection-based dispatch
- Project-specific hacks

## Outputs

- [x] **O1** `cmd/activation-path/` with documented CLI and tests.
- [x] **O2** `pkg/activation/` with graph construction, edge typing, path search, and unit fixtures.
- [x] **O3** Evaluation harness that reads the 72 JSON traces and scores the analyzer.
- [x] **O4** RTA baseline results at `docs/research/runs/SPRINT-0035-rta-baseline.json`.
- [x] **O5** Human-readable baseline report at `docs/research/runs/SPRINT-0035-rta-baseline.md`.
- [x] **O6** This sprint file updated with baseline numbers, feasibility findings, and prioritized augmentation recommendations for the follow-up sprint.

## Task list

### Phase 0 — Edge taxonomy and trace loading

- [x] **0.1** Define `pkg/activation/edgekind.go`: `EdgeKind` constants for the canonical types (`DirectCall`, `ConcreteMethodCall`, `StructFieldFuncValue`, `InterfaceDispatch`, `GoroutineLaunch`, `HTTPHandlerRegistration`, `ChannelFlow`, `ClosureCapture`, `CallbackRegistration`, `StructLiteralFieldAssignment`, `Unsupported`).
- [x] **0.2** Write a mapping from synthesis-trace edge-type strings to canonical `EdgeKind` constants. Long-tail variants map to `Unsupported` with a diagnostic.
- [x] **0.3** Write `pkg/activation/eval/` code to load the per-candidate JSON trace files from `docs/research/activation-paths/traces/`. Parse into structured Go types. Test against 3 traces from different projects.
- [x] **0.4** Enumerate all 72 traces, pair each with its project directory and Go package pattern from `evaluation/MANIFEST.yaml`.
- [x] **0.5** Normalize trace function references into a canonical comparison key: `(package_path, receiver, func_name)`.

### Phase 1 — Package skeleton and CLI

- [x] **1.1** Define `pkg/activation/` public API types: `Analyzer`, `Config`, `Graph`, `Node`, `Edge`, `Path`, `EdgeKind`, `Result`, `Diagnostic`.
- [x] **1.2** Implement `Config.LoadProgram()`: use `golang.org/x/tools/go/packages` to load all packages matching the user's pattern.
- [x] **1.3** Implement `Config.ResolveTarget(file, line)`: find the `*ssa.Function` whose source range contains the target location. Return an error with nearest candidates if no exact match.
- [x] **1.4** Implement `Config.FindEntrypoints()`: discover `main.main` functions in the loaded program's command packages.
- [x] **1.5** Write `cmd/activation-path/main.go` with flags: `--packages`, `--target`, `--format {text,json}`, `--verbose`, `--timeout`. Emit stable text and JSON output.
- [x] **1.6** Add import-guard test that fails if `pkg/activation/` transitively imports `pkg/compiler/entrypath`.
- [x] **1.7** Unit tests for target resolution and entrypoint discovery using small fixture packages under `pkg/activation/testdata/`.
- [x] **1.8** Add per-target timeout (default 120s) and phase timing diagnostics from the start.

### Phase 2 — RTA baseline

- [x] **2.1** Build SSA (`golang.org/x/tools/go/ssa/ssautil.AllPackages`) from the loaded program.
- [x] **2.2** Run RTA (`golang.org/x/tools/go/callgraph/rta`) from discovered entrypoints. Convert the RTA callgraph into an `activation.Graph` with typed edges.
- [x] **2.3** Classify each RTA edge into `DirectCall`, `ConcreteMethodCall`, or `InterfaceDispatch` based on the SSA call instruction type.
- [x] **2.4** Preserve source positions for every node and edge where SSA exposes them.
- [x] **2.5** Implement BFS shortest-path search from any entrypoint to the target node, with deterministic tie-breaking (prefer shorter package paths, then alphabetical function names).
- [x] **2.6** Verify determinism: run the same target twice and diff output.
- [x] **2.7** Fixture tests: a small Go program with direct calls, concrete method calls, and interface dispatch.

### Phase 3 — Evaluation and reporting

- [x] **3.1** Implement Tier 1 scoring: binary reachability — did the analyzer find any path from an entrypoint to the target?
- [x] **3.2** Implement Tier 2 scoring: Jaccard similarity of intermediate functions. Report exact-match and fuzzy-match scores separately (fuzzy ignores SSA wrappers like `$bound`, `init$1`, generic type parameters).
- [x] **3.3** Implement Tier 3 scoring (aspirational): file:line match rate across steps.
- [x] **3.4** Report unsupported-edge breakdown: for each unreachable trace, identify the first edge in the expected path that the analyzer cannot represent and report its `EdgeKind`.
- [x] **3.5** Distinguish miss categories: target-unreachable, unsupported edge kind, package-load failure, timeout, target-not-found.
- [x] **3.6** Aggregate per-project tables (reachability rate, mean Tier 2 similarity) and corpus-wide summary.
- [x] **3.7** **Baseline evaluation run:** run against all 72 traces. Start with smaller codebases (miniflux, listmonk, pocketbase) to validate the harness, then run caddy, gitea, mattermost. Save to `docs/research/runs/SPRINT-0035-rta-baseline.json`.
- [x] **3.8** Write human-readable report to `docs/research/runs/SPRINT-0035-rta-baseline.md` with per-project tables and gap analysis.

### Phase 4 — Closeout

- [x] **4.1** Run `go vet ./pkg/activation/...` and the import-guard test.
- [x] **4.2** Verify determinism: run full evaluation twice and diff JSON.
- [x] **4.3** Rank the unsupported edge types by how many traces they block. This is the augmentation priority for the follow-up sprint.
- [x] **4.4** Update this sprint file with: baseline numbers, feasibility findings on large codebases, and ranked augmentation recommendations.

## Sequencing

```
Phase 0 ─┬─► Phase 1 ──► Phase 2 ──► Phase 3 ──► Phase 4
          │                              ▲
          └──────────────────────────────┘
```

- Phase 0 and Phase 1 can proceed in parallel; Phase 2 needs both.
- Phase 3 (evaluation) needs Phase 0 (trace loading) + Phase 2 (analyzer).
- Run evaluation against small codebases first (miniflux, listmonk, pocketbase) before attempting gitea/mattermost.

## Risks

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Large codebases blow up RTA memory or wall time | Medium | Blocks eval on 2 of 6 projects | Per-target timeout. Start with small codebases. Record timeouts rather than failing. If RTA is too expensive, try VTA. |
| SSA names don't match synthesis trace names | High | Tier 2 scores artificially low | Fuzzy matching: match on `(package, func_name)` ignoring SSA wrappers. Report exact and fuzzy scores separately. |
| Interface over-approximation (RTA resolves too many concrete types) | Medium | Tier 2 degrades from wrong-path selection | Record and report. If severe, evaluate VTA for follow-up sprint. |
| Synthesis traces contain LLM errors | Low-Medium | False negatives | When analyzer finds a shorter valid path, report as "analyzer-found alternative." |

## Guardrails

- [x] **Q1** Run `go test ./pkg/activation/...` after each phase.
- [x] **Q2** Run `go test ./cmd/activation-path/...` after the CLI exists.
- [x] **Q3** Run the import guard proving `pkg/activation` does not import `pkg/compiler/entrypath`.
- [x] **Q4** Run the 72-trace evaluator before closeout.
- [x] **Q5** Record any target that fails to load with exact package pattern, error, and pinned SHA.

## Acceptance criteria

1. `cmd/activation-path` compiles and produces a path or structured miss for any Go package pattern + `file:line` target.
2. `pkg/activation/` has zero transitive imports from `pkg/compiler/entrypath/` (enforced by test).
3. The evaluation harness runs against all 72 traces without manual intervention.
4. RTA baseline Tier 1 and Tier 2 scores are recorded per-project and corpus-wide.
5. Every miss is classified by cause (unsupported edge type, timeout, load failure, target not found).
6. A ranked list of augmentation priorities exists for the follow-up sprint, grounded in the miss data.
7. Feasibility on gitea and mattermost is documented (works, or times out with recorded diagnostics).

## Closeout — RTA baseline

Artifacts:
- JSON: `docs/research/runs/SPRINT-0035-rta-baseline.json`
- Markdown: `docs/research/runs/SPRINT-0035-rta-baseline.md`
- Full deterministic evaluation was run twice with `--deterministic`; the two JSON outputs diffed cleanly.
- Evaluation required `GOTOOLCHAIN=go1.26.2` so all pinned targets type-check under their declared `go` versions. Mattermost also required a temporary `GOWORK` that includes `evaluation/mattermost/server` and `evaluation/mattermost/server/public`; the evaluator creates this workspace without modifying the pinned clone.

Baseline numbers:

| Scope | Traces | Reachable | Reachability | Mean Tier 2 exact | Mean Tier 2 fuzzy | Mean Tier 3 file:line |
|---|---:|---:|---:|---:|---:|---:|
| Corpus | 72 | 49 | 68.1% | 0.173 | 0.199 | 0.000 |
| caddy | 6 | 0 | 0.0% | 0.000 | 0.000 | 0.000 |
| gitea | 18 | 16 | 88.9% | 0.133 | 0.159 | 0.000 |
| listmonk | 10 | 10 | 100.0% | 0.565 | 0.576 | 0.000 |
| mattermost | 15 | 0 | 0.0% | 0.000 | 0.000 | 0.000 |
| miniflux | 12 | 12 | 100.0% | 0.356 | 0.451 | 0.000 |
| pocketbase | 11 | 11 | 100.0% | 0.015 | 0.028 | 0.000 |

Feasibility by codebase:

| Project | Pattern | Completed | Timed out | Wall time ms | Heap alloc bytes | SHA |
|---|---|---:|---:|---:|---:|---|
| miniflux | `.` | true | false | 3241 | 1655259896 | `2916831cb1038e7150969fd41d1eb968ee62696e` |
| listmonk | `./cmd` | true | false | 2598 | 1607747208 | `3f4917035f63a82c93e19dedee8a48e55e291974` |
| pocketbase | `./examples/base` | true | false | 5536 | 2737837008 | `c3a53cb183cc102da4cd59f52b06270f9283b87f` |
| caddy | `./cmd/caddy` | true | false | 1463 | 2593746848 | `4430756d5c3047564c4d5d72793de6685ba3efda` |
| gitea | `.` | true | false | 23148 | 6278271176 | `b31eef282816294dc8d2ecc913d36e304f5348cb` |
| mattermost | `./cmd/mattermost` | true | false | 3543 | 4117713104 | `bf84301784777a6e08f9709ee882b0eac029437a` |

Heap alloc is the post-project `runtime.MemStats.HeapAlloc` sample, not a true peak RSS measurement. No codebase timed out under the 120s per-project evaluation budget; gitea is the large-codebase long pole.

Package loading note: the final baseline had no package-load failures. The exact package patterns and pinned SHAs used for every project are recorded in the feasibility table above.

Miss inventory and first blocker:

| Trace | Miss category | First blocker kind | Step | Raw edge type | Pattern to support |
|---|---|---|---:|---|---|
| caddy/M-1 | unsupported-edge-kind | StructFieldFuncValue | 2 | function-value-in-struct-field | Cobra command function stored during `defaultFactory.Build().Execute()`: `cmd/main.go:72` to `cmd/commandfuncs.go:172` `cmdRun` |
| caddy/M-2 | unsupported-edge-kind | StructFieldFuncValue | 2 | function-value-in-struct-field | Same Caddy Cobra command field dispatch: `cmd/main.go:72` to `cmd/commandfuncs.go:172` `cmdRun` |
| caddy/M-3 | unsupported-edge-kind | StructFieldFuncValue | 2 | function-value-in-struct-field | Same Caddy Cobra command field dispatch: `cmd/main.go:72` to `cmd/commandfuncs.go:172` `cmdRun` |
| caddy/M-4 | unsupported-edge-kind | Unsupported | 2 | init-populated-registry | Init-populated command registry: `cmd/main.go:72` `defaultFactory.Build()` to `cmd/commands.go:166` `cmd.RunE = WrapCommandFuncForCobra(cmdRun)` |
| caddy/M-5 | unsupported-edge-kind | StructFieldFuncValue | 2 | function-value-in-struct-field | Same Caddy Cobra command field dispatch: `cmd/main.go:72` to `cmd/commandfuncs.go:172` `cmdRun` |
| caddy/M-7 | unsupported-edge-kind | StructFieldFuncValue | 2 | function-value-in-struct-field | Same Caddy Cobra command field dispatch: `cmd/main.go:72` to `cmd/commandfuncs.go:172` `cmdRun` |
| gitea/M-13 | unsupported-edge-kind | StructFieldFuncValue | 1 | function-value-in-struct-field | urfave/cli action field dispatch: `cmd/main.go:160` to `cmd/web.go:251` `runWeb` |
| gitea/M-16 | unsupported-edge-kind | StructFieldFuncValue | 2 | function-value-in-struct-field | urfave/cli action field dispatch: `cmd/main.go:160` to `cmd/admin_user_change_password.go:47` `runChangePassword` |
| mattermost/M-1 | unsupported-edge-kind | StructFieldFuncValue | 2 | function-value-in-struct-field | Cobra `RunE` field dispatch: write at `server/cmd/mattermost/commands/server.go:30`, read through `RootCmd.Execute()` at `server/cmd/mattermost/commands/root.go:17`, target `serverCmdF` at `server/cmd/mattermost/commands/server.go:39` |
| mattermost/M-2 | unsupported-edge-kind | StructFieldFuncValue | 2 | function-value-in-struct-field | Same Mattermost Cobra `RunE` field dispatch to `serverCmdF` |
| mattermost/M-3 | unsupported-edge-kind | StructFieldFuncValue | 2 | function-value-in-struct-field | Same Mattermost Cobra `RunE` field dispatch to `serverCmdF` |
| mattermost/M-4 | target-not-found | StructFieldFuncValue | 2 | function-value-in-struct-field | Same Mattermost Cobra `RunE` field dispatch to `serverCmdF`; final target `server/enterprise/elasticsearch/common/indexing_job.go:412` was not in the loaded command package under current build tags/package pattern |
| mattermost/M-5 | unsupported-edge-kind | StructFieldFuncValue | 3 | function-value-in-struct-field | Cobra dispatch to export command callback: external Cobra dispatch to `server/cmd/mattermost/commands/export.go:123` `bulkExportCmdF` |
| mattermost/M-6 | unsupported-edge-kind | StructFieldFuncValue | 2 | function-value-in-struct-field | Same Mattermost Cobra `RunE` field dispatch to `serverCmdF` |
| mattermost/M-7 | unsupported-edge-kind | StructFieldFuncValue | 2 | function-value-in-struct-field | Same Mattermost Cobra `RunE` field dispatch to `serverCmdF` |
| mattermost/M-8 | unsupported-edge-kind | StructFieldFuncValue | 2 | function-value-in-struct-field | Same Mattermost Cobra `RunE` field dispatch to `serverCmdF` |
| mattermost/M-9 | unsupported-edge-kind | StructFieldFuncValue | 2 | function-value-in-struct-field | Same Mattermost Cobra `RunE` field dispatch to `serverCmdF` |
| mattermost/M-10 | unsupported-edge-kind | StructFieldFuncValue | 2 | function-value-in-struct-field | Same Mattermost Cobra `RunE` field dispatch to `serverCmdF` |
| mattermost/M-11 | unsupported-edge-kind | StructFieldFuncValue | 3 | function-value-in-struct-field | Cobra dispatch from `vendor/github.com/spf13/cobra/command.go:1015` to import command callback `server/cmd/mattermost/commands/import.go:104` `bulkImportCmdF` |
| mattermost/M-12 | unsupported-edge-kind | StructFieldFuncValue | 1 | init-time-function-field-dispatch | Init-time command field dispatch: `server/cmd/mattermost/main.go:20` through `RootCmd.Execute` to `serverCmdF` |
| mattermost/M-13 | unsupported-edge-kind | StructFieldFuncValue | 2 | function-value-in-struct-field | Same Mattermost Cobra `RunE` field dispatch to `serverCmdF` |
| mattermost/M-14 | unsupported-edge-kind | StructFieldFuncValue | 3 | function-value-in-struct-field | Cobra `RunE` field load in `(*cobra.Command).Execute` to `server/cmd/mattermost/commands/server.go:39` `serverCmdF` |
| mattermost/M-15 | unsupported-edge-kind | StructFieldFuncValue | 2 | function-value-in-struct-field | Cobra dispatch to Slack import callback: `server/cmd/mattermost/commands/root.go:17` to `server/cmd/mattermost/commands/import.go:52` `slackImportCmdF` |

Ranked augmentation recommendations:

1. Struct-field function-value tracking: 22 first blockers. Build a pass that tracks function values written into struct fields, especially Cobra/urfave command fields (`Run`, `RunE`, action callbacks), then adds edges from framework field reads/invocations to the stored function. This unlocks all Mattermost misses except the target-build-tag issue, all but one Caddy miss, and the two Gitea misses.
2. Init-populated registry handling: 1 first blocker (`caddy/M-4`). Treat this as a registry-specific extension of struct-field tracking: recognize init/build-time command registration where callbacks are wrapped and stored before command execution.
3. After struct-field tracking lands, rerun the same baseline. The currently reachable traces include many paths with low Tier 2 similarity because RTA over-approximates and chooses short static alternatives; follow-up scoring should inspect path quality, not just reachability.
4. Mattermost target coverage: `mattermost/M-4` resolved the command-dispatch blocker but the final enterprise target was not found in the loaded command package. A follow-up sprint should decide whether to add Mattermost enterprise build tags/package patterns before treating that trace as an algorithmic miss.

## Follow-up sprint (deferred)

Augmentation passes, ordered by the baseline miss data from this sprint:
- Struct-field function-value tracking (expected highest value: 70 corpus edges)
- Async boundaries: goroutine launch + channel flow (29 + 8 edges)
- Closure capture (~12 edges)
- Registration-pattern recognition: HTTP handlers + callbacks (19 + 7 edges)

Each pass adds edges to the graph and is measured as a delta against the RTA baseline.
