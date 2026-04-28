# Archetype catalog v1 (opus run)

**SPRINT-0013 deliverable.** Disciplined vocabulary of distribution
archetypes that survived the four gates against the pinned evaluation
corpus. This is one of three parallel runs (opus / gpt-5.4 / gemini);
the synthesis step will merge the three into the canonical catalog.

## Vocabulary discipline (the four gates)

Every archetype must pass all four gates. Per-archetype pass/fail is
recorded. Archetypes failing any gate are retired with a
one-paragraph "why it didn't survive" note kept as research output.

1. **Coverage gate.** Labels ≥2 regions across ≥2 targets, or carries
   an argued exception.
2. **Evidence gate.** Distinguishable from its nearest neighbor by an
   evidence signal the classifier already collects (citing
   `docs/specs/liftability-properties.md` or
   `pkg/compiler/stateclass/`) or by a named signal we can point at
   adding.
3. **Emission gate.** A ≤30-line Go pseudocode emission sketch is
   writeable. Two archetypes with essentially the same sketch merge.
4. **Boundary gate.** Auto-lift / suggest / refuse thresholds are
   stated as concrete evidence conditions.

## v0 vocabulary → v1 outcome summary

| v0 label | v1 outcome | renamed to | note |
|---|---|---|---|
| singleton-actor | kept | `serialized-actor` | narrow sense: serialize access to receiver-scoped state |
| worker-pool-consumer / queue-consumer | kept | `bounded-worker-pool` | state class pair with replicated-service admission |
| periodic-scheduler | kept | `periodic-invocation` | strongest coverage across corpus |
| sharded-keyed-state / sharded-stateful-service | kept | `keyed-partitioned-state` | keyed access invariant is the gate |
| event-bus-publisher | kept | `fanout-publisher` | subscriber side is not a distinct archetype |
| event-bus-subscriber | merged | — | absorbed into `serialized-actor` or `bounded-worker-pool` depending on state effect |
| ttl-cache-managed | kept | `ttl-cache` | externalized-durable-cache state class |
| session-scoped-state | kept | `session-affinity-state` | session-ID keyed state with request-scoped lifetime |
| pipeline-stage | **retired** | — | every observed site collapses into periodic-invocation + serialized-actor, or bounded-worker-pool with a closure |
| ephemeral-worker | **retired** | — | fissioned into `session-affinity-state` (lifecycled) or TERMINAL (fire-and-forget) |
| replicated-stateless-service | kept as ADMITTED baseline | — | not part of AUTO/SUGGEST triage |

Proposed new archetypes (evaluated but **retired**):

| proposed | outcome | reason |
|---|---|---|
| `lifecycle-state-machine` | retired | fails emission gate: no v1 property captures distributed state-machine transitions. Flag for post-v1 work. |
| `websocket-fanout-hub` | retired | fails coverage gate (1 target); expressible as composite `keyed-partitioned-state` + `fanout-publisher` + per-connection send queue |
| `keyed-queue-state-guard` | retired | fails coverage gate (1 target, gitea baseChannel); absorbs cleanly into `keyed-partitioned-state` once broker dedup is assumed |

---

## v1 archetypes

### 1. `serialized-actor`

**Definition.** Stateful struct (exported or package-scoped) whose
operations all mutate receiver-scoped state under a single mutex. No
cross-instance shared state. No pointer escape of mutated fields to
external callers. Pointer-receiver methods are the API; no alias of
the protected state leaks beyond the receiver.

**Candidate state class (ADR-0016 addition).** `serialized-actor`:
rule inference when (a) struct field is `sync.Mutex`/`sync.RWMutex`,
(b) every store site on receiver-owned fields lies inside the
Lock/Unlock span, (c) no pointer-to-protected-field escape is
observable in SSA.

**Evidence signals (cite).**
- `effects.no-param-heap-mutation` (gate, Hold): store-through-param
  must not reach protected state.
- `effects.no-param-escape` (advisory, Hold): prevents pointer-to-field
  aliasing outside the receiver.
