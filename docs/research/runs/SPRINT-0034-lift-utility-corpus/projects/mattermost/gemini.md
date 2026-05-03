### Project read
Mattermost is a complex, enterprise-grade collaborative messaging platform written in Go. Its computationally expensive paths cluster around media processing (image resizing and thumbnailing), content indexing/search, and background administrative tasks such as compliance exporting and analytics aggregation. The system's architecture is heavily based on an `App` struct that orchestrates various services (Store, Search, Email, LDAP), many of which are invoked asynchronously via goroutines or background workers, making them ideal candidates for the Monolift "lift and offload" model.

### C-1: Image Thumbnail Generation

- **Region root:** `server/channels/app/file.go:1184` — `App.generateThumbnailImage` handles the decoding, resizing, and encoding of image thumbnails.
- **Caller(s):** `server/channels/app/file.go:1151` — Called during file upload processing.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — Heavy CPU work involving image decoding and Lanczos resizing.
  - Load profile: yes — Highly bursty based on user upload activity.
  - Coherent unit: yes — Clear inputs (image, path) and uses defined app interfaces for writing results.
  - State independence: yes — Relies on service interfaces (`a.WriteFile`) which can be remotely satisfied.
  - Latency / failure: yes — Usually called in a goroutine; thumbnail delay is acceptable to the user.
- **Activation shape:** Goroutine launched from file upload handler.
- **Confidence:** high — Classic lift candidate for any media-heavy application.
- **Risk notes:** Depends on `image.Image` interface which requires serialization support if passed directly.

### C-2: Document Content Extraction

- **Region root:** `server/channels/app/file.go:1624` — `App.ExtractContentFromFileInfo` extracts text from PDFs, Office docs, and archives for indexing.
- **Caller(s):** `server/channels/app/file.go:862`, `1129` — Called during post-processing of uploaded files.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — High CPU/Memory cost for parsing complex file formats (PDF, DOCX).
  - Load profile: yes — Spiky based on document upload volume.
  - Coherent unit: yes — Bounded by file ID and result string; uses `docextractor` service.
  - State independence: yes — Reads from file store and writes text back to DB via Store interface.
  - Latency / failure: yes — Purely background task for search indexing; non-critical path.
- **Activation shape:** Background worker or post-upload goroutine.
- **Confidence:** high — Parsing untrusted, complex documents is exactly what should be offloaded.
- **Risk notes:** Large dependency closure due to document parsing libraries.

### C-3: Link Preview (OpenGraph) Generation

- **Region root:** `server/channels/app/post_metadata.go:892` — `App.getLinkMetadata` fetches and parses external URLs to generate link previews.
- **Caller(s):** `server/channels/app/post_metadata.go:566`, `663` — Called during post metadata enrichment.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — IO-bound (outbound HTTP) and CPU-bound (HTML/OpenGraph parsing).
  - Load profile: yes — Bursty based on message frequency containing links.
  - Coherent unit: yes — Input is a URL, output is metadata struct.
  - State independence: yes — Stateless except for a local cache that can be made replica-local.
  - Latency / failure: yes — Slow by nature; extra hop is negligible compared to outbound HTTP.
- **Activation shape:** Synchronous or async metadata enrichment.
- **Confidence:** high — Offloading outbound IO and parsing protects the main process from slow upstreams.
- **Risk notes:** Requires outbound network access from the remote replica.

### C-4: Markdown to HTML Conversion

- **Region root:** `server/channels/utils/markdown.go:57` — `utils.MarkdownToHTML` converts user-provided markdown into HTML for display.
- **Caller(s):** `server/channels/app/email/notification_email.go:153` — Used for rendering email notifications.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — CPU-intensive string processing and regex replacement for large messages.
  - Load profile: yes — Scales with message volume and notification frequency.
  - Coherent unit: yes — Pure functional unit: string in, string out.
  - State independence: yes — Zero side effects.
  - Latency / failure: yes — Already adds ~ms latency; offloading is fine for async paths like email.
- **Activation shape:** Helper function used in various app flows.
- **Confidence:** high — Perfectly stateless and CPU-bound.
- **Risk notes:** None; this is a trivial lift if serialization of strings is efficient.

### C-5: Notification Email Body Rendering

- **Region root:** `server/channels/app/email/notification_email.go:30` — `Service.GetMessageForNotification` renders complex email templates with post content.
- **Caller(s):** `server/channels/app/notification_email.go:121` — Called when building notification emails.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — Combines template rendering, i18n translation, and markdown conversion.
  - Load profile: yes — Highly variable; peaks with mentions and system-wide notifications.
  - Coherent unit: yes — Method on email Service with well-defined inputs.
  - State independence: yes — Reads from Store/Config; purely constructive work.
  - Latency / failure: yes — Email delivery is inherently async and high-latency.
