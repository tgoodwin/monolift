# listmonk annotation — SPRINT-0013 (opus run)

**Corpus pin:** `evaluation/MANIFEST.yaml` as of 2026-04-19. 92 Go files.
No committed golden report; source walk only.

## Target synthesis

Listmonk's distribution surface concentrates in three loci that the
current compiler refuses wholesale, but which have high archetype
coherence once named:

1. A **campaign-manager** that looks like a `worker-pool-consumer` fed
   by `Manager.campMsgQ`/`msgQ` channels, but whose batches come from a
   DB-driven scan (`scanCampaigns`) rather than an inbound queue. The
   DB-driven production side makes this a two-part candidate:
   `periodic-scheduler` feeding `worker-pool-consumer`.
2. A **bounce subsystem** that is a single-processor `periodic-scheduler`
   (`runMailboxScanner`) feeding an inbound queue consumed by a single
   goroutine — a degenerate worker-pool (pool size = 1) that still fits
   the archetype shape.
3. An in-process **pub/sub** (`internal/events`) with fanout over a
   mutex-protected `map[string]chan Event` — a textbook
   `event-bus-publisher`/`subscriber`.

**Hardest ambiguity.** `Manager.pipes` (RWMutex-protected
`map[int]*pipe` of active campaigns) and `trackLink()`'s URL cache are
`sharded-keyed-state` candidates, but the DB also owns canonical
state — so the in-process map is a cache, not the authority, which
weakens the case that sharding it in-process buys anything; the real
transform is "externalize to a managed cache". Calls the distinction
between `sharded-keyed-state` and `ttl-cache-managed` less sharp than
v0 suggests.

**Evidence gaps.**
- Whether multi-replica worker pools preserve within-campaign ordering
  (pipe.sent, pipe.rate are atomic but replay ordering is unclear).
- Whether the `tmptokens` cleanup interval is coordination-sensitive
  (cross-replica it becomes N independent cleanup loops that are fine
  if idempotent).

## AUTO set

| # | subsystem | region (pkg.Symbol, file:line) | archetype | candidate state class | transform | evidence signals (cite) | missing evidence |
|---|---|---|---|---|---|---|---|
| L1 | manager | `internal/manager.Manager.scanCampaigns`, manager.go:422-458 | `periodic-scheduler` | `periodic-invocation` (new) | cron-triggered scheduler emits campaign-due events into queue | `time.NewTicker(tick)`; body reads DB, pushes jobs (no captured mutable state). `lifecycle.long-running-loop` bias; `effects.no-global-writes` holds on body | cross-replica idempotency of "campaign picked" — DB checkpoint visible to compiler? |
| L2 | manager | `internal/manager.Manager.worker`, manager.go:462-559; `newMessage`, pipe.go:172-181 | `worker-pool-consumer` | `bounded-worker-pool` (new) | replace shared `campMsgQ` with broker-backed queue; replicas consume | `select` on two channels; jobs are `CampaignMessage` (serializable); N workers spawned in loop manager.go:273-275 | pipe.sent/lastID are per-campaign counters — do they require a single writer? |
| L3 | bounce | `internal/bounce.Manager.runMailboxScanner`, bounce.go:135-143 | `periodic-scheduler` | `periodic-invocation` | cron-triggered mailbox poll + webhook queue | `time.Sleep(ScanInterval)` in infinite loop; no captured state | provider registration singletons (SES/Sendgrid) — can multiple replicas read same webhook endpoints? |
| L4 | events | `internal/events.Events.Publish`/`Subscribe`, events.go:41-76 | `event-bus-publisher` | `fanout-publisher` (new) | managed pub/sub broker; subscribers are independent services | `sync.RWMutex` + `subs map[string]chan Event`; Publish iterates and fans out; channel buffer=100 | subscriber set is dynamic — discovery mechanism maps cleanly onto broker subscribe API |

## SUGGEST set

| # | subsystem | region | archetype | why SUGGEST not AUTO | missing evidence |
|---|---|---|---|---|---|
| L5 | manager | `Manager.pipes` map + `trackLink` link cache, manager.go:72-81, 586-614 | `sharded-keyed-state` / `ttl-cache-managed` | pipes map is an in-process authority for stats (GetCampaignStats reads it); DB has different granularity | cross-replica stats consistency model — is eventual consistency acceptable? |
| L6 | auth | `internal/auth.Auth.apiUsers` + session prune loop, auth.go:62-110 | `ttl-cache-managed` | prune interval (12h) is loose; session store is Postgres-backed | is cache invalidation on user update required to be linearized? |
| L7 | tmptokens | package-global `tokens` + hourly cleanup, tmptokens.go:29-42 | `ttl-cache-managed` | package-level state; init() spawns unnamed cleanup goroutine | are tokens legitimately per-instance (2FA/reset) or global? |
| L8 | subimporter | `Session.subQueue` channel + batching, subimporter.go:83, 273-349 | `worker-pool-consumer` | single-session-at-a-time guard (isDone) prevents concurrency; TX semantics tied to one DB session | can Tx boundary be moved to the batch boundary so workers can parallelize? |
| L9 | bounce | `Manager.queue` + single-reader `Run`, bounce.go:48, 118-132 | `worker-pool-consumer` (degenerate) | single processor; no replay log for failure recovery | would the lift require a durable intermediary queue? |

## TERMINAL set

| # | region | reason |
|---|---|---|
| L10 | `manager/pipe.newPipe` WaitGroup cleanup, pipe.go:58-64 | termination-detection protocol across children — no v1 archetype captures "wait for N spawned goroutines and then release resources" without inventing distributed TD |
| L11 | `cmd/init.go` signal handling (sigChan, chReload) | OS-signal primitives are orthogonal to application distribution archetypes |
| L12 | `internal/auth.initOIDC` token flow | Request-scoped protocol exchange; no long-lived distribution shape |

## ADMITTED set

- Core CRUD on `internal/core` — already stateless; DB provides state.
- `models` package — immutable value types.
- `internal/messenger`, `internal/media` (S3, FS) — stateless per-message or external-store-backed.
- `internal/i18n`, `internal/utils`, `internal/buflog` — no distribution surface.

## Subsystem coverage ledger

| subsystem | file count | finding |
|---|---|---|
| cmd/ | ~11 | orchestration spawning 5 long-lived loops; each loop attributed to its owning subsystem |
| internal/manager/ | 2 | 2 AUTO (L1, L2), 1 SUGGEST (L5), 1 TERMINAL (L10) |
| internal/events/ | 1 | 1 AUTO (L4) |
| internal/bounce/ | 1 + webhooks | 1 AUTO (L3), 1 SUGGEST (L9) |
| internal/subimporter/ | 1 | 1 SUGGEST (L8) |
| internal/auth/ | 2 | 1 SUGGEST (L6) |
| internal/tmptokens/ | 1 | 1 SUGGEST (L7) |
| internal/buflog/ | 1 | no relevant archetype surface observed — per-replica log buffer, no coordination required |
| internal/core/ | ~10 | ADMITTED (stateless DB CRUD) |
| internal/messenger/, media/, i18n/, notifs/, captcha/, utils/ | varies | no relevant archetype surface observed — stateless or external-store-backed |
| internal/migrations/ | ~15 | no relevant archetype surface observed — one-shot schema DDL |
| models/ | ~8 | ADMITTED (immutable value types) |

**Net:** 4 AUTO, 5 SUGGEST, 3 TERMINAL across a 92-file corpus.
