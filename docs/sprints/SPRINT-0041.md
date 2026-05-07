# SPRINT-0041: End-to-end lift pipeline with HTTP/JSON codegen

**Status:** planned
**Predecessors:** SPRINT-0040 (cut-placement analyzer), ADR-0028 (monolith as gateway)

## Intent

Build the first end-to-end lift pipeline: given a developer-annotated function, run activation-path discovery, cut placement, liftability check, and generate Go source files that split the monolith at the recommended cut point. The output is two artifacts: an HTTP/JSON server wrapping the lifted function with config-driven state reconstruction at startup, and a client stub in the monolith that replaces the local call with an HTTP POST. The overall goal is supporting the 62/71 corpus traces with trivial/serializable boundary data.

The sprint prioritizes stateless lift targets for integration testing (fully verifiable with no external dependencies), then exercises state reconstruction as a stretch target.

## Concrete targets

### Primary MVP: miniflux `SanitizeHTML` (stateless)

```go
// evaluation/miniflux/internal/reader/sanitizer/sanitizer.go
func SanitizeHTML(baseURL, input string) string
```

- **Boundary data:** `string` x2 (trivial).
- **Return:** `string` (trivial).
- **State:** Stateless — no reconstruction needed.
- **Callbacks:** Zero confirmed.
- **Transport:** HTTP/JSON POST to `/invoke`.

This is a pure function: generate the server, start it, POST `{"base_url": "...", "input": "<script>..."}`, verify sanitized HTML comes back. Fully testable end-to-end with no external dependencies.

### Secondary target: miniflux `RefreshFeed` (client-reconstructible state)

```go
// evaluation/miniflux/internal/reader/handler/handler.go:207
func RefreshFeed(store *storage.Storage, userID, feedID int64, forceRefresh bool) *locale.LocalizedErrorWrapper
```

- **Boundary data:** `int64` x2 (trivial), `bool` (trivial). `*storage.Storage` is reconstructible — excluded from the wire request, rebuilt on the server from `DATABASE_URL`.
- **Return:** `*locale.LocalizedErrorWrapper` — serializable as `{"error": "...", "message": "..."}`, nullable (nil = success).
- **State:** `*storage.Storage` wraps `*sql.DB`. Client-reconstructible: `sql.Open("postgres", os.Getenv("DATABASE_URL"))` → `storage.NewStorage(db)`.
- **Callbacks:** Zero confirmed.
- **Transport:** HTTP/JSON POST to `/invoke`.

This exercises state reconstruction codegen. Integration testing verifies the generated code compiles and the JSON round-trip works, but does not require a running database.

## Scope boundaries

**In scope:**
- New CLI command `monolift lift` orchestrating the full pipeline
- New `pkg/codegen/` package producing server and client `.go` files from report + cut data
- State-reconstruction registry keyed on Go type identity, not application-specific config APIs
- Server template: HTTP/JSON POST `/invoke` with config-driven startup
- Client stub template: drop-in replacement in the monolith package that forwards via HTTP POST
- Callsite patching: rewrite the original call expression to use the generated stub
- Generator-admission layer separating liftability from HTTP/JSON-specific gates
- Golden-file tests for generated output
- Artifact manifest (`monolift_lift_manifest.json`)

**Out of scope:**
- gRPC, streaming, or bidirectional transport
- Composite cuts or multi-node extraction
- Infeasible boundary-data cases (per ADR-0028, streaming types at cut = Infeasible)
- Shared-state reconstruction (the 8/71 traces)
- Kubernetes manifests, Dockerfiles, service discovery
- Modifying the activation-path or cut-placement algorithms
- Production error handling, retries, circuit breakers

## Relationship to existing codegen infrastructure

Two codegen paths already exist:

1. **`pkg/lift/`** — the v1 lift pipeline. Interface-oriented: expects `(context.Context, req) (resp, error)`. Not suitable for activation-path cut-point functions with arbitrary signatures.
2. **`pkg/compiler/transport/emit/`** — the v2 emit pipeline. Has `httpjson/` and `liftpatch/` templates using `emit.Context` + `FieldSpec`. Closer to what we need but tightly coupled to the pragma-annotated interface model, lacks state reconstruction, and has hardcoded package names.

This sprint creates new templates in `pkg/codegen/`, reusing `emit.FieldSpec` for parameter/result descriptions but not extending the existing templates. The new pipeline starts from a cut-point on an activation path — a different input model. A follow-up sprint can unify the two paths once the new codegen is validated on the corpus.

## Task list

### Phase 0: MVP contract and pipeline types

