# SPRINT-0050: External persistence lift escalation

**Status:** in-progress
**Predecessors:** SPRINT-0048, SPRINT-0049

## Intent

SPRINT-0050 follows up on SPRINT-0049 by pushing activation lifts that depend on durable state outside the Go process. The primary goal is to prove that an extracted service can reconstruct access to external persistence, execute real workload traffic, survive the env-off and fail-mode checks, and, where feasible, reach the full stage-10 proof path.

The sprint also investigates why SPRINT-0049 stopped earlier than expected on several promising targets. Those stops were not all the same kind of failure:

| Target | SPRINT-0049 state | Meaning for this sprint |
|---|---|---|
| `miniflux/M-1` `RefreshFeed` | Stage 7 with real Postgres-backed `*storage.Storage`; stage 8 blocked by direct `/invoke` result handling for a nullable localized-error wrapper. | Primary DB/SQL stage-10 candidate. Treat as a harness/oracle policy problem before assuming the DB reconstructor failed. |
| `miniflux/M-5` `(*iconChecker).UpdateOrCreateFeedIcon` | Stage 4, but admission selected parent `RefreshFeed`, not the intended leaf. | Admission/ranking question. Do not count this as an icon/file leaf proof unless the selected cut is the intended durable-resource cut. |
| `miniflux/M-6` `ParseFeed` | Existing target reportedly reaches stage 7 while the manifest still says `admission-skip`. | Manifest drift, not an external-persistence proof. Reconcile before using counts. |
| `pocketbase/M-3` `Password.Validate` | Coverage says stage 10, manifest still says `e2e-retry`. | Manifest drift and regression candidate after harness/oracle changes. Do not count as a new persistence graduation. |
| `mattermost/M-14` `PBKDF2.Hash` | Stage 7; direct oracle skipped because hash output is nondeterministic. | Useful oracle-policy regression case, but not a persistence target. |
| `gitea/M-16` `Argon2.Hash` | Stage 8 required `GITEA__security__PASSWORD_HASH_ALGO=argon2` so the workload actually hit the lifted symbol. | Workload-fitness lesson. Required config belongs in target metadata, not tribal knowledge. |

This sprint should be more aggressive than SPRINT-0049, but not broader. The right shape is one DB/SQL stage-10 push, one filesystem/object-store runtime proof, and at most one stretch candidate that fits the same durable-resource pattern.

## External Persistence Boundary

In scope:

- Postgres/SQL handles and known wrappers, including `*sql.DB`, `*storage.Storage`, and DB wrapper types that can be reconstructed from explicit env/config.
- SQLite/file-backed DB research, but only promoted to implementation if Phase 0 proves an admissible target that does not require reconstructing `core.App` or app-owned shared state.
- Local filesystem roots used as durable backing stores.
- Object/blob/package storage clients accessed through a root path, URL, or bucket-like configuration, with local or in-cluster fixtures.
- Target-level workload requirements, such as a config knob needed for the host workload to hit the lifted symbol.

Out of scope:

- Web/request contexts such as `*http.Request`, `gin.Context`, `echo.Context`, `*core.RequestEvent`, or service context types.
- Queues, schedulers, mailers, SMTP, repository/git semantics, and generic HTTP-client expansion.
- Whole-app reconstructors such as `PocketBase.App`, `core.App`, or app/service roots that own goroutines, hooks, caches, or lifecycle state.
- Generic interface-receiver concrete-type dispatch.
- Schema migrations in the extracted service. The host fixture remains responsible for schema setup.
- In-process shared-state coordination, multi-call writer synchronization, and write-back of mutable boundary state.

## Stage Ladder

SPRINT-0049 exposed drift between the strategy docs, the activation harness, and how results were reported. Phase 0 must make the current ladder explicit before implementation starts.

