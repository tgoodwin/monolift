# ADR-0009: Treat PLOS '25 conceptual-model claims as preserve / revise / retire

**Status:** accepted
**Date:** 2026-04-19
**Context docs:** `docs/specs/monolift-v2-contract.md` §Conceptual-Model Baseline, `research/monolift_PLOS.pdf`, `docs/sprints/SPRINT-0003.md` §"PLOS '25 revisions in play"

## Context

The v1 compiler was designed alongside the PLOS '25 paper. Several of the
paper's load-bearing claims turned out to be overfit to the demo app (the
audit shows this explicitly — see ADR-0002). But the paper is a published
artifact and a recognizable prior version of the design; wholesale retirement
would discard useful framing (delegate expressions, pay-as-you-go, lift points).

The renegotiation (ADR-0002) needs a clear principle for how to relate the
v2 spec to the paper.

## Decision

Audit each load-bearing PLOS '25 claim and tag it as one of:

- **preserve** — still load-bearing and correct; v2 retains it unchanged.
- **revise** — the shape is right, but the details change in v2.
- **retire** — the claim was wrong or overfit; v2 explicitly contradicts it.

Initial pre-dispositions (to be confirmed or rejected by the v2 spec v1.0):

| Paper claim | Disposition |
|-------------|-------------|
| Pay-as-you-go, monolith still runs uncompiled | **preserve** — the single non-negotiable invariant |
| Lifts are stateless | **revise** — see ADR-0005 (bounded state taxonomy) |
| Annotation site is an interface | **revise** — see ADR-0004 (generalized surface) |
| Wiring lives in `main` | **retire** — see ADR-0003 (call-graph extraction) |
| Dual dispatch at interface granularity | **revise** — dispatch point is the lift-point expression; granularity depends on annotation surface |
| Bounded lift model (lifts don't absorb the whole app) | **preserve** — v2 adds explicit "closure too large" refusal to enforce it |
| Kubernetes as one backend | **preserve** — reserve extension points for other backends without committing to them |

Every change from paper → v2 must be annotated inline in the v2 spec with a
one-line rationale. No silent retirements.

## Consequences

- The v2 spec is a principled revision of the paper's model, not a greenfield
  redesign — useful for a PLOS follow-up narrative.
- Future ADRs that change the v2 spec itself can cite back to this framing:
  "preserve, revise, retire — which is this?"
- Readers of the paper can bring their mental model to v2 and know exactly
  what's changed and why.
- Forces honesty: claims that reviewers of the original paper might have
  waved through get re-justified against audit evidence.

## References

- `research/monolift_PLOS.pdf` — PLOS '25 paper.
- `research/RESEARCH_BRIEF.md` — ongoing research context.
- `docs/specs/monolift-v2-contract.md` §Conceptual-Model Baseline.
- ADR-0002 (renegotiate contract) — parent decision.
