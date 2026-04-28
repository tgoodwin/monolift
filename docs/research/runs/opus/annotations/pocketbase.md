# pocketbase annotation — SPRINT-0013 (opus run)

**Corpus pin:** 2026-04-19. 445 Go files.
Golden report: `test/e2e/targets/pocketbase/golden/report.json` (pragma
verdict `refuse-blocking` on `pocketbase-app`).

## Target synthesis

Pocketbase is the canonical **TERMINAL** case: its app root wraps an
embedded SQLite DB (4 `dbx.DB` instances, transaction isolation via
`RunInTransaction`, checkpoint/WAL semantics). That terminal refusal
is load-bearing and must not be lifted. **The interesting question**
per the sprint plan is: once the embedded-DB core is fenced, what
around it cleanly fits named archetypes?

The peripheral surface organizes into five liftable loci:

1. **Hook-event dispatch** (`tools/hook/`) — RWMutex-protected handler
   chain, synchronous, deterministic priority sort. Textbook
   `singleton-actor` (narrow sense: serialize access to a shared
   object, no cross-replica sharding).
2. **Cron scheduler** (`tools/cron/`) — ticker + `go j.Run()` per due
   job. `periodic-scheduler`.
3. **Subscriptions broker** (`tools/subscriptions/`) — RWMutex per-client
   store + unbuffered channels for message dispatch. Clients register,
   receive async messages. `event-bus-publisher`.
4. **Generic keyed store** (`tools/store/`) — RWMutex-protected map +
   shrink heuristic; underlies rate-limiter and caches. Textbook
   `sharded-keyed-state` / `ttl-cache-managed`.
5. **Batch logger** (`tools/logger/batch_handler.go`) — mutex + batch
   buffer. `singleton-actor` with periodic flush.

**Hardest ambiguity.** The JS VM pool (`plugins/jsvm/pool.go`) is a
work-stealing pool with nested Mutex + per-item Mutex + busy-flag
atomicity, plus *unbounded* fallback VM allocation. Structurally a
`singleton-actor` wrapping a `worker-pool-consumer`, but the unbounded
fallback breaks the `bounded-worker-pool` candidate state class's core
invariant. Either split (actor + pool) or SUGGEST with explicit
bounded-pool requirement.

**Evidence gaps.**
- Hook `Trigger()` synchronization semantics — does it serialize
  internally or delegate to caller? Needs evidence rule beyond
  `syncPrimitiveRule`.
- Subscription message-send loop: is there an async per-client pump,
  or purely synchronous sends from broker? Affects whether the broker
  is a `fanout-publisher` or a pure directory.

## AUTO set

| # | subsystem | region (file:line) | archetype | candidate state class | transform | evidence signals | missing evidence |
|---|---|---|---|---|---|---|---|
| P1 | hook | `tools/hook.Hook[T]`, hook.go:55-57 | `singleton-actor` | `serialized-actor` (new) | serialize access via wire-level RPC; handler registry static post-init | `sync.RWMutex` + handler slice + deterministic priority sort; no goroutines | `Trigger()` concurrency contract (caller-serialized or internal?) |
| P2 | cron | `tools/cron.Cron`, cron.go:176-206 | `periodic-scheduler` | `periodic-invocation` | cron-triggered per-job invocation; ticker goroutine fenced to lifecycle | ticker + AfterFunc + `go j.Run()`; mutex guards job list | job recovery/panic semantics — does a panicking job block future ticks? |
| P3 | store | `tools/store.Store[K,T]`, store.go:12-40 | `sharded-keyed-state` | `keyed-partitioned-state` | RWMutex-protected map → managed KV store with key-based routing | RWMutex + map + ShrinkThreshold | whether shrink is user-facing or background; eviction interop with TTL |
| P4 | subscriptions | `tools/subscriptions.Broker`, broker.go:11-65 | `event-bus-publisher` | `fanout-publisher` | per-client channels as message sinks; broker registry as managed directory | `Store[string, Client]`; per-client unbuffered channels; no internal goroutines | how messages reach clients — is there a pump loop? |
| P5 | logger-batch | `tools/logger.BatchHandler`, batch_handler.go:54-88 | `singleton-actor` | `serialized-actor` | mutex-serialized Handle; periodic-or-threshold flush → log pipeline producer | mutex serializes Handle; buffer is immutable snapshot at flush | timer/threshold trigger (sync vs. async) |

