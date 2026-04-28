# Liftability Properties

Status: accepted for implementation in SPRINT-0009
Date: 2026-04-21

This document narrows the Phase 1 brainstorm to the implementable property
set for the SPRINT-0009 classifier rewrite. Admission is decided from named
properties and their evidence. Canonical shapes remain in the system, but as
downstream selector outputs rather than admissibility gates.

## Property naming and IDs

Every property has:

- A stable spec name in namespaced kebab-case, such as
  `boundary.variadic-free`
- A stable Go `PropertyID`, such as `PropertyBoundaryVariadicFree`
- A fixed outcome class: `gate`, `bias`, or `advisory`

Future additions append to this set; they do not rename existing entries.

## Evidence record convention

Every detector emits zero or more evidence records with the following fields:

- `PropertyID`
- `Subject`: `receiver`, `param[n]`, `result[n]`, or `body`
- `Verdict`: `Hold`, `Violate`, or `Unknown`
- `Source`: `types`, `ssa`, or `callgraph`
- `Detail`: deterministic short string

Deterministic ordering rules:

1. Sort by `PropertyID` lexical order.
2. Within one property, sort subjects as `receiver`, `param[0..n]`,
   `result[0..n]`, then `body`.
3. Within the same subject, sort sources as `types`, `ssa`, `callgraph`.
4. Within the same source, sort `Detail` lexically.

Detectors that analyze multiple SSA instructions MUST normalize details to
stable strings such as package-qualified symbol names or field paths. Raw
program counters, map iteration order, and Go pointer addresses are forbidden
in `Detail`.

## Heuristic containment rule

Sound detectors may gate admission. Heuristic detectors default to `Unknown`
and remain advisory unless later evidence promotes them. This is the
anti-regression rule for SPRINT-0009: a heuristic may bias transport or
enrich reports, but it must not create a new false refusal against the corpus.

## Property catalog

| Name | PropertyID | Pass | Outcome | Refusal code | Evidence template |
|---|---|---|---|---|---|
| `boundary.context-first` | `PropertyBoundaryContextFirst` | `types` | `bias` | - | `first parameter is context.Context` |
| `boundary.variadic-free` | `PropertyBoundaryVariadicFree` | `types` | `gate` | `MLV2_SHAPE_UNSUPPORTED` | `signature is variadic` |
| `boundary.no-callable-values` | `PropertyBoundaryNoCallableValues` | `types` | `gate` | `MLV2_SHAPE_UNSUPPORTED` | `boundary contains func-typed value at <subject>` |
| `boundary.no-streaming-values` | `PropertyBoundaryNoStreamingValues` | `types` | `gate` | `MLV2_CHANNEL_BOUNDARY` | `boundary contains channel-typed value at <subject>` |
| `boundary.no-sync-primitives` | `PropertyBoundaryNoSyncPrimitives` | `types` | `gate` | `MLV2_SERIALIZATION_UNSUPPORTED` | `boundary exposes sync primitive <type>` |
| `boundary.fully-instantiated` | `PropertyBoundaryFullyInstantiated` | `types` | `gate` | `MLV2_SURFACE_DEFERRED_GENERIC_DECL` | `boundary contains unresolved type parameter <name>` |
| `boundary.serializable-via-custom-encoding` | `PropertyBoundarySerializableViaCustomEncoding` | `types` | `gate` | `MLV2_SERIALIZATION_UNSUPPORTED` | `type <T> is structurally serializable or has MarshalJSON/UnmarshalJSON` |
| `contract.error-last` | `PropertyContractErrorLast` | `types` | `gate` | `MLV2_NO_ERROR_CHANNEL` | `terminal result is error` |
| `effects.no-param-heap-mutation` | `PropertyEffectsNoParamHeapMutation` | `ssa` | `gate` | `MLV2_POINTER_ALIAS_UNSUPPORTED` | `store through param/receiver-derived address <path>` |
| `effects.no-param-escape` | `PropertyEffectsNoParamEscape` | `ssa` | `advisory` | - | `boundary-derived alias escapes to closure/global/goroutine` |
| `effects.no-global-writes` | `PropertyEffectsNoGlobalWrites` | `ssa` | `gate` | `MLV2_SHARED_MUTABLE_STATE` | `store to mutable global <pkg>.<name>` |
| `effects.no-global-reads` | `PropertyEffectsNoGlobalReads` | `ssa` | `advisory` | - | `load from mutable global <pkg>.<name>` |
| `effects.no-param-interface-callbacks` | `PropertyEffectsNoParamInterfaceCallbacks` | `ssa` | `advisory` | - | `invoke on boundary-derived interface receiver <method>` |
| `effects.no-reflect-unsafe` | `PropertyEffectsNoReflectUnsafe` | `callgraph` | `gate` | `MLV2_UNSAFE_CODE` | `reachable call hits reflect/unsafe/finalizer boundary <symbol>` |
| `effects.no-os-side-effects` | `PropertyEffectsNoOSSideEffects` | `callgraph` | `advisory` | - | `reachable call hits filesystem/process/socket package <symbol>` |
| `contract.no-panic-only-failure` | `PropertyContractNoPanicOnlyFailure` | `ssa` | `gate` | `MLV2_NO_ERROR_CHANNEL` | `panic path exists without error return` |
| `contract.receiver-read-only` | `PropertyContractReceiverReadOnly` | `ssa` | `advisory` | - | `store through receiver-derived address <path>` |
| `lifecycle.no-async-fork` | `PropertyLifecycleNoAsyncFork` | `ssa` | `bias` | - | `body spawns goroutine` |
| `lifecycle.long-running-loop` | `PropertyLifecycleLongRunningLoop` | `ssa` | `bias` | - | `loop body contains receive/select/back-edge worker pattern` |
| `lifecycle.execution-profile` | `PropertyLifecycleExecutionProfile` | `ssa` | `bias` | - | `detail=sync-short` or `detail=long-running` |
| `transport.handler-boundary` | `PropertyTransportHandlerBoundary` | `types` | `bias` | - | `signature matches net/http or caddy handler boundary` |
| `transport.receiver-returns-self` | `PropertyTransportReceiverReturnsSelf` | `types` | `gate` | `MLV2_BUILDER_CHAIN_ROOT` | `method returns receiver/builder self` |

## Admission rule in this sprint

An operation is admitted when:

- Every `gate` property either `Hold`s or is irrelevant for that subject.
- No `gate` property emits `Violate`.
- Heuristic/advisory detectors may emit `Unknown` or `Violate` without
  refusing admission.

Root aggregation is strict: every exposed operation in the root must be
admitted for the root to be liftable. Mixed operation outcomes produce a
single root refusal with per-operation evidence.

## Selector-facing facts in this sprint

The selector consumes:

- `PropertyTransportHandlerBoundary`
- `PropertyLifecycleLongRunningLoop`
- `PropertyLifecycleExecutionProfile`
- `PropertyContractErrorLast`
- The preserved transport-signal helpers derived from the old archetype code:
  handler, ctx/request/response, multi-domain-args, no-response, and
  channel-consumer signals

The guardrail is explicit: selector rules may inspect raw signature helpers,
but no selector rule is terminal on a raw signature predicate alone. A named
property fact must participate in every terminal rule.
