# Caddy — lift-utility candidates (Phase 1, claude)

## Project read

Caddy is a modular HTTP/HTTPS server written in Go: configuration is parsed (Caddyfile or JSON) into an app/module tree, and per-request work flows through a chain of middleware handlers (`MiddlewareHandler.ServeHTTP`). The framework's own dispatch (`caddyhttp.Server.ServeHTTP` at `modules/caddyhttp/server.go:310`, the route mux, the `App.Start` listeners loop) is infrastructure and excluded by the rubric. The interesting compute clusters in three places: (1) per-request **transformations of response bytes** — compression (`encode`), Go-template + Goldmark rendering (`templates`, fileserver `serveBrowse`); (2) per-request **CPU-bound auth/crypto** — Argon2id and bcrypt comparisons inside `caddyauth/HTTPBasicAuth.Authenticate`; (3) **certificate issuance** off the TLS handshake / ACME server path — `InternalIssuer.Issue`, `ACMEIssuer.Issue`, `PermissionByHTTP.CertificateAllowed`. Outbound transports (`reverseproxy.Handler.ServeHTTP`, `fastcgi.Transport.RoundTrip`) do meaningful per-request work but tend to fail the rubric on streaming/WebSocket coupling or local-socket assumptions.

---

### C-1: HTTP basic-auth credential check (Argon2id / bcrypt path)

- **Region root:** `modules/caddyhttp/caddyauth/basicauth.go:143` — `func (hba HTTPBasicAuth) Authenticate(w http.ResponseWriter, r *http.Request) (User, bool, error)`. Internally calls `correctPassword` → `hba.Hash.Compare(account.password, plaintextPassword)`, where `Compare` is implemented by `Argon2idHash.Compare` (`modules/caddyhttp/caddyauth/argon2id.go:61`, `argon2.IDKey` — memory-hard KDF) or `BcryptHash.Compare` (`modules/caddyhttp/caddyauth/bcrypt.go:51`).
- **Caller(s):** `modules/caddyhttp/caddyauth/caddyauth.go:84` — `Authentication.ServeHTTP` iterates registered providers and calls `prov.Authenticate(w, r)` for each request hitting a `basic_auth`-protected route.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — Argon2id with default `memory=64MiB, time=1, threads=1` is tens of milliseconds of CPU per call; bcrypt at default cost is similar. This is exactly the "crypto over more than a small constant" example in the rubric.
  - Load profile: yes — bursty around login flurries / API key rotation / cron-driven scrapers; uneven across protected vs. unprotected routes.
  - Coherent unit: yes — `Authenticate(w, r)` returns `(User, bool, error)`; the `Comparer` interface (`Compare(hashed, plaintext []byte) (bool, error)`) is already a clean, value-typed contract.
  - State independence: yes — `hba.Accounts` is read-only after `Provision`; the `HashCache` (`basicauth.go:222`) is a `sync.RWMutex`-protected map plus singleflight, fine to make replica-local since cache-miss correctness is unchanged.
  - Latency / failure: yes — the region's own contribution is already O(10–100 ms) of KDF work, so a network hop is comfortably in the noise; on failure the caller surfaces 401, no broken invariant.
- **Activation shape:** HTTP middleware handler invoked per request whose route includes `basicauth`/`basic_auth`.
- **Confidence:** high — this is a textbook positive example. Would change my mind only if real deployments overwhelmingly use bcrypt at default cost (10) on already-cached entries, in which case lift overhead would dominate.
- **Risk notes:** The one cross-replica wrinkle is the `WWW-Authenticate` response header set inside `promptForCredentials` (`basicauth.go:215`); easiest seam is to lift `correctPassword`/`Compare` (the pure function) and keep the small header-prompt logic local, or accept the response-header set as a clean side effect captured in the return.

---

### C-2: Templates middleware — render response body as Go template

- **Region root:** `modules/caddyhttp/templates/templates.go:410` — `func (t *Templates) ServeHTTP(w, r, next)`. Buffers the downstream response, calls `executeTemplate` (`templates.go:455`) which builds a `TemplateContext` and runs `executeTemplateInBuffer` (`tplcontext.go:240`): `c.tpl.Parse(buf.String())` then `c.tpl.Execute(buf, c)`.
- **Caller(s):** `modules/caddyhttp/templates/templates.go:410` is registered as `http.handlers.templates`; included in any route stack that uses the `templates` directive.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — Go `text/template` parse + execute on every response, plus the full Sprig FuncMap (`tplcontext.go:71`), markdown via Goldmark+chroma, etc. CPU scales with template size and dynamic includes.
  - Load profile: yes — bursty per-page; static-site-with-dynamic-snippets workloads spike with cache misses or content campaigns.
  - Coherent unit: yes — input is a buffered response body + request; output is a transformed buffer written back via `rec.WriteResponse()`. Buffered, not streaming — well suited to remote dispatch.
  - State independence: maybe — `TemplateContext.Root` is an `http.FileSystem` for `include`/`readFile`/`listFiles`; if present, the remote replica needs the same view of files. `httpInclude` issues an in-process sub-request (`tplcontext.go:173`), which is the only piece that genuinely needs same-process state. Configurations without these helpers are clean.
  - Latency / failure: maybe — on the response path, but already buffered and CPU-bound enough (large pages, sprig calls, markdown blocks) that an extra hop fits inside existing budget.
