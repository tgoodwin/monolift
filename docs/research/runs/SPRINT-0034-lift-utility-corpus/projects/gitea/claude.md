# Gitea — Phase 1 candidate lift regions (claude)

## Project read

Gitea is a self-hosted Git forge: it serves Git over HTTP/SSH, stores repositories on a local-or-mounted filesystem, persists metadata in PostgreSQL/MySQL/SQLite, and runs an Actions-style CI runner protocol over ConnectRPC. The hot computational paths cluster in three places: (1) **batch queue workers** under `services/{webhook,mirror,repository/archiver,mailer}` and `modules/indexer/{code,issues}`, all built on the in-process `modules/queue.WorkerPoolQueue` and naturally suited to async dispatch; (2) **per-call CPU/IO transforms** like `services/gitdiff.ParsePatch`, `modules/markup.Render`, and `modules/avatar.ProcessAvatarImage` invoked from web handlers; (3) **long-running orchestrations** that shell out to `git` and write to repo storage — `services/migrations.MigrateRepository`, `services/pull.doMergeAndPush`, `services/mirror.runSync`. Region (3) is gated by a hard constraint Monolift will need to address head-on: Gitea expects the local filesystem under `setting.RepoRootPath` to be readable by the process executing the git command. Where that constraint is binding I score `state independence: maybe` and call it out in risk notes.

A second observation: Gitea is the rare project in this corpus where the framework owners themselves have already picked the lift seams. Every queue handler closure (`webhook.handler`, `mirror.queueHandler`, `archiver.Init`'s closure, `mailer.NewContext`'s closure, `repository/push.handler`) is a one-line dispatch into a typed worker function with value-typed arguments — exactly the shape Monolift wants. The most informative candidates are these underlying worker functions, not the handler closures, because the handler closure's only job is to deserialize a queue item and call into the function that does the work.

Twelve candidates follow, ranked by lift utility from strongest to most marginal.

---

### C-1: Webhook delivery worker

- **Region root:** `services/webhook/deliver.go:153` — `Deliver(ctx context.Context, t *webhook_model.HookTask) error`. Loads the webhook config, builds an HTTP request (per-type request builder), POSTs it via `webhookHTTPClient`, records response status/body/headers on the task, persists.
- **Caller(s):** `services/webhook/webhook.go:98` — invoked from the queue handler `handler(items ...int64)` registered at `services/webhook/webhook.go:330` via `queue.CreateUniqueQueue("webhook_sender", handler)`.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — TLS handshake + signed POST + body marshal + bounded response read (`util.ReadWithLimit(resp.Body, 1024*1024)` at `:268`); aggregates well at fan-out.
  - Load profile: yes — bursty per repo activity; a chatty repo can produce hundreds of deliveries per push, while idle repos are silent.
  - Coherent unit: yes — value-typed inputs (`*HookTask` is a model row, `context.Context`); package-level `webhookHTTPClient` (`:278`) is the only out-of-band dependency and is initialized once from settings at `Init()` (`:312`).
  - State independence: yes — reads/writes go through `webhook_model.UpdateHookTask` and `UpdateWebhookLastStatus`; HTTP client is replica-local with replicated config.
  - Latency / failure: yes — already async behind a queue; `MarkTaskDelivered` (`:207`) provides idempotency; failure is logged and persisted on the task row, not propagated to a caller.
- **Activation shape:** queue worker registered with `queue.CreateUniqueQueue`.
- **Confidence:** high — would change my mind only if I discovered `webhookHTTPClient`'s `hostmatcher.NewDialContext` resolver depends on hot-reloaded admin settings.
- **Risk notes:** the package-global allow-list (`hostMatchers` at `:281`, populated by `webhookProxy` in a `sync.Once`) and the proxy `DialContext` need to be reconstructed in the remote replica from the same config keys; trivially replicable. Panics in user-supplied URLs are recovered at `:160`, so the lifted impl must preserve that.

---

### C-2: Repository archive generator

