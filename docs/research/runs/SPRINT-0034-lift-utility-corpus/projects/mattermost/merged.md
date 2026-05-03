# mattermost — merged lift-region candidate set (Phase 2b final)

## Methodology note

Phase 1 drafts: claude, codex, gemini.
Phase 2a critics: each model reviewed the other two.
Phase 2b aggregator: claude (opus).

Inclusion rules are applied deterministically per `PHASE2-PLAN.md` §"Inclusion rules". MODIFY corrections from critiques (line-cite drift, scope narrowing) are applied before merging. Where the rules require aggregator judgment (Rule 3 disputed cases, Rule 5 active defense, Rule 7 single-critic OVERLOOKED), the reasoning is made explicit and grounded in the rubric.

## Merged candidates (ranked strongest → weakest)

### M-1: Document text extraction

- **pick_provenance:** claude+codex+gemini (3/3) — claude C-1 anchored at the inner pure function `docextractor.Extract`; codex C-1 and gemini C-2 anchored at the App wrapper `ExtractContentFromFileInfo`.
- **critique_status:** KEEP from all 3 critics. Both anchors are defensible; the merged entry records both seams.
- **Region root:** `evaluation/mattermost/server/platform/services/docextractor/docextractor.go:21` — `Extract(logger, filename, r io.ReadSeeker, settings)` is the pure-compute boundary; the App-wrapper seam is `evaluation/mattermost/server/channels/app/file.go:1624` — `App.ExtractContentFromFileInfo`. The inner extractor is the lift-friendliest anchor (no `*App`, no DB); the wrapper is the natural call site to instrument.
- **Caller(s):** `evaluation/mattermost/server/channels/app/file.go:861` (post-upload `GoBuffered` goroutine) and `evaluation/mattermost/server/channels/jobs/extract_content/worker.go:67` (`extract_content` job worker, batches up to 1000 file infos per pass).
- **Why useful (rubric scoring):**
  - Compute envelope: **yes** — opens the file, runs the document/PDF/archive/plaintext extractor chain (recursive archive walks, multi-MB JSON parse paths) and returns bounded text.
  - Load profile: **yes** — bursty per-upload (one customer dumps a tarball of PDFs); the job worker batches over `FileInfo` rows since a timestamp.
  - Coherent unit: **yes** at the inner anchor (`(logger, filename, ReadSeeker, ExtractSettings)` only); **maybe** at the wrapper (uses `App.FileReader` / `Store().FileInfo().SetContent`).
  - State independence: **yes** — extractors are pure compute over the byte stream; persisted output is one `FileInfo.SetContent` call.
  - Latency / failure: **yes** — caller is a background job worker or post-upload goroutine; failure logs and skips.
- **Activation shape:** background `SimpleWorker` (`extract_content` job) and post-upload goroutine.
- **Confidence:** high — the only thing that would change my mind is if a particular extractor (e.g. the `mmpreview` HTTP extractor) turned out to dominate cost and was already a remote call, making the lift a no-op.
- **Risk notes:** `archive_extractor` recurses through nested archives (`evaluation/mattermost/server/platform/services/docextractor/archive.go:54`–`:105`); the extractor closure has a self-reference for nested archives. The remote replica must not read from the local Mattermost filestore directly — pass bytes (or a presigned URL) rather than a local-FS `ReadSeeker`. The wrapper's `Store().FileInfo().SetContent` should remain in-process.

---

### M-2: Image upload post-processing

- **pick_provenance:** claude+codex+gemini (3/3) — claude C-2 and codex C-2 anchored at `UploadFileTask.postprocessImage` (file.go:931); gemini C-1 originally at `generateThumbnailImage` (file.go:1184) but MODIFIED by both claude and codex to anchor at the wider fan-out at file.go:931.
- **critique_status:** KEEP from claude and codex; KEEP for gemini's pick after MODIFY (broaden from one of three preview legs to the full decode/orient/resize/encode fan-out).
- **Region root:** `evaluation/mattermost/server/channels/app/file.go:931` — `(t *UploadFileTask) postprocessImage(file io.Reader)` decodes the upload, fixes orientation, and fans out three goroutines: thumbnail (`evaluation/mattermost/server/channels/app/imaging/preview.go:16`), preview (`:28`), mini-preview (`:41`). The non-task variant `App.HandleImages` lives at `file.go:1139` with `generateThumbnailImage` / `generatePreviewImage` at `:1184` / `:1206`.
- **Caller(s):** `evaluation/mattermost/server/channels/app/file.go:846` (`UploadFileTask.Run`) and `evaluation/mattermost/server/channels/app/upload.go:318` (chunked uploads via `App.UploadData`).
- **Why useful (rubric scoring):**
  - Compute envelope: **yes** — JPEG/PNG decode, EXIF orientation correction, Lanczos resize, JPEG/PNG re-encode. Canonical CPU-bound calibrated positive from the rubric.
  - Load profile: **yes** — bursty around active hours and screenshot-heavy channels; mobile clients drive thumbnail demand.
  - Coherent unit: **yes** — `imaging.GeneratePreview(img image.Image, width int) image.Image` is pure; surrounding orchestrator needs `(image.Image, paths)` and a `WriteFile`-shaped sink.
  - State independence: **yes** — pure functional pipeline over bytes; `WriteFile` is the only side effect.
  - Latency / failure: **yes** — three preview goroutines run after the original bytes are persisted (`sync.WaitGroup` at `:984`/`:989`/`:994`), so user-visible upload latency does not gate on them; failure drops the preview/thumbnail.
