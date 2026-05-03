# SPRINT-0037 — Fix RTA-augmentation dead-end nodes + re-evaluate

**Status:** planned
**Executor:** TBD
**Predecessor:** SPRINT-0036 (augmentation passes implemented, 49/72 reachable unchanged due to dead-end problem)

## Problem statement

The pipeline runs `BuildRTAGraph() → Augment()` sequentially. RTA explores call trees from `main`. Then struct-field/predicate passes add edges to functions RTA never visited — e.g., cobra's `execute → cmdRun`. But `cmdRun`'s own callees (direct calls to `caddy.Load`, `caddy.Run`, etc.) were never explored by RTA, so `cmdRun` is a dead-end node: incoming edges, zero outgoing edges. BFS reaches it and stops.

Verified concretely: caddy `cmdRun` has **1 outgoing edge** (a goroutine launch) after augmentation when it should have **dozens**. `caddy.Load` is already in the RTA graph via other paths, but there is no edge from `cmdRun` to it.

The augmentation edges are correct — they just lead into unexplored territory.

## Goals

- [ ] **G1** Eliminate dead-end nodes: every function added by augmentation must have its transitive callees explored.
- [ ] **G2** Preserve `--augmentations rta` as the SPRINT-0035 baseline (49/72).
- [ ] **G3** Re-evaluate all 72 traces with augmentation + callee exploration.
- [ ] **G4** Achieve ≥60/72 reachable (83%+). Some traces will hit secondary blockers (HTTP registration, closure capture, channel flow).
- [ ] **G5** Identify and rank the new wall — classify what blocks remaining misses.

## Scope boundaries

**In scope:**
- Post-augmentation callee exploration via re-rooted RTA
- Iterative augment-explore convergence loop (new code may contain new struct-field stores)
- Full 72-trace re-evaluation with delta reporting
- New-wall analysis and blocker classification

**Out of scope:**
- HTTP handler registration edges (5 traces — future sprint)
- General closure-capture tracking (2 traces — future sprint)
- Channel-flow edges (1 trace — future sprint)
- Caddy M-4 init-populated string-keyed registry (not statically resolvable)
- Mattermost M-4 enterprise package (package-loading issue)
- Changes to `pkg/compiler/entrypath/` or downstream packages

## Approach

