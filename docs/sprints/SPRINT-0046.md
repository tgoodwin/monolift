# SPRINT-0046: Activation pipeline profiling, optimization & second-lift proof

**Status:** planned
**Predecessors:** SPRINT-0043 (reverse-import scoping & 6-project activation-path coverage)

## Intent

Profile and optimize the activation-path compiler pipeline end-to-end. The primary benchmark is Mattermost, where SPRINT-0043 showed ~498s total dominated by augmentation. Instrument all pipeline phases at sub-pass granularity, establish a reproducible baseline across all 6 corpus projects, then implement data-driven optimizations — targeting at least 2x speedup on the dominant compiler-side cost. Additionally, prove multi-lift capability by implementing a second lift target for miniflux within the same package as the existing `SanitizeHTML` target.

## Scope boundaries

**In scope:**
- Sub-phase instrumentation within `Augment()` and across the full pipeline (codegen, patching, verification)
- Benchmark script and structured timing report across all 6 corpus projects
- Augment-phase optimizations: index caching across iterations, mapfunc index reuse, propagation worklist, ExploreCallees dedup, early termination
- Data-gated scope caching and Docker layer optimization
- A second activation-path e2e target for miniflux (multi-lift proof)

**Out of scope:**
- gRPC, streaming, multi-cut, Helm/Kustomize
- Receiver-method lift support
- Non-primitive boundary types beyond current codec
- CI/CD integration or production deployment
- Parallelizing pipeline phases across goroutines
- On-disk SSA serialization (high risk, low ROI this sprint)
- Modifying activation edge taxonomy, cut-placement logic, or HTTP/JSON server/client protocol
- Broadening the corpus beyond the 6 existing projects

## Concrete targets

### Optimization benchmark

| # | Project | Function | File:line | Current total | Bottleneck |
|---|---------|----------|-----------|--------------|------------|
| 1 | mattermost | `GeneratePublicLinkHash` | `channels/app/file.go:588` | ~498s | augment (~490s) |

### Second-lift target (multi-lift proof)

| # | Project | Function | File:line | Signature | Why |
|---|---------|----------|-----------|-----------|-----|
| 2 | miniflux | `HasValidURIScheme` | `internal/reader/sanitizer/sanitizer.go:258` | `(rawURL string) bool` | Same package as existing `SanitizeHTML` — exercises closest-proximity multi-lift (shared imports, namespace, co-deployment). Pure function, primitive params/return, many callers in sanitizer pipeline. |

## Task list

### Phase 0: Instrumentation and baseline

Instrument the pipeline at sub-pass granularity and establish a reproducible timing baseline before any optimization.

- [x] 0.1: Add `SubTimings []PhaseTiming` to `activation.Result` for per-sub-pass durations within augment
- [x] 0.2: Instrument each call in `runAugmentationPasses()` with `timePhase` wrappers: `AugmentStructField`, `ApplyPredicates`, `AugmentGoroutine`, `AugmentPackageVars`, `AugmentFuncArgs`, `AugmentMapFuncValues`, `AugmentInterfaceFields`, `ExploreCallees`
- [x] 0.3: Split hidden SSA construction by explicitly timing `Program.BuildSSA()` as an `ssa` phase after `load`, so later phases don't absorb first-call SSA build cost
- [x] 0.4: Add augmentation counters to timing metadata: total SSA functions, graph functions, scanned instructions, added nodes/edges, iteration count, limit-hit status
- [x] 0.5: Add `codegen.LiftResult.Timings` with phases for `activation`, `cut`, `extract-report`, `admit-cut`, `build-plan`, `render-server`, `render-client`, `render-dockerfiles`, `render-kubernetes`, `write-artifacts`, `patch-function`, `patched-package-verify`
- [x] 0.6: Split `PatchCutFunction` timing so `load-cut-package`, `rename-func`, `write-patched-file`, `write-client-stub`, and `verify-patched-build` are visible separately
- [x] 0.7: Add a `--profile` flag to `cmd/activation-path` CLI that emits a structured JSON timing report (phase timings + augment sub-timings + graph stats) to stdout or a file
- [x] 0.8: Create `scripts/benchmark_corpus.sh` that runs `cmd/activation-path --profile` against all 6 corpus targets and produces a Markdown summary table
- [x] 0.9: Record machine and cache state in baseline: Go version, OS/arch, Docker version, `GOCACHE`, module cache warmth
- [x] 0.10: Run the benchmark script and save baseline to `docs/research/runs/SPRINT-0046-baseline.md`
- [x] 0.11: For Mattermost, record per-augment-iteration sub-pass timings to identify whether cost is iteration count vs. per-iteration scan cost
- [x] 0.12: Rank bottlenecks by project and phase; identify top 3 Mattermost costs. Write a decision note choosing the optimization order from measured data

