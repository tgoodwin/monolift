# Gitea Annotation Notes - gpt-5.4 run

Catalog links: [queue-backed worker](../archetype-catalog-v1.md#queue-backed-worker), [scheduled-reconciler](../archetype-catalog-v1.md#scheduled-reconciler), [connection-hub-buffer](../archetype-catalog-v1.md#connection-hub-buffer), [serialized-singleton-owner](../archetype-catalog-v1.md#serialized-singleton-owner).

## Target Synthesis

Gitea contains real queue, timer, session, and fanout surfaces, but the repo is large enough that the main research risk is false positives from orchestration code. The convincing v1 story is concentrated in three places:

- `modules/queue` and the indexer services: durable or semi-durable queue workers
- `modules/eventsource`: timer-driven per-user fanout
- `modules/session`: session ownership surfaces that look remediable but still need tighter lifecycle proof

Headline AUTO set:

- `modules/queue.WorkerPoolQueue` and the indexer queues built on top of it
- `modules/eventsource.Manager.Run` as a scheduled reconciler
- `modules/eventsource.Manager` / `Messenger` as a connection/fanout hub

Headline SUGGEST set:

- `modules/session.DBProvider` / `RedisProvider` / `VirtualSessionProvider`

Headline TERMINAL set:

- synchronous adapters like `services/mailer/sender/SMTPSender`
- most ingress glue under `routers/*` and `modules/web`

## Owned-Directory Bundle Registration

Recorded before dispatch as required by SPRINT-0013.

| Bundle | Owned paths | Go file count |
|---|---|---:|
| boot/lifecycle | `cmd`, `routers/install`, `modules/setting`, `modules/graceful` | 145 |
| ingress | `routers/api`, `routers/web`, `services/context`, `modules/web`, `modules/reqctx` | 446 |
| domain services | `services/auth`, `services/user`, `services/org`, `services/repository`, `services/pull`, `services/issue`, `services/packages`, `services/oauth2_provider`, `services/mirror`, `services/wiki` | 217 |
| background/async | `services/mailer`, `services/notify`, `services/task`, `services/webhook`, `services/cron`, `services/actions`, `modules/queue` | 112 |
| infra/runtime | `modules/cache`, `modules/storage`, `modules/indexer`, `modules/session`, `modules/eventsource`, `modules/private`, `modules/process` | 97 |
| persistence | `models` | 649 |

## Dispatch Log

| Bundle | Prompt version | Status | Return summary | Re-dispatch reason |
|---|---|---|---|---|
| boot/lifecycle | `v1` | shutdown without usable return | first pass never terminated | over-broad prompt |
| ingress | `v1` | shutdown without usable return | first pass never terminated | over-broad prompt |
| domain services | `v1` | shutdown without usable return | first pass never terminated | over-broad prompt |
| background/async | `v1` | shutdown without usable return | first pass never terminated | over-broad prompt |
| infra/runtime | `v1` | shutdown without usable return | first pass never terminated | over-broad prompt |
| persistence | `v1` | shutdown without usable return | first pass never terminated | over-broad prompt |
| ingress | `v2` | shutdown without usable return | corrected path, still slow | thin-return timeout |
| background/async | `v2` | shutdown without usable return | corrected path, still slow | thin-return timeout |
| infra/runtime | `v2` | invalid return | responded as a code-review bug hunt rather than sprint schema | ignored in synthesis |

## Parent Spot-Checks

Parent read and cited raw source in:

- `evaluation/gitea/modules/queue/manager.go`
- `evaluation/gitea/modules/queue/workergroup.go`
- `evaluation/gitea/modules/indexer/issues/indexer.go`
- `evaluation/gitea/modules/indexer/code/indexer.go`
- `evaluation/gitea/modules/session/db.go`
- `evaluation/gitea/modules/session/redis.go`
- `evaluation/gitea/modules/eventsource/manager.go`
- `evaluation/gitea/modules/eventsource/messenger.go`
- `evaluation/gitea/modules/eventsource/manager_run.go`
- `evaluation/gitea/services/mailer/sender/smtp.go`

## Coverage Ledger

| Bundle | Status | Note |
|---|---|---|
| boot/lifecycle | no relevant archetype surface observed | Mostly startup and config plumbing. |
| ingress | no relevant archetype surface observed | Mostly request adapters and policy wrappers. |
| domain services | no relevant archetype surface observed | Broad business logic, but not a distinct distribution transform surface in the parent reads. |
| background/async | findings | Queue worker evidence, plus synchronous adapters that stay terminal. |
| infra/runtime | findings | Event fanout, session ownership, process lifecycle helpers. |
| persistence | no relevant archetype surface observed | Large relational model layer; no stronger v1 transform than the queue/session/eventsource regions already cited. |

## Region Findings

### Region 1

- `subsystem`: async indexing and general worker queues
- `owned directories`: `evaluation/gitea/modules/queue`, `evaluation/gitea/modules/indexer/issues`, `evaluation/gitea/modules/indexer/code`
- `region or operation identity`: `WorkerPoolQueue`, `doWorkerHandle`, `doStartNewWorker`, issue/code indexer queues
- `admitted or refused`: refused today because the queue and worker pool are process-local
- `triage`: `AUTO`
- `proposed archetype`: `queue-backed worker`
- `proposed candidate state class`: `queued-workset`
- `proposed transform`: externalize queue storage and keep the current handler bodies as worker replicas with retry/requeue semantics preserved
- `competing archetypes considered`: `scheduled-reconciler`
- `evidence signals seen`: explicit queue manager, worker pool, requeue on unhandled items, indexer queues already isolate serializable IDs from worker logic
- `missing evidence`: none material for the queue core; leaf handlers may still need per-job review
- `file references`: `evaluation/gitea/modules/queue/manager.go:19`, `evaluation/gitea/modules/queue/workergroup.go:92`, `evaluation/gitea/modules/queue/workergroup.go:155`, `evaluation/gitea/modules/indexer/issues/indexer.go:47`, `evaluation/gitea/modules/indexer/code/indexer.go:29`

### Region 2

- `subsystem`: per-user event delivery
- `owned directories`: `evaluation/gitea/modules/eventsource`
- `region or operation identity`: `Manager`, `Messenger`, `Register`, `SendMessage`
- `admitted or refused`: refused today because subscription channels and delivery are process-local
- `triage`: `AUTO`
- `proposed archetype`: `connection-hub-buffer`
- `proposed candidate state class`: `connection-hub-buffer`
- `proposed transform`: emit a per-user fanout service with explicit register/unregister and bounded buffered delivery
- `competing archetypes considered`: `serialized-singleton-owner`
- `evidence signals seen`: explicit per-user messenger registry, buffered per-connection channels, non-blocking send and blocking send variants
- `missing evidence`: none material beyond choosing the sticky routing key (`uid`)
- `file references`: `evaluation/gitea/modules/eventsource/manager.go:11`, `evaluation/gitea/modules/eventsource/manager.go:33`, `evaluation/gitea/modules/eventsource/messenger.go:24`, `evaluation/gitea/modules/eventsource/messenger.go:58`, `evaluation/gitea/modules/eventsource/messenger.go:71`

### Region 3

- `subsystem`: timed event refresh
- `owned directories`: `evaluation/gitea/modules/eventsource`
- `region or operation identity`: `Manager.Init`, `Manager.Run`
- `admitted or refused`: refused today because the timer loop is process-local
- `triage`: `AUTO`
- `proposed archetype`: `scheduled-reconciler`
- `proposed candidate state class`: `scheduled-reconciler`
- `proposed transform`: move the periodic notification-count and stopwatch refresh to an external scheduler that invokes the same per-user emission body
- `competing archetypes considered`: `queue-backed worker`
- `evidence signals seen`: ticker, paused/no-listener behavior, refresh body separate from delivery
- `missing evidence`: none material if duplicate ticks are acceptable
- `file references`: `evaluation/gitea/modules/eventsource/manager_run.go:23`, `evaluation/gitea/modules/eventsource/manager_run.go:31`

### Region 4

- `subsystem`: session ownership
- `owned directories`: `evaluation/gitea/modules/session`
- `region or operation identity`: `DBProvider.Read`, `RedisProvider.Read` and corresponding mutable store wrappers
- `admitted or refused`: refused today because request-scoped mutable snapshots and provider lifecycle are still process-owned
- `triage`: `SUGGEST`
- `proposed archetype`: `serialized-singleton-owner`
- `proposed candidate state class`: `owned-mutable-singleton`
- `proposed transform`: lift session ownership behind a session service and route reads/writes through that service while preserving external DB/Redis durability
- `competing archetypes considered`: `connection-hub-buffer`
- `evidence signals seen`: mutex-protected maps, externalized durable backing stores, narrow CRUD-style store API
- `missing evidence`: alias and regeneration lifecycle are not closed-form enough to auto-lift safely
- `file references`: `evaluation/gitea/modules/session/db.go:18`, `evaluation/gitea/modules/session/db.go:105`, `evaluation/gitea/modules/session/redis.go:20`, `evaluation/gitea/modules/session/redis.go:124`

### Region 5

- `subsystem`: outbound mail adapter
- `owned directories`: `evaluation/gitea/services/mailer`
- `region or operation identity`: `SMTPSender.Send`
- `admitted or refused`: currently unlifted, but not because it hides a new archetype
- `triage`: `TERMINAL`
- `proposed archetype`: none survived
- `proposed candidate state class`: none
- `proposed transform`: none; this is a synchronous external client adapter, not a new distribution transform
- `competing archetypes considered`: `queue-backed worker`
- `evidence signals seen`: direct network dial, TLS setup, SMTP client lifecycle, no internal work queue or owner state
- `missing evidence`: a separate enqueuing layer, which would be a different region entirely
- `file references`: `evaluation/gitea/services/mailer/sender/smtp.go:22`
