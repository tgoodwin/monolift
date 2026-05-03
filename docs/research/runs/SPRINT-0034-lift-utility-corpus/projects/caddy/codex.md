Project read: Caddy's Go tree is a modular server with most interesting work expressed as HTTP handlers, TLS automation modules, config adapters, and reverse-proxy support code. I focused on regions that are larger than a trivial helper, have clear call boundaries, and could plausibly absorb CPU, I/O, or latency without requiring the whole server to move with them. The strongest request-path candidates are response encoding, password hash verification, Markdown/template rendering, and directory browse generation. The control-plane and background candidates are config adaptation, active health checks, dynamic upstream DNS, and internal certificate issuance; they are useful but carry more state or failure semantics.

### C-1: Dynamic response encoding stream

- **Region root:** `evaluation/caddy/modules/caddyhttp/encode/encode.go:313` — `(*responseWriter).Write` decides when to initialize compression and streams bytes through the selected encoder.
- **Caller(s):** `evaluation/caddy/modules/caddyhttp/encode/encode.go:160` wraps the downstream response writer with the chosen encoding, and `evaluation/caddy/modules/caddyhttp/encode/encode.go:181` invokes the next handler through that wrapper.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — gzip/zstd encoding is CPU-bound per response body, with encoder implementations created at `evaluation/caddy/modules/caddyhttp/encode/gzip/gzip.go:84` and `evaluation/caddy/modules/caddyhttp/encode/zstd/zstd.go:118`.
  - Load profile: yes — every eligible response body can stream through `rw.w.Write(p)` at `evaluation/caddy/modules/caddyhttp/encode/encode.go:356`, so load scales with traffic volume and body size.
  - Coherent unit: yes — the wrapper owns negotiation, header mutation, encoder lifecycle, and write-through behavior.
  - State independence: maybe — writer pools and headers are local enough, but the lifted path would need to preserve streaming semantics and flush/close behavior.
  - Latency / failure: maybe — compression sits on the user response path; failures are local write errors, but added latency is directly visible.
- **Activation shape (informational, not a selection criterion):** HTTP middleware wrapping response writes.
- **Confidence:** high — only unusual `ReadFrom`/flush edge cases would change the ranking.
- **Risk notes:** Streaming response contracts are the hard part: headers are delayed until compression eligibility is known, encoders are pooled, and `Close` must flush bytes before the response completes.

### C-2: Basic auth password hash verification

- **Region root:** `evaluation/caddy/modules/caddyhttp/caddyauth/basicauth.go:165` — `HTTPBasicAuth.correctPassword` performs cached or uncached secure hash comparison.
- **Caller(s):** `evaluation/caddy/modules/caddyhttp/caddyauth/caddyauth.go:84` invokes configured authentication providers, and `evaluation/caddy/modules/caddyhttp/caddyauth/basicauth.go:157` calls `correctPassword`.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — bcrypt comparison runs at `evaluation/caddy/modules/caddyhttp/caddyauth/bcrypt.go:51`, and Argon2id derives a key at `evaluation/caddy/modules/caddyhttp/caddyauth/argon2id.go:67`.
  - Load profile: yes — protected routes, login bursts, or password spraying can repeatedly trigger the intentionally expensive comparison path.
  - Coherent unit: yes — input is account hash plus plaintext password, and output is a boolean/error authentication decision.
  - State independence: maybe — the hash cache and singleflight group at `evaluation/caddy/modules/caddyhttp/caddyauth/basicauth.go:190` are shared optimization state that would need replica-local handling.
  - Latency / failure: maybe — this is on the request path, but the existing cache comment explicitly calls the hashes expensive at `evaluation/caddy/modules/caddyhttp/caddyauth/basicauth.go:219`.
- **Activation shape (informational, not a selection criterion):** HTTP authentication provider.
- **Confidence:** high — the region is intentionally isolated and already guarded against thundering herds.
- **Risk notes:** Preserving side-channel behavior matters: nonexistent accounts still use a fake password at `evaluation/caddy/modules/caddyhttp/caddyauth/basicauth.go:151`, so any lift must not short-circuit that work.

### C-3: Markdown-to-HTML template helper

