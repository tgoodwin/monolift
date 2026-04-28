# SPRINT-0029 - Generic touchpoint-to-boundary bridge seed source

**Status:** planned  
**Predecessor:** SPRINT-0028 built an oracle/loss-table diagnostic and showed that a bounded oracle bridge can recover the Mattermost `connectWebSocket -> APIHandlerTrustRequester -> http.Handler` chain more cheaply than exhaustive mode. The loss table showed the frontier path does not lose because `connectWebSocket` is absent; it loses because the nearby function-value and boundary owners never become useful seed/index owners.

## Intent

Implement a generic touchpoint-to-boundary bridge seed source for EntryPath. The bridge should start from reverse-BFS touchpoints, discover nearby function-value reference owners and boundary predicate owners under explicit budgets, and feed those owners into the existing function-ref/value-flow machinery. The sprint should validate the result against the SPRINT-0028 oracle table without relying on oracle-provided start names.

This is an implementation sprint with diagnostic validation. It should not wire new output into reportv2, emission, liftpatch, or user-facing pass behavior yet.

## Goals

- Turn the SPRINT-0028 oracle bridge idea into a generic bounded seed source.
- Derive bridge starts from reverse-BFS touchpoints, not Mattermost names.
- Recover the known Mattermost chain under oracle validation without oracle-provided start names.
- Preserve and extend cost accounting so bridge behavior can be compared to exhaustive/all-mode and SPRINT-0027 frontier rows.
- Produce a readable algorithm sketch/explanation that future work can critique and refine.

## Non-goals

- No Mattermost-specific recognizers, route-name matching, package-name hacks, or hardcoded framework rules.
- No reportv2, transport emission, liftpatch output, bootpath extraction, or surface classification wiring.
- No deeper frontier rows or larger owner-budget tuning as the main fix.
- No memory optimization pass unless a small local improvement is necessary to keep validation runnable.
- No ADR unless the implementation reveals a stable architectural decision that exceeds this sprint.

## Phase 0 - Algorithm Sketch

- [x] **0.1** Create `docs/research/runs/SPRINT-0029-bridge-algorithm.md`.
- [x] **0.2** Sketch the proposed algorithm in plain language and pseudocode: inputs, bridge starts, local owner discovery, boundary-owner discovery, seed assembly, function-ref indexing, value-flow recovery, and stop conditions.
- [x] **0.3** Explain how the bridge remains generic across HTTP, gRPC, callback registration, and other typed boundary sinks.
- [x] **0.4** Name the expected failure modes: missing refs, too-broad owner discovery, predicate rejection, ordering issues, duplicate seeds, budget exhaustion, and false-positive bridge owners.
- [x] **0.5** Keep the sketch current if implementation changes the algorithm.

## Phase 1 - Bridge Start Selection

- [x] **1.1** Derive bridge starts from reverse-BFS touchpoints produced by the existing EntryPath analysis.
- [x] **1.2** Filter or rank bridge starts using generic signal only: region proximity, callgraph relation, function identity stability, and existing touchpoint metadata.
- [x] **1.3** Add stats for bridge start counts, selected starts, skipped starts, and skip reasons.
- [x] **1.4** Add tests proving bridge starts do not require service-specific names.

## Phase 2 - Bounded Local Owner Discovery

- [x] **2.1** Implement a bounded owner-discovery pass that starts from bridge starts and finds nearby function-value reference owners.
- [x] **2.2** Include owners that reference, pass, store, return, wrap, or register bridge-start functions where the existing SSA/value APIs make this practical.
- [x] **2.3** Include bounded boundary predicate owner discovery so likely registration/boundary owners can enter the seed set.
- [x] **2.4** Keep discovery local and budgeted by owner count, package/member scope, instruction count, elapsed time, and duplicate suppression.
- [x] **2.5** Emit stop reasons for each budget separately.
- [x] **2.6** Add focused tests covering owner discovery, duplicate suppression, independent budget stops, and boundary-owner inclusion.

## Phase 3 - Seed Source Integration

- [x] **3.1** Integrate bridge-discovered owners as a seed source in EntryPath diagnostic/seed construction.
- [x] **3.2** Preserve existing modes and behavior unless the bridge mode is explicitly enabled.
- [x] **3.3** Add separate stats for bridge starts, bridge owners, bridge boundary owners, bridge seed owners, indexed bridge owners, and bridge diagnostics.
- [x] **3.4** Ensure final function-ref indexing and function-value flow can consume bridge seeds without report/emission wiring.
- [x] **3.5** Add tests proving bridge seeds can recover a small fixture callback/registration chain that reverse touchpoints alone do not classify.

## Phase 4 - Oracle Validation

- [x] **4.1** Run the SPRINT-0028 oracle spec against the new non-oracle bridge mode.
- [x] **4.2** Compare the new run against `SPRINT-0028-oracle-bridge-v2`, SPRINT-0027 frontier large, and the SPRINT-0025 exhaustive upper bound.
- [x] **4.3** Produce or update `docs/research/runs/SPRINT-0029-bridge-validation.md` with run commands, costs, seed stats, loss table deltas, and target recovery status.
- [x] **4.4** Record whether the Mattermost chain is recovered without oracle-provided start names.
- [x] **4.5** If the target chain is not recovered, identify the first missing phase and stop with a specific next diagnostic recommendation rather than tuning broad budgets.

## Phase 5 - Closeout

- [x] **5.1** Update `docs/research/runs/SPRINT-0029-bridge-algorithm.md` to reflect the implemented behavior.
- [x] **5.2** Update this sprint file closeout with phases run, phases cut, artifacts, target-recovery result, and cost comparison.
- [x] **5.3** Recommend exactly one next step: cost hardening, product integration, one more targeted diagnostic, or redesign.
- [x] **5.4** Add a "Suggestions" section with one immediate follow-up sprint shape, one thing not to pursue next, and any algorithm refinements suggested by validation.

