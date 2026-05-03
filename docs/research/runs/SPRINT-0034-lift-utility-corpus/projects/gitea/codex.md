**Project read.** Gitea is a Go forge with HTTP/API routes, background queues, repository storage operations, package registries, webhooks, Actions workflow handling, and search/indexing services. The most promising lift regions cluster around queued or naturally long-running work: webhook delivery, archive generation, pull-request conflict checks, migrations, mirror synchronization, and repository indexing. There are also useful but more marginal request-path regions in markup, diff rendering, package metadata rebuilds, and upload parsing where work scales with user payload size. I focused on named functions or methods that already have durable inputs such as task IDs, repository IDs, package owner/group keys, or content readers, and avoided muxes, queue schedulers, and long-lived connection handlers.

### C-1: Webhook task delivery

- **Region root:** `evaluation/gitea/services/webhook/deliver.go:153` — `Deliver` creates the per-webhook HTTP request, sends it, and records request/response status.
- **Caller(s):** `evaluation/gitea/services/webhook/webhook.go:98` invokes `Deliver` from the hook queue handler; `evaluation/gitea/services/webhook/webhook.go:176` enqueues newly created hook tasks.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — request construction includes payload conversion, HMAC headers, outbound HTTP, and response capture up to a bounded body.
  - Load profile: yes — repository, owner, and system webhooks can fan out sharply on push/release/issue bursts.
  - Coherent unit: yes — the task input is a `*HookTask`, and durable effects go through webhook task/status database updates.
  - State independence: yes — it reads webhook configuration and writes delivery results through models rather than relying on caller-local state.
  - Latency / failure: yes — it is already queue-backed and marks delivery in the database before attempting the remote call.
- **Activation shape (informational, not a selection criterion):** queue worker for webhook delivery tasks.
- **Confidence:** high — this is the clearest async, payload-and-network-scaled unit I found.
- **Risk notes:** requester functions are registered in a package-level map, and the lifted implementation would need equivalent webhook type registration, HTTP client policy, and secret handling.

### C-2: Repository archive generation

- **Region root:** `evaluation/gitea/services/repository/archiver/archiver.go:146` — `doArchive` generates a repository archive or bundle and stores the finished artifact.
- **Caller(s):** `evaluation/gitea/services/repository/archiver/archiver.go:237` calls it from the archive queue handler; `evaluation/gitea/services/repository/archiver/archiver.go:341` waits for an archive when serving downloads.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — archive generation streams `git archive`/bundle output and writes a payload-sized artifact to archive storage.
  - Load profile: yes — archive downloads are user-triggered and can spike on popular refs, releases, or automation.
  - Coherent unit: yes — the region is driven by an `ArchiveRequest` containing repo, archive type, commit ID, and paths.
  - State independence: maybe — correctness is durable through archive metadata and storage, but remote replicas need access to repository and archive storage.
  - Latency / failure: yes — generation is queue-backed and cached; callers can await readiness and reuse an existing archive.
- **Activation shape (informational, not a selection criterion):** archive queue worker, with HTTP download callers waiting on the generated artifact.
- **Confidence:** high — this is a large, bounded, already-queued unit; shared storage assumptions are the main caveat.
- **Risk notes:** streaming mode and path-specific archives bypass the queued cache path, so the lift boundary should target `doArchive` rather than all archive serving.

### C-3: Pull-request mergeability check

- **Region root:** `evaluation/gitea/services/pull/check.go:427` — `checkPullRequestMergeable` loads a PR, checks whether it is still open, and runs branch mergeability/conflict checks.
- **Caller(s):** `evaluation/gitea/services/pull/check.go:488` invokes it from the PR patch checker queue; `evaluation/gitea/services/pull/pull.go:422` enqueues checks after PR/head updates.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — it reaches git merge-tree, diff-tree, temporary-repo fallback, and protected-file checks through `checkPullRequestBranchMergeable`.
  - Load profile: yes — a base branch update can enqueue many open PRs, and individual checks scale with repository and diff complexity.
  - Coherent unit: yes — the queued unit is the PR ID, and the function owns loading and updating the PR status.
  - State independence: maybe — durable DB state is used, but the region also takes a global PR work lock and needs git repository access.
  - Latency / failure: yes — it is explicitly queue-backed because conflict checking can be time-consuming.
