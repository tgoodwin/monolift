# gitea/M-7 - PR mergeability check (`checkPullRequestMergeable`)

## Header

- Trace ID: `gitea/M-7`
- Project: `gitea`
- Region root: `services/pull/check.go:427`
- Path length: 6
- Source trace: `projects/gitea/traces/M-7.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `RunMainApp` | `direct-function-call` | Very-large | Trivial | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 2 | `runWeb` | `function-value-in-struct-field` | Large | Serializable | Shared-state | Low | OK | Weak | Feasible |
| 3 | `serveInstalled` | `direct-function-call` | Medium | Trivial | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 4 | `InitWebInstalled` | `direct-function-call` | Medium | Serializable | Shared-state | 0 (estimated) | Infeasible | Anti | Infeasible |
| 5 | `Init` | `function-value-parameter-call` | Small | Trivial | Client-reconstructible | 0 (estimated) | OK | Weak | Feasible |
| 6 | `checkPullRequestMergeable` | `direct-function-call` | Minimal | Trivial | Client-reconstructible | 0 (confirmed) | Needs-wrapper | Anti | Feasible |

## Recommended Cut

Cut at step 6, `checkPullRequestMergeable`. This point keeps extraction surface at `Minimal`, avoids hard-gated boundary values, and leaves state reconstruction at `Client-reconstructible`. The inspected path reaches `services/pull/check.go:427`, and this candidate is the latest feasible boundary before the region root or at the root itself.

## Tension Notes

The tradeoff is surface area versus state reconstruction. The cut avoids the large framework prefix, but the remote side must provide `Client-reconstructible` state for the extracted code.

## Observations

- Recommended source evidence: `func checkPullRequestMergeable(id int64)`.
