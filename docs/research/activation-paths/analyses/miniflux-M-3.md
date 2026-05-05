# miniflux/M-3 - HTML sanitizer (`SanitizeHTML`)

## Header

- Trace ID: `miniflux/M-3`
- Project: `miniflux`
- Region root: `internal/reader/sanitizer/sanitizer.go:217`
- Path length: 6
- Source trace: `projects/miniflux/traces/M-3.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `Parse` | `direct-function-call` | Very-large | Serializable | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 2 | `refreshFeeds` | `direct-function-call` | Large | Reconstructible | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 3 | `<worker closure body>` | `goroutine-launch-of-closure` | Medium | Proxy-required | Client-reconstructible | Moderate | Infeasible | Anti | Infeasible |
| 4 | `RefreshFeed` | `direct-function-call` | Medium | Reconstructible | Client-reconstructible | 0 (estimated) | OK | Anti | Feasible |
| 5 | `ProcessFeedEntries` | `direct-function-call` | Small | Reconstructible | Client-reconstructible | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 6 | `SanitizeHTML` | `direct-function-call` | Minimal | Trivial | Stateless | 0 (confirmed) | Needs-wrapper | Anti | Feasible |

## Recommended Cut

Cut at step 6, `SanitizeHTML`. This point keeps extraction surface at `Minimal`, avoids hard-gated boundary values, and leaves state reconstruction at `Stateless`. The inspected path reaches `internal/reader/sanitizer/sanitizer.go:217`, and this candidate is the latest feasible boundary before the region root or at the root itself.

## Tension Notes

The recommended cut has no major Pareto conflict: it is late in the path, has preserveable errors, and does not require proxying live request or stream state.

## Observations

- The path includes a goroutine launch; those rows are treated as anti-boundaries unless followed by a queue/channel payload boundary.
- At least one candidate carries proxy-required data, most often an HTTP writer, stream, channel, or queue runtime object.
- Recommended source evidence: `func SanitizeHTML(baseURL, rawHTML string, sanitizerOptions *SanitizerOptions) string`.
