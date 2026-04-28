# ADR-0024: Multi-root region pragma

**Status:** accepted _(SPRINT-0022)_
**Date:** 2026-04-27
**Context docs:** ADR-0012, ADR-0022, ADR-0023; `docs/sprints/SPRINT-0022.md`

## Decision

Multiple `//monolift:lift` pragmas with the same non-empty `name=` denote peer roots of one region. The parser keeps declaration-local attachment, then `RegroupPragmas` builds a `Region` whose roots have stable IDs derived from declaration identity.

Region-wide options are compared after defaults. Disagreement on `mode`, `transport`, `policy`, `dispatch`, or `affinity` emits `MLV2_PRAGMA_REGION_CONFLICT`.

## Rationale

Mattermost Hub/WebConn is not a primary-root-plus-helper shape. Hub owns fanout and connection indexes; WebConn owns write-pump and replay state; the `WebConn.send` channel is the inter-root seam. Shared-name peer pragmas express that both declarations are roots without adding a file-level attachment site or an asymmetric `roots=` mini-language.

## Consequences

Peer discovery remains explicit. Seam analysis can validate and characterize declared roots, but it does not invent roots during parsing.
