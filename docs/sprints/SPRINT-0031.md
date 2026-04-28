# SPRINT-0031 - Bridge-index priority scheduling

**Status:** completed; main chain recovered with 65s index budget  
**Executor:** Codex  
**Predecessor:** SPRINT-0030 proved that generic boundary-owner discovery admits the target Mattermost owners. Default validation still missed because the function-ref index scanned 0 admitted bridge owners; the `index180` control indexed those owners and recovered the main chain.

## Intent

Make bridge mode work end to end under a reasonable default cost envelope by prioritizing admitted bridge owners before function-ref indexing. The primary fix should be scheduling/ranking, not broad budget increases. Budget increases are allowed only as measured fallback evidence after diagnostics show that prioritized indexing still cannot scan enough high-value owners.

## Goals

- Explain why the default SPRINT-0030 validation indexed 0 bridge owners while `index180` indexed 1,670 and recovered the main chain.
- Add diagnostics that make bridge-index scheduling visible: owner priority class, owner seed reasons, whether indexed, and why skipped.
- Implement exactly one generic bridge-index prioritization policy.
- Validate the known Mattermost chain with the SPRINT-0028 oracle spec under default or near-default budgets.
- If a budget increase is still needed, justify it with a small matrix that shows the minimum useful increase.

## Non-goals

- No Mattermost-specific production recognizers, route-name matching, package-name hacks, or framework-specific string checks.
- No broader bridge discovery expansion unless diagnostics prove index prioritization is not the limiting phase.
- No reportv2, transport emission, liftpatch, bootpath extraction, or user-facing compiler behavior outside entrypath/probe diagnostics.
- No speculative multi-fix rewrite. Pick one index-priority policy and validate it.
- No success claim based only on higher counts; success requires oracle/loss-table evidence about the known chain.

## Phase 0 - Context and Baseline

- [x] **0.1** Read SPRINT-0030 plan, coverage report, validation report, and raw summary artifacts.
- [x] **0.2** Summarize the SPRINT-0030 default-vs-`index180` loss in `docs/research/runs/SPRINT-0031-bridge-index-priority.md`.
- [x] **0.3** Preserve the SPRINT-0030 conclusion: the target owners now enter the bridge seed set; the next missing phase is bridge owner indexing under default budgets.

## Phase 1 - Index Scheduling Diagnostic

- [x] **1.1** Add diagnostics showing every admitted bridge owner passed to the function-ref index with seed reasons and package path.
- [x] **1.2** Record per-owner priority inputs: bridge seed, boundary seed, boundary evidence count, selected touchpoint package membership, direct touchpoint-reference count, and existing seed reasons.
- [x] **1.3** Record per-owner index result: indexed, skipped, skip reason, budget responsible, and index order/rank.
- [x] **1.4** Record aggregate counts by priority class and by skip reason.
- [x] **1.5** Add focused tests for index scheduling diagnostics using a small fixture with bridge owners, boundary bridge owners, and low-priority owners.

## Phase 2 - Choose One Priority Policy

- [x] **2.1** Define one generic owner-priority policy before implementing it.
- [x] **2.2** Candidate features may include boundary seed reason, bridge seed reason, selected touchpoint package, generic boundary evidence, and direct touchpoint refs.
- [x] **2.3** The policy must not use oracle node names, Mattermost names, route strings, or package-specific rules.
- [x] **2.4** Document the selected policy and rejected alternatives in `docs/research/runs/SPRINT-0031-bridge-index-priority.md`.

## Phase 3 - Implement Priority Scheduling

- [x] **3.1** Apply the selected priority policy before bridge owners are scanned by the function-ref index.
- [x] **3.2** Preserve existing non-bridge index behavior unless bridge mode is enabled.
- [x] **3.3** Preserve existing bridge discovery budgets, seed stats, and stop reasons.
- [x] **3.4** Ensure priority sorting is deterministic.
- [x] **3.5** Add focused tests proving high-priority bridge/boundary owners are indexed before lower-priority owners under a constrained index budget.
- [x] **3.6** Add focused tests proving existing all/reverse/targeted index behavior is not reordered unintentionally.

## Phase 4 - Validation Matrix

- [x] **4.1** Run the SPRINT-0028 oracle spec against updated bridge mode with the current default budgets.
- [x] **4.2** Produce raw JSON/stderr/summary artifacts under `docs/research/runs/SPRINT-0031-*`.
- [x] **4.3** Report whether `connectWebSocket`, `APIHandlerTrustRequester`, and `InitWebSocket` are selected, seeded, indexed, and finally classified.
- [x] **4.4** Report whether `connect-to-api-handler`, `connect-registered-at-init`, and `init-has-http-boundary` recover.
- [x] **4.5** If default budgets still miss, run a minimal controlled budget matrix. Only vary index budget/owner cap, not bridge discovery, unless diagnostics prove bridge discovery regressed.
- [x] **4.6** Compare against SPRINT-0025 exhaustive, SPRINT-0028 oracle bridge v2, SPRINT-0030 default validation, and SPRINT-0030 `index180`.

## Phase 5 - Cost Accounting

