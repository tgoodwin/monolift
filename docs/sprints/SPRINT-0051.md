# SPRINT-0051: Boundary-adapter compiler pass + listmonk/M-4 stage-10 proof

**Status:** planned
**Predecessors:** SPRINT-0050 (external-persistence escalation, stage ladder, stage-binding doc), SPRINT-0049 (admission-aware cut placement), ADR-0028 (monolith as gateway / FeasibleWithProxy retirement)

## Intent

SPRINT-0051 introduces the boundary-adapter compiler pass specified in `docs/research/activation-paths/boundary-adapter-strategy.md` and proves it end-to-end through `listmonk/M-4` (`processImage`) at stage 10 (full transcript comparison per the SPRINT-0050 stage ladder). The pass runs as a **recovery branch** after primary admission refuses the preferred semantic cut, synthesizes an `AdapterPlan` IR from a small pattern library, discharges six static feasibility obligations as named proof checks, and emits a host wrapper plus a normalized remote helper. The corpus deliverable is a green `listmonk/M-4` row at stage 10 with a declared oracle policy for thumbnail bytes.

This is an **implementation sprint with a narrow Phase 0 evidence-and-decision gate**, not a research sprint. Phase 0 is capped at three days and produces a one-page decision doc; if it overruns, drop the stretch pattern (`reader_read_all`) before extending the gate. The work is shaped by three constraints from existing decisions:

1. **Fallback framing, not ranking** (spec §"Phase Ordering vs Ranking"). The pass fires only when primary admission refuses the preferred semantic unit. No cost model is built this sprint.
2. **Multi-result-DTO normalization is generic, not adapter-gated** (spec §"Pattern Library"). The `(*bytes.Reader, int, int, error)` return shape currently refuses with `unsupported_result_shape` — that refusal is what blocks `processImage` today, even before the multipart input becomes a concern. DTO normalization must run for every boundary, and is sequenced first because of this.
3. **ADR-0028 is upstream of this work.** `FeasibleWithProxy` is retired; the new `AdapterClass` is an orthogonal axis to `BoundaryDataClass` and explicitly does not resurrect proxy semantics. `AdapterPossible` adapters render *local* code that marshals finite values; no live proxy ships.

## Goals

- [ ] Add `AdapterClass` enum + `AdapterPlan` IR to the compiler, orthogonal to existing `BoundaryDataClass`.
- [ ] Implement the six static feasibility obligations as named proof checks with explicit refusal codes.
- [ ] Implement two adapter patterns: `multipart_file_read_all` (input) and `bytes_reader_return` (output).
- [x] Land generic multi-result-DTO normalization in codegen, independent of `AdapterClass`.
- [ ] Wire the adapter pass as a recovery branch in `admitCutCandidates` after refusal of the preferred semantic cut, before the pipeline commits to a broader parent.
- [ ] Guarantee admission recovery selects `processImage` and **does not climb to `(*App).UploadMedia`** for `listmonk/M-4`.
- [ ] Reach stage 10 on `listmonk/M-4` with declared oracle policy (direct PNG byte comparison by default; declared normalizer if Phase 0 finds nondeterminism).
- [ ] Add a `MONOLIFT_BOUNDARY_ADAPTER` feature flag and prove flag-off parity with the SPRINT-0050 admission baseline.
- [ ] Publish ADR-0032 `boundary-adapter-recovery`, update `docs/evolution.md`, and refresh `analyses/listmonk-M-4.md` to retire "Proxy-required"/"Feasible-with-proxy" terminology.

## Scope

**In scope:**

- `AdapterClass` in `pkg/activation/cut_types.go` and `AdapterPlan` in `pkg/codegen/types.go`, wired through admission as a recovery branch in `pkg/codegen/cut_admit.go`.
- Six obligations as named proof checks: `adapter_finite_input`, `adapter_local_lifecycle`, `adapter_use_shape`, `adapter_return_rehydration`, `adapter_error_order`, `adapter_call_site`. Additional refusal codes: `adapter_payload_too_large`, `adapter_unknown`, `adapter_impossible`, `live_proxy_required`.
- Two patterns: `multipart_file_read_all` (`*multipart.FileHeader -> []byte`) and `bytes_reader_return` (`[]byte -> *bytes.Reader`).
- Generic multi-result-DTO normalization for any function returning `(T, U, ..., error)` or `(T, U, ...)` shapes, fired for every boundary admitted with > 1 non-error return, **independent** of `AdapterClass`.
- Body rewriting via pattern-matched AST prologue replacement (not general SSA rewrite).
- Inline JSON/base64 payloads with an 8 MiB ceiling pinned in `AdapterPlan.TransportPolicy`. `Transport: staged_object` is reserved as an enum value with no renderer.
- `MONOLIFT_BOUNDARY_ADAPTER` feature flag (default-on locally; flag-off parity sweep is an acceptance criterion).
- `listmonk/M-4` e2e target at `test/e2e/targets/activation_listmonk_processimage/` with workload, oracle, transcript policy, manifest registration.
- ADR-0032 (`docs/decisions/0032-boundary-adapter-recovery.md`), `docs/evolution.md` update, full rewrite of `analyses/listmonk-M-4.md`, and a cut-placement-synthesis archetype touch-up.

