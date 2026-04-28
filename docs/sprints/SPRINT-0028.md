# SPRINT-0028 - Oracle trace for bounded EntryPath recovery

**Status:** planned  
**Executor:** gpt-5.5-xhigh  
**Predecessor:** SPRINT-0027 showed that budget-partitioned frontier search fixes adjacent-owner starvation but approaches exhaustive-mode cost without recovering the known Mattermost target chain.

## Intent

Build an oracle-backed diagnostic that uses exhaustive/all-mode recovery as the known-good upper bound, then explains where bounded strategies lose the chain:

`connectWebSocket -> APIHandlerTrustRequester -> http.Handler`

The goal is not to hardcode Mattermost into production logic. The goal is to create a reusable oracle/loss-table diagnostic so future candidate strategies can be compared against known-good traces quickly and aggressively.

## Budget Posture

This sprint has a high diagnostic budget. Memory consumption is not the main concern during diagnosis. It is acceptable to reproduce the exhaustive/all-mode Mattermost run if useful, because prior runs completed in roughly 206-221s and 10.9-12.5 GB RSS.

The success criterion is explanatory power: identify the first missing phase, edge kind, owner-selection rule, or value-flow bridge that prevents bounded modes from producing the known recovered chain.

## Non-goals

- Do not wire reportv2, emission, transport extraction, liftpatch output, or user-facing pass behavior.
- Do not implement a Mattermost-specific production recognizer.
- Do not pursue deeper frontier rows or larger owner budgets as the next fix.
- Do not treat memory reduction as the objective of this diagnostic.
- Do not create ADRs.

## Phase 0 - Ground Truth

- [x] **0.1** Read SPRINT-0025, SPRINT-0027, and the relevant entrypath/probe code.
- [x] **0.2** Reproduce or re-summarize the exhaustive/all-mode Mattermost recovery as the oracle upper bound, including wall time, peak RSS, and recovered target evidence.
- [x] **0.3** Record the exact known target identities and expected relationships in a reusable oracle spec or equivalent structured diagnostic input.

## Phase 1 - Oracle Trace Instrument

- [x] **1.1** Add a diagnostic mode or helper that can trace oracle target presence across entrypath phases without changing production classification behavior.
- [x] **1.2** For each oracle node, report whether it appears in loaded SSA, reverse-BFS touchpoints, reverse frontier owners, adjacent owners, boundary candidates, boundary predicate evidence, seed set, function-ref index, and final classifications.
- [x] **1.3** For each oracle edge or relationship, report whether the diagnostic can explain the relationship, and if not, the first missing phase.
- [x] **1.4** Keep the mechanism generic enough for future oracle specs such as gRPC registration chains or callback/value-flow chains.
- [x] **1.5** Add focused fixture tests for the oracle/loss-table logic where practical.

## Phase 2 - Mattermost Loss Table

- [x] **2.1** Run the oracle trace against the known-good exhaustive/all-mode evidence or a reproduced exhaustive run.
- [x] **2.2** Run the oracle trace against the SPRINT-0027 bounded frontier configuration.
- [x] **2.3** Run at least one bounded bridge experiment starting from the known `connectWebSocket` touchpoint toward boundary/value-flow evidence.
- [x] **2.4** Produce a loss table that shows the first phase where each target node or edge disappears.
- [x] **2.5** Record whether the missing mechanism appears to be owner selection, graph edge coverage, boundary predicate rejection, function-value flow, ordering, or final classification.

## Phase 3 - Synthesis

- [x] **3.1** Create `docs/research/runs/SPRINT-0028-oracle-trace.md`.
- [x] **3.2** Include the oracle chain, per-phase presence/absence table, first missing transition, and cost comparison to exhaustive mode.
- [x] **3.3** Recommend exactly one next step: implementation sprint, one more diagnostic, or redesign.
- [x] **3.4** Add a "Suggestions" section with one immediate next sprint shape, one thing not to pursue next, and whether the oracle indicates a generic bridge rather than a Mattermost workaround.
- [x] **3.5** Update this sprint file closeout with phases run, phases cut, artifacts, findings, recommendation, and suggestions.

## Guardrails

