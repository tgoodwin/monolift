# Mattermost — useful lift regions (Phase 1, claude)

**Project read.** Mattermost server is a large Go monolith (~`server/channels/app` is the bulk of the business logic, plus `server/platform/services` for cross-cutting services and `server/channels/jobs` for cron/queue workers). It is HTTP-and-WebSocket-driven: chat posts, file uploads, integrations (incoming/outgoing webhooks), search, push/email notifications, exports, and an AI "recap" feature. Computationally expensive paths cluster in: (a) attachment handling — image decode/resize/encode and document text extraction; (b) integration fan-out — outgoing webhooks, HTTP-driven post actions, opengraph link previews; (c) per-message side effects on `CreatePost` — search-engine indexing, push notification building+dispatch, batched email rendering; (d) admin one-shots — Slack import, bulk export, bulk import; (e) AI summarization for the recap feature. Most of these already live behind a goroutine, a `Srv().Go(...)`, or a job worker, so the activation shape is already async and lift-friendly.

---

### C-1: Document text extraction (`docextractor.Extract`)

- **Region root:** `server/platform/services/docextractor/docextractor.go:21` — `Extract(logger, filename, r io.ReadSeeker, settings)` — runs PDF / Office / archive / plaintext extractors against an attachment and returns the text body.
- **Caller(s):** `server/channels/app/file.go:1624` — `App.ExtractContentFromFileInfo`, which is called from the `extract_content` job worker at `server/channels/jobs/extract_content/worker.go:71` and inline from upload paths.
- **Why useful (rubric scoring):**
  - Compute envelope: **yes** — parses PDFs, recurses through archives, decodes Word/Excel/etc.; size-bounded by `MaxFileSize` but routinely tens of MB per call.
  - Load profile: **yes** — bursty per-upload (one big customer dumps a tarball of PDFs); the `extract_content` job also batches over `FileInfo` rows since a timestamp.
  - Coherent unit: **yes** — `Extract` takes only `(logger, filename, ReadSeeker, ExtractSettings)`; no `*App`, no DB. The combined extractor is built fresh per call.
  - State independence: **yes** — wholly stateless; the extractors are pure compute over the byte stream. Persisted output is a single `FileInfo().SetContent` call by the caller.
  - Latency / failure: **yes** — caller is the `extract_content` job worker (background) or a post-upload goroutine. Failure path already just logs and skips.
- **Activation shape:** background job worker (`SimpleWorker`) and goroutine after upload.
- **Confidence:** high — would change my mind only if a particular extractor (e.g. the `mmpreview` HTTP one) turns out to dominate and is itself a remote call, making the lift a no-op.
- **Risk notes:** `archive_extractor` recurses; the extractor closure has a self-reference for nested archives. Need to handle the stream-from-S3 hand-off so the remote replica isn't reading the original Mattermost filestore directly — pass bytes (or a presigned URL) rather than the `ReadSeeker` over a local FS.

---

### C-2: Attachment image post-processing (`UploadFileTask.postprocessImage` / `App.HandleImages`)

- **Region root:** `server/channels/app/file.go:931` — `(t *UploadFileTask) postprocessImage(file io.Reader)` decodes the upload and fans out three goroutines: thumbnail, preview, mini-preview (`imaging.GenerateThumbnail`/`GeneratePreview`/`GenerateMiniPreviewImage` at `server/channels/app/imaging/preview.go:16`,`28`,`41`). The non-task variant `App.HandleImages` is at `server/channels/app/file.go:1139`, with `generateThumbnailImage` / `generatePreviewImage` at `:1184` / `:1206`.
- **Caller(s):** `server/channels/app/file.go:846` (`UploadFileTask.Run`) and `server/channels/app/upload.go:318` (chunked uploads via `App.UploadData`).
- **Why useful (rubric scoring):**
  - Compute envelope: **yes** — JPEG/PNG decode, Lanczos resize, JPEG/PNG re-encode; this is the canonical CPU-bound lift example from the rubric.
  - Load profile: **yes** — bursty around active hours and screenshot-heavy channels; mobile clients drive thumbnail demand.
  - Coherent unit: **yes** — `imaging.GeneratePreview(img image.Image, width int) image.Image` is pure; the surrounding orchestrator only needs `(image.Image, paths)` and a `WriteFile`-shaped sink.
  - State independence: **yes** — pure functional pipeline over bytes; `WriteFile` is the only side effect and goes to filestore (S3 / local FS abstracted).
  - Latency / failure: **yes** — already runs in a `sync.WaitGroup` of three goroutines after the bytes are persisted; failure just drops the preview/thumbnail.
