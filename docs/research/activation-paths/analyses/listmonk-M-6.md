# listmonk/M-6 - SES/SNS bounce processing (`SES.ProcessBounce`)

## Header

- Trace ID: `listmonk/M-6`
- Project: `listmonk`
- Region root: `internal/bounce/webhooks/ses.go:108`
- Path length: 4
- Source trace: `projects/listmonk/traces/M-6.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `initHTTPServer` | `direct-function-call` | Very-large | Reconstructible | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 2 | `initHTTPHandlers` | `direct-function-call` | Medium | Reconstructible | Shared-state | Low | Needs-wrapper | Anti | Feasible |
| 3 | `(*App).BounceWebhook` | `method-value-handler-registration` | Small | Reconstructible | Client-reconstructible | Low | OK | Strong | Feasible |
| 4 | `(*SES).ProcessBounce` | `method-call-on-concrete-field-type` | Minimal | Serializable | Client-reconstructible | 0 (confirmed) | OK | Anti | Feasible |

## Recommended Cut

Cut at step 3, `(*App).BounceWebhook`. This point keeps extraction surface at `Small`, avoids hard-gated boundary values, and leaves state reconstruction at `Client-reconstructible`. The inspected path reaches `internal/bounce/webhooks/ses.go:108`, and this candidate is the latest feasible boundary before the region root or at the root itself.

## Tension Notes

The tradeoff is surface area versus state reconstruction. The cut avoids the large framework prefix, but the remote side must provide `Client-reconstructible` state for the extracted code.

## Observations

- The trace contains at least one strong edge signal, but the recommendation also accounts for boundary data and state cost.
- Recommended source evidence: `func (a *App) BounceWebhook(c echo.Context) error`.
- Listmonk's smaller codebase reduces surface-area pressure; boundary data and client reconstruction dominate the decision.
