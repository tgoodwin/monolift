Project read: Mattermost is a large Go server with most heavy user-visible work concentrated in `server/channels/app`, background job workers, enterprise search indexing, and platform services such as remote clusters. The strongest lift regions are not the websocket or scheduler loops, but bounded units already reached through uploads, jobs, queues, or integration callbacks. I prioritized regions that have expensive file/image/search/index/network behavior and a naturally serializable request/response boundary. Several good candidates still carry a large `*App` dependency, so the practical lift boundary would need thin adapters around file storage, store access, config, metrics, and plugin hooks.

### C-1: File content extraction

- **Region root:** `evaluation/mattermost/server/channels/app/file.go:1624` — `App.ExtractContentFromFileInfo` extracts searchable text from one stored file and writes it back to `FileInfo.Content`.
- **Caller(s):** `evaluation/mattermost/server/channels/app/file.go:859` triggers extraction after upload via `GoBuffered`; `evaluation/mattermost/server/channels/jobs/extract_content/worker.go:67` invokes it from the extract-content job over batches of file infos.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — it opens the stored file, runs the document/PDF/archive/plain extractor chain, and persists bounded text (`evaluation/mattermost/server/channels/app/file.go:1630`, `evaluation/mattermost/server/platform/services/docextractor/docextractor.go:26`, `evaluation/mattermost/server/channels/app/file.go:1642`).
  - Load profile: yes — upload bursts and the extract-content worker can process up to 1000 candidate files per pass (`evaluation/mattermost/server/channels/jobs/extract_content/worker.go:60`).
  - Coherent unit: yes — the input is a single `FileInfo` plus file bytes, and the output is extracted text saved to one store row (`evaluation/mattermost/server/channels/app/file.go:1624`).
  - State independence: maybe — the work is pure extraction once the file reader is available, but the method reaches through `App` for file storage, config, store update, and cache reload (`evaluation/mattermost/server/channels/app/file.go:1630`, `evaluation/mattermost/server/channels/app/file.go:1646`).
  - Latency / failure: yes — upload-path extraction is already asynchronous and job-path failures are logged per file (`evaluation/mattermost/server/channels/app/file.go:861`, `evaluation/mattermost/server/channels/jobs/extract_content/worker.go:71`).
- **Activation shape (informational, not a selection criterion):** Upload continuation goroutine and extract-content job worker.
- **Confidence:** high — only hidden extractor dependencies, especially external document/PDF tooling, would change the ranking.
- **Risk notes:** The archive extractor writes a temp file, walks archive contents, and recursively extracts entries (`evaluation/mattermost/server/platform/services/docextractor/archive.go:54`, `evaluation/mattermost/server/platform/services/docextractor/archive.go:72`, `evaluation/mattermost/server/platform/services/docextractor/archive.go:105`), so remote execution must preserve file-size limits, temp-file behavior, and store/cache side effects.

### C-2: Image upload post-processing

- **Region root:** `evaluation/mattermost/server/channels/app/file.go:931` — `UploadFileTask.postprocessImage` decodes an uploaded image, fixes orientation, generates preview variants, and writes them to storage.
- **Caller(s):** `evaluation/mattermost/server/channels/app/file.go:774` starts `UploadFileX`; `evaluation/mattermost/server/channels/app/file.go:840` calls `postprocessImage` for uploaded images after the original file is stored.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — it decodes with memory bounds, rotates/uprights the image, resizes thumbnails/previews, and JPEG/PNG-encodes output (`evaluation/mattermost/server/channels/app/file.go:941`, `evaluation/mattermost/server/channels/app/file.go:949`, `evaluation/mattermost/server/channels/app/imaging/preview.go:28`, `evaluation/mattermost/server/channels/app/file.go:960`).
  - Load profile: yes — image uploads are bursty and cost scales with image size and preview count (`evaluation/mattermost/server/channels/app/file.go:980`).
  - Coherent unit: maybe — the core transformation is bounded to one file, but the receiver holds file paths, encoders, logger, writer, and mutable `FileInfo` fields (`evaluation/mattermost/server/channels/app/file.go:954`, `evaluation/mattermost/server/channels/app/file.go:1001`).
  - State independence: maybe — it can be expressed as input bytes to output variants, but the current code streams variant writes through the task's storage callback (`evaluation/mattermost/server/channels/app/file.go:972`).
  - Latency / failure: maybe — it runs synchronously inside the upload request path, so lifting reduces local CPU pressure but adds a hop to user-facing upload latency (`evaluation/mattermost/server/channels/app/file.go:846`).
