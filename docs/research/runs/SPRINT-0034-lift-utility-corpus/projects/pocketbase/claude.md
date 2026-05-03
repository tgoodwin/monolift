# SPRINT-0034 Lift-utility candidates — PocketBase

## Project read

PocketBase is a single-binary backend-as-a-service: an embedded SQLite store fronted by a REST router (`apis/`), a typed event/hook system (`tools/hook`), a small mailer (`tools/mailer`), a blob filesystem abstraction over local FS or S3 (`tools/filesystem`), and an optional goja JS plugin host (`plugins/jsvm`). The DB is the dominant runtime resource; almost all request paths terminate in a `RunInTransaction(...)` over the local SQLite handle, which means anything that needs to be transactional with a write is structurally hard to lift. Computationally expensive paths cluster outside the DB: image thumbnailing (`tools/filesystem.System.CreateThumb`), bcrypt hash/verify on auth records (`core/field_password.go`), OAuth2 token exchange + userinfo + avatar download (`apis/record_auth_with_oauth2.go`), SMTP send (`tools/mailer/smtp.go`), zip archiving for backups (`tools/archive`), batched log inserts, and recursive relation expansion for record responses (`core/record_query_expand.go`). The realtime SSE endpoint is a built-in long-lived per-request resource (`apis/realtime.go:40`) and is excluded by disqualifier.

## Candidates

### C-1: Image thumbnail generator

- **Region root:** `tools/filesystem/filesystem.go:489` — `(*System).CreateThumb(originalKey, thumbKey, thumbSize string) error`. Decodes the source image (with webp support), runs `imaging.Resize`/`Fit`/`Fill`, and re-encodes to a blob writer.
- **Caller(s):** `apis/file.go:171` — `api.createThumb(...)`, which wraps the call in a `singleflight.Group` plus a `semaphore.Weighted` keyed by `PB_THUMBS_MAX_WORKERS` (defaults to `NumCPU()+2`); registered at `apis/file.go:45` (`GET /files/{collection}/{recordId}/{filename}`).
- **Why useful (rubric scoring):**
  - Compute envelope: yes — full image decode, resize, and re-encode dominate; the file already coordinates concurrency via a semaphore precisely because this work is heavy enough to exhaust CPU.
  - Load profile: yes — bursty on first view of a freshly-uploaded image and on cache-miss after S3 eviction; the `singleflight` collapse + `PB_THUMBS_MAX_WAIT` (60 s default) confirm authors expect spikes.
  - Coherent unit: yes — three string args (`originalKey`, `thumbKey`, `thumbSize`) plus a `*System` whose only state is a `blob.Bucket` constructed from declarative S3/local config.
  - State independence: yes — reads/writes go through the `blob.Bucket` interface; no in-process mutable state participates in correctness; the per-process `singleflight.Group` is a deduplication optimization, not a correctness invariant.
  - Latency / failure: yes — the calling handler already tolerates `thumbGenMaxWait` of 60 s and a fallback to the original image on error (`apis/file.go:178-184`).
