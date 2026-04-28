# Pocketbase Annotations

## Target Synthesis
Pocketbase's monolithic `App` interface (190+ methods) is the primary obstacle to distribution. However, our walk identified the internal `hook.Hook` mechanism as a prime candidate for the **Event-Bus Publisher / Subscriber** archetype. By decoupling these hooks, we can lift custom domain logic (validators, side-effects) out of the core binary.

## Annotations

### BaseApp.onRecordCreate (and other hooks)
- **Subsystem**: `domain logic` (Hooks)
- **Owned Directories**: `core`
- **Region or Operation Identity**: `github.com/pocketbase/pocketbase/core.BaseApp.onRecordCreate:hook`
- **Admitted or Refused**: Refused
- **Triage**: AUTO
- **Proposed Archetype**: Event-Bus Publisher / Subscriber
- **Proposed Transform**: Replace `hook.Hook` with a managed Pub/Sub (NATS). `BaseApp` publishes; lifted handlers subscribe.
- **Evidence Signals Seen**: `syncPrimitiveRule` (inside hook), `broadcast-event-bus`.
- **Missing Evidence**: Statically provable hook-handler serializability.
- **File References**: `core/base.go:41`

### BaseApp (The God Object)
- **Subsystem**: `platform`
- **Owned Directories**: `core`
- **Region or Operation Identity**: `github.com/pocketbase/pocketbase/core.App:interface`
- **Admitted or Refused**: Refused
- **Triage**: TERMINAL
- **Proposed Archetype**: N/A
- **Proposed Transform**: N/A
- **Evidence Signals Seen**: `closure-size` (> 1000 symbols), `embedded-db` (SQLite).
- **Missing Evidence**: A sharded DB archetype and significant interface decomposition.
- **File References**: `core/app.go:29`
