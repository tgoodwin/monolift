# mattermost/M-2 - Image upload post-processing (`postprocessImage`)

## Header

- Trace ID: `mattermost/M-2`
- Project: `mattermost`
- Region root: `server/channels/app/file.go:931`
- Path length: 9
- Source trace: `projects/mattermost/traces/M-2.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `Run` | `direct-function-call` | Very-large | Serializable | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 2 | `serverCmdF` | `function-value-in-struct-field` | Very-large | Serializable | Shared-state | Low | OK | Weak | Feasible |
| 3 | `runServer` | `direct-function-call` | Large | Proxy-required | Shared-state | 0 (estimated) | OK | Anti | Feasible-with-proxy |
| 4 | `Init` | `direct-function-call` | Large | Reconstructible | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 5 | `(*API).InitFile` | `method-call-on-concrete-type` | Medium | Serializable | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 6 | `uploadFileStream` | `http-handler-registration` | Medium | Proxy-required | Shared-state | Moderate | Needs-wrapper | Strong | Feasible-with-proxy |
| 7 | `uploadFileSimple` | `direct-function-call` | Small | Serializable | Client-reconstructible | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 8 | `(*App).UploadFileX` | `method-call-on-concrete-type` | Small | Infeasible | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Infeasible |
| 9 | `(*UploadFileTask).postprocessImage` | `method-call-on-concrete-type` | Minimal | Proxy-required | Config-only | 0 (confirmed) | Needs-wrapper | Anti | Feasible-with-proxy |

## Recommended Cut

Cut at step 7, `uploadFileSimple`. This point keeps extraction surface at `Small`, avoids hard-gated boundary values, and leaves state reconstruction at `Client-reconstructible`. The inspected path reaches `server/channels/app/file.go:931`, and this candidate is the latest feasible boundary before the region root or at the root itself.

## Tension Notes

The main tension is natural-boundary signal versus data/state cost. Step 6 has a strong `http-handler-registration` edge, but it scores `Proxy-required` boundary data and `Shared-state` state. Step 7 is later and smaller, so the recommendation prioritizes lift feasibility over edge taxonomy.

## Observations

- At least one candidate carries proxy-required data, most often an HTTP writer, stream, channel, or queue runtime object.
- The trace contains at least one strong edge signal, but the recommendation also accounts for boundary data and state cost.
- Recommended source evidence: `func uploadFileSimple(c *Context, r *http.Request, timestamp time.Time) *model.FileUploadResponse`.
- Mattermost `App`/server receivers are scored conservatively as shared state unless the cut is an isolated algorithmic helper.