- [ ] Define `pkg/codegen/` package with `Plan`, `Artifact`, and `Manifest` types. The `Plan` carries: cut-point function identity, boundary params (wire-serialized), reconstructed params (server-side only), return type codec, service name, import paths, output paths.
- [ ] Record both MVP contracts as test fixtures: (a) `SanitizeHTML` — target `file:line`, request fields (`base_url`, `input`), no excluded params, string response; (b) `RefreshFeed` — target `file:line`, request fields (`user_id`, `feed_id`, `force_refresh`), excluded param (`store`), error-wrapper response envelope.
- [ ] Add `BuildPlan(report reportv2.Report, cut activation.CutResult) (*Plan, error)` that partitions parameters into boundary vs. reconstructed using cut boundary classification and `report.State`/`ExternalDeps`. Unit-test on both fixtures.

### Phase 1: CLI and pipeline orchestration

- [ ] Add `monolift lift` subcommand to the existing Cobra root with flags: `--source` (module root), `--target` (file:line), `--trace` (optional: pin activation path to a trace JSON), `--output` (directory for generated files), `--service-name`, `--write-monolith-stub` (enables callsite patching).
- [ ] Wire the CLI to run in sequence: activation-path analysis → `AnalyzeCut` → extract report → generator admission → `BuildPlan` → codegen → write artifacts.
- [ ] Add generator-admission layer: accept only `Feasible` cuts with trivial/serializable/reconstructible boundary data. Emit structured refusal diagnostics for rejected cuts (callable boundary values, streaming types, sync primitives, missing reconstructors).
- [ ] Fail generation if `cut.Recommended == nil` or if admission is refused. Print the refusal reason.

### Phase 2: Type mapping and state reconstruction

- [ ] Implement type-mapping in `pkg/codegen/typemap.go`: map Go parameter/return types to JSON field specs. Handle primitives (`int64`, `bool`, `string`), pointer-to-struct (serializable or reconstructed depending on cut classification), error-wrapping returns (`*locale.LocalizedErrorWrapper` → nullable JSON object).
- [ ] Implement reconstruction registry in `pkg/codegen/recon.go` keyed on Go type identity. First entry: `*sql.DB` / types wrapping `*sql.DB` → `sql.Open("postgres", os.Getenv("DATABASE_URL"))`. Also: `*http.Client` → `&http.Client{Timeout: ...}`, `*log.Logger` → `log.New(os.Stderr, ...)`.
- [ ] For parameters classified as `ClientReconstructible`, exclude from the wire request and generate server-side init. The client stub does not send them; the server creates them at startup from env vars.
- [ ] Unit tests for type mapping covering the miniflux `RefreshFeed` signature and table-driven cases for each type category.

### Phase 3: Server code generation

- [ ] Implement `RenderServer(plan *Plan) (map[string][]byte, error)` in `pkg/codegen/server.go` using Go templates.
- [ ] Server template produces a standalone `main.go`: config-driven state reconstruction at startup (reconstruction registry entries for each `ReconstructedParam`), HTTP handler at `POST /invoke` (JSON decode → call cut-point function → JSON encode), `GET /healthz`, configurable listen address via `MONOLIFT_HTTP_ADDR`.
- [ ] Generated server imports the cut-point function's package directly — no function body copying. The generated `go.mod` must live in the same module or use a replace directive for `internal/` packages.
- [ ] Apply `go/format.Source()` and import sorting to all generated output. Verify with `go vet`.
- [ ] Golden-file test: render server for miniflux `RefreshFeed`, compare against `pkg/codegen/testdata/miniflux_refreshfeed_server.go.golden`.

### Phase 4: Client stub generation and callsite patching

- [ ] Implement `RenderClient(plan *Plan) (map[string][]byte, error)` in `pkg/codegen/client.go`.
- [ ] Client stub template: function matching the cut-point signature, POSTs boundary params as JSON to `MONOLIFT_<SERVICE>_ENDPOINT` (default `http://127.0.0.1:8081/invoke`), decodes response. Gated by `MONOLIFT_LIFT_<SERVICE>=on` env var. Fail-open by default: on remote failure, call the original local function. Fail-closed available via `MONOLIFT_LIFT_FAILMODE=closed`.
- [ ] Client stub `package` declaration matches the original function's package. Generated file lives in the same package directory.
- [ ] Callsite patching: locate the incoming cut-edge call expression from `activation.Edge.Position`, verify it resolves to the selected callee, rewrite the AST to call the generated stub function instead. Only patch when `--write-monolith-stub` is set. Verify patched file builds.
- [ ] Golden-file test for client stub. Unit test for callsite patching on a synthetic fixture.

### Phase 5: Artifact writing and integration test