- **Activation shape:** goroutines launched from the upload handler / upload-session completion.
- **Confidence:** high — calibrated positive example from the rubric.
- **Risk notes:** the `t.imgEncoder`/`t.imgDecoder` are pooled bounded-memory wrappers (see `server/channels/app/imaging/decode.go`); the lifted version would need its own pool so the bound is enforced replica-side. Output is written via `t.writeFile` / `a.WriteFile`, which is a filestore handle — pass the bytes back rather than smuggling the filestore client.

---

### C-3: Outgoing-webhook delivery (`App.TriggerWebhook`)

- **Region root:** `server/channels/app/webhook.go:99` — `(a *App) TriggerWebhook(rctx, payload, hook, post, channel)` JSON-marshals the payload, fan-outs an HTTP POST per `hook.CallbackURLs`, optionally exchanges OAuth, then processes the response (Slack-text translation, attachment normalization, response-post creation).
- **Caller(s):** `server/channels/app/post.go:684` (inside `App.handleWebhookEvents` from the post-create pipeline at `server/channels/app/webhook.go:37`).
- **Why useful (rubric scoring):**
  - Compute envelope: **yes** — JSON marshal + outbound HTTPS per URL + Slack-format `ProcessSlackText` / `ProcessMessageAttachments` on the response; aggregates well at fan-out scale.
  - Load profile: **yes** — bursty on chatty channels with one or more registered hooks (CI bots, alerting, ChatOps).
  - Coherent unit: **yes** — payload and hook are POJOs; the per-callback work is naturally per-URL.
  - State independence: **maybe** — the response handler calls back into `a.CreateWebhookPost` (DB write) and reads `a.OutgoingOAuthConnections()`; both can be expressed through interface boundaries. No in-process pub/sub.
  - Latency / failure: **yes** — already async (each callback is its own goroutine inside a `WaitGroup`); failure is logged and dropped.
- **Activation shape:** goroutine fan-out launched from the post-create pipeline.
- **Confidence:** high — calibrated positive example from the rubric.
- **Risk notes:** the response post creation (`CreateWebhookPost`) currently sits inside the same goroutine and re-enters the post-create path. A clean lift sends only the request and returns a `(text, attachments, props)` tuple to the caller, leaving the response post creation in-process.

---

### C-4: Link-preview metadata fetch + parse (`App.getLinkMetadataForURL` + `App.parseLinkMetadata`)

- **Region root:** `server/channels/app/post_metadata.go:1021` — `App.getLinkMetadataForURL(rctx, requestURL)` fetches the URL with a configured timeout and content-type negotiation; the parser at `:1169` (`App.parseLinkMetadata`) dispatches to image-config decode (`parseImages` at `:1199`) or HTML opengraph extraction.
- **Caller(s):** `server/channels/app/post_metadata.go:892` — `App.getLinkMetadata`, in turn called from the post-prepare path that cache-misses into a background `Srv().Go(...)` after `CreatePost`.
- **Why useful (rubric scoring):**
  - Compute envelope: **yes** — outbound HTTPS GET, HTML parse with goldmark/dyatlov-opengraph, image-config decode (with GIF frame counting and EXIF orientation). Image config decode dominates for image links.
  - Load profile: **yes** — every post with a URL fans this out; trends/news links cause spikes.
  - Coherent unit: **yes** — `(requestURL string) -> (og, image, error)` is a clean signature; the cache fetch can sit on the caller side.
  - State independence: **yes** — the LRU cache (`platform.LinkCache`) and DB save (`saveLinkMetadataToDatabase`) are write-through, replica-local-safe.
  - Latency / failure: **yes** — caller already runs this in a background goroutine after the post is persisted; failures degrade the preview, not the post.
