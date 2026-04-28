# SPRINT-0026 - Boundary-frontier EntryPath diagnostic

**Status:** done
**Predecessor:** SPRINT-0025 (targeted EntryPath diagnostics). SPRINT-0025 showed: `all` mode recovers the Mattermost target chain but is too expensive; `reverse-path` is cheap but misses `connectWebSocket`; current `http-sinks` / `targeted` spend the budget in whole-program boundary discovery before useful seeded scanning.

## Intent

Can a protocol-agnostic **invocation-boundary frontier search** recover the Mattermost HTTP registration chain without whole-program boundary scanning? This sprint is diagnostic only. It should clarify whether a bounded frontier from region roots plus boundary predicates can find `connectWebSocket -> APIHandlerTrustRequester -> http.Handler` registration under a reasonable incremental cost gate.

## Vocabulary

- **InvocationBoundary:** semantic place where external control reaches owned code.
- **BoundaryPredicate:** pluggable detector for a boundary family. First implementation: `net/http`.
- **BoundarySeed:** owner/instruction selected because a boundary predicate matched.
- **RegistrationSite:** source location/instruction where a callable is registered.
- **ValueSink:** internal value-flow endpoint where a function value lands.
- **SeedSet:** generic worklist input for bounded search.

## Non-goals

- No reportv2 schema changes.
- No `surface.DeriveWithTrace`.
- No `entrypath.Pass` promotion or InvocationTrace artifact.
- No emission work.
- No ADR and no files under `docs/decisions/`.
- No Mattermost-specific analyzer branch, package-name check, route-name check, or framework recognizer.
- No package-pruning strategy that changes analysis semantics.
- No attempt to make Mattermost fully pass beyond recording diagnostic evidence.

## Phase 0 - Baseline and Naming

- [x] **0.1** Create `docs/research/runs/SPRINT-0026-boundary-frontier-baseline.md` summarizing SPRINT-0025 results and the precise diagnostic question.
- [x] **0.2** Introduce boundary vocabulary in `pkg/compiler/entrypath` types or comments without breaking existing CLI behavior.
- [x] **0.3** Keep existing `http-sinks` CLI compatibility if present, but document `boundary` as the conceptual name for new work.

## Phase 1 - Boundary Predicate Abstraction

- [x] **1.1** Add a small `BoundaryPredicate` abstraction in `pkg/compiler/entrypath` that is protocol-agnostic.
- [x] **1.2** Implement `net/http` as the first boundary predicate using existing structural predicates: `http.Handler`, `http.HandlerFunc`, `*http.Server`, and `ServeHTTP`-shaped interfaces.
- [x] **1.3** Ensure predicate evidence records owner function, instruction, static type, and reason.
- [x] **1.4** Add tests proving the `net/http` predicate finds HTTP-shaped boundaries and ignores unrelated callbacks without framework names.

## Phase 2 - Frontier Boundary Discovery

- [x] **2.1** Add `--boundary-discovery-mode=all|frontier`, defaulting to current behavior.
- [x] **2.2** Implement frontier discovery from reverse-path owners plus bounded callgraph-adjacent owners.
- [x] **2.3** Add knobs: `--boundary-frontier-max-owners`, `--boundary-frontier-depth`, and `--boundary-frontier-max-packages` or equivalent bounded controls.
- [x] **2.4** Emit separate phase timings/stats for reverse frontier, adjacent expansion, boundary predicate scanning, seed-set assembly, and final seeded index scan.
- [x] **2.5** Add stop diagnostics for owner budget, depth budget, package budget, and duration budget.
- [x] **2.6** Add a fixture where frontier boundary discovery succeeds without whole-program boundary scanning.

## Phase 3 - Mattermost Ladder

Run a staged ladder, not a full Cartesian matrix. Stop early if frontier evidence is not moving toward the target or if budgets explode.

- [x] **3.1** Run depth 1 / 500 owner budget and record boundary seeds plus target evidence.
- [x] **3.2** Run depth 1 / 5k owner budget and record the same metrics.
- [x] **3.3** Run depth 2 / 5k owner budget and record the same metrics.
- [x] **3.4** Run depth 2 / 10k owner budget only if prior rows are close or informative.
- [x] **3.5** Run depth 3 / 10k owner budget only if prior rows show movement toward the target.
- [x] **3.6** Record closeness indicators: `channels/api4` reached, `connectWebSocket` found, `APIHandlerTrustRequester` found, registration owner found, `http.Handler` sink found, shortest observed edge chain if available, and top missing edge/stop reason.

