# SPRINT-0023 Mattermost boot-path probe

## Command

The probe was run with the SPRINT-0021 Mattermost workspace because
`evaluation/mattermost/server` depends on the local `server/public` submodule.

```sh
GOWORK=off go build -o /tmp/monolift-s23-bootpath-probe/probe ./.tmp/sprint-0023-bootpath-probe

GOWORK=$PWD/.tmp/sprint-0021-a1-go.work \
MONOLIFT_PROFILE_DIR=/tmp/monolift-s23-bootpath-probe \
/usr/bin/time -l timeout 1800 \
  /tmp/monolift-s23-bootpath-probe/probe \
  > /tmp/monolift-s23-bootpath-probe/summary.json
```

The scratch probe constructs the Hub/WebConn multi-root region, loads Mattermost
with `packages.Load`, builds SSA, derives the region surface, and runs
`bootpath.Walk` over the union closure.

## Result

- Status: success.
- Wall time: 55.41s.
- Max RSS: 2,346,369,024 bytes.
- Budget: 30 minutes wall / 16 GiB RSS.
- Main package: `github.com/mattermost/mattermost/server/v8/cmd/mattermost`.
- Union functions walked: 1,009.
- Boot entry-path items: 1,010.
- Config sources recorded: 1,030.
- Dependency inits recorded: 4.
- Goroutine launches recorded: 7.
- Boot-path refusals: 0.
- Summary JSON: `/tmp/monolift-s23-bootpath-probe/summary.json`.

The run is below both B.gate-1 limits. This is not Cliff 2.

## GOWORK validation

A normal e2e compile with the same workspace also completed successfully before
the direct boot-path probe:

```sh
GOWORK=$PWD/.tmp/sprint-0021-a1-go.work \
MONOLIFT_PROFILE_DIR=/tmp/monolift-s23-probe \
/usr/bin/time -l timeout 1800 ./bin/e2e-compile \
  --target=mattermost \
  --output=/tmp/monolift-s23-out/mattermost \
  --source=evaluation/mattermost/server \
  --source=test/e2e/targets/mattermost
```

- Status: success.
- Wall time: 71.52s.
- Max RSS: 2,155,298,816 bytes.
- Included symbols: 3,025.
- `go list` with the workspace loaded 139 Mattermost packages.

This confirms the earlier undefined `model`/`mlog` symbols were caused by
running without the Mattermost multi-module workspace.