| Stage | Current proof value | Common blockers |
|---|---|---|
| 0-2 | Fixture setup, baseline deploy, and baseline workload. The host app can serve the chosen request. | Fixture ordering, auth setup, migrations, readiness. |
| 3 | Activation analysis produced a path and recommended cut. | Reverse-import scope, augmentation cost, recommender priors. |
| 4 | Admission, plan, verdict, and compile-clean generated output. | Receiver class, codec classification, missing reconstructor, selected parent instead of intended leaf. |
| 5 | Extracted artifact builds. | Driver imports, module resolution, generated main/server linking. |
| 6 | Image loads into Kind. | Docker/Kind image plumbing. |
| 7 | Lifted deployment is Ready under the reconstructor startup contract. | Env propagation, DB reachability, filesystem mount shape, startup probe. |
| 8 | Real lifted workload reaches the extracted service, plus direct `/invoke` or an explicit replacement signal. | Workload does not hit the symbol, direct-invoke result shape mismatch, nondeterministic result. |
| 9 | Env-off and fail-mode behavior matches the declared client policy. | Stale state from prior runs, fail-open/fail-closed mismatch, unstable workload setup. |
| 10 | Cleanup and transcript comparison, or a declared behavioral invariant plus normalizer/substitution. | IDs, timestamps, random salts, side-effect ordering, missing normalizer. |

Stage 10 means the full proof path ran, not just that a target deployed. Any substitution for direct compare must be declared in target metadata and justified by the stage-binding doc.

## Candidate Priority

| Priority | Candidate | Durable resource pattern | Initial judgment |
|---|---|---|---|
| P0 | `miniflux/M-1` `RefreshFeed` | Postgres through `*storage.Storage` | Primary DB/SQL path. Already reached stage 7; likely blocked by direct-invoke/oracle policy and fresh-state handling. |
| P0 | `pocketbase/M-1` `(*filesystem.System).CreateThumb` | Local/blob filesystem through `*filesystem.System` | Primary filesystem/object-store path if Phase 0 confirms activation reaches the intended cut. Requires a durable shared root and root-relative payloads. |
| P1 | `pocketbase/M-4` `archive.Create` | Local filesystem root and zip output | Fallback filesystem target if `CreateThumb` pulls too much app state. Cleaner package-level shape if reachable. |
| P1 | `miniflux/M-5` `(*iconChecker).UpdateOrCreateFeedIcon` | Postgres plus feed-icon fetch/store | Only attempt if the intended leaf can be selected without generic HTTP-client work. Otherwise record parent-cut-only status. |
| P2 | `pocketbase/M-8` `writer.Write` | Writer-backed durable output | Likely out of scope unless one callable boundary includes the complete durable write, including close/finalization. |
| P2 | SQLite/PocketBase record expansion candidates such as `pocketbase/M-10` | SQLite through app/database wrapper | Research/backlog unless Phase 0 proves an admissible non-`core.App` boundary. Do not make this a primary stage-10 deliverable by default. |
| P2 | Gitea package/blob service cuts below routers | DB plus package/blob content store | Stretch research only. Reject router, auth context, queue, repository, and web-context cuts. |
| P3 | Listmonk media provider cuts | Filesystem/S3 media store | Stretch research only. Provider-level media store calls may fit; `(*App).UploadMedia` does not. |
| P3 | Mattermost file/filestore cuts | Local/S3 filestore | Stretch research only. Most obvious cuts are app-bound and historically expensive to load. |

## Phase 0: Research Gate

No reconstructor code starts before this phase produces the decision summary. The aim is to pick the targets from evidence, not from optimism.

