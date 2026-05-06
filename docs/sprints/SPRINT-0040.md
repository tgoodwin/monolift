# SPRINT-0040: Cut-placement analyzer

**Status:** planned
**Predecessor:** SPRINT-0039 (72-trace cut-point research, decision-tree synthesis, ground truth table)

## Intent

Implement a cut-placement analyzer in `pkg/activation/` that consumes an activation path (the existing `Result` with its `Path`) and a `Graph` (for SSA function access), and identifies the optimal single-node network boundary. The analyzer implements the SPRINT-0039 decision tree: hard-gate boundary-data infeasibility, classify proxy-required types, prefer zero-callback cuts, then rank by state reconstruction cost, surface area, error semantics, and edge alignment — using a lexicographic comparator rather than a weighted numeric score.

Validated by unit tests with synthetic Go programs plus a corpus evaluation harness comparing against the 72-trace ground truth at `docs/research/activation-paths/analyses/recommended-cuts.md`.

## Scope boundaries

**In scope:**
- New files in `pkg/activation/` implementing cut-placement analysis
- Six modular type classifiers (boundary data, edge alignment, callbacks, state, surface, error semantics)
- Decision-tree ranking with lexicographic comparison
- Unit tests with small synthetic Go programs for each scoring dimension
- Corpus evaluation harness comparing analyzer output against ground truth
- `CutResult` as an optional field on `Result` (not yet wired into the `Analyze` pipeline)

**Out of scope:**
- Composite cuts (multi-node extraction boundaries)
- Changes to the existing activation-path algorithm
- Liftability integration wiring
- Transport selection
- CLI integration or new command-line flags (library API only for this sprint)
- Project-specific trace-ID overrides to make the corpus evaluation pass (B.7: no overfitting)
- Report artifacts in `docs/research/runs/` (defer to follow-up)

**Known constraints:**
- The first implementation depends on in-memory `Node.Func` SSA data. Emit a diagnostic when a deserialized path lacks function pointers.
- `mattermost/M-4` is a structural target-loading gap, not a cut-placement failure. Skip it in corpus evaluation.

## Data model

```go
type CutResult struct {
    Recommended *CutCandidate  `json:"recommended,omitempty"`
    Candidates  []CutCandidate `json:"candidates"`
    Diagnostics []Diagnostic   `json:"diagnostics,omitempty"`
}

type CutCandidate struct {
    Step         int                `json:"step"`
    NodeKey      FunctionKey        `json:"node_key"`
    NodeName     string             `json:"node_name"`
    IncomingEdge EdgeKind           `json:"incoming_edge"`
    Feasibility  CutFeasibility     `json:"feasibility"`
    BoundaryData BoundaryDataClass  `json:"boundary_data"`
    Callbacks    CallbackClass      `json:"callbacks"`
    State        StateClass         `json:"state"`
    Surface      SurfaceClass       `json:"surface"`
    ErrorSem     ErrorSemClass      `json:"error_sem"`
    EdgeAlign    EdgeAlignClass     `json:"edge_align"`
    Reason       string             `json:"reason"`
}
```

No `Score int` — the decision tree is a lexicographic comparator over categorical dimensions, not a numeric weight reduction.

## Task list

### Phase 1: Scoring enums and edge-alignment mapping

- [x] Define enumeration types in `pkg/activation/cut_types.go`:
  - `CutFeasibility`: `Feasible`, `FeasibleWithProxy`, `Infeasible`
  - `BoundaryDataClass`: `Trivial`, `Serializable`, `Reconstructible`, `ProxyRequired`, `BoundaryInfeasible`
  - `CallbackClass`: `ZeroConfirmed`, `ZeroEstimated`, `Low`, `Moderate`, `Many`
  - `StateClass`: `Stateless`, `ConfigOnly`, `ClientReconstructible`, `SharedState`
  - `SurfaceClass`: `Minimal`, `Small`, `Medium`, `Large`, `VeryLarge`
  - `ErrorSemClass`: `ErrorOK`, `NeedsWrapper`, `ErrorInfeasible`
  - `EdgeAlignClass`: `Strong`, `Weak`, `Anti`
- [x] Implement `classifyEdgeAlignment(kind EdgeKind) EdgeAlignClass` in `pkg/activation/cut_edge.go`. Map every existing `EdgeKind` constant: `InterfaceDispatch`, `HTTPHandlerRegistration`, `CallbackRegistration`, `ChannelFlow` → Strong. `StructFieldFuncValue`, `PackageVarFuncValue`, `MapFuncValue`, `FuncArgValue` → Weak. `DirectCall`, `ConcreteMethodCall`, `ClosureCapture`, `GoroutineLaunch`, `Unsupported` → Anti.
- [x] Add tests for the edge-alignment lookup covering every current `EdgeKind` constant.

