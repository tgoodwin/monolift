# listmonk/M-5 - Bulk subscriber CSV ingest (`Session.LoadCSV`)

## Header

- Trace ID: `listmonk/M-5`
- Project: `listmonk`
- Region root: `internal/subimporter/importer.go:452`
- Path length: 4
- Source trace: `projects/listmonk/traces/M-5.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `initHTTPServer` | `direct-function-call` | Very-large | Reconstructible | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 2 | `initHTTPHandlers` | `direct-function-call` | Medium | Reconstructible | Shared-state | Low | Needs-wrapper | Anti | Feasible |
| 3 | `(*App).ImportSubscribers` | `http-handler-registration-via-wrapper-closure` | Small | Reconstructible | Client-reconstructible | Low | OK | Strong | Feasible |
| 4 | `(*Session).LoadCSV` | `goroutine-launch-on-concrete-method` | Minimal | Proxy-required | Client-reconstructible | 0 (confirmed) | OK | Anti | Feasible-with-proxy |

## Recommended Cut

Cut at step 3, `(*App).ImportSubscribers`. This point keeps extraction surface at `Small`, avoids hard-gated boundary values, and leaves state reconstruction at `Client-reconstructible`. The inspected path reaches `internal/subimporter/importer.go:452`, and this candidate is the latest feasible boundary before the region root or at the root itself.

## Tension Notes

The tradeoff is surface area versus state reconstruction. The cut avoids the large framework prefix, but the remote side must provide `Client-reconstructible` state for the extracted code.

## Observations

- The path includes a goroutine launch; those rows are treated as anti-boundaries unless followed by a queue/channel payload boundary.
- At least one candidate carries proxy-required data, most often an HTTP writer, stream, channel, or queue runtime object.
- The trace contains at least one strong edge signal, but the recommendation also accounts for boundary data and state cost.
- Recommended source evidence: `func (a *App) ImportSubscribers(c echo.Context) error`.
- Listmonk's smaller codebase reduces surface-area pressure; boundary data and client reconstruction dominate the decision.
