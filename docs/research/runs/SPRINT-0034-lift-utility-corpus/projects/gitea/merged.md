# gitea — merged lift-region candidate set (Phase 2b final)

## Methodology note

Phase 1 drafts: claude, codex, gemini.
Phase 2a critics: each model reviewed the other two.
Phase 2b aggregator: claude (opus).

Inclusion rules from `PHASE2-PLAN.md` §"Inclusion rules" applied deterministically. MODIFY corrections (line cite drift, scope narrowing, region reframing) applied before producing each merged entry. Where critics disagreed materially, the disagreement is recorded under "Discrepancies" with the rubric criterion that justified my call.

Twenty-one candidates pass the inclusion rules; ranked strongest → weakest by combined cross-model consensus and rubric cleanliness. The first eight are the high-confidence corpus; the remainder are useful-but-marginal evidence (synchronous request paths, disputed picks, or sub-region overlaps with stronger picks).

---

## Merged candidates (ranked strongest → weakest)

### M-1: Webhook delivery worker

- **pick_provenance:** claude+codex+gemini (3/3)
- **critique_status:** KEEP from all 3 critics; MODIFY from claude and codex on gemini's wrong line cite (`:125` → `:153`).
- **Region root:** `evaluation/gitea/services/webhook/deliver.go:153` — `Deliver(ctx context.Context, t *webhook_model.HookTask) error`. Loads webhook config, builds the per-type HTTP request, signs it, POSTs via `webhookHTTPClient`, captures bounded response body, persists status to the `HookTask` row.
- **Caller(s):** `evaluation/gitea/services/webhook/webhook.go:98` — invoked from the queue handler `handler(items ...int64)` registered at `services/webhook/webhook.go:330` via `queue.CreateUniqueQueue("webhook_sender", handler)`.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — TLS handshake + signed POST + body marshal + bounded response read (`util.ReadWithLimit(resp.Body, 1024*1024)`); aggregates well at fan-out.
  - Load profile: yes — bursty per repo activity; chatty repo can produce hundreds of deliveries per push, idle repos are silent.
  - Coherent unit: yes — value-typed `*HookTask` and `context.Context`; package-level `webhookHTTPClient` is the only out-of-band dependency, initialized once from settings.
  - State independence: yes — reads/writes go through `webhook_model.UpdateHookTask` and `UpdateWebhookLastStatus`; HTTP client is replica-local with replicated config.
  - Latency / failure: yes — already async behind a queue; `MarkTaskDelivered` provides idempotency; failures are persisted on the task row, not propagated to a caller.
- **Activation shape:** queue worker (`queue.CreateUniqueQueue`).
- **Confidence:** high — would change my mind only if `webhookHTTPClient`'s `hostmatcher.NewDialContext` resolver depends on hot-reloaded admin settings.
- **Risk notes:** package-global allow-list (`hostMatchers`, populated by `webhookProxy` in a `sync.Once`) and the proxy `DialContext` need to be reconstructed in the remote replica from the same config keys — trivially replicable. Panics in user-supplied URLs are recovered at `:160`; the lifted impl must preserve that.

---

### M-2: Repository archive generator

- **pick_provenance:** claude+codex+gemini (3/3)
- **critique_status:** KEEP from all 3 critics; MODIFY from claude and codex on gemini's wrong line cite (`:120` → `:146`).
- **Region root:** `evaluation/gitea/services/repository/archiver/archiver.go:146` — `doArchive(ctx, *ArchiveRequest) (*RepoArchiver, error)`. Creates the archiver DB row in `Generating` state, opens an `io.Pipe`, kicks `aReq.Stream` (which calls `gitrepo.CreateArchive`/`CreateBundle`) into a goroutine that writes to the pipe, then `storage.RepoArchives.Save` reads from the pipe and uploads to the configured storage backend.
- **Caller(s):** `evaluation/gitea/services/repository/archiver/archiver.go:237` — queue handler closure registered at `:246` via `queue.CreateUniqueQueue("repo-archive", handler)`. User-facing trigger: `(*ArchiveRequest).Await` at `:81` polls the DB row.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — git tree walk to enumerate commit blobs, tar/zip encode, optional bundle serialization; archive size scales with repo size and can be hundreds of MB.
  - Load profile: yes — bursty (release announcement → many users hit `/archive/refs/heads/main.zip`).
  - Coherent unit: yes — `*ArchiveRequest` is value-shaped (`{Repo, Type, CommitID, Paths, archiveRefShortName}`).
  - State independence: maybe — `gitrepo.CreateArchive` shells out to `git archive`, which requires the repo on disk under `setting.RepoRootPath`; `storage.RepoArchives` is an injected `Storage` interface so the upload side is virtualized; DB row goes through `db.Insert`.
  - Latency / failure: yes — caller polls via `Await`; on-failure the row stays in `Generating` until retried.
- **Activation shape:** queue worker (`queue.CreateUniqueQueue`).
- **Confidence:** high.
- **Risk notes:** the inner goroutine + `io.Pipe` topology must run colocated with the on-disk repo, or the lift has to replace the pipe with a streamed RPC body. Codex's framing is sharper here: stream-mode and path-specific archive serving bypass the queued cache path, so the lift boundary should target `doArchive`, not all archive serving.

---

### M-3: Avatar image processing