- **Activation shape:** Background worker / goroutine.
- **Confidence:** medium — Large input objects (Post, User, Team) might increase serialization cost.
- **Risk notes:** Needs access to the i18n translation maps and system config.

### C-6: Global Search Result Processing

- **Region root:** `server/channels/app/post.go:2127` — `App.SearchPostsForUser` orchestrates and filters results from search engines.
- **Caller(s):** `server/channels/api4/post.go:1145` — HTTP handler for searching posts.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — Orchestration, merging results from multiple engines, and permission filtering.
  - Load profile: yes — Very spiky; user search behavior is highly unpredictable.
  - Coherent unit: yes — High-level entry point for search logic.
  - State independence: maybe — Depends on `Store` and `SearchEngine` interfaces.
  - Latency / failure: yes — Search is expected to take 100ms+; users are tolerant of variable latency.
- **Activation shape:** API route handler.
- **Confidence:** medium — The complexity of the `Store` and `SearchEngine` interfaces may complicate lifting.
- **Risk notes:** Passing `finalParamsList` and context might be heavy.

### C-7: Password Hashing (PBKDF2)

- **Region root:** `server/channels/app/password/hashers/pbkdf2.go:151` — `PBKDF2.Hash` performs the actual work-intensive password hashing.
- **Caller(s):** `server/channels/app/password/hashers/hashers.go:141` — Top-level hashing entry point.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — Intentionally CPU-heavy (600,000 iterations of SHA256).
  - Load profile: maybe — Bursty during login storms, though usually rate-limited.
  - Coherent unit: yes — Purely functional: password in, hash out.
  - State independence: yes — Stateless.
  - Latency / failure: yes — Hashing already takes O(100ms); extra hop is unnoticeable.
- **Activation shape:** Synchronous on login/password-change path.
- **Confidence:** high — The quintessential CPU-bound lift candidate.
- **Risk notes:** Very low risk due to simple interface.

### C-8: Analytics Aggregation

- **Region root:** `server/channels/app/analytics.go:21` — `App.GetAnalytics` aggregates system-wide statistics for reporting.
- **Caller(s):** `server/channels/api4/analytics.go:28` — API endpoint for admin dashboard.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — Significant DB aggregation and in-memory processing of result sets.
  - Load profile: yes — Periodic; triggered by admins loading dashboards.
  - Coherent unit: yes — Simple input parameters (name, teamID).
  - State independence: yes — Read-only from Store.
  - Latency / failure: yes — Admin dashboards are not latency-critical.
- **Activation shape:** API route handler.
- **Confidence:** medium — Effectiveness depends on how much aggregation happens in Go vs. the DB.
- **Risk notes:** If all work is in SQL, the lift benefit is minimal.

### C-9: LDAP Password Verification

- **Region root:** `server/channels/app/authentication.go:186` — `checkLdapUserPasswordAndAllCriteria` orchestrates the verification of LDAP credentials.
- **Caller(s):** `server/channels/app/authentication.go:119` — Main login flow for LDAP users.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — IO-bound (external LDAP round-trip) and logic-heavy (checking MFA, criteria).
  - Load profile: yes — Peak load during start of business hours.
  - Coherent unit: yes — Clear inputs (User, password, token).
  - State independence: yes — Relies on LDAP interface.
  - Latency / failure: yes — LDAP latency is high; adding a hop is acceptable.
- **Activation shape:** Synchronous login path.
- **Confidence:** medium — High utility for scaling login capacity, but depends on external network connectivity.
- **Risk notes:** Needs access to the LDAP service provider.

### Honest assessment
I am most confident in the **Image Thumbnail Generation (C-1)** and **Document Extraction (C-2)** candidates. These represent clearly bounded, heavy CPU/IO units of work that are already commonly offloaded in large-scale systems. The **Markdown to HTML (C-4)** and **Password Hashing (C-7)** candidates are the most "pure" in terms of state independence and would be trivial to implement. **Search Orchestration (C-6)** and **LDAP Verification (C-9)** are genuinely marginal because their utility is highly dependent on how much work is done in the Go process versus the external engine (DB/LDAP server). A candidate that I suspect is a great lift target but couldn't include is **Audit Log Streaming/Writing**; it's likely very bursty and IO-heavy, but the rubric's "state independence" and "failure model" requirements are hard to satisfy without deeper evidence of how Mattermost handles audit durability guarantees during failures.
