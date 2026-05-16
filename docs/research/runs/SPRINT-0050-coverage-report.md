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
