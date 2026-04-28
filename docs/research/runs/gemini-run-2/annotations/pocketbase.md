# PocketBase Annotation & Coverage Ledger

## Target Information
- **Name**: PocketBase
- **Total Go Files**: 445

## Coverage Ledger

| Bundle | Subsystem | Directories | File Count | Status |
| :--- | :--- | :--- | :--- | :--- |
| **P-ALL** | full | `.` | 445 | DONE |

## Annotations

### P-APP-001: PocketBase App Interface
- **subsystem**: core
- **owned directories**: `core/`
- **region or operation identity**: `github.com/pocketbase/pocketbase/core:App` (interface)
- **admitted or refused**: refused
- **triage**: AUTO
- **proposed archetype**: singleton-actor
- **proposed candidate state class**: externalized-durable (for DB) / singleton-mutable (for hooks/bus)
- **proposed transform**: Transform `App` into a distributed service; externalize SQLite to a managed DB (or sidecar); route hooks via an event bus.
- **competing archetypes considered**: None
- **evidence signals seen**: `embedded-db`, widespread use of `sync.Mutex` in implementations, centralized hook registry.
- **missing evidence**: Proof that SQLite state can be transparently externalized without breaking transaction semantics.
- **file references**: `evaluation/pocketbase/core/app.go:29`