### Phase 1: Augment-phase optimizations

Attack the dominant cost centers identified in the baseline. The augment phase runs up to 10 fixed-point iterations, each re-scanning the full SSA program.

#### 1A: Cached SSA function list

- [x] 1A.1: Add a cached sorted SSA function list to `activation.Program` — `Program.Functions() []*ssa.Function` — computed once after `BuildSSA()`
- [x] 1A.2: Replace all calls to `sortedFunctions(program.SSAProgram)` in resolve.go, entrypoints.go, explore.go, structfield.go, pkgvar.go, mapfunc.go, interfacefield.go with `program.Functions()`
- [x] 1A.3: Regression test: repeated `program.Functions()` calls return deterministic order and do not rescan `ssautil.AllFunctions`

#### 1B: Struct-field index caching

- [x] 1B.1: Refactor `AugmentStructField` to accept an optional `*StructFieldIndex`. When non-nil, skip the full-program scan and only scan functions discovered since the last iteration
- [x] 1B.2: Add `UpdateStructFieldIndex(index *StructFieldIndex, newFuncs []*ssa.Function)` for incremental store/read discovery
- [x] 1B.3: Thread the `StructFieldIndex` through the augment loop in `Augment()` so it persists across iterations
- [x] 1B.4: Unit test: verify incremental index matches full-rebuild index on a test module with 3 augment iterations

#### 1C: MapFuncValues index reuse

- [x] 1C.1: Refactor `AugmentMapFuncValues` to return the computed `mapFuncIndex`
- [x] 1C.2: Refactor `AugmentInterfaceFields` to accept a `mapFuncIndex` parameter instead of calling `mapFactoryResultTypes()` internally
- [x] 1C.3: Thread the `mapFuncIndex` from `AugmentMapFuncValues` to `AugmentInterfaceFields` in `runAugmentationPasses()`
- [x] 1C.4: Add incremental propagation: on subsequent iterations, only scan new functions for `MapUpdate` instructions
- [x] 1C.5: Unit test: verify shared index produces identical edges to rebuild-per-pass

#### 1D: Propagation loop optimization

- [x] 1D.1: Replace `propagateMapParamStores` full-function rescans with a worklist over callsites keyed by static callee and changed parameter-store facts
- [x] 1D.2: Add a callsite index for `AugmentFuncArgs` so callback propagation does not rescan every function body per iteration
- [x] 1D.3: Track a propagation iteration cap (20) with a diagnostic warning if hit
- [x] 1D.4: Benchmark the propagation loop in isolation on Mattermost — record iteration count, functions scanned per iteration, new stores found per iteration

#### 1E: ExploreCallees deduplication

- [x] 1E.1: Track a global `exploredRoots` set across augment iterations. Skip roots already explored in a prior iteration
- [x] 1E.2: Verify edge coverage: confirm no edges are dropped by deduplication using Mattermost as validation

#### 1F: Augment early termination

- [x] 1F.1: After `BuildRTAGraph`, run a fast BFS reachability check for the target function. If found, skip the augment phase and proceed directly to path extraction
- [x] 1F.2: Add `SkippedAugment bool` to `Result` and a diagnostic noting the skip
- [x] 1F.3: Test: verify targets reachable via pure RTA (e.g., caddy `CleanPath`) skip augmentation and produce the same path and recommended cut
- [x] 1F.4: Test: verify targets requiring augmentation (e.g., mattermost) still enter the augment loop
- [x] 1F.5: If early-termination path differs in cut quality from the augmented path for any existing target, make early termination opt-in rather than default