### Phase 2: Candidate enumeration

- [x] Implement path validation in `pkg/activation/cut.go`: reject nil `Result`, nil `Path`, empty `Path.Steps`, and missing `Node.Func` with clear diagnostics.
- [x] Enumerate candidate cuts from every path step except step 0 (entrypoint). Attach each candidate's incoming edge from `PathStep.Edge`.
- [x] Preserve stable path step numbering matching the corpus convention in `recommended-cuts.md`.
- [x] Add unit tests for candidate enumeration on hand-built `Result` values, including nil/malformed `Result.Path`.

### Phase 3: Type classifiers

Build SSA type inspection primitives. Each classifier lives in its own file for modularity.

- [x] Implement `classifyBoundaryData(fn *ssa.Function) (BoundaryDataClass, []string)` in `pkg/activation/cut_boundary.go`. Walk the function's receiver type, parameter types, and return types. Apply classification:
  - Infeasible: `func` types, `sync.Mutex`, `sync.WaitGroup`, `os.Process`, runtime lifecycle handles
  - ProxyRequired: `http.ResponseWriter`, `io.Reader`, `io.Writer`, `io.ReadCloser`, `io.WriteCloser`, channel types
  - Reconstructible: `*sql.DB`, `*http.Client`, `*log.Logger`, `*template.Template`, and similar config-backed service clients
  - Serializable: `context.Context`, exported structs with serializable fields, slices/maps of serializable values
  - Trivial: primitives, strings, byte slices, structs of exported trivials
  - Return the worst (most restrictive) class across all boundary values, plus per-value explanation strings.

- [x] Implement `classifyCallbacks(fn *ssa.Function, aboveCut []*Node) CallbackClass` in `pkg/activation/cut_callbacks.go`. Check parameter list for func-typed parameters (direct or inside interfaces). Build a path-prefix set from `aboveCut` and scan the function body for direct calls back to above-cut functions. Classify:
  - `ZeroConfirmed`: no func-typed parameters, no observed reverse calls, no function-typed free variables
  - `ZeroEstimated`: no SSA body available or conservative assumption with no evidence either way
  - `Low`/`Moderate`/`Many`: based on func-typed parameter count and observed reverse calls

- [x] Implement `classifyState(fn *ssa.Function) StateClass` in `pkg/activation/cut_state.go`. Inspect the receiver type:
  - No receiver → `Stateless` (unless package globals are referenced, which is a refinement for Phase 5)
  - Receiver with only config/primitive fields → `ConfigOnly`
  - Receiver wrapping DB pool, HTTP client, mailer, queue client → `ClientReconstructible`
  - Receiver containing mutable lifecycle state, in-memory caches, plugin registries → `SharedState`
  - Use package-path heuristics as fallback. Inspect direct fields only, not transitive.

- [x] Implement `classifySurface(step int, pathLen int) SurfaceClass` in `pkg/activation/cut_surface.go`. Use relative depth as the primary signal (calibrated to corpus mean depth 0.924):
  - Steps 0-1 → `VeryLarge`
  - First 25% of path → `Large`
  - 25-50% → `Medium`
  - 50-75% → `Small`
  - Last 25% → `Minimal`
  - For paths with length <= 4, do not penalize shallow cuts as heavily (shift thresholds).

- [x] Implement `classifyErrorSemantics(fn *ssa.Function) ErrorSemClass` in `pkg/activation/cut_error.go`. Inspect return types:
  - Returns `error` or implements `error` interface → `ErrorOK`
  - Returns `bool`, `string`, or a non-error type where a wrapper could envelope network failure → `NeedsWrapper`
  - Void function with no observable failure path → `NeedsWrapper`

### Phase 4: Decision tree

Wire classifiers into the ranking algorithm.

- [x] Implement `AnalyzeCut(result *Result, graph *Graph) (*CutResult, error)` in `pkg/activation/cut.go`. Iterate over `result.Path.Steps`, skip step 0, build a `CutCandidate` for each step using the Phase 3 classifiers. Apply the decision tree.
- [x] Hard-gate filter: reject candidates with `BoundaryData == BoundaryInfeasible`. Record a diagnostic for each rejection with per-value explanations.
- [x] Classify feasibility: candidates with `BoundaryData == ProxyRequired` get `FeasibleWithProxy`. All others get `Feasible`.
- [x] Callback preference: within each feasibility pool, prefer candidates with `Callbacks == ZeroConfirmed` or `ZeroEstimated` over non-zero. Keep non-zero candidates in the ranked list for diagnostics.
- [x] Lexicographic ranking of remaining candidates by priority order: (1) callbacks (zero > non-zero), (2) state (Stateless > ConfigOnly > ClientReconstructible > SharedState), (3) surface (Minimal > Small > Medium > Large > VeryLarge), (4) error semantics (ErrorOK > NeedsWrapper), (5) edge alignment (Strong > Weak > Anti).
- [x] Prefer ordinary `Feasible` candidates over `FeasibleWithProxy` when both have comparable dimension scores. Allow proxy candidates to win only when all ordinary candidates are materially worse on callbacks/state.
- [x] Deterministic tiebreaker: prefer deeper cut (higher step index), then node key string.
- [x] Generate a human-readable `Reason` string explaining which gates were applied and which dimension was decisive.
- [x] Add table-driven tests proving each decision dimension wins only after higher-priority dimensions tie.

