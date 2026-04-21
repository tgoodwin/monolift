# ADR-0011: Build the e2e harness before the v2 compiler

**Status:** accepted
**Date:** 2026-04-19
**Context docs:** `docs/specs/e2e-test-strategy.md`, `docs/sprints/SPRINT-0004.md`

## Context

SPRINT-0003 landed the v2 contract spec at v1.0. The natural next step is
SPRINT-0005+ — implement the v2 compiler epics (SSA extraction, canonical
shapes, state-class inference, pragma parser, refusal-diagnostic framework,
miniflux smoke). But a coding agent working on those epics needs a concrete,
reliable feedback loop: does my change to the canonical-shape classifier
produce the expected closure report for Caddy? Does it correctly emit the
refusal diagnostic for Pocketbase? Without that loop, agents write compiler
code blind and hope.

The e2e test strategy (`docs/specs/e2e-test-strategy.md`) specifies the
harness. But the strategy is not the harness — it's the spec. Someone has to
build it.

## Decision

**Build the harness first, as its own sprint (SPRINT-0004), before any v2
compiler implementation starts.**

Three coupled commitments:

1. **Harness uses a stub compiler** (`test/e2e/stubcompiler/`) that emits
   hard-coded golden closure reports matching the shape the real v2 compiler
   *will* eventually emit. This lets the harness go green before any
   compiler code exists.

2. **Real compiler replaces the stub target-by-target.** Each SPRINT-0005+
   epic removes one target's dependence on the stub. When all active
   targets run against the real compiler, the stub is deleted.

3. **Harness-before-compiler is a process invariant for this project.**
   Future spec revisions (v2.1, v3, …) that change the closure-report
   schema or add new verdicts must update the harness *first* with stub
   fixtures showing the new expected shape, then compiler work follows.
   The harness is the contract; the compiler is the implementation.

## Consequences

- **SPRINT-0004 has no compiler code.** Scope is pure test infrastructure:
  Kind lifecycle, `test/e2e/harness/*` Go scaffold, Caddy baseline + workload
  green, Pocketbase refusal green, Miniflux scaffolded, 3 other targets
  declared-and-skipped.

- **SPRINT-0005+ agents get a concrete feedback loop.** They run `make e2e`,
  see a red target, edit compiler code, re-run, see the stub-source entry
  shrink by one. Each epic produces visible progress.

- **Closure-report Go struct lives in `pkg/compiler/reportv2/`.** The
  harness imports it; the compiler (once built) emits it. Shared type =
  no schema drift.

- **JSON Schema validates the report in CI.** The Go struct is the
  practical type; the JSON Schema is the normative shape. Diverging
  between them is a CI failure.

- **Stub compiler is test-only code** (`test/e2e/stubcompiler/`, not `pkg/`).
  Never ships; exists for harness-before-compiler sequencing only.

- **Deliberate overhead.** Building the harness before the compiler means
  SPRINT-0004 doesn't itself ship a user-visible improvement. The payoff
  is that SPRINT-0005+ moves faster and with less risk.

- **Ledger scoping.** The sprint-planner skill's ledger is anchored on the
  nearest `.git` ancestor of CWD, so each project gets its own
  `docs/sprints/ledger.yaml` with an independent counter. Monolift's sequence
  is SPRINT-0003 → SPRINT-0004 (harness) → SPRINT-0005+ (compiler epics)
  with no cross-project collisions.

## References

- `docs/specs/e2e-test-strategy.md` — the harness strategy this sprint implements.
- `docs/sprints/SPRINT-0004.md` — the harness sprint plan.
- `docs/sprints/SPRINT-0003.md` §Follow-on sprints — seed list of SPRINT-0005+ compiler epics.
- ADR-0010 (spec-review triage) — sibling process discipline.
