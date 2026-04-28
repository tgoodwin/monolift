# Target Annotation: Miniflux

## Synthesis
- **Dominant archetypes**: Worker Pool (Feed Refresh), Replicated Stateless Service (Storage/API).
- **The AUTO set**:
    - `internal/worker.Pool`: classic Worker Pool.
    - `internal/storage.Storage`: already admitted as replicated via externalized-durable.
- **Hardest ambiguities**:
    - Feed Processor logic. Some parts might be stateful (caching), but current report shows it's admitted.
- **Most important evidence gaps**:
    - Better classification of "process-local-cache" vs "stateless" for feed parsing.

---

## Annotations

### Subsystem: Feed Refresh Worker Pool
- **owned directories**: `internal/worker/`
- **region or operation identity**: `miniflux.app/v2/internal/worker.Pool` / struct
- **admitted or refused**: REFUSED (usually, because of channel usage)
- **triage**: AUTO
- **proposed archetype**: Worker Pool / Queue Consumer
- **proposed candidate state class**: `worker-pool-channel`
- **proposed transform**: Broker-backed queue feeding Worker replicas.
- **competing archetypes**: Pipeline Stage.
- **evidence signals seen**: `chan model.Job` field; `go worker.Run` loop.
- **missing evidence**: None.
- **file references**: `internal/worker/pool.go:12`

### Subsystem: Storage
- **owned directories**: `internal/storage/`
- **region or operation identity**: `miniflux.app/v2/internal/storage.Storage` / struct
- **admitted or refused**: ADMITTED
- **triage**: ADMITTED
- **proposed archetype**: Replicated Stateless Service
- **proposed candidate state class**: `externalized-durable`
- **proposed transform**: N replicas behind LB; shared DB.
- **competing archetypes**: None.
- **evidence signals seen**: Capture of `*sql.DB`.
- **missing evidence**: None.
- **file references**: `internal/storage/storage.go:12`

### Subsystem: API / UI Handlers
- **owned directories**: `internal/api/`, `internal/ui/`
- **region or operation identity**: `miniflux.app/v2/internal/api` / package
- **admitted or refused**: ADMITTED (likely)
- **triage**: ADMITTED
- **proposed archetype**: HTTP Handler
- **proposed candidate state class**: `stateless-http`
- **proposed transform**: Standard ingress.
- **competing archetypes**: None.
- **evidence signals seen**: Usage of `net/http` handlers.
- **missing evidence**: None.
- **file references**: `internal/api/api.go:1`
