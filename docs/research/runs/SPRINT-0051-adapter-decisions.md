# SPRINT-0051 Adapter Decisions

**Date:** 2026-05-17
**Sprint:** SPRINT-0051 — Boundary-adapter compiler pass + listmonk/M-4 stage-10 proof
**Author:** Phase 0 evidence gate

---

## One-Page Summary

### 0.3 — AdapterClass Placement

**Decision:** Add `AdapterClass` as an orthogonal field on `CutCandidate` in `pkg/activation/cut.go`. The type definition (`AdapterClass` enum with values `DirectBoundary`, `AdapterPossible`, `AdapterUnknown`, `LiveProxyRequired`, `AdapterImpossible`) lives in `pkg/activation/cut_types.go` alongside the existing classification types.

**Package boundary:** The enum is defined in `pkg/activation/` (the data model layer). The logic that populates it — `tryAdapterPass` and the pattern matchers — lives in `pkg/codegen/` (the compiler layer). This mirrors the existing separation where `BoundaryDataClass` is defined in `pkg/activation/` but classified by `classifyBoundaryData()` in the same package.

**What this is not:** `AdapterClass` is *not* folded into `BoundaryDataClass`. The two axes are orthogonal:
- `BoundaryDataClass` describes how hard boundary values are to serialize (`Trivial` → `BoundaryInfeasible`).
- `AdapterClass` describes whether the compiler can synthesize a local wrapper to normalize the boundary shape.
A candidate can be `Reconstructible` on the boundary-data axis and simultaneously `AdapterPossible` on the adapter axis (this is exactly the `processImage` case).

### 0.4 — Recovery-Branch Wiring

**Decision:** Wire `tryAdapterPass(report, candidate)` into `admitCutCandidates` (`pkg/codegen/cut_admit.go`) as an additional path **before** demotion.

**Trigger:** When `tryAdmitCandidate` returns a non-accepted verdict with a shape-compatible refusal code — specifically `unsupported_boundary_data`, `unsupported_result_shape`, or `unsupported_param_shape` — the adapter pass fires before the candidate is demoted.

**On success:** Attach the `AdapterPlan` to the candidate and stop (the candidate is accepted with adapter normalization).

**On failure:** Mark the candidate's `AdapterClass` field (e.g., `AdapterUnknown`, `LiveProxyRequired`, `AdapterImpossible`) and fall through to the existing demotion logic.

**Exclusions (code-level guards, not just doc):**
- Do NOT run adapter planning for `receiver_requires_reconstruction` refusals.
- Do NOT run adapter planning for `non_serializable_receiver` refusals (shared-state receivers).
- Do NOT run adapter planning for `missing_reconstructor` refusals that reference receiver types (as opposed to parameter types).
- Do NOT run adapter planning for `missing_reconstructor` refusals that reference DB/filesystem types (`*sql.DB`, `*sql.Tx`, `*gorm.DB`, `*os.File`, `*bolt.DB`, and similar infrastructure handles). These are dependency-injection concerns, not boundary-shape problems; the adapter pass cannot synthesize a reconstructor for a live database connection or filesystem handle.
- Do NOT run adapter planning for candidates with `VeryLarge` or `Large` surface class (broad parent cuts).
- Do NOT run adapter planning for candidates where `Callbacks` is `Moderate` or `Many`.

**Interaction with demotion:** The existing demotion chain is preserved. When adapter planning also fails, the demotion reason includes both the original refusal code and the adapter failure reason.

### 0.5 — Inline-Payload Size Policy

**Decision:** Pin at **8 MiB**. Refusal code: `adapter_payload_too_large`.

Codified in `AdapterPlan.TransportPolicy`:
- `inline_json_bytes` — default; inline JSON/base64 up to 8 MiB.
- `staged_object` — reserved as an enum value with no renderer. Implementation deferred to SPRINT-0052+.

**Rationale:** The listmonk fixture is ~77 KiB (well below), and typical Listmonk media uploads are also well below this ceiling. The 8 MiB threshold provides forward-compat insurance. Any corpus target whose typical payloads exceed 8 MiB would surface the `adapter_payload_too_large` refusal and require the staging transport.

### 0.6 — M-4 Stage-10 Oracle Policy

**Decision:** Default **direct PNG byte comparison**.

**Verification result:** The `processImage` function (using `disintegration/imaging` Lanczos resize + PNG encode) produces byte-identical output when run twice against the same fixture. Tested on CloudLab (tgoodwin-305638, c220g5, Go 1.26.0, linux/amd64):

```
Run 1: 60807 bytes, 350x260, sha256=67a6587cc2aaffed72c5fcae0fa551588620737b7b1559ec04c54f7e10e1be25
Run 2: 60807 bytes, 350x260, sha256=67a6587cc2aaffed72c5fcae0fa551588620737b7b1559ec04c54f7e10e1be25
DETERMINISTIC: byte-identical thumbnails across both runs
```

