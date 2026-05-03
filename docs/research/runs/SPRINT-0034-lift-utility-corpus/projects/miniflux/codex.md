Project read: Miniflux is a Go/PostgreSQL feed reader whose expensive paths cluster around fetching feeds, parsing feed formats, processing entries, scraping article pages, and notifying third-party services. The background scheduler and worker pool hand off per-feed refresh jobs, but the liftable work is the feed refresh and its nested processing, not the scheduler loop itself. There are also user-triggered heavy paths for subscription discovery, OPML import, manual content fetching, icon discovery, and media-proxy HTML rewriting. I treated request-path candidates as lower confidence unless the work is already network- or payload-dominated enough that a remote hop would be tolerable.

### C-1: Refresh one feed

- **Region root:** `evaluation/miniflux/internal/reader/handler/handler.go:207` — `RefreshFeed` fetches an existing feed, handles cache/rate-limit metadata, parses the body, processes entries, writes refreshed entries, launches integrations, and updates icons.
- **Caller(s):** `evaluation/miniflux/internal/worker/worker.go:40` invokes it from background jobs; `evaluation/miniflux/internal/api/feed_handlers.go:67` invokes it for manual API refreshes.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — does outbound HTTP, bounded body read, feed parsing, per-entry processing, DB refresh, integration fan-out, and icon discovery.
  - Load profile: yes — scheduled batches vary by feed size, update frequency, host limits, and tenant feed counts.
  - Coherent unit: maybe — the function has a clear `(store, userID, feedID, forceRefresh)` contract, but combines fetch, parse, persistence, integrations, and icon work.
  - State independence: maybe — primary effects go through the database and external services, but it reads global config and proxy-rotator state.
  - Latency / failure: yes — the primary caller is a background worker with metrics and retry-by-next-refresh semantics.
- **Activation shape (informational, not a selection criterion):** background feed-refresh worker, also reachable from manual HTTP refresh.
- **Confidence:** high — this is the central variable-cost unit; only an unexpectedly large remote dependency closure would change the ranking.
- **Risk notes:** Broad dependency closure (`storage`, HTTP fetcher, parser, processor, integrations, icon checker) and partial side effects across DB/external calls make the failure model more complex than a pure transform.

### C-2: Process entries from a fetched feed

- **Region root:** `evaluation/miniflux/internal/reader/processor/processor.go:27` — `ProcessFeedEntries` filters, cleans, optionally scrapes, rewrites, sanitizes, and computes reading time for every entry in a feed.
- **Caller(s):** `evaluation/miniflux/internal/reader/handler/handler.go:86` calls it during feed creation from discovery; `evaluation/miniflux/internal/reader/handler/handler.go:329` calls it during feed refresh.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — loops over feed entries, applies filters and URL cleanup, can scrape article pages, rewrites HTML, sanitizes content, and estimates or fetches reading time.
  - Load profile: yes — cost scales with number of entries, feed crawler settings, content size, and video-reading-time options.
  - Coherent unit: maybe — named function with a bounded purpose, but mutates the feed in place and consults storage for user and entry state.
  - State independence: maybe — durable store reads are acceptable, but per-entry `IsNewEntry`/`GetReadTime` calls and in-place mutation need care.
  - Latency / failure: yes — normally runs inside feed creation/refresh work where heavy per-feed latency is expected.
- **Activation shape (informational, not a selection criterion):** feed-refresh/feed-create inner transform.
- **Confidence:** high — this is where per-entry CPU and optional scraper work concentrates.
- **Risk notes:** A remote version would need to return the mutated entry list cleanly rather than relying on pointer mutation, and scraper subcalls can create nested remote/network behavior.

### C-3: Fetch and extract an entry web page

- **Region root:** `evaluation/miniflux/internal/reader/processor/processor.go:180` — `ProcessEntryWebPage` fetches an entry URL, extracts readable content, rewrites/sanitizes it, and updates reading time.
- **Caller(s):** `evaluation/miniflux/internal/api/entry_handlers.go:486` invokes it from the API fetch-content endpoint; `evaluation/miniflux/internal/ui/entry_scraper.go:54` invokes it from the web UI.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — combines outbound HTTP, HTML extraction, minification, rewrite rules, sanitization, and reading-time estimation.
  - Load profile: maybe — user-triggered per-entry fetches are uneven and payload-dependent, but not obviously a steady background hot path.
  - Coherent unit: yes — takes feed, entry, and user models, returns an error, and leaves persistence to the caller.
  - State independence: maybe — mostly per-call data plus config/proxy metrics, but it mutates the passed entry.
  - Latency / failure: maybe — synchronous API/UI path, though already dominated by remote page fetch and HTML processing.
