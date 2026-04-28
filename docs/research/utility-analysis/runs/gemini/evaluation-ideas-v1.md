# Evaluation ideas v1 — SPRINT-0015 (Gemini run)

This document outlines concrete evaluation and demonstration scenarios surfaced by the qualitative utility analysis. These scenarios are designed to showcase Monolift's value in real-world application contexts.

## 1. The "Miniflux Feed Storm" (Scalability & Isolation)

**Goal:** Demonstrate the power of `periodic-invocation` and `bounded-worker-pool` for handling bursty background work.
- **Scenario:** A Miniflux instance with 10,000 active RSS feeds.
- **Baseline (Monolith):** All feeds are fetched by the main process. During a "fetch storm," CPU/RAM spikes, and the web UI becomes unresponsive or the process OOMs.
- **Monolift Lifted:** `feedScheduler` is lifted as a Platform Scheduler (`periodic-invocation`), and `worker.Pool` is lifted as a Broker-backed Queue (`bounded-worker-pool`).
- **Outcome:** The web UI remains snappy. Fetchers scale out as independent pods on Kubernetes, handling the storm without affecting user experience. This is the "perfect demo" for Monolift.

## 2. The "Caddy Connection Hub" (Stateful Scaling)

**Goal:** Demonstrate `session-affinity-state` and `serialized-actor` for long-running stateful connections.
- **Scenario:** A Caddy-based reverse proxy managing thousands of hijacked WebSocket connections.
- **Baseline (Monolith):** All connection state is in a single global map. Scaling horizontally is impossible because connection state isn't shared.
- **Monolift Lifted:** `Handler.connections` is lifted as a `session-affinity-state` service with sticky routing.
- **Outcome:** Caddy can be replicated across a cluster. Each node handles a subset of connections, with the load balancer ensuring requests for a specific session reach the node holding that session's state.

## 3. "Listmonk Campaign Blast" (Decoupling & Throughput)

**Goal:** Demonstrate `fanout-publisher` and `bounded-worker-pool` for complex asynchronous workflows.
- **Scenario:** Sending an email campaign to 1 million subscribers.
- **Baseline (Monolith):** The campaign loop is a single long-running task. If the process restarts, the campaign state must be painstakingly recovered.
- **Monolift Lifted:** Campaign events are lifted via `fanout-publisher`. Mail delivery is handled by a `bounded-worker-pool`.
- **Outcome:** The campaign progress is decoupled from delivery. Subscribers (stats, logs, third-party hooks) can be added/removed without touching the core campaign logic. Delivery can be throttled or scaled independently of the main Listmonk UI.

## 4. "S3-backed Pocketbase" (Cloud-Native Persistence)

**Goal:** Demonstrate `filesystem-bound-singleton` for cloud migration.
- **Scenario:** A Pocketbase app that needs to scale beyond a single node but relies on local disk for uploaded files.
- **Baseline (Monolith):** Requires a shared network filesystem (NFS/EFS), which is complex and slow.
- **Monolift Lifted:** Filesystem operations in the storage layer are lifted as a `filesystem-bound-singleton` with an S3 adapter.
- **Outcome:** Pocketbase can run on multiple nodes (stateless app servers) while files are stored in a durable, scalable object store. This shows Monolift's utility in modernizing legacy disk-bound code.

## 5. "Gitea Multi-Tenant Sharding" (Massive Horizontal Scale)

**Goal:** Demonstrate `keyed-partitioned-state` for extreme scale.
- **Scenario:** A Gitea instance hosting 100,000 repositories.
- **Baseline (Monolith):** Repository metadata and caches are in global maps, leading to lock contention and memory exhaustion on a single large node.
- **Monolift Lifted:** Repository state maps are lifted as `keyed-partitioned-state` sharded by repository ID.
- **Outcome:** State is distributed across a cluster of stateful services. Lock contention is reduced by sharding, allowing Gitea to handle significantly higher concurrency.

## 6. Benchmarking "Utility Breakeven"

**Goal:** Quantify the "net-negative" threshold for different archetypes.
- **Benchmark:** Create a synthetic workload that varies task "heaviness" (CPU/IO) and "frequency."
- **Measurement:** Find the point where the network RTT of a lifted call exceeds the benefit of isolation.
- **Utility:** This would provide the first empirical data to back up the qualitative claims in this research, helping tune the compiler's `AUTO` vs `SUGGEST` thresholds.
