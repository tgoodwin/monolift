# gitea/M-16 - Password hashing Argon2 (`Argon2Hasher.HashWithSaltBytes`)

## Header

- Trace ID: `gitea/M-16`
- Project: `gitea`
- Region root: `modules/auth/password/hash/argon2.go:29`
- Path length: 8
- Source trace: `projects/gitea/traces/M-16.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `RunMainApp` | `direct-function-call` | Very-large | Trivial | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 2 | `runChangePassword` | `function-value-in-struct-field` | Very-large | Serializable | Shared-state | Low | OK | Weak | Feasible |
| 3 | `UpdateAuth` | `direct-function-call` | Large | Serializable | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 4 | `(*User).SetPassword` | `method-call-on-concrete-type` | Medium | Trivial | Stateless | 0 (estimated) | OK | Anti | Feasible |
| 5 | `Parse` | `direct-function-call` | Medium | Trivial | Stateless | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 6 | `NewArgon2Hasher` | `map-indexed-function-value-call` | Small | Trivial | Client-reconstructible | Low | Needs-wrapper | Weak | Feasible |
| 7 | `(*PasswordHashAlgorithm).Hash` | `method-call-on-concrete-type` | Small | Trivial | Stateless | 0 (estimated) | OK | Anti | Feasible |
| 8 | `(*Argon2Hasher).HashWithSaltBytes` | `interface-method-dispatch` | Minimal | Serializable | Stateless | 0 (confirmed) | Needs-wrapper | Strong | Feasible |

## Recommended Cut

Cut at step 8, `(*Argon2Hasher).HashWithSaltBytes`. This point keeps extraction surface at `Minimal`, avoids hard-gated boundary values, and leaves state reconstruction at `Stateless`. The inspected path reaches `modules/auth/password/hash/argon2.go:29`, and this candidate is the latest feasible boundary before the region root or at the root itself.

## Tension Notes

The recommended cut has no major Pareto conflict: it is late in the path, has preserveable errors, and does not require proxying live request or stream state.

## Observations

- The trace contains at least one strong edge signal, but the recommendation also accounts for boundary data and state cost.
- Recommended source evidence: `func (hasher *Argon2Hasher) HashWithSaltBytes(password string, salt []byte) string`.
