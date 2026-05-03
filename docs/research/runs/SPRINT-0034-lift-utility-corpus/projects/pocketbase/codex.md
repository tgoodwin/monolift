Project read: PocketBase is a Go backend that combines an HTTP API, SQLite-backed collection records, auth flows, file storage, backups, mail helpers, and optional plugins. The computationally useful work clusters around payload-sized operations: image thumbnailing, file and S3 uploads, zipped backups, record search/filter/expand paths, and outbound auth/OAuth work. Most of the server surface is request-handler driven, but some heavy paths already run behind admin actions or background goroutines, which makes them better lift candidates than routing, cron, or realtime connection infrastructure. I avoided the mux, scheduler, realtime stream handlers, test-only forms, and generic initialization paths, and treated broad `core.App` access as a practical lift risk even when the enclosed work is useful.

### C-1: Image thumbnail generation

- **Region root:** `evaluation/pocketbase/tools/filesystem/filesystem.go:489` — `(*filesystem.System).CreateThumb` decodes an original image, resizes/crops it, encodes it, and stores the thumbnail.
- **Caller(s):** `evaluation/pocketbase/apis/file.go:171` creates a thumbnail on a file download cache miss; `evaluation/pocketbase/apis/file.go:225` delegates the actual work to `fsys.CreateThumb`.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — the method does payload-proportional image decode, resize/fill/fit, encode, and storage write (`evaluation/pocketbase/tools/filesystem/filesystem.go:512`, `evaluation/pocketbase/tools/filesystem/filesystem.go:521`, `evaluation/pocketbase/tools/filesystem/filesystem.go:569`).
  - Load profile: yes — thumbnail work is triggered only for requested sizes that do not already exist, so it spikes with image uploads, cache misses, and image-heavy tenants.
  - Coherent unit: yes — the contract is a filesystem receiver plus original key, thumbnail key, and size string, with a single durable side effect.
  - State independence: yes — it uses the filesystem abstraction and per-call readers/writers, not process-local mutable caches.
  - Latency / failure: maybe — it is on an HTTP download path, but the caller already falls back to the original file on thumbnail errors (`evaluation/pocketbase/apis/file.go:171`).
- **Activation shape (informational, not a selection criterion):** HTTP file download cache-miss subcall.
- **Confidence:** high — this is the cleanest payload-sized CPU region; only very small thumbnails or universally warm caches would weaken it.
- **Risk notes:** The current caller uses in-process singleflight and a semaphore before entering `CreateThumb`; a lifted version would need equivalent duplicate suppression or accept redundant thumbnail generation.

### C-2: Backup archive creation

- **Region root:** `evaluation/pocketbase/core/base_backup.go:44` — `(*BaseApp).CreateBackup` archives `pb_data`, checkpoints databases, and uploads the zip to the configured backup filesystem.
- **Caller(s):** `evaluation/pocketbase/apis/backup_create.go:30` invokes it from the backup create API; `evaluation/pocketbase/plugins/ghupdate/ghupdate.go:237` invokes it during self-update when backups are enabled.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — it runs SQLite checkpoints, zips the app data directory via `archive.Create`, then uploads the resulting file (`evaluation/pocketbase/core/base_backup.go:77`, `evaluation/pocketbase/core/base_backup.go:84`, `evaluation/pocketbase/core/base_backup.go:109`).
  - Load profile: maybe — backups are usually manual/admin-triggered, but cost varies sharply with tenant data size and can collide with operational events like upgrades.
  - Coherent unit: maybe — the method has a clear `context,name -> error` contract, but it reaches through `BaseApp` for data paths, transactions, events, settings, and filesystem construction.
  - State independence: maybe — the durable result is a zip in backup storage, but the implementation depends on local `DataDir()` visibility and temporarily blocks writes with DB transactions.
  - Latency / failure: yes — backup creation is an admin/background-style operation where seconds or minutes of latency and explicit failure are acceptable.
