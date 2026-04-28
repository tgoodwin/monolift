# SPRINT-0025 - Targeted EntryPath diagnostic search

**Status:** done
**Predecessor:** SPRINT-0024 (`pkg/compiler/entrypath/` and `cmd/entrypath-probe/` landed; Phase 1 stopped at gate-A after the Mattermost diagnostic timed out inside whole-program `function_ref_index`).

## Intent

SPRINT-0025 is a short diagnostic sprint. It does not try to make the Mattermost invocation trace pass end-to-end. Its job is to turn SPRINT-0024's broad, expensive EntryPath search into measured, bounded experiments so the next Mattermost-focused sprint can choose a strategy with evidence. The old 60s / 2.5 GB gate remains a starting hypothesis, not a requirement to defend; the output of this sprint is a defensible cost model and a next-sprint recommendation.

SPRINT-0024's five-minute diagnostic gave the starting profile: package load ~4.9s / 2.73 GB, SSA build ~4.8s / 4.49 GB, root resolution ~16.3s / 6.43 GB, callgraph ~39.7s / 6.68 GB, reverse BFS ~0.35s, then timeout during whole-program `function_ref_index`. This sprint focuses on the opaque phases: root resolution and function-reference indexing.

## Goals

- Reproduce and preserve the SPRINT-0024 Mattermost cost baseline with exact commands, environment, phase timings, and raw artifacts.
- Make `function_ref_index` observable: scanned counts, reference-kind counts, progress events, and clean internal budget exits.
- Measure targeted alternatives to whole-program function-reference indexing: reverse-path seeded, HTTP-sink seeded, and combined targeted worklist search.
- Keep targeted searches honest with toy fixtures that prove the cheaper modes still find known function-value paths.
- Produce one search matrix and one recommendation for the next sprint: implementation, another diagnostic, or structural redesign.

## Non-goals

- No reportv2 schema changes.
- No `surface.DeriveWithTrace`.
- No `entrypath.Pass` promotion or InvocationTrace artifact.
- No emission work.
- No ADR and no files under `docs/decisions/`.
- No Mattermost-specific analyzer branch, package-name check, route-name check, or framework recognizer.
- No package-pruning strategy that changes analysis semantics to make one Mattermost run cheaper.
- No requirement that Mattermost fully passes during this sprint.

## Working Hypotheses

Each hypothesis must be marked confirmed, falsified, or unmeasured in the final matrix.

- **H1:** Reverse BFS is genuinely cheap; the Mattermost cost cliff is downstream of it.
- **H2:** Whole-program `function_ref_index` is the dominant incremental EntryPath cost after loader/SSA/callgraph.
- **H3:** Reverse-path seeding alone is probably too narrow for Mattermost registration chains.
- **H4:** HTTP-sink seeding plus bounded on-demand expansion is the most plausible next strategy.
- **H5:** The old 2.5 GB memory gate is not a good single gate because loader/SSA/callgraph already exceed it; a split gate may be more honest.

## Phase 0 - Reproduce Baseline

- [x] **0.1** Create `docs/research/runs/SPRINT-0025-entrypath-baseline.md` with exact Mattermost command, required `GOWORK`, region roots, Go version, OS/hardware notes, timeout, stdout/stderr paths, and the SPRINT-0024 reference numbers.
- [x] **0.2** Re-run `cmd/entrypath-probe --diagnostic-timings` against the full Mattermost overlay and record exit status, wall time, phase lines, stdout JSON size, and peak memory.
- [x] **0.3** Store the raw JSON output or truncated stderr as `docs/research/runs/SPRINT-0025-entrypath-baseline.json` and link it from the baseline note.
- [x] **0.4** If any completed phase differs by more than 2x from SPRINT-0024's recorded number, stop and write a baseline-divergence note before running search experiments.
- [x] **0.5** Run one smaller Mattermost subsystem probe, if a valid target package is available without new setup, and record whether EntryPath completes at reduced scale. This is a sanity check only, not a success gate.

## Phase 1 - Make Index Cost Observable

- [x] **1.1** Add `FunctionRefIndexStats` in `pkg/compiler/entrypath`: scanned functions, blocks, instructions, discovered function sources, closure sources, operand refs, call-arg refs, store refs, return refs, skipped functions, elapsed millis, and peak RSS.
- [x] **1.2** Thread `FunctionRefIndexStats` into EntryPath probe stats without touching `reportv2`.
- [x] **1.3** Extend `ProbeOptions.PhaseObserver` to emit `function_ref_index` progress events at a configurable instruction interval. Include scanned counts, current package path, elapsed millis, and RSS.
- [x] **1.4** Add `ProbeOptions.FunctionRefIndexBudget`; when exceeded, return a partial index plus `Diagnostic{Kind: "function_ref_index_budget_exceeded"}` instead of relying on outer `timeout`.
- [x] **1.5** Add `ProbeOptions.FunctionRefIndexMaxFunctions` and `cmd/entrypath-probe --function-index-max-functions=<n>` for deterministic prefix sampling.
- [x] **1.6** Add CLI flags `--function-index-budget=<duration>` and `--function-index-progress-interval=<n>`; defaults preserve current behavior.
- [x] **1.7** Add tests proving budgeted and max-function index runs return deterministic partial stats and do not panic.
- [x] **1.8** Run full Mattermost with a 120s index budget and record progress hotspots in the baseline note.
- [x] **1.9** Capture a heap profile at or near the function-index budget boundary if this can be done without destabilizing the run; if not, document why it was cut.

