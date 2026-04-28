# Usefulness scenarios v1 — SPRINT-0015 (Gemini run)

This document explores the workload shapes and code structures that determine the utility of Monolift's distribution archetypes. Grounded in the project's thesis (PLOS '25), we define "utility" as the degree to which distribution improves scalability, fault tolerance, or isolation without introducing unacceptable costs in latency, complexity, or consistency.

## 1. The Breakeven Inequality of Monolift

The decision to lift a region is governed by a qualitative breakeven inequality:

`Benefit(Scaling + Isolation + Fault Tolerance) > Cost(Network Latency + Operational Complexity + Consistency Relaxation)`

Monolift's goal is to find regions where this inequality is strongly positive.

## 2. Workload Shapes that Favor Lifting

### 2.1 The "Heavy" Background Task
Regions that perform CPU-intensive work or wait on slow I/O (e.g., Miniflux feed fetchers, Gitea git operations) are prime candidates. Lifting these prevents "noisy neighbor" effects where a background burst degrades the primary request-response path.
- **Archetypes:** `bounded-worker-pool`, `periodic-invocation`.
- **Structural Tell:** Lack of a synchronous return path to the end-user.

### 2.2 The Shardable State
Workloads that manage large amounts of data that can be partitioned by a stable key (e.g., Listmonk subscribers, Caddy connections) benefit from horizontal scaling. Lifting these allows the application to outgrow the memory/CPU limits of a single node.
- **Archetypes:** `keyed-partitioned-state`, `session-affinity-state`.
- **Structural Tell:** Every access site uses a key derived from the request or connection.

### 2.3 The Ephemeral Cache
High-read, low-write data (e.g., auth tokens, session metadata) can be offloaded to managed infrastructure. This reduces memory pressure on the monolith and allows for faster restarts (as the cache persists).
- **Archetypes:** `ttl-cache`.
- **Structural Tell:** A "get-or-load" pattern where data is non-authoritative.

## 3. Scenarios where Lifting is a Wash or Loss

### 3.1 The Synchronous Hot Path
In high-frequency, low-latency code (e.g., Caddy's reverse-proxy packet routing), adding even a few hundred microseconds of network RTT for a remote call can be a net-negative. These regions should remain in the `replicated-stateless-service` baseline or be handled with extreme care.
- **Example:** Caddy reverseproxy Handler.

### 3.2 Trivial Coordination
Lifting a simple `sync.Mutex` that protects a single integer counter (e.g., a "total requests" metric) into a distributed actor is almost always a loss. The overhead of the distributed machinery far exceeds the benefit of "scalability" for such a small state.
- **Example:** Internal heartbeats and trivial metrics.

### 3.3 The Tightly Coupled "God Object"
Lifting a central coordination object (e.g., Pocketbase `core.App`) as a single actor creates a distributed bottleneck. While it might be technically "liftable," it centralizes the entire application's contention onto a single network endpoint, often resulting in worse performance than the original monolith.

## 4. Operational Complexity as a Utility Ceiling

Lifting introduces "Operational Debt." Each transform requires new infrastructure (Brokers, Redis, S3, CronJobs). For small-scale deployments, this debt may never be repaid by the scaling benefits. Monolift is most useful for applications that have outgrown "single-node VPS" but don't want the cognitive load of a full microservices rewrite.

## 5. Consistency Trade-offs

Distribution often forces a move from strong to eventual consistency.
- **Acceptable:** Subscriber counts, background cleanup, feed updates.
- **Unacceptable:** Financial transactions, distributed state machines with strict ordering (Gitea `graceful.Manager`), security-critical access control lists that must be atomic across all nodes.

## 6. Summary of Utility by Archetype

| Archetype | Utility Anchor | Risk Factor |
|---|---|---|
| `serialized-actor` | Entity isolation | Global bottleneck |
| `bounded-worker-pool` | Throughput / Burst handling | Broker complexity |
| `periodic-invocation` | Reconciliation / Cleanup | Missed ticks |
| `keyed-partitioned-state` | Horizontal scaling | Cross-shard operations |
| `fanout-publisher` | System decoupling | Delivery guarantees |
| `ttl-cache` | Read-path optimization | Cache invalidation |
| `session-affinity-state` | Protocol state (WS) | Load imbalance |
| `filesystem-bound-singleton` | Cloud-native storage | Network RTT to disk |
