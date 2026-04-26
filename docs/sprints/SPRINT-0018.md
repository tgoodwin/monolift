# SPRINT-0018 — Vertical extract→deploy→remote-execute slice (real Caddy symbol with AST call-site injection)

**Status:** planned
**Predecessor:** SPRINT-0017 (`Kind: "actor"` adapter, `transport.Selection.TemplateHandler`, ADR-0022 candidate-set machinery).
**Anchor ADRs:** ADR-0017, ADR-0018, ADR-0006. New: ADR-0023 (sidecar emission + real-symbol execution + AST call-site patching).
**Drafts (v3 round, post-AST-patch scope):** `docs/sprints/drafts/SPRINT-0018-{CODEX,GEMINI,CLAUDE}.md` plus `-critique.md`.
**Prior plan:** `docs/sprints/SPRINT-0018.v2.md` (executor-revised after the v1 committee proposed Caddyfile reverse_proxy and the user rejected it).

## Intent

Demonstrate end-to-end through the existing kind-based e2e harness that the Monolift toolchain can: (1) pick a real symbol from one of the OSS targets in `evaluation/`, (2) confirm it satisfies the existing liftability boundary properties (marshalability is `PropertyBoundarySerializableViaCustomEncoding`), (3) generate an extracted service Go module that imports and invokes the real symbol with `replace` into `evaluation/caddy/` — no synthetic body, (4) **inject a thin client into the lifted Caddy via AST-level source patch on a copy of the host tree**, so that when `MONOLIFT_LIFT_CLEANPATH=on` Caddy's request-handling path transfers control to the remote service, (5) deploy lifted Caddy + extracted service to kind, and (6) drive the existing baseline workload (no sandbox URLs, no Caddyfile fragments — `CleanPath` fires on every inbound request through `matchers.go:481,490` and `rewrite/rewrite.go:268,276,452`), then verify the lift fired by combining a per-request `/calls` counter delta, a `/invocations` structured-record endpoint with invocation-ID correlation, an in-process oracle-equality assertion against every recorded invocation, and transcript parity with the unlifted baseline.

The scope expanded materially after v2: this is now a source-rewrite sprint as much as an extracted-service sprint. AST patch correctness, host build correctness, source-tree integrity, runtime failure semantics, and multi-pod readiness are first-class concerns. Six blocks (A–F) separate the patcher from the extracted-service emitter so each gates independently.

## Chosen slice (settled — do not re-litigate)

**Target:** `caddy` (`evaluation/caddy/`, module `github.com/caddyserver/caddy/v2`).

**Symbol:** `caddyhttp.CleanPath`.

**File:** `evaluation/caddy/modules/caddyhttp/caddyhttp.go:279`.

**Signature (verified verbatim):**
```go
func CleanPath(p string, collapseSlashes bool) string
```

**Verified call sites inside Caddy.** `evaluation/caddy/modules/caddyhttp/matchers.go:481,490` invoke `CleanPath` at request-handling time during path matching. `evaluation/caddy/modules/caddyhttp/rewrite/rewrite.go:268,276,452` invoke it from the `rewrite` module. **Every inbound request to Caddy that matches a path matcher calls `CleanPath` internally** — the existing baseline workload (`/static/hello.txt`, `/headers`, `/proxy?x=1`) exercises it without any sandbox URL.

**Verified in closure.** `closure.includedSymbols` of `test/e2e/targets/caddy/golden/report.json` contains `{package_path: github.com/caddyserver/caddy/v2/modules/caddyhttp, object_name: CleanPath, kind: function, file: modules/caddyhttp/caddyhttp.go}`. Block A pins this with a regression test.

**Reservation (visible in ADR-0023):** `CleanPath` is a pure function over basic types. The lift demo proves transport plumbing and call-transfer mechanics but is informationally trivial — no receiver state, no closure capture, no boundary-property tension. Document explicitly that the next sprint must pick something with more semantic teeth.

**Alternatives reconsidered, all rejected:**
- `caddyhttp.StatusCodeMatches(int, int) bool`: smaller signature, but called only from `reverseproxy.statusError` paths, less deterministic call frequency from the workload.
- `caddyhttp.SanitizedPathJoin` and friends: not in `closure.includedSymbols` per the current Caddy report.
- `miniflux.app/v2/internal/reader/readingtime.EstimateReadingTime`: blocked by Go's `internal/` rule. `replace` does not bypass it.

## Cliff disposition

| Cliff | Status | Mechanism |
|---|---|---|
| 1 — boundary marshalability | Hold | `(string, bool) → string`. Six boundary properties + `lifecycle.execution-profile=sync-short` all Hold. `boundary.context-first=Violate` recorded but **not** a marshalability gate. |
| 2 — receiver state | N/A | Package-level function. Receiver-bearing deferred. |
| 3 — closure / source acquisition | `replace` directive | Generated extracted-service `go.mod` requires Caddy at `v2.0.0` (placeholder; `replace` overrides resolution), replaces to a stable in-context path. No source copying for the extracted service. |
| 4 — call-site replacement | **AST patch + thin client injection** | `liftpatch.PatchSymbolBody` rewrites a copy of `evaluation/caddy/modules/caddyhttp/caddyhttp.go`: prepends an `*ast.IfStmt` to `CleanPath`'s body that consults a cached package-init `monoliftLiftEnabled` bool and dials `monoliftLiftCleanPath(p, collapseSlashes) (string, bool)`. The dialer's `(result, ok)` return puts fail-mode policy at the call site, not in the dialer. A sibling generated `monolift_lift_cleanpath.go` in the same package supplies the dialer, init, and HTTP client. **The patched file gets zero new imports** — all dialer machinery lives in the sibling file. |

### Mechanism alternatives considered (recorded in ADR-0023)

- **`go build -overlay file.json`** (Go 1.16+): substitutes a JSON-listed set of files at compile time without touching the source on disk. Pros: no AST surgery, no idempotency concerns, host tree byte-identical without needing a copy at all. Cons: still requires generating a *complete* replacement file (means parsing the original to copy other declarations), Docker builds need the overlay JSON wired in. **Not chosen for v3** because the AST-prelude approach produces a smaller, easier-to-review diff (one prepended `*ast.IfStmt`) and the patched-copy mechanism integrates with existing harness Dockerfile conventions. Recorded for future evaluation; SPRINT-0019+ may switch.
- **Build tags**: a `//go:build monolift_lift` replacement file would force excluding the original `CleanPath` declaration in the same file (which contains many other functions) or splitting `CleanPath` into its own file in the patched copy. More invasive than the AST prelude. Rejected.
- **Module-level `replace` to a fork**: solves where the patched source lives, not the call-site mechanism. Adopted as the host-build vehicle (extracted service uses it for Cliff 3) but does not by itself satisfy Cliff 4.
- **Wrapper package re-export**: cannot intercept existing intra-Caddy references to `caddyhttp.CleanPath` in `matchers.go` and `rewrite.go`. Rejected.

