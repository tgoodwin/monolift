# miniflux/M-6 - Feed format parser (`ParseFeed`)

## Header

- Trace ID: `miniflux/M-6`
- Project: `miniflux`
- Region root: `internal/reader/parser/parser.go:20`
- Path length: 5
- Source trace: `projects/miniflux/traces/M-6.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `Parse` | `direct-function-call` | Very-large | Serializable | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 2 | `refreshFeeds` | `direct-function-call` | Large | Reconstructible | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 3 | `closure / dispatch site` | `goroutine-launch` | Medium | Proxy-required | Client-reconstructible | Moderate | Infeasible | Anti | Infeasible |
| 4 | `RefreshFeed` | `direct-function-call` | Small | Reconstructible | Client-reconstructible | 0 (estimated) | OK | Anti | Feasible |
| 5 | `ParseFeed` | `direct-function-call` | Minimal | Serializable | Stateless | 0 (confirmed) | OK | Anti | Feasible |

## Recommended Cut

Cut at step 5, `ParseFeed`. This point keeps extraction surface at `Minimal`, avoids hard-gated boundary values, and leaves state reconstruction at `Stateless`. The inspected path reaches `internal/reader/parser/parser.go:20`, and this candidate is the latest feasible boundary before the region root or at the root itself.

## Tension Notes

The recommended cut has no major Pareto conflict: it is late in the path, has preserveable errors, and does not require proxying live request or stream state.

## Observations

- The path includes a goroutine launch; those rows are treated as anti-boundaries unless followed by a queue/channel payload boundary.
- At least one candidate carries proxy-required data, most often an HTTP writer, stream, channel, or queue runtime object.
- Recommended source evidence: `func ParseFeed(baseURL string, r io.ReadSeeker) (*model.Feed, error)`.