**Out of scope:**

- **Do not move the target to `(*App).UploadMedia` and count that as the M-4 semantic-unit proof.** The whole point is the adapter recovery preserves `processImage` as the selected cut.
- Cost-model ranking between direct cuts and adapter-normalized cuts. Spec calls this hypothetical; we explicitly do not pre-commit.
- General-purpose source/SSA rewriting. Body rewrite is pattern-specific prologue replacement only.
- Patterns beyond `multipart_file_read_all` and `bytes_reader_return`. `reader_read_all` is stretch only.
- Refactoring `pkg/activation/cut_boundary.go` to fold `AdapterClass` into `BoundaryDataClass`. The two axes stay orthogonal.
- Streaming/chunked transport. Inline JSON/base64 only; above 8 MiB the pass refuses.
- Removing the retained `FeasibleWithProxy` JSON constant. Wire-compat cleanup is SPRINT-0052+.
- Other corpus targets. No `pocketbase/M-7` / `gitea/M-13` callback work, even if obligation #6 makes them look tractable.
- Live-proxy transport for multipart files, readers, writers, callbacks, channels, transactions, or `*os.File`.

## Open Questions: Positions

| Question | Position | Refusal code | What would change this |
|---|---|---|---|
| **(a) Body-rewriting representation** | Pattern-matched AST prologue replacement, scoped per pattern. Not a general SSA rewrite. | `adapter_use_shape` | A corpus case where the awkward param is referenced in two distinct spots in the helper body (not just the prologue). Then either a more general rewrite is needed, or we refuse. Refusing is fine for SPRINT-0051. |
| **(b) Inline JSON/base64 vs staging** | Inline JSON/base64 with an 8 MiB ceiling pinned in `AdapterPlan.TransportPolicy`. `Transport: staged_object` reserved as an enum value with no renderer. | `adapter_payload_too_large` | The chosen fixture for M-4 stage-10 exceeds the ceiling, OR a Phase 0 audit of typical Listmonk media sizes shows real uploads above 8 MiB. |
| **(c) Fallback vs ranking** | Strict fallback. Add a lightweight diagnostic when a feasible-but-deep direct cut is accepted on a function whose descendant *would have been* `AdapterPossible`. Revisit when at least one additional adapter family exists and there is a concrete corpus example. | — | A single corpus case where the direct cut is clearly suboptimal but is chosen because it admits, AND the deeper cut would adapt cleanly. Currently hypothetical. |

## Phase 0: Evidence + Decision Gate (3-day cap)

Phase 1 does not start until tasks 0.1–0.9 are checked off and the decisions are summarized in a one-page section at the top of `docs/research/runs/SPRINT-0051-adapter-decisions.md`. If Phase 0 grows past three days, drop the stretch `reader_read_all` pattern before extending the gate.