- **Activation shape:** goroutines launched from the upload handler / upload-session completion.
- **Confidence:** high — calibrated positive example from the rubric.
- **Risk notes:** `t.imgEncoder` / `t.imgDecoder` are pooled bounded-memory wrappers (`evaluation/mattermost/server/channels/app/imaging/decode.go`); the lifted version needs its own pool so the bound is enforced replica-side. Output is written via `t.writeFile` / `a.WriteFile` (filestore handle) — pass bytes back rather than smuggling the filestore client. The `MiniPreview` mutation must be returned atomically enough for the caller's expectations.

---

### M-3: Outgoing webhook fan-out

- **pick_provenance:** claude+codex (2/3) — claude C-3 and codex C-4, both at `webhook.go:99`.
- **critique_status:** KEEP from gemini (the third critic).
- **Region root:** `evaluation/mattermost/server/channels/app/webhook.go:99` — `(a *App) TriggerWebhook(rctx, payload, hook, post, channel)` JSON-marshals the payload, fan-outs an HTTP POST per `hook.CallbackURLs`, optionally exchanges OAuth, and processes the response (Slack-text translation via `ProcessSlackText`, attachment normalization via `ProcessMessageAttachments`, response-post creation).
- **Caller(s):** `evaluation/mattermost/server/channels/app/post.go:684` (inside `App.handleWebhookEvents` from the post-create pipeline at `evaluation/mattermost/server/channels/app/webhook.go:37`); already off the post path via `Srv().Go` from `post.go:683`.
- **Why useful (rubric scoring):**
  - Compute envelope: **yes** — JSON marshal + outbound HTTPS per URL + Slack-format text/attachment processing on responses; aggregates well at fan-out scale.
  - Load profile: **yes** — bursty on chatty channels with registered hooks (CI bots, alerting, ChatOps).
  - Coherent unit: **yes/maybe** — payload and hook are POJOs and per-callback work is naturally per-URL; the response-post creation reaches `a.CreateWebhookPost` and `a.OutgoingOAuthConnections()`.
  - State independence: **maybe** — DB write through interface boundaries, no in-process pub/sub.
  - Latency / failure: **yes** — already async (each callback in its own goroutine inside a `WaitGroup`); failure logged and dropped.
- **Activation shape:** goroutine fan-out launched from the post-create pipeline.
- **Confidence:** high — calibrated positive from the rubric.
- **Risk notes:** `CreateWebhookPost` currently sits inside the same goroutine and re-enters the post-create path. Cleanest lift is to send only the request and return a `(text, attachments, props)` tuple, leaving response-post creation in-process. The method waits for all callback goroutines before returning (`webhook.go:200`), so a lift must preserve ordering and response-side effects.

---

### M-4: Elasticsearch bulk indexing

