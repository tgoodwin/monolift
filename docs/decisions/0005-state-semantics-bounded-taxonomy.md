# ADR-0005: State semantics: bounded taxonomy replaces stateless-only lifts

**Status:** accepted _(v2 spec v1.0, 2026-04-19)_
**Date:** 2026-04-19
**Context docs:** `docs/specs/monolift-v2-contract.md` §State Semantics, `docs/evaluation/generalization-analysis-2026-04-19.md` §"Stateful services are the norm"

## Context

v1 assumed lifted services are stateless — any persistence happens via an
external store (the demo uses Dapr). The audit found that **every one of the
six real targets** holds meaningful in-process state:

- mattermost: WebSocket hubs, broadcast channels, caches, scheduled-task tables
- listmonk: campaign worker channels, template/link caches, rate-limit state
- gitea: mailer queue, session store, cache context
- caddy: per-module runtime state, connection pools
- pocketbase: embedded SQLite, subscriptions broker, hook registry
- miniflux: worker pool, proxy rotator singleton

Refusing to lift any of this rules out 5 / 6 targets outright. But lifting it
naively also fails — a replicated stateless service can't host a WebSocket
hub that needs sticky sessions.

## Decision

Adopt a **bounded state taxonomy** and classify each lift by state class:

| Class | Disposition |
|-------|-------------|
| stateless | replicated (N replicas, any can handle any call) |
| immutable-captured config | replicated (config travels with each replica) |
| externalized durable (DB/KV/Dapr) | replicated (state stays in external store) |
| process-local cache | replicated with local cache (may diverge across replicas) |
| singleton mutable (worker pool, hub, subscription broker) | **singleton deployable** — one instance, all callers share |
| connection/session state | **affinity-routed** — sticky routing per session key |
| shared mutable across unrelated callers | **refused** — requires consensus, out of v2 scope |

Classification may be inferred by the compiler from the closure (ADR-0003) or
declared explicitly in the pragma, with the explicit declaration winning.

Failure / cancellation / deadline / panic / zero-value semantics are specified
at the contract level — v2 does **not** claim remote invocation is
indistinguishable from local (honor Waldo).

## Consequences

- v2 can lift listmonk's campaign worker (singleton), mattermost's
  WebHub (affinity-routed or refused with rationale), miniflux's feed-fetch
  pool (singleton with internal replicas).
- Deliberately refuses to lift anything requiring consensus or cross-node
  shared mutable state — that's a distributed-systems problem, not a Monolift problem.
- The spec's singleton and affinity-routed classes require runtime support
  that v1 doesn't have — new ground for the v2 compiler implementation.
- Bounds the risk of drifting into a full actor/DSM system (Orleans/Akka scale).

## References

- `docs/specs/monolift-v2-contract.md` §State Semantics.
- `docs/evaluation/generalization-analysis-2026-04-19.md` §"Stateful services are the norm" (5/6 violations).
- ADR-0002 (renegotiate contract) — parent decision.
- ADR-0003 (call-graph extraction) — state classification reads from the closure.