## Patcher API and dialer surface (settled)

```go
// pkg/compiler/transport/emit/liftpatch/types.go
type PatchRequest struct {
    ModuleRoot        string             // <output>/lifted/host-patch
    PackageImportPath string             // github.com/caddyserver/caddy/v2/modules/caddyhttp
    PackageDir        string             // <ModuleRoot>/modules/caddyhttp
    FuncName          string             // CleanPath
    ExpectedSignature string             // "func(string, bool) string"
    PreludeSpec       PreludeSpec
    GeneratedFiles    []GeneratedFile    // sibling .go files to drop into PackageDir
    SentinelIdent     string             // "monoliftLiftEnabled"  (used for structural idempotence + collision scan)
}
type PreludeSpec struct {
    GoSource         string              // parsed inside the patcher; prepended to funcDecl.Body.List
    RequiredImports []string             // not used when sibling file holds all imports; kept for general API
}
type PatchResult struct {
    PatchedFile     string
    AddedImports    []string             // empty for CleanPath case
    GeneratedFiles  []string
    OriginalSHA256  string
    PatchedSHA256   string
    AlreadyApplied  bool
}

// pkg/compiler/transport/emit/liftpatch/patcher.go
func PatchSymbolBody(req PatchRequest) (PatchResult, error)
```

**Required behavior:**
- Walk every `.go` in `PackageDir` excluding `_test.go`. Error if `FuncName` not found, or if found in more than one active file (build-tagged or otherwise).
- Type-check the discovered `*ast.FuncDecl`'s signature exactly against `ExpectedSignature`. Name match alone is insufficient. (Catches future upstream renames or signature drift.)
- **Refuse:** generic functions (`Type.TypeParams != nil`), methods with receivers, functions with named-result naked returns, declarations gated by unsupported build tags. Each refusal returns a typed diagnostic.
- **Pre-write package-scope collision scan:** before emitting `GeneratedFiles`, scan all `.go` in `PackageDir` for any package-level identifier matching the `monoliftLift*` prefix. If found, return a collision diagnostic — do not pick a surprising name.
- **Idempotence: structural detection.** Detect prior application by checking `funcDecl.Body.List[0]` is `*ast.IfStmt` whose condition is `*ast.Ident{Name: SentinelIdent}`. If detected, return `AlreadyApplied: true` and produce byte-identical output. Marker comments are decorative only and may be reflowed by `go/format`; do not rely on them.
- **Import discipline:** the patched file gets zero new imports. All `os`, `net/http`, `bytes`, `encoding/json`, `time` imports live in the generated sibling. If `RequiredImports` is non-empty (general API for future symbols), use `golang.org/x/tools/go/ast/astutil.AddImport` (alias-safe, idempotent).
- **Preserve original body.** Tests compare AST statement sequence after the prelude, not raw bytes (gofmt may normalize whitespace).
- **Emit `LIFTPATCH.json`** alongside the patched file recording: package import path, file path, function name, expected signature, sentinel identifier, original/patched SHA-256, generated sibling file paths. Used by stubcompiler integrity tests and future revert/multi-lift work.

**Patched function (post-patch).**
```go
// modules/caddyhttp/caddyhttp.go (in the patched copy, NOT in evaluation/caddy/)
func CleanPath(p string, collapseSlashes bool) string {
    // BEGIN MONOLIFT-LIFT-INJECTION
    if monoliftLiftEnabled {
        if result, ok := monoliftLiftCleanPath(p, collapseSlashes); ok {
            return result
        }
        if !monoliftLiftFailOpen {
            return monoliftLiftFailureSentinel
        }
        // fail-open: fall through to original body
    }
    // END MONOLIFT-LIFT-INJECTION
    if collapseSlashes {
        return cleanPath(p)
    }
    // ...rest of original body...
}
```

