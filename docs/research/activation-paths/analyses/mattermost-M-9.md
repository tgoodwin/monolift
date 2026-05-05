# mattermost/M-9 - Recap channel processing (`ProcessRecapChannel`)

## Header

- Trace ID: `mattermost/M-9`
- Project: `mattermost`
- Region root: `server/channels/app/recap.go:185`
- Path length: 13
- Source trace: `projects/mattermost/traces/M-9.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `Run` | `direct-function-call` | Very-large | Serializable | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 2 | `serverCmdF` | `function-value-in-struct-field` | Very-large | Serializable | Shared-state | Low | OK | Weak | Feasible |
| 3 | `runServer` | `direct-function-call` | Very-large | Proxy-required | Shared-state | 0 (estimated) | OK | Anti | Feasible-with-proxy |
| 4 | `NewServer` | `direct-function-call` | Large | Reconstructible | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 5 | `(*Server).initJobs` | `method-call-on-concrete-type` | Large | Reconstructible | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 6 | `MakeWorker` | `direct-function-call` | Medium | Reconstructible | Shared-state | Moderate | Needs-wrapper | Anti | Feasible |
| 7 | `SimpleWorker.execute` | `closure-capture-into-struct-field` | Medium | Infeasible | Shared-state | Low | OK | Anti | Infeasible |
| 8 | `workers.workers[name] = worker` | `interface-upcast-into-container` | Medium | Serializable | Client-reconstructible | Moderate | Needs-wrapper | Weak | Feasible |
| 9 | `(*Workers).Start` | `method-call-on-concrete-type` | Medium | Serializable | Client-reconstructible | Moderate | Needs-wrapper | Anti | Feasible |
| 10 | `(*SimpleWorker).Run` | `interface-method-dispatch` + `goroutine-launch` | Small | Proxy-required | Client-reconstructible | Moderate | Infeasible | Strong | Infeasible |
| 11 | `(*SimpleWorker).DoJob` | `channel-send-receive` | Small | Serializable | Client-reconstructible | Moderate | Needs-wrapper | Strong | Feasible |
| 12 | `execute` | `function-value-in-struct-field` | Small | Reconstructible | Shared-state | 0 (estimated) | Needs-wrapper | Weak | Feasible |
| 13 | `processRecapJob` | `direct-function-call` | Minimal | Infeasible | Client-reconstructible | 0 (confirmed) | OK | Anti | Infeasible |
| 14 | `(*App).ProcessRecapChannel` | `interface-method-dispatch` | Minimal | Reconstructible | Shared-state | 0 (estimated) | Needs-wrapper | Strong | Feasible |

## Recommended Cut

Cut at step 12, `execute`. This point keeps extraction surface at `Small`, avoids hard-gated boundary values, and leaves state reconstruction at `Shared-state`. The inspected path reaches `server/channels/app/recap.go:185`, and this candidate is the latest feasible boundary before the region root or at the root itself.

## Tension Notes

The main tension is natural-boundary signal versus data/state cost. Step 14 has a strong `interface-method-dispatch` edge, but it scores `Reconstructible` boundary data and `Shared-state` state. Step 12 is later and smaller, so the recommendation prioritizes lift feasibility over edge taxonomy.

## Observations

- The path includes a goroutine launch; those rows are treated as anti-boundaries unless followed by a queue/channel payload boundary.
- At least one candidate carries proxy-required data, most often an HTTP writer, stream, channel, or queue runtime object.
- The trace contains at least one strong edge signal, but the recommendation also accounts for boundary data and state cost.
- Recommended source evidence: `func MakeWorker(jobServer *jobs.JobServer, storeInstance store.Store, appInstance AppIface) *jobs.SimpleWorker`.
- Mattermost `App`/server receivers are scored conservatively as shared state unless the cut is an isolated algorithmic helper.
