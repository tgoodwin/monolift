# listmonk — merged lift-region candidate set (Phase 2b final)

## Methodology note

Phase 1 drafts: claude, codex, gemini.
Phase 2a critics: each model reviewed the other two.
Phase 2b aggregator: claude (opus).

Inclusion rules from PHASE2-PLAN.md §"Inclusion rules" applied deterministically.
MODIFY corrections from critics (line cite drift, scope narrowing) folded into the
merged entries below before ranking.

## Merged candidates (ranked strongest → weakest)

### M-1: Per-recipient campaign message render

- **pick_provenance:** claude+codex+gemini (3/3)
- **critique_status:** KEEP from all 3 critics — the canonical hot loop, no disputes.
- **Region root:** `evaluation/listmonk/internal/manager/message.go:13` —
  `(*Manager).NewCampaignMessage(c *models.Campaign, s models.Subscriber)` with its
  inner `(*CampaignMessage).render()` at `internal/manager/message.go:33`. Executes
  the precompiled `Campaign.Tpl` (base + content), `SubjectTpl`, and optional
  `AltBodyTpl` against the (campaign, subscriber) pair, producing the final HTML,
  subject, and plaintext body.
- **Caller(s):** `internal/manager/pipe.go:173` — `(*pipe).newMessage`, invoked
  once per subscriber inside the campaign send loop. Secondary callers on the
  request path: `cmd/campaigns.go:183` (`PreviewCampaign`), `cmd/archive.go:166`
  (`CampaignArchivePage`), `cmd/public.go:185` (`ViewCampaignMessage`).
- **Why useful (rubric scoring):**
  - Compute envelope: yes — `html/template` + `text/template` execute over
    user-supplied campaign body that may include sprig helpers, link-tracking
    rewrites, view-pixel injection; cost paid per recipient.
  - Load profile: yes — a single campaign launch fans this out to N subscribers
    (millions in production deployments) within a short window.
  - Coherent unit: yes — input is `*Campaign` (precompiled template, config,
    attachments) plus value-typed `Subscriber`; output is rendered byte slices
    on the message struct. No `*App` reachable.
  - State independence: yes — does not mutate package globals; the only shared
    mutable state is `Manager.links` (tracked-link UUID memo), which can be made
    replica-local without correctness loss because every link is durably
    registered in the DB via `store.CreateLink`.
  - Latency / failure: yes — caller is the campaign worker goroutine, off the
    request path; preview/archive callers tolerate hundreds of ms.
- **Activation shape:** Method on `Manager` invoked from a worker goroutine
  draining a buffered channel; secondary callers are HTTP handlers.
- **Confidence:** high — the canonical "expensive thing under load" in this binary.
- **Risk notes:** the compiled `*template.Template` lives on `Campaign.Tpl` and
  is shared across recipients; lifting requires shipping the source body and
  recompiling once per replica per campaign (Go templates aren't directly
  serializable). Tracking-link registration via `TrackLink` calls back into
  manager/store state on link-cache miss; either pre-register links or let the
  remote call back to the home node.

---

### M-2: HTTP webhook delivery (`Postback.Push`)

- **pick_provenance:** claude+codex+gemini (3/3)
- **critique_status:** KEEP from all 3 critics — cleanest of the three messenger
  paths on rubric criterion (4) state independence.
- **Region root:** `evaluation/listmonk/internal/messenger/postback/postback.go:97` —
  `(*Postback).Push(m models.Message) error`. Builds a `postback` struct,
  marshals to JSON via easyjson, POSTs to a configured remote URL with optional
  Basic auth and retries.
- **Caller(s):** `internal/manager/manager.go:523` — the campaign worker
  dispatch loop, when `msg.Campaign.Messenger` resolves to a postback messenger.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — JSON marshal of full message (attachments,
    recipient, campaign metadata) plus outbound HTTPS POST. Aggregates well;
    latency-dominated.
  - Load profile: yes — campaign burstiness; users wire postback messengers to
    internal APIs / Mailgun / Sendgrid HTTP APIs and drive them at MessageRate.
  - Coherent unit: yes — input is `models.Message`, output is an error;
    `*Postback` holds an `*http.Client`, an auth string, and an `Options`
    config struct.
  - State independence: yes — `http.Client` is connection-pooled but
    replica-local; nothing is mutated globally.
  - Latency / failure: yes — async caller (worker), errors flow through
    `pipe.OnError`.