- [x] 0.1: Baseline audit. On CloudLab, run `cmd/activation-path` against `evaluation/listmonk/cmd/media.go` with reverse-import scope; capture the exact refusal code(s) admission produces for `processImage` today, the current selected cut, the demotion chain, and whether admission climbs to `(*App).UploadMedia`. Confirm the primary blocker is `unsupported_result_shape` before the multipart input becomes a factor. Save artifacts under `.moab/runs/sprint-0051-baseline/`.
- [x] 0.2: Call-site scan. Run `rg -n "processImage\\(" evaluation/listmonk/` and grep for address-of / function-value uses (`&processImage`, assignments). Record exact call-site set under `.moab/runs/sprint-0051-baseline/listmonk-callsites.md`. Obligation #6 (call-site compatibility) hinges on this being a small set with no reflective use.
- [x] 0.3: Decide where `AdapterClass` lives — recommend extending `CutCandidate` (`pkg/activation/cut.go`) with an orthogonal `AdapterClass AdapterClass` field. **Do not** fold the new values into `BoundaryDataClass`; that would re-conflate the axes ADR-0028 just retired. Document the package boundary in the decision doc.
- [x] 0.4: Decide how the recovery branch hooks in. The existing admission loop is `pkg/codegen/cut_admit.go:admitCutCandidates`; the adapter pass should be wired as an additional path **before** demotion. On candidate refusal with a shape-compatible code (`unsupported_boundary_data`, `unsupported_result_shape`, `unsupported_param_shape`), call `tryAdapterPass(report, candidate)`. On success: attach the plan and stop. On failure: mark `AdapterClass` and fall through to existing demotion. Explicitly **do not** run adapter planning for receiver reconstruction failures, shared-state receivers, missing DB/filesystem reconstructors, or broad parent cuts.
- [x] 0.5: Pin inline-payload size policy at 8 MiB; record threshold + refusal code in the decision doc. Threshold is forward-compat insurance — Listmonk thumbnail inputs are well below.
- [x] 0.6: Decide M-4 stage-10 oracle policy. Default: direct PNG byte comparison. **Verify** by running the helper twice against the same fixture and byte-comparing (Go test helper, no Kind needed). If divergence is found (e.g., `tIME` chunk, DEFLATE level drift), the declared normalizer is a decoded-image pixel-hash + dimension comparison (per SPRINT-0050 stage-binding); document the substitution in the decision doc.
- [x] 0.7: Fixture plan. Commit a deterministic ~64 KiB PNG under `test/e2e/targets/activation_listmonk_processimage/testdata/`. Record source format, dimensions, expected original width/height, and expected thumbnail byte policy.
- [x] 0.8: Confirm `LiveProxyRequired` exclusion list. Codify that `http.ResponseWriter`, `io.Writer` output parameters, channels, transaction callbacks, function values, `*os.File`, and mutable write-back objects with aliasing remain refused as `LiveProxyRequired` or `AdapterImpossible`. Codify before pattern matchers ship to prevent scope creep.
- [x] 0.9: Write decision doc (`docs/research/runs/SPRINT-0051-adapter-decisions.md`) with a one-page summary at the top covering 0.3, 0.4, 0.5, 0.6, 0.8 and the falsifiability hooks from the Open Questions table above.

## Phase 1: IR types and admission scaffolding

- [x] 1.1: Add `AdapterClass` enum to `pkg/activation/cut_types.go` with five values: `DirectBoundary`, `AdapterPossible`, `AdapterUnknown`, `LiveProxyRequired`, `AdapterImpossible`. Map existing `Trivial`/`Serializable` boundary classes to `DirectBoundary` by default in `classifyBoundaryData` (`pkg/activation/cut_boundary.go`) — label propagation only, no behavior change.
- [x] 1.2: Add `AdapterClass AdapterClass` field to `CutCandidate` (`pkg/activation/cut.go`) and `AdapterReason string`. `rankCutCandidates` records the field but does **not** use it for ranking (fallback framing).
- [x] 1.3: Add `AdapterPlan` type in `pkg/codegen/types.go` next to `Plan`. Fields: `SourceFunction`, `HostSignature`, `RemoteSignature`, `InputTransforms []AdapterPattern`, `BodyRewrite AdapterBodyRewrite`, `OutputTransforms []AdapterPattern`, `Proofs []AdapterProof`, `TransportPolicy AdapterTransport`. JSON-tagged for manifest/debug emission.
- [x] 1.4: Attach `*AdapterPlan` as an optional field on `Plan`. When non-nil, codegen rendering uses the adapter path; when nil, renders direct as today.
- [x] 1.5: Add the full refusal-code vocabulary to `pkg/codegen/admission.go` / `types.go`: `adapter_finite_input`, `adapter_local_lifecycle`, `adapter_use_shape`, `adapter_return_rehydration`, `adapter_error_order`, `adapter_call_site`, `adapter_payload_too_large`, `adapter_unknown`, `adapter_impossible`, `live_proxy_required`.
- [x] 1.6: Add `MONOLIFT_BOUNDARY_ADAPTER` env-var read at admission-loop entry. When `0`, `admitCutCandidates` skips the adapter branch entirely. Default `1` locally; default `0` for the first regression e2e sweep.
- [x] 1.7: Synthetic boundary classification tests (`pkg/activation/cut_synthetic_test.go`) for `*multipart.FileHeader`, `*bytes.Reader`, `io.Writer`, `http.ResponseWriter`, channel, and `*os.File`. Each maps to the expected `AdapterClass`.

## Phase 2: Generic multi-result-DTO normalization (NOT gated on AdapterClass)

This phase unblocks `processImage` before the multipart input becomes a concern, and benefits any future trace with `(T, U, ..., error)` shape. The work runs for every boundary, not just adapter-eligible ones — that is the whole point, but it is also the regression risk.

