# pocketbase/M-2 - OAuth2 outbound exchange (`recordAuthWithOAuth2`)

## Header

- Trace ID: `pocketbase/M-2`
- Project: `pocketbase`
- Region root: `apis/record_auth_with_oauth2.go:30`
- Path length: 8
- Source trace: `projects/pocketbase/traces/M-2.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `(*PocketBase).Start` | `method-call-on-concrete-type` | Very-large | Trivial | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 2 | `RunE` | `function-value-in-struct-field` | Large | Serializable | Shared-state | Low | Needs-wrapper | Weak | Feasible |
| 3 | `RunE` | `function-value-in-struct-field` | Large | Serializable | Shared-state | Low | Needs-wrapper | Weak | Feasible |
| 4 | `Serve` | `direct-function-call` | Medium | Trivial | Config-only | 0 (estimated) | OK | Anti | Feasible |
| 5 | `NewRouter` | `direct-function-call` | Medium | Trivial | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 6 | `bindRecordAuthApi` | `direct-function-call` | Small | Serializable | Client-reconstructible | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 7 | `Route.Action = action` | `function-value-in-struct-field` | Small | Infeasible | Shared-state | 0 (estimated) | OK | Weak | Infeasible |
| 8 | `recordAuthWithOAuth2` | `function-value-as-argument` | Minimal | Trivial | Config-only | 0 (confirmed) | OK | Weak | Feasible |

## Recommended Cut

Cut at step 8, `recordAuthWithOAuth2`. This point keeps extraction surface at `Minimal`, avoids hard-gated boundary values, and leaves state reconstruction at `Config-only`. The inspected path reaches `apis/record_auth_with_oauth2.go:30`, and this candidate is the latest feasible boundary before the region root or at the root itself.

## Tension Notes

The recommended cut has no major Pareto conflict: it is late in the path, has preserveable errors, and does not require proxying live request or stream state.

## Observations

- Recommended source evidence: `func recordAuthWithOAuth2(e *core.RequestEvent) error`.
- PocketBase hook/router edges are natural dispatch points, but app and request event receivers often carry shared framework state.