#### 1G: Correctness verification

- [x] 1G.1: Preserve deterministic edge ordering after cached indexes by sorting keys and emitted edges with existing `FunctionKey` and source-position comparators
- [x] 1G.2: Add unit tests for each cached augmentation pass using existing fixtures proving edge sets are identical before and after cache reuse
- [x] 1G.3: Run `Augment` twice on the same loaded fixture and assert no duplicate edges and stable graph stats
- [x] 1G.4: Run `go test ./pkg/activation/...` after all Phase 1 changes

### Phase 2: Post-optimization benchmarking

- [x] 2.1: Run Mattermost activation-only profiling after Phase 1 optimizations; store as `docs/research/runs/SPRINT-0046-mattermost-optimized.json`
- [x] 2.2: Require every comparison to include `scope`, `load`, `ssa`, `rta`, total `augment`, each augment subphase, `bfs`, graph nodes/edges, found path length, recommended cut step
- [x] 2.3: Verify Mattermost found target remains `GeneratePublicLinkHash` at `channels/app/file.go:588` and direct invocation result matches oracle
- [x] 2.4: Re-run `scripts/benchmark_corpus.sh` across all 6 projects after optimizations; save to `docs/research/runs/SPRINT-0046-optimized.md`
- [x] 2.5: Produce a comparison table: baseline vs. optimized per phase per project
- [x] 2.6: Flag any project whose activation-only time regresses by more than 10% and either fix or document with profiler evidence
- [x] 2.7: Verify graph stats, path length, target key, and cut step remain unchanged for all 6 existing targets

### Phase 3: Data-gated secondary optimizations

Implement only if Phase 0 baseline data shows these costs are material.

- [x] 3.1: Measure whether `ReverseImportScope` time exceeds 5% of total for any project. If not, record the decision and skip scope caching this sprint
- [x] 3.2: If scope is material for repeated analyses of the same module, add a per-process reverse-import inventory cache keyed by `(moduleRoot, GOWORK, go.mod hash, go.sum hash, env, buildFlags)`. Invalidation test: cache misses when `go.mod` or source imports change
- [x] 3.3: Measure `patched-package-verify` cost. If it dominates for Mattermost or Gitea, add an option to verify only the target package plus direct reverse-import roots, with a negative test proving it catches compile failures
- [x] 3.4: Measure Docker build cost. If generated Docker builds are material, add BuildKit cache mounts for Go build and module caches in host/extracted/oracle Dockerfiles. Add golden tests for cache-mount rendering

### Phase 4: Multi-lift proof — miniflux second target

Prove the pipeline supports multiple extracted services from a single corpus project. Can start after Phase 0 baseline confirms existing miniflux target still passes.

- [x] 4.1: Verify `HasValidURIScheme` at `internal/reader/sanitizer/sanitizer.go:258` is a package-level function with signature `(rawURL string) bool` — no receiver, single return
- [x] 4.2: Verify `HasValidURIScheme` has production callsites reachable from an HTTP handler in the sanitizer pipeline via activation-path analysis with reverse-import scoping
- [x] 4.3: If `HasValidURIScheme` is not reachable under the activation algorithm, choose a replacement from the sanitizer package (e.g., `isValidTag`) and document the rejection reason
- [x] 4.4: Create `test/e2e/targets/activation_miniflux_validurischeme/target.go`. Name: `activation-miniflux-validurischeme`. Target: `internal/reader/sanitizer/sanitizer.go:258`. Source dirs: `["evaluation/miniflux"]`. Deploy options: same postgres fixture as `activation-miniflux-sanitizehtml`, host port 8080, readiness `/healthcheck`
- [x] 4.5: Create `workload.go` — exercise a miniflux RSS feed processing path that calls `HasValidURIScheme` (subscribe to feed with mixed URI schemes in entry content)
- [x] 4.6: Create `oracle.go` — directly call `sanitizer.HasValidURIScheme(rawURL)` for invocation comparison
- [x] 4.7: Register in `e2e_test.go` as the 7th activation target
- [x] 4.8: Run focused Kind e2e for `activation-miniflux-validurischeme` — all stages pass
- [x] 4.9: Run both miniflux targets together: verify no interference (independent namespaces, independent extracted services)
- [x] 4.10: Verify codegen produces distinct service names, Dockerfiles, and k8s manifests for the two miniflux lifts

