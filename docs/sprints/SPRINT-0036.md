# SPRINT-0036 — Activation-path augmentation: struct-field tracking + framework predicates

**Status:** planned
**Executor:** TBD
**Predecessor:** SPRINT-0035 (RTA baseline: 49/72 reachable, 22 blocked by `StructFieldFuncValue`)

## Intent

SPRINT-0035 proved RTA is feasible across all 6 codebases but hits a wall at struct-field function-value dispatch — 22 of 23 missed traces are blocked at the same chokepoint: a function stored into a framework struct field (cobra `RunE`, urfave/cli `Action`) that RTA cannot follow. This sprint attacks that wall with two complementary pieces: a generic SSA pass that connects struct-field stores to loads, and a framework predicate registry that provides known dispatch-site mappings as data. It also adds goroutine-launch edges (cheap, 29 corpus edges) and partial-path emission (diagnostic value for still-missed traces).

The key insight: dispatch sites like `(*cobra.Command).execute()` are fixed framework locations. Predicates are data, not algorithm changes — adding a new framework is adding a table row.

## Baseline (SPRINT-0035)

| Project | Traces | Reachable | Rate | Mean T2 Fuzzy |
|---|---:|---:|---:|---:|
| caddy | 6 | 0 | 0.0% | 0.000 |
| gitea | 18 | 16 | 88.9% | 0.159 |
| listmonk | 10 | 10 | 100.0% | 0.576 |
| mattermost | 15 | 0 | 0.0% | 0.000 |
| miniflux | 12 | 12 | 100.0% | 0.451 |
| pocketbase | 11 | 11 | 100.0% | 0.028 |
| **Corpus** | **72** | **49** | **68.1%** | **0.199** |

Miss breakdown: 22 `StructFieldFuncValue`, 1 `Unsupported` (caddy/M-4 init-populated-registry), 1 `target-not-found` (mattermost/M-4 enterprise package). The 2 non-StructFieldFuncValue misses are expected to remain — caddy/M-4 is not statically resolvable, mattermost/M-4 is a package-loading issue.

## Goals

- [ ] **G1** Unblock ≥18 of the 22 `StructFieldFuncValue`-blocked traces.
- [ ] **G2** Add goroutine-launch edges to improve path precision on already-reachable traces.
- [ ] **G3** Emit partial paths with labeled gaps on still-missed traces.
- [ ] **G4** Measure each augmentation incrementally against the SPRINT-0035 baseline.
- [ ] **G5** Identify the next wall after struct-field dispatch is resolved.

## Scope boundaries

**In scope:**
- Generic struct-field function-value tracking (SSA `FieldAddr` + `Store`/`UnOp` + `Call`)
- Framework predicate table (cobra `RunE`/`Run`, urfave/cli v3 `Action`)
- Narrow wrapper-return recognizer for Caddy's `WrapCommandFuncForCobra(cmdRun)` pattern
- Goroutine-launch edges (`*ssa.Go`)
- Partial-path emission with labeled gaps
- Incremental delta evaluation per phase

**Out of scope:**
- General closure capture (deferred — the narrow wrapper recognizer handles the critical path)
- Channel-flow tracking
- HTTP handler registration patterns
- Caddy's init-populated string-keyed module registry (caddy/M-4)
- Cross-process dispatch (mattermost go-plugin RPC)
- Reflection-based dispatch (`text/template` FuncMap)
- Changes to `pkg/compiler/entrypath/` or downstream compiler packages

## Architecture

Augmentation passes run **after** `BuildRTAGraph()` and **before** `ShortestPath()`. Each pass takes a `*Graph` and a `*Program` and adds edges in place. The graph's `Out`/`In` adjacency maps and `Edges` slice are extended through `Graph.AddEdge`/`Graph.AddNode` methods that handle deduplication and sequential IDs.

```
LoadProgram → BuildSSA → FindEntrypoints → BuildRTAGraph
                                                │
                                         AugmentStructField(graph, program)
                                                │
                                         ApplyPredicates(graph, program, registry)
                                                │
                                         AugmentGoroutine(graph, program)
                                                │
                                         ShortestPath(graph, entrypoints, target)
```