- [x] 2.1: Audit `pkg/codegen/multireturn_test.go`, `client.go`, `server.go` for current behavior on multi-result returns. Save audit summary in the decision doc.
- [x] 2.2: Add `ResultDTO` representation to `pkg/codegen/types.go` alongside `Result`. Each multi-return function with > 1 non-error return generates a synthetic `<FuncName>Result` struct with named fields (declared return names if available, else `Result0..N`).
- [x] 2.3: Update `AdmitPlan` (`pkg/codegen/admission.go`) so the `unsupported_result_shape` refusal becomes conditional: if all non-error returns can be packed into a single DTO with JSON-codable fields, allow with codec `CodecResultDTO`; otherwise refuse as before.
- [x] 2.4: Update the affected functions to thread the DTO through both directions: `ReturnCodecFor`, `computeStubReturnSig`, `computeRemoteReturnSig`, `computeTransportErrZeros`, plus server response rendering and client unpacking.
- [x] 2.5: Preserve app-facing signatures in generated host stubs (e.g. `(*bytes.Reader, int, int, error)` is unchanged on the call-site side).
- [x] 2.6: Unit tests for return shapes: `(T, error)` unchanged, `(T)` unchanged, `(T, U, error)`, `([]byte, int, int, error)` (the M-4 shape), `(T, T)`, void. Plus negative cases for non-JSON-codable result types.
- [x] 2.7: Regression test fixture: run SPRINT-0049/0050 stage-10 targets (`miniflux/M-1`, `pocketbase/M-1`) through admission with the new DTO code path and confirm result envelopes are unchanged or intentionally migrated with refreshed goldens.
- [x] 2.8: Bump `GeneratorVersion` to `SPRINT-0051` with the first codegen output change; refresh affected goldens in the same patch.

## Phase 3: Pattern library and proofs

- [x] 3.1: Define `AdapterPattern` and `AdapterProof` interfaces in `pkg/codegen/adapter_patterns.go`. Each pattern owns: `Name() string`, `Matches(param ssa.Value) bool`, `Discharge(ctx ProofContext) []AdapterProof`, `RenderInputExtraction(...) string`, `RenderRemoteReconstruction(...) string`.
- [x] 3.2: Implement `multipart_file_read_all`. Match: parameter type `*mime/multipart.FileHeader`. Discharge: `adapter_finite_input` (`Open + io.ReadAll`), `adapter_local_lifecycle` (host owns `Open`/`Close`/`defer`), `adapter_use_shape` (helper SSA references the parameter only through `Open()` then read; no `Filename`/`Header`/`Size` access, no multiple opens, no mutation, no alias escape).
- [x] 3.3: Implement `bytes_reader_return`. Match: declared return type `*bytes.Reader` whose value in helper SSA is `bytes.NewReader(byteSlice)` with no other producer. Discharge: `adapter_return_rehydration`. Render remote: return `[]byte`. Render host: `bytes.NewReader(out.<Field>)`.
- [x] 3.4: Implement the six proof checks (pattern-specific predicates where useful, generic SSA scans where required):
  - [x] 3.4.1: `adapter_finite_input` — every adapter-required param has a pattern with a finite-extraction renderer.
  - [x] 3.4.2: `adapter_local_lifecycle` — no helper SSA instruction calls `Close`, holds a `defer`, or escapes the awkward-typed value as a non-finite payload.
  - [x] 3.4.3: `adapter_use_shape` — pattern-specific predicate; refuse on unrecognized operations.
  - [x] 3.4.4: `adapter_return_rehydration` — every awkward return has a rehydration pattern.
  - [x] 3.4.5: `adapter_error_order` — read errors that would have occurred inside helper now occur host-side before RPC; record divergence in plan diagnostics but accept (per spec §5).
  - [x] 3.4.6: `adapter_call_site` — reverse-import-scope scan: function not used as a function value, address-of, or via reflection. Bound to the activation-path scope.
- [x] 3.5: `LiveProxyRequired` detectors for the full exclusion list from 0.8 (`http.ResponseWriter`, `io.Writer` output params, channels, transaction callbacks, function values, `*os.File`, mutable write-back). When matched, `tryAdapterPass` returns `live_proxy_required` immediately without attempting patterns.
- [x] 3.6: Negative pattern fixtures (must refuse): multiple `Open` calls on one `*multipart.FileHeader`, `file.Filename`/`Header`/`Size` use, returned `multipart.File`, `io.Writer` output parameters, function-value use, reflective access, `http.ResponseWriter` params.
- [x] 3.7: Golden JSON for the expected `AdapterPlan` for `processImage`, including `InputTransforms`, `OutputTransforms`, `BodyRewrite`, all six `Proofs`, and `TransportPolicy: inline_json_bytes`.
- [x] 3.8: Integration test: feed `processImage` SSA into `tryAdapterPass`; assert it produces the golden `AdapterPlan` end-to-end.

