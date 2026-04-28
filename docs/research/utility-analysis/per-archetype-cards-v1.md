# Per-archetype utility cards — composite SPRINT-0015

Cross-run composite. For each archetype: when lifting pays off (with corpus anchors), when it's net-negative (with anti-anchors), new failure modes, operational cost, consistency trade-offs, code-structural tells, and the archetype's fit to the PLOS §4.2 dynamic-placement thesis.

Per-run source detail at `runs/{opus,gpt-5.4,gemini}/per-archetype-cards-v1.md`. Opus's cards are the deepest (374 lines); the composite here synthesizes convergences and notes divergences rather than reproducing every detail.

---

## 1. `serialized-actor`

**Pays off when** the actor is a **coordinator** and single-ownership is the real semantic constraint — low-frequency coordination where the in-process mutex is already a known serialization point.
- **Corpus anchors (pays off):** gitea `queue.Manager` G4 (`evaluation/gitea/modules/queue/manager.go:18`), `eventsource.Manager` G6 (`manager.go:11`), `services/cron` task registry G15, `modules/process.Manager` G18 (`manager.go:70-71`); pocketbase `Hook[T]` P1 (`hook.go:55-57`); pocketbase `BatchHandler` P5; mattermost cluster-leader-listeners MM11.

**Net-negative when** the actor is on a **user-synchronous hot path** and the method is microsecond-level — RPC round-trip dominates the actor's own work cost.
- **Corpus anti-anchor:** miniflux `ProxyRotator` M6 (`evaluation/miniflux/internal/proxyrotator/proxyrotator.go:14`) — "pick next proxy" is called synchronously on every feed fetch; the proxy list is small and rarely changes; distributing this regresses every fetch.