Integration sites: `eval/runner.go:runProject` (calls `Augment()` after `BuildRTAGraph()`, before trace loop) and `analyzer.go:Analyze()` (after RTA phase).

### File layout

```
pkg/activation/
  augment.go          — Augment() orchestrator + augmentation mode type
  structfield.go      — struct-field function-value SSA scan
  predicates.go       — framework predicate registry + matching
  goroutine.go        — goroutine-launch edge scan
  partial.go          — partial-path construction + Gap type
```

## Task list

### Phase 0 — Measurement harness and augmentation hooks

Wire up mode selection and graph mutation API before any augmentation work begins.

- [ ] **0.1** Add `AugmentMode` type in `pkg/activation/augment.go` with values `ModeRTAOnly`, `ModeStructField`, `ModePredicates`, `ModeGoroutine`, `ModeAll`.
- [ ] **0.2** Add `Augment(graph *Graph, program *Program, mode AugmentMode) error` that dispatches to sub-passes based on mode.
- [ ] **0.3** Add `Graph.AddEdge(from, to int, kind EdgeKind, pos Position, desc string) *Edge` — the single mutation point for augmentation passes. Deduplicates by `(from, to, kind)` key. Returns existing edge if duplicate.
- [ ] **0.4** Add `Graph.AddNode(key FunctionKey, fn *ssa.Function) *Node` — for functions not in the RTA graph (e.g., stored but never directly called). Deduplicates by `*ssa.Function` pointer.
- [ ] **0.5** Add `--augmentations` flag to `cmd/activation-path/` accepting `rta`, `structfield`, `predicates`, `goroutine`, `all`. Default: `all`.
- [ ] **0.6** Wire evaluator plumbing in `eval/runner.go` so `runProject` accepts the augmentation mode and calls `Augment()` after `BuildRTAGraph()`.
- [ ] **0.7** Smoke test: run the evaluator in `ModeRTAOnly` and verify it reproduces the SPRINT-0035 baseline of 49/72. Save as `docs/research/runs/SPRINT-0036-phase0-rta-only.json`.

### Phase 1 — Generic struct-field function-value tracking

The SSA pass that finds function values stored into struct fields and connects them to invocation sites.

- [ ] **1.1** Implement **write-side scan** in `pkg/activation/structfield.go`: iterate all instructions in all loaded SSA functions (not just RTA-reachable — stores happen in init/setup code that RTA may not visit). For each `*ssa.FieldAddr` whose result flows into a `*ssa.Store` where the stored value has a function type — record `(structType, fieldIndex, fieldName) → []storedFunc`. Use a stable field key containing package path, named struct type, field index, field name, and function signature.
- [ ] **1.2** Normalize stored callable values through `*ssa.MakeInterface`, `*ssa.ChangeType`, `*ssa.Convert`, and other transparent SSA wrappers before resolving to the underlying `*ssa.Function`.
- [ ] **1.3** Handle `*ssa.MakeClosure` on the write side: target the closure's `Fn` field. If the closure wraps a bound method, target the method, not the wrapper.
- [ ] **1.4** Add a **narrow wrapper-return recognizer**: for stored values that are calls to wrapper functions (like `WrapCommandFuncForCobra(cmdRun)`), check if the wrapper is a single-return function whose return value is a `MakeClosure` that immediately delegates to a captured parameter. If so, track the original argument (`cmdRun`) as the stored function. Scope: single-return, immediate-delegate only — not a general closure-capture pass.
- [ ] **1.5** Handle struct-literal field assignment: scan `*ssa.Alloc` + `FieldAddr` + `Store` sequences for `Config{Action: runWeb}` patterns. Tag edges as `StructLiteralFieldAssignment` to distinguish from direct field assignment in reporting.
- [ ] **1.6** Implement **read-side scan**: for each `*ssa.FieldAddr` whose result flows into a `*ssa.UnOp` (load) whose result is the callee of a `*ssa.Call` — record the calling function and `(structType, fieldIndex)`.
- [ ] **1.7** **Connect writes to reads**: for each `(structType, fieldIndex)` that appears in both maps, add a `StructFieldFuncValue` edge from the read-side calling function to each stored function. Type-filter: stored function's signature must be assignable to the field's function type.
- [ ] **1.8** Fixture tests in `pkg/activation/testdata/structfield/`:
  - `direct/` — function stored into `handler.Run`, then loaded and called
  - `literal/` — struct literal `Handler{Run: myFunc}`
  - `methodvalue/` — `handler.Run = obj.Method` (bound method value)
  - `wrapper/` — `handler.Run = wrap(innerFunc)` where wrap returns a delegating closure
