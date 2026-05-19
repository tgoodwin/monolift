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

## Residual Backlog

- `reader_read_all` input adapter pattern.
- Staged-object transport for payloads above the 8 MiB inline ceiling.
- Removal of `MONOLIFT_BOUNDARY_ADAPTER` after the documented two-release
  cleanup window.
- Additional corpus candidates that may become adapter-recovered once more
  pattern families exist.