- **pick_provenance:** claude+gemini (2/3); codex KEEPs claude's pick.
- **critique_status:** KEEP from all 3 critics; MODIFY from claude and codex on gemini's wrong line cite (`:92` → `:101`).
- **Region root:** `evaluation/gitea/modules/avatar/avatar.go:101` — `ProcessAvatarImage(data []byte) ([]byte, error)`. Trampolines into `processAvatarImage` at `:46` (the purer inner transform takes `maxOriginSize int64` as a value), which decodes via `image.Decode`, validates dimensions against `setting.Avatar.MaxWidth/MaxHeight`, square-crops and bilinear-resizes to `DefaultAvatarSize * RenderedSizeFactor`, re-encodes as PNG, and returns the smaller of original vs. resized.
- **Caller(s):** `evaluation/gitea/services/user/avatar.go:22` (inside `UploadAvatar`); `evaluation/gitea/services/repository/avatar.go:22` (repo avatar upload). Both are HTTP handler paths.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — image decode + bilinear resize on a 4K input is hundreds of milliseconds and is allocation-heavy; pure CPU.
  - Load profile: yes — bursty on signup/onboarding waves (an enterprise onboarding can produce thousands of avatar uploads in minutes).
  - Coherent unit: yes — `(data []byte) -> ([]byte, error)`. As pure as a region gets in this codebase.
  - State independence: yes — only depends on `setting.Avatar.*` (replicable config); no DB, no filesystem.
  - Latency / failure: yes — caller is a setting-update HTTP handler, not on a request critical path; tolerable to add a hop.
- **Activation shape:** HTTP route (web/api avatar setting handler).
- **Confidence:** high.
- **Risk notes:** the inner `processAvatarImage(data, maxOriginSize int64)` already takes the size as a value parameter — the actual lift target. Cleanest pure-CPU candidate in the corpus.

---

### M-4: Repository migration

- **pick_provenance:** claude+codex (2/3); gemini KEEPs both.
- **critique_status:** KEEP from all 3 critics.
- **Region root:** `evaluation/gitea/services/migrations/migrate.go:111` — `MigrateRepository(ctx, *User, ownerName, MigrateOptions, Messenger) (*Repository, error)`. Validates the source URL, builds a downloader for the source git host, creates a `GiteaLocalUploader`, calls `migrateRepository` (`:179`) which streams repo info, releases, milestones, labels, issues, PRs, comments, and wiki from upstream into the uploader.
- **Caller(s):** `evaluation/gitea/services/task/migrate.go:123` executes it inside a migration task; `evaluation/gitea/services/task/task.go:66` pushes created migration tasks to the task queue. The web migration form (`routers/web/user/home.go`) and REST API (`routers/api/v1/repo/migrate.go`) produce the tasks.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — multi-minute outbound API/git fetch; the heaviest single-call workload in the project.
  - Load profile: yes — bursty on enterprise onboarding waves; per-tenant uneven (one big GitHub org dominates).
  - Coherent unit: maybe — value-typed `MigrateOptions` and `Messenger` interface, but the dependency closure is broad (`Downloader`/`Uploader` factories).
  - State independence: yes — durable repository, package, issue, and task state; failure path uses `Rollback()` and `system_model.CreateRepositoryNotice`.
  - Latency / failure: yes — naturally async via the task queue; UI polls.
- **Activation shape:** migration task queue worker.
- **Confidence:** high — though the dependency surface is the largest in the corpus.
- **Risk notes:** `NewGiteaLocalUploader` writes the new repo to `setting.RepoRootPath` (same on-disk caveat as M-7/M-10/M-12); cancellation/progress messaging from a remote worker would need an explicit story.

---

### M-5: Code indexer (per-repo `index`)

- **pick_provenance:** claude+codex (2/3); gemini's draft picked the Bleve-specific `addUpdate` (gemini C-10) which both critics MODIFY-redirected to this same region.
- **critique_status:** KEEP from all 3 critics on `code/indexer.go:41`; gemini's own `bleve.go:142 addUpdate` was DROPped by claude and MODIFY-redirected by codex into this candidate.
- **Region root:** `evaluation/gitea/modules/indexer/code/indexer.go:41` — `index(ctx, indexer internal.Indexer, repoID int64) error`. Loads the repo, decides whether to skip (forks/mirrors/templates per config), computes the default-branch SHA, calls `getRepoChanges` to diff against the previously-indexed SHA, and pushes the change set to the active indexer (Bleve or Elasticsearch).
- **Caller(s):** `evaluation/gitea/modules/indexer/code/indexer.go:125` — invoked from the queue handler closure at `:121`, registered at `:134` via `queue.CreateUniqueQueue("code_indexer", handler)`. Producers: `services/indexer/notify.go:93` on default-branch pushes, plus the reindex cron.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — `getRepoChanges` walks `git diff-tree`/`git ls-files` between two SHAs; `indexer.Index` ships those blobs to ES (network) or writes to Bleve (local file). Both scale with churn.
  - Load profile: yes — bursty on push storms; periodic on populate cron.
  - Coherent unit: yes — `(ctx, Indexer, int64)`; `Indexer` is interface-typed, repoID is a value.
  - State independence: maybe — for ES backend, fully remote-friendly; for Bleve, the index file is local at `setting.Indexer.RepoPath` and a remote replica would diverge. Lift is clean only when ES is selected.
  - Latency / failure: yes — queue handler returns `nil` on failure to avoid re-queueing broken repos.
- **Activation shape:** queue worker (`queue.CreateUniqueQueue("code_indexer", ...)`).
- **Confidence:** high for ES-mode, medium for Bleve-mode.
- **Risk notes:** the global indexer is read via `*globalIndexer.Load()` (atomic pointer at `:33`); remote replica must initialize from the same `setting.Indexer.*` config. `getRepoChanges` shells to git on disk; same on-disk constraint as M-2/M-7. The Bleve-specific `addUpdate` (gemini C-10) is correctly subordinated under this region.

---

### M-6: Mirror pull synchronization

