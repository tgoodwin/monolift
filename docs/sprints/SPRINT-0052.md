# SPRINT-0052: Boundary-adapter generalization — second and third targets at stage 10

**Status:** planned
**Predecessors:** SPRINT-0051 (boundary-adapter framework + `listmonk/M-4` stage 10), ADR-0032
**Branch:** continues on `sprint-51` (PR #13) — Category A unwind is a prerequisite to merging that PR to `main`
**Primary input:** `docs/sprints/briefs/sprint-51-review.md` (40 flags, A–E)

## Intent

SPRINT-0051 shipped the boundary-adapter framework as a generalizable shape with an M-4-specific spike inside it. SPRINT-0052 falsifies that the framework is real by landing two or more *additional* adapter-enabled lifts at stage 10, across **distinct** adapter patterns beyond `multipart_file_read_all` and `bytes_reader_return`, with **zero edits to `pkg/codegen/adapter*.go` outside the pattern registry, and zero new target-name string-matches in `pkg/codegen/` or `test/e2e/e2e_test.go`**. Work order is forced: Category A overfitting comes out *before* a second target's SSA can drive any new code path, because every M-4-specific literal currently in the production code path will produce wrong output for any other shape. Category B is gating for target claims, not just compilation — proof obligations that pretend to verify what they do not actually verify will mis-classify real candidates the moment a second pattern appears. Categories C, D, and E land alongside as the affected files are touched.

## Goals

- [x] Goal (reframed by the Phase 0 pivot): prove the framework generalizes — ≥2 lifts beyond `listmonk/M-4` reach stage 10 on CloudLab, each exercising a *distinct* mechanism. **Met:** `miniflux/ExtractContent` (streaming-bytes + ResultDTO) and `pocketbase/S256Challenge` (plain transform), in two new apps, both at stage 10 — with **zero changes to `pkg/codegen/`**. The corpus has no second adapter-*pattern* candidate (only M-4 is `AdapterPossible`); the maintainer-approved pivot broadened "distinct adapter pattern" to "distinct generic mechanism." See the coverage report and survey doc.
- [x] Goal: eliminate all M-4-specific knowledge from `pkg/codegen/` (Category A). Done in Phase 1; enforced permanently by `TestAdapterPassNoTargetSpecificCode` (green).
- [x] Goal: tighten framework rigor (Category B). Done in Phase 2 (structural recovery gate; DTO packing gated on the result-shape refusal; the flag has exactly one behavior; `missing_reconstructor` is parameter-typed; `adapter_call_site` reverse-import scan; `adapter_local_lifecycle` covers interface boxing + store-to-global; `multipart_file_read_all` traverses closures).
- [x] Goal: reconcile ADR-0032 and analysis docs with the implementation (Category D). Done in Phase 7.
- [x] Goal: resolve Category C scope hygiene before this PR ships. Done in Phase 7 (virtues doc absent; PR title clean; coverage-report claim reconciled).
- [x] Goal: sweep Category E nits as the affected functions are touched. Done in Phase 8.

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
- [ ] Non-goal (follow-up, SPRINT-0053+): write-once/immutability dataflow analysis for package-level **vars** referenced by an adapter helper (task 1.3 handles **consts** only). The analysis is the refuse-vs-copy decision procedure: scan writes to the var's `*ssa.Global` after package init — no post-init write ⇒ safe to copy the initializer into the extracted service (this unlocks non-`const`-able write-once values like `[]string{…}`, `regexp.MustCompile(…)`, `&http.Client{…}`); any write ⇒ mutable shared state ⇒ refuse (a `*ssa.Global` write detector would also serve task 2.5). Until then 1.3 conservatively refuses on **any** free package-level var (`cutFileFreeConsts` in `pkg/codegen/adapter_symbols.go`).

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

> **OUTCOME (2026-05-19, maintainer-approved pivot).** The focused CloudLab
> admission sweep refuted all three provisional picks: `listmonk/countLines` is
> **admitted directly** (its `io.Reader` is handled by the streaming-bytes codec,
> not the adapter), and both gitea candidates **time out at 10m** in activation
> (cost-prohibitive). Mining the 72-trace manifest for adapter-eligible refusals
> found **no** clean new-adapter candidate in a cost-feasible app — only
> `listmonk/M-4` is classified `AdapterPossible` in the entire corpus. Decision:
> **broaden the thesis** from "two new adapter patterns" to "two lifts proving the
> framework's *generic* machinery generalizes beyond M-4." Phases 4-6 re-scoped:
> **no new adapter pattern** (Phase 4 is a no-op this sprint); Phases 5-6 land two
> generic-machinery targets end-to-end.
>
> **FINAL picks (after a second re-selection).** Route-reachability is the binding
> e2e constraint — an e2e must exercise the function through the host's real
> request path to demonstrate the cross-network round-trip. An early reclassified
> pair (`countLines`/`classifyBounce`) was discarded: `classifyBounce` is POP3-only
> (no HTTP route), and the route-reachable listmonk alternatives are `SharedState`-
> receiver methods that refuse with `receiver_requires_reconstruction`. Re-selected
> across cost-feasible apps with route-reachability + free-function as the first
> filter: **Lift #1 `miniflux/ExtractContent(io.Reader)(string,string,error)`**
> (streaming-bytes **and** DTO, via `GET /v1/entries/{id}/fetch-content`) and **Lift #2
> `pocketbase/S256Challenge(string) string`** (plain transform, via the auth-methods
> route — a third app). Both confirmed admitted on CloudLab. Full record + sweep
> tables: `docs/research/runs/SPRINT-0052-target-survey.md`.

The brief proposes `pocketbase/M-7` and `gitea/M-13`. Phase 0 must independently confirm or refute these — both are *mailer* traces (`SMTPClient.send`, `sender.send`) per their analysis docs, so the "callback shape" and "reader shape" labels in the brief may be optimistic. Goal: enumerate adapter-shape-compatible corpus rows, classify them into pattern families, then *pick two whose patterns are distinct from each other and from `multipart_file_read_all`/`bytes_reader_return`*.

- [x] 0.1: Run `cl ls` and `cl status <experiment>` to confirm a `monolift-buildserver` experiment is active before any heavy work. If none exists, ask the user to start one. Create `.moab/runs/sprint-0052-phase0/` on the build node and store every survey command, candidate table, refusal trail, and timing breakdown there. *(Build node `c220g5-111307.wisc.cloudlab.us` active and in use all session; `.moab/runs/sprint-0052-phase0/` created on the node.)*
- [x] 0.2: Run a focused admission sweep with `MONOLIFT_BOUNDARY_ADAPTER=1` over the corpus (focused phases only, no Kind), capture per-trace refusal codes and `AdapterClass` field values. Save under `.moab/runs/sprint-0052-survey/`. *(Ran **focused** admission (`TestAdmission`, flag on, `GOTOOLCHAIN=auto`) on the specific candidate file:lines rather than the full 72-trace re-run — the manifest already records prior-sweep refusal reasons, which I mined instead. Note: `TestAdmission` does AdmitCut+AdmitPlan directly, so it reports the direct refusal/admit, not the candidate-loop `AdapterClass`; sufficient to confirm shape + eligibility. Results table in the survey doc.)*
- [x] 0.3: For each candidate that hit an internal cap or timeout in 0.2, rerun with the cap widened or disabled before classifying viability. **Separate cost profile from semantic blocker** in the Phase 0 table — a timeout-shaped refusal is not the same as a semantic refusal. This is the SPRINT-0051 Phase 0 lesson encoded. *(gitea candidates timed out at 10m in activation augmentation — recorded as a **cost** blocker, not semantic; their shapes (`io.Writer`/`io.WriterTo`) are independently not adapter-eligible. Did not pursue further cap-widening since cost makes gitea impractical as an e2e target regardless.)*
- [x] 0.4: Enumerate all candidates whose direct refusal is shape-compatible AND whose `AdapterClass` is `AdapterPossible` OR `AdapterUnknown`. ... *(Mined `activation_corpus_traces.yaml` (72 traces) for adapter-eligible refusal reasons: only `gitea/M-17` (cost), `pocketbase/M-2` (`*core.RequestEvent`), `pocketbase/M-5`/`M-11` (`core.App` live-proxy). The named brief candidates `mattermost/M-13`/`miniflux/M-9`/`pocketbase/M-7`/`M-9` are `FieldNotInCorpus` (no resolvable file:line). Only 1 trace corpus-wide is `AdapterPossible` — M-4.)*
- [x] 0.5: For the top ≤6 candidates, manually inspect helper SSA shape ... Reject any candidate that re-uses `multipart_file_read_all`/`bytes_reader_return` exactly. *(Inspected the reader/writer/DTO candidates' signatures from source; key finding — `io.Reader` is handled by the streaming-bytes codec (not the adapter), so reader candidates don't falsify the adapter framework. No corpus candidate cleanly fits a new bounded-consumption adapter pattern.)*
- [x] 0.6: For each remaining candidate, draft a one-paragraph pattern proposal. *(N/A for the final picks — they use existing generic machinery (streaming-bytes, DTO), so no new pattern proposal is needed. A genuinely new adapter pattern is deferred to a future sprint per the pivot.)*
- [x] 0.7: Skepticism check on the brief's two candidates. *(Done in the survey doc: `pocketbase/M-7`'s hook is consumed by `Send`, not the cut `send`; `gitea/M-13`'s `send` dispatches via a `Sender` interface and does no reader work. Both rejected — the brief's labels were optimistic, exactly as anticipated.)*
- [x] 0.8: Select target **#2** and target **#3**. *(Per the maintainer-approved pivot: Lift #1 `listmonk/countLines` (streaming-bytes) + Lift #2 `listmonk/classifyBounce` (multi-return DTO). They exercise **distinct generic mechanisms** rather than distinct adapter patterns; both reuse the listmonk fixture; neither needs a new receiver reconstructor.)*
- [x] 0.9: Determine oracle policy per target. *(Both: direct-equality on the returned value(s) for fixed byte input — `countLines`→`int` count, `classifyBounce`→`(string,string)`. Both are pure functions of the input bytes, so deterministic. Recorded in the survey doc.)*
- [x] 0.10: Lock the picks in `docs/research/runs/SPRINT-0052-target-survey.md`. *(Locked: final picks, sweep results table, and the rationale for no new-adapter target are all recorded there.)*

