# Miniflux — useful lift regions (Phase 1)

## Project read

Miniflux is a single-binary RSS/Atom/JSON feed aggregator backed by Postgres. The HTTP surface (REST API at `internal/api/`, browser UI at `internal/ui/`, plus Fever and Google Reader compatibility shims at `internal/fever/` and `internal/googlereader/`) is thin glue around three computationally substantive subsystems: (1) **feed refresh** — fetch the feed, parse XML/JSON, scrape original article HTML for each new entry, run the readability extractor, sanitize the HTML, estimate reading time — driven by a worker pool over a database-derived job queue (`internal/worker`, `internal/cli/scheduler.go`); (2) **third-party fan-out** — push new or saved entries to ~30 external integrations (`internal/integration/`) over HTTP; (3) **on-demand transforms on read paths** — favicon discovery/resize, media-proxy URL rewriting, custom-content fetch on user click. The expensive work clusters squarely in `internal/reader/{processor,scraper,readability,sanitizer,icon,parser}` and `internal/integration/`. The HTTP layer is a Go 1.22 stdlib `ServeMux` plus a few middleware wrappers — nothing exotic. Below are the candidate lift regions, ranked best to most marginal.

---

### C-1: `RefreshFeed` — full per-feed refresh pipeline

- **Region root:** `internal/reader/handler/handler.go:207` — `RefreshFeed(store, userID, feedID, forceRefresh)`. Fetches the feed URL, parses RSS/Atom/JSON/RDF, runs `processor.ProcessFeedEntries` (which scrapes + sanitizes + reading-time-estimates new entries), persists deltas, and forks integration push-out.
- **Caller(s):** `internal/worker/worker.go:40` (background pool); `internal/cli/refresh_feeds.go:56` (cron-style CLI mode); `internal/api/feed_handlers.go:67` (REST `PUT /v1/feeds/{id}/refresh`); `internal/ui/feed_refresh.go:21` (web UI). Same region, four activation paths.
- **Why useful (rubric scoring):**
  - Compute envelope: **yes** — outbound HTTP fetch (gzip/charset-decoded), full XML/JSON parse, then per-new-entry: another outbound fetch + readability extraction (DOM walking, candidate scoring) + HTML sanitization + reading-time estimation. Easily 100s of ms to seconds per feed; minutes for crawler-enabled feeds.
  - Load profile: **yes** — bursty by construction. The scheduler (`internal/cli/scheduler.go:33`) and the per-user "refresh all" handler push batches of jobs onto a buffered queue at the polling tick or on user demand.
  - Coherent unit: **yes** — clean `(store, userID, feedID, forceRefresh)` signature; all I/O goes through the `storage.Storage` interface and the `fetcher.RequestBuilder`. No package globals mutated.
  - State independence: **yes** — reads `originalFeed` from DB, writes deltas via `store.RefreshFeedEntries` and `store.UpdateFeed`. The only in-process side-effect is `metric.BackgroundFeedRefreshDuration.Observe` (Prometheus collector) and a fire-and-forget `go integration.PushEntries(...)` at handler.go:349 — both are replica-local-friendly.
  - Latency / failure: **yes** — every caller is async (worker goroutine) or is a refresh endpoint that already has multi-second latency. Failure path is recorded via `store.UpdateFeedError` and the user gets a translated error; nothing breaks atomically.
- **Activation shape:** channel-fed worker goroutine + three direct HTTP-handler call sites.
- **Confidence:** high — this is the canonical "scheduled batch worker" lift target, exactly the calibration shape the rubric calls out.
- **Risk notes:** drags in `processor`, `scraper`, `readability`, `sanitizer`, `parser`, `fetcher`, `icon`, `integration` — non-trivial dependency closure. The `processor.ProcessFeedEntries` call inside it is itself a candidate (C-2), so a lift here would also lift C-2; if the goal is per-region experimentation, picking the inner region is cleaner.

### C-2: `ProcessFeedEntries` — per-entry scrape/sanitize/score loop

