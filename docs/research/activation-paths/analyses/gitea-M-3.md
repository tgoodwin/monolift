# gitea/M-3 - Avatar image processing (`ProcessAvatarImage`)

## Header

- Trace ID: `gitea/M-3`
- Project: `gitea`
- Region root: `modules/avatar/avatar.go:101`
- Path length: 7
- Source trace: `projects/gitea/traces/M-3.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `runWeb` | `function-value-in-struct-field` | Very-large | Serializable | Shared-state | Low | OK | Weak | Feasible |
| 2 | `serveInstalled` | `direct-function-call` | Large | Trivial | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 3 | `NormalRoutes` | `direct-function-call` | Large | Serializable | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 4 | `Routes` | `direct-function-call` | Medium | Serializable | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 5 | `UpdateAvatar` | `http-handler-registration` | Medium | Serializable | Client-reconstructible | Low | Needs-wrapper | Strong | Feasible |
| 6 | `UploadAvatar` | `direct-function-call` | Small | Serializable | Client-reconstructible | 0 (estimated) | OK | Anti | Feasible |
| 7 | `ProcessAvatarImage` | `direct-function-call` | Minimal | Serializable | Stateless | 0 (confirmed) | OK | Anti | Feasible |

## Recommended Cut

Cut at step 5, `UpdateAvatar`. This point keeps extraction surface at `Medium`, avoids hard-gated boundary values, and leaves state reconstruction at `Client-reconstructible`. The inspected path reaches `modules/avatar/avatar.go:101`, and this candidate is the latest feasible boundary before the region root or at the root itself.

## Tension Notes

The tradeoff is surface area versus state reconstruction. The cut avoids the large framework prefix, but the remote side must provide `Client-reconstructible` state for the extracted code.

## Observations

- The trace contains at least one strong edge signal, but the recommendation also accounts for boundary data and state cost.
- Recommended source evidence: `func UpdateAvatar(ctx *context.APIContext)`.