- **pick_provenance:** claude+codex (2/3); gemini KEEPs both.
- **critique_status:** KEEP from all 3 critics.
- **Region root:** `evaluation/gitea/services/mirror/mirror_pull.go:109` — `runSync(ctx, *Mirror) ([]*SyncResult, bool)`. Executes `git fetch --tags [--prune]` against the remote, retries with prune on broken-reference errors, writes the commit graph, opens the repo, optionally pulls LFS objects, then reconciles branch/tag refs.
- **Caller(s):** `evaluation/gitea/services/mirror/mirror_pull.go:298` — `SyncPullMirror(ctx, repoID)` at `:269` is the orchestrator (acquires `globallock` at `:280`, runs `runSync`, schedules next update). Invoked from the queue handler at `services/mirror/queue.go:33` and on the 10-minute cron at `services/cron/tasks_basic.go:31`. Mirror queue dispatch entry: `services/mirror/mirror.go:26`.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — outbound network fetch over HTTPS/SSH, bounded by `Git.Timeout.Mirror`; LFS sync at `mirror_pull.go:177` adds a second network leg whose cost scales with payload.
  - Load profile: yes — periodic (every 10m, all mirrors batched) and bursty (manual sync trigger); per-tenant uneven (one heavy mirror dominates).
  - Coherent unit: yes — `runSync(ctx, *Mirror)` is a clean unit; `*Mirror` carries the joined `*Repo` but is a value-typed model row.
  - State independence: maybe — global lock at `SyncPullMirror:280` is via `globallock` (remote-friendly backend); but `gitrepo.RunCmdString` runs the local `git` binary against the on-disk repo.
  - Latency / failure: yes — invoked async (cron + queue); failure path writes `system_model.CreateRepositoryNotice` and calls `repo_model.TouchMirror` to advance the schedule.
- **Activation shape:** queue worker (per-repo); also driven by cron `update_mirrors`.
- **Confidence:** medium — pull mechanics are clean and async, but on-disk repo dependency is a real constraint.
- **Risk notes:** repo-on-disk dependency dominates. The recoverable-error retry at `:137`–`:151` re-invokes `cmdFetch()`; any lift must be re-entrant on the same input. Wiki sync, branch-cache invalidation, and notification fan-out after updated refs increase the remote dependency set.

---

### M-7: PR mergeability check

- **pick_provenance:** codex only (1/3); claude and gemini both KEEP in critique (claude explicitly: "I missed this one and it should be in the merged set").
- **critique_status:** KEEP from claude and gemini critiques (rule 4 weak consensus, but unanimous endorsement among critics).
- **Region root:** `evaluation/gitea/services/pull/check.go:427` — `checkPullRequestMergeable(id int64)`. Loads a PR, checks whether it is still open, and runs branch mergeability/conflict checks (reaches `git merge-tree`, `diff-tree`, temporary-repo fallback, and protected-file checks through `checkPullRequestBranchMergeable`).
- **Caller(s):** `evaluation/gitea/services/pull/check.go:488` invokes it from the PR patch checker queue; `evaluation/gitea/services/pull/pull.go:422` enqueues checks after PR/head updates.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — git merge-tree + diff-tree + temp-repo fallback merge + protected-file evaluation; multi-second on non-trivial PRs.
  - Load profile: yes — base-branch update can enqueue many open PRs; per-PR cost scales with diff complexity.
  - Coherent unit: yes — queued unit is the PR ID; the function owns loading and updating PR status.
  - State independence: maybe — durable DB state plus a global PR work lock and shared git repository access.
  - Latency / failure: yes — explicitly queue-backed; conflict-check failures don't strand the system.
- **Activation shape:** background PR patch checker queue.
- **Confidence:** high — strong lift candidate if global lock and repository storage are available to replicas.
- **Risk notes:** PR status transitions and `globallock` semantics must remain exactly-once per PR check; a remote worker must not race local queue workers. Same on-disk repo caveat as M-2/M-5/M-6.

---

### M-8: Renderable git diff (`GetDiffForRender`)

- **pick_provenance:** codex+gemini (2/3); claude's draft picked the inner `ParsePatch` (claude C-10), which both critics MODIFY-redirected to this larger envelope.
- **critique_status:** KEEP from claude and gemini on codex's pick; KEEP from claude and codex on gemini's pick. Claude's own C-10 (`ParsePatch`) was MODIFY-redirected by codex into this same target; claude's critique acknowledges `GetDiffForRender` is structurally a *better* pick because it bundles parsing, attribute checks, and Chroma highlighting under one envelope.
- **Region root:** `evaluation/gitea/services/gitdiff/gitdiff.go:1333` — `GetDiffForRender(ctx, repoLink string, gitRepo *git.Repository, opts *DiffOptions, files ...string) (*Diff, error)`. Runs `git diff`, parses the unified patch (via `ParsePatch` at `:631`), checks attributes, detects generated/vendor files, and applies Chroma highlighting within limits.
- **Caller(s):** `evaluation/gitea/routers/web/repo/pull.go:805` (PR files view); also the commit/compare diff handlers under `routers/web/repo/`.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — runs `git diff`, parses unified patches across MB-scale streams on big PRs, applies Chroma highlighting (CPU- and allocation-bound).
  - Load profile: yes — large PRs and commit ranges create highly variable diff size and parsing/highlighting cost; spikes on monorepo refactor PRs and bots that scrape diffs.
  - Coherent unit: yes — inputs are repository link, git repository, diff options, and optional file filters; output is a `*Diff`.
  - State independence: maybe — derived from git object state; requires an open repository and attribute checker.
  - Latency / failure: maybe — synchronous page-render path, though heavy diff pages already tolerate comparatively high latency and truncation.
- **Activation shape:** synchronous web handler (PR/commit/compare diff rendering).
- **Confidence:** medium — the work is real and proportional to payload, but request-path streaming and local git process management make the lift more marginal than queue-backed candidates.
- **Risk notes:** caller passes an `io.Reader` connected to a running `git diff` command's stdout; lifting requires shared repository access and careful cancellation so abandoned requests kill the git process. Subordinate region `services/gitdiff/highlightdiff.go:152 diffLineWithHighlight` (gemini C-7) is correctly captured under this envelope and excluded as a standalone pick.

---

### M-9: RPM repository metadata rebuild