- `boundary.no-sync-primitives` (gate, Violate today): this is the
  refusal that auto-lift replaces — once the receiver-scope invariant
  is satisfied, the mutex is semantic and no longer breaks the
  boundary.
- `effects.no-global-writes` (gate, Hold): protected state is
  receiver-local, not package-global.

**≤30-line emission sketch.**

```go
// generated: actor skeleton
type rateLimiterActor struct {
    inner RateLimiter
    ops   chan actorCmd
    done  chan struct{}
}

func (a *rateLimiterActor) run() {
    for op := range a.ops {
        op.reply <- a.inner.apply(op)
    }
    close(a.done)
}

// generated: wire-level entry
func HandleAllow(ctx context.Context, req AllowRequest) (*AllowResponse, error) {
    reply := make(chan actorReply, 1)
    a.ops <- actorCmd{kind: cmdAllow, payload: req, reply: reply}
    r := <-reply
    return r.resp, r.err
}
```

Runtime dependency: a serial-dispatch harness (single goroutine per
actor, channel mailbox). No external broker. Invariants preserved:
operation atomicity (serial execution), receiver-scope isolation.
User-facing API unchanged (method calls become RPC).

**Thresholds (boundary gate).**
- **AUTO** when: protected state is wholly receiver-owned (no
  package-global backing, no escape via returned pointers); mutex
  span encloses every store; no reachability into reflect/unsafe.
- **SUGGEST** when: receiver-scope looks right but the mutex also
  protects external state (external client types, shared globals)
  and the compiler cannot rule out aliasing.
- **TERMINAL** when: the mutex guards a structure that embeds an
  un-externalizable durable client (e.g. SQLite handle) — same
  composite rule that produces `MLV2_EMBEDDED_DB_APP_ROOT`.

**Citations (≥2 targets required).**
- caddy `Handler.connections` + `connectionsMu` (streaming.go:302-324) — C5
- caddy `HTTPBasicAuth.Cache` + `mu *sync.RWMutex` (basicauth.go:105-110) — C7
- pocketbase `Hook[T]` (hook.go:55-57) — P1
- pocketbase `BatchHandler` (batch_handler.go:54-88) — P5
- gitea `queue.Manager` registry (manager.go:18) — G4
- gitea `eventsource.Manager` (manager.go:11) — G6
- gitea `services/cron` task registry (tasks.go:28-31) — G15
- gitea `modules/process.Manager` (manager.go:70-71) — G18
- mattermost cluster-leader-listeners composite (cluster.go:164-187) — MM11
- miniflux `ProxyRotator` (proxyrotator.go:20-51) — M6

**Gate pass:** Coverage ✓ (6 targets), Evidence ✓, Emission ✓, Boundary ✓.

---

### 2. `bounded-worker-pool`

**Definition.** N-goroutine pool reading serializable jobs from a
shared channel (explicit or via semaphore), each worker processes a
job independently and shares no mutable state with peers. Pool size
bounded statically or by config.

**Candidate state class (ADR-0016 addition).** `bounded-worker-pool`:
rule inference when (a) struct has a field of kind `chan T` where T
is serializable, (b) a fixed-count loop spawns goroutines consuming
it, (c) job handler accesses no package-global mutable state, (d)
handler has an error-return vocabulary (`contract.error-last`).

**Evidence signals.**
- `boundary.no-streaming-values` (gate, Violate today): channels on
  the boundary. Auto-lift replaces this when job type is serializable.
- `boundary.serializable-via-custom-encoding` (gate, Hold): job type
  must serialize.
- `lifecycle.long-running-loop` (bias): consumer loop.
- `effects.no-global-writes` (gate, Hold on handler): no shared mutable
  state in handler body.
- `effects.no-param-escape` (advisory, Hold): handler does not leak
  job-derived pointers.

**Emission sketch.**

```go
// generated: broker-backed queue
var q = broker.Subscribe("jobs-rateLimit")

func worker(ctx context.Context) {
    for msg := range q {
        var job Job
        if err := json.Unmarshal(msg.Body, &job); err != nil {
            msg.Nack()
            continue
        }
        if err := handle(ctx, job); err != nil {
            msg.Nack()
            continue
        }
        msg.Ack()
    }
}

// user-level API preserved: Enqueue(job) publishes to broker
func Enqueue(ctx context.Context, job Job) error {
    body, _ := json.Marshal(job)
    return broker.Publish("jobs-rateLimit", body)
}
```

