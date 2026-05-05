# gitea/M-2 - Repository archive generator (`doArchive`)

## Header

- Trace ID: `gitea/M-2`
- Project: `gitea`
- Region root: `services/repository/archiver/archiver.go:146`
- Path length: 13
- Source trace: `projects/gitea/traces/M-2.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `RunMainApp` | `direct-function-call` | Very-large | Trivial | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 2 | `runWeb` | `function-value-in-struct-field` | Very-large | Serializable | Shared-state | Low | OK | Weak | Feasible |
| 3 | `serveInstalled` | `direct-function-call` | Very-large | Trivial | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 4 | `InitWebInstalled` | `direct-function-call` | Large | Serializable | Shared-state | 0 (estimated) | Infeasible | Anti | Infeasible |
| 5 | `Init` | `function-value-argument-call` | Large | Serializable | Shared-state | Low | OK | Weak | Feasible |
| 6 | `w.origHandler = handler` | `callback-registration` | Medium | Proxy-required | Shared-state | Moderate | OK | Strong | Feasible-with-proxy |
| 7 | `(*Manager).RunWithCancel` | `goroutine-launch` | Medium | Proxy-required | Shared-state | Moderate | Infeasible | Anti | Infeasible |
| 8 | `(*WorkerPoolQueue[T]).Run` | `interface-method-dispatch` | Medium | Proxy-required | Client-reconstructible | Moderate | Needs-wrapper | Strong | Feasible-with-proxy |
| 9 | `worker` | `goroutine-launch` | Medium | Proxy-required | Client-reconstructible | Moderate | Infeasible | Anti | Infeasible |
| 10 | `<-q.batchChan` | `channel-send-receive` | Small | Proxy-required | Client-reconstructible | Low | Needs-wrapper | Strong | Feasible-with-proxy |
| 11 | `safeHandler` | `function-value-in-struct-field` | Small | Proxy-required | Client-reconstructible | Moderate | OK | Weak | Feasible-with-proxy |
| 12 | `handler` | `function-value-in-struct-field` | Small | Serializable | Client-reconstructible | Low | OK | Weak | Feasible |
| 13 | `doArchive` | `direct-function-call` | Minimal | Serializable | Client-reconstructible | 0 (confirmed) | OK | Anti | Feasible |

## Recommended Cut

Cut at step 12, `handler`. This point keeps extraction surface at `Small`, avoids hard-gated boundary values, and leaves state reconstruction at `Client-reconstructible`. The inspected path reaches `services/repository/archiver/archiver.go:146`, and this candidate is the latest feasible boundary before the region root or at the root itself.

## Tension Notes

The main tension is natural-boundary signal versus data/state cost. Step 10 has a strong `channel-send-receive` edge, but it scores `Proxy-required` boundary data and `Client-reconstructible` state. Step 12 is later and smaller, so the recommendation prioritizes lift feasibility over edge taxonomy.

## Observations

- The path includes a goroutine launch; those rows are treated as anti-boundaries unless followed by a queue/channel payload boundary.
- At least one candidate carries proxy-required data, most often an HTTP writer, stream, channel, or queue runtime object.
- The trace contains at least one strong edge signal, but the recommendation also accounts for boundary data and state cost.
- Recommended source evidence: `func Init(ctx context.Context) error`.
