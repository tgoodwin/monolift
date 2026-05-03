Project read: listmonk is a Go newsletter and mailing-list server with campaign delivery, subscriber import/export, media upload, transactional mail, and bounce processing paths. The strongest lift candidates cluster around asynchronous campaign fan-out in `internal/manager`, bulk subscriber import in `internal/subimporter`, outbound messenger backends in `internal/messenger`, and bounce processors in `internal/bounce`. The codebase already separates many expensive units behind functions or small interfaces, especially `manager.Messenger`, importer sessions, and webhook processors. Request-path candidates exist too, but I ranked background or naturally batched work higher because Monolift's extra network hop fits those failure and latency models better.

### C-1: Campaign message render per subscriber

- **Region root:** `evaluation/listmonk/internal/manager/message.go:13` — `(*Manager).NewCampaignMessage` builds and renders one campaign message for one subscriber.
- **Caller(s):** `evaluation/listmonk/internal/manager/pipe.go:173` calls it from the campaign pipe; `evaluation/listmonk/cmd/campaigns.go:653` calls it for test campaign sends.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — executes compiled subject, body, and optional alt-body templates and copies rendered bytes with cost proportional to template and subscriber data size.
  - Load profile: yes — campaign sends fan out across subscriber batches and can spike heavily for large lists.
  - Coherent unit: yes — inputs are a campaign and subscriber, and output is a `CampaignMessage`.
  - State independence: maybe — render-time template functions may call manager config and link tracking, so the link cache/store coupling would need an explicit remote contract.
  - Latency / failure: yes — the main caller is the background campaign pipe before enqueueing to send workers.
- **Activation shape (informational, not a selection criterion):** campaign manager batch worker.
- **Confidence:** high — this is a central per-recipient CPU path; only hidden template function side effects could lower feasibility.
- **Risk notes:** `TrackLink` can register links through manager state during render, so a lift should either pre-register links or make the link store/cache explicitly remote-safe.

### C-2: SMTP message push

- **Region root:** `evaluation/listmonk/internal/messenger/email/email.go:111` — `(*Emailer).Push` turns a `models.Message` into an SMTP email and sends it.
- **Caller(s):** `evaluation/listmonk/internal/manager/manager.go:523` invokes messenger `Push` from campaign workers; `evaluation/listmonk/cmd/public.go:629` directly pushes the subscriber data export email.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — copies attachments, builds headers/envelopes, and performs an outbound SMTP send whose latency dominates per message.
  - Load profile: yes — campaign delivery creates bursty fan-out proportional to recipient count.
  - Coherent unit: yes — `Push(models.Message) error` is already an interface method with a clear input/output contract.
  - State independence: maybe — SMTP pools and server selection are process-local but can be rebuilt from config on a remote replica.
  - Latency / failure: yes — campaign sends are processed by background workers with per-message error handling.
- **Activation shape (informational, not a selection criterion):** messenger interface method called by queue workers.
- **Confidence:** high — this is a classic async outbound-network unit; the main caveat is preserving SMTP pool configuration and rate behavior.
- **Risk notes:** remote replicas must mirror SMTP configuration and any per-provider connection limits; random server selection from multiple SMTP servers should remain acceptable.

### C-3: Bulk CSV subscriber load

- **Region root:** `evaluation/listmonk/internal/subimporter/importer.go:452` — `(*Session).LoadCSV` counts, parses, validates, and queues subscriber rows from an uploaded CSV.
- **Caller(s):** `evaluation/listmonk/cmd/import.go:101` starts it for CSV uploads; `evaluation/listmonk/cmd/import.go:115` starts it after ZIP extraction.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — scans the file for line counts, parses CSV rows, validates email/domain rules, unmarshals JSON attributes, and queues every subscriber.
  - Load profile: yes — imports are tenant-driven and can jump from tiny lists to very large subscriber files.
  - Coherent unit: yes — takes a source path and delimiter and emits `SubReq` entries into the session queue.
  - State independence: maybe — it mutates importer status and writes to a session channel, but those are per-import session state rather than global application state.
  - Latency / failure: yes — the HTTP handler starts it in a goroutine and returns import status immediately.
- **Activation shape (informational, not a selection criterion):** async goroutine launched by upload handler.
- **Confidence:** high — this is a bounded, expensive batch transform with visible progress state.
- **Risk notes:** the current API passes a temp file path; a remote lift would need shared object storage or pass the uploaded bytes/stream explicitly.

### C-4: HTTP postback message push

