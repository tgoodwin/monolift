# SPRINT-0033 - EntryPath contract and corpus shape validation

**Status:** planned  
**Executor:** TBD  
**Predecessor:** SPRINT-0032 froze the EntryPath bridge as algorithm v1, clarified phase-local bridge/index budgets, and validated the Mattermost oracle chain without adding Mattermost-specific search expansion. The bridge can recover useful path evidence, but the output is still shaped like an exploratory probe rather than a stable compiler contract.

## Intent

Before downstream compiler passes consume EntryPath results to reason about activation boundaries and distribution cut points, prove that the EntryPath algorithm is viable across a locked candidate set, then implement the normalized data structure those later passes should consume. This sprint first validates generality across varied activation shapes, then turns exploratory `ProbeResult` data into a concrete consumable EntryPath contract grounded in those observations.

In this sprint, an "entry path" means a statically recovered path from code in or near a lifted region back to an activation boundary. That boundary may be an API endpoint, but it may also be a background routine bootstrap, queue consumer, cron job, CLI command, lifecycle hook, callback registry, or framework/module registration site.

## Goals

- Define and implement the stable EntryPath producer contract that downstream passes may consume.
- Separate consumable path data from diagnostics, oracle traces, stats, and budget instrumentation.
- Preserve enough evidence to support a future activation-boundary and cut-point selector without making the cut decision in this sprint.
- Build a concrete catalog of entry-path examples from the application corpus.
- Validate that EntryPath can reliably produce those corpus paths through reproducible commands, summaries, and small committed artifacts.
- Use the catalog to prove the normalized EntryPath contract can represent varied activation shapes, including at least one non-Mattermost and one background/lifecycle/job-oriented shape.

## Non-Goals

- No `surface`, `transport`, `liftability`, report persistence, or emission consumption of EntryPath data.
- No cut-point selection algorithm beyond preserving candidate evidence for a future selector.
- No new broad graph-search strategy or bridge budget tuning.
- No Mattermost-specific production recognizers, route-name checks, package-name checks, or framework string matching.
- No reportv2 schema promotion unless a minimal internal contract proves impossible without it.
- No attempt to support every boundary family. The sprint should identify gaps honestly and leave unsupported shapes as explicit follow-up work.

## Phase 0 - Locked Candidate Viability Pass

- [x] **0.1** Treat the Initial Probe Target Matrix as locked unless a row fails to load or is technically impossible within the sprint.
- [ ] **0.2** Run the current EntryPath algorithm against each locked candidate with the existing `ProbeResult` output before designing the normalized contract.
- [ ] **0.3** For each candidate, classify recovery as: viable, partial with bounded gap, fixture-required, or not currently viable.
- [ ] **0.4** Record the observed roots, touchpoints, activation boundary evidence, registration/bootstrap evidence, wrapper links, missing edges, timing, RSS, and budget stops in `docs/research/runs/SPRINT-0033-entrypath-candidate-viability.md`.
- [ ] **0.5** If a candidate is partial or not viable, identify whether the failure is algorithmic, boundary-family coverage, SSA/package loading, budget/cost, or root-spec ambiguity.
- [ ] **0.6** Run candidate probes serially, never in parallel. Complete one corpus probe, record its wall time/RSS/budget stops, and only then start the next candidate.
- [ ] **0.7** If the current approach does not generalize across the locked candidate set, do not force the normalized contract. Instead, document what evidence is missing, what algorithmic changes or boundary-family predicates would be needed, and what strategy the next sprint should pursue.
- [ ] **0.8** Do not finalize or implement the normalized contract until this viability pass has identified the evidence shapes the contract must carry and shown that the approach is viable enough to justify a stable contract.

## Phase 1 - Current Probe Contract Audit

- [ ] **1.1** Audit `pkg/compiler/entrypath/types.go`, `entrypath.go`, `funcvalue.go`, `bridge.go`, and `oracle.go` to list every field currently emitted in `ProbeResult`.
- [ ] **1.2** Classify each `ProbeResult` field as one of: consumer contract, diagnostic-only, validation-only, budget/cost instrumentation, or legacy compatibility, using the Phase 0 candidate observations as evidence.
- [ ] **1.3** Document the audit in `docs/research/runs/SPRINT-0033-entrypath-contract-audit.md`.
- [ ] **1.4** Explicitly decide whether the consumable contract should be a new type derived from `ProbeResult` or a narrowed stable subset of `ProbeResult`.
- [ ] **1.5** Preserve backwards compatibility for `cmd/entrypath-probe` JSON unless the sprint explicitly documents and updates the expected output.

## Phase 2 - Consumable EntryPath Data Model