- **Activation shape:** Interface-method (`Messenger.Push`) called from a
  worker loop draining `campMsgQ` / `msgQ`.
- **Confidence:** high — sibling of M-3 with HTTP transport instead of SMTP.
- **Risk notes:** attachment byte copy on every call (`postback.go:128`) is
  wasteful per-call but lift-friendly (no aliasing back to the caller).

---

### M-3: SMTP message send (`Emailer.Push`)

- **pick_provenance:** claude+codex+gemini (3/3)
- **critique_status:** KEEP from all 3 critics — same shape as M-2; state
  independence scored "maybe" rather than "yes" because of the long-lived
  SMTP/TLS connection pool.
- **Region root:** `evaluation/listmonk/internal/messenger/email/email.go:111` —
  `(*Emailer).Push(m models.Message) error`. Picks an SMTP server from the pool,
  builds the `smtppool.Email` (headers, attachments, body, alt-body), resolves
  Return-Path / Bcc / Cc envelope semantics, and calls `srv.pool.Send(em)` which
  performs the actual TCP+TLS+SMTP dialog.
- **Caller(s):** `internal/manager/manager.go:523` — `worker()` calls
  `m.messengers[msg.Campaign.Messenger].Push(out)` for every campaign message.
  Also `cmd/public.go:629` (`SelfExportSubscriberData`) and the `(*App).sendNotif`
  path. (Gemini's `:527` cite drifted ~4 lines; corrected to `:523`.)
- **Why useful (rubric scoring):**
  - Compute envelope: yes — outbound SMTP round-trip per message, dominated by
    network latency to the relay, plus per-call attachment copying and header
    serialization. Aggregates well across many recipients.
  - Load profile: yes — one campaign sustains this at `MessageRate`/sec for
    minutes-to-hours; tx and notification messages add a smaller bursty stream.
  - Coherent unit: yes — input is value-typed `models.Message`; output is an
    error. `*Emailer` carries config but holds no per-message mutable state.
  - State independence: maybe — `srv.pool` (smtppool) is a long-lived TCP/TLS
    connection pool; a remote replica can hold its own pool. Not OK if the lift
    insists on shared pools.
  - Latency / failure: yes — caller is the worker goroutine; failure path is
    an `error` that increments `pipe.OnError()` with bounded retry semantics.
- **Activation shape:** Interface-method (`Messenger.Push`) called from a
  worker loop draining `campMsgQ` / `msgQ`.
- **Confidence:** high — IO-bound, embarrassingly parallel, the natural
  scale-out shape for any mailer.
- **Risk notes:** the `smtppool` keeps long-lived auth'd TCP connections;
  short-lived remote replicas lose reconnect amortization.

---

### M-4: Image thumbnail generation (`processImage`)

- **pick_provenance:** claude+codex+gemini (3/3)
- **critique_status:** KEEP from all 3 critics; gemini's caller cite drifted
  (`cmd/media.go:49` → corrected to `:99` per claude's and codex's MODIFY).
- **Region root:** `evaluation/listmonk/cmd/media.go:212` —
  `processImage(file *multipart.FileHeader) (*bytes.Reader, int, int, error)`.
  Decodes an uploaded image with `imaging.Decode`, runs Lanczos resize to
  250px width, encodes as PNG, and returns thumbnail bytes plus original W/H.
- **Caller(s):** `cmd/media.go:99` — `(*App).UploadMedia`, the only caller,
  invoked synchronously inside the `POST /api/media` handler. (Verified at
  `:99`.)
- **Why useful (rubric scoring):**
  - Compute envelope: yes — full decode of GIF/PNG/JPEG, Lanczos resampling
    (CPU-bound, scales with image area), and PNG re-encode. The canonical
    "image processing on user upload" example from the rubric.
  - Load profile: yes — bursty per-tenant on campaign authoring days.
  - Coherent unit: maybe — input is `*multipart.FileHeader` holding an open
    file handle; the lift-friendly shape is `(reader []byte, contentType string)
    -> (thumb []byte, w, h int, err error)`. Thin refactor.
  - State independence: yes — pure function over input bytes; no globals, no DB.
  - Latency / failure: yes — caller is on the request path but already pays
    decode + resize + encode locally; an extra hop is in the noise next to
    Lanczos on a multi-megabyte image.
- **Activation shape:** Synchronous helper invoked from an HTTP route handler
  (`POST /api/media`).
- **Confidence:** high — textbook CPU-bound, side-effect-free, naturally
  parallel.
- **Risk notes:** the larger handler (`UploadMedia`) also calls `a.media.Put`
  to push to the configured store (filesystem or S3); that side-effect stays
  with the caller. If lifted, the caller hands a byte blob over and gets a
  thumbnail blob back.

---

### M-5: Bulk subscriber CSV ingest (`Session.LoadCSV`)

- **pick_provenance:** claude+codex+gemini (3/3)
- **critique_status:** KEEP from all 3 critics — coherent-unit "maybe" scored
  consistently across drafts due to the temp-filepath input.
- **Region root:** `evaluation/listmonk/internal/subimporter/importer.go:452` —
  `(*Session).LoadCSV(srcPath string, delim rune) error`. Counts file lines,
  parses CSV header, then per-row: validates+sanitizes email (regex,
  domain blocklist/allowlist), builds a `SubReq`, JSON-decodes the
  `attributes` column, and pushes onto `subQueue`.
- **Caller(s):** `cmd/import.go:101,115` — `go sess.LoadCSV(...)` after the
  upload is staged to a temp file by `ImportSubscribers`. Always launched as
  a goroutine; the HTTP handler returns 200 immediately with the import stats.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — CSV parse + per-row JSON unmarshal +
    `mail.ParseAddress` + domain map lookups + Title-casing for missing names.
    Cost scales with file size and column count.
  - Load profile: yes — periodic, heavy, and bounded only by upload size; an
    operator dump of millions of rows lights this up for many minutes.
  - Coherent unit: maybe — input is a temp filepath (string) and a delimiter
    rune; the validation+parse loop itself is a pure function of bytes-in.
    The filepath is a soft disqualifier unless the upload bytes are streamed
    to the replica.
  - State independence: yes — only mutates session-local progress counter via
    `incrementImportCount`; domain block/allow-lists are read-only.
  - Latency / failure: yes — explicitly run as a background goroutine; HTTP
    caller returns immediately.
- **Activation shape:** Background goroutine kicked off by an HTTP handler
  after staging an upload to a temp file.
- **Confidence:** high if the input is the byte stream (CSV blob), medium if
  it must remain a filepath.
- **Risk notes:** the `Importer` enforces single-import-at-a-time via a
  package-level lock; replicating this across remote replicas requires
  externalizing the lock or accepting one replica per importer. Only `LoadCSV`
  (the parse phase) lifts cleanly; the sibling drain `Session.Start` is bound
  to the local prepared statement and a poor lift.

---

### M-6: SES/SNS bounce notification processing (`SES.ProcessBounce`)

- **pick_provenance:** claude+codex (2/3); gemini did not pick directly but
  KEEPed both claude's and codex's picks.
- **critique_status:** KEEP from all 3 critics. Codex's MODIFY on claude's
  draft (split sibling processors instead of treating them as one) is applied
  by promoting Sendgrid into its own merged entry (M-7).
- **Region root:** `evaluation/listmonk/internal/bounce/webhooks/ses.go:108` —
  `(*SES).ProcessBounce(b []byte) (models.Bounce, error)`. Unmarshals the SNS
  envelope, calls `verifyNotif` (which fetches and caches the SNS signing
  cert via HTTPS at `:232` and runs `cert.CheckSignature(SHA1WithRSA, ...)`),
  then unmarshals the inner SES payload, classifies bounce type, and walks
  message headers for the campaign UUID.
- **Caller(s):** `cmd/bounce.go:167` — `(*App).BounceWebhook` for
  `service == "ses"`.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — JSON unmarshal of envelope + nested payload, RSA
    signature verification (and a one-time-per-cert HTTPS fetch on cache
    miss), header walk.
  - Load profile: yes — bursty in lockstep with campaign sends (one notif per
    bounced recipient).
  - Coherent unit: yes — input is `[]byte` (raw POST body) and an optional
    signature string; output is `models.Bounce`. Receiver carries only a cert
    cache.
  - State independence: yes — cert cache is `map[string]*x509.Certificate`
    populated from a public URL; replica-local is fine.
  - Latency / failure: yes — caller is an HTTP webhook handler whose response
    is just an ack; SNS retry budget is forgiving (10s+); `bounce.Record(b)`
    is already async (buffered channel push at `cmd/bounce.go:243`).
- **Activation shape:** Interface-method-style processor invoked from an HTTP
  route handler that switches on the `:service` URL param.
- **Confidence:** high — matches the rubric's positive example "OAuth-token-
  exchange handler that does an outbound HTTPS round-trip plus signature
  verification."
- **Risk notes:** SES `getCert` hits an HTTPS URL on cache miss; in a remote
  lift each replica pays one such fetch per signing cert seen.

---

### M-7: Campaign template compilation (`Campaign.CompileTemplate`)

- **pick_provenance:** claude+codex (2/3); gemini picked the narrower
  `ConvertContent` (gemini C-2) instead.
- **critique_status:** Effectively KEEP from all 3 critics; gemini's `ConvertContent`
  was MODIFY'd by claude and DROPped by codex in favor of `CompileTemplate`,
  which subsumes the same Markdown work plus the load-bearing template parse.
- **Region root:** `evaluation/listmonk/models/campaigns.go:138` —
  `(*Campaign).CompileTemplate(f template.FuncMap) error`. Substitutes
  registered template-function regex aliases, parses the base template,
  conditionally runs `markdown.Convert` (goldmark) for Markdown campaigns,
  parses the content template, links the two trees via `AddParseTree`, and
  parses the alt-body template.
- **Caller(s):** `internal/manager/pipe.go:35` — `newPipe` calls it once at
  campaign send start. On the request path: `cmd/campaigns.go:176`
  (`PreviewCampaign`), `cmd/archive.go:247` (`compileArchiveCampaigns`),
  `cmd/public.go:178` (`ViewCampaignMessage`).
- **Why useful (rubric scoring):**
  - Compute envelope: yes — three template parses (base, content, alt) plus
    optional Markdown→HTML conversion via goldmark, all over user-supplied
    bodies that may include sprig functions and link-tracking markup.
  - Load profile: maybe — once per campaign at send time (cheap), but on
    preview/archive views can hit per request, and the public archive page
    is publicly cacheable, so traffic spikes on viral newsletters land here.
  - Coherent unit: yes — receiver `*Campaign` value, input `template.FuncMap`;
    output mutates `Tpl`, `SubjectTpl`, `AltBodyTpl` on the receiver. Cleanly
    re-shapes as `(body, altBody, subject string, ct string, funcs FuncMap) ->
    parsed trees`.
  - State independence: yes — modulo the funcmap closure capture of `*Manager`
    for `TrackLink`. No globals.
  - Latency / failure: maybe — campaign-send caller is the worker (off path);
    preview/archive callers are HTTP but admin-tolerant.
- **Activation shape:** Method-on-value, called once per campaign-start and
  again on each preview/archive page render.
- **Confidence:** medium — dominant cost compared to M-1 only when the body
  is large/Markdown-heavy. The Markdown-conversion branch is the strongest
  sub-region.
- **Risk notes:** `template.FuncMap` includes closures over `*Manager` for
  `TrackLink` (which writes new tracking URLs to the DB on cache miss). A
  remote lift either ships a stub funcmap and re-resolves links locally, or
  accepts that link registration calls back to the home node.

---

### M-8: POP3 bounce mailbox scan (`POP.Scan`)

- **pick_provenance:** claude+codex (2/3); gemini picked the narrower
  `classifyBounce` helper inside this region.
- **critique_status:** Effectively KEEP from all 3 critics; gemini's
  `classifyBounce` was MODIFY'd by claude and DROPped by codex in favor of
  the surrounding `POP.Scan`, which carries the heavier MIME-parse + multi-
  regex + POP-retrieval-and-deletion work.
- **Region root:** `evaluation/listmonk/internal/bounce/mailbox/pop.go:79` —
  `(*POP).Scan(limit int, ch chan models.Bounce) error`. Connects, auths,
  lists messages, and per message: retrieves raw bytes, parses MIME, walks
  multipart parts, runs header lookups + regex fallbacks, parses dates,
  classifies hard/soft via `classifyBounce`, pushes `models.Bounce` onto a
  channel, then deletes processed messages from the server.
- **Caller(s):** `internal/bounce/bounce.go:138` —
  `(*Manager).runMailboxScanner`, an infinite loop sleeping
  `m.opt.Mailbox.ScanInterval` between calls.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — MIME parse + multipart walk + multiple multi-line
    regex passes per message (`reSMTPStatus`, `reHardBounce`, plus seven
    `headerLookups` regexes), batched up to 1000 per scan.
  - Load profile: maybe — periodic with configurable interval, but each scan
    can be heavy if a campaign just went out.
  - Coherent unit: yes — input is (`limit int`, `chan models.Bounce`); the
    POP3 client is per-instance and re-instantiable remotely with the same
    `Opt`.
  - State independence: maybe — pushes onto a Go channel owned by the bounce
    `Manager`; remote-side this becomes "push to a queue or HTTP endpoint."
  - Latency / failure: yes — caller is a background goroutine sleeping
    between scans; failure logs and retries on the next tick.
- **Activation shape:** Long-lived background goroutine in an infinite poll
  loop.
- **Confidence:** medium — the per-call MIME+regex work is real, but lifting
  is most valuable if the channel sink is replaced with a durable queue.
- **Risk notes:** the function deletes messages from the POP server after
  pushing onto the channel; current code uses
  `select { case ch <- ...: default: }`, dropping silently on full channel —
  bounces are still deleted. Lifting must preserve that contract; ideally the
  durable queue ack happens before the POP3 `Dele`.

---

### M-9: Transactional message render (`TxMessage.Render`)

- **pick_provenance:** codex picked `TxMessage.Render` directly (1/3); claude
  picked the surrounding `SendTxMessage` HTTP handler. Aggregator applied
  codex's MODIFY of claude's pick (replace handler with the inner render
  leaf) — making this effectively a 2/3 region with both claude and codex
  agreeing on the leaf shape.
- **critique_status:** Both claude and codex MODIFY each other toward the
  inner render leaf; gemini KEEPs both. Resolved as the leaf.
- **Region root:** `evaluation/listmonk/models/messages.go:74` —
  `(*TxMessage).Render(sub models.Subscriber, tpl *template.Template,
  funcs template.FuncMap) error`. Executes the body template, optional
  alt-body template, and subject template against the subscriber, populating
  rendered fields on the receiver.
- **Caller(s):** `cmd/tx.go:132` — `m.Render(sub, tpl,
  a.manager.GenericTemplateFuncs())` invoked per-recipient inside the
  `SendTxMessage` handler loop. The surrounding handler stays on the home
  node.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — three template executes per recipient; cost grows
    with template complexity and recipient count.
  - Load profile: yes — application-driven and bursty (signup flurries,
    password-reset surges, OAuth callbacks); the rubric explicitly cites
    "OAuth callback flurries" as a positive example.
  - Coherent unit: yes — inputs are `sub`, `tpl`, `funcs`; output is the
    populated `TxMessage` rendered fields. Cleaner than picking the wider
    handler, which also reaches into `a.core` (DB), `a.importer.SanitizeEmail`,
    and `a.manager.PushMessage`.
  - State independence: yes — manager template cache (`m.tpls`) is read-only
    at this site; no global mutation.
  - Latency / failure: maybe — API caller is synchronous and waits for
    `c.JSON(...)`, so adding a hop costs the caller. But the call already
    does N DB lookups + N renders + N enqueues, so per-call latency is
    already O(N × tens of ms); an extra hop is in the noise.
- **Activation shape:** Render leaf invoked from the per-recipient loop in
  the HTTP route handler under `/api/tx`.
- **Confidence:** medium — strong "compute under variable load" candidate;
  the lift surface is narrow once the framing is fixed to "render only,
  surrounding loop stays on the home node."
- **Risk notes:** the handler reuses one `TxMessage` across recipients; a
  remote lift should return immutable rendered payloads to avoid
  cross-recipient mutation bleed.

---

### M-10: Sendgrid bounce batch processing (`Sendgrid.ProcessBounce`)

- **pick_provenance:** codex (1/3) — claude folded Sendgrid into C-7 as a
  variant rather than scoring it separately; gemini did not score it.
- **critique_status:** Weak consensus. KEEP from claude (explicitly endorsed
  codex's choice to score it independently as "more honest" given the
  meaningfully different compute mix); KEEP from gemini. Included per Rule 4.
- **Region root:** `evaluation/listmonk/internal/bounce/webhooks/sendgrid.go:53` —
  `(*Sendgrid).ProcessBounce(sig, ts string, b []byte) ([]models.Bounce, error)`.
  Base64/ASN.1 decodes the signature, hashes timestamp + body, verifies ECDSA
  P-256, and parses a multi-event JSON array into bounce records.
- **Caller(s):** `cmd/bounce.go:186` — `(*App).BounceWebhook` for
  `service == "sendgrid"`.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — base64/ASN.1 decode + SHA-256 hash + ECDSA
    verify + multi-event JSON parse over a potentially batched payload.
    Meaningfully different compute mix from SES (RSA + single-event).
  - Load profile: yes — Sendgrid explicitly batches multiple bounce events
    per webhook; volume follows campaign sends.
  - Coherent unit: yes — signature, timestamp, and body are explicit inputs;
    output is a slice of bounce records.
  - State independence: yes — uses only the configured public key and request
    bytes.
  - Latency / failure: yes — webhook request path with upstream retry.
- **Activation shape:** HTTP webhook handler helper.
- **Confidence:** medium — better batch shape than SES, but workload size
  depends on webhook batch size.
- **Risk notes:** signature verification must be done over the exact raw
  request bytes; any lift boundary must preserve the body byte-for-byte.
  Public key is stored on `*Sendgrid` post-`NewSendgrid`; lift must arrange
  for the same key on the remote side.

---

### M-11: Public campaign archive page render (`CampaignArchivePage`)

- **pick_provenance:** claude (1/3) — codex and gemini did not pick.
- **critique_status:** Disputed weak consensus. Codex DROPped (structurally
  worse than M-1 and M-7, which it already composes); gemini KEEPed (citing
  variable/spikable load from viral content). Aggregator includes per Rule 4
  (1 of 3 picked, KEEP from at least one critic) with a "disputed" note.
  Defense grounded in rubric criterion (2) load profile: a public,
  unauthenticated, CDN-cacheable endpoint exposed when `EnablePublicArchive`
  is true does carry the "newsletter-goes-viral" burst pattern, and the
  region composes M-1 and M-7 inside an HTTP request boundary that has
  different load characteristics than either alone.
- **Region root:** `evaluation/listmonk/cmd/archive.go:119` —
  `(*App).CampaignArchivePage(c echo.Context) error`. Looks up the campaign
  by UUID or slug, calls `compileArchiveCampaigns([]models.Campaign{pubCamp})`
  (which runs `CompileTemplate` + subject render against archive metadata at
  `archive.go:239`), then `manager.NewCampaignMessage(camp, sub)` to render
  the body, and writes the HTML response.
- **Caller(s):** `cmd/handlers.go:283` —
  `g.GET("/archive/:id", a.CampaignArchivePage)`, public unauthenticated
  endpoint.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — composes M-7 (compile + Markdown convert) and
    M-1 (full message render) per request.
  - Load profile: maybe — public URL with viral-spike potential; otherwise
    lukewarm.
  - Coherent unit: yes — input is URL params + DB-fetched campaign; output
    is rendered HTML; no writes.
  - State independence: yes — read-only path.
  - Latency / failure: yes — public-facing, no strict p99 budget; a few
    hundred ms is acceptable on a viral campaign.
- **Activation shape:** HTTP route handler, public.
- **Confidence:** low-medium — codex's objection (it composes other regions
  rather than introducing new compute) is fair; the case for keeping rests
  on the load-profile axis specific to this public endpoint.
