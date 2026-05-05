# gitea/M-13 - Mailer send (`sender.send`)

## Header

- Trace ID: `gitea/M-13`
- Project: `gitea`
- Region root: `services/mailer/sender/sender.go:17`
- Path length: 6
- Source trace: `projects/gitea/traces/M-13.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `runWeb` | `function-value-in-struct-field` | Very-large | Serializable | Shared-state | Low | OK | Weak | Feasible |
| 2 | `serveInstalled` | `direct-function-call` | Large | Trivial | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 3 | `InitWebInstalled` | `direct-function-call` | Medium | Serializable | Shared-state | 0 (estimated) | Infeasible | Anti | Infeasible |
| 4 | `NewContext` | `direct-function-call` | Medium | Serializable | Client-reconstructible | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 5 | `closure / dispatch site` | `closure-callback-registration` | Small | Serializable | Client-reconstructible | Low | Needs-wrapper | Strong | Feasible |
| 6 | `send` | `call-through-package-level-function-variable` | Minimal | Trivial | Client-reconstructible | 0 (confirmed) | OK | Anti | Feasible |

## Recommended Cut

Cut at step 6, `send`. This point keeps extraction surface at `Minimal`, avoids hard-gated boundary values, and leaves state reconstruction at `Client-reconstructible`. The inspected path reaches `services/mailer/sender/sender.go:17`, and this candidate is the latest feasible boundary before the region root or at the root itself.

## Tension Notes

The main tension is natural-boundary signal versus data/state cost. Step 5 has a strong `closure-callback-registration` edge, but it scores `Serializable` boundary data and `Client-reconstructible` state. Step 6 is later and smaller, so the recommendation prioritizes lift feasibility over edge taxonomy.

## Observations

- The trace contains at least one strong edge signal, but the recommendation also accounts for boundary data and state cost.
- Recommended source evidence: `func send(sender Sender, msg *Message) error`.
