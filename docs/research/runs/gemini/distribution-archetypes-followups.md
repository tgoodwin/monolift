# Candidate State-Class Additions (ADR-0016 Follow-up)

Based on the SPRINT-0013 corpus walk, the following state classes are proposed for addition to `pkg/compiler/stateclass/`. These classes refine the `refused` state and enable the "AUTO" triage for new distribution archetypes.

## 1. `worker-pool-queue`
- **Archetype**: Worker Pool / Queue Consumer
- **Description**: Channel-typed state that acts as a task producer or consumer.
- **Evidence**: `chan` types in struct fields or function arguments; usage in `for-range` or `select` loops.
- **Transform**: Managed Broker (Redis/NATS).

## 2. `mutex-protected-singleton`
- **Archetype**: Singleton Actor
- **Description**: State protected by a `sync.Mutex` or `sync.RWMutex` that manages a local resource or configuration.
- **Evidence**: `syncPrimitiveRule` + field-level mutation + single-instance constructor (`init` or `sync.Once`).
- **Transform**: Singleton Service with RPC proxy.

## 3. `broadcast-event-bus`
- **Archetype**: Event-Bus Publisher / Subscriber
- **Description**: A slice or map of interfaces/functions used for event fan-out.
- **Evidence**: Registry pattern (`Register(...)`), iteration over subscribers during a "notify" operation.
- **Transform**: Managed Pub/Sub.

## 4. `replicated-cache-affinity`
- **Archetype**: Session-Scoped State
- **Description**: Keyed state in a local map or LRU cache that can be distributed or sharded.
- **Evidence**: `sync.RWMutex` + `map[K]V` + `Get/Set` patterns + Key derived from Request/Session.
- **Transform**: Distributed Cache (Redis/Memcached).

## 5. `filesystem-bound-singleton`
- **Archetype**: Singleton Actor
- **Description**: State or operations that directly interact with the local OS filesystem.
- **Evidence**: `os` and `filepath` package calls in closures.
- **Transform**: Sidecar with volume mapping or Object Store client.

## 6. `captured-config-singleton`
- **Archetype**: Replicated Stateless Service / Singleton Actor
- **Description**: Read-heavy state initialized at boot; rarely updated.
- **Evidence**: `modules/setting` or `config` package usage; few or no writes after bootstrap.
- **Transform**: Replicated configuration with broadcast updates.