No `tIME` chunk drift, no DEFLATE level variation, no nondeterminism detected. The declared oracle policy is `oracle-compare` (direct byte comparison). No pixel-hash or dimension normalizer is needed.

**Fallback:** If future testing reveals nondeterminism (e.g., Go version upgrade changes PNG encoder behavior), the declared substitute is decoded-image pixel-hash + dimension comparison, per SPRINT-0050 stage-binding convention.

### 0.8 — LiveProxyRequired Exclusion List

**Decision:** The following types are codified as `LiveProxyRequired` or `AdapterImpossible` in the adapter pass. When matched in a candidate's parameter or return types, `tryAdapterPass` returns `live_proxy_required` immediately without attempting patterns.

| Type / Category | Reason | Refusal |
|---|---|---|
| `http.ResponseWriter` | Streaming write to active HTTP request; ordering and lifecycle tied to the connection. | `live_proxy_required` |
| `io.Writer` output parameters | Remote code would stream back to a host-owned sink; the entire output cannot be captured as a finite value without changing semantics. | `live_proxy_required` |
| Channels (`chan T`, `<-chan T`, `chan<- T`) | Send/receive order and goroutine scheduling are part of the semantics. | `live_proxy_required` |
| Transaction callbacks (`func(...) error` parameters used as DB transaction closures) | Callback execution owns transaction lifetime; cannot be serialized. | `live_proxy_required` |
| Function values (parameters or returns of function type) | Runtime dispatch across the network boundary; no finite serialization. | `adapter_impossible` |
| `*os.File` | OS file handle with seek/lock/close lifecycle tied to the process. | `live_proxy_required` |
| Mutable write-back objects with aliasing | Local aliases may observe mutation order; serializing the object loses aliasing semantics. | `adapter_impossible` |

---

## Falsifiability Hooks (from Open Questions)

| Question | Position | What would change it |
|---|---|---|
| **(a) Body-rewriting representation** | Pattern-matched AST prologue replacement, scoped per pattern. Not a general SSA rewrite. | A corpus case where the awkward param is referenced in two distinct spots in the helper body (not just the prologue). Then either a more general rewrite is needed, or we refuse. Refusing is fine for SPRINT-0051. |
| **(b) Inline JSON/base64 vs staging** | Inline JSON/base64 with 8 MiB ceiling. `staged_object` reserved with no renderer. | The chosen fixture exceeds 8 MiB, OR a Phase 0 audit of typical Listmonk media sizes shows real uploads above 8 MiB. Neither applies: fixture is 77 KiB. |
| **(c) Fallback vs ranking** | Strict fallback. Add diagnostic when a feasible-but-deep direct cut is accepted on a function whose descendant *would have been* `AdapterPossible`. | A single corpus case where the direct cut is clearly suboptimal but is chosen because it admits, AND the deeper cut would adapt cleanly. Currently hypothetical; no cost model exists. |

---

## Baseline Evidence

### processImage Refusal Codes (Today)

`AdmitPlan` refuses `processImage` with two codes:
1. `missing_reconstructor`: `*multipart.FileHeader` has no registered reconstructor
2. `unsupported_result_shape`: `(*bytes.Reader, int, int, error)` — more than two return values

Both are in `retryableAdmissionRefusals`. After demotion, the ranking loop selects `(*App).UploadMedia` — the climb behavior that SPRINT-0051 prevents.

### processImage Call-Site Scan

Single call site at `cmd/media.go:99` inside `(*App).UploadMedia`. No address-of, function-value, or reflective use. Obligation #6 (call-site compatibility) is satisfiable.

### Fixture Properties

- Path: `test/e2e/targets/activation_listmonk_processimage/testdata/fixture.png`
- Format: PNG, RGBA, 350x260
- Size: 79,102 bytes (~77 KiB)
- Content: Deterministic sine-wave gradient (generated, not sampled)
- Expected original dimensions: width=350, height=260
- Expected thumbnail: 250px wide, ~60.8 KiB, byte-deterministic

### Artifacts

- `.moab/runs/sprint-0051-baseline/processimage-baseline.json` — activation-path JSON
- `.moab/runs/sprint-0051-baseline/admission-processimage.log` — admission refusal log
- `.moab/runs/sprint-0051-baseline/baseline-summary.md` — narrative summary
- `.moab/runs/sprint-0051-baseline/listmonk-callsites.md` — call-site scan

---

## Phase 2: Multi-Result-DTO Normalization Audit

### Current Behavior (pre-DTO)

