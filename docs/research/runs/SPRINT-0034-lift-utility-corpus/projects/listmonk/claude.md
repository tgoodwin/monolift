# Listmonk — candidate lift regions (Phase 1)

## Project read

Listmonk is a self-hosted bulk-mailer / newsletter manager: a single Go binary that
ingests subscriber lists, compiles per-campaign templates, and fans out millions of
personalized e-mails through SMTP pools or HTTP "postback" webhooks. The hot loops
sit in three orthogonal places: (1) the campaign send pipeline, where each
running campaign drains batches of subscribers, renders one HTML/plaintext message
per recipient, and pushes through a `Messenger.Push` interface
(`internal/manager/`); (2) the bounce ingestion path, where webhook handlers
perform signature verification + JSON unmarshal, and a periodic POP3 mailbox
scanner pulls and classifies bounce e-mails (`internal/bounce/`); and (3) the
admin-side bulk operations — CSV/ZIP subscriber import (`internal/subimporter/`),
image-thumbnail generation (`cmd/media.go`), and Markdown→HTML campaign
compilation (`models/campaigns.go`). The `Manager` and `Importer` are the
framework dispatchers — *not* lift candidates — but every per-recipient,
per-image, or per-webhook unit they hand off to is a coherent region with
clean inputs and durable outputs (DB, SMTP server, queue), making listmonk
unusually rich in liftable workload units for its codebase size.

---

### C-1: Per-recipient campaign message render

- **Region root:** `internal/manager/message.go:13` —
  `(*Manager).NewCampaignMessage(c *models.Campaign, s models.Subscriber)`
  with its inner `(*CampaignMessage).render()` at `internal/manager/message.go:33`.
  Executes the precompiled `Campaign.Tpl` (base + content), `SubjectTpl`, and
  optional `AltBodyTpl` against the (campaign, subscriber) pair, producing the
  final HTML, subject, and plaintext body.
- **Caller(s):** `internal/manager/pipe.go:172` — `(*pipe).newMessage`, which is
  invoked once per subscriber inside the campaign send loop at
  `internal/manager/pipe.go:96` (`NextSubscribers`). Also called on the
  request path at `cmd/campaigns.go:183` (`PreviewCampaign`),
  `cmd/archive.go:166` (`CampaignArchivePage`), and
  `cmd/public.go:185` (`ViewCampaignMessage`).
- **Why useful (rubric scoring):**
  - Compute envelope: yes — runs `html/template` + `text/template` execute over a
    user-supplied campaign body that may include sprig helpers, link tracking
    rewrites, and view-pixel injection; cost scales with template size and is
    paid per recipient.
  - Load profile: yes — a single campaign launch can fan this out to N
    subscribers (millions, in production deployments) within a short window;
    classic batch burst.
  - Coherent unit: yes — input is a `*Campaign` (already-compiled template,
    config strings, attachment list) plus a value-typed `Subscriber`; output is
    the rendered byte slices on the message struct. No `*App` is reachable.
  - State independence: yes — does not mutate package globals; the only shared
    mutable state is `Manager.links` (a tracked-link UUID memo), which can be
    made replica-local without correctness loss because every link is durably
    registered in the DB via `store.CreateLink`.
  - Latency / failure: yes — caller is the campaign worker goroutine
    (`worker()` at `manager.go:462`), already off the request path; on the
    preview path the budget is generous (admin UI, ~hundreds of ms tolerated).
- **Activation shape:** Method on `Manager` invoked from a worker goroutine
  draining a buffered channel; secondary callers are HTTP handlers.
- **Confidence:** high — the canonical "expensive thing under load" in this
  binary. Would change my mind only if profiling shows the SMTP send dominates
  render cost by orders of magnitude (which I expect for trivial templates,
  but not for the typical sprig-heavy newsletter).
- **Risk notes:** the compiled `*template.Template` lives on `Campaign.Tpl`
  and is shared across recipients; lifting requires either shipping the
  precompiled tree (Go templates aren't directly serializable — would need to
  re-parse from the body source on the remote side) or shipping the source
  body and recompiling once per replica per campaign. The
  `Manager.tpls` cache and `Manager.links` map are read-paths and can be made
  per-replica.

---

### C-2: SMTP message send (`Emailer.Push`)