### Phase 5: Verification and closeout

- [x] 5.1: Run `go test ./pkg/activation/... ./pkg/codegen/...`
- [x] 5.2: Run focused Kind e2e for all 7 activation targets individually
- [ ] 5.3: Run combined activation sweep: `MONOLIFT_E2E=1 go test -tags e2e -v -run 'TestE2E/activation-' -count=1 -timeout=3h ./test/e2e/`
- [ ] 5.4: Verify all generated manifests list correct artifact kinds and deploy metadata
- [ ] 5.5: Verify all generated extracted Deployments contain no `MONOLIFT_LIFT_*` env vars
- [ ] 5.6: Verify env-off mode produces zero extracted `/calls` deltas for all 7 targets
- [ ] 5.7: Verify fail-closed and fail-open behavior after scaling each extracted service to 0 and back
- [ ] 5.8: Update `GeneratorVersion` constant from `"SPRINT-0043"` to `"SPRINT-0046"`
- [ ] 5.9: Write `docs/research/runs/SPRINT-0046-optimization-report.md` with final baseline, selected optimizations, rejected opportunities, before/after phase table, and residual bottlenecks

## Sequencing

```
Phase 0 (instrumentation + baseline) ← GATE: baseline must exist before optimizing
    │
    ├──→ Phase 1 (augment optimizations)  ← 1A-1F can proceed in parallel
    │         │                              1G verifies after all land
    │         ↓
    │    Phase 2 (post-optimization benchmark) ← GATE: verify 2x before expanding
    │         │
    │         ↓
    │    Phase 3 (data-gated secondary optimizations) ← implement only if measured
    │
    ├──→ Phase 4 (multi-lift proof) ← can start after Phase 0, independent of optimization
    │
    └──→ Phase 5 (verification + closeout) ← after Phases 1-4 complete
```

Phase 0 is the hard gate — no optimization without measured data. Phase 1 sub-tasks (1A-1F) can proceed in parallel since they touch different passes, with 1G verifying correctness after all land. Phase 4 (multi-lift) is independent of optimization work and can start as soon as the existing miniflux target is confirmed passing. Phase 3 is data-gated — implement scope/Docker/patch optimizations only if the baseline shows they matter.

## Risks

**R1: Augment may remain expensive even after index caching.** If SSA construction or RTA itself dominates rather than per-pass scanning, index caching yields modest gains. *Mitigation:* Phase 0.3 splits SSA timing; Phase 0.11 records per-iteration costs. If per-iteration cost is dominated by SSA/RTA, document the bottleneck and propose SSA scoping as future work.

**R2: Cached whole-program indexes may increase memory pressure.** Mattermost and Gitea SSA programs are large. *Mitigation:* Record peak RSS in profiling output where practical. Keep caches per-analysis-run, not global. Drop caches if memory use is problematic.

**R3: Index caching may miss edges if incremental scan logic is incomplete.** Functions with cross-package field stores could be overlooked. *Mitigation:* Phase 1G compares edge sets against full-rebuild baselines. Run Mattermost end-to-end to validate path equivalence.

**R4: Early termination may change path quality.** Skipping augmentation when the target is RTA-reachable produces a path from a sparser graph. The augmented graph may have a *shorter* path or a *better* cut. *Mitigation:* Phase 1F.3 compares paths and cuts for all existing targets. If quality differs, make early termination opt-in (1F.5).

**R5: Worklist propagation for mapfunc may change fixed-point behavior.** *Mitigation:* Phase 1D.3 caps iterations with diagnostic logging. Phase 1G.2 verifies edge-set equivalence.

**R6: miniflux `HasValidURIScheme` may not be reachable under activation analysis.** *Mitigation:* Phase 4.2 verifies reachability before implementation. Phase 4.3 defines a documented fallback target.

