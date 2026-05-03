# Lift Utility Corpus: Gitea

Project read: Gitea is a comprehensive self-hosted Git service providing repository management, code review (PRs), issue tracking, and a built-in CI/CD engine. Computationally expensive paths cluster around Git data processing (diffing, blame, archive generation), rich content rendering (Markdown, Org-mode), search indexing (Bleve/Elasticsearch), and security (password hashing, avatar processing). These tasks are often bursty, triggered by user actions like pushing code, opening pull requests, or updating profiles, and are loosely coupled enough to be viable lift candidates.

### C-1: Markdown Rendering

- **Region root:** `evaluation/gitea/modules/markup/markdown/markdown.go:155` — `render` function which converts raw markdown bytes into HTML using the Goldmark library and Gitea-specific extensions.
- **Caller(s):** `evaluation/gitea/modules/markup/markdown/markdown.go:218` (Renderer.Render)
- **Why useful (rubric scoring):**
  - Compute envelope: yes — CPU-intensive parsing and HTML generation proportional to input size.
  - Load profile: yes — Bursty based on user activity (viewing READMEs, issues, PR comments).
  - Coherent unit: yes — Clean `io.Reader`/`io.Writer` interface with a `RenderContext`.
  - State independence: yes — Depends on configuration and context but not on pervasive in-process mutable state.
  - Latency / failure: yes — Markdown rendering is already a non-trivial part of request latency; an extra hop is tolerable.
- **Activation shape:** HTTP route handler (indirectly via markup middleware).
- **Confidence:** high — Standard "heavy" task in any web application.
- **Risk notes:** The `RenderContext` contains some metadata that needs to be serialized, but the core logic is pure transformation.

### C-2: Git Diff Parsing

- **Region root:** `evaluation/gitea/services/gitdiff/gitdiff.go:1333` — `GetDiffForRender` generates a structured diff object from a git repository for UI display.
- **Caller(s):** `evaluation/gitea/routers/web/repo/diff.go` (various handlers)
- **Why useful (rubric scoring):**
  - Compute envelope: yes — Heavy parsing of git diff output, especially for large commits or PRs.
  - Load profile: yes — Concentrated on PR reviews and commit inspections.
  - Coherent unit: yes — Takes a repository and options, returns a structured `Diff` object.
  - State independence: maybe — Requires access to the git repository (filesystem), but the parsing logic itself is independent.
  - Latency / failure: yes — PR diffs can be slow; offloading parsing helps keep the web process responsive.
- **Activation shape:** HTTP route handler.
- **Confidence:** high — Diffs are one of the most resource-intensive parts of Gitea's UI.
- **Risk notes:** Depends on `git.Repository` which usually points to a local path; virtualization or a remote git service would be needed.

### C-3: Avatar Image Processing

- **Region root:** `evaluation/gitea/modules/avatar/avatar.go:92` — `ProcessAvatarImage` handles image decoding, cropping, resizing, and encoding to PNG.
- **Caller(s):** `evaluation/gitea/services/user/avatar.go:42` (UploadAvatar)
- **Why useful (rubric scoring):**
  - Compute envelope: yes — Image processing (scaling/encoding) is CPU and memory intensive.
  - Load profile: yes — Highly bursty during user profile updates or organization creation.
  - Coherent unit: yes — Takes a `[]byte` and returns a `[]byte`.
  - State independence: yes — Purely functional image transformation.
  - Latency / failure: yes — Naturally async-capable or slightly slower response on upload is acceptable.
- **Activation shape:** HTTP POST handler.
- **Confidence:** high — Classic case for offloading to a dedicated worker.
- **Risk notes:** Minimal risk; very clean separation.

### C-4: Syntax Highlighting

- **Region root:** `evaluation/gitea/modules/highlight/highlight.go:102` — `RenderCodeSlowGuess` uses Chroma to detect and highlight code based on filename, language, and content.
- **Caller(s):** `evaluation/gitea/modules/markup/html_codepreview.go:104` (renderCodePreview)
- **Why useful (rubric scoring):**
  - Compute envelope: yes — Chroma tokenization and formatting is CPU-bound and slow for large files.
  - Load profile: yes — Triggered whenever users view code files in the browser.
  - Coherent unit: yes — Function takes strings/bytes and returns `template.HTML`.
  - State independence: yes — Depends on static styles and mapping.
  - Latency / failure: yes — Already marked "SlowGuess" in the name, implying latency tolerance.
- **Activation shape:** HTTP route handler.
- **Confidence:** high — Very expensive per-call cost for large source files.
- **Risk notes:** Large dependency closure (Chroma), which is exactly what lifting helps isolate.

### C-5: Webhook Delivery

- **Region root:** `evaluation/gitea/services/webhook/deliver.go:125` — `Deliver` executes the HTTP request for a webhook task and records the response.
- **Caller(s):** `evaluation/gitea/services/webhook/deliver.go:278` (handler for the worker queue)
- **Why useful (rubric scoring):**
  - Compute envelope: maybe — Mostly IO-bound but involves payload signing (HMAC-SHA256).
  - Load profile: yes — Massive fan-out spikes during push events to popular repos.
  - Coherent unit: yes — Operates on a single `HookTask`.
  - State independence: yes — Completely independent network call.
  - Latency / failure: yes — Async queue-worker context makes it perfect for remote execution.
- **Activation shape:** Queue worker.
- **Confidence:** high — Standard practice to scale webhook delivery independently of the main API.
- **Risk notes:** Requires outbound network access from the remote replica.

