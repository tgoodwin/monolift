# SPRINT-0019 — `internal/` symbol lift via cmd-inside-host emission

**Status:** planned
**Predecessor:** SPRINT-0018 (Caddy `caddyhttp.CleanPath` end-to-end via AST patch + separate-module extracted service).
**Anchor ADRs:** ADR-0017, ADR-0018 (NOT amended this sprint), ADR-0023 (amended additively).
**Drafts:** `docs/sprints/drafts/SPRINT-0019-{CODEX,GEMINI,CLAUDE}.md` plus `-critique.md`.

## Intent

SPRINT-0018 emits the extracted service as its own Go module (`module monolift/extracted-<symbol>`) with `replace github.com/.../host/v2 => ../upstream`. This works for exported symbols but fails for any symbol under `<host>/internal/...` because Go enforces the `internal/` rule on the *import path string*, not on resolution — `replace` cannot bypass it. SPRINT-0019 closes that gap by switching the extracted service to a `cmd/monolift-extracted-<symbol>/main.go` package emitted *inside* the patched host module. Two `docker build` targets read from a single staging tree (the `upstream/` clean copy disappears). The lift patch in the extracted-service binary is dormant because its deployment omits all `MONOLIFT_LIFT_*` env vars. End-to-end demo: lift `github.com/caddyserver/caddy/v2/internal/metrics.SanitizeMethod(string) string` (real internal symbol, in closure, per-request call site) and verify it through the existing kind harness with the same four-layer verification stack from SPRINT-0018. SPRINT-0018's `caddyhttp.CleanPath` demo continues to pass under the new emitter as a regression target.

## Mechanism (settled)

**Choice: A — cmd-inside-host.** All three drafts converged on A. The merged plan is A and nothing else.

Why not the others:
- **B (admission-rule reject):** trades reach for clarity. A removes the constraint at the emission layer entirely; B preserves the SPRINT-0018 blind spot for no compensating benefit. Gemini's draft proposed a Layer-1 property `import.legality.external-import-allowed` plus a detector — both critiques rejected this as scope creep over a problem mechanism A makes vanish. ADR-0018 is **not amended** this sprint.
- **C (re-export shim):** doubles the AST mutation surface (one for the symbol body, one for the re-export package) and changes the host's exported-package shape in the patched copy. Bisect/review cost without compensating benefit.
- **D (hybrid):** A handles both internal and exported symbols uniformly. No motivating case for a hybrid.

Mechanism A's load-bearing property: the lift patch in the extracted-service binary is dormant via env-gate. The extracted service's package path is now under `github.com/caddyserver/caddy/v2/...`, so `internal/` imports are legal.

ADR impact: ADR-0023 gets a small additive amendment titled "Internal-rule compliance via cmd-inside-host emission" recording the shift away from separate-module emission. ADR-0018 untouched.

## Demo target (settled)

**Symbol:** `github.com/caddyserver/caddy/v2/internal/metrics.SanitizeMethod`.

**File:** `evaluation/caddy/internal/metrics/metrics.go:33`.

**Signature (verbatim):**
```go
func SanitizeMethod(m string) string
```

**Verified in closure.** `closure.includedSymbols` of `test/e2e/targets/caddy/golden/report.json` contains `{module_path: github.com/caddyserver/caddy/v2, package_path: github.com/caddyserver/caddy/v2/internal/metrics, object_name: SanitizeMethod, kind: function, file: internal/metrics/metrics.go, line_start: 33}`. Block A pins this with a closure regression test alongside the existing CleanPath pin.

**Verified call sites.**
- `evaluation/caddy/modules/caddyhttp/metrics.go:237`: `method := metrics.SanitizeMethod(r.Method)` inside `metricsInstrumentedRoute.ServeHTTP` — **per-request firing path**, the load-bearing call site for the e2e workload.
- `evaluation/caddy/metrics.go:59`: `"method": metrics.SanitizeMethod(r.Method)` inside the admin metrics endpoint — secondary, not on the workload path.

