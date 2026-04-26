# ADR-0023 — Sidecar emission and real-symbol execution

**Status:** accepted
**Date:** 2026-04-26
**Related:** ADR-0006, ADR-0017, ADR-0018, ADR-0022

## Context

SPRINT-0018 proves Monolift can move beyond report-only classification into an end-to-end extract, deploy, and execute slice. The chosen target is Caddy, and the chosen symbol is:

```go
func caddyhttp.CleanPath(p string, collapseSlashes bool) string
```

The symbol is intentionally simple. It proves transport plumbing, source acquisition, AST call-site patching, Kubernetes deployment, remote invocation, oracle equality, and fail-mode behavior. It does not prove receiver-state handling, closure capture, or complex semantic boundaries.

## Decision

### Boundary admission rule v0

The transport admission rule admits a candidate only when all six boundary properties hold:

- `boundary.serializable-via-custom-encoding`
- `boundary.no-callable-values`
- `boundary.no-streaming-values`
- `boundary.fully-instantiated`
- `boundary.variadic-free`
- `boundary.no-sync-primitives`

It also requires `lifecycle.execution-profile=sync-short`. Missing evidence is default-deny. `boundary.context-first` remains report evidence, not a marshalability gate.

### Source acquisition

The extracted service imports the real Caddy package and calls the real symbol. Its `go.mod` uses:

```go
require github.com/caddyserver/caddy/v2 v2.0.0
replace github.com/caddyserver/caddy/v2 => ../upstream
```

The `v2.0.0` version is a placeholder required by Go module semantics for `/v2` modules; the local `replace` controls resolution. `../upstream` is a clean copy of `evaluation/caddy/`, not the patched host tree, so the extracted service cannot recurse into the lift client.

### AST source patch

The lifted host image builds from a copied Caddy source tree. `liftpatch.PatchSymbolBody` locates the target function by package, name, and exact signature, then prepends a small AST-generated prelude to the function body. For `CleanPath`, the prelude checks a cached package-level `monoliftLiftEnabled` bool and calls:

```go
monoliftLiftCleanPath(p, collapseSlashes) (string, bool)
```

The `(result, ok)` dialer signature keeps transport failure separate from fail-mode policy. The call site decides whether to return the remote result, return a failure sentinel, or fall through to the original body.

The patched `caddyhttp.go` receives no new imports. HTTP client code, JSON encoding, logging, env-var reads, and timeout configuration live in the generated sibling file `monolift_lift_cleanpath.go`. Env vars are cached at package init:

- `MONOLIFT_LIFT_CLEANPATH=on`
- `MONOLIFT_LIFT_FAILMODE=open`
- `MONOLIFT_LIFT_CLEANPATH_ENDPOINT=<url>`

### Fail mode

Fail-closed is the default. If the lift is enabled and the dialer fails, `CleanPath` returns a sentinel string. In the Caddy route matcher path, that sentinel fails the normal workload path matchers and falls through to an explicit `handle { error 404 }` catch-all. This makes lift dependency failure visible in the transcript.

Fail-open is opt-in via `MONOLIFT_LIFT_FAILMODE=open`. In that mode, dialer failure falls through to the original `CleanPath` body. The workload remains available, and the extracted-service counter does not increase while the service is down.

### Production startup ordering

The sprint harness waits for both the lifted host and extracted service before running workload traffic. Production deployment needs an equivalent startup policy: either a Caddy startup/readiness gate tied to extracted readiness, or an explicit grace window and operator-owned fail-mode decision per symbol.

## Mechanism alternatives considered

### `go build -overlay`

An overlay could substitute generated files at build time without writing a patched source copy. It avoids idempotency and revert concerns. It was not chosen for this sprint because the AST prelude produces a small reviewable source diff and fits the existing Docker build-context flow. Overlay remains a candidate if multi-target patching becomes brittle.

### Build tags

A build-tagged replacement function would require excluding the original `CleanPath` declaration or splitting the original file in the patched copy. That is more invasive than prepending a prelude to one function body.

### Module-level fork

A fork selected with `replace` solves source resolution but not call-site interception. Existing calls inside Caddy still need either patched source or another mechanism that changes the function body those calls reach.

### Wrapper package

A wrapper package cannot intercept existing intra-module references to `caddyhttp.CleanPath` from Caddy's matcher and rewrite code. Those call sites already point at the original package symbol.

## Known traps and deferrals

Go `internal/` package rules still apply under `replace`. A future symbol such as Miniflux's internal reading-time helper cannot be imported by an extracted service outside the parent tree just because its module is replaced locally. Future admission or selection should record import legality explicitly.

Receiver-bearing symbols are deferred. `CleanPath` has no receiver state, no shared mutable state, and no closure capture, so this sprint does not exercise Cliff 2.

Non-Caddy generalization is deferred. The emitter and patcher APIs are parameterized, but the deployed proof point is intentionally one target and one symbol.

## Consequences

The compiler now has a concrete HTTP/JSON sidecar emission path that calls a real OSS symbol, not a stub. The host patch is idempotent, signature-checked, collision-checked, and recorded in `LIFTPATCH.json`.

The same lifted host image can run lifted or unlifted by changing environment variables. That supports later canary or A/B deployment work without requiring two binaries.

The proof remains deliberately narrow. The next sprint should pick a symbol with more semantic weight, ideally one that stresses receiver state, import legality, or a non-Caddy host layout.
