# gitea/M-1 - Webhook delivery worker (`Deliver`)

## Header

- Trace ID: `gitea/M-1`
- Project: `gitea`
- Region root: `Deliver` at `services/webhook/deliver.go:153`
- Path length: 13
- Source trace: `projects/gitea/traces/M-1.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `RunMainApp` | `direct-function-call` | Very-large | Serializable | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 2 | `runWeb` | `function-value-in-struct-field` | Very-large | Serializable | Shared-state | Low | OK | Weak | Feasible |
| 3 | `serveInstalled` | `direct-function-call` | Very-large | Serializable | Shared-state | Low | OK | Anti | Feasible |
| 4 | `InitWebInstalled` | `direct-function-call` | Very-large | Serializable | Shared-state | Many | Infeasible | Anti | Infeasible |
| 5 | `webhook.Init` | `function-value-as-argument` | Very-large | Serializable | Shared-state | Many | OK | Weak | Feasible |
| 6 | `w.origHandler = handler` | `function-value-as-argument-stored-in-struct-field` | Large | Infeasible | Shared-state | Moderate | Needs-wrapper | Weak | Infeasible |
| 7 | `RunWithCancel` | `goroutine-launch` | Large | Reconstructible | Shared-state | Moderate | Infeasible | Anti | Infeasible |
| 8 | `(*WorkerPoolQueue[int64]).Run` | `interface-method-dispatch` | Large | Proxy-required | Shared-state | Moderate | Infeasible | Strong | Feasible-with-proxy |
| 9 | `doRun` | `method-call-on-concrete-type` | Large | Proxy-required | Shared-state | Moderate | Infeasible | Anti | Feasible-with-proxy |
| 10 | worker closure | `goroutine-launch` | Medium | Proxy-required | Shared-state | Moderate | Infeasible | Anti | Feasible-with-proxy |
| 11 | `safeHandler` closure | `function-value-in-struct-field` | Medium | Serializable | Client-reconstructible | Low | OK | Weak | Feasible |
| 12 | `handler` | `closure-capture-of-struct-field` | Small | Trivial | Client-reconstructible | Low | OK | Weak | Feasible |
| 13 | `Deliver` | `direct-function-call` | Minimal | Serializable | Client-reconstructible | 0 (confirmed) | OK | Anti | Feasible |

## Recommended Cut

Cut at step 12, `handler(items ...int64) []int64`. The boundary is queue work-item IDs in and retry IDs out, which are trivial to serialize, while the remote side can load `HookTask` records, use its own DB connection, and own the webhook HTTP client lifecycle. Step 13 is slightly smaller, but it requires passing and mutating a `*HookTask` object across the boundary and loses the queue handler's batch/retry contract.

## Tension Notes

The queue runtime at steps 8-11 is the natural architectural boundary, but the concrete `WorkerPoolQueue` receiver owns channels, mutexes, cancellation, and base queue state. The deepest `Deliver` cut wins on surface area and callbacks, but step 12 is the better Pareto point because it keeps queue semantics at the call boundary without extracting the worker-pool machinery.

## Observations

- The brief's step 5, 8, 12, and 13 profiles match the main shape: initialization is too broad, the queue boundary is natural but heavy, and the handler/leaf cuts are the competitive options.
- I score step 8 as `Proxy-required` because the `WorkerPoolQueue` receiver contains live channels, cancellation functions, mutex-protected counters, and a base queue implementation. That is not a normal serializable receiver even though the interface edge is strong.
- `context.Context` at `Deliver` is treated as serializable under the sprint rule; the expensive part is reconstructing DB and HTTP clients, not the context value itself.
- The handler logs and fetches task state from the DB before calling `Deliver`, so callback count is `Low` rather than confirmed zero for step 12.
