# SPRINT-0049: DB/SQL reconstructor runtime + admission-aware cut placement

**Status:** planned
**Predecessors:** SPRINT-0048 (shape axis — receiver policies, `(T,error)`, context/logger, streaming-bytes), SPRINT-0046 (pipeline optimization & multi-lift), SPRINT-0043 (reverse-import scoping)

## Intent

SPRINT-0048 closed the *shape* axis of activation-path codegen and lifted corpus e2e coverage from 1 to 5 traces. SPRINT-0049 opens the *reconstructor families* axis with DB/SQL as the single primary family (~15 traces blocked by it; rank #1 in the 0048 backlog). The work splits into three concrete jobs:

1. **Make existing DB/SQL scaffolding runtime-correct.** `pkg/codegen/recon.go` already has `directReconstructor` (`*sql.DB`/`*http.Client`/`*log.Logger`) and `sqlWrapperReconstructor` (any struct wrapping `*sql.DB`, including miniflux's `*storage.Storage`). `pkg/codegen/server.go:192-215` emits `sql.Open("postgres", os.Getenv("DATABASE_URL"))` plus `<pkg>.New<Type>(db)`. Admission accepts miniflux/M-1 today and the pipeline renders through `StopAtStage: 4`. The gap is the last mile: the extracted Deployment has no `env:` block (so `DATABASE_URL` never reaches the extracted pod), `serverReconstructorInit` never calls `db.PingContext`, `StateCloseLines` aren't emitted on shutdown, and `sqlWrapperReconstructor` hardcodes `ConstructorName = "New" + typeName` — which happens to match miniflux but is unverified for any other wrapper.

2. **Admission-aware cut placement, sized to unlock gitea/M-16.** The activation path for gitea/M-16 is `main → cli.Run → runChangePassword → UpdateAuth → User.SetPassword → (*PasswordHashAlgorithm).Hash → (*Argon2Hasher).HashWithSaltBytes`. `rankCutCandidates` (`pkg/activation/cut.go:165`) recommends step 5; admission then refuses with `receiver_requires_reconstruction` because `PasswordHashAlgorithm` embeds the non-serializable `PasswordSaltHasher` interface. The leaf at step 6 — `(*Argon2Hasher).HashWithSaltBytes(password string, salt []byte) string` at `modules/auth/password/hash/argon2.go:29` — is a pointer receiver whose factory `NewArgon2Hasher(config string) *Argon2Hasher` (`argon2.go:40`) is already in the receiver factory registry (`pkg/codegen/receiver.go:19-29`, registered in SPRINT-0048 §2B.4). If the recommender re-ran with the parent demoted, the leaf would be selected. **Crucially**, even after admission flips, the rendered server fails to compile because `server.go:171` emits `<Factory>()` with no args; `NewArgon2Hasher` needs a `config string`. The factory-arg extension is necessary for M-16 admission to mean "server + client rendered and compile-clean."

3. **Wire a real Postgres into e2e for at least one DB/SQL trace.** miniflux/M-1 is the natural acceptance target: baseline already deploys Postgres (`test/e2e/fixtures/postgres.yaml`) with `RUN_MIGRATIONS=1` on the host, and a single-namespace harness makes `postgres:5432` reachable from both host and extracted pods. The unknowns are env propagation, migration ordering (host migrates; extracted must not), driver linking (`_ "github.com/lib/pq"`) in the extracted module, and whether the extracted Pod can come up before Postgres is Ready.

### Design call on gitea/M-16

Pick approach **(a) admission-aware cut placement** over (b) interface receiver resolution. Reasoning:

- **Generality.** (a) is one feedback loop between activation ranking and codegen admission; it fixes M-16 and every other case where the top-ranked cut is refused for a reason a deeper admissible candidate would resolve. (b) only helps interface-embedding receivers and requires new wire-format machinery (discriminated-union codec, concrete-impl registry).
- **Evidence.** The leaf is already in the candidate set and the factory is already registered. The missing piece is the feedback path `AdmitPlan refusal → Demote(parent) → rerank → leaf`.
- **Cost.** (a) is ~200-400 LOC in `pkg/codegen` plus a small `DemoteCandidate` mutator on `*CutResult` in `pkg/activation`. (b) needs new types, a wire-format change, and server-template surgery.
- **Risk asymmetry.** (a) can be disabled with `MONOLIFT_ADMISSION_AWARE_RANK=0` and a regression test pins the 7 hand-picked targets' recommendations unchanged. (b) introduces a new boundary contract affecting every receiver-bearing lift.

(b) remains the right unlock for the ~5 traces where the interface receiver *is* the leaf and no deeper concrete step exists — deferred to SPRINT-0050+.

## Scope boundaries

**In scope:**
- Admission-aware retry-deeper in the cut→admit→re-rank loop driven from `pkg/codegen`, with `DemoteCandidate` mutator on `*CutResult` in `pkg/activation` (no codegen import in activation).
- Receiver factory constructor-argument metadata so `NewArgon2Hasher("")` (or a registered default) renders correctly.
- DB/SQL reconstructor runtime correctness: extracted-Deployment env block, `DATABASE_URL` propagation gated on `sql_db`/`sql_db_wrapper`, driver linking, `db.PingContext` startup validation, `StateCloseLines` shutdown emission, wrapper-constructor metadata.
- One DB/SQL corpus trace passing focused Kind e2e through deploy (stage 7) with real reconstructed DB — primary target miniflux/M-1.
- Best-effort stretch on one additional DB/SQL trace selected from a Phase 0 taxonomy.
- gitea/M-16 admission accepted with route resolved, plan built, server/client rendered and compile-clean.
- Manifest updates in `test/e2e/activation_corpus_traces.yaml` for every trace whose status changes.

**Out of scope (do not expand):**
- New reconstructor families: HTTP client expansion, App/config, mailer/SMTP, object store, queue, repository/git context — SPRINT-0050+.
- Interface receiver concrete-type dispatch (approach (b)) — SPRINT-0050+.
- Shared-state coordination, proxy transport, mutable boundary write-back, gRPC, streaming beyond bytes.
- Schema migrations executed by the extracted service. Migrations remain the host binary's responsibility.
- mattermost/M-1 package-load failure (SPRINT-0048 §3F) — orthogonal evaluation-tree issue.
- Re-writing the activation graph or path; only the candidate ordering and recommendation pointer change.
- Full env-off-fail-modes (stage 10) as the must-have for DB/SQL targets — stretch only.

## Concrete targets

| # | Project | Function | File:line | State class | Acceptance |
|---|---------|----------|-----------|-------------|------------|
| 1 | miniflux | `RefreshFeed` (M-1) | `internal/reader/handler/handler.go:207` | wraps `*sql.DB` via `*storage.Storage` | Stage 7 (deploy) with real Postgres |
| 2 | gitea | `(*Argon2Hasher).HashWithSaltBytes` (M-16) | `modules/auth/password/hash/argon2.go:29` | pointer receiver + factory | Admission accepted + render compile-clean (stage 4) |
| 3 | TBD by Phase 0 taxonomy | — | — | — | Stretch — stage 4 minimum, stage 7 preferred |

## Test execution discipline (read before running anything)

**No full e2e sweeps in this sprint.** Do not run all 12 targets, do not run `make e2e`, do not run `scripts/run_activation_corpus_sweep.sh --phases all`. Only run Kind e2e on the **specific targets this sprint touches** (miniflux/M-1, gitea/M-16, the one stretch target) — and even then, one target per `go test` process.

Hard rules:

- **One target per `go test` invocation.** `MONOLIFT_E2E=1 go test -tags e2e -run 'TestE2E/<exact-target-name>' -count=1 -timeout=15m ./test/e2e/` — a single concrete target name, never a `(a|b|c|...)` alternation regex. SPRINT-0048 §6.2/§6.5: bundled runs cause codegen memory accumulation and spurious timeouts.
- **Never** run `make e2e`, a multi-target `-run` regex, or a full corpus e2e sweep as a "check everything" shortcut. If you think you need a broad regression check, you don't — re-run only the targets whose code paths you changed.
- The `--admission-only` corpus sweep (`scripts/run_activation_corpus_sweep.sh --admission-only`) is **fine** — it runs no Kind e2e, just admission checks, and is the cheap pre/post measurement. The full `--phases all` e2e sweep is **out**.
- Run targets sequentially, never in parallel. Between targets, let the prior namespace clean up (harness deferred cleanup, SPRINT-0048 §0.7).
- Per-target timeout: 15m for shape targets, 25m for the heaviest (mattermost, gitea). A single target exceeding its own timeout is a real signal — record it, don't raise the timeout.
- If you SIGKILL or otherwise abort an e2e run, the harness's deferred namespace cleanup does **not** fire — you must `kind delete cluster --name monolift-e2e` (or delete the orphaned `mlv2-*` namespaces) before the next run, or the leftover pods will slow everything down.

### Phase 0: Baseline, taxonomy, target selection

No code changes. Anchor the coverage report to a pre/post measurement and pick stretch targets from real grouping.

- [x] 0.1: Run `scripts/run_activation_corpus_sweep.sh --admission-only --output-dir .moab/runs/sprint-0049-admission-baseline`. Save full refusal-code histogram to `docs/research/runs/SPRINT-0049-baseline.md`. Confirm 5 pass / 6 admission-skip / 5 timeout-skip / 56 manifest-skip matches the 0048 coverage report; document any drift.
- [x] 0.2: Cross-reference `test/e2e/activation_corpus_traces.yaml` against the 0048 backlog and enumerate the ~15 DB/SQL-blocked traces. Group by reconstructor pattern: (a) wraps `*sql.DB` via named field (miniflux `*storage.Storage`), (b) wraps `*sql.DB` indirectly through another struct, (c) embeds a DB-bearing interface, (d) takes `*sql.DB` as a direct parameter, (e) other. Write the grouping into the baseline doc.
- [x] 0.3: From the grouping, pick: primary = miniflux/M-1 (group (a), already stage 4); stretch candidate = one from group (a) or (d). Record reason.
- [x] 0.4: For each picked trace, run `cmd/activation-path` with reverse-import scoping. Capture full path, recommended cut, all candidates with feasibility, and admission verdict against the recommended candidate. Save under `docs/research/runs/SPRINT-0049-target-analysis/`.
- [x] 0.5: Confirm `(*Argon2Hasher).HashWithSaltBytes` is reachable at step 6 of the gitea/M-16 path under current reverse-import scope. If absent from the candidate set, admission-aware rerank cannot help and the M-16 plan must be reconsidered before Phase 1C.
- [x] 0.6: Record the pre-code baseline by **reference, not re-run**. Copy SPRINT-0048's documented per-target results (coverage report §"Per-Target Results", `docs/research/runs/SPRINT-0048-coverage-report.md`) into `docs/research/runs/SPRINT-0049-baseline.md` as the starting state for the 7 hand-picked + 5 corpus targets. No e2e sweep — regression checking happens per-target in Phase 5, only on targets this sprint actually touches.

### Phase 1: Admission-aware cut placement

Close the loop between activation ranking and codegen admission so a top candidate refused by admission cedes to the next-best admissible candidate.

#### 1A: Admission-against-candidate helper

- [x] 1A.1: Create `pkg/codegen/cut_admit.go` with the `tryAdmitCandidate` signature stubbed: `func tryAdmitCandidate(report reportv2.Report, candidate activation.CutCandidate) (codegen.AdmissionVerdict, *codegen.Plan, error)`. Returns a not-implemented verdict for now. Compiles.
- [x] 1A.2: Implement the `AdmitCut` step inside `tryAdmitCandidate` — build a `CutResult` view with `Recommended` set to the candidate, call `AdmitCut`, return early on refusal.
- [x] 1A.3: Implement the `BuildPlan` step with a per-call timeout (default 5s); on timeout return a `plan_build_timeout` verdict, not a panic.
- [x] 1A.4: Implement the `AdmitPlan` step and merge the cut + plan verdicts into the returned `AdmissionVerdict`.
- [x] 1A.5: Add a result cache keyed by `(packagePath, funcName, receiverType)` so the same candidate isn't re-planned. Document the timeout + cache rationale in the file header.
- [x] 1A.6: Unit test: candidate that admits cleanly returns an accept verdict + non-nil plan.
- [x] 1A.7: Unit test: candidate refused by `AdmitCut`, and candidate refused by `AdmitPlan` with `receiver_requires_reconstruction` — both return the refusal code.
- [x] 1A.8: Unit test: candidate whose `BuildPlan` times out returns the `plan_build_timeout` verdict without panicking.

#### 1B: Demote-and-rerank loop

- [x] 1B.1: Add `func (cut *CutResult) DemoteCandidate(step int, nodeKey FunctionKey, reason string)` in `pkg/activation/cut.go` — marks the matching candidate `Feasibility = Infeasible`, `Reason` prefixed `admission-refused: <code>: ...`.
- [x] 1B.2: Make `DemoteCandidate` re-run `rankCutCandidates` over the remaining candidates and confirm existing `Infeasible` filtering at `cut.go:172-176` already excludes the demoted one.
- [x] 1B.3: Unit test for `DemoteCandidate` alone: demoting the recommended candidate shifts `Recommended` to the next feasible one.
- [x] 1B.4: In the lift entry point (`codegen.RunLift` or equivalent), add the retry loop skeleton: try the recommended candidate via `tryAdmitCandidate`; on accept, proceed as today. No demotion yet — just prove the loop is wired without behavior change.
- [x] 1B.5: Add demotion to the loop: if refused with a code in the retry set `{receiver_requires_reconstruction, non_serializable_receiver, unsupported_result_shape, missing_reconstructor}`, call `DemoteCandidate` and retry. Cap at `len(Candidates)` retries and a hard 60s wall-clock budget.
- [x] 1B.6: Emit a structured `DemotionChain` diagnostic in `LiftResult` — every demoted candidate, its node key, the refusal code.
- [x] 1B.7: Gate the whole loop behind `MONOLIFT_ADMISSION_AWARE_RANK` (default `1`); `0` preserves exact single-pass behavior.
- [x] 1B.8: Unit test: synthetic 3-step path, step 2 admits-then-refuses, step 3 admits cleanly — rerank on → recommended is step 3.
- [x] 1B.9: Unit test: 3-step path where every candidate is refused — clean diagnostic, no panic, `LiftResult.AdmissionVerdict` carries the final refusal + demotion chain.
- [x] 1B.10: Regression test: demotion loop against the 7 hand-picked targets — recommended cut unchanged from SPRINT-0048 (uses cached SPRINT-0048 cut data, not a fresh e2e run).

#### 1C: gitea/M-16 leaf admission + factory-args extension

- [x] 1C.1: Extend `pkg/codegen/receiver.go:12` `receiverFactoryEntry` with a `ConstructorArgs []string` (or equivalent) field for factories that take more than `*sql.DB`. Default the Argon2 entry to a single empty-string arg, matching gitea's production fall-through behavior.
- [x] 1C.2: Update `pkg/codegen/server.go:171` so factory rendering emits the constructor args from the registry entry, not a hardcoded zero-arg call.
- [x] 1C.3: Golden test: rendered server for a `ReceiverFactory` plan with non-empty `ConstructorArgs` produces a syntactically valid factory invocation.
- [x] 1C.4: Re-run codegen for gitea/M-16 with `MONOLIFT_ADMISSION_AWARE_RANK=1`. Expected: step 5 demoted with `receiver_requires_reconstruction`; step 6 admitted with `ReceiverFactory` policy and the correct factory call rendered.
- [x] 1C.5: If admission still refuses at step 6, check the `[]byte salt` codec classification: `[]byte` should be JSON-serializable (base64), not in `ReconstructedParams`. Fix the codec mapping if mis-classified (per SPRINT-0048 §3E known issue with `[]byte`).
- [x] 1C.6: Update `test/e2e/targets/activation_gitea_argon2hash/target.go` (scaffolded but skipped in SPRINT-0048 §3B). Set `ActivationLift.Target` to `modules/auth/password/hash/argon2.go:29`, `StopAtStage: 4` initially, expected verdict matching the new admission outcome.
- [x] 1C.7: Update `test/e2e/activation_corpus_traces.yaml` for gitea/M-16: `status: pass`, `phase: "1"`, `e2e_package: activation_gitea_argon2hash`. Clear `skip_reason` if e2e passes stage 4.
- [x] 1C.8: Run focused Kind e2e for `activation-gitea-argon2hash`. Stage 4 minimum; stages 5-7 stretch.

### Phase 2: DB/SQL reconstructor runtime correctness

Make `sql_db` and `sql_db_wrapper` reconstructors produce an extracted service that actually opens a working connection. Phase 1 and Phase 2 are independent and can proceed in parallel.

#### 2A: Driver linking and module resolution

- [x] 2A.1: Confirm `pkg/codegen/server.go` translates the reconstructor `Imports` entry `"_ github.com/lib/pq"` (recon.go:40, 121) into a blank import in the rendered server file. Add a render unit test asserting the driver import appears.
- [x] 2A.2: Determine whether `cmd/extracted/<target>` generates its own `go.mod`/`go.work` or shares the host's module. Inspect `pkg/codegen/writer.go`. Document the finding.
- [x] 2A.3: If a separate module is generated, ensure `github.com/lib/pq` is declared. If shared, verify `lib/pq` is in the host's `go.mod` (miniflux: confirmed). Add a build test that compiles the rendered extracted-service for miniflux/M-1 in a scratch directory.

#### 2B: Extracted Deployment env block

- [x] 2B.1: Audit `pkg/codegen/kubernetes.go` `extractedDeploymentTemplate`. Current state: no `env:` section, so `DATABASE_URL` never reaches the extracted pod. The host template appends `HostEnvVars`; the extracted template is the gap.
- [x] 2B.2: Add `ExtractedEnvVars []EnvVar` (or equivalent) to `DeployOptions` / `Plan` so targets list extracted env vars explicitly. Auto-include `DATABASE_URL` *only when a `sql_db` or `sql_db_wrapper` reconstructor is present in the plan*; do not propagate all `HostEnvVars`. Project-specific names (`GITEA_DB_*`, etc.) stay explicit per-target.
- [x] 2B.3: Render the `env:` block in `extractedDeploymentTemplate` from `ExtractedEnvVars`.
- [x] 2B.4: Codegen test: extracted Deployment YAML contains the `DATABASE_URL` env var when the plan has a `sql_db`/`sql_db_wrapper` reconstructor, and contains no env block when no SQL reconstructor is present.
- [x] 2B.5: For miniflux/M-1, set `ExtractedEnvVars` to `DATABASE_URL=postgres://miniflux:miniflux@postgres:5432/miniflux?sslmode=disable` in `test/e2e/targets/activation_miniflux_refreshfeed/target.go`. Verify dormant invariant from SPRINT-0048 §6.7 (no `MONOLIFT_LIFT_*` env vars).

#### 2C: Startup validation and shutdown cleanup

- [x] 2C.1: Update `serverReconstructorInit()` in `pkg/codegen/server.go:192` so SQL reconstructors call `db.PingContext` (or equivalent) and surface a clean error on failure. Distinguishes "rendered a DB handle" from "runtime-correct DB reconstruction." Anti-flake for the "Pod ready before Postgres ready" race.
- [x] 2C.2: Update `serverTemplate` in `pkg/codegen/server.go` to execute `StateCloseLines` (e.g., `db.Close()`) on server shutdown via deferred cleanup in `main`. The metadata is set by `sqlWrapperReconstructor` (`recon.go:125`) but is currently inert.
- [x] 2C.3: Golden test: rendered `main.go` for a direct `*sql.DB` reconstructor includes `Open → Ping → defer Close`.
- [x] 2C.4: Golden test: rendered `main.go` for a `*storage.Storage` wrapper reconstructor includes the same plus the wrapper constructor call.

#### 2D: Wrapper constructor metadata

- [x] 2D.1: Extend the `Reconstructor` type in `pkg/codegen/types.go` with explicit constructor metadata: `ConstructorPkg string`, `ConstructorFunc string`, `ConstructorArgOrder []string`. Default to the current "New<Type>(*sql.DB)" convention; override per known wrapper.
- [x] 2D.2: Update `sqlWrapperReconstructor()` (`recon.go:100`) to use explicit metadata. Verify the miniflux constructor signature in `evaluation/miniflux/internal/storage/storage.go:18` (`NewStorage(db *sql.DB) *Storage` — confirmed matches default).
- [x] 2D.3: Unit test in `pkg/codegen/recon_test.go`: `BuildPlan` against a fixture mimicking miniflux/M-1 produces a `sql_db_wrapper` reconstructor with correct constructor metadata.
- [x] 2D.4: Admission test in `pkg/codegen/admission_test.go`: a reconstructed param with empty `Reconstructor.ID` is refused with `missing_reconstructor` (pins the existing `admission.go` gate).

#### 2E: Migration ordering invariant

- [x] 2E.1: Confirm miniflux baseline runs migrations via `RUN_MIGRATIONS=1` at host startup. Document the invariant: **the host migrates, the extracted service never migrates**. Write it into `SPRINT-0049-baseline.md`.
- [x] 2E.2: Verify the harness deploys host first and gates stage-7 workload on host `/healthcheck` *before* exercising the extracted service. If not, add an explicit synchronization point in `test/e2e/e2e_test.go` or `test/e2e/harness/`.

### Phase 3: miniflux/M-1 full DB/SQL e2e

Lift `activation_miniflux_refreshfeed` from `StopAtStage: 4` to `StopAtStage: 7` (deploy) with real reconstructed DB. Stage 10 stretch.

- [x] 3.1: Update `test/e2e/targets/activation_miniflux_refreshfeed/target.go`: `ExtractedEnvVars` per 2B.5, `ExpectedVerdict` matching the post-rerank admission outcome. Leave `StopAtStage: 4` for now — raise it incrementally as each stage passes.
- [x] 3.2: Extend `workload.go` to drive `PUT /v1/feeds/{id}/refresh` and assert feed entries appear via the host API after refresh.
- [x] 3.3: Oracle decision — direct invocation only if it shares the fixture DB without nondeterminism; otherwise document that workload + extracted `/calls` delta is the correctness signal (policy, not exception).
- [x] 3.4: Run focused Kind e2e at `StopAtStage: 4` (compile+verdict). Confirm it still passes with the Phase 1/2 changes in place.
- [x] 3.5: Raise to `StopAtStage: 5` (artifact build). Run focused e2e; if the extracted-service build fails, fix per Phase 2A (driver linking) before continuing.
- [x] 3.6: Raise to `StopAtStage: 6` (kind load). Run focused e2e; confirm the extracted image loads into the cluster.
- [x] 3.7: Raise to `StopAtStage: 7` (deploy). Run focused e2e. If the extracted Pod CrashLoops, collect pod logs — likely culprits: missing `lib/pq`, unreachable `postgres` Service, missing/wrong `DATABASE_URL`, ping failure. Fix and re-run.
- [x] 3.8: Once stage 7 is green, update the manifest entry for miniflux/M-1: stage reached, duration, any deferred behavior.
- [x] 3.9: Stretch only — attempt `StopAtStage: 10`. If unstable due to the error-wrapper return shape, revert to 7 and document the gap. Do not block closeout on this.

### Phase 4: Stretch DB/SQL target

Best-effort, skip-on-failure. Picked from the Phase 0.3 grouping.

- [x] 4.1: Run activation analysis and codegen for the stretch candidate. If admission refuses, capture the refusal code; either fix (e.g., wrapper metadata extension) or defer.
- [x] 4.2: If admission passes, scaffold `test/e2e/targets/<package>/` with target/workload/oracle following the miniflux/M-1 pattern. Reuse Postgres fixture where possible; add project-specific baseline manifests.
- [x] 4.3: Run focused Kind e2e. Stage 4 minimum, stage 7 preferred.
- [x] 4.4: Update the manifest with current status, refusal code if skipped, `e2e_package` if scaffolded.
- [x] 4.5: If group (d) (`*sql.DB` as direct param) has no admissible candidates without scope expansion, fall back to another group (a) candidate; record the choice.

### Phase 5: Verification and closeout

- [x] 5.1: Bump `GeneratorVersion` at `pkg/codegen/types.go:8` from `"SPRINT-0048"` to `"SPRINT-0049"`.
- [x] 5.2: Update the writer-test golden (`pkg/codegen/writer_test.go:47`) for the version bump; run `go test ./pkg/codegen/...` to confirm no other goldens reference the string.
- [x] 5.3: Run `go test ./pkg/activation/...` — all pass.
- [x] 5.4: Run `go test ./pkg/codegen/... ./test/e2e/harness/...` — all pass.
- [x] 5.5: Targeted regression e2e — **only** the targets this sprint's code paths could affect. Run `activation-miniflux-refreshfeed` as its own `go test -run` invocation; confirm it passes at its new stage.
- [x] 5.6: Targeted regression e2e — run `activation-gitea-argon2hash` as its own invocation; confirm the admission/render outcome from Phase 1C holds.
- [x] 5.7: Targeted regression e2e — pick the 2 SPRINT-0048 targets most likely affected by the admission-aware rerank change (a receiver-method target and a multi-return target, e.g. `activation-pocketbase-passwordvalidate` and `activation-mattermost-pbkdf2hash`); run each as its own invocation; confirm unchanged. Do **not** run the other 8 — the 1B.10 regression test already pins recommendation stability, and a full sweep is out of scope.
- [x] 5.8: Run `scripts/run_activation_corpus_sweep.sh --admission-only`. Diff the refusal-code histogram against the Phase 0.1 baseline; gitea/M-16 should move from `admission-skip` to `pass` (or a clean post-rerank refusal code if 1C ran out of road).
- [x] 5.9: Verify the dormant invariant for the DB/SQL targets touched (no `MONOLIFT_LIFT_*` env vars in extracted Deployments, no `/calls` deltas in env-off).
- [x] 5.10: Write `docs/research/runs/SPRINT-0049-coverage-report.md` mirroring the SPRINT-0048 format: executive summary, trace matrix before/after, per-target results, capabilities added (admission-aware ranking + factory args + DB/SQL runtime), refusal-code histogram diff, residual blockers, next-sprint backlog (interface receiver resolution, HTTP client expansion, App/config, mailer).
- [ ] 5.11: Update ledger entry for SPRINT-0049 via `~/.claude/skills/sprint-planner/scripts/ledger.py set-status SPRINT-0049 done`. Record executor.

## Sequencing

```
Phase 0 (baseline + taxonomy) ← GATE: no code changes until grouping is clear
    │
    ├──→ Phase 1 (admission-aware rerank + factory args + gitea/M-16) ─┐
    │                                                                  │
    ├──→ Phase 2 (DB/SQL runtime correctness) ─────┐                   │
    │         │                                    │                   │
    │         └──→ Phase 3 (miniflux/M-1 to stage 7)│                   │
    │                       │                      │                   │
    │                       ↓                      │                   │
    │                  Phase 4 (stretch DB/SQL)    │                   │
    │                                              ↓                   ↓
    └──────────────────────────────────────→ Phase 5 (verify + closeout)
```

**Phase 1 and Phase 2 run in parallel** — admission-aware ranking and DB/SQL runtime touch disjoint code paths and have independent unit tests. Phase 1C (gitea/M-16) lands inside Phase 1 once 1A/1B exist. Phase 3 depends on Phase 2. Phase 4 is purely best-effort and must not block Phase 5.

## Risks

**R1: Demotion loop pushes the recommendation into a meaningless leaf.** Admission-aware rerank could pick a tiny helper that admits but isn't a useful service boundary. *Mitigation:* the 1B.10 regression test pins the 7 hand-picked targets' recommendations; any change is a hard failure unless explicitly accepted. Retry only on the configured refusal-code set, not all refusals. `MONOLIFT_ADMISSION_AWARE_RANK=0` opt-out for emergency disable.

**R2: Per-candidate `BuildPlan` cost on Mattermost-sized graphs.** Building plans against N candidates could blow stage-3 timeouts. *Mitigation:* 5s per-candidate timeout (1A.3) with `plan_build_timeout` as a refusal reason; result cache keyed by candidate identity (1A.5). Measure Mattermost wall-clock after 1A lands; reconsider caching strategy only if measurements demand it.

**R3: Factory-args extension misses gitea/M-16 still.** Even with `ConstructorArgs`, the leaf may refuse for a different reason (e.g., `[]byte` codec misclassification per 1C.5). *Mitigation:* Phase 0.4 captures the admission verdict against the leaf before Phase 1 begins, so the gap surfaces early. If 1C.5 reveals another codec issue, fix or document; M-16 acceptance bar drops to "admission accepts + documented compile gap" only as last resort.

**R4: Extracted Pod ready before Postgres.** `sql.Open` doesn't connect; the first request fails. *Mitigation:* 2C.1 adds `PingContext` at startup so the pod fails fast instead of CrashLoopBackoff. K8s readiness probe is a follow-up.

**R5: Migration race.** Extracted pod hits a schema-less DB if host migrations haven't finished. *Mitigation:* 2E enforces the invariant — host migrates, extracted never does — and gates stage-7 workload on host `/healthcheck`.

**R6: `sqlWrapperReconstructor` brittleness.** The hardcoded `New<Type>(*sql.DB)` convention fits miniflux but may not fit gitea's xorm or listmonk's goyesql wrappers. *Mitigation:* 2D adds explicit metadata so per-project overrides are possible. Phase 4 stretch may surface this; document and defer to a follow-up sprint if needed.

**R7: `[]byte` codec classification regression.** SPRINT-0048 §3E flagged that `[]byte` params can be mis-routed to `ReconstructedParams`. If gitea/M-16's `salt []byte` triggers the same path, admission refuses at the leaf too. *Mitigation:* 1C.5 patches the classification if it surfaces; Phase 0.4 catches it before Phase 1C.

**R8: Cross-namespace DNS for `postgres` Service.** The harness creates per-target namespaces; if extracted and host land in different namespaces, the bare `postgres` Service name won't resolve. *Mitigation:* the existing harness puts both Deployments in the same namespace per target — confirm this when the stage-7 run in Phase 3.7 first deploys the extracted Pod; if DNS fails, that's the first thing to check.

**R9: `GeneratorVersion` bump propagates to many goldens.** *Mitigation:* The 0048 retro confirmed only `writer_test.go:47` hardcodes the string. If other goldens break, that's a regression worth investigating.

**R10: Stretch target reveals a wrapper variant `sqlWrapperReconstructor` can't handle.** *Mitigation:* Out of scope to extend the wrapper this sprint beyond the metadata work in 2D; document the limitation in the coverage report and add to the SPRINT-0050 backlog.

## Design decisions

**D1: Admission-aware ranking over interface-receiver resolution.** Argued in Intent. Approach (a) generalizes; approach (b) is narrow and substantially larger.

**D2: Re-rank lives in `pkg/codegen`, not `pkg/activation`.** Keeps `pkg/activation` free of any codegen import. `pkg/activation` exposes `DemoteCandidate`; `pkg/codegen` drives the loop. Prevents the import cycle that would otherwise arise from passing admission verdicts back into the ranker.

**D3: Reuse existing `Infeasible` enum for demoted candidates.** `rankCutCandidates` already filters `Infeasible` at `cut.go:172-176`. Reusing it means no changes to the ranker's core logic, only a new mutator entry point. The `Reason` field carries the discrimination string `admission-refused: <code>: ...`.

**D4: Per-candidate timeout, not global retry budget.** A global budget could starve later candidates if the first one's `BuildPlan` is slow. Per-candidate 5s default plus an overall 60s wall-clock cap is the belt-and-suspenders.

**D5: Explicit `ExtractedEnvVars`, gated propagation only for known DB env names.** Auto-propagating all `HostEnvVars` to extracted pods leaks App/config, license keys, feature flags, and project-specific secrets into the lifted service — a footgun even in research contexts. Explicit per-target spec mirrors `HostEnvVars` and stays predictable across projects. The narrow auto-propagation rule (only `DATABASE_URL` when a SQL reconstructor is in the plan) gives the convenience without the leak.

**D6: Migrations stay in the host.** The extracted service must not run migrations: it would race the host, and migrations are project-specific (miniflux uses sqlx, gitea uses xorm, listmonk uses goyesql). Keeping migration responsibility on the host keeps the reconstructor stateless and project-agnostic.

**D7: Factory-args extension is part of admission-aware ranking, not "renderer cleanup".** Without it, M-16 admission flips but the rendered server doesn't compile. The acceptance bar for M-16 — "route resolved, plan built, server + client rendered" — must mean compile-clean output, not just bytes-on-disk. Schedule it explicitly in 1C; do not defer.

**D8: Workload-only correctness signal is authorized policy for awkward oracle cases.** `RefreshFeed`'s `*locale.LocalizedErrorWrapper` return makes direct invocation comparison fragile. Workload-level invariants (entries appear after refresh) plus extracted `/calls` deltas is sufficient signal. Future targets with similar shapes can adopt the same pattern without a new design decision.

**D9: Stage 7 is the must-have for miniflux/M-1; stage 10 is stretch.** Real DB reconstruction proven at stage 7 (extracted Pod deploys with `DATABASE_URL`, opens a real connection, handles a real request) is the sprint-defining artifact. Stage 10 fail-modes are valuable but the error-wrapper return shape may surface oracle issues unrelated to DB/SQL — don't let those block sprint acceptance.

**D10: Compile-only fallback for gitea/M-16.** M-16 doesn't need full Kind e2e to count as the secondary unlock. Admission accepted + compile-clean render is the artifact that proves admission-aware ranking works on a real corpus trace. Forcing stage-7 deploy through gitea (the heaviest evaluation codebase) would consume the sprint's budget for marginal additional evidence.

## Acceptance criteria

**Minimum (must-have):**
- [ ] At least 1 DB/SQL corpus trace passes focused Kind e2e through stage 7 with real reconstructed DB at runtime. Compile-only (stage 4) accepted only if a real DB reconstruction gap remains and is documented with a concrete refusal/error code.
- [x] `gitea/M-16` admission accepted with route resolved, plan built, and server + client rendered compile-clean. Stage 4 e2e passes.
- [ ] No regressions on touched targets: targeted per-target e2e (Phase 5.5–5.7) passes, and the 1B.10 recommendation-stability test pins the 7 hand-picked targets' cuts unchanged. No full e2e sweep is run or required.
- [ ] Admission-aware ranking is on by default with `MONOLIFT_ADMISSION_AWARE_RANK=0` opt-out and a regression test pinning the 7 hand-picked targets' recommendations unchanged.
- [ ] Coverage report at `docs/research/runs/SPRINT-0049-coverage-report.md` mirroring the SPRINT-0048 format, including a refusal-code histogram diff vs. Phase 0.1 baseline.
- [ ] `GeneratorVersion` bumped to `"SPRINT-0049"` at `pkg/codegen/types.go:8`; `writer_test.go` golden updated.
- [ ] `go test ./pkg/activation/... ./pkg/codegen/... ./test/e2e/harness/...` passes.
- [ ] Every deferred row in `test/e2e/activation_corpus_traces.yaml` has a stable manifest skip reason and admission refusal code.

**Target:**
- [ ] 2-3 DB/SQL corpus traces pass through stage 7.
- [ ] Admission-only sweep classifies all 72 rows with stable refusal codes; new refusals introduced by this sprint have clean reason strings.
- [x] `gitea/M-16` reaches stage 5 (artifact build) cleanly.

**Stretch:**
- [ ] 4+ DB/SQL corpus traces reach stage 7+; one DB/SQL trace reaches stage 10 (full env-off-fail-modes).
- [x] `gitea/M-16` reaches stage 7 (extracted service deployed).
- [ ] Mattermost wall-clock for admission-aware ranking measured and recorded; demonstrates per-candidate timeout (1A.2) keeps the path bounded.

## Reference

- SPRINT-0048: `docs/sprints/SPRINT-0048.md` — predecessor; shape axis complete
- SPRINT-0048 coverage: `docs/research/runs/SPRINT-0048-coverage-report.md`
- SPRINT-0048 baseline: `docs/research/runs/SPRINT-0048-baseline.md`
- Corpus manifest: `test/e2e/activation_corpus_traces.yaml`
- Reconstructor code: `pkg/codegen/recon.go`, server emission `pkg/codegen/server.go:192-215`, factory registry `pkg/codegen/receiver.go:19-29`
- Admission: `pkg/codegen/admission.go`
- Cut ranking: `pkg/activation/cut.go:165` (`rankCutCandidates`), `pkg/activation/cut.go:181` (`betterCutCandidate`)
- Gitea hash chain: `evaluation/gitea/modules/auth/password/hash/hash.go:36-59` (parent), `evaluation/gitea/modules/auth/password/hash/argon2.go:29` (leaf), `argon2.go:40` (factory)
- Miniflux storage wrapper: `evaluation/miniflux/internal/storage/storage.go:18` (`NewStorage(db *sql.DB) *Storage`)
- Existing targets: `test/e2e/targets/activation_miniflux_refreshfeed/`, `test/e2e/targets/activation_gitea_argon2hash/`
- Sweep runner: `scripts/run_activation_corpus_sweep.sh`
- CloudLab harness: `cloudlab/setup.sh` (c220g5, 40 cores, 187GB RAM)

## Blockers

- 2026-05-14, task 5.7: resolved. Initial evidence from `.moab/runs/sprint-0049-e2e/5.7b-activation-mattermost-pbkdf2hash-regression.log` showed the harness timing out in stage 5, but stage logging and profiling proved Docker was only where the remaining budget expired. The real bottleneck was full activation augmentation for Mattermost PBKDF2: `all` mode spent ~1201.5s in `augment`, while `structfield` mode reached the same 12-step path and recommended `PBKDF2.Hash` cut in ~4m45s. Wired per-target augmentation mode and set `activation-mattermost-pbkdf2hash` to `structfield`; focused e2e passed in 9.6m with log `.moab/runs/sprint-0049-debug/mattermost-pbkdf2-e2e-stage-log-structfield.log`.
- 2026-05-14, task 5.11: blocked by explicit operator instruction in the resume prompt: do not modify `docs/sprints/ledger.yaml`; the orchestrator handles ledger updates. All implementation, verification, admission sweep, dormant-invariant, and coverage-report tasks through 5.10 are complete. Leave 5.11 unchecked for the orchestrator.
