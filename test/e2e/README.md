# Monolift v2 E2E Harness

This harness validates the Monolift v2 compiler contract against real Go
targets using a local Kind cluster. The test-only compile driver in
`test/e2e/e2ecompile/` produces closure reports and lifted artifacts from the
real compiler.

## Prerequisites

- Go 1.23.6+
- Docker daemon
- `kind`
- `kubectl`

## One-Command Run

```sh
make e2e
```

The target runs:

```sh
MONOLIFT_E2E=1 go test -tags=e2e -v ./test/e2e/... -timeout=30m
```

`go test ./...` without the `e2e` tag does not run the Kind-backed tests.

## Environment

- `MONOLIFT_E2E=1` enables the e2e rows. Without it, rows skip with
  `MONOLIFT_E2E=1 required`.
- `MONOLIFT_E2E_KEEP=1` leaves namespaces in place for debugging.
- `MONOLIFT_COMPILER=<path>` selects the compiler binary. Default:
  `./bin/e2e-compile`.
- `MONOLIFT_E2E_UPDATE_GOLDEN=1` enables golden updates.

## Failure Messages

Every actionable failure starts with:

```text
[stage=N target=X kind=(harness|compiler|artifact|workload)]
```

Kinds mean:

- `harness`: Kind, Kubernetes, namespace, apply, or test orchestration.
- `compiler`: compiler exit, report parsing, verdict, or golden mismatch.
- `artifact`: Docker build or Kind image load.
- `workload`: HTTP workload execution or transcript comparison.

Stages follow the strategy doc: setup, baseline deploy/workload, compile,
report, artifact build/load, lifted deploy/workload, compare, cleanup.

## Add A Target

1. Add `test/e2e/targets/<name>/target.go` returning a `harness.TargetCase`.
2. Add baseline manifests under `targets/<name>/baseline/` if the target runs
   past stage 0.
3. Add `workload.go` implementing `harness.WorkloadExecutor`.
4. Add `golden/report.json` using the `reportv2` schema.
5. Import the target package in `test/e2e/e2e_test.go` and add it to the table.

Deferred rows should set `SkipReason` with the sprint that will activate them.

## Update Goldens

```sh
make e2e-update-golden
```

When the normative report subset differs, the harness writes the current report
to the target golden and exits non-zero so the diff must be reviewed before
commit.

## Debug A Target

Use:

```sh
MONOLIFT_E2E_KEEP=1 make e2e
```

Then inspect the namespaces named:

```text
mlv2-baseline-<target>-<runid>
mlv2-lifted-<target>-<runid>
```

Artifacts and compiler output live under:

```text
/tmp/monolift-e2e/<target>/<runid>/
```

Use `make e2e-clean` to remove local artifact dumps and `make e2e-reset` to
recreate the Kind cluster.