- **Activation shape (informational, not a selection criterion):** background PR patch checker queue.
- **Confidence:** high — this is a strong lift candidate if the global lock and repository storage are available to replicas.
- **Risk notes:** the PR status transitions and `globallock` semantics must remain exactly once per PR check; a remote worker must not race local queue workers.

### C-4: Repository migration

- **Region root:** `evaluation/gitea/services/migrations/migrate.go:111` — `MigrateRepository` validates a migration request, builds downloader/uploader components, and runs the repository migration.
- **Caller(s):** `evaluation/gitea/services/task/migrate.go:123` executes it inside a migration task; `evaluation/gitea/services/task/task.go:66` pushes created migration tasks to the task queue.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — migration clones git data, pages through milestones, labels, releases, issues, PRs, reviews, comments, tags, and branches.
  - Load profile: yes — cost varies widely by source host, repository size, and issue/PR/comment volume.
  - Coherent unit: maybe — `MigrateRepository` has a clear options object and interfaces, but the dependency closure is broad.
  - State independence: yes — effects are durable repository, package, issue, and task state; failures roll back or record task errors.
  - Latency / failure: yes — web migration uses a background task with progress messages and failure status.
- **Activation shape (informational, not a selection criterion):** migration task queue worker.
- **Confidence:** high — migration is expensive and already shaped as an async job, though it is one of the largest regions in dependency surface.
- **Risk notes:** downloader/uploader implementations pull in much of the migration subsystem, and cancellation/progress messaging would need an explicit remote story.

### C-5: Pull mirror synchronization

- **Region root:** `evaluation/gitea/services/mirror/mirror_pull.go:109` — `runSync` fetches from a pull mirror remote, synchronizes LFS, branches, releases, wiki data, sizes, and branch caches.
- **Caller(s):** `evaluation/gitea/services/mirror/mirror_pull.go:298` calls `runSync` from `SyncPullMirror`; `evaluation/gitea/services/mirror/mirror.go:26` dispatches pull mirror queue requests to `SyncPullMirror`.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — the work includes network git fetches, optional LFS object synchronization, branch/tag/release reconciliation, and repository size updates.
  - Load profile: yes — mirror jobs are periodic but highly variable by upstream churn and repository/LFS size.
  - Coherent unit: yes — the unit is a single mirror record and repository sync attempt.
  - State independence: maybe — durable model updates dominate, but the function needs mutable git repository storage and cache invalidation.
  - Latency / failure: yes — mirror sync is queue/cron driven and already records errors and schedules next updates.
- **Activation shape (informational, not a selection criterion):** mirror queue worker and scheduled mirror update path.
- **Confidence:** medium — the unit is useful, but repository filesystem access makes the lift more operationally demanding.
- **Risk notes:** wiki sync, branch-cache invalidation, and notification fan-out after updated refs increase the remote dependency set.

### C-6: Repository code indexing

- **Region root:** `evaluation/gitea/modules/indexer/code/indexer.go:41` — `index` computes repository code changes and pushes updated/deleted files into the selected code indexer.
- **Caller(s):** `evaluation/gitea/modules/indexer/code/indexer.go:125` invokes it from the code indexer queue handler; `evaluation/gitea/services/indexer/notify.go:93` enqueues it on default-branch pushes.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — it runs git ref/diff/ls-tree commands, reads blobs, sniffs text, converts encodings, detects language, and batches index writes.
  - Load profile: yes — default-branch pushes and migrations vary by repo size and changed-file count.
  - Coherent unit: yes — the queued input is a repository ID plus an `internal.Indexer` interface.
  - State independence: maybe — Elasticsearch-backed indexing is naturally remote-friendly, while local Bleve requires shared or replica-local index handling.
  - Latency / failure: yes — indexing is a background queue and failed items are not on the user request critical path.
- **Activation shape (informational, not a selection criterion):** repository code indexer queue worker.
- **Confidence:** medium — the work is substantial and queued, but backend choice strongly affects lift feasibility.
- **Risk notes:** local repository access, `globalIndexer`, and local Bleve index files would need careful separation from the in-process service.

