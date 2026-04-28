# SPRINT-0022 Mattermost pragma overlay

## Decision

Use `test/e2e/targets/mattermost/pragma_overlay.go` as a sidecar overlay. The
overlay is parsed by the e2e-compile pragma loader but is not compiled into the
Mattermost module. It exists only to declare the multi-root region while keeping
`evaluation/mattermost/` byte-identical.

## Resolver contract

The overlay declares peer pragmas on `Hub` and `WebConn` aliases in package
`platform`. Before extraction, the e2e-compile Mattermost loader must resolve
those overlay roots to the original source identities:

- `Hub` maps to
  `evaluation/mattermost/server/channels/app/platform/web_hub.go`, declaration
  `Hub`.
- `WebConn` maps to
  `evaluation/mattermost/server/channels/app/platform/web_conn.go`,
  declaration `WebConn`.

The shared region name and region-wide options come from the overlay pragmas.
The source filename, declaration name, declaration kind, and method list used by
closure analysis come from the real Mattermost declarations after alias
resolution.