## Guardrails

- [x] **G.1** Run focused tests for `pkg/compiler/entrypath` and `cmd/entrypath-probe`.
- [x] **G.2** Keep forbidden-string lint passing against `pkg/compiler/entrypath/*.go` non-test sources. Banned strings: `"websocket"`, `"Mux"`, `"HandleFunc"`, `"mattermost"`, `"gorilla"`, `"chi"`, `"echo"`, `"gin"`. Allowlist remains `"http.Handler"`, `"ServeHTTP"`, `"net/http"`.
- [x] **G.3** Verify no files under `pkg/compiler/reportv2/`, `pkg/compiler/surface/`, `pkg/compiler/transport/emit/`, `pkg/compiler/extract/bootpath/`, `evaluation/mattermost/`, or `docs/decisions/` changed.
- [x] **G.4** Do not mark the sprint successful only because generic counts improved. Success requires oracle/loss-table evidence about the known chain.

## Acceptance Criteria

- [x] `docs/research/runs/SPRINT-0029-bridge-algorithm.md` explains the algorithm clearly enough for another engineer to critique it.
- [x] Bridge starts are derived from reverse-BFS touchpoints, not oracle-provided target names.
- [x] Bridge owner discovery is bounded and reports independent stop reasons.
- [x] Bridge seeds are integrated behind an explicit diagnostic/seed mode.
- [x] The SPRINT-0028 oracle spec is rerun against the non-oracle bridge mode.
- [x] Validation records whether the Mattermost target chain is recovered and compares cost to exhaustive/frontier/oracle-bridge baselines.
- [x] Recommendation names exactly one next step.
- [x] Focused tests pass, or failures are recorded with relevance.

## Closeout

### Phases Run

All phases were run.

- Phase 0 created and updated the bridge algorithm sketch.
- Phase 1 implemented bridge starts from reverse-BFS touchpoints.
- Phase 2 implemented bounded local owner and boundary-owner discovery.
- Phase 3 integrated bridge owners behind explicit `--function-index-mode=bridge`.
- Phase 4 reran the SPRINT-0028 oracle spec in non-oracle bridge mode.
- Phase 5 closed out with a failed-recovery diagnostic and next-step recommendation.

### Phases Cut

No planned phase was cut. Broad budget tuning and deeper frontier rows were not
run after validation missed the target.

### Guardrail Notes

Focused tests passed:

- `go test ./pkg/compiler/entrypath` -> pass in 220.271s
- `go test ./cmd/entrypath-probe` -> pass from cache

Forbidden-string lint passed with no matches:

```sh
rg -n '"(websocket|Mux|HandleFunc|mattermost|gorilla|chi|echo|gin)"' pkg/compiler/entrypath --glob '*.go' --glob '!*_test.go'
```

The forbidden directories listed in G.3 still show dirty files from the
pre-existing worktree state. SPRINT-0029 did not edit those paths.

### Target Recovery

Not recovered.

The SPRINT-0029 bridge selected and indexed `connectWebSocket` from reverse-BFS
touchpoints, but `APIHandlerTrustRequester` and `InitWebSocket` did not enter
the bridge seed set or function-ref index. Boundary evidence stayed at zero.

First missing phase: bridge local owner discovery after start selection. In the
oracle trace, this surfaces as `function_ref_index` absence for
`APIHandlerTrustRequester`, `InitWebSocket`, and the target relationships.

Cost comparison:

| Run | Probe wall ms | Wrapper real | Peak RSS | Key work | Target result |
|---|---:|---:|---:|---|---|
| SPRINT-0025 exhaustive upper bound | 206,689 | not used | 12,470,170,336 | 140,801 indexed owners, 5,625,718 indexed instructions | recovered |
| SPRINT-0028 frontier large | 141,600 | 188.87s | 9,951,744,280 | 5k reverse + 5k adjacent + 10k candidates, 72 indexed owners | missed |
| SPRINT-0028 oracle bridge v2 | 78,380 | 103.62s | 8,935,415,848 | oracle start, package-local bridge, 94 indexed owners | recovered |
| SPRINT-0029 bridge | 80,766 | 125.41s | 6,066,748,888 | reverse-touchpoint starts, 1,074 indexed owners | missed |

### Recommendation

One more targeted diagnostic.

Record bridge package scheduling and per-start package scan coverage, including
whether each selected start's package was scanned before `instruction_budget` or
`boundary_owner_budget` stopped discovery.

### Suggestions

Immediate follow-up sprint shape: add bridge package-scheduling diagnostics,
rerun the same SPRINT-0028 oracle spec, and report for each oracle touchpoint
whether its package was selected, scheduled, scanned, and stopped before local
owner admission.

Do not pursue next: larger bridge budgets, deeper frontier rows, or package-name
special casing.

Algorithm refinements suggested by validation: make package scheduling visible
before changing it; then consider scheduling packages in selected-start order
instead of plain package-name order if the diagnostic proves ordering is the
first loss.

### Pointers

- Algorithm sketch: `docs/research/runs/SPRINT-0029-bridge-algorithm.md`
- Validation report: `docs/research/runs/SPRINT-0029-bridge-validation.md`
- Raw validation JSON: `docs/research/runs/SPRINT-0029-bridge.json`
- Validation summary: `docs/research/runs/SPRINT-0029-bridge.summary.json`
- Validation stderr/timings: `docs/research/runs/SPRINT-0029-bridge.stderr`