Runtime dependency: message broker client (NATS / SQS / Pub-Sub /
Rabbit). Invariants preserved: at-least-once delivery, worker
isolation, backpressure via broker.
User-facing API: the enqueue call site replaces its direct channel
send with a broker publish; local callers unchanged.

**Thresholds.**
- **AUTO** when: job type is serializable, handler holds
  `effects.no-global-writes`, pool size is static-config driven,
  per-job ordering is not load-bearing.
- **SUGGEST** when: job ordering is potentially load-bearing
  (within-key FIFO required), or handler reaches a shared resource
  that is not yet declared externalized.
- **TERMINAL** when: handler has unbounded fanout goroutines of its
  own, or pool is unbounded (fallback-spawn-on-full).

**Citations.**
- listmonk `Manager.worker` + `campMsgQ` (manager.go:462-559) — L2
- gitea `modules/queue.WorkerPoolQueue` (workerqueue.go:22) — G1
- mattermost `PushNotificationsHub` (notification_push.go:44-52) — MM6
- miniflux `worker.Pool` (ADMITTED baseline)

**Gate pass:** Coverage ✓ (4 targets), Evidence ✓, Emission ✓, Boundary ✓.

---

### 3. `periodic-invocation`

**Definition.** Long-running goroutine whose body is a
`time.Ticker`/`time.Sleep`-driven loop invoking an idempotent
function. No captured mutable state beyond what is immediately
consumed; external state (DB, broker, external cache) is where work
persists.

**Candidate state class.** `periodic-invocation`: rule inference when
(a) goroutine body contains a `for {}` with a `time.Ticker.C` or
`time.Sleep` on every branch, (b) loop body calls a function that
matches `boundary.*` gates for remote rewrite, (c) no captured
mutable state outside what the idempotent body produces.

**Evidence signals.**
- `lifecycle.long-running-loop` (bias, Hold): ticker-driven loop.
- `lifecycle.no-async-fork` (bias, Violate today, but for an
  admission-compatible reason): the goroutine is the scheduler
  trigger; distribution moves it to a platform scheduler.
- `effects.no-global-writes` (gate, Hold) on the body.
- `contract.error-last` (gate, Hold) on the body.

**Emission sketch.**

```go
// generated: scheduled-invocation skeleton
// body replaces: go func() { t := time.NewTicker(d); for { <-t.C; doWork(ctx) } }()

func Handler(ctx context.Context, _ ScheduledTrigger) error {
    return doWork(ctx)
}

// infra: cron registration (at build time, from config)
// CRON_EXPR = "*/15 * * * *"
// invocation replaces the in-process goroutine with platform-triggered Handler.
```

Runtime dependency: platform scheduler (cron, k8s CronJob, serverless
scheduled trigger). Invariants preserved: idempotency requirement
(explicit), interval is config-driven and survives redeployment.
User-facing API: `Start`/`Stop` calls that previously spawned/joined
the goroutine become no-ops — the scheduler owns lifecycle.

**Thresholds.**
- **AUTO** when: body is idempotent or tolerates occasional
  skipped-or-duplicated ticks; no captured mutable state; interval
  is config-driven (not derived from in-process state).
- **SUGGEST** when: body mutates local state that persists across
  invocations (e.g. counter increment) and is not reducible to
  external state.
- **TERMINAL** when: interval is dynamically derived from previous
  tick's result (self-tuning loop) — no generic scheduler captures
  this.

**Citations.**
- listmonk `scanCampaigns` (manager.go:422-458) — L1
- listmonk `runMailboxScanner` (bounce.go:135-143) — L3
- caddy `stayUpdated` (sessiontickets.go:114-148) — C1
- caddy `keepStorageClean` (tls.go:1050-1072) — C2
- pocketbase `Cron` (cron.go:176-206) — P2
- miniflux `feedScheduler`, `cleanupScheduler`, watchdog, metrics —
  M1, M2, M3, M4