- [ ] **2.1** Define and implement the stable consumer-facing type, tentatively `EntryPathResult` or `EntryPathTraceSet`, in `pkg/compiler/entrypath`.
- [ ] **2.2** Include stable path primitives observed in Phase 0: region roots, touchpoints, activation boundary candidates, registration/bootstrap sites, wrapper/path links, edge kinds, source positions, static sink/type evidence, boundary-family evidence, and explicit unsupported-gap evidence where needed.
- [ ] **2.3** Exclude oracle traces, bridge coverage, phase timings, RSS, budget stops, and raw debug diagnostics from the consumer-facing type.
- [ ] **2.4** Add a pure conversion function from `ProbeResult` to the consumer-facing type. It must be deterministic and must not rerun SSA, callgraph, indexing, or bridge discovery.
- [ ] **2.5** Add doc comments that explain what the contract proves: it describes statically recovered entry paths and activation-boundary evidence; it does not choose the distribution cut, prove serializability, select transport, or guarantee emission patchability.
- [ ] **2.6** Add deterministic ordering rules for every slice in the consumer-facing type.
- [ ] **2.7** Add unit tests with hand-built `ProbeResult` values covering empty output, reverse-callgraph-only output, registration-site output, wrapper-chain output, multiple candidate paths, and unsupported/unknown boundary evidence.
- [ ] **2.8** Expose the normalized contract from package code as a real API, not only as documentation. Preferred shape: `NormalizeProbeResult(result ProbeResult) EntryPathResult` or `ProbeEntryPaths(... ) (EntryPathResult, error)` layered over existing probe internals.
- [ ] **2.9** Add CLI support to emit normalized output from `cmd/entrypath-probe`, either as a separate flag such as `--normalized` or as a documented secondary artifact in validation scripts. Preserve legacy `ProbeResult` JSON output by default.
- [ ] **2.10** Add compile-time or test coverage showing downstream code can import and inspect the normalized type without depending on `Stats`, `OracleTrace`, bridge coverage, or diagnostics.

## Phase 3 - Corpus Entry-Path Example Catalog

- [ ] **3.1** Create `docs/research/runs/SPRINT-0033-entrypath-corpus-catalog.md`.
- [ ] **3.2** Include the locked candidates from the Initial Probe Target Matrix: Mattermost WebSocket hub, Gitea SSE eventsource, Miniflux feed refresh, Miniflux Fever handler, and Gitea issue indexer queue worker.
- [ ] **3.3** Use PocketBase autobackup as a fallback/gap fixture if the Gitea issue indexer cannot load or produces no useful evidence within the sprint.
- [ ] **3.4** For each example, record: application, source location, region root, expected activation boundary, expected registration/bootstrap/wrapper path, boundary family, current support level, and reproducible command or test that produces the EntryPath output.
- [ ] **3.5** Mark each example as one of: reliably produced now, fixture-only now, partially produced with known gap, or follow-up.
- [ ] **3.6** Prefer existing fixtures and existing evaluation targets; do not clone or vendor new applications unless the existing corpus cannot cover the needed shape.
- [ ] **3.7** For every "reliably produced now" example, commit a concise summary artifact that includes the normalized EntryPath path, not a large raw JSON dump.

### Initial Scout Seeds

These were the scout findings used to lock the target matrix. Prefer examples that can produce concise normalized EntryPath summaries reliably; keep lower-confidence rows as documented gaps or fixture seeds rather than forcing them into the reliable set.

| Candidate | Shape | Why it is useful | Initial confidence |
|---|---|---|---|
| Mattermost WebSocket hub: `connectWebSocket -> HubRegister -> WebConn.Pump` | Known complex HTTP/WebSocket registration path | Control row from SPRINT-0032; validates the new contract preserves known bridge evidence without becoming the only shape. | High |
| Gitea SSE eventsource: `/user/events -> events.Events -> eventsource.Manager.Register` | HTTP-adjacent streaming/fanout registration | Non-Mattermost long-lived session path with manager/messenger state and a clearer registration surface than Mattermost. | Medium-high |
| Miniflux feed refresh: `PUT /v1/feeds/{feedID}/refresh -> RefreshFeed` | Plain `ServeMux.HandleFunc` plus optional scheduler/worker path | Real extraction candidate with both user-triggered and background-entry variants into the same domain work. | Medium for HTTP path |
| Miniflux Fever handler: `/fever/ -> fever.NewHandler -> (*feverHandler).serve` | Handler factory, method value, middleware wrapper | Compact non-Mattermost object-method path likely to be easier to reproduce than larger custom frameworks. | Medium-high |
| Caddy tracing middleware: directive/module registration -> `(*Tracing).ServeHTTP` | Module/directive handler registration | Good framework-registration shape, but full directive-to-route recovery may require custom boundary predicates. | Partial/gap candidate |
| PocketBase realtime: `/api/realtime -> realtimeConnect -> Broker.Register` | Custom router plus hook-triggered action | Useful Mattermost analogue outside Mattermost, but likely exposes current gaps around stored actions and hook registries. | Gap candidate |
| Gitea issue indexer or PocketBase autobackup | Queue/job/cron callback registration | Good non-endpoint activation boundaries; include as fixture or follow-up if full corpus recovery is too expensive. | Gap candidate |