After `Augment()` adds new function nodes, collect the set of functions not in the original RTA graph. Run `rta.Analyze()` from those new roots only (not the union with original entrypoints — `main`'s callees are already explored, and re-running from `main` is pure waste). Convert the resulting call graph edges and merge into the existing graph. Then re-run augmentation passes on newly-added nodes, because struct-field stores in the newly-explored code may reveal further dispatch edges.

This requires iterating until no new functions are added. In practice, convergence is 1–2 rounds: the first round explores command handlers, the second explores anything they store into struct fields. A max-iteration cap prevents pathological cases.

Why not manual SSA body walking: RTA handles interface dispatch correctly — manual walking would need to replicate RTA's type-propagation logic. Why not interleaving with the RTA worklist: requires modifying `x/tools/go/callgraph/rta` internals, high risk for marginal benefit over re-rooted RTA.

## Task list

### Phase 0 — Reproduce and pin the bug

Confirm the dead-end behavior before writing any code. This is the diagnostic baseline.

- [ ] **0.1** Run `activation-path --target cmd/commandfuncs.go:172 --augmentations all` against caddy. Confirm path reaches `cmdRun` (6 steps).
- [ ] **0.2** Count outgoing edges from `cmdRun` under `--augmentations all`. Confirm it has only 1 (goroutine launch to `watchConfigFile`).
- [ ] **0.3** Run `activation-path --target caddy.go:115 --augmentations all`. Confirm `caddy.Load` is `target-unreachable` despite being present elsewhere in the RTA graph.
- [ ] **0.4** Save the reproduction evidence in the eventual SPRINT-0037 report.

### Phase 1 — Graph diffing and callee exploration

- [ ] **1.1** Add `Graph.FunctionSet() map[*ssa.Function]bool` — snapshots all SSA functions currently in the graph.
- [ ] **1.2** Add `Graph.NewFunctionsSince(before map[*ssa.Function]bool) []*ssa.Function` — returns functions added since the snapshot, sorted deterministically by `FunctionKey`.
- [ ] **1.3** Implement `ExploreCallees(graph *Graph, program *Program, roots []*ssa.Function) error` in `pkg/activation/explore.go`:
  1. If `roots` is empty, return nil (convergence reached).
  2. Call `rta.Analyze(roots, true)` to get a call graph rooted at the new functions.
  3. For each node/edge in the RTA result, call `graph.AddNode()` / `graph.AddEdge()` with edge classification via `classifyRTAEdge()`.
  4. Preserve source positions and edge descriptions from the re-rooted call graph.
- [ ] **1.4** Export `classifyRTAEdge` and `positionFor` (or extract into shared helpers) so `explore.go` can use them.
- [ ] **1.5** Unit test: build a program with `main → A` (RTA-reachable) and `B → C` (not RTA-reachable). Build RTA graph from `main`. Verify `C` is absent. Call `ExploreCallees([B])`. Verify `B → C` edge now exists.
- [ ] **1.6** Unit test: program where `B` calls through an interface implemented by `D`. Verify `ExploreCallees([B])` recovers the interface dispatch edge `B → D.Method`.

### Phase 2 — Iterative augment-explore loop

- [ ] **2.1** Modify `Augment()` in `augment.go` to implement the iterative loop:
  ```
  snapshot = graph.FunctionSet()
  run augmentation passes (struct-field, predicates, goroutine)
  newFuncs = graph.NewFunctionsSince(snapshot)
  while len(newFuncs) > 0:
      ExploreCallees(graph, program, newFuncs)
      snapshot = graph.FunctionSet()
      run augmentation passes again
      newFuncs = graph.NewFunctionsSince(snapshot)
  ```
- [ ] **2.2** Add max-iteration cap (10 rounds) with a diagnostic warning if hit.
- [ ] **2.3** Log iteration count per `Augment()` call for convergence diagnostics.
- [ ] **2.4** Ensure `ModeRTAOnly` bypasses the loop entirely — returns after initial RTA, no augmentation, no exploration.
- [ ] **2.5** Ensure per-mode semantics are correct: `ModeStructField` runs struct-field + exploration loop. `ModePredicates` runs struct-field + predicates + exploration loop. `ModeAll` runs all passes + exploration loop.
- [ ] **2.6** Verify idempotency: run `Augment` twice on the same graph. Node and edge counts must be identical.

### Phase 3 — Smoke tests

Gate on targeted probes before the full evaluation.

- [ ] **3.1** Caddy: run `--target cmd/commandfuncs.go:172 --augmentations all`. Confirm `cmdRun` has outgoing edges to its direct callees.
- [ ] **3.2** Caddy: run `--target caddy.go:115 --augmentations all`. This was `target-unreachable`. After the fix, it should find a path through `cmdRun → caddy.Load`. **If still unreachable, debug before proceeding.**
- [ ] **3.3** Caddy: run traces M-1, M-2, M-3, M-5, M-7 and record which move past the old `cmdRun` dead end.
- [ ] **3.4** Mattermost: run traces M-3, M-5, M-6, M-11, M-13, M-15 and record which move past `serverCmdF`.
- [ ] **3.5** Gitea: run traces M-13, M-16 and record whether downstream handler bodies are explored.

### Phase 4 — Full evaluation

- [ ] **4.1** Verify RTA-only mode still reproduces 49/72 (SPRINT-0035 baseline gate).
- [ ] **4.2** Run full 72-trace evaluation with `--augmentations all`. Save to `docs/research/runs/SPRINT-0037-full.json`.
- [ ] **4.3** Verify determinism: run evaluation twice, diff JSONs.
- [ ] **4.4** Compare against SPRINT-0035 baseline (49/72) and SPRINT-0036 final (49/72). Report:
  - Corpus-level: reachable count, rate, mean Tier 2 exact/fuzzy
  - Per-project: reachable count and rate
  - Delta table: which of the 22 struct-field-blocked traces are now reachable vs. still blocked
  - Status of the 8 SPRINT-0036 `target-unreachable` cases
- [ ] **4.5** Verify no regression: all 49 previously-reachable traces must still be reachable.
- [ ] **4.6** Spot-check 3–5 newly-reachable paths for plausibility (correct command-dispatch → handler → target chain).

### Phase 5 — New-wall analysis

- [ ] **5.1** For every still-blocked trace, classify the first blocker edge type. Produce a ranked table: HTTP registration, closure capture, channel flow, unsupported patterns, target-unreachable with no identified blocker.
- [ ] **5.2** Record partial-path depth for still-blocked traces — how far did we get? This shows whether the fix extended paths even for traces that remain blocked.
- [ ] **5.3** Check for Tier 2 regressions on previously-reachable traces (broader RTA roots causing wrong-path selection).
- [ ] **5.4** Rank remaining blocker classes by (a) trace count, (b) implementation feasibility, (c) cross-project impact. This is the next sprint's priority list.

### Phase 6 — Closeout

- [ ] **6.1** Write `docs/research/runs/SPRINT-0037-report.md` with:
  - Fix summary (re-rooted RTA from augmentation roots, iterative convergence)
  - Before/after corpus metrics
  - Per-project comparison table
  - Resolution table for the 22 struct-field-blocked traces
  - Status table for the 8 SPRINT-0036 `target-unreachable` cases
  - New-wall blocker ranking
  - Convergence statistics (rounds per project)
  - Recommendation for next sprint
- [ ] **6.2** Update `docs/research/activation-paths/README.md` sprint history with SPRINT-0037 summary.
- [ ] **6.3** Run `go vet ./pkg/activation/...` — clean.
- [ ] **6.4** Run `go test ./pkg/activation/... ./cmd/activation-path/...` — all pass.
- [ ] **6.5** Verify import guard: `pkg/activation/` has zero imports from `pkg/compiler/entrypath/`.

## Sequencing

```
Phase 0 (reproduce bug)
    ↓
Phase 1 (graph diffing + ExploreCallees)
    ↓
Phase 2 (iterative loop in Augment)
    ↓
Phase 3 (caddy/mattermost/gitea smoke tests)  ← gate: stop if caddy.Load still unreachable
    ↓
Phase 4 (full 72-trace evaluation)
    ↓
Phase 5 (new-wall analysis)
    ↓
Phase 6 (closeout)
```

Phases 0–2 are the core implementation (~60% of effort). Phase 3 is a fast feedback loop before committing to the full evaluation. Phases 5–6 are analysis and documentation.

## Risks

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Re-rooted RTA from ~20 command handlers pulls in large subtrees, inflating graph size | Low | Wall-clock increase on mattermost/gitea | The roots' callee trees overlap heavily with the existing graph; `AddNode`/`AddEdge` dedup handles this. Monitor node/edge counts before/after. |
| Iterative loop doesn't converge | Very low | Infinite loop | Max-iteration cap (task 2.2). In practice, convergence is 1–2 rounds. |
| Some struct-field traces hit secondary blockers (HTTP registration, closure capture) even after the dead-end fix | High (expected) | Reachability < 69/72 | Expected — G4 targets ≥60/72 to account for this. Task 5.1 classifies the secondary blockers. |
| New RTA edges introduce false positives, degrading Tier 2 on previously-reachable traces | Low | Path quality regression | Task 5.3 checks for Tier 2 regressions. RTA is sound — the struct-field pass's allocation-insensitivity is the real false-positive risk, unchanged. |
| Caddy M-4 and mattermost M-4 remain permanently blocked | Certain | 2 traces missed | Known — not graph-edge issues. Documented separately. |

## Acceptance criteria

1. `--augmentations rta` still reproduces 49/72 (no regression).
2. Every augmentation-discovered function has its transitive callees explored (no dead-end nodes).
3. Caddy `cmdRun` has outgoing edges to `caddy.Load` and other direct callees after augmentation.
4. Corpus reachability ≥ 60/72 (83%), up from 49/72 (68%).
5. The 49 previously-reachable traces are all still reachable.
6. Iterative loop converges in ≤ 3 rounds on all 6 codebases.
7. Evaluation produces deterministic JSON output.
8. New-wall analysis classifies every remaining miss by blocker type with a ranked table.
9. `go test` and `go vet` pass on `pkg/activation/...` and `cmd/activation-path/...`.
10. Import guard holds: zero imports from `pkg/compiler/entrypath/`.

## Expected outcome

Best case: 67/72 reachable (93%) — all 22 struct-field traces unblock except those with secondary blockers. Realistic: 60–65/72 (83–90%) — some traces hit HTTP registration (5 traces in mattermost), closure capture (2 traces in caddy/mattermost), or channel flow (1 trace) after the struct-field hop resolves. Caddy goes from 0/6 to 4–5/6. Mattermost goes from 0/15 to 5–10/15.

The remaining wall will be HTTP handler registration (mattermost API wrappers), closure capture (caddy middleware chains, mattermost workers), and a handful of unsupported patterns (map-keyed dispatch, reflection). This sets up a clean next sprint focused on the single largest remaining blocker class.

## Deferred follow-up

- **HTTP handler registration** — likely the next largest blocker (5+ traces)
- **Closure capture** — caddy middleware, mattermost workers (2+ traces)
- **Channel flow** — mattermost queue dispatch (1 trace)
- **Map-indexed function-value dispatch** — gitea password hashers
- **Tagged-union dispatch** — mattermost
- **Caddy module registry** — string-keyed, likely not statically resolvable