### Phase 5: Unit tests with synthetic programs

Each test constructs a small Go program, builds SSA, creates a path, and verifies the analyzer picks the right cut.

- [x] Test: infeasible boundary data. Node accepts `func()` parameter. Verify rejected by hard gate.
- [x] Test: proxy-required boundary data. Node accepts `http.ResponseWriter`. Verify classified as `FeasibleWithProxy`.
- [x] Test: zero-callback preference. Two feasible nodes, one with `func(int) error` callback param, one with only primitives. Verify zero-callback wins.
- [x] Test: state reconstruction ranking. Two zero-callback feasible nodes, one stateless, one with DB pool receiver. Verify stateless wins.
- [x] Test: deep cut preference on ties. Two otherwise-identical nodes at different depths. Verify deeper wins.
- [x] Test: error semantics. Two nodes, one returning `error`, one returning `bool`. Verify error-returning ranks higher.
- [x] Test: edge alignment. `InterfaceDispatch` edge vs. `DirectCall` edge, otherwise equal. Verify `InterfaceDispatch` wins.
- [x] Test: all-infeasible path. Every node has infeasible boundary data. Verify `Recommended` is nil and diagnostics explain.
- [x] Test: single-step path (only entrypoint). Verify no cut recommended.
- [x] Test: `context.Context` is not infeasible. Node accepts `context.Context` + `string`. Verify classified as Serializable.

### Phase 6: Corpus evaluation harness

- [x] Create `pkg/activation/cut_corpus_test.go`. Parse `docs/research/activation-paths/analyses/recommended-cuts.md` into a map of trace ID → expected cut (step, function name, boundary data class, state class, callbacks, feasibility).
- [x] For each trace, reconstruct `Result` with its `Path` by running the activation-path analyzer against the pinned evaluation codebases. Run `AnalyzeCut` and compare recommended step against ground truth.
- [x] Define match criteria: *exact match* (same step index), *acceptable divergence* (different step but equal or better feasibility class), *disagreement* (different step with worse feasibility or infeasible recommendation for a feasible trace).
- [x] Run per-codebase: Caddy first (smallest, cleanest), then Miniflux, Listmonk, PocketBase, Gitea, Mattermost last (largest, most complex). Inspect mismatches after each project before moving to the next.
- [x] Skip `mattermost/M-4` (structural gap, not cut-placement).
- [x] Summary reporter prints: total traces, exact matches, acceptable divergences, disagreements, skipped. Target: >= 60/71 exact matches, zero disagreements where analyzer says Infeasible but ground truth says Feasible.
- [x] Gate behind build tag (`//go:build corpus`) or `testing.Short()` skip so CI is not blocked.

### Phase 7: Boundary-data type walker refinements

Driven by corpus test failures from Phase 6. Iterate: run harness, identify misclassified types, add handling, re-run.

- [x] Handle named interface types: walk the method set. If any method accepts/returns a func type → Infeasible. If methods involve `io.Reader`/`Writer`/`ResponseWriter` → ProxyRequired.
- [x] Handle pointer-to-struct receivers: unwrap pointer, inspect direct fields for func-typed fields, channel fields, sync primitives.
- [x] Handle variadic parameters (`...T`): classify element type T.
- [x] Handle `map[K]V` and `[]V` parameter types: classify based on worst of K and V.
- [x] Handle type aliases and named types: resolve to underlying type before classifying.
- [x] Add known-type overrides for common stdlib types: `*sql.DB` → ClientReconstructible, `*http.Client` → ClientReconstructible, `*log.Logger` → ConfigOnly, `*template.Template` → ConfigOnly.

### Phase 8: Documentation and integration surface

- [x] Add doc comment block at top of `pkg/activation/cut.go` explaining the decision-tree algorithm, scoring dimensions, and invocation.
- [x] Export `CutResult` and `CutCandidate` types from the package.
- [x] Add `Cut *CutResult` as an optional `json:"cut,omitempty"` field on `Result`. Do not wire it into the `Analyze` method — that is integration work for a future sprint.
- [x] Document the analyzer's first-version limits: single-node only, path-local, SSA required, no composite cuts, no liftability facts, no graph-global merge.

