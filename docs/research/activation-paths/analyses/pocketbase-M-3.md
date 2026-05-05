# pocketbase/M-3 - Bcrypt password verify (`ValidatePassword`)

## Header

- Trace ID: `pocketbase/M-3`
- Project: `pocketbase`
- Region root: `core/field_password.go:317`
- Path length: 8
- Source trace: `projects/pocketbase/traces/M-3.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `(*PocketBase).Start` | `method-call-on-concrete-type` | Very-large | Trivial | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 2 | `RunE` | `function-value-in-struct-field` | Large | Serializable | Shared-state | Low | Needs-wrapper | Weak | Feasible |
| 3 | `Serve` | `direct-function-call` | Large | Trivial | Config-only | 0 (estimated) | OK | Anti | Feasible |
| 4 | `NewRouter` | `direct-function-call` | Medium | Trivial | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 5 | `bindRecordAuthApi` | `direct-function-call` | Medium | Serializable | Client-reconstructible | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 6 | `recordAuthWithOTP` | `http-handler-registration` | Small | Trivial | Client-reconstructible | Low | OK | Strong | Feasible |
| 7 | `(*Record).ValidatePassword` | `promoted-method-call-through-embedded-field` | Small | Serializable | Stateless | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 8 | `PasswordFieldValue.Validate` | `type-asserted-method-call` | Minimal | Trivial | Stateless | 0 (confirmed) | Needs-wrapper | Anti | Feasible |

## Recommended Cut

Cut at step 8, `PasswordFieldValue.Validate`. This point keeps extraction surface at `Minimal`, avoids hard-gated boundary values, and leaves state reconstruction at `Stateless`. The inspected path reaches `core/field_password.go:317`, and this candidate is the latest feasible boundary before the region root or at the root itself.

## Tension Notes

The main tension is natural-boundary signal versus data/state cost. Step 6 has a strong `http-handler-registration` edge, but it scores `Trivial` boundary data and `Client-reconstructible` state. Step 8 is later and smaller, so the recommendation prioritizes lift feasibility over edge taxonomy.

## Observations

- The trace contains at least one strong edge signal, but the recommendation also accounts for boundary data and state cost.
- Recommended source evidence: `func (pv PasswordFieldValue) Validate(pass string) bool`.
- PocketBase hook/router edges are natural dispatch points, but app and request event receivers often carry shared framework state.
