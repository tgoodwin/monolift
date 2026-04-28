# Target Annotation: Gitea

## Synthesis
- **Dominant archetypes**: Worker Pool (Queue/Webhook), Sharded Stateful Service (Cache), Singleton Actor (Registry/Events), Ephemeral Worker (Tasks).
- **The AUTO set**:
    - `modules/queue`: Worker Pool.
    - `modules/cache`: Sharded Stateful Service.
    - `services/webhook`: Worker Pool.
- **Hardest ambiguities**:
    - `services/task`: Long-running migrations might have complex side effects that make "Ephemeral Worker" risky without manual verification.
- **Most important evidence gaps**:
    - Non-serializable closure capture in workers.

---

## Annotations

### Subsystem: Background Queues
- **owned directories**: `evaluation/gitea/modules/queue/`
- **region or operation identity**: package `queue` / `WorkerPool` struct
- **admitted or refused**: REFUSED
- **triage**: AUTO
- **proposed archetype**: Worker Pool / Queue Consumer
- **proposed candidate state class**: `worker-pool-channel`
- **proposed transform**: Broker-backed worker replicas.
- **competing archetypes**: Pipeline Stage.
- **evidence signals seen**: `workergroup.go` (pool management), `workerqueue.go` (serialization).
- **missing evidence**: Final check for non-serializable closure capture.
- **file references**: `evaluation/gitea/modules/queue/workerqueue.go`

### Subsystem: Keyed Cache
- **owned directories**: `evaluation/gitea/modules/cache/`
- **region or operation identity**: package `cache` / `Cache` interface
- **admitted or refused**: REFUSED
- **triage**: AUTO
- **proposed archetype**: Sharded Stateful Service
- **proposed candidate state class**: `singleton-mutable-sharded`
- **proposed transform**: Deploy as a sharded service with key-based affinity or externalize to Redis.
- **competing archetypes**: Singleton Actor.
- **evidence signals seen**: Keyed access via `Get`, `Put` in `cache.go`.
- **missing evidence**: None.
- **file references**: `evaluation/gitea/modules/cache/cache.go`

### Subsystem: Webhook Delivery
- **owned directories**: `evaluation/gitea/services/webhook/`
- **region or operation identity**: `deliver.go` / `WebhookWorker`
- **admitted or refused**: REFUSED
- **triage**: AUTO
- **proposed archetype**: Worker Pool / Queue Consumer
- **proposed candidate state class**: `worker-pool-webhook`
- **proposed transform**: Decouple as a dedicated subscriber service.
- **competing archetypes**: Ephemeral Worker.
- **evidence signals seen**: Asynchronous delivery pattern; worker loops.
- **missing evidence**: None.
- **file references**: `evaluation/gitea/services/webhook/deliver.go`

### Subsystem: Long-running Tasks
- **owned directories**: `evaluation/gitea/services/task/`
- **region or operation identity**: `task.go` / `RunMigration`
- **admitted or refused**: REFUSED
- **triage**: SUGGEST
- **proposed archetype**: Ephemeral Worker
- **proposed candidate state class**: `ephemeral-task`
- **proposed transform**: Run as K8s Job.
- **competing archetypes**: Singleton Actor.
- **evidence signals seen**: Discrete units of work.
- **missing evidence**: Proof of idempotency for migrations.
- **file references**: `evaluation/gitea/services/task/task.go`
