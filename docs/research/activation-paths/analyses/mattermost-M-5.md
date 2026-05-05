# mattermost/M-5 - Bulk team export (`BulkExport`)

## Header

- Trace ID: `mattermost/M-5`
- Project: `mattermost`
- Region root: `server/channels/app/export.go:113`
- Path length: 4
- Source trace: `projects/mattermost/traces/M-5.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `Run` | `direct-function-call` | Very-large | Serializable | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 2 | `closure / dispatch site` | `method-call-on-concrete-type` | Medium | Serializable | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 3 | `bulkExportCmdF` | `function-value-in-struct-field` | Small | Serializable | Client-reconstructible | 0 (estimated) | OK | Weak | Feasible |
| 4 | `(*App).BulkExport` | `method-call-on-concrete-type` | Minimal | Proxy-required | Shared-state | 0 (confirmed) | Needs-wrapper | Anti | Feasible-with-proxy |

## Recommended Cut

Cut at step 3, `bulkExportCmdF`. This point keeps extraction surface at `Small`, avoids hard-gated boundary values, and leaves state reconstruction at `Client-reconstructible`. The inspected path reaches `server/channels/app/export.go:113`, and this candidate is the latest feasible boundary before the region root or at the root itself.

## Tension Notes

The tradeoff is surface area versus state reconstruction. The cut avoids the large framework prefix, but the remote side must provide `Client-reconstructible` state for the extracted code.

## Observations

- At least one candidate carries proxy-required data, most often an HTTP writer, stream, channel, or queue runtime object.
- Recommended source evidence: `func bulkExportCmdF(command *cobra.Command, args []string) error`.
- Mattermost `App`/server receivers are scored conservatively as shared state unless the cut is an isolated algorithmic helper.
