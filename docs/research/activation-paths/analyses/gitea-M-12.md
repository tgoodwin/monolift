# gitea/M-12 - Language statistics (`GetLanguageStats`)

## Header

- Trace ID: `gitea/M-12`
- Project: `gitea`
- Region root: `modules/git/languagestats/language_stats_nogogit.go:22`
- Path length: 10
- Source trace: `projects/gitea/traces/M-12.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `runWeb` | `function-value-in-struct-field` | Very-large | Serializable | Shared-state | Low | OK | Weak | Feasible |
| 2 | `serveInstalled` | `direct-function-call` | Very-large | Trivial | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 3 | `InitWebInstalled` | `direct-function-call` | Large | Serializable | Shared-state | 0 (estimated) | Infeasible | Anti | Infeasible |
| 4 | `Init` | `function-passed-as-parameter-then-invoked` | Large | Trivial | Shared-state | Low | OK | Weak | Feasible |
| 5 | `Init` | `direct-function-call` | Medium | Trivial | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 6 | `populateRepoIndexer` | `goroutine-launch` | Medium | Serializable | Client-reconstructible | Moderate | Infeasible | Anti | Infeasible |
| 7 | `(*WorkerPoolQueue[int64]).doWorkerHandle` | `asynchronous-queue-handoff` | Medium | Proxy-required | Client-reconstructible | Moderate | Needs-wrapper | Anti | Feasible-with-proxy |
| 8 | `handler` | `function-value-in-struct-field` | Small | Serializable | Client-reconstructible | Low | OK | Weak | Feasible |
| 9 | `(*DBIndexer).Index` | `interface-method-dispatch` | Small | Trivial | Client-reconstructible | 0 (estimated) | OK | Strong | Feasible |
| 10 | `GetLanguageStats` | `direct-function-call` | Minimal | Reconstructible | Client-reconstructible | 0 (confirmed) | OK | Anti | Feasible |

## Recommended Cut

Cut at step 8, `handler`. This point keeps extraction surface at `Small`, avoids hard-gated boundary values, and leaves state reconstruction at `Client-reconstructible`. The inspected path reaches `modules/git/languagestats/language_stats_nogogit.go:22`, and this candidate is the latest feasible boundary before the region root or at the root itself.

## Tension Notes

The main tension is natural-boundary signal versus data/state cost. Step 9 has a strong `interface-method-dispatch` edge, but it scores `Trivial` boundary data and `Client-reconstructible` state. Step 8 is later and smaller, so the recommendation prioritizes lift feasibility over edge taxonomy.

## Observations

- The path includes a goroutine launch; those rows are treated as anti-boundaries unless followed by a queue/channel payload boundary.
- At least one candidate carries proxy-required data, most often an HTTP writer, stream, channel, or queue runtime object.
- The trace contains at least one strong edge signal, but the recommendation also accounts for boundary data and state cost.
- Recommended source evidence: `func handler(items ...int64) []int64`.