- **Region root:** `internal/messenger/email/email.go:111` —
  `(*Emailer).Push(m models.Message) error`. Picks an SMTP server from the
  pool, builds the `smtppool.Email` (headers, attachments, body, alt-body),
  resolves Return-Path / Bcc / Cc envelope semantics, and calls
  `srv.pool.Send(em)` which performs the actual TCP+TLS+SMTP dialog.
- **Caller(s):** `internal/manager/manager.go:523` — `worker()` calls
  `m.messengers[msg.Campaign.Messenger].Push(out)` for every campaign message;
  also `cmd/public.go:629` (`SelfExportSubscriberData`) and the
  `(*App).sendNotif` path.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — outbound SMTP round-trip per message, dominated by
    network latency to the relay, plus per-call attachment copying and header
    serialization. Aggregates well across many recipients.
  - Load profile: yes — one campaign sustains this at `MessageRate`/sec for
    minutes-to-hours; tx and notification messages add a smaller bursty stream.
  - Coherent unit: yes — input is a value-typed `models.Message`; output is an
    error. The `*Emailer` carries config (`servers []*Server`, each with a
    pool) but holds no per-message mutable state; `rand.Intn` for round-robin
    is stateless modulo entropy.
  - State independence: maybe — `srv.pool` (smtppool) is a long-lived TCP/TLS
    connection pool; a remote replica can hold its own pool, so this is fine
    for a per-replica state model. Not OK if the lift insists on shared pools.
  - Latency / failure: yes — caller is the worker goroutine, off the request
    path; failure path is an `error` that increments `pipe.OnError()` and may
    pause the campaign — bounded retry semantics already exist.
- **Activation shape:** Interface-method (`Messenger.Push`) called from a worker
  loop draining `campMsgQ` / `msgQ`.
- **Confidence:** high — IO-bound, embarrassingly parallel, and the natural
  scale-out shape for any mailer.
- **Risk notes:** the `smtppool` keeps long-lived auth'd TCP connections; if a
  remote replica is short-lived, reconnect amortization is lost. Attachments
  are copied byte-for-byte on every call (`make([]byte, len(f.Content))`),
  which is wasteful per-call but lift-friendly (no aliasing back to the
  caller).

---

### C-3: HTTP webhook delivery (`Postback.Push`)

- **Region root:** `internal/messenger/postback/postback.go:97` —
  `(*Postback).Push(m models.Message) error`. Builds a `postback` struct,
  marshals to JSON via easyjson, and POSTs to a configured remote URL with
  optional Basic auth and retries.
- **Caller(s):** `internal/manager/manager.go:523` (the same worker dispatch
  loop) when `msg.Campaign.Messenger` resolves to a postback messenger.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — JSON marshal of the full message including
    attachments + recipient + campaign metadata, plus an outbound HTTPS POST.
    Aggregates well; latency-dominated.
  - Load profile: yes — same campaign burstiness as C-2; users wire postback
    messengers to internal APIs / Mailgun / Sendgrid HTTP APIs and drive them
    at MessageRate.
  - Coherent unit: yes — input is `models.Message`, output is an error;
    `*Postback` holds an `*http.Client`, an auth string, and an `Options`
    config struct. Clean.
  - State independence: yes — `http.Client` is connection-pooled but
    replica-local; nothing is mutated globally.
  - Latency / failure: yes — async caller (worker), errors flow through
    `pipe.OnError`.
- **Activation shape:** Interface-method (`Messenger.Push`) called from worker.
- **Confidence:** high — sibling of C-2 with a different transport; both
  satisfy the rubric independently and benefit identically from being
  scale-out workers.
- **Risk notes:** identical to C-2. The attachment byte copy on every call
  (`postback.go:128`) doubles memory traffic, mitigated by lift since the
  remote can free immediately.

---

### C-4: Bulk subscriber CSV ingest (`Session.LoadCSV`)

- **Region root:** `internal/subimporter/importer.go:452` —
  `(*Session).LoadCSV(srcPath string, delim rune) error`. Counts file lines,
  parses CSV header, then per-row: validates+sanitizes email (regex,
  domain blocklist/allowlist), builds a `SubReq`, JSON-decodes the
  `attributes` column, and pushes onto `subQueue`. Sibling drain loop
  `(*Session).Start()` at `:273` then commits in 10k-row transactions.