- **Activation shape (informational, not a selection criterion):** HTTP file upload handler path.
- **Confidence:** high — the image-processing boundary is obvious; only upload latency policy would affect priority.
- **Risk notes:** The method launches three goroutines for thumbnail, preview, and mini-preview generation (`evaluation/mattermost/server/channels/app/file.go:984`, `evaluation/mattermost/server/channels/app/file.go:989`, `evaluation/mattermost/server/channels/app/file.go:994`), so remote execution must return all artifacts and the `MiniPreview` mutation atomically enough for the caller's expectations.

### C-3: Remote cluster attachment transfer

- **Region root:** `evaluation/mattermost/server/platform/services/remotecluster/sendfile.go:84` — `Service.sendFileToRemote` streams one attachment to a remote cluster and decodes the returned `FileInfo`.
- **Caller(s):** `evaluation/mattermost/server/platform/services/remotecluster/sendfile.go:45` enqueues file-send tasks; `evaluation/mattermost/server/platform/services/remotecluster/send.go:45` dispatches `sendFileTask` from the remote-cluster send loop.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — it opens the file, constructs an authenticated remote upload request, performs HTTP I/O, reads the response body, and unmarshals `FileInfo` (`evaluation/mattermost/server/platform/services/remotecluster/sendfile.go:98`, `evaluation/mattermost/server/platform/services/remotecluster/sendfile.go:110`, `evaluation/mattermost/server/platform/services/remotecluster/sendfile.go:121`, `evaluation/mattermost/server/platform/services/remotecluster/sendfile.go:137`).
  - Load profile: yes — shared-channel attachments can be large and fan out to remote clusters (`evaluation/mattermost/server/platform/services/sharedchannel/attachment.go:99`, `evaluation/mattermost/server/platform/services/sharedchannel/attachment.go:102`).
  - Coherent unit: yes — `sendFileTask` carries the remote cluster, upload session, file info, reader provider, and callback (`evaluation/mattermost/server/platform/services/remotecluster/sendfile.go:46`).
  - State independence: maybe — the send itself is external I/O, but it depends on remote credentials, file backend access, metrics, and callbacks (`evaluation/mattermost/server/platform/services/remotecluster/sendfile.go:115`, `evaluation/mattermost/server/platform/services/remotecluster/sendfile.go:86`, `evaluation/mattermost/server/platform/services/remotecluster/sendfile.go:79`).
  - Latency / failure: yes — the file send is already queue-driven with a timeout and callback error channel (`evaluation/mattermost/server/platform/services/remotecluster/sendfile.go:53`, `evaluation/mattermost/server/platform/services/remotecluster/sendfile.go:118`).
- **Activation shape (informational, not a selection criterion):** Remote-cluster queue worker.
- **Confidence:** high — this is already an isolated queue task with clear I/O boundaries.
- **Risk notes:** Delivery semantics and callback timing matter; `sendFile` converts transport errors into a `Response` and calls the provided callback (`evaluation/mattermost/server/platform/services/remotecluster/sendfile.go:57`, `evaluation/mattermost/server/platform/services/remotecluster/sendfile.go:78`).

### C-4: Outgoing webhook fan-out

- **Region root:** `evaluation/mattermost/server/channels/app/webhook.go:99` — `App.TriggerWebhook` sends an outgoing webhook payload to each callback URL and optionally creates response posts.
- **Caller(s):** `evaluation/mattermost/server/channels/app/webhook.go:37` selects relevant outgoing hooks for a post; `evaluation/mattermost/server/channels/app/post.go:682` invokes webhook handling asynchronously after post creation.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — it marshals payloads, loops callback URLs, retrieves optional outgoing OAuth tokens, posts HTTP requests, decodes limited JSON responses, and processes response attachments/text (`evaluation/mattermost/server/channels/app/webhook.go:105`, `evaluation/mattermost/server/channels/app/webhook.go:116`, `evaluation/mattermost/server/channels/app/webhook.go:139`, `evaluation/mattermost/server/channels/app/webhook.go:158`, `evaluation/mattermost/server/channels/app/webhook.go:227`).
  - Load profile: yes — it runs per matching post and fans out by callback URL (`evaluation/mattermost/server/channels/app/webhook.go:78`, `evaluation/mattermost/server/channels/app/webhook.go:116`).
  - Coherent unit: maybe — payload, hook, source post, and channel define the unit, but response-post creation brings in post services and permission-shaped behavior (`evaluation/mattermost/server/channels/app/webhook.go:194`).
  - State independence: maybe — outbound HTTP is independent, while OAuth token lookup and response post creation use `App` state (`evaluation/mattermost/server/channels/app/webhook.go:142`, `evaluation/mattermost/server/channels/app/webhook.go:194`).
  - Latency / failure: yes — activation is already off the post path via `Srv().Go`, and individual callback failures are logged without aborting post creation (`evaluation/mattermost/server/channels/app/post.go:683`, `evaluation/mattermost/server/channels/app/webhook.go:159`).
