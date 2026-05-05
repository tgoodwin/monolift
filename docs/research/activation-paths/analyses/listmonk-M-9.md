# listmonk/M-9 - Transactional message render (`TxMessage.Render`)

## Header

- Trace ID: `listmonk/M-9`
- Project: `listmonk`
- Region root: `models/messages.go:74`
- Path length: 5
- Source trace: `projects/listmonk/traces/M-9.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `initHTTPServer` | `direct-function-call` | Very-large | Reconstructible | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 2 | `initHTTPHandlers` | `direct-function-call` | Large | Reconstructible | Shared-state | Low | Needs-wrapper | Anti | Feasible |
| 3 | `anonymous` | `http-handler-registration` | Medium | Trivial | Shared-state | Low | Needs-wrapper | Strong | Feasible |
| 4 | `(*App).SendTxMessage` | `closure-captured-function-value` | Small | Reconstructible | Client-reconstructible | Low | OK | Weak | Feasible |
| 5 | `(*TxMessage).Render` | `method-call-on-concrete-type` | Minimal | Trivial | Config-only | 0 (confirmed) | OK | Anti | Feasible |

## Recommended Cut

Cut at step 3, `anonymous`. This point keeps extraction surface at `Medium`, avoids hard-gated boundary values, and leaves state reconstruction at `Shared-state`. The inspected path reaches `models/messages.go:74`, and this candidate is the latest feasible boundary before the region root or at the root itself.

## Tension Notes

The tradeoff is surface area versus state reconstruction. The cut avoids the large framework prefix, but the remote side must provide `Shared-state` state for the extracted code.

## Observations

- The trace contains at least one strong edge signal, but the recommendation also accounts for boundary data and state cost.
- Recommended source evidence: `func (o *Auth) Perm(next echo.HandlerFunc, perms ...string) echo.HandlerFunc`.
- Listmonk's smaller codebase reduces surface-area pressure; boundary data and client reconstruction dominate the decision.
