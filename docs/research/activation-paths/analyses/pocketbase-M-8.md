# pocketbase/M-8 - S3 multipart upload (`Uploader.Upload`)

## Header

- Trace ID: `pocketbase/M-8`
- Project: `pocketbase`
- Region root: `tools/filesystem/internal/s3blob/s3/uploader.go:71`
- Path length: 11
- Source trace: `projects/pocketbase/traces/M-8.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `(*PocketBase).Start` | `method-call-on-concrete-type` | Very-large | Trivial | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 2 | `(*PocketBase).Execute` | `method-call-on-concrete-type` | Large | Trivial | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 3 | `RunE` | `function-value-in-struct-field` | Large | Serializable | Shared-state | Low | Needs-wrapper | Weak | Feasible |
| 4 | `Serve` | `direct-function-call` | Large | Trivial | Config-only | 0 (estimated) | OK | Anti | Feasible |
| 5 | `mux` | `http-server-handler-dispatch` | Medium | Serializable | Shared-state | Low | OK | Strong | Feasible |
| 6 | `backupUpload` | `function-value-in-struct-field` | Medium | Serializable | Shared-state | Low | OK | Weak | Feasible |
| 7 | `(*System).UploadFile` | `method-call-on-concrete-type` | Medium | Reconstructible | Client-reconstructible | 0 (estimated) | OK | Anti | Feasible |
| 8 | `(*Writer).Write` | `method-call-on-concrete-type` | Small | Serializable | Client-reconstructible | 0 (estimated) | OK | Anti | Feasible |
| 9 | `(*writer).Write` | `interface-method-dispatch` | Small | Serializable | Client-reconstructible | Low | OK | Strong | Feasible |
| 10 | `(*writer).open` | `method-call-on-concrete-type` | Small | Proxy-required | Client-reconstructible | 0 (estimated) | Needs-wrapper | Anti | Feasible-with-proxy |
| 11 | `(*Uploader).Upload` | `goroutine-launch` | Minimal | Infeasible | Client-reconstructible | 0 (confirmed) | OK | Anti | Infeasible |

## Recommended Cut

Cut at step 9, `(*writer).Write`. This point keeps extraction surface at `Small`, avoids hard-gated boundary values, and leaves state reconstruction at `Client-reconstructible`. The inspected path reaches `tools/filesystem/internal/s3blob/s3/uploader.go:71`, and this candidate is the latest feasible boundary before the region root or at the root itself.

## Tension Notes

The tradeoff is surface area versus state reconstruction. The cut avoids the large framework prefix, but the remote side must provide `Client-reconstructible` state for the extracted code.

## Observations

- The path includes a goroutine launch; those rows are treated as anti-boundaries unless followed by a queue/channel payload boundary.
- At least one candidate carries proxy-required data, most often an HTTP writer, stream, channel, or queue runtime object.
- The trace contains at least one strong edge signal, but the recommendation also accounts for boundary data and state cost.
- Recommended source evidence: `func (w *writer) Write(p []byte) (int, error)`.
- PocketBase hook/router edges are natural dispatch points, but app and request event receivers often carry shared framework state.