- **Activation shape (informational, not a selection criterion):** Async post-created integration fan-out.
- **Confidence:** high — the fan-out and timeout boundary are explicit.
- **Risk notes:** The method waits for all callback goroutines before returning (`evaluation/mattermost/server/channels/app/webhook.go:200`) and may create posts from remote responses, so a lift must preserve ordering and response-side effects well enough for integration users.

### C-5: Bulk import processing

- **Region root:** `evaluation/mattermost/server/channels/app/import.go:218` — `App.BulkImportWithPath` scans an import JSONL/zip payload and imports teams, channels, posts, direct posts, and attachments.
- **Caller(s):** `evaluation/mattermost/server/channels/jobs/import_process/worker.go:126` reads import job options; `evaluation/mattermost/server/channels/jobs/import_process/worker.go:136` calls `BulkImportWithPath` with CPU-derived worker count.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — it scans large JSONL lines, maps zip attachments, unmarshals import records, starts workers per segment, and batches post/direct-post imports (`evaluation/mattermost/server/channels/app/import.go:227`, `evaluation/mattermost/server/channels/app/import.go:241`, `evaluation/mattermost/server/channels/app/import.go:255`, `evaluation/mattermost/server/channels/app/import.go:309`, `evaluation/mattermost/server/channels/app/import.go:173`).
  - Load profile: yes — import jobs process tenant-scale export payloads, including attachment upload and optional content extraction (`evaluation/mattermost/server/channels/app/import_functions.go:1682`, `evaluation/mattermost/server/channels/app/import_functions.go:1687`).
  - Coherent unit: maybe — the job has clear readers/options and line-number errors, but the region spans many domain import functions (`evaluation/mattermost/server/channels/app/import.go:218`, `evaluation/mattermost/server/channels/app/import.go:316`).
  - State independence: maybe — it is a background job, but it locks store access to master and performs broad database/file mutations (`evaluation/mattermost/server/channels/app/import.go:233`).
  - Latency / failure: yes — it is already a job worker with line-specific error reporting (`evaluation/mattermost/server/channels/jobs/import_process/worker.go:136`, `evaluation/mattermost/server/channels/jobs/import_process/worker.go:138`).
- **Activation shape (informational, not a selection criterion):** Import job worker.
- **Confidence:** high — it is expensive and job-shaped, though wide.
- **Risk notes:** Segment ordering and worker drain points are part of correctness (`evaluation/mattermost/server/channels/app/import.go:277`, `evaluation/mattermost/server/channels/app/import.go:287`); lifting the whole import is easier than lifting only post batches because attachment upload and channel/user creation share state.

### C-6: Bulk export archive generation

