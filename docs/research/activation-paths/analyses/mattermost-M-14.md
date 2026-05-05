# mattermost/M-14 - PBKDF2 password hashing (`PBKDF2.Hash`)

## Header

- Trace ID: `mattermost/M-14`
- Project: `mattermost`
- Region root: `server/channels/app/password/hashers/pbkdf2.go:151`
- Path length: 12
- Source trace: `projects/mattermost/traces/M-14.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `Run` | `direct-function-call` | Very-large | Serializable | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 2 | `closure / dispatch site` | `method-call-on-concrete-type` | Very-large | Serializable | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 3 | `serverCmdF` | `function-value-in-struct-field` | Very-large | Serializable | Shared-state | Low | OK | Weak | Feasible |
| 4 | `runServer` | `direct-function-call` | Large | Proxy-required | Shared-state | 0 (estimated) | OK | Anti | Feasible-with-proxy |
| 5 | `Init` | `direct-function-call` | Large | Reconstructible | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 6 | `(*API).InitBot` | `method-call-on-concrete-type` | Medium | Serializable | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 7 | `(Handler).ServeHTTP` | `http-handler-registration` | Medium | Proxy-required | Shared-state | Moderate | Needs-wrapper | Strong | Feasible-with-proxy |
| 8 | `convertBotToUser` | `function-value-in-struct-field` | Medium | Proxy-required | Client-reconstructible | Low | Needs-wrapper | Weak | Feasible-with-proxy |
| 9 | `(*App).ConvertBotToUser` | `method-call-on-concrete-type` | Small | Reconstructible | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 10 | `(*App).UpdatePassword` | `method-call-on-concrete-type` | Small | Reconstructible | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 11 | `Hash` | `direct-function-call` | Small | Trivial | Stateless | 0 (estimated) | OK | Anti | Feasible |
| 12 | `(PBKDF2).Hash` | `interface-method-dispatch` | Minimal | Trivial | Stateless | 0 (confirmed) | OK | Strong | Feasible |

## Recommended Cut

Cut at step 12, `(PBKDF2).Hash`. This point keeps extraction surface at `Minimal`, avoids hard-gated boundary values, and leaves state reconstruction at `Stateless`. The inspected path reaches `server/channels/app/password/hashers/pbkdf2.go:151`, and this candidate is the latest feasible boundary before the region root or at the root itself.

## Tension Notes

The recommended cut has no major Pareto conflict: it is late in the path, has preserveable errors, and does not require proxying live request or stream state.

## Observations

- At least one candidate carries proxy-required data, most often an HTTP writer, stream, channel, or queue runtime object.
- The trace contains at least one strong edge signal, but the recommendation also accounts for boundary data and state cost.
- Recommended source evidence: `func (p PBKDF2) Hash(password string) (string, error)`.
- Mattermost `App`/server receivers are scored conservatively as shared state unless the cut is an isolated algorithmic helper.