- **Activation shape (informational, not a selection criterion):** Admin API handler or CLI/plugin update subtask.
- **Confidence:** high — this is expensive, bounded, and operationally separable; shared local disk assumptions are the main caveat.
- **Risk notes:** Remote execution would need access to the same `pb_data` contents or a virtualized snapshot, and write-blocking transaction semantics are easy to break if the archiver moves too far from the database owner.

### C-3: S3 multipart object upload

- **Region root:** `evaluation/pocketbase/tools/filesystem/internal/s3blob/s3/uploader.go:71` — `(*s3.Uploader).Upload` chooses single-object or multipart upload and drives the S3 request sequence.
- **Caller(s):** `evaluation/pocketbase/tools/filesystem/internal/s3blob/s3blob.go:374` runs the uploader behind the S3 writer; `evaluation/pocketbase/tools/filesystem/filesystem.go:251` streams uploaded file contents into that writer.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — large payloads are split into parts and uploaded concurrently, with XML completion and abort handling (`evaluation/pocketbase/tools/filesystem/internal/s3blob/s3/uploader.go:90`, `evaluation/pocketbase/tools/filesystem/internal/s3blob/s3/uploader.go:333`, `evaluation/pocketbase/tools/filesystem/internal/s3blob/s3/uploader.go:383`).
  - Load profile: yes — user file uploads and backup uploads are bursty and payload-sized, with one tenant able to dominate bandwidth.
  - Coherent unit: yes — the uploader owns the S3 client, key, metadata, reader, and per-upload state for a single object.
  - State independence: yes — mutable state is per-upload (`uploadId`, part list, mutex), and the durable side effect is the object in S3.
  - Latency / failure: maybe — uploads are often synchronous with record or backup operations, but they are already network-bound and have natural retry/abort behavior.
- **Activation shape (informational, not a selection criterion):** Filesystem storage writer for record file uploads and backups.
- **Confidence:** high — this is a classic payload-sized IO lift target if credentials and input streaming are made remote-callable.
- **Risk notes:** The current API streams from an `io.Reader`; lifting it naively may require buffering, presigned handoff, or a remote-readable object source to avoid sending the same bytes through two network hops.

### C-4: Record file-field upload processing

- **Region root:** `evaluation/pocketbase/core/field_file.go:512` — `(*FileField).processFilesToUpload` extracts pending record files and uploads each to collection storage.
- **Caller(s):** `evaluation/pocketbase/core/field_file.go:347` calls it during file-field create/update interception; `evaluation/pocketbase/apis/record_crud.go:350` and `evaluation/pocketbase/apis/record_crud.go:489` submit create/update forms that lead into record saving and field interceptors.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — each upload opens record storage and calls `fsys.UploadFile` for every uploaded file, with cleanup on partial failure (`evaluation/pocketbase/core/field_file.go:522`, `evaluation/pocketbase/core/field_file.go:532`, `evaluation/pocketbase/core/field_file.go:544`).
  - Load profile: yes — record writes with files are uneven by tenant and request payload size.
  - Coherent unit: maybe — it is a named method with clear inputs, but it is private interceptor code tied to `Record`, `FileField`, and `App`.
  - State independence: yes — side effects go through the filesystem abstraction and record metadata rather than package globals.
  - Latency / failure: maybe — it is on the record write path, but file uploads already dominate request latency and failures are returned with cleanup attempts.
- **Activation shape (informational, not a selection criterion):** Record create/update save interceptor.
- **Confidence:** medium — the work is useful, but the save-interceptor coupling makes the boundary less clean than lifting the lower-level uploader.
- **Risk notes:** The upload must remain coordinated with record persistence so that newly uploaded blobs are cleaned up when the DB save fails.

### C-5: OAuth2 token and profile exchange

