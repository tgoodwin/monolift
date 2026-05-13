# SPRINT-0048: Phased codegen expansion — receiver stubs, multi-return, corpus e2e coverage

**Status:** planned
**Predecessors:** SPRINT-0046 (pipeline optimization & multi-lift proof), SPRINT-0043 (reverse-import scoping & 6-project coverage), SPRINT-0039 (72-trace corpus analysis)

## Intent

Expand activation-path e2e lift coverage from the current 7 hand-picked targets (0 corpus traces from the SPRINT-0039 matrix) into the 72-row corpus trace matrix by adding codegen capabilities in ROI order. Each capability unlocks a batch of corpus traces, which are then scaffolded as e2e targets and verified in Kind.

The 72-trace matrix reveals the codegen pipeline's current reach and its walls:

| Blocker category | Trace count | Capability needed |
|---|---:|---|
| Already generator-eligible | 2 | None (miniflux/M-1 RefreshFeed, miniflux/M-3 SanitizeHTML) |
| Receiver/method stub (stateless) | 5 | Codegen: method call syntax, receiver policies |
| `(T, error)` multi-return | ~49 | Codegen: multi-return result codec, error semantics |
| Streaming-to-bytes codec | 1 | Codegen: `io.ReadSeeker` → `[]byte` |
| Config-only state | ~11 | Receiver serialization + config struct serializability |
| Client-reconstructible (DB/HTTP/mailer/store) | ~40 | Reconstructor registry families — **future sprints** |
| Shared-state (App receivers) | 10 | Deferred — shared-state coordination |
| Proxy-required | 4 | Deferred — HTTP stream proxy transport |
| Infeasible | 1 | mattermost/M-4 structural gap |

This sprint focuses on **shape** capabilities — receiver method support, `(T, error)` multi-return, same-package invocation adapters, `context.Context`/logger reconstruction, and streaming-to-bytes codec — which together unlock ~12 corpus traces without requiring new reconstructor families. Reconstructor families (DB pools, HTTP clients, mailers, object stores) are the next-largest unlock (~40 traces) and are explicitly deferred to future sprints.

The sprint is designed for overnight AI-model execution via sprint-execute with best-effort skip-on-failure so the pipeline doesn't get stuck.

## Scope boundaries

**In scope:**
- Machine-readable corpus manifest with per-trace metadata and structured skip/pass/fail statuses
- Best-effort subprocess-based sweep runner for overnight execution
- Receiver/method codegen: patching, plan building, server template, client template, admission
- Receiver policy taxonomy: `receiver_boundary` (serialize as request data), `receiver_zero` (zero-value construction), `receiver_factory` (known constructor)
- Same-package exported invocation adapter for unexported functions and methods
- `(T, error)` multi-return support: result codec, transport vs. application error distinction, fail-open/fail-closed behavior
- `context.Context` parameter reconstruction as `context.Background()`
- No-op logger reconstruction for `mlog.LoggerIFace` and similar
- Streaming-to-bytes codec for `io.ReadSeeker` / `io.Reader` boundary params
- E2e targets for unlocked corpus traces across all 6 projects
- Config-only receiver corpus traces as stretch targets

**Out of scope:**
- New reconstructor families: DB pools (`*sql.DB`), HTTP clients, mailers, object stores — **deferred to SPRINT-0049+** as the next-highest-ROI backlog
- Shared-state traces: `caddy/M-5`, `caddy/M-7`, `listmonk/M-9`, `mattermost/M-3`, `mattermost/M-6`, `mattermost/M-7`, `mattermost/M-8`, `mattermost/M-9`, `mattermost/M-12`, `mattermost/M-15`
- Proxy-required boundaries: `caddy/M-2`, `listmonk/M-8`
- Mutable boundary write-back (e.g., `pocketbase/M-6` `(*PasswordField).setValue`)
- `mattermost/M-4` structural activation gap
- gRPC, streaming beyond finite-reader-to-bytes, multi-cut, Helm/Kustomize
- CI/CD integration, production deployment

## Trace disposition summary

| Project | First-wave attempts | Stretch attempts | Deferred |
|---|---|---|---|
| Caddy | `M-1` (`funcMarkdown`) | `M-3` (`correctPassword`), `M-4` (`Issue`) | `M-2` (proxy), `M-5`, `M-7` (shared-state) |
| Gitea | `M-16` (`HashWithSaltBytes`), `M-17` (`RenderFullFile`) | `M-13` (`send`) | `M-1`, `M-2`, `M-3`–`M-12`, `M-14`, `M-15`, `M-19` (DB/queue/bootstrap state) |
| Listmonk | — | `M-7` (`CompileTemplate`) | `M-1`–`M-6`, `M-8`–`M-10` (DB/app/proxy state) |
| Mattermost | `M-14` (`PBKDF2.Hash`), `M-1` (`Extract`) | — | `M-2`–`M-13`, `M-15` (shared-state, DB/app) |
| Miniflux | `M-1` (`RefreshFeed`), `M-3` (verify), `M-6` (`ParseFeed`) | — | `M-2`, `M-4`, `M-5`, `M-7`–`M-10`, `M-13`, `M-14` (DB/HTTP/client state) |
| PocketBase | `M-3` (`Validate`) | `M-11` (`resolveEmailTemplate`), `M-5` (`SendRecordPasswordReset`), `M-2` (`recordAuthWithOAuth2`) | `M-1`, `M-4`, `M-6`–`M-10` (DB/app/file/mutable state) |

