# Lift Utility Corpus: Miniflux

Miniflux is a minimalist and self-hosted RSS feed reader. Its computationally expensive paths cluster around the background worker loop that refreshes feeds, which involves fetching external XML/JSON documents, parsing them into a normalized model, extracting full content from linked websites using a readability algorithm, and sanitizing the resulting HTML. Additionally, bursty IO-bound work occurs during feed discovery (icon finding/resizing) and when pushing new entries to various third-party integration services (webhooks, Slack, Matrix, etc.).

### C-1: RefreshFeed

- **Region root:** `internal/reader/handler/handler.go:207` — `RefreshFeed` coordinates the full lifecycle of a feed refresh (fetch, parse, process, store).
- **Caller(s):** `internal/worker/worker.go:42`
- **Why useful (rubric scoring):**
  - Compute envelope: yes — Involves network IO, XML/JSON parsing, and complex entry processing.
  - Load profile: yes — Triggered by background workers across many feeds with variable update frequencies.
  - Coherent unit: yes — Clear input (feed ID, user ID) and returns a localized error wrapper.
  - State independence: maybe — Depends on `*storage.Storage` for DB access, which is a shared resource but standard for Monolift lift regions.
  - Latency / failure: yes — Called from a background worker; extra latency is acceptable and retries are natural.
- **Activation shape:** "queue worker" (via background job channel).
- **Confidence:** high — This is the primary unit of work in Miniflux.
- **Risk notes:** Significant dependency on the `storage` package for database operations.

### C-2: ProcessFeedEntries

- **Region root:** `internal/reader/processor/processor.go:27` — `ProcessFeedEntries` applies filters, scrapers, and sanitizers to a list of feed entries.
- **Caller(s):** `internal/reader/handler/handler.go:277`, `internal/reader/handler/handler.go:94`
- **Why useful (rubric scoring):**
  - Compute envelope: yes — Iterates over entries performing scraping, content rewriting, and HTML sanitization.
  - Load profile: yes — Workload scales with the number of entries in a feed and whether the crawler is enabled.
  - Coherent unit: yes — Processes a slice of entries for a specific feed/user.
  - State independence: maybe — Reads user/feed config from the DB via `storage`.
  - Latency / failure: yes — Usually on the background refresh path.
- **Activation shape:** "background job sub-unit".
- **Confidence:** high — This contains the bulk of the CPU-heavy text processing logic.
- **Risk notes:** Dependency on `storage` to check if entries are new and to fetch user preferences.

### C-3: SanitizeHTML

- **Region root:** `internal/reader/sanitizer/sanitizer.go:193` — `SanitizeHTML` parses raw HTML and removes disallowed tags and attributes.
- **Caller(s):** `internal/reader/processor/processor.go:111`, `internal/reader/processor/processor.go:148`
- **Why useful (rubric scoring):**
  - Compute envelope: yes — CPU-bound HTML tree traversal and re-rendering.
  - Load profile: yes — Directly proportional to the size and complexity of feed entry content.
  - Coherent unit: yes — Pure function (mostly) taking strings and returning a string.
  - State independence: yes — No dependency on external state other than configuration.
  - Latency / failure: yes — Pure compute; adding a hop is just a performance trade-off.
- **Activation shape:** "library-style compute unit".
- **Confidence:** high — Perfect example of a compute-heavy pure function.
- **Risk notes:** Very low risk; highly decoupled.

### C-4: ScrapeWebsite

- **Region root:** `internal/reader/scraper/scraper.go:21` — `ScrapeWebsite` fetches a web page and extracts the main content using rules or readability.
- **Caller(s):** `internal/reader/processor/processor.go:87`, `internal/reader/processor/processor.go:133`
- **Why useful (rubric scoring):**
  - Compute envelope: yes — Combines network IO with DOM-heavy content extraction.
  - Load profile: yes — Triggered when a feed has the "crawler" enabled; bursty based on new entries.
  - Coherent unit: yes — Takes a URL and rules; returns extracted content.
  - State independence: yes — Relies on an external `RequestBuilder` for network calls.
  - Latency / failure: yes — Already an O(seconds) operation due to network; failures are expected.
- **Activation shape:** "IO/Compute hybrid unit".
- **Confidence:** high — Highly variable cost and well-bounded.
- **Risk notes:** None.

### C-5: ExtractContent (Readability)

- **Region root:** `internal/reader/readability/readability.go:73` — `ExtractContent` implements a readability algorithm to find relevant content in a page.
- **Caller(s):** `internal/reader/scraper/scraper.go:56`
- **Why useful (rubric scoring):**
  - Compute envelope: yes — CPU-intensive DOM analysis, scoring, and transformation.
  - Load profile: yes — Proportionate to page complexity; used when no specific scraper rules exist.
  - Coherent unit: yes — Pure compute taking an `io.Reader` and returning strings.
  - State independence: yes — Completely self-contained.
  - Latency / failure: yes — Extra hop is acceptable for this level of compute.
- **Activation shape:** "library-style compute unit".
- **Confidence:** high — Very clear compute envelope.
- **Risk notes:** Minimal.

### C-6: ParseFeed

