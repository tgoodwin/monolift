# caddy — merged lift-region candidate set (Phase 2b final)

## Methodology note

Phase 1 drafts: claude, codex, gemini.
Phase 2a critics: each model reviewed the other two.
Phase 2b aggregator: claude (opus).

The merged set was produced by applying the deterministic inclusion rules from `PHASE2-PLAN.md` §"Inclusion rules" to the three drafts and three critiques. MODIFY corrections (line cite drift, scope narrowing) are folded into the merged entry. Two MODIFY corrections were verified against the source tree under `evaluation/caddy/`: `InternalIssuer.Issue` is at `modules/caddytls/internalissuer.go:103` (not `modules/caddypki/...`); `correctPassword` is at `modules/caddyhttp/caddyauth/basicauth.go:165`.

The trio's three drafts collectively raised 19 distinct candidates (after dedupe across slightly different framings of the same region). Twelve made the merged set; seven were excluded.

---

## Merged candidates (ranked strongest → weakest)

### M-1: Goldmark markdown render helper (`funcMarkdown`)

- **pick_provenance:** claude+codex+gemini (3/3)
- **critique_status:** KEEP from all 3 critics (no MODIFY)
- **Region root:** `modules/caddyhttp/templates/tplcontext.go:350` — `func (TemplateContext) funcMarkdown(input any) (string, error)`. Builds a fresh `goldmark.New(...)` with GFM, Footnote, and chroma syntax-highlighting on every call and runs `md.Convert([]byte(inputStr), buf)`.
- **Caller(s):** Bound into the template FuncMap at `modules/caddyhttp/templates/tplcontext.go:85` (`"markdown": c.funcMarkdown`); invoked from any user template that pipes content through `{{markdown ...}}` and from markdown-served pages.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — Goldmark + GFM parsing + chroma syntax highlighting is genuinely CPU-heavy on non-trivial Markdown blocks; rebuilds the parser each call.
  - Load profile: yes — uneven per page; spikes with content publishing or doc-site traffic.
  - Coherent unit: yes — pure function: `string → (string, error)`. No request, no FS, no globals.
  - State independence: yes — nothing mutated outside a local pooled buffer.
  - Latency / failure: maybe — invoked synchronously inside template execution; the markdown work itself already dominates per-call latency for non-trivial inputs.