- **Region root:** `evaluation/listmonk/internal/messenger/postback/postback.go:97` — `(*Postback).Push` marshals a message payload and posts it to an HTTP messenger endpoint.
- **Caller(s):** `evaluation/listmonk/internal/manager/manager.go:523` calls messenger `Push` from campaign workers; `evaluation/listmonk/cmd/init.go:711` registers postback messengers from config.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — copies attachments, marshals JSON with easyjson, and performs an outbound HTTP request.
  - Load profile: yes — postback delivery can fan out for every campaign recipient and burst during campaign launches.
  - Coherent unit: yes — `Push(models.Message) error` is a compact interface method.
  - State independence: yes — it depends mainly on postback options and an HTTP client, both replica-local.
  - Latency / failure: yes — campaign worker callers already tolerate per-message send failures.
- **Activation shape (informational, not a selection criterion):** messenger interface method called by queue workers.
- **Confidence:** high — the method is already a remote-call-like boundary around another remote HTTP call.
- **Risk notes:** retries are configured at the messenger layer but not visible in this method; lift orchestration should avoid double retry storms.

### C-5: Media thumbnail generation

- **Region root:** `evaluation/listmonk/cmd/media.go:212` — `processImage` decodes an uploaded image, resizes it, encodes a PNG thumbnail, and reports dimensions.
- **Caller(s):** `evaluation/listmonk/cmd/media.go:99` invokes it from `UploadMedia` for image uploads.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — image decode, Lanczos resize, and PNG encode scale with uploaded image dimensions.
  - Load profile: yes — media uploads are user-driven and can spike around campaign authoring.
  - Coherent unit: yes — takes a multipart file header and returns thumbnail bytes plus width and height.
  - State independence: yes — no app state is mutated inside the image transform.
  - Latency / failure: maybe — it is synchronous in the upload request, but image processing latency is already user-visible and often large enough to hide an extra hop.
- **Activation shape (informational, not a selection criterion):** HTTP upload handler helper.
- **Confidence:** high — this is the cleanest CPU-bound leaf function in the surveyed tree.
- **Risk notes:** the current input is a multipart file handle; a lift would be cleaner if the caller passed bytes or an object-store reference.

### C-6: POP bounce mailbox scan

- **Region root:** `evaluation/listmonk/internal/bounce/mailbox/pop.go:79` — `(*POP).Scan` downloads, parses, classifies, queues, and deletes POP bounce messages.
- **Caller(s):** `evaluation/listmonk/internal/bounce/bounce.go:138` calls `m.mailbox.Scan(1000, m.queue)` from the mailbox scanner loop.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — each scan does network retrieval, MIME parsing, multipart traversal, regex header/body matching, JSON metadata assembly, and POP deletion.
  - Load profile: yes — bounce volume is bursty after large sends and capped per scan at 1000 messages.
  - Coherent unit: yes — `Scan(limit int, ch chan models.Bounce) error` has a bounded mailbox-processing contract.
  - State independence: maybe — the POP client and output channel are process-local, but mailbox credentials and emitted bounce records can be represented remotely.
  - Latency / failure: yes — scanner runs in a background loop and records bounces asynchronously.
- **Activation shape (informational, not a selection criterion):** periodic mailbox scanner goroutine.
- **Confidence:** medium — it is a strong workload match, but the channel output shape is less lift-friendly than returning a slice.
- **Risk notes:** remote execution must coordinate POP deletion semantics carefully so a failure does not lose or duplicate bounce messages.

### C-7: Campaign template compilation

- **Region root:** `evaluation/listmonk/models/campaigns.go:138` — `(*Campaign).CompileTemplate` compiles subject/body/alt templates and converts Markdown bodies to HTML.
- **Caller(s):** `evaluation/listmonk/internal/manager/pipe.go:35` compiles templates when a campaign pipe starts; `evaluation/listmonk/cmd/campaigns.go:176` compiles templates for preview.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — parses Go templates, rewrites template helper calls, and runs Markdown-to-HTML conversion for markdown campaigns.
  - Load profile: maybe — it is not per-recipient, but previews, archive rendering, and campaign starts can burst around campaign authoring.
  - Coherent unit: yes — method input is the campaign plus a function map and output is compiled template state on the campaign.
  - State independence: yes — it operates on the campaign value and supplied functions without reaching into application globals.
  - Latency / failure: maybe — some callers are synchronous previews, while campaign-start compilation naturally fails before background delivery proceeds.
