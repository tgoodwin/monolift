# Distribution Archetype Research (v1) - Gemini Run

## Summary of Findings

This research phase identifies five primary distribution archetypes that provide clear, automated paths for lifting currently "REFUSED" regions in the evaluation corpus.

### Key Finding: The AUTO Set
The following regions, currently refused by the Monolift compiler, can be moved to **AUTO-lift** status by applying the discovered archetypes:
1.  **Listmonk Core CRUD**: `internal/core.Core` -> **Singleton Actor**.
2.  **Listmonk Background**: `internal/manager.Manager` -> **Worker Pool**.
3.  **Miniflux Workers**: `internal/worker.Pool` -> **Worker Pool**.
4.  **PocketBase Realtime**: `tools/subscriptions.Broker` -> **Event-Bus Publisher** (Wait, I retired this in catalog... actually Broker is more of a Singleton Actor if not using a real bus).
5.  **Gitea Queues**: `modules/queue` -> **Worker Pool**.
6.  **Gitea/Mattermost Caches**: `modules/cache` -> **Sharded Stateful Service**.

### Candidate State Classes for ADR-0016
To implement these archetypes, we propose adding the following state classes:
- `singleton-mutable-mutex`: For mutex-guarded components like Listmonk Core.
- `singleton-mutable-sharded`: For keyed components like Gitea Cache.
- `worker-pool-channel`: For components using Go channels for task distribution.
- `stateless-http`: Explicit label for ingress-compatible handlers.

### Vocabulary Gates and Retirements
- **Retired**: `Pipeline Stage`, `Event-Bus Publisher/Subscriber`, `Session-Scoped State`.
- **Reason**: Lack of clear distribution-specific transforms that weren't already covered by simpler archetypes or existing ADMITTED status.

---

## Target Syntheses

### Listmonk
Dominant archetypes are Singleton Actor (Core) and Worker Pool (Manager). The primary blocker is `boundary.context-first` violations, which can be mitigated by auto-injecting contexts in the generated service.

### Caddy
Most HTTP-related modules are already admitted. The remaining core state (UsagePool) fits the Singleton Actor pattern perfectly.

### PocketBase
The "God Object" App is the hardest ambiguity. While it can be a Singleton Actor, its performance will suffer. Sharded Stateful Service is a strong candidate but requires better join-analysis evidence.

### Miniflux
Extremely clean architecture. Already largely ADMITTED or fits classic Worker Pool patterns.

### Gitea / Mattermost
Massive targets that confirm the scalability of the Worker Pool and Sharded Stateful Service archetypes for large-scale monoliths.
