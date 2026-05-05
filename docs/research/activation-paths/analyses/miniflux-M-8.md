# miniflux/M-8 - Readability extractor (`ExtractContent`)

## Header

- Trace ID: `miniflux/M-8`
- Project: `miniflux`
- Region root: `internal/reader/readability/readability.go:73`
- Path length: 7
- Source trace: `projects/miniflux/traces/M-8.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `Parse` | `direct-function-call` | Very-large | Serializable | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 2 | `refreshFeeds` | `direct-function-call` | Large | Reconstructible | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 3 | `closure / dispatch site` | `goroutine-launch` | Large | Proxy-required | Client-reconstructible | Moderate | Infeasible | Anti | Infeasible |
| 4 | `RefreshFeed` | `direct-function-call` | Medium | Reconstructible | Client-reconstructible | 0 (estimated) | OK | Anti | Feasible |
| 5 | `ProcessFeedEntries` | `direct-function-call` | Medium | Reconstructible | Client-reconstructible | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 6 | `ScrapeWebsite` | `direct-function-call` | Small | Trivial | Client-reconstructible | 0 (estimated) | OK | Anti | Feasible |
| 7 | `ExtractContent` | `direct-function-call` | Minimal | Proxy-required | Client-reconstructible | 0 (confirmed) | OK | Anti | Feasible-with-proxy |

## Recommended Cut

Cut at step 6, `ScrapeWebsite`. This point keeps extraction surface at `Small`, avoids hard-gated boundary values, and leaves state reconstruction at `Client-reconstructible`. The inspected path reaches `internal/reader/readability/readability.go:73`, and this candidate is the latest feasible boundary before the region root or at the root itself.

## Tension Notes

The tradeoff is surface area versus state reconstruction. The cut avoids the large framework prefix, but the remote side must provide `Client-reconstructible` state for the extracted code.

## Observations

- The path includes a goroutine launch; those rows are treated as anti-boundaries unless followed by a queue/channel payload boundary.
- At least one candidate carries proxy-required data, most often an HTTP writer, stream, channel, or queue runtime object.
- Recommended source evidence: `func ScrapeWebsite(requestBuilder *fetcher.RequestBuilder, pageURL, rules string) (baseURL string, extractedContent string, err error)`.