**Admission (`pkg/codegen/admission.go:102-106`):** `AdmitPlan` enforces a strict two-result gate:
- `len(plan.Results) > 2` → refuses with `unsupported_result_shape` ("more than two return values")
- `len(plan.Results) == 2 && plan.Results[1].Codec != CodecError` → refuses with `unsupported_result_shape` ("multi-return must have error as last result")

This means only three shapes are admitted today:
1. `(T, error)` — standard case, supported
2. `(T)` — single non-error return, supported
3. `()` — void, refused separately as `void_side_effect`

Any function with `(T, U, error)` or `(T, U, V, error)` is unconditionally refused. This is the primary blocker for `processImage` which returns `(*bytes.Reader, int, int, error)`.

**Server rendering (`pkg/codegen/server.go:136-161`):** The server template assumes at most one non-error result. The `serverTemplateView` iterates over `plan.Results`, takes the first non-error result as the single `ResponseField`, and sets `HasResult`/`HasErrorResult` flags. The `invokeResponse` struct has one non-error field and one `Error` field. Multi-value packing is not supported.

**Client rendering (`pkg/codegen/client.go:124-145`):** Same single-result assumption. The `clientTemplateView` picks the first non-error result and records `ResultType`/`ResultZero`. The `computeStubReturnSig`, `computeRemoteReturnSig`, `computeTransportErrZeros`, and `computeFailClosedReturn` functions all iterate over results but only to form comma-separated lists of the declared return types — they do not pack multiple non-error results into a DTO.

**ReturnCodecFor (`pkg/codegen/typemap.go:49-59`):** Takes `results[0]` and records its codec, nullability, and type. Does not consider results beyond index 0.

**Golden tests (`pkg/codegen/multireturn_test.go`):** Four golden tests exist:
- `TestRenderServerStringErrorGolden` — `(string, error)`
- `TestRenderClientStringErrorGolden` — `(string, error)` client side
- `TestRenderServerBoolGolden` — `(bool)` single non-error
- `TestRenderServerVoidGolden` — void (no results)

No golden test exists for multi-value returns because they are refused before reaching rendering.

### DTO Design

A synthetic `ResultDTO` type will be generated for any plan with > 1 non-error return. The DTO packs all non-error returns into a single struct with named JSON-tagged fields, which is used as the single non-error result for both server response rendering and client unpacking. The generated struct name follows the pattern `<funcName>Result`.

**Field naming:** Use declared return names if present (`result`, `err`, etc. from the function signature). If absent, generate `Result0`, `Result1`, etc. The error return is never packed into the DTO — it remains the separate error channel.

**JSON codability check:** All non-error return types must be JSON-codable (primitives, structs, slices, maps, pointers to these). Channel types, function types, sync primitives, and io.Reader/Writer types cannot be packed into a DTO and trigger the existing `unsupported_result_shape` refusal.

**App-facing signature preservation:** The DTO is a transport-layer detail. Generated host stubs (client.go) preserve the original multi-value return signature. The stub unpacks the DTO fields into individual return values. On the server side, the handler packs individual call results into the DTO for transport.

### Impact on Existing Targets

The DTO normalization changes admission behavior for ALL boundaries, not just adapter-eligible ones. Existing `(T, error)` and `(T)` shapes must pass through unchanged. The `unsupported_result_shape` refusal continues to fire for genuinely non-codable shapes.

### CloudLab Regression Evidence (Phase 2)

Both SPRINT-0049/0050 stage-10 regression targets pass on CloudLab (tgoodwin-305638, c220g5) with the new DTO code path. Four regression runs performed across two sessions; the reviewer-triggered Run 4 on 2026-05-18 independently confirms the results after Runs 1-3 were rejected due to CloudLab DNS unreachability from the review session.

**Run 4 (2026-05-18, independent reviewer-triggered fresh verification):**

```
miniflux/M-1 (activation-miniflux-refreshfeed):
  Status: PASS
  Stage: 10 (full proof path)
  Duration: 4.4m (283.76s)
  Node: c220g5-111307.wisc.cloudlab.us
  Experiment: tgoodwin-305638
  Commit: e3086a5 (docs: fresh CloudLab regression evidence for Phase 2 DTO (Run 3))
  MONOLIFT_BOUNDARY_ADAPTER: 1 (default)
  MONOLIFT_E2E: 1
  Build tag: e2e
  Test command: go test -tags e2e ./test/e2e -run "^TestE2E/activation-miniflux-refreshfeed$" -count=1 -v -timeout 15m
  Log: .moab/runs/sprint-0051-phase2-regression-miniflux-m1-fresh.log
  Kind cluster: freshly created (previous monolift-e2e cluster deleted before run)
  Generated code: RefreshFeed returns (*locale.LocalizedErrorWrapper) — no DTO applied (correct)

pocketbase/M-1 (activation-pocketbase-createthumb):
  Status: PASS
  Stage: 10 (full proof path)
  Duration: 4.6m (297.92s)
  Node: c220g5-111307.wisc.cloudlab.us
  Experiment: tgoodwin-305638
  Commit: e3086a5 (docs: fresh CloudLab regression evidence for Phase 2 DTO (Run 3))
  MONOLIFT_BOUNDARY_ADAPTER: 1 (default)
  MONOLIFT_E2E: 1
  Build tag: e2e
  Test command: go test -tags e2e ./test/e2e -run "^TestE2E/activation-pocketbase-createthumb$" -count=1 -v -timeout 15m
  Log: .moab/runs/sprint-0051-phase2-regression-pocketbase-m1-fresh.log
  Kind cluster: reused from miniflux run (harness manages namespace isolation)
  Generated code: CreateThumb returns (error) — no DTO applied (correct)
```