**Generated sibling file `monolift_lift_cleanpath.go`** (same package):
```go
package caddyhttp

import (
    "bytes"
    "encoding/json"
    "log"
    "net/http"
    "os"
    "time"
)

const monoliftLiftFailureSentinel = "\x00MONOLIFT_LIFT_FAILED\x00"

var (
    monoliftLiftEnabled  = os.Getenv("MONOLIFT_LIFT_CLEANPATH") == "on"
    monoliftLiftFailOpen = os.Getenv("MONOLIFT_LIFT_FAILMODE") == "open"
    monoliftLiftEndpoint = func() string {
        if v := os.Getenv("MONOLIFT_LIFT_CLEANPATH_ENDPOINT"); v != "" { return v }
        return "http://monolift-extracted-cleanpath:8081/invoke"
    }()
    monoliftLiftClient = &http.Client{
        Timeout:   2 * time.Second,
        Transport: &http.Transport{MaxIdleConnsPerHost: 16},
    }
)

func monoliftLiftCleanPath(p string, collapseSlashes bool) (string, bool) {
    payload, err := json.Marshal(struct {
        P                string `json:"p"`
        CollapseSlashes  bool   `json:"collapse_slashes"`
        InvocationID     string `json:"invocation_id,omitempty"`
    }{P: p, CollapseSlashes: collapseSlashes})
    if err != nil {
        log.Printf("monolift cleanpath remote error: marshal: %v", err)
        return "", false
    }
    req, err := http.NewRequest("POST", monoliftLiftEndpoint, bytes.NewReader(payload))
    if err != nil {
        log.Printf("monolift cleanpath remote error: newrequest: %v", err)
        return "", false
    }
    req.Header.Set("Content-Type", "application/json")
    resp, err := monoliftLiftClient.Do(req)
    if err != nil {
        log.Printf("monolift cleanpath remote error: do: %v", err)
        return "", false
    }
    defer resp.Body.Close()
    if resp.StatusCode != 200 {
        log.Printf("monolift cleanpath remote error: status %d", resp.StatusCode)
        return "", false
    }
    var out struct {
        Result string `json:"result"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
        log.Printf("monolift cleanpath remote error: decode: %v", err)
        return "", false
    }
    return out.Result, true
}
```

**Fail-mode semantics.** Default is **fail-closed**. When `MONOLIFT_LIFT_CLEANPATH=on` and the dialer fails, `CleanPath` returns the sentinel string. The sentinel cascades through Caddy's matcher path to a 404 (no route matches a sentinel path) — the failure is observable in the transcript without panic-recovery dependencies. Set `MONOLIFT_LIFT_FAILMODE=open` to fall through to the original body instead (degraded but available; counter stays at 0). This pins fail-mode policy at the call site, not in the dialer, so the same `(string, bool)` dialer signature serves both modes. Production semantics deferred — ADR-0023 records that real deployments need a Caddy startup probe gated on extracted readiness or an explicit grace window.

## Goals

1. `pkg/compiler/transport/admission.go` exists; v0 rule = six boundary properties + `lifecycle.execution-profile=sync-short`. Wired into `transport.Select` fallthrough so SPRINT-0017 selectors are unaffected.
2. `pkg/compiler/transport/emit/` exists with three subpackages: dispatcher (`emit.go`), `httpjson/` (extracted-service module), `liftpatch/` (host AST patcher + dialer template).
3. Extracted-service module imports `github.com/caddyserver/caddy/v2/modules/caddyhttp` and calls `caddyhttp.CleanPath` literally. Verified by AST inspection in render tests.
4. `liftpatch.PatchSymbolBody` rewrites a host-source copy with the `PatchRequest`/`PatchResult` API above; idempotent (structural); collision-checked; signature-validated; generic/method/named-naked-return refused.
5. Stubcompiler emits a single `<output>/lifted/` build-context root containing `host-patch/` and `extracted-cleanpath/` subtrees, two Dockerfiles, k8s manifests, `MANIFEST.json`, `LIFTPATCH.json`.
6. Harness builds two images, loads into kind, applies manifests, waits for both pods Ready (HTTP `/healthz` probes), runs unchanged baseline workload.
7. Verification: per-request `/calls` counter delta `>= 1`; aggregate `<= 50`; `/invocations` structured-record endpoint correlates by `X-Monolift-Invocation-ID`; in-process oracle equality on every recorded invocation; `kubectl logs` greppable for workload paths; transcript parity vs. unlifted Caddy.
8. ADR-0023 + `evolution.md` + ledger updated.

## Non-goals

- Pragma-driven leaf-symbol selection. Symbol fixed.
- Closure source vendoring beyond the `host-patch/` copy.
- Receiver-bearing symbols (Cliff 2 in earnest). Deferred to SPRINT-0019+.
- Multi-symbol lift in one extracted-service binary.
- Generalising the emitter to non-Caddy targets. SPRINT-0019.
- Concurrency / serialization-CAS invariant tests.
- `LiftedOnly` invariant flag — no longer needed (no lifted-only URL).
- Caddyfile reverse_proxy fragments. Caddyfile is unchanged from baseline.
- `cmd/main.go` legacy path. e2e routes through stubcompiler.
- Schema churn across non-Caddy goldens. `Selection.Admission` `omitempty`.
- Modifying `evaluation/caddy/` source in place. Patch always to a copy.
- Composite-archetype emission (Mattermost). Future sprint.
- `go build -overlay` mechanism (recorded as alternative; not implemented this sprint).

## Architecture

```
    test process                                kind cluster (lifted namespace)
    ┌──────────────────┐                        ┌──────────────────────────────────────┐
    │ e2e_test.go      │   GET /static/...      │ Service: caddy                       │
    │   Workload:      ├──────────────────────► │   Pod: caddy-lifted (LIFTED IMAGE)   │
    │   3 baseline     │                        │     binary built from patched        │
    │   requests       │                        │     evaluation/caddy/ source         │
    │                  │                        │     env:                             │
    │   Oracle:        │                        │       MONOLIFT_LIFT_CLEANPATH = on   │
    │   imports real   │                        │       MONOLIFT_LIFT_FAILMODE=closed  │
    │   caddyhttp.     │                        │       MONOLIFT_LIFT_CLEANPATH_       │
    │   CleanPath      │                        │         ENDPOINT = http://monolift-  │
    │   in-process     │                        │           extracted-cleanpath:8081/  │
    │                  │                        │             invoke                   │
    │   Asserts:       │                        │     CleanPath body has prepended     │
    │    - per-req     │                        │     if monoliftLiftEnabled { ... }   │
    │      delta >= 1  │                        │     dialing the extracted service    │
    │    - aggregate   │                        │             │                        │
    │      ≤ 50        │                        │             ▼                        │
    │    - oracle ==   │                        │ Service: monolift-extracted-cleanpath│
    │      every       │                        │   Pod: extracted (LIFT IMAGE)        │
    │      invocation  │                        │     Go binary imports                │
    │    - logs ⊇      │                        │     caddyhttp.CleanPath              │
    │      workload    │                        │     real call, returns JSON          │
    │      paths       │                        │     /invoke, /calls, /invocations,   │
    │    - transcript  │                        │     /healthz                         │
    │      parity vs   │                        │                                      │
    │      env-off run │                        │ Service: echo-upstream               │
    │                  │                        │   (baseline, unchanged)              │
    └──────────────────┘                        └──────────────────────────────────────┘