- **Region root:** `services/repository/archiver/archiver.go:146` — `doArchive(ctx context.Context, r *ArchiveRequest) (*repo_model.RepoArchiver, error)`. Creates the archiver DB row in `Generating` state, opens an `io.Pipe`, kicks `aReq.Stream` (which calls `gitrepo.CreateArchive`/`CreateBundle`) into a goroutine that writes to the pipe, then `storage.RepoArchives.Save(rPath, rd, -1)` reads from the pipe and uploads to the configured storage backend.
- **Caller(s):** `services/repository/archiver/archiver.go:237` — queue handler closure registered at `:246` via `queue.CreateUniqueQueue("repo-archive", handler)`. User-facing trigger goes through `(*ArchiveRequest).Await` at `:81` which polls the DB row.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — git tree walk to enumerate the commit's blobs, tar/zip encode, optional bundle serialization; archive size scales with repo size and can be hundreds of MB.
  - Load profile: yes — clearly bursty (release announcement → many users hit `/archive/refs/heads/main.zip`); the storage backend `Save` further amortizes.
  - Coherent unit: yes — `*ArchiveRequest` is a value-shaped struct (`{Repo, Type, CommitID, Paths, archiveRefShortName}` at `:36`).
  - State independence: maybe — `gitrepo.CreateArchive` shells out to `git archive`, which requires the repo on disk under `setting.RepoRootPath`; `storage.RepoArchives` is an injected `Storage` interface so the upload side is virtualized; DB row goes through `db.Insert`.
  - Latency / failure: yes — caller polls via `Await` (`:81`), so adding a network hop has no user-visible impact; on-failure the row stays in `Generating` until retried.
- **Activation shape:** queue worker registered with `queue.CreateUniqueQueue`.
- **Confidence:** high.
- **Risk notes:** `aReq.Stream` shells out to `git archive` and so the inner goroutine and the outer `storage.RepoArchives.Save` must run colocated with the on-disk repo (`gitrepo` package). If the lift is to truly remote compute, the repo storage must be reachable (NFS, object-store-backed git, or a sidecar). The `io.Pipe` between Stream goroutine and Save reader (`:189`) means the lift has to keep both halves in the same address space, or replace the pipe with a streamed RPC body.

---

### C-3: Mirror pull sync (`runSync` + `SyncPullMirror`)

- **Region root:** `services/mirror/mirror_pull.go:109` — `runSync(ctx, *repo_model.Mirror) ([]*SyncResult, bool)`. Executes `git fetch --tags [--prune]` against the remote, retries with prune on broken-reference errors, writes the commit graph, opens the repo, optionally pulls LFS objects, then reconciles branch/tag refs.
- **Caller(s):** `services/mirror/mirror_pull.go:298` — `SyncPullMirror(ctx, repoID)` at `:269` is the orchestrator (acquires `globallock` at `:280`, runs `runSync`, schedules next update). It is invoked from the queue handler `queueHandler` at `services/mirror/queue.go:33` and on a 10-minute cron registered at `services/cron/tasks_basic.go:31`.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — outbound network fetch over HTTPS/SSH, bounded by `Git.Timeout.Mirror`; LFS sync at `mirror_pull.go:177` adds a second network leg whose cost scales with payload.
  - Load profile: yes — periodic (every 10m, all mirrors batched) and bursty (manual sync trigger); per-tenant uneven (one heavy mirror dominates).
  - Coherent unit: yes — `runSync(ctx, *Mirror)` is a clean unit; `*Mirror` carries the joined `*Repo` but is a value-typed model row.
  - State independence: maybe — the global lock at `SyncPullMirror:280` (`getRepoPullMirrorLockKey`) is via `globallock` which already has a remote-friendly backend; **but** `gitrepo.RunCmdString` runs the local `git` binary against the on-disk repo. If `setting.RepoRootPath` is virtualized, this works; otherwise it must run colocated.
  - Latency / failure: yes — invoked async (cron + queue); failure path writes `system_model.CreateRepositoryNotice` and calls `repo_model.TouchMirror` to advance the schedule.
- **Activation shape:** queue worker (per-repo); also driven by cron `update_mirrors`.
- **Confidence:** medium — the pull mechanics are clean and async, but the on-disk repo dependency is a real constraint.
- **Risk notes:** repo-on-disk dependency dominates. Suggest pairing this candidate with K4 (Gitea SSE eventsource) and N-GT-5 (Mirror sync queue handler) from SPRINT-0033 only as siblings. The recoverable-error retry at `:137`–`:151` re-invokes `cmdFetch()`, so any lift must be re-entrant on the same input.

---

### C-4: Code indexer (per-repo `index`)