**Code-structural tells for utility (within the archetype):**
- **Positive tell:** the actor's method is called from across packages (coordinator role) rather than from a single hot handler.
- **Positive tell:** method signatures accept request-scoped inputs rather than microsecond-fine primitives.
- **Negative tell:** the actor is reachable via SSA from an ADMITTED request handler without crossing an async boundary (opus's proposed utility heuristic).
- **Negative tell:** the owned state is tiny (single int counter, small slice) and access is frequent — gemini's "trivial coordination" anti-pattern.

**New failure modes.** Remote actor unavailability blocks all calls. Retries may double-apply mutations that were idempotent only because the mutex *was* the dedup. Structurally a single point of failure by its own semantics.

**Operational complexity.** One new service per actor instance; observability for actor-call latency; lifecycle management (the actor must outlive all callers). Gemini: "global bottleneck risk" if the actor is incorrectly identified as single-owner.

**Consistency / ordering trade-offs.** In-process mutex gave sequential consistency on the actor's state; RPC preserves it for a single actor instance but the caller's view of "observable order" across actors relaxes.

**PLOS §4.2 fit.** Mixed within the archetype. Coordinator-actors preserve local fast path cleanly (method call ↔ RPC, same signature); hot-path actors foreclose it.

**Composite disagreement.** All three runs converge on the bifurcation (coordinator useful, hot-path net-negative). None of the runs resolve the within-archetype distinction cleanly in the catalog's current vocabulary — flagged as the single most important utility-heuristic proposal for future sprint work.

---

## 2. `bounded-worker-pool`

**Pays off when** work is async-completable, per-job ordering is not user-visible, and the existing channel is already a coarse buffer between request ingress and background processing.
- **Corpus anchors (pays off):** listmonk `manager.worker` + `campMsgQ` L2 (`evaluation/listmonk/internal/manager/manager.go:462`) — campaign email delivery; gitea `WorkerPoolQueue` G1 (`evaluation/gitea/modules/queue/workergroup.go:92`); mattermost `PushNotificationsHub` MM6 (`notification_push.go:44-52`); miniflux `worker.Pool` (already ADMITTED — validates shape).

**Net-negative when** the caller is on a synchronous hot path expecting *"enqueue returned → the job is durably queued"* (broker-publish has latency and durability costs the channel doesn't), *or* per-key FIFO is load-bearing (broker dedup can't cheaply preserve it), *or* the pool is unbounded (fallback-spawn-on-full).
- **Corpus anti-anchors / borderline:** listmonk bounce subsystem — liftable but lower-payoff because the single callback-queue gives less new leverage than campaign or job runtimes. Pocketbase P6 JS-VM pool and P9 S3 uploader — SUGGEST-flagged in v1 for pool-unboundedness.

**Code-structural tells for utility.**
- **Positive tell:** explicit `chan T` field with `T` serializable; a static loop spawns N goroutines consuming it; handler is a named function (not inline closure).
- **Positive tell:** handler body already reads/writes external state (DB, API), so lifting doesn't break a local-state contract.
- **Negative tell:** pool size is growable on overflow (fallback-spawn); ordering is explicitly per-key FIFO.

**New failure modes.** Broker unavailability blocks enqueue. At-least-once delivery forces handlers to be idempotent (the in-process version did not have to be). Handler crashes may redeliver and duplicate visible effects.

**Operational complexity.** Broker infrastructure (NATS / SQS / Pub-Sub / Rabbit). Worker services need their own deployment. Observability now includes broker lag and DLQ.

**Consistency / ordering trade-offs.** Within-queue ordering → broker ordering semantics (typically: per-partition or per-consumer-group FIFO; not global). Acceptable for subscriber counts, background cleanup, feed updates; not acceptable for financial transactions.

**PLOS §4.2 fit: highest of any archetype.** All three runs converge. The enqueue call site is the single code signature where the §4.2 dynamic-offload story maps exactly: channel send ↔ broker publish at the same shape. Delegate DSL can express "offload when queue depth > N" naturally.

---

## 3. `periodic-invocation`

**Pays off when** the body is genuinely background maintenance — no request waits on it, duplicate or skipped ticks are tolerable, main application loses nothing by outsourcing the loop.
- **Corpus anchors:** caddy `stayUpdated` C1 (`sessiontickets.go:114-148`) — certificate housekeeping, skip-tolerant; caddy `keepStorageClean` C2; miniflux `feedScheduler` M1, `cleanupScheduler` M2, watchdog M3, metrics M4; listmonk `runMailboxScanner` L3; pocketbase `Cron` P2; gitea cron G14; mattermost `email_batching` MM8 (post-transform).

**Net-negative when** the "periodic" loop is actually a control loop with cross-tick state (backoff counter, self-tuning interval, watermark pointer reading from previous tick's local memory), or the loop has user-visible impact if it pauses / duplicates.
- **Corpus anti-anchor:** listmonk `scanCampaigns` L1 — fits the shape but is the primary control loop of listmonk's campaign distributor; pause/duplicate degrades user-facing behavior (campaigns firing late / twice).

**Code-structural tells.**
- **Positive tell:** `for { select { case <-ticker.C: body(); } }` or `for { time.Sleep(d); body(); }` — cadence cleanly separated from work body.
- **Positive tell:** body is named function, signature is context-only, no captured mutable state.
- **Negative tell:** interval derived from previous tick's result (self-tuning); body writes to captured mutable state that persists across ticks.

**New failure modes.** Scheduler partition (tick missed or delayed); cold-start on serverless scheduled triggers; tick overlap when previous tick runs long and scheduler fires the next one anyway.

**Operational complexity.** Platform scheduler infrastructure (cron, k8s CronJob, serverless trigger). Low incremental — most deployments already have schedulers.

**Consistency / ordering trade-offs.** Duplicate-or-skipped-tick tolerance required. Declared via `monolift:idempotent=true` pragma (load-bearing evidence per ADR-0021).

**PLOS §4.2 fit: high but subtle.** API cost is zero (`Start`/`Stop` become no-ops); local-fast-path preservation is trivial (cron is just running elsewhere). But there is no workload condition where running it remotely vs. locally affects *user-visible* latency — periodic ticks are not on the user hot path by definition. Utility is operational / resource-packing, which is real but thinner than the §4.2 headline.

**Cross-run disagreement.** Gemini ranks this #1 by utility (solves noisy-neighbor problem, minimal risk, cleanest transform — "perfect demonstration of utility"). Opus demotes to #4 (no user-visible latency story, so doesn't demonstrate §4.2). GPT-5.4 splits ("incremental-adoption hero, not flagship demo"). Composite view: **first implementation win, not the flagship thesis demo.** Ship it early because it's easy and exercises pragma infrastructure.

---

## 4. `keyed-partitioned-state`

**Pays off when** key space is naturally partitionable, per-key load is skewed enough that sharding relieves hot shards, no consumer depends on a global view across keys.
- **Corpus anchors (pays off):** mattermost `Hub.hubConnectionIndex` MM1 (composite with fanout-publisher + session-affinity-state) — per-user keying is operationally meaningful; caddy `Handler.connections` C5 (composite).

**Net-negative when** (a) iteration across all keys in hot path (listmonk L5 `Manager.pipes` — campaign manager iterates to dispatch; lifting turns O(shards) aggregation into a distributed operation), (b) map encodes cross-key invariants (sum-of-values-must-equal-X), (c) key cardinality is small enough that in-process mutex-map was already fine.
- **Corpus anti-anchor:** gitea `queue.baseChannel.set` G2 — small cardinality, uniqueness guard, broker dedup subsumes it entirely.

**Code-structural tells.**
- **Positive tell:** every access site indexes by a key derived from input (the proposed `keyed-access-invariant` classifier signal).
- **Negative tell:** `for k, v := range m { ... }` loops in hot paths; size- or sum-based aggregations over the map.

**New failure modes.** Shard routing mistakes (cross-shard writes silently go to the wrong partition). Shard rebalancing during operation. Loss of cross-shard iteration semantics.

**Operational complexity.** Consistent-hash router OR managed KV store (Redis Cluster, DynamoDB). New infrastructure if targeting KV; smaller if using routing over existing service replicas.

**Consistency / ordering trade-offs.** Per-key atomicity preserved; cross-key linearization is not (the original mutex-on-map already violated it if using per-key locks).

**PLOS §4.2 fit: mixed.** Managed-KV-target transform has low preservability (every access pays network). Consistent-hash-router transform has high preservability in principle but the runtime needs to know "when is sharding active vs. not" — not a shape the current delegate DSL handles.

**Utility concentrates in composites.** Standalone regions in the corpus are thinner than feasibility claimed. Strong case is inside composites like the mattermost hub.

---

## 5. `fanout-publisher`

**Pays off when** subscribers are already logically independent services that happen to co-reside with the producer for convenience.
- **Corpus anchors:** mattermost cluster `Publish` MM7 (already ADMITTED — validates shape); gitea `eventsource.Messenger` G7 (`messenger.go:9`) — server-sent events, per-subscriber independence near-total; pocketbase `Broker` P4 (`broker.go:11-65`); listmonk `Events.Publish` L4 (when subscribers are "write to log / emit metric" shape).

**Net-negative when** fanout encodes a distributed transaction (every subscriber must succeed before publish is considered done), or subscribers read back shared state that is *also* being lifted (creates read-your-writes issues across the broker boundary). Also net-negative when subscriber count is fixed-and-small and subscribers are tightly co-designed with the publisher — listmonk L4 is borderline because non-blocking send + queue-full behavior suggest local policy that broker semantics would obscure (gpt-5.4 anti-anchor).

**Code-structural tells.**
- **Positive tell:** `[]chan T` or `map[K]chan T` under mutex; `Publish` iterates and sends; `T` is serializable; subscriber-register entry point exists.
- **Negative tell:** Publish awaits acknowledgements from all subscribers before returning (distributed-transaction shape); subscriber set is tightly coupled to publisher lifetime.

**New failure modes.** At-least-once delivery forces consumer idempotency. Broker unavailability blocks publish (or drops events if async). Ordering across subscribers relaxes to broker ordering.

**Operational complexity.** Pub/sub broker (NATS / Kafka / Pub-Sub). Each subscriber becomes a service of its own.

**Consistency / ordering trade-offs.** Per-topic broker ordering, typically; no cross-topic ordering preserved.

**PLOS §4.2 fit: medium.** API preservation is clean (Publish/Subscribe don't change), but dynamic swap between local fanout and broker fanout is not cleanly expressible — subscribers are either in-process or on the broker, not both. Fits "pay-as-you-go for one decision" but not "runtime-adaptive."

**Low-independence warning.** Pulling all subscribers onto the broker at once is a large coordinated transform, not pay-as-you-go single-region.

---

## 6. `ttl-cache`

**Pays off when** the cache is genuinely shared read-only state (same value fetched by every replica under `replicated-stateless-service` baseline), value lookup is expensive, and the cache would otherwise rebuild per-replica.
- **Corpus anchors:** mattermost session cache MM4 / status cache MM5 (`evaluation/mattermost/server/channels/app/platform/session.go:45`) — sessions hit on every authenticated request; listmonk `Auth.apiUsers` L6 + prune loop; listmonk `tmptokens` L7.

**Net-negative when** cache is local memoization of a local computation (gitea `EphemeralCache` G10 — short-lived request-scoped calculation), in-process cache is the sole defense against source-of-truth latency (replacing with another network call is circular), or cache value carries function pointers / callbacks (compiler must reject).
- **Corpus anti-anchors:** gitea `EphemeralCache` G10; caddy `HTTPBasicAuth.Cache` C7 borderline (authentication lookups are user-visible, remote cache miss path is fragile).

**Code-structural tells.**
- **Positive tell:** value type is plain data (no pointers to in-process state); cache-miss loader pulls from external source of truth.
- **Negative tell:** value holds callbacks; cache is the source of truth (no loader); cache scope is single-request.

**New failure modes.** Managed cache unavailability amplifies load on source-of-truth (treat-as-miss behavior). Cross-replica visibility of stale data under eventual consistency. Cache stampede when TTL expires across replicas simultaneously (the in-process version was handling this implicitly via per-key singleflight, if any).

**Operational complexity.** Managed cache infrastructure (Redis / memcached). Eviction policy tuning. Latency envelope now includes network-hop for every Get/Set.

**Consistency / ordering trade-offs.** Cache value lifetime → TTL-based; freshness across replicas → eventual.

**PLOS §4.2 fit: low — finding worth recording.** Once the cache is externalized, every Get/Set pays the network hop; there is no "local at low load" regime. Useful archetype, but its usefulness argument is closer to "automated hand-port to managed cache" than to the §4.2 dynamic-placement story.

**Cross-run disagreement.** Gemini ranks #3 by utility ("day-1 optimization"); opus ranks #7 ("low PLOS fit"); gpt-5.4 at #5 (mixed). Composite leans toward opus's framing — easy to lift and useful in the hand-port sense, but weakest on thesis-demonstration value.

---

## 7. `session-affinity-state`

**Pays off when** per-connection state is bounded by connection lifetime, many concurrent connections, and per-connection work is substantial enough to benefit from cross-replica load-spreading.
- **Corpus anchors:** mattermost `WebConn` MM2 (`evaluation/mattermost/server/channels/app/platform/web_conn.go:88-149`) + composite hub region MM1 — millions of concurrent websockets is canonical; gitea session stores G11/G12/G13 (`evaluation/gitea/modules/session/db.go:93`) — when deployment runs many concurrent users; caddy hijacked-upgrade state C6.

**Net-negative when** session lifetime is sub-request (request-scoped, not connection-scoped), sessions routinely migrate across connections (mobile reconnects), or cross-session invariants make per-replica sticky-routing story break down (mattermost cluster model with single user across multiple connections — flagged as out-of-v1-scope).

**Code-structural tells.**
- **Positive tell:** state map keyed by session-ID field; key ingress at connection-accept time; state is removed at session close (observable via lifecycle API).
- **Negative tell:** state references cross-session shared objects; session-ID derived from request at request-time rather than connection-accept time.

**New failure modes.** Session loss on replica crash (in-process version also lost it but silently — distributed version has to decide whether to reconstruct or fail). Sticky-routing misroutes under rebalancing. Partition between session-replica and cross-session coordination service.

**Operational complexity.** Session-affinity-aware load balancer (consistent-hash on session ID, sticky routing). Per-replica state management.

**Consistency / ordering trade-offs.** Per-session serialization preserved; session-scoped lifetime preserved; migration across replicas is not supported (state becomes non-migratable mid-connection).

**PLOS §4.2 fit: high at the top end of the load curve** — exactly the "scale-out when load increases" scenario §4.2 motivates. Low-load benefit is zero. The delegate DSL (`metric=CPU threshold=75%`) can express this cleanly.

**Composite regions are where the demo lives.** Mattermost MM1+MM2 (session-affinity-state + fanout-publisher + keyed-partitioned-state = connection-hub-buffer composite per v1 catalog) is the single strongest corpus demo for the Monolift thesis.

---

## 8. `filesystem-bound-singleton`

**Pays off when** local disk state is *data* (not lifecycle configuration), deployment needs horizontal scaling of the service that reads/writes it, and path/object semantics match object-store primitives.
- **Corpus anchor:** caddy `filestorage` for certificates — certificates are data, current storage is operational bottleneck for multi-instance caddy deployments, object-store semantics (put / get / list under prefix) match directly.

**Net-negative when** filesystem is used for *local* lifecycle state the process needs on the same machine (lock files signaling "I am running here"; gitea `process.Manager`'s lock file usage is this), filesystem access encodes crash-safety via fsync/rename-dance that object-store replacement doesn't preserve, or paths carry invariants (parent-before-child creation ordering) that don't survive translation to flat object keys.
- **Corpus anti-anchors:** gitea local-storage when used for build/cache artifacts; gitea process.Manager lock files.

**Code-structural tells.**
- **Positive tell:** `os.File` / `filepath` calls in methods whose structs hold path-config fields; operations are single-shot (open → read/write → close); no in-memory state bridges invocations.
- **Negative tell:** long-held file handles; locks on files signaling liveness; directory-creation ordering invariants.

**New failure modes.** Object-store unavailability becomes a hard dependency. Eventual-consistency surprises on list-after-write. Latency floor rises from local-disk μs to network-ms for every operation.

**Operational complexity.** Object-store client + credentials; or sidecar runtime with volume mapping.

**Consistency / ordering trade-offs.** fsync-and-rename semantics of POSIX → eventual-consistency put/get of object stores. Parent-before-child directory invariants don't exist in flat key spaces.

**PLOS §4.2 fit: low** (same as `ttl-cache` — one-way externalization, no local fast path). **High-value automation for cloud migration, but outside §4.2.**

**Cross-run note.** Only gemini surfaced this as a distinct archetype originally (v1 research). The utility lens validates keeping it separate: transform is distinct (object-store adapter vs. actor harness), even though corpus coverage is narrow. Gemini's "S3-backed Pocketbase" evaluation scenario captures the cloud-native-persistence utility case clearly.

---

## Cross-archetype summary table

| Archetype | PLOS §4.2 fit | Coverage (v1) | Utility rank | Failure mode severity | Ops cost |
|---|---|---|---|---|---|
| `bounded-worker-pool` | **highest** | 4/6 | **1** | medium (broker as new dep) | medium |
| `periodic-invocation` | high-but-subtle | 6/6 | 2 | low (missed ticks tolerable) | low |
| `session-affinity-state` | high (load-curve top) | 3/6 | 3 (composite-driven) | medium (sticky routing) | medium |
| `fanout-publisher` | medium | 4/6 | 4 | medium (delivery semantics) | medium |
| `serialized-actor` (coordinator) | medium (bifurcated) | 5/6 | 5 | medium (SPOF) | medium |
| `ttl-cache` | low (one-way) | 5/6 | 6 | low (miss amplifies load) | low |
| `keyed-partitioned-state` | mixed (target-dependent) | 5/6 | 7 | medium (shard invariants) | high (routing) |
| `filesystem-bound-singleton` | low (one-way) | 2/6 | 8 | high (new hard dep) | medium |

**Three cross-cutting observations:**

1. **Dynamic-delegate eligibility** (can the transform's local and remote forms coexist in the runtime's delegate DSL?) varies across archetypes. Worth carrying as an explicit catalog bit — see `utility-scenarios-v1.md` §5.2.
2. **Within-archetype utility bimodality** affects `serialized-actor` most strongly. The archetype label alone is insufficient for AUTO-vs-SUGGEST decisions; region-level utility heuristics needed.
3. **Low-independence archetypes** (`fanout-publisher`, composite `session-affinity-state` regions) deliver high value but require coordinated co-lifts — the opposite of pay-as-you-go. Composite-archetype support (ADR-0022) is load-bearing for the PLOS thesis demo.

See `prioritization-implications-v1.md` for sprint-sequencing consequences.