- **Region root:** `internal/reader/processor/processor.go:27` — `ProcessFeedEntries(store, feed, userID, forceRefresh)`. Iterates the just-parsed entry list; for each one: URL clean, conditional crawler scrape (outbound HTTP + Readability extract), rewrite-rules pass, HTML sanitization, reading-time estimation, optional bulk YouTube watch-time fetch.
- **Caller(s):** `internal/reader/handler/handler.go:189` (CreateFeed), `:329` (RefreshFeed). Always called on the back of a feed fetch.
- **Why useful (rubric scoring):**
  - Compute envelope: **yes** — for crawler-enabled feeds the inner loop does an outbound HTTP fetch *per entry*, then runs `readability.ExtractContent` (DOM scoring at `internal/reader/readability/readability.go:73`, ~389 LOC of goquery walking) and `sanitizer.SanitizeHTML` (~673 LOC, recursive HTML walk with tag/attribute allowlists). Easily the dominant CPU+IO consumer in the binary.
  - Load profile: **yes** — same bursty profile as C-1, scaled by `len(feed.Entries)`. A noisy upstream feed amplifies the cost.
  - Coherent unit: **maybe** — signature is clean (`*Storage`, `*Feed`, `userID`, `bool`), but mutates `feed.Entries` in place. That's an in/out parameter, not external state, so it's fine if the caller passes the feed across the boundary.
  - State independence: **yes** — same as C-1. No package globals; metric collector is the only side observer.
  - Latency / failure: **yes** — only called from `RefreshFeed`/`CreateFeed`, both async or already slow. A scraper failure for one entry is logged and the entry keeps its original content.
- **Activation shape:** function called by the feed-refresh region on every refresh.
- **Confidence:** high — this is the actual hot-loop. Lifting at this granularity (vs. C-1) gives better isolation: the metric oracle can decide based on entry count or per-entry scrape time.
- **Risk notes:** mutates the passed `*model.Feed.Entries` slice — fine over RPC if the feed is round-tripped, but worth noting. Reuses one `requestBuilder` across iterations; the bulk YouTube call (`fetchYouTubeWatchTimeInBulk`, processor.go:173) is an additional outbound batch dispatch.

### C-3: `ScrapeWebsite` — single-URL scrape + Readability extraction

- **Region root:** `internal/reader/scraper/scraper.go:21` — `ScrapeWebsite(requestBuilder, pageURL, rules)`. Outbound HTTP, content-type check, charset decode, then either custom CSS-rule extraction (goquery) or Mozilla-style Readability (DOM scoring + candidate selection).
- **Caller(s):** `internal/reader/processor/processor.go:111` (per-entry inside ProcessFeedEntries) and `:195` (`ProcessEntryWebPage`, called by both `internal/api/entry_handlers.go:486` `fetchContentHandler` and `internal/ui/entry_scraper.go:54` `fetchContent` — both are user-pull "fetch full article" actions).
- **Why useful (rubric scoring):**
  - Compute envelope: **yes** — outbound HTTP, then a 389-LOC Readability pass (goquery DOM walking, candidate scoring across `section,h2..h6,p,td,pre,div`). The dedicated `BenchmarkExtractContent` at `internal/reader/readability/readability_test.go:18` is itself evidence the maintainers consider this hot.
  - Load profile: **yes** — bursty: amplifies with crawler-enabled feed refreshes, plus user "fetch original content" clicks.
  - Coherent unit: **yes** — `(requestBuilder, pageURL, rules) → (baseURL, content, err)`. One of the cleanest signatures in the codebase.
  - State independence: **yes** — pure-ish: takes a configured `requestBuilder` (value-typed), produces strings. No package globals beyond `predefinedRules` (read-only map of per-domain CSS rules).
  - Latency / failure: **yes** — caller in the refresh path is async (goroutine); user-facing `fetchContent` is an explicit "give me the article" click where multi-second latency is already expected.
- **Activation shape:** function call from feed-refresh loop and from two HTTP handlers.
- **Confidence:** high — narrow contract, clearly CPU-bound in the readability path, repeatedly invoked under load.
- **Risk notes:** the predefined-rules map (`internal/reader/scraper/rules.go`) is the only external coupling, and it's static.

### C-4: `SanitizeHTML` — HTML allowlist sanitizer