- **Activation shape:** HTTP middleware handler that wraps a `ResponseRecorder` around `next.ServeHTTP`.
- **Confidence:** medium-high — strongest if the workload uses `markdown`, `httpInclude`-free templates, or large dynamic pages. Would change my mind if the typical config uses `httpInclude` heavily, since that pulls per-call request state back into the origin.
- **Risk notes:** `httpInclude` requires a local virtual sub-request; lifts must either disable that helper, marshal it as a callback, or document the tradeoff. Custom template extensions registered via `http.handlers.templates.functions.*` would also need to ride the lift.

---

### C-3: Goldmark markdown render helper

- **Region root:** `modules/caddyhttp/templates/tplcontext.go:350` — `func (TemplateContext) funcMarkdown(input any) (string, error)`. Builds a fresh `goldmark.New(...)` with GFM, Footnote, and chroma syntax-highlighting on every call and runs `md.Convert([]byte(inputStr), buf)`.
- **Caller(s):** Bound into the template FuncMap at `tplcontext.go:85` (`"markdown": c.funcMarkdown`); invoked from any user template that pipes content through `{{markdown ...}}`. Also used implicitly by markdown-served pages and by the `splitFrontMatter` + `markdown` idiom.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — Goldmark + GFM parsing + chroma syntax highlighting is genuinely CPU-heavy on non-trivial Markdown blocks; rebuilds the parser each call.
  - Load profile: yes — uneven per page; spikes with content publishing or doc-site traffic.
  - Coherent unit: yes — pure function: `string → (string, error)`. No request, no FS, no globals.
  - State independence: yes — nothing mutated outside the local buffer pool.
  - Latency / failure: maybe — invoked synchronously inside template execution; the markdown work itself already dominates per-call latency for non-trivial inputs.
