# SPRINT-0050 Coverage Report

## Phase 4 Filesystem Target

`pocketbase/M-1` (`(*filesystem.System).CreateThumb`) now has a focused e2e
target at `activation-pocketbase-createthumb`.

Proof reached:

- Stage 4 passed: `.moab/runs/sprint-0050-pocketbase-createthumb/stage4-auth-20260516-165128.log`
- Stage 5 passed: `.moab/runs/sprint-0050-pocketbase-createthumb/stage5-20260516-165435.log`
- Stage 6 passed: `.moab/runs/sprint-0050-pocketbase-createthumb/stage6-20260516-165900.log`
- Stage 7 passed: `.moab/runs/sprint-0050-pocketbase-createthumb/stage7-hostpath-20260516-171957.log`
- Stage 8 passed: `.moab/runs/sprint-0050-pocketbase-createthumb-debug/stage8-shared-data-20260516-181457.log`
- Stage 9 passed: `.moab/runs/sprint-0050-pocketbase-createthumb-debug/stage9-restore-retry-20260516-183205.log`
- Stage 10 passed: `.moab/runs/sprint-0050-pocketbase-createthumb-debug/stage10-shared-data-20260516-183727.log`

Runtime proof kind: extracted service reconstructs a PocketBase
`*filesystem.System` from `MONOLIFT_FILESYSTEM_ROOT` and shares a durable local
root with the lifted host. The CloudLab Kind cluster does not dynamically
provision the generated PVC, so the e2e target uses an explicit hostPath-backed
shared root under the Kind workers' shared `/data` mount. This is still a
shared durable root, not per-pod `emptyDir`.

Resolved Stage 8 blocker:

- `.moab/runs/sprint-0050-pocketbase-createthumb/stage8-seed-filename-20260516-174618.log`
- The lifted fresh-upload workload returned the original `300x300` image rather
  than the expected `100x100` thumbnail because the host and extracted pods were
  scheduled on different Kind workers while the generated hostPath was
  node-local under `/tmp`.
- The extracted invocation record contained `stat /monolift/durable/...: no
  such file or directory`, proving that the extracted service could not see the
  freshly uploaded original.
- The target now uses `/data/monolift-e2e/pocketbase-createthumb-durable-root`,
  which is backed by the same host directory on every Kind worker in
  `test/e2e/fixtures/kind-config.yaml`.

Stage 9 policy note:

- PocketBase fail-closed mode returns a non-5xx fallback response with the
  original image when thumbnail generation reports the extracted-service error.
  The harness now allows targets to define a fail-closed request variant without
  weakening the normal Stage 8 behavioral predicate.
- Restore checks retry transient workload failures while Kubernetes endpoint
  propagation catches up after scaling the extracted service back to one replica.

## Phase 5 Stretch Research

Research timeout policy:

- Internal caps are cost instrumentation, not viability decisions. If a cap
  fires during research, rerun with the cap disabled or widened before
  classifying the candidate.
- Candidate blockers must be semantic or runtime facts: wrong selected cut,
  shared-state/app receiver, unsupported shape, missing reconstructor, generated
  build failure, fixture scope, or violated workload contract.

Stretch result:

- `gitea/M-9` (`services/packages/rpm.BuildSpecificRepositoryFiles`) reached
  focused Kind e2e stage 7 with target `activation-gitea-rpmrepo`.
- Earlier source-local stage 4/generation log:
  `.moab/runs/sprint-0050-phase5/gitea-rpmrepo-stage4-clean-lift-20260516-192525.log`
- Earlier source-local stage 5 extracted build log:
  `.moab/runs/sprint-0050-phase5/gitea-rpmrepo-stage5-build-20260516-193842.log`
- Focused e2e stage 4 log:
  `.moab/runs/sprint-0050-gitea-rpmrepo/stage4-20260516-195418.log`
- Focused e2e stage 5 log:
  `.moab/runs/sprint-0050-gitea-rpmrepo/stage5-20260516-200736.log`
- Focused e2e stage 6 log:
  `.moab/runs/sprint-0050-gitea-rpmrepo/stage6-20260516-202500.log`
- Focused e2e stage 7 log:
  `.moab/runs/sprint-0050-gitea-rpmrepo/stage7-20260516-204212.log`
- Stage 7 Kubernetes evidence:
  `.moab/runs/sprint-0050-gitea-rpmrepo/stage7-kubectl-20260516-205745.log`
- Selected cut:
  `code.gitea.io/gitea/services/packages/rpm.BuildSpecificRepositoryFiles` at
  `services/packages/rpm/repository.go:163`.
- Runtime evidence: the lifted namespace reached ready deployments for
  `gitea-lifted`, `monolift-extracted-gitea-rpmrepo`, and `postgres`. The
  stage-7 workload is intentionally health/readiness only, so this is a
  generated deployment proof, not an RPM metadata behavior proof.
- Cost profile: the stage-7 run passed in about 15.0m. Activation took about
  11m29s, dominated by about 9m13s in augmentation. Extraction report took
  about 23s, build-plan about 12s, and patch-function about 13s.
- Stage 8+ blocker: not a timeout and not runtime readiness. A meaningful RPM
  metadata workload still needs Gitea package DB rows, RPM metadata inputs,
  package blob storage, and an upload or metadata rebuild action. That fixture
  work remains too large for the Phase 5 stretch slot.

Deferred stretch candidates:

- `gitea/M-19` has the analogous Debian repository metadata cut, but Phase 5
  promoted only the RPM target.
- `listmonk/M-4` remains deferred. Provider-level filesystem probes were cheap,
  but selected cuts climbed back to a synthetic `UploadMedia$1` closure or a
  `*Manager` shared-state receiver.
- Mattermost filestore work remains deferred. The image path is rooted in
  `channels/app` upload state, and the remote-cluster path needs queue/channel
  dispatch plus generic HTTP-client behavior.
