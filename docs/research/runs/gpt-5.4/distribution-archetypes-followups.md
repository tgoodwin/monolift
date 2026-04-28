# Distribution Archetypes Follow-ups - gpt-5.4 run

## Candidate State-Class Additions for ADR-0016

### `queued-workset`

- enables: `queue-backed worker`
- evidence required: explicit queue boundary, serializable item type, worker body independent of hidden process-local state, retry/requeue semantics visible
- transform unlocked: broker-backed queue plus worker replicas
- earned in: Listmonk, Miniflux, Gitea, Mattermost

### `scheduled-reconciler`

- enables: `scheduled-reconciler`
- evidence required: explicit timer or cron loop, separated work body, tolerable duplicate or missed tick semantics
- transform unlocked: platform scheduler invoking the work body or a queue enqueuer
- earned in: Listmonk, PocketBase, Gitea, Mattermost

### `owned-mutable-singleton`

- enables: `serialized-singleton-owner`
- evidence required: one owner instance, narrow mutation surface, no load-bearing alias leakage, no requirement for raw address identity
- transform unlocked: single remote owner service with serialized request handling
- earned in: Listmonk strongly; Miniflux, PocketBase, Gitea, and Mattermost as boundary pressure

### `connection-hub-buffer`

- enables: `connection-hub-buffer`
- evidence required: explicit routing key, register/unregister lifecycle, bounded fanout or replay buffer semantics, connection or subscriber ownership surface
- transform unlocked: sticky-owned hub service with externalized replay/fanout buffer
- earned in: PocketBase, Gitea, Mattermost, with Listmonk as the suggest boundary case

## ADRs Ripe to Draft

- `ADR-0019: archetype-driven remediation surface`
- `ADR-0020: auto-lift evidence thresholds`
- `ADR-0021: candidate state-class extensions for queued, scheduled, singleton-owned, and connection-hub state`
- `ADR-0022: pragma as additive evidence, not override, for archetype selection`

## Still-Open Empirical Questions

- Is `serialized-singleton-owner` worth an AUTO implementation immediately, or should it stay SUGGEST-first until alias and lifecycle evidence improves?
- Does `connection-hub-buffer` need one state class, or should replay-capable hubs and fire-and-forget fanout hubs split later?
- How often do queue-backed workers actually require stronger ordering guarantees than the source makes explicit?
- Should decorated persistence layers remain outside the archetype vocabulary, or is there a narrower persistence-projection transform hiding in large targets?

Current best characterization:

- `queue-backed worker` and `scheduled-reconciler` are ready for ADR work now.
- `connection-hub-buffer` is likely ready, but only if the ADR is explicit about sticky routing and replay semantics.
- `serialized-singleton-owner` is the most valuable but also the easiest place to overclaim.

## Classifier Evidence-Signal Gaps and Implementation Spikes

- Add a queue/worker evidence pass that recognizes bounded channels, queue managers, and requeue loops as one shape instead of separate refusals.
- Add a timer-loop recognizer that separates cadence from work body and records whether the work body is already factored.
- Add a single-owner analysis pass that can distinguish "one mutable owner behind a lock" from "shared mutable state across callers."
- Add explicit hub evidence for register/unregister, routing key, active queue, dead queue, and replay buffer.
- Investigate root narrowing support so terminal app roots like PocketBase can surface narrower AUTO regions without changing source.
- Tooling spike: make scratch `extract-report` corpus work tolerant of target-specific toolchain requirements, or pin a matching Go toolchain in research sessions.
- Tooling spike: harden subagent prompts for large-target fanout so path mistakes and non-terminating first passes are caught earlier.