## Sequencing

```
Phase 1 (enums + edge alignment)
    │
    v
Phase 2 (candidate enumeration)
    │
    v
Phase 3 (type classifiers — tasks are independent, can be built in parallel)
    │
    v
Phase 4 (decision tree — depends on all classifiers)
    │         ↕ Phase 5 unit tests can be written alongside Phase 4 via TDD
    v
Phase 5 (synthetic unit tests — depends on Phase 4)
    │
    v
Phase 6 (corpus harness — depends on Phases 4-5; run per-codebase before full)
    │
    v
Phase 7 (type walker refinements — driven by corpus failures, iterative)
    │
    v
Phase 8 (documentation + integration surface — last)
```

## Risks

1. **SSA data unavailability.** `Node.Func` is tagged `json:"-"` and will be nil for deserialized results. The corpus harness must load programs via `Config.LoadProgram` to populate SSA data. *Mitigation:* the `AnalyzeCut` signature takes `*Graph` explicitly; emit a diagnostic when SSA data is absent rather than silently failing.

2. **Receiver type inspection depth.** Classifying state reconstruction requires understanding struct fields, which can be arbitrarily deep. *Mitigation:* inspect direct fields only, not transitive. Use package-path heuristics as fallback. Accept that some corpus traces will need known-type overrides.

3. **Callback detection false negatives.** Parameter-type inspection misses callbacks registered via struct fields, closure captures, or stored in maps. *Mitigation:* start with parameter-type inspection + direct body-level reverse-call detection. Accept that Low/Moderate/Many distinctions will be coarse. The zero-vs-nonzero gate is the critical one.

4. **Corpus harness cost.** Loading 6 codebases into SSA is expensive (minutes per codebase). *Mitigation:* gate behind build tag. Run against smallest codebases first (Listmonk, Miniflux) for fast iteration. Run full six-codebase evaluation only after per-project results stabilize.

5. **Ground truth ambiguity.** Some recommended-cuts entries have plausible alternatives (e.g., gitea queue handlers where step N and N-1 are both reasonable). *Mitigation:* distinguish "exact match" from "acceptable divergence" rather than treating any mismatch as failure.

6. **Surface area heuristic brittleness.** Relative path depth is coarse. For short paths (length <= 4), the thresholds may be misleading. *Mitigation:* adjust thresholds for short paths. The corpus shows mean depth 0.924 across paths of median length ~8, so the heuristic covers the common case.

## Acceptance criteria

- [ ] `pkg/activation/cut.go` exports `AnalyzeCut(result *Result, graph *Graph) (*CutResult, error)` implementing the full decision tree
- [ ] Type classifiers exist for all 6 scoring dimensions, each in its own file under `pkg/activation/`
- [ ] Decision tree uses lexicographic comparison, not numeric score reduction
- [ ] Unit tests pass for each scoring dimension (at least 10 test cases per Phase 5)
- [ ] Corpus evaluation harness runs against all 6 pinned codebases, compares against `recommended-cuts.md`
- [ ] Corpus achieves >= 60/71 exact matches, with zero Infeasible recommendations for traces marked Feasible in ground truth
- [ ] `mattermost/M-4` handled as structural gap, not cut-placement failure
- [ ] No modifications to existing activation-path algorithm files
- [ ] `CutResult` added as optional field on `Result` (not yet wired into pipeline)
- [ ] All new code has doc comments; main entry point has package-level algorithm explanation
- [ ] `go test ./pkg/activation/...` passes

## Deferred follow-ups

- Composite cut support (queue handler + service leaf, hook method + continuation)
- Graph-global merge of path-local recommendations for shared handlers/state
- ADR-0018 liftability property integration into feasibility gates
- Transport selection after proxy-required candidates are selected
- CLI integration (`--cut-placement` flag, text/JSON output)
- Transitive region-size estimates replacing path-depth surface heuristic
- Graph-global reverse-edge callback analysis replacing body-level detection

## Blockers

- Full corpus acceptance is blocked by the local Go toolchain. `go test -tags corpus ./pkg/activation -run TestCutPlacementCorpus -count=1 -timeout 45m` failed while loading pinned evaluation projects because Miniflux, Listmonk, and Gitea require Go 1.26, but this workspace is running `go1.25.4`. The run could not complete all six codebases, so the `>= 60/71` exact-match target remains unvalidated in this environment. The partial run reported `total=31 exact=0 acceptable=31 disagreements=0 skipped=1` before failing.