- **Region root:** `modules/indexer/code/indexer.go:41` — `index(ctx, indexer internal.Indexer, repoID int64) error`. Loads the repo, decides whether to skip (forks/mirrors/templates per config), computes the default-branch SHA, calls `getRepoChanges` to diff against the previously-indexed SHA, and pushes the change set to the active indexer (`Bleve` or `Elasticsearch`).
- **Caller(s):** `modules/indexer/code/indexer.go:125` — invoked from the queue handler closure at `:121`, registered at `:134` via `queue.CreateUniqueQueue("code_indexer", handler)`. Producers of the queue items are the push pipeline and the reindex cron.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — `getRepoChanges` walks `git diff-tree`/`git ls-files` between two SHAs; `indexer.Index` ships those blobs to ES (network) or writes to Bleve (local file). Both scale with churn.
  - Load profile: yes — bursty on push storms; periodic on populate cron.
  - Coherent unit: yes — `(ctx, Indexer, int64)`; `Indexer` is interface-typed (`internal.Indexer`), repoID is a value.
  - State independence: maybe — for ES backend, fully remote-friendly; for Bleve backend, the index file is local at `setting.Indexer.RepoPath` and a remote replica would diverge. The lift is clean only when ES is selected.
  - Latency / failure: yes — queue handler returns `nil` on failure (`:131`) to avoid re-queueing broken repos; tolerant.
- **Activation shape:** queue worker.
- **Confidence:** high for ES-mode, medium for Bleve-mode.
- **Risk notes:** the global indexer is read via `*globalIndexer.Load()` (atomic pointer at `:33`), which the remote replica must initialize from the same `setting.Indexer.*` config. `getRepoChanges` shells to git on disk, so the same on-disk constraint as C-3 applies.

---

### C-5: Issue indexer per-item handler

- **Region root:** `modules/indexer/issues/indexer.go:166` — `getIssueIndexerQueueHandler(ctx)` returns the per-item handler that calls `getIssueIndexerData(ctx, item.ID)` (loads the issue and all its comments/labels/etc) and then `indexer.Index(ctx, data)` or `indexer.Delete(ctx, item.IDs...)`.
- **Caller(s):** the queue handler is registered when the issues indexer queue is created (search for `queue.CreateUniqueQueue("issue_indexer", ...)` in `indexer.go`). Producers are issue/comment create/update/delete paths and the population sweep `PopulateIssueIndexer` at `:214`.
- **Why useful (rubric scoring):**
  - Compute envelope: maybe — per-item is a DB read (issue + comments) plus a write to ES/Bleve/Meilisearch; small for short issues, large for issues with thousands of comments.
  - Load profile: yes — bursty on comment activity; the handler returns unhandled items for retry (`:200`), which itself can spike under indexer-backend backpressure.
  - Coherent unit: yes — closure body operates on `*IndexerMetadata` (`{ID, IDs, IsDelete}`), value-typed.
  - State independence: yes for ES/Meili backends; same Bleve caveat as C-4.
  - Latency / failure: yes — queue-backed with explicit retry list.
- **Activation shape:** queue worker.
- **Confidence:** medium — the per-item DB load can be heavy for big issues but is small in the common case; lift utility is real but the cost-per-call is more variable than C-1/C-2.
- **Risk notes:** `getIssueIndexerData` (not opened) loads associated comments/attachments; a remote replica would issue cross-table reads against the shared DB, which is the same hop already paid by every Gitea request. Bleve-backend caveat from C-4 applies.

---

### C-6: Mailer queue worker

- **Region root:** `services/mailer/mailer.go:48` — the queue handler closure inside `NewContext`, signature `func(items ...*sender_service.Message) []*sender_service.Message`. For each message it calls `msg.ToMessage()` then `sender_service.Send(sender, msg)`.
- **Caller(s):** registered at `:48` via `queue.CreateSimpleQueue("mail", ...)`. Producers are everything in `services/mailer/mail_*.go` (e.g. `mail_issue.go:186` `SendIssueAssignedMail`) which call `SendAsync` (`:67`) → `mailQueue.Push`.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — template rendering happens in the producer (`templates.MailRenderer()`), but the worker still does an SMTP TLS handshake + send per message; bounded but non-trivial under fan-out.
  - Load profile: yes — bursty on issue/PR activity, comment storms, password-reset campaigns.
  - Coherent unit: yes — `*sender_service.Message` is value-typed; `sender` is package-level (`mailer.go:22`) but is one of `SendmailSender{}`, `DummySender{}`, `SMTPSender{}`, all initialized once from `setting.MailService.Protocol`.
  - State independence: yes — sender is replica-local; SMTP is a stateless outbound call.
  - Latency / failure: yes — queue-backed; failures are logged and dropped (`:53`).
