# mattermost/M-11 - Bulk import processing (`bulkImport`)

## Header

- Trace ID: `mattermost/M-11`
- Project: `mattermost`
- Region root: `server/channels/app/import.go:226`
- Path length: 5
- Source trace: `projects/mattermost/traces/M-11.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `Run` | `direct-function-call` | Very-large | Serializable | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 2 | `(*Command).Execute` | `method-call-on-concrete-type` | Large | Serializable | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 3 | `bulkImportCmdF` | `function-value-in-struct-field` | Medium | Serializable | Client-reconstructible | Low | OK | Weak | Feasible |
| 4 | `(*App).BulkImportWithPath` | `method-call-on-concrete-type` | Small | Proxy-required | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Feasible-with-proxy |
| 5 | `(*App).bulkImport` | `method-call-on-concrete-type` | Minimal | Proxy-required | Shared-state | 0 (confirmed) | Needs-wrapper | Anti | Feasible-with-proxy |

## Recommended Cut

Cut at step 3, `bulkImportCmdF`. This point keeps extraction surface at `Medium`, avoids hard-gated boundary values, and leaves state reconstruction at `Client-reconstructible`. The inspected path reaches `server/channels/app/import.go:226`, and this candidate is the latest feasible boundary before the region root or at the root itself.

## Tension Notes

The tradeoff is surface area versus state reconstruction. The cut avoids the large framework prefix, but the remote side must provide `Client-reconstructible` state for the extracted code.

## Observations

- At least one candidate carries proxy-required data, most often an HTTP writer, stream, channel, or queue runtime object.
- Recommended source evidence: `func bulkImportCmdF(command *cobra.Command, args []string) error`.
- Mattermost `App`/server receivers are scored conservatively as shared state unless the cut is an isolated algorithmic helper.
