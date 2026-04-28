# Usefulness scenarios v1 — opus run

**Status:** run artifact. Parallel run with gpt-5.4 and gemini; composite synthesis after.

SPRINT-0013 asked *what can Monolift lift*; this run asks *when does lifting pay off and when is it net-negative*, grounded in the PLOS '25 paper's framing and the v1 archetype catalog.

## 1. Framing anchor — what "utility" means in the PLOS '25 paper

Reading the paper (`inspiration/papers/monolift-plos25.pdf`) before reasoning about archetypes is load-bearing, because Monolift's notion of utility differs from a generic "when to microservice" argument in three specific ways:

1. **Utility = workload-responsive placement, not scaling per se.** §4.2 positions Monolift against both the monolith (best at low load, tail-latency spike at saturation) and the static microservice (worse at low load, stable under load). The claim is "best of both worlds": near-monolith latency at low load *and* microservice scalability at high load. Utility is the area under that Pareto improvement, not the top-end scaling ceiling alone.
2. **Pay-as-you-go.** §1 cites Service Weaver's abandonment — "it required rewriting large parts of existing applications." Monolift's utility argument rests on being worth using *for a handful of lifts* in an otherwise-monolithic app. An archetype that is only useful when 90% of the application is already lifted does not fit the thesis.
3. **Rapid exploration, not a priori "correct" decomposition.** §4.1's timeline-only result is the headline: the ideal decomposition was not obvious before measurement, and finding it required Monolift to generate multiple architectures cheaply from the same source. Utility includes *being cheap to try and un-try*, not just "helps when applied."

**Implication for this research.** An archetype's usefulness in the Monolift sense is highest when its lift (a) preserves a local-execution fast path the runtime can take at low load, (b) can be introduced for a small region without requiring co-lifting neighbors, and (c) delivers a credible win under some workload the application actually experiences. An archetype is *less* useful when its transform is "all or nothing" (state moves to a managed substrate and cannot go back), even if the transform itself is clean and liftable.

This single framing does more work for per-archetype reasoning than the individual trade-off lists below, because it predicts which archetypes have the best utility ceiling in the *Monolift* setting specifically.

## 2. Cross-archetype structural axes

Two structural axes predict usefulness across the eight archetypes. These are the same two axes the v1 composite note used to explain evidence thresholds (evidence-locality × externalization-affinity) but reinterpreted for utility rather than confidence.

### 2.1 Local-fast-path preservability

Does the lifted region retain a cheap, in-process implementation that the Monolift runtime can use at low load, with remote execution only kicking in under pressure? This is the PLOS '25 §4.2 property directly.

- **High preservability** — local call is the natural zero-op at low IPS/CPU, and the runtime flips to remote transparently. Fits `periodic-invocation` (local goroutine ↔ remote scheduler is the same invocation boundary), `bounded-worker-pool` (local channel send ↔ broker publish is swappable at the enqueue call), `serialized-actor` (local method call ↔ RPC round-trip; same signature), `fanout-publisher` (local subscribers ↔ broker topic subscribers), `session-affinity-state` (request stays on local replica when lightly loaded; sticky-routing engages when replicas scale out).
- **Low preservability — transform is a one-way externalization.** Fits `ttl-cache` (once you replace the in-process cache with Redis, every Get/Set pays the network cost, even at 1 RPS — the dynamic-offload story is weakened), `filesystem-bound-singleton` (once paths become object keys, the local disk path is structurally gone), `keyed-partitioned-state` when the transform target is a managed KV store (same reason).

Low-preservability archetypes aren't useless — they still scale — but their *Monolift-specific* advantage over writing a direct Redis client is smaller. The usefulness is primarily in the automation (no hand-porting), not in the dynamic-placement property.

### 2.2 Independence of the lifted unit

Can the archetype's region be lifted alone, or does the transform pull a constellation of neighbors with it? This is the pay-as-you-go property.

- **High independence** — `periodic-invocation` (a single scheduled function), `bounded-worker-pool` (queue + N replicas, self-contained), `ttl-cache` (cache is a closed surface), `filesystem-bound-singleton` (storage adapter behind an existing interface), single-region `serialized-actor` instances.
- **Low independence** — `fanout-publisher` pulls all subscribers onto the broker at once, and each subscriber becomes a service of its own (high-value but high-commitment). Composite `connection-hub-buffer` regions (mattermost `web_hub.go` MM1 / `web_conn.go` MM2) co-lift `keyed-partitioned-state` + `fanout-publisher` + `session-affinity-state` simultaneously; they deliver outsized value but are the opposite of pay-as-you-go. `keyed-partitioned-state` with cross-key iteration in hot paths drags iteration-protocol redesign with it.