- **Region root:** `internal/reader/sanitizer/sanitizer.go:217` — `SanitizeHTML(baseURL, rawHTML, *SanitizerOptions) string`. Parses HTML with `golang.org/x/net/html`, walks the DOM recursively (max depth 512) applying tag/attribute allowlists, srcset/iframe domain checks, link-rel rewriting.
- **Caller(s):** `internal/reader/processor/processor.go:165` and `:221` (every entry on every refresh / on user content-fetch); `internal/api/entry_handlers.go:314` (PUT entry) and `:401` (POST entry import).
- **Why useful (rubric scoring):**
  - Compute envelope: **yes** — recursive HTML parse + walk is proportional to entry size; ~673 LOC of allowlist logic; non-trivial constant. Runs on essentially every entry produced or modified.
  - Load profile: **yes** — bursty (rides on top of C-1/C-2's batches) and also on the import-entry API path which can flood during OPML import or external-client backfills.
  - Coherent unit: **yes** — pure function: `(baseURL, rawHTML, options) → sanitizedHTML`. No I/O, no DB, no globals beyond static allowlist tables. Trivial to wire over RPC.
  - State independence: **yes** — pure. The allowlist maps are package-level constants. `config.Opts.YouTubeEmbedDomain()` and `InvidiousInstance()` are read once per call but are stable config.
  - Latency / failure: **yes** — caller is the refresh worker (async) or an HTTP handler that already does DB work. A sanitizer failure currently returns `""` — the failure mode is already silent.
- **Activation shape:** ordinary function call from refresh pipeline and from a handful of API handlers.
- **Confidence:** high — pure CPU-bound function with a clean signature is essentially the textbook lift candidate.
- **Risk notes:** very small — the only thing to think about is that the lifted replica needs the same `config.Opts` snapshot for the embed-domain decisions, which is config not state.

### C-5: `iconChecker.UpdateOrCreateFeedIcon` — favicon discovery, decode, resize, store

- **Region root:** `internal/reader/icon/checker.go:28` — builds an HTTP requestBuilder, runs `iconFinder.findIcon` (`internal/reader/icon/finder.go:49`) which does HTML scraping for `<link rel="icon">` candidates, downloads icons, and `resizeIcon` (`finder.go:186`) which decodes JPEG/PNG/GIF/WebP, runs bilinear resize via `golang.org/x/image/draw`, re-encodes as PNG, or minifies SVG.
- **Caller(s):** `internal/reader/handler/handler.go:110` (CreateFeedFromSubscriptionDiscovery), `:201` (CreateFeed), `:358` and `:360` (RefreshFeed — `UpdateOrCreate` on force, `CreateFeedIconIfMissing` otherwise).
- **Why useful (rubric scoring):**
  - Compute envelope: **yes** — image decode + bilinear-scale + PNG re-encode is real CPU work; SVG minification is meaningful too. Plus N outbound HTTP calls per discovery.
  - Load profile: **maybe** — runs at most once per feed creation and on forced refreshes; `CreateFeedIconIfMissing` short-circuits when an icon exists. Bursty during OPML import (when many feeds are created at once) but less hot in steady state.
  - Coherent unit: **yes** — a small struct (`store`, `feed`) and a no-arg method; everything else is local.
  - State independence: **yes** — output written via `store.StoreFeedIcon`; no shared in-process mutable state.
  - Latency / failure: **yes** — never on a tight critical path; failure is logged and the feed proceeds without an icon.
- **Activation shape:** function called from the feed-refresh / feed-create paths.
- **Confidence:** medium — clear CPU envelope on the resize step, but call frequency is much lower than C-1..C-4.
- **Risk notes:** depends on `c4.image` (webp), `golang.org/x/image/draw`, `tdewolff/minify` — these are heavy but self-contained.

### C-6: `integration.PushEntries` — third-party fan-out for new entries

- **Region root:** `internal/integration/integration.go:511` — `PushEntries(feed, entries, userIntegrations)`. Branches on each enabled integration flag (Matrix, Webhook, Ntfy, Apprise, Discord, Slack, Pushover, Telegram, Readeck) and dispatches outbound HTTP/SMTP/etc.
- **Caller(s):** `internal/reader/handler/handler.go:349` — `go integration.PushEntries(originalFeed, newEntries, userIntegrations)` after a successful refresh produces new entries.
- **Why useful (rubric scoring):**
  - Compute envelope: **yes** — IO-dominated outbound HTTP fan-out. Per-integration JSON marshal + outbound POST + retry semantics. Cost scales with both number of integrations enabled per user and `len(entries)`.
  - Load profile: **yes** — bursty fan-out triggered by every successful refresh that produced new entries; spikes when a popular feed publishes a batch.
  - Coherent unit: **yes** — pure-value inputs (`*Feed`, `Entries`, `*Integration`); no return value. Each integration has its own client constructor under `internal/integration/{name}`.
  - State independence: **yes** — strictly outbound; no in-process state mutated.
  - Latency / failure: **yes** — already invoked as a fire-and-forget goroutine (`go integration.PushEntries(...)`); failures are logged. Perfectly natural offload.
- **Activation shape:** goroutine launched from the refresh handler.
- **Confidence:** high — already async, already bounded I/O, naturally batched. The fan-out also makes it a useful experimental target where the metric oracle can decide based on number-of-enabled-integrations × entry count.
- **Risk notes:** drags in ~25 integration sub-packages (~30 KLOC total of HTTP clients). Could be sliced narrower per-integration if dependency closure becomes a problem.

### C-7: `integration.SendEntry` — per-entry "save to bookmarking service" fan-out

- **Region root:** `internal/integration/integration.go:41` — `SendEntry(entry, userIntegrations)`. ~470-line `if userIntegrations.XEnabled { client.Save(...) }` cascade across ~20 services (Pinboard, Wallabag, Pocket-likes, Notion, Linkding, Linkwarden, Readeck, Readwise, Cubox, Shaarli, Archive.org, Webhook, Omnivore, Karakeep, Raindrop, ...).
- **Caller(s):** `internal/api/entry_handlers.go:264` (`go integration.SendEntry(entry, settings)` from the REST `POST /v1/entries/{id}/save`); `internal/ui/entry_save.go:36`; `internal/fever/handler.go:461`; `internal/googlereader/handler.go:315`. Four call sites, all `go ...`.
- **Why useful (rubric scoring):**
  - Compute envelope: **yes** — outbound HTTP per enabled integration; some integrations submit the full entry content (Wallabag, NunuxKeeper, Linktaco, Readeck, Linkwarden), so payloads are not trivial.
  - Load profile: **yes** — bursty on user "save" clicks; a power user save-storming a backlog produces a spike.
  - Coherent unit: **yes** — clean `(*Entry, *Integration)` signature, no shared state.
  - State independence: **yes** — strictly outbound HTTP; no in-process mutation.
  - Latency / failure: **yes** — every caller is `go ...`; the API responds `JSONAccepted` immediately. Failure-tolerance is built into the calling contract.
- **Activation shape:** goroutine launched from each of four HTTP handlers.
- **Confidence:** high — same shape as C-6 but on the user-driven side. SPRINT-0033 already flagged this as the canonical "many-to-one fan-in" structural target; it also passes the utility rubric on its own merits.
- **Risk notes:** same dependency-closure concern as C-6. The fact that four different framework dialects (REST, web UI, Fever, Google Reader) all enter at the same region is also why it's interesting.

### C-8: `subscriptionFinder.FindSubscriptions` — feed discovery from website URL

- **Region root:** `internal/reader/subscription/finder.go:44` — fetches the website, detects whether it's already a feed, parses HTML, walks `<link rel=alternate>` meta tags, runs YouTube-page heuristics, falls back to RSSBridge probing, and may try a curated list of well-known feed paths.
- **Caller(s):** `internal/api/subscription_handlers.go:52` (REST `POST /v1/discover`); `internal/ui/subscription_submit.go:72` (browser "subscribe" form); `internal/googlereader/handler.go:357`.
- **Why useful (rubric scoring):**
  - Compute envelope: **maybe → yes** — at minimum one outbound HTTP + HTML parse; in many cases a series of follow-up probes (YouTube channel ID lookup, well-known feed paths, RSSBridge query). Mixed CPU+IO; bounded but non-trivial.
  - Load profile: **yes** — bursty around campaign launches and OPML imports; otherwise spaced-out user actions.
  - Coherent unit: **yes** — `(websiteURL, rssBridgeURL, rssBridgeToken) → (Subscriptions, error)`. Constructed via `NewSubscriptionFinder(requestBuilder)`; clean.
  - State independence: **yes** — mutates only its own struct fields (`feedDownloaded`, `feedResponseInfo`); no globals.
  - Latency / failure: **yes** — invoked from the user-facing "Add subscription" flow which already shows a spinner; failure is a localized error message.
- **Activation shape:** synchronous handler call from three HTTP entry points.
- **Confidence:** medium — work envelope varies a lot (sometimes the very first byte is already a feed and the function returns immediately). The worst-case cost is high but the median may be modest.
- **Risk notes:** depends on `integration/rssbridge` for the optional fallback.

### C-9: `OPML Handler.Import` — bulk feed import

- **Region root:** `internal/reader/opml/handler.go:41` — parses an OPML file, then for each `<outline>` looks up or creates the category and inserts the feed.
- **Caller(s):** `internal/api/opml_handlers.go:27` (REST `POST /v1/import`); also wired through the UI's `opml_upload.go`.
- **Why useful (rubric scoring):**
  - Compute envelope: **maybe** — XML parse cost scales with the import file (some users import 100s–1000s of subscriptions); each subscription triggers a category lookup + a `CreateFeed` insert. Note that `CreateFeed` here does *not* immediately fetch the feed (it just inserts a row); the real refresh work is deferred to the worker pool.
  - Load profile: **maybe** — uniformly low traffic in steady state; spiky on initial onboarding. The rubric calls "uniformly low-traffic" out as negative; this one only spikes during onboarding so it's borderline.
  - Coherent unit: **yes** — `(userID int64, data io.Reader) → error`.
  - State independence: **yes** — DB-only side-effects.
  - Latency / failure: **yes** — already a slow user action; the OPML upload page expects a multi-second wait.
- **Activation shape:** synchronous handler call from the import endpoint.
- **Confidence:** medium — useful as a "rare but heavy" target; less compelling than C-1..C-6.
- **Risk notes:** the call to `store.CreateFeed` does not run the refresh fetch, so most of the "expense" in OPML import really shows up later in the worker pool's consumption of the freshly inserted rows — i.e. C-1's load is amplified.

### C-10: `mediaproxy.RewriteDocumentWithAbsoluteProxyURL` — HTML rewrite for media-proxy URLs

- **Region root:** `internal/mediaproxy/rewriter.go:23` (and the shared `genericProxyRewriter` at `:27`). Parses the entry HTML with goquery and rewrites `<img>`/`<picture>`/`<audio>`/`<video>` `src`/`srcset`/`poster` URLs to point at the local media-proxy endpoint.
- **Caller(s):** `internal/api/entry_handlers.go:39` (`getEntryFromBuilder`, called by every entry GET — single, feed-scoped, category-scoped); `:191` (`findEntries`, in a loop over the page of returned entries); `:499` (`fetchContentHandler`); `internal/googlereader/handler.go:694`; `internal/fever/handler.go:319`.
- **Why useful (rubric scoring):**
  - Compute envelope: **maybe → yes** — goquery parse-and-walk per entry; trivial for short text but real for long articles, and importantly *amortized over batch reads* in `findEntries` where it loops over up to 100 entries per page.
  - Load profile: **yes** — every list-entries API call rewrites every entry; mobile clients that poll aggressively (Fever / Google Reader API consumers) drive uniform-but-substantial load.
  - Coherent unit: **yes** — pure `string → string`.
  - State independence: **yes** — pure; no shared state.
  - Latency / failure: **maybe → no** — this one is on the request critical path of a list-entries response that mobile clients expect to feel snappy. An extra network hop per entry × 100 entries would be a regression unless batched.
- **Activation shape:** function call inside read-path HTTP handlers.
- **Confidence:** medium-low — would only be a useful lift candidate if rewritten in batched form (one RPC for the whole page); per-entry remote dispatch would lose. Keeping it for completeness but flagging the latency concern.
- **Risk notes:** the per-entry call site at `entry_handlers.go:191` is the main tension with the rubric's "tight synchronous request path" negative.

### C-11: `oauth2.googleProvider.Profile` / `oidcProvider.Profile` — OAuth2 token exchange + profile fetch

- **Region root:** `internal/oauth2/google.go:57` (`(*googleProvider).Profile`) and `internal/oauth2/oidc.go:64` (`(*oidcProvider).Profile`). Each does a token-exchange POST to the provider, then a GET of the user-info endpoint, then JSON decode and (for OIDC) ID-token verification.
- **Caller(s):** `internal/ui/oauth2_callback.go:56` — `authProvider.Profile(r.Context(), code, codeVerifier)`.
- **Why useful (rubric scoring):**
  - Compute envelope: **maybe** — two outbound HTTPS round-trips + JSON decode + (OIDC only) JWT signature verification. Latency-dominated but nontrivial.
  - Load profile: **yes** — calibration example in the rubric explicitly names "OAuth callback flurries during a campaign launch" as a target.
  - Coherent unit: **yes** — `Profile(ctx, code, codeVerifier) → (*UserProfile, error)`; behind the `Provider` interface (`internal/oauth2/provider.go`), which is exactly the kind of seam Monolift annotates today.
  - State independence: **yes** — provider struct is stable config; no shared mutable state.
  - Latency / failure: **maybe** — caller is on the OAuth callback request path, but that path is already user-visible IO-bound (two outbound HTTPS calls), so an extra hop is in the noise per the rubric's own framing.
- **Activation shape:** interface-method call from a single HTTP handler.
- **Confidence:** medium — fits the calibration shape exactly, but per-call cost is moderate and traffic at most installs is low. Useful as a "small-but-clean interface-method" specimen.
- **Risk notes:** OIDC needs the `*oidc.Provider` discovery doc populated at construction; the lifted replica must initialize it once.

### C-12: `mediaProxy` — proxy-fetch handler for inline media

- **Region root:** `internal/ui/proxy.go:27` — `(*handler).mediaProxy`. Validates an HMAC-signed URL, executes an outbound fetch through the `fetcher.RequestBuilder`, forwards headers, and streams the response body to the client with a 72-hour cache header.
- **Caller(s):** registered as a route in `internal/ui/routes.go` (the `mediaProxy` URL pattern referenced from `mediaproxy.ProxifyAbsoluteURL`).
- **Why useful (rubric scoring):**
  - Compute envelope: **maybe → no** — it's a streaming reverse-proxy: outbound fetch + body relay. Body bytes pass through; CPU is low. The rubric's "negative: a function that does only a single tiny DB read and returns a struct" doesn't quite apply, but neither does its "compute-bound" criterion.
  - Load profile: **yes** — every HTML page render with images can issue dozens of these.
  - Coherent unit: **yes** — `http.Handler` shape.
  - State independence: **yes** — pure stream relay; no shared state.
  - Latency / failure: **no** — it streams the response body inline; the caller is a `<img src=...>` browser request expecting fast first-byte. Adding an extra hop would degrade page paint.
- **Activation shape:** HTTP route handler.
- **Confidence:** low — included for completeness; the streaming-relay shape and inline-resource latency profile push it under the bar. Keeping it as a documented near-miss rather than a positive recommendation; **downgrade with reasoning**.
- **Risk notes:** disqualified-adjacent: it's not a long-lived per-request connection (no SSE/WebSocket), but it does stream a response body whose timing matters to the browser.

---

## Honest assessment

Most confident: **C-1, C-2, C-3, C-4, C-6, C-7**. The feed-refresh pipeline (C-1) and its hot inner loop (C-2) are textbook fits — bursty cron-driven worker, async caller, expensive per-call work, clean function signatures. The Readability extractor (C-3) and HTML sanitizer (C-4) are the cleanest pure-CPU lifts in the repo. The integration fan-outs (C-6, C-7) are already invoked as goroutines, so the "latency tolerance" criterion is satisfied by the existing call convention. **C-5** (icon discovery+resize) is solid but called less often. **C-8** (subscription discovery) and **C-9** (OPML import) are real lift targets that just don't fire as often as the others.

Genuinely marginal: **C-10** (mediaproxy HTML rewrite) is on the fence because the per-entry call site loops over up to 100 entries inside a request handler — a per-entry remote call would lose, but a batched version would be useful; I included it because the load profile is correct even if the contract isn't ideal as written. **C-11** (OAuth2 Profile) is a clean interface and matches the rubric's calibration example, but per-call cost is moderate and traffic at typical installs is low. **C-12** (`mediaProxy` HTTP handler) is downgraded explicitly: streaming a response body inline to a browser image fetch is the wrong latency profile to lift.

Suspected-but-uncited: I suspect the **Postgres full-text search path** (`store.NewEntryQueryBuilder().WithSearchQuery(...)`, exercised from `internal/api/entry_handlers.go:553` and the UI search) is computationally meaningful per query and bursty per-user, but I couldn't find a coherent function-level region — search is buried inside the entry-query-builder fluent chain that already smuggles a `*Storage` and assembles SQL — so there's no clean "named, callable region with a clear input/output contract" to point at, which the Coherent-unit criterion rules out without source-restructuring. Similarly, the **Postgres entry-write path** during `store.RefreshFeedEntries` is plausibly the dominant cost per refresh after scraping, but the rubric's State-independence criterion is squarely about the in-process side; the heavy work is already in Postgres, not in Go, so lifting it offers nothing.
