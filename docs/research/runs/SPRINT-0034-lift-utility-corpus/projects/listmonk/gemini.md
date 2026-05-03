Project read: listmonk is a self-hosted newsletter and mailing list manager. Its computationally expensive paths cluster around campaign execution (template rendering for thousands of subscribers), high-volume message delivery (SMTP and webhooks), and data ingestion (importing large CSV files). The system is designed with a manager-worker pattern where core logic units are handed off to concurrent processes, making it a fertile ground for region lifting.

### C-1: (m *Manager) NewCampaignMessage

- **Region root:** `internal/manager/message.go:13` — `NewCampaignMessage` initializes and renders a campaign message for a specific subscriber using Go templates.
- **Caller(s):** `internal/manager/pipe.go:174` (inside the subscriber iteration loop).
- **Why useful (rubric scoring):**
  - Compute envelope: yes — Involves executing Go templates for both subject and body, potentially multiple times if an alt-body exists.
  - Load profile: yes — Highly bursty; campaign starts trigger thousands to millions of renders in a tight loop.
  - Coherent unit: yes — Clear contract: takes a Campaign and a Subscriber, returns a CampaignMessage.
  - State independence: yes — Depends on the template and subscriber data, both of which are passed in; does not mutate global state.
  - Latency / failure: yes — Render errors are handled per-subscriber; adding a hop is negligible compared to the total campaign time.
- **Activation shape:** worker-goroutine-dispatched unit.
- **Confidence:** high — This is the quintessential Monolift candidate: heavy, repetitive, and stateless.
- **Risk notes:** None; the templates are pre-compiled and attached to the campaign object.

### C-2: (c *Campaign) ConvertContent

- **Region root:** `models/campaigns.go:214` — `ConvertContent` transforms campaign body content from Markdown to HTML.
- **Caller(s):** `cmd/campaigns.go:213` (when creating or updating a campaign).
- **Why useful (rubric scoring):**
  - Compute envelope: yes — CPU-bound markdown-to-HTML transformation using an external library.
  - Load profile: maybe — Typically called on user action (saving a campaign), but can be spiky if many campaigns are drafted or updated via API.
  - Coherent unit: yes — Bounded string-to-string transformation.
  - State independence: yes — Functional transformation with no side effects.
  - Latency / failure: yes — Sync request path but O(10ms-100ms) work; a network hop is acceptable.
- **Activation shape:** HTTP route handler synchronous path.
- **Confidence:** high — Markdown parsing is a textbook example of a liftable compute-bound task.
- **Risk notes:** Relies on the `markdown` library; ensures the dependency is portable.

### C-3: processImage

- **Region root:** `cmd/media.go:212` — `processImage` decodes an uploaded image and generates a PNG thumbnail.
- **Caller(s):** `cmd/media.go:49` (inside `UploadMedia` handler).
- **Why useful (rubric scoring):**
  - Compute envelope: yes — CPU and memory intensive image decoding, resizing (Lanczos resampling), and PNG encoding.
  - Load profile: yes — Bursty based on user uploads of media assets.
  - Coherent unit: yes — Takes a multipart file header, returns a reader for the processed bytes and dimensions.
  - State independence: yes — Purely functional image processing.
  - Latency / failure: yes — Users expect upload/processing delay; failures are easily catchable and retryable.
- **Activation shape:** HTTP route handler (upload).
- **Confidence:** high — Image processing is one of the most common reasons for scaling separate workers.
- **Risk notes:** Uses the `imaging` library which may have a larger memory footprint for large images.

### C-4: (e *Emailer) Push

- **Region root:** `internal/messenger/email/email.go:111` — `Push` prepares a MIME message and dispatches it via an SMTP connection pool.
- **Caller(s):** `internal/manager/manager.go:527` (inside the main worker loop).
- **Why useful (rubric scoring):**
  - Compute envelope: maybe — Preparing MIME headers and body is light, but the network IO of SMTP delivery is the primary cost.
  - Load profile: yes — Extremely bursty during campaign sends.
  - Coherent unit: yes — Part of a clean `Messenger` interface.
  - State independence: yes — Uses a connection pool that can be local to the remote replica.
  - Latency / failure: yes — Asynchronous background task with a clear failure model.