### C-7: Mirror LFS object synchronization

- **Region root:** `evaluation/gitea/modules/repository/repo.go:61` — `StoreMissingLfsObjectsInRepository` scans git LFS pointer blobs and downloads missing objects into local LFS storage.
- **Caller(s):** `evaluation/gitea/services/mirror/mirror_pull.go:177` calls it during pull mirror synchronization when LFS mirroring is enabled.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — it enumerates pointer blobs, batches LFS downloads, creates LFS metadata, and writes object content.
  - Load profile: yes — LFS repositories have highly skewed object counts and object sizes across tenants and pushes.
  - Coherent unit: yes — inputs are a repository, opened git repository, and LFS client.
  - State independence: maybe — writes are durable, but replicas need access to the same content store and git object graph.
  - Latency / failure: yes — it runs inside mirror background work and tolerates missing upstream LFS objects.
- **Activation shape (informational, not a selection criterion):** subtask of background pull mirror synchronization.
- **Confidence:** medium — this is a strong payload-scaled subregion, but it has only one direct production caller in the lines I opened.
- **Risk notes:** object storage, upstream LFS client credentials, and repository pointer enumeration must all be available remotely.

### C-8: RPM repository metadata rebuild

- **Region root:** `evaluation/gitea/services/packages/rpm/repository.go:163` — `BuildSpecificRepositoryFiles` rebuilds RPM repository metadata files for an owner/group.
- **Caller(s):** `evaluation/gitea/routers/api/packages/rpm/rpm.go:212` rebuilds after RPM upload; `evaluation/gitea/routers/api/packages/rpm/rpm.go:313` rebuilds after RPM file deletion.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — it scans package files, unmarshals cached metadata, emits primary/filelists/other/updateinfo XML, gzips data, and signs `repomd.xml`.
  - Load profile: yes — cost grows with package count per owner/group and can spike during CI publishing bursts.
  - Coherent unit: yes — the unit is owner ID plus RPM group, with durable package storage inputs and outputs.
  - State independence: yes — reads and writes go through package models/storage and signing key lookup.
  - Latency / failure: maybe — upload/delete routes currently expect the metadata rebuild to finish before returning.
- **Activation shape (informational, not a selection criterion):** package upload/delete HTTP route post-processing.
- **Confidence:** medium — the region is bounded and expensive, but it may need a synchronous compatibility shim so package clients see fresh metadata immediately.
- **Risk notes:** OpenPGP key handling and overwrite semantics for repository metadata files must stay atomic from client perspective.

### C-9: Actions workflow detection

- **Region root:** `evaluation/gitea/modules/actions/workflows.go:120` — `DetectWorkflows` lists workflow files, reads each workflow, parses events, and matches them against a triggering event/payload.
- **Caller(s):** `evaluation/gitea/services/actions/notifier_helper.go:186` detects workflows for an event; `evaluation/gitea/services/actions/notifier_helper.go:221` detects base-branch workflows for pull-request target handling.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — it recursively lists workflow entries, reads file contents, parses YAML workflow events, validates workflow structure, and evaluates event filters.
  - Load profile: yes — push and PR event bursts vary by repository workflow count and workflow file size.
  - Coherent unit: yes — inputs are a git repository, commit, event type, payload, and schedule flag; outputs are detected workflow lists.
  - State independence: maybe — most work is pure over git objects and payload, but remote workers need repository object access.
  - Latency / failure: maybe — it runs in the action notifier path and failures affect workflow scheduling rather than the original git object write.
- **Activation shape (informational, not a selection criterion):** notification-triggered Actions workflow discovery.
- **Confidence:** medium — this is a clean parser/filter unit, but its current caller appears synchronous with notification handling.
- **Risk notes:** payload type assertions and event-specific matching must behave identically remotely; git object access is the main coupling.

### C-10: Markdown rendering