## Phase 4: Host wrapper + normalized helper rendering

- [x] 4.1: Extend `pkg/codegen/adapter.go` (`RenderAdapter`) to render the host-side wrapper when `Plan.AdapterPlan != nil`. The wrapper preserves the original function name, drains awkward inputs (`file.Open()`, `defer src.Close()`, `io.ReadAll`, inline-size check), calls the normalized remote helper, rebuilds awkward returns (`bytes.NewReader(out.Thumbnail)`).
- [x] 4.2: Render the normalized helper as the extracted target. Body is the original body with the pattern-matched AST prologue replaced (e.g. `src, err := file.Open(); img, err := imaging.Decode(src)` → `img, err := imaging.Decode(bytes.NewReader(input))`). Implementation: in-place AST surgery against the helper function guarded by the `adapter_use_shape` proof. Fall back to clone-and-replace if package boundaries force it.
- [x] 4.3: Run `goimports` on rendered output to keep import blocks consistent after the body rewrite drops `mime/multipart` from the helper and adds `bytes` / `io`. Golden test: generated file compiles standalone.
- [x] 4.4: Update `pkg/codegen/server.go` so the normalized helper signature renders correctly and packs into the synthetic `processImageResult` DTO from Phase 2.
- [x] 4.5: Generated client renders fail-open as call to the renamed local implementation; fail-closed returns `(nil, 0, 0, error)` for `processImage`. Confirm `MONOLIFT_LIFT_PROCESSIMAGE`-style env-var handling is consistent with the renamed wrapper.
- [x] 4.6: Confirm extracted deployment manifests do **not** include `MONOLIFT_LIFT_*` env vars (per SPRINT-0050 invariant).
- [x] 4.7: Golden test: render the M-4 host wrapper and normalized helper end-to-end. Compare against committed goldens; wrapper must match the spec's example almost verbatim (`docs/research/activation-paths/boundary-adapter-strategy.md` Part 2).

## Phase 5: Recovery-branch pipeline integration

This phase is separate from Phase 1 IR because the recovery-branch policy (what triggers it, what does not, how it interacts with demotion) is the architectural decision, not the data structure.

- [x] 5.1: Wire `tryAdapterPass` into `admitCutCandidates` (`pkg/codegen/cut_admit.go`) after direct `AdmitCut`/`AdmitPlan` refusal of the preferred semantic cut.
- [x] 5.2: Restrict retry-eligibility to shape-compatible refusals: `unsupported_boundary_data`, `unsupported_result_shape`, `unsupported_param_shape`, `adapter_unknown`. Other refusals fall straight through to demotion.
- [x] 5.3: Do **not** run adapter planning for receiver reconstruction failures, shared-state receivers, missing DB/filesystem reconstructors, or broad parent cuts. Codify the exclusion as code-level guards, not just doc.
- [x] 5.4: Preserve the existing demotion chain; add an `AdapterRecovery` diagnostic showing direct refusal code, adapter class, proof verdicts, and selected normalized boundary.
- [x] 5.5: **UploadMedia guardrail.** Pipeline test: direct admission refuses `processImage`, adapter recovery accepts it, and the pipeline does not select `(*App).UploadMedia`. This is the load-bearing invariant for the sprint.
- [x] 5.6: Pipeline test: adapter proof fails (synthetic shape that matches `multipart_file_read_all` but violates `adapter_use_shape`), existing demotion behavior still chooses the next admissible candidate.
- [x] 5.7: Pipeline test: fallback does **not** change recommendation when the preferred semantic cut is already direct-admissible (no false adapter selection).
- [x] 5.8: Bound retries at `len(Candidates)` per existing budget; adapter pass is one-shot per candidate.

## Phase 6: listmonk/M-4 stage-10 proof