- **Caller(s):** `cmd/import.go:101,115` — `go sess.LoadCSV(...)` after the
  upload is staged to a temp file by `ImportSubscribers`. Always launched as
  a goroutine; the HTTP handler returns 200 immediately with the import stats.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — CSV parse + per-row JSON unmarshal +
    `mail.ParseAddress` + domain map lookups (with wildcard fallback) + Title-
    casing for missing names. Cost scales with file size and column count.
  - Load profile: yes — periodic, heavy, and bounded only by upload size; an
    operator dump of millions of rows lights this up for many minutes.
  - Coherent unit: maybe — input is a temp filepath (string) and a delimiter
    rune; output goes to a channel owned by the `Session`. The filepath input
    is a soft disqualifier for a remote lift unless the upload bytes are
    streamed to the replica instead. The validation + parse loop itself is a
    pure function of bytes-in.
  - State independence: yes — only mutates the session-local progress counter
    via `incrementImportCount` (which takes a mutex on the `Importer`). The
    domain block/allow-lists are read-only.
  - Latency / failure: yes — explicitly run as a background goroutine, the
    HTTP caller returns immediately; failure surfaces via the
    `import.alreadyRunning` / `StatusFailed` status, no synchronous contract.
- **Activation shape:** Background goroutine kicked off by an HTTP handler
  after staging an upload to a temp file.
- **Confidence:** high if the input is the byte stream (CSV blob), medium if
  it must remain a filepath. The rubric explicitly accepts "filesystem-heavy
  transforms... that are not bound to local-only paths," and the temp file
  here is incidental — the caller has the bytes in hand.
- **Risk notes:** the `Importer` enforces single-import-at-a-time via a
  package-level lock and singleton `*Importer`; replicating that across
  remote replicas requires either externalizing the lock or accepting that
  the lift reduces to one replica per importer. Also reads from
  `s.im.opt.UpsertStmt` (a prepared statement bound to the local `*sql.DB`)
  via the drain loop — that drain (`Session.Start`) is a separate region and
  a poor lift; only `LoadCSV` (the parse phase) lifts cleanly.

---

### C-5: Image thumbnail generation (`processImage`)

- **Region root:** `cmd/media.go:212` — `processImage(file *multipart.FileHeader)
  (*bytes.Reader, int, int, error)`. Decodes an uploaded image with
  `imaging.Decode`, runs Lanczos resize to 250px width, encodes as PNG,
  and returns thumbnail bytes plus original W/H.
- **Caller(s):** `cmd/media.go:99` — `(*App).UploadMedia`, the only caller,
  invoked synchronously inside the `POST /api/media` handler.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — full decode of GIF/PNG/JPEG, Lanczos resampling
    (CPU-bound, scales with image area), and PNG re-encode. This is the
    canonical "image processing on user upload" example from the rubric.
  - Load profile: yes — bursty per-tenant on campaign authoring days when
    users upload many assets in quick succession; otherwise idle.
  - Coherent unit: maybe — input is a `*multipart.FileHeader`, which holds an
    open file handle; ideally lifted as `(reader []byte, contentType string)
    -> (thumb []byte, w, h int, err error)`. The two-arg shape is a thin
    refactor away.
  - State independence: yes — pure function over the input bytes; no globals,
    no DB.
  - Latency / failure: yes — caller is on the request path but already pays
    decode + resize + encode locally; an extra hop is in the noise next to
    Lanczos on a multi-megabyte image. Errors flow through to the HTTP
    response naturally.
- **Activation shape:** Synchronous helper invoked from an HTTP route handler
  (`POST /api/media`).
- **Confidence:** high — textbook CPU-bound, side-effect-free, naturally
  parallel. The only friction is the `*multipart.FileHeader` input.
- **Risk notes:** the larger handler (`UploadMedia`) also calls `a.media.Put`
  to push to the configured store (filesystem or S3); that's a separate
  side-effect and stays with the caller. If lifted, the caller hands a byte
  blob over and gets a thumbnail blob back.

---

### C-6: POP3 bounce mailbox scan (`POP.Scan`)

- **Region root:** `internal/bounce/mailbox/pop.go:79` —
  `(*POP).Scan(limit int, ch chan models.Bounce) error`. Connects, auths, lists
  messages, and for each one: retrieves the raw bytes, parses MIME, walks
  multipart parts, runs header lookups + regex fallbacks, parses dates,
  classifies bounce hard/soft via `classifyBounce` (regex over the body), and
  pushes a `models.Bounce` onto a channel. Then deletes processed messages
  from the server.
