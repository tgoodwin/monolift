# Lift Utility Report: Caddy

Caddy is a highly extensible, modular web server written in Go. Its architecture relies on a central configuration that is adapted from various formats (Caddyfile, JSON, etc.) and a pipeline of HTTP handlers (middleware) that process requests. Computationally expensive paths cluster around response encoding (compression), template rendering, dynamic request matching (CEL expressions), and cryptographic operations related to automatic TLS certificate issuance and PKI management.

## Candidates

### 1. `templates.(*TemplateContext).funcMarkdown`
- **File:Line:** `evaluation/caddy/modules/caddyhttp/templates/tplcontext.go:350`
- **Compute envelope:** `compute`. Renders Markdown input into HTML using the `goldmark` library and optionally `chroma` for syntax highlighting. This is a CPU-intensive operation that scales with the size and complexity of the document.
- **Load profile:** `uneven`. Call frequency depends entirely on user-generated content and template usage, making it a prime candidate for offloading during spikes in rendering requests.
- **Data coupling:** `stateless`. It takes an `any` input (typically a string) and returns a string (HTML) and an error. It does not rely on global state or complex local dependencies.
- **Shape-preserving transport:** `yes`. The input is a simple string/any and the output is a string. Both are easily serializable.
- **Failure isolation:** `yes`. It is a pure transformation of data with no side effects.

### 2. `templates.(*Templates).executeTemplate`
- **File:Line:** `evaluation/caddy/modules/caddyhttp/templates/templates.go:455`
- **Compute envelope:** `compute`. Executes Go `text/template` or `html/template` against a buffered response. This involves parsing (if not cached) and executing logic-heavy templates, which can be computationally expensive for complex pages.
- **Load profile:** `uneven`. Driven by per-request hits on templated endpoints.
- **Data coupling:** `read-heavy`. Requires access to the `TemplateContext` and the buffered response body, but does not mutate shared global state.
- **Shape-preserving transport:** `yes`. Although it takes a `ResponseRecorder`, the core data needed is the buffered body and request metadata, which Monolift is designed to handle.
- **Failure isolation:** `yes`. The execution is isolated to the response buffer.

### 3. `caddyhttp.MatchExpression.MatchWithError`
- **File:Line:** `evaluation/caddy/modules/caddyhttp/celmatcher.go:193`
- **Compute envelope:** `compute`. Evaluates a Common Expression Language (CEL) program against an HTTP request. CEL evaluation is designed to be safe but involves significant overhead for complex logic and regular expressions.
- **Load profile:** `uneven`. This runs per-request for every request that hit a route using expression matchers.
- **Data coupling:** `read-heavy`. Reads from the `*http.Request` and replacer.
- **Shape-preserving transport:** `yes`. Takes a request and returns a boolean.
- **Failure isolation:** `yes`. Matching is a read-only operation with respect to the system state.

### 4. `caddyfile.Adapter.Adapt`
- **File:Line:** `evaluation/caddy/caddyconfig/caddyfile/adapter.go:32`
- **Compute envelope:** `compute`. Parses a Caddyfile, builds a server block "pile", and marshals the resulting `caddy.Config` into JSON. For large, complex configurations, this involves significant string processing and recursive structure building.
- **Load profile:** `bursty`. Typically called during server startup or config reload. While infrequent, reloads of massive configs can spike CPU and memory.
- **Data coupling:** `stateless`. It takes a `[]byte` (Caddyfile) and returns a `[]byte` (JSON).
- **Shape-preserving transport:** `yes`. Byte slices are perfectly serializable.
- **Failure isolation:** `yes`. Pure transformation from one format to another.

### 5. `acmeissuer.(*ACMEIssuer).Issue`
- **File:Line:** `evaluation/caddy/modules/caddytls/acmeissuer.go:300`
- **Compute envelope:** `crypto`/`io`. Coordinates the ACME certificate issuance process. While it involves significant network I/O, the underlying cryptographic operations (CSR signing, key generation) are CPU-bound.
- **Load profile:** `bursty`. Certificate renewals often happen in batches or are triggered by on-demand TLS requests, leading to unpredictable load.
- **Data coupling:** `read-heavy`. Depends on the ACME account and issuer configuration.
- **Shape-preserving transport:** `yes`. Takes a `*x509.CertificateRequest` and returns a `*certmagic.IssuedCertificate`. Both are standard Go structs.
- **Failure isolation:** `no`. Issuance has side effects (updating storage with new certs), but these are managed through the `certmagic.Storage` interface which can be distributed.