- **Region root:** `evaluation/caddy/modules/caddyhttp/templates/tplcontext.go:350` — `TemplateContext.funcMarkdown` converts Markdown input to HTML through Goldmark and Chroma.
- **Caller(s):** `evaluation/caddy/modules/caddyhttp/templates/tplcontext.go:85` registers the helper in the template function map, and `evaluation/caddy/modules/caddyhttp/templates/tplcontext.go:250` executes templates that may invoke it.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — the helper constructs a Goldmark renderer with GFM, footnotes, syntax highlighting, heading IDs, and unsafe HTML support at `evaluation/caddy/modules/caddyhttp/templates/tplcontext.go:353`.
  - Load profile: yes — Markdown-heavy pages and syntax-highlighted code blocks can make request cost scale with document size.
  - Coherent unit: yes — the unit maps one template value to one HTML string and returns a local error from `md.Convert` at `evaluation/caddy/modules/caddyhttp/templates/tplcontext.go:375`.
  - State independence: yes — the helper takes input content, uses a pooled buffer, and does not mutate server state.
  - Latency / failure: maybe — it runs synchronously during template response rendering, so compute delay is user-visible even though failures are ordinary template errors.
- **Activation shape (informational, not a selection criterion):** Template helper invoked from HTTP response templates.
- **Confidence:** high — the dependency closure is mostly Markdown/parser libraries rather than Caddy server state.
- **Risk notes:** It returns unescaped HTML by design at `evaluation/caddy/modules/caddyhttp/templates/tplcontext.go:348`, so a lifted version must preserve exact rendering and escaping behavior.

### C-4: Caddyfile config adaptation

- **Region root:** `evaluation/caddy/caddyconfig/caddyfile/adapter.go:32` — `Adapter.Adapt` converts Caddyfile bytes into Caddy JSON.
- **Caller(s):** `evaluation/caddy/caddyconfig/load.go:97` adapts non-JSON config on `/load`, and `evaluation/caddy/caddyconfig/load.go:209` dispatches to the selected adapter.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — adaptation parses tokens at `evaluation/caddy/caddyconfig/caddyfile/adapter.go:45`, runs server-type setup at `evaluation/caddy/caddyconfig/caddyfile/adapter.go:50`, checks formatting at `evaluation/caddy/caddyconfig/caddyfile/adapter.go:57`, and marshals JSON at `evaluation/caddy/caddyconfig/caddyfile/adapter.go:61`.
  - Load profile: maybe — admin reload/adapt calls are not steady traffic, but large configs or frequent deployments can produce meaningful spikes.
  - Coherent unit: yes — the input is config bytes plus options; the output is JSON bytes, warnings, and an error.
  - State independence: maybe — parsing itself is bounded, but imports can read files through `doSingleImport` at `evaluation/caddy/caddyconfig/caddyfile/parse.go:588`, and setup depends on registered directives/modules.
  - Latency / failure: yes — this is control-plane work where a 400/adaptation error is already part of the contract.
- **Activation shape (informational, not a selection criterion):** Admin API config load/adapt operation.
- **Confidence:** high — the boundary is already an adapter interface.
- **Risk notes:** Import resolution, environment expansion, and module setup make this less pure than a text parser; a lifted version would need access to the same files and module registry assumptions.

### C-5: Buffered HTTP template execution

- **Region root:** `evaluation/caddy/modules/caddyhttp/templates/templates.go:455` — `(*Templates).executeTemplate` builds a request-scoped template context and executes the buffered response body as a template.
- **Caller(s):** `evaluation/caddy/modules/caddyhttp/templates/templates.go:437` calls `executeTemplate` after the response recorder buffers an eligible body.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — the lower-level executor parses the full buffered body at `evaluation/caddy/modules/caddyhttp/templates/tplcontext.go:243` and executes it at `evaluation/caddy/modules/caddyhttp/templates/tplcontext.go:250`.
  - Load profile: yes — it scales with request traffic and with the size/complexity of templated response bodies.
  - Coherent unit: yes — one buffered response plus request context becomes a rewritten response buffer.
  - State independence: maybe — template functions can read files, inspect request state, include HTTP content, and use custom functions from `evaluation/caddy/modules/caddyhttp/templates/templates.go:467`.
  - Latency / failure: maybe — this is synchronous request-path rendering, and parse/execution errors become HTTP handler errors.
