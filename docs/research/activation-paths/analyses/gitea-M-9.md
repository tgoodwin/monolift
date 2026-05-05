# gitea/M-9 - RPM repo metadata rebuild (`BuildSpecificRepositoryFiles`)

## Header

- Trace ID: `gitea/M-9`
- Project: `gitea`
- Region root: `services/packages/rpm/repository.go:163`
- Path length: 7
- Source trace: `projects/gitea/traces/M-9.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `RunMainApp` | `direct-function-call` | Very-large | Trivial | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 2 | `runWeb` | `function-value-in-struct-field` | Large | Serializable | Shared-state | Low | OK | Weak | Feasible |
| 3 | `serveInstalled` | `direct-function-call` | Large | Trivial | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 4 | `NormalRoutes` | `direct-function-call` | Medium | Serializable | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 5 | `CommonRoutes` | `direct-function-call` | Medium | Serializable | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 6 | `UploadPackageFile` | `http-handler-registration` | Small | Serializable | Client-reconstructible | Low | Needs-wrapper | Strong | Feasible |
| 7 | `BuildSpecificRepositoryFiles` | `direct-function-call` | Minimal | Reconstructible | Client-reconstructible | 0 (confirmed) | OK | Anti | Feasible |

## Recommended Cut

Cut at step 6, `UploadPackageFile`. This point keeps extraction surface at `Small`, avoids hard-gated boundary values, and leaves state reconstruction at `Client-reconstructible`. The inspected path reaches `services/packages/rpm/repository.go:163`, and this candidate is the latest feasible boundary before the region root or at the root itself.

## Tension Notes

The tradeoff is surface area versus state reconstruction. The cut avoids the large framework prefix, but the remote side must provide `Client-reconstructible` state for the extracted code.

## Observations

- The trace contains at least one strong edge signal, but the recommendation also accounts for boundary data and state cost.
- Recommended source evidence: `func UploadPackageFile(ctx *context.Context)`.
