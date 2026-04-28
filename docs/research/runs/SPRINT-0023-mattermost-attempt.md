# SPRINT-0023 Mattermost attempt

## Verdict

Branch: **R**. The machinery lands for the toy stream-proxy and manifest cases,
but Mattermost stops at characterized compiler gaps before the S branch emission
attempt.

## Gap 1: session surface not derived from route binding

- Triggering shape:
  - `channels/api4/websocket.go:54` binds `connectWebSocket`.
  - `channels/api4/websocket.go:64` calls `upgrader.Upgrade(w, r, nil)`.
  - The Hub/WebConn region closure contains WebConn pump/router websocket
    machinery, but the current surface pass resolves only one region entry point
    and classifies it as `Call` / `httpjson`.
- Distribution feasibility:
  - The external API is the HTTP websocket route, not the internal Hub lifecycle
    methods alone. Stream-proxy remains the right wire shape once the compiler
    ties the route entry point to the Hub/WebConn region.
- Classification: tooling immaturity.
- Follow-up:
  - Extend surface derivation to include route-bound external handlers that
    construct or register region values, then choose streamproxy from that
    external route surface.

## Gap 2: boot path does not recover Mattermost startup config

- Triggering shape:
  - The direct Mattermost boot probe completed in 55.41s / 2.35GB RSS, so scale
    is acceptable.
  - Captured BootSpec has 1,030 literal sources, 4 plugin-related dependency
    inits, 7 goroutine launches, and 0 refusals.
  - It does not recover `MM_SQLSETTINGS_DATASOURCE`, broader `MM_*` env sources,
    `--config`, `config.json`, or the expected
    `app.New() -> server.New() -> platform.NewService() -> HubsStart()` chain.
- Distribution feasibility:
  - Mattermost's startup configuration is externally representable, but the
    current reverse walk is not yet reconstructing the command/server
    initialization path that supplies the Hub/WebConn instances.
- Classification: tooling immaturity.
- Follow-up:
  - Replace the current union-plus-main scan with an actual reverse path search
    from the route/region construction sites back to `cmd/mattermost/main.go`,
    preserving constructor order and config-source evidence.

## Dominant gap

The dominant gap is compiler reachability modeling around Mattermost's external
route and boot construction path. The stream-proxy mechanism itself passed the
toy byte-flow, byte-parity, lifetime, auth header, fail-open/closed, and
internal-Service tests; Mattermost needs better route-to-region and boot-chain
derivation before emission is honest.