- [ ] Implement artifact writer: create parent directories, write files atomically, emit generated-file header with cut identity and generator version. Write `monolift_lift_manifest.json` with server path, stub path, patched file, cut identity, and admission verdict.
- [ ] `SanitizeHTML` integration test (primary): run the full pipeline on miniflux targeting `SanitizeHTML`. Verify: activation path found, cut recommended, admission accepted, server and client stub generated, both pass `go build` and `go vet`, manifest written. This is stateless — no external dependencies.
- [ ] `SanitizeHTML` network test: start the generated server with `httptest.Server`, call the stub function with test HTML input, verify the JSON round-trip produces sanitized output. This is the end-to-end proof that the generated code pieces integrate correctly over the network.
- [ ] `RefreshFeed` codegen test (secondary): run the pipeline targeting `RefreshFeed`. Verify generated server and client stub compile and pass `go vet`. Verify the generated server includes state-reconstruction code for `*storage.Storage`. Do not require a running database — verify codegen correctness, not runtime behavior.
- [ ] Smoke test: run `monolift lift` on both targets and verify output files are non-empty and deterministic across repeated runs.

### Phase 6: Documentation and corpus runway

- [ ] Write `docs/decisions/0029-codegen-pipeline.md`: why Go templates over AST rewriting, why HTTP/JSON for phase 1, state-reconstruction taxonomy, relationship to existing `emit` infrastructure, and the intended unification path.
- [ ] Build a corpus support matrix from `recommended-cuts.md`: for each of the 62/71 trivial/serializable traces, record whether every boundary param has a codec and every reconstructed state value has a reconstructor. Mark generator-eligible vs. needs-new-reconstructor.
- [ ] Document the next reconstructor families needed: DB pools (postgres, mysql, sqlite), HTTP clients, mailers, object stores, loggers, indexers. Each is a registry entry, not a bespoke template.

## Sequencing

```
Phase 0 (plan types + MVP contract)
    │
    v
Phase 1 (CLI + pipeline orchestration)
    │
    +──> Phase 2 (type mapping + state recon) ──+
    │                                            │
    +──> Phase 3 (server codegen) ──────────────+──> Phase 5 (integration)
    │                                            │
    +──> Phase 4 (client stub + callsite patch) ─+
                                                  │
                                                  v
                                            Phase 6 (docs + runway)
```

Phase 0 must complete first (the Plan type is the contract). Phase 1 wires the pipeline. Phases 2-4 can proceed in parallel once the Plan type and pipeline are stable. Phase 5 depends on all of 2-4. Phase 6 is last.

## Risks

1. **`internal/` import constraint.** Generated server must import `miniflux.app/v2/internal/storage` and `miniflux.app/v2/internal/locale`. The generated binary must live in the same module or use a `go.mod` replace directive. *Mitigation:* require generated code to live under the source module root for internal-package lifts.

2. **Return-type serialization.** `*locale.LocalizedErrorWrapper` embeds an `error` interface. The generated server must decide what to serialize. *Mitigation:* Phase 1 serializes error string + message; fidelity improvements follow.

3. **Liftability false blocking.** The existing `report.Root.Admission` may refuse functions that are actually liftable as HTTP/JSON. *Mitigation:* generator-admission layer separates liftability from HTTP/JSON admissibility, records both.

4. **Callsite rewrite safety.** AST rewriting that preserves surrounding comments and formatting is fragile. *Mitigation:* patch only the single selected call expression; verify the patched file builds; golden-file test the rewrite.

5. **State-reconstruction portability.** If reconstruction is keyed on miniflux-specific config APIs (`config.Opts.DatabaseURL()`), it won't transfer to other codebases. *Mitigation:* key on Go type identity and generic patterns (`DATABASE_URL` env var), not application-specific config helpers.

6. **Activation-path analysis cost.** Loading SSA for a codebase takes 10-30s. *Mitigation:* the CLI must handle timeouts and provide progress feedback.

7. **Scope creep.** Refuse proxy, streaming, composite, and missing-reconstructor cases with explicit diagnostics instead of partial generation.

## Acceptance criteria

- [ ] `monolift lift` runs end-to-end from target annotation to generated Go files
- [ ] Pipeline runs activation-path discovery, cut placement, extract report, and generator admission before codegen
- [ ] Generation refuses non-feasible cuts with structured diagnostics
- [ ] `SanitizeHTML` (stateless): generated server handles `POST /invoke`, network round-trip test passes with `httptest.Server` — no external dependencies
- [ ] `RefreshFeed` (client-reconstructible): generated server includes state-reconstruction code for `*storage.Storage`, compiles, passes `go vet`
- [ ] Generated client stub sends only boundary params over HTTP/JSON, gated by env var, fail-open by default
- [ ] Callsite is patched when `--write-monolith-stub` is set
- [ ] Generated `.go` files pass `go build`, `go vet`, and `go/format`
- [ ] Output is deterministic across repeated runs
- [ ] Golden-file tests exist for server and client stub output
- [ ] `monolift_lift_manifest.json` written with cut identity, paths, and admission verdict
- [ ] No modifications to activation-path or cut-placement algorithm code
- [ ] ADR-0029 documents the codegen architecture
