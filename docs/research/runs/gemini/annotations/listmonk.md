# Listmonk Annotations

## Target Synthesis
Listmonk demonstrates two clear archetypes:
1. **Worker Pool / Queue Consumer**: The central `manager.worker` loop which processes campaign messages from a channel. This is currently a hard refusal due to the channel boundary, but is a prime candidate for a managed queue transform.
2. **Replicated Stateless Service**: Most API handlers on the `App` struct are thin wrappers around database calls. They are "stateless" from the compiler's perspective once the database is externalized.

## Coverage Ledger
- `internal/manager`: 100% (primary worker identified)
- `cmd`: ~50% (representative API handlers walked)
- Total Files: 92

## Annotations

### Manager.worker
- **Subsystem**: `background/async`
- **Owned Directories**: `internal/manager`
- **Region or Operation Identity**: `github.com/knadh/listmonk/internal/manager.(*Manager).worker:method`
- **Admitted or Refused**: Refused
- **Triage**: AUTO
- **Proposed Archetype**: Worker Pool / Queue Consumer
- **Proposed Transform**: Managed queue (SQS/NATS) replaces `campMsgQ`; `worker` becomes a worker service.
- **Evidence Signals Seen**: `channelLoopRule`, `lifecycle.long-running-loop`
- **Missing Evidence**: Message serializability proof.
- **File References**: `internal/manager/manager.go:462`

### App.GetUser
- **Subsystem**: `ingress`
- **Owned Directories**: `cmd`
- **Region or Operation Identity**: `github.com/knadh/listmonk/cmd.(*App).GetUser:method`
- **Admitted or Refused**: Refused
- **Triage**: AUTO
- **Proposed Archetype**: Replicated Stateless Service
- **Proposed Transform**: Shape-preserving `handler` transport; `App` replicated.
- **Evidence Signals Seen**: `transport.handler-boundary`, `externalized-durable`
- **Missing Evidence**: `echo.Context` adapter metadata.
- **File References**: `cmd/users.go:12`

### App.CreateUser
- **Subsystem**: `ingress`
- **Owned Directories**: `cmd`
- **Region or Operation Identity**: `github.com/knadh/listmonk/cmd.(*App).CreateUser:method`
- **Admitted or Refused**: Refused
- **Triage**: AUTO
- **Proposed Archetype**: Replicated Stateless Service
- **Proposed Transform**: Same as GetUser.
- **Evidence Signals Seen**: `transport.handler-boundary`, `externalized-durable`
- **Missing Evidence**: None beyond GetUser.
- **File References**: `cmd/users.go:30`
