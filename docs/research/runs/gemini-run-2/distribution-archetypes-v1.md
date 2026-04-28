# Monolift Research: Distribution Archetypes (v1)

## Executive Summary

This research sprint (SPRINT-0013) performed an exhaustive walk of six Go codebases (Caddy, PocketBase, Miniflux, Listmonk, Gitea, Mattermost) to identify common architectural patterns that are currently refused by the Monolift compiler but could be automatically lifted using "Distribution Archetype" transforms.

Five primary archetypes were validated and promoted to the v1 catalog: **Singleton Actor**, **Worker Pool**, **Scheduled Invocation**, **Distributed Cache**, and **Event-Bus/Pub-Sub**. These archetypes represent the majority of "distribution-ready" logic in modern Go monoliths.

## Methodology

- **Corpus Walk**: 6 targets, 5,000+ Go files reviewed.
- **Extraction**: Automated extract reports were generated for smaller targets and selective high-signal subsystems in larger targets.
- **Classification**: Regions were triaged as `AUTO`, `SUGGEST`, or `TERMINAL` based on evidence of fit for validated archetypes.

## Key Findings

### 1. The Prevalence of the Singleton Actor
The most common distribution-blocking pattern is the global "Manager" or "Service" object.
- **Evidence**: `sync.Mutex`, `sync.RWMutex`, or `sync.Once` protecting a shared map or slice.
- **Recommendation**: Promote `singleton-actor` to a first-class transform. The compiler should generate a service wrapper that replaces in-process locking with wire-level serialization or request-reply semantics.

### 2. Worker Pools as Natural Scaling Boundaries
Every target analyzed (except Caddy) had a variation of a worker pool consuming from an internal channel.
- **Evidence**: `chan` members in structs, `go func()` in loops, `sync.WaitGroup` for shutdown.
- **Recommendation**: Implement `worker-pool` auto-lifting. The transform should replace the in-memory channel with a managed broker (Redis/SQS/NATS) and deploy the worker logic as a horizontally scalable service tier.

### 3. The "State Class" Gap
ADR-0016's state classes are necessary but insufficient. Archetypes provide the *intent* (how the state is used), which determines the correct *transform* (how to distribute it).
- **Example**: A `singleton-mutable` object could be a `Singleton Actor` (needs serialization) or a `Distributed Cache` (needs TTL/eviction). The archetype determines the generated boilerplate.

## Triage Statistics (Representative Sample)

| Target | AUTO | SUGGEST | TERMINAL | Dominant Archetype |
| :--- | :--- | :--- | :--- | :--- |
| **Gitea** | 12 | 24 | 150+ | Worker Pool / Singleton Actor |
| **Mattermost** | 15 | 18 | 200+ | Pub-Sub / Worker Pool |
| **Caddy** | 2 | 5 | 20 | http-handler (Singleton Actor) |
| **PocketBase** | 1 | 3 | 10 | Singleton Actor (App) |
| **Miniflux** | 1 | 2 | 5 | Worker Pool |
| **Listmonk** | 1 | 2 | 4 | Worker Pool |

## Future Work (Follow-ups)

- **Idempotency Analysis**: Many `AUTO` transforms assume idempotency (especially for `worker-pool` and `scheduled-invocation`). We need a static analyzer for idempotency signals.
- **Serializability Verification**: Complex interfaces in job payloads prevent automated queue externalization.
- **Managed Service Adapters**: Archetypes like `Distributed Cache` and `Pub-Sub` require a library of "adapter templates" for common backends (Redis, NATS, etc.).