- [x] 6.1: Scaffold `test/e2e/targets/activation_listmonk_processimage/` (`target.go`, `workload.go`, baseline manifests). Reuse `activation_listmonk_sanitizeuri/` as the structural reference (deals with Listmonk auth + Postgres fixture).
- [x] 6.2: Register `activation_listmonk_processimage` in `test/e2e/e2e_test.go`. Set `ActivationLift.Target` to `cmd/media.go:processImage` and `ServiceName` to `monolift-extracted-processimage`.
- [x] 6.3: Workload: authenticate as admin, upload the deterministic PNG fixture via the existing Listmonk `/api/media` route (which invokes `(*App).UploadMedia` → `processImage`).
- [x] 6.4: Oracle: compute the expected thumbnail by running `processImage` locally against the fixture in a Go test helper (preserves Phase 0.6 determinism check artifact).
- [x] 6.5: Direct-invoke expectation: declared oracle policy from Phase 0.6 (`oracle-compare` if direct-byte succeeded; declared pixel-hash + dimension normalizer otherwise).
- [x] 6.6: Transcript checks: thumbnail object written through `a.media.Put`; original-width / original-height returned in the response; content type recorded; extracted-service `/calls` records exactly one call per upload.
- [x] 6.7: Stage progression on CloudLab, **one stage per exact `go test` process, never jump**: 4 → 5 → 6 → 7 → 8 → 9 → 10. Per SPRINT-0050 stage ladder.
- [x] 6.8: Env-off check: with `MONOLIFT_LIFT_*` off, the host falls back to the local helper and the extracted service records zero `/calls`. Thumbnail is still correct.
- [x] 6.9: Fail-open / fail-closed check: with the extracted service unavailable, the generated client policy for `processImage` (fail-open by default) returns the correct result via local fallback; fail-closed mode returns `(nil, 0, 0, error)`.
- [x] 6.10: Update `test/e2e/activation_corpus_traces.yaml` row for `listmonk/M-4`: `status: pass`, `phase: 10`, `boundary_class: AdapterPossible`, `selected_cut: processImage`, `proof_kind: adapter-direct-compare` (or `adapter-png-normalized` if applicable). The selected cut field must be `processImage`, not `(*App).UploadMedia`.

## Phase 7: Documentation

- [ ] 7.1: Write `docs/decisions/0032-boundary-adapter-recovery.md`. Cover: the five-class taxonomy and why it is orthogonal to `BoundaryDataClass`; recovery-branch placement (not ranking) with falsifiability hook; the six obligations and their refusal codes; why multi-result-DTO normalization is independent of adapter classification; inline-only transport policy with the 8 MiB ceiling and `Transport: staged_object` as future enum; the `MONOLIFT_BOUNDARY_ADAPTER` flag rollout policy with a removal target (SPRINT-0053+ after two clean releases); explicit link to ADR-0028 explaining why `AdapterPossible` is *not* a resurrection of `FeasibleWithProxy`.
- [ ] 7.2: Update `docs/evolution.md` with a paragraph on the adapter pass: what it does, what it does not (no ranking, no live proxy, no SSA rewrite), and the `listmonk/M-4` stage-10 result.
- [ ] 7.3: Rewrite `docs/research/activation-paths/analyses/listmonk-M-4.md`. Replace the "Proxy-required / Feasible-with-proxy" column entries with `AdapterPossible`. Replace "Recommended Cut" with the adapted semantic unit. Reference ADR-0032.
- [ ] 7.4: Update `docs/research/activation-paths/cut-placement-synthesis.md` "Framework Callback" archetype row: `listmonk/M-4` has migrated to adapter-recovered.
- [ ] 7.5: Add `docs/research/runs/SPRINT-0051-coverage-report.md` with before/after M-4 status, exact commands, artifacts, oracle policy, and residual backlog (e.g. `reader_read_all`, staging transport).

## Phase 8: Verification and closeout

- [ ] 8.1: Run `go test ./pkg/activation/... ./pkg/codegen/... ./test/e2e/harness/...` on CloudLab; store logs under `.moab/runs/sprint-0051-closeout/`.
- [ ] 8.2: Focused e2e for `activation_listmonk_processimage` per Phase 6.7 stage ladder. One stage, one `go test` process at a time.
- [ ] 8.3: Regression e2e on adjacent and result-shape-affected targets: `activation_listmonk_sanitizeuri` (closest in shape, shared codegen path), `activation_miniflux_refreshfeed` and `activation_pocketbase_createthumb` (SPRINT-0050 stage-10 winners — confirm flag-on does not regress them).
- [ ] 8.4: Flag-off parity sweep: admission-only corpus sweep with `MONOLIFT_BOUNDARY_ADAPTER=0`, compare to SPRINT-0050 admission baseline. Save under `.moab/runs/sprint-0051-admission-flag-off/`. Differences must be zero.
- [ ] 8.5: Flag-on admission-only corpus sweep with `MONOLIFT_BOUNDARY_ADAPTER=1`, save under `.moab/runs/sprint-0051-admission-flag-on/`. Differences from baseline should be limited to the M-4 row flip plus any incidental adapter-recovered candidates (record each one).
- [ ] 8.6: Confirm generated artifacts contain no `MONOLIFT_LIFT_*` env vars in extracted deployments.
- [ ] 8.7: Confirm `GeneratorVersion`, goldens, manifest, ADR, analysis note, evolution note, and corpus row all agree on the SPRINT-0051 changes.
- [ ] 8.8: Confirm no full e2e sweep, broad multi-target `-run` regex, or whole-repository focused admission was used as proof.
- [ ] 8.9: Update sprint ledger to `status: done` with executor recorded.

