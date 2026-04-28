# Per-archetype utility cards — SPRINT-0015 (Gemini run)

This document provides qualitative utility cards for the eight archetypes identified in the v1 catalog. Each card evaluates when lifting an archetype produces value, when it is detrimental, and identifies the structural markers that correlate with these outcomes.

---

## 1. `serialized-actor`

**Pays off when:**
- Encapsulated state is accessed concurrently but requires strict serialization to maintain invariants.
- Workloads involve "entity-scoped" state (e.g., a single long-running connection's metadata, a specific hardware handle).
- Lifting enables independent failure domains for individual actors, preventing a crash in one from affecting others.

**Net-negative when:**
- State is a "God Object" or global bottleneck where every request must contend for a single serial mailbox (e.g., Caddy's main `Handler` if treated as a single actor).
- Latency-sensitive hot paths where RPC round-trip overhead to the actor exceeds the critical path budget.
- Read-heavy workloads that could be served from replicas with eventual consistency.

**Code-structural tells:**
- `struct` with a `sync.Mutex` guarding almost all fields.
- Methods are "command-like" (mutating state) rather than "query-like" (read-only).
- State lifetime is often tied to an external resource (connection, file).

**New failure modes introduced:**
- Mailbox overflow / backpressure.
- Actor death vs. caller timeout.
- Deadlocks shifted to the distributed layer (e.g., actor A calling actor B).

**Operational complexity added:**
- Actor lifecycle management (restart, placement).
- Monitoring mailbox depth.

**Consistency/ordering trade-offs:**
- Strong consistency per actor; no cross-actor linearization guarantees.

**Corpus regions (Plausibly useful):**
- **Caddy C5/C10/C11**: `Handler.connections` and `connectionsMu`. Moving connection tracking to an actor allows the reverse proxy to scale while keeping connection state consistent.
- **Miniflux M6**: `ProxyRotator`. A singleton managing proxy rotation is a natural actor; lifting it isolates rotation logic from the fetchers.

**Corpus regions (Not useful):**
- **Pocketbase P1/P5**: If `core.App` is lifted as a single actor, it becomes the bottleneck for the entire application, negating the benefits of distribution.

---

## 2. `bounded-worker-pool`

**Pays off when:**
- Work is bursty and "heavy" (CPU intensive or I/O bound with long tail latency).
- Jobs are independent and idempotent (retryable).
- Throughput is more important than immediate latency (e.g., background email sending, image processing).

**Net-negative when:**
- Trivial, low-latency tasks where the overhead of broker publish/subscribe dominates.
- Strict FIFO ordering is required across all jobs (broker-backed queues often relax this or require complex sharding).

**Code-structural tells:**
- `chan T` used as a work queue.
- `for i := 0; i < N; i++ { go worker() }` pattern.
- Lack of immediate return value to the original caller (fire-and-forget).

**New failure modes introduced:**
- Poison pill messages (repeated retries failing).
- Broker unavailability.
- Worker cold starts.

**Operational complexity added:**
- External message broker (NATS, Redis, SQS).
- Dead-letter queue (DLQ) management.

**Consistency/ordering trade-offs:**
- At-least-once delivery (may result in duplicates).
- Per-key ordering usually lost unless explicitly handled.

**Corpus regions (Plausibly useful):**
- **Listmonk L2**: Mail delivery. A classic worker pool scenario where bursty delivery should not block the UI.
- **Gitea G1**: Background tasks/queues. Moving these to a dedicated worker set allows the main web process to remain responsive.

**Corpus regions (Not useful):**
- **Miniflux ADMITTED baseline**: Already well-separated; auto-lifting trivial worker pools where the work is essentially "just a DB write" might add unnecessary complexity.

---

## 3. `periodic-invocation`

**Pays off when:**
- Tasks are resource-intensive (e.g., full DB scan, expensive cleanup).
- Reconciliation logic that needs to run regardless of user activity.
- Lifting allows "drifting" the invocation time to off-peak hours or dedicated nodes.

**Net-negative when:**
- Trivial, high-frequency "watchdog" loops where network overhead to a platform scheduler is significant.
- Tasks that require sub-second precision (platform schedulers like K8s CronJobs are coarse-grained).

**Code-structural tells:**
- `time.Ticker` or `time.Sleep` in a `for` loop.
- Side-effect heavy body (cleanup, sync).
- No caller waiting for completion.

**New failure modes introduced:**
- Missed ticks (skipped execution).
- Overlapping executions (if the prior run didn't finish).

**Operational complexity added:**
- Platform scheduler configuration (K8s CronJob, etc.).
- Centralized logging for "hidden" background tasks.

**Consistency/ordering trade-offs:**
- Eventual consistency (reconciliation eventually fixes drift).
- No guarantees on exact execution time relative to other events.

**Corpus regions (Plausibly useful):**
- **Miniflux M1–M4**: `feedScheduler`, `cleanupScheduler`. These are independent, idempotent, and benefit from being moved out of the main process to avoid OOM or CPU spikes during fetch storms.
- **Caddy C2**: `keepStorageClean`. Periodic cleanup is a perfect candidate for an external scheduler.

**Corpus regions (Not useful):**
- **Pocketbase P2**: Trivial heartbeat-style loops that just update a memory-resident flag.

---

## 4. `keyed-partitioned-state`

**Pays off when:**
- State is large and naturally sharded by a unique key (e.g., User ID, Project ID).
- High-concurrency access patterns that would bottleneck on a single global lock.
- Multi-tenant isolation requirements (each shard can be its own failure domain).

**Net-negative when:**
- Frequent cross-key operations (e.g., "count all users where status='active'").
- Key distribution is highly skewed ("hot keys").

**Code-structural tells:**
- `map[K]V` protected by a `sync.RWMutex`.
- Access patterns always provide the key (SSA-visible).

**New failure modes introduced:**
- Partial availability (some shards up, others down).
- Resharding complexity.

**Operational complexity added:**
- Managed KV store (Redis Cluster, DynamoDB) or custom consistent-hashing proxy.
- Shard monitoring.

**Consistency/ordering trade-offs:**
- Strong consistency per key; no cross-key atomicity.

**Corpus regions (Plausibly useful):**
- **Listmonk L5**: Subscriber state. Sharding by subscriber ID allows scaling the mailing list management indefinitely.
- **Caddy C5 composite**: Connections map. Partitioning by connection ID allows high-throughput connection tracking.

**Corpus regions (Not useful):**
- **Mattermost MM1 composite**: If the map is used for global state machines that require cross-key invariants (e.g., total active user count).

---

## 5. `fanout-publisher`

**Pays off when:**
- Multiple independent systems need to react to the same event.
- Decoupling producer from consumer lifecycle.
- Asynchronous side effects (e.g., logging, analytics, webhooks).

**Net-negative when:**
- Strict ordering across subscribers is required.
- Low subscriber count where the cost of a broker outweighs direct calls.
- Synchronous feedback is needed from subscribers.

**Code-structural tells:**
- `[]chan T` or `map[K]chan T` for subscribers.
- A `Notify` or `Publish` method that iterates and sends.

**New failure modes introduced:**
- Backpressure from slow subscribers affecting others (depending on broker config).
- Lost events in non-durable brokers.

**Operational complexity added:**
- Pub/Sub broker (NATS, SNS, Kafka).
- Subscription management.

**Consistency/ordering trade-offs:**
- Eventual consistency across the system.
- Ordering guaranteed only per topic (if supported by broker).

**Corpus regions (Plausibly useful):**
- **Listmonk L4**: Campaign events. Multiple subscribers (stats, logs, third-party integrations) can consume campaign progress independently.
- **Gitea G7**: Webhooks. Moving fanout to a broker ensures slow webhooks don't block internal Gitea operations.

**Corpus regions (Not useful):**
- **Mattermost MM7 (ADMITTED)**: Internal event buses that are low-volume and high-frequency within a single node.

---

## 6. `ttl-cache`

**Pays off when:**
- High read-to-write ratio on data that can tolerate some staleness.
- Reducing load on a primary source of truth (DB, external API).
- Improving tail latency by serving from memory/local-cache.

**Net-negative when:**
- Data churn is high, leading to low hit rates.
- Strong consistency is required (cache invalidation is hard).
- Cache-stampede scenarios.

**Code-structural tells:**
- `map[K]V` with a TTL field or `sync.Map` with periodic eviction.
- "Get-or-load" pattern.

**New failure modes introduced:**
- Stale data usage.
- Cache-fill heavy load on DB during cold starts.

**Operational complexity added:**
- Managed cache (Redis, Memcached).
- Cache invalidation strategies.

**Consistency/ordering trade-offs:**
- eventual consistency.

**Corpus regions (Plausibly useful):**
- **Listmonk L6/L7**: Auth/Session cache. Moving this to a managed cache allows horizontal scaling of the API servers.
- **Mattermost MM4/MM5**: Session caches.

**Corpus regions (Not useful):**
- **Caddy C7 (overlap)**: Trivial caches for static config that rarely changes and is small enough to replicate everywhere.

---

## 7. `session-affinity-state`

**Pays off when:**
- Stateful protocols (WebSockets, long-polling) are used.
- Per-request state transfer is expensive (e.g., large session objects).
- Latency reduction by keeping state near the processing logic for the duration of a session.

**Net-negative when:**
- Session imbalance (some replicas overloaded, others idle).
- Sessions are short-lived, making the affinity overhead not worth it.
- Difficulty in handling replica failure (state loss).

**Code-structural tells:**
- State keyed by a Connection or Session ID.
- Access is serialized per session.

**New failure modes introduced:**
- Sticky session failures.
- Imbalanced load.

**Operational complexity added:**
- Session-affinity-aware load balancer (Sticky LB).
- Distributed session tracking.

**Consistency/ordering trade-offs:**
- Strong consistency per session; no guarantees across sessions.

**Corpus regions (Plausibly useful):**
- **Caddy C6**: Hijacked connections. Moving these to affinity-routed replicas preserves the "long connection" semantics.
- **Mattermost MM2**: WebSocket hub. Keeping a user's WebSocket session on a specific node reduces cross-node chatter for that user's events.

**Corpus regions (Not useful):**
- **Gitea G11/G12**: Trivial session cookies that could just be stored in a stateless JWT.

---

## 8. `filesystem-bound-singleton`

**Pays off when:**
- Scaling a monolith that relies on local disk for persistence or storage (e.g., uploaded avatars, local logs).
- Moving to cloud-native storage (S3/GCS) without rewriting the application logic.
- Ensuring durability across node restarts/replacements.

**Net-negative when:**
- High-frequency, low-latency small file writes where network RTT to an object store is a regression.
- Workloads relying on OS-level file locking or exact filesystem semantics (e.g., symlinks, hardlinks) not supported by object stores.

**Code-structural tells:**
- `os.File`, `os.Open`, `filepath.Join` in core logic.
- Structs holding path strings or file handles.

**New failure modes introduced:**
- Object store unavailability.
- Eventual consistency of some object stores.
- Path/Key mapping errors.

**Operational complexity added:**
- Object store (S3, etc.) or CSI volume mapping.
- Secret management for storage credentials.

**Consistency/ordering trade-offs:**
- varies by backend (S3 is eventually consistent for some operations).

**Corpus regions (Plausibly useful):**
- **Caddy filestorage**: Moving Caddy's certificate storage to an object store allows multiple Caddy instances to share the same TLS state.
- **Gitea local storage**: Moving Gitea's attachment/avatar storage to S3 is a common production requirement.

**Corpus regions (Not useful):**
- **Gitea process manager lock files**: These are meant for local process coordination and don't make sense to move to a distributed object store.
