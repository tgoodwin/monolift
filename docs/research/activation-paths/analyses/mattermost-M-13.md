# mattermost/M-13 - Batched email render+send (`sendBatchedEmailNotification`)

## Header

- Trace ID: `mattermost/M-13`
- Project: `mattermost`
- Region root: `server/channels/app/email/email_batching.go:252`
- Path length: 11
- Source trace: `projects/mattermost/traces/M-13.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `Run` | `direct-function-call` | Very-large | Serializable | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 2 | `serverCmdF` | `function-value-in-struct-field` | Very-large | Serializable | Shared-state | Low | OK | Weak | Feasible |
| 3 | `runServer` | `direct-function-call` | Large | Proxy-required | Shared-state | 0 (estimated) | OK | Anti | Feasible-with-proxy |
| 4 | `NewServer` | `direct-function-call` | Large | Reconstructible | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 5 | `NewService` | `direct-function-call` | Medium | Trivial | Config-only | 0 (estimated) | OK | Anti | Feasible |
| 6 | `(*Service).InitEmailBatching` | `method-call-on-concrete-type` | Medium | Serializable | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 7 | `(*EmailBatchingJob).Start` | `method-call-on-concrete-type` | Medium | Serializable | Client-reconstructible | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 8 | `CreateRecurringTask` | `direct-function-call` | Small | Trivial | Client-reconstructible | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 9 | `(*EmailBatchingJob).CheckPendingEmails` | `goroutine-launch + indirect-call-through-captured-parameter` | Small | Proxy-required | Client-reconstructible | Moderate | Infeasible | Anti | Infeasible |
| 10 | `(*EmailBatchingJob).checkPendingNotifications` | `method-call-on-concrete-type` | Small | Infeasible | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Infeasible |
| 11 | `(*Service).sendBatchedEmailNotification` | `indirect-call-through-parameter` | Minimal | Serializable | Client-reconstructible | 0 (confirmed) | Needs-wrapper | Anti | Feasible |

## Recommended Cut

Cut at step 11, `(*Service).sendBatchedEmailNotification`. This point keeps extraction surface at `Minimal`, avoids hard-gated boundary values, and leaves state reconstruction at `Client-reconstructible`. The inspected path reaches `server/channels/app/email/email_batching.go:252`, and this candidate is the latest feasible boundary before the region root or at the root itself.

## Tension Notes

The tradeoff is surface area versus state reconstruction. The cut avoids the large framework prefix, but the remote side must provide `Client-reconstructible` state for the extracted code.

## Observations

- The path includes a goroutine launch; those rows are treated as anti-boundaries unless followed by a queue/channel payload boundary.
- At least one candidate carries proxy-required data, most often an HTTP writer, stream, channel, or queue runtime object.
- Recommended source evidence: `func (es *Service) sendBatchedEmailNotification(userID string, notifications []*batchedNotification)`.
- Mattermost `App`/server receivers are scored conservatively as shared state unless the cut is an isolated algorithmic helper.
