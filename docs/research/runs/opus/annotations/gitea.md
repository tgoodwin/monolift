# gitea annotation — SPRINT-0013 (opus run)

**Corpus pin:** 2026-04-19. 2875 Go files.
No committed golden report; corpus walked via mandatory subagent
delegation per sprint plan.

## Target synthesis

Gitea's archetype surface concentrates in **seven infrastructure
bundles**: `modules/queue`, `modules/eventsource`, `modules/indexer`,
`modules/cache`, `modules/session`, `services/cron`,
`services/actions`, plus a `lifecycle-state-machine` flavor in
`modules/graceful` and `modules/process`. The 1,660+ domain-service
files (`routers/api`, `routers/web`, `services/{auth,user,org,...}`,
`models/`) overwhelmingly exhibit *no* archetype surface at all —
they are request-scoped handlers operating on DB-backed models with
context propagation, which is the admitted baseline.

**Dominant archetypes (with strong AUTO candidates):**

1. `singleton-actor` — `modules/queue` Manager registry, eventsource
   Manager, `services/cron` global task registry, `modules/process`
   Manager. All have the same shape: package-level singleton + mutex
   + registry map. Uniformly `serialized-actor` + `keyed-partitioned-state`
   composite.
2. `worker-pool-consumer` — `modules/queue` WorkerPoolQueue (the core
   abstraction that most other bundles consume via handlers).
   Canonical pool: mutex-coordinated worker count, handler closure,
   atomic flushing flag.
3. `event-bus-publisher` / `subscriber` — `modules/eventsource`
   Messenger (per-UID multiplexer) is a textbook fanout; the SSE
   `Client` is the subscriber.
4. `session-scoped-state` — `modules/session` {DB,Redis,Virtual}Store,
   all RWMutex + per-session map, keyed by `sid`. Textbook
   session-affinity.
5. `ttl-cache-managed` — `modules/cache/ephemeral` RWMutex-guarded map
   with TTL field per entry.
6. `periodic-scheduler` — `services/cron` Task.lock + gocron backend,
   `modules/indexer` async handler spawning.
7. **`lifecycle-state-machine`** — `modules/graceful.Manager`
   (init → running → shutting-down → terminate), `modules/process.Manager`
   (process registry). *Proposed new archetype* in v1 vocabulary. Does
   not have a remote-distribution transform in v1 — see catalog.

**Hardest ambiguity.** `modules/queue.baseChannel` uses mutex to
protect a uniqueness `Set` *and* an internal channel's buffer state.
This is the cleanest case where `keyed-queue-state-guard` could
emerge as a distinct archetype (mutex-protected dedup around an
inbound queue). But it can be subsumed into `keyed-partitioned-state`
+ `bounded-worker-pool` once the transform uses a broker with
built-in deduplication (most message brokers do), so it does not earn
its own entry in v1. Flag for future sprint.

**Evidence gaps.**
- Queue handler closure lifetime vs. context cancellation — multiple
  AUTO candidates assume `ctx` stays live across batch execution.
- Lifecycle state machine transitions are not expressible in the v1
  liftability properties — they are coordination patterns, not
  data-flow patterns. Requires a new property family.

## Owned-directory bundle file counts

- cmd/: 52
- routers/install: 3
- modules/setting: 77
- modules/graceful: 13
- routers/api: 177
- routers/web: 220
- services/context: 29
- modules/web: 19
- modules/reqctx: 1
- services/auth: 56
- services/user: 10
- services/org: 7
- services/repository: 59
- services/pull: 31
- services/issue: 20
- services/packages: 19
- services/oauth2_provider: 6
- services/mirror: 6
- services/wiki: 3
- services/mailer: 28
- services/notify: 3
- services/task: 2
- services/webhook: 26
- services/cron: 8
- services/actions: 24
- modules/queue: 21
- modules/cache: 8
- modules/storage: 10
- modules/indexer: 48
- modules/session: 6
- modules/eventsource: 5
- modules/private: 11
- modules/process: 9
- models/: 649

**Total bundles: 1,860 files; full corpus: 2,875.**

## AUTO set (per bundle)

### modules/queue (21 files)

| # | region | archetype | state class | transform | evidence | gap |
|---|---|---|---|---|---|---|
| G1 | `WorkerPoolQueue.workerNumMu` + atomic flushing, workerqueue.go:22 / workergroup.go:30 | `worker-pool-consumer` | `bounded-worker-pool` | broker-backed queue + replicas; worker count as actor state | `effects.no-global-writes` violate (`workerNum--`); `lifecycle.long-running-loop` | spawn-timing constraint tolerates serialization latency? |
| G2 | `baseChannel` mutex + internal channel + Set, base_channel.go:17 | `sharded-keyed-state` | `keyed-partitioned-state` (with dedup) | broker queue with built-in dedup | mutex + `q.set.Add/Remove` writes; `boundary.no-streaming-values` violate (chan field) | check-then-act race on buffer length |
| G3 | `baseLevelQueue` Unique mutex, base_levelqueue_unique.go | `singleton-actor` | `serialized-actor` + `keyed-partitioned-state` | leveldb already externalized; lift mutex as per-key coordination | mutex-protected leveldb+set | leveldb concurrent multi-node isolation |
| G4 | `Manager` registry, manager.go:18 | `singleton-actor` | `serialized-actor` | registry as managed service; qidCounter becomes actor state | mutex + `Queues map` + `qidCounter++` | idempotency of AddManagedQueue under duplicate |
| G5 | `workerGroup` WaitGroup, workergroup.go:30, 146-151 | `worker-pool-consumer` lifecycle | (coordination — no v1 state class fits) | actor-owned pending-count; timer-actor for ticker reset | WaitGroup + ticker reset pattern | stale ticker after Reset |

