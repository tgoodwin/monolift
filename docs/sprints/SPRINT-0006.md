# SPRINT-0006 — SSA Extraction (Caddy flip) + Refusal-Diagnostic Framework (Pocketbase flip)

**Status:** planned · **Scope:** two paired harness flips on top of SPRINT-0005's pragma parser
**Primary deliverables:**
1. Real SSA-based extraction that produces `reportv2.Root` + `reportv2.Closure` + `reportv2.ExternalDeps` for Caddy, flipping the Caddy target from `test/e2e/stubcompiler/fixtures/caddy/closure-report.json` to real compiler output.
2. Compiler-owned `compiler.Diagnostic` → `reportv2.Diagnostic` translation framework (new sibling package under `pkg/compiler/`), flipping Pocketbase off its stub fixture with real-compiler refusal output.

**Primary inputs:** `docs/sprints/SPRINT-0005.md` (Phase 7 seam + SPRINT-0006 seed list); `docs/specs/monolift-v2-contract.md` §Extraction Root and Closure, §Closure Rules, §Refusal Diagnostic Index, §PocketBase refusal spec; ADR-0011 (harness-before-compiler); ADR-0012 (pragma-parser-diagnostics boundary); `pkg/compiler/pragma.go` (parser API); `pkg/compiler/reportv2/report.go`; `test/e2e/stubcompiler/main.go` (current translation seam); `test/e2e/stubcompiler/fixtures/caddy/closure-report.json`; `test/e2e/stubcompiler/fixtures/pocketbase/closure-report.json`; `test/e2e/harness/report.go` and `verdict.go` (harness normative-subset logic); `test/e2e/targets/caddy/target.go`; `test/e2e/targets/pocketbase/target.go`.

**Prerequisite for:** SPRINT-0007+ canonical-shape classifier, state-class inference, adapter codegen, broader refusal-code coverage, Miniflux unskip.

---

## Why this sprint exists

SPRINT-0005 gave Monolift a real pragma parser and proved `pkg/compiler → reportv2` can carry parser diagnostics end-to-end. The harness still reaches stage 10 / stage 4 for Caddy and Pocketbase only because the stub compiler *copies a hand-authored JSON fixture* into the output directory. The compiler owns none of the two most important downstream truths in the v2 contract:

- **Extraction** — given a parsed root + surface + options, what is the bounded closure, and what are the external dependencies? (Caddy.)
- **Refusal** — given compiler-internal findings, how do they become `reportv2.Diagnostic` values with file-relative spans, byte offsets, line ranges, and remediation text? (Pocketbase.)

This sprint flips both, one target at a time, preserving ADR-0011's invariant: Caddy and Pocketbase go red against real-compiler expectations *before* any compiler code lands to turn them green. VTA / pointer-analysis tuning stays explicitly deferred — documented in ADR-0013 with precision-vs-cost rationale, not dropped silently.

## Goals

- [x] Real SSA-backed extraction in `pkg/compiler` consumes SPRINT-0005 parser output and emits `reportv2.Root` + `reportv2.Closure` + `reportv2.ExternalDeps` for Caddy.
- [x] Package loading uses `golang.org/x/tools/go/packages` in `LoadAllSyntax` mode over the module reachable from the annotated root; SSA built via `golang.org/x/tools/go/ssa`. Build tags and `CGO_ENABLED` propagate from the environment into the loader config.
- [x] Interface-call resolution uses CHA as default and RTA-refined dispatch where a registry-keyed root narrows the assignable set. VTA / pointer analysis is deferred with explicit rationale in ADR-0013.
- [x] Unresolved edges (reflection, `unsafe`, dynamic plugin loads) refuse with the code chosen in Phase 0 rather than silently lifting, widening to whole-program, or falling back.
- [x] Caddy flips: `test/e2e/stubcompiler/fixtures/caddy/closure-report.json` is retired; Caddy e2e stages 0–10 stay green against real compiler output.
- [x] `compiler.Diagnostic` → `reportv2.Diagnostic` translation moves from `test/e2e/stubcompiler/main.go` into a new sibling package under `pkg/compiler/`. Final name is decided in Phase 0 after inventory (working placeholder: `pkg/compiler/diagnostics/`).
- [x] Source-span formatting in that package emits file-relative path (rebased against `BuildConfig.ModuleRoot`), byte offsets, line start/end, plus remediation text. Parser spans from SPRINT-0005 and extraction spans from this sprint share the same seam.
- [x] Only the `MLV2_*` refusal codes Pocketbase currently asserts are implemented compiler-side: `MLV2_EMBEDDED_DB_APP_ROOT` and `MLV2_CLOSURE_TOO_LARGE`. Other refusal codes stay stubbed / fixture-based.
- [x] Pocketbase flips: `test/e2e/stubcompiler/fixtures/pocketbase/closure-report.json` is retired; Pocketbase e2e stages 0–4 stay green against real compiler output.
- [x] ADR-0013 records SSA precision policy (CHA/RTA chosen; VTA/pointer deferred) with precision-vs-cost rationale. A "Future work — SSA precision" section in this sprint file echoes the deferral.
- [x] `reportv2.Analysis.Algorithm` and `reportv2.Analysis.Deterministic` are populated meaningfully; precision triggers are recorded deterministically.

## Non-goals

- No canonical-shape classifier beyond what is already required to preserve Caddy's current accepted handler/registry shape.
- No general state-class inference. Pocketbase's `shared-mutable-across-callers` state row is reproduced by narrow Pocketbase-shaped detection, not by a general inference pass.
- No adapter codegen, lifted-deployable codegen, or Kubernetes/manifest changes.
- No Miniflux/Listmonk/Gitea/Mattermost unskip. No new positive or refusal target beyond Caddy and Pocketbase.
- No VTA / pointer-analysis tuning (deferred via ADR-0013).
- No implementation of `MLV2_*` refusal codes Pocketbase does not exercise.
- No v1 demo repair. No new refusal codes beyond the one chosen in Phase 0 (e.g., do not introduce `MLV2_CGO_UNLIFTABLE` in this sprint).
- No deletion of `test/e2e/stubcompiler/`. Other targets still rely on its fixture-copy path; only Caddy + Pocketbase stop depending on it.
- No changes to `pkg/lift/*`, `pkg/pragma/*`, `pkg/metrics/*`, runtime code, or `demo/`.