- [ ] **1.9** Update `rtaRepresents()` in `eval/scoring.go` to accept `StructFieldFuncValue` and `StructLiteralFieldAssignment`.
- [ ] **1.10** **Delta evaluation**: run all 72 traces with `ModeStructField`. Save to `docs/research/runs/SPRINT-0036-phase1-structfield.json`. Report which of the 22 blocked traces are now reachable.

**Adaptive checkpoint**: If ≥18 of 22 traces are now reachable, Phase 2 predicates can be simplified (just the table definition, minimal lookup). If <5, predicates are the critical path — prioritize accordingly.

### Phase 2 — Framework predicate registry

When the generic pass finds stores but can't find the framework's internal read-site, predicates bridge the gap.

- [ ] **2.1** Define `FrameworkPredicate` type in `pkg/activation/predicates.go`:
  ```go
  type FrameworkPredicate struct {
      ImportPath  string // e.g., "github.com/spf13/cobra"
      TypeName    string // e.g., "Command"
      FieldName   string // e.g., "RunE"
      DispatchFn  string // e.g., "(*Command).execute"
  }
  ```
- [ ] **2.2** Implement predicate matching: after the struct-field pass, for each `(structType, fieldIndex) → functions` mapping, check if any registered predicate matches by type name and field name (match by SSA type identity, not string). If so, find or create a node for the predicate's dispatch function, add `StructFieldFuncValue` edges from that dispatch node to each stored function.
- [ ] **2.3** Dispatch-function lookup: search graph nodes by `FunctionKey{PackagePath, Receiver, FuncName}`. If not found, scan `program.SSAPackages` for the function. If still not found, fall back to generic read-side edges only and log a diagnostic.
- [ ] **2.4** Add cobra predicates:
  - `spf13/cobra.Command.RunE` → dispatched from `(*Command).execute`
  - `spf13/cobra.Command.Run` → dispatched from `(*Command).execute`
- [ ] **2.5** Add urfave/cli v3 predicates:
  - `urfave/cli/v3.Command.Action` → dispatched from `(*Command).Run`
  - `urfave/cli/v3.App.Action` → dispatched from `(*App).RunContext`
- [ ] **2.6** Scan remaining miss details — if additional framework fields appear as blockers, add predicates. Only add rows with corpus evidence.
- [ ] **2.7** Fixture test: mock framework struct with `Handler func()` field and a registered predicate. Verify predicate creates the correct edge.
- [ ] **2.8** **Focused evaluation**: run caddy, gitea, and mattermost only with `ModePredicates`. Save to `docs/research/runs/SPRINT-0036-phase2-predicates-focused.json`. This catches predicate bugs fast on the three projects that matter.
- [ ] **2.9** **Full evaluation**: run all 72 traces with `ModePredicates`. Save to `docs/research/runs/SPRINT-0036-phase2-predicates.json`. Report which of the original 22 blocked traces remain blocked.

### Phase 3 — Goroutine-launch edges

29 goroutine-launch edges in the corpus. Cheap, independently valuable for path precision.