```

**Wire format.** `POST /invoke` with body `{"p":"<string>","collapse_slashes":<bool>,"invocation_id":"<string>"}`. Response `200 OK` body `{"result":"<string>"}`. `GET /calls` returns `{"count":<int>}`. `GET /invocations?since=<id>` returns `{"records":[{...}]}` with the last N invocation records (request id, p, collapse_slashes, result, timestamp). `GET /healthz` returns `200 OK`. Errors return 4xx/5xx with `{"error":"<msg>"}`.

## Sequencing

Six blocks; per-block validation gates explicit. Each block lands in smaller commits as appropriate.

### Block A — Boundary admission filter

- [x] **A.1** `pkg/compiler/transport/admission.go` with `func Admit(props []reportv2.PropertyEvidence) (admitted bool, reasons []string)`. Rule v0: admit iff all of `boundary.serializable-via-custom-encoding`, `boundary.no-callable-values`, `boundary.no-streaming-values`, `boundary.fully-instantiated`, `boundary.variadic-free`, `boundary.no-sync-primitives` are `Hold`, AND `lifecycle.execution-profile` is `Hold` with detail containing `sync-short`.
- [x] **A.2** Unit tests: positive `(string,bool)→string`, plus negatives — func-typed param, channel result, type-parameter result, sync-primitive arg, variadic, missing `sync-short` detail (default-deny), long-running execution profile.
- [x] **A.3** Wire `Admit` into `pkg/compiler/transport/transport_select.go` fallthrough path *after* `selectorRuleImplicitHandler`. When admission holds and no prior selector matched, set `Selection.Template = TemplateHTTPJSON`. Test asserts SPRINT-0017's `Handler.connections` selection unaffected.
- [x] **A.4** Add additive `Admission *AdmissionRecord` (`Admitted bool`, `Reasons []string`) on `reportv2.Selection`, `omitempty`. Bump `pkg/compiler/reportv2/schema.json`. Regenerate Caddy golden. Regression test: pre-existing non-Caddy goldens (gitea, listmonk, mattermost, miniflux, pocketbase) validate unchanged against bumped schema.
- [x] **A.5** `pkg/compiler/transport/closure_pin_test.go`: load Caddy report, assert `closure.includedSymbols` contains `{package_path: github.com/caddyserver/caddy/v2/modules/caddyhttp, object_name: CleanPath, kind: function}`.

**Block A gate:** `go test ./pkg/compiler/...` green; Caddy golden regenerated; non-Caddy goldens validate unchanged.

### Block B — Extracted service emitter (httpjson/)

- [x] **B.1** Create `pkg/compiler/transport/emit/emit.go`: dispatcher `Emit(sel transport.Selection, ctx Context) (Artifact, error)`; `Artifact{ Files map[string][]byte; Manifest Manifest; HostPatchOps []HostPatchOp }`; `Context{ SymbolImportPath, ObjectName, ParamFields []FieldSpec, ResultFields []FieldSpec, UpstreamModulePath, UpstreamLocalPath, ServiceName, EnvVarPrefix }`. Typed `var ErrTemplateUnsupported = errors.New(...)`.
- [x] **B.2** `pkg/compiler/transport/emit/httpjson/` templates via `//go:embed`:
  - `templates/main.go.tmpl`: `package main`; `http.HandleFunc("/invoke", ...)` decodes JSON, increments counter via `atomic.AddInt64` *before* calling `caddyhttp.CleanPath(in.P, in.CollapseSlashes)` literally, appends `InvocationRecord` to a bounded ring buffer, returns `{"result": <value>}`. `/calls` returns `{"count": atomic.LoadInt64(&counter)}`. `/invocations?since=<id>` returns the last N records. `/healthz` returns 200. Listens `:8081`. Structured logging on every invocation: `log.Printf("LIFT_INVOKE id=%s p=%q collapse_slashes=%v result=%q", id, p, c, r)`.
  - `templates/gomod.tmpl`: `module monolift/extracted-cleanpath`, `require github.com/caddyserver/caddy/v2 v2.0.0`, `replace github.com/caddyserver/caddy/v2 => ../upstream`. **Note on version string:** Go semver requires `/v2`-suffixed modules to declare a `v2+` version even when resolution is via local `replace`; `v0.0.0` would fail `go build` with `version "v0.0.0" invalid: should be v2, not v0`. Use `v2.0.0` as a placeholder — the `replace` directive overrides resolution. **Important:** the `replace` points at `../upstream`, a **clean staged copy** of `evaluation/caddy/` *without* the patch — the extracted service must run unpatched `CleanPath` to avoid recursion.
  - `templates/dockerfile.tmpl`: multi-stage `golang:<version>` builder → `gcr.io/distroless/static`. Build context root is `<output>/lifted/`. Stage 1: `WORKDIR /src/extracted-cleanpath`, `COPY ./extracted-cleanpath /src/extracted-cleanpath`, `COPY ./upstream /src/upstream`, `RUN go build -mod=mod -o /out/extracted`. Final listens `:8081`; HTTP healthcheck on `/healthz`.
  - `templates/service.yaml.tmpl`, `templates/deployment.yaml.tmpl`: standard k8s. **Deployment env explicitly omits all `MONOLIFT_LIFT_*` vars** so the extracted service runs the unlifted code path even though it links the same package.
- [x] **B.3** Anti-stub artifact tests `pkg/compiler/transport/emit/httpjson/httpjson_test.go`:
  - `TestRenderImportsRealSymbol`: parse rendered `main.go`; assert `*ast.ImportSpec` for `github.com/caddyserver/caddy/v2/modules/caddyhttp` AND `*ast.SelectorExpr{X: caddyhttp, Sel: CleanPath}` invoked as a `*ast.CallExpr` inside the `/invoke` handler.
  - `TestRenderRejectsSyntheticBody`: a fixture template containing a hand-written CleanPath body fails the guard.
  - `TestCounterIncrementsBeforeRealCall`: AST inspection confirms `atomic.AddInt64(&counter, 1)` precedes the `caddyhttp.CleanPath` call site (catches a "stub increments counter without ever calling the symbol" cheat).
  - `TestRenderProducesGofmtClean`: every `.go` file equals `format.Source(bytes)` exactly.
  - `TestRenderDeterministic`: two consecutive renders byte-identical.
  - `TestRenderUnknownTemplate`: dispatching unsupported templates returns `ErrTemplateUnsupported`.
  - `TestRenderGoBuild`: render to temp dir, stage `evaluation/caddy/` as `../upstream`, run `go build ./...` via `os/exec`. Exit 0.
- [x] **B.4** Goldens at `pkg/compiler/transport/emit/httpjson/testdata/cleanpath/`. `-update-golden` flag.

**Block B gate:** `go test ./pkg/compiler/transport/emit/httpjson/...` green including `TestRenderGoBuild` and all anti-stub tests.

### Block C — Liftpatch emitter and patcher

- [x] **C.1** `pkg/compiler/transport/emit/liftpatch/types.go` and `patcher.go` per the API surface above. `func PatchSymbolBody(req PatchRequest) (PatchResult, error)`.
- [x] **C.2** `pkg/compiler/transport/emit/liftpatch/templates/monolift_lift.go.tmpl` parameterised on `(ServiceName, EnvVarPrefix, ObjectName, ParamFields, ResultFields)`. Emits the dialer, package-level cached vars (`monoliftLiftEnabled`, `monoliftLiftFailOpen`, `monoliftLiftEndpoint`, `monoliftLiftClient` shared `*http.Client` with 2s timeout + idle pool), `monoliftLiftFailureSentinel` constant, structured logging on every error path. File name: `monolift_lift_<lowercased ObjectName>.go` to avoid future upstream collision.
- [x] **C.3** `Render(ctx Context) (Artifact, error)` returns: rendered sibling-file bytes + `[]HostPatchOp` describing patch operations. The actual filesystem mutation happens in Block D — the emitter is pure (no I/O on host source tree).
- [x] **C.4** Liftpatch unit tests `pkg/compiler/transport/emit/liftpatch/liftpatch_test.go`:
  - `TestPatchInjectsPrelude`: fixture `func Foo(s string) string { return s }` → patch → first stmt is `*ast.IfStmt` with cond `*ast.Ident{Name: "monoliftLiftEnabled"}`.
  - `TestPatchIdempotentStructural`: applying twice returns `AlreadyApplied: true` on the second call AND produces byte-identical files.
  - `TestPatchPreservesOriginalBody`: original body statements appear after the prelude in source order; AST sequence preserved.
  - `TestPatchSignatureMismatch`: target with wrong signature returns descriptive error (not silent injection).
  - `TestPatchRefusesGenerics`: target with `Type.TypeParams != nil` returns refusal diagnostic.
  - `TestPatchRefusesReceiver`: method with receiver returns refusal.
  - `TestPatchRefusesNamedNakedReturn`: function with named results and naked return returns refusal.
  - `TestPatchRefusesBuildTagDuplicate`: package with two build-tag-active files containing the target returns ambiguity diagnostic.
  - `TestPatchMultiFilePackage`: package with two `.go` files, target only in one — patcher finds it; if duplicated across active files, returns error.
  - `TestPatchTargetNotFound`: missing function returns descriptive error.
  - `TestPatchScansForCollisions`: package containing a pre-existing `monoliftLiftFoo` identifier returns collision diagnostic.
  - `TestPatchEmitsLIFTPATCHJson`: `LIFTPATCH.json` written with package import path, file path, function name, expected signature, sentinel, original/patched SHA-256, generated sibling paths.
  - `TestRenderLiftClient`: rendered sibling parses, gofmts, contains shared `*http.Client` package-level var, init reads env via `os.Getenv`, `monoliftLiftFailureSentinel` constant present.