## Scope boundaries

**In scope:**
- New v2 analysis files under `pkg/compiler/` for package loading, SSA construction, root resolution, closure walking, pruning, and report assembly. File layout (single package vs. `pkg/compiler/extract/` or similar) is decided in Phase 0 after caller inventory.
- New sibling package under `pkg/compiler/` for diagnostic translation + span formatting + remediation. Name finalized in Phase 0 (`pkg/compiler/diagnostics/` unless inventory suggests a stronger alternative; **do not** use `pkg/compiler/ssa/` — collides with `golang.org/x/tools/go/ssa` — and **do not** use `diagv2` as there is no `diag` v1 to version against).
- `pkg/compiler/compiler.go` wiring edits to connect the new extraction entrypoint to parser output.
- `pkg/compiler/reportv2/` only for validator widening forced by real-report content the stub path never populated (additive; no schemaVersion bump).
- `test/e2e/stubcompiler/main.go` — stop short-circuiting Caddy/Pocketbase via fixture copy; delegate diagnostic translation to the new package.
- `test/e2e/targets/caddy/*`, `test/e2e/targets/pocketbase/*`, `test/e2e/harness/*` — target metadata, `SourceDirs` plumbing, normative-subset comparison.
- `test/e2e/stubcompiler/fixtures/caddy/` and `.../pocketbase/` — retired at end of sprint.
- `docs/decisions/0013-ssa-closure-precision.md` (new) — SSA precision policy (CHA/RTA now, VTA/pointer deferred).
- `docs/decisions/0014-unbounded-edge-refusal-code.md` (new) — records the Phase-0 decision on `MLV2_CLOSURE_UNBOUNDED` taxonomy.
- `docs/specs/monolift-v2-contract.md` only if ADR-0014 decides to add `MLV2_CLOSURE_UNBOUNDED` to §Refusal Diagnostic Index + §Extraction Root and Closure.
- `docs/evolution.md` closeout entry.

**Out of scope (do not touch unless a compile break forces it):**
- `pkg/compiler/pragma*` (parser is stable; do not revise SPRINT-0005 surfaces).
- `pkg/lift/*`, `pkg/pragma/*`, `pkg/metrics/*`, runtime code.
- Other stub fixtures (Miniflux/Listmonk/Gitea/Mattermost closure-reports stay as-is; the stub copyTree path continues to serve them).
- Schema-shape changes to `reportv2.Report` structs (additive-only if forced).

---

## Tasks

All concrete work is checkboxed. Phases ordered. **Phase 1 (red-first harness flip) lands before Phases 2–6 (compiler implementation)** per ADR-0011.

### Phase 0 — Baseline, inventory, contract alignment, de-risking spike

- [x] Capture pre-sprint baseline: `go test ./pkg/compiler/... ./test/e2e/stubcompiler ./test/e2e/harness` and `MONOLIFT_E2E=1 make e2e`. Record Caddy green @ stage 10, Pocketbase green @ stage 4, SPRINT-0005 pragma rows green, deferred targets skipped.
  Baseline 2026-04-20: `go test ./pkg/compiler/... ./test/e2e/stubcompiler ./test/e2e/harness` passed; `MONOLIFT_E2E=1 make e2e` passed in 126.683s for `./test/e2e`, with Caddy green at stage 10, Pocketbase green at stage 4, all seven pragma targets green, and Miniflux/Listmonk/Gitea/Mattermost skipped as deferred.
- [x] **Harness normative-subset inventory — Caddy.** Read `test/e2e/stubcompiler/fixtures/caddy/closure-report.json` against `test/e2e/targets/caddy/golden/report.json` and the comparison logic in `test/e2e/harness/report.go` + `verdict.go`. Enumerate every field the harness actually compares: `analysisAlgorithm`, `root`, `pragmaVerdict`, `boundedPruning`, `stateDispositions`, `adapterKinds`, `externalAccessPaths`, `diagnosticCodes`, and any additional fields the current harness logic touches. Record the must-reproduce field list — this is Caddy's acceptance target.
  Inventory: `CompareNormativeSubset` compares only `schemaVersion`, `analysis.algorithm`, `root.identity`, `pragma.options["verdict"]`, `pruning.bounded`, sorted `state[].symbol.object_name + "=" + disposition`, sorted `adapters[].kind`, sorted `externalDependencies[].accessPath`, and sorted `diagnostics[].code`. `AssertAccept` additionally requires `pragma.options["verdict"] == "accept"` and zero error-severity diagnostics. Caddy fixture and golden are identical on this surface; the harness does not compare registry key, exposed operations, precision triggers, spans, messages, remediations, closure body, or build config.
- [x] **Harness normative-subset inventory — Pocketbase.** Same exercise: required diagnostic codes + remediation strings + state rows (including the `shared-mutable-across-callers` / `BaseApp.db=refused` row) + any other compared fields.
  Inventory: the same normative subset is compared as for Caddy. For Pocketbase that means `schemaVersion`, `analysis.algorithm`, `root.identity`, `pragma.options["verdict"]`, `pruning.bounded`, sorted `stateDispositions` (currently `BaseApp.db=refused`), sorted `adapterKinds` (empty), sorted `externalAccessPaths` (empty), and sorted `diagnosticCodes` (currently `MLV2_CLOSURE_TOO_LARGE`, `MLV2_EMBEDDED_DB_APP_ROOT`). `AssertRefuse` additionally requires `pragma.options["verdict"] == "refuse-blocking"` and presence of both required error codes. The current harness does not compare diagnostic remediation strings, messages, rule IDs, spans, precision triggers, or state classes/evidence even though the golden contains them.
- [x] **Caller inventory.** Enumerate every current use of `compiler.Parse`, `compiler.Diagnostic`, and `reportv2.Report` assembly in `test/e2e/stubcompiler/main.go` and any other entrypoint. The v2 analysis path and new diagnostic package must not widen into unrelated compiler code. Output: a scoped list of edit sites.
  Scoped edit sites from `rg`: `test/e2e/stubcompiler/main.go` is the only current caller of `compiler.Parse` and the only place assembling a `reportv2.Report` from parser output (`emitPragmaReport`, `buildPragmaReport`, `verdictForDiagnostics`, `toReportDiagnostics`, `toReportSpan`). Downstream consumers that must keep working without widening the seam are `test/e2e/harness/compiler.go` (loads `closure-report.json` into `reportv2.Report`), `test/e2e/harness/report.go` (normative subset compare), and `test/e2e/harness/verdict.go` (verdict/required-code assertions). No unrelated compiler entrypoint currently consumes `compiler.Diagnostic` or assembles v2 reports.