### C-6: Repository Archive Generation

- **Region root:** `evaluation/gitea/services/repository/archiver/archiver.go:120` — `doArchive` generates a ZIP or TAR.GZ archive of a repository at a specific commit.
- **Caller(s):** `evaluation/gitea/services/repository/archiver/archiver.go:189` (archiverQueue handler)
- **Why useful (rubric scoring):**
  - Compute envelope: yes — Highly CPU and IO intensive compression work.
  - Load profile: yes — Infrequent but heavy "spiky" load.
  - Coherent unit: yes — Bounded by `ArchiveRequest`.
  - State independence: maybe — Requires git repo access and storage access.
  - Latency / failure: yes — Already handled as an async background task with status tracking.
- **Activation shape:** Queue worker.
- **Confidence:** high — One of the heaviest background tasks in the system.
- **Risk notes:** Needs efficient access to repository data (e.g. via a shared volume or Git-over-RPC).

### C-7: Highlighted Code Diffing

- **Region root:** `evaluation/gitea/services/gitdiff/highlightdiff.go:120` — `diffLineWithHighlight` performs a semantic diff on code that has already been highlighted, preserving HTML tags.
- **Caller(s):** `evaluation/gitea/services/gitdiff/gitdiff.go:501` (DiffFile.Highlight)
- **Why useful (rubric scoring):**
  - Compute envelope: yes — Complex algorithm (DiffMatchPatch) combined with HTML tag preservation logic.
  - Load profile: yes — Viewing side-by-side diffs in large PRs.
  - Coherent unit: yes — Operates on two `template.HTML` inputs.
  - State independence: yes — Purely computational.
  - Latency / failure: yes — This is often the bottleneck in PR page load times.
- **Activation shape:** HTTP route handler.
- **Confidence:** medium — Computationally expensive but requires very specific input formatting.
- **Risk notes:** Tight coupling to how Chroma emits HTML tags.

### C-8: Password Hashing (Argon2)

- **Region root:** `evaluation/gitea/modules/auth/password/hash/argon2.go:32` — `Argon2Hasher.HashWithSaltBytes` performs the actual Argon2ID key derivation.
- **Caller(s):** `evaluation/gitea/modules/auth/password/hash/hash.go:51` (PasswordHashAlgorithm.Hash)
- **Why useful (rubric scoring):**
  - Compute envelope: yes — Designed to be slow and memory-intensive to prevent brute-force.
  - Load profile: yes — Spikes during login/signup or when being target of a brute-force attack.
  - Coherent unit: yes — Takes a password and salt, returns a hex string.
  - State independence: yes — Stateless transformation.
  - Latency / failure: maybe — On the synchronous login path, but the lift helps isolate CPU saturation.
- **Activation shape:** HTTP POST handler (Login/Signup).
- **Confidence:** high — Offloading expensive crypto is a standard security/scaling pattern.
- **Risk notes:** Adds 10-20ms network latency to a 100ms+ hashing operation.

### C-9: Repository Language Statistics

- **Region root:** `evaluation/gitea/modules/git/languagestats/language_stats_nogogit.go:21` — `GetLanguageStats` crawls a repository tree to calculate the percentage of each language used.
- **Caller(s):** `evaluation/gitea/modules/indexer/stats/db.go:62` (DBIndexer.Index)
- **Why useful (rubric scoring):**
  - Compute envelope: yes — Crawling thousands of files and running heuristics on each.
  - Load profile: yes — Periodic or triggered by first push.
  - Coherent unit: yes — Takes a repository and a commit ID.
  - State independence: maybe — Needs access to the Git repository.
  - Latency / failure: yes — Background indexing task.
- **Activation shape:** Queue worker.
- **Confidence:** high — Very heavy task that doesn't need to block any user-facing operation.
- **Risk notes:** Performance depends on Git cat-file batching efficiency over the network if lifted.

### C-10: Search Indexing (Content Analysis)

- **Region root:** `evaluation/gitea/modules/indexer/code/bleve/bleve.go:142` — `addUpdate` extracts content from Git blobs, detects language, and prepares it for the indexer.
- **Caller(s):** `evaluation/gitea/modules/indexer/code/bleve/bleve.go:210` (Indexer.Index)
- **Why useful (rubric scoring):**
  - Compute envelope: yes — Content extraction, charset conversion, and language detection.
  - Load profile: yes — Bursty spikes on pushes.
  - Coherent unit: yes — Processes a single file update.
  - State independence: yes — Bounded inputs, output goes to an indexer (which could be remote like Elasticsearch).
  - Latency / failure: yes — Background worker.
- **Activation shape:** Queue worker.
- **Confidence:** high — Moving the "preparation" of searchable documents off-process is highly beneficial.
- **Risk notes:** High volume of calls during initial indexing of large repositories.

Honest assessment: I am most confident about the "pure" computational candidates like Markdown rendering, Image processing, and Syntax highlighting (C-1, C-3, C-4, C-7). These have very clean boundaries and clear CPU-bound profiles. The Git-related tasks (C-2, C-6, C-9) are genuinely great lift candidates in terms of work, but they present a "state coupling" challenge because they require repository access; in a real Monolift deployment, this would necessitate a shared filesystem or a Git-RPC layer. Webhook delivery (C-5) is a classic lift but leans more towards IO-bound scaling. I suspect the "Actions" runner logic (in `services/actions`) is also a prime candidate, but it was too complex to isolate a single "unit of work" without deeper investigation into the runner-orchestrator contract.
