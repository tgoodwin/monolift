# gitea/M-8 - Renderable git diff (`GetDiffForRender`)

## Header

- Trace ID: `gitea/M-8`
- Project: `gitea`
- Region root: `services/gitdiff/gitdiff.go:1333`
- Path length: 7
- Source trace: `projects/gitea/traces/M-8.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `runWeb` | `indirect-call-through-struct-field` | Very-large | Serializable | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 2 | `serveInstalled` | `direct-function-call` | Large | Trivial | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 3 | `NormalRoutes` | `direct-function-call` | Large | Serializable | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 4 | `Routes` | `direct-function-call` | Medium | Serializable | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 5 | `registerWebRoutes` | `direct-function-call` | Medium | Serializable | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 6 | `Diff` | `reflect-call-on-function-value` | Small | Serializable | Client-reconstructible | 0 (estimated) | Needs-wrapper | Weak | Feasible |
| 7 | `GetDiffForRender` | `direct-function-call` | Minimal | Reconstructible | Client-reconstructible | 0 (confirmed) | OK | Anti | Feasible |

## Recommended Cut

Cut at step 7, `GetDiffForRender`. This point keeps extraction surface at `Minimal`, avoids hard-gated boundary values, and leaves state reconstruction at `Client-reconstructible`. The inspected path reaches `services/gitdiff/gitdiff.go:1333`, and this candidate is the latest feasible boundary before the region root or at the root itself.

## Tension Notes

The tradeoff is surface area versus state reconstruction. The cut avoids the large framework prefix, but the remote side must provide `Client-reconstructible` state for the extracted code.

## Observations

- Recommended source evidence: `func GetDiffForRender(ctx context.Context, repoLink string, gitRepo *git.Repository, opts *DiffOptions, files ...string) (*Diff, error)`.
