# gitea/M-6 - Mirror pull sync (`runSync`)

## Header

- Trace ID: `gitea/M-6`
- Project: `gitea`
- Region root: `services/mirror/mirror_pull.go:109`
- Path length: 13
- Source trace: `projects/gitea/traces/M-6.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `RunMainApp` | `direct-function-call` | Very-large | Trivial | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 2 | `runWeb` | `function-value-in-struct-field` | Very-large | Serializable | Shared-state | Low | OK | Weak | Feasible |
| 3 | `serveInstalled` | `direct-function-call` | Very-large | Trivial | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 4 | `InitWebInstalled` | `direct-function-call` | Large | Serializable | Shared-state | 0 (estimated) | Infeasible | Anti | Infeasible |
| 5 | `InitSyncMirrors` | `direct-function-call` | Large | Serializable | Client-reconstructible | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 6 | `StartSyncMirrors` | `direct-function-call` | Medium | Serializable | Client-reconstructible | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 7 | `w.origHandler = handler` | `function-value-stored-in-struct-field` | Medium | Proxy-required | Client-reconstructible | Moderate | OK | Weak | Feasible-with-proxy |
| 8 | `(*Manager).RunWithCancel` | `goroutine-launch` | Medium | Proxy-required | Client-reconstructible | Moderate | Infeasible | Anti | Infeasible |
| 9 | `(*WorkerPoolQueue[*SyncRequest]).Run` | `interface-method-dispatch` | Medium | Proxy-required | Client-reconstructible | Moderate | Needs-wrapper | Strong | Feasible-with-proxy |
| 10 | `queueHandler` | `function-value-in-struct-field` | Small | Serializable | Client-reconstructible | Low | Needs-wrapper | Weak | Feasible |
| 11 | `doMirrorSync` | `direct-function-call` | Small | Serializable | Client-reconstructible | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 12 | `SyncPullMirror` | `enum-guarded-direct-call` | Small | Serializable | Client-reconstructible | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 13 | `runSync` | `direct-function-call` | Minimal | Serializable | Client-reconstructible | 0 (confirmed) | Needs-wrapper | Anti | Feasible |

## Recommended Cut

Cut at step 10, `queueHandler`. This point keeps extraction surface at `Small`, avoids hard-gated boundary values, and leaves state reconstruction at `Client-reconstructible`. The inspected path reaches `services/mirror/mirror_pull.go:109`, and this candidate is the latest feasible boundary before the region root or at the root itself.

## Tension Notes

The main tension is natural-boundary signal versus data/state cost. Step 9 has a strong `interface-method-dispatch` edge, but it scores `Proxy-required` boundary data and `Client-reconstructible` state. Step 10 is later and smaller, so the recommendation prioritizes lift feasibility over edge taxonomy.

## Observations

- The path includes a goroutine launch; those rows are treated as anti-boundaries unless followed by a queue/channel payload boundary.
- At least one candidate carries proxy-required data, most often an HTTP writer, stream, channel, or queue runtime object.
- The trace contains at least one strong edge signal, but the recommendation also accounts for boundary data and state cost.
- Recommended source evidence: `func queueHandler(items ...*SyncRequest) []*SyncRequest`.