### Initial Probe Target Matrix

The executor should begin from this explicit matrix, then refine it only when a target fails to load, proves ambiguous, or cannot produce useful evidence within the sprint budget. The probe entrypoint is `cmd/entrypath-probe`, which loads SSA for the target package/module, resolves each repeated `--region-root` into an SSA function, and calls `entrypath.ProbeWithOptions`.

| Priority | Application | Package/module directory | Region roots to try first | Activation boundary to recover |
|---|---|---|---|---|
| 1 | Mattermost | `evaluation/mattermost/server` | `github.com/mattermost/mattermost/server/v8/channels/app/platform.(*Hub).Start`; `github.com/mattermost/mattermost/server/v8/channels/app/platform.(*WebConn).Pump` | `api4.InitWebSocket` route registration through `APIHandlerTrustRequester(connectWebSocket)` |
| 2 | Gitea SSE | `evaluation/gitea` | `code.gitea.io/gitea/modules/eventsource.(*Manager).Register`; optionally `code.gitea.io/gitea/modules/eventsource.(*Messenger).Register` | `/user/events` registration in `routers/web/web.go` through `routers/web/events.Events` |
| 3 | Miniflux feed refresh | `evaluation/miniflux` | `miniflux.app/v2/internal/reader/handler.RefreshFeed`; optionally `miniflux.app/v2/internal/reader/processor.ProcessFeedEntries` | `PUT /v1/feeds/{feedID}/refresh` registration in `internal/api.NewHandler`; optional scheduler/worker bootstrap path |
| 4 | Miniflux Fever | `evaluation/miniflux` | `miniflux.app/v2/internal/fever.(*feverHandler).serve` | `/fever/` registration through `fever.NewHandler`, `fever.Middleware`, and `server.newRouter` |
| 5 | Non-endpoint gap row | `evaluation/gitea` or `evaluation/pocketbase` | `code.gitea.io/gitea/modules/queue.(*WorkerPoolQueue).Run` or `github.com/pocketbase/pocketbase/core.(*BaseApp).CreateBackup` | Queue/job/cron activation from startup, expected to expose current contract gaps |

Use fully qualified roots where possible so root resolution stays deterministic. If generic method roots or package loading make a row impractical, record that in the catalog and replace the row with a small fixture that preserves the same activation shape.

## Phase 4 - Fixture Coverage For Shape Diversity

- [ ] **4.1** Add or update lightweight `pkg/compiler/entrypath/testdata` fixtures for any catalog shape that lacks a small deterministic fixture.
- [ ] **4.2** Ensure fixtures cover at least: direct call path, function/callback registration, method-value or object-method registration, and unknown/future boundary family.
- [ ] **4.3** Add fixture tests proving the normalized EntryPath contract preserves all evidence needed by a future cut-point selector.
- [ ] **4.4** Add a negative fixture where no activation boundary should be produced and the normalized result is empty or explicitly unknown.
- [ ] **4.5** Keep fixtures generic: no Mattermost names, WebSocket names, route names, or framework-package string matching in production code.

## Phase 5 - Corpus Validation Runs

- [ ] **5.1** Rerun the consolidated SPRINT-0032 Mattermost bridge profile and emit both raw `ProbeResult` and normalized EntryPath contract output.
- [ ] **5.2** Run at least two non-Mattermost corpus examples from the catalog through the EntryPath probe or an equivalent application-corpus harness.
- [ ] **5.3** If a full corpus target is too expensive or lacks an executable harness, run the corresponding fixture and record the full-corpus blocker in the catalog.
- [ ] **5.4** Run validation probes serially, never in parallel, for the same memory-risk reason as Phase 0.
- [ ] **5.5** Save raw artifacts under `docs/research/runs/SPRINT-0033-*`, avoiding large JSON commits. Commit only concise summaries unless a raw artifact is demonstrably small.
- [ ] **5.6** Produce `docs/research/runs/SPRINT-0033-entrypath-contract-validation.md` summarizing which shapes the contract represents well, which shapes are partial, and which require future boundary predicates.
- [ ] **5.7** Compare the normalized contract against raw `ProbeResult` to prove no consumer-relevant evidence was lost.
- [ ] **5.8** Ensure the catalog contains a final "entry path examples" section with concrete recovered paths that can be used as inputs to the next activation-boundary and cut-point identification sprint.

## Phase 6 - Guardrails And Closeout