- **pick_provenance:** codex (1/3) — codex C-7 at `indexing_job.go:412`.
- **critique_status:** KEEP from claude (with the explicit note that this *supersedes* claude's own C-9 per-post `IndexPost` pick); KEEP from gemini (also notes superiority over claude C-9). Strong weak-consensus by virtue of unanimous critic endorsement.
- **Region root:** `evaluation/mattermost/server/enterprise/elasticsearch/common/indexing_job.go:412` — `(worker *IndexerWorker) BulkIndexPosts(posts []*model.PostForIndexing, progress IndexingProgress) (*model.Post, *model.AppError)` builds index names per post, skips unsupported types, converts posts to ES documents, JSON-marshals them, and enqueues bulk processor items (`addItemToBulkProcessor` at `:435`/`:440`); returns the last post for resumability.
- **Caller(s):** `evaluation/mattermost/server/enterprise/elasticsearch/common/indexing_job.go:337` (dispatches batches from the indexing worker) and `:390` (post-fetch invocation in the batch loop).
- **Why useful (rubric scoring):**
  - Compute envelope: **yes** — index-name computation, JSON marshal per post, bulk processor enqueue; cost scales with batch size.
  - Load profile: **yes** — indexing jobs repeatedly fetch configured-size batches until progress is complete; spiky during reindexes and steady backfill.
  - Coherent unit: **yes** — clean signature: posts slice + progress in, last-post + error out.
  - State independence: **maybe** — transformation is local; index naming uses config; bulk submission goes through an injected processor callback.
  - Latency / failure: **yes** — background indexing job with progress metadata and resumable last-IDs/timestamps.
- **Activation shape:** enterprise Elasticsearch indexing job worker (background).
- **Confidence:** high — batch-oriented, naturally async, cleanly bounded.
- **Risk notes:** Progress depends on the last post returned by the batch (`:403`/`:406`); a lift must preserve ordering and idempotent delete/index behavior. The bulk processor itself is a long-lived stateful flusher — keep it on the calling side and ship marshaled items, rather than lifting the processor.

---

### M-5: Bulk team export

- **pick_provenance:** claude+codex (2/3) — claude C-10 and codex C-6, both at `export.go:113`.
- **critique_status:** KEEP from gemini (the third critic).
- **Region root:** `evaluation/mattermost/server/channels/app/export.go:113` — `(a *App) BulkExport(rctx, writer, outPath, job, opts)` walks teams → channels → users → bots → posts → emoji and writes a JSONL stream into a zip; size scales with the entire workspace.
- **Caller(s):** `evaluation/mattermost/server/channels/jobs/export_process/worker.go:72` — `ExportProcess` job worker streams the writer through an `io.Pipe` into `WriteExportFileContext` (pipe wired at `:59`/`:61`).
- **Why useful (rubric scoring):**
  - Compute envelope: **yes** — JSONL serialization of every entity in the workspace, profile-picture pulls, attachment manifest construction; explicitly scales with workspace size; `exportAllPosts` pages parents 1000 at a time and resolves replies/followers.
  - Load profile: **maybe** — invoked periodically (admin or scheduled compliance export); fits the rubric's "periodic but heavy" bucket.
  - Coherent unit: **maybe** — entry takes `(writer, outPath, job, opts)`, but the body delegates to ~8 sub-`exportAll*` helpers that read from many stores; large internal closure.
  - State independence: **yes** — output goes to a writer; intermediate state lives in goroutine stack / pipe.
  - Latency / failure: **yes** — long-running background job; resumability via job state; pipe propagates write failures to the job.
- **Activation shape:** background `SimpleWorker` (`ExportProcess` job) with streaming pipe.
- **Confidence:** medium — strong on compute and async, weaker on coherent-unit because the work is spread across `exportAll*` helpers; lifting only `BulkExport` lifts the orchestrator while the helpers stay near the data.
- **Risk notes:** Profile-picture pulls call back into the filestore — in a remote replica the file backend must be reachable (S3/MinIO yes, repo-local FS no). Attachment export logs warnings rather than hard failures (`export.go:258`).

---

### M-6: Link-preview metadata fetch + parse

- **pick_provenance:** claude+gemini (2/3) — claude C-4 anchored at `getLinkMetadataForURL` (post_metadata.go:1021); gemini C-3 anchored at `getLinkMetadata` (`:892`) but MODIFIED by both claude and codex to narrow to `:1021` (so the LRU cache + DB cache lookup at `:902`/`:914` stays caller-side).
- **critique_status:** MODIFY from codex on claude's C-4 (corrects activation framing — caller is the synchronous post-prepare path `:270`/`:566`, not a background `Srv().Go`); MODIFY from claude+codex on gemini's C-3 (narrow root from `:892` to `:1021`). Both modifications applied below.
- **Region root:** `evaluation/mattermost/server/channels/app/post_metadata.go:1021` — `(a *App) getLinkMetadataForURL(rctx, requestURL)` performs the outbound HTTPS GET with content-type negotiation and configured timeout. Parser at `evaluation/mattermost/server/channels/app/post_metadata.go:1169` — `App.parseLinkMetadata` — dispatches to image-config decode (`parseImages` at `:1199`, with GIF frame counting and EXIF orientation) or HTML opengraph extraction. Cache reads/writes at `:902`/`:938` should remain caller-side.
- **Caller(s):** `evaluation/mattermost/server/channels/app/post_metadata.go:892` — `App.getLinkMetadata`, reached from the post-prepare path at `:270` via `:566`. The corrected understanding (per codex): post preparation is synchronous, though preview rendering on the post-create path is itself decoupled from delivery.
- **Why useful (rubric scoring):**
  - Compute envelope: **yes** — outbound HTTPS GET, HTML parse with goldmark/dyatlov-opengraph, image-config decode (GIF frame counting and EXIF orientation). Image config decode dominates for image links.
  - Load profile: **yes** — every post with a URL fans this out; trends/news links cause spikes.
  - Coherent unit: **yes** — `(requestURL string) -> (og, image, error)` is a clean signature.
  - State independence: **yes** — once the cache is moved caller-side, the lift is stateless. The DB save (`saveLinkMetadataToDatabase`) is write-through.
  - Latency / failure: **yes** — outbound HTTP is high-latency by nature; an extra hop is in the noise. Failure degrades the preview, not the post.
- **Activation shape:** post-prepare path; preview-fetching is offloaded behind the cache and tolerates failures.
- **Confidence:** high.
- **Risk notes:** `a.HTTPService().MakeClient(false)` returns a configured `http.Client` that respects allow-list rules — the lifted region needs the same allow-list passed in (don't smuggle `*App`). The `/api/v4/image` shortcut at `:1032` calls `a.ImageProxy().GetImageDirect`; either expose that as a callback or force the lifted version to take the same proxy interface.

---

### M-7: Slash command HTTP execution

- **pick_provenance:** codex (1/3) — codex C-10 at `command.go:521`.
- **critique_status:** KEEP from claude; KEEP from gemini.
- **Region root:** `evaluation/mattermost/server/channels/app/command.go:521` — `(a *App) DoCommandRequest(cmd, p url.Values)` constructs an integration request (with optional outgoing OAuth), performs the outbound HTTP call, reads a bounded body, and parses a `CommandResponse`.
- **Caller(s):** `evaluation/mattermost/server/channels/app/command.go:518` (inside `App.executeCustomCommand` at `:393`); the API4 entry is `evaluation/mattermost/server/channels/api4/command.go:357` (`/commands/execute`) and `:420`.
- **Why useful (rubric scoring):**
  - Compute envelope: **maybe** — mostly outbound HTTP latency, plus OAuth-token lookup, request construction, bounded body read, and response parsing.
  - Load profile: **yes** — bursty around workflows and incident response; each custom command hits an external integration.
  - Coherent unit: **yes** — `(cmd, url.Values) -> CommandResponse` is a clean contract.
  - State independence: **maybe** — OAuth-connection lookup and shared webhook HTTP client live on `App`, but both are interface-typed.
  - Latency / failure: **maybe** — synchronous request path, but command UX already absorbs the integration timeout.
- **Activation shape:** API4 slash-command request path (synchronous).
- **Confidence:** medium — clean contract, but external latency dominates rather than local CPU. Lift value is offloaded orchestration during command storms, not raw compute.
- **Risk notes:** The wider caller (`App.executeCustomCommand`) does channel/team/user lookups, mention expansion, and response-URL creation — lifting only `DoCommandRequest` is the safer seam.

---

### M-8: Push notification fan-out

- **pick_provenance:** codex (1/3) — codex C-11 at `notification_push.go:93`.
- **critique_status:** KEEP from claude; KEEP from gemini.
- **Region root:** `evaluation/mattermost/server/channels/app/notification_push.go:93` — `(a *App) sendPushNotificationToAllSessions(...)` runs plugin hooks, loads mobile sessions, deep-copies messages, signs JWT claims per session, and posts to the push proxy.
- **Caller(s):** `evaluation/mattermost/server/channels/app/notification_push.go:70` (push-message build) and `:437` (sync send from the push notification hub).
- **Why useful (rubric scoring):**
  - Compute envelope: **yes** — per-session JWT signing is genuine in-process CPU at scale; deep-copy of messages compounds with session count.
  - Load profile: **yes** — message notifications fan out across recipients and sessions during active channel bursts.
  - Coherent unit: **maybe** — one user/message fan-out is coherent, but plugin hooks, session-store mutation (`RemoveDeviceForSession`), and push-proxy credentials are coupled.
  - State independence: **maybe** — sign/send loop is separable; device removal updates session props.
  - Latency / failure: **yes** — already pushed through a hub channel with semaphore-limited goroutines (`:413`/`:423`).
- **Activation shape:** push notification hub queue worker.
- **Confidence:** medium — operationally attractive but with several side-effect hooks.
- **Risk notes:** Plugin rejection and push-proxy remove-device responses are observable behavior; a lift must keep those side effects (and metrics) intact. The `RemoveDeviceForSession` callback should probably stay in-process.

---

### M-9: Recap channel processing

- **pick_provenance:** claude (1/3) — claude C-5 at `recap.go:185`.
- **critique_status:** KEEP from codex; KEEP from gemini. Weak consensus, but unanimous critic endorsement after the single pick.
- **Region root:** `evaluation/mattermost/server/channels/app/recap.go:185` — `(a *App) ProcessRecapChannel(rctx, recapID, channelID, userID, agentID)` fetches posts since `lastViewedAt`, enriches with usernames, calls `SummarizePosts` (LLM round-trip via the agents bridge), and persists the per-channel result.
- **Caller(s):** `evaluation/mattermost/server/channels/jobs/recap/worker.go:66` — invoked once per channel in the recap job's loop.
- **Why useful (rubric scoring):**
  - Compute envelope: **yes** — orchestrates an LLM call (`SummarizePosts` at `evaluation/mattermost/server/channels/app/summarization.go:39`) that is the dominant cost; aggregates per channel.
  - Load profile: **yes** — fan-out shape: a single user-triggered recap can spawn N parallel channel processings; bursty around morning login.
  - Coherent unit: **yes** — four string IDs in, `*RecapChannelResult` out.
  - State independence: **yes** — DB-backed (`Recap()`, `Channel()`, `Post()` stores) plus the agents bridge HTTP client.
  - Latency / failure: **yes** — runs inside the recap job worker; LLM call is already O(seconds), so a network hop is in the noise.
- **Activation shape:** background `SimpleWorker` (`Recap` job).
- **Confidence:** high.
- **Risk notes:** `SummarizePosts` reaches `a.ch.agentsBridge`, which holds the HTTP client to the LLM provider. The lift either re-creates the bridge replica-side (config in `model.Config`) or moves the bridge call to the caller. The compute being upstream at the LLM does not disqualify the lift — it just means the lift trades wall-clock for offloaded orchestration during morning recap spikes.

---

### M-10: Remote-cluster attachment transfer

- **pick_provenance:** codex (1/3) — codex C-3 at `sendfile.go:84`.
- **critique_status:** KEEP from claude (called out as a strong novel pick claude missed); KEEP from gemini.
- **Region root:** `evaluation/mattermost/server/platform/services/remotecluster/sendfile.go:84` — `(rcs *Service) sendFileToRemote(timeout, task)` opens the file (`task.rp.FileReader`), constructs an authenticated remote upload request, performs HTTP I/O, reads the response, and unmarshals `FileInfo`.
- **Caller(s):** `evaluation/mattermost/server/platform/services/remotecluster/sendfile.go:45` (`enqueueTask`) and `evaluation/mattermost/server/platform/services/remotecluster/send.go:45` (dispatches `sendFileTask` from the remote-cluster send loop).
- **Why useful (rubric scoring):**
  - Compute envelope: **yes** — open-file streaming + outbound HTTPS + response unmarshal; cost scales with attachment size.
  - Load profile: **yes** — shared-channel attachments can be large and fan out to multiple remote clusters.
  - Coherent unit: **yes** — `sendFileTask` carries the remote cluster, upload session, file info, reader provider, and callback (a clean envelope).
  - State independence: **maybe** — depends on remote credentials, file backend access, metrics, callbacks.
  - Latency / failure: **yes** — already queue-driven with timeout and callback error channel (`sendfile.go:53`/`:118`).
- **Activation shape:** remote-cluster queue worker.
- **Confidence:** high — already an isolated queue task with explicit I/O boundaries.
- **Risk notes:** Delivery semantics and callback timing matter; `sendFile` converts transport errors into a `Response` and calls the provided callback. The `task.rp.FileReader` must be addressable from the remote replica (presigned URL or byte-stream forwarding).

---

### M-11: Bulk import processing

- **pick_provenance:** codex (1/3) — codex C-5 originally at `BulkImportWithPath` (`import.go:218`); MODIFIED by claude to anchor at `bulkImport` at `:226` (the `:218` function is a one-line forwarder; the actual scan/work lives in `bulkImport`).
- **critique_status:** MODIFY from claude (corrected line cite, applied below); KEEP from gemini.
- **Region root:** `evaluation/mattermost/server/channels/app/import.go:226` — `(a *App) bulkImport(rctx, jsonlReader, attachmentsReader, dryRun, extractContent, workers, importPath)` scans JSONL with a buffered scanner, locks the store to master, fans out segment workers per record type, and batches post/direct-post imports.
- **Caller(s):** `evaluation/mattermost/server/channels/jobs/import_process/worker.go:126` (reads import job options) and `:136` (invokes via `App.BulkImportWithPath` with CPU-derived worker count).
- **Why useful (rubric scoring):**
  - Compute envelope: **yes** — large JSONL scan, zip attachment mapping, per-record unmarshal, segment workers, batched post imports.
  - Load profile: **yes** — import jobs process tenant-scale export payloads, including attachment upload and optional content extraction (`evaluation/mattermost/server/channels/app/import_functions.go:1682`).
  - Coherent unit: **maybe** — clear readers/options/line-error contract, but the body spans many domain import functions.
  - State independence: **maybe** — background job, but it locks store access to master (`:233`) and performs broad DB/file mutations.
  - Latency / failure: **yes** — already a job worker with line-specific error reporting.
- **Activation shape:** import job worker.
- **Confidence:** medium — expensive and job-shaped, but wide.
- **Risk notes:** Segment ordering and worker-drain points are correctness-critical. Lifting the whole import is easier than lifting only post batches because attachment upload and channel/user creation share state. The `LockToMaster` at `:233` is a hint that read replicas will not satisfy this path.

---

### M-12: Async per-recipient email render+send

- **pick_provenance:** codex (1/3 directly) — codex C-12 at `notification_email.go:144`. Gemini C-5 was at `email/notification_email.go:30` (`GetMessageForNotification`); codex MODIFY broadens that to the same `notification_email.go:144` target. Claude DROP on gemini's C-5 (as mischaracterized) but KEEP on codex's C-12 (treats it as distinct from his own C-6 batched-email pick).
- **critique_status:** KEEP from claude; KEEP from gemini. Weak consensus is supported by gemini's pick folding into the same target.
- **Region root:** `evaluation/mattermost/server/channels/app/notification_email.go:144` — `(a *App) sendNotificationEmail(...)` selects DM/GM team, runs plugin-customizable email content hooks, decides batching, embeds sender images, renders templates, and sends mail (or hands off to the batching path).
- **Caller(s):** `evaluation/mattermost/server/channels/app/notification.go:432` (preference check) and `:437` (`Srv().Go` async invocation from the notification fan-out).
- **Why useful (rubric scoring):**
  - Compute envelope: **maybe** — plugin-content build, template rendering, base64 image embedding, then SMTP submission.
  - Load profile: **yes** — notification bursts create many async email tasks for mentioned users.
  - Coherent unit: **maybe** — one notification/user/team maps to one email or batch entry, but plugin hooks and email-service state are involved.
  - State independence: **maybe** — template rendering and SMTP are separable; team lookup, preferences, and batch storage use server state.
  - Latency / failure: **yes** — fully background (`Srv().Go`), failure logged.
- **Activation shape:** async notification goroutine.
- **Confidence:** medium — useful as an isolation target; plugin-hook surface is the main wart.
- **Risk notes:** Template rendering pulls localized props and attachment rendering before `SendMailWithEmbeddedFiles` (`notification_email.go:347`/`:368`/`:395`); the lifted version needs the same templates and locale data. SMTP credentials live in config; replica needs them.

---

### M-13: Batched email notification render+send

- **pick_provenance:** claude (1/3) — claude C-6 at `email_batching.go:252`.
- **critique_status:** KEEP from codex; KEEP from gemini (gemini's critique acknowledges this as a better-scoped target than her own C-5 helper anchor). Distinct from M-12: M-13 is the *batched* path inside `email.Service`, M-12 is the unbatched per-recipient path. Claude's critique called out this distinction explicitly.
- **Region root:** `evaluation/mattermost/server/channels/app/email/email_batching.go:252` — `(es *Service) sendBatchedEmailNotification(userID, notifications)` looks up the recipient, fetches sender/channel/profile-image per pending post, renders the `messages_notification` template, and ships the resulting HTML (with embedded images) to SMTP.
- **Caller(s):** `evaluation/mattermost/server/channels/app/email/email_batching.go:161` — `EmailBatchingJob.checkPendingNotifications`, invoked every 30s by `EmailBatchingJob.handleNewNotifications` at `:141`.
- **Why useful (rubric scoring):**
  - Compute envelope: **yes** — markdown→email-safe HTML rendering, template execution, base64 image embedding, SMTP submission. Cost scales with batch size.
  - Load profile: **yes** — hot at end-of-period flushes; uneven per recipient (active-mention recipients accumulate the largest batches).
  - Coherent unit: **maybe** — bound to `email.Service` for `userService` / `store` / `config` / `templatesContainer`, but all interface-typed; no `*App` smuggling.
  - State independence: **yes** — input is `(userID, []*batchedNotification)`; output via SMTP. The batch-pending map lives in `EmailBatchingJob` (the framework, kept in-process); the per-call function does not mutate it.
  - Latency / failure: **yes** — fully background; failure logs and drops.
- **Activation shape:** background goroutine inside `EmailBatchingJob`.
- **Confidence:** medium-high — `Service` boundary already exists; main risk is template-container reload semantics under config changes.
- **Risk notes:** Profile-image bytes flow through `embeddedFiles map[string]io.Reader` and must be materialized before crossing the wire. SMTP credentials live in config; replica needs them.

---

### M-14: PBKDF2 password hashing

- **pick_provenance:** gemini (1/3) — gemini C-7 at `pbkdf2.go:151` (the `Hash` entry).
- **critique_status:** KEEP from claude (with explicit note that claude missed it and credits gemini). MODIFY from codex (extend coverage to the verify path at `pbkdf2.go:197`, reached via `authentication.go:65`/`:77`; downgrade load and latency/failure to "maybe" because login is synchronous and runs under login-attempt locking).
- **Region root:** `evaluation/mattermost/server/channels/app/password/hashers/pbkdf2.go:151` — `PBKDF2.Hash` for new-hash generation; the actually load-bearing call site under login is `pbkdf2.go:197` (`PBKDF2.Compare`/verify) reached from `evaluation/mattermost/server/channels/app/authentication.go:65`/`:77`. Hashing uses `pbkdf2.Key` at `:202` with `DefaultIterations = 600_000`, SHA-256.
- **Caller(s):** `evaluation/mattermost/server/channels/app/password/hashers/hashers.go:141` (top-level entry); login path enters via `evaluation/mattermost/server/channels/app/authentication.go:65`/`:77`.
- **Why useful (rubric scoring):**
  - Compute envelope: **yes** — intentionally CPU-heavy (600k SHA-256 iterations). Pure function.
  - Load profile: **maybe** — bursty during login storms, though usually rate-limited; codex correctly downgraded from yes to maybe given login-attempt locking.
  - Coherent unit: **yes** — password in, hash out; pure.
  - State independence: **yes** — fully stateless.
  - Latency / failure: **maybe** — hashing already takes O(100ms) so a hop is in the noise, but the path is synchronous and runs under login-attempt locking; codex correctly downgraded.
- **Activation shape:** synchronous on login / password-change paths.
- **Confidence:** high on rubric criterion 1 (canonical CPU-bound calibrated positive); medium overall after codex's load/latency downgrade.
- **Risk notes:** Very simple interface; risk is operational. The login-attempt mutex (`a.ch.ldapLoginAttemptsMut` and equivalents) sits *outside* the hash call, so the lift seam is preserved. Care must be taken not to leak credentials across the wire — the hop must be authenticated and TLS-protected.

---

### M-15: Slack workspace import

- **pick_provenance:** claude (1/3) — claude C-8 at `slackimport/slackimport.go:131`.
- **critique_status:** MODIFY from codex (narrow to the parse/convert lift only — zip walk and JSON parse start at `:131`/`:150`; mention/markup conversion is `:213`; object creation from `:217` and `InvalidateAllCaches` at `:226` should remain local or be represented as explicit action calls); KEEP from gemini.
- **Region root:** `evaluation/mattermost/server/platform/services/slackimport/slackimport.go:131` — `(si *SlackImporter) SlackImport(rctx, fileData multipart.File, fileSize, teamID)`. Per the codex MODIFY: the lift unit is the parse/convert pass (zip walk at `:150`, mention/markup conversion at `:213`); object creation at `:217` and `InvalidateAllCaches` at `:226` are explicit action callbacks that should stay in-process.
- **Caller(s):** `evaluation/mattermost/server/channels/api4/team.go:1510` (admin team-import endpoint) via `App.SlackImport` (`evaluation/mattermost/server/channels/app/slack.go:21`).
- **Why useful (rubric scoring):**
  - Compute envelope: **yes** — large zip walk, multi-MB JSON parses, per-post markup conversion, image upload helper (`PrepareImage` → preview/thumbnail generation for avatars).
  - Load profile: **maybe** — infrequent admin operation, but each call is huge and CPU-heavy; rubric "periodic but heavy" applies.
  - Coherent unit: **yes** — explicit `Actions` interface (`Actions struct` at `:86`) defines the entire dependency surface.
  - State independence: **yes** — `SlackImporter` is created fresh per call (`New` at `:112`); the `Actions` callbacks are the only side-effect surface.
  - Latency / failure: **yes** — admin POST handler, expected to be long; client polls progress.
- **Activation shape:** synchronous request from a long-running admin endpoint.
- **Confidence:** medium — defensible as "periodic but heavy"; the size of the `Actions` callback set means the remote replica needs RPC stubs back into the monolith.
- **Risk notes:** `InvalidateAllCaches` at `:226` reaches across the whole binary; do not move that callback. Per codex's MODIFY, the right model is to lift the parse/convert pass and stream "create channel" / "create post" requests back via `Actions`, rather than lifting the whole orchestrator.

---

### M-16: File search request execution

- **pick_provenance:** codex (1/3) — codex C-9 at `file.go:1445`.
- **critique_status:** KEEP from gemini; DROP from claude (same critique as post-search: the cost is in the search engine / SQL, the path is synchronous on a user-facing HTTP request). **Disputed (Rule 4 weak consensus, with one DROP).** Aggregator includes per Rule 4 (1 pick + at least one critic KEEP), but flags the dispute prominently — see Discrepancies below.
- **Region root:** `evaluation/mattermost/server/channels/app/file.go:1445` — `(a *App) SearchFilesInTeamForUser(...)` parses file search terms, resolves channel/user filters, runs file-info search (ES or SQL fallback), and filters inaccessible-file/channel-permission results.
- **Caller(s):** `evaluation/mattermost/server/channels/api4/file.go:954` (HTTP handler) and `:995` (invocation).
- **Why useful (rubric scoring):**
  - Compute envelope: **maybe** — term parsing, channel/user resolution, permission filtering; the heavy text search lives in the search engine. Less weak than post-search because file content searches over `FileInfo.Content` (i.e., text extracted by M-1) which can be very large.
  - Load profile: **maybe** — less frequent than post search, but content search over names + extracted content can be expensive.
  - Coherent unit: **maybe** — request terms and page inputs map to a `FileInfoList`; visibility checks couple to user/session permissions.
  - State independence: **maybe** — uses search-engine layer or SQL fallback; row fetch by ID.
  - Latency / failure: **maybe** — synchronous HTTP, but already tracked with search-duration metrics; somewhat tolerant.
- **Activation shape:** API4 HTTP file-search handler.
- **Confidence:** low — included on Rule 4 weak consensus, but with claude-critique reservations. The corpus benefits from a search-shaped candidate, and file search is a stronger pick than post search because the in-process work scales with extracted-content sizes (which can be tens of MB per file). If implementation later confirms the work is dominated by the engine call, demote in Phase 4.
- **Risk notes:** SQL fallback builds a large joined query with membership/channel/user/extension/date/tsquery filters; preserving query semantics is the hard part. Permissions and sanitization are correctness-critical for cross-channel results.

---

### M-17: LDAP user/group synchronization

- **pick_provenance:** OVERLOOKED by gemini (1/3 critics) — `App.SyncLdap` (gemini's nominated entry was `ldap.go:20`, which is the `Srv().Go` line; the actual heavy work lives behind `ldapI.StartSynchronizeJob` in the enterprise LDAP package).
- **critique_status:** Rule 7 — single-critic OVERLOOKED, included only because gemini's rubric scoring is unambiguous (5/5 yes). Aggregator notes this is a Rule 7 inclusion that warrants verification in Phase 4.
- **Region root:** `evaluation/mattermost/server/channels/app/ldap.go:18` — `(a *App) SyncLdap(rctx)` launches the sync via `Srv().Go` and calls `ldapI.StartSynchronizeJob(rctx, false)` (the heavy compute lives in the enterprise LDAP implementation behind that interface — not in the open-source `evaluation/mattermost` tree, so the actual job-body line cite is unverifiable from the present source tree).
- **Caller(s):** triggered by admin manual sync or scheduled job; the `Srv().Go` activation at `ldap.go:19` is the goroutine entry.
- **Why useful (rubric scoring, per gemini):**
  - Compute envelope: **yes** — heavy IO (LDAP queries) and CPU (diffing large result sets against internal Store, updating memberships).
  - Load profile: **yes** — periodic but extremely heavy; can dominate system resources during sync of large enterprise directories.
  - Coherent unit: **yes** — clean entry point; the job worker encapsulates the logic.
  - State independence: **yes** — reads from external LDAP and writes to durable stores.
  - Latency / failure: **yes** — fully background; failure logged and the job is retried by the scheduler.
- **Activation shape:** background job (launched via `Srv().Go`).
- **Confidence:** low — Rule 7 inclusion based on gemini's unambiguous scoring. The actual job body is in the enterprise package and was not verified against this corpus's source tree. Distinct from gemini's draft C-9 (`checkLdapUserPasswordAndAllCriteria`) which was excluded for state-independence violations.
- **Risk notes:** Requires LDAP configuration and credentials replica-side. The enterprise-package boundary is opaque from the open-source tree — the lift seam is `ldapI.StartSynchronizeJob`, but the implementation may carry hidden state (connection pool, sync mutex, progress maps) that would change the rubric scoring on criterion 4. Phase 4 must verify state-independence by reading the enterprise package.

---

## Discrepancies

### D-1: Post search (`SearchPostsForUser`)

- **Picks:** codex C-8 + gemini C-6 (2/3 drafts).
- **Critic verdicts:** claude DROP (both); codex critique on gemini's pick recommends DROP-by-alternative; gemini does not directly critique codex's pick beyond noting overlap with her own.
- **Rule 3 disputed.** Aggregator excludes.
- **Reasoning, grounded in the rubric:**
  - Criterion 1 (compute envelope): the heavy work is `Srv().Store().Post().SearchPostsForUser(...)` which dispatches into Elasticsearch or Postgres tsvector; the in-process Go work is param parsing, channel/user-name resolution, and three permission-filter passes (`filterInaccessiblePosts`, `FilterPostsByChannelPermissions`, `filterBurnOnReadPosts`). The Go function is a thin orchestrator — the rubric's "Negative" example ("a function that does only a single tiny DB read and returns a struct") generalizes here.
  - Criterion 5 (latency / failure): the path is the synchronous user-facing search request (`api4/post.go:982`); a user expects results in O(100 ms). Adding a network hop on this budget does not improve it.
  - Codex and gemini themselves rated this maybe/maybe on those criteria.
- **Side taken:** sided with claude. M-16 (file search) is included over M-post-search because file search's in-process work scales with extracted-content sizes from M-1, making criterion 1 stronger.

### D-2: File search (`SearchFilesInTeamForUser`)

- **Picks:** codex C-9 (1/3).
- **Critic verdicts:** gemini KEEP; claude DROP (same reasoning as post search).
- **Rule 4** — included with weak consensus annotation, but the dispute is recorded.
- **Reasoning:** included because M-16 has a marginally stronger compute envelope than post search (the function searches over `FileInfo.Content`, which is the text extracted by M-1 and can be tens of MB per file). Confidence marked low; if Phase 4 analysis confirms the work is dominated by the engine call, demote.
- **Side taken:** sided with gemini KEEP, but flagged the disagreement and lowered confidence.

### D-3: Link-preview activation framing

- **Picks:** claude C-4 + gemini C-3 (after MODIFY).
- **Critic verdicts:** codex MODIFY on claude's framing — claude wrote "background goroutine spawned from the post-create pipeline", but codex notes the caller is the synchronous post-prepare path (`post_metadata.go:270` via `:566`).
- **Resolution:** corrected the activation-shape sentence in M-6 to reflect codex's reading. The candidate itself remains; only the activation framing was off.
- **Side taken:** sided with codex on framing; the lift is still defensible because per-link metadata fetching is offloaded behind the cache and is failure-tolerant.

### D-4: Per-post indexing vs. bulk indexing

- **Picks:** claude C-9 (`IndexPost`, per-post, 1/3).
- **Critic verdicts:** codex DROP, gemini MODIFY (merge with codex's batch pick).
- **Resolution:** excluded claude C-9 in favor of M-4 (`BulkIndexPosts`). Claude's own draft conceded medium confidence on C-9 specifically due to the bulk-processor state issue (criterion 4 = no), so this is straightforward Rule 5.

---

## Excluded candidates

- **Claude C-7 (`HandleIncomingWebhook`, webhook.go:739):** 1 pick, both critics DROP. Coherent-unit failure — the heavy lifting is in `CreatePost`, not in the orchestrator. Rule 5.
- **Claude C-9 (`OpensearchInterfaceImpl.IndexPost`, opensearch.go:321):** 1 pick, both critics DROP/MODIFY in favor of M-4. Bulk-processor state failure on criterion 4. Rule 5.
- **Codex C-8 (`SearchPostsForUser`, post.go:2127) and gemini C-6 (same):** 2 picks, claude DROP. Rule 3 disputed; aggregator excludes per criteria 1 and 5 (see D-1).
- **Gemini C-4 (`utils.MarkdownToHTML`, markdown.go:57):** 1 pick, both critics DROP. Compute envelope failure (per-message conversion is sub-millisecond; the network hop dominates). Rule 5.
- **Gemini C-5 (`email.Service.GetMessageForNotification`, notification_email.go:30):** 1 pick. Claude DROP (mischaracterized — the function is i18n placeholder selection, not template rendering); codex MODIFY → broaden to `notification_email.go:144`. The MODIFY-broadened target is M-12; this specific helper anchor is excluded as covered.
- **Gemini C-8 (`App.GetAnalytics`, analytics.go:21):** 1 pick, both critics DROP. Compute envelope failure — work is in Postgres, the Go function aggregates rows. Rule 5.
- **Gemini C-9 (`checkLdapUserPasswordAndAllCriteria`, authentication.go:186):** 1 pick, both critics DROP. State-independence disqualifier — `a.ch.ldapLoginAttemptsMut` is a process-global mutex held across the LDAP round-trip (MM-37585). Rule 5.