- **Region root:** `evaluation/gitea/modules/markup/markdown/markdown.go:186` — `render` converts Markdown input to HTML with Gitea-specific Goldmark extensions and output limiting.
- **Caller(s):** `evaluation/gitea/modules/markup/markdown/markdown.go:259` exposes it through the Markdown renderer; `evaluation/gitea/routers/common/markup.go:98` calls generic markup rendering for preview/markup endpoints.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — it reads the full input, normalizes EOLs, extracts frontmatter, runs Goldmark extensions, math handling, and syntax highlighting wrappers.
  - Load profile: yes — README, wiki, comment preview, and rendered file workloads vary by document size and user activity.
  - Coherent unit: yes — the contract is a render context, reader, and writer.
  - State independence: maybe — render helpers and repo-aware link handling can consult repository context, but most rendering is content-local.
  - Latency / failure: maybe — usually request-path work, but large markup rendering is already payload-proportional and failure can return a render error.
- **Activation shape (informational, not a selection criterion):** HTTP route handler and repository file/wiki rendering helper.
- **Confidence:** medium — useful for large documents and previews, but less clean than queue-backed candidates.
- **Risk notes:** remote invocation would likely need to return a rendered HTML buffer rather than stream through `io.Writer`, and repo-aware helpers may expand dependencies.

### C-11: Renderable git diff construction

- **Region root:** `evaluation/gitea/services/gitdiff/gitdiff.go:1333` — `GetDiffForRender` builds a git diff, parses patch output, annotates files, and highlights renderable content.
- **Caller(s):** `evaluation/gitea/routers/web/repo/pull.go:805` calls it for the pull request files view.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — it runs `git diff`, parses unified patches, checks attributes, detects generated/vendor files, and applies Chroma highlighting within limits.
  - Load profile: yes — large PRs and commit ranges create highly variable diff size and parsing/highlighting cost.
  - Coherent unit: yes — inputs are repository link, git repository, diff options, and optional file filters; output is a `*Diff`.
  - State independence: maybe — it is mostly derived from git object state, but requires an open repository and attribute checker.
  - Latency / failure: maybe — it is on a synchronous page-render path, though heavy diff pages already tolerate comparatively high latency and truncation.
- **Activation shape (informational, not a selection criterion):** HTTP request path for PR/commit/compare diff rendering.
- **Confidence:** medium — the computation is real, but request-path streaming and local git process management make it more marginal.
- **Risk notes:** moving git command execution remotely requires shared repository access and careful cancellation so abandoned requests kill the git process.

### C-12: NPM package upload parsing

- **Region root:** `evaluation/gitea/modules/packages/npm/creator.go:203` — `ParsePackage` parses an npm publish payload, decodes its attachment, and verifies integrity.
- **Caller(s):** `evaluation/gitea/routers/api/packages/npm/npm.go:157` calls it at the start of npm package upload.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — it decodes JSON, base64-decodes the tarball attachment, and hashes the full data with SHA-1 or SHA-512 for integrity verification.
  - Load profile: yes — npm publish traffic is bursty in CI and cost scales with package tarball size.
  - Coherent unit: yes — input is an `io.Reader`, output is a parsed package struct with metadata and data.
  - State independence: yes — parsing and hash verification are pure with no durable side effects.
  - Latency / failure: maybe — it is upload request-path work, but the client is already sending a payload-sized body and parse failures are clean validation errors.
- **Activation shape (informational, not a selection criterion):** package registry HTTP upload parser.
- **Confidence:** low — it is a clean function, but lifting may mostly move payload bytes around unless package sizes are large enough.
- **Risk notes:** the current implementation materializes the decoded tarball in memory; a lifted version should avoid adding another full-copy boundary.

**Honest assessment.** I am most confident in C-1 through C-6: they are already asynchronous or queue-shaped and have clearly variable, expensive work. C-7 and C-8 are useful but depend on shared repository/package storage semantics, while C-9 through C-12 are genuinely more marginal because they sit closer to synchronous HTTP paths or would require moving large request bodies across the lift boundary. I suspect SMTP mail delivery, Debian/Arch package repository rebuilds, and container registry blob finalization may also contain viable lift regions, but I did not include them separately because the evidence I opened either duplicated the selected package-metadata pattern or did not show enough additional compute beyond network/storage calls to outrank these candidates.