- **Region root:** `internal/reader/parser/parser.go:20` — `ParseFeed` detects feed format and dispatches to the appropriate parser (Atom, RSS, JSON, etc.).
- **Caller(s):** `internal/reader/handler/handler.go:61`, `internal/reader/handler/handler.go:254`
- **Why useful (rubric scoring):**
  - Compute envelope: yes — CPU-bound XML/JSON parsing and model normalization.
  - Load profile: yes — Central to every feed refresh and discovery.
  - Coherent unit: yes — Clean interface: takes an `io.ReadSeeker`, returns a `model.Feed`.
  - State independence: yes — Purely functional transformation.
  - Latency / failure: yes — Already a distinct phase in feed processing.
- **Activation shape:** "parser unit".
- **Confidence:** high — Classic lift candidate for compute-heavy parsing.
- **Risk notes:** None.

### C-7: PushEntries (Integrations)

- **Region root:** `internal/integration/integration.go:511` — `PushEntries` sends new feed entries to activated third-party providers.
- **Caller(s):** `internal/reader/handler/handler.go:292`
- **Why useful (rubric scoring):**
  - Compute envelope: maybe — Mostly IO-bound (outbound HTTP), but fan-out can be significant.
  - Load profile: yes — Bursty based on when feeds are refreshed and how many new entries are found.
  - Coherent unit: yes — Takes a feed, entries, and integration settings.
  - State independence: yes — Independent of local app state once inputs are provided.
  - Latency / failure: yes — Explicitly called in a goroutine (`go integration.PushEntries`) in the caller, making it a perfect async lift.
- **Activation shape:** "async integration hook".
- **Confidence:** high — The async nature is already acknowledged in the code.
- **Risk notes:** None.

### C-8: UpdateOrCreateFeedIcon

- **Region root:** `internal/reader/icon/checker.go:28` — `UpdateOrCreateFeedIcon` finds, downloads, and stores a feed's icon.
- **Caller(s):** `internal/reader/handler/handler.go:111`, `internal/reader/handler/handler.go:300`
- **Why useful (rubric scoring):**
  - Compute envelope: yes — Involves HTML parsing (icon discovery), image downloading, and image resizing/re-encoding.
  - Load profile: maybe — Typically done once per feed or during forced refresh.
  - Coherent unit: yes — Specific task with clear goal.
  - State independence: maybe — Uses `storage` for persistence.
  - Latency / failure: yes — Background task; non-critical for core feed reading.
- **Activation shape:** "background maintenance task".
- **Confidence:** medium — Utility is lower because it's infrequent, but the work per call is high.
- **Risk notes:** Image processing logic (decoding/encoding) is a heavy dependency.

### C-9: CreateFeed

- **Region root:** `internal/reader/handler/handler.go:116` — `CreateFeed` handles the creation of a new feed, including fetching and initial parsing.
- **Caller(s):** `internal/api/feed.go`, `internal/ui/feed_create.go` (hypothesized UI paths)
- **Why useful (rubric scoring):**
  - Compute envelope: yes — Heavy first-time fetch and parse.
  - Load profile: yes — Bursty based on user activity (adding new feeds).
  - Coherent unit: yes — Well-defined request/response.
  - State independence: maybe — Depends on `storage`.
  - Latency / failure: yes — Users expect a delay when adding a new feed.
- **Activation shape:** "HTTP request-response (sync)".
- **Confidence:** high — Similar utility to RefreshFeed but on the interactive path.
- **Risk notes:** Directly impacts user-facing latency.

### C-10: EstimateReadingTime

- **Region root:** `internal/reader/readingtime/readingtime.go:17` — `EstimateReadingTime` calculates the estimated reading time for an article.
- **Caller(s):** `internal/reader/processor/processor.go:113`, `internal/reader/processor/processor.go:145`
- **Why useful (rubric scoring):**
  - Compute envelope: maybe — O(N) string processing and tag stripping.
  - Load profile: yes — Run for every entry in every feed refresh.
  - Coherent unit: yes — Pure function.
  - State independence: yes — Purely functional.
  - Latency / failure: yes — Tiny contribution to overall latency.
- **Activation shape:** "utility compute function".
- **Confidence:** medium — Genuinely marginal because the work might be too small to justify the hop unless batched.
- **Risk notes:** None.

## Honest assessment

I am most confident about **C-1 through C-6**, as these represent the core "value-add" of Miniflux and involve substantial CPU or IO work that is clearly bounded. **C-3 (SanitizeHTML)** and **C-5 (ExtractContent)** are particularly strong as they are pure compute-heavy transformations. **C-7 (PushEntries)** is a classic async worker pattern. **C-8 and C-10** are more marginal; `UpdateOrCreateFeedIcon` is heavy but infrequent, while `EstimateReadingTime` is frequent but relatively lightweight.

One region that is likely a great lift candidate but hard to justify with the rubric is the **template rendering engine** (`internal/template/engine.go`). While it is CPU-bound and used for every page view, the current implementation is heavily tied to an `Engine` struct that loads all templates from an embedded filesystem at startup. Lifting the `Render` method would require sharing this large template state or re-implementing the engine's initialization on the remote side, which complicates the "state independence" and "coherent unit" criteria.