- **Activation shape:** HTTP route handler (GET file → conditional thumb generation goroutine via singleflight).
- **Confidence:** high — would change my mind only if the `*System.bucket` turned out to hold per-process credentials that cannot be re-instantiated remotely (it doesn't; `NewS3` is parameterized by config).
- **Risk notes:** the source/destination keys must be reachable from the lifted replica's `*System`; if S3 is configured this is trivially true, if local FS is configured both replicas must mount the same `pb_data/storage` path. The semaphore/singleflight only provides per-process deduplication — a remote replica gets its own.

### C-2: Bcrypt password hashing on record save

- **Region root:** `core/field_password.go:286` — `(*PasswordField).setValue(record *Record, raw any)`. Calls `bcrypt.GenerateFromPassword` at the field's configured `Cost` (default `bcrypt.DefaultCost`).
- **Caller(s):** invoked through the field setter chain on every record set of a password field, e.g. `forms/record_upsert.go:115` via `record.SetIfFieldExists`, ultimately reachable from `apis/record_crud.go:32` (`POST /collections/{collection}/records`) and the password-reset/change paths (`apis/record_auth_password_reset_confirm.go`, `apis/record_auth_email_change_confirm.go`).
- **Why useful (rubric scoring):**
  - Compute envelope: yes — bcrypt at default cost 10 is ~50–100 ms of pure CPU; this is the canonical CPU-bound auth primitive.
  - Load profile: yes — registration bursts, bulk-import scripts (`apis/collection_import.go`), and password rotations all spike this; OAuth2 sign-up additionally calls `record.SetRandomPassword()` (`apis/record_auth_with_oauth2.go:344`) which goes through the same setter.
  - Coherent unit: yes — pure function: plaintext bytes + cost → `(hash, error)`. The `*Record` arg is only used to stash the result via `SetRaw`; the hashing itself is standalone.
  - State independence: yes — no shared state read or written; result is per-call.
  - Latency / failure: maybe — synchronous on the request path, but bcrypt itself is already O(50–100 ms) so an extra network hop is in the noise; failure is a clean error returned through `PasswordFieldValue.LastError`.
  - Note: the rubric "any `no` excludes" rule — this scores yes/maybe/yes/yes/yes, no `no`s.
- **Activation shape:** Field-setter callback invoked from CRUD/auth HTTP handlers and JSVM bindings.
- **Confidence:** high — classic textbook lift target.
- **Risk notes:** the setter must produce a `PasswordFieldValue` that the caller then `SetRaw`s on the in-memory record; the lifted version must return both `Hash` and `LastError` so the local `Record` can be updated. Trivial to wrap, but it is not a top-of-handler entry — it sits inside a field-setter dispatch table.

### C-3: Bcrypt password verification on login

- **Region root:** `core/field_password.go:317` — `(PasswordFieldValue).Validate(pass string) bool`, which calls `bcrypt.CompareHashAndPassword`.
- **Caller(s):** `apis/record_auth_with_password.go:87` — `e.Record.ValidatePassword(e.Password)` inside the `POST /collections/{collection}/auth-with-password` handler (registered at `apis/record_auth.go`); reused from `forms/record_upsert.go:204` (old-password check).
- **Why useful (rubric scoring):**
  - Compute envelope: yes — symmetric to C-2; bcrypt verify is the same CPU cost as hash.
  - Load profile: yes — login is the spikiest event in the system (campaign launches, password-reuse attacks, cold-mornings); per-tenant uneven.
  - Coherent unit: yes — `(hashBytes, plainBytes) → bool`.
  - State independence: yes — pure CPU, no state.
  - Latency / failure: maybe — synchronous on the auth request; same "already O(100 ms)" argument as C-2.
- **Activation shape:** Method invoked synchronously from the `auth-with-password` HTTP handler.
- **Confidence:** high — alongside C-2 this is the most defensible CPU-only lift target in the codebase.
- **Risk notes:** failure must be indistinguishable from local in timing for a constant-time security argument; a remote replica adds variance that may be exploitable as a side-channel for "user exists / does not exist." This is partially mitigated already by `recordAuthWithOAuth2`'s pattern of running a dummy bcrypt against a fixed hash, but the lifted version would need similar care.

### C-4: SMTP send

- **Region root:** `tools/mailer/smtp.go:62` — `(*SMTPClient).send(m *Message) error`. Builds a `mailyak` envelope, attaches files, generates a Message-ID, dials the SMTP server (optionally TLS), authenticates, and writes the message.
- **Caller(s):** `tools/mailer/smtp.go:54` (`SMTPClient.Send` wrapper which calls the registered `OnSend` hook); `mails/record.go:48,86,125,160,199,239` (each of the five record-mailer wrappers — auth alert, OTP, password reset, verification, email change).
- **Why useful (rubric scoring):**
  - Compute envelope: yes — full TCP+TLS handshake, SMTP DATA round trip, attachment streaming with mimetype detection. Latency-dominated, scales with attachment size.
  - Load profile: yes — bursty on signup waves, OTP campaigns, password-reset storms; per-tenant uneven (one workspace running an invite blast).
  - Coherent unit: yes — a single `*Message` value (addresses, subject, HTML, text, headers, attachments) + a configured `*SMTPClient` whose state is host/port/credentials.
  - State independence: yes — config is immutable for the call's duration; no shared mutable in-process state. The optional `OnSend` hook is per-call.
  - Latency / failure: yes — every observed call site is wrapped in `routine.FireAndForget` (e.g. `apis/record_auth_password_reset_request.go:56`, `apis/record_auth_otp_request.go:103`); the user-facing response is already returned before send completes.
- **Activation shape:** Goroutine launched via `routine.FireAndForget` from a request-event hook handler.
- **Confidence:** high.
- **Risk notes:** the `OnSend` hook may be configured by user code (JS plugin or Go) to do per-process side effects (rate limits, metrics). If the hook is registered in-process, lifting the SMTP send must either re-fire the hook on the local side or pass it through.

### C-5: Composite record-mailer (template + SMTP)

- **Region root:** `mails/record.go:128` — `SendRecordPasswordReset(app core.App, authRecord *core.Record) error`. Mints a password-reset token, resolves the email template (`resolveEmailTemplate` at `mails/record.go:251` — does field-substitution and HTML escaping over `authRecord.Collection().Fields`, then renders the layout via `html/template`), constructs a `mailer.Message`, fires the `OnMailerRecordPasswordResetSend` hook, and sends. Sibling functions: `SendRecordOTP`, `SendRecordVerification`, `SendRecordChangeEmail`, `SendRecordAuthAlert`.
- **Caller(s):** `apis/record_auth_password_reset_request.go:57` inside a `routine.FireAndForget` block (registered handler at `apis/record_auth.go` for `POST /collections/{collection}/request-password-reset`); analogous for OTP at `apis/record_auth_otp_request.go:104`.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — `html/template` Layout+Body render with dynamic placeholder map, optional `html2text` derivation of plaintext (`mailer/html2text.go`), then SMTP round-trip. Mixed CPU + IO.
  - Load profile: yes — same bursty profile as C-4 but at a larger granularity (token mint + render included).
  - Coherent unit: yes — `(app, *Record) → error`. The `*Record` is a value with no live DB cursor attached at this point (it was already fetched).
  - State independence: maybe — the function takes a `core.App` and uses it to mint a token (DB read) and to fetch the mailer client; this is a clean interface, but it does mean a remote replica needs an `App` shim (or the function must be split so the token is minted locally and only the render+send is lifted).
  - Latency / failure: yes — caller is `routine.FireAndForget`, errors are logged not surfaced.
- **Activation shape:** Goroutine via `routine.FireAndForget` from a request handler, after the response is committed.
- **Confidence:** medium-high — would prefer to lift only the `(message *mailer.Message) → error` tail (which is C-4) unless the template render is shown to be non-trivial in profile.
- **Risk notes:** `app.Save(otp)` in `SendRecordOTP` reaches back into the local DB to update `sentTo`; that branch must remain local. The token mint at the top of each function is a DB read.

### C-6: OAuth2 callback exchange + userinfo

- **Region root:** `apis/record_auth_with_oauth2.go:30` — `recordAuthWithOAuth2(e *core.RequestEvent) error`. The expensive sub-region within it is the block at lines 71–98: `provider.InitProvider() / FetchToken(code) / FetchAuthUser(token)`, each of which is an outbound HTTPS round-trip. The downstream `oauth2Submit` (line 259) does DB work and is not part of the proposed lift.
- **Caller(s):** registered as the handler for `POST /collections/{collection}/auth-with-oauth2` (`apis/record_auth.go`). Each provider implements `FetchAuthUser` in `tools/auth/<provider>.go` (~30 providers).
- **Why useful (rubric scoring):**
  - Compute envelope: yes — two outbound TLS round-trips (token exchange + userinfo) plus JSON decode of provider responses; sometimes a third for `safeFileFromURL` avatar.
  - Load profile: yes — bursty during user campaigns and OAuth callback flurries; per-provider asymmetric (a Discord outage spikes Discord callbacks).
  - Coherent unit: maybe — the full handler is a 140-line composition, but the outbound block (lines 71–98) is a clear sub-region with inputs `(providerConfig, code, codeVerifier, redirectURL)` and outputs `(*oauth2.Token, *auth.AuthUser)`.
  - State independence: yes for the outbound sub-region — `provider` is built per-call from `providerConfig`; the only shared state is `e.App.Store()` for Apple's name-relay (line 103), which can stay local.
  - Latency / failure: yes — the handler already wraps each call in a 30 s `context.WithTimeout`; total expected latency is multi-hundred ms; failures return `BadRequestError`.
- **Activation shape:** HTTP route handler.
- **Confidence:** medium — depends on cleanly separating the "outbound IO" sub-region from the surrounding DB work; if the lift granularity is the whole handler, the DB transaction in `oauth2Submit` is a coupling problem.
- **Risk notes:** `recordAuthWithOAuth2` re-enters the API via `processInternalRequest` to create the auth record (`sendOAuth2RecordCreateRequest`, line 387). That re-entry is structurally local-only and must remain so.

### C-7: Avatar download from OAuth2 provider

- **Region root:** `apis/record_auth_with_oauth2.go:468` — `safeFileFromURL(ctx context.Context, url string) (*filesystem.File, error)`. Builds a `safeHTTPClient` (loopback/private-IP guard via `net.Dialer.Control`), fetches the URL, limits to `DefaultMaxBodySize`, and constructs a `filesystem.File`.
- **Caller(s):** `apis/record_auth_with_oauth2.go:307` — invoked when the OAuth2 mapped avatar field is a file-type field; runs inline in `oauth2Submit` (which is itself inside a `RunInTransaction`).
- **Why useful (rubric scoring):**
  - Compute envelope: yes — outbound HTTPS, body read up to `DefaultMaxBodySize`, mime detect on the resulting bytes inside `filesystem.NewFileFromBytes`.
  - Load profile: yes — same OAuth burst profile as C-6, only on first-login of new users.
  - Coherent unit: yes — `(ctx, urlString) → (*File, error)`; the only side effect is the outbound request.
  - State independence: yes — the `safeHTTPClient` is built per-call.
  - Latency / failure: yes — already wrapped in a 10 s `context.WithTimeout` (line 302); failure is logged and the avatar field is dropped, not fatal.
- **Activation shape:** Synchronous call inside an OAuth2 record-creation transaction.
- **Confidence:** medium — would change my mind if the surrounding `RunInTransaction` made it impractical to round-trip a separate replica from inside an open SQLite tx.
- **Risk notes:** lives inside `oauth2Submit`'s transaction — the outbound HTTP is on the critical path of a tx-holding goroutine, which means lifting it adds a network RTT to the time the SQLite write lock is held. Acceptable for OAuth (rare, multi-hundred-ms anyway) but sharper than it looks.

### C-8: Backup zip writer

- **Region root:** `tools/archive/create.go:18` — `Create(src string, dest string, skipPaths ...string) error`. Walks the `src` filesystem, creates a `zip.Writer` over a destination file with `flate.BestSpeed`, and copies each file entry.
- **Caller(s):** `core/base_backup.go:84` — `archive.Create(txApp.DataDir(), tempPath, e.Exclude...)` from inside `(*BaseApp).CreateBackup`, which is reachable both from the cron-registered `__pbAutoBackup__` job (`core/base_backup.go:308`) and from the `POST /backups` handler (`apis/backup_create.go:30`). Also called in test fixtures.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — flate compression over the whole `pb_data` tree (DB file plus all uploaded local files when not on S3); routinely tens to hundreds of MB; CPU + IO mixed.
  - Load profile: yes — periodic-but-heavy (cron-driven on operator schedule) plus ad-hoc admin-triggered. Marginal on the spike axis (it isn't bursty in the request-path sense), but the periodic-but-heavy clause in the rubric covers it.
  - Coherent unit: yes — three params `(src, dest, skipPaths)`; no methods on a struct, no shared state.
  - State independence: maybe — the function itself is pure over the input filesystem, but the *caller* `CreateBackup` wraps it in `RunInTransaction(...)` to block writes during the snapshot. If the lift target is `archive.Create` alone, this is fine; if it includes `CreateBackup`, the tx coupling makes it a no-go.
  - Latency / failure: yes — caller is the cron tick or an admin-initiated synchronous request that already returns 204 immediately and lets the work proceed asynchronously (`apis/backup_create.go:35-37`).
- **Activation shape:** Cron-registered closure (`core/base_backup.go:308`) and admin HTTP handler (`apis/backup_create.go`).
- **Confidence:** medium — strong on compute and shape, weaker on whether it's actually "spikable" in the rubric's sense.
- **Risk notes:** the `src` is a local filesystem path on the originating replica; lifting it to a separate replica requires either a shared mount or streaming the source bytes over the wire, which defeats the lift's purpose. Practically this only makes sense if the lift target is given a `fs.FS` it can resolve remotely — `zipAddFS` already takes one.

### C-9: Record relation expansion

- **Region root:** `core/record_query_expand.go:34` — `(*BaseApp).ExpandRecords(records []*Record, expands []string, optFetchFunc ExpandFetchFunc) map[string]error`. Recursively walks an expand path (up to `maxNestedRels=6`), resolves the relation field (direct or `_via_` back-relation), runs DB queries for related records, indexes them by id, and assembles the `expand` tree on each input record.
- **Caller(s):** `apis/record_helpers.go:355` — `app.ExpandRecords(records, expands, expandFetch(app, requestInfo))` inside `defaultEnrichRecords`, which is the tail of `EnrichRecords` (`apis/record_helpers.go:265`). Called from `recordsList` (`apis/record_crud.go:100`), `recordView` (`apis/record_crud.go:199`), `recordCreate` and `recordUpdate` (`apis/record_crud.go:355,494`), and from the realtime broadcast path.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — recursive DB queries (one per relation level), JSON-aware indexing/joining of relations, plus `JSONEach` SQL for back-relations; the `record.GetStringSlice` + reindex loops scale with the cartesian product of `len(records) × len(relIds)`.
  - Load profile: yes — list endpoints with `expand=` are the common dashboard/UI pattern; per-tenant uneven (one big `?perPage=200&expand=author.team.org` query dominates).
  - Coherent unit: maybe — public method on `*BaseApp` with three explicit args; the `ExpandFetchFunc` callback is interface-typed and the fetch function passed by `expandFetch` itself reaches into permission checks (`apis/record_helpers.go:375`).
  - State independence: maybe — uses `app.ConcurrentDB()` for queries; no shared mutable state but the SQLite handle itself is process-local.
  - Latency / failure: yes — runs synchronously inside the response path, but each list response is already O(100 ms) when expansion is used; failure returns `failed` map and is logged but does not break the response.
- **Activation shape:** Synchronous call inside the response-finalization path of CRUD HTTP handlers and the realtime broadcast worker.
- **Confidence:** medium — the right granularity is probably `defaultEnrichRecords` rather than `ExpandRecords` itself, because the permission-checking `expandFetch` closure is the part that holds the request-scoped `RequestInfo`.
- **Risk notes:** the `ExpandFetchFunc` closure captures `*RequestInfo` (auth state) and the field resolver, so any lift must serialize that context. Not a deal-breaker, but the function-value-as-callback hides what otherwise looks like a clean signature.

### C-10: Batched log writer

- **Region root:** `core/base.go:1430` — the `WriteFunc` closure passed to `logger.NewBatchHandler` (`tools/logger/batch_handler.go:53`). Drains up to 200 accumulated `*logger.Log` entries and writes them via `txApp.AuxSave(model)` inside an `AuxRunInTransaction`. Triggered by either the size threshold (`Handle` at `tools/logger/batch_handler.go:185`) or the 3-second ticker (`core/base.go:1466`).
- **Caller(s):** invoked from `BatchHandler.WriteAll` (`tools/logger/batch_handler.go:226`); on the timer goroutine at `core/base.go:1459-1470`, on the size threshold inside `BatchHandler.Handle`, and on `OnTerminate`.
- **Why useful (rubric scoring):**
  - Compute envelope: maybe — N small INSERTs (default batch 200); per-call work scales with batch size and attribute serialization (`normalizeLogAttrValue` walks each attribute, including validation-error trees).
  - Load profile: yes — log volume is request-traffic-shaped, so it spikes with the request rate.
  - Coherent unit: yes — `(ctx, []*Log) → error`; no embedded state, the `app` is captured by closure.
  - State independence: no — the function writes through `app.AuxRunInTransaction` to the local SQLite logs DB; the whole point is to persist into that DB.
  - Latency / failure: yes — caller is a background goroutine on a 3-second tick; failure is silently logged.
- **Activation shape:** Background goroutine driven by ticker + size-threshold callback from inside `slog.Handler.Handle`.
- **Confidence:** low — clean shape but the state-independence `no` excludes it under the rubric. Listed only as the kind of region that *would* be ideal if the destination were a remote log store rather than the local Aux DB.
- **Risk notes:** the disqualifying factor is that the lift target's effect is "write to the same SQLite handle the parent process holds." Any lift would have to either replicate that DB connection (defeats the purpose) or change the destination. Including this row to make the rejection explicit rather than silent.

## Honest assessment

The candidates I'm most confident about are **C-1 (CreateThumb)**, **C-2 / C-3 (bcrypt hash and verify)**, and **C-4 (SMTP send)**. These are all archetypal "CPU-bound or IO-bound work that the existing code has already isolated behind a clean signature, with naturally lenient latency budgets." `CreateThumb` is the strongest single example in the codebase because the authors themselves built a per-process semaphore + singleflight queue around it — they already believe it's worth deduplicating and rate-limiting, which is one step short of "worth lifting." **C-7 (safeFileFromURL)** is similarly clean; I gave it slightly lower confidence only because it lives inside an open DB transaction at its only real call site.

Genuinely marginal: **C-9 (ExpandRecords)** and **C-10 (BatchHandler.WriteFunc)**. ExpandRecords passes the rubric on every axis except that the right cut point is fuzzy — the public method has a clean signature but the actually-interesting work happens inside an `ExpandFetchFunc` closure that captures request-scoped permission state. WriteFunc is included only because it's the kind of region that *should* be liftable but is structurally tied to writing the local SQLite logs DB; it earns a `no` on state independence and is honestly listed for completeness rather than as a recommendation. **C-8 (archive.Create)** is the inverse case: clean signature, real CPU + IO, but the rubric's "spikable load" criterion fits awkwardly — backups are periodic-heavy rather than bursty, and the calling `CreateBackup` couples to a DB transaction that defeats the lift unless the boundary is `archive.Create` itself with a remote-resolvable `fs.FS`.

A region I suspect would be a great lift candidate but couldn't justify under the rubric is the **JSVM hook execution** (`plugins/jsvm/jsvm.go:315` + the `executors` pool). Each `goja.Runtime` is a per-call CPU sandbox with no inherent shared state, the pool is sized exactly because the authors expect concurrent contention, and user-defined hooks can be arbitrarily expensive. But I cannot cite a hot path that demonstrably has a meaningful compute envelope in *this* codebase — it depends entirely on what scripts the operator deploys to `pb_hooks/`. With no in-tree fixture proving the workload, the rubric's "compute envelope" criterion gets a `maybe` at best, so I left it out rather than smuggle it in. If the corpus later admits "regions whose cost is operator-supplied," the JSVM executor pool deserves first-class status.