- [x] **Unbounded-edge refusal code decision (ADR-0014).** Sprint intent calls for `MLV2_CLOSURE_UNBOUNDED`. The v2 contract §Refusal Diagnostic Index currently has `MLV2_REFLECTION_DISPATCH`, `MLV2_DYNAMIC_PLUGIN`, and `MLV2_DISPATCH_SET_UNBOUNDED`. Make one decision (no option trees): either (a) add `MLV2_CLOSURE_UNBOUNDED` to the spec as an umbrella for non-dispatch unbounded edges (e.g., `unsafe`-mediated crossings and opaque function-value escapes) while keeping the existing dispatch-specific codes, or (b) fold all such cases into the existing specific codes and drop `MLV2_CLOSURE_UNBOUNDED` from the sprint. Record the decision in a new ADR at `docs/decisions/0014-unbounded-edge-refusal-code.md` with Context (existing taxonomy, gap analysis), Decision (chosen option + scope boundaries vs dispatch-specific codes), and Consequences (which future refusals inherit this taxonomy) before compiler code emits any unbounded refusal.
  Decision: option (a). ADR-0014 adds `MLV2_CLOSURE_UNBOUNDED` as the umbrella for non-dispatch unbounded frontier cases (notably `unsafe`-mediated crossings and opaque function-value escapes) while preserving `MLV2_REFLECTION_DISPATCH`, `MLV2_DYNAMIC_PLUGIN`, and `MLV2_DISPATCH_SET_UNBOUNDED` for their existing dispatch-specific roles.
- [x] If ADR-0014 adds `MLV2_CLOSURE_UNBOUNDED`: amend `docs/specs/monolift-v2-contract.md` §Refusal Diagnostic Index + §Extraction Root and Closure, distinguishing its scope from the dispatch-specific codes. Cross-link the spec entry to ADR-0014.
  Spec amended in §Extraction Root and Closure (`EC-TERM-7`) and §Refusal Diagnostic Index; both now define `MLV2_CLOSURE_UNBOUNDED` as the non-dispatch umbrella and cross-link to ADR-0014.
- [x] **SSA de-risking spike.** Write a throwaway test in `pkg/compiler/testdata/ssa_spike_test.go` (or similar) that loads a trivial local Go package with `packages.LoadAllSyntax`, builds an `ssa.Program`, and produces a CHA callgraph. Confirm the tooling cost and that `CGO_ENABLED` / build-tag propagation works as expected. This spike de-risks Phase 2 before the harness flip commits the project.
  Added `pkg/compiler/ssa_spike_test.go` plus `pkg/compiler/testdata/ssaspike/`. `go test ./pkg/compiler -run TestSSASpikeBuildsCHACallgraphAndPropagatesEnv -v` passed; measured `LoadAllSyntax+SSA+CHA` at 113.875208ms with `CGO_ENABLED=1` and `-tags=monoliftspike`, and 57.961875ms with `CGO_ENABLED=0` and no tags. The test asserts the compiled file set flips between `tagged_on.go`/`tagged_off.go` and `cgo_on.go`/`cgo_off.go`, confirming env propagation works on a local module.
- [x] **Caddy call-graph complexity probe.** Run a small diagnostic script (can reuse the spike) against the Caddy module sources referenced by `test/e2e/targets/caddy/target.go` to estimate: number of SSA functions, CHA callgraph node count, interface-dispatch fan-out at the reverse-proxy root. Record as a Phase-0 artifact. This is the early hedge against "CHA too imprecise for Caddy" — if the probe reveals a pathological fanout, Phase 2 can pre-commit to RTA-refinement before Phase 3 starts.
  Added `pkg/compiler/ssa_probe_test.go` (env-gated diagnostic). Under `GOTOOLCHAIN=go1.25.4 MONOLIFT_SSA_PROBE=1 go test ./pkg/compiler -run TestCaddySSAProbe -v`, the Caddy reverse-proxy probe reported `ssaFunctions=78289`, `chaNodes=78290`, and one direct interface invoke at the root with fanout `22` (`Value` at `modules/caddyhttp/reverseproxy/reverseproxy.go:453`). This is high enough to justify the sprint’s planned RTA refinement at registry-keyed roots, but it is not pathological enough to make CHA unusable as the default baseline.
- [x] Open ADR-0013 stub at `docs/decisions/0013-ssa-closure-precision.md` with Context section (why CHA + RTA, what VTA/pointer would buy, why cost is not justified yet). Decision + Consequences filled in Phase 4.
  Stub added with Context only. It records the CHA/RTA/VTA tradeoff and Phase-0 probe data; Decision and Consequences remain intentionally open for Phase 4 finalization.
- [x] Confirm `golang.org/x/tools` is pinned in `go.mod`; if not, add it with a pinned version and record the bump in the baseline.
  Confirmed present and pinned already: `golang.org/x/tools v0.34.0`. No `go.mod` bump required for Phase 0.
- [x] Decide final names for the two new packages (extraction and diagnostics) based on caller inventory + existing `pkg/compiler/` conventions. Record the names in this sprint file before Phase 2 starts. **Avoid** `pkg/compiler/ssa/` (collides with `x/tools/go/ssa`) and `diagv2` (no `diag` v1).
  Final names: `pkg/compiler/extract/` for the SSA-backed analysis substrate and `pkg/compiler/diagnostics/` for the `compiler.Diagnostic -> reportv2.Diagnostic` translation seam. The public entrypoint can still remain a `compiler.Extract(...)` facade, but the implementation packages for this sprint are fixed to those names.

### Phase 1 — Red-first harness flip *(lands before compiler implementation)*

- [x] Add `SourceDirs` to `test/e2e/targets/pocketbase/target.go` pointing at the Pocketbase module sources the compiler should analyze.
  Added `SourceDirs: []string{"evaluation/pocketbase"}` to the Pocketbase target so the red-first flip and later real compiler path analyze the actual evaluation module instead of relying on fixture-only metadata.
