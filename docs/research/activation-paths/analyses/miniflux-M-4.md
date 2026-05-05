# miniflux/M-4 - Integration push fan-out (`PushEntries`)

## Header

- Trace ID: `miniflux/M-4`
- Project: `miniflux`
- Region root: `internal/integration/integration.go:511`
- Path length: 5
- Source trace: `projects/miniflux/traces/M-4.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `Parse` | `direct-function-call` | Very-large | Serializable | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 2 | `refreshFeeds` | `direct-function-call` | Large | Reconstructible | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 3 | `worker` | `goroutine-launch-of-anonymous-closure` | Medium | Proxy-required | Client-reconstructible | Moderate | Infeasible | Anti | Infeasible |
| 4 | `RefreshFeed` | `direct-function-call` | Small | Reconstructible | Client-reconstructible | 0 (estimated) | OK | Anti | Feasible |
| 5 | `PushEntries` | `goroutine-launch-of-named-function` | Minimal | Serializable | Client-reconstructible | 0 (confirmed) | Needs-wrapper | Anti | Feasible |

## Recommended Cut

Cut at step 5, `PushEntries`. This point keeps extraction surface at `Minimal`, avoids hard-gated boundary values, and leaves state reconstruction at `Client-reconstructible`. The inspected path reaches `internal/integration/integration.go:511`, and this candidate is the latest feasible boundary before the region root or at the root itself.

## Tension Notes

The tradeoff is surface area versus state reconstruction. The cut avoids the large framework prefix, but the remote side must provide `Client-reconstructible` state for the extracted code.

## Observations

- The path includes a goroutine launch; those rows are treated as anti-boundaries unless followed by a queue/channel payload boundary.
- At least one candidate carries proxy-required data, most often an HTTP writer, stream, channel, or queue runtime object.
- Recommended source evidence: `func PushEntries(feed *model.Feed, entries model.Entries, userIntegrations *model.Integration)`.
