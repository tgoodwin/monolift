# miniflux/M-5 - Feed icon discovery+resize (`UpdateOrCreateFeedIcon`)

## Header

- Trace ID: `miniflux/M-5`
- Project: `miniflux`
- Region root: `internal/reader/icon/checker.go:28`
- Path length: 7
- Source trace: `projects/miniflux/traces/M-5.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `Parse` | `direct-function-call` | Very-large | Serializable | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 2 | `refreshFeeds` | `direct-function-call` | Large | Reconstructible | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 3 | `closure / dispatch site` | `goroutine-launch-of-closure` | Large | Proxy-required | Client-reconstructible | Moderate | Infeasible | Anti | Infeasible |
| 4 | `closure / dispatch site` | `channel-send-receive` | Medium | Serializable | Client-reconstructible | Low | Needs-wrapper | Strong | Feasible |
| 5 | `RefreshFeed` | `direct-function-call` | Medium | Reconstructible | Client-reconstructible | 0 (estimated) | OK | Anti | Feasible |
| 6 | `(*iconChecker).CreateFeedIconIfMissing` | `method-call-on-concrete-type` | Small | Serializable | Client-reconstructible | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 7 | `(*iconChecker).UpdateOrCreateFeedIcon` | `method-call-on-concrete-type` | Minimal | Serializable | Client-reconstructible | 0 (confirmed) | Needs-wrapper | Anti | Feasible |

## Recommended Cut

Cut at step 7, `(*iconChecker).UpdateOrCreateFeedIcon`. This point keeps extraction surface at `Minimal`, avoids hard-gated boundary values, and leaves state reconstruction at `Client-reconstructible`. The inspected path reaches `internal/reader/icon/checker.go:28`, and this candidate is the latest feasible boundary before the region root or at the root itself.

## Tension Notes

The main tension is natural-boundary signal versus data/state cost. Step 4 has a strong `channel-send-receive` edge, but it scores `Serializable` boundary data and `Client-reconstructible` state. Step 7 is later and smaller, so the recommendation prioritizes lift feasibility over edge taxonomy.

## Observations

- The path includes a goroutine launch; those rows are treated as anti-boundaries unless followed by a queue/channel payload boundary.
- At least one candidate carries proxy-required data, most often an HTTP writer, stream, channel, or queue runtime object.
- The trace contains at least one strong edge signal, but the recommendation also accounts for boundary data and state cost.
- Recommended source evidence: `func (c *iconChecker) UpdateOrCreateFeedIcon()`.
