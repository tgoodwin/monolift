# Target Annotation: PocketBase

## Synthesis
- **Dominant archetypes**: Singleton Actor (App/BaseApp), Event-Bus Publisher (Subscriptions), Worker Pool (Cron/Backup).
- **The AUTO set**:
    - `tools/subscriptions.Broker`: Event-Bus Publisher.
    - `tools/cron.Cron`: Worker Pool.
- **Hardest ambiguities**:
    - `core.App`: It's a massive monolith component. Singleton Actor is the only safe default, but performance will be poor. Sharding by "collection" is a candidate for Sharded Stateful Service.
- **Most important evidence gaps**:
    - Cross-collection join detection to determine if collection-based sharding is safe.

---

## Annotations

### Subsystem: Core App
- **owned directories**: `core/`
- **region or operation identity**: `github.com/pocketbase/pocketbase/core.App` / interface
- **admitted or refused**: REFUSED
- **triage**: SUGGEST
- **proposed archetype**: Singleton Actor
- **proposed candidate state class**: `singleton-mutable-embedded-db`
- **proposed transform**: Emit a single service instance for the App; serialize all method calls.
- **competing archetypes**: Sharded Stateful Service (rejected pending evidence of collection isolation).
- **evidence signals seen**: `MLV2_EMBEDDED_DB_APP_ROOT` diagnostic.
- **missing evidence**: Isolation evidence between collections to allow sharding.
- **file references**: `core/app.go:29`

### Subsystem: Realtime Subscriptions
- **owned directories**: `tools/subscriptions/`
- **region or operation identity**: `github.com/pocketbase/pocketbase/tools/subscriptions.Broker` / struct
- **admitted or refused**: REFUSED
- **triage**: AUTO
- **proposed archetype**: Event-Bus Publisher
- **proposed candidate state class**: `publisher-state`
- **proposed transform**: Replace internal broker with a managed pub/sub system (e.g., Redis, NATS).
- **competing archetypes**: Singleton Actor.
- **evidence signals seen**: Usage of `clients` map and broadcast loops.
- **missing evidence**: None.
- **file references**: `tools/subscriptions/broker.go:20`

### Subsystem: Cron / Tasks
- **owned directories**: `tools/cron/`
- **region or operation identity**: `github.com/pocketbase/pocketbase/tools/cron.Cron` / struct
- **admitted or refused**: REFUSED
- **triage**: AUTO
- **proposed archetype**: Worker Pool / Queue Consumer
- **proposed candidate state class**: `worker-pool-cron`
- **proposed transform**: Distributed scheduler feeding a worker pool.
- **competing archetypes**: Ephemeral Worker.
- **evidence signals seen**: Timer-based execution; task registry.
- **missing evidence**: None.
- **file references**: `tools/cron/cron.go:15`