**Required Caddyfile change (Block B.4).** `metricsInstrumentedRoute.ServeHTTP` is wrapped around routes only when the server's `Metrics` field is set. The baseline Caddyfile (`test/e2e/targets/caddy/baseline/caddyfile-configmap.yaml`) does not enable metrics today; this sprint adds a `servers { metrics }` directive in the global options block. Workload paths (`/static/hello.txt`, `/headers`, `/proxy?x=1`) then route through `metricsInstrumentedRoute`, calling `SanitizeMethod` exactly once per request.

**Why this symbol.** Per-request firing means SPRINT-0018's verification stack (per-request `/calls` delta `>= 1`, aggregate upper bound, oracle equality on every `/invocations` record, transcript parity, fail-closed 404, fail-open 200) translates 1-for-1. Pure `string → string` signature is JSON-trivial. In-closure: no closure regeneration prerequisite. Capitalized export from an internal/ package: exercises mechanism A's load-bearing property.

**Alternatives reconsidered, rejected:**
- `internal.SplitUnixSocketPermissionsBits(string) (string, fs.FileMode, error)` (Codex draft): in closure, multi-return signature exercises emitter generality, but called only at admin-load / listener-provisioning time. Requires a new admin-API workload step. Strictly more harness work for strictly weaker per-request liveness verification.
- `internal.PrivateRangesCIDR() []string` (Claude draft): NOT in current closure (verified via `grep`); would force a closure-regeneration step mid-sprint. Also fires at matcher-provisioning, not per-request. Both critiques flagged.

**Miniflux: explicitly out of scope.** `test/e2e/targets/miniflux/target.go:7-9` is `SkipReason: deferred`; bringing it up means standing up Postgres + RSS feed server + Dockerfile + workload + golden — a cross-target e2e sprint, not a thin emitter extension. SPRINT-0020 candidate.

## CleanPath regression preservation (mandatory)

The lift target `test/e2e/targets/caddy/target.go` keeps **both** symbols:
1. `caddyhttp.CleanPath` — SPRINT-0018's exported-symbol demo, regenerated under the new cmd-inside-host layout to prove A is uniformly correct (not just for internal symbols).
2. `internal/metrics.SanitizeMethod` — the new internal-symbol demo.

Two extracted-service Pods deployed in parallel: `monolift-extracted-cleanpath` and `monolift-extracted-sanitizemethod`. Each has its own Service, its own image (built from the same `host-patch/` tree, different `cmd/` target), its own deployment, its own oracle. Both env-var-gated independently (`MONOLIFT_LIFT_CLEANPATH` and `MONOLIFT_LIFT_SANITIZEMETHOD`). Both are verified by the four-layer stack.

This adds harness surface — two pods, two oracles, two counters, two fail-mode toggles — but it's the cost of preserving SPRINT-0018 while extending. The critiques flagged Gemini's draft for replacing CleanPath with SanitizeMethod (catastrophic regression); the merged plan keeps both.

## Goals

1. `pkg/compiler/transport/emit/httpjson/` emits `cmd/monolift-extracted-<symbol>/main.go` inside the patched host module. No `gomod.tmpl`, no separate `go.mod`, no `replace` directive.
2. Stubcompiler materialization produces a single `<output>/lifted/host-patch/` tree containing both the patched host (with sibling dialer files) AND `cmd/monolift-extracted-<symbol>/` subtrees. `<output>/lifted/upstream/` is removed.
3. Two Dockerfiles share `<output>/lifted/host-patch/` as build context; one builds Caddy, two build extracted-service binaries (one per lifted symbol).
4. Two extracted-service Pods deploy in parallel, both reach Ready before workload, both gated by independent env vars.
5. Lifted Caddy gets a one-line Caddyfile addition (`servers { metrics }`) so `SanitizeMethod` fires per-request.
6. e2e demo passes the four-layer stack for *both* symbols: per-request `/calls` delta, oracle equality on every `/invocations` record, transcript parity, fail-closed 404, fail-open 200.
7. Recursion safety: extracted-service Deployment YAMLs grep-clean for `MONOLIFT_LIFT_*` (static check) AND a `/invoke` test against an extracted Pod with no env set returns the oracle result with counter incrementing exactly once per call (runtime check, from Codex draft).
8. ADR-0023 amended additively. ADR-0018 untouched.
9. `evaluation/caddy/` byte-identical pre/post stubcompiler. `make verify-evaluation-untouched` passes.
10. `cmd/main.go` unchanged.