- [x] **5.1** Record wall time, probe wall time, phase timings, peak RSS, indexed owner count, and skipped owner count for each validation row.
- [x] **5.2** State whether the selected policy improves recall at equal budget.
- [x] **5.3** If any budget increase is recommended, state the smallest observed increase that changes target recovery and why it is justified.
- [x] **5.4** Do not recommend a budget increase if prioritization alone recovers the target chain.

## Phase 6 - Closeout

- [x] **6.1** Create `docs/research/runs/SPRINT-0031-bridge-index-validation.md`.
- [x] **6.2** Update this sprint file closeout with phases run/cut, selected policy, target recovery, costs, artifacts, and recommendation.
- [x] **6.3** Add a "Suggestions" section with one immediate follow-up sprint shape, one thing not to pursue next, and any algorithm refinements suggested by validation.

## Guardrails

- [x] **G.1** Run `go test ./pkg/compiler/entrypath`.
- [x] **G.2** Run `go test ./cmd/entrypath-probe`.
- [x] **G.3** Keep forbidden-string lint passing against `pkg/compiler/entrypath/*.go` non-test sources. Banned strings: `"websocket"`, `"Mux"`, `"HandleFunc"`, `"mattermost"`, `"gorilla"`, `"chi"`, `"echo"`, `"gin"`. Allowlist remains `"http.Handler"`, `"ServeHTTP"`, `"net/http"`.
- [x] **G.4** Verify no intentional changes under `pkg/compiler/reportv2/`, `pkg/compiler/surface/`, `pkg/compiler/transport/emit/`, `pkg/compiler/extract/bootpath/`, `evaluation/mattermost/`, or `docs/decisions/`.
- [x] **G.5** If executing with a subagent, use local filesystem and shell only; do not use MCP/connectors/web/plugin tools.

## Acceptance Criteria

- [x] Diagnostics explain which admitted bridge owners were indexed or skipped and why.
- [x] The selected priority policy is generic, deterministic, and documented.
- [x] Default-budget validation either recovers the main target chain or identifies the smallest justified index-budget increase.
- [x] Validation reports target node and relationship recovery status with oracle/loss-table evidence.
- [x] Cost comparison includes SPRINT-0025, SPRINT-0028, SPRINT-0030 default, and SPRINT-0030 `index180`.
- [x] Focused tests pass, or failures are recorded with relevance.
- [x] Recommendation names exactly one next step.

## Closeout

### Phases Run

All phases ran. Phase 0 preserved the SPRINT-0030 conclusion that the target
owners now enter the bridge seed set and default loss is at bridge-owner
indexing. Phases 1-3 added bridge-index owner diagnostics and implemented the
single selected priority policy. Phases 4-5 ran the default validation and a
minimal index-budget matrix. Phase 6 produced the validation report and
closeout.

### Phases Cut

No planned phase was cut. The matrix stopped after the 65s index-budget row
recovered the main chain. Owner-cap rows were not run because diagnostics
showed the default miss was full index-budget exhaustion before owner scanning,
not bridge discovery regression or owner-cap ordering.

### Selected Priority Policy

Admitted bridge owners are sorted as: boundary bridge owners first, then
selected-package bridge owners with direct touchpoint references, then other
selected-package bridge owners, then other bridge owners. Ties use boundary
evidence count descending, direct touchpoint refs descending, package path,
object name, function string, and seed reasons. The policy is generic and
does not use oracle names, route names, application package names, or framework
strings.

### Target Recovery

Default 60s validation missed. It admitted 1,676 bridge owners, including 93
bridge boundary owners, but indexed 0; every admitted bridge owner had
`skipReason=index_budget`.

The 65s index-budget row recovered the main chain. It indexed all 1,676 bridge
owners. `connectWebSocket`, `APIHandlerTrustRequester`, and `InitWebSocket`
were indexed and present in final classification. `connect-to-api-handler` and
`connect-registered-at-init` recovered in final classification.
`init-has-http-boundary` retained boundary evidence but remained absent as a
final relationship record.

### Cost Result

Default 60s: 79,961 ms probe wall, 97.32s wrapper real, 8,849,690,648 peak RSS,
0 indexed bridge owners, 1,676 skipped bridge owners.

Index65: 87,658 ms probe wall, 106.27s wrapper real, 9,171,816,536 peak RSS,
1,676 indexed bridge owners, 0 skipped bridge owners.

The selected policy did not improve recall at equal 60s budget because bridge
seed discovery consumed the whole budget before index scanning began. The
smallest observed useful increase was 65s.

### Recommendation

Next step: make bridge mode reserve or configure a small dedicated index slice
equivalent to the observed 65s function-index budget floor, rather than raising
bridge discovery budgets.

### Suggestions

Immediate follow-up sprint shape: implement a bridge/index budget split or
index reserve, then validate the same oracle spec with the 60s and 65s rows.

Do not pursue broader bridge discovery budgets next. The target owners are
already admitted.

Algorithm refinements suggested by validation: keep the new priority-rank and
skip-reason diagnostics in every bridge validation row, and use target rank
evidence before choosing any future owner-cap default.

### Pointers

- `docs/research/runs/SPRINT-0031-bridge-index-priority.md`
- `docs/research/runs/SPRINT-0031-bridge-index-validation.md`
- `docs/research/runs/SPRINT-0031-bridge-validation-default.{json,stderr,summary.json}`
- `docs/research/runs/SPRINT-0031-bridge-validation-index65.{json,stderr,summary.json}`