- [x] 0.1: Run an admission-only corpus baseline on the CloudLab build node, saving artifacts under `.moab/runs/sprint-0050-admission-baseline/`.
- [x] 0.2: Reconcile `test/e2e/activation_corpus_traces.yaml` against SPRINT-0049 coverage, including `pocketbase/M-3`, `miniflux/M-6`, and the `miniflux/M-5` parent-cut caveat.
- [x] 0.3: Write `docs/research/runs/SPRINT-0050-stage-binding.md` with one section per stage 3-10: assertion, generated artifact, target toggles, allowed substitutions, and open questions.
- [x] 0.4: Resolve the direct-invoke policy tension for nullable localized-error wrappers, including whether the right fix is probe generalization, a typed target expectation, or a workload/calls-delta substitution.
- [x] 0.5: Resolve the admission-rerank tension for parent-over-leaf cuts. Decide explicitly among no change, opt-in deepest-admissible ranking, or a default ranking change.
- [x] 0.6: Resolve the workload-fitness tension from `gitea/M-16`: required config/env that makes a workload hit the lifted symbol must live in target metadata.
- [x] 0.7: Enumerate external-persistence candidates in `docs/research/runs/SPRINT-0050-candidates.md`, recording trace ID, source function, signature, durable resource, current status, selected cut if known, and accept/defer rationale.
- [x] 0.8: Run focused activation-path/admission analysis on CloudLab for `miniflux/M-1`, `miniflux/M-5`, `pocketbase/M-1`, `pocketbase/M-4`, and any one stretch candidate selected from the enumeration.
- [x] 0.9: Record for each focused candidate whether admission selected the intended durable-resource cut or reranked to a parent/helper cut.
- [x] 0.10: Choose exactly one DB/SQL primary, one filesystem/object-store primary, and at most one stretch target.
- [x] 0.11: For each chosen target, declare the stage-10 acceptance contract before implementation: workload signal, oracle or substitution, env-off/fail-mode behavior, fresh-resource policy, and transcript normalization if needed.
- [x] 0.12: Add a one-page decision summary at the top of `docs/research/runs/SPRINT-0050-candidates.md`. Phase 1+ is blocked until this is complete.

## Phase 1: Stage Contract and Harness Policy

- [x] 1.1: Update `docs/specs/e2e-test-strategy.md` or add a linked ADR/note that makes `SPRINT-0050-stage-binding.md` the authoritative activation stage contract.
- [x] 1.2: Add structured target metadata for direct-invoke expectations: `oracle-compare`, `non-nil-result`, `nullable-localized-error`, `status-only`, `behavioral-invariant`, or `workload-calls-delta`.
- [x] 1.3: Gate behavioral-invariant and workload/calls-delta substitutions by declared predicates. They are not generic opt-outs from stage 8-10 proof.
- [x] 1.4: Inspect the stage-8 direct-invoke path and `/invoke` handler for `miniflux/M-1`; record the exact decode/envelope behavior before changing code.
- [x] 1.5: Implement the chosen localized-error policy from Phase 0, with tests for nullable localized-error results, non-nil primitive results, oracle compare, and status-only results.
- [x] 1.6: Add or extend transcript normalization helpers for declared nondeterminism such as timestamps, IDs, random salts, and generated paths.
- [x] 1.7: Add behavioral-invariant hooks for side-effect-oriented targets, such as "feed entries exist", "thumbnail exists", or "login succeeds", while preserving stage-binding rigor.
- [x] 1.8: Define a fresh-resource policy for DB/file env-off and fail-mode checks so env-on side effects cannot pollute later comparisons.
- [x] 1.9: Improve diagnostics so direct-invoke expectation failures, workload-fitness failures, and transcript-normalizer failures are reported as harness/oracle classifications rather than generic workload failures.
- [x] 1.10: Implement the Phase 0 admission-rerank decision with synthetic-path tests. If a deepest-admissible mode is added, keep it opt-in unless Phase 0 explicitly proves the default should change.
- [x] 1.11: Add target-level workload requirements for config knobs, starting with `activation-gitea-argon2hash` and `GITEA__security__PASSWORD_HASH_ALGO=argon2`.
- [x] 1.12: Add a cheap workload-fitness check where feasible so a target fails early if the baseline workload never exercises the lifted symbol.

## Phase 2: DB/SQL Primary

