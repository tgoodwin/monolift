# Archetype Catalog (v1)

**Status:** v1 — Promoted from SPRINT-0013 Research.

This catalog defines the distribution archetypes recognized by the Monolift research. Every archetype has passed the four discipline gates.

## Vocabulary Discipline Rules

Every archetype in this catalog has passed:
1.  **Coverage Gate**: Labeled ≥2 regions across ≥2 targets.
2.  **Evidence Gate**: Distinguishable by existing or plausible classifier signals.
3.  **Emission Gate**: A ≤30-line Go pseudocode emission sketch is writeable.
4.  **Boundary Gate**: Auto-lift / suggest / refuse threshold is stated in concrete evidence conditions.

---

## v1 Promoted Archetypes

### Replicated Stateless Service
- **Definition**: Request-scoped logic with no mutable state across calls; relies on externalized persistence (e.g., DB).
- **Transform**: Horizontal replication; shape-preserving `handler` or `http-json` transport.
- **Evidence**: `stateless` (ADR-0016), `externalized-durable` state, `boundary.context-first`.
- **Targets**: Gitea (API/Web handlers, Domain services), Mattermost (api4 handlers, App methods), Listmonk (App handlers).
- **Thresholds**:
  - **AUTO**: No local sync primitives; context-first; serializable arguments.
  - **SUGGEST**: Minimal local state (e.g., counters); non-standard context.

### Singleton Actor
- **Definition**: Stateful component requiring serialized access to local state or resources (e.g., local FS, config).
- **Transform**: Emit a singleton service; proxy calls via gRPC/NATS; optional leader election for exactly-once tasks.
- **Evidence**: `syncPrimitiveRule`, `os-side-effects`, `lifecycle.long-running-loop`.
- **Targets**: Caddy (reverseproxy Handler), Gitea (ProcessManager, LocalStorage), Mattermost (Config Store, TelemetryService).
- **Thresholds**:
  - **AUTO**: Clear ownership of a local resource (e.g., file path, mutex).
  - **SUGGEST**: Ambiguous ownership; state potentially sharding-ready.

### Worker Pool / Queue Consumer
- **Definition**: Goroutines consuming tasks from a channel or queue.
- **Transform**: Replace internal channel with a managed broker (Redis/NATS/SQS); lift workers into a scalable service.
- **Evidence**: `channelLoopRule`, `lifecycle.long-running-loop`.
- **Targets**: Listmonk (manager.worker), Miniflux (worker.Run), Gitea (modules/queue), Mattermost (JobServer).
- **Thresholds**:
  - **AUTO**: Single-channel loop; serializable task payload.
  - **SUGGEST**: Shared state among workers; complex fan-out.

### Event-Bus Publisher / Subscriber
- **Definition**: Decoupled event-driven interaction (Pub/Sub).
- **Transform**: Map internal event dispatch/hooks to a distributed message bus.
- **Evidence**: Registry pattern (slice of interfaces), `broadcast-event-bus` signal.
- **Targets**: Pocketbase (hooks), Gitea (services/notify, modules/eventsource), Mattermost (localcache invalidation).
- **Thresholds**:
  - **AUTO**: Clear event schema; independent subscribers.
  - **SUGGEST**: Subscribers with execution-order dependencies.

### Session-Scoped State (Request Affinity)
- **Definition**: State tied to a connection/session (e.g., WebSocket).
- **Transform**: Sticky-session routing or affinity-aware service; state carried in context.
- **Evidence**: `transport.websocket-boundary`, `keyed-request-context-state`.
- **Targets**: Gitea (reqctx.requestDataStore), Mattermost (wsapi, WebConn).
- **Thresholds**:
  - **AUTO**: Keyed by UserID/SessionID; serializable session snapshot.

### Ephemeral Worker
- **Definition**: Short-lived task spawned for a single heavy operation (e.g., search indexing, git GC).
- **Transform**: Serverless function or background job execution.
- **Evidence**: `goRoutineRule`, `lifecycle.no-async-fork`.
- **Targets**: Gitea (GitGcRepo), Mattermost (searchlayer indexPost).
- **Thresholds**:
  - **AUTO**: Side-effect only or return-via-channel; no long-lived state.

---

## Retired Archetypes

### Pipeline Stage
- **Reason**: Overlaps too heavily with "Worker Pool" and "Ephemeral Worker" in the current corpus. No clear examples found that weren't better modeled as a simple worker pool.

### Sharded Stateful Service
- **Reason**: While theoretically sound, true sharding (horizontal partitioning of state) was rarely seen as a first-class pattern in the monoliths walked; most "sharded" cases were better modeled as "Singleton Actors" with keyed access to an external DB.
