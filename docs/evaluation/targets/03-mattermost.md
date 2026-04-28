# Mattermost

SPRINT-0022 pins the Mattermost `Hub` / `WebConn` region as `connection-hub-buffer`.

Pragmas live in `test/e2e/targets/mattermost/pragma_overlay.go`; `evaluation/mattermost/` remains byte-identical. The region roots are Hub methods (`Start`, `Broadcast`, `Register`, `Unregister`, `CheckConn`, `SendMessage`, `ProcessAsync`, `Stop`) and WebConn methods (`Pump`, `writePump`).

The generated union closure includes `(*WebConn).writePump` with `WebConn` provenance. Symbol provenance is reachability-based and may include both Hub and WebConn on mutually referenced methods.

The SSA seam report records `WebConn.send` as the load-bearing channel-field seam with Hub writers and WebConn readers. Additional mutex and atomic seams are metadata only.

SPRINT-0022 lands branch (R): admission accepts the in-region channel seam, but emission stops at G.gate-1 because the liftpatch API only patches one free function per request and rejects receiver methods. The gap is documented in `docs/research/runs/SPRINT-0022-emission-gap.md`.

SPRINT-0023 lands the additive machinery that closed that patcher API gap:
`RegionPatchRequest`, boot-path extraction, manifest rendering, and a
stream-proxy emitter for session surfaces. Mattermost still lands branch (R).
The bounded boot-path probe completed under budget, but the current compiler
does not yet derive the websocket route as the external session surface and does
not reconstruct the Mattermost config/init chain from `cmd/mattermost/main.go`.
The gap is documented in `docs/research/runs/SPRINT-0023-mattermost-attempt.md`.

Union probe metrics: 3,025 included symbols, 4,889 excluded symbols, 127.96s wall, 2.07GB max RSS.
