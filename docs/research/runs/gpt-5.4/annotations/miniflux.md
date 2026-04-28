# Miniflux Annotation Notes - gpt-5.4 run

Catalog links: [queue-backed worker](../archetype-catalog-v1.md#queue-backed-worker), [serialized-singleton-owner](../archetype-catalog-v1.md#serialized-singleton-owner).

## Target Synthesis

Miniflux is useful because it contains both ends of the sprint story:

- one already admitted narrow root, `ProcessFeedEntries`, showing the current compiler surface
- one clean refused worker-pool shape that should become AUTO under an archetype-aware compiler

Headline AUTO set:

- `internal/worker.Pool` plus `worker.Run`

Headline SUGGEST set:

- `internal/proxyrotator.ProxyRotator`

Headline ADMITTED set:

- `internal/reader/processor.ProcessFeedEntries` with `externalized-durable` storage already reflected in the committed golden report

## Coverage Ledger

| Bundle | Status | Note |
|---|---|---|
| `internal/reader/processor` | findings | Existing admitted root plus a good contrast case for the current compiler surface. |
| `internal/worker` | findings | Strong queue-backed worker evidence. |
| `internal/proxyrotator` | findings | Singleton-owner shape with incomplete alias proof. |
| remainder of `internal` | no relevant archetype surface observed | Mostly synchronous readers, HTTP helpers, or data-model code. |

## Region Findings

### Region 1

- `subsystem`: feed processing root
- `owned directories`: `evaluation/miniflux/internal/reader/processor`
- `region or operation identity`: `ProcessFeedEntries`
- `admitted or refused`: already admitted
- `triage`: `ADMITTED`
- `proposed archetype`: existing replicated service over externalized durable state
- `proposed candidate state class`: none; committed report already assigns `externalized-durable`
- `proposed transform`: current `http-json-function` style adapter is already sufficient
- `competing archetypes considered`: `queue-backed worker`
- `evidence signals seen`: committed report, external Postgres store, no in-process mutable owner requirement
- `missing evidence`: none
- `file references`: `test/e2e/targets/miniflux/golden/report.json`, `evaluation/miniflux/internal/reader/processor/processor.go:27`

### Region 2

- `subsystem`: background refresh workers
- `owned directories`: `evaluation/miniflux/internal/worker`
- `region or operation identity`: `Pool`, `NewPool`, `worker.Run`
- `admitted or refused`: refused today because job dispatch is in-process channel plus goroutine ownership
- `triage`: `AUTO`
- `proposed archetype`: `queue-backed worker`
- `proposed candidate state class`: `queued-workset`
- `proposed transform`: replace the in-process job queue with a durable queue and keep `RefreshFeed` as the worker body
- `competing archetypes considered`: `scheduled-reconciler`
- `evidence signals seen`: explicit job channel, worker pool, independent job payloads, side effects already externalized through storage
- `missing evidence`: none material for v1
- `file references`: `evaluation/miniflux/internal/worker/pool.go:15`, `evaluation/miniflux/internal/worker/pool.go:34`, `evaluation/miniflux/internal/worker/worker.go:24`

### Region 3

- `subsystem`: proxy selection
- `owned directories`: `evaluation/miniflux/internal/proxyrotator`
- `region or operation identity`: package-global `ProxyRotatorInstance`, `ProxyRotator`
- `admitted or refused`: refused today because mutable selection state is process-local
- `triage`: `SUGGEST`
- `proposed archetype`: `serialized-singleton-owner`
- `proposed candidate state class`: `owned-mutable-singleton`
- `proposed transform`: emit a single-owner proxy-rotation service and route `NextProxy` through it
- `competing archetypes considered`: `queue-backed worker`
- `evidence signals seen`: singleton instance, mutable index, sequential access requirement
- `missing evidence`: alias and lifecycle proof for the singleton instance is not yet closed-form from the source alone
- `file references`: `evaluation/miniflux/internal/proxyrotator/proxyrotator.go:11`, `evaluation/miniflux/internal/proxyrotator/proxyrotator.go:14`, `evaluation/miniflux/internal/proxyrotator/proxyrotator.go:21`, `evaluation/miniflux/internal/reader/processor/processor.go:57`
