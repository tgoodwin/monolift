# SPRINT-0027 - Budget-partitioned boundary frontier diagnostic

**Status:** in-progress
**Predecessor:** SPRINT-0026 (boundary-frontier diagnostic). SPRINT-0026 showed frontier discovery could avoid whole-program boundary scanning, but reverse-frontier owners consumed the owner budget before adjacent expansion contributed (`adjacentExpansionOwners=0` in every Mattermost row). The target chain was not recovered.

## Intent

Can a **budget-partitioned invocation-boundary frontier** reserve enough search capacity for adjacent expansion and boundary predicate scanning to recover the Mattermost registration evidence without falling back to whole-program boundary discovery?

This sprint is diagnostic only. It should not wire reportv2, surface classification, or emission. It should answer whether reserved budgets move the search closer to `connectWebSocket -> APIHandlerTrustRequester -> http.Handler` registration.

## Budget Model

Separate budgets must be enforced and reported:

- **Reverse owner budget:** owners from reverse BFS / region-root side.
- **Adjacent owner budget:** callgraph-adjacent owners reached after reverse frontier.
- **Boundary candidate budget:** owners/instructions scanned by BoundaryPredicate.
- **Index budget:** final seeded function-reference indexing.
- **Duration budget:** total incremental boundary phase after callgraph.

Use slightly larger diagnostic budgets than SPRINT-0026 when informative, while preserving bounded runs:

- small: reverse 500, adjacent 500, boundary candidates 1k, depth 1, duration 45s
- medium: reverse 2k, adjacent 2k, boundary candidates 5k, depth 2, duration 60s
- large: reverse 5k, adjacent 5k, boundary candidates 10k, depth 2, duration 90s
- exploratory: reverse 5k, adjacent 10k, boundary candidates 20k, depth 3, duration 120s, only if prior rows show target movement

## Non-goals

- No reportv2 schema changes.
- No `surface.DeriveWithTrace`.
- No `entrypath.Pass` promotion or InvocationTrace artifact.
- No emission work.
- No ADR and no files under `docs/decisions/`.
- No Mattermost-specific analyzer branch, package-name check, route-name check, or framework recognizer.
- No package-pruning strategy that changes analysis semantics.
- No whole-program boundary discovery as a "fix"; whole-program runs may appear only as baseline references.

## Phase 0 - Baseline

- [x] **0.1** Create `docs/research/runs/SPRINT-0027-budgeted-frontier-baseline.md` summarizing SPRINT-0026's matrix and the new budget-partitioning question.
- [x] **0.2** Record the SPRINT-0026 failure mode explicitly: reverse owners exhausted owner budget; adjacent expansion contributed zero owners; target chain not recovered.
- [x] **0.3** Preserve boundary terminology: InvocationBoundary, BoundaryPredicate, BoundarySeed, RegistrationSite, ValueSink, SeedSet.

## Phase 1 - Budget Partitioning

- [x] **1.1** Add separate frontier budget options for reverse owners, adjacent owners, boundary candidates, expansion depth, and boundary duration.
- [x] **1.2** Ensure reverse-owner collection cannot consume adjacent-owner budget.
- [x] **1.3** Stream boundary predicate scanning during or immediately after adjacent expansion instead of collecting all reverse owners first.
- [x] **1.4** Emit separate stats: reverseOwners, adjacentExpansionOwners, boundaryCandidateOwners, boundaryEvidence, boundarySeedOwners, finalIndexedOwners, stop reasons per budget.
- [x] **1.5** Add stop diagnostics for each budget: reverse owner, adjacent owner, boundary candidate, depth, duration, and index budget.
- [x] **1.6** Add tests proving adjacent expansion still runs when the reverse owner budget is saturated.
- [x] **1.7** Add tests proving boundary candidate budget and adjacent owner budget are independently enforced.

## Phase 2 - Mattermost Ladder

Run ladder rows in order and stop early only if the matrix shows no target movement and no new adjacent owners.

- [x] **2.1** Run small row: reverse 500 / adjacent 500 / boundary 1k / depth 1 / duration 45s.
- [x] **2.2** Run medium row: reverse 2k / adjacent 2k / boundary 5k / depth 2 / duration 60s.
- [x] **2.3** Run large row: reverse 5k / adjacent 5k / boundary 10k / depth 2 / duration 90s.
- [x] **2.4** Run exploratory row only if useful: reverse 5k / adjacent 10k / boundary 20k / depth 3 / duration 120s.
- [x] **2.5** For each row, record closeness indicators: `channels/api4`, `connectWebSocket` touchpoint, `connectWebSocket` external surface, `APIHandlerTrustRequester`, registration owner, `http.Handler` sink, shortest observed edge chain if available, and top missing edge/stop reason.
- [x] **2.6** Record whether adjacent expansion contributes nonzero owners and whether those owners include boundary evidence closer to the target chain.

## Phase 3 - Synthesis

