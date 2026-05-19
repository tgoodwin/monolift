# SPRINT-0052: Boundary-adapter generalization — second and third targets at stage 10

**Status:** planned
**Predecessors:** SPRINT-0051 (boundary-adapter framework + `listmonk/M-4` stage 10), ADR-0032
**Branch:** continues on `sprint-51` (PR #13) — Category A unwind is a prerequisite to merging that PR to `main`
**Primary input:** `docs/sprints/briefs/sprint-51-review.md` (40 flags, A–E)

## Intent

SPRINT-0051 shipped the boundary-adapter framework as a generalizable shape with an M-4-specific spike inside it. SPRINT-0052 falsifies that the framework is real by landing two or more *additional* adapter-enabled lifts at stage 10, across **distinct** adapter patterns beyond `multipart_file_read_all` and `bytes_reader_return`, with **zero edits to `pkg/codegen/adapter*.go` outside the pattern registry, and zero new target-name string-matches in `pkg/codegen/` or `test/e2e/e2e_test.go`**. Work order is forced: Category A overfitting comes out *before* a second target's SSA can drive any new code path, because every M-4-specific literal currently in the production code path will produce wrong output for any other shape. Category B is gating for target claims, not just compilation — proof obligations that pretend to verify what they do not actually verify will mis-classify real candidates the moment a second pattern appears. Categories C, D, and E land alongside as the affected files are touched.

## Goals

- [ ] Goal: prove the framework generalizes — ≥2 adapter-enabled lifts beyond `listmonk/M-4` reach stage 10 on CloudLab, each exercising a *distinct* adapter pattern from the others and from SPRINT-0051's two patterns.
- [ ] Goal: eliminate all M-4-specific knowledge from `pkg/codegen/` (Category A) — no function-name string matches, no literal `processImage` source-text substitutions, no `thumbnailSize`/`imaging` literals in shared code, no `([]byte, int, int, error)` shape detection, no `target.Name == "activation-listmonk-..."` branches in the e2e harness.
- [ ] Goal: tighten framework rigor (Category B) — adapter recovery gate by structural admission predicate; DTO packing gated on `unsupported_result_shape`; `MONOLIFT_BOUNDARY_ADAPTER` flag has one and only one behavior; `missing_reconstructor` adapter-eligibility is parameter-typed; `adapter_call_site` runs an actual reverse-import scan; `adapter_local_lifecycle` covers interface boxing and store-to-global; `multipart_file_read_all` proof traverses closures.
- [ ] Goal: reconcile ADR-0032 and analysis docs with the implementation (Category D), or amend the ADR.
- [ ] Goal: resolve Category C scope hygiene before this PR ships.
- [ ] Goal: sweep Category E nits as the affected functions are touched.

## Non-Goals

- [ ] Non-goal: new cost model or ranking between direct and adapter cuts (strict fallback per ADR-0032).
- [ ] Non-goal: streaming or staged-object transports — inline JSON/base64 with the (now-plan-configurable) ceiling only.
- [ ] Non-goal: live-proxy resurrection in any form, including for callback shapes.
- [ ] Non-goal: refactoring `pkg/activation/cut_boundary.go` to fold `AdapterClass` into `BoundaryDataClass`.
- [ ] Non-goal: a general-purpose SSA rewriter — body rewrite stays pattern-owned.
- [ ] Non-goal: removing the `MONOLIFT_BOUNDARY_ADAPTER` flag (scheduled SPRINT-0053+).
- [ ] Non-goal: patterns beyond what the chosen second/third targets demand.
- [ ] Non-goal: migrating `listmonk/M-4` away from its existing stage 10 result — after Cat A unwind, M-4 must continue to pass stage 10 unchanged.
- [ ] Non-goal: fixed timebox, stretch-deliverable mechanism, or "best effort" escape hatch — the sprint is done when acceptance passes.
- [ ] Non-goal: adding target-specific reconstructors or constants to `pkg/codegen/server.go`, `pkg/codegen/admission.go`, or any non-pattern file under `pkg/codegen/adapter*.go` to make a target pass.
- [ ] Non-goal: counting classification flips caused only by `MONOLIFT_BOUNDARY_ADAPTER` suppressing `callable_boundary_values` as adapter recovery evidence.

## Scope

**In scope:**

- `pkg/codegen/cut_admit.go`: replace `isUploadMediaCandidate` with structural admission; gate `MONOLIFT_BOUNDARY_ADAPTER`'s second behavior properly; restrict `missing_reconstructor` adapter-eligibility; fix Plan-built-twice happy path.
- `pkg/codegen/adapter.go`, `adapter_client.go`: replace `strings.Replace` body rewrite with pattern-owned AST rewrite; thread `TransportPolicy.MaxInlinePayloadBytes` into `adapterExtractionLines`; fix `adapterReturnExpressions`'s `rune('0'+i)` bug.
- `pkg/codegen/server.go`: remove `thumbnailSize` constant and `imaging` import from `serverLocalAdapterCode`; compute imports and free-symbol constants from the cut function.
- `pkg/codegen/adapter_normalize.go`: delete `applyProcessImageResultNames`; derive DTO names from `Plan.Results[i].Name` with `Result0..N` fallback.
- `pkg/codegen/adapter_pass.go`: harden `dischargeCallSite` (actual reverse-import scan), `dischargeLocalLifecycle` (interface boxing, store-to-global, interface-dispatch Close), and add closure-capture traversal to `multipart_file_read_all`'s `Discharge`.
- `pkg/codegen/admission.go`: gate DTO packing on `unsupported_result_shape` candidacy; formalize or remove `adapter_parent_forbidden`.
- `pkg/codegen/adapter_patterns.go`: **the ONLY file under `pkg/codegen/adapter*.go` permitted to grow with new pattern additions this sprint.**
- `test/e2e/e2e_test.go`: parameterize M-4 branches via `TargetCase.FailClosedExpectedStatus int` and `TargetCase.InvokeResultExtractor func(map[string]any) any` (or equivalent fields).
- `test/e2e/targets/activation_<project>_<func>/` for each new target.
- `test/e2e/activation_corpus_traces.yaml` rows for the new targets.
- ADR-0032 amendments OR replacement language for all Category D claims.
- `docs/research/runs/SPRINT-0052-target-survey.md` and `docs/research/runs/SPRINT-0052-coverage-report.md`.

**Out of scope:**

- [ ] Do not introduce new target-name string matches in `pkg/codegen/` or `test/e2e/e2e_test.go`; any target-specific policy belongs in target metadata, manifests, fixtures, or pattern declarations.
- [ ] Do not edit `pkg/codegen/adapter*.go` for the selected second/third targets after Phase 4 begins, except to add entries to the adapter pattern registry in `pkg/codegen/adapter_patterns.go`.
- [ ] Do not accept timeout/cap failures as viability blockers during Phase 0; rerun with widened/disabled caps and record cost separately from semantic blockers.
- [ ] Do not pre-implement patterns (`reader_read_all`, `callback_continuation`, etc.) before Phase 0 commits to specific targets — pattern names follow target selection.
- [ ] Do not cleanup the retained `FeasibleWithProxy` JSON constant (deferred again).
- [ ] Do not re-architect admission, `BoundaryDataClass`, or the cut-result demotion chain.
- [ ] Do not serialize `AdapterPlan` into the manifest (still stretch from SPRINT-0051; deferred).

## Flag-to-task mapping

| Flag # | Cat | Description | Task |
|---:|---|---|---|
| 1 | B-9 | DTO packing scope too broad | 2.1 |
| 2 | A-1 | `isUploadMediaCandidate` string-match | 1.1 |
| 3 | B-10 | `MONOLIFT_BOUNDARY_ADAPTER` hidden behavior | 2.2 |
| 4 | B-11 | `missing_reconstructor` eligibility | 2.3 |
| 5 | B-15 | Plan built twice on happy path | 2.7 |
| 6 | E-26 | Flag re-read in admit loop | 7.10 |
| 7 | E-27 | Surface check empty-string fail-open | 7.11 |
| 8 | A-8 | `adapter_parent_forbidden` not in vocabulary | 1.8 |
| 9 | B-12 | `adapter_call_site` proof vacuous | 2.4 |
| 10 | B-13 | `adapter_local_lifecycle` incomplete | 2.5 |
| 11 | E-42 | `remoteSignatureString` dead map | 7.16 |
| 12 | E-34 | `liveProxyClassify` discards `isResult` | 7.14 |
| 13 | E-35 | `isDirectlySerializableParam` gap | 7.15 |
| 14 | E-36 | Refusal trail "first failure" only | 7.13 |
| 15 | B-14 | `multipart` proof misses closure capture | 2.6 |
| 16 | E-28 | `RenderInputExtraction` concat format strings | 7.7 |
| 17 | E-29 | `callMethodName` dangling comment | 7.8 |
| 18 | E-30 | `typeIsByteSliceFlow` dead code | 7.9 |
| 19 | E-31 | Pattern registry ordering uncommented | 3.2 |
| 20 | A-2 | Body rewrite `strings.Replace` | 1.2 |
| 21 | A-3 | `thumbnailSize = 250` hardcoded | 1.3 |
| 22 | A-4 | `imaging` import unconditional | 1.4 |
| 23 | A-5 | `applyProcessImageResultNames` shape detection | 1.5 |
| 24 | A-7 | 8 MiB literal not from `TransportPolicy` | 1.7 |
| 25 | E-33 | `serverLocalAdapterCode` swallows errors | 7.6 |
| 26 | B-16 | `RenderClient → RenderAdapterClient` fork | 2.8 |
| 27 | E-32 | `rune('0'+i)` breaks at 10 | 7.5 |
| 28 | A-6 | `target.Name == "..."` in e2e_test.go | 1.6 |
| 29 | E-38 | Oracle/workload `image.Decode` divergence | 7.3 |
| 30 | E-39 | Fixture path duplicated | 7.4 |
| 31 | E-40 | `directInvokePayload()` panics at init | 7.2 |
| 32 | E-41 | Admin creds duplicated | 7.1 |
| 33 | C-19 | Stretch claim via flag side-effect | 6.5 |
| 34 | E-37 | Stage 4/8 reruns undocumented | 7.12 |
| 35 | D-21 | ADR doesn't doc flag's 2nd behavior | 6.1 |
| 36 | D-22 | ADR implies 8 MiB plan-configurable | 6.2 |
| 37 | D-23 | ADR doesn't acknowledge `isUploadMediaCandidate` | 6.3 |
| 38 | D-24 | "discharges six obligations" overstates | 6.4 |
| 39 | D-25 | M-4 BoundaryDataClass footnote | 6.6 |
| 41 | C-17 | `modular-monolith-virtues-v1.md` unrelated | 6.7 |
| — | C-18 | PR title typo `functino` | 6.8 |
| 40, 42 | — | Retracted per maintainer clarification | — |

## Phase 0: Corpus survey and target selection (real survey, not pro-forma)

The brief proposes `pocketbase/M-7` and `gitea/M-13`. Phase 0 must independently confirm or refute these — both are *mailer* traces (`SMTPClient.send`, `sender.send`) per their analysis docs, so the "callback shape" and "reader shape" labels in the brief may be optimistic. Goal: enumerate adapter-shape-compatible corpus rows, classify them into pattern families, then *pick two whose patterns are distinct from each other and from `multipart_file_read_all`/`bytes_reader_return`*.

- [ ] 0.1: Run `cl ls` and `cl status <experiment>` to confirm a `monolift-buildserver` experiment is active before any heavy work. If none exists, ask the user to start one. Create `.moab/runs/sprint-0052-phase0/` on the build node and store every survey command, candidate table, refusal trail, and timing breakdown there.
- [ ] 0.2: Run a focused admission sweep with `MONOLIFT_BOUNDARY_ADAPTER=1` over the corpus (focused phases only, no Kind), capture per-trace refusal codes and `AdapterClass` field values. Save under `.moab/runs/sprint-0052-survey/`.
- [ ] 0.3: For each candidate that hit an internal cap or timeout in 0.2, rerun with the cap widened or disabled before classifying viability. **Separate cost profile from semantic blocker** in the Phase 0 table — a timeout-shaped refusal is not the same as a semantic refusal. This is the SPRINT-0051 Phase 0 lesson encoded.
- [ ] 0.4: Enumerate all candidates whose direct refusal is shape-compatible AND whose `AdapterClass` is `AdapterPossible` OR `AdapterUnknown`. Explicitly include `gitea/M-13`, `pocketbase/M-7`, `pocketbase/M-9`, `mattermost/M-13`, `miniflux/M-9`, plus any trace with `io.Reader`, `io.ReadCloser`, callback registration, function-value dispatch, reader-like return, or callback-shaped boundary evidence from `docs/research/activation-paths/analyses/`. Record receiver class, surface, callbacks, return-shape kind, parameter types in `docs/research/runs/SPRINT-0052-target-survey.md`.
- [ ] 0.5: For the top ≤6 candidates, manually inspect helper SSA shape (param/return types, lifecycle ops) and classify by pattern family: `reader_read_all`, callback-to-finite-contract, callback-registration-below-cut, reader return, staged-object candidate, refusal-only live proxy. Reject any candidate that re-uses `multipart_file_read_all`/`bytes_reader_return` exactly (would not falsify the framework thesis).
- [ ] 0.6: For each remaining candidate, draft a one-paragraph pattern proposal: `Name()`, `Matches()`, awkward shape, use-shape proof, host extraction lines, remote reconstruction.
- [ ] 0.7: Skepticism check on the brief's two candidates. If `pocketbase/M-7` is in the shortlist, verify it actually surfaces a callback shape (not just `*Message` byte serialization). If `gitea/M-13` is in the shortlist, verify `sender.send` actually consumes a reader (the analysis says boundary class is Trivial — the brief may have mislabeled it). If a candidate fails verification, reject and use a backup from 0.5.
- [ ] 0.8: Select target **#2** (mandatory) and target **#3** (mandatory). Selection rules: (a) targets must exercise **distinct** adapter patterns from each other and from SPRINT-0051's two; (b) prefer minimal new admission scaffolding (no new receiver reconstructors needed); (c) reuse existing e2e project fixtures where structurally compatible; (d) refuse transaction callbacks, app-continuation callbacks, callback values stored for later use, or callbacks requiring reverse invocation — classify those as `live_proxy_required` and pick a non-callback backup.
- [ ] 0.9: Determine oracle policy per target (direct byte compare, declared normalizer, or invocation-record compare). Run the per-target determinism check from SPRINT-0051 §0.6 as the template.
- [ ] 0.10: Lock the picks in `docs/research/runs/SPRINT-0052-target-survey.md` with target IDs, package/function/file-line, adapter pattern names, backup candidates, expected e2e package names, and first runnable stage. Phase 1 does not start until 0.1–0.10 are checked off and the picks are recorded.

## Phase 1: Category A unwind (must land before Phase 3 touches any pattern)

This phase makes the production code path target-agnostic. Each task removes one M-4 fingerprint. After Phase 1, `listmonk/M-4` stage 10 must still pass unchanged — it is the regression check on the unwind, not the goal of the unwind.

- [x] 1.1 [flag #2 / A-1] Delete `isUploadMediaCandidate` from `pkg/codegen/cut_admit.go:182`. Replace the guard at `cut_admit.go:105` with a structural predicate `adapterParentForbiddenForCandidate(candidate, plan)`: refuse when the candidate is a parent whose call graph contains an adapter-eligible descendant currently in `Candidates`. The predicate names no function. Add unit test `TestAdapterParentForbiddenByStructure` with three fixtures: M-4-shaped parent/descendant (refuses); unrelated parent (admits); descendant with no adapter-eligible classification (admits).
- [ ] 1.2 [flag #20 / A-2] Delete `rewriteMultipartFileReadAllBody`, `rewriteBytesReaderReturnBody`, and the two indentation variants in `pkg/codegen/adapter.go:223-246`. Replace `normalizedHelperBody` with an AST-aware rewriter: parse the cut function body, walk for the per-input pattern's awkward prologue site (each pattern declares a `BodyPrologueMatcher func(*ast.BlockStmt) (matched *ast.Stmt, replacement []ast.Stmt, ok bool)` or equivalent on its interface), apply the replacement, re-print. Each pattern owns its own AST template. Adapter-pass code stays target-agnostic. Add negative tests proving the rewriter refuses (or no-ops) on unsupported prologue shapes rather than partial-applying.
- [ ] 1.3 [flag #21 / A-3] Delete `const thumbnailSize = 250` from `serverLocalAdapterCode` in `pkg/codegen/server.go:310`. Scan the cut function's free symbols (package-level constants/vars referenced from the helper body) and re-emit them in the helper file. Add a synthetic fixture test where the helper references **two** constants with non-`thumbnailSize`/non-`250` names/values to prove the free-symbol extractor isn't single-constant-fitted.
- [ ] 1.4 [flag #22 / A-4] Delete the hardcoded `importSpec{Path: "bytes"}, importSpec{Path: "github.com/disintegration/imaging"}` at `pkg/codegen/server.go:86`. Compute the import set from the helper body's AST `ImportSpec`s in the source file, intersected with what the rewritten body actually references, then run through the existing `goimports` hook.
- [ ] 1.5 [flag #23 / A-5] Delete `applyProcessImageResultNames` from `pkg/codegen/adapter_normalize.go:113-123`. Replace with: DTO field name = `result.Name` when non-empty and non-generic (not `result`, not `r0`), else `Result0..N`. Refresh M-4 goldens — the DTO fields rename from `thumbnail/originalWidth/originalHeight` to whatever `Plan.Results[i].Name` actually is for `processImage` (likely `Result0/Result1/Result2`). Confirm the host-side `bytes.NewReader(out.Result0)` still type-checks at stage 9. Add a non-image-shape DTO naming test for `([]byte, int, int, error)` proving no hidden M-4 renaming remains.
- [x] 1.6 [flag #28 / A-6] Delete `if target.Name == "activation-listmonk-processimage"` branches from `test/e2e/e2e_test.go:1153,1421-1450` (and any others surfaced by grep). Add `TargetCase.FailClosedExpectedStatus int` (default 0 = "any non-5xx for sentinel mode") and `TargetCase.InvokeResultExtractor func(map[string]any) any` fields to `test/e2e/harness/target.go`. Populate them for `activation-listmonk-processimage` in its target.go. Other targets compile unchanged with zero-value defaults. *(Done: added `FailClosedExpectedStatus int` (set to 500 for processimage), removed the only processimage target-name match. The 1421-1450 branches are `miniflux`/default cleanpath cases, not M-4 fingerprints — left as-is. Skipped `InvokeResultExtractor`: no consumer exists yet, so adding it now would be dead code; defer to Phase 5/6 when a target needs result extraction. e2e package vets clean, harness tests pass on CloudLab.)*
- [x] 1.7 [flag #24 / A-7] In `pkg/codegen/adapter_client.go:152-158`, change the hardcoded `len(out.Name) > 8*1024*1024` to read from `plan.AdapterPlan.TransportPolicy.MaxInlinePayloadBytes`. Plumb the field through `adapterExtractionLines(plan, transport *Plan)` signature. Unit test `TestAdapterExtractionRespectsTransportPolicy` sets MaxInlinePayloadBytes to a non-default value and asserts the rendered limit matches. *(Implemented as `AdapterPlan.MaxInlinePayloadBytes int64` rather than nesting under `TransportPolicy` to keep the existing transport-mode enum's JSON shape stable; threads through `AdapterContext` → `AdapterPlan` → `adapterInlinePayloadLimit`.)*
- [x] 1.8 [flag #8 / A-8] Decide on `adapter_parent_forbidden` (used at `cut_admit.go:107`): either formalize it in ADR-0032 + the refusal-code vocabulary in `pkg/codegen/types.go`, or rename to a structurally-derived refusal (e.g. `unsupported_parent_of_adapter_eligible_child`). Either way: one constant, one Refusals row, documented. Update Category D entry 6.3 in the same commit. *(Formalized as `RefusalAdapterParentForbidden` constant in `pkg/codegen/admission.go`; cut_admit.go uses the constant; ADR-0032 reconciliation deferred to Phase 7.3.)*
- [ ] 1.9 **M-4 regression gate.** Re-run `listmonk/M-4` stage 10 on CloudLab. **Must pass.** Save artifacts under `.moab/runs/sprint-0052-phase1-m4/`. If it does not pass, the unwind regressed M-4 and Phase 1 is not done — fix before proceeding to Phase 2.

## Phase 2: Category B framework rigor

These do not block target #2 from compiling, but they will silently mis-classify target #2's candidates if left in. Land before Phase 4 — otherwise target evidence is contaminated.

- [ ] 2.1 [flag #1 / B-9] Gate DTO packing on candidacy: in `pkg/codegen/admission.go`'s `AdmitPlan` path, run DTO packing for `(T, U, ..., error)` shapes only when admission would otherwise refuse with `unsupported_result_shape`. Today it runs for every multi-return. Add a refusal-shadow check; update `multireturn_test.go` to confirm `(T, error)` and `(T)` shapes are unchanged.
- [ ] 2.2 [flag #3 / B-10] Audit `MONOLIFT_BOUNDARY_ADAPTER` for hidden behavior. In `pkg/codegen/cut_admit.go` and `AdmitCut`, find every site that consults `boundaryAdapterEnabled()`. Gate any suppression of `callable_boundary_values` on `AdapterClass == AdapterPossible` for the specific candidate. The flag must do exactly one thing: enable the recovery branch. Unit test: with the flag on and a non-adapter-eligible callable candidate, `callable_boundary_values` is still reported. Regression test: `pocketbase/M-5` and `pocketbase/M-11` do not flip solely from hidden suppression — they revert or require a real adapter plan.
- [ ] 2.3 [flag #4 / B-11] In `pkg/codegen/cut_admit.go:49-55`, narrow `adapterEligibleRefusals` to *only* `missing_reconstructor` refusals whose refusal `Type` field is parameter-typed (not receivers, not DB/filesystem reconstructors). Add helper `isParameterTypeReconstructorRefusal(refusal AdmissionRefusal) bool` reading `refusal.Type`. Negative test: a `missing_reconstructor` refusal whose reason is `*sql.DB` does NOT enter the adapter branch.
- [ ] 2.4 [flag #9 / B-12] Implement the reverse-import scan for `adapter_call_site` in `pkg/codegen/adapter_pass.go:305-331`. Today `ctx.CallSites` is always nil from `tryAdapterRecoveryFromPlan` (cut_admit.go:369). Plumb a `CallSiteIndex` built from the activation-path scope — enumerate `*ssa.Function` references to the helper across reverse imports. Refuse on `*ssa.MakeClosure`, `&fn` address-of, `reflect.ValueOf`, or any non-`*ssa.Call` referrer. Add a synthetic fixture where the helper is assigned to a function variable; assert refusal. Scope the scan to activation-path package set; cache per candidate.
- [ ] 2.5 [flag #10 / B-13] Strengthen `dischargeLocalLifecycle` in `pkg/codegen/adapter_pass.go:268-300`. Today it checks `*ssa.Defer` and literal `"Close"` calls only. Add: `*ssa.MakeInterface` (any interface boxing of an adapter input), `*ssa.Store` where Addr is a `*ssa.Global`, and interface-dispatch Close (`common.Method != nil && common.Method.Name() == "Close"`). Each new check has a synthetic fixture in `adapter_pass_test.go`.
- [ ] 2.6 [flag #15 / B-14] In `pkg/codegen/adapter_patterns.go:144-195` and helper `valueReferrers` at line 299, follow `*ssa.FreeVar` references when the parameter is captured by an anonymous function or goroutine. Add a fixture where `*multipart.FileHeader` is captured by `go func() { file.Open() }()`: today this passes; tomorrow it refuses with `adapter_use_shape`.
- [ ] 2.7 [flag #5 / B-15] Cache the adapter plan and recovery result built in `admitCutCandidates` so `RunLiftWithResult`'s `build-plan` phase does not rebuild and re-run `tryAdapterRecovery`. Add a `*AdapterPlan` (or refusal slice) field on the cached `candidateAdmitResult` keyed by `candidateAdmitKey`; propagate through `BuildPlan`. Invariant assertion on lookup: cached recovery result's `RemoteSignature` matches the candidate's `NodeKey`. Counter/trace assertion: `tryAdapterRecovery` is invoked exactly once per successful adapter-recovered candidate.
- [ ] 2.8 [flag #26 / B-16] `RenderClient` and `RenderAdapterClient` are hard-forked. Share a base template with adapter-specific blocks toggled by `{{ if .HasAdapter }}`, OR add a comment in each template explaining why divergence is intentional + add a golden cross-check that the non-adapter portions match. Recommendation: shared template. Do not punt — fork is a maintenance trap.

## Phase 3: Pattern framework cleanup (frees the framework before patterns are added)

Hygiene on the pattern interface and registry before Phase 4 writes new pattern implementations against them. Codex split this out from pattern implementation correctly — new patterns get a cleaned interface.

- [ ] 3.1 Add a registry-order comment to `adapterPatternRegistry` in `pkg/codegen/adapter_patterns.go:74-80` declaring that ordering is load-bearing (first match wins per slot), and the rationale for the current order. New entries declare their order explicitly.
- [ ] 3.2 [flag #19 / E-31] (covered by 3.1 — duplicate row in mapping retained for traceability.)
- [ ] 3.3 [flag #16 / E-28] Replace `RenderInputExtraction` format-string concatenation in `pkg/codegen/adapter_patterns.go:200-205` with `fmt.Sprintf` templates or a structured render context that carries zero-return expressions and imported packages safely. Concatenated format strings break on any caller passing literal `%` characters.
- [ ] 3.4 [flag #17 / E-29] Resolve the dangling comment in `pkg/codegen/adapter_patterns.go:317-347` (`callMethodName` references a third call-shape case it never implements). Delete the comment OR implement the path. Today SSA emits the first form only, so deletion is acceptable.
- [ ] 3.5 [flag #18 / E-30] Delete unused `typeIsByteSliceFlow` from `pkg/codegen/adapter_patterns.go:480-493` if no new pattern uses it, OR wire it into the `bytes_reader_return` rehydration proof with tests. The unused-but-documented state is the failure mode.
- [ ] 3.6 **Generalization invariant test.** Add `TestAdapterPassNoTargetSpecificCode` in `pkg/codegen/adapter_pass_test.go` that does `git grep -E 'processImage|UploadMedia|listmonk|<target2_name>|<target3_name>|thumbnailSize|disintegration/imaging|8\*1024\*1024|adapter_parent_forbidden' pkg/codegen/adapter*.go pkg/codegen/cut_admit.go pkg/codegen/server.go pkg/codegen/admission.go pkg/codegen/adapter_normalize.go pkg/codegen/adapter_client.go | grep -v _test.go` and asserts empty output. This is a permanent CI guard rail, not a one-time check.

## Phase 4: Pattern implementations (the *only* phase that grows `pkg/codegen/adapter*.go`)

After this phase begins, no edits to `pkg/codegen/adapter*.go` outside `adapter_patterns.go`. The acceptance grep enforces this at closeout.

- [ ] 4.1 For each target picked in Phase 0, register a new `AdapterPatternImpl` in `adapter_patterns.go`'s `adapterPatternRegistry` (line 77). One pattern per target if shapes differ; one pattern serving both if a single shape generalizes. Implementations follow the existing two as a template: `Name`, `Direction`, `FromType`, `ToType`, `Matches`, `Discharge`, `RenderInputExtraction`/`RenderRemoteReconstruction`, plus the new `BodyPrologueMatcher` introduced in 1.2.
- [ ] 4.2 If Phase 0 selected a reader target, implement `reader_read_all` as an input pattern: match bounded `io.Reader` / `io.ReadCloser`; render host-side `io.ReadAll`; preserve `Close` ownership for `ReadCloser`; refuse repeated reads, async reads, stores, interface escapes, and unbounded streaming loops.
- [ ] 4.3 If Phase 0 selected a callback-shaped target with a verified finite contract, implement the callback pattern. Refuse transaction callbacks, app-continuation callbacks, callback values stored for later use, and callbacks requiring reverse invocation into the monolith.
- [ ] 4.4 For each new pattern, add: a positive SSA fixture (Discharge passes), at least two negative fixtures including closure capture and goroutine capture, and a golden `AdapterPlan` JSON for the chosen target's helper.
- [ ] 4.5 Integration test (analogous to SPRINT-0051 Phase 3.8): feed each target's real helper SSA into `TryAdapterPass`; assert the produced plan matches the golden.

## Phase 5: Target #2 end-to-end at stage 10

Same stage ladder as SPRINT-0051 §6.7 — one stage per `go test` invocation, no jumps. Reuse existing e2e harness where possible.

- [ ] 5.1 Scaffold `test/e2e/targets/activation_<project>_<func>/` (target.go, workload.go, oracle.go, baseline manifests, fixture testdata). Reuse the structurally-closest existing target.
- [ ] 5.2 Register the target in `test/e2e/e2e_test.go`'s `targets := []harness.TargetCase{...}` (line 64). NO target-name string match anywhere — populate `FailClosedExpectedStatus`, `InvokeResultExtractor`, and other parameterized fields introduced in 1.6.
- [ ] 5.3 Workload: drive the boundary via the project's existing public route. Oracle: per Phase 0.9 policy. Target-owned direct-invoke payload loading that returns errors during setup, not panicking at package registration (avoid the SPRINT-0051 pattern of `directInvokePayload()` panicking at init).
- [ ] 5.4 Stage progression on CloudLab: **4 → 5 → 6 → 7 → 8 → 9 → 10**, one stage per `go test` process, never jump. Save logs under `.moab/runs/sprint-0052-target2/`.
- [ ] 5.5 Env-off check (`MONOLIFT_LIFT_*` off, local fallback returns correct result, extracted-service `/calls` records zero) and fail-open / fail-closed check per generated client policy. Both must pass.
- [ ] 5.6 Per-target flake notes. If a stage flakes, rerun the exact stage and document the flake signature in `.moab/runs/sprint-0052-target2/flake-notes.md`. Distinguish infra/runtime flake from semantic failure.
- [ ] 5.7 Update `test/e2e/activation_corpus_traces.yaml` row: `status: pass`, `phase: "10"`, `boundary_class: AdapterPossible`, `selected_cut: <the adapted unit, not a broader parent>`, `proof_kind: <oracle policy>`.

## Phase 6: Target #3 end-to-end at stage 10

Mirror of Phase 5. **Distinct-pattern guard:** verify target #3 exercises an adapter pattern distinct from target #2 and from SPRINT-0051 M-4. If it does not, return to Phase 0 and select the backup candidate — do not artificially diversify by picking a worse-fit target, but also do not accept overlap.

- [ ] 6.1 Scaffold `test/e2e/targets/activation_<project>_<func>/` for target #3.
- [ ] 6.2 **Distinct-pattern verification.** Confirm target #3's pattern differs from target #2's. If overlap, roll back to Phase 0.8 and pick the backup; do not proceed with overlap.
- [ ] 6.3 Register in `e2e_test.go` with parameterized fields only.
- [ ] 6.4 Workload + oracle per Phase 0.9.
- [ ] 6.5 Stage progression 4 → 10, one stage per `go test`. Artifacts under `.moab/runs/sprint-0052-target3/`.
- [ ] 6.6 Env-off and fail-mode checks.
- [ ] 6.7 Per-target flake notes in `.moab/runs/sprint-0052-target3/flake-notes.md`.
- [ ] 6.8 Corpus YAML row update.

## Phase 7: Doc reconciliation (Category D) and scope hygiene (Category C)

- [ ] 7.1 [flag #35 / D-21] Update `docs/decisions/0032-boundary-adapter-recovery.md`: after Phase 2.2, `MONOLIFT_BOUNDARY_ADAPTER`'s sole behavior is "gate the recovery branch." Restate that.
- [ ] 7.2 [flag #36 / D-22] After Phase 1.7, `TransportPolicy.MaxInlinePayloadBytes` is plan-configurable in rendering. Update ADR-0032 with an example and document the policy field.
- [ ] 7.3 [flag #37 / D-23] After Phase 1.1 and 1.8, the `isUploadMediaCandidate` mention is gone or structurally replaced. Update ADR-0032 to reference the structural admission rule.
- [ ] 7.4 [flag #38 / D-24] Tighten "discharges six named obligations" language in ADR-0032. Replace with a per-obligation table: `adapter_finite_input` (summary), `adapter_local_lifecycle` (SSA scan — list which forbidden ops are checked after Phase 2.5), `adapter_use_shape` (pattern-owned predicate, allowlist), `adapter_return_rehydration` (pattern-owned producer scan), `adapter_error_order` (accepted-with-divergence record), `adapter_call_site` (reverse-import scan after Phase 2.4, explicit scope). No prose claim exceeds what the implementation actually checks.
- [ ] 7.5 [flag #33 / C-19] Update SPRINT-0051 coverage report: after Phase 2.2, the "additional corpus candidate flips classification" stretch claim either reverts (`pocketbase/M-5`/`M-11` go back to SPRINT-0050 status) or is rewritten to document a real adapter-eligibility predicate. Pick after 2.2 lands.
- [ ] 7.6 [flag #39 / D-25] Footnote in `docs/research/activation-paths/analyses/listmonk-M-4.md`: reconcile "Reconstructible" `BoundaryDataClass` with Phase 0's `missing_reconstructor` baseline finding. State which is the source of truth and why both can co-exist.
- [ ] 7.7 [flag #41 / C-17] Decide on `docs/research/modular-monolift-virtues-v1.md`: either remove from this PR (preferred — land separately if there's reason) or annotate with an explicit "post-sprint commentary" header.
- [ ] 7.8 [flag — C-18] Fix PR #13 title typo `"functino" → "function"` on the squash commit.

## Phase 8: Category E nits (sweep alongside relevant Phase 1–4 commits where files are open)

These should land *opportunistically* alongside the Phase 1–4 commits that already touch the same files. Phase 8 is the catch-up sweep for nits that didn't get cleared earlier.

- [ ] 8.1 [flag #32 / E-41] Admin credentials duplicated as string literals in listmonk fixtures (`target.go:58-59`, `workload.go:88`). Add target-package constant.
- [ ] 8.2 [flag #31 / E-40] `directInvokePayload()` in listmonk processimage oracle panics at test-registration time if fixture missing. Move file read inside function body so panic occurs at test run, not init.
- [ ] 8.3 [flag #29 / E-38] Harmonize `image.Decode` (oracle) vs `imaging.Decode` (helper) in `test/e2e/targets/activation_listmonk_processimage/`. If divergence is intentional (oracle uses stdlib for independence), comment why.
- [ ] 8.4 [flag #30 / E-39] Fixture path duplicated as string literal across `oracle.go` and `workload.go` in listmonk processimage target. Add a target-local `const fixturePath = "testdata/..."`.
- [ ] 8.5 [flag #27 / E-32] Replace every `r` + `string(rune('0'+i))` variable generator in `pkg/codegen/server.go:208-221` and `pkg/codegen/adapter_client.go:191-223` with `fmt.Sprintf("r%d", i)` (or `strconv.Itoa(i)`). Add a DTO test with at least 11 non-error fields.
- [ ] 8.6 [flag #25 / E-33] Make `serverLocalAdapterCode` return `(string, error)` and propagate `normalizedHelperBody` errors instead of swallowing at `pkg/codegen/server.go:302-306`.
- [ ] 8.7 [flag #16 / E-28] (covered by Phase 3.3.)
- [ ] 8.8 [flag #17 / E-29] (covered by Phase 3.4.)
- [ ] 8.9 [flag #18 / E-30] (covered by Phase 3.5.)
- [ ] 8.10 [flag #6 / E-26] In `pkg/codegen/cut_admit.go`, `boundaryAdapterEnabled()` is read twice in `admitCutCandidates` (line 87 and line 105). Cache once at loop entry.
- [ ] 8.11 [flag #7 / E-27] In `pkg/codegen/cut_admit.go:adapterRecoveryAllowed:196-201`, Surface switch treats `""` (empty) as eligible. Decide: explicit refusal on empty, OR document intent. Recommend explicit refusal.
- [ ] 8.12 [flag #34 / E-37] Document the SPRINT-0051 stage-4/8 flake source in `.moab/runs/sprint-0051-closeout/flake-notes.md`. If the source is reproducible (likely Kind cold-start or port-forward race in `harness.StartPortForward`), fix it; if not, record the symptom.
- [ ] 8.13 [flag #14 / E-36] In `pkg/codegen/adapter_pass.go:planInputTransforms`/`planOutputTransforms` (lines 150-156, 197-204), collect all unsatisfied proofs and return them as the refusal trail, or document that "first failure" is intent.
- [ ] 8.14 [flag #12 / E-34] In `pkg/codegen/adapter_pass.go:421-447`, `liveProxyClassify(typ, isResult)` accepts and discards `isResult`. Either use it (for io.Writer-only-as-result vs io.Writer-as-input distinction) or drop the parameter.
- [ ] 8.15 [flag #13 / E-35] Document the gap between `isDirectlySerializableParam` (conservative — `pkg/codegen/adapter_pass.go:479`) and `AdmitPlan` (broader) with a file-comment paragraph stating the adapter pass defers to `AdmitPlan` for admission and uses a conservative gate only to skip unnecessary transforms.
- [ ] 8.16 [flag #11 / E-42] In `pkg/codegen/adapter_pass.go:567-615`, `remoteSignatureString` builds `transformByParamIndex` and never uses it. Delete the dead map.

## Phase 9: Verification and closeout

- [ ] 9.1 Run `go test ./pkg/activation/... ./pkg/codegen/... ./test/e2e/harness/...` on CloudLab. Save logs under `.moab/runs/sprint-0052-closeout/`.
- [ ] 9.2 Stage 10 verification for `listmonk/M-4` (regression), target #2, target #3 — three separate `go test` invocations, one stage each, on CloudLab.
- [ ] 9.3 Flag-off parity sweep (`MONOLIFT_BOUNDARY_ADAPTER=0`) — compare to SPRINT-0050 admission baseline. After Phase 2.2 fix, parity must be exact (no `pocketbase/M-5`/`M-11` side-effect flips).
- [ ] 9.4 Flag-on admission-only focused corpus sweep — record the set of corpus candidates that flip classification, separating those flipping due to the recovery branch (intent) from those flipping due to any residual side-effect (bug). Goal: only intentional flips appear.
- [ ] 9.5 Acceptance diff review: `git diff sprint-0051-closeout..HEAD -- pkg/codegen/adapter.go pkg/codegen/adapter_client.go pkg/codegen/adapter_normalize.go pkg/codegen/adapter_pass.go` must contain only target-agnostic refactors. The only growing file is `pkg/codegen/adapter_patterns.go` (registry additions). If a per-target conditional snuck in elsewhere, fail the sprint.
- [ ] 9.6 Acceptance grep — must return empty:
  ```
  rg -n 'activation-listmonk-processimage|processImage|UploadMedia|thumbnailSize|github.com/disintegration/imaging|8\*1024\*1024|adapter_parent_forbidden|<target2_name>|<target3_name>' pkg/codegen test/e2e/e2e_test.go | grep -v _test.go
  ```
  AND
  ```
  rg -n 'target\.Name ==' test/e2e/e2e_test.go
  ```
- [ ] 9.7 Confirm `TestAdapterPassNoTargetSpecificCode` (Phase 3.6) is green in CI.
- [ ] 9.8 Confirm every flag in `docs/sprints/briefs/sprint-51-review.md` is mapped to a closed task or explicitly deferred with maintainer approval. No active Category A or B-9/B-10/B-12 item may remain open.
- [ ] 9.9 Confirm no generated extracted deployment YAML under `.moab/runs/sprint-0052-*` contains `MONOLIFT_LIFT_*` environment variables (preserved SPRINT-0050 invariant).
- [ ] 9.10 Write `docs/research/runs/SPRINT-0052-coverage-report.md`: survey table, selected targets, rejected candidates, commands, cost profiles, per-target stage results, adapter patterns added, review-flag closure, residual backlog.
- [ ] 9.11 Update sprint ledger to `status: done`, record executor.

## Remote Test Discipline

Same rules as SPRINT-0050/0051. Highlights:

- [ ] R.1: Before heavy work, run `cl ls` / `cl status <experiment>` locally. If no experiment exists, ask the user to start the `monolift-buildserver` profile.
- [ ] R.2: All `go test ./pkg/...`, e2e, Kind/Docker image builds, `cmd/activation-path` against real corpus targets, and corpus sweeps run on CloudLab.
- [ ] R.3: Local work is limited to editing, source reading, docs, and small codegen/unit/golden tests that do not touch `evaluation/*`.
- [ ] R.4: No `make e2e`, no multi-target `-run` regex, no `scripts/run_activation_corpus_sweep.sh --phases all`.
- [ ] R.5: Use focused target/importer package scope for research; do not use timeout failures from broad package loading as viability evidence (re-run with widened caps per Phase 0.3).
- [ ] R.6: Stage escalation one target, one stage, one `go test` process at a time. Never jump.
- [ ] R.7: If an e2e run is aborted before cleanup, delete `kind` cluster `monolift-e2e` or orphaned `mlv2-*` namespaces before the next run.
- [ ] R.8: Stage all artifacts under `.moab/runs/sprint-0052-*` on the build node.

## Sequencing and dependencies

```
Phase 0 (corpus survey)
       │
       ▼
Phase 1 (Cat A unwind) ──► 1.9 M-4 regression gate (hard)
       │
       ▼
Phase 2 (Cat B rigor) ──► 2.2 lands before 9.3 parity sweep (hard)
       │                  2.4 lands before Phase 4 (hard — patterns need real call-site)
       ▼
Phase 3 (pattern framework cleanup)
       │
       ▼
Phase 4 (pattern implementations)
       │
       ▼
Phase 5 (Target #2)  ◄── parallel with Phase 6 after Phase 4
       │
       ▼
Phase 6 (Target #3) ──► 6.2 distinct-pattern guard; rollback to Phase 0.8 backup if overlap
       │
       ▼
Phase 7 (Cat D docs + Cat C hygiene)  ◄── opportunistic alongside Phases 1–4
       │
       ▼
Phase 8 (Cat E nits)  ◄── opportunistic alongside Phases 1–4
       │
       ▼
Phase 9 (verification + close)
```

**Hard ordering constraints:**

- Phase 1 must complete before Phase 4 — adding a new pattern to a code path with M-4 literals compounds overfitting.
- Phase 1.9 (M-4 regression) is the gate to Phase 2 — if M-4 broke during Cat A unwind, no Cat B work proceeds.
- Phase 2.4 (call-site scan) must complete before Phase 5 — target #2's call-site shape will exercise the scan; relying on the unexported-helper fallback for two more targets is statistically unlikely to hold.
- Phase 2.2 (flag behavior) must complete before Phase 9.3 (parity sweep) — otherwise parity sweep falsely passes by replicating the bug.
- Phase 3.6 (`TestAdapterPassNoTargetSpecificCode`) must land before Phase 4 begins — the CI guard catches regressions before they accumulate.

**Parallelizable:**

- Phase 7 docs land alongside the Phase 1/2 commits that already touch the relevant code paths.
- Phase 8 nits land alongside the Phase 1/2/3 commits that already touch the file.
- Phase 5 and Phase 6 are independent once Phase 4's patterns land.

## Risks and Mitigations

| Risk | Mitigation |
|---|---|
| Phase 0 survey finds no second adapter pattern is feasible without new admission scaffolding. | Phase 0.8 selection rules explicitly prefer "no new admission scaffolding." If the survey truly finds zero candidates, the brief's premise is wrong and this becomes a research sprint — flag immediately to the maintainer; do not paper over. |
| Cat A unwind breaks `listmonk/M-4` stage 10 (M-4 goldens depend on `applyProcessImageResultNames`, `thumbnailSize`, `imaging` literals). | Phase 1.9 is a hard gate. Goldens regenerated *per Cat A task*, not at the end. M-4 functionality must not change; only the data path through generic code does. |
| AST-aware body rewrite (1.2) is more fragile than `strings.Replace`. | Phase 4.4 includes negative fixtures per pattern. Phase 4.5 runs real cut function SSA through the rewriter. Stage-4 golden test catches incorrect output before deploy. |
| Reverse-import call-site scan (2.4) is expensive on large corpora. | Scope to activation-path package set (SPRINT-0051 §0.2 model). Cache per candidate. Note: do NOT add a candidatePlanTimeout-style refusal on scan timeout — timeouts are cost evidence, not admission facts, per AGENTS research-mode instruction. |
| Phase 2.2's flag-behavior cleanup reverts SPRINT-0051 "stretch" classification flips for `pocketbase/M-5`/`M-11`. | Intended. Phase 7.5 updates the coverage report explicitly. If the flips are desirable on their own merits, document as a separate decision and either add an ADR or achieve via a real adapter-eligibility predicate. |
| Cache plumbing in 2.7 introduces stale-plan bugs. | Reuse existing `candidateAdmitKey`. Invariant assertion on lookup: cached recovery `RemoteSignature` matches candidate `NodeKey`. |
| Targets #2 and #3 land on the same pattern, weakening the falsification claim. | Phase 6.2 distinct-pattern guard with rollback. The brief explicitly contrasts new patterns against `multipart_file_read_all`/`bytes_reader_return` — overlap means the framework reused a pattern, but the sprint thesis requires *new* patterns to ship. |
| Closure-capture proof in 2.6 produces false refusals (legitimate closures that don't escape). | Implement minimally: refuse on `*ssa.MakeClosure` capturing the value. Evaluate corpus fallout. If real-world code uses a non-escaping closure, refine OR refuse and let admission demote (both acceptable per ADR-0032). |
| `target.Name` string-match removal in 1.6 ripples to every existing target. | New `TargetCase` fields are optional with zero defaults. Only `activation-listmonk-processimage` and the new targets populate them. Other targets compile unchanged. |
| Acceptance grep (9.6) produces false positives from comments or ADR-adjacent paths. | Scope to `pkg/codegen` and `test/e2e/e2e_test.go`; exclude `_test.go`; word-boundary regex where possible. False positives in code comments are honest signal — fix the comments. |
| Phase 0 widened-cap reruns add significant CloudLab time. | Phase 0.3 separates cost from semantic blocker — this is the SPRINT-0051 Phase 0 lesson encoded. Accept the time cost; misclassifying timeout-shaped refusals as semantic is worse. |
| Brief's `pocketbase/M-7` and `gitea/M-13` are both mailer traces and labels may be optimistic. | Phase 0.7 explicit skepticism check + Phase 0.8 backup candidates required. The survey is real, not pro-forma. |

## Acceptance Criteria

**Minimum (the framework generalization claim):**

- [ ] `docs/research/runs/SPRINT-0052-target-survey.md` exists with two specific targets, distinct adapter patterns, backup candidates documented, oracle policies declared, and rationale grounded in Phase 0 survey data including widened-cap reruns.
- [ ] All Category A flags (#2, #8, #20, #21, #22, #23, #24, #28) resolved with structural fixes. Acceptance grep `rg -n 'processImage|UploadMedia|listmonk|thumbnailSize|disintegration/imaging|8\*1024\*1024|adapter_parent_forbidden' pkg/codegen | grep -v _test.go` returns empty.
- [ ] `listmonk/M-4` continues to pass stage 10 after Cat A unwind, with refreshed goldens but no behavior change.
- [ ] Target #2 and target #3 each reach stage 10 on CloudLab with the 4→5→6→7→8→9→10 ladder.
- [ ] Targets #2 and #3 exercise adapter patterns *distinct* from each other and from SPRINT-0051's two patterns.
- [ ] `pkg/codegen/` diff between SPRINT-0051 closeout and SPRINT-0052 closeout contains only target-agnostic refactors. The only file under `pkg/codegen/adapter*.go` that grows is `pkg/codegen/adapter_patterns.go` (registry additions).
- [ ] `MONOLIFT_BOUNDARY_ADAPTER=0` flag-off parity sweep produces zero delta vs SPRINT-0050 admission baseline (after Phase 2.2 fix removes the side-effect).
- [ ] `TestAdapterPassNoTargetSpecificCode` is green in CI.

**Framework rigor (Category B):**

- [ ] DTO packing runs only for candidates that would otherwise refuse with `unsupported_result_shape` (B-9 verified by `multireturn_test.go` update).
- [ ] `MONOLIFT_BOUNDARY_ADAPTER` flag has one and only one behavior; unit test asserts `callable_boundary_values` is not suppressed for non-adapter-eligible candidates with the flag on.
- [ ] `missing_reconstructor` adapter-eligibility is parameter-typed; unit test asserts `*sql.DB` reconstructor refusals do not enter the adapter branch.
- [ ] `adapter_call_site` runs an actual reverse-import scan; synthetic function-value-use fixture refuses.
- [ ] `adapter_local_lifecycle` checks interface boxing, store-to-global, and interface-dispatch Close (fixtures per check).
- [ ] `multipart_file_read_all`'s use-shape proof traverses `*ssa.FreeVar`; closure-capture fixture refuses.
- [ ] `tryAdapterRecovery` is invoked at most once per candidate on the happy path.

**Doc reconciliation (Category D):**

- [ ] ADR-0032 reflects post-Phase 1/2 state of the code; no claims overstated.
- [ ] `analyses/listmonk-M-4.md` footnote reconciles BoundaryDataClass with refusal-baseline finding.

**Scope hygiene (Category C):**

- [ ] `docs/research/modular-monolith-virtues-v1.md` decision is made and reflected in PR #13 history.
- [ ] PR #13 squash title typo fixed.
- [ ] Stretch-criterion claim about `pocketbase/M-5`/`M-11` is corrected or substantiated in the coverage report.

**Nit sweep (Category E):**

- [ ] All Category E flags (#6, #7, #11, #12, #13, #14, #16, #17, #18, #19, #25, #27, #29, #30, #31, #32, #34) resolved or explicitly noted as wontfix with rationale.

## References

- `docs/sprints/briefs/sprint-51-review.md` — primary input, 40 flags
- `docs/sprints/SPRINT-0051.md` — predecessor sprint
- `docs/decisions/0032-boundary-adapter-recovery.md` — framework ADR (amended this sprint)
- `docs/research/activation-paths/boundary-adapter-strategy.md` — spec
- `docs/research/activation-paths/analyses/listmonk-M-4.md`, `pocketbase-M-7.md`, `gitea-M-13.md`, `pocketbase-M-9.md`, `mattermost-M-13.md`, `miniflux-M-9.md` — candidate analyses
- `pkg/codegen/adapter_pass.go`, `adapter_patterns.go`, `adapter.go`, `adapter_client.go`, `adapter_normalize.go`, `cut_admit.go`, `server.go`, `admission.go`
- `test/e2e/e2e_test.go`, `test/e2e/harness/target.go`, `test/e2e/activation_corpus_traces.yaml`
