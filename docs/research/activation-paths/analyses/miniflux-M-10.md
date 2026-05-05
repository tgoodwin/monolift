# miniflux/M-10 - Feed subscription finder (`FindSubscriptions`)

## Header

- Trace ID: `miniflux/M-10`
- Project: `miniflux`
- Region root: `internal/reader/subscription/finder.go:44`
- Path length: 7
- Source trace: `projects/miniflux/traces/M-10.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `cli.Parse` | `direct-function-call` | Very-large | Serializable | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 2 | `startDaemon` | `direct-function-call` | Large | Reconstructible | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 3 | `StartWebServer` | `direct-function-call` | Large | Reconstructible | Client-reconstructible | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 4 | `newRouter` | `direct-function-call` | Medium | Reconstructible | Client-reconstructible | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 5 | `ui.Serve` | `direct-function-call` | Medium | Reconstructible | Client-reconstructible | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 6 | `(*handler).submitSubscription` | `http-handler-registration` | Small | Proxy-required | Shared-state | Moderate | Needs-wrapper | Strong | Feasible-with-proxy |
| 7 | `(*subscriptionFinder).FindSubscriptions` | `method-call-on-concrete-type` | Minimal | Trivial | Client-reconstructible | 0 (confirmed) | OK | Anti | Feasible |

## Recommended Cut

Cut at step 7, `(*subscriptionFinder).FindSubscriptions`. This point keeps extraction surface at `Minimal`, avoids hard-gated boundary values, and leaves state reconstruction at `Client-reconstructible`. The inspected path reaches `internal/reader/subscription/finder.go:44`, and this candidate is the latest feasible boundary before the region root or at the root itself.

## Tension Notes

The main tension is natural-boundary signal versus data/state cost. Step 6 has a strong `http-handler-registration` edge, but it scores `Proxy-required` boundary data and `Shared-state` state. Step 7 is later and smaller, so the recommendation prioritizes lift feasibility over edge taxonomy.

## Observations

- At least one candidate carries proxy-required data, most often an HTTP writer, stream, channel, or queue runtime object.
- The trace contains at least one strong edge signal, but the recommendation also accounts for boundary data and state cost.
- Recommended source evidence: `func (f *subscriptionFinder) FindSubscriptions(websiteURL, rssBridgeURL string, rssBridgeToken string) (Subscriptions, *locale.LocalizedErrorWrapper)`.
