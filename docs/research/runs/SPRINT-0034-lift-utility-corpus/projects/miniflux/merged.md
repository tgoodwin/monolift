# miniflux — merged lift-region candidate set (Phase 2b final)

## Methodology note

Phase 1 drafts: claude, codex, gemini.
Phase 2a critics: each model reviewed the other two.
Phase 2b aggregator: claude (opus).

Inclusion follows the deterministic rules in `PHASE2-PLAN.md`. Where critics issued MODIFY verdicts (almost always to correct gemini's caller-line cites), the corrections are applied below. Three overlapping picks across drafts collapse onto the same source region; provenance and verdicts are summarized per merged entry.

## Merged candidates (ranked strongest → weakest)

### M-1: `RefreshFeed` — full per-feed refresh pipeline

- **pick_provenance:** claude+codex+gemini (3/3)
- **critique_status:** KEEP from claude (on codex), KEEP from codex (on claude), KEEP from gemini on both; MODIFY from claude and from codex on gemini (caller cite drift fixed).
- **Region root:** `evaluation/miniflux/internal/reader/handler/handler.go:207` — `RefreshFeed(store, userID, feedID, forceRefresh)` fetches the feed URL, parses RSS/Atom/JSON/RDF, runs `processor.ProcessFeedEntries` over new entries, writes deltas, fires the integration push-out as a goroutine, and updates the feed icon.
- **Caller(s):** `evaluation/miniflux/internal/worker/worker.go:40` (background pool); `evaluation/miniflux/internal/cli/refresh_feeds.go:56` (cron-style CLI mode); `evaluation/miniflux/internal/api/feed_handlers.go:67` (REST `PUT /v1/feeds/{id}/refresh`); `evaluation/miniflux/internal/ui/feed_refresh.go:21` (web UI).
- **Why useful (rubric scoring):**
  - Compute envelope: yes — outbound HTTP fetch + full feed parse + per-new-entry scrape/extract/sanitize/reading-time. 100s of ms to multi-second per feed; minutes for crawler-enabled feeds.
  - Load profile: yes — bursty by construction; the scheduler at `internal/cli/scheduler.go:33` pushes batches onto a buffered queue at every polling tick.
  - Coherent unit: maybe — clean `(store, userID, feedID, forceRefresh)` signature, but the orchestrator combines fetch, parse, persistence, integration launch, and icon work; codex flagged the breadth as a hedge.
  - State independence: maybe — DB- and config-mediated, but reads `config.Opts` and the proxy-rotator package state, and forks `go integration.PushEntries(...)` at handler.go:349. Replica-local-friendly with care.
  - Latency / failure: yes — every caller is async (worker goroutine) or already an explicit multi-second refresh endpoint; failure path is recorded via `store.UpdateFeedError` with no atomicity requirement.
- **Activation shape:** channel-fed worker goroutine + three direct HTTP-handler call sites.
- **Confidence:** high — canonical "scheduled batch worker" lift target, the calibration shape the rubric calls out by name.
- **Risk notes:** drags in `processor`, `scraper`, `readability`, `sanitizer`, `parser`, `fetcher`, `icon`, `integration` — broad dependency closure. Lifting at this granularity also lifts every inner candidate (M-2, M-3, M-5, M-6, M-7, M-8, M-9, M-12); experiments that want finer isolation should pick the inner regions.

### M-2: `ProcessFeedEntries` — per-entry scrape/sanitize/score loop

- **pick_provenance:** claude+codex+gemini (3/3)
- **critique_status:** KEEP from all three foreign critiques; MODIFY from claude and from codex on gemini (caller cites corrected).
- **Region root:** `evaluation/miniflux/internal/reader/processor/processor.go:27` — `ProcessFeedEntries(store, feed, userID, forceRefresh)`. Iterates `feed.Entries`; for each one: URL clean, conditional crawler scrape (outbound HTTP + Readability extract), rewrite-rules pass, `sanitizer.SanitizeHTML`, reading-time estimation, optional bulk YouTube watch-time fetch.
- **Caller(s):** `evaluation/miniflux/internal/reader/handler/handler.go:86` (CreateFeedFromSubscriptionDiscovery), `:189` (CreateFeed), `:329` (RefreshFeed). Always called on the back of a feed fetch.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — for crawler-enabled feeds the inner loop does an outbound HTTP fetch *per entry* plus the 389-LOC `readability.ExtractContent` and 673-LOC `sanitizer.SanitizeHTML`. Dominant CPU+IO consumer in the binary.
  - Load profile: yes — same bursty profile as M-1, scaled by `len(feed.Entries)`. A noisy upstream amplifies cost.
  - Coherent unit: maybe — clean signature but mutates `feed.Entries` in place. Usable as in/out over RPC if the feed is round-tripped.
  - State independence: maybe — no package globals beyond the metric collector; reads `IsNewEntry`/`GetReadTime` from storage and consults user prefs, all via the storage interface.
  - Latency / failure: yes — only called from `RefreshFeed`/`CreateFeed`, both async or already slow; per-entry scraper failures are logged and the entry retains its original content.
- **Activation shape:** function called by the feed-refresh region on every refresh.
- **Confidence:** high — actual per-entry hot-loop; preferred granularity over M-1 if the metric oracle should decide on entry count or per-entry scrape time.
- **Risk notes:** mutates `*model.Feed.Entries` (an in/out parameter, not external state). Reuses one `requestBuilder` across iterations; the bulk YouTube watch-time call (`fetchYouTubeWatchTimeInBulk` at processor.go:173) is an additional outbound batch dispatch.

### M-3: `SanitizeHTML` — HTML allowlist sanitizer

- **pick_provenance:** claude+codex+gemini (3/3)
- **critique_status:** KEEP from all three; MODIFY from claude and from codex on gemini (region root corrected `:193`→`:217`, caller cites corrected).
- **Region root:** `evaluation/miniflux/internal/reader/sanitizer/sanitizer.go:217` — `SanitizeHTML(baseURL, rawHTML, *SanitizerOptions) string`. Parses HTML with `golang.org/x/net/html`, walks the DOM recursively (max depth 512) applying tag/attribute allowlists, srcset/iframe domain checks, link-rel rewriting.
- **Caller(s):** `evaluation/miniflux/internal/reader/processor/processor.go:165` and `:221` (every entry on every refresh / on user content-fetch); `evaluation/miniflux/internal/api/entry_handlers.go:314` (PUT entry); `evaluation/miniflux/internal/api/entry_handlers.go:401` (POST entry import).
- **Why useful (rubric scoring):**
  - Compute envelope: yes — recursive HTML parse + walk proportional to entry size; ~673 LOC of allowlist logic; non-trivial constant. Runs on essentially every entry produced or modified.
  - Load profile: yes — bursty, riding M-1/M-2's batches; also fired on the import-entry API path during OPML import or external-client backfills.
  - Coherent unit: yes — pure function: `(baseURL, rawHTML, options) → sanitizedHTML`. No I/O, no DB, no globals beyond static allowlist tables. Trivial RPC contract.
  - State independence: yes — pure. Allowlist maps are package-level constants; `config.Opts.YouTubeEmbedDomain()` and `InvidiousInstance()` are stable config reads.
  - Latency / failure: yes — caller is the refresh worker (async) or an HTTP handler that already does DB work. Sanitizer failure already returns `""` silently.
- **Activation shape:** ordinary function call from refresh pipeline and from a handful of API handlers.
- **Confidence:** high — pure CPU-bound function with a clean signature is the textbook lift candidate.
- **Risk notes:** very small — the lifted replica needs the same `config.Opts` snapshot for embed-domain decisions, but that's config not state. Per-entry granularity could be argued too fine; for crawler-enabled feeds the per-entry payload is large enough that the CPU envelope dominates.

### M-4: `integration.PushEntries` — third-party fan-out for new entries

- **pick_provenance:** claude+codex+gemini (3/3)
- **critique_status:** KEEP from all three; MODIFY from claude and from codex on gemini (caller cite `:292`→`:349`).
- **Region root:** `evaluation/miniflux/internal/integration/integration.go:511` — `PushEntries(feed, entries, userIntegrations)`. Branches on each enabled integration flag (Matrix, Webhook, Ntfy, Apprise, Discord, Slack, Pushover, Telegram, Readeck) and dispatches outbound HTTP/SMTP.
- **Caller(s):** `evaluation/miniflux/internal/reader/handler/handler.go:349` — `go integration.PushEntries(originalFeed, newEntries, userIntegrations)` after a successful refresh produces new entries.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — IO-dominated outbound HTTP fan-out. Per-integration JSON marshal + outbound POST + retry semantics. Cost scales with both number of enabled integrations per user and `len(entries)`.
  - Load profile: yes — bursty fan-out triggered by every successful refresh that produced new entries; spikes when a popular feed publishes a batch.
  - Coherent unit: yes — pure-value inputs (`*Feed`, `[]Entry`, `*Integration`); no return value. Each integration has its own client constructor.
  - State independence: yes — strictly outbound; no in-process state mutated.
  - Latency / failure: yes — already invoked as fire-and-forget goroutine; failures are logged. Perfectly natural offload.
- **Activation shape:** goroutine launched from the refresh handler.
- **Confidence:** high — already async, already bounded I/O, naturally batched. The metric oracle can decide based on enabled-integrations × entry count.
- **Risk notes:** drags in ~25 integration sub-packages (~30 KLOC of HTTP clients). Could be sliced narrower per-integration if dependency closure becomes a problem.

### M-5: `iconChecker.UpdateOrCreateFeedIcon` — favicon discovery, decode, resize, store

- **pick_provenance:** claude+codex+gemini (3/3)
- **critique_status:** KEEP from all three; MODIFY from claude and from codex on gemini (caller cites corrected to `:110`, `:201`, `:358`).
- **Region root:** `evaluation/miniflux/internal/reader/icon/checker.go:28` — builds an HTTP requestBuilder, runs `iconFinder.findIcon` (`internal/reader/icon/finder.go:49`) which scrapes `<link rel="icon">` candidates and downloads icons, then `resizeIcon` (`finder.go:186`) which decodes JPEG/PNG/GIF/WebP, runs bilinear resize via `golang.org/x/image/draw`, re-encodes as PNG, or minifies SVG.
- **Caller(s):** `evaluation/miniflux/internal/reader/handler/handler.go:110` (CreateFeedFromSubscriptionDiscovery), `:201` (CreateFeed), `:358` (RefreshFeed under `forceRefresh`).
- **Why useful (rubric scoring):**
  - Compute envelope: yes — image decode + bilinear scale + PNG re-encode is real CPU work; SVG minification is meaningful too. Plus N outbound HTTP calls per discovery.
  - Load profile: maybe — runs at most once per feed creation and on forced refreshes; `CreateFeedIconIfMissing` short-circuits when an icon exists. Bursty during OPML import; quiet in steady state.
  - Coherent unit: yes — small struct (`store`, `feed`) and a no-arg method.
  - State independence: yes — output written via `store.StoreFeedIcon`; no shared in-process mutable state.
  - Latency / failure: yes — never on a tight critical path; failure is logged and the feed proceeds without an icon.
- **Activation shape:** function called from the feed-refresh / feed-create paths.
- **Confidence:** medium — clear CPU envelope on resize, but call frequency is much lower than M-1..M-4.
- **Risk notes:** depends on `c4.image` (webp), `golang.org/x/image/draw`, `tdewolff/minify` — heavy but self-contained.

### M-6: `ParseFeed` — feed-format dispatch and normalization

- **pick_provenance:** codex+gemini (2/3)
- **critique_status:** KEEP from claude on codex (claude acknowledged this should have been promoted: "codex is right that ParseFeed has the cleanest signature"); MODIFY from claude and from codex on gemini (caller cites `:61`/`:254` → actual `:55`, `:157`, `:297`).
- **Region root:** `evaluation/miniflux/internal/reader/parser/parser.go:20` — `ParseFeed` detects feed format (Atom/RSS/JSON Feed/RDF) and dispatches to the appropriate parser, returning a normalized `*model.Feed`.
- **Caller(s):** `evaluation/miniflux/internal/reader/handler/handler.go:55` (CreateFeedFromSubscriptionDiscovery), `:157` (CreateFeed), `:297` (RefreshFeed).
- **Why useful (rubric scoring):**
  - Compute envelope: yes — XML/JSON parse + per-entry model normalization, payload-proportional CPU.
  - Load profile: yes — every scheduled refresh and feed-create exercises it; payload sizes vary sharply across tenants/feeds.
  - Coherent unit: yes — `(baseURL, io.ReadSeeker) → (*model.Feed, error)` is the cleanest signature in the refresh path.
  - State independence: yes — pure per-call; parser selection and adapters are stateless.
  - Latency / failure: yes — already a distinct phase inside the refresh path; parse errors have localized handling upstream.
- **Activation shape:** parser stage inside refresh/create.
- **Confidence:** high — ranks below M-1/M-2 only because it's a sub-stage; rubric scoring is one of the cleanest in the corpus.
- **Risk notes:** the `io.ReadSeeker` input would need a byte-slice wrapper for remote serialization; lifting only parsing leaves fetch and DB work local.

### M-7: `ScrapeWebsite` — single-URL scrape + Readability extraction

- **pick_provenance:** claude+gemini (2/3)
- **critique_status:** KEEP from codex on claude; MODIFY from claude and from codex on gemini (caller cites corrected to `processor.go:111`, `:195`).
- **Region root:** `evaluation/miniflux/internal/reader/scraper/scraper.go:21` — `ScrapeWebsite(requestBuilder, pageURL, rules)`. Outbound HTTP, content-type check, charset decode, then either custom CSS-rule extraction (goquery) or Mozilla-style Readability (`readability.ExtractContent`).
- **Caller(s):** `evaluation/miniflux/internal/reader/processor/processor.go:111` (per-entry inside `ProcessFeedEntries`); `evaluation/miniflux/internal/reader/processor/processor.go:195` (inside `ProcessEntryWebPage`, fired by `internal/api/entry_handlers.go:486` `fetchContentHandler` and `internal/ui/entry_scraper.go:54` `fetchContent`).
- **Why useful (rubric scoring):**
  - Compute envelope: yes — outbound HTTP, then 389-LOC Readability pass. The maintainers' own `BenchmarkExtractContent` (`readability_test.go:18`) is itself evidence of hotness.
  - Load profile: yes — bursty: amplifies with crawler-enabled refreshes plus user "fetch original content" clicks.
  - Coherent unit: yes — `(requestBuilder, pageURL, rules) → (baseURL, content, err)`. One of the cleanest signatures in the codebase.
  - State independence: yes — pure-ish: takes a configured `requestBuilder`, produces strings. Only external coupling is the static `predefinedRules` map (`internal/reader/scraper/rules.go`).
  - Latency / failure: yes — caller in the refresh path is async; user-facing `fetchContent` is an explicit "give me the article" click where multi-second latency is already expected.
- **Activation shape:** function call from feed-refresh loop and from two HTTP handlers.
- **Confidence:** high — narrow contract, clearly CPU-bound in the readability path, repeatedly invoked under load.
- **Risk notes:** very few; the predefined-rules map is the only external coupling and is read-only static data.

### M-8: `readability.ExtractContent` — pure-CPU readability pass

- **pick_provenance:** codex+gemini (2/3)
- **critique_status:** KEEP from claude on codex; MODIFY from claude and from codex on gemini (caller cite `scraper.go:56` → actual `:61`).
- **Region root:** `evaluation/miniflux/internal/reader/readability/readability.go:73` — `ExtractContent(io.Reader)`. Parses an HTML page, removes unlikely nodes, scores candidates across `section,h2..h6,p,td,pre,div`, and emits the selected article fragment.
- **Caller(s):** `evaluation/miniflux/internal/reader/scraper/scraper.go:61` (inside `ScrapeWebsite` when no custom rule applies).
- **Why useful (rubric scoring):**
  - Compute envelope: yes — recursive DOM scoring across goquery; the strongest pure-CPU candidate in the tree alongside `SanitizeHTML`.
  - Load profile: yes — proportional to page complexity; rides every crawler-enabled refresh and every manual content fetch.
  - Coherent unit: yes — `(io.Reader) → (baseURL, content, err)`. Zero state.
  - State independence: yes — completely self-contained; no store, no globals, no config.
  - Latency / failure: maybe — usually nested inside a network-heavy scrape so an extra hop is in the noise; lifting it solo would not offload the upstream fetch.
- **Activation shape:** inner CPU stage inside the scraper.
- **Confidence:** high — cleanest pure-compute contract in miniflux.
- **Risk notes:** if lifted independently (rather than alongside M-7), the calling site still pays for the outbound HTTP fetch locally; pair with M-7 for full benefit.

### M-9: `integration.SendEntry` — per-entry "save to bookmarking service" fan-out

- **pick_provenance:** claude+codex (2/3)
- **critique_status:** KEEP from gemini on both.
- **Region root:** `evaluation/miniflux/internal/integration/integration.go:41` — `SendEntry(entry, userIntegrations)`. ~470-line `if userIntegrations.XEnabled { client.Save(...) }` cascade across ~20 services (Pinboard, Wallabag, Notion, Linkding, Linkwarden, Readeck, Readwise, Cubox, Shaarli, Archive.org, Webhook, Omnivore, Karakeep, Raindrop, ...).
- **Caller(s):** `evaluation/miniflux/internal/api/entry_handlers.go:264` (`go integration.SendEntry(entry, settings)`); `evaluation/miniflux/internal/ui/entry_save.go:36`; `evaluation/miniflux/internal/fever/handler.go:461`; `evaluation/miniflux/internal/googlereader/handler.go:315`. All four sites use `go ...`.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — outbound HTTP per enabled integration; payloads include full entry content for several services (Wallabag, NunuxKeeper, Linktaco, Readeck, Linkwarden).
  - Load profile: yes — bursty on user "save" clicks; a power user save-storming a backlog drives a spike.
  - Coherent unit: yes — clean `(*Entry, *Integration)` signature, no shared state.
  - State independence: yes — strictly outbound HTTP.
  - Latency / failure: yes — every caller is `go ...`; the API responds `JSONAccepted` immediately. Failure-tolerance is built into the calling contract.
- **Activation shape:** goroutine launched from each of four HTTP handlers.
- **Confidence:** high — same shape as M-4 but on the user-driven side; four framework dialects (REST, web UI, Fever, Google Reader) converge here.
- **Risk notes:** same dependency-closure concern as M-4; lack of durable queue/retry means remote failure handling would need policy. Could be sliced per-integration if needed.

### M-10: `subscriptionFinder.FindSubscriptions` — feed discovery from website URL

- **pick_provenance:** claude+codex (2/3)
- **critique_status:** KEEP from gemini on both.
- **Region root:** `evaluation/miniflux/internal/reader/subscription/finder.go:44` — fetches the website, detects whether it's already a feed, parses HTML, walks `<link rel=alternate>` meta tags, runs YouTube-page heuristics, falls back to RSSBridge probing, and may try a curated list of well-known feed paths.
- **Caller(s):** `evaluation/miniflux/internal/api/subscription_handlers.go:52` (REST `POST /v1/discover`); `evaluation/miniflux/internal/ui/subscription_submit.go:72` (browser "subscribe" form); `evaluation/miniflux/internal/googlereader/handler.go:357`.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — at minimum one outbound HTTP + HTML parse; in many cases follow-up probes (YouTube channel ID lookup, well-known feed paths, RSSBridge query). Mixed CPU+IO; bounded but non-trivial.
  - Load profile: maybe — bursty around campaign launches and OPML imports; otherwise spaced-out user actions. Worst case is high; median may be modest.
  - Coherent unit: maybe — `(websiteURL, rssBridgeURL, rssBridgeToken) → (Subscriptions, error)`, but the `feedDownloaded` / `FeedResponseInfo()` side-channel state on the finder struct must be round-tripped if the caller consumes it post-call.
  - State independence: yes — mutates only its own struct fields; no globals.
  - Latency / failure: yes — invoked from the user-facing "Add subscription" flow which already shows a spinner; failure is a localized error message.
- **Activation shape:** synchronous handler call from three HTTP entry points.
- **Confidence:** medium — strong payload/network envelope; weaker load evidence and the `FeedResponseInfo()` post-call channel needs design.
- **Risk notes:** depends on `integration/rssbridge` for the optional fallback; remote activation must preserve more than just the returned subscription list.

### M-11: `mediaproxy.RewriteDocumentWithAbsoluteProxyURL` — HTML rewrite for media-proxy URLs

- **pick_provenance:** claude+codex (2/3) — *disputed: latency tension*
- **critique_status:** KEEP from gemini on both; both claude and codex flag the per-entry call inside `findEntries` as a request-path latency tension. Both kept it as a marginal/oracle-gated candidate rather than a positive recommendation.
- **Region root:** `evaluation/miniflux/internal/mediaproxy/rewriter.go:23` — `RewriteDocumentWithAbsoluteProxyURL`. Parses entry HTML with goquery and rewrites `<img>`/`<picture>`/`<audio>`/`<video>` `src`/`srcset`/`poster` URLs to point at the local media-proxy endpoint.
- **Caller(s):** `evaluation/miniflux/internal/api/entry_handlers.go:39` (`getEntryFromBuilder`, single-entry GET); `:191` (`findEntries`, in a loop over up to 100 entries per page); `:499` (`fetchContentHandler`); `evaluation/miniflux/internal/googlereader/handler.go:694`; `evaluation/miniflux/internal/fever/handler.go:319`.
- **Why useful (rubric scoring):**
  - Compute envelope: maybe → yes — goquery parse-and-walk per entry; trivial for short text, real for long articles, amortized over batch reads in `findEntries`.
  - Load profile: yes — every list-entries call rewrites every entry; aggressive mobile clients (Fever / Google Reader API) drive uniform-but-substantial load.
  - Coherent unit: yes — pure `string → string`.
  - State independence: yes — pure; depends only on stable media-proxy configuration.
  - Latency / failure: maybe → no at per-entry granularity — on the request critical path of a list-entries response that mobile clients expect to feel snappy. An extra hop per entry × 100 entries would regress unless batched.
- **Activation shape:** function call inside read-path HTTP handlers.
- **Confidence:** medium-low — keep as a documented oracle-gated pick, not a positive recommendation. A batched page-level invocation would be the form worth lifting; per-entry would lose. When `mediaProxyMode=none` the function exits cheaply, so the metric oracle would also need to skip no-ops.
- **Risk notes:** request-path latency tension at `entry_handlers.go:191` is the main rubric-negative pressure.

### M-12: `OPML Handler.Import` — bulk feed import

- **pick_provenance:** claude+codex (2/3)
- **critique_status:** KEEP from gemini on both.
- **Region root:** `evaluation/miniflux/internal/reader/opml/handler.go:41` — `Import(userID, io.Reader)`. Parses an OPML file, then for each `<outline>` looks up or creates the category and inserts the feed (no immediate fetch; refresh deferred to the worker pool).
- **Caller(s):** `evaluation/miniflux/internal/api/opml_handlers.go:27` (REST `POST /v1/import`); `evaluation/miniflux/internal/ui/opml_upload.go:58`.
- **Why useful (rubric scoring):**
  - Compute envelope: maybe — XML parse cost scales with the import file (100s–1000s of subscriptions for some users); each subscription triggers a category lookup + a `CreateFeed` insert, so DB round-trips may dominate over CPU.
  - Load profile: maybe — uniformly low traffic in steady state; spiky on initial onboarding. Borderline against the rubric's "uniformly low-traffic" disqualifier.
  - Coherent unit: yes — `(userID int64, data io.Reader) → error`.
  - State independence: yes — DB-only side-effects.
  - Latency / failure: yes — already a slow user action; the upload page expects multi-second wait.
- **Activation shape:** synchronous handler call from the import endpoint.
- **Confidence:** medium-low — useful as a "rare but heavy" specimen; less compelling than M-1..M-9. The real "expense" of OPML import surfaces later in the worker pool's consumption of the freshly inserted rows (i.e. amplifies M-1).
- **Risk notes:** DB round-trips may dominate the envelope; lift would primarily offload XML parse, not the durable inserts.

### M-13: `ProcessEntryWebPage` — user-pull "fetch full article" entrypoint

- **pick_provenance:** codex (1/3) — *weak consensus*
- **critique_status:** KEEP from claude on codex (claude noted codex's framing is "arguably crisper" than claude's own ScrapeWebsite pick because it identifies the named function that both API and UI handlers enter at). MODIFY from gemini on codex suggesting `ScrapeWebsite` is the cleaner inner target — but that recommendation already lands as M-7, so M-13 is admitted alongside it as the orchestrator that user-facing handlers actually call.
- **Region root:** `evaluation/miniflux/internal/reader/processor/processor.go:180` — `ProcessEntryWebPage(feed, entry, user)`. Fetches the entry URL, calls `scraper.ScrapeWebsite`, applies rewrite rules, sanitizes, and updates reading time.
- **Caller(s):** `evaluation/miniflux/internal/api/entry_handlers.go:486` (`fetchContentHandler`, REST endpoint); `evaluation/miniflux/internal/ui/entry_scraper.go:54` (`fetchContent`, browser action).
- **Why useful (rubric scoring):**
  - Compute envelope: yes — combines outbound HTTP, HTML extraction, minification, rewrite rules, sanitization, and reading-time estimation.
  - Load profile: maybe — user-triggered per-entry fetches are uneven and payload-dependent; not a steady background hot path.
  - Coherent unit: yes — takes feed/entry/user models, returns an error, leaves persistence to the caller. Mutates the passed `*Entry` (content, title, reading-time).
  - State independence: maybe — mostly per-call data plus stable config/proxy metrics; entry mutation is an in/out semantic.
  - Latency / failure: maybe — synchronous API/UI path, but already dominated by remote page fetch and HTML processing.
- **Activation shape:** HTTP route handler on-demand article extraction.
- **Confidence:** medium — distinct from M-7 in that it represents the user-pull *entry* point (not the inner CPU stage); useful as an "interface-method on a request handler" specimen.
- **Risk notes:** remote version must preserve mutated entry fields; overlaps M-7 as inner work, so picking both means dispatch granularity becomes a metric-oracle question.

### M-14: `oauth2.googleProvider.Profile` / `oidcProvider.Profile` — token exchange + profile fetch

- **pick_provenance:** claude (1/3) — *weak consensus*
- **critique_status:** KEEP from codex on claude (rubric criterion satisfied: coherent unit, matches the OAuth calibration example); KEEP from gemini on claude (rubric criterion satisfied: load profile).
- **Region root:** `evaluation/miniflux/internal/oauth2/google.go:57` (`(*googleProvider).Profile`) and `evaluation/miniflux/internal/oauth2/oidc.go:64` (`(*oidcProvider).Profile`). Each does a token-exchange POST, a GET of the user-info endpoint, JSON decode, and (OIDC) ID-token signature verification.
- **Caller(s):** `evaluation/miniflux/internal/ui/oauth2_callback.go:56` — `authProvider.Profile(r.Context(), code, codeVerifier)`.
- **Why useful (rubric scoring):**
  - Compute envelope: maybe — two outbound HTTPS round-trips + JSON decode + (OIDC) JWT verify. Latency-dominated but nontrivial.
  - Load profile: yes — calibration example in the rubric explicitly names "OAuth callback flurries during a campaign launch" as a target.
  - Coherent unit: yes — `Profile(ctx, code, codeVerifier) → (*UserProfile, error)` behind the `Provider` interface (`internal/oauth2/provider.go`) — exactly the kind of seam Monolift annotates.
  - State independence: yes — provider struct is stable config; no shared mutable state.
  - Latency / failure: maybe — caller is on the OAuth callback path, but that path is already user-visible IO-bound (two outbound HTTPS calls), so an extra hop is in the noise.
- **Activation shape:** interface-method call from a single HTTP handler.
- **Confidence:** medium — matches the rubric's calibration shape exactly, but per-call cost is moderate and traffic at typical installs is low. Useful as a "small-but-clean interface-method" specimen.
- **Risk notes:** OIDC needs the `*oidc.Provider` discovery doc populated at construction; the lifted replica must initialize once.

### M-15: `CreateFeed` — first-time feed lifecycle on the interactive path

- **pick_provenance:** gemini (1/3) — *disputed, weak consensus*
- **critique_status:** DROP from claude on gemini (redundant with the inner M-2/M-5/M-6 picks already in the merged set; caller cites were "hypothesized" rather than verified, violating the rubric's no-uncited-candidates rule). MODIFY from codex on gemini ("real but overconfident and mis-cited"; downgrade confidence). The aggregator (claude) admits this as a weak-consensus pick under rule 4 because codex's MODIFY does not constitute a DROP. Aggregator judgment: include with the redundancy concern explicit; the unique signal CreateFeed adds over its constituents is the synchronous-request-path framing.
- **Region root:** `evaluation/miniflux/internal/reader/handler/handler.go:116` — `CreateFeed(...)` coordinates the first-time lifecycle of a feed: outbound fetch, `ParseFeed`, `ProcessFeedEntries`, icon discovery, persistence.
- **Caller(s):** `evaluation/miniflux/internal/api/feed_handlers.go:45`, `evaluation/miniflux/internal/ui/subscription_submit.go:126`, `evaluation/miniflux/internal/ui/subscription_choose.go:47`, `evaluation/miniflux/internal/googlereader/handler.go:430` (corrected per codex's critique; gemini's original `internal/api/feed.go` and `internal/ui/feed_create.go` cites are not real paths).
- **Why useful (rubric scoring):**
  - Compute envelope: yes — heavy first-time fetch + parse + full entry processing.
  - Load profile: maybe — bursty around onboarding (and amplified by OPML import which inserts rows but defers refresh).
  - Coherent unit: yes — well-defined request/response.
  - State independence: maybe — same `*storage.Storage` and config dependencies as M-1.
  - Latency / failure: yes — users expect a delay when adding a new feed; the synchronous path already shows a spinner.
- **Activation shape:** synchronous HTTP request-response.
- **Confidence:** low — mostly redundant with M-1/M-2/M-5/M-6; the only differentiator is the synchronous request-path activation shape (vs. M-1's worker activation), which makes it a useful specimen but not a strong addition to the corpus.
- **Risk notes:** lifting M-15 re-lifts M-2/M-5/M-6 by composition; experiments wanting fine-grained isolation should pick the inner regions and skip M-15.

---

## Discrepancies

**M-11 (mediaproxy rewriter).** Both claude and codex picked it but flagged the per-entry call inside `findEntries` (entry_handlers.go:191) as a request-path latency tension; gemini KEPT both picks without engaging the latency concern. The aggregator sided with claude/codex's framing: the function is a clean string-to-string transform but the per-entry granularity violates the rubric's "tight synchronous request path with strict p99 budget" negative. Included as marginal/oracle-gated rather than as a strong pick. Justification grounded in rubric criterion 5 (latency tolerance).

**M-13 (ProcessEntryWebPage) vs M-7 (ScrapeWebsite).** Codex picked the orchestrator at processor.go:180; claude/gemini picked the inner `ScrapeWebsite`. Gemini's MODIFY of codex's pick recommended replacing it with ScrapeWebsite. The aggregator admits both: ScrapeWebsite (M-7) is the cleaner pure-CPU+IO inner stage, while ProcessEntryWebPage (M-13) is the named function that user-facing handlers actually call — picking different granularities is exactly what the metric oracle is meant to choose between, and it's defensible to expose both as candidates.

**M-15 (CreateFeed).** Claude DROPped (redundancy with inner picks); codex MODIFIED (kept the candidate but downgraded confidence and corrected cites). Aggregator (claude) sided with codex's "real but downgrade" reading on procedural grounds (a MODIFY is not a DROP, so rule 4 admits the candidate as weak consensus) but agrees with the redundancy concern; M-15 is ranked last and flagged explicitly as composition-redundant.

## Excluded candidates

- **claude C-12 (`mediaProxy` HTTP handler at `internal/ui/proxy.go:27`)** — DROP from codex (fails compute envelope and latency / failure: streaming reverse-proxy whose primary effect is forwarding origin response body inline to a browser image fetch; an extra hop worsens first-byte). DROP from gemini (latency tolerance — synchronous browser resource request expecting fast first-byte). Claude's own draft already downgraded it explicitly as a documented near-miss. Excluded under rule 5.
- **gemini C-10 (`EstimateReadingTime` at `internal/reader/readingtime/readingtime.go:17`)** — DROP from claude (fails compute envelope; ~25 LOC StripTags + Fields-count, trivial constant; already runs inside the same loop as the dominant SanitizeHTML/ScrapeWebsite). DROP from codex (fails compute envelope at useful remote granularity; structurally worse than ProcessFeedEntries/ProcessEntryWebPage/SanitizeHTML). Excluded under rule 5.