- **pick_provenance:** codex only (1/3); claude and gemini both KEEP in critique (claude explicitly missed; gemini KEEPs and offers a parallel Debian candidate as M-19).
- **critique_status:** KEEP from claude and gemini.
- **Region root:** `evaluation/gitea/services/packages/rpm/repository.go:163` — `BuildSpecificRepositoryFiles(ctx, ownerID int64, group string) error`. Scans package files, unmarshals cached metadata, emits primary/filelists/other/updateinfo XML, gzips data, signs `repomd.xml`.
- **Caller(s):** `evaluation/gitea/routers/api/packages/rpm/rpm.go:212` rebuilds after RPM upload; `:313` rebuilds after RPM file deletion.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — XML emit + gzip + GPG signing over the package set; cost grows with package count per owner/group.
  - Load profile: yes — bursty during CI publishing waves.
  - Coherent unit: yes — owner ID + RPM group as value inputs; durable package storage.
  - State independence: yes — reads/writes go through package models/storage and signing key lookup.
  - Latency / failure: maybe — upload/delete routes currently expect the metadata rebuild to finish before returning (a synchronous compatibility shim may be needed so package clients see fresh metadata immediately).
- **Activation shape:** package upload/delete HTTP route post-processing.
- **Confidence:** medium — bounded and expensive, but synchronous-on-HTTP-path is the soft spot.
- **Risk notes:** OpenPGP key handling and overwrite semantics for repository metadata files must stay atomic from the client perspective.

---

### M-10: Push-update worker

- **pick_provenance:** claude only (1/3); codex and gemini both KEEP in critique.
- **critique_status:** KEEP from codex and gemini.
- **Region root:** `evaluation/gitea/services/repository/push.go:77` — `pushUpdates(optsList []*PushUpdateOptions) error`. Opens the git repo, updates repo size, iterates each `PushUpdateOptions` in the batch, resolves pushers, walks new commits, generates push action history, fires per-tag/branch downstream hooks (notify, webhook enqueue, indexer enqueue).
- **Caller(s):** `evaluation/gitea/services/repository/push.go:38` — the queue handler `handler` registered at `:48` via `queue.CreateSimpleQueue("push_update", handler)`. Producers: post-receive hooks via `PushUpdates` (`:62`).
- **Why useful (rubric scoring):**
  - Compute envelope: yes — opens git repo, enumerates commits since old SHA, can be expensive on a 10k-commit force-push; cascades downstream queue tasks (webhook, indexer, actions).
  - Load profile: yes — proportional to push activity; bursty around CI green-light campaigns.
  - Coherent unit: yes — slice of value-typed `*PushUpdateOptions`, returns error.
  - State independence: maybe — opens git repo via `gitrepo.OpenRepository`; same on-disk constraint as M-6.
  - Latency / failure: yes — queue-backed; the function is the producer for downstream queues, not a synchronous-critical caller.
- **Activation shape:** queue worker (`queue.CreateSimpleQueue("push_update", handler)`).
- **Confidence:** medium.
- **Risk notes:** triggers a fan-out of follow-up enqueues; the lifted unit must produce the same downstream effects, meaning the lift either preserves access to those queue handles (DB-backed queue makes this easy) or accepts that the side-effecting calls happen in the local proxy. `UpdateRepoSize` writes to the DB.

---

### M-11: Issue indexer per-item handler

- **pick_provenance:** claude only (1/3); codex and gemini both KEEP in critique (codex: "this was not in my draft but passes").
- **critique_status:** KEEP from codex and gemini.
- **Region root:** `evaluation/gitea/modules/indexer/issues/indexer.go:166` — `getIssueIndexerQueueHandler(ctx)` returns the per-item handler that calls `getIssueIndexerData(ctx, item.ID)` (loads the issue and all its comments/labels/etc) and then `indexer.Index(ctx, data)` or `indexer.Delete(ctx, item.IDs...)`.
- **Caller(s):** registered at `evaluation/gitea/modules/indexer/issues/indexer.go:70` via `queue.CreateUniqueQueue("issue_indexer", getIssueIndexerQueueHandler(ctx))`. Producers: issue/comment create/update/delete paths and `PopulateIssueIndexer`.
- **Why useful (rubric scoring):**
  - Compute envelope: maybe — per-item is a DB read (issue + comments) plus a write to ES/Bleve/Meilisearch; small for short issues, large for issues with thousands of comments.
  - Load profile: yes — bursty on comment activity; the handler returns unhandled items for retry, which can spike under indexer-backend backpressure.
  - Coherent unit: yes — closure body operates on `*IndexerMetadata` (`{ID, IDs, IsDelete}`), value-typed.
  - State independence: yes for ES/Meili backends; Bleve caveat from M-5 applies.
  - Latency / failure: yes — queue-backed with explicit retry list.
- **Activation shape:** queue worker (`queue.CreateUniqueQueue("issue_indexer", ...)`).
- **Confidence:** medium — per-item DB load can be heavy for big issues but is small in the common case.
- **Risk notes:** `getIssueIndexerData` loads associated comments/attachments; a remote replica would issue cross-table reads against the shared DB, which is the same hop already paid by every Gitea request. Bleve-backend caveat from M-5 applies.

---

### M-12: Repository language statistics

- **pick_provenance:** gemini only (1/3); claude and codex both KEEP in critique.
- **critique_status:** KEEP from claude and codex (claude: "I missed this; belongs in the merged set").
- **Region root:** `evaluation/gitea/modules/git/languagestats/language_stats_nogogit.go:22` — `GetLanguageStats(repo *git.Repository, commitID string) (map[string]int64, error)`. Crawls a repository tree to calculate the percentage of each language used (runs `enry` heuristics on each blob via `cat-file --batch`).
- **Caller(s):** `evaluation/gitea/modules/indexer/stats/db.go:62` — `DBIndexer.Index` (background language-stats indexer).
- **Why useful (rubric scoring):**
  - Compute envelope: yes — crawls thousands of files and runs heuristics on each.
  - Load profile: yes — periodic or triggered by first push.
  - Coherent unit: yes — takes a repository and a commit ID.
  - State independence: maybe — needs access to the git repository.
  - Latency / failure: yes — background indexing task, not on user-facing critical path.
- **Activation shape:** background language-stats indexer queue worker.
- **Confidence:** high in compute, medium in operational story — same shared-git-storage caveat as other repo-indexing candidates.
- **Risk notes:** performance depends on `git cat-file` batching efficiency over the network if lifted; on-disk repo dependency same as M-2/M-5/M-6.

---

### M-13: Mailer send (`sender.send`)

