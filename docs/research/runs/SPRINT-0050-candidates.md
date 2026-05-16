# SPRINT-0050 Candidates

**Date:** 2026-05-16
**Sprint:** SPRINT-0050
**Remote run root:** `/local/repository-codex-sprint0050/.moab/runs/`

## Decision Summary

DB/SQL primary:
`miniflux/M-1` `RefreshFeed`. Focused admission accepts the intended cut with
three boundary params, one reconstructed param, and one result. SPRINT-0049
already proved stage 7 with a real Postgres-backed `*storage.Storage`; SPRINT-0050
should push the direct-invoke policy and stage 8-10 proof path.

Filesystem/object-store primary:
`pocketbase/M-1` `(*filesystem.System).CreateThumb`. Focused admission reaches
the intended durable-resource receiver and refuses because `*System` has
non-serializable fields. That is the missing reconstructor family this sprint is
meant to add, not a reason to reconstruct the whole app. The narrow constructor
shape is visible in PocketBase: `NewLocal(root)` builds `*System` from a local
root and `Close()` releases it.

Filesystem fallback:
`pocketbase/M-4` `archive.Create`. It admits and source-local lift records the
intended package-level cut at `tools/archive/create.go:18`. Use it only if
`CreateThumb` expands beyond `*filesystem.System` into app-owned state. The
fallback can prove shared-root filesystem behavior, but by itself it does not
prove a `*filesystem.System` reconstructor.

Stretch target:
No stretch target is promoted in Phase 0. `miniflux/M-5` remains parent-cut-only
evidence: source-local lift selects `RefreshFeed`, not `*iconChecker`.
`listmonk/M-4` is deferred because admission refuses `*App` shared state and
lift then fails on callable boundary values.

Research method:
Candidate decisions are based on focused scoped admission and source-local lift
artifacts, not whole-repository admission. The admission-only baseline is useful
only for manifest drift.

## Manifest Reconciliation

Admission-only baseline on 2026-05-16:
`/local/repository-codex-sprint0050/.moab/runs/sprint-0050-admission-baseline/`
reported:

| Status | Count |
|---|---:|
| pass | 5 |
| admission-skip | 12 |
| manifest-skip | 55 |
| timeout-skip | 0 |
| build/e2e/infra | 0 |

Rows needing reconciliation:

| Trace | Manifest / prior evidence | SPRINT-0050 ruling |
|---|---|---|
| `pocketbase/M-3` | SPRINT-0049 coverage says focused e2e reached stage 10; manifest still says `e2e-retry`. | Manifest drift. Keep as regression after harness/oracle changes; do not count as new persistence graduation. |
| `miniflux/M-6` | SPRINT-0049 baseline said admission-only pass and e2e stage 7; current manifest says admission-skip because the pipeline resolves the cut at caller `RefreshFeed`, not `ParseFeed`. | Manifest drift plus target-line mismatch. Do not count as a separate persistence proof. |
| `miniflux/M-5` | Manifest says pass/stage 4, but the generated cut is parent `RefreshFeed`. | Parent-cut-only. Do not count as icon/file leaf proof. |
| `gitea/M-16` | SPRINT-0049 focused e2e required `GITEA__security__PASSWORD_HASH_ALGO=argon2` to exercise the lifted symbol. | Workload-fitness metadata must include required config/env. |

## Focused Candidate Results

