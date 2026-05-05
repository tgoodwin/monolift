# mattermost/M-7 - Slash command HTTP execution (`DoCommandRequest`)

## Header

- Trace ID: `mattermost/M-7`
- Project: `mattermost`
- Region root: `server/channels/app/command.go:521`
- Path length: 13
- Source trace: `projects/mattermost/traces/M-7.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `Run` | `direct-function-call` | Very-large | Serializable | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 2 | `serverCmdF` | `function-value-in-struct-field` | Very-large | Serializable | Shared-state | Low | OK | Weak | Feasible |
| 3 | `runServer` | `direct-function-call` | Very-large | Proxy-required | Shared-state | 0 (estimated) | OK | Anti | Feasible-with-proxy |
| 4 | `Init` | `direct-function-call` | Large | Reconstructible | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 5 | `(*API).InitCommand` | `method-call-on-concrete-type` | Large | Serializable | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 6 | `web.Handler.HandleFunc = h` | `function-value-in-struct-field` | Medium | Serializable | Shared-state | Low | Needs-wrapper | Weak | Feasible |
| 7 | `closure / dispatch site` | `http-handler-registration` | Medium | Serializable | Shared-state | Low | Needs-wrapper | Strong | Feasible |
| 8 | `s.Server.Serve(listener)` | `goroutine-launch` | Medium | Proxy-required | Shared-state | Moderate | OK | Anti | Feasible-with-proxy |
| 9 | `(Handler).ServeHTTP` | `interface-method-dispatch` | Medium | Proxy-required | Shared-state | Moderate | Needs-wrapper | Strong | Feasible-with-proxy |
| 10 | `executeCommand` | `function-value-in-struct-field` | Small | Proxy-required | Client-reconstructible | Low | Needs-wrapper | Weak | Feasible-with-proxy |
| 11 | `(*App).ExecuteCommand` | `method-call-on-concrete-type` | Small | Reconstructible | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 12 | `(*App).tryExecuteCustomCommand` | `method-call-on-concrete-type` | Small | Reconstructible | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 13 | `(*App).DoCommandRequest` | `method-call-on-concrete-type` | Minimal | Reconstructible | Shared-state | 0 (confirmed) | Needs-wrapper | Anti | Feasible |

## Recommended Cut

Cut at step 13, `(*App).DoCommandRequest`. This point keeps extraction surface at `Minimal`, avoids hard-gated boundary values, and leaves state reconstruction at `Shared-state`. The inspected path reaches `server/channels/app/command.go:521`, and this candidate is the latest feasible boundary before the region root or at the root itself.

## Tension Notes

The main tension is natural-boundary signal versus data/state cost. Step 9 has a strong `interface-method-dispatch` edge, but it scores `Proxy-required` boundary data and `Shared-state` state. Step 13 is later and smaller, so the recommendation prioritizes lift feasibility over edge taxonomy.

## Observations

- The path includes a goroutine launch; those rows are treated as anti-boundaries unless followed by a queue/channel payload boundary.
- At least one candidate carries proxy-required data, most often an HTTP writer, stream, channel, or queue runtime object.
- The trace contains at least one strong edge signal, but the recommendation also accounts for boundary data and state cost.
- Recommended source evidence: `func (a *App) DoCommandRequest(rctx request.CTX, cmd *model.Command, p url.Values) (*model.Command, *model.CommandResponse, *model.AppError)`.
- Mattermost `App`/server receivers are scored conservatively as shared state unless the cut is an isolated algorithmic helper.