- **Region root:** `evaluation/pocketbase/apis/record_auth_with_oauth2.go:30` — `recordAuthWithOAuth2` handles OAuth login by exchanging a code, fetching provider user info, optionally downloading an avatar, and linking or creating an auth record.
- **Caller(s):** `evaluation/pocketbase/apis/record_auth.go:35` registers the `/auth-with-oauth2` route; `evaluation/pocketbase/apis/base.go:42` wires the auth API into the main API group.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — it performs external token and profile HTTP calls (`evaluation/pocketbase/apis/record_auth_with_oauth2.go:89`, `evaluation/pocketbase/apis/record_auth_with_oauth2.go:95`) plus optional avatar download (`evaluation/pocketbase/apis/record_auth_with_oauth2.go:301`).
  - Load profile: yes — OAuth callback traffic is bursty during launches, campaigns, and provider outages/retries.
  - Coherent unit: maybe — the handler is named and bounded, but it carries the full request event and performs DB lookup/link/create work as well as provider IO.
  - State independence: maybe — most state is durable DB/provider data, but Apple name forwarding touches the app store (`evaluation/pocketbase/apis/record_auth_with_oauth2.go:103`).
  - Latency / failure: yes — OAuth login already expects provider network latency and maps provider failures to explicit authentication errors.
- **Activation shape (informational, not a selection criterion):** HTTP auth route handler.
- **Confidence:** medium — the outbound provider work is a good lift target, while the DB linking transaction is less separable.
- **Risk notes:** A cleaner lift boundary may be a smaller token/profile/avatar exchange helper; lifting the whole handler would drag request metadata, hooks, and record creation semantics with it.

### C-6: Password reset email send

- **Region root:** `evaluation/pocketbase/mails/record.go:128` — `mails.SendRecordPasswordReset` creates a reset token, renders the configured email template, and sends it through the app mailer.
- **Caller(s):** `evaluation/pocketbase/apis/record_auth_password_reset_request.go:56` runs the send in a background goroutine; `evaluation/pocketbase/apis/record_auth.go:46` registers the password reset request route.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — it generates the token, resolves HTML templates, and sends via the mailer (`evaluation/pocketbase/mails/record.go:129`, `evaluation/pocketbase/mails/record.go:136`, `evaluation/pocketbase/mails/record.go:161`).
  - Load profile: yes — reset/verification-style mail can spike during auth incidents, imports, campaigns, or abuse attempts.
  - Coherent unit: maybe — the function is callable and narrow, but it still depends on `core.App` for settings, template hooks, and mailer construction.
  - State independence: yes — durable effects are external email delivery and app store resend throttling managed by the caller.
  - Latency / failure: yes — the API explicitly dispatches the send through `routine.FireAndForget`, so the client does not depend on SMTP completion (`evaluation/pocketbase/apis/record_auth_password_reset_request.go:54`).
- **Activation shape (informational, not a selection criterion):** Background auth-mail goroutine launched from an HTTP route.
- **Confidence:** medium — useful as an async IO lift, though each individual email may be small.
- **Risk notes:** Retries can duplicate emails or issue fresh tokens; the resend-throttle update currently happens after successful send in the caller.

### C-7: Collection schema import

- **Region root:** `evaluation/pocketbase/core/collection_import.go:36` — `(*BaseApp).ImportCollections` normalizes imported collection definitions, optionally deletes missing collections, saves imports, and validates schema in one transaction.
- **Caller(s):** `evaluation/pocketbase/apis/collection_import.go:30` invokes it from the import request event; `evaluation/pocketbase/apis/collection.go:26` registers the collection import route.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — it marshals/unmarshals collection maps, sorts view collections, runs transactional saves, and validates each imported collection (`evaluation/pocketbase/core/collection_import.go:72`, `evaluation/pocketbase/core/collection_import.go:120`, `evaluation/pocketbase/core/collection_import.go:141`, `evaluation/pocketbase/core/collection_import.go:181`).
  - Load profile: maybe — imports are admin-driven rather than continuously hot, but cost scales with schema size and delete/update breadth.
  - Coherent unit: yes — the method takes imported data plus a delete flag and returns an error.
  - State independence: yes — state changes are through the database transaction and collection models.
  - Latency / failure: yes — it is an admin operation with explicit validation and error reporting.