| Trace | Source target | Focused admission | Source-local lift | Cut fidelity | Ruling |
|---|---|---|---|---|---|
| `miniflux/M-1` | `internal/reader/handler/handler.go:207` | Accepted: `ADMITTED: RefreshFeed (boundary params: 3, reconstructed: 1, results: 1)`. | `lift_rc=0`; manifest cut `miniflux.app/v2/internal/reader/handler.RefreshFeed`. | Intended cut. | DB/SQL primary. |
| `miniflux/M-5` | `internal/reader/icon/checker.go:28` | Refused: `receiver_requires_reconstruction: receiver *iconChecker has state class SharedState`. | `lift_rc=0`; manifest cut is parent `RefreshFeed`. | Parent, not intended leaf. | Defer; do not count as icon/file proof. |
| `pocketbase/M-1` | `tools/filesystem/filesystem.go:489` | Refused: `receiver_requires_reconstruction: receiver *System has non-serializable fields`. | `lift_rc=1` during admit-candidate. | Intended durable receiver reached, missing reconstructor. | Filesystem/object-store primary for reconstructor implementation. |
| `pocketbase/M-4` | `tools/archive/create.go:18` | Accepted: `ADMITTED: Create (boundary params: 3, reconstructed: 0, results: 1)`. | `lift_rc=0`; manifest cut `github.com/pocketbase/pocketbase/tools/archive.Create`. | Intended package-level cut. | Filesystem fallback if `CreateThumb` proves too broad. |
| `listmonk/M-4` | `cmd/media.go` | Refused: `receiver_requires_reconstruction: receiver *App has state class SharedState`. | `lift_rc=1`; callable boundary values required. | App-owned state. | Defer. |

The earlier focused lift run wrote output outside the source module and failed
with `generated_path_outside_module`; the source-local rerun is the lift
evidence used above.

## Candidate Notes

### `miniflux/M-1` `RefreshFeed`

- Signature/resource shape: DB-backed refresh path through reconstructed
  `*storage.Storage`.
- Current status: stage 7 from SPRINT-0049, focused admission and source-local
  lift still select `RefreshFeed`.
- Acceptance contract: workload issues `PUT /v1/feeds/{id}/refresh`; extracted
  service `/calls` delta increases; host API observes refreshed feed entries;
  direct invoke uses `nullable-localized-error`; env-off records no extracted
  calls; fail-mode behavior follows the declared client policy; fresh Postgres
  state is used per stage; transcript comparison may normalize IDs and
  timestamps.
- Rationale: shortest DB/SQL path to stage 10. The known blocker is harness
  expectation, not DB reconstruction.

### `pocketbase/M-1` `(*filesystem.System).CreateThumb`

- Signature/resource shape: local/blob filesystem receiver plus serializable
  keys and thumb size. `filesystem.NewLocal(root)` constructs the local backend.
- Current status: focused admission refuses because the receiver needs a
  reconstructor.
- Acceptance contract: seed an original image object under a shared durable
  root; invoke thumbnail creation with root-relative original and thumbnail
  keys; assert the thumbnail exists and has the expected image properties;
  direct invoke payloads must be root-relative; reject absolute paths and `..`
  traversal where payload paths are intended to stay under the root; env-off
  records no extracted calls; fail modes follow the declared client policy;
  cleanup uses a fresh root per run.
- Rationale: this is the real filesystem/object-store reconstructor target. It
  should stay narrow: reconstruct `*filesystem.System` from local root metadata,
  not `core.App` or app lifecycle state.

### `pocketbase/M-4` `archive.Create`

- Signature/resource shape: pure package function over source path, destination
  path, and skip paths. It uses local filesystem calls directly.
- Current status: focused admission and source-local lift both accept the
  intended cut.
- Acceptance contract if used: seed deterministic files under a shared durable
  root; call with root-relative source/destination; verify zip contents and skip
  behavior; reject unsafe path traversal; use fresh root per stage; env-off and
  fail-mode checks must still use `/calls` deltas.
- Rationale: good fallback for shared-root runtime proof, but not enough by
  itself to satisfy the `*filesystem.System` reconstructor goal.

### Deferred Candidates

`miniflux/M-5` remains useful for future parent-vs-leaf ranking work, but the
current selected cut is `RefreshFeed`, so it cannot prove `*iconChecker` or feed
icon handling.

`listmonk/M-4` is out of scope for SPRINT-0050 implementation. It requires
`*App` shared-state reconstruction and callable boundary values.
