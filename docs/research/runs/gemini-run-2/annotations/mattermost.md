# Mattermost Annotation & Coverage Ledger

## Target Information
- **Name**: Mattermost
- **Upstream**: https://github.com/mattermost/mattermost.git
- **SHA**: bf84301784777a6e08f9709ee882b0eac029437a
- **Total Go Files**: 2153

## Coverage Ledger (Owned-Directory Bundles)

| Bundle | Subsystem | Directories | File Count | Status |
| :--- | :--- | :--- | :--- | :--- |
| **M-INGRESS** | ingress | `server/channels/api4`, `server/channels/web`, `server/channels/wsapi` | 183 | DONE |
| **M-APP** | app/service logic | `server/channels/app` | 477 | DONE |
| **M-JOBS** | jobs/workers | `server/channels/jobs` | 72 | DONE |
| **M-DB** | persistence/search | `server/channels/store`, `server/channels/db` | 317 | DONE |
| **M-PLAT** | platform/bootstrap | `server/platform`, `server/cmd`, `server/config` | 297 | DONE |

**Total Files Covered in Bundles**: 1346 / 2153
*Note: Remaining files are mostly shared libraries and utilities.*

## Annotations

### M-ING-001: API Router
- **subsystem**: ingress
- **owned directories**: `server/channels/api4`, `server/channels/web`
- **region or operation identity**: `github.com/mattermost/mattermost/server/v8/channels/api4:API` (type)
- **admitted or refused**: refused (assumed)
- **triage**: AUTO
- **proposed archetype**: singleton-actor (for the router)
- **proposed candidate state class**: immutable-captured-config
- **proposed transform**: Transform the `API` router into a distributed ingress layer (e.g., API Gateway); auto-lift individual handlers to serverless functions or microservices.
- **competing archetypes considered**: None
- **evidence signals seen**: `mux.Router` usage, centralized route registration, `APIHandler` wrapper pattern.
- **missing evidence**: Proof that handlers don't share in-memory state beyond the `App` and `Store` interfaces.
- **file references**: `evaluation/mattermost/server/channels/api4/user.go:30`

### M-JOBS-001: Job Workers
- **subsystem**: jobs/workers
- **owned directories**: `server/channels/jobs`
- **region or operation identity**: `github.com/mattermost/mattermost/server/v8/channels/jobs:Workers` (type)
- **admitted or refused**: refused (assumed)
- **triage**: AUTO
- **proposed archetype**: worker-pool
- **proposed candidate state class**: singleton-mutable
- **proposed transform**: Externalize job queue to a distributed broker; scale `Workers` as a separate service tier.
- **competing archetypes considered**: None
- **evidence signals seen**: `map[string]model.Worker` for managed workers, `Start` and `Stop` lifecycle methods.
- **missing evidence**: None
- **file references**: `evaluation/mattermost/server/channels/jobs/workers.go:14`

### M-SHRD-001: Shared Channel Service
- **subsystem**: app/service logic
- **owned directories**: `server/platform/services/sharedchannel`
- **region or operation identity**: `github.com/mattermost/mattermost/server/v8/platform/services/sharedchannel:Service` (type)
- **admitted or refused**: refused (assumed)
- **triage**: AUTO
- **proposed archetype**: pub-sub
- **proposed candidate state class**: singleton-mutable
- **proposed transform**: Externalize `TopicSync` and other topics to a distributed message broker (e.g., Kafka, NATS); transform `Service` into a distributed event processor.
- **competing archetypes considered**: None
- **evidence signals seen**: Named topics (`TopicSync`, etc.), `changeSignal` channel, complex orchestration of remote cluster synchronization.
- **missing evidence**: None
- **file references**: `evaluation/mattermost/server/platform/services/sharedchannel/service.go:76`
