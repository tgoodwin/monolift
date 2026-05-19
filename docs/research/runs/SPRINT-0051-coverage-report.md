# SPRINT-0051 Coverage Report

## Summary

Before SPRINT-0051, `listmonk/M-4` was an admission skip that climbed toward
`(*App).UploadMedia`. After boundary-adapter recovery, it selects
`processImage`, records `boundary_class: AdapterPossible`, and reaches stage 10.

Oracle policy: direct PNG byte comparison for thumbnail bytes, with original
width and height compared as scalar DTO fields.

## CloudLab Artifacts

Experiment: `tgoodwin-305638` (`monolift-buildserver`).

Artifact directory on the build node:

```text
/local/repository/.moab/runs/sprint-0051-processimage/
```

Stage ladder commands used one exact `go test` process per stage:

```sh
MONOLIFT_E2E=1 MONOLIFT_E2E_STOP_STAGE=4  go test -tags=e2e -run '^TestE2E/activation-listmonk-processimage$' ./test/e2e -timeout=30m -count=1
MONOLIFT_E2E=1 MONOLIFT_E2E_STOP_STAGE=5  go test -tags=e2e -run '^TestE2E/activation-listmonk-processimage$' ./test/e2e -timeout=45m -count=1
MONOLIFT_E2E=1 MONOLIFT_E2E_STOP_STAGE=6  go test -tags=e2e -run '^TestE2E/activation-listmonk-processimage$' ./test/e2e -timeout=45m -count=1
MONOLIFT_E2E=1 MONOLIFT_E2E_STOP_STAGE=7  go test -tags=e2e -run '^TestE2E/activation-listmonk-processimage$' ./test/e2e -timeout=45m -count=1
MONOLIFT_E2E=1 MONOLIFT_E2E_STOP_STAGE=8  go test -tags=e2e -run '^TestE2E/activation-listmonk-processimage$' ./test/e2e -timeout=45m -count=1
MONOLIFT_E2E=1 MONOLIFT_E2E_STOP_STAGE=9  go test -tags=e2e -run '^TestE2E/activation-listmonk-processimage$' ./test/e2e -timeout=45m -count=1
MONOLIFT_E2E=1 MONOLIFT_E2E_STOP_STAGE=10 go test -tags=e2e -run '^TestE2E/activation-listmonk-processimage$' ./test/e2e -timeout=45m -count=1
```

Successful logs include `stage-4-rerun8.log`, `stage-5-rerun.log`,
`stage-6.log`, `stage-7.log`, `stage-8-rerun3.log`, `stage-9-rerun.log`, and
`stage-10.log`.

Closeout package tests:

```text
/local/repository/.moab/runs/sprint-0051-closeout/pkg-activation-codegen-harness-rerun.log
```

Command:

```sh
go test ./pkg/activation/... ./pkg/codegen/... ./test/e2e/harness/... -count=1
```

Adjacent e2e regressions, each run as one exact target with
`MONOLIFT_BOUNDARY_ADAPTER=1`:

```text
/local/repository/.moab/runs/sprint-0051-regression/activation-listmonk-sanitizeuri.log
/local/repository/.moab/runs/sprint-0051-regression/activation-miniflux-refreshfeed.log
/local/repository/.moab/runs/sprint-0051-regression/activation-pocketbase-createthumb.log
```

Admission-only sweeps:

```text
/local/repository/.moab/runs/sprint-0051-admission-flag-off-baseline-manifest/
/local/repository/.moab/runs/sprint-0051-admission-flag-off/
/local/repository/.moab/runs/sprint-0051-admission-flag-on/
```

The flag-off sweep against the pre-Phase-6 manifest produced the SPRINT-0050
baseline counts exactly: `8 pass`, `12 admission-skip`, `52 manifest-skip`, and
zero build/e2e/timeout/infra failures. The current-manifest flag-off sweep
produced `9 pass`, `11 admission-skip`, `52 manifest-skip` because the manifest
now records the intended `listmonk/M-4` stage-10 row.

The flag-on current-manifest sweep produced `11 pass`, `9 admission-skip`,
`52 manifest-skip`. Besides the intended `listmonk/M-4` manifest flip,
`pocketbase/M-5` and `pocketbase/M-11` changed from
`callable_boundary_values` admission skips to admission passes with the adapter
flag on. They were recorded as incidental adapter-enabled candidates, not e2e
proofs.

No generated extracted deployment YAML under the SPRINT-0051 CloudLab artifact
directories contained `MONOLIFT_LIFT_*` environment variables.

## Residual Backlog

- `reader_read_all` input adapter pattern.
- Staged-object transport for payloads above the 8 MiB inline ceiling.
- Removal of `MONOLIFT_BOUNDARY_ADAPTER` after the documented two-release
  cleanup window.
- Additional corpus candidates that may become adapter-recovered once more
  pattern families exist.