- **Region root:** `evaluation/mattermost/server/channels/app/export.go:113` — `App.BulkExport` writes a Mattermost export stream or zip archive with teams, channels, users, posts, direct posts, files, emoji, and profile pictures.
- **Caller(s):** `evaluation/mattermost/server/channels/jobs/export_process/worker.go:59` wires a pipe to export-file storage; `evaluation/mattermost/server/channels/jobs/export_process/worker.go:72` calls `BulkExport`.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — it creates zip entries, exports multiple object classes, batches posts, and optionally copies attachment files (`evaluation/mattermost/server/channels/app/export.go:117`, `evaluation/mattermost/server/channels/app/export.go:149`, `evaluation/mattermost/server/channels/app/export.go:173`, `evaluation/mattermost/server/channels/app/export.go:196`).
  - Load profile: yes — admin export jobs can traverse tenant-scale history and file sets (`evaluation/mattermost/server/channels/app/export.go:720`, `evaluation/mattermost/server/channels/app/export.go:252`).
  - Coherent unit: maybe — the function has one writer/outPath/job/options contract, but internally spans many export helpers (`evaluation/mattermost/server/channels/app/export.go:113`, `evaluation/mattermost/server/channels/app/export.go:150`, `evaluation/mattermost/server/channels/app/export.go:174`).
  - State independence: maybe — it mostly reads durable store/file data, but progress updates and streaming writer lifetime are coupled to the job server (`evaluation/mattermost/server/channels/app/export.go:729`, `evaluation/mattermost/server/channels/jobs/export_process/worker.go:61`).
  - Latency / failure: yes — it is already invoked from an export job and writes through a pipe so failures propagate to the job (`evaluation/mattermost/server/channels/jobs/export_process/worker.go:62`, `evaluation/mattermost/server/channels/jobs/export_process/worker.go:72`).
- **Activation shape (informational, not a selection criterion):** Export job worker with streaming pipe.
- **Confidence:** medium — good lift value, but the writer/pipe contract makes the remote boundary less tidy than import.
- **Risk notes:** `exportAllPosts` pages parent posts 1000 at a time and builds replies/followers (`evaluation/mattermost/server/channels/app/export.go:720`, `evaluation/mattermost/server/channels/app/export.go:742`, `evaluation/mattermost/server/channels/app/export.go:747`), while attachment export reports warnings rather than hard failures (`evaluation/mattermost/server/channels/app/export.go:258`).

### C-7: Elasticsearch post bulk indexing

- **Region root:** `evaluation/mattermost/server/enterprise/elasticsearch/common/indexing_job.go:412` — `IndexerWorker.BulkIndexPosts` transforms post batches into Elasticsearch bulk index/delete operations.
- **Caller(s):** `evaluation/mattermost/server/enterprise/elasticsearch/common/indexing_job.go:337` dispatches index batches from the indexing worker; `evaluation/mattermost/server/enterprise/elasticsearch/common/indexing_job.go:390` calls `BulkIndexPosts` after fetching a post batch.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — it computes index names, skips unsupported post types, converts posts to ES documents, JSON-marshals them, and enqueues bulk processor items (`evaluation/mattermost/server/enterprise/elasticsearch/common/indexing_job.go:414`, `evaluation/mattermost/server/enterprise/elasticsearch/common/indexing_job.go:418`, `evaluation/mattermost/server/enterprise/elasticsearch/common/indexing_job.go:427`, `evaluation/mattermost/server/enterprise/elasticsearch/common/indexing_job.go:429`, `evaluation/mattermost/server/enterprise/elasticsearch/common/indexing_job.go:435`).
  - Load profile: yes — indexing jobs repeatedly fetch configured-size batches until progress is complete (`evaluation/mattermost/server/enterprise/elasticsearch/common/indexing_job.go:369`, `evaluation/mattermost/server/enterprise/elasticsearch/common/indexing_job.go:407`).
  - Coherent unit: yes — a posts slice and progress state produce bulk index operations plus last-post progress (`evaluation/mattermost/server/enterprise/elasticsearch/common/indexing_job.go:412`).
  - State independence: maybe — transformation is mostly local, but index naming uses config and bulk submission uses an injected processor callback (`evaluation/mattermost/server/enterprise/elasticsearch/common/indexing_job.go:414`, `evaluation/mattermost/server/enterprise/elasticsearch/elasticsearch/indexing_job.go:60`).
  - Latency / failure: yes — it is a background indexing job with progress metadata and resumable last IDs/timestamps (`evaluation/mattermost/server/enterprise/elasticsearch/common/indexing_job.go:297`, `evaluation/mattermost/server/enterprise/elasticsearch/common/indexing_job.go:307`).
- **Activation shape (informational, not a selection criterion):** Enterprise Elasticsearch indexing job worker.
- **Confidence:** high — this is batch-oriented and already abstracts bulk submission.
- **Risk notes:** Progress depends on the last post returned by the batch (`evaluation/mattermost/server/enterprise/elasticsearch/common/indexing_job.go:403`, `evaluation/mattermost/server/enterprise/elasticsearch/common/indexing_job.go:406`), so a lift must preserve ordering and idempotent delete/index behavior.

