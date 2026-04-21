# Monolift Architecture Decision Records

Each file in this directory records one architectural decision — the context,
the decision itself, the consequences. Keep them **small, append-only, and
referenceable**. Supersede an old ADR by writing a new one; do not rewrite history.

## File naming

`NNNN-kebab-title.md`, zero-padded sequentially. Never reuse a number.

## Status values

| Status | Meaning |
|--------|---------|
| **accepted** | Decision stands; implementations should follow it. |
| **proposed** | Drafted but not yet committed (e.g., pending spec v1.0 review). |
| **superseded by ADR-NNNN** | Replaced by a later decision; kept for history. |
| **rejected** | Considered and declined. Kept because the reasoning is still useful. |

## Template

```markdown
# ADR-NNNN: Title

**Status:** accepted | proposed | superseded by ADR-MMMM | rejected
**Date:** YYYY-MM-DD
**Context docs:** path/to/supporting/docs

## Context
What's the situation that forced a decision?

## Decision
What did we decide, stated as plainly as possible?

## Consequences
What changes because of this decision — in the codebase, the research story,
or downstream decisions?

## References
- file paths and line numbers
- related ADRs
```

## Narrative

A chronological read-through of how decisions evolved — with each ADR placed
in story context — lives at `../evolution.md`. The ADRs are the primary
sources; `evolution.md` is the reader's entry point.

## Spec-review triage

When reviewing or revising a major spec, use the Category A / Category B
discipline defined in [ADR-0010](0010-spec-review-triage.md) to prioritize
contract-affecting changes over research-narrative changes.

## Discipline

- **One decision per file.** If a single choice has multiple consequences, that's
  fine; if two independent choices are being made, write two ADRs.
- **New decision → new ADR → one paragraph appended to `evolution.md`.** Keep
  the narrative in sync as decisions land.
- **Don't rewrite an ADR to reflect a later change.** Write a new ADR with
  status `superseded by ADR-NNNN` back-annotated on the old one.