- [ ] **3.1** Implement `AugmentGoroutine(graph *Graph, program *Program)` in `pkg/activation/goroutine.go`: iterate all instructions in reachable functions. For each `*ssa.Go`, add a `GoroutineLaunch` edge from the containing function to the goroutine body.
- [ ] **3.2** Handle `go obj.Method()`: resolve to the concrete method, not a wrapper.
- [ ] **3.3** Handle `go func() { ... }()`: target the anonymous function body.
- [ ] **3.4** Handle `go namedFunc()`: direct static callee.
- [ ] **3.5** Deduplicate: check for existing `(from, to, GoroutineLaunch)` before adding.
- [ ] **3.6** Update `rtaRepresents()` to accept `GoroutineLaunch`.
- [ ] **3.7** Fixture tests in `pkg/activation/testdata/goroutine/`: `direct/`, `method/`, `closure/`. Verify edge kind, target, and source position.
- [ ] **3.8** **Cumulative evaluation**: run all 72 traces with `ModeAll`. Save to `docs/research/runs/SPRINT-0036-phase3-goroutine.json`. Report Tier 1 and Tier 2 delta separately — goroutine edges may not move Tier 1 but should improve Tier 2.

### Phase 4 — Partial-path support

When the path search fails, report how far it got with a labeled gap.

- [ ] **4.1** Define types in `pkg/activation/partial.go`:
  ```go
  type Gap struct {
      AfterStep    int       // last resolved step index in the trace
      ExpectedEdge string    // edge type the trace says should come next
      Reason       string    // labeled gap reason
  }
  type PartialPath struct {
      Prefix *Path
      Gap    Gap
  }
  ```
- [ ] **4.2** Define gap reason labels: `struct-field-not-resolved`, `framework-predicate-not-registered`, `closure-capture-deferred`, `channel-flow-deferred`, `http-registration-deferred`, `reflection-deferred`, `string-keyed-registry-deferred`, `cross-process-deferred`, `target-not-loaded`, `unknown-unreachable`.
- [ ] **4.3** Implement `FindPartialPath(graph, trace)`: walk the trace's expected steps, check if the graph has an edge from the current node to the next expected function. Stop at the first gap. Return the prefix and the gap info. Use fuzzy `FunctionKey` matching.
- [ ] **4.4** Add `PartialPath *PartialPath` field to `Result`. When `ShortestPath` returns `found=false`, attempt `FindPartialPath` (only in eval context where trace is available).
- [ ] **4.5** Update `eval/scoring.go:TraceResult` with `PartialSteps`, `TotalExpectedSteps`, `GapReason`.
- [ ] **4.6** Update `WriteMarkdown()` to show partial-path stats per miss: `resolved X/Y steps, gap: <reason>`.
- [ ] **4.7** Verify caddy/M-4 emits a `string-keyed-registry-deferred` gap, not a generic miss.
- [ ] **4.8** Fixture test: program where path goes `main → A → B → [gap] → C`. Verify partial path is `main → A → B` with correct gap.

### Phase 5 — Final evaluation and closeout

- [ ] **5.1** Run full evaluation with `ModeAll` against all 72 traces. Save to `docs/research/runs/SPRINT-0036-final.json`.
- [ ] **5.2** Verify determinism: run full evaluation twice and diff JSON.
- [ ] **5.3** Write `docs/research/runs/SPRINT-0036-augmentation-report.md` with:
  - Phase-over-phase Tier 1 progression (baseline → +struct-field → +predicates → +goroutine)
  - Per-project Tier 1/Tier 2 tables
  - Resolution table for the 22 SPRINT-0035 `StructFieldFuncValue` blockers (resolved / still-blocked / partial-gap)
  - Partial-path statistics on still-missed traces
  - Remaining blocker edge types ranked by trace count
  - Note on mattermost/M-4 (target-not-found, not a graph issue)
- [ ] **5.4** Run `go vet ./pkg/activation/...` and the import-guard test.
- [ ] **5.5** Run `go test ./pkg/activation/... ./cmd/activation-path/...`.
- [ ] **5.6** Update this sprint file with final numbers and recommendation for the next sprint.

## Sequencing

```
Phase 0 (harness) → Phase 1 (struct-field) → [evaluate + checkpoint]
                                                      ↓
                                     Phase 2 (predicates) → [evaluate]
                                                      ↓
                                     Phase 3 (goroutine) → [evaluate]
                                                      ↓
                                     Phase 4 (partial paths) → Phase 5 (closeout)
```

