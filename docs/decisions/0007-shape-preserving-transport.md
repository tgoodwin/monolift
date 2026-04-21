# ADR-0007: Shape-preserving transport for HTTP-handler lifts

**Status:** accepted _(v2 spec v1.0, 2026-04-19)_
**Date:** 2026-04-19
**Context docs:** `docs/specs/monolift-v2-contract.md` §Transport — Shape-preserving

## Context

If a lifted function is an HTTP handler (e.g. `listmonk` echo handlers, `caddy`
middleware), v1 would force the lift through an HTTP/JSON RPC wrapper —
unmarshal → call original handler → marshal. That means every handler call
becomes: client HTTP request → serialize args → HTTP to lifted service →
lifted service calls the handler with the deserialized args → handler writes
to `ResponseWriter` → lifted service serializes that response → HTTP back to
monolith → monolith unmarshals and writes to its `ResponseWriter`.

Two HTTP request/response cycles for what was already an HTTP request/response.
Pointless re-encoding, doubled latency, lost header/trailer/streaming fidelity.

## Decision

When the canonical shape (ADR-0006) is an HTTP-handler shape, the lifted
deployable **remains an HTTP handler**. The lift point forwards the incoming
HTTP request (or its sufficient representation) to the lifted deployable, and
the lifted deployable's `http.ResponseWriter` output is streamed back. No
JSON/gRPC re-encoding round-trip.

Applies to: `http.Handler`, `http.HandlerFunc`, `echo.HandlerFunc`,
`caddyhttp.MiddlewareHandler`, and any other HTTP-request/response-shaped method.

## Consequences

- HTTP-shaped lifts stay cheap: one transport hop, not two; headers and
  streaming preserved; `ResponseWriter` semantics unchanged.
- Requires a lift-point implementation that can forward HTTP requests (reverse
  proxy-ish) rather than generate an RPC client.
- Router integration: the monolith's routing layer (echo, chi, caddy's own) is
  preserved; the lift point effectively intercepts one route and forwards it.
- HTTP-handler lifts can run behind the same URL path in the cluster (same
  service DNS name, same path) — useful for gradual traffic cutover.

## References

- `docs/specs/monolift-v2-contract.md` §Transport — Shape-preserving.
- ADR-0006 (canonical shapes) — parent decision; this is one shape's transport specialization.
- Applies to: listmonk `cmd/*.go` echo handlers; caddy `modules/caddyhttp/*` middleware.