- **Caller(s):** `internal/bounce/bounce.go:138` —
  `(*Manager).runMailboxScanner`, an infinite loop sleeping
  `m.opt.Mailbox.ScanInterval` between calls; itself launched as a goroutine
  from `(*Manager).Run` at `bounce.go:120`.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — MIME parse + multipart walk + multiple multi-line
    regex passes over the full body (`reSMTPStatus`, `reHardBounce`, plus
    seven `headerLookups` regexes) per message, batched up to 1000 per scan.
  - Load profile: maybe — periodic with a configurable interval, but each
    scan can be heavy if a campaign just went out and bounces are piling up;
    "periodic but heavy" matches the rubric's positive signal.
  - Coherent unit: yes — input is a (`limit int`, `chan models.Bounce`); the
    POP3 client is held on the receiver but is per-instance and can be
    re-instantiated remotely with the same `Opt`.
  - State independence: maybe — pushes onto a Go channel owned by the bounce
    `Manager`; remote-side this becomes "push to a queue or HTTP endpoint."
    Does not mutate shared in-process state otherwise.
  - Latency / failure: yes — caller is a background goroutine sleeping
    between scans; failure logs and retries on the next tick.
- **Activation shape:** Long-lived background goroutine in an infinite poll
  loop.
- **Confidence:** medium — the per-call MIME+regex work is real, but lifting
  is most valuable if the channel sink is replaced with a durable queue,
  which is a small extension to the existing design.
- **Risk notes:** the function deletes messages from the POP server after
  pushing onto the channel — if the channel push fails (the current code does
  `select { case ch <- ...: default: }`, dropping silently), bounces are
  still deleted. Lifting must preserve that contract; ideally the durable
  queue ack happens before the POP3 `Dele`.

---

### C-7: SES/SNS bounce notification processing (`SES.ProcessBounce`)

- **Region root:** `internal/bounce/webhooks/ses.go:108` —
  `(*SES).ProcessBounce(b []byte) (models.Bounce, error)`. Unmarshals the SNS
  envelope, calls `verifyNotif` (which fetches and caches the SNS signing
  cert via HTTPS at `:232` and runs `cert.CheckSignature(SHA1WithRSA, ...)`),
  then unmarshals the inner SES payload, classifies bounce type, and walks
  message headers for the campaign UUID.
- **Caller(s):** `cmd/bounce.go:167` — `(*App).BounceWebhook` for `service ==
  "ses"`, called on every public webhook POST. Sibling regions:
  `(*Sendgrid).ProcessBounce` at `internal/bounce/webhooks/sendgrid.go:53`
  (ECDSA P-256 verify + multi-bounce JSON), `(*Forwardemail).ProcessBounce`
  at `forwardemail.go:49` (HMAC-SHA256 verify + JSON), and the Postmark /
  Lettermint variants.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — JSON unmarshal of envelope + nested payload, RSA
    signature verification (and a one-time-per-cert HTTPS fetch on cache
    miss), and a header walk. Sendgrid's variant additionally does ECDSA
    verify + ASN.1 unmarshal.
  - Load profile: yes — bursty in lockstep with campaign sends (one notif
    per bounced recipient); a bad-list campaign generates a flurry.
  - Coherent unit: yes — input is `[]byte` (the raw POST body) and an
    optional signature string; output is `models.Bounce` (or `[]models.Bounce`
    in the multi-bounce variants). The receiver carries only a cert cache.
  - State independence: yes — the cert cache is a `map[string]*x509.Certificate`
    populated from a public URL; replica-local is fine, since each replica
    will warm its own on first use.
  - Latency / failure: yes — caller is an HTTP webhook handler whose response
    is just an ack; the budget is forgiving (typically 10s+ in SNS retry
    semantics) and the `bounce.Record(b)` call at `cmd/bounce.go:243` is
    already async (pushes to a buffered channel).
- **Activation shape:** Interface-method-style processor invoked from an HTTP
  route handler that switches on the `:service` URL param.
