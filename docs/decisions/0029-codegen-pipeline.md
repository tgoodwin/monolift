# ADR-0029: Activation-cut HTTP/JSON codegen pipeline

**Status:** accepted
**Date:** 2026-05-07
**Context docs:** ADR-0023, ADR-0025, ADR-0028, SPRINT-0041

## Context

ADR-0028 makes the monolith-as-gateway model explicit: after lifting, the
monolith keeps serving external traffic and forwards selected cut-point calls to
a lifted service. SPRINT-0040 added cut placement, but the existing emitters did
not match this input shape.

Two codegen paths already existed:

- `pkg/lift/`, the v1 interface-oriented pipeline, expects
  `(context.Context, req) (resp, error)` surfaces.
- `pkg/compiler/transport/emit/`, the v2 pipeline, is closer but is still
  coupled to pragma-root interface reports and hard-coded transport contexts.

SPRINT-0041 needs an end-to-end path from an activation-path cut candidate to
generated Go source for arbitrary function signatures such as miniflux
`SanitizeHTML` and `RefreshFeed`.

## Decision

Create `pkg/codegen/` as a focused activation-cut codegen pipeline. Its contract
is a `Plan` built from the extraction report and recommended cut. The plan
partitions parameters into:

- **Boundary params:** values sent in the HTTP/JSON request.
- **Reconstructed params:** state rebuilt in the generated server at startup.
- **Results:** values encoded in the HTTP/JSON response.

The generator emits two Go artifacts:

1. A standalone HTTP/JSON server under the source module root.
2. A same-package monolith stub that forwards to the server when enabled by
   environment variables and otherwise calls the original function.

Generated code stays inside the source module root so Go `internal/` imports
remain legal. No `go.mod` replacement is introduced.

## Templates Over AST Rewriting

Server and client source are emitted with Go templates, then formatted with
`go/format`. These files are new artifacts with predictable structure; templates
make the wire contract, env vars, handler shape, and reconstruction code easy to
review and golden-test.

AST rewriting is reserved for the existing monolith callsite. That edit touches
developer-owned source, so it uses `github.com/dave/dst` to preserve comments and
formatting around the single selected call expression while still type-checking
that the call resolves to the intended callee.

## HTTP/JSON First

HTTP/JSON is the phase-1 transport because it is simple, inspectable, and enough
for the 62/71 corpus traces whose boundaries are trivial, serializable, or
client-reconstructible. The generated API is intentionally narrow:

- `POST /invoke` for calls.
- `GET /healthz` for readiness checks.
- `MONOLIFT_HTTP_ADDR` for server listen address.
- `MONOLIFT_<SERVICE>_ENDPOINT` and `MONOLIFT_LIFT_<SERVICE>=on` for client
  routing.

Streaming, bidirectional calls, composite cuts, and shared-state reconstruction
remain outside this pipeline.

## State Reconstruction

State reconstruction is registry-based, keyed by Go type identity rather than
application-specific config APIs. The initial taxonomy is:

- **Stateless:** no server-side state is initialized.
- **Config-only:** values can be rebuilt from environment or static config.
- **Client-reconstructible:** clients such as DB handles or HTTP clients are
  excluded from the wire request and rebuilt once in the generated server.
- **Shared state:** refused for this phase.

The first reconstructors cover SQL DB wrappers, `*http.Client`, and
`*log.Logger`. Adding support for new state families should be a registry entry,
not a bespoke service template.

## Relationship To Existing Emit Infrastructure

This ADR does not replace `pkg/compiler/transport/emit/`. The new package exists
because activation-cut generation starts from a different contract: a concrete
function on a discovered path, not a pragma-described interface surface.

The intended unification path is:

1. Keep `pkg/codegen.Plan` as the explicit activation-cut transport contract.
2. Reuse common field/type specifications with the v2 emit stack where the data
   model aligns.
3. Move shared writer, manifest, and admission utilities behind transport-neutral
   interfaces once both paths have enough real targets to prove the common shape.

## Consequences

The first end-to-end lift path can be validated on real miniflux functions
without changing activation-path or cut-placement algorithms. The tradeoff is a
temporary parallel codegen path, accepted because it keeps the MVP narrow and
avoids forcing activation-cut semantics into the older interface-oriented
templates prematurely.
