# SPRINT-0030 - Bridge coverage diagnostic and one generic fix

**Status:** completed diagnostic; target recovered with extended index budget  
**Executor:** gpt-5.5-xhigh  
**Predecessor:** SPRINT-0029 implemented generic non-oracle bridge mode. It selected and indexed `connectWebSocket`, but did not recover the target chain because `APIHandlerTrustRequester` and `InitWebSocket` never entered the bridge seed set or function-ref index.

## Intent

Continue pursuing EntryPath recovery for the known Mattermost chain:

`connectWebSocket -> APIHandlerTrustRequester -> http.Handler / InitWebSocket boundary evidence`

This sprint combines a targeted diagnostic with a constrained implementation step. First, explain why bridge local owner discovery misses the relevant owners. Then, if the diagnostic identifies a concrete generic fix, implement exactly one algorithm update and validate it against the SPRINT-0028 oracle spec.

## Goals

- Add structured bridge coverage diagnostics for package scheduling, per-start scanning, and oracle-target owner coverage.
- Add a ref-match audit for oracle-relevant owners without making the production bridge depend on oracle names.
- Choose exactly one evidence-backed generic fix bucket.
- Implement the selected fix only if the diagnostic supports it.
- Validate whether the updated non-oracle bridge improves or recovers the known chain.

## Non-goals

- No Mattermost-specific production recognizers, package-name hacks, route-name matching, or framework-specific string checks.
- No broad budget tuning as the primary fix.
- No reportv2, transport emission, liftpatch output, bootpath extraction, or user-facing pass behavior.
- No multi-fix speculative rewrite. Pick one fix bucket only.
- No ADR unless the result creates a durable architectural decision beyond this diagnostic path.

## Phase 0 - Context and Baseline

- [x] **0.1** Read SPRINT-0029, SPRINT-0028 oracle artifacts, and current bridge implementation.
- [x] **0.2** Summarize the SPRINT-0029 failure in `docs/research/runs/SPRINT-0030-bridge-coverage.md`.
- [x] **0.3** Preserve the SPRINT-0029 conclusion: first missing phase was bridge local owner discovery after start selection.

## Phase 1 - Bridge Coverage Diagnostic

- [x] **1.1** Add structured diagnostic output showing each selected bridge start and its mapped package.
- [x] **1.2** Record whether each selected package was scheduled, scanned, and completed.
- [x] **1.3** Record per-package scanned function counts, instruction counts, selected starts, bridge owners admitted, boundary owners admitted, and stop reasons.
- [x] **1.4** For oracle target owners, record whether their package was selected, scheduled, scanned, and whether scanning stopped before the target owner.
- [x] **1.5** Distinguish skip causes: package budget, package-function budget, instruction budget, owner budget, boundary-owner budget, duration, duplicate suppression, no ref match, no boundary predicate evidence.
- [x] **1.6** Add focused tests for coverage diagnostics and stop-reason reporting.

## Phase 2 - Ref-Match Audit

- [x] **2.1** Add a narrowly scoped audit section for oracle-relevant owners using the oracle spec for reporting only.
- [x] **2.2** Record direct references to the touchpoint, call arguments involving the touchpoint, stores/returns/closures, static callees receiving the touchpoint, and boundary predicate evidence.
- [x] **2.3** Record whether audited owners were scanned by bridge discovery and whether they produced bridge seeds.
- [x] **2.4** Ensure the production bridge algorithm does not depend on Mattermost or oracle names.
- [x] **2.5** Add focused tests for ref-match audit behavior where practical.

## Phase 3 - Choose One Fix Bucket

- [x] **3.1** Use the coverage/audit evidence to choose exactly one fix bucket.
- [x] **3.2** Allowed buckets: scheduling fix, reference matcher fix, one-hop expansion fix, boundary-owner discovery fix, or ranking fix.
- [x] **3.3** Record the selected bucket and evidence in `docs/research/runs/SPRINT-0030-bridge-coverage.md`.
- [x] **3.4** If no bucket is supported, stop after diagnostics and recommend the next targeted diagnostic. Not applicable; evidence supported the boundary-owner discovery bucket.

## Phase 4 - Implement Selected Generic Fix

- [x] **4.1** Implement only the selected generic fix.
- [x] **4.2** Keep bridge starts derived from reverse-BFS touchpoints.
- [x] **4.3** Preserve explicit `bridge` mode and existing behavior unless bridge mode is enabled.
- [x] **4.4** Preserve independent stats, budgets, and stop reasons.
- [x] **4.5** Add focused tests proving the selected fix.
- [x] **4.6** Update the algorithm sketch if the implemented behavior materially changes it.

## Phase 5 - Validation

