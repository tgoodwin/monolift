# Distribution Archetype Catalog (v1)

This catalog defines the distribution archetypes discovered and validated during SPRINT-0013.

## Vocabulary Discipline Rules (Phase 4 Results)

| Archetype | Coverage Gate | Evidence Gate | Emission Gate | Boundary Gate | Final Verdict |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **Singleton Actor** | PASS (Gitea, MM, Caddy, PB) | PASS (`sync.Mutex` + global) | PASS | PASS | **PROMOTED** |
| **Worker Pool** | PASS (Gitea, MM, Flux, Monk) | PASS (`chan` + workers) | PASS | PASS | **PROMOTED** |
| **Scheduled Invocation** | PASS (Gitea) | PASS (`gocron`) | PASS | PASS | **PROMOTED** |
| **Distributed Cache** | PASS (Gitea, MM) | PASS (`StringCache` + adapters) | PASS | PASS | **PROMOTED** |
| **Event-Bus / Pub-Sub** | PASS (Gitea, MM) | PASS (Topics + `Messenger`) | PASS | PASS | **PROMOTED** |
| **Pipeline Stage** | FAIL (Insufficient data) | - | - | - | **RETIRED** |
| **Sharded Service** | FAIL (Insufficient data) | - | - | - | **RETIRED** |

---

## Validated Archetypes

### Singleton Actor
- **Definition**: A stateful object with serialized access (usually via a mutex or single-threaded event loop) managing a global lifecycle or resource.
- **Transform**: Emit a service that owns the instance; serialize access at the request handler (wire-level serialization replaces in-process lock).
- **Evidence Conditions**: Single-instance construction; no cross-instance shared map; `sync.Mutex` or `sync.RWMutex` usage on receiver methods.
- **Candidate State Class**: `singleton-mutable`
- **Differentiating Signals**: `sync.Mutex`, `sync.RWMutex`, `sync.Once`.

### Worker Pool / Queue Consumer
- **Definition**: A set of workers consuming from a shared channel or queue.
- **Transform**: Broker-backed queue (Redis, NATS, SQS) feeding a pool of worker-service replicas.
- **Evidence Conditions**: Jobs serializable; workers share no mutable state beyond the queue and external storage.
- **Candidate State Class**: `singleton-mutable` (the queue) / `stateless` (workers).
- **Differentiating Signals**: `chan` receive in a `for` loop, `go func()` worker pools, `sync.WaitGroup` for lifecycle.

### Scheduled Invocation
- **Definition**: Periodic background work triggered by a timer or cron spec.
- **Transform**: Cron-triggered serverless function or scheduled service job.
- **Evidence Conditions**: `doWork` is idempotent; no shared state across invocations.
- **Candidate State Class**: `stateless` or `externalized-durable`.
- **Differentiating Signals**: `gocron.Scheduler`, `time.Ticker` in a loop.

### Distributed Cache
- **Definition**: TTL-based lookup state with background expiry and network-aware adapters.
- **Transform**: External cache (Redis / memcached) with managed eviction.
- **Evidence Conditions**: Cache contents are serializable; adapters for Redis/Memcache already exist in the source.
- **Candidate State Class**: `process-local-cache` (local) -> `externalized-durable` (distributed).
- **Differentiating Signals**: `StringCache` interface, explicit adapter registration for Redis/Memcached.

### Event-Bus / Pub-Sub
- **Definition**: One producer distributing events to N independent subscribers via named topics or messengers.
- **Transform**: Managed pub/sub broker with subscriber services.
- **Evidence Conditions**: Subscribers are independent; event is serializable.
- **Candidate State Class**: `shared-mutable-across-callers`.
- **Differentiating Signals**: Named topics, `map[uid]*Messenger` for subscribers, `SendMessage` fan-out.

---

## Retired Archetypes

### Pipeline Stage
- **Why it didn't survive**: While present in theory, it was not clearly distinguishable from a simple Worker Pool in the sampled corpus. Most "pipelines" were just workers reading from a queue. No explicit multi-stage pure transforms were cited with enough evidence to warrant a distinct archetype.

### Sharded Service
- **Why it didn't survive**: No clear "affinity-routed" map access pattern was found that wasn't already covered by a Singleton Actor or a simple Map-under-lock (which we triage as Refused/Terminal for now).