- **Activation shape:** invoked from `text/template` execution (effectively a synchronous helper called by C-2's region).
- **Confidence:** high on shape, medium on usage frequency in the real Caddy fleet.
- **Risk notes:** Constructing the goldmark instance per call (`tplcontext.go:353`) is itself wasteful but unrelated to lift; lifting would amortize with a remote-side warm pool. The function imports a heavy dependency tree (goldmark + chroma) — no caddy-internal coupling.

---

### C-4: HTTP response compression (encode middleware)

- **Region root:** `modules/caddyhttp/encode/encode.go:154` — `func (enc *Encode) ServeHTTP(w, r, next)`, plus the streaming work in `responseWriter.Write` (`encode.go:313`) and `responseWriter.Close` (`encode.go:407`). Encoders themselves are registered modules: `gzip` (`encode/gzip/gzip.go`), `zstd` (`encode/zstd/zstd.go:118`), `brotli`.
- **Caller(s):** Inserted into any route stack that uses the `encode` directive; effectively wraps every response that matches `Encode.Matcher` (text-y MIME types).
- **Why useful (rubric scoring):**
  - Compute envelope: yes — gzip/zstd/brotli compression on every text-y response body; CPU scales linearly with bytes encoded.
  - Load profile: yes — bursty with traffic; payload-driven (large HTML/JSON benefits most).
  - Coherent unit: maybe — `Encode.ServeHTTP` itself is clean, but the actual compressor is a wrapped `http.ResponseWriter` (`encode.go:238`) that intercepts every `Write(p)` call. Lifting requires either streaming the response bytes to the remote or buffering the whole body — both add per-byte overhead that may erase the compression cost saving.
  - State independence: yes — uses a `sync.Pool` of writers (`encode.go:204`), trivially replica-local.
  - Latency / failure: maybe — sits on the response critical path; only helps if encoding cost > round-trip cost (so: large bodies, expensive level like zstd best).
- **Activation shape:** HTTP middleware handler that wraps the downstream `ResponseWriter`.
- **Confidence:** medium — the *function* is great compute-per-call, but the streaming response-writer integration makes it a poor structural fit for lift-without-buffering. Best treated as a stress case for the lift mechanism rather than a slam-dunk win.
- **Risk notes:** The `responseWriter` wraps stdlib `http.ResponseWriter` and even implements `ReadFrom` for sendfile (`encode.go:374`); a remote lift would have to break sendfile and either buffer or stream. Disqualifier-adjacent: it does not hold a per-request socket itself, but it is *intimately* tied to the one held by the parent server.

---

### C-5: PKI internal CA — issue certificate from CSR

- **Region root:** `modules/caddypki/internalissuer.go:103` — `func (iss InternalIssuer) Issue(ctx context.Context, csr *x509.CertificateRequest) (*certmagic.IssuedCertificate, error)`. Calls `iss.ca.NewAuthority(authCfg)` (`modules/caddypki/ca.go:219`) then `auth.SignWithContext(ctx, csr, ...)` and PEM-encodes the chain.
- **Caller(s):** Plugged into certmagic via the `Issuer` interface; certmagic's per-request `manageOne`/`obtainCert` paths invoke `Issue` whenever a managed name needs a fresh cert (TLS handshakes, renewal sweeps, on-demand TLS).
- **Why useful (rubric scoring):**
  - Compute envelope: yes — RSA/ECDSA signing of an X.509 cert; the smallstep `authority` stack also runs policy + name validation. Tens of ms each on RSA keys.
  - Load profile: yes — periodic renewal sweeps (cron-shaped), plus on-demand TLS bursts when new SNIs appear (campaign launches, multi-tenant onboarding).
  - Coherent unit: yes — `Issue(ctx, *x509.CertificateRequest) → (*certmagic.IssuedCertificate, error)`; defined by the third-party `certmagic.Issuer` interface, so the contract is enforced.
  - State independence: maybe — `iss.ca` carries the (root or intermediate) signing key. The lift either ships the key to the remote (security tradeoff) or keeps the signer local and lifts only the CSR pre-processing — narrowing the win.
  - Latency / failure: yes — caller is either certmagic's renewal goroutine (background) or a TLS handshake (which has seconds of budget for issuance, since this only runs on cold cache).
- **Activation shape:** library callback (`certmagic.Issuer.Issue`) invoked from certmagic's renewal/issue paths.
- **Confidence:** medium — the signature work is real but bounded; lift payoff is largest under bursty on-demand TLS or fleet-wide renewal storms.
- **Risk notes:** Signing-key custody is the dominant constraint, not code coupling. `ca.NewAuthority` (`ca.go:219`) constructs a fresh `authority.Authority` each call — fine for lift, mildly wasteful otherwise.

---

### C-6: Templates context — file include/readFile

- **Region root:** `modules/caddyhttp/templates/tplcontext.go:112` — `funcInclude(filename string, args ...any) (string, error)` and `tplcontext.go:136` `funcReadFile(filename string) (string, error)`. Both hit `c.Root` (an `http.FileSystem`) and stream the file into a buffer (`readFileToBuffer`, `tplcontext.go:150`).
- **Caller(s):** Registered into the template FuncMap at `tplcontext.go:80–81`; invoked any time a user template uses `{{include ...}}` or `{{readFile ...}}`.
- **Why useful (rubric scoring):**
  - Compute envelope: maybe — file IO + (for `include`) recursive template parse/execute; expensive if many included files per page.
  - Load profile: yes — uneven per page.
  - Coherent unit: yes — clear `(filename, args) → string` contract.
  - State independence: maybe — depends on `c.Root`. The `Root` is configurable via `FileRoot` (`templates.go:349`) and is virtualized as `http.FileSystem`, so it can in principle be backed by a network or virtual FS. As-shipped, deployments use a local directory.
  - Latency / failure: maybe — synchronous within template execution; per-file cost is small but compounds with many includes.
- **Activation shape:** template helper invoked from C-2's region.
- **Confidence:** medium-low — useful only if the file system can be virtualized for the lifted replica; otherwise the disqualifier on hardcoded local-FS reads bites.
- **Risk notes:** This is genuinely on the edge of disqualifier 2 ("direct read of repo-local filesystem paths that cannot be virtualized"). Listed because the `http.FileSystem` indirection means it *can* be virtualized — but that's an integration cost that has to be paid.

---

### C-7: FastCGI transport round-trip

- **Region root:** `modules/caddyhttp/reverseproxy/fastcgi/fastcgi.go:163` — `func (t Transport) RoundTrip(r *http.Request) (*http.Response, error)`. Builds the FastCGI env, dials backend, runs `client.Get/Post/Head/Options/...` over the FastCGI binary protocol.
- **Caller(s):** Registered as a reverse-proxy transport (`http.reverse_proxy.transport.fastcgi`); invoked from `Handler.reverseProxy` (`modules/caddyhttp/reverseproxy/reverseproxy.go:993`) `h.Transport.RoundTrip(req)` for every PHP request.
- **Why useful (rubric scoring):**
  - Compute envelope: maybe — protocol marshalling + IO; per-call cost dominated by PHP-FPM execution on the other side.
  - Load profile: yes — every dynamic PHP page hit.
  - Coherent unit: yes — implements stdlib `http.RoundTripper`: `(*Request) → (*Response, error)`, a clean, value-only contract.
  - State independence: yes — opens a fresh `net.Dialer` connection per call (`fastcgi.go:207`), no shared per-process state.
  - Latency / failure: maybe — on the response path but the call is already a network/socket hop, so an extra one is noticeable but not pathological.
- **Activation shape:** `http.RoundTripper` invoked from the reverse-proxy middleware.
- **Confidence:** medium — strong contract, but the lift only helps if the FastCGI backend is itself reachable from the lifted replica. If PHP-FPM runs on a Unix socket on the original host (very common), lift breaks unless that socket is exported.
- **Risk notes:** Hardcoded local-Unix-socket configurations are a structural blocker. TCP-PHP-FPM deployments lift cleanly. The transport also reads `r.Body` as a stream into FCGI Stdin records, so request-body buffering still has to happen somewhere.

---

### C-8: On-demand TLS — ask permission endpoint

- **Region root:** `modules/caddytls/ondemand.go:119` — `func (p PermissionByHTTP) CertificateAllowed(ctx context.Context, name string) error`. Builds an `ask` URL, issues `onDemandAskClient.Get(askURLString)`, returns based on status.
- **Caller(s):** Wired into certmagic's on-demand permission interface; invoked from the TLS handshake path the first time a client presents an unseen SNI under `on_demand_tls`.
- **Why useful (rubric scoring):**
  - Compute envelope: maybe — the work *is* the outbound HTTP call; CPU is negligible. Counts as IO-bound that scales with traffic shape.
  - Load profile: yes — bursty when new domains hit (campaign launches, multi-tenant onboarding); zero when steady-state.
  - Coherent unit: yes — `(ctx, name) → error`, no shared state besides the configured endpoint string.
  - State independence: yes — `p.Endpoint` and `p.replacer` are read-only post-Provision; HTTP client is package-level singleton (`onDemandAskClient`), trivially replica-local.
  - Latency / failure: yes — already does a synchronous HTTP round trip, so an extra hop is in the noise; certmagic handles failure (deny → no cert).
- **Activation shape:** library callback invoked from certmagic's on-demand-TLS permission check (off the TLS handshake path).
- **Confidence:** medium-low — easy to lift but the per-call work is one HTTP request; the lift only pays off if there's a way to colocate this with a heavier upstream policy engine. Listed for completeness of the on-demand-TLS surface.
- **Risk notes:** Borderline on the compute-envelope criterion. Included as the cleanest "shape" example in the TLS subtree, not because the per-call work is heavy.

---

## Honest assessment

I'm most confident in **C-1 (basic-auth Argon2id/bcrypt compare)** — it's a textbook positive example: real CPU per call, a clean two-argument pure function, replica-local cache, and a caller that already tolerates 10–100ms of latency. I'm next-most confident in **C-3 (funcMarkdown)** for the same reason at finer grain (a pure `string → (string, error)`), with the caveat that I can't prove it dominates a real workload from reading code alone. **C-2 (templates ServeHTTP)** and **C-5 (InternalIssuer.Issue)** are good shape-fits with realistic state caveats — `httpInclude` and signing-key custody respectively. **C-4 (encode)** is the most marginal of the "definitely real compute" candidates — the work is undeniable, but the streaming-`ResponseWriter` wrap is structurally hostile to remote dispatch, so I included it as a stress case rather than a recommendation. **C-7 (fastcgi RoundTrip)** depends entirely on backend deployment topology; **C-8 (PermissionByHTTP)** is borderline on compute envelope and listed mostly to show the shape. **C-6 (template include/readFile)** is on the edge of disqualifier 2 and only included because the `http.FileSystem` indirection makes virtualization plausible.

The region I most suspect *should* be a great lift candidate but couldn't justify here is `reverseproxy.Handler.ServeHTTP` (`modules/caddyhttp/reverseproxy/reverseproxy.go:454`). It's the heart of Caddy's compute when it's running as an L7 proxy: request rewriting, retry-with-backoff loop, body buffering, header transformation, response copy. But the same function holds WebSocket/HTTP-CONNECT upgrade handling (`reverseproxy.go:468–483`, `streaming.go:60` `handleUpgradeResponse`) and bidirectional streaming for those upgrades — disqualifier 1 ("holds an open per-request connection… for its full lifetime") fires for the upgrade path. A finer-grain split (e.g. lifting only `prepareRequest`/`addForwardedHeaders` at `reverseproxy.go:717`/`843`, or only the `finalizeResponse` body copy at `reverseproxy.go:1134`) might pass the rubric, but I couldn't justify either as a single coherent unit from reading alone — they're called from inside `ServeHTTP` with enough shared state (`repl`, `reqHost`, `reqHeader`) that a clean argument list isn't visible.
