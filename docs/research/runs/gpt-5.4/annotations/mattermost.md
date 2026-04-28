# Mattermost Annotation Notes - gpt-5.4 run

Catalog links: [queue-backed worker](../archetype-catalog-v1.md#queue-backed-worker), [scheduled-reconciler](../archetype-catalog-v1.md#scheduled-reconciler), [connection-hub-buffer](../archetype-catalog-v1.md#connection-hub-buffer).

## Target Synthesis

Mattermost is the richest single target in the corpus for archetype-driven remediation. Its strongest currently-refused AUTO surface is the websocket and job runtime, not the ordinary REST handlers. Three archetypes recur across independent subsystems:

- connection-aware hub ownership with replay/backpressure state
- queue and worker orchestration with resumable batch variants
- scheduler-owned periodic job launch

Headline AUTO set:

- websocket hub and reliable queue surfaces under `app/platform`
- job server, simple workers, batch workers, and periodic schedulers under `channels/jobs`
- narrower broker-like and hub-like surfaces inside the app and subscriptions code

Headline SUGGEST set:

- `SendNotifications` and adjacent notification fanout, because ordering, online-state, and multi-channel delivery semantics are not fully closed-form

Headline TERMINAL set:

- config store, CLI bootstrap, and decorated relational persistence as distribution-transform candidates for v1

## Owned-Directory Bundle Registration

Recorded before dispatch as required by SPRINT-0013.

The `long-lived / fanout` bundle is discovery-based and intentionally carved out of broader directories so websocket and notification paths have one owner.

| Bundle | Owned paths | Go file count |
|---|---|---:|
| ingress | `server/channels/api4` except `api4/websocket.go`; `server/channels/web`; `server/channels/wsapi` except `wsapi/websocket_handler.go` | 181 |
| app/service logic | `server/channels/app` except `app/platform/websocket_router.go`, `app/platform/websocket_reliable.go`, `app/notification.go`, `app/notification_push.go`, `app/notification_email.go`, `app/notify_admin.go` | 471 |
| long-lived / fanout | `server/channels/api4/websocket.go`, `server/channels/wsapi/websocket_handler.go`, `server/channels/app/platform/websocket_router.go`, `server/channels/app/platform/websocket_reliable.go`, `server/channels/app/notification.go`, `server/channels/app/notification_push.go`, `server/channels/app/notification_email.go`, `server/channels/app/notify_admin.go`, `server/channels/jobs/notify_admin` | 11 |
| jobs/workers | `server/channels/jobs` except `jobs/notify_admin` | 69 |
| persistence/search | `server/channels/store`, `server/channels/db` | 317 |
| platform/bootstrap | `server/platform`, `server/cmd`, `server/config` | 297 |

## Dispatch Log

| Bundle | Prompt version | Status | Return summary | Re-dispatch reason |
|---|---|---|---|---|
| ingress | `v1` | invalid | prompt omitted `evaluation/` prefix on first pass | corrected path |
| app/service logic | `v1` | invalid | prompt omitted `evaluation/` prefix on first pass | corrected path |
| long-lived / fanout | `v1` | invalid | prompt omitted `evaluation/` prefix on first pass | corrected path |
| jobs/workers | `v1` | invalid | prompt omitted `evaluation/` prefix on first pass | corrected path |
| persistence/search | `v1` | invalid | prompt omitted `evaluation/` prefix on first pass | corrected path |
| platform/bootstrap | `v1` | invalid | returned thin e2e target fixture after wrong path | corrected path |
| ingress | `v2` | completed | usable ingress summary with citations | - |
| app/service logic | `v2` | completed | usable app/platform summary with citations | - |
| long-lived / fanout | `v2` | shutdown without usable return | still slow after correction | thin-return timeout |
| jobs/workers | `v2` | completed | usable jobs summary with citations | - |
| persistence/search | `v2` | completed | usable persistence summary with citations | - |
| platform/bootstrap | `v2` | completed | usable bootstrap/config summary with citations | - |

## Parent Spot-Checks

Parent read and cited raw source in:

- `evaluation/mattermost/server/channels/api4/websocket.go`
- `evaluation/mattermost/server/channels/app/platform/websocket_router.go`
- `evaluation/mattermost/server/channels/app/platform/websocket_reliable.go`
- `evaluation/mattermost/server/channels/app/platform/web_hub.go`
- `evaluation/mattermost/server/channels/app/platform/web_conn.go`
- `evaluation/mattermost/server/channels/app/notification.go`
- `evaluation/mattermost/server/channels/jobs/jobs.go`
- `evaluation/mattermost/server/channels/jobs/base_workers.go`
- `evaluation/mattermost/server/channels/jobs/base_schedulers.go`

## Coverage Ledger

| Bundle | Status | Note |
|---|---|---|
| ingress | findings | Valid subagent return plus parent websocket spot-check. |
| app/service logic | findings | Valid subagent return plus parent websocket/notification spot-check. |
| long-lived / fanout | findings | No usable bundle return, but parent spot-check covered the intended hotspot files directly. |
| jobs/workers | findings | Valid subagent return plus parent reads. |
| persistence/search | findings | Valid subagent return; used mostly as terminal contrast. |
| platform/bootstrap | findings | Valid subagent return; used mostly as terminal contrast. |

## Region Findings

### Region 1

- `subsystem`: websocket connection entry and router
- `owned directories`: `evaluation/mattermost/server/channels/api4`, `evaluation/mattermost/server/channels/app/platform`
- `region or operation identity`: `connectWebSocket`, `WebSocketRouter`, `PlatformService.NewWebConn`, `HubRegister`, `GetWSQueues`, `WebConn`, `Hub`
- `admitted or refused`: refused today because per-connection queues, replay state, and hub ownership are process-local
- `triage`: `AUTO`
- `proposed archetype`: `connection-hub-buffer`
- `proposed candidate state class`: `connection-hub-buffer`
- `proposed transform`: preserve one owner per user/connection, route by sticky connection or user key, and externalize active/dead queue replay state behind that owner
- `competing archetypes considered`: `serialized-singleton-owner`
- `evidence signals seen`: explicit connection IDs, replay queues, active/dead queue distinction, explicit register/unregister path, per-connection state object
- `missing evidence`: none material; reconnect and replay semantics are already explicit in source
- `file references`: `evaluation/mattermost/server/channels/api4/websocket.go:57`, `evaluation/mattermost/server/channels/app/platform/websocket_router.go:18`, `evaluation/mattermost/server/channels/app/platform/websocket_reliable.go:14`, `evaluation/mattermost/server/channels/app/platform/web_hub.go:77`, `evaluation/mattermost/server/channels/app/platform/web_hub.go:175`, `evaluation/mattermost/server/channels/app/platform/web_conn.go:88`, `evaluation/mattermost/server/channels/app/platform/web_conn.go:200`

### Region 2

- `subsystem`: job runtime
- `owned directories`: `evaluation/mattermost/server/channels/jobs`
- `region or operation identity`: `JobServer.CreateJob`, `SimpleWorker`, batch workers
- `admitted or refused`: refused today because worker ownership and queues are process-local
- `triage`: `AUTO`
- `proposed archetype`: `queue-backed worker`
- `proposed candidate state class`: `queued-workset`
- `proposed transform`: externalize job dispatch and keep the current worker and batch-worker bodies as remote worker handlers
- `competing archetypes considered`: `scheduled-reconciler`
- `evidence signals seen`: explicit job creation, optimistic claim semantics, worker channels, resumable batch worker loops, cancellation and panic handling already localized
- `missing evidence`: none material for the core runtime
- `file references`: `evaluation/mattermost/server/channels/jobs/jobs.go:37`, `evaluation/mattermost/server/channels/jobs/base_workers.go:13`, `evaluation/mattermost/server/channels/jobs/batch_worker.go:53`, `evaluation/mattermost/server/channels/jobs/batch_report_worker.go:52`, `evaluation/mattermost/server/channels/jobs/batch_migration_worker.go:55`

### Region 3

- `subsystem`: scheduler layer
- `owned directories`: `evaluation/mattermost/server/channels/jobs`
- `region or operation identity`: `PeriodicScheduler`, `DailyScheduler`
- `admitted or refused`: refused today because cadence ownership is process-local
- `triage`: `AUTO`
- `proposed archetype`: `scheduled-reconciler`
- `proposed candidate state class`: `scheduled-reconciler`
- `proposed transform`: move periodic and daily launch semantics to platform scheduling and keep `ScheduleJob` as the enqueuer body
- `competing archetypes considered`: `queue-backed worker`
- `evidence signals seen`: explicit next-schedule computation, jitter handling, enabled predicates, clean separation between cadence and work body
- `missing evidence`: none material
- `file references`: `evaluation/mattermost/server/channels/jobs/base_schedulers.go:15`, `evaluation/mattermost/server/channels/jobs/base_schedulers.go:46`

### Region 4

- `subsystem`: notification fanout
- `owned directories`: `evaluation/mattermost/server/channels/app`
- `region or operation identity`: `App.SendNotifications`
- `admitted or refused`: refused today because delivery fanout mixes goroutines, presence checks, thread follower rules, and multi-channel notification policy in-process
- `triage`: `SUGGEST`
- `proposed archetype`: `connection-hub-buffer`
- `proposed candidate state class`: `connection-hub-buffer`
- `proposed transform`: split policy computation from transport fanout, then push delivery onto broker-backed notification workers or connection hubs
- `competing archetypes considered`: `queue-backed worker`
- `evidence signals seen`: many concurrent fetches and fanout goroutines, user-specific delivery decisions, explicit online/offline and channel membership checks
- `missing evidence`: durable ordering and de-duplication rules across websocket, push, email, and thread semantics are not closed-form from the current code alone
- `file references`: `evaluation/mattermost/server/channels/app/notification.go:54`

### Region 5

- `subsystem`: decorated persistence and search
- `owned directories`: `evaluation/mattermost/server/channels/store`, `evaluation/mattermost/server/channels/db`
- `region or operation identity`: store contract, `searchlayer`, `sqlstore`
- `admitted or refused`: currently unlifted, but not a useful v1 distribution archetype
- `triage`: `TERMINAL`
- `proposed archetype`: none survived
- `proposed candidate state class`: none
- `proposed transform`: none; this is a decorated persistence stack, not the next auto-lift surface
- `competing archetypes considered`: `serialized-singleton-owner`
- `evidence signals seen`: layered contracts, search projections, SQL entity stores, migration and integrity control
- `missing evidence`: a narrower selected region with explicit queue, timer, or owner semantics
- `file references`: `evaluation/mattermost/server/channels/store/store.go:25`, `evaluation/mattermost/server/channels/store/searchlayer/layer.go:17`, `evaluation/mattermost/server/channels/store/sqlstore/store.go:62`, `evaluation/mattermost/server/channels/db/assets.go:8`

### Region 6

- `subsystem`: config and process bootstrap
- `owned directories`: `evaluation/mattermost/server/config`, `evaluation/mattermost/server/cmd`, `evaluation/mattermost/server/platform`
- `region or operation identity`: config store plus CLI bootstrap commands
- `admitted or refused`: not a v1 distribution-transform target
- `triage`: `TERMINAL`
- `proposed archetype`: none survived
- `proposed candidate state class`: none
- `proposed transform`: none; this is orchestration and adapter code
- `competing archetypes considered`: `serialized-singleton-owner`
- `evidence signals seen`: command wiring, config persistence backends, file/S3 IO strategy objects
- `missing evidence`: a narrower selected region with a real concurrency or ownership transform
- `file references`: `evaluation/mattermost/server/cmd/mattermost/commands/root.go:15`, `evaluation/mattermost/server/config/store.go:28`, `evaluation/mattermost/server/platform/shared/filestore/filesstore.go:25`
