# Critique of pocketbase drafts by gemini

## Verdicts on claude's draft

| Candidate ID | Verdict | One-paragraph reasoning |
|---|---|---|
| C-1 (Image thumbnail generator) | KEEP | rubric criterion satisfied: Compute envelope. This is the most computationally expensive path in PocketBase that is cleanly separable, as confirmed by the authors' own use of singleflight and semaphores at `apis/file.go:171`. |
| C-2 (Bcrypt password hashing) | KEEP | rubric criterion satisfied: Compute envelope. Bcrypt is the textbook CPU-bound lift target. Moving the hash generation at `core/field_password.go:286` offloads significant CPU work from the write path. |
| C-3 (Bcrypt password verification) | KEEP | rubric criterion satisfied: Compute envelope. Symmetric to C-2, but for the authentication path. Verify at `core/field_password.go:317` is spiky and CPU-intensive. |
| C-4 (SMTP send) | KEEP | rubric criterion satisfied: Latency tolerance. The existing code already uses `routine.FireAndForget`, making this an ideal candidate for offloading IO-bound work that doesn't need to block the response. |
| C-5 (Composite record-mailer) | KEEP | rubric criterion satisfied: Coherent unit. Aggregates template rendering and SMTP sending into a single async-capable unit. |
| C-6 (OAuth2 callback exchange) | KEEP | rubric criterion satisfied: IO-bound. Offloads waiting on multiple external HTTPS round-trips. |
| C-7 (Avatar download) | KEEP | rubric criterion satisfied: IO-bound. A very clean, isolated unit of work at `apis/record_auth_with_oauth2.go:468` that performs outbound IO. |
| C-8 (Backup zip writer) | KEEP | rubric criterion satisfied: Compute envelope. Compression of the entire data directory is a heavy CPU and IO task that is naturally background-able. |
| C-9 (Record relation expansion) | DROP | fails rubric criterion: State independence. The implementation at `core/record_query_expand.go:34` is tightly coupled to the local SQLite database and requires a callback (`ExpandFetchFunc`) that often captures request-scoped `RequestInfo`, making remote execution fragile. |
| C-10 (Batched log writer) | DROP | fails rubric criterion: State independence. As Claude correctly noted, this is disqualified because its primary purpose is writing to the process-local SQLite logs database. |

## Verdicts on codex's draft

| Candidate ID | Verdict | One-paragraph reasoning |
|---|---|---|
| C-1 (Image thumbnail generation) | KEEP | rubric criterion satisfied: Compute envelope. Consistent with all other drafts as a high-utility lift. |
| C-2 (Backup archive creation) | KEEP | rubric criterion satisfied: Compute envelope. High-impact task for large datasets. |
| C-3 (S3 multipart upload) | KEEP | rubric criterion satisfied: Compute envelope (IO parallelism). Specifically the concurrent multipart logic at `uploader.go:71` is a great candidate for offloading IO orchestration. |
| C-4 (Record file-field upload) | MODIFY | Incorrect scope. The target at `core/field_file.go:512` is a private interceptor method tightly coupled to the `Record` model. The more useful lift is the lower-level `fsys.UploadFile` or the S3 uploader identified in C-3. |
| C-5 (OAuth2 exchange) | KEEP | rubric criterion satisfied: IO-bound. |
| C-6 (Password reset email) | KEEP | rubric criterion satisfied: Latency tolerance. |
| C-7 (Collection schema import) | DROP | fails rubric criterion: Variable / spikable load. Schema imports at `core/collection_import.go:36` are rare admin-only operations, not a spiky runtime load that justifies the complexity of a lift. |
| C-8 (Search query parse/exec) | DROP | fails rubric criterion: State independence. Search execution at `tools/search/provider.go:363` is inherently tied to the local SQLite storage and carries complex, non-serializable query builder state. |
| C-9 (Record relation expansion) | DROP | fails rubric criterion: State independence. Identical to Claude's C-9; too coupled to the local DB and request-scoped fetch closures. |

## Overlooked

### C-11: JavaScript Hook Execution

- **Region root:** `evaluation/pocketbase/plugins/jsvm/binds.go:81` (and surrounding executor logic) — Anonymous JS handler wrapper that executes user-defined scripts in a `goja` runtime.
- **Caller(s):** Any framework hook point (e.g., `onRecordBeforeCreateRequest`) where users have registered JS logic.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — User-defined JS can be arbitrarily expensive and is much slower than native Go.
  - Load profile: yes — Completely unpredictable and spiky based on the custom logic deployed by the operator.
  - Coherent unit: yes — Wrapped in a generic executor interface that takes event data and returns results.
  - State independence: maybe — The `$app` interface must be shimmed, but the rubric's goal is precisely to isolate such "unknown" compute.
  - Latency / failure: yes — Users expect hooks to add overhead; many hooks are logically async.
- **Activation shape:** Event-driven hook execution.
- **Confidence:** high — This represents the "unknown" CPU consumer in PocketBase.
- **Risk notes:** Requires serializing the event object and potentially shimming the `$app` Go methods.

### C-12: Apple Client Secret Generation

- **Region root:** `evaluation/pocketbase/forms/apple_client_secret_create.go:63` — `(*AppleClientSecretCreate).Submit` generates a JWT signed with an ES256 private key.
- **Caller(s):** Admin UI settings for Apple OAuth2 configuration.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — Cryptographic signing (ES256) is a distinct, CPU-heavy operation.
  - Load profile: maybe — Infrequent, but a very clean "utility" lift.
  - Coherent unit: yes — Encapsulated in a form struct with string inputs and output.
  - State independence: yes — Operates purely on provided keys.
  - Latency / failure: yes — Part of a settings update path where sub-second latency is not critical.
- **Activation shape:** Admin settings form submission.
- **Confidence:** medium — Clean but low frequency.
- **Risk notes:** Secure transport of private keys is required.

## Overall observations

The foreign drafts converge cleanly on the "big three" utility targets: image thumbnailing, backup compression, and OAuth/Email IO. Claude is more rigorous in identifying specific Bcrypt paths (C-2, C-3), which I agree are high-utility. Codex identifies the specific multipart S3 uploader (C-3), which is a better cut point than a generic "upload" handler. Both foreign drafts struggled with the state-independence of DB-heavy paths like Relation Expansion and Search, which I believe should be excluded due to their tight coupling to the local SQLite handle and request-scoped closures.