- **Activation shape:** queue worker registered with `queue.CreateSimpleQueue`.
- **Confidence:** high.
- **Risk notes:** SMTP keepalive/connection pooling state lives inside `SMTPSender`; if the lift maintains a connection pool per replica that's correct, but rate-limit accounting (if any) would also be per-replica.

---

### C-7: PR merge-and-push (`doMergeAndPush`)

- **Region root:** `services/pull/merge.go:334` — `doMergeAndPush(ctx, *PullRequest, *User, MergeStyle, expectedHeadCommitID, message, PushTrigger) (string, error)`. Builds a temporary repo via `createTemporaryRepoForMerge`, dispatches to one of `doMergeStyleMerge`/`doMergeStyleRebase`/`doMergeStyleSquash`/`doMergeStyleFastForwardOnly`, captures the merge head SHA, and force-pushes back to the base.
- **Caller(s):** `services/pull/merge.go:267` — invoked from `Merge` (the public entry at `:223`) which holds the `globallock` for `getPullWorkingLockKey(pr.ID)`. `Merge` itself is invoked from web/api PR-merge handlers and from the `automergequeue` worker.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — clones base and head into a temp dir, runs git merge/rebase/squash, computes commit, pushes refs; multi-second per call on non-trivial PRs.
  - Load profile: maybe — bursty around release windows when many PRs land; otherwise modest.
  - Coherent unit: maybe — `*PullRequest` requires a fully-loaded relationship graph (`LoadBaseRepo`/`LoadHeadRepo` at `:224`/`:227`); this is more state-pull than a pure `(repoID, prID)` would be.
  - State independence: maybe — caller holds `globallock` (remote-friendly); the merge runs against a temp directory under `setting.RepoRootPath/temp` and the underlying repo on disk; same on-disk constraint as C-3.
  - Latency / failure: yes — already O(seconds), an extra hop is in the noise; the duplicate `AddTestPullRequestTask` in `Merge`'s defer (`:256`) means a failure here doesn't strand the system.
- **Activation shape:** HTTP route (web/api PR merge) and queue worker (auto-merge).
- **Confidence:** medium.
- **Risk notes:** the temp-dir strategy plus the post-receive hook re-invocation (`:273` reloads the PR after the hook fires) means the lift must run colocated with the post-receive plumbing or accept that the post-receive notification arrives after the remote call returns. The "DUPLICATE-PR-TASK" comment at `:255` flags this exact coupling — worth reading before lifting.

---

### C-8: Repository migration (`MigrateRepository`)

- **Region root:** `services/migrations/migrate.go:111` — `MigrateRepository(ctx, *User, ownerName, MigrateOptions, Messenger) (*Repository, error)`. Validates the source URL, builds a downloader for the source git host, creates a `GiteaLocalUploader`, calls `migrateRepository` (`:179`) which streams repo info, releases, milestones, labels, issues, PRs, comments, wiki from upstream and into the uploader.
- **Caller(s):** `routers/web/user/home.go` (web migration form) and `routers/api/v1/repo/migrate.go` (REST). User-triggered.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — multi-minute outbound API/git fetch; the heaviest single-call workload in the project.
  - Load profile: yes — bursty on enterprise onboarding waves; per-tenant uneven (one big GitHub org dominates).
  - Coherent unit: yes — clean signature with value-typed `MigrateOptions`; `Messenger` is an interface (`base.NilMessenger` is the default at `migrate.go:181`); `Downloader`/`Uploader` are interfaces created internally.
  - State independence: maybe — `NewGiteaLocalUploader` writes the new repo to `setting.RepoRootPath`; the rollback path at `:131` is the only durable cleanup. Same on-disk constraint as C-3/C-7.
  - Latency / failure: yes — naturally async (today already runs in a service goroutine); failure goes through `Rollback()` and `system_model.CreateRepositoryNotice`.
- **Activation shape:** HTTP route, runs to completion in a request goroutine; the user UI polls.
- **Confidence:** medium.
- **Risk notes:** `NewGiteaLocalUploader` is a heavy constructor that mutates `*GiteaLocalUploader` state across many DB writes; lifting `MigrateRepository` requires that the uploader's filesystem and DB writes happen at the lift target, which is a strong sibling of the C-3 disk-locality concern.

