# Gitea Coverage Ledger

| Bundle | Subsystem | Owned Directories | File Count (*.go) |
|---|---|---|---|
| boot/lifecycle | Bootstrap | `cmd/`, `routers/install`, `modules/setting`, `modules/graceful` | 145 |
| ingress | Ingress/Routing | `routers/api`, `routers/web`, `services/context`, `modules/web`, `modules/reqctx` | 446 |
| domain services | Domain Logic | `services/auth`, `services/user`, `services/org`, `services/repository`, `services/pull`, `services/issue`, `services/packages`, `services/oauth2_provider`, `services/mirror`, `services/wiki` | 217 |
| background/async | Background Tasks | `services/mailer`, `services/notify`, `services/task`, `services/webhook`, `services/cron`, `services/actions`, `modules/queue` | 112 |
| infra/runtime | Infra/Runtime | `modules/cache`, `modules/storage`, `modules/indexer`, `modules/session`, `modules/eventsource`, `modules/private`, `modules/process` | 97 |
| persistence | Persistence | `models/` | 649 |

**Total Files**: 1666 (in bundles) / 2875 (total)

## Synthesis
Gitea is a mature monolith with highly decoupled subsystems. The most significant finding is the pervasive use of internal queues (`modules/queue`) which maps directly to the **Worker Pool / Queue Consumer** archetype. Ingress and domain services are largely **Replicated Stateless Services** once database and configuration state are externalized.

## Key Annotations

### WorkerPoolQueue (modules/queue)
- **Subsystem**: `background`
- **Triage**: AUTO
- **Archetype**: Worker Pool / Queue Consumer
- **Transform**: Replace internal channels with Redis/NATS.
- **Evidence**: `WorkerPoolQueue` struct, process-local channels.
- **File**: `modules/queue/queue.go`

### routing.requestRecordsManager
- **Subsystem**: `ingress`
- **Triage**: AUTO
- **Archetype**: Singleton Actor
- **Transform**: Lift into stateful service; serialize access.
- **Evidence**: `sync.Mutex`, long-running background detector loop.
- **File**: `modules/web/routing/logger_manager.go:34`

### UserSignIn (services/auth)
- **Subsystem**: `domain logic`
- **Triage**: AUTO
- **Archetype**: Replicated Stateless Service
- **Transform**: Standard RPC lift.
- **Evidence**: `boundary.context-first`, `externalized-durable` (DB).
- **File**: `services/auth/signin.go:26`

### AppState (models/system)
- **Subsystem**: `persistence`
- **Triage**: AUTO
- **Archetype**: Singleton Actor
- **Transform**: Mutex-protected actor service.
- **Evidence**: Global application state with sync primitives.
- **File**: `models/system/appstate.go`