- **Confidence:** high — every variant of `ProcessBounce` matches the
  webhook-fan-in / signature-verify + parse profile from the rubric's
  positive examples ("OAuth-token-exchange handler that does an outbound
  HTTPS round-trip plus signature verification").
- **Risk notes:** SES `getCert` hits an HTTPS URL on cache miss; in a remote
  lift each replica pays one such fetch per signing cert seen. Sendgrid
  needs its public key passed through configuration; today it's stored on
  `*Sendgrid` post-`NewSendgrid`, so the lift must arrange for the same key
  on the remote side.

---

### C-8: Campaign template compilation (`Campaign.CompileTemplate`)

- **Region root:** `models/campaigns.go:138` —
  `(*Campaign).CompileTemplate(f template.FuncMap) error`. Substitutes
  registered template-function regex aliases, parses the base template,
  conditionally runs `markdown.Convert` (goldmark) for Markdown campaigns,
  parses the content template, links the two trees via `AddParseTree`, and
  parses the alt-body template. Returns parsed `*template.Template` on the
  receiver.
- **Caller(s):** `internal/manager/pipe.go:35` — `newPipe` calls it once at
  campaign send start. On the request path: `cmd/campaigns.go:176`
  (`PreviewCampaign`), `cmd/archive.go:247` (`compileArchiveCampaigns`,
  invoked from both the public archive page and the API preview), and
  `cmd/public.go:178` (`ViewCampaignMessage`).
- **Why useful (rubric scoring):**
  - Compute envelope: yes — three template parses (base, content, alt) +
    optional Markdown→HTML conversion via goldmark, all over user-supplied
    bodies that may include sprig functions and link-tracking markup.
  - Load profile: maybe — once per campaign at send time (cheap), but on
    preview / archive views it can be hit per request, and the public archive
    page is publicly cacheable, so traffic spikes on viral newsletters land
    here.
  - Coherent unit: yes — receiver is a `*Campaign` value, input is a
    `template.FuncMap`; output mutates fields on the receiver
    (`Tpl`, `SubjectTpl`, `AltBodyTpl`). Lifts cleanly as
    `(body, altBody, subject string, ct string, funcs FuncMap) -> (parsed
    trees)` — the funcs map is the awkward bit since some closures capture
    `*Manager`, but those closures only read config + call `store.CreateLink`.
  - State independence: yes — modulo the closure capture above. No globals.
  - Latency / failure: maybe — on the campaign-send path the caller is the
    worker, off the critical path. On the preview / archive path the caller
    is HTTP, but the budget is admin-tolerant.
- **Activation shape:** Method-on-value, called once per campaign-start and
  again on each preview / archive page render.
- **Confidence:** medium — dominant cost compared to C-1 only when the body
  is large and complex, which is correlated but not guaranteed. The
  Markdown-conversion branch is pure CPU and the strongest sub-region.
- **Risk notes:** the `template.FuncMap` includes closures over `*Manager`
  for `TrackLink` (which writes new tracking URLs to the DB on cache miss).
  A remote lift either ships a stub funcmap and re-resolves links locally,
  or accepts that link registration calls back to the home node.

---

### C-9: Transactional message handler (`SendTxMessage`)

- **Region root:** `cmd/tx.go:17` — `(*App).SendTxMessage(c echo.Context)
  error`. Parses the API request (multipart or JSON), validates fields,
  resolves N subscriber records (DB lookups), and for each: calls
  `m.Render(sub, tpl, funcs)` (`models/messages.go:74` — body + alt +
  subject template execution), builds a `models.Message`, and pushes via
  `a.manager.PushMessage`.
- **Caller(s):** Registered at `cmd/handlers.go:196` —
  `g.POST("/api/tx", pm(a.SendTxMessage, "tx:send"))`. This is *the* public
  transactional-mail API endpoint.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — for N recipients, runs three template executes
    each, plus a DB subscriber lookup; both grow with N.
  - Load profile: yes — application-driven and bursty (signup flurries,
    password-reset surges, OAuth callbacks); the rubric explicitly cites
    "OAuth callback flurries" as a positive example.
  - Coherent unit: maybe — the handler reaches into `a.core` (DB),
    `a.importer.SanitizeEmail` (validation), `a.manager.GetTpl` (cached
    template), `a.manager.GenericTemplateFuncs`, and `a.manager.PushMessage`.
    Five distinct collaborators. Lifts cleanly only if those are
    interface-typed and shipped together — the abstraction is already in
    place (`Manager`, `Core`), so it's a refactor rather than a redesign.
  - State independence: yes — the manager template cache (`m.tpls`) is
    read-only at this site, and `PushMessage` enqueues onto a buffered
    channel. No mutation of globals.
  - Latency / failure: maybe — the API caller is synchronous and waits for
    `c.JSON(...)`, so adding a hop costs the caller. But the call already
    does N DB lookups + N renders + N enqueues, so the inherent latency is
    already O(N × tens of ms). An extra hop is in the noise.
- **Activation shape:** HTTP route handler under `/api/tx`.
- **Confidence:** medium — this is a strong "compute under variable load"
  candidate, but the dependency closure (DB + importer + manager) is wider
  than C-1/C-2/C-3, so the lift surface is larger.
- **Risk notes:** `a.manager.GetTpl` returns a pointer to a shared cached
  template; the lift must ensure the remote replica has access to the same
  cache or lazily refetches. `PushMessage` writes to a buffered channel
  internal to the manager and times out at 3s — that timeout is preserved
  cleanly as long as the remote can call back into the home node's manager.

---

### C-10: Public campaign archive page render (`CampaignArchivePage`)

- **Region root:** `cmd/archive.go:119` — `(*App).CampaignArchivePage(c
  echo.Context) error`. Looks up the campaign by UUID or slug, calls
  `compileArchiveCampaigns([]models.Campaign{pubCamp})` (which runs
  `CompileTemplate` + subject render against archive metadata at
  `archive.go:239`), then `manager.NewCampaignMessage(camp, sub)` to render
  the body, and writes the HTML response.
- **Caller(s):** `cmd/handlers.go:283` —
  `g.GET("/archive/:id", a.CampaignArchivePage)`, public unauthenticated
  endpoint exposed when `EnablePublicArchive` is true.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — composes C-8 (one campaign template compile +
    Markdown convert) and C-1 (one full message render) per request. For a
    campaign that uses the rich-content + sprig stack, this is a
    non-trivial render budget.
  - Load profile: maybe — public URL, so it gets the full burst pattern of
    "newsletter goes viral, archive link gets hit hard"; the rest of the
    time it's lukewarm.
  - Coherent unit: yes — input is the URL params + DB-fetched campaign;
    output is the rendered HTML body. The function reads from `a.core` and
    `a.manager` but does no writes.
  - State independence: yes — read-only path; no mutation of globals; no
    long-lived per-request resource (returns full HTML, not a stream).
  - Latency / failure: yes — public-facing but with no strict p99 budget;
    template render of a few hundred ms is acceptable on a viral campaign.
    Failure path is a normal HTTP error page.
- **Activation shape:** HTTP route handler, public.
- **Confidence:** medium — the per-call cost is real, but absent a CDN cache
  miss storm the absolute traffic is modest for most installations. Lifts
  best as part of a "render service" alongside C-1/C-8.
- **Risk notes:** depends on the same `TemplateFuncs` closure-over-manager
  as C-8. A reasonable lift is "the rendering service" comprising C-1, C-8,
  and C-10 together — they share the same template machinery.

---

## Honest assessment

I am most confident in **C-1** (per-recipient render), **C-2** (SMTP send),
**C-3** (postback delivery), **C-5** (image thumbnail), and **C-7** (webhook
bounce processing) — these are textbook lift targets: bursty, compute- or
IO-bound, value-typed inputs, durable outputs (DB / queue / SMTP relay), and
already invoked from async contexts. **C-4** (CSV import) is strong but only
if we accept that the input is the byte stream (the temp-file path is
incidental); the singleton-`*Importer` lock is a real limitation but lives
*outside* the region itself. **C-6** (POP3 scan), **C-8** (template
compile), **C-9** (`SendTxMessage`), and **C-10** (archive render) are
genuinely marginal — each fails one rubric criterion softly: C-6 because the
in-process channel sink couples it to the bounce manager, C-8 because the
preview / archive callers are the only request-path callers and admin
tolerance softens the win, C-9 because of the wide collaborator closure, and
C-10 because traffic volume is modest most of the time. The region I
*suspect* is a great lift candidate but couldn't justify is the bounce
manager's `Run`/`runMailboxScanner` *taken as a whole* — it's a recurring,
batch-like compute-and-IO loop that obviously wants its own scaling axis,
but the rubric forbids picking the framework dispatcher and forces me to
pick `Scan` itself, which has the awkward channel sink. A small refactor
that turned the `chan models.Bounce` sink into a `BounceSink` interface
would make this a clean C-1-class candidate.
