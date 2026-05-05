# pocketbase/M-6 - Bcrypt password hash on save (`setValue`)

## Header

- Trace ID: `pocketbase/M-6`
- Project: `pocketbase`
- Region root: `core/field_password.go:286`
- Path length: 8
- Source trace: `projects/pocketbase/traces/M-6.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `(*PocketBase).Start` | `method-call-on-concrete-type` | Very-large | Trivial | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 2 | `RunE` | `struct-literal-field-assignment` | Large | Serializable | Shared-state | Low | Needs-wrapper | Weak | Feasible |
| 3 | `RunE` | `registered-callback-dispatch` | Large | Serializable | Shared-state | Low | Needs-wrapper | Anti | Feasible |
| 4 | `(*Record).SetPassword` | `method-call-on-concrete-type` | Medium | Serializable | Stateless | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 5 | `(*Record).Set` | `method-call-on-concrete-type` | Medium | Serializable | Client-reconstructible | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 6 | `(*Record).SetIfFieldExists` | `method-call-on-concrete-type` | Small | Serializable | Client-reconstructible | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 7 | `(*PasswordField).FindSetter` | `interface-method-dispatch` | Small | Trivial | Stateless | 0 (estimated) | Needs-wrapper | Strong | Feasible |
| 8 | `(*PasswordField).setValue` | `method-value-call` | Minimal | Serializable | Stateless | 0 (confirmed) | Needs-wrapper | Anti | Feasible |

## Recommended Cut

Cut at step 8, `(*PasswordField).setValue`. This point keeps extraction surface at `Minimal`, avoids hard-gated boundary values, and leaves state reconstruction at `Stateless`. The inspected path reaches `core/field_password.go:286`, and this candidate is the latest feasible boundary before the region root or at the root itself.

## Tension Notes

The main tension is natural-boundary signal versus data/state cost. Step 7 has a strong `interface-method-dispatch` edge, but it scores `Trivial` boundary data and `Stateless` state. Step 8 is later and smaller, so the recommendation prioritizes lift feasibility over edge taxonomy.

## Observations

- The trace contains at least one strong edge signal, but the recommendation also accounts for boundary data and state cost.
- Recommended source evidence: `func (f *PasswordField) setValue(record *Record, raw any)`.
- PocketBase hook/router edges are natural dispatch points, but app and request event receivers often carry shared framework state.