- **Activation shape (informational, not a selection criterion):** HTTP route handler on-demand article extraction.
- **Confidence:** high — the work is obviously payload-sized; the main uncertainty is request-path tolerance.
- **Risk notes:** Caller-visible failure is immediate, and a remote version must preserve the mutated entry content/title/reading-time result.

### C-4: Push new entries to integrations

- **Region root:** `evaluation/miniflux/internal/integration/integration.go:511` — `PushEntries` sends a batch of newly discovered entries to enabled notification/webhook services.
- **Caller(s):** `evaluation/miniflux/internal/reader/handler/handler.go:349` starts it in a goroutine after a refresh creates new entries.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — fans out entry batches to Matrix, webhooks, Ntfy, Apprise, Discord, Slack, Pushover, Telegram, and Readeck with payload serialization and outbound calls.
  - Load profile: yes — burst size follows feed update bursts and tenant integration choices.
  - Coherent unit: yes — inputs are feed, entries, and integration settings; effects are external provider calls.
  - State independence: yes — does not require in-process mutable state beyond stable integration config and external clients.
  - Latency / failure: yes — launched asynchronously and logs provider failures rather than blocking feed refresh completion.
- **Activation shape (informational, not a selection criterion):** async goroutine after feed-refresh persistence.
- **Confidence:** high — this is a classic async fan-out region.
- **Risk notes:** Provider calls are sequential and mostly best-effort; retries/idempotency would need provider-specific treatment.

### C-5: Discover subscriptions for a URL

- **Region root:** `evaluation/miniflux/internal/reader/subscription/finder.go:44` — `FindSubscriptions` fetches a URL, detects direct feeds, parses HTML, checks canonical/YouTube/meta/RSS-Bridge/well-known feed sources, and returns candidates.
- **Caller(s):** `evaluation/miniflux/internal/api/subscription_handlers.go:52` invokes it from the API; `evaluation/miniflux/internal/ui/subscription_submit.go:72` invokes it from the UI subscription form.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — performs HTTP fetches, feed-format detection, charset-aware HTML parsing, DOM searches, optional RSS-Bridge detection, and multiple well-known URL probes.
  - Load profile: maybe — subscription creation is user-driven, but cost varies sharply by site and can spike during migrations or onboarding.
  - Coherent unit: maybe — the method has a clear return type but stores `feedDownloaded` and response info on the finder for later caller use.
  - State independence: maybe — per-call finder state is bounded, but the request builder and RSS-Bridge/config dependencies would need to be serialized or rebuilt remotely.
  - Latency / failure: maybe — synchronous request path, but already network-heavy and has clear error returns.
- **Activation shape (informational, not a selection criterion):** HTTP route handler for feed discovery.
- **Confidence:** medium — strong payload/network envelope, weaker load evidence.
- **Risk notes:** The post-call use of `FeedResponseInfo()` means remote activation must preserve more than just the returned subscription list.

### C-6: Extract readable article content

- **Region root:** `evaluation/miniflux/internal/reader/readability/readability.go:73` — `ExtractContent` parses an HTML page, removes unlikely nodes, scores candidates, and emits the selected article fragment.
- **Caller(s):** `evaluation/miniflux/internal/reader/scraper/scraper.go:61` invokes it when no same-site custom scraper rules apply.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — parses a DOM with goquery, recursively scores text/link density, removes candidates, and serializes the selected article.
  - Load profile: yes — cost scales with article page size and is triggered by crawler/manual content extraction on uneven feeds.
  - Coherent unit: yes — accepts an `io.Reader` and returns base URL, extracted content, and error.
  - State independence: yes — pure per-call HTML processing with no store or mutable global state needed for correctness.
  - Latency / failure: maybe — usually nested in scraper work that is already network-heavy, but a per-entry remote hop may be noticeable for small pages.