## Phase 1a - Root Resolution Shortcut

This branch is useful but cuttable. Do not let it block seeded search experiments.

- [x] **1a.1** Add root-resolution stats to `cmd/entrypath-probe`: functions inspected, matched specs, fast-path hits, fallback hits, elapsed millis, and RSS delta.
- [x] **1a.2** Add an exact-spec resolver for fully qualified roots that avoids sorting all `ssautil.AllFunctions(prog)` results.
- [x] **1a.3** Keep suffix/fuzzy root resolution as fallback and emit a diagnostic when fallback is used.
- [x] **1a.4** Add tests for exact method roots, bare `(*Type).Method` roots, ambiguous suffix roots, and missing roots.
- [x] **1a.5** Re-run Mattermost with and without exact root resolution and record whether the ~16s root-resolution cost moved.
- [x] **1a.6** Cut this branch if it takes more than roughly half a working session; record root resolution as a known unknown instead.

## Phase 2 - Targeted Search Experiments

All modes are diagnostic flags on `cmd/entrypath-probe`. Default `all` behavior must remain SPRINT-0024-compatible.

- [x] **2.1** Add `FunctionIndexSeedSet` with owner functions and reason tags: `reverse_path`, `http_sink`, `on_demand_expansion`.
- [x] **2.2** Add `BuildFunctionRefIndexForSeeds` that scans only seeded owner functions and reports the same stats as whole-program indexing.
- [x] **2.3** Add `ProbeOptions.FunctionIndexMode` and `cmd/entrypath-probe --function-index-mode=all|reverse-path|http-sinks|targeted`.
- [x] **2.4** Implement `reverse-path` seeds from region roots, reverse-BFS touchpoints, and one-hop static callees inside those owner functions.
- [x] **2.5** Add a reverse-path toy fixture proving the seeded index finds a handler only when the reverse-reachable owner is seeded.
- [x] **2.6** Run Mattermost in `reverse-path` mode with a 60s index budget and record scanned functions, instructions, elapsed time, peak RSS, external surfaces, registration sites, wrapper chains, and whether `connectWebSocket` appears.
- [x] **2.7** Implement `http-sinks` seeds structurally using existing `boundary.go` predicates for `http.Handler`, `http.HandlerFunc`, `*http.Server`, and `ServeHTTP`-shaped interfaces. No framework or Mattermost strings.
- [x] **2.8** Add an HTTP-sink toy fixture with one HTTP-shaped handler and one unrelated callback; assert only the HTTP-shaped owner is seeded.
- [x] **2.9** Run Mattermost in `http-sinks` mode with a 60s index budget and record the same metrics as 2.6 plus seed counts and rejected non-HTTP interface owners.
- [x] **2.10** Implement `targeted` mode as the union of reverse-path and HTTP-sink seeds plus bounded on-demand expansion.
- [x] **2.11** Split targeted expansion rules by cause: function value passed to static callee, and function value returned/wrapped by another function.
- [x] **2.12** Add deterministic targeted bounds: max scanned functions, max expansion depth, max elapsed duration, and max queued work items.
- [x] **2.13** Add stop diagnostics: `targeted_completed`, `targeted_expansion_budget_exceeded`, `targeted_queue_overflow`, and `targeted_index_budget_exceeded`.
- [x] **2.14** Add a targeted-only fixture where `targeted` finds a wrapper path that both `reverse-path` and `http-sinks` miss.
- [x] **2.15** Run Mattermost in `targeted` mode with default and larger expansion budgets. Record whether it recovers `connectWebSocket`, `APIHandlerTrustRequester`, and any `http.Handler` registration sink.
- [x] **2.16** Run a small root-count scaling curve only for modes that complete: one root, both real region roots, and any cheap synthetic/root fixture set. Record whether cost scales with roots or with scanned program size.

## Phase 3 - Synthesize

- [x] **3.1** Create `docs/research/runs/SPRINT-0025-entrypath-search-matrix.md`.
- [x] **3.2** Add one row per measured mode: `all`, `reverse-path`, `http-sinks`, `targeted-default`, `targeted-expanded`, and any subsystem/scaling run that was cheap enough to perform.
- [x] **3.3** Matrix columns: command, budget, function-index wall ms, peak RSS, scanned functions, scanned instructions, external surfaces, registration sites, wrapper chains, `connectWebSocket` recovered, `APIHandlerTrustRequester` recovered, `http.Handler` sink reached, stop reason, and confidence.
- [x] **3.4** Add a confirmed/falsified/unmeasured section for H1-H5.
- [x] **3.5** Add a "recommended next sprint shape" section naming exactly one: Mattermost implementation with a chosen mode, another diagnostic with a precise next question, or structural redesign.
- [x] **3.6** Add a "do not pursue next" section for modes or ideas that were too expensive, too imprecise, or semantically unsafe.
- [x] **3.7** Add a proposed cost gate. It may keep 60s/2.5GB, revise it, or split it into baseline loader/SSA/callgraph and incremental EntryPath gates; the recommendation must cite measured data.
- [x] **3.8** Add a closeout section to this file with phases run, phases cut, links to the baseline and matrix, and the one-paragraph next-sprint recommendation.
- [x] **3.9** Add a short `docs/evolution.md` entry only if the recommendation is stable enough to matter narratively. Do not create an ADR.