- [x] **G.1** Run focused tests for `pkg/compiler/entrypath` and `cmd/entrypath-probe`.
- [x] **G.2** Keep forbidden-string lint passing against `pkg/compiler/entrypath/*.go` non-test sources. Banned strings: `"websocket"`, `"Mux"`, `"HandleFunc"`, `"mattermost"`, `"gorilla"`, `"chi"`, `"echo"`, `"gin"`. Allowlist remains `"http.Handler"`, `"ServeHTTP"`, `"net/http"`.
- [x] **G.3** Verify no files under `pkg/compiler/reportv2/`, `pkg/compiler/surface/`, `pkg/compiler/transport/emit/`, `pkg/compiler/extract/bootpath/`, `evaluation/mattermost/`, or `docs/decisions/` changed.
- [x] **G.4** If the oracle bridge recovers the target chain, stop at recording evidence and recommendations. Do not start report or emission wiring.

## Acceptance Criteria

- [x] Exhaustive/all-mode recovery is available as the oracle upper bound, either by reproduction or by a precise artifact summary.
- [x] The loss table identifies first missing phases for the known target nodes and relationships.
- [x] At least one bounded bridge experiment is compared against the oracle.
- [x] The recommendation is about a generic mechanism, not a Mattermost-specific workaround.
- [x] Focused tests pass, or failures are recorded with relevance.
- [x] Suggestions are included.

## Closeout

### Phases Run

- Phase 0: read SPRINT-0025/0027, prior matrices, baseline artifacts, and
  entrypath/probe code. Recorded the Mattermost oracle spec at
  `docs/research/runs/SPRINT-0028-mattermost-oracle.json`.
- Phase 1: added diagnostic-only oracle tracing plus `oracle-bridge` seed mode
  for bounded bridge experiments. Default EntryPath behavior is unchanged.
- Phase 2: reproduced all-mode with oracle trace, ran the SPRINT-0027 large
  frontier row with oracle trace, then ran the bounded oracle bridge. The bridge
  recovered the target chain, so implementation stopped at evidence.
- Phase 3: created `docs/research/runs/SPRINT-0028-oracle-trace.md` with the
  oracle chain, loss table, costs, recommendation, and suggestions.
- Guardrails: ran focused tests and lint checks; verified no sprint edits were
  made in frozen paths.

### Phases Cut

- No reportv2, surface, transport emission, extraction, liftpatch, or
  user-facing pass wiring was started.
- No Mattermost-specific production recognizers were implemented.
- No deeper frontier rows or larger owner budgets were run after the bridge
  recovered the target chain.
- No memory optimization pass was attempted; bridge cost is recorded as a
  follow-up concern.

### Recommendation

Recommended next step: an implementation sprint for a generic
touchpoint-to-boundary bridge seed source. It should derive starts from
reverse-BFS touchpoints, find nearby function-value ref owners and boundary
predicate owners without a whole-program sort/scan, and preserve independent
bridge discovery, seed, index, and flow stats.

### Suggestions

- Immediate next sprint shape: implement the generic bridge as a bounded seed
  source with package/member indexes and cost accounting, then rerun the same
  oracle spec without oracle-provided start names.
- Do not pursue deeper frontier rows, larger owner budgets, package pruning, or
  Mattermost-specific recognizers next.
- The oracle indicates a generic bridge mechanism rather than a Mattermost
  workaround; the same shape should apply to callback registration chains and
  other typed boundary sinks once predicates exist.

### Pointers

- Oracle report: `docs/research/runs/SPRINT-0028-oracle-trace.md`
- Oracle spec: `docs/research/runs/SPRINT-0028-mattermost-oracle.json`
- All-mode oracle row:
  `docs/research/runs/SPRINT-0028-oracle-all.summary.json`
- Frontier-large oracle row:
  `docs/research/runs/SPRINT-0028-oracle-frontier-large.summary.json`
- Bridge oracle row:
  `docs/research/runs/SPRINT-0028-oracle-bridge-v2.summary.json`
- Focused tests: `go test ./pkg/compiler/entrypath ./cmd/entrypath-probe`
- Forbidden-string lint:
  `rg -n --glob '*.go' --glob '!*_test.go' '"(websocket|Mux|HandleFunc|mattermost|gorilla|chi|echo|gin)"' pkg/compiler/entrypath`

### Guardrail Notes

- `git status --short -- pkg/compiler/reportv2 pkg/compiler/surface
  pkg/compiler/transport/emit pkg/compiler/extract/bootpath
  evaluation/mattermost docs/decisions` still lists pre-existing dirty and
  untracked files in frozen paths. This sprint did not edit those paths.
- `/usr/bin/time -l` returned wrapper exit status 1 after each Mattermost run
  because `sysctl kern.clockrate` is not permitted in this sandbox. Probe JSON
  completed and is used for phase/RSS evidence.