- **Activation shape (informational, not a selection criterion):** inner scraper CPU stage.
- **Confidence:** high — this is one of the cleanest CPU-heavy utility functions in the tree.
- **Risk notes:** Remote payload would be the fetched HTML body; lifting this alone does not offload the preceding network fetch.

### C-7: Sanitize entry HTML

- **Region root:** `evaluation/miniflux/internal/reader/sanitizer/sanitizer.go:217` — `SanitizeHTML` parses raw HTML, filters allowed tags/attributes recursively, resolves URLs, and renders safe HTML.
- **Caller(s):** `evaluation/miniflux/internal/reader/processor/processor.go:165` uses it for feed entries; `evaluation/miniflux/internal/reader/processor/processor.go:221` uses it for fetched entry pages.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — HTML parsing and recursive filtering scale with content size and nesting depth.
  - Load profile: yes — invoked per entry during feed processing and on manual content extraction, so large feeds or crawler bursts multiply calls.
  - Coherent unit: yes — string input/output plus options and base URL form a compact contract.
  - State independence: yes — uses stable allowlists/config and no durable or in-process mutable state for correctness.
  - Latency / failure: maybe — often on background paths, but also appears on synchronous content update/fetch paths.
- **Activation shape (informational, not a selection criterion):** inner feed/article HTML transformation.
- **Confidence:** medium — technically clean, but per-entry call granularity may make remote overhead unattractive unless content is large.
- **Risk notes:** A batched sanitizer would probably be better under feed refresh than lifting each small HTML fragment independently.

### C-8: Parse a feed document

- **Region root:** `evaluation/miniflux/internal/reader/parser/parser.go:20` — `ParseFeed` detects feed format and dispatches to Atom, RSS, JSON Feed, or RDF parsers to return a normalized feed model.
- **Caller(s):** `evaluation/miniflux/internal/reader/handler/handler.go:157` invokes it during feed creation; `evaluation/miniflux/internal/reader/handler/handler.go:297` invokes it during refresh after reading a response body.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — parses potentially large XML/JSON feed bodies and normalizes all entries into model objects.
  - Load profile: yes — scheduled refreshes create uneven parse bursts across tenants and feed sizes.
  - Coherent unit: yes — base URL plus `io.ReadSeeker` input returns `*model.Feed` or error.
  - State independence: yes — parser selection and adapters are pure per-call work.
  - Latency / failure: yes — often nested in background refresh; parse errors already have localized error handling upstream.
- **Activation shape (informational, not a selection criterion):** feed-refresh/feed-create parser stage.
- **Confidence:** high — clean contract and payload-proportional CPU.
- **Risk notes:** The `io.ReadSeeker` input would likely need a byte-slice wrapper for remote serialization; lifting only parsing leaves fetch and DB work local.

### C-9: Update or create a feed icon

- **Region root:** `evaluation/miniflux/internal/reader/icon/checker.go:28` — `UpdateOrCreateFeedIcon` discovers, downloads, resizes/minifies, hashes, and stores a feed icon.
- **Caller(s):** `evaluation/miniflux/internal/reader/handler/handler.go:201` runs it after feed creation; `evaluation/miniflux/internal/reader/handler/handler.go:358` runs it on forced refresh.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — may fetch pages/icons, parse HTML for icon links, decode/resize images, minify SVG, hash bytes, and write icon records.
  - Load profile: maybe — normally bounded per feed, but imports, new subscriptions, and forced refreshes can create bursts.
  - Coherent unit: maybe — method purpose is clear, but it captures `store` and `feed` through the checker object.
  - State independence: maybe — persistent side effects are through storage, while request setup uses stable config/proxy state.
  - Latency / failure: maybe — often attached to feed creation/refresh; failures are logged and non-fatal, but create-time icon work is synchronous.
- **Activation shape (informational, not a selection criterion):** feed refresh/create post-processing.
- **Confidence:** medium — useful when icons are large or remote sites are slow, but ordinary favicons are small.
- **Risk notes:** The helper hides significant work in `findIcon`/`resizeIcon`; lifting at this method boundary would include DB writes and network discovery.

### C-10: Send one saved entry to integrations

