# Per-archetype usefulness cards v1 — opus run

**Status:** run artifact. Parallel with gpt-5.4 and gemini. See `usefulness-scenarios-v1.md` for narrative; this file is one card per v1 archetype with the six facets the brief asks for.

Region IDs (C*, G*, L*, M*, MM*, P*) reference the composite per-target annotations at `docs/research/annotations/<target>.md` and the v1 catalog at `docs/research/archetype-catalog-v1.md`.

---

## 1. `serialized-actor`

**Pays off when:**
- The actor is a *coordinator* whose in-process mutex is already a known serialization point and the method call rate is moderate (not microsecond hot path). Examples: gitea `queue.Manager` (G4), `eventsource.Manager` (G6), `process.Manager` (G18). RPC round-trip replaces mutex contention, and the coordinator's own replica has no competing callers on its machine.
- The actor holds state worth centralizing for cross-replica correctness (single owner of a scheduling decision, singleton view of queue status). The horizontal scaling happens *around* the actor — the actor is the one thing that doesn't scale, by design.

**Net-negative when:**
- On the user-visible request hot path with short method bodies: miniflux `ProxyRotator` (M6). Picking the next proxy is μs-level; making it an RPC regresses every feed fetch.
- The mutex is guarding not an actor but an un-externalizable resource (durable embedded client): pocketbase core.App's SQLite is TERMINAL for this reason and already caught by the catalog.
- Small, rarely-contended state where the in-process mutex was effectively free (caddy `HTTPBasicAuth.Cache` C7 seen as `serialized-actor` rather than `ttl-cache` — the cache semantics are the dominant story, and request-synchronous hot path makes RPC a loss).

**Code-structural tells that correlate with usefulness:**
- Methods are called from goroutines *other* than the one holding the request → asynchronous call pattern → RPC cost less visible.
- Actor is named `Manager`, `Registry`, `Coordinator`, `Scheduler` in corpus — hint of coordinator-shape.
- Call graph shows fan-in from many callers with low per-caller rate, rather than concentrated calls from a single hot loop.
- Mutex hold time in the method body is non-trivial (work inside the critical section, not just a field read) → RPC cost amortizes.

**New failure modes introduced:**
- Remote unavailability blocks every method call (in-process mutex never failed).
- Retry-after-timeout may double-apply a mutation that was previously idempotent-by-mutex.
- Network partition isolates the actor; callers must choose between blocking and degraded fallback.

**Operational complexity added:**
- One more deployable service with its own health, metrics, rolling-update story.
- Mailbox / command-dispatch harness needs monitoring (queue depth as a health signal).
- The actor is a single point of failure by its own semantics — operators need to understand this.

