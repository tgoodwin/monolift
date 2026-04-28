# Miniflux Annotations

## Target Synthesis
Miniflux is a highly "liftable" application because it already externalizes state to a Postgres database. We identified two primary patterns: **Replicated Stateless Service** for core processing logic (already partially admitted) and **Worker Pool / Queue Consumer** for the background feed refresh system.

## Annotations

### worker.Run
- **Subsystem**: `background/async` (Worker Pool)
- **Owned Directories**: `internal/worker`
- **Region or Operation Identity**: `miniflux.app/v2/internal/worker.(*worker).Run:method`
- **Admitted or Refused**: Refused
- **Triage**: AUTO
- **Proposed Archetype**: Worker Pool / Queue Consumer
- **Proposed Transform**: Replace internal channel with an external broker (Redis/Postgres queue).
- **Evidence Signals Seen**: `channelLoopRule`, `lifecycle.long-running-loop`.
- **Missing Evidence**: `model.Job` serializability.
- **File References**: `internal/worker/worker.go:21`

### ProcessFeedEntries
- **Subsystem**: `core`
- **Owned Directories**: `internal/reader/processor`
- **Region or Operation Identity**: `miniflux.app/v2/internal/reader/processor.ProcessFeedEntries:function`
- **Admitted or Refused**: Admitted
- **Triage**: ADMITTED
- **Proposed Archetype**: Replicated Stateless Service
- **Proposed Transform**: Shape-preserving `http-json` transport.
- **Evidence Signals Seen**: `stateless`, `externalized-durable` (Postgres).
- **Missing Evidence**: None.
- **File References**: `internal/reader/processor/processor.go:1`