- [x] **3.1** Create `docs/research/runs/SPRINT-0027-budgeted-frontier-matrix.md`.
- [x] **3.2** Add rows for every ladder run with all separate budget stats and closeness indicators.
- [x] **3.3** Answer directly: does budget partitioning recover Mattermost target evidence without whole-program boundary discovery?
- [x] **3.4** Recommend exactly one next step: implementation sprint, one more specific diagnostic, or structural redesign.
- [x] **3.5** Update cost-gate recommendation. Start from SPRINT-0026's split gate, but explicitly note if larger diagnostic budgets were required and whether they are acceptable only for diagnosis.
- [x] **3.6** Add a short "Suggestions" section to the matrix or closeout with concrete follow-up suggestions based on the measured findings, including at least one suggested next sprint shape and one thing not to pursue.
- [x] **3.7** Update this sprint file closeout with phases run, phases cut, matrix link, recommendation, and suggestions.
- [x] **3.8** Add `docs/evolution.md` narrative only if the result is stable enough to matter. Do not create an ADR.

## Guardrails

- [x] **G.1** Keep forbidden-string lint passing against `pkg/compiler/entrypath/*.go` non-test sources. Banned strings: `"websocket"`, `"Mux"`, `"HandleFunc"`, `"mattermost"`, `"gorilla"`, `"chi"`, `"echo"`, `"gin"`. Allowlist remains `"http.Handler"`, `"ServeHTTP"`, `"net/http"`.
- [x] **G.2** Run focused tests for `pkg/compiler/entrypath` and `cmd/entrypath-probe`.
- [x] **G.3** Verify no files under `pkg/compiler/reportv2/`, `pkg/compiler/surface/`, `pkg/compiler/transport/emit/`, `pkg/compiler/extract/bootpath/`, `evaluation/mattermost/`, or `docs/decisions/` changed.
- [x] **G.4** If a budgeted frontier mode recovers Mattermost target evidence, stop at recording evidence. Do not start report wiring or surface classification.

## Acceptance Criteria

- [x] Reverse, adjacent, boundary-candidate, index, and duration budgets are independently measured.
- [x] Adjacent expansion contributes nonzero owners in at least one Mattermost row, or the matrix explains why not.
- [x] At least three ladder rows are recorded unless an early stop condition is documented.
- [x] Matrix includes target-closeness indicators, not only binary pass/fail.
- [x] Recommendation names exactly one next step.
- [x] Focused tests pass, or failures are recorded with relevance.
- [x] Frozen boundaries and forbidden-string guardrails hold.

## Closeout

### Phases Run

- Phase 0 baseline: created
  `docs/research/runs/SPRINT-0027-budgeted-frontier-baseline.md`.
- Phase 1 budget partitioning: added separate reverse-owner,
  adjacent-owner, boundary-candidate, depth, duration, and final-index
  accounting for frontier diagnostics.
- Phase 2 Mattermost ladder: ran small, medium, and large rows with exact
  `(*Hub).Start` and `(*WebConn).Pump` roots. Recorded raw JSON, stderr, meta,
  and summaries under `docs/research/runs/`.
- Phase 3 synthesis: created
  `docs/research/runs/SPRINT-0027-budgeted-frontier-matrix.md`.

### Phases Cut

- Exploratory row was cut. Prior rows produced nonzero adjacent owners but no
  target-specific movement toward `connectWebSocket` as an ExternalSurface,
  `APIHandlerTrustRequester`, or the target registration owner. See
  `docs/research/runs/SPRINT-0027-budgeted-frontier-exploratory-cut.md`.
- `docs/evolution.md` narrative was cut because the result is a diagnostic
  negative, not a stable architectural milestone.

### Recommendation

Recommended next step: one more specific diagnostic. Run a bounded
touchpoint-to-boundary value-flow bridge diagnostic from reverse-BFS
touchpoints toward existing InvocationBoundary evidence.

### Suggestions

- Suggested next sprint shape: SPRINT-0028 should test a generic
  touchpoint-to-boundary function-value bridge with independent queue, depth,
  candidate, index, and duration budgets.
- Keep independent budget stats and target-closeness summaries in diagnostic
  artifacts; they made the SPRINT-0027 outcome clear.
- Larger implementation follow-up should wait until a diagnostic recovers the
  target registration chain under the split cost gate.
- Do not pursue deeper frontier rows, larger owner budgets, package pruning, or
  Mattermost-specific recognizers as the next step.

### Guardrail Notes

- Forbidden-path verification was run with `git diff --name-only` and
  `git ls-files --others --exclude-standard` against the frozen paths. Those
  commands still list pre-existing dirty files under frozen directories that
  were present before SPRINT-0027 started. This sprint did not edit those
  paths.
- SPRINT-0027-edited paths are limited to `cmd/entrypath-probe/main.go`,
  `pkg/compiler/entrypath/`, `docs/sprints/SPRINT-0027.md`, and
  `docs/research/runs/SPRINT-0027-*`.

### Pointers

- Baseline: `docs/research/runs/SPRINT-0027-budgeted-frontier-baseline.md`
- Matrix: `docs/research/runs/SPRINT-0027-budgeted-frontier-matrix.md`
- Closeness: `docs/research/runs/SPRINT-0027-budgeted-frontier-closeness.md`
- Exploratory cut:
  `docs/research/runs/SPRINT-0027-budgeted-frontier-exploratory-cut.md`