**R7: Multi-lift namespace collision.** Two extracted services from the same project could collide on service names, ports, or namespace resources. *Mitigation:* Phase 4.9 verifies independent namespaces. The existing harness creates per-target namespaces.

**R8: Full 7-row Kind sweep may exceed local timeouts.** *Mitigation:* Use 3h timeout. Keep focused-row acceptance as the primary correctness gate; combined sweep finds cleanup/resource interference.

## Design decisions

**D1: Sub-phase timing via existing `timePhase` generic.** Extend the existing `timePhase[T]` pattern to sub-passes within augmentation rather than introducing pprof or tracing. Keeps instrumentation lightweight, JSON-serializable, and consistent with `Result.Timings`.

**D2: Index caching is in-memory, not on-disk.** Struct-field and mapfunc indices are cached across iterations of a single `Analyze()` call, not persisted between runs. On-disk SSA caching would be complex and high-risk. In-memory caching within a single analysis addresses the primary cost: redundant re-scanning within the augment loop.

**D3: Scope and Docker optimizations are data-gated.** Phase 3 implements these only if Phase 0 baseline shows they contribute >5% of total time. This prevents premature optimization on costs that may be dwarfed by augmentation.

**D4: Early termination is default-on only if path/cut equivalence holds.** Augmentation only adds edges, so skipping it when the target is already RTA-reachable cannot prevent finding the target. But the augmented graph may offer a *shorter* path or *better* cut. Phase 1F.5 makes this conditional.

**D5: miniflux for multi-lift proof.** Miniflux is the simplest corpus project (42 scoped packages, fast analysis). The existing `SanitizeHTML` target is in the same package as `HasValidURIScheme`, exercising the closest-proximity multi-lift scenario: shared imports, namespace collision risk, and co-deployment. This is a tighter proof than two functions from different projects.

## Acceptance criteria

- [ ] Structured per-phase and per-sub-pass timing report exists for all 6 corpus projects (baseline + optimized)
- [ ] Mattermost's dominant compiler-side phase is at least 2x faster than SPRINT-0043 baseline, or the optimization report documents profiler evidence explaining why the best optimization could not meet that threshold
- [ ] Mattermost focused Kind e2e passes after optimization
- [ ] All 6 pre-existing activation rows pass focused Kind e2e after optimization
- [ ] Graph stats, path length, target key, and cut step remain unchanged for all 6 existing targets
- [ ] miniflux multi-lift: two independent extracted services (`SanitizeHTML` + `HasValidURIScheme`) pass e2e
- [ ] Both miniflux activation rows pass together in one command without interference
- [ ] All 7 activation targets pass the combined `TestE2E` sweep, or every failure is documented as a harness/resource issue with focused-pass evidence
- [ ] `go test ./pkg/activation/... ./pkg/codegen/...` passes
- [ ] Optimization report lists implemented changes, rejected candidates, measured speedups, residual bottlenecks, and reproduction commands

## Blockers

- 2026-05-10: Phase 5.3 combined activation sweep did not pass. All seven activation targets passed focused Kind e2e individually, but the combined `MONOLIFT_E2E=1 go test -tags e2e -v -run 'TestE2E/activation-' -count=1 -timeout=3h ./test/e2e/` run recorded a stage-9 failure for `activation-listmonk-sanitizeuri`: lifted `/admin/login` health check timed out during negative lifted assertions. The same sweep then hung in the `activation-mattermost-publiclinkhash` tail and was stopped after the run was already failed.
- 2026-05-10 follow-up: After `kind delete cluster --name monolift-e2e`, a clean combined-sweep rerun got past `activation-caddy-cleanpath`, both Miniflux targets, `activation-gitea-pathescapesegments`, `activation-listmonk-sanitizeuri`, and `activation-pocketbase-columnify`. The earlier Listmonk timeout did not reproduce. The rerun then stuck in `activation-mattermost-publiclinkhash` with the baseline Mattermost port-forward still active, indicating a combined-harness Mattermost baseline workload/port-forward issue rather than an activation/codegen correctness failure. The run was stopped as diminishing returns after the targeted Mattermost focused e2e had already passed.