- **Activation shape:** invoked from `text/template` execution (synchronous helper called from M-2's region).
- **Confidence:** high — textbook positive example of a coherent CPU-bound pure function.
- **Risk notes:** Constructing the goldmark instance per call (`tplcontext.go:353`) is wasteful but unrelated to lift; lifting amortizes naturally with a remote-side warm pool. Returns unescaped HTML by design (`tplcontext.go:348`); a lifted version must preserve exact rendering and escaping behavior.

---

### M-2: Buffered HTTP template execution (`executeTemplate`)

- **pick_provenance:** claude+codex+gemini (3/3, after MODIFY)
- **critique_status:** MODIFY from codex (region root narrowed from `Templates.ServeHTTP` at `templates.go:410` to `(*Templates).executeTemplate` at `templates.go:455`); KEEP from claude and gemini on the narrowed framing.
- **Region root:** `modules/caddyhttp/templates/templates.go:455` — `func (t *Templates) executeTemplate(rec caddyhttp.ResponseRecorder, r *http.Request) error`. Builds a request-scoped `TemplateContext` and calls `executeTemplateInBuffer` (`tplcontext.go:240`): `c.tpl.Parse(buf.String())` then `c.tpl.Execute(buf, c)`.
- **Caller(s):** `modules/caddyhttp/templates/templates.go:437` invokes `executeTemplate` after the response recorder buffers an MIME-eligible body inside `Templates.ServeHTTP` (`templates.go:410`).
- **Why useful (rubric scoring):**
  - Compute envelope: yes — Go `text/template` parse + execute on every response, plus the full Sprig FuncMap (`tplcontext.go:71`), markdown via Goldmark+chroma, etc. CPU scales with template size and dynamic includes.
  - Load profile: yes — bursty per-page; static-site-with-dynamic-snippets workloads spike with cache misses or content campaigns.
  - Coherent unit: yes — at the inner `executeTemplate` boundary, input is a buffered response body + request; output is a transformed buffer. Buffered, not streaming — well suited to remote dispatch.
  - State independence: maybe — `TemplateContext.Root` is an `http.FileSystem` for `include`/`readFile`/`listFiles`; if used, the remote replica needs the same view of files. `httpInclude` issues an in-process sub-request (`tplcontext.go:173`), the only piece that genuinely needs same-process state. Configurations without these helpers are clean.
  - Latency / failure: maybe — on the response path, but already buffered and CPU-bound enough (large pages, sprig calls, markdown blocks) that an extra hop fits inside existing budget.
- **Activation shape:** HTTP middleware handler that wraps a `ResponseRecorder` around `next.ServeHTTP`; `executeTemplate` runs after the buffer is populated.
- **Confidence:** medium-high — strongest if the workload uses `markdown` and `httpInclude`-free templates.
- **Risk notes:** `httpInclude` requires a local virtual sub-request; lifts must either disable it, marshal it as a callback, or document the tradeoff. Custom template extensions registered via `http.handlers.templates.functions.*` would also need to ride the lift. The MIME-gating in `Templates.ServeHTTP` (`templates.go:427`) must be preserved by leaving the outer middleware local and lifting only `executeTemplate`.

---

### M-3: Basic auth password hash verification (`correctPassword`)

- **pick_provenance:** claude+codex (2/3, after MODIFY)
- **critique_status:** MODIFY from codex (lift root narrowed from `HTTPBasicAuth.Authenticate` at `basicauth.go:143` to `HTTPBasicAuth.correctPassword` at `basicauth.go:165`); KEEP from claude on codex's framing; KEEP from gemini on both framings. Verified: `correctPassword` is at `basicauth.go:165`.
- **Region root:** `modules/caddyhttp/caddyauth/basicauth.go:165` — `func (hba HTTPBasicAuth) correctPassword(account Account, plaintextPassword []byte) (bool, error)`. Calls `hba.Hash.Compare(account.password, plaintextPassword)`, where `Compare` is `Argon2idHash.Compare` (`argon2id.go:67`, `argon2.IDKey` — memory-hard KDF) or `BcryptHash.Compare` (`bcrypt.go:51`).
- **Caller(s):** `modules/caddyhttp/caddyauth/basicauth.go:157` from `HTTPBasicAuth.Authenticate`, which is itself invoked from `Authentication.ServeHTTP` at `modules/caddyhttp/caddyauth/caddyauth.go:84` for each request hitting a `basic_auth`-protected route.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — Argon2id with default `memory=64MiB, time=1, threads=1` is tens of milliseconds of CPU per call; bcrypt at default cost is similar. Textbook "crypto over more than a small constant" example.
  - Load profile: yes — bursty around login flurries / API key rotation / cron-driven scrapers.
  - Coherent unit: yes — `(account, plaintext) → (bool, error)` is value-typed and pure; the `Comparer` interface (`Compare(hashed, plaintext []byte) (bool, error)`) is already a clean contract.
  - State independence: maybe — the singleflight `HashCache` (`basicauth.go:190`/`:222`) is shared optimization state but trivially replica-local since cache-miss correctness is unchanged.
  - Latency / failure: yes — the region's own contribution is already O(10–100 ms) of KDF work, so a network hop is comfortably in the noise; on failure the caller surfaces 401.
- **Activation shape:** HTTP middleware handler invoked per request whose route includes `basic_auth`.
- **Confidence:** high — textbook positive example. Would change my mind only if real deployments overwhelmingly use bcrypt at default cost on already-cached entries, in which case lift overhead would dominate.
- **Risk notes:** Side-channel timing matters: nonexistent accounts still run a fake password through `Compare` (`basicauth.go:151`); a lifted version must not short-circuit that work. The outer `Authenticate` writes `WWW-Authenticate` via `promptForCredentials` (`basicauth.go:215`) — keeping the outer middleware local and lifting only `correctPassword` cleanly separates pure compute from response-header mutation.

---

### M-4: Internal CA certificate issuance (`InternalIssuer.Issue`)

- **pick_provenance:** claude+codex (2/3, after MODIFY)
- **critique_status:** MODIFY from codex (corrected path: `modules/caddytls/internalissuer.go:103`, not `modules/caddypki/internalissuer.go:103`); KEEP from claude on codex's path; KEEP from gemini on both framings. Verified: only the `caddytls/` path exists in the source tree.
- **Region root:** `modules/caddytls/internalissuer.go:103` — `func (iss InternalIssuer) Issue(ctx context.Context, csr *x509.CertificateRequest) (*certmagic.IssuedCertificate, error)`. Calls `iss.ca.NewAuthority(authCfg)` then `auth.SignWithContext(ctx, csr, ...)` and PEM-encodes the chain.
- **Caller(s):** Plugged into certmagic via the `Issuer` interface; certmagic's per-request `manageOne`/`obtainCert` paths invoke `Issue` whenever a managed name needs a fresh cert. `modules/caddytls/automation.go:233` installs configured issuers into the automation policy; `modules/caddytls/tls.go:585` starts certmagic management for names that can use those issuers.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — RSA/ECDSA signing of an X.509 cert; the smallstep `authority` stack also runs policy + name validation. Tens of ms each on RSA keys.
  - Load profile: yes — periodic renewal sweeps (cron-shaped), plus on-demand TLS bursts when new SNIs appear (campaign launches, multi-tenant onboarding).
  - Coherent unit: yes — `Issue(ctx, *x509.CertificateRequest) → (*certmagic.IssuedCertificate, error)`; defined by certmagic's `Issuer` interface, so the contract is enforced.
  - State independence: maybe — `iss.ca` carries the (root or intermediate) signing key. Lifting either ships the key to the remote (security tradeoff) or keeps the signer local and lifts only the CSR pre-processing — narrowing the win.
  - Latency / failure: yes — caller is either certmagic's renewal goroutine (background) or a TLS handshake on cold cache, both with seconds of budget.
- **Activation shape:** library callback (`certmagic.Issuer.Issue`) invoked from certmagic's renewal/issue paths.
- **Confidence:** medium — the signature work is real but bounded; lift payoff is largest under bursty on-demand TLS or fleet-wide renewal storms.
- **Risk notes:** Signing-key custody is the dominant constraint, not code coupling. `ca.NewAuthority` constructs a fresh `authority.Authority` each call — fine for lift, mildly wasteful otherwise.

---

### M-5: HTTP response compression (`Encode.ServeHTTP`)

- **pick_provenance:** claude+codex (2/3, with disputed framing)
- **critique_status:** Disputed framing — claude critic and codex critic each issued MODIFY in opposite directions (claude: lift the middleware boundary `Encode.ServeHTTP` at `encode.go:154`; codex: lift the per-byte `responseWriter.Write` at `encode.go:313`). KEEP from gemini on both framings. Aggregator sided with claude's framing on rubric criterion 3 (Coherent unit): per-byte `Write` becomes one network round-trip per `Write([]byte)` call, which violates the "value-typed in/out" contract; the middleware wrapper boundary is the only framing that yields a buffered one-in-one-out unit. See Discrepancies §1.
- **Region root:** `modules/caddyhttp/encode/encode.go:154` — `func (enc *Encode) ServeHTTP(w, r, next)`. Wraps the downstream `ResponseWriter` with the chosen encoder; per-byte compute happens in `responseWriter.Write` (`encode.go:313`) and `responseWriter.Close` (`encode.go:407`). Encoders are registered modules: `gzip` (`encode/gzip/gzip.go:84`), `zstd` (`encode/zstd/zstd.go:118`), `brotli`.
- **Caller(s):** Inserted into any route stack that uses the `encode` directive; effectively wraps every response that matches `Encode.Matcher` (text-y MIME types).
- **Why useful (rubric scoring):**
  - Compute envelope: yes — gzip/zstd/brotli compression on every text-y response body; CPU scales linearly with bytes encoded.
  - Load profile: yes — bursty with traffic; payload-driven (large HTML/JSON benefits most).
  - Coherent unit: maybe — the middleware boundary itself is clean, but the actual compressor is structurally tied to a wrapped `http.ResponseWriter`. Lifting requires either streaming the response bytes to the remote or buffering the whole body — both add per-byte overhead that may erase the compression cost saving.
  - State independence: yes — uses a `sync.Pool` of writers (`encode.go:204`), trivially replica-local.
  - Latency / failure: maybe — sits on the response critical path; only helps if encoding cost > round-trip cost (large bodies, expensive level like zstd best).
- **Activation shape:** HTTP middleware handler that wraps the downstream `ResponseWriter`.
- **Confidence:** medium — the *function* is great compute-per-call, but the streaming response-writer integration makes it a poor structural fit for lift-without-buffering. Best treated as a stress case for the lift mechanism rather than a slam-dunk win.
- **Risk notes:** The `responseWriter` even implements `ReadFrom` for sendfile (`encode.go:374`); a remote lift would have to break sendfile and either buffer or stream. Disqualifier-adjacent — it does not hold a per-request socket itself, but it is intimately tied to the one held by the parent server.

---

### M-6: Caddyfile config adaptation (`Adapter.Adapt`) [disputed]

- **pick_provenance:** codex+gemini (2/3)
- **critique_status:** DISPUTED — claude critic DROPped (fails criterion 2: control-plane-only, abuts the rubric's "initialization functions" exclusion); codex and gemini both KEEP. Aggregator decision: include with a "disputed" annotation. Rubric defensibility: criterion 3 (Coherent unit) is unambiguously satisfied — `bytes → (jsonBytes, warnings, error)` is the cleanest pure-transform shape in the entire corpus. Criterion 2 (Variable/spikable load) is genuinely marginal: `Adapt` fires on every admin `/load` reload, not strictly at boot, so it is bursty rather than uniformly low-traffic; large multi-tenant configs and frequent deployment cadences push it into a usefully heavy regime. The rubric's negative example is "uniformly low-traffic (e.g. once-a-day vacuum)", which `Adapt` does not match. See Discrepancies §2.
- **Region root:** `caddyconfig/caddyfile/adapter.go:32` — `func (a Adapter) Adapt(body []byte, options map[string]any) ([]byte, []caddyconfig.Warning, error)`. Tokenizes (`adapter.go:45`), runs server-type setup (`:50`), checks formatting (`:57`), and marshals JSON (`:61`).
- **Caller(s):** `caddyconfig/load.go:97` adapts non-JSON config on the admin `/load` endpoint; `caddyconfig/load.go:209` dispatches to the selected adapter.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — tokenize + parse + setup + JSON marshal on the full config. Scales with config size.
  - Load profile: maybe — admin reload work, not steady traffic. Bursty during deployment campaigns; quiescent otherwise.
  - Coherent unit: yes — `bytes → (jsonBytes, warnings, error)`. Cleanest contract in the corpus.
  - State independence: maybe — parsing is bounded, but imports can read files via `doSingleImport` (`caddyconfig/caddyfile/parse.go:588`), and setup depends on registered directives/modules.
  - Latency / failure: yes — control-plane work where adaptation errors are already part of the contract (return 400).
- **Activation shape:** Admin API config load/adapt operation.
- **Confidence:** low-medium — depends entirely on whether the deployment has many large configs or frequent reloads. Single-tenant boot-only deployments would not benefit.
- **Risk notes:** Import resolution and module-registry assumptions complicate a "pure transform" framing. The lifted replica needs the same module registry built into its binary; for vanilla caddy that's automatic (modules register at init time), but custom-module deployments must ride along.

---

### M-7: Directory listing materialization (`loadDirectoryContents`)

- **pick_provenance:** codex (1/3, after MODIFY) — weak consensus
- **critique_status:** MODIFY from claude (lift root narrowed from `serveBrowse` at `browse.go:78` to `loadDirectoryContents` at `browse.go:119` to drop redirect/conditional-GET/content-negotiation mixed concerns); KEEP from gemini on the broader `serveBrowse` framing.
- **Region root:** `modules/caddyhttp/fileserver/browse.go:119` — `loadDirectoryContents` (called from `serveBrowse`). Reads up to the directory entry limit (`browse.go:226`, default 10000 entries), stats entries (`browsetplcontext.go:60`), handles symlinks (`:94`), and sorts (`:223`).
- **Caller(s):** `modules/caddyhttp/fileserver/browse.go:78` `serveBrowse` calls `loadDirectoryContents`; `serveBrowse` itself is invoked from `modules/caddyhttp/fileserver/staticfiles.go:371` for directory requests.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — for large browsable directories, stat + symlink resolution + sort over thousands of entries is real CPU and IO.
  - Load profile: yes — crawlers, indexers, and large file shares can repeatedly trigger listing work.
  - Coherent unit: yes (after MODIFY) — narrowing to `loadDirectoryContents` drops HTTP-response-protocol coupling; the contract becomes `(ctx, fs, dir, root, urlPath, repl) → (listing, error)`.
  - State independence: maybe — depends on the configured filesystem, hidden-file rules, symlink policy, and request query parameters (sort key).
  - Latency / failure: maybe — request-path filesystem work; large directories can already be visibly slow.
- **Activation shape:** Static file server directory request.
- **Confidence:** medium — only material for deployments that actually serve directory listings on large directories.
- **Risk notes:** Filesystem locality dominates practical lift difficulty; like M-9 (`funcInclude`/`readFile`), this depends on the deployment's `http.FileSystem` being virtualizable or replicated to the lifted replica. Disqualifier-adjacent on rubric §"Disqualifiers" item 2 (local-FS reads).

---

### M-8: FastCGI transport round-trip [weak consensus]

- **pick_provenance:** claude (1/3) — weak consensus
- **critique_status:** KEEP from gemini ("clean RoundTripper interface"); DROP from codex (compute envelope and state independence — mostly proxies to PHP-FPM, often via host-local Unix socket). Aggregator includes per Rule 4 with the codex caveat preserved as a deployment-topology constraint. See Discrepancies §3.
- **Region root:** `modules/caddyhttp/reverseproxy/fastcgi/fastcgi.go:163` — `func (t Transport) RoundTrip(r *http.Request) (*http.Response, error)`. Builds the FastCGI env, dials backend (`fastcgi.go:207`), runs `client.Get/Post/Head/Options/...` over the FastCGI binary protocol.
- **Caller(s):** Registered as a reverse-proxy transport (`http.reverse_proxy.transport.fastcgi`); invoked from `Handler.reverseProxy` at `modules/caddyhttp/reverseproxy/reverseproxy.go:993` for every PHP request.
- **Why useful (rubric scoring):**
  - Compute envelope: maybe — protocol marshalling + IO; per-call cost dominated by PHP-FPM execution on the other side.
  - Load profile: yes — every dynamic PHP page hit.
  - Coherent unit: yes — implements stdlib `http.RoundTripper`: `(*Request) → (*Response, error)`, value-only contract.
  - State independence: yes — opens a fresh `net.Dialer` connection per call, no shared per-process state.
  - Latency / failure: maybe — on the response path but the call is already a network/socket hop, so an extra one is noticeable but not pathological.
- **Activation shape:** `http.RoundTripper` invoked from the reverse-proxy middleware.
- **Confidence:** low-medium — lift is structurally clean, but the practical win depends on backend topology.
- **Risk notes:** Hardcoded local-Unix-socket configurations (very common for PHP-FPM) are a structural blocker for lift unless the socket is exported. TCP-PHP-FPM deployments lift cleanly. The transport reads `r.Body` as a stream into FCGI Stdin records, so request-body buffering still has to happen somewhere.

---

### M-9: Templates `funcInclude` / `funcReadFile` [weak consensus]

- **pick_provenance:** claude (1/3) — weak consensus
- **critique_status:** KEEP from gemini ("offloading recursive template parsing within includes is useful"); DROP from codex (state-independence/disqualifier coupling — primary effect is reading `c.Root`, narrower-and-more-coupled subset of M-2). Aggregator includes per Rule 4 with the disqualifier-edge preserved. See Discrepancies §4.
- **Region root:** `modules/caddyhttp/templates/tplcontext.go:112` — `funcInclude(filename string, args ...any) (string, error)`; and `tplcontext.go:136` `funcReadFile(filename string) (string, error)`. Both hit `c.Root` (an `http.FileSystem`) and stream the file into a buffer (`readFileToBuffer` at `tplcontext.go:150`).
- **Caller(s):** Registered into the template FuncMap at `tplcontext.go:80–81`; invoked any time a user template uses `{{include ...}}` or `{{readFile ...}}`.
- **Why useful (rubric scoring):**
  - Compute envelope: maybe — file IO + (for `include`) recursive template parse/execute; expensive if many included files per page.
  - Load profile: yes — uneven per page.
  - Coherent unit: yes — clear `(filename, args) → string` contract.
  - State independence: maybe — depends on `c.Root`, configurable via `FileRoot` (`templates.go:349`). Virtualized as `http.FileSystem`, so it *can* be backed by a network or virtual FS, but as-shipped deployments use a local directory.
  - Latency / failure: maybe — synchronous within template execution; per-file cost is small but compounds with many includes.
- **Activation shape:** Template helper invoked from M-2's region.
- **Confidence:** low — useful only if the file system can be virtualized for the lifted replica.
- **Risk notes:** On the edge of disqualifier 2 ("direct read of repo-local filesystem paths that cannot be virtualized"). Listed because the `http.FileSystem` indirection makes virtualization plausible — but that integration cost has to be paid.

---

### M-10: Filter encoder for log entries (`FilterEncoder.EncodeEntry`) [weak consensus]

- **pick_provenance:** gemini (1/3) — weak consensus
- **critique_status:** KEEP from codex (regex/multi-regex/hashing/masking per entry; coherent zap encoder boundary); DROP from claude (per-entry filtering is sub-microsecond field-walking; the rubric's "O(1) constant-time helper" negative). Aggregator includes per Rule 4 with the compute-envelope concern preserved. See Discrepancies §5.
- **Region root:** `modules/logging/filterencoder.go:449` — `EncodeEntry(entry zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error)` applies a series of filters (regex replace, hash, masking) to log fields before final encoding.
- **Caller(s):** Zap logger core when configured with Caddy's filtering encoder; invoked per log entry on every emitting goroutine.
- **Why useful (rubric scoring):**
  - Compute envelope: maybe — sub-microsecond on simple field walks; meaningful only when configured with multiple regex / hash / mask filters over verbose request logs.
  - Load profile: yes — scales linearly with request volume and log verbosity.
  - Coherent unit: yes — `(entry, fields) → buffer` is a clean structured-data transform.
  - State independence: yes — pure transformation of log data; no shared mutable state outside a local buffer pool.
  - Latency / failure: maybe — synchronous on every log emission, including request-path emissions; if remoted, network overhead would dwarf typical encoding cost.
- **Activation shape:** zap encoder invoked per log entry.
- **Confidence:** low — the lift only pays off in deployments configured with heavy regex/hash/mask filtering at high log volume; vanilla configurations fail criterion 1.
- **Risk notes:** Synchronous on every log emission means the latency case is fragile — a slow remote replica would slow every emitting goroutine. Fits the rubric only when the filter chain itself is non-trivial.

---

### M-11: Reverse-proxy active health check (`doActiveHealthCheck`) [weak consensus]

- **pick_provenance:** codex (1/3) — weak consensus
- **critique_status:** KEEP from gemini (background task, body regex evaluation, off-main-path); DROP from claude (fails criterion 4: probe mutates `upstream.Host` health counters via `markUnhealthy`/`markHealthy` closures and emits events — rubric's "in-process pub-sub fan-out where every replica must see every event" negative; also fails criterion 1: per-probe CPU is sub-millisecond). Aggregator includes per Rule 4 but flags state-coupling as the blocking concern. See Discrepancies §6.
- **Region root:** `modules/caddyhttp/reverseproxy/healthchecks.go:391` — `(*Handler).doActiveHealthCheck` sends a health request, evaluates status/body criteria, and updates upstream health.
- **Caller(s):** `modules/caddyhttp/reverseproxy/reverseproxy.go:421` starts the active health checker goroutine; `modules/caddyhttp/reverseproxy/healthchecks.go:371` invokes `doActiveHealthCheck` per upstream target via per-upstream goroutines launched at `healthchecks.go:299`.
- **Why useful (rubric scoring):**
  - Compute envelope: maybe — per-probe is one HTTP GET plus an optional regex over a body bounded by `MaxSize`. Sub-millisecond unless the regex is non-trivial on a large body.
  - Load profile: maybe — work scales with upstream count and probe interval rather than external traffic.
  - Coherent unit: maybe — the probe itself is coherent, but it mutates upstream health counters and emits events.
  - State independence: no — the closures at `healthchecks.go:464`/`:485` mutate health truth (`upstream.Host` counters) and emit `h.events`; cross-replica consistency is non-trivial.
  - Latency / failure: yes — background work; probe failures are reduced to health-state changes, not client failures.
- **Activation shape:** Periodic background goroutine per reverse-proxy handler.
- **Confidence:** low — a "no" on State independence is a near-disqualifier under the rubric; included only because gemini's KEEP and codex's own pick give Rule 4 weak consensus.
- **Risk notes:** Splitting health truth across replicas is a real correctness risk. The `markHealthy`/`markUnhealthy` closures are the structural coupling point; lifting only the probe-and-evaluate step (returning a verdict for the local handler to apply) would fix the state-coupling issue but is structurally a different region than the one codex picked.

---

### M-12: SRV dynamic upstream refresh (`SRVUpstreams.GetUpstreams`) [weak consensus]

- **pick_provenance:** codex (1/3) — weak consensus
- **critique_status:** KEEP from gemini (DNS SRV lookup is offload-able); DROP from claude (the source comment at `reverseproxy.go:1581` explicitly says this "should be very fast and stable" and "may be called multiple times per request" — direct hit on rubric criterion 5's "tight synchronous request path with strict p99 budget" negative; the global `srvsMu`/`srvs` cache also creates per-replica freshness divergence). Aggregator includes per Rule 4 but flags the latency-budget concern as the blocking issue. See Discrepancies §7.
- **Region root:** `modules/caddyhttp/reverseproxy/upstreams.go:122` — `SRVUpstreams.GetUpstreams(r *http.Request)` resolves SRV records and converts them into reverse-proxy upstreams with a freshness-gated cache (`upstreams.go:129`/`:141`); cache miss does a DNS SRV lookup at `:153` and allocates upstreams at `:174`.
- **Caller(s):** `modules/caddyhttp/reverseproxy/reverseproxy.go:577` calls the dynamic upstream source during proxy loop iteration; `reverseproxy.go:1586` documents that this source may be called multiple times per request.
- **Why useful (rubric scoring):**
  - Compute envelope: maybe — cache hits are O(1); only stale refreshes do the DNS SRV lookup and slice allocation.
  - Load profile: maybe — calls can happen per retry, but expensive lookup work is gated by freshness checks.
  - Coherent unit: yes — `request + SRV-source-config → ([]Upstream, error)`.
  - State independence: maybe — global SRV cache and lock at `upstreams.go:126` make behavior replica-local and time-dependent; per-replica caches diverge on freshness.
  - Latency / failure: maybe — explicit grace-period fallback to cached upstreams (`upstreams.go:159`), but caller comment requires fast-and-stable.
- **Activation shape:** Reverse-proxy request/retry upstream discovery.
- **Confidence:** low — the caller contract documented in the source ("very fast and stable") is itself a near-disqualifier under criterion 5.
- **Risk notes:** Lifting a request-path callback that the caller explicitly requires to be fast and stable is structurally counterproductive. Only the cache-miss branch has any compute envelope, and gating it via the existing freshness check is what already keeps the steady-state cost low.

---

## Discrepancies

### §1 — Encode middleware framing (M-5): per-byte vs. middleware boundary

Codex picked the per-byte streaming write at `encode.go:313` (`responseWriter.Write`); claude picked the middleware boundary at `encode.go:154` (`Encode.ServeHTTP`); claude's critique flagged codex's framing as MODIFY (lift the middleware, not the per-write), and codex's critique flagged claude's framing as MODIFY (lift the per-write, not the middleware). Gemini KEEPs both.

**Aggregator sided with claude's framing.** Rubric criterion 3 (Coherent unit) requires "a function or method with parameters that are values" with a clear input/output contract. Per-byte `Write([]byte)` produces one network round-trip per write — that is not a coherent value-typed unit; it is a streaming dispatch where the round-trip cost dominates the compression saving. The middleware boundary at least permits a buffered "one body in, one compressed body out" framing if buffering is added. Either framing is structurally challenged on coherent-unit; the middleware framing is at least repairable.

### §2 — Caddyfile Adapter.Adapt (M-6): control-plane vs. bursty

Codex and gemini both picked `Adapter.Adapt`; claude critiqued both as DROP citing rubric criterion 2's "uniformly low-traffic" negative and the "initialization functions" exclusion.

**Aggregator sided with the codex+gemini majority** but tagged the entry as "disputed" and dropped confidence to low-medium. Rubric defensibility: criterion 3 is unambiguously satisfied (`bytes → (jsonBytes, warnings, error)` is the cleanest pure-transform shape in the corpus); criterion 2 is genuinely marginal but not negative — `Adapt` runs on every admin `/load` reload, not strictly at boot, and the rubric's negative example is "uniformly low-traffic (e.g. once-a-day vacuum)", which `Adapt` does not match. Bursty deployment cadences (multi-tenant hosting platforms, frequent canary rollouts) push it into the heavy regime. I marked the load profile as "maybe" rather than "yes" to reflect the real concern.

### §3 — FastCGI RoundTrip (M-8)

Gemini KEEP, codex DROP (compute envelope + Unix-socket state coupling). Aggregator included per Rule 4 with codex's deployment-topology caveat preserved as a Risk note, and lowered confidence to low-medium. The structural shape (stdlib `http.RoundTripper`) is genuinely clean; the practical win depends on PHP-FPM running over TCP rather than a host-local Unix socket. Under the rubric's "lift only pays off if the region has work worth offloading and is loosely coupled" framing, this is a real lift target only for a subset of deployments — but that subset is not negligible.

### §4 — Templates `funcInclude`/`funcReadFile` (M-9)

Gemini KEEP, codex DROP (state-independence/disqualifier coupling: primary effect is reading `c.Root`, narrower-and-more-coupled subset of M-2). Aggregator included per Rule 4 with the disqualifier-edge preserved as a Risk note. The candidate is on the edge of rubric §"Disqualifiers" item 2 ("direct read of repo-local filesystem paths that cannot be virtualized"); the `http.FileSystem` indirection moves it just inside the line, but only conditionally on the deployment's FS being virtualizable.

### §5 — FilterEncoder.EncodeEntry (M-10)

Codex KEEP (regex/hash/mask per entry can add up at scale); claude DROP (sub-microsecond field-walking on a small map — rubric's "O(1) constant-time helper" negative). Aggregator included per Rule 4 with confidence lowered to low. The honest read: in a vanilla configuration this fails criterion 1, but in a heavy-filter / high-volume configuration it can plausibly pass. The synchronous-on-every-emit latency case is fragile — flagged in Risk notes.

### §6 — doActiveHealthCheck (M-11)

Gemini KEEP, claude DROP (mutates upstream health counters via closures, emits events — rubric's "in-process pub-sub fan-out where every replica must see every event" negative under criterion 4; also sub-millisecond CPU per probe under criterion 1). Aggregator included per Rule 4 but the entry is honestly one of the weaker picks in the merged set: the State Independence score is "no" (not "maybe"), which under the rubric is a near-disqualifier. The Risk notes call out that lifting only the probe-and-evaluate step (returning a verdict for the local handler to apply) would address the state-coupling, but that is structurally a different region than codex's pick.

### §7 — SRV upstreams (M-12)

Gemini KEEP, claude DROP (caller contract documented at `reverseproxy.go:1581` requires fast-and-stable; calls "may be called multiple times per request" — direct hit on rubric criterion 5's "tight synchronous request path" negative). Aggregator included per Rule 4 but with the caller-contract concern preserved. This and M-11 are the two weakest entries in the merged set; both depend on Rule 4's "at least one critic KEEPs" threshold and both have a strong rubric argument from the dissenting critic.

---

## Excluded candidates

- **claude C-8 / On-demand TLS ask (`PermissionByHTTP.CertificateAllowed` at `caddytls/ondemand.go:119`)** — Rule 5: codex DROP and gemini DROP, both citing criterion 1 (compute envelope). Single outbound HTTP call with no payload-scaled work; no aggregator defense.
- **gemini 3 / CEL `MatchExpression.MatchWithError` (corrected line cite: `celmatcher.go:213`)** — Rule 5: claude DROP and codex DROP, both citing criterion 5 (sub-ms request matching path). Adding a network hop per matcher evaluation is incompatible with the mux-dispatch budget.
- **gemini 5 / `ACMEIssuer.Issue` at `caddytls/acmeissuer.go:300`** — Rule 5: claude DROP and codex DROP. The function body is a one-line delegation to certmagic's external issuer; no caddy-internal work to lift at this site. M-4 (`InternalIssuer.Issue`) is the structurally analogous candidate that contains the actual caddy-internal signing work.
- **gemini 7 / `CA.genRoot` at `caddypki/ca.go:315`** — Rule 5: claude DROP and codex DROP, both citing the rubric's exact "Initialization functions whose work happens once at boot" exclusion. Fires once per multi-year CA lifetime.
- **gemini 8 / `HeaderOps.ApplyTo` (corrected line cite: `headers/headers.go:220`)** — Rule 5: claude DROP and codex DROP. Per-request header mutation on small maps; fails criterion 1 (compute envelope) and criterion 5 (sub-ms request path).
- **gemini 9 / `Rewrite.Rewrite` at `rewrite/rewrite.go:158`** — Rule 5: claude DROP and codex DROP. Per-request URI/path manipulation with tiny payloads and tight latency budget; same shape failure as the headers candidate.
- **gemini 10 / `caddyfile.Tokenize` at `caddyfile/lexer.go:56`** — Rule 5: claude DROP and codex DROP. Subordinate to M-6 (`Adapter.Adapt`) — Adapt already calls Parse which calls Tokenize, so picking both is double-counting; even standalone it fails criterion 2 (control-plane only).