- [x] **5.1** Run the SPRINT-0028 oracle spec against the updated non-oracle bridge mode.
- [x] **5.2** Produce raw JSON/stderr/summary artifacts for the Mattermost run under `docs/research/runs/SPRINT-0030-*`.
- [x] **5.3** Create `docs/research/runs/SPRINT-0030-bridge-validation.md`.
- [x] **5.4** Report whether `connectWebSocket` is selected/indexed.
- [x] **5.5** Report whether `APIHandlerTrustRequester` enters bridge seeds/index.
- [x] **5.6** Report whether `InitWebSocket` enters bridge seeds/index.
- [x] **5.7** Report whether target relationships recover.
- [x] **5.8** If still not recovered, report the first missing phase.
- [x] **5.9** Compare cost against SPRINT-0025 exhaustive, SPRINT-0028 oracle bridge v2, SPRINT-0028 frontier large, and SPRINT-0029 non-oracle bridge.

## Phase 6 - Closeout

- [x] **6.1** Update this sprint file closeout with phases run, phases cut, chosen fix bucket, target-recovery result, artifacts, and cost comparison.
- [x] **6.2** Recommend exactly one next step.
- [x] **6.3** Add a "Suggestions" section with one immediate follow-up sprint shape, one thing not to pursue next, and any algorithm refinements suggested by validation.

## Guardrails

- [x] **G.1** Run `go test ./pkg/compiler/entrypath`.
- [x] **G.2** Run `go test ./cmd/entrypath-probe`.
- [x] **G.3** Keep forbidden-string lint passing against `pkg/compiler/entrypath/*.go` non-test sources. Banned strings: `"websocket"`, `"Mux"`, `"HandleFunc"`, `"mattermost"`, `"gorilla"`, `"echo"`, `"gin"`. Allowlist remains `"http.Handler"`, `"ServeHTTP"`, `"net/http"`.
- [x] **G.4** Verify no files under `pkg/compiler/reportv2/`, `pkg/compiler/surface/`, `pkg/compiler/transport/emit/`, `pkg/compiler/extract/bootpath/`, `evaluation/mattermost/`, or `docs/decisions/` changed. These paths remain dirty from pre-existing worktree state, but SPRINT-0030 did not require or intentionally make changes there.
- [x] **G.5** Do not mark success based only on generic counts. Success requires oracle/loss-table evidence about the known chain.

## Acceptance Criteria

- [x] Coverage diagnostics explain whether target packages/functions were selected, scheduled, scanned, skipped, or stopped before admission.
- [x] Ref-match audit explains whether oracle-relevant owners contain recognizable touchpoint relationships.
- [x] Exactly one generic fix bucket is selected, implemented, or explicitly deferred with evidence.
- [x] Validation reports target-recovery status and first missing phase if still not recovered.
- [x] Validation compares costs against SPRINT-0025, SPRINT-0028, and SPRINT-0029 baselines.
- [x] Focused tests pass, or failures are recorded with relevance.
- [x] Recommendation names exactly one next step.

## Closeout

### Phases Run

All phases ran. The first agent attempt stalled after Phase 0 because the
environment exhausted file descriptors. The retry produced diagnostics,
implemented the selected generic fix, ran validation, and generated raw
artifacts. The parent session reconciled stale markdown/checklist reporting
after the retry worker stopped responding.

### Phases Cut

No planned phase was cut. A follow-up `index180` validation was added to test
whether the newly admitted bridge owners recover once indexed.

### Chosen Fix Bucket

Boundary-owner discovery fix. The diagnostic showed that selected local
packages contain generic boundary owners that were not admitted to the bridge
seed set; admitting those owners moved the loss downstream.

### Target Recovery

Default validation: partial. `connectWebSocket`, `APIHandlerTrustRequester`,
and `InitWebSocket` were selected, scanned, and seeded, but none were indexed
under the default validation budget.

`index180` validation: the three target owners were indexed and reached final
classification. The main `connect-to-api-handler` and
`connect-registered-at-init` relationships recovered in final classification;
`init-has-http-boundary` retained boundary evidence but was still absent as a
final relationship record.

### Recommendation

Next sprint: bridge-index priority scheduling. Make the function-ref index scan
admitted bridge owners, especially boundary bridge owners and selected-package
owners, before lower-value owners can consume the default budget.

### Suggestions

Immediate follow-up sprint shape: implement and validate bridge-index priority
scheduling using the default and `index180` loss table.

Do not pursue broader local bridge discovery budgets next. The current fix
already admits the target owners.

Algorithm refinement: treat bridge owner admission and bridge owner indexing as
separate phases with separate ordering/ranking diagnostics.

### Pointers

- `docs/research/runs/SPRINT-0030-bridge-coverage.md`
- `docs/research/runs/SPRINT-0030-bridge-validation.md`
- `docs/research/runs/SPRINT-0030-bridge-diagnostic.{json,stderr,summary.json}`
- `docs/research/runs/SPRINT-0030-bridge-validation.{json,stderr,summary.json}`
- `docs/research/runs/SPRINT-0030-bridge-validation-index180.{json,stderr,summary.json}`