- **Region root:** `evaluation/miniflux/internal/integration/integration.go:41` — `SendEntry` sends a manually saved entry to enabled bookmarking/archive/webhook providers.
- **Caller(s):** `evaluation/miniflux/internal/api/entry_handlers.go:264` launches it from the API save endpoint; `evaluation/miniflux/internal/ui/entry_save.go:36` launches it from the UI.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — fans one entry out to many possible external providers, some with auth exchanges and full-content JSON payloads.
  - Load profile: maybe — user saves are uneven and provider count varies, but this is not as naturally bursty as feed refresh.
  - Coherent unit: yes — entry plus integration settings are explicit inputs.
  - State independence: yes — no local store dependency; effects are external API calls.
  - Latency / failure: yes — both callers run it asynchronously and return accepted/created immediately.
- **Activation shape (informational, not a selection criterion):** async goroutine after user save action.
- **Confidence:** medium — strong isolation and failure tolerance, weaker load profile.
- **Risk notes:** Large provider switchboard increases dependency closure; lack of durable queue/retry means remote failure handling would need policy.

### C-11: Import OPML subscriptions

- **Region root:** `evaluation/miniflux/internal/reader/opml/handler.go:41` — `Import` parses an OPML document, creates missing categories, and creates missing feed records.
- **Caller(s):** `evaluation/miniflux/internal/api/opml_handlers.go:27` invokes it for API import; `evaluation/miniflux/internal/ui/opml_upload.go:58` invokes it for uploaded files.
- **Why useful (rubric scoring):**
  - Compute envelope: maybe — XML parsing and recursive outline traversal are payload-sized, with DB work per subscription.
  - Load profile: maybe — imports are infrequent but can be large and spiky during migrations or onboarding.
  - Coherent unit: maybe — clear `(userID, io.Reader)` method, but bound to a storage-backed handler.
  - State independence: maybe — durable DB effects only, but category/feed existence checks and creation are interleaved.
  - Latency / failure: maybe — synchronous upload/fetch request path, though a large import is already a long-running user operation.
- **Activation shape (informational, not a selection criterion):** HTTP OPML upload/API import handler.
- **Confidence:** low — useful for large imports, but not a recurring hot path.
- **Risk notes:** This import does not fetch or validate each feed; DB round trips may dominate more than app CPU.

### C-12: Rewrite entry HTML media URLs for proxying

- **Region root:** `evaluation/miniflux/internal/mediaproxy/rewriter.go:23` — `RewriteDocumentWithAbsoluteProxyURL` parses entry HTML and rewrites media URLs to signed proxy URLs.
- **Caller(s):** `evaluation/miniflux/internal/api/entry_handlers.go:39` rewrites a single API entry; `evaluation/miniflux/internal/api/entry_handlers.go:191` rewrites each entry in API list responses.
- **Why useful (rubric scoring):**
  - Compute envelope: maybe — goquery DOM parsing, src/srcset iteration, HMAC/base64 URL signing, and serialization scale with entry HTML and media count.
  - Load profile: maybe — called on API reads and list responses, so heavy clients with large entries can create bursts.
  - Coherent unit: yes — pure string-to-string function from the caller perspective.
  - State independence: yes — depends only on stable media-proxy configuration.
  - Latency / failure: maybe — synchronous response path, so remote overhead only makes sense for large media-heavy documents.
- **Activation shape (informational, not a selection criterion):** API response HTML transformation.
- **Confidence:** low — clean boundary, but many calls may be too small to amortize remote dispatch.
- **Risk notes:** When media proxy mode is `none`, the function exits cheaply; a lift oracle would need to avoid dispatching those no-op calls.

Honest assessment: I am most confident in `RefreshFeed`, `ProcessFeedEntries`, `ProcessEntryWebPage`, `PushEntries`, `ParseFeed`, and `ExtractContent` because their work is plainly variable and either background or already network/payload dominated. `SanitizeHTML` is technically clean but may be too fine-grained without batching, while icon discovery, save-entry integrations, OPML import, and media-proxy rewriting are genuinely marginal and depend heavily on tenant behavior and configuration. I suspect the feed-refresh path could be split into an even better "fetch/parse/process one feed payload" boundary, but the existing code exposes that work mainly through the broad `RefreshFeed` orchestration and nested mutating helpers.
