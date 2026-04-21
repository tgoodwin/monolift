# ADR-0002: Renegotiate the Monolift compiler contract (v1 → v2)

**Status:** accepted
**Date:** 2026-04-19
**Context docs:** `docs/evaluation/generalization-analysis-2026-04-19.md`, `docs/specs/monolift-v2-contract.md`, `docs/sprints/SPRINT-0003.md`

## Context

The six-target audit (ADR-0001) scored Monolift's v1 input contract against
eight dimensions on six real Go monoliths. The result:

| Target | Fit |
|--------|-----|
| miniflux | 4/8 |
| pocketbase | 3/8 |
| caddy | 2/8 |
| listmonk | 2/8 |
| gitea | 1/8 |
| mattermost | 1/8 |

Five universal failures emerged — wiring never lives in `main`; services are
concrete structs, not interfaces; multi-implementer is common (plugin systems,
multiple auth providers); method shapes are heterogeneous (echo handlers, HTTP
handlers, domain-arg methods, builder chains); every target holds significant
in-process state (worker pools, caches, hubs, connection pools).

The v1 compiler could be patched indefinitely without ever becoming applicable
to any of these targets, because the v1 *contract itself* is wrong — not the
implementation against it. The demo fits because the demo was written to the
compiler; real code isn't.

## Decision

Fix the contract before writing more compiler code.

- Produce a versioned v2 contract specification at `docs/specs/monolift-v2-contract.md`.
- Use a structured planning process (multi-model sprint-planner + Opus merge) to define the sprint; execute with codex. See `docs/sprints/SPRINT-0003.md`.
- The v2 spec must resolve seven design axes: annotation surface, extraction root, state semantics, transport, dispatch granularity, multi-implementer handling, pragma syntax.
- The v2 spec is quality-first: validated against all six evaluation targets before close, with pocketbase as the intentional negative case.

This decision is meta — it commits to renegotiation. The specific technical
answers are sub-decisions in ADR-0003 through ADR-0008.

## Consequences

- All v1 compiler code freezes for compiler-development purposes until v2 lands.
  Runtime/demo work can proceed on the existing monolith.
- The "Monolift compiler roadmap" becomes: SPRINT-0003 (spec) → SPRINT-0005 (implementation of new spec) → evaluation against targets.
- The PLOS '25 paper's conceptual model is not preserved wholesale; see ADR-0009.
- Accepts upfront that the v2 spec will deliberately *refuse* some targets
  (pocketbase) rather than pretending a universal solution exists.

## References

- `docs/evaluation/generalization-analysis-2026-04-19.md` — the audit evidence that forced this decision.
- `docs/sprints/SPRINT-0003.md` — sprint plan that produces the v2 spec.
- `docs/specs/monolift-v2-contract.md` — the v2 spec itself (in review as of date of this ADR).
- ADR-0003 … ADR-0009 — the concrete technical decisions inside v2.