- gitea `services/cron.Task` + gocron (tasks.go:36) — G14
- mattermost `email_batching` (email_batching.go:71-159, post-transform) — MM8

**Gate pass:** Coverage ✓ (6 targets — strongest coverage in the
corpus), Evidence ✓, Emission ✓, Boundary ✓.

---

### 4. `keyed-partitioned-state`

**Definition.** `map[K]V` protected by mutex or RWMutex, where every
access is keyed (no key-free iteration that assumes all entries live
in one process). Keys partition the state into independent shards.

**Candidate state class.** `keyed-partitioned-state`: rule inference
when (a) protected field is map-typed, (b) every access site uses a
key derived from request input, (c) no iteration over all entries
appears in hot paths (background cleanup iterations are tolerable
with a separate advisory).

**Evidence signals.**
- `boundary.no-sync-primitives` (gate): the mutex refuses today.
- `effects.no-global-writes` (gate): map must be receiver-owned.
- New proposed signal: `keyed-access-invariant` — every call site
  reaching the map indexes by key from input. Classifier addition
  needed; flag as follow-up.

**Emission sketch.**

```go
// generated: sharded state service
// key-routing at the gateway

type shard struct {
    mu   sync.Mutex
    data map[Key]Value
}
var shards [N]*shard

func route(k Key) *shard { return shards[hash(k)%N] }

// generated RPC handlers preserve user-facing API shape:
func Get(ctx context.Context, k Key) (Value, error) {
    s := route(k)
    s.mu.Lock(); defer s.mu.Unlock()
    return s.data[k], nil
}
```

Runtime dependency: consistent-hash router, or a managed KV store
(Redis cluster, DynamoDB). Invariants preserved: per-key atomicity,
no cross-key linearization (which the original mutex implementation
already violates if it uses per-key locks).
User-facing API: keyed operations are RPC; iteration requires new
API (not part of the lift).

**Thresholds.**
- **AUTO** when: every call site is keyed, iteration (if any) is
  background-cleanup and idempotent-per-shard, no cross-key
  invariants visible.
- **SUGGEST** when: key-free iteration appears in a hot path (e.g.
  GetAll, GetSize) with user-visible semantics.
- **TERMINAL** when: the map encodes cross-key invariants (e.g. sum
  of all values must equal X) that sharding breaks.

**Citations.**
- listmonk `Manager.pipes` + `links` (manager.go:72-81) — L5
- caddy `Handler.connections` — composite with C5
- pocketbase `tools/store.Store[K,T]` (store.go:12-40) — P3
- gitea `queue.baseChannel.set` (base_channel.go:17) — G2
- gitea `modules/process.Manager.processMap` — G18 (composite)
- mattermost `Hub.hubConnectionIndex` (web_hub.go:77-120) — MM1
  (composite with fanout-publisher)

**Gate pass:** Coverage ✓ (5 targets), Evidence ✓ (one proposed
signal pending), Emission ✓, Boundary ✓.

---

### 5. `fanout-publisher`

**Definition.** Producer struct holding a mutex-protected collection
of subscriber channels (`[]chan T` or `map[K]chan T`). `Publish`
iterates and sends. Subscribers are independent; event is
serializable; subscriber set is discoverable.

**Candidate state class.** `fanout-publisher`: rule inference when
(a) struct holds `[]chan T` or `map[K]chan T` under mutex, (b)
Publish method iterates-and-sends over the slice/map, (c) event type
T is serializable, (d) subscriber-register entry point exists.

**Evidence signals.**
- `boundary.serializable-via-custom-encoding` (gate): event type.
- `effects.no-global-writes` (gate): subscriber set receiver-owned.
- `lifecycle.long-running-loop` absent on publisher (bias): publish
  is synchronous per call.

**Emission sketch.**

```go
// generated: managed broker publisher
type events struct { topic string }

func New(topic string) *events { return &events{topic: topic} }

func (e *events) Publish(ctx context.Context, ev Event) error {
    body, err := json.Marshal(ev)
    if err != nil { return err }
    return broker.Publish(ctx, e.topic, body)
}

// subscribers become named services consuming the topic.
// user-level Subscribe() API retained as thin client over broker.Subscribe().
```

