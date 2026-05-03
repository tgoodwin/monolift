# PocketBase Lift Utility Survey

PocketBase is an open-source backend-in-a-box that provides a real-time database, authentication, and file storage. It is built as a single Go binary that bundles SQLite, HTTP routing, and various tools. The computationally expensive paths cluster around image processing (thumbnailing), authentication (password hashing, OAuth2), backup creation (compression and IO), and its embedded JavaScript engine (JSVM) which allows users to run custom business logic via hooks.

### C-1: Image Thumbnail Generation

- **Region root:** `evaluation/pocketbase/tools/filesystem/filesystem.go:489` — `filesystem.(*System).CreateThumb` - Resizes an image to a specified size and uploads it to the active filesystem.
- **Caller(s):** `evaluation/pocketbase/apis/file.go:148`
- **Why useful (rubric scoring):**
  - Compute envelope: yes — Heavy CPU and memory usage for image decoding, resizing, and encoding (JPEG/PNG/GIF).
  - Load profile: yes — Triggered on-demand when a user requests a file with a `thumb` parameter; highly variable and bursty.
  - Coherent unit: yes — Clean interface taking keys and size strings; interacts with storage via a defined `bucket` abstraction.
  - State independence: yes — Reads/writes from/to potentially remote storage (S3); no local-only mutable state.
  - Latency / failure: yes — Managed by a semaphore and singleflight; added network hop is small compared to image processing time.
- **Activation shape (informational, not a selection criterion):** HTTP route handler (indirectly via a singleflight group in `fileApi`).
- **Confidence:** high — Clear CPU-bound work that is naturally separable from the main request flow.
- **Risk notes:** Requires access to the same storage backend (S3 or shared filesystem).

### C-2: Application Backup Creation

- **Region root:** `evaluation/pocketbase/core/base_backup.go:44` — `core.(*BaseApp).CreateBackup` - Generates a zip archive of the `pb_data` directory and uploads it to the backups storage.
- **Caller(s):** `evaluation/pocketbase/apis/backup_create.go:30`
- **Why useful (rubric scoring):**
  - Compute envelope: yes — Involves walking the filesystem and compressing large amounts of data (zip/deflate).
  - Load profile: yes — Triggered manually via API or periodically via cron; load depends on the size of the data directory.
  - Coherent unit: yes — Self-contained process with a clear beginning and end; uses an internal transaction to ensure consistency.
  - State independence: maybe — Blocks writes during the archive phase; lifting might require a shared-nothing or read-only snapshot strategy.
  - Latency / failure: yes — Long-running background-compatible task; natural failure model with error reporting.
- **Activation shape (informational, not a selection criterion):** HTTP route handler (superuser) or cron-registered closure.
- **Confidence:** high — Extremely heavy task that can significantly impact local CPU and IO.
- **Risk notes:** The current implementation uses a transaction to block writes, which might be harder to coordinate remotely.

### C-3: Email Template Resolution

- **Region root:** `evaluation/pocketbase/mails/record.go:251` — `mails.resolveEmailTemplate` - Resolves system and record placeholders in email templates and wraps them in a layout.
- **Caller(s):** `evaluation/pocketbase/mails/record.go:21`, `evaluation/pocketbase/mails/record.go:48`
- **Why useful (rubric scoring):**
  - Compute envelope: maybe — Involves HTML escaping and template rendering for potentially complex data structures.
  - Load profile: yes — Bursty flurries of password resets, verifications, or OTP requests.
  - Coherent unit: yes — Pure function-like behavior taking a record, a template, and placeholders; returns strings.
  - State independence: yes — Only depends on the inputs provided; no global side effects.
  - Latency / failure: yes — Often part of an async flow (though not always); extra hop is negligible compared to SMTP latency.
- **Activation shape (informational, not a selection criterion):** Request-triggered goroutines (via `Trigger` on mailer events).
- **Confidence:** medium — While not extremely heavy, it aggregates well with the subsequent SMTP send which is IO-bound.
- **Risk notes:** Minimal; a very clean candidate for extraction.

### C-4: Password Validation (Bcrypt)

- **Region root:** `evaluation/pocketbase/core/record_model_auth.go:78` — `core.(*Record).ValidatePassword` - Verifies a plain text password against a stored hash using `bcrypt`.
- **Caller(s):** `evaluation/pocketbase/apis/record_auth_with_password.go:82`
- **Why useful (rubric scoring):**
  - Compute envelope: yes — Bcrypt is intentionally CPU-intensive to slow down brute-force attacks.
  - Load profile: yes — Spikable during login flurries or automated credential stuffing attacks.
  - Coherent unit: yes — Simple boolean check taking a string; encapsulated in the record model.
  - State independence: yes — Only requires the stored hash (already in the record) and the plain password.
  - Latency / failure: yes — Bcrypt already adds ~100ms+ of latency; a network hop is within the same order of magnitude.
- **Activation shape (informational, not a selection criterion):** HTTP route handler (authentication).
- **Confidence:** high — Purely CPU-bound and isolated logic.
- **Risk notes:** Extremely frequent call path; might need efficient caching of the remote result.