Default target: `activation-miniflux-refreshfeed` unless Phase 0 chooses a better DB/SQL primary.

- [x] 2.1: Update the Miniflux target to use the direct-invoke expectation or workload/calls-delta policy selected in Phase 1.
- [x] 2.2: Raise the target monotonically on CloudLab from its current stage to stage 8, then stage 9, then stage 10, with one exact `go test` process per stage.
- [x] 2.3: At stage 8, verify the lifted workload creates observable feed entries through the host API and records an extracted-service `/calls` delta.
- [x] 2.4: At stage 9, verify env-off fallback through the renamed original and confirm the extracted service records no calls.
- [x] 2.5: At stage 9, verify fail-open and fail-closed behavior against the declared client policy, narrowing only if the stage-binding doc justifies it.
- [x] 2.6: At stage 10, run transcript comparison or the declared normalized/behavioral substitute against fresh resources.
- [x] 2.7: Re-verify the dormant invariant: the extracted deployment has required resource env vars, does not receive `MONOLIFT_LIFT_*`, and performs no side effects unless invoked.
- [ ] 2.8: If the target still cannot reach stage 10, capture the exact stage, response envelope, pod logs, generated manifests, and classification under `.moab/runs/sprint-0050-miniflux-m1/`.
- [x] 2.9: Update the manifest row with the stage reached, selected cut, proof kind, and residual blocker if any.

## Phase 3: Filesystem/Object-Store Reconstructor

Default target: `pocketbase/M-1` `(*filesystem.System).CreateThumb` if Phase 0 confirms the intended cut. Fallback: `pocketbase/M-4` `archive.Create`.

- [x] 3.1: Document the selected filesystem/object-store reconstructor family in `docs/decisions/` or the project’s current decision-log location.
- [x] 3.2: Extend reconstructor metadata so a state reconstructor can declare generated imports, init code, close code, extracted env vars, startup probes, and required mounts from one registry entry.
- [x] 3.3: Implement the minimal local filesystem/root reconstructor for the selected target. For PocketBase `*filesystem.System`, render `filesystem.NewLocal(root)` and close the handle during shutdown.
- [x] 3.4: Use an explicit env var for the durable root and reject unsafe absolute paths or `..` traversal when payload paths are meant to be root-relative.
- [x] 3.5: Add deployment support for a shared durable root between host and extracted service. Do not use per-pod `emptyDir` for data that both pods must see.
- [x] 3.6: Add a startup probe or init check that distinguishes rendered code from usable state, such as statting the durable root or probing a bucket.
- [x] 3.7: Add unit and golden tests for reconstructor detection, constructor metadata, init/probe/close rendering, selected env propagation, and mount propagation.
- [x] 3.8: Add admission tests proving the selected reconstructed parameter admits, and that missing reconstructor metadata refuses with a clear reason.
- [x] 3.9: Bump `GeneratorVersion` to `SPRINT-0050` with the first codegen output, reconstructor, or generated deployment-shape change, and update goldens in the same patch.

## Phase 4: Filesystem/Object-Store Target

- [x] 4.1: Scaffold the selected target only after Phase 0 confirms the intended cut and resource shape.
- [x] 4.2: For `CreateThumb`, build the workload around a shared durable local root: seed an original image object, invoke thumbnail creation through the host path, and assert the thumbnail exists with expected content type or dimensions.
- [x] 4.3: Use root-relative direct-invocation payloads. Do not pass host absolute paths as target payload convenience.
- [ ] 4.4: If `archive.Create` is selected, seed deterministic files in the durable root and verify the zip output contents through a root-relative output path.
- [x] 4.5: Run stage 4, 5, 6, and 7 on CloudLab as separate exact-target processes.
- [x] 4.6: Treat stage 7 as the minimum runtime proof for the new filesystem/object-store family.
- [x] 4.7: Best effort: raise the filesystem target to stage 8, 9, and 10 using the same monotonic process if transcript and env-off semantics are stable.
- [x] 4.8: If the target stops before stage 10, document the binding stage and reason in the coverage report instead of relabeling a deploy-only pass as full proof.
- [x] 4.9: Update the manifest row with the selected cut, resource kind, stage reached, and refusal/blocker reason if applicable.

