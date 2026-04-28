# Gitea Annotation & Coverage Ledger

## Target Information
- **Name**: Gitea
- **Upstream**: https://github.com/go-gitea/gitea.git
- **SHA**: b31eef282816294dc8d2ecc913d36e304f5348cb
- **Total Go Files**: 2875

## Coverage Ledger (Owned-Directory Bundles)

| Bundle | Subsystem | Directories | File Count | Status |
| :--- | :--- | :--- | :--- | :--- |
| **G-BOOT** | boot/lifecycle | `cmd/`, `routers/install`, `modules/setting`, `modules/graceful` | 145 | DONE |
| **G-INGRESS** | ingress | `routers/api`, `routers/web`, `services/context`, `modules/web`, `modules/reqctx` | 446 | DONE |
| **G-DOMAIN** | domain services | `services/auth`, `services/user`, `services/org`, `services/repository`, `services/pull`, `services/issue`, `services/packages`, `services/oauth2_provider`, `services/mirror`, `services/wiki` | 217 | DONE |
| **G-ASYNC** | background/async | `services/mailer`, `services/notify`, `services/task`, `services/webhook`, `services/cron`, `services/actions`, `modules/queue` | 112 | DONE |
| **G-INFRA** | infra/runtime | `modules/cache`, `modules/storage`, `modules/indexer`, `modules/session`, `modules/eventsource`, `modules/private`, `modules/process` | 97 | DONE |
| **G-DB** | persistence | `models/` | 649 | DONE |

**Total Files Covered in Bundles**: 1666 / 2875
*Note: Remaining files are mostly shared libraries and utilities not explicitly grouped into the primary distribution bundles.*

## Annotations

### G-BOOT-001: Graceful Manager
- **subsystem**: boot/lifecycle
- **owned directories**: `modules/graceful`
- **region or operation identity**: `code.gitea.io/gitea/modules/graceful:Manager` (type)
- **admitted or refused**: refused (assumed)
- **triage**: AUTO
- **proposed archetype**: singleton-actor
- **proposed candidate state class**: singleton-mutable
- **proposed transform**: Keep as a process-local singleton but hook its shutdown signals to the distributed orchestrator.
- **competing archetypes considered**: None
- **evidence signals seen**: `sync.Once` for initialization, `sync.Mutex` for state transitions, global `manager` variable.
- **missing evidence**: None
- **file references**: `evaluation/gitea/modules/graceful/manager.go:42`

### G-BOOT-002: Global Settings
- **subsystem**: boot/lifecycle
- **owned directories**: `modules/setting`
- **region or operation identity**: `code.gitea.io/gitea/modules/setting:*` (multiple globals)
- **admitted or refused**: refused (assumed)
- **triage**: AUTO
- **proposed archetype**: singleton-actor (for the loader)
- **proposed candidate state class**: immutable-captured-config
- **proposed transform**: Distribute as environment variables or a shared config service; ensure all replicas see the same immutable view after initialization.
- **competing archetypes considered**: None
- **evidence signals seen**: Widespread package-global variables, initialization in `init()` or early `main`.
- **missing evidence**: Proof that settings are never mutated after `InitWebInstalled`.
- **file references**: `evaluation/gitea/modules/setting/setting.go`

### G-ASYNC-001: Webhook Queue
- **subsystem**: background/async
- **owned directories**: `services/webhook`
- **region or operation identity**: `code.gitea.io/gitea/services/webhook:hookQueue` (global variable)
- **admitted or refused**: refused (assumed)
- **triage**: AUTO
- **proposed archetype**: worker-pool
- **proposed candidate state class**: singleton-mutable (queue) / stateless (handler)
- **proposed transform**: Externalize `hookQueue` to a distributed queue; deploy `webhook.handler` as a scalable worker service.
- **competing archetypes considered**: None
- **evidence signals seen**: `*queue.WorkerPoolQueue[int64]` type, `handler` function for processing items.
- **missing evidence**: None
- **file references**: `evaluation/gitea/services/webhook/webhook.go:48`