### C-8: Post search request execution

- **Region root:** `evaluation/mattermost/server/channels/app/post.go:2127` — `App.SearchPostsForUser` parses user search terms, runs store/search-engine lookup, and filters inaccessible results.
- **Caller(s):** `evaluation/mattermost/server/channels/api4/post.go:938` handles post-search HTTP requests; `evaluation/mattermost/server/channels/api4/post.go:982` calls `SearchPostsForUser`.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — it parses search params, converts channel/user filters, invokes search, filters inaccessible and burn-on-read posts, and returns membership metadata (`evaluation/mattermost/server/channels/app/post.go:2129`, `evaluation/mattermost/server/channels/app/post.go:2146`, `evaluation/mattermost/server/channels/app/post.go:2162`, `evaluation/mattermost/server/channels/app/post.go:2173`, `evaluation/mattermost/server/channels/app/post.go:2182`).
  - Load profile: maybe — user searches can be bursty and expensive on large tenants, especially when falling back to SQL goroutines per params list (`evaluation/mattermost/server/channels/store/sqlstore/post_store.go:2843`, `evaluation/mattermost/server/channels/store/sqlstore/post_store.go:2850`).
  - Coherent unit: maybe — inputs and outputs are clear, but permission filtering and client sanitization remain close to request/session state (`evaluation/mattermost/server/channels/api4/post.go:996`, `evaluation/mattermost/server/channels/api4/post.go:997`).
  - State independence: maybe — it reads search indexes or SQL and then store-backed post permissions (`evaluation/mattermost/server/channels/store/searchlayer/post_layer.go:171`, `evaluation/mattermost/server/channels/store/searchlayer/post_layer.go:179`).
  - Latency / failure: maybe — it is synchronous HTTP search, but already measured as a potentially slow request (`evaluation/mattermost/server/channels/api4/post.go:980`, `evaluation/mattermost/server/channels/api4/post.go:988`).
- **Activation shape (informational, not a selection criterion):** API4 HTTP search handler.
- **Confidence:** medium — the compute cost is real, but the user-facing synchronous path makes lift latency sensitive.
- **Risk notes:** The search layer may use Elasticsearch or SQL fallback (`evaluation/mattermost/server/channels/store/searchlayer/post_layer.go:195`, `evaluation/mattermost/server/channels/store/searchlayer/post_layer.go:210`), and permissions/sanitization are correctness-critical for cross-channel results.

### C-9: File search request execution

- **Region root:** `evaluation/mattermost/server/channels/app/file.go:1445` — `App.SearchFilesInTeamForUser` parses file search terms, queries file info search, and filters result visibility.
- **Caller(s):** `evaluation/mattermost/server/channels/api4/file.go:954` handles file-search HTTP requests; `evaluation/mattermost/server/channels/api4/file.go:995` calls `SearchFilesInTeamForUser`.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — it parses terms, resolves channel/user filters, searches file metadata/content, and filters inaccessible files/channel permissions (`evaluation/mattermost/server/channels/app/file.go:1446`, `evaluation/mattermost/server/channels/app/file.go:1460`, `evaluation/mattermost/server/channels/app/file.go:1476`, `evaluation/mattermost/server/channels/app/file.go:1487`, `evaluation/mattermost/server/channels/app/file.go:1491`).
  - Load profile: maybe — less frequent than post search, but content search over `FileInfo.Name` and `FileInfo.Content` can be expensive (`evaluation/mattermost/server/channels/store/sqlstore/file_info_store.go:648`, `evaluation/mattermost/server/channels/store/sqlstore/file_info_store.go:651`).
  - Coherent unit: maybe — request terms and page inputs map cleanly to a `FileInfoList`, but visibility checks bind it to user/session permissions (`evaluation/mattermost/server/channels/app/file.go:1445`, `evaluation/mattermost/server/channels/app/file.go:1491`).
  - State independence: maybe — it can use active search engines or SQL fallback and must fetch file rows by ID (`evaluation/mattermost/server/channels/store/searchlayer/file_info_layer.go:184`, `evaluation/mattermost/server/channels/store/searchlayer/file_info_layer.go:203`, `evaluation/mattermost/server/channels/store/searchlayer/file_info_layer.go:220`).
  - Latency / failure: maybe — it is synchronous HTTP but already tracked with search duration metrics (`evaluation/mattermost/server/channels/api4/file.go:993`, `evaluation/mattermost/server/channels/api4/file.go:1001`).