Codegen unit tests also freshly verified on CloudLab in this session:
- Log: `.moab/runs/sprint-0051-phase2-fresh-codegen-tests.log`
- All 238 tests pass (339.6s)
- DTO-specific tests: 18 pass (6 golden, 9 admission shape, 2 round-trip, 1 cut-candidates)

Both targets use the `(T, error)` or `(error)` return shape, which passes through `admitResultShape` unchanged (no DTO built). The DTO path is exercised only for `> 1 non-error return`; these targets confirm that the new admission logic does not regress existing behavior.

**Run 3 (2026-05-18, fresh git pull + full e2e with MONOLIFT_E2E=1):**

```
miniflux/M-1 (activation-miniflux-refreshfeed):
  Status: PASS
  Stage: 10 (full proof path)
  Duration: 3.1m (210.41s)
  Node: c220g5-111307.wisc.cloudlab.us
  Experiment: tgoodwin-305638
  Commit: 35619e9 (docs: add CloudLab regression evidence for Phase 2 DTO normalization)
  MONOLIFT_BOUNDARY_ADAPTER: 1 (default)
  MONOLIFT_E2E: 1
  Build tag: e2e
  Test command: go test -tags e2e ./test/e2e -run "^TestE2E/activation-miniflux-refreshfeed$" -count=1 -v -timeout 20m
  Log: .moab/runs/sprint-0051-phase2-regression-miniflux-m1-v4.log
  Generated code: RefreshFeed returns (*locale.LocalizedErrorWrapper) — no DTO applied (correct)

pocketbase/M-1 (activation-pocketbase-createthumb):
  Status: PASS
  Stage: 10 (full proof path — corpus traces record phase "4" but e2e runs full stage ladder)
  Duration: 4.5m (294.29s)
  Node: c220g5-111307.wisc.cloudlab.us
  Experiment: tgoodwin-305638
  Commit: 35619e9 (docs: add CloudLab regression evidence for Phase 2 DTO normalization)
  MONOLIFT_BOUNDARY_ADAPTER: 1 (default)
  MONOLIFT_E2E: 1
  Build tag: e2e
  Test command: go test -tags e2e ./test/e2e -run "^TestE2E/activation-pocketbase-createthumb$" -count=1 -v -timeout 20m
  Log: .moab/runs/sprint-0051-phase2-regression-pocketbase-m1.log
  Generated code: CreateThumb returns (error) — no DTO applied (correct)
```

**Run 2 (2026-05-18, initial fresh run):**
- miniflux/M-1: PASS 3.1m (207.63s), pocketbase/M-1: PASS 4.5m (294.02s)
- Logs: `.moab/runs/sprint-0051-regression-v2/`

**Run 1 (2026-05-17, initial implementation):**
- Logs: `.moab/runs/sprint-0051-regression/miniflux-m1-stage10.log` (4.5m pass), `pocketbase-m1-stage10.log` (6.3m pass)
- Rejected by reviewer due to CloudLab DNS unreachability from review session

**Codegen unit/golden tests on CloudLab:**
- Run 4 log: `.moab/runs/sprint-0051-phase2-fresh-codegen-tests.log` (all pass, 339.6s on c220g5)
- Run 3 log: `.moab/runs/sprint-0051-phase2-codegen-tests-v3.log` (all pass, 340.7s on c220g5)
- Run 2 log: `.moab/runs/sprint-0051-phase2-codegen-tests-v2.log`
- Run 1 log: `.moab/runs/sprint-0051-phase2-codegen-tests.log`
- All 238 tests pass including six DTO golden-file tests and nine DTO admission shape tests
- Admission tests (19 tests): All pass including DTO-specific tests for (T,error) no-DTO, (T) no-DTO, (T,U,error) DTO, M-4 shape DTO, (T,T) DTO, void refused, chan refused, func refused, io.Writer refused