- **Phase 0** must complete first — establishes augmentation modes and verifies baseline reproduction.
- **Phase 1** depends on Phase 0's graph mutation API.
- **Phase 2** consumes Phase 1's store index — predicates add edges for struct-field stores the generic pass found but couldn't connect to a dispatch site.
- **Phase 3** is logically independent but sequenced after Phase 2 for cumulative delta measurement.
- **Phase 4** is diagnostic — sequenced last so partial paths reflect the final augmented graph.
- **Adaptive checkpoint after Phase 1**: if generic struct-field alone unblocks ≥18 traces, Phase 2 predicates can be simplified. If <5, predicates are critical path.

## Risks

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Generic struct-field tracking produces false edges (allocation-insensitive: any function stored in field F is a candidate at every load of F) | Medium | BFS finds wrong paths, Tier 2 degrades | Type-filter by signature compatibility. If still noisy, add per-package scoping. Report edge count per field key in delta notes. |
| RTA graph doesn't include cobra/urfave internal dispatch functions | Medium | Generic pass finds stores but no read sites; Phase 1 delta is zero | This is exactly why Phase 2 predicates exist. Measure Phase 1 delta to decide Phase 2 priority. |
| Caddy's `WrapCommandFuncForCobra` wrapping breaks generic tracking | High | Caddy traces remain blocked despite struct-field pass | Task 1.4 adds narrow wrapper-return recognizer (single-return, immediate-delegate only). |
| BFS performance degrades with denser graph | Low | Wall-clock increase on large codebases | Monitor wall-clock in delta evaluations. RTA pruning limits base graph; struct-field adds O(stores × loads) per field key. |
| Goroutine edges don't move Tier 1 | Medium | Phase 3 shows no reachability delta | Expected — goroutines improve Tier 2 (path precision). Report Tier 1 and Tier 2 separately. |
| Predicate dispatch function not found in graph or loaded packages | Medium | Predicate silently does nothing | Task 2.3 falls back to generic edges + logs diagnostic. Report unresolved predicates. |
| Mattermost M-4 remains target-not-found | High | 1 trace permanently missed | Expected — enterprise package. Document separately from graph-edge misses. |

## Acceptance criteria

1. RTA-only mode reproduces SPRINT-0035 baseline (49/72) — no regression.
2. Struct-field tracking has fixture tests for direct assignment, struct literal, method value, and wrapper-return.
3. Framework predicate registry has cobra `RunE`/`Run` and urfave/cli v3 `Action` entries, with fixture tests.
4. Goroutine-launch edges have fixture tests for named function, method, and closure launches.
5. ≥18 of 22 `StructFieldFuncValue`-blocked traces are now reachable (target: corpus ≥ 93%).
6. Phase-over-phase delta JSONs exist under `docs/research/runs/SPRINT-0036-*`.
7. Still-missed traces emit partial paths with labeled gaps.
8. Final report includes the 22-blocker resolution table, per-project metrics, and ranked remaining gaps.
9. `pkg/activation/` maintains zero imports from `pkg/compiler/entrypath/`.
10. Evaluation is deterministic (two runs produce identical JSON).

## Expected outcome

Best case: 20 of 22 `StructFieldFuncValue` traces unblocked (excluding mattermost/M-4 target-not-found and possible edge cases), reaching **69/72 reachable (95.8%)**. Caddy/M-4 (init-populated-registry) emits a clean partial path. Goroutine edges improve mean Tier 2 fuzzy from 0.199 to ~0.25–0.35.

## Follow-up (deferred)

Based on expected remaining gaps:
- **Closure capture** (~12 corpus edges) — next most common non-RTA edge type
- **Channel-flow tracking** (8 corpus edges) — queue-worker patterns
- **HTTP handler registration** (19 corpus edges) — may partially overlap with struct-field tracking
- **Additional framework predicates** — gorilla/mux, net/http.ServeMux, gocron
- **Tier 2 path-quality focus** — once Tier 1 is high, the work shifts to finding the *right* path