- **Activation shape (informational, not a selection criterion):** HTTP middleware after downstream response buffering.
- **Confidence:** medium — the template core is clear, but enabled helper functions expand the dependency surface.
- **Risk notes:** The response recorder only buffers matching MIME types at `evaluation/caddy/modules/caddyhttp/templates/templates.go:427`; preserving that gating and the exact buffer mutation contract would be necessary.

### C-6: Directory browse listing and rendering

- **Region root:** `evaluation/caddy/modules/caddyhttp/fileserver/browse.go:78` — `(*FileServer).serveBrowse` materializes a directory listing and renders JSON, text, or HTML.
- **Caller(s):** `evaluation/caddy/modules/caddyhttp/fileserver/staticfiles.go:371` delegates directory requests to browse mode, and `evaluation/caddy/modules/caddyhttp/fileserver/browse.go:119` loads directory contents for the response.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — the path reads up to the directory entry limit at `evaluation/caddy/modules/caddyhttp/fileserver/browse.go:226`, stats entries at `evaluation/caddy/modules/caddyhttp/fileserver/browsetplcontext.go:60`, handles symlinks at `evaluation/caddy/modules/caddyhttp/fileserver/browsetplcontext.go:94`, and sorts at `evaluation/caddy/modules/caddyhttp/fileserver/browsetplcontext.go:223`.
  - Load profile: yes — large browsable directories or crawlers can repeatedly trigger listing, sort, and render work.
  - Coherent unit: maybe — the directory materialization is coherent, but the full handler mixes redirects, conditional headers, content negotiation, and template rendering.
  - State independence: maybe — it depends on the configured filesystem, hidden file rules, symlink policy, and request query parameters.
  - Latency / failure: maybe — it is request-path filesystem work with ordinary HTTP errors, and large directories can be visibly slow.
- **Activation shape (informational, not a selection criterion):** Static file server directory request.
- **Confidence:** medium — `loadDirectoryContents` would be a cleaner lift root than the entire handler if scope had to shrink.
- **Risk notes:** The default directory entry cap is 10000 at `evaluation/caddy/modules/caddyhttp/fileserver/browse.go:75`; filesystem locality and symlink behavior would dominate practical lift difficulty.

### C-7: Reverse proxy active health check

- **Region root:** `evaluation/caddy/modules/caddyhttp/reverseproxy/healthchecks.go:391` — `(*Handler).doActiveHealthCheck` sends a health request, evaluates status/body criteria, and updates upstream health.
- **Caller(s):** `evaluation/caddy/modules/caddyhttp/reverseproxy/reverseproxy.go:421` starts the active health checker goroutine, and `evaluation/caddy/modules/caddyhttp/reverseproxy/healthchecks.go:371` invokes `doActiveHealthCheck` per upstream target.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — each check builds an HTTP request at `evaluation/caddy/modules/caddyhttp/reverseproxy/healthchecks.go:444`, performs it at `evaluation/caddy/modules/caddyhttp/reverseproxy/healthchecks.go:509`, drains/limits bodies at `evaluation/caddy/modules/caddyhttp/reverseproxy/healthchecks.go:520`, and can run a body regexp at `evaluation/caddy/modules/caddyhttp/reverseproxy/healthchecks.go:554`.
  - Load profile: maybe — work scales with upstream count and interval rather than external traffic, with per-upstream goroutines launched at `evaluation/caddy/modules/caddyhttp/reverseproxy/healthchecks.go:299`.
  - Coherent unit: maybe — the probe itself is coherent, but it mutates upstream health counters and emits events.
  - State independence: maybe — health status, logger, events, client, and upstream state live on the handler.
  - Latency / failure: yes — this is background work; probe failures are already reduced to health state changes rather than client response failures.
- **Activation shape (informational, not a selection criterion):** Periodic background goroutine per reverse-proxy handler.
- **Confidence:** medium — it is liftable if health-state mutation stays local or is explicitly synchronized.
- **Risk notes:** Incorrectly isolating this could split health truth across replicas; the mark healthy/unhealthy closures at `evaluation/caddy/modules/caddyhttp/reverseproxy/healthchecks.go:464` and `evaluation/caddy/modules/caddyhttp/reverseproxy/healthchecks.go:485` are the coupling point.

