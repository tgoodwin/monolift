# gitea/M-11 - Issue indexer handler

## Header

- Trace ID: `gitea/M-11`
- Project: `gitea`
- Region root: `modules/indexer/issues/indexer.go:166`
- Path length: 8
- Source trace: `projects/gitea/traces/M-11.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `RunMainApp` | `direct-function-call` | Very-large | Trivial | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 2 | `closure / dispatch site` | `method-call-on-concrete-type` | Very-large | Trivial | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 3 | `runWeb` | `function-value-in-struct-field` | Large | Serializable | Shared-state | Low | OK | Weak | Feasible |
| 4 | `serveInstalled` | `direct-function-call` | Medium | Trivial | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 5 | `InitWebInstalled` | `direct-function-call` | Medium | Serializable | Shared-state | 0 (estimated) | Infeasible | Anti | Infeasible |
| 6 | `Init` | `function-value-passed-as-argument` | Small | Trivial | Client-reconstructible | Low | OK | Weak | Feasible |
| 7 | `InitIssueIndexer` | `direct-function-call` | Small | Trivial | Client-reconstructible | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 8 | `getIssueIndexerQueueHandler` | `direct-function-call` | Minimal | Infeasible | Client-reconstructible | Low | Needs-wrapper | Anti | Infeasible |

## Recommended Cut

Cut at step 7, `InitIssueIndexer`. This point keeps extraction surface at `Small`, avoids hard-gated boundary values, and leaves state reconstruction at `Client-reconstructible`. The inspected path reaches `modules/indexer/issues/indexer.go:166`, and this candidate is the latest feasible boundary before the region root or at the root itself.

## Tension Notes

The tradeoff is surface area versus state reconstruction. The cut avoids the large framework prefix, but the remote side must provide `Client-reconstructible` state for the extracted code.

## Observations

- Recommended source evidence: `func InitIssueIndexer(syncReindex bool)`.