- **pick_provenance:** claude only (1/3); codex MODIFY (reframe around named `sender.send` instead of anonymous queue closure); gemini KEEP.
- **critique_status:** KEEP from gemini, MODIFY from codex (apply: lift root is `services/mailer/sender/sender.go:17` rather than the anonymous queue closure at `services/mailer/mailer.go:48`).
- **Region root:** `evaluation/gitea/services/mailer/sender/sender.go:17` — `send(sender Sender, msg *Message) error`. Renders the message and dispatches to the configured `Sender` (one of `SMTPSender`, `SendmailSender`, `DummySender`); SMTP path performs the TLS handshake and `SMTPSender.Send` at `services/mailer/sender/smtp.go:27`.
- **Caller(s):** `evaluation/gitea/services/mailer/mailer.go:48` — the queue handler closure inside `NewContext`, registered via `queue.CreateSimpleQueue("mail", ...)`. Producers: everything in `services/mailer/mail_*.go` (e.g. `mail_issue.go:186 SendIssueAssignedMail`) which call `SendAsync` → `mailQueue.Push`.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — template rendering happens in the producer, but the worker still does an SMTP TLS handshake + send per message; bounded but non-trivial under fan-out.
  - Load profile: yes — bursty on issue/PR activity, comment storms, password-reset campaigns.
  - Coherent unit: yes — `Sender` interface and value-typed `*Message`.
  - State independence: yes — sender is replica-local; SMTP is a stateless outbound call.
  - Latency / failure: yes — queue-backed; failures are logged and dropped.
- **Activation shape:** queue worker (`queue.CreateSimpleQueue("mail", ...)` invokes `sender.send` per item).
- **Confidence:** high.
- **Risk notes:** SMTP keepalive/connection pooling state lives inside `SMTPSender`; if the lift maintains a connection pool per replica, that's correct, but rate-limit accounting (if any) would also be per-replica.

---

### M-14: Actions workflow detection

- **pick_provenance:** codex only (1/3); claude and gemini both KEEP in critique (claude with `compute envelope: maybe`).
- **critique_status:** KEEP from claude and gemini.
- **Region root:** `evaluation/gitea/modules/actions/workflows.go:120` — `DetectWorkflows(...)`. Lists workflow files in the repository, reads each workflow, parses events, and matches them against a triggering event/payload.
- **Caller(s):** `evaluation/gitea/services/actions/notifier_helper.go:186` detects workflows for an event; `:221` detects base-branch workflows for pull-request target handling.
- **Why useful (rubric scoring):**
  - Compute envelope: maybe — recursive workflow file listing, file reads, YAML parsing, event-filter evaluation. Most workflow YAML is modest, but Actions-busy repos with many workflow files compound the per-event cost (PR-target double-detect at `:221`).
  - Load profile: yes — push and PR event bursts vary by repository workflow count and workflow file size.
  - Coherent unit: yes — inputs are a git repository, commit, event type, payload, and schedule flag; outputs are detected workflow lists.
  - State independence: maybe — mostly pure over git objects and payload; remote workers need git object access.
  - Latency / failure: maybe — runs in the action notifier path; failures affect workflow scheduling rather than the original git-object write.
- **Activation shape:** notification-triggered Actions workflow discovery (synchronous within the notifier path).
- **Confidence:** medium — clean parser/filter unit, but caller is synchronous with notification handling.
- **Risk notes:** payload type assertions and event-specific matching must behave identically remotely; git object access is the main coupling.

---

### M-15: Mirror LFS object synchronization

- **pick_provenance:** codex only (1/3); claude MODIFY (overlap with M-6 — this is a sub-region of `runSync`); gemini KEEP ("valuable specialization of mirror sync").
- **critique_status:** KEEP from gemini, MODIFY from claude (record overlap explicitly).
- **Region root:** `evaluation/gitea/modules/repository/repo.go:61` — `StoreMissingLfsObjectsInRepository(ctx, *Repository, *git.Repository, lfs.Client) error`. Enumerates pointer blobs, batches LFS downloads, creates LFS metadata, writes object content.
- **Caller(s):** `evaluation/gitea/services/mirror/mirror_pull.go:177` — invoked inside `runSync` (M-6) when LFS mirroring is enabled.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — enumerates pointer blobs, batches LFS downloads, creates LFS metadata, writes object content.
  - Load profile: yes — LFS repositories have highly skewed object counts and sizes across tenants and pushes.
  - Coherent unit: yes — inputs are a repository, opened git repository, and LFS client.
  - State independence: maybe — writes are durable, but replicas need access to the same content store and git object graph.
  - Latency / failure: yes — runs inside mirror background work and tolerates missing upstream LFS objects.
- **Activation shape:** subtask of M-6 background pull mirror synchronization.
- **Confidence:** medium — strong payload-scaled subregion, but only one direct production caller and overlaps M-6.
- **Risk notes:** **Overlap with M-6:** this is the IO-scaled inner loop where lift utility concentrates within M-6's `runSync`; if M-6 is lifted, the LFS step likely lifts with it. Kept as a separate candidate to record the sub-region annotation, not as an independent activation path. Object storage, upstream LFS client credentials, and repository pointer enumeration must all be available remotely.

---

### M-16: Password hashing (Argon2)