- [x] Confirm or add `SourceDirs` on `test/e2e/targets/caddy/target.go` — do not assume it exists. The Phase 1 red-first failure signature depends on this being correct for both targets.
  Confirmed existing `SourceDirs` on the Caddy target: `[]string{"evaluation/caddy", "test/e2e/targets/caddy"}`. No metadata change required before the harness flip.
- [x] Modify `test/e2e/stubcompiler/main.go` so Caddy and Pocketbase no longer match the `test/e2e/stubcompiler/fixtures/<target>/` fixture-copy short-circuit path. Other stub-backed targets continue to copy fixtures unchanged.
  Added `usesRealCompiler(target)` in the stubcompiler so only Caddy and Pocketbase bypass the fixture-copy short-circuit; Miniflux and the other still-stubbed targets continue to use fixture emission unchanged.
- [x] Replace fixture-copy for Caddy + Pocketbase with a call into `pkg/compiler`'s (not-yet-implemented) real extraction entrypoint. The build must fail because `pkg/compiler` does not yet expose the API — this is the red-first baseline. (Choosing to rewire the seam rather than just move fixture files produces a precise "real path empty" failure signature rather than a file-not-found one.)
  Rewired the Caddy/Pocketbase path to call `compiler.Extract(sources, pragmas)` after `compiler.Parse(...)`. Verified the intentional red-first build failure with `go build ./test/e2e/stubcompiler`: `undefined: compiler.Extract`.
- [x] Keep `test/e2e/targets/caddy/golden/report.json` and `test/e2e/targets/pocketbase/golden/report.json` unchanged.
  Confirmed: no edits made to either golden during the red-first flip.
- [x] Land a **stub** `compiler.Extract(sources []string, pragmas []*compiler.Pragma) (reportv2.Report, []compiler.Diagnostic, error)` that returns an empty `reportv2.Report{}` with no diagnostics and no error. This keeps the build green so the red-first failure surfaces at the harness layer, not at `go build`. Phase 2 replaces the body with the real SSA-backed implementation — do not design the final signature here; Phase 2 owns that. Add a `// TODO(SPRINT-0006-phase-2): real SSA extraction` marker on the function.
- [x] Run `MONOLIFT_E2E=1 make e2e`; confirm Caddy + Pocketbase rows now fail at stage ≥3 because the empty report is rejected by the harness — at `reportv2` parse/validation (e.g., missing `schemaVersion`), at golden comparison, or at any subsequent normative-subset check. Any of these is a valid red-first signature: the real compiler path is reached and produces nothing useful. Record the observed failure signature verbatim in sprint notes; Phase 7 references it as the "before" state.
  Red-first signature (2026-04-20): `MONOLIFT_E2E=1 make e2e` failed at stage 3 for both real-compiler targets with the empty-report validation path: `[stage=3 target=caddy kind=compiler] compile exit=0 verdict=got_missing want_accept stderr: : reportv2: schemaVersion="" want "1.0"` and `[stage=3 target=pocketbase kind=compiler] compile exit=0 verdict=got_missing want_refuse-blocking stderr: : reportv2: schemaVersion="" want "1.0"`.

### Phase 2 — Shared v2 analysis substrate

- [x] Design and land the v2 extraction entrypoint. Signature finalized after the Phase 0 caller inventory; candidate shape: `Extract(sources []string, pragmas []*compiler.Pragma) (reportv2.Report, []compiler.Diagnostic, error)`. It consumes SPRINT-0005 parser output — does **not** re-parse comments, does **not** invoke any v1 codepath.
  Landed `compiler.Extract(sources, pragmas)` as the public facade over new `pkg/compiler/extract.Analyze(Request)`. The facade converts parser-owned `compiler.Pragma` values into subpackage request data and does not re-parse comments or route through any v1 compiler path. Added source-level v2 pragmas at the real Caddy and Pocketbase roots so `compiler.Parse` now discovers one root pragma in each evaluation module (`caddy-reverse-proxy` on `reverseproxy.Handler`, `pocketbase-app` on `core.App`).
