# mattermost/M-12 - Per-recipient email render+send (`sendNotificationEmail`)

## Header

- Trace ID: `mattermost/M-12`
- Project: `mattermost`
- Region root: `server/channels/app/notification_email.go:144`
- Path length: 12
- Source trace: `projects/mattermost/traces/M-12.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `serverCmdF` | `init-time-function-field-dispatch` | Very-large | Serializable | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 2 | `runServer` | `direct-function-call` | Very-large | Proxy-required | Shared-state | 0 (estimated) | OK | Anti | Feasible-with-proxy |
| 3 | `Init` | `direct-function-call` | Very-large | Reconstructible | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 4 | `(*API).InitPost` | `method-call-on-concrete-type` | Large | Serializable | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 5 | `(Handler).ServeHTTP` | `http-handler-registration` | Large | Proxy-required | Shared-state | Moderate | Needs-wrapper | Strong | Feasible-with-proxy |
| 6 | `createPost` | `function-value-in-struct-field` | Medium | Proxy-required | Shared-state | Low | Needs-wrapper | Weak | Feasible-with-proxy |
| 7 | `(*App).CreatePostAsUser` | `method-call-on-concrete-type` | Medium | Reconstructible | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 8 | `(*App).CreatePost` | `method-call-on-concrete-type` | Medium | Reconstructible | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 9 | `(*App).handlePostEvents` | `method-call-on-concrete-type` | Small | Reconstructible | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 10 | `(*App).SendNotifications` | `method-call-on-concrete-type` | Small | Reconstructible | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 11 | `closure / dispatch site` | `goroutine-launch-with-closure-capture` | Small | Proxy-required | Shared-state | Moderate | OK | Anti | Feasible-with-proxy |
| 12 | `(*App).sendNotificationEmail` | `method-call-on-concrete-type` | Minimal | Reconstructible | Shared-state | 0 (confirmed) | OK | Anti | Feasible |

## Recommended Cut

Cut at step 12, `(*App).sendNotificationEmail`. This point keeps extraction surface at `Minimal`, avoids hard-gated boundary values, and leaves state reconstruction at `Shared-state`. The inspected path reaches `server/channels/app/notification_email.go:144`, and this candidate is the latest feasible boundary before the region root or at the root itself.

## Tension Notes

The main tension is natural-boundary signal versus data/state cost. Step 5 has a strong `http-handler-registration` edge, but it scores `Proxy-required` boundary data and `Shared-state` state. Step 12 is later and smaller, so the recommendation prioritizes lift feasibility over edge taxonomy.

## Observations

- The path includes a goroutine launch; those rows are treated as anti-boundaries unless followed by a queue/channel payload boundary.
- At least one candidate carries proxy-required data, most often an HTTP writer, stream, channel, or queue runtime object.
- The trace contains at least one strong edge signal, but the recommendation also accounts for boundary data and state cost.
- Recommended source evidence: `func (a *App) sendNotificationEmail(rctx request.CTX, notification *PostNotification, user *model.User, team *model.Team, senderProfileImage []byte) (*model.EmailNotification, error)`.
- Mattermost `App`/server receivers are scored conservatively as shared state unless the cut is an isolated algorithmic helper.
