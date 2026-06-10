# SPRINT-0051 Codex Handoff (Phases 4–8)

You are gpt-5.4 implementing **SPRINT-0051** (`docs/sprints/SPRINT-0051.md`). Phases 0–3 are already complete (boxes checked). Begin at **Phase 4** and continue through Phase 8, ticking `- [ ]` boxes to `- [x]` as each task is genuinely done.

## What Phase 3 already shipped (commit 1f37144)

- `pkg/codegen/adapter_pass.go` — `TryAdapterPass(AdapterContext) (*AdapterPlan, []AdmissionRefusal)`.
- `pkg/codegen/adapter_patterns.go` — `AdapterPatternImpl` interface, `multipart_file_read_all`, `bytes_reader_return`, LiveProxyRequired exclusion list, SSA producer resolution.
- `pkg/codegen/adapter_patterns_test.go` — golden test for `processImage` AdapterPlan plus negative fixtures.
- `pkg/codegen/testdata/adapter_processimage_plan.golden.json` — committed golden.

`AdapterContext` fields: `Fn *ssa.Function`, `CallSites []*ssa.CallCommon`, `MaxInlinePayloadBytes int` (default 8 MiB), `FunctionExported bool`. Pattern impls live in `adapterPatternRegistry`. The pattern interface owns its render methods (`RenderInputExtraction`, `RenderRemoteReconstruction`).

## Recommended Phase 4 design

The host wrapper must preserve the original function name. The clean split:

1. **HTTP client** (renamed): `monoliftRemote<FuncName>(inputs...) (DTO, error)` — generic `RenderClient` output, takes the *normalized* signature derived from `AdapterPlan.InputTransforms` / `OutputTransforms`.
2. **Host wrapper** (new file `monolift_wrapper_<env>.go` in the cut package): `<FuncName>(originalParams) (originalReturns)` — drains awkward inputs via pattern `RenderInputExtraction`, calls `monoliftRemote<FuncName>`, on error falls back to the renamed original (`monoliftOriginal<FuncName>`), then rehydrates returns via pattern `RenderRemoteReconstruction`.
3. **Normalized helper** (new file `monolift_normalized_<env>.go` in the cut package): `monoliftNormalized<FuncName>(normalizedInputs...) (DTO, error)` — the rewritten body with the per-pattern prologue substituted (e.g. `bytes.NewReader(input)` for the FileHeader-Open prologue).
4. **MonoliftInvoke adapter** (`adapter.go`): when `Plan.AdapterPlan != nil`, change the call target from `monoliftOriginal<FuncName>` to `monoliftNormalized<FuncName>` so the extracted service exercises the rewritten body, not the original.

The patcher (`PatchCutFunction`) still renames the original to `monoliftOriginal<FuncName>` and writes the new file containing `<FuncName>`. Adjust so that when `AdapterPlan != nil`, the new `<FuncName>` is the host wrapper (not the bare HTTP client). The renderer for the normalized helper produces a separate file alongside.

For the body rewrite (Phase 4.2), use `github.com/dave/dst` (already a dependency in `pkg/codegen/patch.go`). The pattern-specific surgery for `multipart_file_read_all`: locate `<param>.Open()` call, the matching error check, and the `defer Close()`, strip them, and replace the single use of the opened reader with `bytes.NewReader(input)`. The return rewrite for `bytes_reader_return` substitutes `bytes.NewReader(out.Bytes())` (or similar) with the corresponding DTO field. Guarded by the `adapter_use_shape` and `adapter_return_rehydration` proofs that already passed in Phase 3.

If pattern-specific surgery turns out to be infeasible for the M-4 case, fall back to clone-and-replace per spec §4.2 — generate the helper from scratch using a pattern-aware template informed by the original body's SSA. Either approach is acceptable; pick the one with the cleaner golden.

## Phase 5 (recovery branch)