- **Activation shape (informational, not a selection criterion):** Admin HTTP route handler.
- **Confidence:** medium — good for large tenant schema imports, less compelling for small projects where imports are rare.
- **Risk notes:** `deleteMissing` can remove collections and records; remote execution must preserve transaction boundaries and hook behavior exactly.

### C-8: Search query parse and execution

- **Region root:** `evaluation/pocketbase/tools/search/provider.go:363` — `(*search.Provider).ParseAndExec` parses URL query options and executes the configured search provider.
- **Caller(s):** `evaluation/pocketbase/apis/record_crud.go:88` uses it for record list queries; `evaluation/pocketbase/apis/logs.go:28` uses it for logs listing.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — execution builds filter expressions, sort expressions, count queries, and model queries, with count and fetch running concurrently (`evaluation/pocketbase/tools/search/provider.go:242`, `evaluation/pocketbase/tools/search/provider.go:259`, `evaluation/pocketbase/tools/search/provider.go:337`).
  - Load profile: yes — list endpoints are tenant- and query-dependent, with expensive filters, sorts, and large pages concentrating load.
  - Coherent unit: yes — the provider encapsulates the resolver, query, pagination, filters, and output slice.
  - State independence: yes — it is read/query oriented and relies on the database and resolver interfaces rather than mutable globals.
  - Latency / failure: maybe — this is synchronous request-path work, but complex searches are already DB-bound enough that a hop may be tolerable under load.
- **Activation shape (informational, not a selection criterion):** HTTP list endpoint utility.
- **Confidence:** medium — strong for expensive record/log listing, weaker for common small pages and indexed filters.
- **Risk notes:** The provider carries a live `dbx.SelectQuery` and resolver; a remote boundary would likely need a serializable query plan or a replica with equivalent DB access.

### C-9: Record relation expansion

- **Region root:** `evaluation/pocketbase/core/record_query_expand.go:34` — `(*BaseApp).ExpandRecords` expands relation fields for a list of records across direct, indirect, and nested expand paths.
- **Caller(s):** `evaluation/pocketbase/apis/record_helpers.go:355` calls it during default record enrichment; `evaluation/pocketbase/apis/record_crud.go:100` triggers enrichment from record list responses.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — expansion normalizes paths, fetches related records, recursively expands nested relations, indexes related records, and merges expand data (`evaluation/pocketbase/core/record_query_expand.go:35`, `evaluation/pocketbase/core/record_query_expand.go:172`, `evaluation/pocketbase/core/record_query_expand.go:179`, `evaluation/pocketbase/core/record_query_expand.go:186`).
  - Load profile: yes — cost varies with requested `expand` paths, page size, relation fanout, and tenant data shape.
  - Coherent unit: yes — it has a direct records/expands/fetch-function contract and returns per-expand errors.
  - State independence: maybe — relation reads are durable DB reads, but the caller-supplied fetch function can enforce request auth and trigger enrichment hooks (`evaluation/pocketbase/apis/record_helpers.go:375`, `evaluation/pocketbase/apis/record_helpers.go:407`).
  - Latency / failure: maybe — it is synchronous response enrichment, but optional relation expansion is already a heavier user-requested feature.
- **Activation shape (informational, not a selection criterion):** Record list/view response enrichment utility.
- **Confidence:** medium — useful for relation-heavy datasets, marginal for small pages or shallow expands.
- **Risk notes:** Hook execution and request-aware access rules make this less isolated than a pure graph expansion helper.

Honest assessment: I am most confident in thumbnail generation, backup creation, and S3 multipart upload because their work scales directly with payload size and has clear durable side effects. The file-field upload, OAuth2, and mail candidates are useful but more coupled to request/auth semantics, while collection import, search execution, and relation expansion are genuinely marginal because they depend heavily on DB locality and exact hook/rule behavior. I suspect backup restore and JS hook execution can be expensive, but restore has a severe local filesystem/restart failure model and JS hook execution is too arbitrary and app-state-coupled to justify as a clean utility lift under this rubric.
