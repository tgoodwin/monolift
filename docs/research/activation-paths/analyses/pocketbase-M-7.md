# pocketbase/M-7 - SMTP send (`SMTPClient.send`)

## Header

- Trace ID: `pocketbase/M-7`
- Project: `pocketbase`
- Region root: `tools/mailer/smtp.go:62`
- Path length: 12
- Source trace: `projects/pocketbase/traces/M-7.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `(*PocketBase).Start` | `method-call-on-concrete-type` | Very-large | Trivial | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 2 | `closure / dispatch site` | `function-value-in-struct-field` | Large | Serializable | Shared-state | Low | Needs-wrapper | Weak | Feasible |
| 3 | `Serve` | `direct-function-call` | Large | Trivial | Config-only | 0 (estimated) | OK | Anti | Feasible |
| 4 | `NewRouter` | `direct-function-call` | Large | Trivial | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 5 | `bindRecordAuthApi` | `direct-function-call` | Large | Serializable | Client-reconstructible | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 6 | `recordRequestEmailChange` | `function-value-in-struct-field` | Medium | Serializable | Config-only | Low | OK | Weak | Feasible |
| 7 | `closure / dispatch site` | `closure-passed-as-argument` | Medium | Trivial | Config-only | Low | OK | Weak | Feasible |
| 8 | `SendRecordChangeEmail` | `direct-function-call` | Medium | Trivial | Config-only | 0 (estimated) | OK | Anti | Feasible |
| 9 | `closure / dispatch site` | `closure-passed-as-argument` | Small | Trivial | Config-only | Low | OK | Weak | Feasible |
| 10 | `(*SMTPClient).Send` | `interface-method-dispatch` | Small | Reconstructible | Config-only | Low | OK | Strong | Feasible |
| 11 | `closure / dispatch site` | `closure-passed-as-argument` | Small | Reconstructible | Config-only | Low | OK | Weak | Feasible |
| 12 | `(*SMTPClient).send` | `method-call-on-concrete-type` | Minimal | Reconstructible | Config-only | 0 (confirmed) | OK | Anti | Feasible |

## Recommended Cut

Cut at step 12, `(*SMTPClient).send`. This point keeps extraction surface at `Minimal`, avoids hard-gated boundary values, and leaves state reconstruction at `Config-only`. The inspected path reaches `tools/mailer/smtp.go:62`, and this candidate is the latest feasible boundary before the region root or at the root itself.

## Tension Notes

The main tension is natural-boundary signal versus data/state cost. Step 10 has a strong `interface-method-dispatch` edge, but it scores `Reconstructible` boundary data and `Config-only` state. Step 12 is later and smaller, so the recommendation prioritizes lift feasibility over edge taxonomy.

## Observations

- The trace contains at least one strong edge signal, but the recommendation also accounts for boundary data and state cost.
- Recommended source evidence: `func (c *SMTPClient) send(m *Message) error`.
- PocketBase hook/router edges are natural dispatch points, but app and request event receivers often carry shared framework state.