The Phase 1 stub already exists in `pkg/codegen/cut_admit.go:115-132` (`if adapterEnabled && isAdapterEligibleRefusal(refusal)`). Replace it with a real call to `TryAdapterPass`. On success, attach `AdapterPlan` to the candidate's plan and return accepted; on failure, mark `candidate.AdapterClass` (`AdapterUnknown`/`LiveProxyRequired`/`AdapterImpossible`) with the refusal reason and fall through to demotion. The exclusion guards from `docs/research/runs/SPRINT-0051-adapter-decisions.md §0.4` must be codified before calling `TryAdapterPass` (skip receiver reconstruction failures, infrastructure reconstructors, broad parent cuts, Moderate/Many callbacks).

Critical pipeline test (Phase 5.5): direct admission of `processImage` refuses, recovery accepts, pipeline does NOT select `(*App).UploadMedia`. Reuse `cut_admit_test.go` fixture patterns.

## Phase 6 (CloudLab e2e)

Requires an active CloudLab experiment. **Before doing Phase 6 work**, run `cl ls` / `cl status` locally to confirm the `monolift-buildserver` profile is up. If no experiment is active, write a `## Blockers` section at the bottom of `docs/sprints/SPRINT-0051.md` saying "Phase 6 requires CloudLab experiment; user must start the monolift-buildserver profile" and exit Phase 6 only — continue with Phase 7 if possible.

Build node layout: repo is at `/local/repository` on `monolift-buildserver`; cd there for heavy work, never re-clone. All artifacts under `.moab/runs/sprint-0051-*`. One stage per `go test` process, never jump (per R.6). The stage ladder is 4 → 5 → 6 → 7 → 8 → 9 → 10.

Scaffold from `test/e2e/targets/activation_listmonk_sanitizeuri/` (closest in shape, shared Listmonk auth + Postgres fixture). The fixture PNG already exists at `test/e2e/targets/activation_listmonk_processimage/testdata/fixture.png` per Phase 0.7.

## Phase 7 (documentation)

- `docs/decisions/0032-boundary-adapter-recovery.md` — full ADR per task 7.1. Reference ADR-0028 explicitly and explain why `AdapterPossible` is *not* a resurrection of `FeasibleWithProxy`.
- `docs/evolution.md` — one paragraph on the adapter pass and the M-4 result.
- `docs/research/activation-paths/analyses/listmonk-M-4.md` — full rewrite. Retire "Proxy-required"/"Feasible-with-proxy" terminology.
- `docs/research/activation-paths/cut-placement-synthesis.md` — touch up the "Framework Callback" row.
- `docs/research/runs/SPRINT-0051-coverage-report.md` — new file per task 7.5.

## Phase 8 (verification)

Run on CloudLab per R.2. `go test ./pkg/activation/... ./pkg/codegen/... ./test/e2e/harness/...`. The flag-on / flag-off parity sweeps are the load-bearing checks. Don't run a broad `make e2e` — focused targets only.

## Hard rules

- **Never select `(*App).UploadMedia`** as the M-4 cut. Multiple reinforcing checks pin `selected_cut == processImage`.
- **Don't fold `AdapterClass` into `BoundaryDataClass`.** They stay orthogonal.
- **Don't ship a live proxy renderer.** `staged_object` enum value exists; renderer does not.
- **Don't touch the ledger** — `executor: gpt-5.4` is already set. The orchestrator manages ledger updates.
- **Don't touch `docs/sprints/drafts/`** — historical.
- **Inline JSON/base64 only**, 8 MiB ceiling. Above that: refuse with `adapter_payload_too_large`.
- **No `--no-verify`, no `--no-gpg-sign`, no `Co-Authored-By: Claude ...` trailer.** Project preference.

## Working style

- Tick `- [x]` boxes in `docs/sprints/SPRINT-0051.md` as each task is genuinely done, not at the end.
- On a real blocker (missing CloudLab access, unrecognized SSA shape that needs a spec call), write a short `## Blockers` section at the bottom of the plan and exit. Otherwise keep going until every box is checked.
- Commit at natural boundaries (end of each phase is reasonable). Use focused commit messages. Don't bundle unrelated changes.
