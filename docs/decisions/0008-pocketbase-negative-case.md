# ADR-0008: Pocketbase as intentional refusal — v2's lower bound

**Status:** accepted
**Date:** 2026-04-19
**Context docs:** `docs/evaluation/generalization-analysis-2026-04-19.md` target row for pocketbase, `docs/specs/monolift-v2-contract.md` §Refusal Diagnostics

## Context

Of the six evaluation targets, pocketbase scored 3/8 on the v1 contract —
superficially higher than gitea/mattermost/caddy/listmonk — but the audit
concluded pocketbase was the *least* liftable of the six. Its architecture:

- A single `core.App` interface with 190+ methods — a god object.
- An embedded SQLite datastore (`modernc.org/sqlite`, WAL mode) hardcoded into the App.
- 40+ lifecycle hook fields on `BaseApp`, all firing within a single process's
  transaction context.
- Feature initialization tied to `app.Settings()` at bootstrap time; no
  runtime re-injection.

Lifting *any* piece of pocketbase requires wholesale rewriting — extracting
the DB abstraction, breaking the App monolith into service interfaces, adding
an event bus, rewriting the hook system as external subscriptions. That's not
Monolift's problem to solve; that's a user-initiated refactoring project.

## Decision

Document pocketbase as an **intentional refusal** — the concrete lower bound
of what v2 can accept.

Refusal criteria named in the spec (paraphrasing the spec's refusal diagnostics):

1. **God-object interface** — a single interface with >N methods (exact N in spec) is refused because lift scope becomes unboundable.
2. **Embedded persistent state** — file-backed process-local stores (SQLite,
   Bolt, BadgerDB, …) that are structurally inseparable from the application
   are refused; no "lift a shard of the database" support.
3. **Bootstrap-time-only configuration** — features configured at startup with
   no runtime re-injection cannot be lifted without application-level refactoring.
4. **Monolithic lifecycle coupling** — features whose lifecycle is owned by a
   single root object and fires transactionally are refused.

The validation matrix in the v2 spec Phase 9 *blocks* on having a concrete
pocketbase refusal verdict. A vague "future work" verdict does not close the phase.

## Consequences

- Establishes that v2 is opinionated about what it won't do — useful for
  framing expectations in the PLOS follow-up paper.
- Provides a clear category of "needs refactoring first" that application
  developers can recognize without running the compiler.
- The four refusal criteria are reusable diagnostics: other targets hitting
  any one of them get the same refusal with the same rationale.
- If a future project wants to lift pocketbase-like architectures, it's a
  separate research effort, not an incremental Monolift enhancement.

## References

- `docs/evaluation/generalization-analysis-2026-04-19.md` — pocketbase row.
- `docs/evaluation/targets/05-pocketbase.md` — per-target dossier.
- `docs/specs/monolift-v2-contract.md` §Refusal Diagnostics, §Cross-target validation — pocketbase.