## Remote Test Discipline

Same rules as SPRINT-0050. Highlights:

- [ ] R.1: Before heavy work, run `cl ls` / `cl status <experiment>` locally. If no experiment exists, ask the user to start the `monolift-buildserver` profile.
- [ ] R.2: All `go test ./pkg/...`, e2e, Kind/Docker image builds, `cmd/activation-path` against real corpus targets, and corpus sweeps run on CloudLab.
- [ ] R.3: Local work is limited to editing, source reading, docs, and small codegen/unit/golden tests that do not touch `evaluation/*`.
- [ ] R.4: No `make e2e`, no multi-target `-run` regex, no `scripts/run_activation_corpus_sweep.sh --phases all`.
- [ ] R.5: Use focused target/importer package scope for research; do not use timeout failures from broad package loading as viability evidence.
- [ ] R.6: Stage escalation one target, one stage, one `go test` process at a time. Never jump stages.
- [ ] R.7: If an e2e run is aborted before cleanup, delete `kind` cluster `monolift-e2e` or orphaned `mlv2-*` namespaces before the next run.
- [ ] R.8: Stage all artifacts under `.moab/runs/sprint-0051-*` on the build node.

## Risks and Mitigations

| Risk | Mitigation |
|---|---|
| Admission recovery selects `(*App).UploadMedia` anyway, defeating the entire sprint thesis. | Phase 5.5 pipeline test plus Phase 6.10 manifest constraint plus acceptance criterion all pin `selected_cut == processImage`. Three reinforcing checks. |
| Phase 0 drifts into a research sprint. | 3-day cap. If Phase 0 grows past three days, drop `reader_read_all` from stretch before extending the gate. Decision doc is one page. |
| Generic multi-result-DTO normalization (Phase 2) breaks an existing stage-10 target by accident, since it changes admission for ALL boundaries, not just adapter-eligible ones. | Phase 2.7 explicit regression fixture against SPRINT-0049/0050 stage-10 winners. Phase 8.4 flag-off parity sweep gives a second check. |
| Pattern-matched AST prologue replacement is fragile against an edge case in the M-4 body. | Phase 3.8 integration test runs the actual `processImage` SSA through `tryAdapterPass` before any rendering. If AST surgery is fragile, fall back to clone-and-replace within the same patch. |
| `processImage` read errors shift from `imaging.Decode` to host `io.ReadAll` (error-order divergence). | Acceptable because the error remains on the host side and before any helper side effects; explicitly encoded in `adapter_error_order` proof and recorded in plan diagnostics. |
| PNG output is not byte-deterministic and Phase 0.6 verification finds nondeterminism. | Declared pixel-hash + dimension normalizer per SPRINT-0050 stage-binding convention. No silent skip. |
| Inline JSON/base64 is too large for some future Listmonk fixture. | 8 MiB ceiling pinned in `AdapterPlan.TransportPolicy` with `adapter_payload_too_large` refusal. Staging deferred to SPRINT-0052+. |
| Obligation #3 (`adapter_use_shape`) lets through a case it should not. | Pattern-specific predicates with explicit allowlist of permitted operations. Refuse anything else. Phase 3.6 negative fixtures. |
| `MONOLIFT_BOUNDARY_ADAPTER` flag creates two admission modes that need testing across future sprints. | ADR-0032 documents a removal target (SPRINT-0053+ after two clean releases). Acceptance criterion is parity with flag off. |
| ADR-0028 conflict — `AdapterPossible` misread as resurrected `FeasibleWithProxy`. | ADR-0032 explicitly references ADR-0028 and explains adapters render local code marshalling finite values; no proxy ships. Analyses doc rewrite eliminates retired terminology. |
| Body-rewrite AST surgery interacts badly with imports (e.g. `mime/multipart` import dropped but still in import block). | Phase 4.3 `goimports` pass plus a golden test that generated file compiles standalone. |
| Recovery branch causes infinite retry. | Phase 5.8 bounds retries at `len(Candidates)`; adapter pass is one-shot per candidate. |
| Listmonk multipart fixture upload through `/api/media` requires auth setup the existing harness doesn't cover. | Phase 6.1 starts from `activation_listmonk_sanitizeuri` target which already handles Listmonk auth + Postgres fixture. Reuse, not rebuild. |
| Proof checks need whole-program information for obligation #6 (call-site compatibility). | Phase 0.2 scopes call-site scan to reverse-import scope; codify in `adapter_call_site` proof. No broadening of admission scope. |