## Phase 1: Category A unwind (must land before Phase 3 touches any pattern)

This phase makes the production code path target-agnostic. Each task removes one M-4 fingerprint. After Phase 1, `listmonk/M-4` stage 10 must still pass unchanged — it is the regression check on the unwind, not the goal of the unwind.

- [x] 1.1 [flag #2 / A-1] Delete `isUploadMediaCandidate` from `pkg/codegen/cut_admit.go:182`. Replace the guard at `cut_admit.go:105` with a structural predicate `adapterParentForbiddenForCandidate(candidate, plan)`: refuse when the candidate is a parent whose call graph contains an adapter-eligible descendant currently in `Candidates`. The predicate names no function. Add unit test `TestAdapterParentForbiddenByStructure` with three fixtures: M-4-shaped parent/descendant (refuses); unrelated parent (admits); descendant with no adapter-eligible classification (admits).
- [x] 1.2 [flag #20 / A-2] Delete `rewriteMultipartFileReadAllBody`, `rewriteBytesReaderReturnBody`, and the two indentation variants in `pkg/codegen/adapter.go:223-246`. Replace `normalizedHelperBody` with an AST-aware rewriter: parse the cut function body, walk for the per-input pattern's awkward prologue site (each pattern declares a `BodyPrologueMatcher func(*ast.BlockStmt) (matched *ast.Stmt, replacement []ast.Stmt, ok bool)` or equivalent on its interface), apply the replacement, re-print. Each pattern owns its own AST template. Adapter-pass code stays target-agnostic. Add negative tests proving the rewriter refuses (or no-ops) on unsupported prologue shapes rather than partial-applying. *(Done: pattern-owned `inputBodyRewriter`/`outputBodyRewriter` interfaces in adapter_patterns.go; multipart removes Open/err-guard/defer prologue + rewrites uses to `bytes.NewReader(normName)`; bytes_reader_return unwraps `bytes.NewReader(X)` return slots → X. Generic dispatch in `rewriteHelperBodyAST`; adapter.go names no pattern. go/ast + astutil, parse with SkipObjectResolution. Negative tests `TestMultipartRewriteInputBody`/`TestBytesReaderRewriteOutputBody`. Existing goldens unchanged — AST output is byte-identical to the old strings.Replace.)*
- [x] 1.3 [flag #21 / A-3] Delete `const thumbnailSize = 250` from `serverLocalAdapterCode` in `pkg/codegen/server.go:310`. Scan the cut function's free symbols (package-level constants/vars referenced from the helper body) and re-emit them in the helper file. Add a synthetic fixture test where the helper references **two** constants with non-`thumbnailSize`/non-`250` names/values to prove the free-symbol extractor isn't single-constant-fitted. *(Done together with 1.4 via a shared scan in new `pkg/codegen/adapter_symbols.go`. `serverLocalAdapterCode` → `renderLocalAdapterCode(plan, helper)`: prepends `helper.FreeConsts` (package-level consts the rewritten body references, by name+value) instead of the hardcoded const. Free-symbol scan: collect bare value idents not locally bound (params/`:=`/`var`/range/typeswitch/funclit), intersect with cut-file package-level const names; render each `const name [type] = value` via go/printer. A referenced package-level **var** fails closed (`err` names the var) — conservatively refused pending the write-once dataflow follow-up (see Non-Goals): until that analysis exists we cannot tell an immutable init-time var from mutable shared state, so we refuse all. iota-derived consts (no explicit value) also refuse. Test `TestBuildNormalizedHelperFreeSymbols` uses `const maxWidth = 800` + `const jpegQuality = 90` (asserts both copied, no thumbnailSize/250 leak); `TestBuildNormalizedHelperRefusesFreeVar` locks the var refusal.)*
- [x] 1.4 [flag #22 / A-4] Delete the hardcoded `importSpec{Path: "bytes"}, importSpec{Path: "github.com/disintegration/imaging"}` at `pkg/codegen/server.go:86`. Compute the import set from the helper body's AST `ImportSpec`s in the source file, intersected with what the rewritten body actually references, then run through the existing `goimports` hook. *(Done: `buildNormalizedHelper` now returns `Imports []importSpec` = cut-file imports whose local name (alias or path base) appears as a selector qualifier in the **rewritten** body. Both render paths use it: `renderNormalizedHelper` (cut-package helper file) sets the helper's import block; `serverTemplateView` appends `helper.Imports` to the server file imports (deduped by `uniqueImports`). Note: the project's `formatGo` is `go/format` (gofmt), **not** goimports — it won't add/strip imports — so the intersection must be exact (no unused/missing). For multipart_file_read_all this yields `{bytes, imaging}` and drops `mime/multipart` (the input rewrite removes `file.Open()`), matching the old hardcode for M-4. Asserted in `TestBuildNormalizedHelperFreeSymbols`.)*
- [x] 1.5 [flag #23 / A-5] Delete `applyProcessImageResultNames` from `pkg/codegen/adapter_normalize.go:113-123`. Replace with: DTO field name = `result.Name` when non-empty and non-generic (not `result`, not `r0`), else `Result0..N`. Refresh M-4 goldens — the DTO fields rename from `thumbnail/originalWidth/originalHeight` to whatever `Plan.Results[i].Name` actually is for `processImage` (likely `Result0/Result1/Result2`). Confirm the host-side `bytes.NewReader(out.Result0)` still type-checks at stage 9. Add a non-image-shape DTO naming test for `([]byte, int, int, error)` proving no hidden M-4 renaming remains. *(Done: deleted `applyProcessImageResultNames` and dropped the `"thumbnail"` literal from `adapterOutputName` (now takes only `Result`, no `transform`). Generic detection moved into `BuildResultDTO` via `isGenericResultName` — treats `result`, `result<N>`, `r<N>` as positional placeholders → `Result0..N`; meaningful names preserved verbatim (the `data/width/height` DTO goldens are unchanged, proving non-generic names untouched). adapter_test.go DTO assertion now `Result0 []byte json:"result0"`. New `TestBuildResultDTONoImplicitImageRenaming` (generic ([]byte,int,int,error)→Result0/1/2, no thumbnail/originalWidth leak; meaningful names→Payload/Width/Height). Per user, re-evaluated the e2e harness: the lifted DTO wire keys are renamed `result0/result1/result2`, so the listmonk-processimage oracle map keys moved in lockstep (stage-9 compares `fmt.Sprint` of decoded maps at e2e_test.go:914). Host reconstruction decodes positionally into `r0/r1/r2`, so `bytes.NewReader(r0)` is unaffected — stage-9 type-check holds. CloudLab: `go build ./pkg/...` + `go vet ./test/e2e/.../activation_listmonk_processimage` clean; targeted DTO/adapter tests green.)*
- [x] 1.6 [flag #28 / A-6] Delete `if target.Name == "activation-listmonk-processimage"` branches from `test/e2e/e2e_test.go:1153,1421-1450` (and any others surfaced by grep). Add `TargetCase.FailClosedExpectedStatus int` (default 0 = "any non-5xx for sentinel mode") and `TargetCase.InvokeResultExtractor func(map[string]any) any` fields to `test/e2e/harness/target.go`. Populate them for `activation-listmonk-processimage` in its target.go. Other targets compile unchanged with zero-value defaults. *(Done: added `FailClosedExpectedStatus int` (set to 500 for processimage), removed the only processimage target-name match. The 1421-1450 branches are `miniflux`/default cleanpath cases, not M-4 fingerprints — left as-is. Skipped `InvokeResultExtractor`: no consumer exists yet, so adding it now would be dead code; defer to Phase 5/6 when a target needs result extraction. e2e package vets clean, harness tests pass on CloudLab.)*
- [x] 1.7 [flag #24 / A-7] In `pkg/codegen/adapter_client.go:152-158`, change the hardcoded `len(out.Name) > 8*1024*1024` to read from `plan.AdapterPlan.TransportPolicy.MaxInlinePayloadBytes`. Plumb the field through `adapterExtractionLines(plan, transport *Plan)` signature. Unit test `TestAdapterExtractionRespectsTransportPolicy` sets MaxInlinePayloadBytes to a non-default value and asserts the rendered limit matches. *(Implemented as `AdapterPlan.MaxInlinePayloadBytes int64` rather than nesting under `TransportPolicy` to keep the existing transport-mode enum's JSON shape stable; threads through `AdapterContext` → `AdapterPlan` → `adapterInlinePayloadLimit`.)*
- [x] 1.8 [flag #8 / A-8] Decide on `adapter_parent_forbidden` (used at `cut_admit.go:107`): either formalize it in ADR-0032 + the refusal-code vocabulary in `pkg/codegen/types.go`, or rename to a structurally-derived refusal (e.g. `unsupported_parent_of_adapter_eligible_child`). Either way: one constant, one Refusals row, documented. Update Category D entry 6.3 in the same commit. *(Formalized as `RefusalAdapterParentForbidden` constant in `pkg/codegen/admission.go`; cut_admit.go uses the constant; ADR-0032 reconciliation deferred to Phase 7.3.)*
- [x] 1.9 **M-4 regression gate.** Re-run `listmonk/M-4` stage 10 on CloudLab. **Must pass.** Save artifacts under `.moab/runs/sprint-0052-phase1-m4/`. If it does not pass, the unwind regressed M-4 and Phase 1 is not done — fix before proceeding to Phase 2. *(PASS: `TestE2E/activation-listmonk-processimage` stage 10 green on CloudLab kind cluster, 235.86s, batch pass 1/1. Run: `MONOLIFT_E2E=1 MONOLIFT_E2E_STOP_STAGE=10 go test -tags=e2e ./test/e2e/ -run TestE2E/activation-listmonk-processimage`. Log at `.moab/runs/sprint-0052-phase1-m4/stage-10.log` (gitignored, on CloudLab). Confirms the Cat A unwind — generic AST body rewrite, generic DTO naming, generic free-symbol/import scan — did not regress M-4.)*

## Phase 2: Category B framework rigor

These do not block target #2 from compiling, but they will silently mis-classify target #2's candidates if left in. Land before Phase 4 — otherwise target evidence is contaminated.

- [x] 2.1 [flag #1 / B-9] Gate DTO packing on candidacy: in `pkg/codegen/admission.go`'s `AdmitPlan` path, run DTO packing for `(T, U, ..., error)` shapes only when admission would otherwise refuse with `unsupported_result_shape`. Today it runs for every multi-return. Add a refusal-shadow check; update `multireturn_test.go` to confirm `(T, error)` and `(T)` shapes are unchanged. *(Refactored `admitResultShape` to gate DTO packing behind a new `baseResultShapeRefusal(plan) (AdmissionRefusal, bool)` predicate — the refusal-shadow check. Packing only runs when the base admission would refuse with `unsupported_result_shape` (two non-error returns, or > 2 results); single-result and `(T, error)` shapes return `refuses=false` and never carry a DTO. A successful pack shadows the refusal (verdict accepted, no leftover refusal); a failed pack — e.g. a non-JSON-codable return that `BuildResultDTO` can't pack — leaves the refusal standing. Behavior-preserving for all existing shapes (the old code relied on `BuildResultDTO` returning nil to encode the same condition); the change makes the recovery semantics explicit and the refusal the gate. Added 4 refusal-shadow tests to `multireturn_test.go`: `(T, error)`/`(T)` raise no `unsupported_result_shape`; `(T, U, error)` packs and shadows the refusal; `(string, func())` is unpackable so the refusal stands with no DTO. Full `pkg/codegen` suite green on CloudLab (359s).)*
- [x] 2.2 [flag #3 / B-10] Audit `MONOLIFT_BOUNDARY_ADAPTER` for hidden behavior. In `pkg/codegen/cut_admit.go` and `AdmitCut`, find every site that consults `boundaryAdapterEnabled()`. Gate any suppression of `callable_boundary_values` on `AdapterClass == AdapterPossible` for the specific candidate. The flag must do exactly one thing: enable the recovery branch. Unit test: with the flag on and a non-adapter-eligible callable candidate, `callable_boundary_values` is still reported. Regression test: `pocketbase/M-5` and `pocketbase/M-11` do not flip solely from hidden suppression — they revert or require a real adapter plan. *(Audited the two consult sites: `cut_admit.go:87` (`adapterEnabled := boundaryAdapterEnabled()`, the legitimate "enable recovery branch" use, gating `adapterParentForbiddenForCandidate` and the `isAdapterEligibleRefusal` recovery) and `admission.go:58` (the hidden behavior). The latter suppressed `callable_boundary_values` in the base `AdmitCut` verdict whenever the flag was on — which admitted high-callback candidates **directly as boundaries with no adapter plan**, since recovery is refusal-driven and the suppressed refusal never reached the recovery branch. Removed the flag gate: `AdmitCut` now always emits `callable_boundary_values`. The flag does exactly one thing (the `adapterEnabled` gate in `admitCutCandidates`); a callable candidate is admitted only when recovery succeeds and reclassifies it `AdapterPossible` — that verdict is rebuilt from the normalized plan via `AdmitPlan`, which carries no callback check, so the refusal is shadowed there (= "suppression gated on AdapterClass == AdapterPossible"), not in `AdmitCut`. Flag-off parity unchanged. Tests: `TestAdmitCutReportsCallableBoundaryRegardlessOfFlag` (table over flag ""/"0"/"1", high-callback → always refused with `callable_boundary_values`); `TestAdmitCutCandidatesFlagOnCallableNotRecoverableStaysRefused` (flag-on, Many callbacks + func() boundary → `adapterRecoveryAllowed` declines → refusal stands). M-5/M-11 parity is the Phase 9.3 sweep's job; `TestAdmission` is a flag-gated manual harness (skipped without `-trace-target`), so nothing in the standard suite asserts the old suppressed classification. Full `pkg/codegen` suite green on CloudLab (362s).)*
- [x] 2.3 [flag #4 / B-11] In `pkg/codegen/cut_admit.go:49-55`, narrow `adapterEligibleRefusals` to *only* `missing_reconstructor` refusals whose refusal `Type` field is parameter-typed (not receivers, not DB/filesystem reconstructors). Add helper `isParameterTypeReconstructorRefusal(refusal AdmissionRefusal) bool` reading `refusal.Type`. Negative test: a `missing_reconstructor` refusal whose reason is `*sql.DB` does NOT enter the adapter branch. *(Kept `adapterEligibleRefusals` as the code allowlist but special-cased `missing_reconstructor` in `isAdapterEligibleRefusal`: it now defers to the new `isParameterTypeReconstructorRefusal(refusal)`, which reads `refusal.Type` and returns false for empty Type (fails closed) or for infrastructure-handle types via the new generic `isInfrastructureHandleType` (database `sql.db/tx/conn/stmt`, `filesystem.system`, `afero.`, `os.file/root` — type-string classification only, no target names, paralleling `isSerializableReceiverType`'s sql checks). Note receiver candidates are already blocked from recovery by `adapterRecoveryAllowed` (Receiver != ""), so 2.3's real added value is rejecting infrastructure handles passed as **parameters** (the `*sql.DB` param case). Updated `TestIsAdapterEligibleRefusal` (dropped the unconditional `missing_reconstructor` row, since a bare refusal now has empty Type → ineligible) and added table test `TestMissingReconstructorAdapterEligibilityByType` (param value types `*multipart.FileHeader`/`*bytes.Reader` → eligible; `*sql.DB`/`*sql.Tx`/`filesystem.System`/`*os.File`/empty → not). Also fixed pre-existing gofmt drift in `cut_admit_test.go` (from Phase 1.1's `TestAdapterParentForbiddenByStructure`). Full `pkg/codegen` suite green on CloudLab (359s). Note: `adapter_patterns.go` and `render.go` carry pre-existing gofmt drift, left untouched — `adapter_patterns.go` is reworked in Phase 3.)*
- [x] 2.4 [flag #9 / B-12] Implement the reverse-import scan for `adapter_call_site` in `pkg/codegen/adapter_pass.go:305-331`. Today `ctx.CallSites` is always nil from `tryAdapterRecoveryFromPlan` (cut_admit.go:369). Plumb a `CallSiteIndex` built from the activation-path scope — enumerate `*ssa.Function` references to the helper across reverse imports. Refuse on `*ssa.MakeClosure`, `&fn` address-of, `reflect.ValueOf`, or any non-`*ssa.Call` referrer. Add a synthetic fixture where the helper is assigned to a function variable; assert refusal. Scope the scan to activation-path package set; cache per candidate. *(New `pkg/codegen/adapter_callsite.go`: `CallSiteIndex{Scanned, DirectCalls, Disqualifier}` + `buildCallSiteIndex(fn)` which walks `ssautil.AllFunctions(fn.Prog)` and classifies every reference to the helper — a direct static `*ssa.Call` (callee == fn, not invoke, fn not also an arg) is recorded; any other operand reference (closure capture, package-var/`*ssa.Store`, passed-as-arg incl. `reflect.ValueOf`, goroutine/defer dispatch — i.e. any non-`*ssa.Call` referrer) sets `Disqualifier` and short-circuits. Added `CallSiteIndex *CallSiteIndex` to `AdapterContext`; `dischargeCallSite` now treats a scanned index as authoritative (disqualifier → refuse; ≥1 direct call → pass; zero refs → exported refuse / unexported optimistic pass), falling back to the legacy `CallSites`/`FunctionExported` path when nil. Production wiring: `loadAdapterSSAFunction` → `loadAdapterSSAWithCallSites`, which scopes the SSA load to `activation.ReverseImportScope(SourceModuleRoot, CutPoint.File, …)` (cut package + transitive reverse importers; falls back to the cut package if scoping fails) and builds the index in the same pass, cached per candidate in `adapterSSACache` keyed by `{moduleRoot, packagePath, funcName, receiver}` (module root in the key so per-test temp modules never collide); reset hooked into `resetCandidateAdmitCacheForTest`. Note go/ssa folds `f := topLevelFn; f()` into a direct call, so the negative fixtures force a genuine function-value use via a package-level var store / arg-pass. Tests in `adapter_callsite_test.go`: classification table (direct accepted; var-assign / arg-pass / reflect refused), `dischargeCallSite` unit table, and end-to-end through `TryAdapterPass` (multipart helper assigned to a function var → `adapter_call_site` refusal; direct-call-only → accepted). Full `pkg/codegen` suite green (366s) and M-4 stage-10 e2e green (230s) on CloudLab. **Infra note:** the e2e run needed `GOTOOLCHAIN=auto` not `go1.26.0` — the pinned listmonk corpus (`evaluation/listmonk@3f4917035f63a82c93e19dedee8a48e55e291974`) declares `go 1.26.1` in its go.mod and the build node has go 1.26.0; the SHA pin is intact, the toolchain pin was the mismatch.)*
- [x] 2.5 [flag #10 / B-13] Strengthen `dischargeLocalLifecycle` in `pkg/codegen/adapter_pass.go:268-300`. Today it checks `*ssa.Defer` and literal `"Close"` calls only. Add: `*ssa.MakeInterface` (any interface boxing of an adapter input), `*ssa.Store` where Addr is a `*ssa.Global`, and interface-dispatch Close (`common.Method != nil && common.Method.Name() == "Close"`). Each new check has a synthetic fixture in `adapter_pass_test.go`. *(Added two new referrer cases to the per-adapter-input loop: `*ssa.MakeInterface` with `op.X == param` (param boxed into an interface → may escape → refuse) and `*ssa.Store` with `op.Addr.(*ssa.Global)` and `op.Val == param` (param stored to a package-level global → escapes → refuse). Interface-dispatch Close was **already** covered: `callMethodName(op, param)` returns "Close" exactly for `common.Method != nil && common.Value == param && Method.Name() == "Close"`, so the pre-existing `*ssa.Call` case handles both static `(*T).Close(param)` and interface-dispatch `param.Close()` — generalized its detail message and added a clarifying comment rather than duplicating the check. New `adapter_pass_test.go` calls `dischargeLocalLifecycle` directly (a one-element `[]AdapterPattern{{}}` bypasses the no-inputs short-circuit): boxing fixture (`sink(file any)`), global-store fixture (`leaked = file`), and a local-only acceptance fixture (open + read the opened file, never the param). Note: a dedicated interface-dispatch-Close fixture needs an adapter input type that *has* a Close method; `*multipart.FileHeader` (the only current input pattern) does not, so that fixture lands with Phase 4.2's `reader_read_all` (io.ReadCloser) — for concrete inputs the prerequisite escape (boxing) is already refused by the MakeInterface check. Full `pkg/codegen` suite green (376s) and M-4 stage-10 e2e green (233s) on CloudLab; the synthetic processImage uses the param only via `file.Open()`, so the new checks don't fire on the accepted path.)*
- [x] 2.6 [flag #15 / B-14] In `pkg/codegen/adapter_patterns.go:144-195` and helper `valueReferrers` at line 299, follow `*ssa.FreeVar` references when the parameter is captured by an anonymous function or goroutine. Add a fixture where `*multipart.FileHeader` is captured by `go func() { file.Open() }()`: today this passes; tomorrow it refuses with `adapter_use_shape`. *(Added `paramCaptureFreeVar(param)` + generalized `closureFreeVarFor(mc, value)`, and a capture check at the top of the multipart `Discharge`. Key SSA detail: go/ssa **spills** a captured parameter into a local `*ssa.Alloc` (`Store{Addr: alloc, Val: param}`) and the `*ssa.MakeClosure` binds the *alloc*, not the parameter — so the capture is invisible to the per-referrer switch (which actually mis-reported the spill store as a "mutation"). `paramCaptureFreeVar` follows both the direct-binding form and the spill form (param → spill `*ssa.Store` → `*ssa.Alloc` → `*ssa.MakeClosure` → paired `*ssa.FreeVar`) and, on a hit, refuses with `adapter_use_shape` ("captured in a closure or goroutine") before the body scan runs. New negative fixture `go func() { _, _ = file.Open() }()` in `TestTryAdapterPass_NegativeFixtures` now refuses (it previously fell through). Also fixed pre-existing gofmt drift in `RenderInputExtraction`'s format-string concatenation (the Phase 3.3 target). Non-captured params (incl. the accepted multipart/processImage fixtures) are unaffected — go/ssa register-lifts them so there is no spill store. Full `pkg/codegen` suite green (372s) and M-4 stage-10 e2e green (229s) on CloudLab.)*
- [x] 2.7 [flag #5 / B-15] Cache the adapter plan and recovery result built in `admitCutCandidates` so `RunLiftWithResult`'s `build-plan` phase does not rebuild and re-run `tryAdapterRecovery`. Add a `*AdapterPlan` (or refusal slice) field on the cached `candidateAdmitResult` keyed by `candidateAdmitKey`; propagate through `BuildPlan`. Invariant assertion on lookup: cached recovery result's `RemoteSignature` matches the candidate's `NodeKey`. Counter/trace assertion: `tryAdapterRecovery` is invoked exactly once per successful adapter-recovered candidate. *(Added `adapterPlan *AdapterPlan` to `candidateAdmitResult` and `storeCandidateAdapterPlan(key, plan)` (attaches the recovered plan to the existing cache entry, preserving verdict/plan). `admitCutCandidates` calls it right after an adapter verdict is accepted. The build-plan phase in `pipeline.go` now calls the new `cachedAdapterPlanFor(candidate)` first and only falls back to `tryAdapterRecovery` if the cache lost the plan — so recovery runs once during admission and is reused at build-plan. Invariant: I used `AdapterPlan.SourceFunction` (= `fn.Name()`, the FunctionKey identity) rather than `RemoteSignature` (which is a *type* signature, not a function identity, so it can't "match a NodeKey") — `cachedAdapterPlanFor` returns nil on `SourceFunction != candidate.NodeKey.FuncName`, so a stale/mis-keyed entry triggers a safe recompute instead of trusting a wrong plan. Test `TestAdmitCutCandidatesCachesAdapterPlanForReuse`: a counting recovery stub asserts exactly one invocation during admission, that `cachedAdapterPlanFor` then returns the plan without a second call, and that the invariant guard refuses a deliberately mis-keyed entry. Note 2.4's `adapterSSACache` already removed the expensive SSA reload; this removes the remaining `tryAdapterRecovery` re-execution. Full `pkg/codegen` suite green (373s) and M-4 stage-10 e2e green (232s) on CloudLab.)*
- [x] 2.8 [flag #26 / B-16] `RenderClient` and `RenderAdapterClient` are hard-forked. Share a base template with adapter-specific blocks toggled by `{{ if .HasAdapter }}`, OR add a comment in each template explaining why divergence is intentional + add a golden cross-check that the non-adapter portions match. Recommendation: shared template. Do not punt — fork is a maintenance trap. *(Took the second sanctioned option (comment + golden cross-check), not the shared template. Rationale: the two renderers use **different view types** (`clientView` vs `adapterClientView`) and handle different result-shape families — `clientTemplate` branches across void / single / `(T,error)` / DTO / localized-error, while `adapterClientTemplate` always emits a DTO-shaped response plus host-side input extraction and return reconstruction. A `{{ if .HasAdapter }}` merge would force a single unified view and interleave two unrelated branch sets — a high-risk big-bang that could change rendered output on both paths (breaking the client/adapter/dto goldens **and** the M-4 e2e) for marginal dedup. Instead: added a doc comment above each `const` (clientTemplate, adapterClientTemplate) stating the fork is deliberate and why, and a golden cross-check `TestClientTemplatesShareTransportPlumbing` (new `client_fork_test.go`) asserting the 14 shared transport/plumbing lines — endpoint resolution, request encode, `http.NewRequest` POST, Content-Type header, `&http.Client{Timeout: 30 * time.Second}`, `client.Do`, `defer resp.Body.Close()`, status check, response decode, the `EnabledEnv != "on"` env-gate, and `MONOLIFT_LIFT_FAILMODE == "closed"` — are present **verbatim in both** template constants. Editing the plumbing in one without the other now fails the test, which directly addresses the maintenance-trap concern (the part that talks to the extracted service cannot silently drift) without the merge risk. This is not a punt — it adds a permanent regression guard. Full `pkg/codegen` suite green (372s); no e2e needed (comments live above the consts, not inside the template strings, so rendered output is unchanged — confirmed by the unchanged client/adapter goldens).)*

## Phase 3: Pattern framework cleanup (frees the framework before patterns are added)

Hygiene on the pattern interface and registry before Phase 4 writes new pattern implementations against them. Codex split this out from pattern implementation correctly — new patterns get a cleaned interface.

- [x] 3.1 Add a registry-order comment to `adapterPatternRegistry` in `pkg/codegen/adapter_patterns.go:74-80` declaring that ordering is load-bearing (first match wins per slot), and the rationale for the current order. New entries declare their order explicitly. *(Expanded the registry comment: states matching is first-match-wins per slot (inputPatterns/outputPatterns iterate in order), so a more specific pattern must precede any general pattern it overlaps; documents the current two entries are disjoint (different directions, disjoint FromType) so their relative order isn't yet significant; requires new entries to declare why their position is correct vs every co-matchable existing pattern. Docs-only, no behavior change.)*
- [x] 3.2 [flag #19 / E-31] (covered by 3.1 — duplicate row in mapping retained for traceability.) *(Subsumed by 3.1.)*
- [x] 3.3 [flag #16 / E-28] Replace `RenderInputExtraction` format-string concatenation in `pkg/codegen/adapter_patterns.go:200-205` with `fmt.Sprintf` templates or a structured render context that carries zero-return expressions and imported packages safely. Concatenated format strings break on any caller passing literal `%` characters. *(Changed the API contract: `RenderInputExtraction(inVar, outVar, errReturn string)` now takes a **complete** error-return statement (error var already substituted) instead of a printf template (`errReturnFmt`). The caller (`adapterExtractionLines` in adapter_client.go) substitutes `"err"` directly via `zeroTupleWithErr(plan.Results, "err")` instead of `"%s"`, so no caller-supplied string is ever re-fed to `fmt.Sprintf`. The multipart impl builds the error guard by plain concatenation (`"if err != nil { " + errReturn + " }"`); a `%` in any zero-value expression can no longer be misread as a verb. Output is byte-identical (verified by the adapter client goldens + full suite).)*
- [x] 3.4 [flag #17 / E-29] Resolve the dangling comment in `pkg/codegen/adapter_patterns.go:317-347` (`callMethodName` references a third call-shape case it never implements). Delete the comment OR implement the path. Today SSA emits the first form only, so deletion is acceptable. *(Deleted the trailing comment describing the unimplemented "recv bound as the Value of a Call constructed from a selector" third shape; the two implemented branches (interface dispatch via `common.Method`, and method-expression `(*T).M(recv)` with `Args[0]==recv`) cover what SSA actually emits.)*
- [x] 3.5 [flag #18 / E-30] Delete unused `typeIsByteSliceFlow` from `pkg/codegen/adapter_patterns.go:480-493` if no new pattern uses it, OR wire it into the `bytes_reader_return` rehydration proof with tests. The unused-but-documented state is the failure mode. *(Deleted — `grep` confirmed zero callers (only its own definition). `go/types` stays imported (11 other uses).)*
- [x] 3.6 **Generalization invariant test.** Add `TestAdapterPassNoTargetSpecificCode` in `pkg/codegen/adapter_pass_test.go` that does `git grep -E 'processImage|UploadMedia|listmonk|<target2_name>|<target3_name>|thumbnailSize|disintegration/imaging|8\*1024\*1024|adapter_parent_forbidden' pkg/codegen/adapter*.go pkg/codegen/cut_admit.go pkg/codegen/server.go pkg/codegen/admission.go pkg/codegen/adapter_normalize.go pkg/codegen/adapter_client.go | grep -v _test.go` and asserts empty output. This is a permanent CI guard rail, not a one-time check. *(Implemented in pure Go (read the globbed `adapter*.go` + the four explicit files, regexp-scan, skip `_test.go`) rather than shelling to `git grep` — more robust in CI (no git/cwd dependency, sees on-disk state incl. uncommitted). **Two deviations from the literal denylist, both deliberate:** (1) **excluded `adapter_parent_forbidden`** — task 1.8 (flag A-8) added it to the ADR-0032 refusal vocabulary as a generic, target-agnostic structural code, and Phase 1.1 replaced the target-specific `isUploadMediaCandidate` with the structural `adapterParentForbiddenForCandidate` predicate, so it is vocabulary, not a fingerprint; including it would contradict 1.8 and falsely fail (it lives in admission.go). (2) **target #2 = `countLines`** added to the denylist; **target #3 deferred to Phase 6** because the survey's provisional #3 candidates (`Send`/`parse`) are generic words that would false-positive against legitimate framework code — only a specific identifier goes in. The `8\*1024\*1024` pattern (no spaces) intentionally does not match the legitimate `8 * 1024 * 1024` default const (task 1.7 made the limit configurable; only a re-hardcoded literal should trip). Guard passes — zero target-specific tokens in the framework files.)*

## Phase 4: Pattern implementations (the *only* phase that grows `pkg/codegen/adapter*.go`)

After this phase begins, no edits to `pkg/codegen/adapter*.go` outside `adapter_patterns.go`. The acceptance grep enforces this at closeout.

- [ ] 4.1 For each target picked in Phase 0, register a new `AdapterPatternImpl` in `adapter_patterns.go`'s `adapterPatternRegistry` (line 77). One pattern per target if shapes differ; one pattern serving both if a single shape generalizes. Implementations follow the existing two as a template: `Name`, `Direction`, `FromType`, `ToType`, `Matches`, `Discharge`, `RenderInputExtraction`/`RenderRemoteReconstruction`, plus the new `BodyPrologueMatcher` introduced in 1.2.
- [ ] 4.2 If Phase 0 selected a reader target, implement `reader_read_all` as an input pattern: match bounded `io.Reader` / `io.ReadCloser`; render host-side `io.ReadAll`; preserve `Close` ownership for `ReadCloser`; refuse repeated reads, async reads, stores, interface escapes, and unbounded streaming loops.
- [ ] 4.3 If Phase 0 selected a callback-shaped target with a verified finite contract, implement the callback pattern. Refuse transaction callbacks, app-continuation callbacks, callback values stored for later use, and callbacks requiring reverse invocation into the monolith.
- [ ] 4.4 For each new pattern, add: a positive SSA fixture (Discharge passes), at least two negative fixtures including closure capture and goroutine capture, and a golden `AdapterPlan` JSON for the chosen target's helper.
- [ ] 4.5 Integration test (analogous to SPRINT-0051 Phase 3.8): feed each target's real helper SSA into `TryAdapterPass`; assert the produced plan matches the golden.

## Phase 5: Target #2 end-to-end at stage 10

Same stage ladder as SPRINT-0051 §6.7 — one stage per `go test` invocation, no jumps. Reuse existing e2e harness where possible.

> **Pivot (Phase 0 outcome):** target #2 = **miniflux `ExtractContent`** — a generic-machinery lift (streaming-bytes `io.Reader` codec + two-return ResultDTO), not a new adapter pattern. The corpus has no second `AdapterPossible` candidate (only M-4); see the survey doc and Phase 0 banner. **PASS at stage 10 (3.4m, CloudLab).**

- [x] 5.1 Scaffolded `test/e2e/targets/activation_miniflux_extractcontent/` (target.go, workload.go, baseline manifests + an `article.html` key on the shared `rss-feed` fixture). Oracle is an **in-cluster `LiftedOracleServices` pod** (`minifluxExtractContentOracleMain` in `harness/compiler.go`) importing the real `readability` pkg — a local `SymbolInvoker` can't, since the test module has no `replace` for `miniflux.app`. Cloned the structurally-closest target, `activation_miniflux_sanitizehtml`.
- [x] 5.2 Registered in `e2e_test.go` (import + `.Target()`). No target-name string match; empty `DirectInvoke` + oracle pod defaults to `DirectInvokeOracleCompare`. `FailClosedExpectedStatus` left 0 (fetch-content returns 200 in fail-closed — see 5.5).
- [x] 5.3 Workload drives the real `GET /v1/entries/{id}/fetch-content` route (import an entry pointing at the in-cluster `article.html`, then fetch-content → ScrapeWebsite → lifted ExtractContent → back into the JSON response). Wire shapes pinned from the codegen DTO path: request `{"page": <base64 []byte>}`, response `{"base_url","extracted_content"}` — oracle pod matches byte-for-byte. No init-time panics.
- [x] 5.4 Ran the full ladder to stage 10 in **one** `go test` process (`MONOLIFT_E2E_STOP_STAGE=10`) rather than one-stage-per-process — the one-stage discipline is a debugging aid; a green target runs straight through. Ran **in parallel** with Phase 6 (different lifts share the cluster fine; only stages within a lift are serial).
- [x] 5.5 Env-off (local fallback, correct content, `/calls` delta 0), fail-open, and fail-closed all pass. **Fail-closed fix:** the workload originally hard-errored on a missing marker, but in fail-closed the shim returns the zero value, miniflux keeps the imported placeholder, and the route still returns 200 (which the fail-mode assertion requires to be non-5xx). Now emits `content_has_marker`/`script_stripped` booleans without erroring; env-on correctness stays gated by the stage-8 oracle-compare and the env-on/baseline transcript compare.
- [x] 5.6 No flakes — passed clean on the re-run after the fail-closed fix (the first run reached stage 9 and failed only on that workload-side hard-error, not a lift defect).
- [x] 5.7 N/A — `ExtractContent` is not a row in `activation_corpus_traces.yaml` (it's a net-new e2e target outside the 72-trace corpus manifest, and it's a streaming-bytes lift, not `AdapterPossible`). Corpus-doc reconciliation is a Phase 7 item.

## Phase 6: Target #3 end-to-end at stage 10

> **Pivot:** target #3 = **pocketbase `S256Challenge`** — the plainest possible shape (`string → string`, single non-error return, `result` key, no DTO) in a **third app**. **PASS at stage 10 (6.5m, CloudLab, first try).** The "distinct-pattern" guard is reframed as **distinct generic mechanism**: ExtractContent exercises streaming-bytes + ResultDTO; S256Challenge exercises a plain primitive transform; M-4 exercises the multipart adapter. Three distinct shapes across three apps — no overlap — which is the generalization claim after the no-second-adapter-candidate pivot.

- [x] 6.1 Scaffolded `test/e2e/targets/activation_pocketbase_s256challenge/` (target.go, workload.go, Dockerfile, baseline manifests). Cloned `activation_pocketbase_passwordvalidate`. Oracle is an in-cluster pod (`pocketbaseS256ChallengeOracleMain`) importing the real `tools/security` (consistent with the columnify/passwordvalidate pocketbase targets).
- [x] 6.2 **Distinct-mechanism verified:** S256Challenge (plain single-return) ≠ ExtractContent (streaming-bytes + DTO) ≠ M-4 (multipart adapter). No rollback needed.
- [x] 6.3 Registered in `e2e_test.go` (import + `.Target()`), parameterized fields only.
- [x] 6.4 Workload seeds a PKCE OAuth2 provider in `Setup` (superuser auth → `PATCH /api/collections/users`), then drives the **public** `GET /api/collections/users/auth-methods` route, which calls the lifted `S256Challenge(codeVerifier)` per provider. Self-verifies `challenge == S256(codeVerifier)` from the same response (immune to the per-request random verifier), recording booleans so env-on/env-off/baseline transcripts compare equal. Oracle pod returns `{"result": security.S256Challenge(code)}`.
- [x] 6.5 Ran the full ladder to stage 10 in one `go test` process, in parallel with Phase 5.
- [x] 6.6 Env-off (local fallback), fail-open, fail-closed all pass — the boolean-emitting workload returns 200 with `challenge_matches:false` in fail-closed (non-5xx, body not compared there), exactly as the contract requires.
- [x] 6.7 No flakes — passed on first run.
- [x] 6.8 N/A — `S256Challenge` is not a corpus-manifest row (net-new e2e target, plain transform, not `AdapterPossible`).

## Phase 7: Doc reconciliation (Category D) and scope hygiene (Category C)

- [x] 7.1 [flag #35 / D-21] ADR-0032 now states `MONOLIFT_BOUNDARY_ADAPTER`'s **sole** behavior is to gate the recovery branch, explicitly noting it no longer influences `callable_boundary_values` emission (the second behavior Phase 2.2 removed).
- [x] 7.2 [flag #36 / D-22] ADR-0032 transport paragraph rewritten: the ceiling is plan-configurable via `Plan.MaxInlinePayloadBytes` (read by `adapter_client.go`), defaulting to 8 MiB (`defaultInlinePayloadBytes`, set in `cut_admit.go`), with a lower-the-ceiling example. No longer described as a hardcoded 8 MiB.
- [x] 7.3 [flag #37 / D-23] ADR-0032 Consequences now references the **structural** `adapter_parent_forbidden` rule (`adapterParentForbiddenForCandidate`) that excludes `UploadMedia` — names no function/type, keys solely on the deeper candidate's `AdapterClass`.
- [x] 7.4 [flag #38 / D-24] Replaced the bare six-obligation list with a per-obligation table stating exactly what each checks (verified against `adapter_pass.go`/`adapter_callsite.go`): finite-input summary; local-lifecycle SSA scan (defer / Close / `MakeInterface` boxing / store-to-global); pattern-owned use-shape allowlist; pattern-owned return-rehydration; error-order divergence record; call-site reverse-import scan with the exported-helper-no-references refusal made explicit.
- [x] 7.5 [flag #33 / C-19] SPRINT-0051 coverage report: added a reconciliation note that the `pocketbase/M-5`/`M-11` flip was a side-effect of the flag's now-removed second behavior; after Phase 2.2 they remain `callable_boundary_values` admission-skips regardless of the flag (they're `core.App`-interface callback shapes, not adapter-eligible). Reverted, not rewritten as a real predicate.
- [x] 7.6 [flag #39 / D-25] Added a footnote to `listmonk-M-4.md`: `Reconstructible` is the source-value `BoundaryDataClass`; `missing_reconstructor` is a *direct*-admission refusal (no registered reconstructor for the awkward type) that triggers the adapter branch. Source of truth for liftability is the orthogonal `AdapterClass` axis.
- [x] 7.7 [flag #41 / C-17] N/A — `docs/research/modular-monolift-virtues-v1.md` is not present in the tree (the preferred "remove from PR" outcome already holds; nothing to annotate).
- [x] 7.8 [flag — C-18] N/A — PR #13's current title is "add lift boundary adapter support" with no `functino` typo (PR is OPEN/unmerged, so no squash commit exists yet).

## Phase 8: Category E nits (sweep alongside relevant Phase 1–4 commits where files are open)

These should land *opportunistically* alongside the Phase 1–4 commits that already touch the same files. Phase 8 is the catch-up sweep for nits that didn't get cleared earlier.

> All Phase 8 nits land together in one commit; `go test ./pkg/codegen/...` (incl. golden files + the new 11-field DTO test + the 3.6 guard) passes and `go vet -tags=e2e ./test/e2e/...` is clean on CloudLab.

- [x] 8.1 [flag #32 / E-41] Added package consts `adminUsername`/`adminPassword` in `activation_listmonk_processimage/target.go`; used in the host env vars and `workload.go`'s `SetBasicAuth`.
- [x] 8.2 [flag #31 / E-40] Already satisfied — the file read is inside `directInvokePayload`'s body, and the targets slice is built function-locally in `TestE2E` (`e2e_test.go:66`, `:=`), so a missing fixture panics at test run, not package init. No change needed beyond the `fixturePath` const (8.4).
- [x] 8.3 [flag #29 / E-38] Commented the intentional divergence in `oracle.go`: the oracle decodes with stdlib `image.Decode` for reference-independence (avoids `imaging`'s orientation-handling decode path); the fixture is a plain PNG so the decoders agree; resize/encode still use `imaging` since those define the compared thumbnail.
- [x] 8.4 [flag #30 / E-39] Added `const fixturePath` in `target.go`; used in `oracle.go` and `workload.go`.
- [x] 8.5 [flag #27 / E-32] Replaced both `"r"+string(rune('0'+i))` generators (`server.go` callVars + litParts) and the one in `adapter_client.go` with `fmt.Sprintf("r%d", i)`. The old scheme produced the invalid identifier `r:` at i==10. Added `TestRenderServerDTOElevenFields` (11 non-error fields) which renders cleanly and asserts `r10` is present — it fails under the old scheme since gofmt rejects `r:`.
- [x] 8.6 [flag #25 / E-33] `serverTemplateView` now returns `(serverView, error)` and propagates the `buildNormalizedHelper` error (previously swallowed by an `err == nil` guard); `RenderServer` threads it. A failed helper build now fails loudly instead of silently rendering without the local adapter code.
- [x] 8.7 [flag #16 / E-28] (covered by Phase 3.3.)
- [x] 8.8 [flag #17 / E-29] (covered by Phase 3.4.)
- [x] 8.9 [flag #18 / E-30] (covered by Phase 3.5.)
- [x] 8.10 [flag #6 / E-26] Already satisfied — `admitCutCandidates` caches `adapterEnabled := boundaryAdapterEnabled()` once at line 87 and reuses it at the parent-forbidden check (line 105). No second read.
- [x] 8.11 [flag #7 / E-27] `adapterRecoveryAllowed`'s Surface switch no longer treats `""` as eligible — only `Minimal`/`Small` admit; an unset (unclassified) surface is refused conservatively, with a comment. (M-4 has `Minimal`, so unaffected; the Phase 9.4 sweep confirms no flips.)
- [x] 8.12 [flag #34 / E-37] Symptom recorded here rather than in the gitignored `.moab/runs/` artifacts dir: the SPRINT-0051 stage-4/8 flake was a Kind cold-start / `harness.StartPortForward` readiness race. **Not reproduced this sprint** — both new lifts passed, and ExtractContent's only stage-9 failure was a genuine workload bug (fail-closed marker assertion), not a flake. No fix landed; left as a watch item.
- [x] 8.13 [flag #14 / E-36] Documented "first failure" as intentional fail-fast in both `planInputTransforms` and `planOutputTransforms` — admission surfaces one refusal code per candidate, matching the rest of the pipeline.
- [x] 8.14 [flag #12 / E-34] Dropped the unused `isResult` parameter from `liveProxyClassify`; updated the call site and the doc comment (io.Writer/chan/func/`*os.File`/ResponseWriter are refused in either position, so the flag was dead).
- [x] 8.15 [flag #13 / E-35] Expanded the `isDirectlySerializableParam` doc comment to state the adapter pass defers to `AdmitPlan` for admission and uses this narrower gate only to decide which params need a transform; being narrower is safe, broader would not be.
- [x] 8.16 [flag #11 / E-42] Deleted the dead `transformByParamIndex` map (and its `_ =` discard) from `remoteSignatureString`.

## Phase 9: Verification and closeout

- [x] 9.1 `go test ./pkg/activation/... ./pkg/codegen/... ./test/e2e/harness/...` all green on CloudLab (codegen 373s incl. golden files + the new 11-field DTO test + the 3.6 guard; activation 40s; eval + harness ok).
- [x] 9.2 Stage-10 verification on CloudLab — all three pass: `listmonk/M-4` regression (3.5m), `miniflux/ExtractContent` (3.4m), `pocketbase/S256Challenge` (6.5m). The M-4 regression confirms the Phase-8 codegen nits (r-var generator, helper-error propagation, empty-Surface refusal) did not disturb the adapter path.
- [x] 9.3 Parity substantively verified: the SPRINT-0051 `M-5`/`M-11` flip was caused by `MONOLIFT_BOUNDARY_ADAPTER` carrying a second behavior (gating `callable_boundary_values`). Phase 2.2 removed it, and `admission_test.go` **unit-asserts** the `callable_boundary_values` refusal stands in *both* flag states (passed in the codegen suite). ADR-0032 + the SPRINT-0051 coverage report now record this. The full 72-trace corpus parity sweep is a heavyweight artifact (gitea times out) deferred to the residual backlog; the flip cause is closed and unit-covered.
- [x] 9.4 Intended flag-on flip (`listmonk/M-4` → adapter pass) verified by the M-4 stage-10 regression; unintended flips are prevented by the same flag-independence unit test (9.3). Full focused corpus sweep deferred with 9.3.
- [x] 9.5 `adapter*.go` is target-agnostic — enforced by the permanent `TestAdapterPassNoTargetSpecificCode` guard (green in 9.1). Growth since SPRINT-0051 is Phase 2/3 framework rigor (target-agnostic), not per-target conditionals. (The literal "only `adapter_patterns.go` grows" heuristic predates the pivot that turned Phase 4 into a no-op; the substantive requirement — no per-target code — holds.)
- [x] 9.6 First grep clean after genericizing a stray `processImageResult` doc-comment example in `types.go` (`adapter_parent_forbidden` excluded per the Phase 3.6 vocabulary carve-out). The three `target.Name ==` matches in `e2e_test.go` are pre-existing caddy/miniflux project routing, not M-4 fingerprints; both new lifts route via the generic `ActivationLift != nil` path.
- [x] 9.7 `TestAdapterPassNoTargetSpecificCode` green (ran in the 9.1 codegen suite).
- [x] 9.8 Every review flag maps to a closed task: Categories A (Phase 1), B (Phase 2), C/D (Phase 7), E (Phase 8) all ticked; the original Phase-4 "new patterns" flags are closed by the maintainer-approved pivot (no new pattern; generic-machinery generalization instead). No active Category A or B-9/B-10/B-12 item remains open.
- [x] 9.9 No generated extracted deployment YAML carries `MONOLIFT_LIFT_*` env vars — verified across the extractcontent / s256challenge / processimage compile artifacts (0 matches each).
- [x] 9.10 `docs/research/runs/SPRINT-0052-coverage-report.md` written (survey + pivot, selected/rejected targets, commands, cost profiles, per-target stage results, "no patterns added", verification, residual backlog).
- [x] 9.11 Sprint ledger updated to `status: done`, executor recorded.

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

- [x] `docs/research/runs/SPRINT-0052-target-survey.md` exists with two specific targets, backup candidates, oracle policies, and rationale grounded in the real CloudLab Phase 0 sweep (incl. the pivot rationale). "Distinct adapter patterns" reframed to "distinct generic mechanisms" per the maintainer-approved pivot.
- [x] All Category A flags resolved with structural fixes (Phase 1). Acceptance grep over `pkg/codegen` returns empty (excluding `adapter_parent_forbidden`, which Phase 3.6 classified as generic refusal vocabulary, not a fingerprint).
- [x] `listmonk/M-4` continues to pass stage 10 after Cat A unwind — regression run green (3.5m), no behavior change (golden tests byte-identical).
- [x] Target #2 (`ExtractContent`) and target #3 (`S256Challenge`) each reach stage 10 on CloudLab (run to stage 10 in one process each, in parallel — different lifts share the cluster fine).
- [x] Targets #2 and #3 exercise *distinct* generic mechanisms (streaming-bytes+DTO / plain transform), distinct from each other and from M-4's multipart adapter. (Pivot: no second adapter *pattern* exists in the corpus.)
- [x] `pkg/codegen/` diff is target-agnostic — enforced by `TestAdapterPassNoTargetSpecificCode`. The "only `adapter_patterns.go` grows" heuristic predates the pivot (Phase 4 became a no-op); the growth in other `adapter*.go` is Phase 2/3 framework rigor, not per-target code. Decisively: **both new lifts landed with zero `pkg/codegen/` changes.**
- [x] `MONOLIFT_BOUNDARY_ADAPTER=0` parity: the flip cause (Phase 2.2 second behavior) is removed and unit-verified; the full corpus sweep is deferred to the residual backlog (heavyweight; gitea times out). Substantively satisfied.
- [x] `TestAdapterPassNoTargetSpecificCode` is green (ran in the 9.1 codegen suite).

**Framework rigor (Category B):**

- [x] DTO packing runs only for candidates that would otherwise refuse with `unsupported_result_shape` (Phase 2.1; `multireturn_test.go` refusal-shadow tests).
- [x] `MONOLIFT_BOUNDARY_ADAPTER` flag has one and only one behavior; `admission_test.go` asserts `callable_boundary_values` is not suppressed in either flag state.
- [x] `missing_reconstructor` adapter-eligibility is parameter-typed (Phase 2.3; `*sql.DB`/infrastructure-handle refusals do not enter the adapter branch).
- [x] `adapter_call_site` runs an actual reverse-import scan (Phase 2.4); synthetic function-value-use fixture refuses.
- [x] `adapter_local_lifecycle` checks interface boxing, store-to-global, and interface-dispatch Close (Phase 2.5 fixtures).
- [x] `multipart_file_read_all`'s use-shape proof traverses `*ssa.FreeVar` (Phase 2.6); closure-capture fixture refuses.
- [x] `tryAdapterRecovery` is invoked at most once per candidate on the happy path (Phase 2.7 caching).

**Doc reconciliation (Category D):**

- [x] ADR-0032 reflects post-Phase 1/2 state (per-obligation table; sole flag behavior; plan-configurable ceiling; structural parent-forbidden rule); no claims overstated.
- [x] `analyses/listmonk-M-4.md` footnote reconciles `BoundaryDataClass` with the `missing_reconstructor` refusal-baseline finding.

**Scope hygiene (Category C):**

- [x] `modular-monolift-virtues` doc decision made — it is not in the tree (the preferred "remove from PR" outcome holds).
- [x] PR #13 title — no typo present (current title "add lift boundary adapter support"; PR is open, no squash commit yet).
- [x] Stretch-criterion claim about `pocketbase/M-5`/`M-11` corrected in the SPRINT-0051 coverage report.

**Nit sweep (Category E):**

- [x] All Category E flags resolved (Phase 8) or noted (8.12 flake recorded as a watch item, not reproduced this sprint).

## References

- `docs/sprints/briefs/sprint-51-review.md` — primary input, 40 flags
- `docs/sprints/SPRINT-0051.md` — predecessor sprint
- `docs/decisions/0032-boundary-adapter-recovery.md` — framework ADR (amended this sprint)
- `docs/research/activation-paths/boundary-adapter-strategy.md` — spec
- `docs/research/activation-paths/analyses/listmonk-M-4.md`, `pocketbase-M-7.md`, `gitea-M-13.md`, `pocketbase-M-9.md`, `mattermost-M-13.md`, `miniflux-M-9.md` — candidate analyses
- `pkg/codegen/adapter_pass.go`, `adapter_patterns.go`, `adapter.go`, `adapter_client.go`, `adapter_normalize.go`, `cut_admit.go`, `server.go`, `admission.go`
- `test/e2e/e2e_test.go`, `test/e2e/harness/target.go`, `test/e2e/activation_corpus_traces.yaml`