- **Activation shape:** worker-goroutine dispatch.
- **Confidence:** medium — The benefit depends heavily on whether the SMTP pool can be efficiently managed remotely.
- **Risk notes:** Depends on an external SMTP server; network proximity between the lifted region and the SMTP server becomes critical.

### C-5: (p *Postback) Push

- **Region root:** `internal/messenger/postback/postback.go:97` — `Push` marshals a campaign message to JSON and dispatches it as an HTTP POST webhook.
- **Caller(s):** `internal/manager/manager.go:527` (inside the main worker loop).
- **Why useful (rubric scoring):**
  - Compute envelope: maybe — JSON marshaling (easyjson) plus outbound HTTP IO.
  - Load profile: yes — Bursty fan-out during campaign sends.
  - Coherent unit: yes — Clean interface-based dispatch.
  - State independence: yes — Entirely dependent on input message and config.
  - Latency / failure: yes — Naturally async and tolerant of network-induced latency.
- **Activation shape:** worker-goroutine dispatch.
- **Confidence:** high — Webhook fan-out is a classic candidate for independent scaling to handle varied upstream latencies.
- **Risk notes:** None; very standard pattern.

### C-6: classifyBounce

- **Region root:** `internal/bounce/mailbox/pop.go:214` — `classifyBounce` uses multiple regex patterns to categorize an email body as a hard or soft bounce.
- **Caller(s):** `internal/bounce/mailbox/pop.go:142` (while scanning a bounce mailbox).
- **Why useful (rubric scoring):**
  - Compute envelope: maybe — Multiple regex matches over potentially large raw email bodies.
  - Load profile: maybe — Periodic spikes when the mailbox scanner runs.
  - Coherent unit: yes — Pure function: `[]byte -> (string, string)`.
  - State independence: yes — No side effects or global state dependencies.
  - Latency / failure: yes — Background scanning task.
- **Activation shape:** cron/periodic scanner goroutine.
- **Confidence:** high — A very "clean" candidate for lifting due to its pure-functional nature.
- **Risk notes:** None; very low risk.

### C-7: (s *Session) LoadCSV

- **Region root:** `internal/subimporter/importer.go:452` — `LoadCSV` parses a CSV file and streams subscriber records into a processing queue.
- **Caller(s):** `cmd/import.go:101`, `cmd/import.go:115`.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — Parsing large files, validating fields, and unmarshaling JSON attributes.
  - Load profile: yes — Rare but heavy: triggered by manual CSV imports which can involve millions of rows.
  - Coherent unit: maybe — Heavily tied to the `Session` and its `subQueue` channel.
  - State independence: maybe — Relies on a local file path; sends results via an internal channel.
  - Latency / failure: yes — Always runs in a background goroutine.
- **Activation shape:** background importer goroutine.
- **Confidence:** medium — Useful work, but the channel-based coupling makes it a "heavy" lift.
- **Risk notes:** The use of a channel (`s.subQueue`) to return results requires the Monolift runtime to bridge channels across nodes or requires lifting the consumer as well.

### C-8: (s *Session) ExtractZIP

- **Region root:** `internal/subimporter/importer.go:373` — `ExtractZIP` decompresses a ZIP archive and identifies CSV files for import.
- **Caller(s):** `cmd/import.go:95`.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — Decompression (CPU) and file IO.
  - Load profile: yes — Triggered on user upload of compressed subscriber lists.
  - Coherent unit: yes — Takes a source path, returns a list of files.
  - State independence: no — Writes to a local temp directory (`os.MkdirTemp`).
  - Latency / failure: yes — Background task.
- **Activation shape:** background importer goroutine.
- **Confidence:** low — Included because the work is heavy, but the filesystem dependency is a major disqualifier for naive lifting.
- **Risk notes:** Requires a shared filesystem or the ability to return the extracted directory contents across the wire.

Honest assessment: I am most confident in `NewCampaignMessage`, `processImage`, and `ConvertContent`; these are high-utility, CPU-bound units that are perfectly decoupled. `Emailer.Push` and `Postback.Push` are great for scaling IO, though they carry less "compute" weight. `LoadCSV` and `ExtractZIP` are genuinely marginal due to their tight coupling with local resources (channels and temp files), but are included because they represent the most significant occasional load on the system. I suspect the SQL query generation and explain-plan parsing in `subscribers.go` (e.g., `traverseQueryPlan`) is also a great candidate for lifting if it were more complex, but in this codebase, it seems relatively lightweight compared to the template and image work.
