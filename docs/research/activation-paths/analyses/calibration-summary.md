# Calibration Summary

## Header

- Sprint: `SPRINT-0039`
- Calibration traces: `caddy/M-3`, `gitea/M-1`, `miniflux/M-1`
- Purpose: establish the scoring rubric used for the remaining corpus analyses.

## Methodology Check

The brief's worked examples are directionally correct: deep leaf cuts often dominate on extraction surface, boundary data, state, and callbacks, while interface, queue, and handler registration edges identify natural but sometimes heavier boundaries. The only material discrepancy found during source inspection is `caddy/M-3` step 10: `HTTPBasicAuth.Authenticate` takes `http.ResponseWriter`, so I score it `Proxy-required` rather than ordinary serializable request data.

## Decision Rules

| Question | Rule used for the corpus |
|---|---|
| `context.Context` | Score as `Serializable`; deadlines, values, and cancellation can be propagated with standard context metadata. Do not let context alone force proxy classification. |
| `http.ResponseWriter`, `io.Reader`, `io.Writer`, channels | Score as `Proxy-required`; these are live streams or synchronization objects. If a channel appears only as a queue payload boundary, score the payload type separately and mark the channel edge as strong. |
| Function values, closures, mutexes, runtime/process handles | Score as `Infeasible` when they must cross the boundary. If they are only internal to reconstructed remote state, classify the state cost instead. |
| Surface area without full callee counts | Use relative architectural position: CLI/server bootstrap is `Very-large`, app/router/framework dispatch is `Large`, middleware/worker runtime is `Medium`, service handler is `Small`, and leaf utility/algorithm code is `Minimal`. |
| Callback counts without full reverse graph | Use source inspection for obvious calls back above the cut. If none are visible but the code invokes captured handlers/hooks or framework callbacks, mark `Low` or `Moderate`. Use `0 (confirmed)` only for leaf code with no observed reverse calls. |
| Reconstructible vs. shared state | DB pools, HTTP clients, mailers, loggers, and queue clients rebuilt from config are `Client-reconstructible`. In-memory caches, app/server structs with mutable lifecycle state, plugin registries, cancellation managers, and request writers are `Shared-state` or proxy state. |
| Error semantics | `OK` when the candidate returns `error` or a project-specific error object that the caller already handles. `Needs-wrapper` when network failure must be encoded into a boolean/void/localized wrapper path. `Infeasible` when the caller cannot reasonably observe or propagate failure without changing broad control flow. |
| Edge alignment | Treat exact strong edges from the plan as `Strong`. Treat HTTP server callbacks through handler fields and callback-registration variants as `Strong`. Treat struct-field/function-argument storage as `Weak`. Treat direct calls, concrete method calls, goroutine launches, closure captures, and reflective calls as `Anti` unless the trace evidence shows an explicit handler/queue registration contract. |

## Composite Feasibility

`Feasible` means no hard-gated boundary value crosses the network and error semantics are preserveable with ordinary wrapping. `Feasible-with-proxy` means the cut requires a streaming or synchronization proxy, most commonly `http.ResponseWriter`, `io.Reader`/`Writer`, or a channel. `Infeasible` means the cut requires sending a function value, closure, mutex/process handle, or introduces unobservable network failure in a void lifecycle path.

## Calibration Outcomes

- `caddy/M-3`: recommended step 11, with a note that step 10 would be attractive if rewritten into a verification-only boundary.
- `gitea/M-1`: recommended step 12, preserving the queue work-item contract while avoiding extraction of the worker-pool runtime.
- `miniflux/M-1`: recommended step 5, because the named feed refresh function is smaller than the channel receive boundary and avoids splitting the goroutine.

These rules are the rubric for Phases 1-6 and the aggregation dimensions in Phases 7-8.
