# miniflux/M-9 - Per-entry save fan-out (`SendEntry`)

## Header

- Trace ID: `miniflux/M-9`
- Project: `miniflux`
- Region root: `internal/integration/integration.go:41`
- Path length: 8
- Source trace: `projects/miniflux/traces/M-9.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `Parse` | `direct-function-call` | Very-large | Serializable | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 2 | `startDaemon` | `direct-function-call` | Large | Reconstructible | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 3 | `StartWebServer` | `direct-function-call` | Large | Reconstructible | Client-reconstructible | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 4 | `newRouter` | `direct-function-call` | Medium | Reconstructible | Client-reconstructible | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 5 | `Serve` | `direct-function-call` | Medium | Reconstructible | Client-reconstructible | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 6 | `(*handler).saveEntry` | `callback-registration` | Small | Proxy-required | Client-reconstructible | Moderate | Needs-wrapper | Strong | Feasible-with-proxy |
| 7 | `(*handler).saveEntry` | `interface-method-dispatch` | Small | Proxy-required | Client-reconstructible | Moderate | Needs-wrapper | Strong | Feasible-with-proxy |
| 8 | `SendEntry` | `goroutine-launch` | Minimal | Serializable | Client-reconstructible | 0 (confirmed) | Needs-wrapper | Anti | Feasible |

## Recommended Cut

Cut at step 8, `SendEntry`. This point keeps extraction surface at `Minimal`, avoids hard-gated boundary values, and leaves state reconstruction at `Client-reconstructible`. The inspected path reaches `internal/integration/integration.go:41`, and this candidate is the latest feasible boundary before the region root or at the root itself.

## Tension Notes

The main tension is natural-boundary signal versus data/state cost. Step 7 has a strong `interface-method-dispatch` edge, but it scores `Proxy-required` boundary data and `Client-reconstructible` state. Step 8 is later and smaller, so the recommendation prioritizes lift feasibility over edge taxonomy.

## Observations

- The path includes a goroutine launch; those rows are treated as anti-boundaries unless followed by a queue/channel payload boundary.
- At least one candidate carries proxy-required data, most often an HTTP writer, stream, channel, or queue runtime object.
- The trace contains at least one strong edge signal, but the recommendation also accounts for boundary data and state cost.
- Recommended source evidence: `func SendEntry(entry *model.Entry, userIntegrations *model.Integration)`.
