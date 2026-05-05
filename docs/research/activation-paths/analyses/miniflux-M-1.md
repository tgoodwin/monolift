# miniflux/M-1 - Full feed refresh (`RefreshFeed`)

## Header

- Trace ID: `miniflux/M-1`
- Project: `miniflux`
- Region root: `RefreshFeed` at `internal/reader/handler/handler.go:207`
- Path length: 5
- Source trace: `projects/miniflux/traces/M-1.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `Parse` | `direct-function-call` | Very-large | Trivial | Shared-state | 0 (estimated) | Infeasible | Anti | Infeasible |
| 2 | `refreshFeeds` | `direct-function-call` | Large | Reconstructible | Client-reconstructible | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 3 | worker closure | `goroutine-launch` | Medium | Proxy-required | Shared-state | Low | Needs-wrapper | Anti | Feasible-with-proxy |
| 4 | channel receive / job dispatch | `channel-send-receive` | Medium | Serializable | Client-reconstructible | 0 (estimated) | Needs-wrapper | Strong | Feasible |
| 5 | `RefreshFeed` | `direct-function-call` | Small | Reconstructible | Client-reconstructible | 0 (confirmed) | OK | Anti | Feasible |

## Recommended Cut

Cut at step 5, `RefreshFeed(store *storage.Storage, userID, feedID int64, forceRefresh bool)`. The remote side needs its own database handle and HTTP clients, but the call boundary is otherwise primitive feed identity plus a boolean and the function already returns a localized error wrapper that callers handle. This avoids splitting the worker goroutine or proxying the channel.

## Tension Notes

Step 4 has the best edge signal because `model.Job` is a real queue payload crossing a channel, but cutting there extracts the worker loop and logging around `RefreshFeed`. Step 5 sacrifices the channel boundary signal for smaller surface, simpler error semantics, and no channel proxy.

## Observations

- The brief's scoring for steps 2, 3, and 5 matches the source-backed shape: `refreshFeeds` is broad, the goroutine body is an anti-boundary, and `RefreshFeed` is the preferred deep cut.
- `storage.Storage` wraps `*sql.DB`; it is client-reconstructible, not serializable. The remote instance should open its own DB pool from config.
- I added an explicit score for step 4 because the trace records `channel-send-receive`. It is feasible and strong, but not recommended because the adjacent named function cut is smaller.
