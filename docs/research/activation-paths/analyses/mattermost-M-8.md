# mattermost/M-8 - Push notification fan-out (`sendPushNotificationToAllSessions`)

## Header

- Trace ID: `mattermost/M-8`
- Project: `mattermost`
- Region root: `server/channels/app/notification_push.go:93`
- Path length: 9
- Source trace: `projects/mattermost/traces/M-8.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `Run` | `direct-function-call` | Very-large | Serializable | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 2 | `serverCmdF` | `function-value-in-struct-field` | Very-large | Serializable | Shared-state | Low | OK | Weak | Feasible |
| 3 | `runServer` | `direct-function-call` | Large | Proxy-required | Shared-state | 0 (estimated) | OK | Anti | Feasible-with-proxy |
| 4 | `NewServer` | `direct-function-call` | Large | Reconstructible | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 5 | `(*Server).createPushNotificationsHub` | `method-call-on-concrete-type` | Medium | Reconstructible | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 6 | `(*PushNotificationsHub).start` | `goroutine-launch` | Medium | Proxy-required | Client-reconstructible | Moderate | Infeasible | Anti | Infeasible |
| 7 | `anonymous` | `goroutine-launch` | Small | Proxy-required | Client-reconstructible | Moderate | Infeasible | Anti | Infeasible |
| 8 | `(*App).sendPushNotificationSync` | `tagged-union-dispatch` | Small | Reconstructible | Shared-state | 0 (estimated) | Needs-wrapper | Weak | Feasible |
| 9 | `(*App).sendPushNotificationToAllSessions` | `method-call-on-concrete-type` | Minimal | Reconstructible | Shared-state | 0 (confirmed) | Needs-wrapper | Anti | Feasible |

## Recommended Cut

Cut at step 9, `(*App).sendPushNotificationToAllSessions`. This point keeps extraction surface at `Minimal`, avoids hard-gated boundary values, and leaves state reconstruction at `Shared-state`. The inspected path reaches `server/channels/app/notification_push.go:93`, and this candidate is the latest feasible boundary before the region root or at the root itself.

## Tension Notes

The tradeoff is surface area versus state reconstruction. The cut avoids the large framework prefix, but the remote side must provide `Shared-state` state for the extracted code.

## Observations

- The path includes a goroutine launch; those rows are treated as anti-boundaries unless followed by a queue/channel payload boundary.
- At least one candidate carries proxy-required data, most often an HTTP writer, stream, channel, or queue runtime object.
- Recommended source evidence: `func (a *App) sendPushNotificationToAllSessions(rctx request.CTX, msg *model.PushNotification, userID string, skipSessionId string) *model.AppError`.
- Mattermost `App`/server receivers are scored conservatively as shared state unless the cut is an isolated algorithmic helper.
