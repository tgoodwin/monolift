# Target Annotation: Mattermost

## Synthesis
- **Dominant archetypes**: Singleton Actor (App/Server), Sharded Stateful Service (Cache), Worker Pool (Scheduler/Jobs).
- **The AUTO set**:
    - `server/platform/services/cache`: Sharded Stateful Service.
    - `server/channels/jobs` (or scheduler): Worker Pool.
- **Hardest ambiguities**:
    - `server/channels/app`: Decomposing the "God Object" into smaller distributable services. Singleton Actor is the current fallback.
- **Most important evidence gaps**:
    - Key-only access verification for caches (no global scans).
    - Serializability of job payloads.

---

## Annotations

### Subsystem: Core Monolith (app)
- **owned directories**: `evaluation/mattermost/server/channels/app`
- **region or operation identity**: package `app` / `App` struct
- **admitted or refused**: REFUSED
- **triage**: SUGGEST
- **proposed archetype**: Singleton Actor
- **proposed candidate state class**: `singleton-mutable-monolith`
- **proposed transform**: Service owning App instance; serialized gRPC interface.
- **competing archetypes**: Sharded Stateful Service (rejected due to deep state complexity).
- **evidence signals seen**: Extensive `sync.Mutex` usage; centralized service registry.
- **missing evidence**: Request isolation proof for stateless replication.
- **file references**: `evaluation/mattermost/server/channels/app/app.go`

### Subsystem: Keyed Cache
- **owned directories**: `evaluation/mattermost/server/platform/services/cache`
- **region or operation identity**: package `cache` / `Cache` interface
- **admitted or refused**: REFUSED
- **triage**: AUTO
- **proposed archetype**: Sharded Stateful Service
- **proposed candidate state class**: `singleton-mutable-sharded`
- **proposed transform**: Shard keys across multiple cache-service replicas.
- **competing archetypes**: Singleton Actor.
- **evidence signals seen**: `Get(key)`, `Set(key, value)` access patterns.
- **missing evidence**: Proof of no 'Keys()' operations needing global sync.
- **file references**: `evaluation/mattermost/server/platform/services/cache/cache.go`

### Subsystem: Job Scheduler
- **owned directories**: `evaluation/mattermost/server/channels/jobs`
- **region or operation identity**: package `jobs` / `Scheduler` interface
- **admitted or refused**: REFUSED
- **triage**: AUTO
- **proposed archetype**: Worker Pool / Queue Consumer
- **proposed candidate state class**: `worker-pool-scheduler`
- **proposed transform**: Broker-backed queue feeding worker replicas.
- **competing archetypes**: Singleton Actor.
- **evidence signals seen**: Goroutine pools; decoupled task execution.
- **missing evidence**: Job parameter serializability check.
- **file references**: `evaluation/mattermost/server/channels/jobs/scheduler.go`