**The two axes are partially independent.** A high-independence / low-preservability archetype (e.g., `ttl-cache`) is fine to ship in isolation but gives up the monolith-floor property. A low-independence / high-preservability archetype (e.g., `connection-hub-buffer` composite in mattermost) preserves the local fast path but requires a large coordinated transform. The most "Monolift-native" archetypes are the ones that are high on both axes: `periodic-invocation` and `bounded-worker-pool` (along the single-region facet).

## 3. Per-archetype reasoning (narrative form; cards live in `per-archetype-cards-v1.md`)

### `serialized-actor`

**Pays off when** the actor's method calls are already the application's coarsest-grained unit of work and the caller already expects the call to block on a mutex for a non-trivial fraction of the time — the RPC cost replaces the uncontended mutex cost but becomes the *contended* mutex's relief once the actor has its own replica. Gitea's queue.Manager (G4) and eventsource.Manager (G6) fit: these are coordinator singletons whose in-process mutex is already a known serialization point. Gitea's process.Manager (G18) fits for the same reason — the lookup-by-pid shape is coordinator-y. Caddy's connections registry (C5) fits *only* if viewed as a coordinator serving handler requests; if viewed as connection state hanging off the handler, see §3.7.

**Net-negative when** the actor is on a user-visible sync call path and the method is short enough that RPC round-trip dominates. Miniflux's ProxyRotator (M6) is the clearest miss: the actor's method is "pick next proxy" — a microsecond-level decision the feed-fetch hot path calls synchronously. Distributing it would regress every feed fetch to pay a network round-trip for what is currently a local mutex/index increment, and the actor holds no state worth centralizing — the proxy list is small and rarely changes. Local fast path preservation (§2.1) nominally helps, but the operational overhead of running the actor as a remote service for the low-load case is itself the loss.

**New failure modes.** Remote actor unavailability blocks every call; retries may double-apply mutations that were idempotent at the in-process layer (because the duplicate detection was the mutex itself). Network partitions isolate the actor from its callers without giving either side a correct fallback — the actor is structurally a single-point-of-failure by its own semantics.

### `bounded-worker-pool`

**Pays off when** the work is naturally asynchronous-completable, per-job ordering is not user-visible, and the existing channel is already a coarse buffer between request ingress and background processing. Listmonk's manager.worker (L2) is the canonical fit: it consumes from `campMsgQ`, each job is an email-send, per-email ordering across the campaign doesn't matter, and the existing channel is already an async boundary. Gitea's WorkerPoolQueue (G1) fits for the same reason — it is the queue infrastructure, ordering is already non-strict across tasks of different types. Mattermost's PushNotificationsHub (MM6) fits: push delivery is fire-and-forget, no ordering contract.

**Net-negative when** the in-process channel was chosen precisely because enqueue is synchronous and fast, and the caller depends on "enqueue returned → the job is durably queued" semantics. A broker enqueue adds a network-round-trip plus durability-dependent latency on the enqueue side; if the caller is on a hot synchronous path, that cost hits every request even if the job processing itself is still async. Also net-negative when per-key FIFO is load-bearing — the catalog flags this as SUGGEST, but the usefulness reading is stronger: per-key FIFO often cannot be reconstructed cheaply on top of a broker without a custom dispatcher that erases most of the transform's value.

**New failure modes.** Broker unavailability on enqueue; at-least-once delivery forces handlers to be idempotent (the in-process version did not have to be); handler crashes may now redeliver and duplicate visible effects.

**PLOS framing fit.** High. The enqueue call site preserves the local-fast-path cleanly (channel send ↔ broker publish at the same signature), and the pool is a single-region lift. This is the archetype most aligned with the paper's story.

### `periodic-invocation`

**Pays off when** the body is genuinely background maintenance — no request is waiting on it, duplicate or skipped ticks are tolerable, and the main application loses nothing by outsourcing the loop to a platform scheduler. Caddy's `stayUpdated` (C1) and `keepStorageClean` (C2) fit perfectly — certificate housekeeping, skip-tolerant, idempotent-on-retry. Miniflux's feed scheduler M1 and cleanup M2 fit. Listmonk's `runMailboxScanner` (L3) fits. Pocketbase Cron (P2) and gitea Cron (G14) are thin wrappers around this archetype already and arguably ADMITTED.

