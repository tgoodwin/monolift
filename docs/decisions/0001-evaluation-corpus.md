# ADR-0001: Adopt real-world Go monolith corpus for compiler evaluation

**Status:** accepted
**Date:** 2026-04-19 _(retroactive)_
**Context docs:** `docs/evaluation/README.md`, `evaluation/MANIFEST.yaml`

## Context

Monolift's v1 compiler was developed against a single test application at
`demo/monolith/` — a greenfield social-network backend built *to the compiler's
conventions*. After ~9 months of project inactivity, the obsidian "Monolift"
master doc listed six candidate real-world Go monoliths as evaluation targets:
gitea, mattermost, caddy, listmonk, pocketbase, miniflux.

Without a second corpus of targets, the compiler risked being overfit to the
demo's shape — interface-annotated services, `New<Iface>` constructors,
wiring-in-main, stateless services. Real Go monoliths almost certainly violate
several of these, but without local clones and a structured way to study them,
we'd never know which assumptions were load-bearing and which were accidental.

## Decision

Adopt the six targets as a **versioned evaluation corpus**:

- Clones live in `evaluation/` (gitignored, like `inspiration/`), one subdirectory per target.
- `evaluation/MANIFEST.yaml` (committed) pins upstream URL + commit SHA for each target so the corpus is reproducible across machines.
- `docs/evaluation/` (committed) holds the semantic index: `README.md` target matrix, one `targets/NN-<name>.md` dossier per target, and dated `experiments/YYYY-MM-DD-*.md` notes.
- Dossier structure: Snapshot, Architecture notes, Lift candidates, Experiments table, Compiler-capability gaps, Blockers/open questions.

This mirrors the pre-existing `inspiration/` + `docs/research/index/` pattern
(gitignored raw assets + committed semantic index).

## Consequences

- Enables ADR-0002 (renegotiate the compiler contract): the corpus is what
  generated the evidence that the v1 contract was overfit.
- Establishes a reusable pattern for *any* "locally available related assets"
  with a committed semantic index — already applied to `inspiration/`, now to
  `evaluation/`.
- Compiler-capability gaps surface structurally (one section per target), so
  gap evidence accumulates and is citeable.
- Future targets can be added without restructuring: append to MANIFEST and
  add a new dossier.

## References

- `.gitignore:4-6` — `inspiration/`, `evaluation/`, `docs/sprints/drafts/` ignored.
- `evaluation/MANIFEST.yaml` — pinned SHAs as of 2026-04-19.
- `docs/evaluation/README.md` — target matrix.
- `docs/evaluation/generalization-analysis-2026-04-19.md` — the cross-target audit enabled by this corpus.