## Non-goals

- Miniflux target bring-up (SPRINT-0020).
- Multi-symbol lift in one extracted-service binary.
- Receiver-bearing symbol slice (Cliff 2 — still deferred).
- Switching the host-build mechanism to `go build -overlay` (SPRINT-0018's recorded alternative; not this sprint).
- Changes to `pkg/compiler/transport/emit/liftpatch/` (patcher API frozen).
- Changes to the v0 admission rule in `pkg/compiler/transport/admission.go` and the seven liftability properties.
- Any new Layer-1 property. ADR-0018 not amended.
- Caddyfile structural redesign — only the one-line `servers { metrics }` addition.

## Sequencing — three blocks

### Block A — Re-emit `httpjson` as cmd-inside-host

- [x] **A.1** `pkg/compiler/transport/emit/httpjson/httpjson.go` `Render(ctx Context) (Artifact, error)`: emit files at `cmd/monolift-extracted-<symbol>/main.go` (relative to host module root) instead of `extracted-<symbol>/{main.go,go.mod}`. `Artifact.Files` keys are relative paths into the host module's staging tree.
- [x] **A.2** Delete `pkg/compiler/transport/emit/httpjson/templates/gomod.tmpl` and the corresponding test assertions on `go.mod` contents.
- [x] **A.3** Rewrite `pkg/compiler/transport/emit/httpjson/templates/dockerfile.tmpl`: build context is the host-patch root (passed as a `--build-context` or `COPY . /src`); build command is `go build -mod=mod -o /out/extracted ./cmd/monolift-extracted-<symbol>`. Distroless final stage unchanged. Listens `:8081`.
- [x] **A.4** Generalize the existing CleanPath-only `main.go.tmpl` to render both `CleanPath(string, bool) string` and `SanitizeMethod(string) string` from the same template. Parametrize `(SymbolImportPath, ObjectName, ParamFields, ResultFields, EnvVarSuffix, ServiceName)`. No general IDL or multi-symbol framework — just enough parametrization for these two.
- [x] **A.5** Anti-stub render tests in `pkg/compiler/transport/emit/httpjson/httpjson_test.go`:
  - `TestRenderImportsRealSymbol_CleanPath`: parse rendered `main.go`, assert `*ast.ImportSpec` for `github.com/caddyserver/caddy/v2/modules/caddyhttp` AND `*ast.SelectorExpr{X: caddyhttp, Sel: CleanPath}` invoked as a `*ast.CallExpr` inside `/invoke`.
  - `TestRenderImportsRealSymbol_SanitizeMethod`: parse rendered `main.go`, assert `*ast.ImportSpec` for `github.com/caddyserver/caddy/v2/internal/metrics` AND `*ast.SelectorExpr{X: metrics, Sel: SanitizeMethod}` invoked as a `*ast.CallExpr` inside `/invoke`.
  - `TestRenderRejectsSyntheticBody`: fixture template with a hand-written body fails the guard. Carried over.
  - `TestCounterIncrementsBeforeRealCall`: assert `atomic.AddInt64` precedes the selector call. Carried over.
  - `TestRenderProducesGofmtClean`, `TestRenderDeterministic`, `TestRenderUnknownTemplate`: carried over.
- [x] **A.6** Update `pkg/compiler/transport/emit/httpjson/testdata/cleanpath/` goldens: expect `cmd/monolift-extracted-cleanpath/main.go` (no `extracted-cleanpath/go.mod`, no separate-module Dockerfile). Add `pkg/compiler/transport/emit/httpjson/testdata/sanitizemethod/` goldens with the same shape.
- [x] **A.7** `TestRenderGoBuild`: stage `evaluation/caddy/` to a temp dir as the host-patch root, drop `cmd/monolift-extracted-cleanpath/main.go` AND `cmd/monolift-extracted-sanitizemethod/main.go` into it, run `go build -mod=mod ./cmd/...` via `os/exec`. Both binaries exit 0.

**Block A gate:** `go test ./pkg/compiler/transport/emit/httpjson/...` green including all anti-stub tests, two-symbol golden test, and `TestRenderGoBuild`.

### Block B — Stubcompiler integration + Caddyfile metrics-enable + multi-pod harness

- [x] **B.1** Update `test/e2e/stubcompiler/main.go` extracted-service materialization. For caddy: emit *two* sets of `cmd/monolift-extracted-<symbol>/` artifacts (one for `cleanpath`, one for `sanitizemethod`) into `<output>/lifted/host-patch/`. Drop the `<output>/lifted/upstream/` copy entirely. Drop the `extracted-<symbol>/` parent directory. The `MANIFEST.json` reflects the new layout.
- [x] **B.2** Apply each symbol's `HostPatchOp` via `liftpatch.PatchSymbolBody` and drop both sibling files (`monolift_lift_cleanpath.go` in `modules/caddyhttp/`, `monolift_lift_sanitizemethod.go` in `internal/metrics/`) into `host-patch/`.
- [ ] **B.3** Stubcompiler integration test asserts: (i) `host-patch/cmd/monolift-extracted-cleanpath/main.go` and `host-patch/cmd/monolift-extracted-sanitizemethod/main.go` both exist and AST-contain the real selector calls; (ii) both `caddyhttp.go` and `internal/metrics/metrics.go` differ from the original at exactly the function body (prepended `*ast.IfStmt` with sentinel cond) — and only there; (iii) `go build ./cmd/...` from `<output>/lifted/host-patch/` succeeds for both binaries; (iv) `make verify-evaluation-untouched` passes; (v) `<output>/lifted/upstream/` does not exist.
- [ ] **B.4** Update `test/e2e/targets/caddy/baseline/caddyfile-configmap.yaml`: add `servers { metrics }` in the global options block so `metricsInstrumentedRoute` wraps every route. Workload paths (`/static/hello.txt`, `/headers`, `/proxy?x=1`) now route through `metricsInstrumentedRoute.ServeHTTP`, calling `SanitizeMethod(r.Method)` once per request. The `handle { error 404 }` catch-all from SPRINT-0018 stays for sentinel cascade.
- [ ] **B.5** Update `test/e2e/targets/caddy/target.go`: `LiftedExtractedServices` becomes a slice of two — `monolift-extracted-cleanpath` and `monolift-extracted-sanitizemethod`. Each entry has its own Dockerfile path (the same file rendered with different cmd target), image tag, deployment YAML, service YAML, readiness path. Lifted Caddy deployment env: `MONOLIFT_LIFT_CLEANPATH=on`, `MONOLIFT_LIFT_SANITIZEMETHOD=on`, `MONOLIFT_LIFT_FAILMODE=closed` (default for both), `MONOLIFT_LIFT_CLEANPATH_ENDPOINT`, `MONOLIFT_LIFT_SANITIZEMETHOD_ENDPOINT` set to the respective Service URLs.
- [ ] **B.6** Add the SanitizeMethod oracle: `test/e2e/targets/caddy/oracle.go` extends `SymbolInvoker` to dispatch on a symbol identifier — `Invoke({"symbol": "cleanpath", ...})` returns `caddyhttp.CleanPath(p, collapseSlashes)`; `Invoke({"symbol": "sanitizemethod", ...})` returns `metrics.SanitizeMethod(m)`. The harness already has a `replace` for `evaluation/caddy/` so the import works.
- [ ] **B.7** Image build: extend `test/e2e/harness/imagebuild.go` to build *three* images per kind run (`monolift-e2e/caddy-lifted:e2e`, `monolift-e2e/extracted-cleanpath:e2e`, `monolift-e2e/extracted-sanitizemethod:e2e`) all from the same shared context (`<output>/lifted/host-patch/`), each with a different Dockerfile target. Load all three.
- [ ] **B.8** Deployer: extend the readiness-wait loop to await *all* `LiftedExtractedServices[*]` plus the lifted Caddy deployment before workload runs.

**Block B gate:** Stubcompiler integration test passes; `make verify-evaluation-untouched` passes; `evaluation/caddy/` checksum unchanged; both extracted-service `go build`s exit 0.

### Block C — e2e + recursion-safety + ADR + closeout

- [ ] **C.1** Recursion-safety static test (Claude draft, line 161): an integration test parses each `LiftedExtractedServices[*].DeploymentYAML` post-stubcompiler-run and asserts zero matches for the regex `MONOLIFT_LIFT_[A-Z_]+:` in the env block.
- [ ] **C.2** Recursion-safety runtime test (Codex draft, line 126): post-deploy, port-forward each extracted-service Pod individually, send one `POST /invoke` with no `MONOLIFT_LIFT_*` env on the extracted-service container, verify the Pod's `/calls` counter increments by exactly 1 (not 2 or more — proves no recursion through the dormant patched body) and the response equals the oracle result. Run for both `cleanpath` and `sanitizemethod` extracted Pods.
- [ ] **C.3** Per-request counter delta + aggregate: harness reads each extracted Pod's `/calls` before each workload request, asserts `>= 1` delta for SanitizeMethod (per-request firing) and `>= 1` delta for CleanPath (per-request firing — `matchers.go:481,490`). Aggregate `<= 50` total per pod (catches recursion / accidental client loops).
- [ ] **C.4** Per-invocation oracle equality: for each record returned by `GET /invocations` on each extracted Pod, harness independently invokes `caddy.Oracle.Invoke({symbol: ..., args})` with the same args and asserts result equality. **Load-bearing falsifiability check** for both symbols.
- [ ] **C.5** Transcript parity: capture baseline transcript (env-off deployment, same image), capture lifted transcript (both env vars on), assert response equality on `/static/hello.txt`, `/headers`, `/proxy?x=1` (modulo `Date`/`Server`).
- [ ] **C.6** Negative test: re-deploy lifted Caddy with both `MONOLIFT_LIFT_CLEANPATH` and `MONOLIFT_LIFT_SANITIZEMETHOD` *unset*; assert both pods' `/calls` counters stay at 0 across the workload; transcripts identical to env-on case.
- [ ] **C.7** Fail-mode tests for both symbols:
  - **Fail-closed (default).** Scale `monolift-extracted-cleanpath` to 0; fire workload; assert workload requests return 404 (sentinel cascades through path-pattern matchers to the `handle { error 404 }` catch-all from SPRINT-0018). Scale back to 1, run again, assert 200s and counter increments resumed. Repeat for `monolift-extracted-sanitizemethod` (sentinel here cascades through `metricsInstrumentedRoute` — verify the same 404 mechanism applies, or document a different expected status if metrics-route handling differs).
  - **Fail-open.** Re-deploy lifted Caddy with `MONOLIFT_LIFT_FAILMODE=open`, scale each extracted to 0 in turn, fire workload, assert 200s (degraded but available, original body executed). Counters stay at 0.
- [ ] **C.8** SPRINT-0018 actor-adapter assertions from Caddy report (`archetype_kind`, primary `serialized-actor`, alternative `keyed-partitioned-state` `[TOPOLOGY]`, adapter `Kind: actor`) unchanged.
- [ ] **C.9** ADR-0023 additive amendment: section "Internal-rule compliance via cmd-inside-host emission" recording the shift from separate-module (SPRINT-0018) to cmd-inside-host (SPRINT-0019), the env-var dormancy mechanism in extracted binaries, the recursion-safety dual gate (static YAML grep + runtime single-increment test), and the closure of the `internal/`-import trap previously flagged as a future admission-rule extension.
- [ ] **C.10** `docs/evolution.md`: append a paragraph summarising the SPRINT-0019 landing.
- [ ] **C.11** Update `docs/sprints/ledger.yaml` to `done` once Block C is green (handled by the orchestrator).
- [ ] **C.12** Verify `cmd/main.go` unchanged.

**Block C gate (sprint acceptance):** `MONOLIFT_E2E=1 go test -tags=e2e ./test/e2e -run TestE2E/caddy -count=1` green: both symbols' verification stacks pass, recursion safety verified by static + runtime checks, transcript parity, fail-mode tests, SPRINT-0017 actor-adapter assertions unchanged.

## Acceptance criteria

All must hold at sprint close:

- [ ] `pkg/compiler/transport/emit/httpjson/` no longer emits `gomod.tmpl`. `Artifact.Files` keys point at `cmd/monolift-extracted-<symbol>/main.go` paths inside the host module root.
- [ ] `pkg/compiler/transport/emit/httpjson/testdata/cleanpath/` and `pkg/compiler/transport/emit/httpjson/testdata/sanitizemethod/` goldens both exist and reflect the cmd-inside-host layout.
- [ ] Anti-stub render tests pass for both symbols (`TestRenderImportsRealSymbol_CleanPath`, `TestRenderImportsRealSymbol_SanitizeMethod`).
- [ ] `TestRenderGoBuild` exits 0 when given a fresh staging of `evaluation/caddy/` plus both rendered `cmd/...` directories.
- [ ] Stubcompiler against caddy emits `<output>/lifted/host-patch/` containing both `cmd/monolift-extracted-cleanpath/main.go` and `cmd/monolift-extracted-sanitizemethod/main.go`. `<output>/lifted/upstream/` does not exist. `MANIFEST.json` lists both cmd directories and no `go.mod`s for extracted services.
- [ ] `evaluation/caddy/` byte-identical pre/post stubcompiler. `make verify-evaluation-untouched` passes.
- [ ] Both extracted-service Deployment YAMLs grep-clean for `MONOLIFT_LIFT_[A-Z_]+:` in the env block (recursion-safety static check).
- [ ] Per-extracted-Pod runtime test: `/invoke` with no env returns oracle result and increments counter by exactly 1 (recursion-safety runtime check).
- [ ] Lifted Caddyfile contains `servers { metrics }` in global options + the SPRINT-0018 `handle { error 404 }` catch-all.
- [ ] `MONOLIFT_E2E=1 go test -tags=e2e ./test/e2e -run TestE2E/caddy -count=1` green: per-request `/calls` delta `>= 1` for both pods, aggregate `<= 50` per pod, oracle equality on every `/invocations` record (both pods), transcript parity, env-off zero counters, fail-closed 404 (both symbols), fail-open 200 (both symbols). SPRINT-0018 CleanPath verification stack passes 1-for-1.
- [ ] SPRINT-0017 actor-adapter assertions unchanged.
- [ ] ADR-0023 contains "Internal-rule compliance via cmd-inside-host emission" amendment. ADR-0018 unchanged. `docs/evolution.md` records the slice.
- [ ] `cmd/main.go` unchanged.
- [ ] Patcher API (`pkg/compiler/transport/emit/liftpatch/`) unchanged from SPRINT-0018.
- [ ] No new Layer-1 liftability property exists.
- [ ] No `import.legality.*` property exists.

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| Recursion: extracted-service binary contains the patched body and re-dials itself. | Extracted Deployment env explicitly omits `MONOLIFT_LIFT_*` (env-gate dormant). Static YAML grep test (C.1) + runtime single-increment test (C.2). Belt and suspenders. |
| `servers { metrics }` Caddyfile change alters baseline behavior in some unexpected way. | Workload paths (`/static`, `/headers`, `/proxy`) still match their handlers; `metricsInstrumentedRoute` wraps but does not change response semantics. Transcript parity assertion (C.5) catches any drift. |
| `cmd/monolift-extracted-<symbol>/` collides with an existing Caddy `cmd/` directory. | Pre-write collision check: `liftpatch` integration emits to `cmd/monolift-extracted-<symbol>/`; if directory exists pre-stubcompiler, error. Caddy has `cmd/caddy/` and a few others; the `monolift-extracted-` prefix avoids collision. Add a unit test enumerating existing `cmd/*` entries vs the prefixed name. |
| Two extracted-service pods doubles harness surface and may flake. | Both pods are tiny stdlib-only Go binaries; image build is fast. Readiness probes parallel-wait. If flake rate > 5% revisit timeouts (do not paper over with retries). |
| `SanitizeMethod` not actually called per-request because metrics-route wrapping has surprising conditions. | B.4 enables `servers { metrics }` explicitly. C.3 asserts `/calls` delta `>= 1` per request — fails loudly if the call site doesn't fire. |
| Image-build context grows because the entire host module is in the Docker context. | `<output>/lifted/host-patch/` is already a copy of `evaluation/caddy/`; the build context size matches SPRINT-0018's `Dockerfile.host` context. Two extracted-service images use `--target` against the same context (one `Dockerfile.extracted` parameterized by `--build-arg CMD_TARGET=monolift-extracted-<symbol>`). One docker-build cache pull per layer. |
| Fail-closed sentinel cascade for `SanitizeMethod` may not produce 404 the same way as `CleanPath`. | `metricsInstrumentedRoute` wraps and forwards to the inner handler regardless of the method-string. The sentinel value `\x00MONOLIFT_LIFT_FAILED\x00` would propagate as the metric label, but the request body still routes via the original path matcher. C.7 has explicit verification; if status differs from 404, document the actual status and treat that as the new fail-closed signal (record in ADR-0023 amendment). |
| Migration from separate-module breaks existing tests not enumerated. | A.5 anti-stub tests + A.7 build test catch most. Run full `go test ./...` before commit. Block A gate is the tripwire. |
| `internal/metrics` package not currently linked in the lifted Caddy binary because nothing pulls it in. | The `extracted-sanitizemethod/main.go` directly imports it; the lifted Caddy binary already pulls it via `metricsInstrumentedRoute`. Verify with `go list` against both binaries in B.3. |
| Stubcompiler now generates two `HostPatchOps` per run; ordering matters if both touch the same package. | They patch different files in different packages (`modules/caddyhttp/caddyhttp.go` vs `internal/metrics/metrics.go`), so order-independent. Patcher idempotence (structural detection from SPRINT-0018) protects against double-application anyway. |

## Forward-design sanity check

- [ ] Patcher API in `pkg/compiler/transport/emit/liftpatch/` unchanged.
- [ ] Admission rule in `pkg/compiler/transport/admission.go` unchanged.
- [ ] Liftability property vocabulary in `pkg/compiler/liftability/property.go` unchanged.
- [ ] Same template in `httpjson/main.go.tmpl` renders both `(string, bool) string` and `(string) string` signatures. A future basic-typed signature with three params would render without code changes.
- [ ] Two extracted-service pods deploy in parallel; the harness machinery generalises to N pods.
- [ ] Env-var lift gate is independent per symbol — SPRINT-0019 demonstrates two simultaneously, future sprints can add more.
- [ ] `ErrTemplateUnsupported` and the template dispatcher remain symbol-agnostic.

## Roadmap follow-ups

- **SPRINT-0020:** Miniflux target bring-up (Postgres + RSS feed server + Dockerfile + workload + golden) followed by lifting `miniflux.app/v2/internal/reader/readingtime.EstimateReadingTime`. By that point mechanism A is paid-down infrastructure.
- **Future un-numbered:** Receiver-bearing symbol slice (Cliff 2 in earnest).
- **Future un-numbered:** Multi-symbol lift in one extracted-service binary.
- **Future un-numbered:** Switch host-build to `go build -overlay` if AST-patch becomes a maintenance burden across more targets.
- **Conditional follow-up:** if a future symbol can't satisfy mechanism A (for some reason mechanism A doesn't yet anticipate), revisit the `import.legality.*` property at that point.