**Net-negative when** the "periodic" loop is actually a control loop with cross-tick state — a backoff counter that increases on failure, a self-tuning interval, a watermark pointer that the next tick reads from the previous tick's local memory. Moving to a stateless scheduler either loses this state or requires a durable scratchpad whose overhead dominates for trivial loops. Also net-negative for `scanCampaigns` (L1) *despite* fitting the shape, because it is the primary control loop of listmonk's campaign distributor: if this loop pauses or duplicates, visible user-facing behavior (campaigns firing late / twice) degrades. The catalog correctly flags pragma-supplied `idempotent=true` as load-bearing; the utility reading is that without that declaration, the archetype should not auto-apply even when shape fits.

**New failure modes.** Scheduler partition (tick missed or delayed); cold-start on serverless scheduled triggers; tick overlap when previous tick runs long and scheduler fires the next one anyway.

**PLOS framing fit.** High but subtle. The API cost is zero (`Start`/`Stop` become no-ops), and local-fast-path is trivially preserved (the cron is just running elsewhere). But there is *no* workload condition where running it remotely vs. locally affects *user-visible* latency — periodic ticks are not on the user hot path by definition. So the utility is "operational / resource packing," which is real but thinner than the §4.2 headline benefit.

### `keyed-partitioned-state`

**Pays off when** the key space is naturally partitionable, per-key load is skewed enough that sharding relieves hot shards without contention-creating co-location, and no consumer depends on a global view. Gitea's baseChannel uniqueness set (G2) and process.Manager (G18) lean AUTO by the catalog, but by the PLOS §4.2 reading the utility is modest: neither is a user-perceived bottleneck, and both are small-volume coordination maps. The *interesting* keyed-partitioned-state regions are composites — caddy C5 connections map and mattermost MM1 hub index — where the partitioning *is* the connection routing story, not incidental.

**Net-negative when** (a) iteration across all keys appears in the hot path (listmonk L5 pipes+links: campaign manager iterates to dispatch; lifting turns O(shards) aggregation into a distributed operation that the in-process version was O(1)-amortizing cheaply), (b) the map encodes cross-key invariants ("sum of X across entries"), or (c) the key cardinality is small enough that the in-process mutex map was already fine and sharding adds protocol cost for nothing.

**New failure modes.** Shard routing mistakes (cross-shard writes silently go to the wrong partition); shard rebalancing during operation; loss of cross-shard iteration semantics.

**PLOS framing fit.** Mixed. When the transform target is a managed KV (Redis Cluster, DynamoDB), local-fast-path preservability is low — the v1 archetype trades the monolith-floor for horizontal scale-out. When the transform target is a consistent-hash router + per-shard service, the fast path is in principle preservable but the runtime needs to know "when is sharding active vs. not," which is not a shape the current delegate DSL handles.

### `fanout-publisher`

**Pays off when** subscribers are already logically independent services that happen to co-reside with the producer only for convenience. Mattermost's cluster-level Publish (MM7) is ADMITTED for exactly this reason — already distributed in source. Gitea's eventsource.Messenger (G7) fits: server-sent events to clients, per-subscriber independence is near-total. Listmonk's `events.Publish` (L4) fits when the subscribers are things like "write to log / emit metric" that can become broker consumers.

**Net-negative when** the fanout encodes a distributed transaction ("every subscriber must succeed before the publish is considered done") or when subscribers read back shared state that is also being lifted — lifting one without the other creates read-your-writes issues across the broker boundary. Also net-negative when subscriber count is fixed-and-small and the subscribers are tightly co-designed with the publisher (Pocketbase Broker P4 is borderline; the broker is already a small module and replacing it with an external broker may not change anything users observe while adding an infra dependency).

**New failure modes.** At-least-once delivery forces consumer idempotency; broker unavailability blocks publish (or drops events if async); ordering across subscribers relaxes to broker ordering semantics.

**PLOS framing fit.** Medium. API preservation is clean (Publish/Subscribe do not change), but *dynamic* swap between local fanout and broker fanout is not cleanly expressible — subscribers are either in-process or on the broker, not both. So the archetype fits "pay-as-you-go for one decision" but not "runtime-adaptive."

### `ttl-cache`

**Pays off when** the cache is genuinely shared read-only state (same value would be fetched by every replica under a replicated-stateless-service baseline), the value lookup is expensive, and the cache would have to be rebuilt per-replica absent a managed substitute. Mattermost's session cache (MM4/MM5) fits — sessions are hit on every authenticated request; without a shared cache, every replica would independently revalidate. Listmonk's apiUsers (L6) is similar. Pocketbase's `tools/store` (P3) with TTL entries fits when data is cross-replica-visible source-of-truth elsewhere.

