# gitea/M-5 - Code indexer (`index`)

## Header

- Trace ID: `gitea/M-5`
- Project: `gitea`
- Region root: `modules/indexer/code/indexer.go:41`
- Path length: 9
- Source trace: `projects/gitea/traces/M-5.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `runWeb` | `function-value-in-struct-field` | Very-large | Serializable | Shared-state | Low | OK | Weak | Feasible |
| 2 | `serveInstalled` | `direct-function-call` | Very-large | Trivial | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 3 | `InitWebInstalled` | `direct-function-call` | Large | Serializable | Shared-state | 0 (estimated) | Infeasible | Anti | Infeasible |
| 4 | `mustInit` | `function-value-as-parameter` | Large | Trivial | Shared-state | Low | OK | Weak | Feasible |
| 5 | `Init` | `function-value-call-via-parameter` | Medium | Trivial | Client-reconstructible | Low | OK | Weak | Feasible |
| 6 | `Init` | `direct-function-call` | Medium | Serializable | Client-reconstructible | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 7 | `w.origHandler = handler` | `function-value-in-struct-field` | Small | Proxy-required | Client-reconstructible | Moderate | OK | Weak | Feasible-with-proxy |
| 8 | `registered` | `function-value-in-struct-field` | Small | Serializable | Client-reconstructible | 0 (estimated) | Needs-wrapper | Weak | Feasible |
| 9 | `index` | `direct-function-call` | Minimal | Serializable | Client-reconstructible | 0 (confirmed) | OK | Anti | Feasible |

## Recommended Cut

Cut at step 8, `registered`. This point keeps extraction surface at `Small`, avoids hard-gated boundary values, and leaves state reconstruction at `Client-reconstructible`. The inspected path reaches `modules/indexer/code/indexer.go:41`, and this candidate is the latest feasible boundary before the region root or at the root itself.

## Tension Notes

The tradeoff is surface area versus state reconstruction. The cut avoids the large framework prefix, but the remote side must provide `Client-reconstructible` state for the extracted code.

## Observations

- At least one candidate carries proxy-required data, most often an HTTP writer, stream, channel, or queue runtime object.
- Recommended source evidence: `func Init()`.