## Committee notes

Drafts and critiques preserved at `docs/sprints/drafts/SPRINT-0019-{CODEX,GEMINI,CLAUDE}.md` and `-critique.md`.

**Convergences adopted across drafts/critiques:**
- Mechanism A (cmd-inside-host) — unanimous.
- B (admission reject), C (re-export), D (hybrid) — all rejected with consistent rationale.
- Patcher API (`liftpatch/`) frozen.
- Admission rule and liftability property vocabulary unchanged.
- Single staging tree (`<output>/lifted/host-patch/`); `upstream/` removed.
- Two `Dockerfile`s share build context, build different `cmd/` targets.
- ADR-0023 amended additively; ADR-0018 unchanged.

**Disagreements resolved (committee + critiques):**
- **Symbol pick:** **`SanitizeMethod`** wins (Gemini's draft, both critiques validated as strongest). Per-request firing preserves SPRINT-0018's verification stack 1-for-1; in-closure (no regeneration); pure `string → string`. CODEX's `SplitUnixSocketPermissionsBits` rejected because it requires a new admin-API workload step (more harness work for strictly weaker per-request liveness). CLAUDE's `PrivateRangesCIDR` rejected because not in current closure (would require regeneration mid-sprint) and only fires at matcher-provisioning.
- **Caddyfile change:** **Add `servers { metrics }` to the global block** (Claude-critique caught Gemini's hand-wave on this). Without it, `metricsInstrumentedRoute` does not wrap routes and `SanitizeMethod` is never called per-request. Verified by tracing the call path through `evaluation/caddy/modules/caddyhttp/server.go` and `metrics.go`.
- **CleanPath preservation:** **MANDATORY.** Gemini's draft replaced CleanPath with SanitizeMethod (catastrophic regression of SPRINT-0018). Both critiques flagged. Merged plan keeps both symbols deployed in parallel.
- **Layer-1 property `import.legality.external-import-allowed`:** **REJECTED.** Gemini's Block 1. Mechanism A removes the constraint at the emission layer; admission taxonomy doesn't need to grow. Both critiques agreed.
- **Recursion safety:** **dual gate** — static YAML grep (Claude draft) AND runtime single-increment test (Codex draft). Critiques recommended both, not either-or.
- **Migration:** **replace, not keep both.** Drop `gomod.tmpl`, drop `upstream/`, regenerate cleanpath goldens, retarget `TestRenderGoBuild`. SPRINT-0018's separate-module emitter is gone after this sprint.
- **Multi-return signature exercise:** **deferred.** CODEX's draft picked `SplitUnixSocketPermissionsBits` partly to exercise emitter generality with multi-return + `error`. Useful in principle, but the harness cost (admin-API workload) bloats the sprint. Roadmap entry: future sprint can add a multi-return symbol once mechanism A is settled.
- **Sub-task count:** Codex/Claude both ballooned to 22-29 subtasks; merged plan lands at ~30 subtasks across A/B/C, weighted toward B and C because two-symbol parallel deployment is the load-bearing extension. Honest about the harness expansion (two pods, two oracles).

**Items rejected from drafts:**
- Gemini's Layer-1 property + detector (Block 1).
- Gemini's lift-target replacement (CleanPath dropped).
- Gemini's wrong line number for SanitizeMethod (`:37` → actual `:33`).
- Gemini's hand-waved Caddyfile (no `servers { metrics }` enabling step).
- Codex's admin-API workload + multi-return signature exercise (deferred).
- Claude's `PrivateRangesCIDR` symbol (not in closure).
- Claude's `caddyhttp.PrivateRangesCIDR` re-export oracle (would weaken direct internal-import proof).
- Single-extracted-Pod deployments (replaced with two-pod parallel deployment to preserve CleanPath while adding SanitizeMethod).
