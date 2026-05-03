# Critique of caddy drafts by gemini

## Verdicts on claude's draft

| Candidate ID | Verdict | One-paragraph reasoning |
|---|---|---|
| C-1 (Basic Auth) | KEEP | Rubric criterion satisfied: compute envelope. Argon2id and bcrypt comparisons are intentionally CPU and memory-intensive (KDF work), making them perfect candidates for offloading during authentication bursts. |
| C-2 (Templates) | KEEP | Rubric criterion satisfied: compute envelope. Executing Go templates with large FuncMaps and complex includes is CPU-intensive and occurs on the request path for dynamic content. |
| C-3 (Goldmark) | KEEP | Rubric criterion satisfied: compute envelope. Markdown rendering via Goldmark with Chroma syntax highlighting is a high-cost pure function that scales with document size. |
| C-4 (Encode) | KEEP | Rubric criterion satisfied: compute envelope. While streaming response writers present structural challenges (as Claude notes), the compression work itself (gzip/zstd) is a major CPU consumer in high-traffic deployments. |
| C-5 (Internal CA) | KEEP | Rubric criterion satisfied: compute envelope. RSA/ECDSA signing operations for certificate issuance are computationally expensive, especially during batch renewals or SNI onboarding storms. |
| C-6 (Template Include) | KEEP | Rubric criterion satisfied: coherent unit. While small on its own, offloading the recursive template parsing and rendering within includes is useful when building complex pages. |
| C-7 (FastCGI) | KEEP | Rubric criterion satisfied: coherent unit. Implements a clean RoundTripper interface, making it a viable target for offloading the protocol marshalling and backend communication. |
| C-8 (On-demand TLS) | DROP | Fails rubric criterion: compute envelope. This is a trivial HTTP client call to an "ask" endpoint. The CPU work is negligible, and it does not scale with payload or aggregate well; it is a simple control-plane check. |

## Verdicts on codex's draft

| Candidate ID | Verdict | One-paragraph reasoning |
|---|---|---|
| C-1 (Encode Write) | KEEP | Rubric criterion satisfied: compute envelope. Targeting the `Write` method of the response writer correctly identifies where the actual compression work happens per chunk of data. |
| C-2 (Basic Auth) | KEEP | Rubric criterion satisfied: compute envelope. Picking `correctPassword` is more surgical than the full `Authenticate` method, isolating the expensive hash comparison logic. |
| C-3 (Markdown) | KEEP | Rubric criterion satisfied: compute envelope. Identical to my own draft and Claude's; Goldmark rendering is a clear winner for lift utility. |
| C-4 (Caddyfile Adapt) | KEEP | Rubric criterion satisfied: compute envelope. Configuration adaptation is a bursty, CPU-heavy operation for large configurations, involving lexing, parsing, and JSON marshalling. |
| C-5 (Templates) | KEEP | Rubric criterion satisfied: compute envelope. Executing templates against buffered responses is a well-bounded computational unit with significant per-request cost. |
| C-6 (Directory Browse) | KEEP | Rubric criterion satisfied: compute envelope. Materializing and sorting directory listings (up to 10,000 entries) is an expensive operation that can be triggered by crawlers or large file shares. |
| C-7 (Health Check) | KEEP | Rubric criterion satisfied: latency tolerance. Active health checks are periodic background tasks that can involve body regex matching and state updates, making them ideal for off-main-path execution. |
| C-8 (SRV Upstreams) | KEEP | Rubric criterion satisfied: load profile. Dynamic upstream discovery via DNS SRV lookups adds latency and compute overhead during refresh cycles, which can be offloaded to maintain proxy responsiveness. |
| C-9 (Internal Issuer) | KEEP | Rubric criterion satisfied: compute envelope. Certificate issuance is a classic crypto-heavy task that scales with the number of managed domains and renewal frequency. |

## Overlooked

### O-1: CEL Expression Matcher

- **Region root:** `evaluation/caddy/modules/caddyhttp/celmatcher.go:213` — `MatchWithError` evaluates CEL programs against HTTP requests.
- **Caller(s):** `evaluation/caddy/modules/caddyhttp/matchers.go:30` (via `Matcher` interface).
- **Why useful (rubric scoring):**
  - Compute envelope: yes — CEL evaluation involves overhead for parsing/execution and can include expensive regular expression matching.
  - Load profile: yes — Runs per-request for routes using expression matchers, spiking with traffic volume.
  - Coherent unit: yes — Clear `(*http.Request) -> (bool, error)` contract.
  - State independence: yes — CEL evaluation is designed to be side-effect free.
  - Latency / failure: yes — Read-only operation on request path; failures are easily handled as non-matches.
- **Activation shape:** HTTP request matcher invoked during route evaluation.
- **Confidence:** high — This is a core part of Caddy's dynamic routing logic.
- **Risk notes:** Requires access to request metadata and replacer state.

### O-2: Logging Filter Encoder

- **Region root:** `evaluation/caddy/modules/logging/filterencoder.go:449` — `EncodeEntry` applies filters and regex replacements to log fields.
- **Caller(s):** Zap logger core when configured with Caddy's filtering logic.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — Complex regex-based filtering and masking of structured log entries is CPU-intensive at scale.
  - Load profile: yes — Scales linearly with request volume and log verbosity.
  - Coherent unit: yes — Takes a log entry and fields, returns a buffer.
  - State independence: yes — Pure transformation of log data.
  - Latency / failure: yes — Logging is often async or can tolerate slight delays; failures don't break the application.
- **Activation shape:** Log encoder helper invoked per log entry.
- **Confidence:** medium-high — Very effective for high-volume deployments with strict privacy/compliance filtering needs.
- **Risk notes:** None; the unit is highly isolated.

## Overall observations

The three drafts converge strongly on the "big four" lift areas for Caddy: Markdown rendering, Template execution, Password hashing, and Certificate issuance. Claude and Codex both correctly identified the utility of Basic Auth offloading, which I missed in my initial draft. Codex provided the most diverse set of candidates by including background tasks (Health Checks) and secondary request paths (Directory Browse), whereas Claude stayed closer to the primary middleware chain but included interesting transport-level candidates like FastCGI.
