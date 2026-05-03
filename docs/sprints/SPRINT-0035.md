# SPRINT-0035 — Activation-path algorithm: skeleton + RTA baseline

**Status:** planned
**Executor:** TBD
**Predecessor:** SPRINT-0034 (72 synthesis traces across 6 codebases); SPRINT-0033 (retired — bridge/frontier approach did not generalize)

## Intent

Build a standalone Go binary (`cmd/activation-path/`) backed by a new package (`pkg/activation/`) that answers: given a Go binary and a target function at `file:line`, what is the shortest static path from a `main` entrypoint to that target?

This sprint delivers the **skeleton, RTA baseline, and evaluation harness**. The tool loads a Go program, builds an RTA call graph, runs BFS from entrypoints to the target, and reports a structured path or miss. The evaluation harness reads the 72 ground-truth traces from SPRINT-0034 (structured JSON at `docs/research/activation-paths/traces/`) and scores the analyzer with tiered metrics.

A follow-up sprint will add augmentation passes (struct-field function-value tracking, goroutine/channel edges, closure capture, registration patterns) guided by the baseline miss data this sprint produces.

This is a clean-slate implementation. `pkg/compiler/entrypath/` and `cmd/entrypath-probe/` exist for reference but are not imported.

## Goals

- [ ] **G1** Ship a working `cmd/activation-path` binary that accepts `--packages <pattern>` and `--target <file:line>` and emits an activation path or a structured miss.
- [ ] **G2** Implement `pkg/activation/` with zero imports from `pkg/compiler/entrypath/`.
- [ ] **G3** Establish an RTA baseline: Tier 1 reachability and Tier 2 path similarity scores across all 72 traces.
- [ ] **G4** Classify every miss by cause (unsupported edge type, timeout, load failure, target not found) to inform the follow-up sprint's augmentation priority.
- [ ] **G5** Validate feasibility on large codebases (gitea, mattermost) — can we even load and run RTA within reasonable time/memory?

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

- [ ] **O1** `cmd/activation-path/` with documented CLI and tests.
- [ ] **O2** `pkg/activation/` with graph construction, edge typing, path search, and unit fixtures.
- [ ] **O3** Evaluation harness that reads the 72 JSON traces and scores the analyzer.
- [ ] **O4** RTA baseline results at `docs/research/runs/SPRINT-0035-rta-baseline.json`.
- [ ] **O5** Human-readable baseline report at `docs/research/runs/SPRINT-0035-rta-baseline.md`.
- [ ] **O6** This sprint file updated with baseline numbers, feasibility findings, and prioritized augmentation recommendations for the follow-up sprint.

## Task list

### Phase 0 — Edge taxonomy and trace loading

- [ ] **0.1** Define `pkg/activation/edgekind.go`: `EdgeKind` constants for the canonical types (`DirectCall`, `ConcreteMethodCall`, `StructFieldFuncValue`, `InterfaceDispatch`, `GoroutineLaunch`, `HTTPHandlerRegistration`, `ChannelFlow`, `ClosureCapture`, `CallbackRegistration`, `StructLiteralFieldAssignment`, `Unsupported`).
- [ ] **0.2** Write a mapping from synthesis-trace edge-type strings to canonical `EdgeKind` constants. Long-tail variants map to `Unsupported` with a diagnostic.
- [ ] **0.3** Write `pkg/activation/eval/` code to load the per-candidate JSON trace files from `docs/research/activation-paths/traces/`. Parse into structured Go types. Test against 3 traces from different projects.
- [ ] **0.4** Enumerate all 72 traces, pair each with its project directory and Go package pattern from `evaluation/MANIFEST.yaml`.
- [ ] **0.5** Normalize trace function references into a canonical comparison key: `(package_path, receiver, func_name)`.

### Phase 1 — Package skeleton and CLI

- [ ] **1.1** Define `pkg/activation/` public API types: `Analyzer`, `Config`, `Graph`, `Node`, `Edge`, `Path`, `EdgeKind`, `Result`, `Diagnostic`.
- [ ] **1.2** Implement `Config.LoadProgram()`: use `golang.org/x/tools/go/packages` to load all packages matching the user's pattern.
- [ ] **1.3** Implement `Config.ResolveTarget(file, line)`: find the `*ssa.Function` whose source range contains the target location. Return an error with nearest candidates if no exact match.
- [ ] **1.4** Implement `Config.FindEntrypoints()`: discover `main.main` functions in the loaded program's command packages.
- [ ] **1.5** Write `cmd/activation-path/main.go` with flags: `--packages`, `--target`, `--format {text,json}`, `--verbose`, `--timeout`. Emit stable text and JSON output.
- [ ] **1.6** Add import-guard test that fails if `pkg/activation/` transitively imports `pkg/compiler/entrypath`.
- [ ] **1.7** Unit tests for target resolution and entrypoint discovery using small fixture packages under `pkg/activation/testdata/`.
- [ ] **1.8** Add per-target timeout (default 120s) and phase timing diagnostics from the start.

