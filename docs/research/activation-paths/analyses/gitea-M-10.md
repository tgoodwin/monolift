# gitea/M-10 - Push-update worker (`pushUpdates`)

## Header

- Trace ID: `gitea/M-10`
- Project: `gitea`
- Region root: `services/repository/push.go:77`
- Path length: 15
- Source trace: `projects/gitea/traces/M-10.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `RunMainApp` | `direct-function-call` | Very-large | Trivial | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 2 | `runWeb` | `function-value-in-struct-field` | Very-large | Serializable | Shared-state | Low | OK | Weak | Feasible |
| 3 | `serveInstalled` | `direct-function-call` | Very-large | Trivial | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 4 | `InitWebInstalled` | `direct-function-call` | Large | Serializable | Shared-state | 0 (estimated) | Infeasible | Anti | Infeasible |
| 5 | `mustInitCtx` | `direct-function-call` | Large | Infeasible | Shared-state | 0 (estimated) | OK | Anti | Infeasible |
| 6 | `Init` | `function-value-parameter-invocation` | Large | Serializable | Shared-state | Low | OK | Weak | Feasible |
| 7 | `initPushQueue` | `direct-function-call` | Medium | Trivial | Shared-state | Moderate | OK | Anti | Feasible |
| 8 | `w.origHandler = handler` | `function-value-stored-in-struct-field` | Medium | Proxy-required | Client-reconstructible | Moderate | OK | Weak | Feasible-with-proxy |
| 9 | `RunWithCancel` | `goroutine-launch` | Medium | Proxy-required | Client-reconstructible | Moderate | Infeasible | Anti | Infeasible |
| 10 | `(*WorkerPoolQueue[T]).Run` | `interface-method-dispatch` | Medium | Proxy-required | Client-reconstructible | Moderate | Needs-wrapper | Strong | Feasible-with-proxy |
| 11 | `worker` | `goroutine-launch` | Small | Proxy-required | Client-reconstructible | Moderate | Infeasible | Anti | Infeasible |
| 12 | `q.doWorkerHandle` | `channel-send-receive` | Small | Proxy-required | Client-reconstructible | Moderate | Needs-wrapper | Strong | Feasible-with-proxy |
| 13 | `safeHandler` | `function-value-in-struct-field` | Small | Proxy-required | Client-reconstructible | Moderate | OK | Weak | Feasible-with-proxy |
| 14 | `handler` | `function-value-in-struct-field` | Small | Serializable | Client-reconstructible | Low | Needs-wrapper | Weak | Feasible |
| 15 | `pushUpdates` | `direct-function-call` | Minimal | Serializable | Client-reconstructible | 0 (confirmed) | OK | Anti | Feasible |

## Recommended Cut

Cut at step 14, `handler`. This point keeps extraction surface at `Small`, avoids hard-gated boundary values, and leaves state reconstruction at `Client-reconstructible`. The inspected path reaches `services/repository/push.go:77`, and this candidate is the latest feasible boundary before the region root or at the root itself.

## Tension Notes

The main tension is natural-boundary signal versus data/state cost. Step 12 has a strong `channel-send-receive` edge, but it scores `Proxy-required` boundary data and `Client-reconstructible` state. Step 14 is later and smaller, so the recommendation prioritizes lift feasibility over edge taxonomy.

## Observations

- The path includes a goroutine launch; those rows are treated as anti-boundaries unless followed by a queue/channel payload boundary.
- At least one candidate carries proxy-required data, most often an HTTP writer, stream, channel, or queue runtime object.
- The trace contains at least one strong edge signal, but the recommendation also accounts for boundary data and state cost.
- Recommended source evidence: `func handler(items ...[]*repo_module.PushUpdateOptions) [][]*repo_module.PushUpdateOptions`.