- [x] **C.5** Goldens at `pkg/compiler/transport/emit/liftpatch/testdata/caddyhttp/`. `-update-golden` flag.

**Block C gate:** `go test ./pkg/compiler/transport/emit/liftpatch/...` green; all refusal/ambiguity diagnostics fire correctly.

### Block D — Stubcompiler integration + lifted build context

- [ ] **D.1** Sibling API `compiler.ExtractWithTransport(...) (*reportv2.Report, []emit.Artifact, transport.Result, error)`. Existing `compiler.Extract` unchanged.
- [ ] **D.2** Extend `test/e2e/stubcompiler/main.go`: when `target == "caddy"` and `usesRealCompiler(target)`, call `compiler.ExtractWithTransport`. Stubcompiler then materializes:
  - `<output>/lifted/host-patch/` ← `os.CopyFS` of `evaluation/caddy/`.
  - `<output>/lifted/upstream/` ← second `os.CopyFS` of `evaluation/caddy/` (clean copy for the extracted service's `replace`).
  - Apply each `HostPatchOp` via `liftpatch.PatchSymbolBody` against `<output>/lifted/host-patch/modules/caddyhttp/`.
  - Drop rendered `monolift_lift_cleanpath.go` into `<output>/lifted/host-patch/modules/caddyhttp/`.
  - `<output>/lifted/extracted-cleanpath/{main.go, go.mod, Dockerfile}` from `httpjson` artifacts.
  - `<output>/lifted/Dockerfile.host` building lifted Caddy from `host-patch/`.
  - `<output>/lifted/manifests/{caddy-lifted-deployment.yaml, caddy-lifted-service.yaml, extracted-deployment.yaml, extracted-service.yaml}`. Lifted Caddy deployment env: `MONOLIFT_LIFT_CLEANPATH=on`, `MONOLIFT_LIFT_FAILMODE=closed`, `MONOLIFT_LIFT_CLEANPATH_ENDPOINT=http://monolift-extracted-cleanpath:8081/invoke`.
  - `<output>/lifted/MANIFEST.json` listing all emitted paths.
  - `<output>/lifted/host-patch/modules/caddyhttp/LIFTPATCH.json` from the patcher.
- [ ] **D.3** Source-tree integrity test: hash `evaluation/caddy/` (full directory tree, not just the patched file) before and after stubcompiler run; assert byte-identical. Make target `make verify-evaluation-untouched` invokes the same check.
- [ ] **D.4** Extend `test/e2e/harness/target.go` with `LiftedHostBuild *HostBuildSpec{ Dockerfile, ContextRoot, ImageTag }` and `LiftedExtractedServices []ExtractedServiceSpec{ Name, Dockerfile, ContextRoot, ImageTag, DeploymentYAML, ServiceYAML, ReadinessPath }`. Both share `<output>/lifted/` as `ContextRoot`.
- [ ] **D.5** Update `test/e2e/targets/caddy/target.go`: populate `LiftedHostBuild` and one `LiftedExtractedServices` entry. Baseline `Dockerfile`, `BaselineManifests` (echo-upstream, baseline caddy deployment, configmap, baseline service) unchanged. **Lifted Caddy reuses the same Caddyfile ConfigMap as baseline** — no fragments, no new routes. Same workload runs on both env-on and env-off deployments.
- [ ] **D.6** Snapshot `test/e2e/stubcompiler/fixtures/caddy/lifted/` → `lifted-baseline-snapshot/` in a separate commit (precedes Block E). Final closeout commit (Block F) deletes it.
- [ ] **D.7** Stubcompiler integration test `test/e2e/stubcompiler/stubcompiler_test.go::TestEmitsLiftedTreeForCaddy`:
  - patched `caddyhttp.go` differs from original at exactly the `CleanPath` body (prepended `*ast.IfStmt` with `monoliftLiftEnabled` cond) — and **only** there;
  - `monolift_lift_cleanpath.go` exists, parses, contains `*http.Client` package-level var, `monoliftLiftFailureSentinel` constant;
  - `extracted-cleanpath/main.go` AST contains the real `caddyhttp.CleanPath` selector call inside the `/invoke` handler;
  - `LIFTPATCH.json` round-trips through `encoding/json` and SHA-256 fields are non-empty;
  - `go build ./...` from `<output>/lifted/host-patch/` succeeds;
  - `go build ./...` from `<output>/lifted/extracted-cleanpath/` (with `../upstream` populated) succeeds;
  - `evaluation/caddy/` checksum unchanged.

**Block D gate:** `go test ./pkg/compiler/... ./test/e2e/stubcompiler/...` green; both materialized trees compile; `evaluation/caddy/` checksum unchanged.

### Block E — Kubernetes deploy + e2e oracle + verification

- [ ] **E.1** Extend `test/e2e/harness/imagebuild.go` to honor `LiftedHostBuild` and `LiftedExtractedServices`: build lifted-Caddy image (`monolift-e2e/caddy-lifted:e2e`) and extracted-service image (`monolift-e2e/extracted-cleanpath:e2e`) with shared context root, distinct Dockerfiles. Load both into kind. Sequence: build all → load all → `deployer.Apply`.
- [ ] **E.2** Extend `test/e2e/harness/deployer.go` for multi-pod readiness: wait for each `LiftedExtractedServices[*].ReadinessPath` (HTTP `/healthz`) and the lifted Caddy's existing readiness before workload runs. If any readiness fails within timeout, harness reports which pod is not ready (don't silently proceed).
- [ ] **E.3** `test/e2e/harness/oracle.go`: `type SymbolInvoker interface { Invoke(args map[string]any) (any, error) }`. Caddy implementation in `test/e2e/targets/caddy/oracle.go` imports `github.com/caddyserver/caddy/v2/modules/caddyhttp` and calls `caddyhttp.CleanPath(args["p"].(string), args["collapse_slashes"].(bool))` directly. Confirm repo `go.mod` already has the `replace` for the `caddy` evaluation module.
- [ ] **E.4** Workload: **no URL changes** from baseline (`/static/hello.txt`, `/headers`, `/proxy?x=1`). Each request triggers `CleanPath` inside Caddy via `matchers.go:481,490`.
- [ ] **E.5** Harness assertion `harness.AssertExtractedServiceCallDelta(serviceName string)`: port-forwards the extracted service, reads `/calls` *before each individual workload request*, asserts `delta >= 1` after each, accumulates total. Final aggregate assert: `3 <= total <= 50` (lower: at least one per request; upper: catches recursion / accidental client loops).
- [ ] **E.6** Harness assertion `harness.AssertExtractedInvocations(serviceName string, expectedPaths []string)`: queries `/invocations` after workload, parses records, asserts at least one record per `expectedPaths` entry. For each record, independently invokes `caddy.Oracle.Invoke({p: rec.P, collapse_slashes: rec.CollapseSlashes})`, asserts `oracle.result == rec.Result`. **This is the load-bearing falsifiability check** — a stub returning canned values would fail oracle equality on at least one input.
- [ ] **E.7** Harness assertion `harness.AssertExtractedServiceLogs(serviceName string, expectedSubstrings []string)`: `kubectl logs deployment/<name>` greps for `LIFT_INVOKE id=` and each workload path. Secondary observability check; not load-bearing.
- [ ] **E.8** Transcript parity assertion: capture baseline transcript (env-off deployment, same image), capture lifted transcript (env-on), assert status/headers (modulo Date/Server)/body equality on all baseline workload responses. Lifted Caddy must produce identical responses to baseline Caddy when the lift dialer succeeds.
- [ ] **E.9** Negative test: re-deploy lifted Caddy with `MONOLIFT_LIFT_CLEANPATH` *unset*; assert `/calls` delta = 0; transcript identical to env-on case.
- [ ] **E.10** Fail-mode tests (run after E.9 to avoid masking E.5–E.8):
  - **Fail-closed (default).** Scale extracted deployment to 0 replicas, fire one workload request, assert it returns 404 (sentinel cascades to no-route-match). Scale back to 1, assert subsequent requests succeed and counter increments.
  - **Fail-open.** Re-deploy lifted Caddy with `MONOLIFT_LIFT_FAILMODE=open`, scale extracted to 0, fire workload, assert 200 responses (degraded but available, original `CleanPath` body executed). Counter stays at 0. Restore replicas; counter increments again.