## Phase 4 - Synthesis

- [x] **4.1** Create `docs/research/runs/SPRINT-0026-boundary-frontier-matrix.md`.
- [x] **4.2** Add rows for every ladder run with owner count, package count, boundary seeds, index time, peak RSS, and closeness indicators.
- [x] **4.3** Answer the sprint question directly: can boundary-frontier discovery recover the Mattermost target evidence without whole-program boundary scanning?
- [x] **4.4** Recommend exactly one next step: implementation sprint, one more specific diagnostic, or structural redesign.
- [x] **4.5** Add a cost-gate recommendation using SPRINT-0025's split gate as the baseline: load/SSA/root/callgraph under 90s and 8 GB RSS, incremental boundary EntryPath under 30s and +1.5 GB RSS after callgraph.
- [x] **4.6** Update this file's closeout with phases run, phases cut, matrix link, and recommendation.
- [x] **4.7** Add `docs/evolution.md` narrative only if the result is stable enough to matter. Do not create an ADR.

## Guardrails

- [x] **G.1** Keep forbidden-string lint passing against `pkg/compiler/entrypath/*.go` non-test sources. Banned strings: `"websocket"`, `"Mux"`, `"HandleFunc"`, `"mattermost"`, `"gorilla"`, `"chi"`, `"echo"`, `"gin"`. Allowlist remains `"http.Handler"`, `"ServeHTTP"`, `"net/http"`.
- [x] **G.2** Run focused tests for `pkg/compiler/entrypath` and `cmd/entrypath-probe`.
- [x] **G.3** Verify no files under `pkg/compiler/reportv2/`, `pkg/compiler/surface/`, `pkg/compiler/transport/emit/`, `pkg/compiler/extract/bootpath/`, `evaluation/mattermost/`, or `docs/decisions/` changed.
- [x] **G.4** If a frontier mode appears to recover Mattermost target evidence, stop at recording evidence. Do not start report wiring or surface classification.

## Acceptance Criteria

- [x] Boundary terminology and predicate abstraction exist without Mattermost/framework coupling.
- [x] Frontier boundary discovery is measured separately from final seeded indexing.
- [x] At least three ladder rows are recorded unless an early stop condition is documented.
- [x] The matrix includes closeness indicators, not only binary pass/fail.
- [x] Recommendation names exactly one next step.
- [x] Focused tests pass, or failures are recorded with relevance.
- [x] Frozen boundaries and forbidden-string guardrails hold.

## Closeout

### Phases Run

- Phase 0 baseline and vocabulary.
- Phase 1 protocol-agnostic BoundaryPredicate abstraction with `net/http` as
  the first implementation.
- Phase 2 boundary-frontier discovery, CLI knobs, phase stats, stop
  diagnostics, and frontier fixture coverage.
- Phase 3 Mattermost ladder rows: depth 1 / 500, depth 1 / 5k, depth 2 / 5k,
  and depth 2 / 10k.
- Phase 4 matrix, answer, recommendation, and cost gate.

### Phases Cut

- Depth 3 / 10k Mattermost row. Prior rows showed no target movement and
  demonstrated that reverse-frontier owners consumed the budget before
  adjacent expansion contributed.
- `docs/evolution.md` narrative. The result is diagnostic and negative, not a
  stable architectural milestone.

### Recommendation

Run one more specific diagnostic: a budget-partitioned frontier that reserves
separate owner budgets for reverse-frontier owners, adjacent expansion owners,
and BoundaryPredicate scan candidates. Do not start report wiring or surface
classification from this result.

### Pointers

- Baseline: `docs/research/runs/SPRINT-0026-boundary-frontier-baseline.md`
- Matrix: `docs/research/runs/SPRINT-0026-boundary-frontier-matrix.md`
- Closeness: `docs/research/runs/SPRINT-0026-boundary-frontier-closeness.md`
- Depth 3 cut note: `docs/research/runs/SPRINT-0026-boundary-frontier-d3-o10000-cut.md`

### Guardrail Notes

- The worktree already contains unrelated dirty files under frozen directories
  such as `docs/decisions/`, `pkg/compiler/reportv2/`,
  `pkg/compiler/transport/emit/`, `pkg/compiler/extract/bootpath/`,
  `pkg/compiler/surface/`, and `evaluation/mattermost/`. This sprint did not
  edit those paths.
