# mattermost/M-1 - Document text extraction (`Extract`)

## Header

- Trace ID: `mattermost/M-1`
- Project: `mattermost`
- Region root: `server/platform/services/docextractor/docextractor.go:21`
- Path length: 10
- Source trace: `projects/mattermost/traces/M-1.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `Run` | `direct-function-call` | Very-large | Serializable | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 2 | `serverCmdF` | `function-value-in-struct-field` | Very-large | Serializable | Shared-state | Low | OK | Weak | Feasible |
| 3 | `runServer` | `direct-function-call` | Large | Proxy-required | Shared-state | 0 (estimated) | OK | Anti | Feasible-with-proxy |
| 4 | `Init` | `direct-function-call` | Large | Reconstructible | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 5 | `(*API).InitFile` | `method-call-on-concrete-type` | Medium | Serializable | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 6 | `uploadFileStream` | `http-handler-registration` | Medium | Proxy-required | Shared-state | Moderate | Needs-wrapper | Strong | Feasible-with-proxy |
| 7 | `uploadFileSimple` | `direct-function-call` | Medium | Serializable | Client-reconstructible | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 8 | `(*App).UploadFileX` | `method-call-on-concrete-type` | Small | Infeasible | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Infeasible |
| 9 | `(*App).ExtractContentFromFileInfo` | `closure-capture` + `goroutine-launch` | Small | Proxy-required | Shared-state | Moderate | OK | Anti | Feasible-with-proxy |
| 10 | `Extract` | `direct-function-call` | Minimal | Trivial | Client-reconstructible | 0 (confirmed) | OK | Anti | Feasible |

## Recommended Cut

Cut at step 10, `Extract`. This point keeps extraction surface at `Minimal`, avoids hard-gated boundary values, and leaves state reconstruction at `Client-reconstructible`. The inspected path reaches `server/platform/services/docextractor/docextractor.go:21`, and this candidate is the latest feasible boundary before the region root or at the root itself.

## Tension Notes

The main tension is natural-boundary signal versus data/state cost. Step 6 has a strong `http-handler-registration` edge, but it scores `Proxy-required` boundary data and `Shared-state` state. Step 10 is later and smaller, so the recommendation prioritizes lift feasibility over edge taxonomy.

## Observations

- The path includes a goroutine launch; those rows are treated as anti-boundaries unless followed by a queue/channel payload boundary.
- At least one candidate carries proxy-required data, most often an HTTP writer, stream, channel, or queue runtime object.
- The trace contains at least one strong edge signal, but the recommendation also accounts for boundary data and state cost.
- Recommended source evidence: `func Extract(logger mlog.LoggerIFace, filename string, r io.ReadSeeker, settings ExtractSettings) (string, error)`.
- Mattermost `App`/server receivers are scored conservatively as shared state unless the cut is an isolated algorithmic helper.