---

### C-9: Avatar image processing (`ProcessAvatarImage`)

- **Region root:** `modules/avatar/avatar.go:101` — `ProcessAvatarImage(data []byte) ([]byte, error)`. Trampolines into `processAvatarImage` at `:46`, which decodes via `image.Decode`, validates dimensions against `setting.Avatar.MaxWidth/MaxHeight`, square-crops and bilinear-resizes to `DefaultAvatarSize * RenderedSizeFactor`, re-encodes as PNG, and returns the smaller of original vs. resized.
- **Caller(s):** user/org avatar upload handlers under `routers/web/user/setting/profile.go` (search the routes for "avatar") and the API equivalents.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — image decode + bilinear resize on a 4K input is hundreds of milliseconds and is allocation-heavy; pure CPU.
  - Load profile: yes — bursty on signup waves (an enterprise onboarding can produce thousands of avatar uploads in minutes).
  - Coherent unit: yes — `(data []byte) -> ([]byte, error)`. As pure as a region gets in this codebase.
  - State independence: yes — only depends on `setting.Avatar.*` (replicable config); no DB, no filesystem.
  - Latency / failure: yes — caller is a setting-update HTTP handler, not on a request critical path; tolerable to add a hop.
- **Activation shape:** HTTP route (web setting handler).
- **Confidence:** high.
- **Risk notes:** the only state coupling is `setting.Avatar.MaxOriginSize` and `RenderedSizeFactor`; the inner `processAvatarImage(data, maxOriginSize int64)` already takes the size as a value parameter and is purer — that is the actual lift target.

---

### C-10: Diff parsing (`ParsePatch`)

- **Region root:** `services/gitdiff/gitdiff.go:631` — `ParsePatch(ctx, maxLines, maxLineCharacters, maxFiles int, reader io.Reader, skipToFile string) (*Diff, error)`. Streaming-parses unified-diff text into `*Diff{Files: []*DiffFile}`, enforcing the three caps and supporting partial parses for paginated PR views.
- **Caller(s):** `routers/web/repo/pull.go` (PR diff view) and `routers/web/repo/commit.go` (commit view). Both wire the output of `git diff` into `ParsePatch`.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — string parsing, hunk splitting, and per-line tokenizing across a stream that can be MB-scale on big PRs; CPU- and allocation-bound.
  - Load profile: maybe — uniform on a typical day, but spikes on monorepo refactor PRs and on bots that scrape diffs.
  - Coherent unit: yes — clean numeric-and-reader inputs, returns a value `*Diff`.
  - State independence: yes — pure transformer; no globals, no DB, no fs (the `io.Reader` is pre-built by the caller).
  - Latency / failure: maybe — caller is on the synchronous PR-page render; for small diffs the network hop dominates the parse, for large diffs it does not. Useful only when conditional dispatch (e.g. by content-length) is in play.
- **Activation shape:** synchronous web handler.
- **Confidence:** medium.
- **Risk notes:** the caller passes an `io.Reader` connected to a running `git diff` command's stdout (see `services/gitdiff` callers); lifting requires materializing the diff bytes once before the RPC, or streaming them — a real engineering call, not a free win.

---

### C-11: Push-update worker (`pushUpdates`)

- **Region root:** `services/repository/push.go:77` — `pushUpdates(optsList []*PushUpdateOptions) error`. Opens the git repo, updates repo size, iterates each `PushUpdateOptions` in the batch, resolves pushers, walks new commits, generates push action history, fires per-tag/branch downstream hooks (notify, webhook enqueue, indexer enqueue).
- **Caller(s):** `services/repository/push.go:38` — the queue handler `handler` registered at `:48` via `queue.CreateSimpleQueue("push_update", handler)`. Producers are post-receive hooks via `PushUpdates` (`:62`).
- **Why useful (rubric scoring):**
  - Compute envelope: yes — opens git repo, enumerates commits since old SHA, can be expensive on a 10k-commit force-push; cascades downstream queue tasks (webhook, indexer, actions).
  - Load profile: yes — proportional to push activity; bursty around CI green-light campaigns.
  - Coherent unit: yes — slice of value-typed `*PushUpdateOptions`, returns error.
  - State independence: maybe — opens git repo via `gitrepo.OpenRepository`, same on-disk constraint as C-3.
  - Latency / failure: yes — queue-backed; the function is the producer for downstream queues, not a synchronous-critical caller.
