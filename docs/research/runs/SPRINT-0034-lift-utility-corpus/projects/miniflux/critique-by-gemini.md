# Critique of miniflux drafts by gemini

## Verdicts on claude's draft

| Candidate ID | Verdict | One-paragraph reasoning |
|---|---|---|
| C-1 (RefreshFeed) | KEEP | Rubric criterion satisfied: variable / spikable load. This is the primary entry point for background feed refreshes, which are bursty by nature and involve significant CPU/IO work across many feeds. |
| C-2 (ProcessFeedEntries) | KEEP | Rubric criterion satisfied: compute envelope. This handles the per-entry hot loop including scraping, sanitization, and reading-time estimation, which dominates the CPU profile during a refresh. |
| C-3 (ScrapeWebsite) | KEEP | Rubric criterion satisfied: compute envelope. It combines network IO with expensive DOM analysis and readability extraction, and has a very clean, decoupled interface. |
| C-4 (SanitizeHTML) | KEEP | Rubric criterion satisfied: coherent unit. This is a textbook lift candidate: a pure function with a clean signature that performs CPU-intensive HTML tree traversal and re-rendering. |
| C-5 (Icon) | KEEP | Rubric criterion satisfied: compute envelope. Image decoding, resizing, and re-encoding is a classic CPU-heavy task that fits well into a remote worker for independent scaling. |
| C-6 (PushEntries) | KEEP | Rubric criterion satisfied: latency tolerance. The function is already called in a fire-and-forget goroutine in handler.go:349, making the overhead of a network hop negligible for the caller. |
| C-7 (SendEntry) | KEEP | Rubric criterion satisfied: latency tolerance. Like C-6, this is invoked asynchronously and handles outbound fan-out to third-party services, making it an ideal async lift. |
| C-8 (FindSubscriptions) | KEEP | Rubric criterion satisfied: compute envelope. Feed discovery involves multiple speculative network requests and HTML parsing, which can be offloaded from the UI path to improve responsiveness. |
| C-9 (OPML Handler.Import) | KEEP | Rubric criterion satisfied: compute envelope. Bulk imports of large OPML files involve significant XML parsing and database interactions that can spike during user onboarding. |
| C-10 (RewriteDocument) | KEEP | Rubric criterion satisfied: coherent unit. While the per-entry call site in entry_handlers.go:191 is potentially chatty, the function itself is a pure string-to-string transform that is easy to isolate. |
| C-11 (OAuth2 Profile) | KEEP | Rubric criterion satisfied: load profile. Matches the "bursty OAuth callback" calibration example in the rubric and provides a clean interface-based seam for remote execution. |
| C-12 (mediaProxy) | DROP | Fails rubric criterion: latency tolerance. As a streaming reverse-proxy for media assets, adding a network hop would degrade page rendering speed for synchronous browser requests expecting fast first-byte. |

## Verdicts on codex's draft

| Candidate ID | Verdict | One-paragraph reasoning |
|---|---|---|
| C-1 (RefreshFeed) | KEEP | Rubric criterion satisfied: variable / spikable load. Correctly identifies the central workhorse of Miniflux's background operations and its bursty scheduling profile. |
| C-2 (ProcessFeedEntries) | KEEP | Rubric criterion satisfied: compute envelope. Essential for offloading the main entry-processing loop which concentrates the bulk of the application's CPU and IO work. |
| C-3 (ProcessEntryWebPage) | MODIFY | Identifying the entry-scraping path is correct, but the lower-level `internal/reader/scraper/scraper.go:21` (`ScrapeWebsite`) is a cleaner lift target as it avoids the in-place mutation of the `model.Entry` struct. |
| C-4 (PushEntries) | KEEP | Rubric criterion satisfied: latency tolerance and failure model. Perfectly fits the async integration fan-out pattern already present in the codebase. |
| C-5 (FindSubscriptions) | KEEP | Rubric criterion satisfied: compute envelope. Correctly identifies a variable-cost IO/Compute hybrid task that can significantly delay the user-facing discovery flow. |
| C-6 (ExtractContent) | KEEP | Rubric criterion satisfied: coherent unit. This is the core Readability algorithm; it is a pure-compute function with zero state dependencies and high CPU utilization. |
| C-7 (SanitizeHTML) | KEEP | Rubric criterion satisfied: coherent unit. Identical to Claude C-4; a highly decoupled, CPU-bound HTML sanitizer that is a prime candidate for remote execution. |
| C-8 (ParseFeed) | KEEP | Rubric criterion satisfied: compute envelope. Feed parsing (XML/JSON to model normalization) is a distinct compute-heavy phase with a clear boundary and payload-proportional cost. |
| C-9 (Icon Checker) | KEEP | Rubric criterion satisfied: compute envelope. Image processing tasks like resizing and re-encoding are naturally suited for offloading to specialized compute resources. |
| C-10 (SendEntry) | KEEP | Rubric criterion satisfied: latency tolerance. Validates the utility of offloading user-driven integration pushes which are already handled asynchronously. |
| C-11 (Import) | KEEP | Rubric criterion satisfied: compute envelope. Handles the potentially expensive bulk XML parsing required for large OPML imports during user migrations. |
| C-12 (RewriteDocument) | KEEP | Rubric criterion satisfied: coherent unit. While latency on the read path is a concern, the function itself is a pure string-to-string transform with a well-defined contract. |

## Overlooked

### C-11: CreateFeed

- **Region root:** `internal/reader/handler/handler.go:116` — `CreateFeed` coordinates the first-time lifecycle of a feed (fetch, parse, process, store).
- **Caller(s):** `internal/api/feed.go`, `internal/ui/feed_create.go`
- **Why useful (rubric scoring):**
  - Compute envelope: yes — Heavy first-time fetch, parse, and full entry processing.
  - Load profile: yes — Bursty based on user activity (adding new feeds or OPML imports).
  - Coherent unit: yes — Well-defined request/response structure.
  - State independence: maybe — Depends on `storage`, but no more than `RefreshFeed`.
  - Latency / failure: yes — Users expect a delay when adding a new feed, making it latency-tolerant.
- **Activation shape:** "HTTP request-response (sync)".
- **Confidence:** high — Matches the profile of `RefreshFeed` but for the interactive path.
- **Risk notes:** Directly impacts user-facing latency if the remote hop is slow.

## Overall observations

The three drafts converge strongly on the core computationally expensive regions: feed refresh, entry processing, and HTML sanitization. Claude and Codex both identified the third-party integration fan-out and subscription discovery paths, which are excellent candidates due to their async nature or heavy IO/compute mix. Codex was more granular in identifying pure compute sub-units like `ExtractContent` and `ParseFeed`, while Claude provided a useful "near-miss" in the media proxy handler which correctly highlights the boundaries of the latency-tolerance rubric.
