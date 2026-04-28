# Target Annotation: Listmonk

## Synthesis
- **Dominant archetypes**: Singleton Actor (Core), Worker Pool (Manager).
- **The AUTO set**: 
    - `internal/core.Core`: Would become auto-liftable as a Singleton Actor if context-first is relaxed or auto-injected.
    - `internal/manager.Manager`: Classic Worker Pool archetype.
- **Hardest ambiguities**: 
    - `internal/core.Core` methods that perform DB operations. Are they "stateless" if they only touch DB? Yes, but they currently violate signature rules.
- **Most important evidence gaps**: 
    - Detection of "pure DB" operations to justify Replicated Stateless Service instead of Singleton Actor.

---

## Annotations

### Subsystem: Core CRUD
- **owned directories**: `internal/core/`
- **region or operation identity**: `github.com/knadh/listmonk/internal/core.Core` / struct
- **admitted or refused**: REFUSED
- **triage**: AUTO
- **proposed archetype**: Singleton Actor
- **proposed candidate state class**: `singleton-mutable-mutex`
- **proposed transform**: Emit a service for Core; serialize calls.
- **competing archetypes considered**: Replicated Stateless Service (rejected because it might have internal caches or state I haven't seen yet).
- **evidence signals seen**: `boundary.context-first` violation; DB handle capture.
- **missing evidence**: Proof that internal fields (other than DB) are immutable.
- **file references**: `internal/core/core.go:34`

### Subsystem: Background Manager
- **owned directories**: `internal/manager/`
- **region or operation identity**: `github.com/knadh/listmonk/internal/manager.Manager` / struct
- **admitted or refused**: REFUSED
- **triage**: AUTO
- **proposed archetype**: Worker Pool / Queue Consumer
- **proposed candidate state class**: `worker-pool-channel`
- **proposed transform**: Broker-backed queue feeding Manager replicas.
- **competing archetypes**: Pipeline Stage.
- **evidence signals seen**: Usage of channels for job distribution.
- **missing evidence**: Verification that jobs are fully serializable.
- **file references**: `internal/manager/manager.go:28`

### Subsystem: Messenger
- **owned directories**: `internal/messenger/`
- **region or operation identity**: `github.com/knadh/listmonk/internal/messenger`
- **admitted or refused**: REFUSED
- **triage**: AUTO
- **proposed archetype**: Replicated Stateless Service
- **proposed candidate state class**: `stateless-adapter`
- **proposed transform**: N replicas behind LB.
- **competing archetypes**: Singleton Actor.
- **evidence signals seen**: Stateless transformation of data to email/postback.
- **missing evidence**: None.
- **file references**: `internal/messenger/email/email.go:21`