- **Risk notes:** lifts best as part of a "render service" alongside M-1 and
  M-7, which share the same template machinery and link-tracking closure.

---

## Discrepancies

### CampaignArchivePage (M-11) — codex DROP vs gemini KEEP
Codex argued the region "wraps DB lookup and Echo response handling around
regions already represented by `Campaign.CompileTemplate` and
`Manager.NewCampaignMessage`" and is structurally redundant. Gemini argued
it captures variable/spikable load from viral content. The aggregator sided
with weak inclusion (M-11) on rubric criterion (2): the public endpoint has
a load profile distinct from the workers/admin paths that exercise M-1 and
M-7, and the rubric's "newsletter goes viral" pattern is the canonical
positive example for this exact shape. Confidence intentionally rated low-
medium to reflect the disagreement.

### Sendgrid splitting from SES (M-10) — codex split vs claude bundle
Claude's original C-7 folded Sendgrid into a single bounce-webhook entry
with SES; codex argued for separate scoring because the compute mix differs
materially (RSA+single-event vs ECDSA+ASN.1+multi-event). Claude's critique
endorsed codex's split as "at least as rigorous and arguably more honest";
the aggregator sided with the split. Other variants (Postmark, ForwardEmail,
Lettermint) were noted by claude as structurally identical but were not
individually scored by any draft and are not included here.