Runtime dependency: pub/sub broker (NATS / Kafka / Pub-Sub).
Invariants preserved: at-least-once delivery to each subscriber,
subscriber independence. User-facing API: `Publish` unchanged;
`Subscribe` now returns a channel fed by a background goroutine
consuming the broker.

**Thresholds.**
- **AUTO** when: event type serializable; subscribers are independent
  (no shared state across subscribers); no ordering-across-events
  requirement.
- **SUGGEST** when: event ordering is required across subscribers, or
  subscriber set is dynamic at high churn (broker-subscription API
  bandwidth becomes a constraint).
- **TERMINAL** when: fanout encodes a transaction that must atomically
  apply across all subscribers (distributed-transaction territory).

**Citations.**
- listmonk `Events.Publish` (events.go:41-76) — L4
- pocketbase `Broker` (broker.go:11-65) — P4
- gitea `eventsource.Messenger` (messenger.go:9) — G7
- mattermost cluster `Publish` (cluster.go:189-234, already ADMITTED) —
  MM7

**Gate pass:** Coverage ✓ (4 targets), Evidence ✓, Emission ✓, Boundary ✓.

---

### 6. `ttl-cache`

**Definition.** Key-value cache protected by mutex (or `sync.Map`)
with TTL-based expiry, either via a background cleanup goroutine or
an on-access expiry check. Contents are ephemeral; source of truth
is elsewhere.

**Candidate state class.** `ttl-cache`: rule inference when (a)
protected field is map-typed with value type carrying an expiry
timestamp (or is `sync.Map`), (b) a separate goroutine or on-access
check expires entries, (c) cache misses fall through to a loader
function, (d) no pointer-to-in-process-state stored as value.

**Evidence signals.**
- `effects.no-global-writes` (gate): cache state receiver-owned.
- `boundary.serializable-via-custom-encoding` (gate): cache value
  type must serialize (required for managed cache).
- New proposed signal: `cache-value-no-pointer-escape` — value type
  carries no pointer to other in-process state. Flag as follow-up.

**Emission sketch.**

```go
// generated: managed-cache adapter
type cache struct {
    client redis.UniversalClient
    prefix string
    ttl    time.Duration
}

func (c *cache) Get(ctx context.Context, k Key) (Value, bool, error) {
    body, err := c.client.Get(ctx, c.prefix+k).Bytes()
    if errors.Is(err, redis.Nil) { return Value{}, false, nil }
    if err != nil { return Value{}, false, err }
    var v Value
    return v, true, json.Unmarshal(body, &v)
}

func (c *cache) Set(ctx context.Context, k Key, v Value) error {
    body, _ := json.Marshal(v)
    return c.client.Set(ctx, c.prefix+k, body, c.ttl).Err()
}
```

Runtime dependency: managed cache client (Redis, memcached).
Invariants preserved: TTL semantics, cache-miss fallthrough.
User-facing API: `Get/Set` preserved; background cleanup goroutine
removed (managed cache handles eviction).

**Thresholds.**
- **AUTO** when: value type serializable and carries no in-process
  pointers; TTL is value-agnostic; source-of-truth is elsewhere.
- **SUGGEST** when: cache value holds function-pointer / callback, or
  a pointer to in-process state; or TTL depends on in-process side
  effects.
- **TERMINAL** when: cache *is* the source of truth (no loader).

**Citations.**
- listmonk `Auth.apiUsers` + prune loop (auth.go:62-110) — L6
- listmonk `tmptokens` (tmptokens.go:29-42) — L7
- caddy `HTTPBasicAuth.Cache` — C7 (overlap with serialized-actor; the
  overlap is expected — small serializable values make this
  cache-shaped, pointer-heavy values make it actor-shaped)
