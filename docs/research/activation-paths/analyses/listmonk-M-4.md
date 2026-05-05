# listmonk/M-4 - Image thumbnail generation (`processImage`)

## Header

- Trace ID: `listmonk/M-4`
- Project: `listmonk`
- Region root: `cmd/media.go:212`
- Path length: 4
- Source trace: `projects/listmonk/traces/M-4.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `initHTTPServer` | `direct-function-call` | Very-large | Reconstructible | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 2 | `initHTTPHandlers` | `direct-function-call` | Medium | Reconstructible | Shared-state | Low | Needs-wrapper | Anti | Feasible |
| 3 | `(*App).UploadMedia` | `callback-registration` (method-value + closure-wrapper)` | Small | Reconstructible | Client-reconstructible | Low | OK | Strong | Feasible |
| 4 | `processImage` | `direct-function-call` | Minimal | Proxy-required | Config-only | 0 (confirmed) | OK | Anti | Feasible-with-proxy |

## Recommended Cut

Cut at step 3, `(*App).UploadMedia`. This point keeps extraction surface at `Small`, avoids hard-gated boundary values, and leaves state reconstruction at `Client-reconstructible`. The inspected path reaches `cmd/media.go:212`, and this candidate is the latest feasible boundary before the region root or at the root itself.

## Tension Notes

The tradeoff is surface area versus state reconstruction. The cut avoids the large framework prefix, but the remote side must provide `Client-reconstructible` state for the extracted code.

## Observations

- At least one candidate carries proxy-required data, most often an HTTP writer, stream, channel, or queue runtime object.
- The trace contains at least one strong edge signal, but the recommendation also accounts for boundary data and state cost.
- Recommended source evidence: `func (a *App) UploadMedia(c echo.Context) error`.
- Listmonk's smaller codebase reduces surface-area pressure; boundary data and client reconstruction dominate the decision.