**Consistency / ordering trade-offs:**
- Preserved: per-actor serialization (the whole point).
- Relaxed: any cross-actor invariant that the single-process version got for free by sharing address space — none if the actor was a true singleton, but caddy C5 treated as actor has this risk (connections registry interacts with the handler's own state).

**Corpus regions where lifting seems plausibly useful:**
- gitea G4 `queue.Manager` — coordinator for queue infrastructure, naturally fan-in.
- gitea G6 `eventsource.Manager` — active-subscription registry; consumed by long-running SSE connections, not hot loops.
- gitea G15 cron task registry — low-frequency lifecycle management.
- gitea G18 `process.Manager` — process table, lookup-by-pid shape.
- pocketbase P1 `Hook[T]` — generic hook dispatcher; payoff depends on hooks being invoked off-hot-path (needs verification per hook type).
- pocketbase P5 `BatchHandler` — by construction already accumulates before dispatching, so RPC amortizes over the batch.
- mattermost MM11 cluster-leader-listeners — leader election events are low frequency.

**Corpus regions where lifting seems not useful despite being liftable:**
- miniflux M6 `ProxyRotator` — hot-path microsecond decision, state is tiny, no centralization benefit.
- caddy C7 `HTTPBasicAuth.Cache` seen as actor — request-synchronous, see `ttl-cache` card for the correct framing.
- caddy C5 `Handler.connections` seen as actor — better framed as session-affinity-state or keyed-partitioned-state.

---

## 2. `bounded-worker-pool`

**Pays off when:**
- The existing channel is already a coarse async boundary between request ingress and background processing.
- Per-job ordering is not user-visible; each job is independently completable.
- The pool becomes a scalable group of stateless consumers whose count tracks broker backlog.
- Handler is idempotent or can be made so cheaply.

**Net-negative when:**
- Enqueue is on a sync request path and the caller expects "returned → durably queued" with minimal latency. Broker publish adds a network hop per enqueue, which the channel-send did not.
- Per-key FIFO ordering is load-bearing (specific user's events must be processed in order). Per-key ordering on a broker requires either a single-partition-per-key scheme with skew risk, or a custom dispatcher that re-introduces the serialization the broker was meant to remove.
- Pool is unbounded-on-overflow in source ("fall back to spawning one-off goroutines"). The transform assumes static bounding; dynamic overflow behavior is lost.

**Code-structural tells that correlate with usefulness:**
- Channel is a struct field (not a function-local), implying a persistent queue.
- Enqueue site is not on the request's critical path (request returns before the job runs).
- Handler signature has `error` return but no "wait for result" caller — fire-and-forget in spirit.
- Pool size is a config constant, not runtime-computed.
- Handlers do not share writable in-process state with each other.

**New failure modes introduced:**
- Broker unavailability on enqueue (where fallback was "return quickly" before).
- At-least-once delivery → handler must be idempotent; side effects that were single-execution-by-construction may now be double-applied.
- Handler crash mid-job means broker redelivers; external side effects partially applied before the crash repeat.

**Operational complexity added:**
- Broker is a hard runtime dependency with its own SLAs, auth, retention, DLQ.
- Consumer lag / backlog monitoring replaces channel-depth-at-stop debugging.
- Worker replicas need health and scaling (HPA on consumer lag).

**Consistency / ordering trade-offs:**
- Global FIFO: lost (was already sketchy across pool workers in-process; now formally gone).
- Per-key FIFO: requires explicit broker feature or custom keying; not free.
- Exactly-once: not available by default; must be built at the handler level.

**Corpus regions where lifting seems plausibly useful:**
- listmonk L2 `manager.worker` + `campMsgQ` — the canonical campaign-email dispatcher. Emails per subscriber are independent; enqueue-is-durable semantics already come from the database side. This is the strongest Monolift-demo-shape in the corpus, barring connection-hub-buffer composites.
- gitea G1 `WorkerPoolQueue` — queue infrastructure is *designed* to be this shape.
- mattermost MM6 `PushNotificationsHub` — push delivery is fire-and-forget.

**Corpus regions where lifting seems not useful despite being liftable:**
- None observed in the corpus — every bounded-worker-pool region the annotations cite is a reasonable candidate. The net-negative cases are more about "pool size semantics lost" (unbounded fallback) than "the archetype was wrong."

---

## 3. `periodic-invocation`

**Pays off when:**
- Body is idempotent and skip/duplicate tolerant.
- Body is pure background maintenance — no request blocked on it.
- Body's execution cost is meaningful enough that offloading it frees request-path replicas from running the tick.
- Interval is config-driven, not self-tuning from prior tick.

**Net-negative when:**
- Body carries cross-tick state that would need durable scratchpad if externalized (counter, watermark pointer, backoff state).
- The loop is actually a control loop whose duplicate/skip is user-visible (listmonk L1 `scanCampaigns` if we worry about late campaign firing; safe only with explicit idempotency guarantee).
- The loop is so trivial that platform-scheduler overhead exceeds the goroutine-tick cost.
- Interval is dynamically self-tuning (backoff adjusts based on previous-tick result).

**Code-structural tells that correlate with usefulness:**
- Loop body reads input, does work, writes result — no loop-carried variable.
- No `time.Sleep(backoff)` where backoff is a mutable field.
- Loop body's effects are idempotent-declared or provably global-write-free.
- Interval is read once at loop start (or each tick) from config, not updated by loop.

**New failure modes introduced:**
- Missed ticks under scheduler pressure (k8s CronJob can skip concurrent triggers by default).
- Cold start on serverless scheduled triggers adds latency that the background goroutine didn't have.
- Two concurrent ticks if scheduler fires while previous tick still runs and concurrency policy allows.

**Operational complexity added:**
- A platform scheduler entry per periodic (k8s CronJob spec, cloud cron config).
- Tick execution history needs its own observability (cron job logs, not application logs).
- Concurrency policy choice (allow/forbid/replace) becomes an explicit config decision.

**Consistency / ordering trade-offs:**
- Preserved: "at least one tick every N interval" (with slack).
- Relaxed: "exactly one tick per N interval" — scheduler drift and concurrency policy can duplicate or skip.
- Lost: any implicit "tick K+1 starts after tick K finishes" assumption, unless concurrency policy enforces it.

**Corpus regions where lifting seems plausibly useful:**
- caddy C1 `stayUpdated`, C2 `keepStorageClean` — certificate housekeeping, classic idempotent background.
- miniflux M1 feed scheduler, M2 cleanup — periodic maintenance that doesn't block user reads.
- pocketbase P2 `Cron` — already a cron abstraction; thin wrapper.
- gitea G14 `services/cron.Task` — infrastructure cron.
- listmonk L3 `runMailboxScanner` — bounce detection, not on user path.

**Corpus regions where lifting seems not useful despite being liftable:**
- listmonk L1 `scanCampaigns` — the primary campaign control loop. Late/duplicate firing has user-visible effects; the archetype shape fits, but the utility argument wants strong idempotency evidence before auto-applying.
- mattermost MM8 `email_batching` — if the batching logic carries cross-tick accumulation state, the transform either moves that to a store (overhead) or loses it.

---

## 4. `keyed-partitioned-state`

**Pays off when:**
- Key space is large and load-skewed enough that sharding relieves hot shards.
- Per-key operations are the dominant access pattern.
- No consumer requires a global view or cross-key invariant.
- The transform target is a consistent-hash routed service, allowing per-shard replica growth.

**Net-negative when:**
- Iteration across all keys appears in the hot path (listmonk L5 iterates `pipes` on campaign dispatch — partitioning forces a distributed scatter for what was a local loop).
- Map encodes a cross-key invariant ("total count of active entries == len(map)").
- Key cardinality is small (dozens, not thousands). Partitioning adds routing cost for no load relief.
- Transform target is a managed KV (Redis), which makes the archetype low-preservability — every access pays a network hop.

**Code-structural tells that correlate with usefulness:**
- `map[K]V` field; every access site reads/writes at a single key derived from input.
- No `for k, v := range m` in hot paths (only in cleanup / shutdown).
- No computations over `len(m)` or aggregates across values.
- Key is stable across the lifetime of one request/connection.

**New failure modes introduced:**
- Shard routing failure (request reaches wrong shard silently under partial failure).
- Cross-shard rebalance window where reads may miss.
- Loss of cross-key consistent iteration (backup, metrics, cleanup all now need per-shard coordination).

**Operational complexity added:**
- Routing layer to maintain (consistent-hash ring or managed-KV client).
- Per-shard replicas with their own scaling.
- Rebalancing operations need care (migrating keys between shards under traffic).

**Consistency / ordering trade-offs:**
- Preserved: per-key atomicity (was the point of the in-process mutex; preserved by single-shard ownership).
- Lost: cross-key linearizability the in-process mutex map accidentally provided (rarely relied upon, but exists).

**Corpus regions where lifting seems plausibly useful:**
- mattermost MM1 `Hub.hubConnectionIndex` (in composite with fanout + session-affinity) — large key space (users), skewed load, natural partitioning. The hub is *the* composite region where this archetype earns its keep.
- caddy C5 `Handler.connections` — per-connection state; large key space under load.
- gitea G4 `queue.Manager` registry, G18 `process.Manager` — coordinator maps; useful if coordinator scaling matters. Borderline; see `serialized-actor` for alternate framing.

**Corpus regions where lifting seems not useful despite being liftable:**
- listmonk L5 `pipes` + `links` — iteration appears; DB is source of truth; the map is routing convenience.
- gitea G2 `baseChannel.set` uniqueness set — small key space, low volume, already TERMINAL-shaped by v1's retirement note.
- pocketbase P3 `tools/store` — generic store; many callers use it as local cache, partitioning offers no load relief.

---

## 5. `fanout-publisher`

**Pays off when:**
- Subscribers are already logically independent services co-residing only by accident.
- Subscriber count grows with application scale (more event consumers added over time).
- No cross-subscriber transactional dependency.
- Event payload is small and serializable.

**Net-negative when:**
- Fanout encodes a distributed transaction (every subscriber must commit or nothing does).
- Subscribers read back state the publisher writes, in the same request — broker adds async gap that breaks read-your-writes.
- Subscriber count is tiny and static (2-3 subscribers in source, unlikely to grow).
- Publisher holds the mutex across the fanout (some in-process implementations accidentally serialize everything through the publish path — broker fanout relaxes this, which is usually fine but may expose latent race conditions in subscribers).

**Code-structural tells that correlate with usefulness:**
- `map[K]chan T` or `[]chan T` field under mutex.
- Explicit Subscribe / Unsubscribe API that registers / deregisters channels.
- Publish iterates and sends without waiting for ack.
- Subscriber code in separate packages from publisher (already modular).

**New failure modes introduced:**
- At-least-once delivery → subscribers must be idempotent.
- Broker unavailability blocks publish (or silently drops, depending on config).
- Ordering across subscribers relaxes to broker topic-partition semantics.
- Subscribers may process events out-of-publish-order on the broker.

**Operational complexity added:**
- Broker as hard dependency (new SLO surface).
- Each subscriber is now a deployable, with its own consumer-group, lag, DLQ.
- Schema evolution of event payload becomes a coordination problem across independently deployed subscribers.

**Consistency / ordering trade-offs:**
- Total-order across subscribers: relaxed to partition-order (per-topic).
- At-most-once: lost (broker guarantees at-least-once unless configured exactly-once with overhead).
- Publish-ack semantics: changed — "publish returned" now means "broker accepted," not "all subscribers received."

**Corpus regions where lifting seems plausibly useful:**
- gitea G7 `eventsource.Messenger` — SSE to external clients; subscribers are outside the process already, in spirit.
- listmonk L4 `events.Publish` — pub/sub of application-level events; subscribers have independent responsibilities (log / metric / side-channel).
- mattermost MM7 cluster `Publish` — ADMITTED; validates the shape.
- pocketbase P4 `Broker` — borderline; small module, replacing with managed broker mostly adds infra complexity. Worth lifting only if pocketbase is scaled past single-node.

**Corpus regions where lifting seems not useful despite being liftable:**
- pocketbase P4 `Broker` in single-node deployments — adds infra without load relief.
- Any fanout where the in-process subscriber-set is 2 subscribers in source and unlikely to grow.

---

## 6. `ttl-cache`

**Pays off when:**
- The cache is shared read-only state across many request paths: replicas would each build the same cache without a shared substitute.
- Cache-miss cost is high (DB query, external API call).
- TTL is meaningful (minutes, not milliseconds) so the managed-cache hit rate is high.
- Source of truth is elsewhere; cache is a materialized view.

**Net-negative when:**
- Cache is a local memoization of a local computation (gitea G10 `EphemeralCache`): no cross-replica value.
- Value carries function pointers or pointer-to-in-process state.
- Every access was already μs-fast because cache hit rate was near-100% in-process; Redis hit rate is similar but cost is ms.
- The in-process cache's singleflight / stampede protection would need to be rebuilt on top of the managed cache.

**Code-structural tells that correlate with usefulness:**
- Cache `Get` has a "miss → load from DB/API" fallback path (true loader exists).
- Cache `Set` is the result of the load, not user-input-for-future-read.
- TTL is a field/config, not "until process restart."
- Value type is plain data (no channels, pointers to live in-process state, closures).

**New failure modes introduced:**
- Managed-cache unavailability → every request falls through to source of truth, amplifying load.
- Stampede on simultaneous TTL expiry (in-process singleflight, if relied upon, must be rebuilt).
- Cross-replica inconsistency window for managed-cache propagation.

**Operational complexity added:**
- Redis/memcached as runtime dependency with its own failure modes.
- Eviction configuration becomes a deploy-time concern.
- Hit-rate monitoring replaces in-process metrics.

**Consistency / ordering trade-offs:**
- Read-your-own-writes within one replica: no longer guaranteed (write goes to shared cache; read may hit a different replica's stale view of cluster-cached data).
- Cache is now eventually consistent across replicas.

**Corpus regions where lifting seems plausibly useful:**
- mattermost MM4/MM5 session and status caches — authenticated-request hot path, shared read-mostly data.
- listmonk L6 `apiUsers` / L7 `tmptokens` — API-key validation on every call; shared across replicas.
- pocketbase P3 `tools/store` when used as TTL cache (not when used as general keyed state).

**Corpus regions where lifting seems not useful despite being liftable:**
- gitea G10 `EphemeralCache` — local to one request / one process lifetime.
- caddy C7 `HTTPBasicAuth.Cache` on synchronous auth path when hit rate is near-100% — in-process μs cost vs. Redis ms cost is a regression.

---

## 7. `session-affinity-state`

**Pays off when:**
- Connections are long-lived (websockets, SSE, hijacked HTTP/2 streams).
- Per-connection work is substantial (not just routing).
- Concurrent-connection count grows with user scale.
- Cross-connection invariants are rare.

**Net-negative when:**
- Session lifetime is effectively per-request (no affinity needed).
- Reconnection churn is high (mobile clients) — sticky routing creates rebalance pressure and session-loss visibility.
- Cross-session invariants are load-bearing (mattermost cluster model stresses this — user has multiple concurrent connections, cross-connection broadcast).
- Deployment is single-replica; archetype engages only at >1 replica.

**Code-structural tells that correlate with usefulness:**
- Session ID is connection-accept time (not request time).
- State map keyed by session/connection ID with per-session mutations serialized.
- Explicit session-close path removes state (lifecycle-bounded).
- Connection handling loops are long-running (for-select on channels, not request/response).

**New failure modes introduced:**
- Replica crash loses the sessions it owned (was local already; now requires explicit reconstruction policy).
- Sticky-routing misroutes during replica rebalance.
- Session-to-replica mapping needs its own coordination surface (routing table, consistent-hash ring).

**Operational complexity added:**
- Session-affinity-aware load balancer (consistent-hash on session ID).
- Graceful drain of sessions during replica shutdown.
- Visibility: per-replica session count, session-migration events.

**Consistency / ordering trade-offs:**
- Per-session serialization: preserved (the session's actor lives on one replica).
- Cross-session ordering: relaxed (always was; now distributed).
- Durability of in-session state: unchanged (was always volatile).

**Corpus regions where lifting seems plausibly useful:**
- mattermost MM2 `WebConn` + MM1 hub composite — the canonical websocket scale-out. This is where the PLOS §4.2 story has its strongest corpus demo.
- gitea G11/G12/G13 session stores — moderate-scale self-hosted deployments benefit.
- caddy C6 hijacked-upgrade state — websocket gateway scenarios.

**Corpus regions where lifting seems not useful despite being liftable:**
- Any of the above when the deployment is single-replica / single-node by operational choice (lots of gitea).
- Request-scoped "session" state that is really just per-request data (not enumerated as such in annotations but appears in the TERMINAL tail).

---

## 8. `filesystem-bound-singleton`

**Pays off when:**
- Filesystem is being used as data storage, not configuration/lock files.
- Multiple replicas need to share or scale access to the storage.
- Access pattern matches object-store primitives cleanly (put / get / list under prefix; no random-access mutation of large files).
- Paths are config-driven, and path→object-key translation is mechanical.

**Net-negative when:**
- Filesystem holds lock files or presence signals (gitea process.Manager's lock files) — these are local-by-design.
- Access pattern uses fsync/rename-dance for crash safety that object stores do not preserve for free.
- Latency floor of local disk (μs) is load-bearing for the access pattern; object store adds ms per call.
- Paths encode ordering invariants (parent-before-child) that flat keys don't preserve.

**Code-structural tells that correlate with usefulness:**
- `os.ReadFile` / `os.WriteFile` / `os.Open` on config-driven paths.
- No `os.Rename` as a commit operation (object stores don't atomically rename).
- No `os.File` passed as a streaming handle across a long-running operation (object-store APIs prefer request-scoped streams).
- Path template is a prefix + identifier; identifier maps cleanly to an object key.

**New failure modes introduced:**
- Object store unavailability is hard — no local fallback disk.
- List-after-write eventual consistency window (S3 strong-consistent now, but this hasn't always been true for every backend).
- Latency floor rises uniformly (no μs operations anymore).

**Operational complexity added:**
- Object store credentials / IAM.
- Bucket lifecycle policies, versioning.
- Monitoring: object-store error rates, latency percentiles.

**Consistency / ordering trade-offs:**
- Write ordering across multiple files: relaxed to whatever the object store guarantees.
- Atomic rename: lost (copy + delete is not atomic).
- Local-lock-file semantics: lost (must be replaced with an explicit coordinator).

**Corpus regions where lifting seems plausibly useful:**
- caddy filestorage subsystem — certificate and key storage; multi-instance caddy is a recognized operational shape; object store is a natural target.

**Corpus regions where lifting seems not useful despite being liftable:**
- gitea `modules/process.Manager`'s local lock files — these encode "I am running on this machine" and do not translate to object-store semantics.
- gitea `modules/storage` when the deployment is explicitly single-node (common for self-hosted).