### TxMessage scope — `SendTxMessage` (handler) vs `TxMessage.Render` (leaf)
Claude picked the wider HTTP handler; codex picked the inner render leaf;
each MODIFY'd the other in the same direction (toward the leaf). Aggregator
collapsed to a single entry (M-9) at the leaf, with the handler's wider
collaborator surface explicitly excluded from the lift unit.

### `classifyBounce` (gemini C-6) vs `POP.Scan` (claude+codex)
Gemini picked the inner pure-functional regex helper; both other critics
MODIFY/DROP'd in favor of the surrounding `POP.Scan`, which carries the
heavier compute. Aggregator sided with `POP.Scan` (M-8). Gemini systematically
gravitated to the inner leaf when the surrounding region was the meaningful
unit (also true for `ConvertContent`); claude flagged this as a recurring
pattern in gemini's draft.

### `ConvertContent` (gemini C-2) vs `CompileTemplate` (claude+codex)
Same pattern as above. Gemini picked the standalone Markdown→HTML helper
(an admin-only one-shot endpoint); claude and codex both routed through
`CompileTemplate`, which subsumes the Markdown conversion *and* the
load-bearing template parse on the campaign-send path. Aggregator chose
`CompileTemplate` (M-7).

## Excluded candidates

- **gemini C-2 `Campaign.ConvertContent`** (`models/campaigns.go:214`) —
  one-shot admin "convert markdown" button; load profile is speculative;
  subsumed by M-7 `CompileTemplate`. Caller cite drift (`:213` should be
  `:244`) was noted but not load-bearing on the exclusion.
- **gemini C-6 `classifyBounce`** (`internal/bounce/mailbox/pop.go:214`) —
  pure 3-regex helper; lifts a tiny function and leaves the surrounding
  MIME+multipart compute on the home node. Subsumed by M-8 `POP.Scan`.
- **gemini C-8 `Session.ExtractZIP`** (`internal/subimporter/importer.go:373`) —
  fails rubric §"Disqualifiers" on filesystem coupling (writes to local
  `os.MkdirTemp` and returns local filenames); gemini's own scoring marked
  state independence as `no`. Both other critics DROPped per Rule 5.
- **claude C-9 `SendTxMessage`** (`cmd/tx.go:17`) — wider HTTP handler with
  five distinct collaborators; replaced by the leaf at M-9 per codex's
  MODIFY (and claude's own MODIFY of codex's leaf-pick converged in the same
  direction).
