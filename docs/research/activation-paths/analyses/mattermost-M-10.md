# mattermost/M-10 - Remote-cluster file transfer (`sendFileToRemote`)

## Header

- Trace ID: `mattermost/M-10`
- Project: `mattermost`
- Region root: `server/platform/services/remotecluster/sendfile.go:84`
- Path length: 9
- Source trace: `projects/mattermost/traces/M-10.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `Run` | `direct-function-call` | Very-large | Serializable | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 2 | `serverCmdF` | `function-value-in-struct-field` | Very-large | Serializable | Shared-state | Low | OK | Weak | Feasible |
| 3 | `runServer` | `direct-function-call` | Large | Proxy-required | Shared-state | 0 (estimated) | OK | Anti | Feasible-with-proxy |
| 4 | `(*Server).Start` | `method-call-on-concrete-type` | Large | Reconstructible | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 5 | `(*Server).startInterClusterServices` | `method-call-on-concrete-type` | Medium | Reconstructible | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 6 | `(*Service).Start` | `method-call-on-concrete-type` | Medium | Trivial | Client-reconstructible | 0 (estimated) | OK | Anti | Feasible |
| 7 | `(*Service).sendLoop` | `goroutine-launch` | Small | Proxy-required | Client-reconstructible | Moderate | Infeasible | Anti | Infeasible |
| 8 | `(*Service).sendFile` | `channel-receive-type-switch` | Small | Serializable | Client-reconstructible | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 9 | `(*Service).sendFileToRemote` | `method-call-on-concrete-type` | Minimal | Serializable | Client-reconstructible | 0 (confirmed) | OK | Anti | Feasible |

## Recommended Cut

Cut at step 9, `(*Service).sendFileToRemote`. This point keeps extraction surface at `Minimal`, avoids hard-gated boundary values, and leaves state reconstruction at `Client-reconstructible`. The inspected path reaches `server/platform/services/remotecluster/sendfile.go:84`, and this candidate is the latest feasible boundary before the region root or at the root itself.

## Tension Notes

The tradeoff is surface area versus state reconstruction. The cut avoids the large framework prefix, but the remote side must provide `Client-reconstructible` state for the extracted code.

## Observations

- The path includes a goroutine launch; those rows are treated as anti-boundaries unless followed by a queue/channel payload boundary.
- At least one candidate carries proxy-required data, most often an HTTP writer, stream, channel, or queue runtime object.
- Recommended source evidence: `func (rcs *Service) sendFileToRemote(timeout time.Duration, task sendFileTask) (*model.FileInfo, error)`.
- Mattermost `App`/server receivers are scored conservatively as shared state unless the cut is an isolated algorithmic helper.
