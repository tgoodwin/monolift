# pocketbase — merged lift-region candidate set (Phase 2b final)

## Methodology note

Phase 1 drafts: claude, codex, gemini.
Phase 2a critics: each model reviewed the other two.
Phase 2b aggregator: claude (opus).

Inclusion rules applied per `PHASE2-PLAN.md` §"Inclusion rules". MODIFY corrections (line-cite drift, scope narrowing) applied before producing each merged entry. Source-tree spot-checks performed against `evaluation/pocketbase/` to verify file:line citations.

## Merged candidates (ranked strongest → weakest)

### M-1: Image thumbnail generation

- **pick_provenance:** claude+codex+gemini (3/3)
- **critique_status:** KEEP from all 3 critics (codex MODIFY against gemini's caller cite — corrected here from `apis/file.go:148` to `apis/file.go:171`/`:225`).
- **Region root:** `tools/filesystem/filesystem.go:489` — `(*System).CreateThumb(originalKey, thumbKey, thumbSize string) error`. Decodes the source image (with webp support), runs `imaging.Resize`/`Fit`/`Fill`, and re-encodes to a blob writer.
- **Caller(s):** `apis/file.go:171` — `api.createThumb(e, fsys, originalPath, event.ServedPath, thumbSize)` on a download cache miss; the helper at `apis/file.go:209` wraps the call in a `singleflight.Group` plus a `semaphore.Weighted` keyed by `PB_THUMBS_MAX_WORKERS` and ultimately calls `fsys.CreateThumb` (`apis/file.go:225`).
- **Why useful (rubric scoring):**
  - Compute envelope: yes — full image decode, resize, and re-encode dominate; the existing per-process semaphore + singleflight is independent evidence the authors expect this to exhaust CPU.
  - Load profile: yes — bursty on first view of a freshly-uploaded image and on cache miss after S3 eviction; `PB_THUMBS_MAX_WAIT` (60 s) confirms spike tolerance.
  - Coherent unit: yes — three string args plus a `*System` whose only state is a `blob.Bucket` constructed from declarative S3/local config.
  - State independence: yes — reads/writes go through `blob.Bucket`; the per-process `singleflight.Group` is a deduplication optimization, not a correctness invariant.
  - Latency / failure: yes — caller already tolerates a 60 s budget and falls back to the original image on error (`apis/file.go:178-184`).
- **Activation shape:** HTTP route handler (GET file → conditional thumb generation goroutine via singleflight).
- **Confidence:** high — would change my mind only if `*System.bucket` held per-process credentials that cannot be re-instantiated remotely; `NewS3` is parameterized by config, so it doesn't.
- **Risk notes:** source/destination keys must be reachable from the lifted replica's `*System`; if S3 is configured, trivial; if local FS is configured, both replicas must mount the same `pb_data/storage` path. The semaphore/singleflight only provides per-process deduplication — a remote replica gets its own.

### M-2: OAuth2 outbound exchange (token + userinfo)

- **pick_provenance:** claude+codex+gemini (3/3)
- **critique_status:** KEEP from all 3 critics, with both claude's and codex's MODIFY narrowing applied — the merged scope is the outbound-IO sub-region (`apis/record_auth_with_oauth2.go:71-98`), not the full handler. The downstream `oauth2Submit` (line 259) and the `processInternalRequest` re-entry (line 387) remain local.
- **Region root:** `apis/record_auth_with_oauth2.go:30` — `recordAuthWithOAuth2(e *core.RequestEvent) error`. The lifted sub-region is the block at lines 71–98: `provider.InitProvider() / FetchToken(code) / FetchAuthUser(token)`, each of which is an outbound HTTPS round-trip, plus the optional avatar download at `apis/record_auth_with_oauth2.go:307` (covered separately by M-8).
- **Caller(s):** registered as the `POST /collections/{collection}/auth-with-oauth2` handler at `apis/record_auth.go:35`; each of ~30 providers implements `FetchAuthUser` in `tools/auth/<provider>.go`.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — two outbound TLS round-trips (token exchange + userinfo) plus JSON decode of provider responses.
  - Load profile: yes — bursty during user-acquisition campaigns and OAuth callback flurries; per-provider asymmetric (a Discord outage spikes Discord callbacks).
  - Coherent unit: maybe — the full handler is a 140-line composition, but the outbound block has clear inputs `(providerConfig, code, codeVerifier, redirectURL)` and outputs `(*oauth2.Token, *auth.AuthUser)`.
  - State independence: yes for the outbound sub-region — `provider` is built per-call from `providerConfig`; the only shared-state touchpoint is `e.App.Store()` for Apple's name-relay (line 103), which stays local.
  - Latency / failure: yes — the handler already wraps each call in a 30 s `context.WithTimeout`; multi-hundred-ms latency expected; failures return `BadRequestError`.
- **Activation shape:** HTTP route handler.
- **Confidence:** medium — depends on cleanly separating the outbound-IO sub-region from the surrounding DB work. If the lift granularity collapses into the whole handler, the `RunInTransaction` in `oauth2Submit` becomes a coupling problem.
- **Risk notes:** `recordAuthWithOAuth2` re-enters the API via `processInternalRequest` to create the auth record (`sendOAuth2RecordCreateRequest`, line 387). That re-entry is structurally local-only and must remain so.

### M-3: Bcrypt password verification on login

- **pick_provenance:** claude+gemini (2/3) — same region, different framing/cites.
- **critique_status:** KEEP from all 3 critics. Codex MODIFY against gemini's draft applied: the bcrypt root is `core/field_password.go:317`, not the wrapper at `core/record_model_auth.go:78`; caller is `apis/record_auth_with_password.go:87`, not :82.
- **Region root:** `core/field_password.go:317` — `(PasswordFieldValue).Validate(pass string) bool`, which calls `bcrypt.CompareHashAndPassword`. Wrapped at `core/record_model_auth.go:78` as `(*Record).ValidatePassword(password string) bool`.
- **Caller(s):** `apis/record_auth_with_password.go:87` — `e.Record.ValidatePassword(e.Password)` inside the `POST /collections/{collection}/auth-with-password` handler; reused at `forms/record_upsert.go:204` for the old-password check.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — bcrypt at default cost ~50–100 ms of pure CPU.
  - Load profile: yes — login is the spikiest event in the system (campaign launches, password-reuse attacks, cold-mornings); per-tenant uneven.
  - Coherent unit: yes — `(hashBytes, plainBytes) → bool`.
  - State independence: yes — pure CPU, no state.
  - Latency / failure: maybe — synchronous on the auth path, but bcrypt is already O(100 ms), so an extra hop is in the noise.
- **Activation shape:** Method invoked synchronously from the `auth-with-password` HTTP handler.
- **Confidence:** high — alongside the symmetric hash-on-save (M-6), this is the most defensible CPU-only lift target in the codebase.
- **Risk notes:** failure must remain timing-indistinguishable for constant-time security; a remote replica adds variance that may be exploitable as a side-channel for "user exists / does not exist." Partially mitigated already by `recordAuthWithOAuth2`'s dummy-bcrypt pattern; the lifted version would need similar care.

### M-4: Backup archive zip writer

- **pick_provenance:** claude+gemini (2/3)
- **critique_status:** KEEP from claude (gemini didn't critique its own); codex MODIFY applied — caller is `core/base_backup.go:84` (not :76); state independence held to "maybe" because `tools/archive/create.go:18-35` reads/writes local filesystem paths. Critique-resolved excluded variant: the outer `(*BaseApp).CreateBackup` wrapper (codex C-2, gemini C-2) is dropped — see Discrepancies and Excluded sections.
- **Region root:** `tools/archive/create.go:18` — `Create(src string, dest string, skipPaths ...string) error`. Walks `src`, creates a `zip.Writer` with `flate.BestSpeed`, and copies each entry; `zipAddFS` already accepts an `fs.FS`, which is the natural seam for a remote-resolvable source.
- **Caller(s):** `core/base_backup.go:84` — `archive.Create(txApp.DataDir(), tempPath, e.Exclude...)` from inside `(*BaseApp).CreateBackup`, reachable both from the cron-registered `__pbAutoBackup__` job and from `apis/backup_create.go:30` (`POST /backups`).
- **Why useful (rubric scoring):**
  - Compute envelope: yes — flate compression over the whole `pb_data` tree (DB file plus uploaded local files when not on S3); routinely tens to hundreds of MB; CPU + IO mixed.
  - Load profile: yes — periodic-but-heavy (cron-driven on operator schedule) plus ad-hoc admin-triggered. Not bursty in the request-path sense, but the rubric's "periodic but heavy" clause covers it.
  - Coherent unit: yes — three params `(src, dest, skipPaths)`; no methods on a struct, no shared state.
  - State independence: maybe — the function itself is pure over the input filesystem, but the *caller* `CreateBackup` wraps it in `RunInTransaction(...)` (`core/base_backup.go:77`) to block writes during the snapshot. The lift target is `archive.Create` alone; including the wrapper would couple to the open SQLite tx.
  - Latency / failure: yes — the cron tick or the admin handler tolerate seconds-to-minutes of latency.
- **Activation shape:** Cron-registered closure (auto-backup) and admin HTTP handler (`POST /backups`).
- **Confidence:** medium — strong on compute and shape; weaker on whether it's "spikable" in the request-path sense.
- **Risk notes:** `src` is a local filesystem path on the originating replica; lifting requires either a shared mount or a remote-resolvable `fs.FS`. The wrapping `RunInTransaction` holds the SQLite write lock for the lift's full duration if the lift boundary creeps up to `CreateBackup` — keep the boundary at `archive.Create`.

### M-5: Password-reset record-mailer composite

- **pick_provenance:** claude+codex (2/3); gemini picks the inner template-render sub-cut as a separate candidate (M-11).
- **critique_status:** KEEP from gemini; codex MODIFY against claude applied — narrow scope to the password-reset wrapper specifically, not generalized across all record-mailers (because `SendRecordOTP` reaches back into the local DB at `mails/record.go:99,114`, and the email-change mail is synchronous at `apis/record_auth_email_change_request.go:42`).
- **Region root:** `mails/record.go:128` — `SendRecordPasswordReset(app core.App, authRecord *core.Record) error`. Mints a password-reset token, calls `resolveEmailTemplate` (M-11), constructs a `mailer.Message`, fires `OnMailerRecordPasswordResetSend`, and sends via the configured mailer.
- **Caller(s):** `apis/record_auth_password_reset_request.go:56` inside a `routine.FireAndForget` block; the route is registered for `POST /collections/{collection}/request-password-reset`.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — `html/template` Layout+Body render with dynamic placeholder map, optional `html2text` derivation, then SMTP round-trip. Mixed CPU + IO.
  - Load profile: yes — bursty on signup waves, password-reset storms, abuse spikes.
  - Coherent unit: yes — `(app, *Record) → error`. The `*Record` is a value with no live DB cursor at call time.
  - State independence: maybe — the function takes a `core.App` and uses it to mint a token (DB read) and to fetch the mailer client; clean interface, but a remote replica needs an `App` shim, or the function must be split so the token mint stays local and only the render+send is lifted.
  - Latency / failure: yes — caller is `routine.FireAndForget`; errors are logged.
- **Activation shape:** Goroutine via `routine.FireAndForget` from a request handler, after the response is committed.
- **Confidence:** medium-high — would prefer to lift only the `(message *mailer.Message) → error` tail (M-7) unless the template render's compute is non-trivial in profile. Listed at this granularity because it's what the call site exposes.
- **Risk notes:** sibling `SendRecordOTP` calls `app.Save(otp)` to update `sentTo` — that branch must stay local. The token mint at the top of each function is a DB read.

### M-6: Bcrypt password hashing on record save

- **pick_provenance:** claude (1/3)
- **critique_status:** weak consensus — KEEP from codex, KEEP from gemini.
- **Region root:** `core/field_password.go:286` — `(*PasswordField).setValue(record *Record, raw any)`. Calls `bcrypt.GenerateFromPassword` at the field's configured `Cost` (default `bcrypt.DefaultCost`).
- **Caller(s):** invoked through the field-setter chain on every record set of a password field, e.g. `forms/record_upsert.go:115` via `record.SetIfFieldExists`; reachable from `apis/record_crud.go:32` (`POST /collections/{collection}/records`), the password-reset/email-change confirm paths, and `record.SetRandomPassword()` at `apis/record_auth_with_oauth2.go:344`.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — bcrypt at default cost ~50–100 ms of pure CPU.
  - Load profile: yes — registration bursts, bulk-import scripts (`apis/collection_import.go`), and password rotations all spike this; OAuth2 sign-up additionally calls `record.SetRandomPassword()` through the same setter.
  - Coherent unit: yes — pure: plaintext bytes + cost → `(hash, error)`; the `*Record` arg is only used to stash the result via `SetRaw`.
  - State independence: yes — no shared state read or written.
  - Latency / failure: maybe — synchronous on the request path, but bcrypt is already O(50–100 ms) so an extra hop is in the noise.
- **Activation shape:** Field-setter callback invoked from CRUD/auth HTTP handlers and JSVM bindings.
- **Confidence:** high — classic textbook lift target.
- **Risk notes:** the setter must produce a `PasswordFieldValue` that the caller then `SetRaw`s on the in-memory record; the lifted version must return both `Hash` and `LastError` so the local `Record` can be updated. Trivial to wrap, but it sits inside a field-setter dispatch table rather than at the top of a handler.

### M-7: SMTP send

- **pick_provenance:** claude (1/3)
- **critique_status:** weak consensus — KEEP from codex, KEEP from gemini. Codex notes claude overstates that "all callers are fire-and-forget" (the email-change confirm path is synchronous); the candidate still passes on the rubric's latency/failure criterion.
- **Region root:** `tools/mailer/smtp.go:62` — `(*SMTPClient).send(m *Message) error`. Builds a `mailyak` envelope, attaches files, generates a Message-ID, dials the SMTP server (optionally TLS), authenticates, and writes.
- **Caller(s):** wrapped at `tools/mailer/smtp.go:52` (`SMTPClient.Send`, which fires the `OnSend` hook); reached from each record-mailer wrapper at `mails/record.go:48,86,125,160,199,239`.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — full TCP+TLS handshake, SMTP DATA round trip, attachment streaming with mimetype detection. Latency-dominated, scales with attachment size.
  - Load profile: yes — bursty on signup waves, OTP campaigns, password-reset storms; per-tenant uneven (one workspace running an invite blast).
  - Coherent unit: yes — a single `*Message` value + a `*SMTPClient` whose state is host/port/credentials.
  - State independence: yes — config is immutable for the call's duration; no shared mutable in-process state. The `OnSend` hook is per-call.
  - Latency / failure: yes — most observed call sites wrap in `routine.FireAndForget`; the user-facing response is already returned before send completes.
- **Activation shape:** Goroutine launched via `routine.FireAndForget` from a request-event hook handler (most call sites).
- **Confidence:** high.
- **Risk notes:** the `OnSend` hook may be configured by user code (JS plugin or Go) for per-process side effects (rate limits, metrics). If the hook is registered in-process, lifting the SMTP send must either re-fire the hook on the local side or pass it through.

### M-8: S3 multipart object upload

- **pick_provenance:** codex (1/3)
- **critique_status:** weak consensus — KEEP from claude (claude self-noted missing this in own draft), KEEP from gemini.
- **Region root:** `tools/filesystem/internal/s3blob/s3/uploader.go:71` — `(*Uploader).Upload(ctx, optReqFuncs ...)` chooses single-object or multipart upload and drives the S3 request sequence (`multipartInit`/`multipartUpload`/`multipartComplete`/`multipartAbort` at `:181/:333/:260/:224`).
- **Caller(s):** `tools/filesystem/internal/s3blob/s3blob.go:374` runs the uploader behind the S3 writer; `tools/filesystem/filesystem.go:251` streams uploaded file contents into that writer (record-file uploads, backup uploads).
- **Why useful (rubric scoring):**
  - Compute envelope: yes — large payloads split into parts and uploaded concurrently, with XML completion and abort handling.
  - Load profile: yes — user file uploads and backup uploads are bursty and payload-sized, with one tenant able to dominate bandwidth.
  - Coherent unit: yes — the `Uploader` value owns S3 client, key, metadata, reader, and per-upload state for a single object.
  - State independence: yes — mutable state is per-upload (`uploadId`, part list, mutex); the durable side effect is the object in S3.
  - Latency / failure: maybe — uploads are often synchronous with record or backup operations, but they are already network-bound and have natural retry/abort behavior.
- **Activation shape:** Filesystem storage writer invoked from record file uploads and backup uploads.
- **Confidence:** high — classic payload-sized IO target.
- **Risk notes:** the API streams from an `io.Reader`; lifting it naively may require buffering, presigned handoff, or a remote-readable object source to avoid sending the same bytes through two network hops.

### M-9: Avatar download from OAuth2 provider

- **pick_provenance:** claude (1/3)
- **critique_status:** weak consensus — KEEP from codex, KEEP from gemini.
- **Region root:** `apis/record_auth_with_oauth2.go:468` — `safeFileFromURL(ctx context.Context, url string) (*filesystem.File, error)`. Builds a `safeHTTPClient` (loopback/private-IP guard via `net.Dialer.Control`), fetches the URL, limits to `DefaultMaxBodySize`, and constructs a `filesystem.File`.
- **Caller(s):** `apis/record_auth_with_oauth2.go:307` — invoked when the OAuth2 mapped avatar field is a file-type field; runs inline in `oauth2Submit`, which is itself inside a `RunInTransaction`.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — outbound HTTPS, body read up to `DefaultMaxBodySize`, mime detect on the resulting bytes inside `filesystem.NewFileFromBytes`.
  - Load profile: yes — same OAuth burst profile as M-2, only on first-login of new users.
  - Coherent unit: yes — `(ctx, urlString) → (*File, error)`; only side effect is the outbound request.
  - State independence: yes — the `safeHTTPClient` is built per-call.
  - Latency / failure: yes — already wrapped in a 10 s `context.WithTimeout` (line 302); failure is logged and the avatar field is dropped, not fatal.
- **Activation shape:** Synchronous call inside an OAuth2 record-creation transaction.
- **Confidence:** medium — would change my mind if the surrounding `RunInTransaction` made it impractical to round-trip a separate replica from inside an open SQLite tx.
- **Risk notes:** lives inside `oauth2Submit`'s transaction — the outbound HTTP is on the critical path of a tx-holding goroutine, so lifting adds a network RTT to the time the SQLite write lock is held. Acceptable for OAuth (rare, multi-hundred-ms anyway) but sharper than it looks.

### M-10: Record relation expansion

- **pick_provenance:** claude+codex (2/3)
- **critique_status:** disputed — KEEP from claude (self) and codex; DROP from gemini on state-independence grounds. Aggregator (claude) sides with KEEP — see Discrepancies.
- **Region root:** `core/record_query_expand.go:34` — `(*BaseApp).ExpandRecords(records []*Record, expands []string, optFetchFunc ExpandFetchFunc) map[string]error`. Recursively walks an expand path (up to `maxNestedRels=6`), resolves the relation field (direct or `_via_` back-relation), runs DB queries for related records, indexes them by id, and assembles the `expand` tree on each input record.
- **Caller(s):** `apis/record_helpers.go:355` — `app.ExpandRecords(records, expands, expandFetch(app, requestInfo))` inside `defaultEnrichRecords`; called from `recordsList` (`apis/record_crud.go:100`), `recordView` (`apis/record_crud.go:199`), `recordCreate`/`recordUpdate` (`apis/record_crud.go:355,494`), and the realtime broadcast path.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — recursive DB queries (one per relation level), JSON-aware indexing/joining of relations, plus `JSONEach` SQL for back-relations; loops scale with `len(records) × len(relIds)`.
  - Load profile: yes — list endpoints with `expand=` are the common dashboard/UI pattern; per-tenant uneven (one big `?perPage=200&expand=author.team.org` query dominates).
  - Coherent unit: maybe — public method on `*BaseApp` with three explicit args; the `ExpandFetchFunc` callback is interface-typed and the closure passed by `expandFetch` reaches into permission checks (`apis/record_helpers.go:375`).
  - State independence: maybe — uses `app.ConcurrentDB()` for queries; no shared mutable in-process state, but the SQLite handle itself is process-local.
  - Latency / failure: yes — synchronous inside the response path, but each list response with expansion is already O(100 ms); failure returns `failed` map and is logged but does not break the response.
- **Activation shape:** Synchronous call inside the response-finalization path of CRUD HTTP handlers and the realtime broadcast worker.
- **Confidence:** medium — the right granularity is probably `defaultEnrichRecords` rather than `ExpandRecords` itself, because the permission-checking `expandFetch` closure holds the request-scoped `RequestInfo`.
- **Risk notes:** the `ExpandFetchFunc` closure captures `*RequestInfo` (auth state) and the field resolver, so any lift must serialize that context. Not a deal-breaker, but the function-value-as-callback hides what otherwise looks like a clean signature.

### M-11: Email template resolution (sub-cut)

- **pick_provenance:** gemini (1/3)
- **critique_status:** disputed weak consensus — KEEP from claude (cleaner sub-cut than the full record-mailer wrapper, no in-process side effects); DROP from codex (compute envelope too small, structurally worse than the higher-level wrapper). Aggregator includes — see Discrepancies.
- **Region root:** `mails/record.go:251` — `resolveEmailTemplate(app, record, ...)` resolves system and record placeholders in email templates and wraps them in the layout via `html/template`.
- **Caller(s):** `mails/record.go:48,86,125,160,199` — invoked by each of the five record-mailer wrappers (`SendRecordAuthAlert`, `SendRecordOTP`, `SendRecordPasswordReset`, `SendRecordVerification`, `SendRecordChangeEmail`).
- **Why useful (rubric scoring):**
  - Compute envelope: maybe — placeholder substitution, HTML escaping, layout render via `html/template`; modest by itself but aggregates with the SMTP send tail in M-7 and the wrapper in M-5.
  - Load profile: yes — same bursty profile as M-7 (signup waves, password-reset storms).
  - Coherent unit: yes — pure-ish: `(app, record, template, placeholders) → (subject, body)`; only state read is `app.Settings()`.
  - State independence: yes — no in-process side effects beyond the `app.Settings()` read.
  - Latency / failure: yes — typically inside `routine.FireAndForget` flows; SMTP latency dominates downstream.
- **Activation shape:** Pure-function helper invoked from request-triggered goroutines (mailer event handlers).
- **Confidence:** medium — clean cut, but compute envelope is the weakest of any kept candidate; only worth lifting if profiled to be material.
- **Risk notes:** minimal in isolation. The pragmatic case for keeping it as a separate candidate (rather than folding into M-5) is that it is the only piece of the record-mailer pipeline that is unambiguously state-independent.

### M-12: JavaScript hook execution

- **pick_provenance:** gemini (1/3)
- **critique_status:** disputed weak consensus — claude MODIFY ("real region, but compute envelope is operator-supplied — there is no in-tree workload that demonstrates meaningful CPU consumption"); codex DROP ("operator-supplied rather than evidenced by a specific production path; 'any framework hook point' is not a coherent lift region"). Aggregator includes with the operator-supplied caveat made explicit — see Discrepancies.
- **Region root:** `plugins/jsvm/binds.go:81` — anonymous JS-handler wrapper that dispatches into the `executors` pool; each executor holds a `goja.Runtime` per-call CPU sandbox.
- **Caller(s):** any framework hook point registered by user-supplied JS in `pb_hooks/` (e.g. `onRecordCreateRequest`, `onRecordAfterUpdateSuccess`).
- **Why useful (rubric scoring):**
  - Compute envelope: maybe (operator-dependent) — goja JS execution can be arbitrarily expensive, but the rubric requires demonstrable in-tree compute, which this region does not have because the workload is supplied by the operator's `pb_hooks/` scripts.
  - Load profile: yes — entirely dependent on user-defined logic; can be arbitrarily heavy and spiky.
  - Coherent unit: yes — wrapped in a generic executor that takes event data and returns results; the `executors` pool is sized exactly because the authors expect concurrent contention.
  - State independence: maybe — the `$app` interface must be shimmed; user JS may reach into local state via `$app` methods, but that is bounded by the binding surface.
  - Latency / failure: yes — users expect hooks to add overhead; many hook types (`After*Success`) are logically async.
- **Activation shape:** Event-driven hook execution dispatched from any registered hook point.
- **Confidence:** low — the candidate is structurally clean but only earns a place in the corpus if the corpus admits "regions whose cost is operator-supplied." Listed here so the merged set is honest about the shape rather than silent about it.
- **Risk notes:** requires serializing the `$app` and event objects across the network; the `$app` binding surface is broad enough that a faithful shim is non-trivial.

## Discrepancies

### `(*BaseApp).CreateBackup` — included as the inner `archive.Create` only

Codex (C-2) and gemini (C-2) both picked the outer `CreateBackup` wrapper at `core/base_backup.go:44`. Claude critiqued both as MODIFY: drop the wrapper, keep the inner `archive.Create`. The wrapper holds the SQLite write lock via `RunInTransaction(...)` at `core/base_backup.go:77` for the lift's full duration — that's a hard `no` on rubric criterion 4 (state independence). Codex's and gemini's own scoring marked state independence as `maybe`, gesturing at this very issue. **Aggregator sided with claude:** the wrapper is excluded; the inner `archive.Create` is kept as M-4 (which gemini had picked separately as C-8 and claude had as C-8).

### `ExpandRecords` (M-10) — included over gemini's DROP

Gemini DROPped `ExpandRecords` from both claude's draft and codex's draft on state-independence grounds (tight coupling to the local SQLite handle and request-scoped fetch closures). Claude and codex both KEEP it (with `maybe` scoring on criteria 3 and 4 acknowledging the same concern). **Aggregator (claude) sides with KEEP** based on rubric criterion 4: the rubric defines a state-independence failure as "pervasive in-process mutable state" — `ExpandRecords` reads through `app.ConcurrentDB()`, which is a clean DB-handle interface, not in-process mutable state. The `ExpandFetchFunc` closure captures `*RequestInfo`, but that is a serialization concern, not pervasive shared state. The same critique would disqualify nearly every read-heavy region in PocketBase, which is a sign the criterion is being applied too broadly. Held at medium confidence to reflect the legitimate friction.

### `resolveEmailTemplate` (M-11) — included over codex's DROP

Codex DROPped this on compute-envelope grounds (too small to justify a remote hop). Claude KEEPs it as a cleaner sub-cut than the full record-mailer wrapper. **Aggregator includes with low-medium confidence**, citing rubric criterion 3 (coherent unit) and criterion 4 (state independence): the function is unambiguously pure-ish over its inputs, with the only state read being `app.Settings()`. The compute-envelope concern is real and is reflected in the `maybe` scoring. The candidate stays in the corpus as a disputed weak pick rather than being silently dropped, because it's the only state-independent slice of the mailer pipeline.

### JavaScript hook execution (M-12) — included over codex's DROP, with claude's MODIFY caveat

Codex DROPped on grounds that the workload is operator-supplied and "any framework hook point" is not a coherent lift region. Claude MODIFY-ed to keep with the explicit caveat that compute envelope is operator-dependent. Gemini's draft and OVERLOOKED both included it with a straight `yes` on compute envelope. **Aggregator includes** because the structural shape (per-call sandboxed `goja.Runtime`, executors pool sized for contention) is unambiguously a lift target *if* the corpus admits operator-supplied workloads. Held at low confidence to reflect that this is conditional on corpus-scope decisions outside the rubric.

## Excluded candidates

- **codex C-2 / gemini C-2 — `(*BaseApp).CreateBackup` wrapper at `core/base_backup.go:44`.** Wrapped in `RunInTransaction(...)` at `core/base_backup.go:77` for the lift's full duration; cleaner cut is the inner `archive.Create`, kept as M-4.
- **codex C-4 — `(*FileField).processFilesToUpload` at `core/field_file.go:512`.** Both critics MODIFY toward a different region; thin loop over `fsys.UploadFile` with interceptor-coupling to record persistence. Real lift is the lower-level S3 uploader, kept as M-8.
- **codex C-7 — `(*BaseApp).ImportCollections` at `core/collection_import.go:36`.** Both critics DROP. Runs entirely inside `app.RunInTransaction(...)` writing to the live SQLite handle; rare admin-triggered (criterion 2 weak); destructive cascades on `deleteMissing` (criterion 4 hard fail).
- **codex C-8 — `(*search.Provider).ParseAndExec` at `tools/search/provider.go:363`.** Both critics DROP. Carries a live `dbx.SelectQuery` and a `Resolver` bound to the in-process SQLite handle; the work *is* "run SQL against the local DB."
- **gemini C-6 — `(*AppleClientSecretCreate).Submit` at `forms/apple_client_secret_create.go:63`.** Both critics DROP on criterion 2 (load profile). Fires only when an admin updates Apple OAuth settings; sub-ms ECDSA signing of one JWT.
- **claude C-10 — `WriteFunc` closure at `core/base.go:1430`.** Both critics DROP, and claude self-acknowledged the disqualifier in the original draft. Writes through `app.AuxRunInTransaction` to the local SQLite logs DB; criterion 4 hard fail.