- **Activation shape:** queue worker.
- **Confidence:** medium.
- **Risk notes:** triggers a fan-out of follow-up enqueues; the lifted unit must produce the same downstream effects, meaning the lift either preserves access to those queue handles (DB-backed queue makes this easy) or accepts that the side-effecting calls happen in the local proxy. `UpdateRepoSize` writes to the DB.

---

### C-12: Markdown render (`Render` / `RenderString`) — *marginal*

- **Region root:** `modules/markup/render.go:193` — `Render(rctx *RenderContext, origInput io.Reader, output io.Writer) error`, with the string convenience wrapper at `:202` `RenderString(ctx, content) (string, error)`. The pipeline detects markup type from the first 512 bytes, dispatches to the registered renderer (Goldmark for Markdown — `modules/markup/markdown/markdown.go:264`), and post-processes (link-rewriting, sanitizer).
- **Caller(s):** issue/comment view handlers, wiki page handlers, README rendering — many sites; e.g. `routers/web/repo/wiki.go`, `routers/web/repo/issue_view.go`.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — Goldmark parse + syntax-highlight + sanitize; CPU-bound and scales with content length.
  - Load profile: yes — every page render; per-tenant uneven (one big wiki dominates).
  - Coherent unit: yes — `RenderContext` is per-call, `io.Reader`/`io.Writer` are values.
  - State independence: yes — sanitizer state is package-shared but stable; no per-request mutable globals.
  - Latency / failure: **maybe/no** — most calls render small inputs (an issue body, a comment) where a network RPC would dominate the render itself. Lift utility is conditional on payload size and would require gating the dispatch by input length to be net-positive.
- **Activation shape:** synchronous web handler.
- **Confidence:** low — included for completeness because the underlying function passes the structural rubric, but the latency story argues against lifting it unconditionally.
- **Risk notes:** the post-processor reads `RenderContext.RenderOptions.Metas` (a `map[string]string`) and writes to `output` (often the response writer); lifting requires the writer side to either accept a buffered string return (use `RenderString` form) or stream over RPC. The marginal candidate is best read as "this is what *not* to lift in this corpus" — useful negative evidence.

---

## Honest assessment

The candidates I am most confident about are **C-1 (Webhook delivery)**, **C-6 (Mailer worker)**, and **C-9 (Avatar processing)**: they pass all five rubric criteria cleanly, they don't depend on the on-disk repo store, and the framework has already isolated them as queue-backed or pure-CPU units with value-typed inputs. **C-2 (Archive generator)** is also high-confidence but carries the inner-goroutine-plus-pipe topology that any lift will need to remap to a streamed RPC body.

The genuinely marginal candidates are **C-12 (Markdown render)** and **C-10 (ParsePatch)**: both are CPU-meaningful but sit on synchronous request paths where the network hop is the same order of magnitude as the work itself. They are useful for the corpus only if Monolift's runtime oracle can gate dispatch by input size — without that, they are negative evidence about when the lift overhead exceeds the benefit.

The candidates I downgraded but couldn't outright exclude — **C-3 (mirror sync)**, **C-7 (PR merge)**, **C-8 (migration)**, **C-11 (push updates)** — all share one constraint: they shell out to `git` against the on-disk repo at `setting.RepoRootPath`. The rubric's disqualifier is "filesystem paths that *cannot be virtualized*", and I judged these as virtualizable (NFS, object-backed git, sidecar) rather than impossible — but a Phase 2 reviewer who reads the disqualifier more strictly would be defensible in cutting them. They're the most interesting lift targets *in principle* (long-running, expensive, naturally async) and the least clean *in practice*.

The region in this codebase I suspect is a great lift candidate but couldn't justify under the rubric is the **Actions runner `UpdateLog`** path at `routers/api/actions/runner/runner.go:249`. It is queue-shaped, IO-heavy (writes log shards to DBFS storage), and naturally bursty (an active CI run streams logs at high frequency). I held it back because I didn't have evidence that the per-call compute envelope is non-trivial — most calls are small log appends, and without a measurement of average shard size I couldn't honestly score `compute envelope: yes`. If the cross-review phase has a probe that measures call rate and average payload, it should be reconsidered.
