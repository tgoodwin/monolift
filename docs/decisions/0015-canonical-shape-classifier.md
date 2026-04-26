# ADR-0015: Canonical-shape classifier

**Status:** superseded in classifier logic by ADR-0017 and ADR-0018
**Date:** 2026-04-21
**Context docs:** `docs/sprints/SPRINT-0007.md`, `docs/specs/monolift-v2-contract.md` §Canonical Shapes, `docs/decisions/0012-pragma-parser-diagnostics.md`

## Context

**Supersession note (2026-04-22):** ADR-0015 remains the record of the first
canonical-shape classifier landing, but the live admissibility path now runs
through liftability-first analysis per ADR-0017, with ADR-0018 freezing the
named property set the classifier consumes. Canonical shapes still survive as
downstream selector outputs.

SPRINT-0007 introduces a canonical-shape classifier as a new semantic pass on
top of the SPRINT-0006 SSA extraction seam. The classifier must:

- preserve the ADR-0012 parser boundary by keeping shape-aware validation out
  of `pkg/compiler/pragma*.go`
- follow the TA-SHAPE-1 first-match-wins order
- write shape/default-transport decisions onto `reportv2.Root`
- replace the current `ServeHTTP` suffix heuristic with `go/types`-backed
  handler predicates, including Caddy's three-argument
  `caddyhttp.MiddlewareHandler`

## Decision

- Add a dedicated `pkg/compiler/shape/` package that classifies the root's
  exposed operations into canonical v2 shapes in strict TA-SHAPE-1 order:
  `http-handler`, `channel-consumer`, `builder-chain`,
  `ctx-request-response`, `multi-domain-args`, `no-response`,
  `unsupported`.
- Keep the parser boundary intact. `pkg/compiler/pragma*.go` continues to do
  only grammar/surface validation. Shape-aware checks run after root
  resolution in the extraction orchestration path.
- Use `go/types` and SSA evidence for predicates rather than name heuristics.
  `net/http` handler signatures and Caddy's
  `caddyhttp.MiddlewareHandler.ServeHTTP(http.ResponseWriter, *http.Request, caddyhttp.Handler) error`
  are matched from canonical types/signatures, and the old
  `strings.HasSuffix(..., ".ServeHTTP")` adapter heuristic is retired.
- Surface the classifier result directly on the report root via additive
  `reportv2.Root.Shape` and `reportv2.Root.DefaultTransport` fields. Default
  transport is derived from shape and may be narrowed by compatible pragma
  transport options.
- Route the classifier into `extract.Analyze` through a registration seam in
  `pkg/compiler/extract` rather than a direct `extract -> shape` import. This
  avoids a package cycle while keeping classification in the live extraction
  flow.
- Derive handler adapters from classifier evidence. Caddy's middleware handler
  shape maps to adapter ID `caddy-middleware-handler`; the registry adapter no
  longer emits the off-spec `registry-keyed-module` canonical-shape label.
- For full struct surfaces, aggregate per-operation shapes. All-handler
  surfaces stay `http-handler`; all-domain surfaces collapse to the most
  restrictive domain shape; mixed handler+domain or unsupported/builder-chain
  surfaces refuse with `MLV2_STRUCT_SURFACE_UNSUPPORTED`. Mixed-surface
  support is deferred behind `// TODO(SPRINT-0008-mixed-surface)`.

## Consequences

- Shape semantics are now explicit in the report and no longer inferred from
  adapter side effects.
- New shape-aware pragma validation is possible without destabilizing the
  parser package: `transport=grpc`, `transport=handler` on non-handlers, and
  later method-surface checks all live in the orchestration layer.
- Caddy is the first real framework-specific handler proof point. Future
  framework predicates can extend the classifier beside the Caddy predicate
  without reintroducing string-based matching.
- The registration seam adds a small amount of orchestration plumbing, but it
  keeps the extraction package independent from individual semantic-pass
  packages and leaves room for the state-class pass to register the same way.