### Phase 2 — RTA baseline

- [ ] **2.1** Build SSA (`golang.org/x/tools/go/ssa/ssautil.AllPackages`) from the loaded program.
- [ ] **2.2** Run RTA (`golang.org/x/tools/go/callgraph/rta`) from discovered entrypoints. Convert the RTA callgraph into an `activation.Graph` with typed edges.
- [ ] **2.3** Classify each RTA edge into `DirectCall`, `ConcreteMethodCall`, or `InterfaceDispatch` based on the SSA call instruction type.
- [ ] **2.4** Preserve source positions for every node and edge where SSA exposes them.
- [ ] **2.5** Implement BFS shortest-path search from any entrypoint to the target node, with deterministic tie-breaking (prefer shorter package paths, then alphabetical function names).
- [ ] **2.6** Verify determinism: run the same target twice and diff output.
- [ ] **2.7** Fixture tests: a small Go program with direct calls, concrete method calls, and interface dispatch.

### Phase 3 — Evaluation and reporting

- [ ] **3.1** Implement Tier 1 scoring: binary reachability — did the analyzer find any path from an entrypoint to the target?
- [ ] **3.2** Implement Tier 2 scoring: Jaccard similarity of intermediate functions. Report exact-match and fuzzy-match scores separately (fuzzy ignores SSA wrappers like `$bound`, `init$1`, generic type parameters).
- [ ] **3.3** Implement Tier 3 scoring (aspirational): file:line match rate across steps.
- [ ] **3.4** Report unsupported-edge breakdown: for each unreachable trace, identify the first edge in the expected path that the analyzer cannot represent and report its `EdgeKind`.
- [ ] **3.5** Distinguish miss categories: target-unreachable, unsupported edge kind, package-load failure, timeout, target-not-found.
- [ ] **3.6** Aggregate per-project tables (reachability rate, mean Tier 2 similarity) and corpus-wide summary.
- [ ] **3.7** **Baseline evaluation run:** run against all 72 traces. Start with smaller codebases (miniflux, listmonk, pocketbase) to validate the harness, then run caddy, gitea, mattermost. Save to `docs/research/runs/SPRINT-0035-rta-baseline.json`.
- [ ] **3.8** Write human-readable report to `docs/research/runs/SPRINT-0035-rta-baseline.md` with per-project tables and gap analysis.

### Phase 4 — Closeout

- [ ] **4.1** Run `go vet ./pkg/activation/...` and the import-guard test.
- [ ] **4.2** Verify determinism: run full evaluation twice and diff JSON.
- [ ] **4.3** Rank the unsupported edge types by how many traces they block. This is the augmentation priority for the follow-up sprint.
- [ ] **4.4** Update this sprint file with: baseline numbers, feasibility findings on large codebases, and ranked augmentation recommendations.

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

- [ ] **Q1** Run `go test ./pkg/activation/...` after each phase.
- [ ] **Q2** Run `go test ./cmd/activation-path/...` after the CLI exists.
- [ ] **Q3** Run the import guard proving `pkg/activation` does not import `pkg/compiler/entrypath`.
- [ ] **Q4** Run the 72-trace evaluator before closeout.
- [ ] **Q5** Record any target that fails to load with exact package pattern, error, and pinned SHA.

## Acceptance criteria

1. `cmd/activation-path` compiles and produces a path or structured miss for any Go package pattern + `file:line` target.
2. `pkg/activation/` has zero transitive imports from `pkg/compiler/entrypath/` (enforced by test).
3. The evaluation harness runs against all 72 traces without manual intervention.
4. RTA baseline Tier 1 and Tier 2 scores are recorded per-project and corpus-wide.
5. Every miss is classified by cause (unsupported edge type, timeout, load failure, target not found).
6. A ranked list of augmentation priorities exists for the follow-up sprint, grounded in the miss data.
7. Feasibility on gitea and mattermost is documented (works, or times out with recorded diagnostics).

## Follow-up sprint (deferred)

Augmentation passes, ordered by the baseline miss data from this sprint:
- Struct-field function-value tracking (expected highest value: 70 corpus edges)
- Async boundaries: goroutine launch + channel flow (29 + 8 edges)
- Closure capture (~12 edges)
- Registration-pattern recognition: HTTP handlers + callbacks (19 + 7 edges)

Each pass adds edges to the graph and is measured as a delta against the RTA baseline.
