# mattermost/M-3 - Outgoing webhook fan-out (`TriggerWebhook`)

## Header

- Trace ID: `mattermost/M-3`
- Project: `mattermost`
- Region root: `server/channels/app/webhook.go:99`
- Path length: 14
- Source trace: `projects/mattermost/traces/M-3.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `Run` | `direct-function-call` | Very-large | Serializable | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 2 | `serverCmdF` | `function-value-in-struct-field` | Very-large | Serializable | Shared-state | Low | OK | Weak | Feasible |
| 3 | `runServer` | `direct-function-call` | Very-large | Proxy-required | Shared-state | 0 (estimated) | OK | Anti | Feasible-with-proxy |
| 4 | `Init` | `direct-function-call` | Large | Reconstructible | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 5 | `(*API).InitPost` | `method-call-on-concrete-type` | Large | Serializable | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 6 | `closure / dispatch site` | `function-value-in-struct-field` | Large | Serializable | Shared-state | Low | Needs-wrapper | Weak | Feasible |
| 7 | `(Handler).ServeHTTP` | `interface-method-dispatch` | Medium | Proxy-required | Shared-state | Moderate | Needs-wrapper | Strong | Feasible-with-proxy |
| 8 | `createPost` | `function-value-in-struct-field` | Medium | Proxy-required | Client-reconstructible | Low | Needs-wrapper | Weak | Feasible-with-proxy |
| 9 | `(*App).CreatePostAsUser` | `method-call-on-concrete-type` | Medium | Reconstructible | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 10 | `(*App).CreatePost` | `method-call-on-concrete-type` | Medium | Reconstructible | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 11 | `(*App).handlePostEvents` | `method-call-on-concrete-type` | Small | Reconstructible | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 12 | `closure / dispatch site` | `closure-capture` + `goroutine-launch` | Small | Proxy-required | Shared-state | Moderate | OK | Anti | Feasible-with-proxy |
| 13 | `(*App).handleWebhookEvents` | `method-call-on-concrete-type` | Small | Reconstructible | Shared-state | Low | Needs-wrapper | Anti | Feasible |
| 14 | `(*App).TriggerWebhook` | `method-call-on-concrete-type` | Minimal | Reconstructible | Shared-state | Low | Needs-wrapper | Anti | Feasible |

## Recommended Cut

Cut at step 13, `(*App).handleWebhookEvents`. This point keeps extraction surface at `Small`, avoids hard-gated boundary values, and leaves state reconstruction at `Shared-state`. The inspected path reaches `server/channels/app/webhook.go:99`, and this candidate is the latest feasible boundary before the region root or at the root itself.

## Tension Notes

The main tension is natural-boundary signal versus data/state cost. Step 7 has a strong `interface-method-dispatch` edge, but it scores `Proxy-required` boundary data and `Shared-state` state. Step 13 is later and smaller, so the recommendation prioritizes lift feasibility over edge taxonomy.

## Observations

- The path includes a goroutine launch; those rows are treated as anti-boundaries unless followed by a queue/channel payload boundary.
- At least one candidate carries proxy-required data, most often an HTTP writer, stream, channel, or queue runtime object.
- The trace contains at least one strong edge signal, but the recommendation also accounts for boundary data and state cost.
- Recommended source evidence: `func (a *App) handleWebhookEvents(rctx request.CTX, post *model.Post, team *model.Team, channel *model.Channel, user *model.User) *model.AppError`.
- Mattermost `App`/server receivers are scored conservatively as shared state unless the cut is an isolated algorithmic helper.