**Block E gate (sprint acceptance):** `MONOLIFT_E2E=1 go test -tags=e2e ./test/e2e -run TestE2E/caddy -count=1` green: per-request delta ≥ 1, aggregate 3 ≤ total ≤ 50, oracle equality on every recorded invocation, log substrings present, transcript parity, env-off zero counter, fail-closed 404, fail-open 200. SPRINT-0017 actor-adapter assertions unchanged.

### Block F — ADR + docs + closeout

- [ ] **F.1** `docs/decisions/0023-sidecar-emission-and-real-symbol-execution.md`. Sections: boundary admission rule v0; `replace`-directive sourcing; AST source-patch mechanism with cached-init env-var gate and `(string, bool)` dialer signature; `lifecycle.execution-profile=sync-short`; **Mechanism alternatives considered** (`-overlay`, build tags, module-level fork, wrapper package — with rejection rationale); `internal/`-import trap for future symbol picks; deferred receiver-state cliff; deferred non-Caddy generalization; fail-closed-by-default rationale; production startup-ordering note (deployments need a Caddy startup probe gated on extracted readiness or grace window).
- [ ] **F.2** Append `docs/evolution.md` with the slice landing.
- [ ] **F.3** Update `docs/sprints/ledger.yaml`: SPRINT-0018 → `done` once Block E is green.
- [ ] **F.4** Final closeout commit deletes `lifted-baseline-snapshot/`. Single dedicated commit so bisect lands cleanly.
- [ ] **F.5** `cmd/main.go` unchanged — verified.

## Acceptance criteria

All must hold at sprint close:

- [ ] `pkg/compiler/transport/admission.go` exists; v0 rule with all seven properties; positive + six negative unit tests.
- [ ] `closure_pin_test.go` asserts `caddyhttp.CleanPath` in Caddy report.
- [ ] `Selection.Admission` additive `omitempty`; non-Caddy goldens unchanged.
- [ ] `pkg/compiler/transport/emit/{emit.go,httpjson/,liftpatch/}` exist; dispatcher keys on `Selection.Template`; `ErrTemplateUnsupported` typed sentinel.
- [ ] Anti-stub render tests pass: `TestRenderImportsRealSymbol`, `TestRenderRejectsSyntheticBody`, `TestCounterIncrementsBeforeRealCall`, `TestRenderGoBuild`.
- [ ] Liftpatch tests pass: `TestPatchInjectsPrelude`, `TestPatchIdempotentStructural`, `TestPatchPreservesOriginalBody`, `TestPatchSignatureMismatch`, `TestPatchRefusesGenerics`, `TestPatchRefusesReceiver`, `TestPatchRefusesNamedNakedReturn`, `TestPatchRefusesBuildTagDuplicate`, `TestPatchMultiFilePackage`, `TestPatchTargetNotFound`, `TestPatchScansForCollisions`, `TestPatchEmitsLIFTPATCHJson`, `TestRenderLiftClient`.
- [ ] Stubcompiler against caddy emits the full `<output>/lifted/` tree; `host-patch/`, `upstream/`, and `extracted-cleanpath/` subtrees compile via `go build`; `MANIFEST.json` and `LIFTPATCH.json` round-trip; **zero string-substituted symbol bodies**.
- [ ] `evaluation/caddy/` byte-identical pre/post stubcompiler (checksum integrity test); `make verify-evaluation-untouched` passes.
- [ ] Patched `caddyhttp.go` differs from original at exactly the `CleanPath` body — prepended `*ast.IfStmt` with `monoliftLiftEnabled` cond, `(result, ok)` dialer call, sentinel return on fail-closed, fall-through on fail-open. **Zero new imports** in the patched file (all dialer machinery in the sibling).
- [ ] Both images build and load into kind; both pods Ready before workload (HTTP `/healthz` probes).
- [ ] e2e green: per-request `/calls` delta ≥ 1, aggregate 3 ≤ total ≤ 50, oracle equality on every `/invocations` record, logs contain `LIFT_INVOKE id=` plus all workload paths, transcript parity vs. env-off, env-off zero counter, fail-closed 404, fail-open 200 (degraded but available).
- [ ] SPRINT-0017 actor-adapter assertions (`archetype_kind`, primary `serialized-actor`, alternative `keyed-partitioned-state` `[TOPOLOGY]`, adapter `Kind: actor`) unchanged.
- [ ] `docs/decisions/0023-sidecar-emission-and-real-symbol-execution.md` exists with **Mechanism alternatives considered** subsection covering `-overlay` and the rejection rationale; `docs/evolution.md` records the slice; ledger updated.
- [ ] No concurrency / serialization-CAS test exists in new code.
- [ ] No `LiftedOnly` invariant flag exists in harness.
- [ ] `cmd/main.go` unchanged.
- [ ] Final commit deletes `lifted-baseline-snapshot/`; sprint branch is bisect-clean.