- **Activation shape:** background goroutine spawned from the post-create pipeline.
- **Confidence:** high.
- **Risk notes:** the `a.HTTPService().MakeClient(false)` call returns a configured `http.Client` that respects allow-list rules — the lifted region needs the same allow-list passed in (don't smuggle `*App`).

---

### C-5: Recap channel processing (`App.ProcessRecapChannel`)

- **Region root:** `server/channels/app/recap.go:185` — `(a *App) ProcessRecapChannel(rctx, recapID, channelID, userID, agentID)` fetches posts since `lastViewedAt`, enriches with usernames, calls `SummarizePosts` (which round-trips to the LLM agent), and persists the per-channel result.
- **Caller(s):** `server/channels/jobs/recap/worker.go:66` — invoked once per channel in the recap job's loop.
- **Why useful (rubric scoring):**
  - Compute envelope: **yes** — orchestrates an LLM call (`server/channels/app/summarization.go:39`, `App.SummarizePosts`) that is the dominant cost and aggregates per channel.
  - Load profile: **yes** — fan-out shape: a single user-triggered recap can spawn N parallel channel processings; bursty around morning login.
  - Coherent unit: **yes** — four string IDs in, `*RecapChannelResult` out; explicit boundary.
  - State independence: **yes** — DB-backed (`Recap()`, `Channel()`, `Post()` stores) plus the agents bridge HTTP client; no in-process mutable state needed.
  - Latency / failure: **yes** — runs inside the recap job worker; LLM call is already O(seconds), so a network hop is in the noise.
- **Activation shape:** background `SimpleWorker` (`Recap` job).
- **Confidence:** high.
- **Risk notes:** `SummarizePosts` reaches `a.ch.agentsBridge` — that bridge holds the HTTP client to the LLM provider. The lift either re-creates the bridge replica-side (its config is in `model.Config`) or moves the bridge call to the caller. The "compute" being upstream at the LLM does not disqualify the lift — it just means the lift trades wall-clock for offloaded orchestration.

---

### C-6: Batched email notification render+send (`Service.sendBatchedEmailNotification`)

- **Region root:** `server/channels/app/email/email_batching.go:252` — `(es *Service) sendBatchedEmailNotification(userID, notifications)` looks up the recipient, fetches sender/channel/profile-image per pending post, renders the `messages_notification` template, and ships the resulting HTML (with embedded images) to the SMTP provider.
- **Caller(s):** `EmailBatchingJob.checkPendingNotifications` at `server/channels/app/email/email_batching.go:161` (invoked every 30s by `EmailBatchingJob.handleNewNotifications` at `:141`).
- **Why useful (rubric scoring):**
  - Compute envelope: **yes** — markdown→email-safe HTML rendering, template execution, base64 image embedding, then SMTP submission. Cost scales with batch size.
  - Load profile: **yes** — hot at end-of-period flushes; uneven per recipient (active mentioner gets the largest batches).
  - Coherent unit: **maybe** — the function is bound to the `email.Service` for `userService`/`store`/`config`/`templatesContainer`. All four are interface-typed; no `*App` smuggling.
  - State independence: **yes** — input is `(userID, []*batchedNotification)`; output goes via SMTP. The batch-pending map lives in `EmailBatchingJob` (the framework, kept in-process); the per-call function does not mutate it.
  - Latency / failure: **yes** — fully background (cron-style); failure logs and drops.
- **Activation shape:** background goroutine inside `EmailBatchingJob`.
- **Confidence:** medium-high — the `Service` boundary already exists; the only risk is template-container reload semantics under config changes.
- **Risk notes:** profile-image bytes flow through `embeddedFiles map[string]io.Reader` then the SMTP send — must be materialized before crossing the wire. SMTP credentials live in config; replica needs them.

---

### C-7: Incoming webhook ingestion (`App.HandleIncomingWebhook`)

- **Region root:** `server/channels/app/webhook.go:739` — `(a *App) HandleIncomingWebhook(rctx, hookID, req *model.IncomingWebhookRequest)` resolves the hook, the user, and the channel (with ad-hoc `@user`/`#channel` parsing), processes Slack-format text and message attachments, splits oversized posts, and creates the post(s).
- **Caller(s):** `server/channels/web/webhook.go:96` — invoked from the public `/hooks/{id}` HTTP route.
- **Why useful (rubric scoring):**
  - Compute envelope: **maybe** — JSON parse + Slack-text rewriting + attachment normalization + post split for oversized payloads; small for trivial alerts but meaningful for CI dumps and exception trackers.
  - Load profile: **yes** — bursty (CI failure storms, alerting hosts, deployment pipelines).
  - Coherent unit: **maybe** — entry point is clean (`hookID`, `*IncomingWebhookRequest`), but it then issues 3+ store reads and synchronously creates a post, dragging in the post-create pipeline.
  - State independence: **maybe** — no in-process mutable state; everything goes through stores. Coupling is to the post-create surface, which is a lot of code.
  - Latency / failure: **yes** — caller is an HTTP webhook with no user-facing latency budget; client just wants 200 OK.
- **Activation shape:** HTTP route handler (POST `/hooks/{id}`).
- **Confidence:** medium — strong load and latency case, weaker on coherent-unit because of the call into `CreatePost`.
- **Risk notes:** the easiest split is "parse + rewrite + validate" lifted, "post create" remains in-process; the natural seam is right after `req.Props`/`req.Attachments` rewriting and before the channel/user resolution that touches stores.

---

### C-8: Slack workspace import (`SlackImporter.SlackImport`)

- **Region root:** `server/platform/services/slackimport/slackimport.go:131` — `(si *SlackImporter) SlackImport(rctx, fileData multipart.File, fileSize, teamID)` opens the uploaded zip, parses `channels.json`/`users.json`/per-channel post files, converts mentions and markup, then walks every channel/user/post creating Mattermost objects via the `Actions` callback struct (`:86`).
- **Caller(s):** `server/channels/api4/team.go:1510` — invoked from the admin team-import endpoint via `App.SlackImport` (`server/channels/app/slack.go:21`).
- **Why useful (rubric scoring):**
  - Compute envelope: **yes** — large zip walk, multi-MB JSON parses, per-post markup conversion, image upload helper (`PrepareImage` → `GenerateThumbnailImage`/`GeneratePreviewImage` for avatars).
  - Load profile: **maybe** — infrequent admin operation, but each call is huge and CPU-heavy; fits the "periodic but heavy" bucket from the rubric.
  - Coherent unit: **yes** — explicit `Actions` interface (`Actions struct` at `:86`) defines the entire dependency surface.
  - State independence: **yes** — `SlackImporter` is created fresh per call (`New` at `:112`); the action callbacks are the only side-effect surface.
  - Latency / failure: **yes** — admin POST handler, expected to be long; client polls progress.
- **Activation shape:** synchronous request from a long-running admin endpoint.
- **Confidence:** medium — defensible as "periodic but heavy"; the size of the `Actions` callback set means the remote replica needs RPC stubs for ~10 callbacks back into the monolith.
- **Risk notes:** `InvalidateAllCaches` at `:226` reaches across the whole binary; do not move that callback. The right model is to lift the parse/convert pass and stream "create channel" / "create post" requests back, rather than lifting the whole orchestrator.

---

### C-9: Per-post search-engine indexing (`OpensearchInterfaceImpl.IndexPost`)

- **Region root:** `server/enterprise/elasticsearch/opensearch/opensearch.go:321` — `(os *OpensearchInterfaceImpl) IndexPost(post, teamId, channelType)` builds the index name, marshals the search-shaped post, and either enqueues into the bulk processor or hits the index endpoint synchronously.
- **Caller(s):** `server/channels/store/searchlayer/post_layer.go:37` — fired from a goroutine in the search-layer wrapper around `Post().Save`.
- **Why useful (rubric scoring):**
  - Compute envelope: **maybe** — JSON marshal + outbound to ES; the dominant cost is on the ES side, not Mattermost.
  - Load profile: **yes** — every post create / edit fires this; bursty with traffic.
  - Coherent unit: **yes** — already an interface method (`SearchEngineInterface.IndexPost`); inputs are POJOs.
  - State independence: **no** — `os.bulkProcessor` is a long-lived stateful indexer (and `os.mutex` guards reconfig). Lifting requires either pinning each remote replica to its own bulk processor (acceptable, but then "scaling" is just a thinner shim around ES) or always using the synchronous path.
  - Latency / failure: **yes** — already async, failure logged and dropped.
- **Activation shape:** background goroutine inside the search-layer store wrapper.
- **Confidence:** medium — kept for diversity (stateful-client lift case) and because the interface boundary is unusually clean.
- **Risk notes:** the `bulkProcessor` is the activation handoff for ES. If the lift ends up being "JSON marshal" only, that is too thin; if it includes the bulk processor, you have moved a stateful background flusher to the replica and will need to ensure flushes complete before the replica is killed by the autoscaler.

---

### C-10: Bulk team export (`App.BulkExport`)

- **Region root:** `server/channels/app/export.go:113` — `(a *App) BulkExport(rctx, writer, outPath, job, opts)` walks teams → channels → users → bots → posts → emoji and writes a JSONL stream into a zip; size scales with the entire workspace.
- **Caller(s):** `server/channels/jobs/export_process/worker.go:72` — the `ExportProcess` job worker streams the writer through an `io.Pipe` into `WriteExportFileContext`.
- **Why useful (rubric scoring):**
  - Compute envelope: **yes** — JSONL serialization of every entity in the workspace, plus profile-picture pulls and attachment manifest construction; explicitly scales with workspace size.
  - Load profile: **maybe** — invoked periodically (admin or scheduled compliance export). Not bursty in the upstream sense, but each call is very heavy and the rubric admits "periodic but heavy".
  - Coherent unit: **maybe** — the entry takes `(writer, outPath, job, opts)`, but it then calls ~8 sub-`exportAll*` methods that read from many stores; large internal dependency closure.
  - State independence: **yes** — output goes to a writer; intermediate state lives in goroutine stack / pipe.
  - Latency / failure: **yes** — already a long-running background job; resumability is built into job state.
- **Activation shape:** background `SimpleWorker` (`ExportProcess` job).
- **Confidence:** medium — strong on compute and async, weaker on coherent unit because the actual work is spread across `exportAll*` helpers; lifting just `BulkExport` lifts the orchestrator.
- **Risk notes:** profile-picture pulls call back into the filestore; in a remote replica the file backend must be reachable (S3 is, repo-local FS isn't — this disqualifies the local-FS deployment mode and only the S3/MinIO mode is liftable).

---

## Honest assessment

I'm most confident about **C-1 (`docextractor.Extract`)**, **C-2 (image post-processing)**, **C-3 (`TriggerWebhook`)**, and **C-4 (`getLinkMetadataForURL`)** — each is an existing async path with a clean argument-driven boundary, the work is unambiguously CPU- or IO-bound at scale, and the calibrated positive examples in the rubric map almost 1:1. **C-5 (`ProcessRecapChannel`)** and **C-6 (batched email)** are nearly as strong but a little more entangled with the `App` / `email.Service` surface. The genuinely marginal entries are **C-7 (`HandleIncomingWebhook`)** — strong load story but the post-create coupling makes the lift seam awkward — **C-9 (`IndexPost`)** — the bulk-processor state is a real wart, kept mostly because the interface is unusually clean — and **C-10 (`BulkExport`)** — orchestrator that is hard to lift without also lifting its eight sub-exporters; included because exports are exactly the "periodic but heavy" workload the rubric calls out. **C-8 (`SlackImport`)** sits between: defensible by the rubric, but the right architectural answer is probably to lift only the parse/convert pass and not the whole orchestrator.

The region I most suspect would be a great lift candidate but couldn't justify against the rubric is `App.SendNotifications` at `server/channels/app/notification.go:54` — it spawns 4–5 parallel store-fetch goroutines, computes mention sets, and fans out to email/push/websocket per recipient. The compute and load story are excellent, but the function is ~840 lines reaching into `*App`-bound services in a way that fails the "coherent unit" criterion. A refactor that extracted "compute mention recipients" as a pure function over `(post, channel, profileMap, channelMemberNotifyPropsMap, groups, followers)` would land it on this corpus comfortably; without that refactor, I had to leave it off.
