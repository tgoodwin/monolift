# gitea/M-15 - Mirror LFS sync (`StoreMissingLfsObjectsInRepository`)

## Header

- Trace ID: `gitea/M-15`
- Project: `gitea`
- Region root: `modules/repository/repo.go:61`
- Path length: 12
- Source trace: `projects/gitea/traces/M-15.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `runWeb` | `function-value-in-struct-field` | Very-large | Serializable | Shared-state | Low | OK | Weak | Feasible |
| 2 | `serveInstalled` | `direct-function-call` | Very-large | Trivial | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 3 | `MirrorSync` | `function-value-in-routing-table` | Very-large | Serializable | Client-reconstructible | Low | Needs-wrapper | Weak | Feasible |
| 4 | `addMirrorToQueue` | `direct-function-call` | Large | Trivial | Client-reconstructible | Moderate | Needs-wrapper | Anti | Feasible |
| 5 | `PushToQueue` | `goroutine-launch` + `closure-capture` | Large | Trivial | Client-reconstructible | Low | OK | Anti | Feasible |
| 6 | `(*WorkerPoolQueue[*SyncRequest]).Push` | `method-call-on-concrete-type` + `generic-instantiation` | Medium | Proxy-required | Client-reconstructible | Moderate | OK | Anti | Feasible-with-proxy |
| 7 | `safeHandler` | `channel-send-receive` | Medium | Proxy-required | Client-reconstructible | Moderate | OK | Strong | Feasible-with-proxy |
| 8 | `queueHandler` | `function-value-in-struct-field` | Medium | Serializable | Client-reconstructible | Low | Needs-wrapper | Weak | Feasible |
| 9 | `doMirrorSync` | `direct-function-call` | Small | Serializable | Client-reconstructible | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 10 | `SyncPullMirror` | `enum-tag-dispatch` | Small | Serializable | Client-reconstructible | Low | Needs-wrapper | Weak | Feasible |
| 11 | `runSync` | `direct-function-call` | Small | Serializable | Client-reconstructible | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 12 | `StoreMissingLfsObjectsInRepository` | `direct-function-call` | Minimal | Reconstructible | Client-reconstructible | 0 (confirmed) | OK | Anti | Feasible |

## Recommended Cut

Cut at step 8, `queueHandler`. This point keeps extraction surface at `Medium`, avoids hard-gated boundary values, and leaves state reconstruction at `Client-reconstructible`. The inspected path reaches `modules/repository/repo.go:61`, and this candidate is the latest feasible boundary before the region root or at the root itself.

## Tension Notes

The main tension is natural-boundary signal versus data/state cost. Step 7 has a strong `channel-send-receive` edge, but it scores `Proxy-required` boundary data and `Client-reconstructible` state. Step 8 is later and smaller, so the recommendation prioritizes lift feasibility over edge taxonomy.

## Observations

- The path includes a goroutine launch; those rows are treated as anti-boundaries unless followed by a queue/channel payload boundary.
- At least one candidate carries proxy-required data, most often an HTTP writer, stream, channel, or queue runtime object.
- The trace contains at least one strong edge signal, but the recommendation also accounts for boundary data and state cost.
- Recommended source evidence: `func queueHandler(items ...*SyncRequest) []*SyncRequest`.