**Net-negative when** the cache is a local memoization of a local computation (gitea G10 `EphemeralCache` for a short-lived request-scoped calculation), when the in-process cache is already small enough that miss-rate on the first request of a new replica is fine, or when the cache value carries function pointers / callbacks the compiler must reject. Also net-negative when the source-of-truth is already a managed store and the in-process cache is the sole defense against that store's latency — replacing the defense with another network call is circular.

**New failure modes.** Managed cache unavailability (treat as miss, amplifies load on source-of-truth); cross-replica visibility of stale data under eventual consistency of the managed cache; cache stampede when TTL expires across replicas simultaneously (the in-process cache with per-key singleflight was handling this implicitly).

**PLOS framing fit.** Low. Once the cache is externalized, *every* Get/Set pays the network hop; there is no "local at low load" regime. This archetype is useful — but its usefulness argument is closer to "automated hand-port to managed cache" than to the PLOS §4.2 dynamic-placement story. **Finding worth recording.**

### `session-affinity-state`

**Pays off when** per-connection state is genuinely bounded by the connection lifetime, there are many concurrent connections, and the work-per-connection is substantial enough to benefit from cross-replica load-spreading. Mattermost's WebConn MM2 and the composite hub region MM1 fit: millions of concurrent websockets is the canonical workload. Gitea's session stores G11/G12/G13 fit when the deployment runs many concurrent users (self-hosted gitea for a mid-size org is exactly the load). Caddy's hijacked-upgrade state C6 fits for websocket-gateway deployments.

**Net-negative when** the session lifetime is sub-request (request-scoped, not connection-scoped), when sessions routinely migrate across connections (mobile reconnects), or when the cross-session invariants (mattermost cluster model) make the per-replica sticky-routing story break down. Self-hosted small-scale deployments (single-replica gitea with twenty users) see zero benefit from the lift and pay all the routing complexity.

**New failure modes.** Session loss on replica crash (the in-process version lost it too, but silently — distributed version has to decide whether to reconstruct or fail); sticky-routing misroutes under rebalancing; partition between session-replica and cross-session coordination service.

**PLOS framing fit.** High at the top end of the load curve — this is exactly the "scale-out when load increases" scenario §4.2 motivates — but low-load benefit is zero because the lift is only worth engaging when replica count grows. The delegate DSL (`metric=CPU threshold=75%`) can express this cleanly.

### `filesystem-bound-singleton`

**Pays off when** the state on local disk is data (not configuration), the deployment needs horizontal scaling of the service that reads/writes it, and the path/object semantics are a clean match for object-store primitives. Caddy's filestorage for certificates fits: certificates are data, the storage is currently an operational bottleneck for multi-instance caddy deployments, and object-store semantics (put / get / list under prefix) match directly.