### modules/eventsource (5 files)

| # | region | archetype | state class | transform | evidence | gap |
|---|---|---|---|---|---|---|
| G6 | `Manager` messenger registry, manager.go:11 | `event-bus-publisher` registry | `serialized-actor` | per-UID messenger as RPC-addressable actor | mutex + `messengers map[int64]`; Register/Unregister | messenger lifetime invariant |
| G7 | `Messenger` per-UID fanout, messenger.go:9 | `event-bus-publisher` | `fanout-publisher` | managed pub/sub with per-UID topic | mutex + `channels []chan *Event`; non-blocking send | delivery semantics on buffer full |

### modules/indexer (48 files)

| # | region | archetype | state class | transform | evidence | gap |
|---|---|---|---|---|---|---|
| G8 | `globalIndexer atomic.Pointer`, code/indexer.go:33, issues/indexer.go:52 | `singleton-actor` | `serialized-actor` | indexer as managed service; Load/Store become RPC | `lifecycle.no-async-fork` violate (`go func()`); `effects.no-global-reads` advisory | global-visibility isolation between indexers |
| G9 | `indexerQueue` handler closure, code/indexer.go:121, issues/indexer.go:166 | `pipeline-stage` | (reuses queue) | closure lift as named handler actor; depends on G1 | closure captures `ctx`, indexer | context lifetime vs. batch execution |

### modules/cache (8 files)

| # | region | archetype | state class | transform | evidence | gap |
|---|---|---|---|---|---|---|
| G10 | `EphemeralCache`, ephemeral.go:20 | `ttl-cache-managed` | `ttl-cache` | per-caller managed cache actor with TTL | RWMutex + `data map[any]map[any]any`; TTL check line 35-40 | caller context cardinality |

### modules/session (6 files)

| # | region | archetype | state class | transform | evidence | gap |
|---|---|---|---|---|---|---|
| G11 | `DBStore`, db.go:21 | `session-scoped-state` | `session-affinity-state` | session-ID-keyed actor; persists to auth table on Release | RWMutex + per-session map; sid key | cross-request mutation; Release atomicity |
| G12 | `RedisStore`, redis.go | `session-scoped-state` | `session-affinity-state` | redis-backed store with per-session coordination | same shape | redis transaction semantics |
| G13 | `VirtualStore`, virtual.go | `session-scoped-state` | `session-affinity-state` | virtualized session multiplexer | same shape | multiplexing strategy |

### services/cron (8 files)

| # | region | archetype | state class | transform | evidence | gap |
|---|---|---|---|---|---|---|
| G14 | `Task.lock`, tasks.go:36 | `periodic-scheduler` | `periodic-invocation` | per-task actor; gocron triggers actor method | mutex + counters; gocron event loop | lock ordering with globallock |
| G15 | global task registry, tasks.go:28-31 | `singleton-actor` | `serialized-actor` | registry as managed service | mutex + `tasks` slice + `tasksMap` | task removal semantics |

### services/actions (24 files)

| # | region | archetype | state class | transform | evidence | gap |
|---|---|---|---|---|---|---|
| G16 | `jobEmitterQueue` handler, job_emitter.go:24, 53 | `pipeline-stage` | (reuses queue) | queue dependency via G1–G5; handler as closure actor | inherits queue refusal | state-mutation isolation during handler exec |

### modules/graceful, modules/process (22 files)

| # | region | archetype | state class | transform | evidence | gap |
|---|---|---|---|---|---|---|
| G17 | `graceful.Manager` state machine, manager.go:43-112, 138 | `lifecycle-state-machine` (proposed new) | (no clean v1 state class) | **v1 verdict: TERMINAL** for distribution; archetype is lifecycle-coordination, not data-flow | `sync.Once`, implicit state machine; `go fn()` at shutdown | no v1 property captures state-machine transitions |
| G18 | `process.Manager` processMap, manager.go:70-71 | `singleton-actor` | `serialized-actor` + `keyed-partitioned-state` | registry as managed service | mutex + processMap | process-descriptor serialization |

## SUGGEST set