- **Activation shape (informational, not a selection criterion):** API4 HTTP file-search handler.
- **Confidence:** medium — worth considering after post search because it depends on content extraction and can hit text indexes.
- **Risk notes:** SQL fallback builds a large joined query with membership, channel, user, extension, date, and tsquery filters (`evaluation/mattermost/server/channels/store/sqlstore/file_info_store.go:535`, `evaluation/mattermost/server/channels/store/sqlstore/file_info_store.go:553`, `evaluation/mattermost/server/channels/store/sqlstore/file_info_store.go:656`), so preserving query semantics is the hard part.

### C-10: Custom slash command HTTP execution

- **Region root:** `evaluation/mattermost/server/channels/app/command.go:521` — `App.DoCommandRequest` sends a custom slash command request to an integration endpoint and parses the command response.
- **Caller(s):** `evaluation/mattermost/server/channels/app/command.go:393` finds and prepares a custom command; `evaluation/mattermost/server/channels/app/command.go:518` calls `DoCommandRequest` with the command form values.
- **Why useful (rubric scoring):**
  - Compute envelope: maybe — it is mostly outbound HTTP latency plus OAuth-token lookup, request construction, bounded body read, and response parsing (`evaluation/mattermost/server/channels/app/command.go:528`, `evaluation/mattermost/server/channels/app/command.go:545`, `evaluation/mattermost/server/channels/app/command.go:575`, `evaluation/mattermost/server/channels/app/command.go:586`, `evaluation/mattermost/server/channels/app/command.go:595`).
  - Load profile: yes — slash commands can spike around workflows or incident response, and each custom command hits an external integration (`evaluation/mattermost/server/channels/api4/command.go:357`, `evaluation/mattermost/server/channels/api4/command.go:420`).
  - Coherent unit: yes — command metadata plus URL values produce a `CommandResponse` or app error (`evaluation/mattermost/server/channels/app/command.go:521`).
  - State independence: maybe — it is almost a pure integration call, except outgoing OAuth connections and the shared webhook HTTP client live on `App` (`evaluation/mattermost/server/channels/app/command.go:528`, `evaluation/mattermost/server/channels/app/command.go:575`).
  - Latency / failure: maybe — the path is synchronous and command UX depends on configured integration timeout (`evaluation/mattermost/server/channels/app/command.go:522`, `evaluation/mattermost/server/channels/app/command.go:577`).
- **Activation shape (informational, not a selection criterion):** API4 slash-command request path.
- **Confidence:** medium — it has a clean contract, but external latency dominates rather than local CPU.
- **Risk notes:** The broader caller also performs channel/team/user lookups, mention expansion, and response URL creation (`evaluation/mattermost/server/channels/app/command.go:399`, `evaluation/mattermost/server/channels/app/command.go:494`, `evaluation/mattermost/server/channels/app/command.go:512`); lifting only `DoCommandRequest` is safest.

### C-11: Push notification fan-out

- **Region root:** `evaluation/mattermost/server/channels/app/notification_push.go:93` — `App.sendPushNotificationToAllSessions` applies plugin hooks, signs per-device push messages, and sends them to the push proxy.
- **Caller(s):** `evaluation/mattermost/server/channels/app/notification_push.go:70` builds a push message and calls the root; `evaluation/mattermost/server/channels/app/notification_push.go:437` invokes the sync send from the push notification hub.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — it runs plugin hooks, loads mobile sessions, deep-copies messages, signs JWT claims per session, and posts to the push proxy (`evaluation/mattermost/server/channels/app/notification_push.go:95`, `evaluation/mattermost/server/channels/app/notification_push.go:121`, `evaluation/mattermost/server/channels/app/notification_push.go:166`, `evaluation/mattermost/server/channels/app/notification_push.go:169`, `evaluation/mattermost/server/channels/app/notification_push.go:187`).
  - Load profile: yes — message notifications can fan out across recipients and sessions during active channel bursts (`evaluation/mattermost/server/channels/app/notification.go:569`, `evaluation/mattermost/server/channels/app/notification_push.go:151`).
  - Coherent unit: maybe — one user/message fan-out is coherent, but plugin hooks, session store mutation, and push proxy credentials remain coupled (`evaluation/mattermost/server/channels/app/notification_push.go:95`, `evaluation/mattermost/server/channels/app/notification_push.go:537`, `evaluation/mattermost/server/channels/app/notification_push.go:498`).
  - State independence: maybe — the core sign/send loop is separable, while device removal updates session props (`evaluation/mattermost/server/channels/app/notification_push.go:523`, `evaluation/mattermost/server/channels/app/notification_push.go:537`).
  - Latency / failure: yes — notifications are pushed through a hub channel with semaphore-limited goroutines (`evaluation/mattermost/server/channels/app/notification_push.go:241`, `evaluation/mattermost/server/channels/app/notification_push.go:413`, `evaluation/mattermost/server/channels/app/notification_push.go:423`).
