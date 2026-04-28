# ADR-0027: RegionPatchRequest

**Status:** accepted
**Date:** 2026-04-27
**Related:** ADR-0022, ADR-0023, ADR-0024

## Context

SPRINT-0022 could analyze Mattermost Hub/WebConn as one multi-root region, but
emission stopped because the patcher API patched one free function per request.
Multi-root regions need multiple methods, sometimes across packages, to be
patched as one region operation.

## Decision

`liftpatch` adds `RegionPatchRequest` as an additive sibling to
`PatchSymbolBody`. The legacy `PatchSymbolBody(PatchRequest)` API and method
receiver refusal remain unchanged.

`RegionPatchRequest` contains:

- region name,
- per-symbol package/file/function/receiver/signature/prelude requests,
- shared generated files.

Routing rule: regions with more than one root, or any receiver-bearing root, use
`PatchRegion`; single free-function regions keep `PatchSymbolBody`.

Sentinels are deterministic per `(region name, package import path)` to avoid
cross-package collisions.

## Consequences

The patcher can represent the Mattermost-shaped emission plan without
Mattermost-specific carve-outs. Existing Caddy/Miniflux/PocketBase paths keep
the byte-identical single-symbol patch route.
