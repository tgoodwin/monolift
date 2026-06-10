# SPRINT-0051 PR review consolidation

PR: https://github.com/tgoodwin/monolift/pull/13 (`sprint-51` → `main`)
Reviewer: Tim Goodwin (interactive session with Claude)
Date: 2026-05-19

## Diagnosis

SPRINT-0051 is shaped like a generalizable framework but implemented as an M-4-specific spike with a framework wrapped around it. The framework code (`AdapterClass`, `AdapterPlan`, six obligation slots, pattern registry, refusal-code vocabulary) is in the right place architecturally. The implementation under it leans heavily on M-4-specific knowledge: variable names, package paths, constants, function names, AST shapes. None of it would survive a second adapter target without changes.

The next sprint's correctness test is therefore: **can we land two additional adapter-enabled lifts e2e without modifying anything in `pkg/codegen/adapter*.go` except the pattern registry?** If yes, the framework is real. If no, every "fix" we make for the second target measures the same fragility this review surfaces.

## Categories

### Category A — SPRINT-0052 blockers (must fix before a second adapter target can ship)

These are the hardcoded-for-processImage pieces. They will *actively prevent* a second adapter target from working, not just make it slower to land.

1. **`isUploadMediaCandidate` is a string match in production code** (flag #2). The right fix is to make `adapterRecoveryAllowed` refuse the actual structural property — broad parent of an adapter-eligible child — without naming any function. The current code makes the entire admission policy depend on a corpus function name.

2. **Body rewrite is `strings.Replace` against literal `processImage` source text** (flag #20). Two indentation variants for the *same* substitution, hardcoded variable names (`file`, `src`, `input`, `out`), hardcoded return shape (`nil, 0, 0, err`). Either:
   - Move to AST-aware rewrite using the pattern's `RenderInputExtraction` output (which is already structured), or
   - Make the pattern own the body-rewrite directly (each pattern declares old/new AST templates).
   - The first option is more aligned with the spec; the second is simpler.

3. **`thumbnailSize = 250` constant hardcoded in `serverLocalAdapterCode`** (flag #21). This constant belongs to `processImage`, not to the adapter mechanism. It must come from inspecting the cut function's free symbols and re-emitting them in the helper, or from a pattern declaration.

4. **`github.com/disintegration/imaging` unconditionally imported for any main-package adapter target** (flag #22). Import set must be computed from the cut function's actual imports, not hardcoded.

5. **`applyProcessImageResultNames` renames DTO fields by detecting M-4 return shape** (flag #23). DTO field naming should come from the original return names (already in `Plan.Results[i].Name`) or fall back to `Result0..N`. Detecting `([]byte, int, int, error)` and assigning `thumbnail`/`originalWidth`/`originalHeight` is target-specific.

6. **`e2e_test.go` has `if target.Name == "activation-listmonk-processimage"` branches** (flag #28). Should be `TargetCase.FailClosedExpectedStatus int` and `TargetCase.InvokeResultExtractor func(map[string]any) any` fields. Hardcoding the target name in the harness is the same anti-pattern as in codegen.

7. **8 MiB ceiling hardcoded in generated code, not read from `AdapterPlan.TransportPolicy`** (flag #24). The ADR markets this as plan-configurable policy. Code reads a literal. Fix: thread `plan.AdapterPlan.TransportPolicy.MaxInlinePayloadBytes` into `adapterExtractionLines`.

8. **`adapter_parent_forbidden` refusal code isn't in documented vocabulary** (flag #8). Either add it formally (with an ADR amendment) or replace it with a refusal computed by structural admission policy.

### Category B — Framework rigor (next two targets will expose these)

These don't break a single target but will surface as soon as the second adapter target has a different shape than `processImage`.

9. **DTO packing scope too broad** (flag #1). Currently runs for every boundary with `(T, U, ..., error)`, not just adapter-eligible ones. Gate it: only run DTO packing when admission would otherwise refuse with `unsupported_result_shape`.

10. **`MONOLIFT_BOUNDARY_ADAPTER` flag has hidden second behavior** (flag #3). Silently suppresses `callable_boundary_values` in `AdmitCut` for every candidate, not just adapter-eligible ones. This is the mechanism behind the "stretch deliverable" of `pocketbase/M-5` and `M-11` flipping classification (flag #33). Decide:
   - Is suppressing `callable_boundary_values` an *intentional* design step? Then it needs its own ADR.
   - If not, gate the suppression on the candidate having `AdapterClass == AdapterPossible`.

11. **`missing_reconstructor` adapter-eligibility is incomplete** (flag #4). Currently accepts all `missing_reconstructor` refusals as adapter-eligible; the decision doc says only parameter-type-related ones should trigger recovery (not receivers, not DB/filesystem). Add a refusal-type predicate.

12. **`adapter_call_site` proof is vacuous in production** (flag #9). The pass is called with `CallSites: nil`, so the proof reduces to "is the helper unexported?" The sprint plan §3.4.6 required a reverse-import-scope scan. Either implement the scan or downgrade ADR-0032's claim about the obligation.

13. **`adapter_local_lifecycle` is incomplete** (flag #10). Function comment promises interface-boxing detection; code only checks `*ssa.Defer` and literal `"Close"` calls. Add `*ssa.MakeInterface` check, escape-to-global via `*ssa.Store`, and interface-dispatch Close detection.

14. **`multipart` pattern's use-shape proof misses closure capture** (flag #15). `valueReferrers` doesn't follow `*ssa.FreeVar` references from anonymous functions. A helper that does `go func() { file.Open() }()` passes the proof and silently breaks the adapter.

15. **Plan built twice on happy path** (flag #5). `admitCutCandidates` builds + recovers, then `RunLiftWithResult`'s `build-plan` phase builds again and re-runs `tryAdapterRecovery`. Cache the plan and recovery result; propagate through pipeline.

16. **`RenderClient → RenderAdapterClient` hard fork** (flag #26). No shared structure with the non-adapter renderer. As soon as wire format changes, you maintain two templates. Either share a base template or document that they intentionally diverge with a comment in each.

### Category C — Scope hygiene / process issues

These shouldn't have happened in this PR regardless of M-4 outcome.

17. **`docs/research/modular-monolith-virtues-v1.md` is unrelated to the sprint** (flag #41). 66 lines of industry-trends commentary added as "post-sprint, May 2026." Either remove from this PR or land separately.

18. **PR title has a typo** (`"functino"`). Fix when squashing.

19. **Stretch criterion "additional corpus candidate flips classification" claimed via flag side-effect, not adapter recovery** (flag #33). `pocketbase/M-5` and `M-11` flipped because of flag #10's hidden suppression of `callable_boundary_values`, not because the adapter pass ran on them. Either back out the stretch claim or document the actual mechanism in the coverage report.

> **Note:** flags #40 and #42 (deletion of `validation-ladder.md` and the `working-backwards.md` replacement) were retracted after maintainer clarification — the validation ladder is internal dev tracking, not user-facing product docs. Intentional removal from public site nav, not scope creep.

### Category D — Doc-vs-code drift

ADR-0032 and analysis docs make claims the code doesn't deliver. Fix in this PR or open a follow-up amendment.

21. **ADR-0032 doesn't document `MONOLIFT_BOUNDARY_ADAPTER`'s second behavior** (flag #35). See B-10.

22. **ADR-0032 implies 8 MiB ceiling is plan-configurable** (flag #36). See A-7.

23. **ADR-0032 doesn't acknowledge `isUploadMediaCandidate`** (flag #37). See A-1.

24. **ADR-0032 "discharges six named obligations" overstates rigor** (flag #38). Two are summary-only, two are pattern-delegated, one is heuristic, one is partial. Tighten the language.

25. **M-4 analysis BoundaryDataClass = "Reconstructible" needs a footnote** (flag #39) reconciling with Phase 0's `missing_reconstructor` finding.

### Category E — Nits / minor

26. Flag re-read inside admit loop instead of using cached value (flag #6).
27. `adapterRecoveryAllowed` Surface check fails open on empty string (flag #7).
28. `RenderInputExtraction` builds format strings via concatenation (flag #16).
29. `callMethodName` has dangling comment with no implementation (flag #17).
30. `typeIsByteSliceFlow` is unused dead code (flag #18).
31. Pattern registry ordering is load-bearing but uncommented (flag #19).
32. `adapterReturnExpressions` uses `rune('0'+i)` for variable names — breaks at index 10 (flag #27).
33. `serverLocalAdapterCode` swallows `normalizedHelperBody` errors (flag #25).
34. `liveProxyClassify(typ, isResult)` discards `isResult` (flag #12).
35. `isDirectlySerializableParam` is more conservative than `AdmitPlan` — gap could surprise future authors (flag #13).
36. Refusal trail on pattern proof failures reports only the first failure (flag #14).
37. Stage 4 needed 8 reruns, stage 8 needed 3 — undocumented flake (flag #34).
38. Oracle uses `image.Decode`, helper uses `imaging.Decode` (flag #29).
39. Fixture path duplicated as string literal in oracle.go and workload.go (flag #30).
40. `directInvokePayload()` panics at test-registration time if fixture missing (flag #31).
41. Admin credentials duplicated as string literals (flag #32).
42. `remoteSignatureString` has dead `transformByParamIndex` map — aborted refactor (flag #11).

## What this means for SPRINT-0052

Proposed sprint thesis:

> **SPRINT-0052: Prove the boundary-adapter framework generalizes by landing two additional adapter-enabled lifts e2e (`reader_read_all` + one TBD), gated on Category A clean-up and B-9 / B-10 / B-12 from the SPRINT-0051 review.**

Acceptance condition for "generalizes": the two new adapter targets each ship at stage 10 with **zero** edits to `pkg/codegen/adapter*.go` outside the pattern registry, and **zero** new target-name string-matches in `pkg/codegen/` or `test/e2e/e2e_test.go`.

Candidate second adapter targets (need a Phase 0 survey, but plausible):
- `pocketbase/M-7` (callback). Sprint plan explicitly carved out from SPRINT-0051 scope; would force a callback-shaped pattern.
- `gitea/M-13` (queue handler with `io.Reader`). Would force `reader_read_all`.

The Category A items must land before Phase 0 of SPRINT-0052 begins or the new target work will produce more M-4-specific overfitting that compounds.

## Suggested PR action

Two options:

**Option 1 — Merge with Category A as immediate follow-up.**
- Merge PR #13 as-is to lock in M-4 stage 10 evidence.
- Open SPRINT-0052 with Category A as the explicit Phase 0 input.
- Open separate cleanup PRs for Categories C (revert validation-ladder deletion + revert modular-monolith-virtues add) before SPRINT-0052 starts.

**Option 2 — Address Category A + C in this PR before merge.**
- Revert validation-ladder deletion and modular-monolith-virtues addition (Category C #17, #18) immediately.
- Address `isUploadMediaCandidate` refactor (A-1), `thumbnailSize` parameterization (A-3), and `imaging` import gating (A-4) in this PR. These are the most visible overfitting smells.
- Land the remaining Category A items in SPRINT-0052 Phase 0.

Option 1 is faster but normalizes the overfitting in the merged history. Option 2 is slower but means the merged code looks more like what we want the framework to look like. I lean Option 2 but the call is the maintainer's.
