# mattermost/M-15 - Slack workspace import (`SlackImport`)

## Header

- Trace ID: `mattermost/M-15`
- Project: `mattermost`
- Region root: `server/platform/services/slackimport/slackimport.go:131`
- Path length: 4
- Source trace: `projects/mattermost/traces/M-15.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `Run` | `direct-function-call` | Very-large | Serializable | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 2 | `slackImportCmdF` | `function-value-in-struct-field` | Medium | Serializable | Shared-state | Low | OK | Weak | Feasible |
| 3 | `(*App).SlackImport` | `method-call-on-concrete-type` | Small | Proxy-required | Client-reconstructible | 0 (estimated) | Needs-wrapper | Anti | Feasible-with-proxy |
| 4 | `(*SlackImporter).SlackImport` | `method-call-on-concrete-type` | Minimal | Proxy-required | Client-reconstructible | 0 (confirmed) | Needs-wrapper | Anti | Feasible-with-proxy |

## Recommended Cut

Cut at step 2, `slackImportCmdF`. This point keeps extraction surface at `Medium`, avoids hard-gated boundary values, and leaves state reconstruction at `Shared-state`. The inspected path reaches `server/platform/services/slackimport/slackimport.go:131`, and this candidate is the latest feasible boundary before the region root or at the root itself.

## Tension Notes

The tradeoff is surface area versus state reconstruction. The cut avoids the large framework prefix, but the remote side must provide `Shared-state` state for the extracted code.

## Observations

- At least one candidate carries proxy-required data, most often an HTTP writer, stream, channel, or queue runtime object.
- Recommended source evidence: `func slackImportCmdF(command *cobra.Command, args []string) error`.
- Mattermost `App`/server receivers are scored conservatively as shared state unless the cut is an isolated algorithmic helper.