## SUGGEST set

| # | subsystem | region | archetype | why SUGGEST | missing evidence |
|---|---|---|---|---|---|
| P6 | plugins/jsvm | `vmsPool`, pool.go:22-73 | work-stealing pool | unbounded fallback VM creation breaks bounded-pool invariant | explicit capacity bound or overflow-refuse semantics |
| P7 | apis | rate-limiter `middlewares_rate_limit.go` | `keyed-partitioned-state` + periodic gc | integration with settings reload hook not visible | does config hot-reload preserve rate-limit state? |
| P8 | apis | `realtime_test.go:833` (inferred handler) | `event-bus-subscriber` | handler itself not fully inspected; async send pattern unclear | full realtime-handler source read |
| P9 | tools/filesystem/internal/s3blob/s3/uploader.go | multipart uploader, 71-103 | bounded-worker (parallel parts) | `errgroup.Group` with MaxConcurrency but unclear explicit cap | bounded-concurrency invariant: is MaxConcurrency always set? |

## TERMINAL set

| # | region | reason |
|---|---|---|
| P10 | `core.BaseApp` embedded DB fields, base.go:74-85 | `MLV2_EMBEDDED_DB_APP_ROOT`. Load-bearing terminal refusal per ADR-0016 post-pass. |
| P11 | `core.BaseApp.RunInTransaction` + nonconcurrent DB alias | serializable-isolation semantics depend on in-process SQLite locking; no archetype captures "externalize transactional boundary" |
| P12 | `core.BaseApp.CreateBackup` / `base_backup.go:44-100` | archive-during-transaction; WAL checkpoint. Tied to P10/P11. |
| P13 | `tools/routine.FireAndForget` | fire-and-forget goroutine helper; users own lifecycle. No archetype captures anonymous-spawn without join. Terminal until caller context provides an archetype. |
| P14 | `tools/mailer/smtp.go` (SMTP send) | not a distribution pattern candidate in v1 (network I/O, stateless per-message); already admit-adjacent |

## ADMITTED set

- `tools/search.Provider` — bounded filter evaluation via errgroup; stateless.
- `apis/record_helpers.go` — request handlers; no retained state.
- `apis/batch.go` — transaction-serialized batch; no internal concurrency beyond DB txn.
- `core/event_request.go` — per-request mutex; request-scope naturally isolates.
- `core/settings_model.go` — RWMutex guarding settings; read-mostly, no goroutines.

## Subsystem coverage ledger

| bundle | file count | finding |
|---|---|---|
| core/ | 132 | TERMINAL (embedded DB, P10–P12) + AUTO-adjacent events (routed through hooks, see P1) + ADMITTED settings |
| apis/ | 75 | handlers (mostly ADMITTED), 1 SUGGEST (P7 rate-limiter), 1 SUGGEST (P8 realtime) |
| tools/hook/ | 6 | 1 AUTO (P1) |
| tools/cron/ | 1 | 1 AUTO (P2) |
| tools/subscriptions/ | 2 | 1 AUTO (P4) |
| tools/store/ | 1 | 1 AUTO (P3) |
| tools/logger/ | several | 1 AUTO (P5) |
| tools/routine/ | 1 | 1 TERMINAL (P13) — fire-and-forget helper |
| tools/mailer/ | 7 | 1 TERMINAL-adjacent (P14) |
| tools/filesystem/ | many | 1 SUGGEST (P9 uploader); rest admitted/no-surface |
| tools/search/ | 1 | ADMITTED |
| plugins/jsvm/ | 2 | 1 SUGGEST (P6 pool) |
| plugins/migratecmd/ | 2 | ADMITTED (sequential migration runner) |

**Net:** 5 AUTO, 4 SUGGEST, 5 TERMINAL. Terminal zone is load-bearing
and must remain so; AUTO set is everything else that the research
argues becomes auto-liftable once an archetype is named.