**Net-negative when** the filesystem is used for *local* state the process needs on the same machine (lock files signaling "I am running here"; gitea process.Manager's lock file usage is this). Also net-negative when the filesystem access encodes crash-safety via fsync/rename-dance that the object-store replacement does not preserve for free, or when paths carry invariants (parent-before-child creation ordering) that do not survive translation to flat object keys.

**New failure modes.** Object-store unavailability is now a hard dependency; eventual-consistency surprises on list-after-write; latency floor rises from local-disk μs to network-ms for every operation.

**PLOS framing fit.** Low, for the same reason as `ttl-cache`: the transform is a one-way externalization. High-value but outside the §4.2 dynamic-placement argument.

## 4. Cross-cutting findings

### 4.1 A PLOS-framing finding that the v1 catalog does not make explicit

The v1 catalog ranks archetypes by corpus coverage and feasibility (evidence + emission). The PLOS '25 paper ranks utility by local-fast-path preservability × workload-responsive-placement fit. These two orderings disagree for at least three archetypes:

| archetype | v1 feasibility | PLOS utility fit | cross-run note |
|---|---|---|---|
| `periodic-invocation` | strongest coverage (6 targets) | high-API-neutrality but no user-visible-latency story | re-ranked down by PLOS lens |
| `bounded-worker-pool` | mid coverage (4 targets) | best fit (local channel ↔ broker at same call site) | re-ranked up |
| `ttl-cache` | solid coverage (5 targets) | low — static externalization, no monolith floor | re-ranked down |
| `serialized-actor` | strongest state-class breadth (5 targets) | uneven — coordinator-shaped instances yes, hot-path actors no | bifurcated |

If the compiler picked what to implement first by PLOS-utility rather than by feasibility, the order would be `bounded-worker-pool` then the coordinator-shaped subset of `serialized-actor` then `session-affinity-state`, with `periodic-invocation` / `ttl-cache` / `filesystem-bound-singleton` as "implement because they cover ground and automate hand-ports" rather than "implement because they demonstrate the Monolift thesis."

This is the meta-question the brief asked — yes, usefulness reorders the v1 list; see `prioritization-implications-v1.md`.

### 4.2 Dynamic-placement eligibility is its own predicate and the v1 catalog doesn't carry it

The v1 catalog records AUTO/SUGGEST/TERMINAL per archetype. There is no entry for "lifted region admits a dynamic delegate expression (can flip between local and remote at runtime based on metric thresholds)."

From the PLOS paper that predicate is the headline property, and not every v1 archetype supports it:

- **Supports dynamic delegate naturally:** `periodic-invocation`, `bounded-worker-pool` (at the enqueue site), `serialized-actor` (at the method site), `fanout-publisher` (at the Publish site), `session-affinity-state` (at the routing decision).
- **Does not support dynamic delegate cleanly:** `ttl-cache`, `filesystem-bound-singleton`, and `keyed-partitioned-state` when the target is a managed KV. These have a one-way "externalize to managed substrate" transform.

Suggested finding: either the catalog adds a `dynamic-delegate-eligible` bit per archetype, or ADR-0019's SUGGEST/AUTO output makes the distinction surface in remediation text. Without this, the compiler can auto-lift an archetype whose transform structurally *cannot* deliver the PLOS §4.2 behavior, and the paper's utility claim does not cover those regions.

### 4.3 Usefulness of an archetype is uneven *within* the archetype

`serialized-actor` in gitea's queue.Manager (coordinator, low-frequency coordination) is usefully lifted; `serialized-actor` in miniflux's ProxyRotator (hot-path microsecond decision) is actively counterproductive. The archetype label does not distinguish these. The catalog's AUTO/SUGGEST/TERMINAL thresholds distinguish *feasibility*, not *utility*. This means: even within AUTO regions, the compiler's decision to auto-apply should probably be gated on a utility-prior signal (e.g., "is this region on a request-synchronous path?"). A lightweight heuristic — SSA reachability from an ADMITTED handler entry — would catch the miniflux ProxyRotator case without needing new vocabulary.

### 4.4 Composite regions are where the PLOS demo lives

The v1 composite `connection-hub-buffer` (mattermost MM1 + MM2) is low-independence (co-lifting required) but high-preservability (local hub ↔ distributed cluster at the same API). It is the single region in the corpus that best demonstrates Monolift's §4.2 story: at low load, the hub runs in-process; at high connection count, the hub splits across nodes via consistent-hash on user-id and the API to callers is unchanged. Every other region individually makes a weaker version of this argument.

### 4.5 Three "not useful despite liftable" patterns worth flagging

1. **Hot-path microsecond singletons** (miniflux ProxyRotator M6, potentially caddy basicauth cache C7 if request-synchronous). Fits `serialized-actor` shape, does not repay RPC round-trip.
2. **Ephemeral caches of local computations** (gitea EphemeralCache G10). Fits `ttl-cache` shape, does not repay managed-cache overhead.
3. **Thin in-process wrappers over already-external state** (listmonk L5 pipes+links — DB is the source of truth; the pipes map is a routing convenience). Lifting it creates a second source of truth without retiring the first.

These three patterns recur across targets and should be called out as "liftable-but-SUGGEST-for-utility-reasons" in ADR-0019's remediation text, not just "liftable-but-SUGGEST-for-evidence-reasons." The catalog currently conflates the two.

## 5. Two meta-questions the brief asked

**Does ease-of-auto-lift correlate with usefulness?** Weakly, and not in the direction v1 implies. The easiest archetypes to lift automatically (`periodic-invocation`, `ttl-cache`) are the ones with the thinnest PLOS-utility story. The ones with the strongest utility story (`bounded-worker-pool`, session-affinity-state composites) are mid-difficulty. This is an argument for not letting corpus-coverage feasibility dominate the implementation order.

**Are there archetypes whose usefulness is conditional enough that auto-apply is wrong even with strong evidence?** Yes — `serialized-actor` on hot-path regions, `ttl-cache` on ephemeral local caches, `keyed-partitioned-state` on low-cardinality maps. These should probably stay SUGGEST even when evidence gates close, with the SUGGEST rationale citing utility-heuristic rather than evidence-gap.