### C-5: OAuth2 Identity Processing

- **Region root:** `evaluation/pocketbase/apis/record_auth_with_oauth2.go:30` — `apis.recordAuthWithOAuth2` - Handles the exchange of an auth code for tokens and fetching user info from external providers.
- **Caller(s):** Registered in `bindRecordAuthApi` in `apis/record_auth.go`.
- **Why useful (rubric scoring):**
  - Compute envelope: maybe — Dominated by outbound network IO to external providers (Google, GitHub, etc.).
  - Load profile: yes — Bursty depending on external traffic and campaign launches.
  - Coherent unit: maybe — Large function but with a clear goal of converting an OAuth2 code into a authenticated record.
  - State independence: yes — Primarily interacts with external APIs and the internal DB via well-defined methods.
  - Latency / failure: yes — Caller is already waiting on multiple external round-trips; extra latency is minor.
- **Activation shape (informational, not a selection criterion):** HTTP route handler.
- **Confidence:** medium — More IO-bound than CPU-bound, but benefits from offloading the "waiting" state.
- **Risk notes:** Dependency on external provider availability and potentially complex context propagation.

### C-6: Apple Client Secret Generation

- **Region root:** `evaluation/pocketbase/forms/apple_client_secret_create.go:63` — `forms.(*AppleClientSecretCreate).Submit` - Generates a JWT signed with an ES256 private key for Apple OAuth2.
- **Caller(s):** Used when configuring or updating Apple Auth settings.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — Involves parsing EC private keys and performing cryptographic signing.
  - Load profile: maybe — Infrequent but computationally distinct from standard CRUD.
  - Coherent unit: yes — Encapsulated in a form struct with clear inputs (keys, IDs) and a string output.
  - State independence: yes — Only uses the provided form fields.
  - Latency / failure: yes — Synchronous but acceptable for a settings update path.
- **Activation shape (informational, not a selection criterion):** HTTP route handler (settings management).
- **Confidence:** medium — Very clean unit, but low frequency.
- **Risk notes:** Handling of sensitive private keys needs secure transport.

### C-7: JavaScript Hook Execution

- **Region root:** `evaluation/pocketbase/plugins/jsvm/binds.go:81` — Anonymous JS handler wrapper - Executes a compiled JavaScript program in a `goja` runtime.
- **Caller(s):** Any framework hook point registered by the user (e.g., `onRecordBeforeCreateRequest`).
- **Why useful (rubric scoring):**
  - Compute envelope: yes — Executing arbitrary JS in an embedded VM is significantly more expensive than native Go.
  - Load profile: yes — Entirely dependent on user-defined logic; can be arbitrarily heavy.
  - Coherent unit: yes — Wrapped in a generic handler that passes arguments and captures return values.
  - State independence: maybe — JS logic might try to access local state, but PocketBase exposes state via a bounded `$app` interface.
  - Latency / failure: yes — Users expect hooks to have some overhead; can be made async depending on the hook type.
- **Activation shape (informational, not a selection criterion):** Any framework hook point.
- **Confidence:** high — The most unpredictable compute consumer in PocketBase.
- **Risk notes:** Requires careful serialization of the `$app` and event objects across the network.

### C-8: Directory Archiving (Zip)

- **Region root:** `evaluation/pocketbase/tools/archive/create.go:18` — `archive.Create` - Creates a zip archive from a source directory.
- **Caller(s):** `evaluation/pocketbase/core/base_backup.go:76`
- **Why useful (rubric scoring):**
  - Compute envelope: yes — Intensive CPU (compression) and IO (walking and reading directory tree).
  - Load profile: yes — Burst-heavy during backup operations.
  - Coherent unit: yes — Pure utility function taking paths and return an error.
  - State independence: yes — Operates purely on the filesystem paths provided.
  - Latency / failure: yes — Long-running task; added network overhead is negligible.
- **Activation shape (informational, not a selection criterion):** Utility function used by Backup API and Cron.
- **Confidence:** high — Highly focused unit of work with clear resource demands.
- **Risk notes:** Requires the remote worker to have access to the same `pb_data` (e.g. via shared volume).

## Honest Assessment

The most confident candidates are **C-1 (Image Thumbnail Generation)** and **C-4 (Password Validation)**. These are textbook cases of separable, CPU-bound work that scale poorly on a single machine but can be easily offloaded. **C-7 (JavaScript Hook Execution)** is also a very strong candidate because it represents the "unknown" load in the system—users can write expensive loops or transformations in JS that the main binary shouldn't have to absorb.

**C-5 (OAuth2 Identity Processing)** is genuinely marginal because it is primarily IO-bound, and its utility depends on how many external authentications the system handles. Similarly, **C-6 (Apple Client Secret Generation)** is computationally interesting (crypto) but likely too low-frequency to justify a lift in most real-world scenarios.

One region that is likely a great lift candidate but hard to justify with the current rubric is **the entire `realtime` (SSE) event dispatching**. While it holds long-lived connections (a disqualifier), the *fan-out* logic that computes which events go to which subscribers is often very expensive in highly-active systems. However, since the rubric explicitly excludes persistent connections, I've left it out.
