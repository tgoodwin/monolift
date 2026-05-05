# Miniflux Summary

## Scope

- Traces analyzed: 12 (`miniflux/M-1` through `M-14`, with no `M-11` or `M-12` trace in the corpus)
- Codebase size: ~76k LOC
- Dominant path shapes: refresh worker CLI pipeline, feed-processing helpers, integration fan-out, and UI HTTP handlers.

## Goroutine and Channel Pattern

The `M-1` refresh worker shape recurs in `M-1` through `M-8`: `refreshFeeds` starts worker goroutines that consume jobs and eventually call reader functions. The explicit channel-send/receive edge appears in the recorded paths for `M-1` and `M-5`; other refresh traces compress the same worker context into a goroutine edge followed by direct calls.

## Goroutine Boundary Claim

The corpus supports the claim that goroutine-launch edges are anti-boundaries in Miniflux. The preferred cut is never the launch itself. However, a named function launched in a goroutine can still be a good target after the launch boundary, as with integration fan-out: the anti-boundary is the launch edge, not necessarily the launched function's own signature.

## Cut Placement Findings

- Reader/parser/sanitizer/scraper functions are usually deep, small, and client-reconstructible through `storage.Storage` plus HTTP/config clients.
- UI OAuth/subscription traces move from request-bound handler state to cleaner service/provider functions.
- `storage.Storage` wraps a DB handle, so it is consistently client-reconstructible rather than serializable.

## Synthesis Notes

Miniflux contributes strong evidence for the `Pure Leaf` and `Queue/Worker Payload` patterns. Boundary data is usually simple after feed IDs, user IDs, or provider codes are extracted; the main cost is rebuilding DB and HTTP clients remotely.
