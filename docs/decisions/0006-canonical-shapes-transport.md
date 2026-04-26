# ADR-0006: Canonical shapes as the transport/adapter organizing concept

**Status:** accepted _(v2 spec v1.0, 2026-04-19; narrowed by ADR-0017 on 2026-04-22)_
**Date:** 2026-04-19
**Context docs:** `docs/specs/monolift-v2-contract.md` §Transport, `docs/sprints/drafts/SPRINT-0003-GEMINI.md` (origin of the "canonical shapes" framing)

## Context

**Status note (2026-04-22):** ADR-0017 keeps canonical shapes as the
transport/adapter vocabulary, but admissibility is no longer defined by these
shapes. This ADR remains authoritative for the bounded canonical-shape set and
its transport/adapter role; it is not fully superseded.

v1 requires `(ctx context.Context, req T) (resp U, error)` — a single method
shape, checked strictly, with a `panic()` at `pkg/lift/clientgen.go:110`
if violated.

The audit enumerated the method shapes that show up in real targets:

- `(c echo.Context) error` — listmonk
- `(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error` — caddy middleware
- `(rctx request.CTX, user *model.User, opts UserCreateOptions) (*model.User, error)` — mattermost
- `(ctx, u *user_model.User, newName string, doer *user_model.User) error` — gitea domain methods
- Builder chains (`rb.WithUserAgent(...).WithHeader(...)`)
- Channel consumers reading work items (listmonk campaign worker)

A naive per-target-per-shape adapter-generation approach would explode the
compiler's template surface. A strict one-shape-only approach rules out 5/6 targets.

## Decision

Group method signatures into a **small bounded set of canonical shapes**,
each with a shared adapter template, each mapped to exactly one default
transport or a refusal:

1. **RPC-request/response** — `(ctx, req) → (resp, err)` → HTTP/JSON (default)
2. **Multi-domain-argument** — e.g., `(ctx, a, b, c) → (resp, err)` → HTTP/JSON with struct-synthesis
3. **No-response** — `(ctx, req) → err` → HTTP/JSON one-way
4. **HTTP handler** — `http.Handler`, `echo.HandlerFunc`, caddy middleware → **shape-preserving transport** (see ADR-0007)
5. **Channel consumer** — long-lived goroutine reading a channel → singleton deployable with internal queue (see ADR-0005)
6. **Builder chain** — fluent API → refused or specified extension

Signature classification is deterministic and ordered, so a single method is
never matched by two shapes with conflicting transports.

This framing survived from the Gemini draft of SPRINT-0003, replacing both
Codex's and Claude's broader "per-signature-class template" phrasing.

## Consequences

- Caps the compiler's transport template surface at ~6 templates rather than
  one per target or one per signature-in-the-wild.
- Provides a clean refusal path: if a signature doesn't match any canonical
  shape, the compiler emits a named diagnostic instead of panicking.
- Retires `pkg/lift/clientgen.go:110`'s panic.
- Pairs naturally with ADR-0007: HTTP-handler shapes get a dedicated
  shape-preserving transport rather than being re-encoded to JSON.

## References

- `docs/specs/monolift-v2-contract.md` §Transport.
- `docs/sprints/drafts/SPRINT-0003-GEMINI.md` L90 — "Canonical Shapes" coinage.
- `pkg/lift/clientgen.go:110` — the panic this decision retires.
- ADR-0007 (shape-preserving transport) — companion decision.