## Phase 5: Stretch Candidate

Only one stretch target should be promoted after the primary DB and filesystem paths are stable.

- [x] 5.1: Reevaluate `miniflux/M-5` after the admission-rerank decision. Attempt it only if the intended `*iconChecker` cut can be expressed without adding generic HTTP-client reconstruction.
- [x] 5.2: Research one Gitea package/blob cut below routers if it fits the same durable-resource pattern and avoids auth/web context, repository state, queues, and generic HTTP client expansion.
- [x] 5.3: Research one Listmonk provider-level media store cut if it avoids `(*App).UploadMedia`, `echo.Context`, import goroutines, mailers, and app-owned shared state.
- [x] 5.4: Research Mattermost filestore cuts only if time remains and avoid `channels/app`-rooted candidates unless load stability has already been demonstrated.
- [x] 5.5: Promote at most one stretch target. Reuse the DB or filesystem fixture; do not add a third fixture family this sprint.
- [x] 5.6: Aim for stage 4 minimum and stage 7 if cheap. Do not let stretch stage 10 block closeout.

## Phase 6: Verification and Closeout

- [ ] 6.1: Run `go test ./pkg/activation/... ./pkg/codegen/... ./test/e2e/harness/...` on CloudLab and store logs under `.moab/runs/sprint-0050-closeout/`.
- [ ] 6.2: Run focused e2e for each touched target as one exact `go test -run '^TestE2E/<target>$'` process.
- [ ] 6.3: After any Phase 1 direct-invoke or oracle-policy change, run regression e2e for adjacent SPRINT-0049 targets likely to be affected, including `activation-pocketbase-passwordvalidate` and `activation-mattermost-pbkdf2hash` if applicable.
- [ ] 6.4: Run a final admission-only corpus sweep on CloudLab and store artifacts under `.moab/runs/sprint-0050-admission-final/`.
- [ ] 6.5: Verify dormant invariants for every persistence target: resource env vars present, `MONOLIFT_LIFT_*` absent from extracted deployment, env-off records no extracted-service calls.
- [ ] 6.6: Update `test/e2e/activation_corpus_traces.yaml` for every touched row with accurate status, selected cut, stage reached, proof kind, and skip/refusal reason.
- [ ] 6.7: Write `docs/research/runs/SPRINT-0050-coverage-report.md` with before/after counts, per-target stage results, reconstructor families added, stage-ladder decisions, manifest deltas, residual blockers, and next-sprint backlog.
- [ ] 6.8: Confirm `GeneratorVersion` and codegen goldens are consistent if generated output changed.

## Remote Test Discipline

- [ ] R.1: Before heavy work, verify the current CloudLab experiment with `cl ls` or `cl status`. If no experiment exists, stop and ask the user to start the `monolift-buildserver` profile.
- [ ] R.2: Run all package-wide Go tests, real-corpus activation-path analysis, e2e, Kind, Docker/image builds, and corpus sweeps on CloudLab.
- [ ] R.3: Local work is limited to editing, source reading, docs, manifest edits, and small codegen golden/unit tests that do not touch `evaluation/*`.
- [ ] R.4: Never run `make e2e`, a multi-target `-run` alternation, or `scripts/run_activation_corpus_sweep.sh --phases all`.
- [ ] R.5: Admission-only corpus sweeps are allowed only as coarse manifest drift measurements. Focused candidate research must never use whole-repo `./...` admission; use reverse-import scope or an explicit target/importer package set, and treat timeouts from broad package loading as invalid evidence.
- [ ] R.6: Run e2e stage escalation one target and one stage at a time. Do not jump from stage 7 straight to stage 10.
- [ ] R.7: If an e2e run is aborted before harness cleanup, delete `kind` cluster `monolift-e2e` or orphaned `mlv2-*` namespaces before the next run.
- [ ] R.8: Stage all remote logs, generated artifacts, coverage reports, and target-analysis evidence under `.moab/runs/sprint-0050-*` on the build node.