- pocketbase `tools/store.Store[K,T]` — P3 (overlap with
  keyed-partitioned-state; ttl-cache when entries carry expiry,
  keyed-partitioned-state when they don't)
- gitea `EphemeralCache` (ephemeral.go:20) — G10
- mattermost session/status cache (session.go:44-97) — MM4, MM5

**Gate pass:** Coverage ✓ (5 targets), Evidence ✓ (one proposed
signal pending), Emission ✓, Boundary ✓.

---

### 7. `session-affinity-state`

**Definition.** State keyed by a session / connection ID (not
request-input-derived), with lifetime bounded by connection lifecycle.
State mutations are per-session; no cross-session shared mutable
state.

**Candidate state class.** `session-affinity-state`: rule inference
when (a) state map is keyed by a session-ID field, (b) key ingress
is at connection-accept time (not request-time), (c) state mutations
are serialized per-session, (d) state is ephemeral (removed on
session close).

**Evidence signals.**
- `effects.no-global-writes` (gate): state is per-session-instance.
- `boundary.no-streaming-values` (gate, Hold if channel is internal
  to the per-session actor).
- New proposed signal: `session-id-keyed-access` — access invariant
  is stronger than keyed-partitioned-state's because ingress is
  connection-accept. Flag as follow-up.

**Emission sketch.**

```go
// generated: session-affinity service
// gateway-level routing: session-ID → replica

type sessionState struct { data map[any]any }
var sessions sync.Map  // replica-local view

func routeTo(sid SessionID) Replica { return affinity.Route(sid) }

// per-session actor lives on the routed replica for the session's lifetime.
// when connection closes, actor exits and releases replica-local state.
```

Runtime dependency: session-affinity-aware load balancer (consistent
hash on session ID, sticky session routing).
Invariants preserved: per-session serialization, session-scoped
lifetime. User-facing API: connection APIs unchanged; session state
becomes non-mig­ratable mid-connection.

**Thresholds.**
- **AUTO** when: session ID is stable across the connection; state is
  purely per-session; no cross-session invariants; replicas can carry
  the session to its close without migration.
- **SUGGEST** when: session state references cross-session objects
  (e.g. shared team state from mattermost) — needs a managed backing
  store behind the per-session actor.
- **TERMINAL** when: session state is connection-lifetime-unbounded
  (e.g. user-level across multi-connection logins with consistency
  requirements); out of scope for v1.

**Citations.**
- caddy `handleUpgradeResponse` per-request hijack (streaming.go:147-159) — C6
- gitea `session.DBStore` / `RedisStore` / `VirtualStore` (db.go:21, etc.) — G11, G12, G13
- mattermost `WebConn` session state (web_conn.go:88-149) — MM2

**Gate pass:** Coverage ✓ (3 targets), Evidence ✓ (one proposed
signal pending), Emission ✓, Boundary ✓.

---

## Retirements (kept as research output)

### `pipeline-stage` — retired

Observed in caddy (STEK rotate goroutines) and gitea (queue handler
closures). Four sites across two targets — passes the coverage gate
on count, but fails the emission gate: the pseudocode sketch for
"one-stage channel-to-channel transform" collapses into either
`periodic-invocation` (caddy STEK: timer → produce-once → emit) or
`bounded-worker-pool` with a closure (gitea: queue handler). The
two-service chained-via-queue emission is not distinguishable from
those two archetypes composed in sequence. Retired to prevent the
catalog accumulating a name for what is just "one archetype calling
another". If a future corpus shows a true long-stage pipeline
(multiple intermediate stages with meaningful per-stage state), the
archetype should be re-introduced with a clearer distinguishing
signal.

### `ephemeral-worker` — retired (fissioned)

Observed across miniflux (fire-and-forget integration dispatches),
caddy (per-request hijack goroutines), pocketbase (`FireAndForget`
helper), listmonk (WaitGroup cleanup). The label covers two
fundamentally different things:

1. **Lifecycled ephemeral spawns** — caddy's WebSocket hijack pair.
   These are captured by `session-affinity-state` (the goroutine is
   the session actor).
2. **Unlifecycled fire-and-forget** — miniflux fever/googlereader,
   pocketbase `FireAndForget`. These are TERMINAL in v1: no archetype
   provides recovery, retry, or ordering for anonymous spawn over
   mutable closure.

Fissioning removes an ambiguous label. Every site the label covered
now routes to a cleanly-described outcome (session-affinity-state,
or TERMINAL with an explicit "no archetype captures fire-and-forget").

### `lifecycle-state-machine` (proposed new, 2 targets) — retired

Gitea `graceful.Manager` (init → running → shutting-down → terminate)
and `process.Manager` are textbook distributed-state-machine shapes.
Coverage gate is borderline (2 targets, 2 sites). **Fails the emission
gate**: there is no ≤30-line sketch for "distributed state machine
with ordered transitions" that v1 can express. The Raft / CRDT
literature has full-stack solutions; none reduce to a single
archetype transform at a reasonable engineering cost. Retired for v1.
Flagged as high-value follow-up: a future sprint should evaluate
whether coordinator-backed lifecycle (e.g. k8s leader-election,
etcd-compare-and-swap) fits as a distinct archetype.

### `websocket-fanout-hub` (proposed new, 1 target) — retired

Mattermost `Hub` (14 chan fields + hubConnectionIndex map + sharded
per-user hub affinity). **Fails the coverage gate** (1 target). It is
expressible as a composite of existing archetypes:

- `keyed-partitioned-state` for hubConnectionIndex (per-user keyed)
- `fanout-publisher` for broadcast-to-connections
- `session-affinity-state` for per-connection send queues

Composite expression is not lossless — the per-connection-send-queue
backpressure mechanism has no archetype name — but in v1 that gap
is better characterized as an evidence gap ("no property captures
connection-scoped backpressure") than as a new archetype.

### `keyed-queue-state-guard` (proposed new, 1 target) — retired

Gitea `modules/queue.baseChannel` (mutex protecting uniqueness Set
around an internal channel). Coverage gate fails (1 target). Most
message brokers provide built-in deduplication, which means the
transform for this pattern is `bounded-worker-pool` + broker-dedup
config, not a new archetype. Retired.

---

## ADR / v2-contract naming-collision check

Cross-checked each surviving archetype name against ADR-0015
(canonical-shape classifier), ADR-0016 (state-class inference),
ADR-0017, ADR-0018, and `docs/specs/monolift-v2-contract.md`:

- `serialized-actor` — no collision with `immutable-captured-config` /
  `replicated` / `externalized-durable` / `stateless` state classes.
- `bounded-worker-pool` — no collision with transport-shape
  `channel-consumer` (that was ADR-0006, used today as a transport
  selector, not a state class).
- `periodic-invocation` — no collision.
- `keyed-partitioned-state` — no collision.
- `fanout-publisher` — no collision.
- `ttl-cache` — no collision.
- `session-affinity-state` — no collision.

No renames needed.

---

## v1 catalog state

- **7 archetypes survived** all four gates.
- **5 retirements** recorded (pipeline-stage, ephemeral-worker,
  lifecycle-state-machine, websocket-fanout-hub,
  keyed-queue-state-guard).
- **3 new evidence signals proposed** for the classifier — promoted
  to the follow-up list (`keyed-access-invariant`,
  `cache-value-no-pointer-escape`, `session-id-keyed-access`).
- **7 candidate state classes** for ADR-0016 — the primary engineering
  output. See follow-ups.

## Cross-target citation matrix

| archetype | listmonk | caddy | pocketbase | miniflux | gitea | mattermost | targets |
|---|---|---|---|---|---|---|---|
| serialized-actor | — | 2 | 2 | 1 | 4 | 1 | 5 |
| bounded-worker-pool | 1 | — | — | (adm) | 1 | 1 | 4 |
| periodic-invocation | 2 | 2 | 1 | 4 | 1 | 1 | 6 |
| keyed-partitioned-state | 1 | 1 | 1 | — | 2 | 1 | 5 |
| fanout-publisher | 1 | — | 1 | — | 1 | 1 (adm) | 4 |
| ttl-cache | 2 | 1 | 1 | — | 1 | 2 | 5 |
| session-affinity-state | — | 1 | — | — | 3 | 1 | 3 |

Approximate region counts; see per-target annotations for exact
cites. "adm" = ADMITTED baseline region (counts toward cross-target
coverage for gate purposes).
