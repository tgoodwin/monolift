# SPRINT-0023 Mattermost BootSpec capture

Source: `/tmp/monolift-s23-bootpath-probe/summary.json`, produced by the
B.gate-1 direct boot-path probe with
`GOWORK=$PWD/.tmp/sprint-0021-a1-go.work`.

## Summary

- Main package: `github.com/mattermost/mattermost/server/v8/cmd/mattermost`.
- Region roots: Hub/WebConn overlay from `test/e2e/targets/mattermost/pragma_overlay.go`.
- Union functions walked: 1,009.
- Entry-path items: 1,010.
- Surface result observed by current machinery: `Call` / `httpjson`.
- Entry points resolved by current surface pass: 1.
- Config sources:
  - `literal`: 1,030
  - `env`: 0
  - `flag`: 0
  - `file`: 0
  - `db`: 0
- Dependency inits: 4, all classified `disabled-by-minimal-config`.
- Goroutine launches: 7.
- Boot-path refusals: 0.

## Expected vs observed

Expected sprint signal was a boot path that recovered Mattermost startup config:
`MM_SQLSETTINGS_DATASOURCE`, broader `MM_*` env sources, `--config`, and
`config.json`, plus the initialization chain
`app.New() -> server.New() -> platform.NewService() -> HubsStart()`.

Observed signal does not recover that startup path. The current walker is bounded
well enough for Mattermost scale, but it is still walking the region union plus
`main.main` directly rather than reconstructing the reverse call path from
`cmd/mattermost/main.go` through command/server initialization into the
Hub/WebConn construction path.

This is tooling immaturity, not evidence that the Mattermost boot shape is
fundamentally undistributable.
