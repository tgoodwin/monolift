# SPRINT-0050 Coverage Report

## Phase 4 Filesystem Target

`pocketbase/M-1` (`(*filesystem.System).CreateThumb`) now has a focused e2e
target at `activation-pocketbase-createthumb`.

Proof reached:

- Stage 4 passed: `.moab/runs/sprint-0050-pocketbase-createthumb/stage4-auth-20260516-165128.log`
- Stage 5 passed: `.moab/runs/sprint-0050-pocketbase-createthumb/stage5-20260516-165435.log`
- Stage 6 passed: `.moab/runs/sprint-0050-pocketbase-createthumb/stage6-20260516-165900.log`
- Stage 7 passed: `.moab/runs/sprint-0050-pocketbase-createthumb/stage7-hostpath-20260516-171957.log`

Runtime proof kind: extracted service reconstructs a PocketBase
`*filesystem.System` from `MONOLIFT_FILESYSTEM_ROOT` and shares a durable local
root with the lifted host. The CloudLab Kind cluster does not dynamically
provision the generated PVC, so the e2e target uses an explicit hostPath-backed
shared root. This is still a shared node-level durable root, not per-pod
`emptyDir`.

Stage 8 stop:

- `.moab/runs/sprint-0050-pocketbase-createthumb/stage8-seed-filename-20260516-174618.log`
- The lifted fresh-upload workload returned the original `300x300` image rather
  than the expected `100x100` thumbnail.
- Manual debug deployment from the generated artifacts verified that seeded
  direct `/invoke` and the seeded PocketBase file route both return `100x100`
  thumbnails and record clean extracted invocations.
- Classification: workload/runtime binding blocker for the fresh-upload
  harness path, not an admission, build, image-load, or stage-7 reconstructor
  deployment failure.