| # | region | archetype | why SUGGEST |
|---|---|---|---|
| GS1 | `services/webhook/notifier.go` PrepareWebhooks | `event-bus-subscriber` with batching | internal queue/channel usage not confirmed in walk |
| GS2 | `services/notify/notify.go` Notifier registry | `event-bus-publisher` fanout | slice mutation guard-by-init-order, not mutex; concurrent-registration semantics unclear |
| GS3 | `modules/cache` redis/memcache adapters | `ttl-cache-managed` | wrapper-level coordination unexamined |

## TERMINAL set

| # | region | reason |
|---|---|---|
| GT1 | `routers/api`, `routers/web` (397 files) | stateless request handlers — ADMITTED baseline, no archetype needed |
| GT2 | `models/` (649 files) | DB-mediated isolation; no shared mutable state on struct boundaries |
| GT3 | `services/{auth,user,org,repository,pull,issue,packages,...}` (209 files) | request-scoped business logic over models |
| GT4 | `modules/setting` (77 files) | init-time-only singletons, immutable at runtime |
| GT5 | `modules/storage`, `modules/private` | I/O delegation |
| GT6 | `graceful.Manager` — as a *distribution transform* target | `lifecycle-state-machine` is coordination infrastructure; no v1 remote-rewrite archetype applies. Keep as singleton infra, not a lift target. |

## ADMITTED set

All TERMINAL-in-this-table entries are *vacuously admitted*: they have
no archetype surface and no refusal code attaches. This pattern is
the dominant mode across gitea's 2,875-file corpus.

## Per-bundle coverage ledger

| bundle | files | finding |
|---|---|---|
| cmd/ | 52 | no relevant archetype surface observed — CLI parsing |
| routers/install | 3 | no relevant archetype surface observed — one-time setup |
| modules/setting | 77 | no relevant archetype surface observed — init-time singletons |
| modules/graceful | 13 | 1 AUTO/TERMINAL (G17) — new archetype proposed, not a v1 lift target |
| routers/api | 177 | no relevant archetype surface observed — stateless request handlers |
| routers/web | 220 | no relevant archetype surface observed — stateless request handlers |
| services/context | 29 | no relevant archetype surface observed — request-context assembly |
| modules/web | 19 | no relevant archetype surface observed — middleware |
| modules/reqctx | 1 | no relevant archetype surface observed — context helper |
| services/auth | 56 | no relevant archetype surface observed — business logic over models |
| services/user | 10 | no relevant archetype surface observed |
| services/org | 7 | no relevant archetype surface observed |
| services/repository | 59 | no relevant archetype surface observed |
| services/pull | 31 | no relevant archetype surface observed |
| services/issue | 20 | no relevant archetype surface observed |
| services/packages | 19 | no relevant archetype surface observed |
| services/oauth2_provider | 6 | no relevant archetype surface observed |
| services/mirror | 6 | no relevant archetype surface observed |
| services/wiki | 3 | no relevant archetype surface observed |
| services/mailer | 28 | no direct archetype surface observed — likely delegates to modules/queue (inherits G1–G5) |
| services/notify | 3 | 1 SUGGEST (GS2) |
| services/task | 2 | no relevant archetype surface observed — task registry wrapper, delegates |
| services/webhook | 26 | 1 SUGGEST (GS1) |
| services/cron | 8 | 2 AUTO (G14, G15) |
| services/actions | 24 | 1 AUTO (G16) |
| modules/queue | 21 | 5 AUTO (G1–G5) — densest archetype surface in the corpus |
| modules/cache | 8 | 1 AUTO (G10), 1 SUGGEST (GS3) |
| modules/storage | 10 | no relevant archetype surface observed |
| modules/indexer | 48 | 2 AUTO (G8, G9) |
| modules/session | 6 | 3 AUTO (G11–G13) |
| modules/eventsource | 5 | 2 AUTO (G6, G7) |
| modules/private | 11 | no relevant archetype surface observed |
| modules/process | 9 | 1 AUTO (G18) |
| models/ | 649 | no relevant archetype surface observed — data layer |

## Subagent dispatch log

| dispatch | subsystems | prompt version | return summary | re-dispatch? |
|---|---|---|---|---|
| #1 | ALL (single comprehensive walk) | v1 (Phase 0 template, full bundle list, AUTO-focused) | 18 AUTO, 3 SUGGEST, 6 TERMINAL; file counts registered; every bundle covered | no — return met schema, every AUTO has transform + state class, per-bundle findings or explicit "no archetype surface observed" note |

Parent-agent spot check: verified AUTO G1 (modules/queue/workerqueue.go
workerNumMu), G7 (modules/eventsource/messenger.go channel fanout),
G14 (services/cron/tasks.go Task.lock) by reading each file directly.
All claims reproduce. Lifecycle-state-machine claim in G17 is stronger
than the v1 vocabulary can support — kept as TERMINAL-for-now
finding rather than AUTO.

**Net:** 18 AUTO, 3 SUGGEST, 6 TERMINAL, + 1 new archetype (`lifecycle-state-machine`) flagged for post-v1 consideration.