## Guardrails

- [x] **G.1** Keep forbidden-string lint passing against all `pkg/compiler/entrypath/*.go` non-test sources. Banned strings: `"websocket"`, `"Mux"`, `"HandleFunc"`, `"mattermost"`, `"gorilla"`, `"chi"`, `"echo"`, `"gin"`. Allowlist remains `"http.Handler"`, `"ServeHTTP"`, `"net/http"`.
- [x] **G.2** Run focused tests for `pkg/compiler/entrypath` and `cmd/entrypath-probe`.
- [x] **G.3** Verify no files under `pkg/compiler/reportv2/`, `pkg/compiler/surface/`, `pkg/compiler/transport/emit/`, `pkg/compiler/extract/bootpath/`, `evaluation/mattermost/`, or `docs/decisions/` changed.
- [x] **G.4** If a seeded mode appears to pass Mattermost, stop at recording evidence. Do not start report wiring or surface classification in this sprint.

## Risks and Mitigations

| Risk | Trigger | Mitigation |
|---|---|---|
| Baseline drift | Phase 0 numbers differ by more than 2x from SPRINT-0024 | Stop, document the divergence, and fix the input before comparing modes. |
| Instrumentation changes costs | Progress-enabled `all` mode is much slower than baseline | Keep progress behind observer hooks, compare progress and no-progress rows, and raise interval if needed. |
| Seeded modes are cheap but broken | Toy fixtures fail or modes miss known fixture paths | Fix fixture behavior before trusting Mattermost metrics. |
| Reverse-path mode is too narrow | Cheap run misses registration owners | Treat as expected H3 evidence; use HTTP-sink/targeted mode instead of broadening with framework heuristics. |
| HTTP-sink mode is too broad | Thousands of owners or many unrelated surfaces | Use seed reasons, rejected-owner counts, and targeted mode to bound fanout. |
| Memory remains dominated by loader/SSA/callgraph | All modes peak near the same ~6.6 GB | Recommend a split cost gate; do not pretend function-index targeting solves upstream memory. |
| Scope creep | Tasks touch reportv2, surface, emission, ADRs, or Mattermost-specific branches | Stop and move that work to a follow-up sprint. |

## Acceptance Criteria

- [x] Baseline note and raw artifact exist and are reproducible.
- [x] Function index stats, progress events, budget exits, and max-function sampling exist and are tested.
- [x] At least two targeted modes run on Mattermost and are represented in the search matrix.
- [x] Toy fixtures distinguish the seeded modes, including a targeted-only wrapper fixture.
- [x] Search matrix maps H1-H5 to confirmed/falsified/unmeasured and recommends exactly one next sprint shape.
- [x] Cost gate recommendation is tied to measured data.
- [x] Frozen boundaries and forbidden-string guardrails hold.
- [x] Focused tests pass, or failures are recorded with relevance.

## Closeout

### Phases Run

- Phase 0 baseline reproduction and subsystem sanity probe.
- Phase 1 function-reference index stats, progress, budgets, max-function sampling, tests, 120s budget run, and heap-profile cut note.
- Phase 1a root-resolution stats, exact-root fast path, fallback diagnostics, tests, and Mattermost comparison.
- Phase 2 targeted search experiments through `all`, `reverse-path`, `http-sinks`, `targeted-default`, `targeted-expanded`, and reverse-path scaling.
- Phase 3 synthesis matrix and recommendation.

### Phases Cut

- Heap profile capture at the function-index budget boundary was cut because the probe has no safe heap trigger and the budgeted Mattermost run already reached ~12.5 GB RSS.
- No report wiring, surface classification, emission, ADR, or Mattermost-specific recognition work was started.

### Recommendation

Run another diagnostic with one precise question: can HTTP-shaped seed discovery be made incremental from reverse-path owners and callgraph-adjacent functions, avoiding the current whole-program HTTP seed scan while still recovering `connectWebSocket` and the `APIHandlerTrustRequester` registration chain? Use a split cost gate: baseline loader/SSA/root/callgraph under 90s and 8 GB RSS, then incremental seeded EntryPath under 30s and +1.5 GB RSS after callgraph. Do not proceed to report/surface implementation until a seeded mode recovers the Mattermost target evidence inside that split gate.

### Pointers

- Baseline: `docs/research/runs/SPRINT-0025-entrypath-baseline.md`
- Search matrix: `docs/research/runs/SPRINT-0025-entrypath-search-matrix.md`

### Guardrail Notes

- G.3 verification: guarded paths still show pre-existing user/orchestrator
  work in `git status`, but this sprint did not edit those paths.
