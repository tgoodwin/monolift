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