- **Activation shape (informational, not a selection criterion):** campaign setup and preview helper.
- **Confidence:** medium — the compute is real, but the frequency is lower than per-recipient rendering.
- **Risk notes:** because it mutates template fields on `Campaign`, remote use should return compiled artifacts or be paired with render work rather than trying to ship Go template pointers.

### C-8: Transactional message render

- **Region root:** `evaluation/listmonk/models/messages.go:74` — `(*TxMessage).Render` renders body, optional alt body, and subject for one transactional recipient.
- **Caller(s):** `evaluation/listmonk/cmd/tx.go:132` renders each transactional recipient; `evaluation/listmonk/cmd/tx.go:165` then queues the resulting message.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — executes the stored template, may parse and execute per-request alt-body and subject templates, and copies rendered body bytes.
  - Load profile: yes — transactional mail APIs can receive uneven bursts and can render multiple recipients per request.
  - Coherent unit: yes — inputs are a subscriber, template, and func map; output is the populated transaction message fields.
  - State independence: yes — rendering uses passed values and mutates only the `TxMessage` instance.
  - Latency / failure: maybe — rendering happens on the HTTP request path before queueing, so remote failure would need a clear API error story.
- **Activation shape (informational, not a selection criterion):** transactional email API handler loop.
- **Confidence:** medium — good boundary, but per-call cost may be small for simple templates.
- **Risk notes:** the handler reuses one `TxMessage` across recipients; a remote lift should avoid cross-recipient mutation bleed by returning immutable rendered payloads.

### C-9: SES bounce webhook processing

- **Region root:** `evaluation/listmonk/internal/bounce/webhooks/ses.go:108` — `(*SES).ProcessBounce` verifies an SNS notification, parses nested SES JSON, classifies the bounce, and returns a bounce record.
- **Caller(s):** `evaluation/listmonk/cmd/bounce.go:167` calls it from the `/bounce/ses` webhook path.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — performs JSON decoding, SNS certificate lookup/cache, RSA signature verification, nested payload parsing, and classification.
  - Load profile: yes — SES bounce webhooks arrive in bursts after campaign sends.
  - Coherent unit: yes — byte payload in, `models.Bounce` plus error out.
  - State independence: maybe — certificate cache is mutable process-local state but can safely be replica-local.
  - Latency / failure: maybe — it is on an HTTP webhook path, but webhook senders typically retry failed deliveries.
- **Activation shape (informational, not a selection criterion):** HTTP webhook handler helper.
- **Confidence:** medium — cryptographic verification and JSON parsing are useful work, though individual payloads may be small.
- **Risk notes:** the first uncached certificate fetch is an outbound HTTP dependency; remote replicas need equivalent egress and cache behavior.

### C-10: SendGrid bounce batch processing

- **Region root:** `evaluation/listmonk/internal/bounce/webhooks/sendgrid.go:53` — `(*Sendgrid).ProcessBounce` verifies a SendGrid webhook signature and converts a JSON array into bounce records.
- **Caller(s):** `evaluation/listmonk/cmd/bounce.go:186` calls it from the `/bounce/sendgrid` webhook path.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — base64/ASN.1 decodes the signature, hashes the timestamp and body, verifies ECDSA, and parses a potentially multi-event JSON array.
  - Load profile: yes — SendGrid explicitly sends multiple bounce events per webhook and volume follows campaign sends.
  - Coherent unit: yes — signature, timestamp, and body are explicit inputs and the output is a slice of bounce records.
  - State independence: yes — uses only the configured public key and request bytes.
  - Latency / failure: maybe — it runs on a webhook request path, but upstream retry behavior makes a remote failure model plausible.
- **Activation shape (informational, not a selection criterion):** HTTP webhook handler helper.
- **Confidence:** medium — better batch shape than SES, but workload size depends on webhook batch size.
- **Risk notes:** signature verification must be done over the exact raw request bytes, so any lift boundary must preserve the body byte-for-byte.

Honest assessment: I am most confident in the campaign render, SMTP/postback push, CSV import, and image thumbnail candidates because they combine clear cost, spiky demand, and named units. The POP scanner and webhook processors are also credible, but their external delivery/deletion semantics and request-path webhook behavior make the lift design more delicate. Campaign template compilation and transactional rendering are genuinely marginal compared with per-recipient campaign delivery because their per-call cost depends heavily on template complexity and traffic mix. I suspect subscriber CSV export could be useful for very large tenants, but I did not include it because the current implementation streams directly to an HTTP response, which makes the remote boundary harder to justify under this rubric.
