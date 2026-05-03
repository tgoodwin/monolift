# Critique of caddy drafts by codex

## Verdicts on claude's draft

| Candidate ID | Verdict | One-paragraph reasoning |
|---|---|---|
| C-1 (HTTP basic-auth credential check) | MODIFY | Real target, but the lift root should be `evaluation/caddy/modules/caddyhttp/caddyauth/basicauth.go:165` `HTTPBasicAuth.correctPassword` or the `Hash.Compare` implementation rather than `Authenticate`. The broader method parses the request and writes `WWW-Authenticate`; the narrower method preserves the compute-envelope and coherent-unit criteria while keeping response side effects local. |
| C-2 (Templates middleware render response body) | MODIFY | Keep the candidate, but frame it as `evaluation/caddy/modules/caddyhttp/templates/templates.go:455` `(*Templates).executeTemplate`. `ServeHTTP` includes downstream handler execution, response-recorder gating, and final header writes; `executeTemplate` is the buffered body transform that most clearly satisfies compute envelope and coherent unit. |
| C-3 (Goldmark markdown render helper) | KEEP | rubric criterion satisfied: compute envelope. Goldmark plus GFM, footnotes, heading IDs, and Chroma syntax highlighting is real CPU proportional to Markdown size, and the helper is a clean value-in/value-out unit with minimal Caddy state. |
| C-4 (HTTP response compression) | MODIFY | Keep the compression candidate, but the useful root is the streaming write path at `evaluation/caddy/modules/caddyhttp/encode/encode.go:313` `(*responseWriter).Write`, with close/flush behavior at `evaluation/caddy/modules/caddyhttp/encode/encode.go:407` as part of the same boundary. `Encode.ServeHTTP` mostly installs the wrapper; the per-byte compute happens in the writer. |
| C-5 (PKI internal CA issue certificate from CSR) | MODIFY | The candidate is good, but the cited path is wrong: the region root is `evaluation/caddy/modules/caddytls/internalissuer.go:103` `InternalIssuer.Issue`, not `modules/caddypki/internalissuer.go`. It remains a useful coherent crypto unit, with the same key-custody caveat noted in the draft. |
| C-6 (Template include/readFile) | DROP | Fails the state-independence/disqualifier side of the rubric more than it helps the compute envelope: the primary effect is reading `c.Root` from a deployment filesystem, and it is a narrower, more state-coupled slice of the broader template execution candidate already in my C-5. |
| C-7 (FastCGI transport round-trip) | DROP | Fails compute envelope and state independence. The call is mostly a proxy transport to PHP-FPM, often through a host-local Unix socket, and it returns a response body tied to the backend connection; remote dispatch mostly adds another hop around work performed elsewhere. |
| C-8 (On-demand TLS ask permission endpoint) | DROP | Fails compute envelope. It is a single outbound HTTP policy check with no payload-scaled work or aggregation, and it runs on the TLS permission path where the failure model is more important than offloading a tiny wrapper. |

## Verdicts on gemini's draft

| Candidate ID | Verdict | One-paragraph reasoning |
|---|---|---|
| G-1 (`templates.(*TemplateContext).funcMarkdown`) | KEEP | rubric criterion satisfied: compute envelope. This is the same strong Markdown target as claude C-3 and my C-3: expensive parsing/highlighting, simple output contract, and little mutable state. |
| G-2 (`templates.(*Templates).executeTemplate`) | KEEP | rubric criterion satisfied: coherent unit. The buffered response plus request context becomes a rewritten buffer, and the parse/execute work is meaningful under template-heavy pages; helper functions add state caveats but do not invalidate the region. |
| G-3 (`caddyhttp.MatchExpression.MatchWithError`) | DROP | Fails latency/failure and only maybe satisfies compute envelope. The actual method is at `evaluation/caddy/modules/caddyhttp/celmatcher.go:213`, but more importantly it is request routing/matching work expected to be very fast and reliable on the synchronous dispatch path. |
| G-4 (`caddyfile.Adapter.Adapt`) | KEEP | rubric criterion satisfied: coherent unit. This matches my C-4: config bytes in, JSON/warnings/error out, with parse/setup/format/marshal work that can spike during large reloads. |
| G-5 (`acmeissuer.(*ACMEIssuer).Issue`) | DROP | Fails coherent-unit and Caddy-local compute criteria. In this tree the method is a one-line delegation to CertMagic's issuer, so the real ACME work and side effects live outside the cited region; my internal issuer candidate is the cleaner Caddy-native certificate lift. |
| G-6 (`logging.FilterEncoder.EncodeEntry`) | KEEP | rubric criterion satisfied: variable/spikable load. Under high request/log volume this can apply regex, multi-regex, hashing, masking, and nested field filtering per entry, and the zap encoder boundary is a coherent structured-data transform. |
| G-7 (`caddypki.CA.genRoot`) | DROP | Fails load profile and the rubric's "no initialization functions" guidance. Root generation is expensive, but it happens only when initializing or replacing CA material and immediately persists trust keys, so it is not a useful under-load lift region. |
| G-8 (`headers.HeaderOps.ApplyTo`) | DROP | Fails compute envelope and latency tolerance. Even with regex replacements, this normally mutates small header maps on the request/response critical path; the line cite is also stale because `ApplyTo` is at `evaluation/caddy/modules/caddyhttp/headers/headers.go:220`. |
| G-9 (`rewrite.Rewrite.Rewrite`) | DROP | Fails compute envelope and latency tolerance. URI and query rewrites are small synchronous request mutations, with occasional regex on short paths; this is route plumbing, not work worth a remote call. |
| G-10 (`caddyfile.Tokenize`) | DROP | Structurally worse than my C-4 `Adapter.Adapt`. Tokenization is a real parser substep, but lifting only the lexer would insert a remote boundary inside a larger config adaptation pipeline without producing the useful final artifact. |

## Overlooked

None.

## Overall observations

Both foreign drafts converge cleanly with my draft on Markdown rendering, buffered template execution, Caddyfile adaptation, and the existence of real compression/auth/certificate work. Claude is more rigorous about state and failure caveats but often chooses too-wide roots; Gemini finds a plausible logging candidate but over-includes small request-path mutators and one-time initialization work. The main divergence is around TLS and proxy peripherals, where wrapper methods and outbound transports look clean syntactically but often fail the compute-envelope or state-independence criteria.