## Forward-design sanity check

- [ ] Dispatcher in `pkg/compiler/transport/emit/emit.go` switches on `Selection.Template` only.
- [ ] `emit.Context` struct has no Caddy-specific fields.
- [ ] `liftpatch.PatchSymbolBody` accepts package/function/expected-signature inputs and does not hard-code `CleanPath` except in tests and Caddy fixture wiring.
- [ ] Generated dialer returns `(result, ok)`, leaving fail-mode policy at the call site.
- [ ] `monolift_lift_<symbol>.go` template is parameter-driven; a different basic-typed signature produces a structurally identical sibling file with different field names.
- [ ] Patched file has zero new imports (all imports in sibling) — patcher tested for this discipline.
- [ ] `LIFTPATCH.json` and `MANIFEST.json` provide enough metadata for a later sprint to diff, revert, or apply multiple lift points.
- [ ] Emitter never mutates input source trees; stubcompiler does the I/O against `<output>/lifted/host-patch/` and `<output>/lifted/upstream/`.
- [ ] Env-var-gated lift point lets the same image run lifted or unlifted — same binary, different deployments.
- [ ] `ErrTemplateUnsupported` is a typed sentinel for `handler` (SPRINT-0017 existing), `channel-consumer`, `grpc`.
- [ ] Fail-closed-by-default surfaces lift dependency in transcripts (404 sentinel cascade); fail-open is opt-in.
- [ ] Extracted service deployment env explicitly omits `MONOLIFT_LIFT_*` vars to prevent recursion (extracted service runs unpatched body).

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| AST patch breaks Caddy compilation. | C.4 unit tests cover refusal cases (generics, receiver, named-naked-return, build-tag duplicate, signature mismatch, target-not-found, collision). D.7 runs `go build` against the patched tree before image build. |
| Patcher silently fails to find target (typo, future Caddy refactor). | `PatchSymbolBody` returns descriptive error if no `*ast.FuncDecl` matches; signature mismatch returns separate error. Closure-pin test (A.5) catches upstream rename. |
| Idempotence false-positive applies twice. | Structural detection on first stmt's `*ast.IfStmt` with sentinel ident cond; comment is decorative. `TestPatchIdempotentStructural` is the gate. |
| Per-call `os.Getenv` in hot path (v2 defect). | Cached at package init via `var monoliftLiftEnabled = os.Getenv(...) == "on"`. Same for `monoliftLiftFailOpen`, `monoliftLiftEndpoint`. Single map lookup at startup, not per-call. |
| Hot-path latency / kind cluster jitter. | Shared `*http.Client` with 2s timeout + `MaxIdleConnsPerHost: 16`. Aggregate counter upper bound `<= 50` catches regressions but tolerates matcher iteration variance. If flakes exceed 5%, revisit timeouts; do not paper over with retries. |
| Recursion: extracted service builds patched code and re-dials itself. | Extracted service's `go.mod` `replace`s to `../upstream` (clean copy), not `../host-patch`. Extracted Deployment env explicitly omits `MONOLIFT_LIFT_*`. Both belt and suspenders. |
| Generated identifier collision with future Caddy code. | `liftpatch.PatchSymbolBody` pre-scans package scope for any `monoliftLift*` identifier; refuses to write if found. `TestPatchScansForCollisions` is the gate. |
| Import injection collisions (named/dot/aliased). | The patched file gets zero new imports — all imports live in the sibling file. `astutil.AddImport` is available for the general API but unused in the CleanPath case. |
| Docker build context layout for shared `replace` source. | Single `<output>/lifted/` build context root; both Dockerfiles read sibling subtrees (`host-patch/`, `upstream/`, `extracted-cleanpath/`). Documented in D.2. |
| Fail-open hides regressions. | Default is fail-closed; sentinel cascades to 404. `MONOLIFT_LIFT_FAILMODE=open` is opt-in and only exercised by E.10's dedicated test. |
| Counter assertion flake from matcher iteration count. | Per-request delta `>= 1` (lower bound robust to matcher count) plus aggregate `<= 50`. Records observed values for diagnostics. |
| Boundary admission catches existing `Handler.connections` and breaks SPRINT-0017. | `Admit` consulted only in fallthrough after `selectorRuleImplicitHandler`. A.3 placement is explicit; tested. |
| Schema bump churns non-Caddy goldens. | `Selection.Admission` `omitempty`. A.4 regression test asserts non-Caddy goldens validate unchanged. |
| Extracted service can be a stub returning canned values. | `TestRenderImportsRealSymbol` AST-inspects for the real selector call. `TestCounterIncrementsBeforeRealCall` asserts ordering. **The load-bearing defense is E.6's oracle equality on every `/invocations` record** — a stub returning canned values would fail at least one oracle comparison. |
| Verification spoofable by a goroutine logging `LIFT_INVOKE` at startup. | Logs are secondary; primary verification is `/invocations` structured records correlated by `X-Monolift-Invocation-ID` plus oracle equality. |
| Multi-Pod readiness wait races. | Both deployments have HTTP `/healthz` readiness probes; harness waits per-selector (E.2). |
| Test binary cannot import `caddyhttp` directly. | Repo `go.mod` already `replace`s caddy via the existing baseline e2e plumbing; oracle file uses the same. If missing, additive change in the same PR. |
| Patched copy on disk doubles repo size on every test run. | `<output>/lifted/` lives under `os.TempDir()` or an explicit gitignored path under `.tmp/`; cleaned per-run. |
| Sentinel-string fail-closed mechanism leaks into baseline if env var misconfigured. | The sentinel only returns when `monoliftLiftEnabled=true` AND dialer fails AND `monoliftLiftFailOpen=false`. Deployment YAMLs pin all three; no path returns the sentinel under normal operation. |
| Production semantics vs demo semantics drift. | ADR-0023 records explicit production-vs-demo gap: production needs Caddy startup probe gated on extracted readiness, real fail-mode policy decided per symbol, observability via metrics not log lines. Demo defers all of these. |

## Roadmap follow-ups

