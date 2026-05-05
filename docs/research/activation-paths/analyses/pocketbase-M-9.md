# pocketbase/M-9 - OAuth2 avatar download (`safeFileFromURL`)

## Header

- Trace ID: `pocketbase/M-9`
- Project: `pocketbase`
- Region root: `apis/record_auth_with_oauth2.go:468`
- Path length: 16
- Source trace: `projects/pocketbase/traces/M-9.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `(*PocketBase).Start` | `method-call-on-concrete-type` | Very-large | Trivial | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 2 | `NewServeCommand` | `direct-function-call` | Large | Trivial | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 3 | `RunE` | `struct-literal-field-assignment` | Large | Serializable | Shared-state | Low | Needs-wrapper | Weak | Feasible |
| 4 | `pb.RootCmd.Execute()` | `goroutine-launch` | Large | Trivial | Shared-state | Moderate | OK | Anti | Feasible |
| 5 | `RunE` | `function-value-in-struct-field` | Large | Serializable | Shared-state | Low | Needs-wrapper | Weak | Feasible |
| 6 | `Serve` | `direct-function-call` | Large | Trivial | Config-only | 0 (estimated) | OK | Anti | Feasible |
| 7 | `NewRouter` | `direct-function-call` | Large | Trivial | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 8 | `bindRecordAuthApi` | `direct-function-call` | Medium | Serializable | Client-reconstructible | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 9 | `closure / dispatch site` | `callback-registration` | Medium | Infeasible | Shared-state | Low | OK | Strong | Infeasible |
| 10 | `recordAuthWithOAuth2` | `function-value-in-struct-field` | Medium | Serializable | Config-only | Low | OK | Weak | Feasible |
| 11 | `(*BaseApp).OnRecordAuthWithOAuth2Request` | `interface-method-dispatch` | Medium | Reconstructible | Config-only | Low | Needs-wrapper | Strong | Feasible |
| 12 | `one` | `callback-argument-dispatch` | Small | Trivial | Config-only | Low | OK | Weak | Feasible |
| 13 | `oauth2Submit` | `direct-function-call` | Small | Trivial | Config-only | 0 (estimated) | OK | Anti | Feasible |
| 14 | `(*BaseApp).RunInTransaction` | `interface-method-dispatch` | Small | Infeasible | Client-reconstructible | Low | OK | Strong | Infeasible |
| 15 | `tx` | `callback-argument-dispatch` | Small | Trivial | Config-only | Low | OK | Weak | Feasible |
| 16 | `safeFileFromURL` | `direct-function-call` | Minimal | Reconstructible | Config-only | 0 (confirmed) | OK | Anti | Feasible |

## Recommended Cut

Cut at step 16, `safeFileFromURL`. This point keeps extraction surface at `Minimal`, avoids hard-gated boundary values, and leaves state reconstruction at `Config-only`. The inspected path reaches `apis/record_auth_with_oauth2.go:468`, and this candidate is the latest feasible boundary before the region root or at the root itself.

## Tension Notes

The main tension is natural-boundary signal versus data/state cost. Step 14 has a strong `interface-method-dispatch` edge, but it scores `Infeasible` boundary data and `Client-reconstructible` state. Step 16 is later and smaller, so the recommendation prioritizes lift feasibility over edge taxonomy.

## Observations

- The path includes a goroutine launch; those rows are treated as anti-boundaries unless followed by a queue/channel payload boundary.
- The trace contains at least one strong edge signal, but the recommendation also accounts for boundary data and state cost.
- Recommended source evidence: `func safeFileFromURL(ctx context.Context, url string) (*filesystem.File, error)`.
- PocketBase hook/router edges are natural dispatch points, but app and request event receivers often carry shared framework state.
