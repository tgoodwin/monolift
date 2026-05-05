# gitea/M-14 - Actions workflow detection (`DetectWorkflows`)

## Header

- Trace ID: `gitea/M-14`
- Project: `gitea`
- Region root: `modules/actions/workflows.go:120`
- Path length: 9
- Source trace: `projects/gitea/traces/M-14.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `runWeb` | `function-value-in-struct-field` | Very-large | Serializable | Shared-state | Low | OK | Weak | Feasible |
| 2 | `(*Router).ServeHTTP` | `direct-function-call` | Very-large | Proxy-required | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Feasible-with-proxy |
| 3 | `CreateIssue` | `callback-registration` | Large | Serializable | Shared-state | Low | Needs-wrapper | Strong | Feasible |
| 4 | `NewIssue` | `direct-function-call` | Large | Reconstructible | Client-reconstructible | 0 (estimated) | OK | Anti | Feasible |
| 5 | `NewIssue` | `direct-function-call` | Medium | Serializable | Client-reconstructible | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 6 | `(*actionsNotifier).NewIssue` | `interface-method-dispatch` | Medium | Serializable | Client-reconstructible | Low | Needs-wrapper | Strong | Feasible |
| 7 | `(*notifyInput).Notify` | `method-call-on-concrete-type` | Small | Serializable | Client-reconstructible | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 8 | `notify` | `direct-function-call` | Small | Serializable | Client-reconstructible | 0 (estimated) | OK | Anti | Feasible |
| 9 | `DetectWorkflows` | `direct-function-call` | Minimal | Reconstructible | Client-reconstructible | 0 (confirmed) | OK | Anti | Feasible |

## Recommended Cut

Cut at step 9, `DetectWorkflows`. This point keeps extraction surface at `Minimal`, avoids hard-gated boundary values, and leaves state reconstruction at `Client-reconstructible`. The inspected path reaches `modules/actions/workflows.go:120`, and this candidate is the latest feasible boundary before the region root or at the root itself.

## Tension Notes

The main tension is natural-boundary signal versus data/state cost. Step 6 has a strong `interface-method-dispatch` edge, but it scores `Serializable` boundary data and `Client-reconstructible` state. Step 9 is later and smaller, so the recommendation prioritizes lift feasibility over edge taxonomy.

## Observations

- At least one candidate carries proxy-required data, most often an HTTP writer, stream, channel, or queue runtime object.
- The trace contains at least one strong edge signal, but the recommendation also accounts for boundary data and state cost.
- Recommended source evidence: `func DetectWorkflows( gitRepo *git.Repository, commit *git.Commit, triggedEvent webhook_module.HookEventType, payload api.Payloader, detectSchedule bool, ) ([]*DetectedWorkflow, []*DetectedWorkflow, error)`.