## Acceptance Criteria

Minimum:

- [ ] `docs/research/runs/SPRINT-0050-stage-binding.md` exists and records explicit rulings for direct-invoke result shapes, parent-over-leaf admission, and workload fitness.
- [ ] `docs/research/runs/SPRINT-0050-candidates.md` exists and justifies selected and declined candidates.
- [ ] At least one DB/SQL corpus trace is pushed toward stage 10; if it cannot reach stage 10, the exact binding stage and reason are documented against the stage-binding doc.
- [ ] At least one new filesystem/object-store reconstructor lands with tests and decision-log context.
- [x] At least one filesystem/object-store corpus trace reaches stage 7 with a real reconstructed durable root.
- [x] No target is counted as a persistence proof unless the selected cut actually exercises the intended durable resource.
- [x] CloudLab verification logs are stored under `.moab/runs/sprint-0050-*`.
- [ ] No full e2e sweep or bundled e2e regex is run.

Target:

- [ ] `miniflux/M-1` reaches stage 10 with workload evidence, env-off/fail-mode checks, dormant invariant, and transcript compare or declared substitute.
- [x] The filesystem/object-store primary reaches stage 7 or later with shared durable root proof and path-safety coverage.
- [x] One stretch candidate reaches stage 4 or later without violating scope.
- [ ] Manifest drift from SPRINT-0049 is reconciled.

Stretch:

- [x] Filesystem/object-store primary reaches stage 10.
- [ ] A second DB/SQL or filesystem trace reaches stage 7 or later.
- [ ] SQLite research identifies a clean future target that avoids `core.App` reconstruction.

## Risks

| Risk | Mitigation |
|---|---|
| Phase 0 becomes a research sprint and blocks implementation. | The decision summary is the gate; if it drags, keep `miniflux/M-1` and one PocketBase filesystem target only. |
| Workload/calls-delta becomes a loophole around stage 8-10 proof. | Allow substitutions only through typed target metadata and predicates recorded in the stage-binding doc. |
| Filesystem proof accidentally uses per-pod ephemeral state. | Shared durable root is a hard requirement; per-pod `emptyDir` does not count. |
| `miniflux/M-1` localized-error behavior is a deeper codec issue, not just harness policy. | Inspect the envelope before coding; choose between probe support, typed expectation, or workload-substitute deliberately. |
| Admission-rerank changes destabilize existing targets. | Keep deepest-admissible behavior opt-in unless Phase 0 and tests justify a default change. |
| PocketBase filesystem cuts pull in app-owned state. | Gate scaffolding on activation-path evidence; fall back to `archive.Create` if `CreateThumb` is too app-bound. |
| SQLite/App work expands into whole-app reconstruction. | Keep SQLite in research/backlog unless a non-app boundary admits cleanly. |
| Gitea/Listmonk/Mattermost consume the sprint. | Treat them as one optional stretch slot after primary proof is stable. |
| Transcript comparison is unstable for stateful workloads. | Use fresh resources, explicit normalizers, and declared behavioral invariants rather than ad hoc skips. |
| Generator version/goldens drift. | Bump with the first generated-output change and verify the codegen suite on CloudLab. |

## References

- `docs/sprints/SPRINT-0048.md`
- `docs/sprints/SPRINT-0049.md`
- `docs/specs/e2e-test-strategy.md`
- `test/e2e/activation_corpus_traces.yaml`
- `pkg/codegen/recon.go`
- `pkg/codegen/server.go`
- `pkg/codegen/kubernetes.go`
- `test/e2e/e2e_test.go`
- `test/e2e/harness/target.go`
- `scripts/run_activation_corpus_sweep.sh`
