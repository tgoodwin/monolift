# Mattermost Coverage Ledger

| Bundle | Subsystem | Owned Directories | File Count (*.go) |
|---|---|---|---|
| ingress | Ingress/Routing | `server/channels/api4`, `server/channels/web`, `server/channels/wsapi` | 183 |
| app/service logic | Core App Logic | `server/channels/app` | 477 |
| long-lived / fanout | WebSockets/Events | (subset of app, api4, wsapi) | 167 (files with Hub/WebSocket/Broker) |
| jobs/workers | Background Jobs | `server/channels/jobs` | 72 |
| persistence/search | DB/Store | `server/channels/store`, `server/channels/db` | 317 |
| platform/bootstrap | Bootstrap/Config | `server/platform`, `server/cmd`, `server/config` | 297 |

**Total Files**: 1513 (tracked in bundles) / 2153 (total)

## Synthesis
Mattermost's architecture is highly conducive to distribution via the **Replicated Stateless Service** archetype (for API and App logic) and the **Singleton Actor** archetype (for configuration and WebSocket management). The `Hub` and `WebConn` patterns in the fanout bundle are particularly interesting as candidates for sharded singleton actors.

## Key Annotations

### WebHub (app/platform)
- **Subsystem**: `fanout`
- **Triage**: AUTO
- **Archetype**: Singleton Actor (Sharded)
- **Transform**: Centralized hub service managing user connections.
- **Evidence**: `sync.Mutex`, complex fan-out logic, user-affinity.
- **File**: `server/channels/app/platform/web_hub.go`

### config.Store
- **Subsystem**: `bootstrap`
- **Triage**: AUTO
- **Archetype**: Singleton Actor
- **Transform**: gRPC-backed configuration provider with broadcast updates.
- **Evidence**: `sync.RWMutex`, config listeners.
- **File**: `server/config/store.go:30`

### CreatePost (app)
- **Subsystem**: `core logic`
- **Triage**: AUTO
- **Archetype**: Replicated Stateless Service
- **Transform**: HTTP/RPC delegation.
- **Evidence**: Request-scoped context, delegates all state to `Store`.
- **File**: `server/channels/app/post.go:37`

### JobServer (jobs)
- **Subsystem**: `jobs`
- **Triage**: AUTO
- **Archetype**: Worker Pool / Queue Consumer
- **Transform**: Decouple into a dedicated worker service.
- **Evidence**: `SimpleWorker` and `JobServer` coordination logic.
- **File**: `server/channels/jobs/server.go`