### C-8: SRV dynamic upstream refresh

- **Region root:** `evaluation/caddy/modules/caddyhttp/reverseproxy/upstreams.go:122` — `SRVUpstreams.GetUpstreams` resolves SRV records and converts them into reverse-proxy upstreams with caching.
- **Caller(s):** `evaluation/caddy/modules/caddyhttp/reverseproxy/reverseproxy.go:577` calls the dynamic upstream source during proxy loop iteration, and `evaluation/caddy/modules/caddyhttp/reverseproxy/reverseproxy.go:1586` documents that this source may be called multiple times per request.
- **Why useful (rubric scoring):**
  - Compute envelope: maybe — cache hits are cheap, but stale refreshes take a DNS SRV lookup at `evaluation/caddy/modules/caddyhttp/reverseproxy/upstreams.go:153` and allocate upstreams at `evaluation/caddy/modules/caddyhttp/reverseproxy/upstreams.go:174`.
  - Load profile: maybe — calls can happen per retry, but expensive lookup work is gated by freshness checks at `evaluation/caddy/modules/caddyhttp/reverseproxy/upstreams.go:129` and `evaluation/caddy/modules/caddyhttp/reverseproxy/upstreams.go:141`.
  - Coherent unit: yes — one request plus SRV source config returns a concrete upstream slice or error.
  - State independence: maybe — the global SRV cache and lock at `evaluation/caddy/modules/caddyhttp/reverseproxy/upstreams.go:126` make behavior replica-local and time-dependent.
  - Latency / failure: maybe — it is on the proxy request path, but grace-period fallback to cached upstreams is explicit at `evaluation/caddy/modules/caddyhttp/reverseproxy/upstreams.go:159`.
- **Activation shape (informational, not a selection criterion):** Reverse proxy request/retry upstream discovery.
- **Confidence:** medium — good boundary, but only refresh misses are heavy enough to justify lift.
- **Risk notes:** The interface comment says the function should be very fast and stable at `evaluation/caddy/modules/caddyhttp/reverseproxy/reverseproxy.go:1581`; moving it must not make per-request upstream selection slower or less stable.

### C-9: Internal certificate issuance

- **Region root:** `evaluation/caddy/modules/caddytls/internalissuer.go:103` — `InternalIssuer.Issue` signs a CSR with Caddy's internal CA and returns a PEM certificate chain.
- **Caller(s):** `evaluation/caddy/modules/caddytls/automation.go:233` installs configured issuers into the automation policy, and `evaluation/caddy/modules/caddytls/tls.go:585` starts CertMagic management for names that can use those issuers.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — issuance prepares a signing authority at `evaluation/caddy/modules/caddytls/internalissuer.go:108`, clamps lifetime against the issuer certificate at `evaluation/caddy/modules/caddytls/internalissuer.go:123`, signs at `evaluation/caddy/modules/caddytls/internalissuer.go:133`, and PEM-encodes the chain at `evaluation/caddy/modules/caddytls/internalissuer.go:139`.
  - Load profile: maybe — ordinary renewals are sparse, but startup batches or on-demand internal TLS can create bursts of certificate work.
  - Coherent unit: yes — the method maps a CSR/context to an issued certificate object and error.
  - State independence: maybe — it depends on CA authority state, private key material, lifetime settings, and storage configured elsewhere.
  - Latency / failure: maybe — automated management is asynchronous for batches, but on-demand issuance can affect TLS handshakes indirectly.
- **Activation shape (informational, not a selection criterion):** TLS automation issuer through the CertMagic issuer interface.
- **Confidence:** medium — the local method boundary is clean, but the static caller evidence is interface-based and indirect.
- **Risk notes:** This region touches trust material; lifting would require careful handling of CA keys and exact CertMagic issuer semantics, not just moving CPU.

Honest assessment: The top six candidates are practical lift targets because their work is localized and their call evidence is direct in the Caddy tree. The reverse-proxy and TLS candidates are still useful but more stateful: health checks mutate shared upstream state, SRV lookup relies on a global cache and request-path timing, and internal issuance crosses into CertMagic and CA key management. I did not run Caddy or tests, per instruction; this survey is based only on opened source evidence and search/read commands.
