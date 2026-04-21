# ADR-0010: Multi-model fan-out review + Category A/B triage for major specs

**Status:** accepted
**Date:** 2026-04-19
**Context docs:** `docs/specs/reviews/compiler-review.md`, `docs/specs/reviews/systems-review.md`, `docs/sprints/SPRINT-0003.md`

## Context

The v2 contract spec reached content-complete draft (v0.1-draft) and needed
review before v1.0 bump. SPRINT-0003's Phase 11 required ≥1 compiler-facing
and ≥1 systems/research reviewer, but sourcing external humans with the right
context is slow for a solo researcher returning after a project gap. At the
same time, spec reviews routinely produce feedback that conflates two
different categories: things that change the compiler's contract (block
SPRINT-0005 implementation) and things that change how the spec reads as a
research artifact (block paper submission, not implementation). Treating them
as one list means either (a) every small research-narrative edit blocks the
version bump, or (b) contract-affecting issues get deferred because narrative
issues look louder.

## Decision

Two linked process commitments:

1. **Multi-model fan-out review for major design artifacts.** Extend the
   sprint-planner pattern (3 independent drafts from codex/claude/gemini +
   cross-critique + Opus merge) to reviewing, not just planning. Two parallel
   tracks — one compiler-implementer lens, one PL/systems-research lens — each
   producing a merged review doc at `docs/specs/reviews/`. Subagents that
   hit Bash-permission limits stand down; the parent runs the CLIs directly.
   Reviews supplement, not replace, the author's own manual audit.

2. **Category A / Category B triage.** Classify every review concern as:
   - **Category A — contract-affecting.** Changes what the compiler must
     accept or refuse. Two conforming compilers diverge without this edit.
     Blocks SPRINT-0005 implementation. Must land before spec version bumps
     above 1.0.
   - **Category B — research-narrative.** Thesis statements, prior-art
     citations, framing, editorial honesty, semantics appendices. Doesn't
     change the contract. Blocks paper submission, not implementation.

   Three concrete response paths:
   - **Option 2:** Category A only. Spec becomes buildable; Category B
     deferred to a pre-paper revision.
   - **Option 1a:** Category A + cheap editorial honesty edits (verdict
     downgrades, log replacement, decorative-table pruning). Adds ~3 edits
     for no downside.
   - **Option 1:** Category A + full Category B. Spec becomes both
     buildable and standalone-research-grade.

   **Recommended default is 1a** for version bumps during active compiler
   development. Option 1 is appropriate when a paper submission deadline is
   approaching.

## Consequences

- **Reviewer-role precedent:** AI-merged fan-out reviews count as ≥1
  compiler-facing and ≥1 systems/research-facing reviewer for sprint Phase-11
  gating, provided they cite the review files in the spec change log and
  supplement (not replace) the author's manual audit. External human reviewers
  remain welcome but not blocking.
- **Category B backlog is explicit.** For v2 contract v1.0, Category B items
  (thesis statement, Waldo-delta appendix, prior-art References, PLOS
  retirement table expansion, actor-rejection expansion, shadow-actor framing,
  semantics appendix) are documented in `docs/specs/reviews/systems-review.md`
  §B1/B3/B8 + §S13/S14/T1/T2, waiting on a pre-paper revision sprint.
- **Future spec revisions** have a clear triage discipline: extract edit list
  from merged reviews; split A vs B; pick Option 2 / 1a / 1 based on distance
  to next paper.
- **Permission caveat:** Opus subagents spawned via the Agent tool may not
  inherit Bash permissions for CLI invocations. When using this pattern,
  the parent session should run the CLI fan-out directly rather than
  delegating the orchestration to a subagent.

## References

- `docs/sprints/SPRINT-0003.md` — the sprint that first used this pattern,
  for both planning (drafts → critiques → merge in Phase 0–2) and reviewing
  (Phase 11).
- `docs/specs/reviews/compiler-review.md` / `systems-review.md` — the two
  merged reviews that produced the Category A / Category B split for v2 v1.0.
- `docs/sprints/drafts/SPRINT-0003-REVIEW-*.md` — the 12 draft+critique files
  that fed the merges.
- ADR-0002 — parent decision (renegotiate contract); this ADR records how the
  renegotiation's review cycle was executed.