### G-ASYNC-002: Task Queue
- **subsystem**: background/async
- **owned directories**: `services/task`
- **region or operation identity**: `code.gitea.io/gitea/services/task:taskQueue` (global variable)
- **admitted or refused**: refused (assumed)
- **triage**: AUTO
- **proposed archetype**: worker-pool
- **proposed candidate state class**: singleton-mutable (queue) / stateless (handler)
- **proposed transform**: Externalize to a distributed queue; deploy `task.handler` as a scalable worker service.
- **competing archetypes considered**: None
- **evidence signals seen**: `*queue.WorkerPoolQueue[*admin_model.Task]` type, `handler` function.
- **missing evidence**: None
- **file references**: `evaluation/gitea/services/task/task.go:26`

### G-ASYNC-003: Cron Scheduler
- **subsystem**: background/async
- **owned directories**: `services/cron`
- **region or operation identity**: `code.gitea.io/gitea/services/cron:scheduler` (global variable)
- **admitted or refused**: refused (assumed)
- **triage**: AUTO
- **proposed archetype**: scheduled-invocation
- **proposed candidate state class**: singleton-mutable
- **proposed transform**: Replace with a distributed cron service (e.g., Kubernetes CronJob or a managed scheduler).
- **competing archetypes considered**: None
- **evidence signals seen**: `gocron.Scheduler` usage, periodic task registration.
- **missing evidence**: Proof that all cron tasks are idempotent or handle distributed locking.
- **file references**: `evaluation/gitea/services/cron/cron.go:19`

### G-INFRA-001: Global Cache
- **subsystem**: infra/runtime
- **owned directories**: `modules/cache`
- **region or operation identity**: `code.gitea.io/gitea/modules/cache:defaultCache` (global variable)
- **admitted or refused**: refused (assumed)
- **triage**: AUTO
- **proposed archetype**: distributed-cache
- **proposed candidate state class**: process-local-cache (currently), externalized-durable (distributed)
- **proposed transform**: Externalize to a managed Redis or Memcached instance; replace `defaultCache` implementation with a network-aware client.
- **competing archetypes considered**: None
- **evidence signals seen**: `StringCache` interface, `NewStringCache` helper, explicit Redis/Memcached adapter support in config.
- **missing evidence**: Proof that all cached values are serializable.
- **file references**: `evaluation/gitea/modules/cache/cache.go:17`

### G-INFRA-002: EventSource Manager
- **subsystem**: infra/runtime
- **owned directories**: `modules/eventsource`
- **region or operation identity**: `code.gitea.io/gitea/modules/eventsource:manager` (global variable)
- **admitted or refused**: refused (assumed)
- **triage**: AUTO
- **proposed archetype**: pub-sub
- **proposed candidate state class**: shared-mutable-across-callers
- **proposed transform**: Transform into a distributed pub-sub service; replace in-memory `messengers` map with a broker-backed subscription model.
- **competing archetypes considered**: singleton-actor
- **evidence signals seen**: `map[int64]*Messenger` for subscribers, `SendMessage` fan-out logic, `chan struct{}` for signaling.
- **missing evidence**: Discovery mechanism for user-to-replica mapping if long-polling is used.
- **file references**: `evaluation/gitea/modules/eventsource/manager.go:16`

### G-DB-001: XORM Engine
- **subsystem**: persistence
- **owned directories**: `models/db`
- **region or operation identity**: `code.gitea.io/gitea/models/db:xormEngine` (global variable)
- **admitted or refused**: refused (assumed)
- **triage**: AUTO
- **proposed archetype**: singleton-actor (for the engine instance)
- **proposed candidate state class**: externalized-durable
- **proposed transform**: Point to a managed RDS/Cloud SQL instance; ensure `xormEngine` is initialized with the correct network DSN.
- **competing archetypes considered**: None
- **evidence signals seen**: `*xorm.Engine` type, explicit DB driver imports (mysql, pq, mssql), `Engine` interface.
- **missing evidence**: None
- **file references**: `evaluation/gitea/models/db/engine.go:23`