## Task list

### Phase 0: Corpus manifest, sweep runner, and batch infrastructure

Build the infrastructure needed for overnight best-effort execution before attempting any new targets.

- [x] 0.1: Audit the current 7 passing activation targets and record whether any correspond exactly to matrix trace IDs. Write the mapping to `docs/research/runs/SPRINT-0048-baseline.md`
- [x] 0.2: Create `test/e2e/activation_corpus_traces.yaml` with one row per matrix trace: trace ID, project, function name, target `file:line`, state class, boundary class, matrix status, assigned phase, expected skip reason (when applicable), and e2e target package (when implemented)
- [x] 0.3: Add `scripts/run_activation_corpus_sweep.sh` that reads the manifest, runs one trace per subprocess, applies a per-trace timeout (default 25 min), writes JSONL results, produces a Markdown summary table, and continues after admission refusal, compile failure, e2e failure, or timeout
- [x] 0.4: Define trace result statuses: `pass`, `admission-skip`, `build-skip`, `e2e-fail`, `timeout-skip`, `manifest-skip`, `infra-fail`
- [x] 0.5: Add an admission-only mode to the runner (`--admission-only`) that runs `AdmitCut`/`AdmitPlan` for each trace without starting Kind, logging refusal codes for the full matrix
- [x] 0.6: Seed default manifest skips for proxy-required rows (`caddy/M-2`, `listmonk/M-8`), shared-state rows (10 mattermost/caddy/listmonk traces), `mattermost/M-4` (infeasible), and `pocketbase/M-6` (mutable write-back)
- [x] 0.7: Add deferred cleanup in the e2e harness: each `t.Run` target block deletes its Kind namespace even on panic or timeout, not just on `t.Fatal`
- [x] 0.8: Add a `BatchResult` collector that accumulates `{target, status, stage, duration, error}` tuples and prints a summary table at the end of the test run
- [x] 0.9: Add per-target timeout enforcement (25 min default) with stage-level logging on timeout
- [x] 0.10: Run the existing 7 activation targets through the batch harness. Confirm all 7 pass and the summary table prints correctly

### Phase 1: Prove existing generator-eligible corpus rows

No codegen changes — validate that the pipeline already handles these traces end-to-end. Early wins that prove the corpus-trace wiring works before investing in new capabilities.

- [ ] 1.1: Verify whether the existing `activation-miniflux-sanitizehtml` e2e target corresponds exactly to corpus trace `miniflux/M-3` (`SanitizeHTML` at `internal/reader/sanitizer/sanitizer.go:217`). If so, mark it as corpus coverage in the manifest. If the `file:line` differs, bind a new target
- [ ] 1.2: Locate `RefreshFeed` in the miniflux corpus. Confirm it is the function at the `file:line` from corpus trace `miniflux/M-1`. Record the exact signature and return type
- [ ] 1.3: Run activation-path analysis on `miniflux/M-1` with reverse-import scoping. Confirm path is found and recommended cut is at `RefreshFeed`. Check whether `*storage.Storage` reconstruction is handled by the existing SQL-wrapper path
- [ ] 1.4: Run `codegen.RunLift` for `miniflux/M-1`. If admission accepts, proceed. If it refuses (e.g., `(T, error)` return shape or `*storage.Storage` reconstruction gap), document the refusal code and defer to after Phase 2
- [ ] 1.5: Create `test/e2e/targets/activation_miniflux_refreshfeed/target.go`. Name: `activation-miniflux-refreshfeed`. Source dirs: `["evaluation/miniflux"]`. Deploy: postgres fixture, RSS feed server fixture, host port 8080, readiness `/healthcheck`, env vars matching existing miniflux targets
- [ ] 1.6: Create `workload.go` — exercise the feed refresh path: create admin user, add an RSS feed subscription pointing at the e2e RSS feed server, trigger `PUT /v1/feeds/{feedID}/refresh`, verify entries appear
- [ ] 1.7: Create `oracle.go` — direct invocation of `RefreshFeed` with reconstructed `*storage.Storage` from `DATABASE_URL`. Compare result with extracted service
- [ ] 1.8: Register in `e2e_test.go`. Run focused Kind e2e — all stages pass. If blocked, document and continue to Phase 2

### Phase 2: Callable shape foundation

The primary codegen gate. Adds receiver method support, `(T, error)` multi-return, same-package invocation adapters, and `context.Context`/logger reconstruction. These capabilities are prerequisites for most corpus traces regardless of their state class.