- [ ] **6.1** Run `go test ./pkg/compiler/entrypath`.
- [ ] **6.2** Run `go test ./cmd/entrypath-probe` if probe output or CLI options changed.
- [ ] **6.3** Run a forbidden-string guard for new or modified non-test EntryPath code. Banned strings include `websocket`, `Mux`, `HandleFunc`, `mattermost`, `gorilla`, `chi`, `echo`, and `gin`; existing structural allow-list strings remain `http.Handler`, `ServeHTTP`, and `net/http`.
- [ ] **6.4** Verify no intentional changes under `pkg/compiler/surface/`, `pkg/compiler/transport/`, `pkg/compiler/transport/emit/`, `pkg/compiler/extract/bootpath/`, `evaluation/mattermost/`, or `docs/decisions/`.
- [ ] **6.5** Add a closeout section to this sprint file naming the finalized EntryPath contract, corpus example catalog coverage, validation results, unsupported shapes, tests run, and recommended next sprint.
- [ ] **6.6** State explicitly whether the next sprint should be cut-point selection/consumer integration, more boundary predicate work, or additional corpus validation.

## Sequencing

Phase 0 must happen before contract implementation so the sprint does not codify an algorithm that only works for one shape. Phase 1 audits the current probe data using the candidate observations. Phase 2 defines and implements the normalized contract. Phases 3 and 4 constrain that contract against varied corpus and fixture shapes. Phase 5 validates on Mattermost plus multiple non-Mattermost corpus examples or fixtures. Phase 6 records what is stable enough for downstream consumption.

Treat Phase 0 as an explicit execution checkpoint. After the locked candidate viability pass, write the candidate viability memo and decide whether to proceed in the same agent context. If the approach is not clearly viable, or if the context is becoming overloaded by probe output and diagnostic detail, stop before implementing the contract and leave a concrete decision memo for the next sprint or continuation agent.

Do not load large raw JSON artifacts into the model context. Keep raw probe output on disk, summarize only the fields needed for the viability memo, and use concise committed summaries for durable evidence.

If time is tight, cut full-corpus validation before cutting the target matrix, normalized contract, fixture diversity, or deterministic tests.

## Risks And Mitigations

| Risk | Trigger | Mitigation |
|---|---|---|
| `ProbeResult` becomes the downstream API by accident. | Consumers branch on raw stats, oracle trace, or bridge coverage. | Create a normalized contract and classify raw fields as consumer vs diagnostic. |
| Contract overfits to Mattermost. | The only executable row is the Mattermost chain. | Require a corpus catalog and multiple non-Mattermost validation or fixture rows. |
| Current algorithm does not generalize enough to justify a contract. | Multiple locked candidates are not viable or only produce unhelpful partial evidence. | Stop short of forcing the contract; produce a decision memo naming missing evidence, needed algorithm changes, and recommended next strategy. |
| Contract loses evidence needed for cut selection. | Normalization drops wrapper links, static sink type, or source positions. | Compare normalized output against raw `ProbeResult` and test multiple candidate shapes. |
| Scope drifts into cut-point selection. | Work starts modifying `surface` or `transport`. | Keep consumer packages out of scope and defer activation-boundary and cut-point selection to the next sprint. |
| Large raw artifacts get committed again. | Corpus probe JSON exceeds GitHub file-size limits. | Commit summaries by default; keep raw large artifacts out of git. |
| Parallel probes exhaust memory. | Multiple corpus probes run at once. | Run EntryPath corpus probes serially and record cost after each run before starting the next. |
| Boundary-family language becomes endpoint-only. | Types use protocol-specific fields instead of role/evidence fields. | Include background/lifecycle/job rows and unknown-boundary fixture coverage. |

## Acceptance Criteria

- [ ] A stable consumer-facing EntryPath contract exists, is implemented, and is documented.
- [ ] The contract is derived deterministically from `ProbeResult` without rerunning analysis.
- [ ] Diagnostics, oracle traces, stats, and budget instrumentation are not part of the consumer contract.
- [ ] `cmd/entrypath-probe` can emit or otherwise produce the normalized contract for validation runs while preserving legacy probe JSON by default.
- [ ] The corpus catalog contains at least four concrete entry-path examples, including Mattermost and at least one non-Mattermost registration-shaped path.
- [ ] EntryPath can reliably produce committed concise summaries for the in-scope corpus examples.
- [ ] If EntryPath cannot generalize across the locked set, the sprint produces a concrete decision memo instead of pretending the contract is ready.
- [ ] Fixture or corpus validation proves the contract can represent multiple entry-path shapes without Mattermost-specific production logic.
- [ ] Large raw artifacts are not committed.
- [ ] No downstream cut-point consumer is wired in this sprint.
- [ ] Closeout recommends the next sprint based on whether the contract is ready for activation-boundary and cut-point selection.