### 6. `logging.FilterEncoder.EncodeEntry`
- **File:Line:** `evaluation/caddy/modules/logging/filterencoder.go:449`
- **Compute envelope:** `compute`. Applies a series of filters (including regex replacements) to log fields before final encoding. Under high log volume, this filtering logic can consume significant CPU cycles.
- **Load profile:** `bursty`. Scales linearly with request volume and log verbosity.
- **Data coupling:** `stateless`. Operates on a single log entry and its fields.
- **Shape-preserving transport:** `yes`. Log entries and fields are structured data types.
- **Failure isolation:** `yes`. Does not affect the main request flow or global state.

### 7. `caddypki.CA.genRoot`
- **File:Line:** `evaluation/caddy/modules/caddypki/ca.go:315`
- **Compute envelope:** `crypto`. Generates a new RSA or ECC root certificate and private key. This is an extremely expensive cryptographic operation.
- **Load profile:** `periodic`. Only happens when a CA is first initialized or its root expires.
- **Data coupling:** `stateless`. Parameters are passed via the `CA` struct; results are returned for storage.
- **Shape-preserving transport:** `yes`. Returns standard Go crypto types.
- **Failure isolation:** `no`. Results must be persisted to storage.

### 8. `headers.HeaderOps.ApplyTo`
- **File:Line:** `evaluation/caddy/modules/caddyhttp/headers/headers.go:167`
- **Compute envelope:** `logic`. Performs complex header manipulations, including substring and regular expression replacements across all headers or specific fields.
- **Load profile:** `uneven`. Runs per-request if configured.
- **Data coupling:** `stateless`. Mutates the provided `http.Header` map.
- **Shape-preserving transport:** `yes`. `http.Header` is a simple `map[string][]string`.
- **Failure isolation:** `yes`. Only affects the headers of the current request/response.

### 9. `rewrite.Rewrite.Rewrite`
- **File:Line:** `evaluation/caddy/modules/caddyhttp/rewrite/rewrite.go:158`
- **Compute envelope:** `logic`. Performs URI path and query string manipulations, including regex-based replacements and normalization.
- **Load profile:** `uneven`. Runs per-request for matching routes.
- **Data coupling:** `stateless`. Mutates the `*http.Request`.
- **Shape-preserving transport:** `yes`. Monolift handles request mutations.
- **Failure isolation:** `yes`. Isolated to the current request.

### 10. `caddyfile.Tokenize`
- **File:Line:** `evaluation/caddy/caddyconfig/caddyfile/lexer.go:56`
- **Compute envelope:** `compute`. The lexer that breaks Caddyfile text into a stream of tokens. This involves intensive string scanning and classification.
- **Load profile:** `bursty`. Part of the configuration adaptation path.
- **Data coupling:** `stateless`. Takes a byte slice and returns tokens.
- **Shape-preserving transport:** `yes`. Tokens are simple structs.
- **Failure isolation:** `yes`. No side effects.

## Honest assessment

I am most confident in the **Templates** (`funcMarkdown`, `executeTemplate`) and **CEL Expression** candidates. These represent classic "computational units" that are frequently the bottleneck in high-traffic web applications and have clearly defined boundaries. The **Caddyfile Adapter/Tokenize** and **Header/Rewrite** logic are also excellent candidates for offloading the overhead of complex regular expression processing. 

The **ACME Issuer** and **CA Root Generation** are more marginal in terms of frequency, but their high computational cost and potential for network latency (in the case of ACME) make them interesting candidates for specialized remote compute. 

One region I suspect is a great lift candidate but couldn't justify under the strict rubric is the **Zstd/Gzip compression logic**. While the *middleware* that triggers them is in Caddy, the actual heavy-duty compression happens in external libraries (`klauspost/compress`). If Monolift could lift the call to the external `Write` method, it would likely be the single most effective lift in the entire server, but the rubric's requirement to cite specific internal code makes it harder to propose as a Caddy-native region.