- **Activation shape (informational, not a selection criterion):** Push notification hub queue worker.
- **Confidence:** medium — it is operationally attractive but has many side-effect hooks.
- **Risk notes:** Plugin rejection and push-proxy remove-device responses are observable behavior (`evaluation/mattermost/server/channels/app/notification_push.go:108`, `evaluation/mattermost/server/channels/app/notification_push.go:535`), so a lift must keep those side effects and metrics intact.

### C-12: Email notification rendering and send

- **Region root:** `evaluation/mattermost/server/channels/app/notification_email.go:144` — `App.sendNotificationEmail` builds, customizes, renders, batches, and sends one post email notification.
- **Caller(s):** `evaluation/mattermost/server/channels/app/notification.go:432` checks recipient email preferences; `evaluation/mattermost/server/channels/app/notification.go:437` invokes `sendNotificationEmail` asynchronously.
- **Why useful (rubric scoring):**
  - Compute envelope: maybe — it does team selection for DM/GM, builds plugin-customizable email content, decides batching, embeds sender images, renders templates, and sends mail (`evaluation/mattermost/server/channels/app/notification_email.go:148`, `evaluation/mattermost/server/channels/app/notification_email.go:173`, `evaluation/mattermost/server/channels/app/notification_email.go:207`, `evaluation/mattermost/server/channels/app/notification_email.go:226`, `evaluation/mattermost/server/channels/app/notification_email.go:237`, `evaluation/mattermost/server/channels/app/notification_email.go:258`).
  - Load profile: yes — notification bursts can create many async email tasks for mentioned users (`evaluation/mattermost/server/channels/app/notification.go:437`).
  - Coherent unit: maybe — one notification/user/team maps to one email or batch entry, but plugin hooks and email service state are involved (`evaluation/mattermost/server/channels/app/notification_email.go:177`, `evaluation/mattermost/server/channels/app/notification_email.go:218`).
  - State independence: maybe — template rendering and SMTP are separable, while team lookup, preferences, and batch storage use server state (`evaluation/mattermost/server/channels/app/notification_email.go:149`, `evaluation/mattermost/server/channels/app/notification_email.go:209`).
  - Latency / failure: yes — the caller and final send both run through server goroutines, and send failures are logged asynchronously (`evaluation/mattermost/server/channels/app/notification.go:437`, `evaluation/mattermost/server/channels/app/notification_email.go:257`).
- **Activation shape (informational, not a selection criterion):** Async notification goroutine.
- **Confidence:** medium — useful as an isolation target, less compelling than push because batching/plugin behavior is broader.
- **Risk notes:** Template rendering pulls localized props and attachment rendering before `SendMailWithEmbeddedFiles` (`evaluation/mattermost/server/channels/app/notification_email.go:347`, `evaluation/mattermost/server/channels/app/notification_email.go:368`, `evaluation/mattermost/server/channels/app/notification_email.go:395`), so lifted execution needs the same templates and locale data.

Honest assessment: The best phase-1 lift targets in Mattermost are file extraction, image post-processing, remote file transfer, webhook fan-out, import/export, and Elasticsearch indexing because they are already asynchronous or job-shaped and have obvious expensive work. Search, slash commands, push, and email are still useful but more marginal because they are either user-synchronous or entangled with plugin/session/permission behavior. I would avoid lifting low-level store helpers, websocket broadcast internals, and scheduler loops in this corpus because their boundaries are either too broad or too latency-sensitive. The main practical challenge across the good candidates is not finding compute; it is carving a small adapter over `App` state without accidentally changing durable side effects or delivery semantics.