#### 2A: Patch — method declaration rename

Currently methods fail at `renameFuncDecl()` in `patch.go` because the AST filter skips method declarations (`fn.Recv != nil`).

- [ ] 2A.1: Modify `renameFuncDecl()` in `pkg/codegen/patch.go` to handle method declarations. When `plan.CutPoint.Receiver` is non-empty: (a) match AST declarations where `fn.Recv != nil` AND the receiver type matches, (b) rename `fn.Name.Name` to `monoliftOriginal<FuncName>` on the matching receiver type, (c) preserve the receiver parameter name and pointer/value semantics
- [ ] 2A.2: Unit test: given a source file with `func (h *Argon2Hasher) HashWithSaltBytes(...)`, verify rename to `monoliftOriginalHashWithSaltBytes` with the same `*Argon2Hasher` receiver
- [ ] 2A.3: Unit test: given both a method and a standalone function of the same name, verify only the method matching the receiver type is renamed
- [ ] 2A.4: Unit test: verify no collision when `monoliftOriginal<Name>` already exists on the type — refuse with a diagnostic

#### 2B: Plan builder — receiver policies

Three receiver strategies, selected automatically by the planner:

- `receiver_boundary`: serialize the receiver as JSON request data (for stateless/config-only structs with all exported, serializable fields)
- `receiver_zero`: construct a zero-value receiver on the server (for stateless methods that don't read receiver fields)
- `receiver_factory`: invoke a known factory function on the server (for types like `Argon2Hasher` and `PBKDF2` with standard constructors)

- [ ] 2B.1: Add `ReceiverParam *ReceiverSpec` to the `Plan` struct in `pkg/codegen/types.go`. Fields: `GoType string`, `IsPointer bool`, `Policy ReceiverPolicy`, `FactoryFunc string` (for `receiver_factory`), `Codec Codec` (for `receiver_boundary`)
- [ ] 2B.2: Add `ReceiverPolicy` enum: `ReceiverBoundary`, `ReceiverZero`, `ReceiverFactory`
- [ ] 2B.3: In `BuildPlan()`, when `CutPoint.Receiver` is non-empty: (a) check for a registered factory function in a receiver factory registry, (b) if found, use `receiver_factory`, (c) else if state class is `Stateless` and the type is JSON-serializable, use `receiver_boundary`, (d) else if state class is `Stateless` and fields are all zero-safe, use `receiver_zero`, (e) else refuse with `receiver_requires_reconstruction`
- [ ] 2B.4: Add receiver factory registry entries for `gitea/M-16` (e.g., `NewArgon2Hasher`) and `mattermost/M-14` (e.g., `DefaultPBKDF2` or zero-value construction with env-driven params)
- [ ] 2B.5: Unit test: `BuildPlan` on a stateless value-receiver cut with serializable fields produces `ReceiverBoundary` policy
- [ ] 2B.6: Unit test: `BuildPlan` on a pointer-receiver cut with a registered factory produces `ReceiverFactory` policy with the factory function name
- [ ] 2B.7: Unit test: `BuildPlan` on a receiver with `*sql.DB` field refuses with `receiver_requires_reconstruction`

#### 2C: Same-package invocation adapter

Generate a same-package exported adapter function that the extracted server calls, instead of calling the cut symbol directly. This handles unexported functions and methods (like `caddy/M-1 funcMarkdown`).

- [ ] 2C.1: In `pkg/codegen/render.go`, generate a `MonoliftInvoke<FuncName>` adapter function in the same package as the cut point. For methods: `func MonoliftInvoke<FuncName>(recv <ReceiverType>, args...) results... { return recv.<Method>(args...) }`. For functions: `func MonoliftInvoke<FuncName>(args...) results... { return <func>(args...) }`
- [ ] 2C.2: The adapter function must be exported so the extracted server (in a separate package) can call it
- [ ] 2C.3: The adapter is written to the patched package alongside the renamed original function
- [ ] 2C.4: Unit test: adapter for unexported method `funcMarkdown` produces an exported `MonoliftInvokeFuncMarkdown`
- [ ] 2C.5: Unit test: adapter for exported function `SanitizeHTML` produces `MonoliftInvokeSanitizeHTML` — verify existing targets can use the adapter path without regression

#### 2D: Multi-return and error result support

49/71 corpus traces return `(T, error)`. Without this, most receiver targets will be refused by admission.

- [ ] 2D.1: Extend `Plan.Results` to support zero, one, or multiple return values, each with a Go type and codec
- [ ] 2D.2: Add an `error` result codec that distinguishes transport failure (RPC unreachable) from application `error` return values. Application errors are serialized as `{"error": "message"}` in the response body with HTTP 200. Transport errors trigger fail-open/fail-closed behavior
- [ ] 2D.3: Update the server template: invoke the adapter, capture all return values, serialize them as a JSON object with named fields matching the function signature positions
- [ ] 2D.4: Update the client template: deserialize multi-return JSON, reconstruct the Go return values. On transport failure: fail-open calls the renamed original, fail-closed returns zero values for all results
- [ ] 2D.5: Update fail-open behavior for `(T, error)` returns: on transport failure, call the original function and return its results directly
- [ ] 2D.6: Update fail-closed behavior for `(T, error)` returns: on transport failure, return `("", fmt.Errorf("monolift: extracted service unavailable"))` or equivalent
- [ ] 2D.7: Golden-file test: server template with `(string, error)` return
- [ ] 2D.8: Golden-file test: client stub with `(string, error)` return including fail-open and fail-closed paths
- [ ] 2D.9: Golden-file test: server template with `bool` single return (no error)
- [ ] 2D.10: Golden-file test: void function (no return values)
- [ ] 2D.11: Unit test: round-trip serialization of `(string, error)` where error is nil
- [ ] 2D.12: Unit test: round-trip serialization of `(string, error)` where error is non-nil

#### 2E: Server template — method invocation

- [ ] 2E.1: When `ReceiverParam` is present with `ReceiverBoundary` policy: add a receiver field to the invoke request struct with the appropriate JSON tag and Go type. Deserialize it and call `req.Receiver.Method(args...)`
- [ ] 2E.2: For `ReceiverFactory` policy: call the factory function on the server to construct the receiver, then invoke the method
- [ ] 2E.3: For `ReceiverZero` policy: construct a zero-value receiver and invoke the method
- [ ] 2E.4: For pointer receivers: take the address — `(&req.Receiver).Method(args...)` or construct `*T` via factory
- [ ] 2E.5: Golden-file test: server template with value-receiver `ReceiverBoundary`
- [ ] 2E.6: Golden-file test: server template with pointer-receiver `ReceiverFactory`

#### 2F: Client template — method stub

- [ ] 2F.1: When `ReceiverParam` is present: generate a method stub on the receiver type. The stub serializes `self` + args into the invoke request, POSTs to the extracted service, and on failure calls `self.monoliftOriginal<Method>(args...)`
- [ ] 2F.2: Preserve the original receiver parameter name and pointer/value semantics
- [ ] 2F.3: Golden-file test: client stub for value-receiver `ReceiverBoundary` with receiver serialization
- [ ] 2F.4: Golden-file test: client stub for pointer-receiver `ReceiverFactory` (no receiver serialization — factory on server)

#### 2G: Admission — method and multi-return gates

- [ ] 2G.1: Accept plans with `ReceiverParam` when receiver type is JSON-serializable (no channels, `io.Reader`/`Writer`, sync primitives, function types). Refuse with `non_serializable_receiver`
- [ ] 2G.2: Accept plans with multi-return `(T, error)` results. Refuse void-with-side-effects functions with `void_side_effect` if the function has no observable return
- [ ] 2G.3: When `CutPoint.Receiver` is non-empty but no receiver policy applies (e.g., `ClientReconstructible` state): refuse with `receiver_requires_reconstruction`
- [ ] 2G.4: Unit test: admit a plan with serializable value-receiver
- [ ] 2G.5: Unit test: refuse a plan where receiver contains `*sql.DB`
- [ ] 2G.6: Unit test: admit a plan with `(string, error)` result
- [ ] 2G.7: Unit test: refuse a plan with `io.Writer` result

#### 2H: Context and logger reconstruction

- [ ] 2H.1: Add `context.Context` parameter handling: when a boundary parameter is `context.Context`, reconstruct as `context.Background()` on the server. Do not serialize context across the boundary
- [ ] 2H.2: Add a no-op logger reconstruction for `mlog.LoggerIFace` and similar logger interfaces: construct a discard logger on the server
- [ ] 2H.3: Unit test: plan with `context.Context` param produces server-side `context.Background()`
- [ ] 2H.4: Unit test: plan with `mlog.LoggerIFace` param produces discard logger

#### 2I: Integration verification

- [ ] 2I.1: Run `go test ./pkg/codegen/...` — all tests pass including new golden files
- [ ] 2I.2: Run `go test ./pkg/activation/...` — no regressions
- [ ] 2I.3: Run all 7 existing e2e targets — confirm no regressions from codegen changes
- [ ] 2I.4: Update `GeneratorVersion` constant from `"SPRINT-0046"` to `"SPRINT-0048"`
- [ ] 2I.5: Run the admission-only sweep (`scripts/run_activation_corpus_sweep.sh --admission-only`) and record which traces are now admitted vs. still refused. Update the manifest with current admission statuses

### Phase 3: Shape-unlocked corpus targets

Scaffold and test corpus traces unlocked by Phase 2 capabilities. These are the Pure Leaf and near-leaf archetype: stateless or config-only, no reconstructor families needed. Each target is independent — skip-on-failure.

Sequencing preference: safest single-return targets first, then `(T, error)` targets, then targets needing context/logger.

#### 3A: pocketbase/M-3 `PasswordFieldValue.Validate` (safest — value receiver, single `bool` return)

- [ ] 3A.1: Locate `PasswordFieldValue.Validate` in pocketbase corpus. Confirm: value-receiver, trivial boundary, stateless, return type. Record exact `file:line`
- [ ] 3A.2: Run activation analysis. Confirm path and cut
- [ ] 3A.3: Run `codegen.RunLift`. Confirm admission accepts with `ReceiverBoundary` policy. If refused, document and skip
- [ ] 3A.4: Create `test/e2e/targets/activation_pocketbase_passwordvalidate/target.go`. Deploy: PocketBase baseline (host port 8090, `/api/health`, embedded SQLite)
- [ ] 3A.5: Create `workload.go` — exercise password validation via collection/record operations
- [ ] 3A.6: Create `oracle.go` — instantiate `PasswordFieldValue{...}`, call `.Validate()`
- [ ] 3A.7: Register in `e2e_test.go`. Run focused Kind e2e

#### 3B: gitea/M-16 `(*Argon2Hasher).HashWithSaltBytes` (pointer receiver, factory construction)

- [ ] 3B.1: Locate `(*Argon2Hasher).HashWithSaltBytes` in gitea corpus. Confirm: pointer-receiver, serializable boundary, stateless. Record exact `file:line` and return signature (check for `[]byte` salt param — JSON handles it via base64)
- [ ] 3B.2: Run activation analysis. Confirm path and cut
- [ ] 3B.3: Run `codegen.RunLift`. Confirm admission accepts with `ReceiverFactory` policy. If refused, document and skip
- [ ] 3B.4: Create `test/e2e/targets/activation_gitea_argon2hash/target.go`. Deploy: reuse gitea baseline (runtime image, port 3000, readiness `/api/v1/version`)
- [ ] 3B.5: Create `workload.go` — exercise password hashing: create a gitea user (triggers Argon2), then log in
- [ ] 3B.6: Create `oracle.go` — instantiate `&Argon2Hasher{...}`, call `.HashWithSaltBytes(password, salt)`. Use deterministic salt for oracle comparison
- [ ] 3B.7: Register in `e2e_test.go`. Run focused Kind e2e

#### 3C: mattermost/M-14 `(PBKDF2).Hash` (value receiver, `(string, error)` return)

- [ ] 3C.1: Locate `(PBKDF2).Hash` in mattermost corpus. Confirm: value-receiver, trivial boundary, stateless, `(string, error)` return. Record exact `file:line`
- [ ] 3C.2: Run activation analysis. Confirm path and cut
- [ ] 3C.3: Run `codegen.RunLift`. Confirm admission accepts with `ReceiverBoundary` or `ReceiverFactory` policy and `(string, error)` multi-return. If refused, document and skip
- [ ] 3C.4: Create `test/e2e/targets/activation_mattermost_pbkdf2hash/target.go`. Deploy: mattermost baseline (postgres, host port 8065, workspace support)
- [ ] 3C.5: Create `workload.go` — exercise password hashing: create user account, log in. Use deterministic error-path probe for oracle comparison (random salt makes normal hash output non-deterministic)
- [ ] 3C.6: Create `oracle.go` — instantiate `PBKDF2{...}`, call `.Hash(password)`, compare deterministically
- [ ] 3C.7: Register in `e2e_test.go`. Run focused Kind e2e

#### 3D: caddy/M-1 `(TemplateContext).funcMarkdown` (value receiver, `(string, error)`, `any` param)

- [ ] 3D.1: Locate `(TemplateContext).funcMarkdown` in caddy corpus. Confirm: value-receiver, trivial boundary, stateless. Record exact `file:line`, full signature including `any` param. The edge type is `reflective-call-via-string-keyed-map` — verify activation path traverses this
- [ ] 3D.2: Run activation analysis and `codegen.RunLift`. The `any` param and `(string, error)` return may both require handling. If either causes admission refusal, document the specific code and skip
- [ ] 3D.3: Create `test/e2e/targets/activation_caddy_markdown/target.go`. Deploy: Caddyfile with `templates` directive, host port 8080, reuse caddy baseline
- [ ] 3D.4: Create `workload.go` — request a page triggering Caddy's template markdown rendering
- [ ] 3D.5: Create `oracle.go` — instantiate `TemplateContext{}`, call `.funcMarkdown(input)`
- [ ] 3D.6: Register in `e2e_test.go`. Run focused Kind e2e

#### 3E: gitea/M-17 `RenderFullFile` (package-level, config-only, serializable)

- [ ] 3E.1: Locate `RenderFullFile` in gitea corpus. Confirm: package-level function, serializable boundary, config-only state. Record exact `file:line` and full signature — check for named alias/slice return types that may need special codec handling
- [ ] 3E.2: Run activation analysis and `codegen.RunLift`. If config params are not all serializable, document and skip
- [ ] 3E.3: Create target, workload (exercise code rendering via API), oracle. Run focused Kind e2e

#### 3F: mattermost/M-1 `Extract` (package-level, needs context + logger)

- [ ] 3F.1: Locate `Extract` in mattermost corpus. Confirm: package-level function, trivial boundary, client-reconstructible state. Record exact `file:line` and check whether params include `context.Context` and/or logger interfaces
- [ ] 3F.2: Run activation analysis and `codegen.RunLift`. Verify context/logger reconstruction from Phase 2H is applied. If other params block admission, document and skip
- [ ] 3F.3: Create target, workload (exercise extraction path), oracle. Run focused Kind e2e

### Phase 4: Streaming-to-bytes codec

Independent of Phase 2 receiver work. Adds a codec for `io.ReadSeeker`/`io.Reader` parameters, treating them as bounded-size byte payloads.

- [ ] 4.1: Add `CodecStreamingBytes Codec = "streaming_bytes"` to `pkg/codegen/types.go`
- [ ] 4.2: In `classifyCodec`: when the parameter type implements `io.ReadSeeker`, `io.Reader`, or `io.ReadCloser`, classify as `CodecStreamingBytes`. Keep `io.Writer` as rejected
- [ ] 4.3: Server template: for `CodecStreamingBytes` params, the invoke request field is `[]byte` (base64-encoded by JSON). Wrap in `bytes.NewReader()` before calling the cut function. For `io.ReadSeeker`, the reconstructed reader supports `Seek`
- [ ] 4.4: Client template: for `CodecStreamingBytes` params, read the `io.Reader` to `[]byte` via `io.ReadAll()` before serializing. Cap at 10MB with an error on exceed
- [ ] 4.5: Golden-file test: server template with `io.ReadSeeker` param
- [ ] 4.6: Golden-file test: client stub with `io.Reader` param
- [ ] 4.7: Unit test: round-trip byte serialization
- [ ] 4.8: Locate `ParseFeed` in miniflux corpus (`miniflux/M-6`). Confirm: `io.ReadSeeker` param, direct function call, stateless. Record exact `file:line`
- [ ] 4.9: Run activation analysis and `codegen.RunLift`. Confirm admission accepts with streaming-bytes codec
- [ ] 4.10: Create `test/e2e/targets/activation_miniflux_parsefeed/target.go`. Deploy: postgres fixture, RSS feed server, host port 8080, readiness `/healthcheck`
- [ ] 4.11: Create `workload.go` — exercise feed parsing: add feed subscription, trigger refresh (which calls `ParseFeed` on fetched XML)
- [ ] 4.12: Create `oracle.go` — call `ParseFeed(bytes.NewReader(xmlContent), ...)` directly
- [ ] 4.13: Register in `e2e_test.go`. Run focused Kind e2e

### Phase 5: Config-only stretch targets

Attempt these after Phase 2 receiver support is proven. Each is independent — skip-on-failure. These targets have method receivers with config-only state. Whether they pass depends on the serializability of each project's config structs.

- [ ] 5.1: Attempt `pocketbase/M-11` `resolveEmailTemplate` — direct function call, serializable boundary, config-only. Run admission, scaffold if accepted, focused e2e
- [ ] 5.2: Attempt `pocketbase/M-5` `SendRecordPasswordReset` — direct function call, trivial boundary, config-only. Run admission, scaffold if accepted, focused e2e
- [ ] 5.3: Attempt `pocketbase/M-2` `recordAuthWithOAuth2` — needs OAuth provider config, may need fake provider fixture. Run admission first; skip if config struct is not serializable
- [ ] 5.4: Attempt `caddy/M-3` `(HTTPBasicAuth).correctPassword` — value-receiver, serializable boundary, config-only. Check for private `Account.password` field and `Comparer` interface. Run admission; document if private fields block serialization
- [ ] 5.5: Attempt `caddy/M-4` `(InternalIssuer).Issue` — involves `context.Context`, `*x509.CertificateRequest`, CA material. Run admission; likely refused due to non-serializable params. Document specific blockers
- [ ] 5.6: Attempt `listmonk/M-7` `(*Campaign).CompileTemplate` — pointer-receiver, trivial boundary, config-only. Check for `template.FuncMap` callback risk. Run admission; skip if function-typed fields block
- [ ] 5.7: Attempt `gitea/M-13` `send` — package-level function variable, trivial boundary, client-reconstructible. Run admission; document if function-variable dispatch blocks
- [ ] 5.8: For each stretch target: if admission accepts, create target/workload/oracle, register, run focused Kind e2e. If admission refuses, record the refusal code in the manifest

### Phase 6: Verification and closeout

- [ ] 6.1: Run `go test ./pkg/activation/... ./pkg/codegen/... ./test/e2e/harness/...`
- [ ] 6.2: Run all 7 original activation targets — confirm no regressions
- [ ] 6.3: Run all new corpus-trace targets individually — record pass/fail per target
- [ ] 6.4: Run the best-effort overnight sweep: `scripts/run_activation_corpus_sweep.sh --phases all --timeout-per-trace 25m`
- [ ] 6.5: Run combined activation batch with 4h timeout. Treat combined-only failures as harness/resource issues if focused runs pass
- [ ] 6.6: Verify each generated manifest lists correct artifact kinds and deploy metadata
- [ ] 6.7: Verify each generated extracted Deployment is dormant and contains no `MONOLIFT_LIFT_*` env vars
- [ ] 6.8: Verify env-off mode produces zero extracted `/calls` deltas for all passing targets
- [ ] 6.9: Verify fail-open and fail-closed behavior for each result shape: void, single return, `(T, error)` multi-return
- [ ] 6.10: For each failing corpus-trace target, document: (a) stage that failed, (b) root cause category (admission, codegen, workload, infra), (c) whether fixable this sprint or deferred
- [ ] 6.11: Run admission-only sweep for every deferred row and verify each skip has a stable, actionable refusal code
- [ ] 6.12: Write `docs/research/runs/SPRINT-0048-coverage-report.md` with: trace matrix coverage before/after, per-target results, codegen capability additions, residual blockers, and next-sprint capability backlog (ranked by traces unlocked)

## Sequencing

```
Phase 0 (manifest + sweep runner + batch infra) ← GATE: must exist before overnight execution
    │
    ├──→ Phase 1 (existing eligible corpus rows) ← independent, no codegen changes
    │
    ├──→ Phase 2 (callable shape foundation) ← GATE: must land before Phase 3 and 5
    │         │
    │         ↓
    │    Phase 3 (shape-unlocked corpus targets) ← 3A-3F independent, can parallel
    │         │
    │         ↓
    │    Phase 5 (config-only stretch) ← requires Phase 2, best-effort
    │
    ├──→ Phase 4 (streaming codec + miniflux/M-6) ← independent of Phase 2
    │
    └──→ Phase 6 (verification + closeout) ← after all attempted phases
```

Phase 0 is non-negotiable: without the manifest and sweep infrastructure, overnight execution will stop on the first hard target. Phase 1 can proceed immediately since it needs no codegen changes. Phase 2 is the primary codegen gate — all receiver/method/multi-return work must land before attempting Phase 3 targets. Phase 4 (streaming codec) is independent and can run in parallel with Phase 2. Phase 5 is best-effort stretch after Phase 2+3.

Within Phase 3, all targets (3A-3F) are independent. Sequencing preference: pocketbase/M-3 first (single `bool` return, safest), then gitea/M-16, then mattermost/M-14 (first `(T, error)` test), then caddy/M-1, then gitea/M-17, then mattermost/M-1.

## Risks

**R1: `(T, error)` return shape complexity.** Adding multi-return support alongside receiver codegen is the highest-risk combination. Application error semantics must be distinguished from transport failure, and fail-open/fail-closed behavior becomes more complex. *Mitigation:* Phase 2D is self-contained with dedicated golden-file tests. Sequence single-return targets (3A, 3B) before `(T, error)` targets (3C, 3D) to validate the receiver pipeline independently of multi-return.

**R2: Receiver serialization fidelity.** Config-only receiver structs may contain unexported fields, interface fields, or constructors that enforce invariants — preventing faithful JSON round-tripping. *Mitigation:* The receiver policy taxonomy (boundary/zero/factory) handles different cases. Admission explicitly refuses non-serializable receivers. Factory construction avoids serialization entirely for types with known constructors.

**R3: Method patching may break compilation.** Renaming a method and generating a replacement on the same type could conflict with interface satisfaction, embedding, or other method sets. *Mitigation:* Phase 2A includes unit tests for method rename. Phase 2I verifies existing targets are unaffected. The patched-package-verify step catches compilation failures before deployment.

**R4: Client stub name collisions.** The `monoliftOriginal<Method>` name could collide with an existing method on the type. *Mitigation:* Check for name collisions in the AST before patching — refuse with a diagnostic if the renamed name already exists.

**R5: Same-package adapter may not handle all edge cases.** Unexported types in return values, unexported parameter types, or build constraints could prevent the adapter from compiling. *Mitigation:* Adapter generation runs through `patched-package-verify` before deployment. Failures are caught at build time, not runtime.

**R6: Oracle/workload design for receiver methods.** Finding HTTP requests that reliably exercise hash/validate methods requires understanding each application's request routing. Hash outputs with random salt are non-deterministic. *Mitigation:* Use deterministic error-path probes where normal outputs are random (mattermost/M-14). Use known-input oracle comparisons where outputs are deterministic (pocketbase/M-3).

**R7: Activation path drift since SPRINT-0039.** The 72-trace analysis was done with a potentially different configuration. Automated pipeline (reverse-import scoping + augmentation) might not reproduce the same path. *Mitigation:* Each target has an activation analysis verification step before e2e scaffolding. If path isn't found, document and skip.

**R8: Overnight execution may exhaust Kind cluster resources.** Running 15+ e2e targets sequentially with per-target namespaces can exhaust node memory or disk. *Mitigation:* Phase 0 ensures cleanup even on failure. Subprocess isolation means one panicking target can't corrupt the test process. Per-target timeouts prevent indefinite hangs.

**R9: Scope ceiling without reconstructors.** Shape-only changes (receiver, multi-return, streaming) unlock ~12 corpus traces. The remaining ~40 need reconstructor families (DB, HTTP, mailer, store). This sprint will hit a natural ceiling. *Mitigation:* This is expected and documented. The coverage report (6.12) explicitly ranks the next-sprint capability backlog by traces unlocked.

## Design decisions

**D1: Receiver policy taxonomy over one-size-fits-all.** Three receiver strategies (`boundary`, `zero`, `factory`) handle different real patterns. Some receivers should be serialized (config structs), some zero-constructed (truly stateless), some factory-built (`Argon2Hasher`). A single `ReceiverParam` that always serializes would fail on receivers with unexported fields or non-trivial construction.

**D2: `ReceiverParam` as a dedicated Plan field.** The receiver has special semantics in both templates (method call syntax) and patching (method declaration rename). A dedicated field keeps the distinction clear versus merging it into `BoundaryParams`.

**D3: Same-package adapter for all lifts.** A generated `MonoliftInvoke<FuncName>` adapter in the cut-point's package solves unexported functions/methods and provides a consistent server-side call target. The adapter is small (one forwarding call) and compiles with the patched package.

**D4: `(T, error)` in scope despite complexity.** 49/71 corpus traces need it. Deferring it means only 2-3 receiver targets are achievable (pocketbase/M-3, gitea/M-16). Including it doubles the number of attainable targets this sprint and makes receiver support immediately useful.

**D5: Streaming-to-bytes uses `[]byte` JSON encoding.** JSON base64 encoding is simple and integrates with the existing invoke protocol. The ~33% overhead from base64 is acceptable for bounded-size readers (RSS feeds, config files, not video streams). The 10MB cap prevents memory exhaustion.

**D6: Reconstructor families are out of scope.** DB pools, HTTP clients, mailers, and object stores are each mini-projects requiring per-project fixture design, env-var plumbing, and runtime dependency knowledge. Including them would make the sprint 3-4x larger. The shape work this sprint provides is prerequisite infrastructure that reconstructor sprints will build on.

**D7: Subprocess-based sweep runner over Go test harness alone.** Process isolation means one panicking/hanging target cannot corrupt the test process or prevent later targets from running. The Go test harness additions (cleanup, timeout, BatchResult) complement the subprocess runner for within-process resilience.

## Acceptance criteria

**Minimum:**
- [ ] Corpus manifest (`activation_corpus_traces.yaml`) covers all 72 matrix rows with phase, skip reason, and status metadata
- [ ] Sweep runner completes a full manifest pass without stopping on admission refusal, compile failure, e2e failure, or timeout
- [ ] Receiver/method codegen works: patching, plan building, server/client templates, admission all support method targets
- [ ] `(T, error)` multi-return codegen works: result codec, error semantics, fail-open/fail-closed
- [ ] At least 4 corpus traces pass focused Kind e2e (miniflux/M-1 or M-3, pocketbase/M-3, gitea/M-16, plus one `(T, error)` target)
- [ ] All 7 pre-existing activation targets pass (no regressions)
- [ ] `go test ./pkg/activation/... ./pkg/codegen/...` passes
- [ ] Every deferred row has a stable manifest skip reason and admission refusal code

**Target:**
- [ ] 8-10 corpus traces pass focused Kind e2e, including at least one receiver target, one `(T, error)` target, and one streaming-bytes target
- [ ] Admission-only sweep classifies all 72 rows with actionable statuses

**Stretch:**
- [ ] 12+ corpus traces including config-only stretch targets
- [ ] Combined batch of all targets (original 7 + new) runs with skip-on-failure and produces a summary table
- [ ] Coverage report ranks next-sprint reconstructor families by traces unlocked

## Next-sprint backlog (by traces unlocked)

| Capability | Traces unlocked | Representative traces |
|---|---:|---|
| `*sql.DB` / app-DB reconstructor | ~25 | gitea/M-1–M-15, listmonk/M-1–M-6, miniflux/M-2–M-5, pocketbase/M-10 |
| HTTP client reconstructor | ~8 | miniflux/M-7–M-10, M-14, pocketbase/M-9, listmonk/M-2 |
| Mailer/SMTP reconstructor | ~5 | pocketbase/M-7, listmonk/M-3, gitea/M-13, mattermost/M-13 |
| Object/file store reconstructor | ~5 | pocketbase/M-1, M-4, M-8, gitea/M-3, mattermost/M-2 |
| Shared-state coordination | ~10 | mattermost/M-3, M-6–M-9, M-12, M-15, caddy/M-5, M-7, listmonk/M-9 |
| Proxy/stream transport | ~4 | caddy/M-2, listmonk/M-8, caddy/M-5, M-7 |
