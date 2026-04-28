# miniflux annotation — SPRINT-0013 (opus run)

**Corpus pin:** 2026-04-19. 407 Go files.
Golden report: `test/e2e/targets/miniflux/golden/report.json` —
`ProcessFeedEntries` admitted with pragma `state=external`
(Postgres-backed), accept verdict.

## Target synthesis

Miniflux is the target where the sprint plan predicted
`worker-pool-consumer` would earn its keep. The research finding is
**partial vindication**: the feed worker pool is clean
(channel-fed goroutines, serializable `model.Job`, DB-coordinated
state), but it is **already admitted** because Monolift lifts
`ProcessFeedEntries` with `state=external` and the pool is the trivial
scheduling layer on top. So the pool does not motivate a *new* state
class — it confirms the `bounded-worker-pool` shape but demonstrates
it is the ADMITTED baseline when state is external.

The interesting AUTO findings are:

1. **Periodic-scheduler loops** (feedScheduler, cleanupScheduler,
   watchdog, metrics collector) — four distinct
   `time.Tick`-driven bodies with pure primitive captured state.
   Dominant AUTO archetype.
2. **HTTP listener startup template** — six near-identical closures
   wrapping `server.Listen*`. `replicated-stateless-service`; worth a
   single named transform that covers all six.
3. **Fire-and-forget integration dispatches** (Fever, GoogleReader) —
   `go integration.SendEntry(...)` closures with no join. **TERMINAL**
   under the v1 vocabulary because no archetype captures
   closure-over-mutable-integration-state with no lifecycle.
4. **`ProxyRotator`** — mutex guarding a round-robin counter. Trivial
   `singleton-actor`, already admittable post-init (no meaningful
   mutation beyond the counter).

**Worker-pool-consumer verdict.** In miniflux, the pool collapses into
`replicated-stateless-service` feeding `periodic-scheduler`-driven
batches. The distinguishing evidence between a *true* worker pool
archetype and plain goroutine-per-feed is *not* structural — it is
semantic: whether the spawn is fire-and-forget (TERMINAL) or feeds a
bounded queue with the external state coordinator providing recovery
(already ADMITTED). This is load-bearing for the catalog:
`worker-pool-consumer` is not a standalone archetype; it is the pair
of `bounded-worker-pool` state class + `replicated-stateless-service`
admission, with external state as the coordination substrate.

**Evidence gaps.**
- Does UI handler `pool.Push` have blocking semantics? If so, handler
  is synchronous; if not, it is fire-and-forget (→ suggests refusal).
- Does `RemoveUserAsync` have any recovery path? Without it, the async
  deletion is a hazard, not a lift candidate.

## AUTO set

| # | subsystem | region (file:line) | archetype | candidate state class | transform | evidence signals | missing evidence |
|---|---|---|---|---|---|---|---|
| M1 | cli | `internal/cli.feedScheduler`, scheduler.go:33-50 | `periodic-scheduler` | `periodic-invocation` | extract ticker+batchBuilder to named service, lift to cron + queue-push | `time.Tick(frequency)`; `pool.Push(jobs)` is serialization boundary; jobs are primitives | single-daemon-instance guarantee? |
| M2 | cli | `internal/cli.cleanupScheduler`, scheduler.go:52-55 | `periodic-scheduler` | `periodic-invocation` | cron-triggered cleanup with DB-mediated idempotency | `time.Tick`; side-effect via `runCleanupTasks(store)` | runCleanupTasks idempotency across concurrent invocations |
| M3 | cli | daemon watchdog, daemon.go:57-73 | `periodic-scheduler` | `periodic-invocation` | extract watchdog to service; `store.Ping` + `systemd.SdNotify` are stateless | `time.Sleep(interval/3)`; RPC + stateless notify | context-cancellation uniformity |
| M4 | metric | `GatherStorageMetrics`, metric.go:172-222 | `periodic-scheduler` | `periodic-invocation` | scheduled metrics collector; Prometheus gauges are idempotent | `time.NewTicker` + `select`; DB queries are pure reads | prom registry label-conflict under concurrency |
| M5 | http | 6 listener startup closures, server.go:163-268 | `replicated-stateless-service` | (existing: replicated) | single named `StartServerListener(mode, config)` as replicated service | identical template across 6 sites; no shared state | systemd-socket vs. unix-socket cleanup parity |
| M6 | proxyrotator | `ProxyRotator`, proxyrotator.go:20-51 | `singleton-actor` | `serialized-actor` | wire-serialize the counter read/increment; or: admit as "trivially replicated, counters are advisory" | `sync.Mutex` + counter; round-robin is advisory | is strict round-robin required, or is best-effort acceptable? |

## SUGGEST set

| # | subsystem | region | archetype | why SUGGEST | missing evidence |
|---|---|---|---|---|---|
| M7 | cli | `refreshFeeds`, refresh_feeds.go:17-78 | ad-hoc `worker-pool-consumer` | CLI-only pool recreated per invocation; duplicate of daemon pool | whether the two pools are intentionally distinct or should be unified |
| M8 | ui | `feed_refresh` / `category_refresh` handlers | fire-and-forget `pool.Push` | unclear whether handler semantics are blocking or async-dispatch | push blocking behavior vs. caller expectation |

## TERMINAL set

| # | region | reason |
|---|---|---|
| M9 | `internal/fever.markEntryHandler` | `go func() { integration.SendEntry(entry, settings) }()` — fire-and-forget, no join, no error return. No v1 archetype captures anonymous spawn over mutable closure without lifecycle. |
| M10 | `internal/googlereader.markEntriesHandler` | fire-and-forget goroutine per entry in loop. Same reason as M9. |
| M11 | `internal/storage.RemoveUserAsync`, user.go:189-199 | async user deletion with no error handling, no context cancel, no retry. TERMINAL until archetype provides a `background-job` class with recovery semantics. |

## ADMITTED set

- `internal/worker.Pool.Run` — channel consumer with serializable jobs;
  ADMITTED via `ProcessFeedEntries` pragma + external Postgres.
- `internal/reader/processor` pipeline — 4 sequential stages; pure
  transform; already ADMITTED.
- `internal/reader`, `internal/ui` handler bodies — sequential,
  DB-backed.

## Subsystem coverage ledger

| subsystem | file count | finding |
|---|---|---|
| internal/worker | 2 | ADMITTED (baseline) |
| internal/cli | 3 | 3 AUTO (M1–M3), 1 SUGGEST (M7) |
| internal/http/server | 1 | 1 AUTO (M5) |
| internal/metric | 1 | 1 AUTO (M4) |
| internal/proxyrotator | 1 | 1 AUTO (M6) |
| internal/fever | 1 | 1 TERMINAL (M9) |
| internal/googlereader | 1 | 1 TERMINAL (M10) |
| internal/storage | many | 1 TERMINAL (M11); rest ADMITTED (Postgres-backed CRUD) |
| internal/reader (93) | 93 | ADMITTED — sequential transforms, no archetype surface |
| internal/ui (105) | 105 | 1 SUGGEST (M8); rest ADMITTED — stateless handlers |
| internal/{model,api,config,...} | ~85 | no relevant archetype surface observed — data types, config init, request handlers |

**Net:** 6 AUTO, 2 SUGGEST, 3 TERMINAL. Key vocabulary finding:
`worker-pool-consumer` is not a standalone archetype in v1 — it
reduces to `bounded-worker-pool` state class + replicated-service
admission. See catalog entry for retirement-or-merge argument.