- [x] Implement a deterministic package loader around `packages.Load` with `LoadAllSyntax` + `NeedModule` + needs for types/syntax/imports. Scope to the module reachable from the annotated root (module root resolved from `go.mod` of the root's file).
  Added `pkg/compiler/extract/loader.go`: it selects the single parsed root pragma, resolves the annotated file to an absolute path, walks upward to the nearest `go.mod`, and loads that module with `packages.LoadAllSyntax | packages.NeedModule` from the module root. The loader also identifies the package containing the annotated file for later root resolution. `pkg/compiler/extract/loader_test.go` covers the module-root scoping path against `pkg/compiler/testdata/ssaspike`.
- [x] Propagate environment into loader config: `CGO_ENABLED`, `GOOS`, `GOARCH`, and any project build tags. Caddy and Pocketbase both touch CGO-adjacent code; missing this breaks Phase 3.
  `pkg/compiler/extract/loader.go` now pins `GOOS`, `GOARCH`, and `CGO_ENABLED` in `packages.Config.Env` and parses `GOFLAGS=-tags=...` into deterministic `BuildFlags`. The loader records the selected tuple (`GOOS`, `GOARCH`, `CGO_ENABLED`, sorted build tags) for later report assembly. `TestLoadModulePropagatesEnvAndBuildTags` asserts the selected `ssaspike` compiled file set flips to `tagged_on.go` / `cgo_on.go` under `GOFLAGS=-tags=monoliftspike` and `CGO_ENABLED=1`.
- [x] Build an SSA program via `ssa.AllPackages` with `BuilderMode` configured for callgraph-ready IR. Record the chosen `BuilderMode` flags in the loader's exported doc.
  `pkg/compiler/extract/ssa.go` now builds SSA with `ssautil.AllPackages(..., ssa.InstantiateGenerics)` and calls `Program.Build()`. The `Analyze` doc records that choice explicitly as the callgraph-ready builder mode for SPRINT-0006. `TestBuildProgramProducesCallgraphReadySSA` asserts the loader-built program yields a non-empty CHA callgraph.
- [x] Populate `reportv2.Analysis.Algorithm` with a stable string such as `"ssa-cha+rta"`; set `Analysis.Deterministic = true`; populate `reportv2.BuildConfig.ModuleRoot` from the module root of the root declaration.
  `pkg/compiler/extract/buildSeedReport` now emits a schema-valid seed report with `analysis.algorithm="ssa-cha+rta"`, `analysis.deterministic=true`, and `buildConfig.moduleRoot` derived from the loader-resolved module root. The seed report also carries the selected build tuple and root pragma metadata so later closure/refusal work extends a valid report instead of replacing an empty placeholder. `TestAnalyzeBuildsDeterministicMetadataReport` validates the resulting JSON through `reportv2.Validate`.
- [x] Unit tests against small in-repo fixtures under `pkg/compiler/testdata/` covering: load-error propagation; SSA build on a trivial package; missing-root error; multi-file root package; build-tag/CGO propagation sanity check.
  `pkg/compiler/extract/loader_test.go` now covers missing-root rejection, `packages.Load` error propagation from `pkg/compiler/testdata/loaderror`, multi-file root-package loading via `ssaspike`, SSA construction via a non-empty CHA callgraph, and build-tag/CGO propagation. The `pkg/compiler/testdata/loaderror/` fixture was added solely for the loader-error path.

### Phase 3 — Root resolution + SSA closure extraction (Caddy)

- [x] Root resolution: given a parsed `Pragma` + its attached `*ast.Decl` identity, produce a `reportv2.SymbolIdentity` + `reportv2.Root`. Source `RegistryKey` from `options["registry"]` when present; source `ExposedOperations` from `options["methods"]` filter for interface and struct surfaces.
  `pkg/compiler/extract/resolveRoot` now derives `root.identity` from the parsed declaration name/surface plus the loaded package/module, copies `registry=` into `root.registryKey`, and resolves `methods=` against the loaded type information for struct and interface roots. `pkg/compiler/testdata/rootresolve/` plus the new `Analyze` tests cover `Handler -> (*Handler).ServeHTTP` and `App -> App.Run`.
- [x] Implement a conservative SSA closure walker: starts at the root's SSA function(s); walks direct calls, static method calls, resolved interface calls, captured function values, reachable types, and package-level vars/consts needed by reachable code.
  Added `pkg/compiler/extract/closure.go`. The walker seeds from resolved root functions, traverses SSA direct calls plus invoke edges, follows anonymous closures and function values, records referenced globals, conservatively adds package constants per visited package, and emits reachable named types. `pkg/compiler/testdata/closurewalk/` plus `TestAnalyzeWalksClosureAcrossCallsClosuresAndGlobals` cover direct calls, interface dispatch, captured closure bodies, globals, and constants in a small local module.
- [x] Interface-call resolution via `golang.org/x/tools/go/callgraph/cha` and `.../rta`. Default to CHA; apply RTA-refinement at registry-keyed roots to narrow the assignable set. Explicitly acknowledge (in Phase 4) that a dynamic-plugin-like site can legitimately be finite when a registry key + the compiled package graph render it so.
  `pkg/compiler/extract/dispatchGraph` now uses CHA for ordinary roots and swaps invoke-edge resolution to `rta.Analyze(roots, true).CallGraph` when `root.registryKey` is present. The closure fixture now contains an uninstantiated extra interface implementer so `TestAnalyzeUsesCHADefaultForInterfaceDispatch` proves CHA’s wider edge set, while `TestAnalyzeRefinesRegistryDispatchWithRTA` proves the registry-keyed RTA path excludes the unused implementer.
- [x] Record precision triggers in `reportv2.Analysis.PrecisionTriggers` deterministically (sorted, stable identifiers): e.g., `registry-key:<key>`, `dispatch-growth:<iface>`, `rta-escape:<fn>`.
  The closure walker now returns a sorted precision-trigger set. It records `registry-key:<key>` when registry refinement is active, `dispatch-growth:<pkg>:<caller>:<method>` when an invoke site fans out across multiple callees in the selected dispatch graph, and `rta-escape:<fn>` when registry-keyed RTA narrows a wider CHA site. The new closure tests assert exact trigger ordering for both CHA-default and registry-refined runs.
- [x] Prune stdlib + external-module edges per §Closure Rules EC-TERM-*. Include external edges in `reportv2.ExternalDeps` with `AccessPath` + `ConfigurationSource` populated where derivable; leave `StateEffectSummary` empty unless the Caddy golden requires it in this sprint.
  External/static edges are now pruned out of `closure.includedSymbols` and recorded in `report.externalDependencies` instead. `externalDepForEntry` emits stable `accessPath` values as `<package>.<object>`, keeps `stateEffectSummary` empty, and records `configurationSource=registry:<key>` when the root is registry-keyed. `TestAnalyzePrunesStdlibCallsIntoExternalDeps` verifies a stdlib edge (`strings.ToUpper`) is excluded from the closure body and preserved as an external dependency.
- [x] Populate `reportv2.Closure.IncludedSymbols`, `ExcludedSymbols`, and `WiringPaths` in stable (package-path, object-name) sorted order. Closure body may be richer than the harness normative subset — determinism matters more than minimality.
- [x] Deterministic-ordering unit tests: assert that `IncludedSymbols`, `ExcludedSymbols`, `WiringPaths`, `PrecisionTriggers`, and the diagnostics slice are emitted in the same order across repeat runs on identical input.
  Added `TestAnalyzeEmitsDeterministicOrderingAcrossRuns`, which invokes `Analyze` twice on the same registry-keyed closure fixture and `DeepEqual`s `includedSymbols`, `excludedSymbols`, `wiringPaths`, `precisionTriggers`, and `diagnostics`. This locks the current sort keys into an executable regression check.
- [x] Compiler-level integration test: run extraction against Caddy's `modules/caddyhttp/reverseproxy` (or the exact root the target references) and assert (a) non-empty `root` with the expected identity, (b) `closure.IncludedSymbols` > 0, (c) at least one `externalDependencies` entry, (d) passes `reportv2.Validate`. This test is deliberately stricter than the harness normative subset so a fake/empty closure cannot slip through.
  Added `pkg/compiler/extract_integration_test.go::TestExtractCaddyReverseProxyProducesNonEmptyValidatedReport`, which exercises the public `Parse -> Extract` seam against `evaluation/caddy`. It asserts the exact reverseproxy root identity, non-empty `closure.includedSymbols`, non-empty `externalDependencies`, and successful `reportv2.Validate`.
- [x] Retire `test/e2e/stubcompiler/fixtures/caddy/closure-report.json` (delete file; remove any remaining references).
  Deleted `test/e2e/stubcompiler/fixtures/caddy/closure-report.json`. `test/e2e/stubcompiler/stubcompiler_test.go` now passes Caddy (and Pocketbase) real-source roots to `go run .` so the stubcompiler validation suite no longer assumes those targets are fixture-backed.
- [x] `MONOLIFT_E2E=1 make e2e` — Caddy stage 10 green against real compiler output.

### Phase 4 — Unbounded-edge refusals + precision ADR

- [x] Detect unresolved reflection-driven dispatch in SSA (e.g., `reflect.Value.Call`, `reflect.MethodByName`) on non-registry sites. Refuse with the Phase-0-selected code.
- [x] Detect `unsafe.Pointer` / `unsafe`-mediated edges that would cross the extraction boundary. Refuse with the Phase-0-selected code.
- [x] Detect dynamic plugin loads (`plugin.Open`, `plugin.Lookup` with non-constant args). Refuse with `MLV2_DYNAMIC_PLUGIN` (+ umbrella code if Phase 0 chose option (a)), **unless** `registry=` plus the compiled package graph render the implementation set finite — in which case treat as a resolved registry site and widen closure accordingly.
- [x] Caddy must **not** trip any of these refusals on the reverse-proxy root in its current form — gate via the Phase 3 integration test.
- [x] Focused unit tests per unbounded-edge case under `pkg/compiler/testdata/`: tiny Go packages exercising each path; assert exact refusal code + severity.
- [x] Finalize `docs/decisions/0013-ssa-closure-precision.md`: Decision = CHA + RTA-refined dispatch for SPRINT-0006; Consequences cover determinism guarantees, expected false-positive classes (high-fanout interfaces), and the explicit deferral of VTA + pointer analysis with precision-vs-cost rationale. Phase 0's call-graph complexity probe data informs this section.
- [x] Add a "Future work — SSA precision" subsection to this sprint file's closeout referencing ADR-0013 and listing the concrete next-step experiment (measure VTA false-positive reduction on Caddy + Pocketbase before broadening).

### Phase 5 — Compiler-owned refusal-diagnostic framework

- [x] Create the new diagnostics package (name finalized in Phase 0) with the translation surface: `func Translate(d compiler.Diagnostic) reportv2.Diagnostic` (or an equivalent batch form). Package doc states this is the **only** place the `compiler.Diagnostic → reportv2.Diagnostic` seam lives.
- [x] Move span formatting into the new package: file-relative path rebased against `BuildConfig.ModuleRoot`, byte offsets derived from `token.Pos` via the SSA program's `FileSet`, `LineStart`/`LineEnd`. Parser spans from SPRINT-0005 already carry file + line; extend to byte offsets at this seam.
- [x] Remediation text: one canonical map keyed by diagnostic code → remediation string template. **Fail-fast on unknown codes**: codes without a registered template cause a typed error at translation time. Do not emit empty-remediation diagnostics silently.
- [x] Rule-ID population: codes map to their `RuleIDs` per spec (e.g., `MLV2_EMBEDDED_DB_APP_ROOT` → `["SS-LIFT-6"]`, `MLV2_CLOSURE_TOO_LARGE` → `["EC-PRUNE-3"]`). Table lives inside the new package.
- [x] Rewrite `test/e2e/stubcompiler/main.go::toReportDiagnostics` and `toReportSpan` to delegate to the new package. The stub compiler keeps orchestration but owns **zero** translation rules.
- [x] **Import-boundary guard.** The new diagnostics package does **not** import `pkg/compiler/pragma*` (avoid circular dependency). Parser diagnostics flow through callers that construct `compiler.Diagnostic` values before handing them to `Translate`. Add a mechanical test (build tag or `go vet`-style) that fails on import violations, matching the ADR-0012 precedent.
- [x] Unit tests: span-byte-offset math on a multibyte UTF-8 fixture (assert correct byte offsets for non-ASCII content); unknown-code translation must error; each implemented code's full round-trip including remediation text and rule IDs.
- [x] **Pragma-row regression check.** After the seam move, run the full SPRINT-0005 pragma fixture suite — all seven `MLV2_PRAGMA_*` rows must remain green. This catches parser-diagnostic translation regressions introduced by the move.

### Phase 6 — Pocketbase refusal flip

- [x] Detect the Pocketbase refusal conditions compiler-side from SSA + closure output:
  - [x] `MLV2_EMBEDDED_DB_APP_ROOT`: root is an interface/struct whose closure directly owns an embedded SQLite handle. Keep detection narrow — this sprint only needs Pocketbase to match; a generalized state-class inference is SPRINT-0007+ work. Add a code comment naming the canonical-shape/state-class epic that owns the general form.
  - [x] `MLV2_CLOSURE_TOO_LARGE`: closure bounding fails. Calibrate the threshold against Pocketbase's actual measured closure size (not an arbitrary guess — record the measurement in a code comment); the threshold must be high enough to avoid false positives on Caddy and low enough to catch Pocketbase.
- [x] Emit refusals as `compiler.Diagnostic` values that flow through the new diagnostics package. Verify the resulting `reportv2.Diagnostic` output matches the Pocketbase golden's `code`, `severity`, `ruleIds`, `message`, and `remediation`.
- [x] **State-row reproduction.** The Pocketbase golden expects a `shared-mutable-across-callers` state disposition for `BaseApp.db`. Emit this row via narrow Pocketbase-shaped detection (hardcoded recognition of the SQLite handle on `BaseApp`). Do not build a general state-class inference pass — name the SPRINT-0007+ epic in a code comment.
- [x] Preserve the Pocketbase stage-4 golden shape (`test/e2e/targets/pocketbase/golden/report.json`); update only fields that legitimately change once the real compiler becomes the producer, and flag any such change for reviewer sign-off.
- [x] Retire `test/e2e/stubcompiler/fixtures/pocketbase/closure-report.json` (delete file; remove references).
- [x] Compiler-level integration test: extraction against Pocketbase's `core` package refuses with both `MLV2_EMBEDDED_DB_APP_ROOT` and `MLV2_CLOSURE_TOO_LARGE`, failing for those reasons specifically (not parser or loader failures).
- [x] `MONOLIFT_E2E=1 make e2e` — Pocketbase stage 4 green against real compiler output.

### Phase 7 — Verification, cleanup, closeout

- [x] `go test ./pkg/compiler/...` green (includes new extraction, SSA, unbounded-edge, diagnostics, and ordering tests).
- [x] `go test ./test/e2e/stubcompiler ./test/e2e/harness ./test/e2e/...` green.
- [x] `MONOLIFT_E2E=1 make e2e`: Caddy green @ stage 10 (real compiler), Pocketbase green @ stage 4 (real compiler), SPRINT-0005 pragma rows green, deferred targets still skipped. If Kind unavailable, record the reason and log unit-level verification commands.
- [x] Measure wall-time delta introduced by `LoadAllSyntax` + SSA build on Caddy + Pocketbase vs. the Phase 0 baseline. If the regression exceeds an acceptable budget, record as a follow-up (not a blocker); SPRINT-0007+ owns optimization.
  Current direct compiler-path timings (`/usr/bin/time -p ./bin/stubcompiler ...`): Caddy `real 21.83s`, Pocketbase `real 19.80s`. Task 78 recorded the pre-sprint command outcomes but not comparable wall-clock numbers, so an exact delta is unavailable; keep optimization as a SPRINT-0007+ follow-up rather than a blocker.
- [x] Grep checks:
  - [x] `rg -n "stubcompiler/fixtures/caddy|stubcompiler/fixtures/pocketbase"` returns no live-code hits (only possibly this sprint doc).
  - [x] `rg -n "toReportDiagnostic|toReportSpan" test/e2e/stubcompiler` returns nothing — translation helpers fully migrated.
  - [x] `rg -n "<new-package>/" pkg/compiler/pragma` returns nothing — import-boundary preserved.
- [x] Finalize ADR-0013 (Decision + Consequences complete, including probe data from Phase 0).
- [x] Append a closeout entry to `docs/evolution.md`: Caddy SSA flip, Pocketbase refusal-framework flip, new diagnostics package, ADR-0013 (SSA precision + VTA deferral), ADR-0014 (unbounded-edge refusal code taxonomy).
- [x] Remove dead stub-only code paths and comments that still claim `test/e2e/stubcompiler/main.go` owns diagnostic translation for Caddy or Pocketbase.
- [x] Append a `## SPRINT-0007 Seed Epics` section to the bottom of this sprint file listing: canonical-shape classifier + per-shape adapter templates; state-class inference + singleton/affinity codegen; VTA precision experiment (measure false-positive reduction on Caddy + Pocketbase); remaining `MLV2_*` refusal-code coverage; Miniflux unskip; v1 demo repair-or-retire decision.
- [x] Do **not** modify `docs/sprints/ledger.yaml` from within sprint work; ledger status transitions run separately via `.claude/skills/sprint-planner/scripts/ledger.py`.

---

## Sequencing

Strict: **Phase 0 → Phase 1 → Phase 2 → {Phase 3, Phase 5 parallel} → {Phase 4, Phase 6} → Phase 7.**

- Phase 0 must complete before any unbounded-refusal code is emitted — contract alignment for the Phase-0-selected code is a hard prerequisite. The Caddy complexity probe also lands here so Phase 2's algorithm choice is informed.
- Phase 1 is the ADR-0011 red-first gate: Caddy + Pocketbase fail against real-compiler expectations before compiler code lands.
- Phase 2 is shared substrate; both Phase 3 (Caddy acceptance) and Phase 5 (diagnostics package) depend on it. In particular, `BuildConfig.ModuleRoot` and the SSA `FileSet` from Phase 2 are required for Phase 5's span formatting.
- Phase 3 and Phase 5 can run in parallel once Phase 2 exists. Caddy acceptance and diagnostic translation are largely independent workstreams.
- Phase 4 (unbounded refusals) depends on Phase 2 substrate and feeds both Phase 3 (Caddy must not trip) and Phase 6 (Pocketbase refusals flow through the framework). Do **not** sequence Phase 4 strictly after Phase 3 — refusal semantics shape the closure walker, not the other way around.
- Phase 6 depends on Phase 5 (diagnostic framework) + Phase 4 (detection primitives). Pocketbase cannot flip off the stub until both are in place.
- Phase 7 is closeout only after both flips are green.

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| CHA over-approximation destabilizes Caddy closure (high interface fanout) | RTA-refined dispatch on registry-keyed roots; Phase 0 call-graph complexity probe informs algorithm choice; precision triggers recorded for debugging; scope acceptance to the current Caddy root, not general multi-root |
| Harness normative-subset is lax enough that a fake/empty closure passes e2e | Phase 3 compiler-level integration test asserts non-empty root/closure/deps directly against the real report, not just harness output |
| Phase-0 refusal code emitted before spec alignment | ADR-0014 + spec amendment (if option (a)) lands before any Phase 4 code |
| Pocketbase refusal detection creeps into general state-class inference | Narrow, hardcoded Pocketbase-shaped detection; threshold calibrated by measurement; code comments name the SPRINT-0007+ epic that owns the general form |
| SSA build fails on Caddy / Pocketbase due to build tags / cgo / vendor layout | Phase 0 SSA spike verifies tooling + env propagation; Phase 2 loader threads `CGO_ENABLED` + build tags; Phase 2 unit tests catch regressions before Phase 3 |
| `packages.LoadAllSyntax` wall-time regression exceeds e2e budget | Scope loading to the module reachable from the root; Phase 7 measures delta; record as follow-up, not a blocker, if over budget |
| Diagnostic-seam move regresses SPRINT-0005 pragma rows | Phase 5 explicit pragma-row regression task re-runs all seven parser fixtures; Phase 5 unit tests cover translation for parser diagnostics |
| Byte-offset math wrong for multibyte UTF-8 | Phase 5 unit test with UTF-8 content exercises byte-offset arithmetic directly (not just `token.FileSet` usage) |
| Nondeterministic ordering trips golden comparison | Phase 3 deterministic-ordering unit test; all emitted slices sorted in stable key order; `Analysis.Deterministic = true` asserted |
| VTA / pointer-analysis work creeps into sprint | Explicitly prohibited by non-goals + ADR-0013 records deferral; future-work section lists the concrete follow-up experiment |
| Retiring stub fixtures breaks other stub-backed targets | Only Caddy + Pocketbase fixture subdirs are deleted; stubcompiler's copyTree path continues to serve Miniflux/Listmonk/Gitea/Mattermost |
| New API shape (extraction entrypoint, translate function) hardens before inventory complete | Phase 0 caller inventory precedes Phase 2 API landing; candidate signatures in this plan are explicitly non-binding |
| Naming decision (new extraction + diagnostics packages) locks in ambiguity | Phase 0 finalizes names after inventory; `pkg/compiler/ssa/` and `diagv2` disallowed |
| Arbitrary closure-size threshold for `MLV2_CLOSURE_TOO_LARGE` produces flaky refusal | Phase 6 calibrates threshold against measured Pocketbase closure; measurement recorded in code comment |

## Future work — SSA precision

ADR-0013 locks SPRINT-0006 to CHA as the default dispatch approximation with
RTA refinement only for registry-keyed roots. That deferral is intentional, not
an open-ended placeholder.

The next precision experiment is concrete: measure VTA false-positive reduction
on the current Caddy and Pocketbase roots, compare it against the shipped
`ssa-cha+rta` baseline, and only then decide whether a broader pointer-sensitive
mode is worth the added runtime and tuning cost.

## Acceptance criteria

- [x] Caddy e2e passes stages 0–10 without `test/e2e/stubcompiler/fixtures/caddy/closure-report.json` existing.
- [x] Pocketbase e2e passes stages 0–4 without `test/e2e/stubcompiler/fixtures/pocketbase/closure-report.json` existing.
- [x] `pkg/compiler` exposes a v2 extraction entrypoint that consumes SPRINT-0005 parser output and returns a populated `reportv2.Report`.
- [x] Caddy's `root`, `closure.includedSymbols`, and `externalDependencies` are produced by real SSA analysis, verified by a compiler-level integration test stricter than the harness normative subset.
- [x] Interface-call resolution uses CHA + RTA-refined dispatch; `reportv2.Analysis.Algorithm` reflects the chosen algorithm; `PrecisionTriggers` are recorded in deterministic order.
- [x] Unresolved reflection / `unsafe` / dynamic plugin edges (non-registry) produce a refusal diagnostic with the Phase-0-selected code. Registry-keyed plugin sites with a finite implementation set are accepted.
- [x] The new diagnostics package (name finalized in Phase 0) owns the `compiler.Diagnostic → reportv2.Diagnostic` seam; `test/e2e/stubcompiler/main.go` contains zero translation logic; import boundary vs. `pkg/compiler/pragma*` is mechanically enforced.
- [x] Unknown diagnostic codes at translation time return a typed error (fail-fast).
- [x] Diagnostic spans carry file-relative paths (rebased against `BuildConfig.ModuleRoot`), byte offsets, line ranges, and remediation text; UTF-8 byte-offset math verified by unit test.
- [x] Pocketbase is refused by the real compiler with `MLV2_EMBEDDED_DB_APP_ROOT` + `MLV2_CLOSURE_TOO_LARGE`; the `shared-mutable-across-callers` state row is reproduced.
- [x] SPRINT-0005 pragma fixture suite remains green after the diagnostic-seam move.
- [x] ADR-0013 is committed with Decision + Consequences covering CHA + RTA now, VTA/pointer deferred, precision-vs-cost rationale, and Phase-0 probe data.
- [x] ADR-0014 is committed recording the unbounded-edge refusal code decision with Context, Decision, and Consequences.
- [x] If ADR-0014 added `MLV2_CLOSURE_UNBOUNDED`, `docs/specs/monolift-v2-contract.md` reflects it in §Refusal Diagnostic Index + §Extraction Root and Closure with a cross-link to ADR-0014.
- [x] `go test ./pkg/compiler/... ./test/e2e/stubcompiler ./test/e2e/harness` passes.
- [x] `MONOLIFT_E2E=1 make e2e` passes when Kind is available, or an unreachable reason is recorded.
- [x] All grep checks in Phase 7 pass.
- [x] `docs/evolution.md` closeout entry records both harness flips, the new diagnostics package, the Phase-0 contract decision, and the VTA deferral.
- [x] This sprint file ends with a `## SPRINT-0007 Seed Epics` section.

## Blockers

_(resolved — blocker noted by codex was a genuine plan contradiction. Plan updated: Phase 1 now lands a stub `compiler.Extract` returning an empty report so `go build` succeeds and red-first fires at harness stage ≥3 via golden comparison failure. See Phase 1 tasks.)_

- 2026-04-20: Phase 1 still contains a contradiction against the current harness/parser path. With the required stub `compiler.Extract(sources, pragmas)` returning `reportv2.Report{}`, `MONOLIFT_E2E=1 make e2e` does fail at stage 3 for Caddy and Pocketbase, but the failure is report parse/validation, not golden comparison. Observed signatures: `[stage=3 target=caddy kind=compiler] compile exit=0 verdict=got_missing want_accept stderr: : reportv2: schemaVersion="" want "1.0"` and `[stage=3 target=pocketbase kind=compiler] compile exit=0 verdict=got_missing want_refuse-blocking stderr: : reportv2: schemaVersion="" want "1.0"`. I did not check off the Phase 1 e2e task because the run does not reach the task's required "empty report fails golden comparison" state.
  _(resolved — plan wording over-specified "golden comparison"; both parse/validation and golden failure are valid red-first signatures. Task wording broadened 2026-04-20; proceed by recording the observed `schemaVersion=""` signature in the task's sprint note and marking the task done.)_

## SPRINT-0007 Seed Epics

- canonical-shape classifier + per-shape adapter templates
- state-class inference + singleton/affinity codegen
- VTA precision experiment (measure false-positive reduction on Caddy + Pocketbase)
- remaining `MLV2_*` refusal-code coverage
- Miniflux unskip
- v1 demo repair-or-retire decision