- **pick_provenance:** gemini only (1/3); claude and codex both KEEP in critique.
- **critique_status:** KEEP from claude and codex (codex notes latency/failure should stay `maybe` because this is synchronous authentication work).
- **Region root:** `evaluation/gitea/modules/auth/password/hash/argon2.go:29` — `(*Argon2Hasher).HashWithSaltBytes(password string, salt []byte) string`. Performs the actual Argon2ID key derivation. (Gemini's `:32` was off; correct declaration line is `:29`.)
- **Caller(s):** `evaluation/gitea/modules/auth/password/hash/hash.go:51` — `PasswordHashAlgorithm.Hash`. Invoked from login/signup/password-change paths.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — designed to be slow and memory-intensive (~100 ms+) to resist brute force.
  - Load profile: yes — spikes during signup/login storms or under brute-force attack.
  - Coherent unit: yes — takes a password and salt, returns a hex string.
  - State independence: yes — stateless transformation.
  - Latency / failure: maybe — synchronous login path, but the lift hop (~10–20 ms) is small relative to the hash cost; the lift mainly helps isolate CPU saturation.
- **Activation shape:** HTTP POST handler (login/signup).
- **Confidence:** medium-high — offloading expensive crypto is a standard scaling pattern, conditioned on the secret-transport story.
- **Risk notes:** secret-transport (password and salt) over the lift boundary needs an authenticated/encrypted channel; latency adds ~10–20 ms to a 100 ms+ hash; failure path is a clean error.

---

### M-17: Syntax highlighting (full-file / slow-guess)

- **pick_provenance:** gemini only (1/3); claude MODIFY (line cite and scope — pick `RenderCodeSlowGuess` at `:85` or `RenderFullFile` at `:124`, note overlap with M-8 highlight path); codex MODIFY (reframe as `RenderFullFile` for code-view or `RenderCodeSlowGuess` for blame).
- **critique_status:** MODIFY from both critics (real target, fix scope). Aggregator judgment: rule 4 doesn't strictly apply (no KEEP), but rule 5 doesn't apply either (no DROP). Defended on rubric grounds: compute envelope is unambiguously yes (Chroma tokenization on large files), and the activation shape is distinct from M-8's diff highlighting (standalone code-preview / blame handlers).
- **Region root:** `evaluation/gitea/modules/highlight/highlight.go:124` — `RenderFullFile(fileName, language string, code []byte) ([]template.HTML, string)` (file-view path). Sibling: `evaluation/gitea/modules/highlight/highlight.go:85 RenderCodeSlowGuess` (blame-view path). Both run Chroma tokenization and formatting on full file content.
- **Caller(s):** `evaluation/gitea/routers/web/repo/view_file.go:124` (file-view); `evaluation/gitea/routers/web/repo/blame.go:270` (blame-view). Distinct from the diff-highlight chain in M-8.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — Chroma tokenization and formatting is CPU-bound and slow for large files.
  - Load profile: yes — triggered whenever users view code files in the browser.
  - Coherent unit: yes — function takes strings/bytes and returns `template.HTML`.
  - State independence: yes — depends on static styles and lexer mapping.
  - Latency / failure: maybe — synchronous request path; benefit is conditional on file size.
- **Activation shape:** synchronous web handler (file-view, blame-view).
- **Confidence:** medium — the function name `SlowGuess` itself signals latency tolerance; large dependency closure (Chroma) is exactly what lifting helps isolate.
- **Risk notes:** overlaps M-8 within the diff-rendering chain (M-8's `GetDiffForRender` calls `highlightCodeLines` → `RenderCodeByLexer`); kept here only because the standalone code-preview/blame activation shape is independent.

---

### M-18: Markdown render (`render`)

- **pick_provenance:** claude+codex+gemini (3/3) — but at three different points in the rendering stack. Claude picked the outer dispatcher (`modules/markup/render.go:193 Render`); codex picked the markdown-specific inner `render` at `modules/markup/markdown/markdown.go:186`; gemini picked the same inner `render` but cited `:155` (wrong line). Critics MODIFY-converged on `markdown.go:186 render`.
- **critique_status:** KEEP-after-MODIFY from all 3 critics. Codex MODIFY on claude's outer-dispatcher framing → use markdown-specific `:186`. Claude+codex MODIFY on gemini's `:155` line → fix to `:186`. Latency/failure should stay `maybe`, not unconditional yes.
- **Region root:** `evaluation/gitea/modules/markup/markdown/markdown.go:186` — `render(ctx *markup.RenderContext, input io.Reader, output io.Writer) error`. The Goldmark-based markdown rendering inner function, exposed publicly via `Render` at `:264` and `RenderString` at `:270`.
- **Caller(s):** `evaluation/gitea/modules/markup/markdown/markdown.go:259` (`Renderer.Render`) and the `Render`/`RenderString` wrappers; ultimately invoked from issue/comment view handlers, wiki page handlers, README rendering, and `routers/common/markup.go:98` (preview/markup endpoints).
- **Why useful (rubric scoring):**
  - Compute envelope: yes — Goldmark parse + syntax-highlight + sanitize; CPU-bound and scales with content length.
  - Load profile: yes — every page render; per-tenant uneven (one big wiki dominates).
  - Coherent unit: yes — `RenderContext` is per-call, `io.Reader`/`io.Writer` are values.
  - State independence: maybe — sanitizer/render helpers and repo-aware link handling can consult repository context, but most rendering is content-local.
  - Latency / failure: maybe (or no for small inputs) — most calls render small inputs (an issue body, a comment) where a network RPC would dominate the render itself. Lift utility is conditional on payload size and would require gating dispatch by input length.
- **Activation shape:** synchronous web handler.
- **Confidence:** low — included for completeness because the underlying function passes the structural rubric and 3/3 picks converge here, but the latency story argues against lifting it unconditionally. Useful as **negative evidence** about when an extra hop dominates the work.
- **Risk notes:** the post-processor reads `RenderContext.RenderOptions.Metas` (`map[string]string`) and writes to `output` (often the response writer); lifting requires either a buffered string return (use `RenderString` form) or streaming over RPC.

---

### M-19: Debian repository metadata rebuild

- **pick_provenance:** OVERLOOKED by gemini (rule 7 — single-critic OVERLOOKED with unambiguous rubric scoring).
- **critique_status:** Included per rule 7: rubric scoring is `compute yes, load yes, coherent yes, state yes, latency maybe` — all five criteria yes/maybe, no `no`. Mirrors codex's M-9 (RPM) candidate exactly in structure and load profile.
- **Region root:** `evaluation/gitea/services/packages/debian/repository.go:154` — `BuildSpecificRepositoryFiles(ctx, ownerID int64, distribution, component, architecture string) error`. Walks package database, generates Gzipped `Packages`/`Release` files, performs PGP signing.
- **Caller(s):** `evaluation/gitea/routers/api/packages/debian/debian.go:121` (after upload); `:204` (after delete).
- **Why useful (rubric scoring):**
  - Compute envelope: yes — walks package database, gzip + PGP signing.
  - Load profile: yes — triggered by package uploads; cost scales with total package count in the distribution/component.
  - Coherent unit: yes — clean input parameters (owner, distribution, component, arch).
  - State independence: yes — reads/writes go through package storage models.
  - Latency / failure: maybe — synchronous on the upload path; same compatibility-shim concern as M-9.
- **Activation shape:** HTTP route post-processing (package upload/delete).
- **Confidence:** high (parallel of M-9; same rubric profile).
- **Risk notes:** requires access to signing keys and shared package storage; same atomicity-from-client-perspective concern as M-9.

---

### M-20: PR merge-and-push (`doMergeAndPush`) — *disputed*

- **pick_provenance:** claude only (1/3); codex DROP (structurally worse than M-7 mergeability check — destructive, hook-coupled, temp-repo/filesystem-heavy on a correctness-sensitive path); gemini KEEP (compute envelope: heavy git orchestration).
- **critique_status:** KEEP/DROP split; aggregator judgment per rule 3-style "disputed" inclusion. Defended on rubric grounds: `latency / failure: yes` (caller is already O(seconds), an extra hop is in the noise; failure path goes through `AddTestPullRequestTask` defer), and `compute envelope: yes` (multi-second per non-trivial PR). The codex DROP reasoning is real (correctness-sensitive, hook-coupled), so the candidate is included with explicit dispute annotation rather than treated as a clean pick. See "Discrepancies" below.
- **Region root:** `evaluation/gitea/services/pull/merge.go:334` — `doMergeAndPush(ctx, *PullRequest, *User, MergeStyle, expectedHeadCommitID, message, PushTrigger) (string, error)`. Builds a temporary repo via `createTemporaryRepoForMerge`, dispatches to one of `doMergeStyleMerge`/`doMergeStyleRebase`/`doMergeStyleSquash`/`doMergeStyleFastForwardOnly`, captures the merge head SHA, force-pushes back to base.
- **Caller(s):** `evaluation/gitea/services/pull/merge.go:267` — invoked from `Merge` (`:223`), which holds `globallock` for `getPullWorkingLockKey(pr.ID)`. `Merge` is invoked from web/api PR-merge handlers and the `automergequeue` worker.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — clones base and head into a temp dir, runs git merge/rebase/squash, computes commit, pushes refs; multi-second per call on non-trivial PRs.
  - Load profile: maybe — bursty around release windows; otherwise modest.
  - Coherent unit: maybe — `*PullRequest` requires a fully-loaded relationship graph (`LoadBaseRepo`/`LoadHeadRepo`), more state-pull than a pure `(repoID, prID)`.
  - State independence: maybe — caller holds `globallock` (remote-friendly); merge runs against a temp directory under `setting.RepoRootPath/temp`.
  - Latency / failure: yes — already O(seconds); duplicate `AddTestPullRequestTask` in `Merge`'s defer means a failure here doesn't strand the system.
- **Activation shape:** HTTP route (web/api PR merge) and queue worker (auto-merge).
- **Confidence:** low — disputed; codex's argument that M-7 (mergeability check) captures the expensive PR/git computation with a cleaner failure model is rubric-grounded.
- **Risk notes:** the temp-dir strategy plus the post-receive hook re-invocation (`:273` reloads the PR after the hook fires) means the lift must run colocated with post-receive plumbing or accept that the post-receive notification arrives after the remote call returns. The "DUPLICATE-PR-TASK" comment at `:255` flags this exact coupling.

---

### M-21: NPM package upload parsing — *disputed*

- **pick_provenance:** codex only (1/3); claude KEEP (low-confidence marginal — pair with payload-size threshold); gemini DROP (utility of offloading SHA-512 + Base64 likely outweighed by the latency and bandwidth cost of moving the entire package payload).
- **critique_status:** KEEP/DROP split; aggregator judgment per rule 3-style "disputed" inclusion. Defended on rubric grounds: `compute envelope: yes` (JSON decode + base64 + SHA-1/SHA-512 over the tarball is real CPU on large packages), but gemini's DROP is also rubric-grounded (compute envelope vs. data-transfer cost). Included with explicit dispute annotation as borderline marginal evidence; useful only under conditional dispatch by payload size.
- **Region root:** `evaluation/gitea/modules/packages/npm/creator.go:203` — `ParsePackage(r io.Reader) (*Package, error)`. Decodes JSON, base64-decodes the tarball attachment, hashes the full data with SHA-1 or SHA-512 for integrity verification.
- **Caller(s):** `evaluation/gitea/routers/api/packages/npm/npm.go:157` — invoked at the start of npm package upload.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — JSON decode + base64 + SHA-1/SHA-512 over the tarball.
  - Load profile: yes — npm publish traffic is bursty in CI and cost scales with package tarball size.
  - Coherent unit: yes — input is an `io.Reader`, output is a parsed package struct.
  - State independence: yes — parsing and hash verification are pure with no durable side effects.
  - Latency / failure: maybe — upload request-path work; client is already sending a payload-sized body.
- **Activation shape:** package registry HTTP upload parser.
- **Confidence:** low — disputed; useful as evidence about when an extra hop's bandwidth cost negates the compute saving.
- **Risk notes:** the current implementation materializes the decoded tarball in memory; a lifted version should avoid adding another full-copy boundary. Lift utility is genuinely conditional on payload size — exactly the case where a runtime oracle gating dispatch would be needed.

---

## Discrepancies

### D-1: Markdown render — region-root layer

Claude picked the *outer* `markup.Render` dispatcher at `modules/markup/render.go:193`; codex and gemini picked the *inner* markdown-specific `render` at `modules/markup/markdown/markdown.go:186`. Critics converged on the inner pick (codex MODIFY on claude's outer pick; claude itself acknowledges in its critique that the inner pick is the cleaner lift seam because it skips the type-detect/dispatch shell). **I sided with the inner pick (`markdown.go:186`)**, the rubric criterion being *coherent unit*: the inner `render` has a focused contract (a single markup type, a single Goldmark pipeline) whereas the outer dispatcher subsumes type detection and renderer-registry lookup that aren't lift-meaningful work.

### D-2: Renderable diff — narrow vs. enclosing scope

Claude originally picked the inner `ParsePatch` (`gitdiff.go:631`); codex and gemini picked the enclosing `GetDiffForRender` (`gitdiff.go:1333`). Codex MODIFY-redirected claude's pick to the enclosing scope; claude's critique then explicitly conceded that the enclosing scope is structurally a better pick because it bundles git execution, patch parsing, attribute checks, and Chroma highlighting under one envelope. **I sided with the enclosing `GetDiffForRender`** as M-8, on rubric *compute envelope*: the Chroma pass and attribute checks are where a meaningful share of large-PR latency lives, and isolating only the parser substep loses that work.

### D-3: PR merge-and-push (M-20) vs. PR mergeability check (M-7)

Codex DROP'd claude's PR merge-and-push (M-20) on the grounds that it is "destructive, hook-coupled, temp-repo/filesystem-heavy merge machinery on a correctness-sensitive path", and that codex's own PR mergeability check (M-7) is structurally a cleaner pick that captures the expensive PR/git computation with a cleaner failure model. Gemini KEPT M-20 on compute-envelope grounds. **I included both, with M-7 as the high-confidence pick (M-7 ranks 7th) and M-20 as a low-confidence disputed pick (ranks 20th)**; this reflects codex's argument that the *clean* lift target is M-7, while preserving M-20 as evidence about a structurally adjacent region whose latency tolerance is real but whose state coupling and correctness sensitivity make it a weaker candidate. The rubric criterion that justifies keeping M-20 at all is *latency / failure: yes* — the duplicate-PR-task defer makes failure recoverable.

### D-4: NPM upload parsing (M-21)

Claude KEPT (low-confidence marginal), gemini DROPped. Gemini's argument is that the data-transfer cost of moving a tens-of-MB tarball negates the compute saving — a rubric-grounded argument against *compute envelope* (when the work-to-transfer ratio is unfavourable). **I included with explicit dispute annotation**, on the rubric grounds that *compute envelope: yes* is technically satisfied (SHA-512 over a multi-MB payload is real CPU work), but flagged the candidate as low-confidence and useful only under conditional dispatch by payload size. M-21 is best read as evidence about *when* a lift overhead exceeds the benefit, rather than as a clean lift target.

### D-5: Mailer queue framing (M-13)

Codex MODIFY'd claude's mailer pick (anonymous queue closure in `mailer.go`) on the grounds that the lift target should be a named callable region, suggesting `services/mailer/sender/sender.go:17 send` (or `smtp.go:27 SMTPSender.Send`) with the queue closure as the caller. Gemini KEPT claude's framing as-is. **I applied the codex MODIFY**, on the rubric grounds that *coherent unit* is best satisfied by a named function with a clear input/output contract rather than an anonymous closure.

### D-6: Mirror LFS sync (M-15) — overlap concern

Claude MODIFY'd codex's pick to record explicit overlap with M-6 (`runSync`), since the LFS sync is invoked inside `runSync`. Gemini KEPT as a "valuable specialization" of mirror sync. **I included M-15 with the overlap annotation**, on the rubric grounds that the LFS step is the IO-scaled inner loop where lift utility concentrates within M-6, but it does not have an independent activation path. The aggregate evidence is that this is a *sub-region annotation* on M-6, not a parallel candidate.

### D-7: Bleve `addUpdate` (gemini C-10) collapsed into M-5

Gemini picked the Bleve-specific `addUpdate` at `bleve.go:142`. Claude DROPped (subordinate to `code/indexer.go:41`, plus state-independence is wrong for Bleve since output is a local index file). Codex MODIFY'd in the same direction (replace with `code/indexer.go:41`). **I excluded as a standalone candidate** and folded the activation path into M-5; the rubric criterion is *state independence*, on which gemini's `yes` was incorrect for the Bleve backend.

---

## Excluded candidates

- **gemini C-7 (`services/gitdiff/highlightdiff.go:152 diffLineWithHighlight`)** — both critics DROP. Per-line private helper, too fine-grained for a remote hop; M-8 (`GetDiffForRender`) is the coherent enclosing region.
- **gemini C-10 (`modules/indexer/code/bleve/bleve.go:142 addUpdate`)** — both critics rejected (claude DROP, codex MODIFY-redirect). Subordinate to M-5; gemini's state-independence scoring was incorrect for the Bleve backend.
- **claude C-10 (`services/gitdiff/gitdiff.go:631 ParsePatch`)** — MODIFY-redirected by codex into M-8 (`GetDiffForRender`); not retained as a separate candidate. The enclosing scope is structurally a better lift target.
- **claude C-12 (`modules/markup/render.go:193 markup.Render`)** — MODIFY-redirected by codex into M-18 (markdown-specific `render` at `markdown/markdown.go:186`). The outer dispatcher is the wrong lift seam.
- **gemini C-1 (`modules/markup/markdown/markdown.go:155`)** — line cite drift; MODIFY corrections fold into M-18 at `:186`.
- **gemini C-5 (`services/webhook/deliver.go:125`)** — line cite drift; MODIFY corrections fold into M-1 at `:153`.
- **gemini C-6 (`services/repository/archiver/archiver.go:120`)** — line cite drift; MODIFY corrections fold into M-2 at `:146`.
- **gemini C-3 (`modules/avatar/avatar.go:92`)** — line cite drift; MODIFY corrections fold into M-3 at `:101`.
- **gemini C-4 (`modules/highlight/highlight.go:102`)** — line cite drift and scope; MODIFY corrections fold into M-17 at `:124` (and `:85`).

(No candidate was excluded under rule 5 "1/3 picked + all other critics DROP" — every 1/3 pick was KEPT or MODIFY'd by at least one other critic.)