## Acceptance Criteria

**Minimum (framework lands cleanly):**

- [ ] `docs/research/runs/SPRINT-0051-adapter-decisions.md` exists with explicit decisions on the three open questions and the determinism-verification artifact.
- [ ] `AdapterClass` enum, `AdapterPlan` IR, and all ten refusal codes land in `pkg/activation/` and `pkg/codegen/` with unit tests.
- [ ] Generic multi-result-DTO normalization (Phase 2) lands and is exercised by golden tests for `(T, U, error)`, `(T, int, int, error)` (the M-4 shape), and unchanged passthrough for `(T, error)`/`(T)`.
- [ ] `multipart_file_read_all` and `bytes_reader_return` patterns are implemented with proof discharge and negative fixtures.
- [ ] **Direct admission refusal of `processImage` recovers through adapter planning rather than selecting `(*App).UploadMedia`** (Phase 5.5 pipeline test green).
- [ ] Generated host wrapper preserves the original `processImage` signature `(*bytes.Reader, int, int, error)` and call-site compatibility.
- [ ] ADR-0032 lands; `analyses/listmonk-M-4.md` no longer uses retired proxy terminology; `docs/evolution.md` updated.
- [ ] `MONOLIFT_BOUNDARY_ADAPTER=0` admission sweep produces zero-delta parity with the SPRINT-0050 baseline.

**Target (the sprint's actual claim):**

- [ ] `activation-listmonk-processimage` reaches stage 10 on CloudLab via the strict 4→5→6→7→8→9→10 ladder.
- [ ] Stage 10 uses direct PNG byte comparison, or a declared pixel-hash + dimension normalizer justified under SPRINT-0050 stage-binding.
- [ ] Env-off and fail-mode checks pass per the generated client policy.
- [ ] `test/e2e/activation_corpus_traces.yaml` records `listmonk/M-4` as `status: pass`, `phase: 10`, `selected_cut: processImage`, `boundary_class: AdapterPossible`.
- [ ] SPRINT-0050 stage-10 winners (`miniflux/M-1`, `pocketbase/M-1`) pass unchanged with the flag on.
- [ ] CloudLab artifacts stored under `.moab/runs/sprint-0051-*`.

**Stretch:**

- [ ] `reader_read_all` pattern lands as a bonus (`io.Reader` / `io.ReadCloser` input → `[]byte`).
- [ ] Lightweight diagnostic for "deeper adapter-enabled cut existed" is emitted on at least one corpus row (per open-question (c) evidence collection).
- [ ] One additional corpus candidate flips classification under `MONOLIFT_BOUNDARY_ADAPTER=1` without breaking its existing admission result.
- [ ] Manifest output includes the serialized `AdapterPlan` for every adapter-normalized lift.

## References

- `docs/research/activation-paths/boundary-adapter-strategy.md` — the spec, both Part 1 and Part 2
- `docs/research/activation-paths/analyses/listmonk-M-4.md` — gets updated this sprint
- `docs/research/activation-paths/cut-placement-synthesis.md` — `FeasibleWithProxy` retirement context
- `docs/sprints/SPRINT-0050.md` — stage ladder, stage-binding convention, regression baseline
- `docs/sprints/SPRINT-0049.md` — admission-aware demote/rerank loop this builds on
- `docs/decisions/0028-monolith-as-gateway.md` — referenced by ADR-0032
- `docs/decisions/0029-codegen-pipeline.md` — pipeline placement
- `docs/decisions/0030-focused-admission-scope.md` — admission scope discipline
- `pkg/activation/cut.go`, `pkg/activation/cut_types.go`, `pkg/activation/cut_boundary.go`
- `pkg/codegen/admission.go`, `pkg/codegen/cut_admit.go`, `pkg/codegen/adapter.go`, `pkg/codegen/types.go`, `pkg/codegen/server.go`, `pkg/codegen/client.go`, `pkg/codegen/planner.go`
- `test/e2e/harness/target.go`, `test/e2e/activation_corpus_traces.yaml`
- `test/e2e/targets/activation_listmonk_sanitizeuri/` — structural reference for the new M-4 target
- `evaluation/listmonk/cmd/media.go` — `processImage` and `(*App).UploadMedia`