- **SPRINT-0019:** generalise to a non-Caddy target (Pocketbase or Listmonk). Surfaces `internal/`-import legality property addition to admission. May exercise receiver-bearing symbol slice (Cliff 2 in earnest). May switch to `go build -overlay` if AST-patch turns out to be unwieldy across multiple host targets.
- **Future un-numbered:** Multi-symbol lift in one extracted-service binary.
- **Future un-numbered:** Canary / A-B routing using the env-var lift point — same image, different replicas with different env values, traffic split.
- **Future un-numbered:** Mattermost composite-archetype emission. Consumes SPRINT-0017's `ExtendWithComposites` seam plus this sprint's emitter contract.
- **Future un-numbered:** Replace HTTP/JSON with a typed-codec template (gRPC, capnp) once symbol surfaces grow.
- **Conditional follow-up:** if AST-patch proves brittle in SPRINT-0019, switch the host-patch mechanism to `-overlay`.

## Committee notes

Drafts and critiques (v3 round) preserved at `docs/sprints/drafts/SPRINT-0018-{CODEX,GEMINI,CLAUDE}.md` and `-critique.md`. Prior round at `*.v2.md`.

**Convergences adopted across drafts/critiques:**
- Caddy as target; `replace` directive over source copying for Cliff 3 (unanimous).
- `caddyhttp.CleanPath` as the chosen symbol — branchier than StatusCodeMatches, deterministic call frequency from baseline workload via `matchers.go:481,490`.
- Admission filter as discrete `pkg/compiler/transport/admission.go` (CLAUDE; CODEX/GEMINI critiques validated).
- `harness.SymbolInvoker` oracle (CLAUDE; both critiques validated).
- Pin chosen symbol to `closure.includedSymbols` regression test (CODEX; both critiques validated).
- `lifecycle.execution-profile=sync-short` in admission rule (CODEX; both critiques validated).
- Anti-stub artifact tests inspecting AST for the real selector call (unanimous).
- `internal/`-import trap explicitly documented in ADR-0023 (CODEX; both critiques validated).
- ADR-0023 + `evolution.md` + ledger update.

**Disagreements resolved post-merge (v3 round):**
- **Patcher API:** adopted CODEX's richer `PatchRequest`/`PatchResult` data model with `ExpectedSignature`, `OriginalSHA256`/`PatchedSHA256`, `AlreadyApplied`, `LIFTPATCH.json`. Combined with CLAUDE's structural idempotence detection (first stmt is `*ast.IfStmt` with sentinel cond) — CODEX-critique correctly observed marker-comment detection is fragile under `go/format` reflow.
- **Refusal list:** adopted CODEX's explicit refusal of generics, methods with receivers, named-result naked returns, and build-tag duplicates. CLAUDE's draft was silent on these — silent gap.
- **Collision scan:** adopted CODEX's pre-write package-scope identifier scan. CLAUDE's draft and v2 baseline relied on prefixes alone, which can collide.
- **Import discipline:** adopted CODEX's "patched file gets zero new imports" — all dialer machinery in the sibling. Strictly cleaner than CLAUDE's `astutil.AddImport` machinery (which was doing real work for nothing in the CleanPath case). `astutil.AddImport` retained in the general API for future symbols where the call-site sibling-resolution doesn't fit.
- **Env-var gating:** adopted CLAUDE's cached-at-init `monoliftLiftEnabled` package-level var. Per-call `os.Getenv` (v2 baseline + GEMINI) is wrong because a runtime-mutable lift flag implies an in-process re-entry path nothing in the design needs.
- **Fail-mode:** combined CODEX's `(string, bool)` dialer signature with CLAUDE's fail-closed-by-default policy via a sentinel-string mechanism. CLAUDE's original panic-as-fail-closed was wrong (`CleanPath` returns `string` only; panic surfaces depend on Caddy recovery semantics). The sentinel cascades through Caddy's matcher path to a 404 transcript — observable, falsifiable, no panic recovery dependence. `MONOLIFT_LIFT_FAILMODE=open` is opt-in.
- **Counter assertion:** adopted CLAUDE's per-request delta `>= 1` plus aggregate `<= 50`. CODEX's "measured lower bound" is theoretically nicer but pushes a measurement step into the harness that isn't fully specified.
- **Docker build context:** adopted CLAUDE's single `<output>/lifted/` shared root with both Dockerfiles `WORKDIR`-ing into siblings. Resolved by adding `<output>/lifted/upstream/` as a clean (unpatched) copy so the extracted service's `replace` doesn't point at the patched tree (recursion risk caught by CODEX critique).
- **Verification rigor:** adopted CODEX's `/invocations` structured-record endpoint + `X-Monolift-Invocation-ID` correlation. Strictly stronger than log-grep argument extraction. The load-bearing falsifiability check is per-record oracle equality; logs are secondary.
- **Block structure:** adopted CODEX's A–F separation (liftpatch as its own Block C). Cleaner gating than CLAUDE's combined Block B.
- **`-overlay` alternative:** documented in ADR-0023's "Mechanism alternatives considered" subsection (CLAUDE-critique caught the committee's silence on this; recorded for future evaluation but not implemented this sprint).
- **Startup ordering:** documented in ADR-0023 — production needs a Caddy startup probe gated on extracted readiness or an explicit grace window. Demo bounded by `harness.deployer.Apply` waiting for both pods Ready before workload.
- **GEMINI's signature defect:** rejected. GEMINI's draft used `CleanPath(p string) string` (one arg) and a `text/plain` body losing `collapseSlashes` over the wire. Correct signature is `(p string, collapseSlashes bool) string`; wire format is JSON with both fields.

**Items rejected from drafts:**
- GEMINI's entire wire format (`text/plain`, port 8080, service name `monolift-cleanpath`).
- GEMINI's verification (log-grep + transcript parity only — trivially spoofable by a goroutine logging at startup).
- GEMINI's 50ms HTTP timeout (would induce flakes).
- CLAUDE's panic-as-fail-closed (replaced with sentinel-cascade).
- CLAUDE's `extracted-cleanpath/go.mod` pointing at `host-patch/` (replaced with clean `upstream/` copy to prevent recursion — CODEX-critique caught this).
- v2 baseline's per-call `os.Getenv` injection into the patched file (replaced with cached-init in sibling).
- v2 baseline's marker-comment-only idempotence (replaced with structural detection).
- v2 baseline's `count >= 3` lower-bound-only counter (replaced with per-request delta + aggregate upper bound).

## Resolved blockers

- **Block B.2 go.mod version (resolved):** initial plan specified `require github.com/caddyserver/caddy/v2 v0.0.0`. `go build` rejected with `version "v0.0.0" invalid: should be v2, not v0` because Go semver enforces `v2+` versions for `/v2`-suffixed modules. Resolution: changed required version to `v2.0.0` (placeholder; `replace` overrides resolution). Plan updated. Resume Block B.2.
