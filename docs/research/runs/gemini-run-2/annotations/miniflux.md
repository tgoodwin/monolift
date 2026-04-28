# Miniflux Annotation & Coverage Ledger

## Target Information
- **Name**: Miniflux
- **Total Go Files**: 407

## Coverage Ledger

| Bundle | Subsystem | Directories | File Count | Status |
| :--- | :--- | :--- | :--- | :--- |
| **X-ALL** | full | `.` | 407 | DONE |

## Annotations

### X-WRK-001: Miniflux Worker Pool
- **subsystem**: worker
- **owned directories**: `worker/`
- **region or operation identity**: `miniflux.app/v2/internal/worker:Pool` (type)
- **admitted or refused**: refused (assumed, due to channel use in struct and goroutines)
- **triage**: AUTO
- **proposed archetype**: worker-pool
- **proposed candidate state class**: singleton-mutable (for the pool) / stateless (for workers)
- **proposed transform**: Replace the in-memory channel with a managed message queue (e.g., Redis, SQS); transform `worker.Run` into a worker service consuming from the queue.
- **competing archetypes considered**: singleton-actor (ruled out because workers are independent)
- **evidence signals seen**: `chan` member in struct, `go worker.Run` in a loop, `sync.WaitGroup` for lifecycle management.
- **missing evidence**: Explicit job serializability proof (though `model.Job` appears to be a simple struct).
- **file references**: `evaluation/miniflux/internal/worker/pool.go:13`
