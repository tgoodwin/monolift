# Liftability Properties Brainstorm

Status: working scratchpad for SPRINT-0009 Phase 1
Date: 2026-04-21

This document is intentionally wider than the set that will land in code.
Its purpose is to enumerate Go-expressible properties that matter to
local-to-remote rewriting, label each as admission-gating,
transport-biasing, or advisory, and map every gating candidate onto an
existing `MLV2_*` refusal code.

## Evidence rubric

Each candidate records:

- Name
- Namespace: `boundary.*`, `effects.*`, `lifecycle.*`, `contract.*`, or `transport.*`
- What the property expresses
- Why it matters for local-to-remote rewriting
- Detection sketch: `go/types`, SSA, callgraph, pointer, or AST
- Confidence: cheap-and-sound, cheap-and-heuristic, or expensive
- Outcome class: admission-gating, transport-biasing, or advisory
- Existing `MLV2_*` mapping when admission-gating
- Worked Go example

## Starting set from ADR-0017

### boundary.pass_by_value_boundary

- Namespace: `boundary.*`
- What it expresses: boundary parameters/results do not require caller-visible
  alias sharing across the remote call.
- Why it matters: a remote call cannot preserve direct pointer aliasing to the
  caller's heap.
- Detection sketch: `go/types` scan for pointer-rich boundary shapes combined
  with detector-specific exceptions for safe-by-construction values.
- Confidence: cheap-and-sound for obvious violations; unknown for deep alias
  intent.
- Outcome class: admission-gating
- `MLV2_*`: `MLV2_POINTER_ALIAS_UNSUPPORTED`
- Worked Go example:

```go
func RenameUser(ctx context.Context, u *User) error // pointer alias at boundary
```

### boundary.serializable_boundary

- Namespace: `boundary.*`
- What it expresses: every adapter-visible boundary value is structurally
  serializable or has a deterministic compiler-known encoding path.
- Why it matters: the transport boundary has to encode inputs, outputs, and
  errors without guessing.
- Detection sketch: `go/types` structural walk of params/results/receiver plus
  named-method checks for custom JSON encoding.
- Confidence: cheap-and-sound for obviously unsupported types; some named-type
  cases require conservative `Unknown`.
- Outcome class: admission-gating
- `MLV2_*`: `MLV2_SERIALIZATION_UNSUPPORTED`
- Worked Go example:

```go
type Request struct{ Cache map[chan int]string }
func Run(ctx context.Context, req Request) error
```

### effects.no_param_heap_mutation

- Namespace: `effects.*`
- What it expresses: the operation body does not mutate caller-owned memory
  reachable from receiver/params.
- Why it matters: remote execution cannot preserve in-place mutation semantics
  on the caller's heap.
- Detection sketch: SSA provenance from params/receiver through `Alloc`,
  `FieldAddr`, `IndexAddr`, and `Store`.
- Confidence: cheap-and-sound for direct stores; deeper alias chains may stay
  unknown.
- Outcome class: admission-gating
- `MLV2_*`: `MLV2_POINTER_ALIAS_UNSUPPORTED`
- Worked Go example:

```go
func SetName(ctx context.Context, u *User, name string) error {
    u.Name = name
    return nil
}
```

### contract.error_last

- Namespace: `contract.*`
- What it expresses: the terminal result is `error`, giving the caller a stable
  failure channel.
- Why it matters: Waldo-style remote failure must be visible in the source
  contract.
- Detection sketch: `go/types` result-list inspection.
- Confidence: cheap-and-sound
- Outcome class: admission-gating
- `MLV2_*`: `MLV2_NO_ERROR_CHANNEL`
- Worked Go example:

```go
func Render(ctx context.Context, req Request) Response // no error result
```

### lifecycle.execution_profile

- Namespace: `lifecycle.*`
- What it expresses: the body looks synchronous-short-lived, long-running, or
  unknown.
- Why it matters: the same admitted region may want different transports or
  deployment shapes depending on whether it behaves like a request/response
  operation or a worker loop.
- Detection sketch: SSA CFG walk for back-edges, receives, selects, goroutine
  creation, and explicit loop exits.
- Confidence: cheap-and-heuristic
- Outcome class: transport-biasing
- `MLV2_*`: none; transport bias only
- Worked Go example:

```go
func Worker(ctx context.Context, jobs <-chan Job) error {
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case job := <-jobs:
            handle(job)
        }
    }
}
```

## Candidate superset

### boundary.*

### boundary.context_first

- What it expresses: first parameter is `context.Context`.
- Why it matters: gives the selector and lifecycle detectors explicit
  cancellation/deadline vocabulary.
- Detection sketch: `go/types` first-parameter check.
- Confidence: cheap-and-sound
- Outcome class: transport-biasing
- Worked Go example:

```go
func CreateUser(ctx context.Context, req CreateUserRequest) error
```

### boundary.variadic_free

- What it expresses: public boundary has no variadic parameter.
- Why it matters: variadic forwarding is serializable only after envelope
  synthesis; the first sprint keeps the boundary model simple.
- Detection sketch: `go/types.Signature.Variadic()`.
- Confidence: cheap-and-sound
- Outcome class: admission-gating
- `MLV2_*`: `MLV2_SHAPE_UNSUPPORTED`
- Worked Go example:

```go
func Logf(ctx context.Context, format string, args ...any) error
```

### boundary.no_callable_values

- What it expresses: boundary params/results do not contain function values or
  signatures.
- Why it matters: remote calls cannot safely serialize caller callbacks.
- Detection sketch: recursive `go/types` walk looking for `*types.Signature`.
- Confidence: cheap-and-sound
- Outcome class: admission-gating
- `MLV2_*`: `MLV2_SHAPE_UNSUPPORTED`
- Worked Go example:

```go
func Visit(ctx context.Context, fn func(*Node) error) error
```

### boundary.no_streaming_values

- What it expresses: boundary params/results/receiver do not expose channel
  values.
- Why it matters: open channel identity cannot cross the boundary unchanged.
- Detection sketch: recursive `go/types` walk for `*types.Chan`.
- Confidence: cheap-and-sound
- Outcome class: admission-gating
- `MLV2_*`: `MLV2_CHANNEL_BOUNDARY`
- Worked Go example:

```go
func Consume(ctx context.Context, jobs <-chan Job) error
```

### boundary.no_sync_primitives

- What it expresses: boundary values do not expose `sync.Mutex`,
  `sync.RWMutex`, `sync.WaitGroup`, or obvious atomic wrapper state.
- Why it matters: these values encode process-local concurrency state rather
  than remotely transferrable data.
- Detection sketch: `go/types` named-type/package-path checks through struct
  fields.
- Confidence: cheap-and-sound for standard-library primitives
- Outcome class: admission-gating
- `MLV2_*`: `MLV2_SERIALIZATION_UNSUPPORTED`
- Worked Go example:

```go
type Request struct{ WG sync.WaitGroup }
func Run(ctx context.Context, req Request) error
```

### boundary.fully_instantiated

- What it expresses: receiver, params, and results contain no unresolved
  `TypeParam`.
- Why it matters: stable adapter generation needs a concrete boundary type set.
- Detection sketch: recursive `go/types` walk for `*types.TypeParam`.
- Confidence: cheap-and-sound
- Outcome class: admission-gating
- `MLV2_*`: `MLV2_SURFACE_DEFERRED_GENERIC_DECL`
- Worked Go example:

```go
func Decode[T any](ctx context.Context, raw []byte) (T, error)
```

### boundary.serializable_via_custom_encoding

- What it expresses: named boundary types with `MarshalJSON` and
  `UnmarshalJSON` count as serializable even when a raw structural walk fails.
- Why it matters: the compiler should accept real domain types that already
  expose a deterministic encoding boundary.
- Detection sketch: `go/types` method-set lookup on named types and pointer
  receivers.
- Confidence: cheap-and-sound
- Outcome class: admission-gating
- `MLV2_*`: `MLV2_SERIALIZATION_UNSUPPORTED` when absent and no structural path
- Worked Go example:

```go
type Money struct{ cents int64 }
func (Money) MarshalJSON() ([]byte, error) { return nil, nil }
func (*Money) UnmarshalJSON([]byte) error { return nil }
func Quote(ctx context.Context, m Money) (Money, error)
```

### effects.*

### effects.no_param_escape

- What it expresses: aliases derived from receiver/params do not escape to
  globals, goroutines, or closures.
- Why it matters: once a caller-owned alias escapes, remote execution no
  longer preserves the local sharing model.
- Detection sketch: SSA provenance plus pointer/callgraph assistance for
  closure capture, `Go`, and global stores.
- Confidence: expensive
- Outcome class: advisory in the brainstorm unless the spike proves cheap,
  sound behavior
- Worked Go example:

```go
var lastSeen *User
func Remember(ctx context.Context, u *User) error {
    lastSeen = u
    return nil
}
```

### effects.no_global_writes

- What it expresses: body does not write mutable package globals.
- Why it matters: hidden shared mutable state breaks the "ordinary monolith
  still works" model when part of the logic moves remote.
- Detection sketch: SSA `Store` through `*ssa.Global`.
- Confidence: cheap-and-sound
- Outcome class: admission-gating
- `MLV2_*`: `MLV2_SHARED_MUTABLE_STATE`
- Worked Go example:

```go
var issued int64
func Next(ctx context.Context) (int64, error) {
    issued++
    return issued, nil
}
```

### effects.no_global_reads

- What it expresses: body does not read mutable package globals unless they are
  effectively constant configuration.
- Why it matters: ambient mutable reads create hidden data dependencies across
  the boundary.
- Detection sketch: SSA loads from `*ssa.Global`, paired with const/immutable
  allowlist.
- Confidence: cheap-and-heuristic
- Outcome class: advisory by default
- Worked Go example:

```go
var activeTenant string
func CurrentTenant(ctx context.Context) (string, error) { return activeTenant, nil }
```

### effects.no_param_interface_callbacks

- What it expresses: body does not invoke interface methods on
  boundary-derived receivers/params.
- Why it matters: callback-style behavior can hide process-local control flow
  and side effects behind interface dispatch.
- Detection sketch: SSA invoke-site scan whose receiver provenance reaches a
  boundary value.
- Confidence: cheap-and-heuristic
- Outcome class: advisory
- Worked Go example:

```go
type Hook interface{ Run(context.Context) error }
func Execute(ctx context.Context, hook Hook) error { return hook.Run(ctx) }
```

### effects.no_reflect_unsafe

- What it expresses: reachable code does not depend on `reflect`,
  `unsafe`, or `runtime.SetFinalizer` in ways the compiler cannot model.
- Why it matters: these features defeat structural reasoning and stable
  serialization/closure boundaries.
- Detection sketch: callgraph reachability by package path plus targeted SSA
  inspection for `unsafe.Pointer`.
- Confidence: cheap-and-sound for obvious hits
- Outcome class: admission-gating
- `MLV2_*`: `MLV2_REFLECTION_DISPATCH` for reflect-heavy dynamic dispatch,
  `MLV2_UNSAFE_CODE` for unsafe use
- Worked Go example:

```go
func Cast(ctx context.Context, p unsafe.Pointer) error
```

### effects.no_os_side_effects

- What it expresses: reachable body does not directly own filesystem, process,
  or raw socket side effects that lack a modeled external dependency.
- Why it matters: these effects often require explicit dependency modeling
  rather than transparent remote execution.
- Detection sketch: callgraph package-path denylist with allowlisted pure
  packages.
- Confidence: cheap-and-heuristic
- Outcome class: advisory
- Worked Go example:

```go
func Rotate(ctx context.Context) error { return os.WriteFile("x", nil, 0o644) }
```

### contract.*

### contract.no_panic_only_failure

- What it expresses: body does not rely on `panic` as the only failure path.
- Why it matters: remote transports need a stable caller-visible error channel.
- Detection sketch: SSA `Panic` scan plus absence of terminal `error` result.
- Confidence: cheap-and-sound
- Outcome class: admission-gating
- `MLV2_*`: `MLV2_NO_ERROR_CHANNEL`
- Worked Go example:

```go
func MustParse(ctx context.Context, raw string) Value {
    panic("bad input")
}
```

### contract.deterministic_under_retry

- What it expresses: body avoids volatile sources such as `time.Now`,
  `rand.*`, UUID generation, and monotonic counters.
- Why it matters: remote retries are easier to reason about when the body is
  replay-safe.
- Detection sketch: callgraph reachability to known volatile APIs.
- Confidence: cheap-and-heuristic
- Outcome class: advisory
- Worked Go example:

```go
func Issue(ctx context.Context) (string, error) { return uuid.NewString(), nil }
```

### contract.receiver_read_only

- What it expresses: methods do not mutate receiver fields.
- Why it matters: receiver mutation is a strong signal that state placement,
  not transport alone, decides correctness.
- Detection sketch: SSA stores through receiver-derived addresses.
- Confidence: cheap-and-sound for direct stores
- Outcome class: admission-gating when the receiver is boundary-owned mutable
- `MLV2_*`: `MLV2_SHARED_MUTABLE_STATE`
- Worked Go example:

```go
func (s *Service) SetStore(store Store) error {
    s.store = store
    return nil
}
```

## lifecycle.*

### lifecycle.no_async_fork

- What it expresses: body does not spawn goroutines.
- Why it matters: async fork is a strong signal that the work is not a simple
  request/response call.
- Detection sketch: SSA `*ssa.Go`.
- Confidence: cheap-and-sound
- Outcome class: transport-biasing
- Worked Go example:

```go
func Start(ctx context.Context) error {
    go runLoop()
    return nil
}
```

### lifecycle.goroutine_joined

- What it expresses: any goroutine spawned in the body is joined before return.
- Why it matters: bounded fork/join can still behave like synchronous work.
- Detection sketch: SSA heuristic for `sync.WaitGroup` or channel join patterns.
- Confidence: cheap-and-heuristic
- Outcome class: advisory
- Worked Go example:

```go
func Parallel(ctx context.Context) error {
    var wg sync.WaitGroup
    wg.Add(1)
    go func() { defer wg.Done() }()
    wg.Wait()
    return nil
}
```

### lifecycle.long_running_loop

- What it expresses: body contains a back-edge loop whose progress depends on
  receives, `select`, or ambient state rather than a finite input size.
- Why it matters: long-running workers want channel-consumer-style deployment
  rather than per-request RPC.
- Detection sketch: SSA CFG plus loop-body receive/select inspection.
- Confidence: cheap-and-heuristic
- Outcome class: transport-biasing
- Worked Go example:

```go
func Run(ctx context.Context, jobs <-chan Job) error {
    for job := range jobs {
        handle(job)
    }
    return nil
}
```

### lifecycle.cancellation_honored

- What it expresses: long-running loops consult `ctx.Done()` or propagate
  `ctx.Err()`.
- Why it matters: remote workers need a cancellation story to remain bounded.
- Detection sketch: SSA/callgraph heuristic for `Done` and `Err` use in loops.
- Confidence: cheap-and-heuristic
- Outcome class: advisory
- Worked Go example:

```go
func Run(ctx context.Context, jobs <-chan Job) error {
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-jobs:
        }
    }
}
```

### lifecycle.bounded_work

- What it expresses: body has no unbounded loop whose exit relies on state not
  represented at the signature.
- Why it matters: a bounded request path is easier to map to `http-json`.
- Detection sketch: CFG loop inspection plus signature-state comparison.
- Confidence: cheap-and-heuristic
- Outcome class: advisory
- Worked Go example:

```go
func Poll(ctx context.Context) error {
    for !globalReady {
    }
    return nil
}
```

## transport.*

### transport.handler_boundary

- What it expresses: the operation boundary matches a known handler surface
  such as `net/http.Handler` or Caddy middleware.
- Why it matters: selector evidence for shape-preserving handler transport.
- Detection sketch: current handler predicates, moved out of admission.
- Confidence: cheap-and-sound
- Outcome class: transport-biasing
- Worked Go example:

```go
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request)
```

### transport.receiver_returns_self

- What it expresses: builder-chain pattern where the receiver or closely
  related value is returned for fluent configuration.
- Why it matters: root builders are wiring/configuration logic, not remotely
  invocable service work.
- Detection sketch: existing builder-chain signature predicate.
- Confidence: cheap-and-sound
- Outcome class: admission-gating
- `MLV2_*`: `MLV2_BUILDER_CHAIN_ROOT`
- Worked Go example:

```go
func (b *Builder) WithHeader(k, v string) *Builder
```

## Outcome labels and code mappings

The intended phase-2 split from this brainstorm is:

- Admission-gating now: `boundary.pass_by_value_boundary`,
  `boundary.serializable_boundary`, `boundary.variadic_free`,
  `boundary.no_callable_values`, `boundary.no_streaming_values`,
  `boundary.no_sync_primitives`, `boundary.fully_instantiated`,
  `boundary.serializable_via_custom_encoding`, `effects.no_param_heap_mutation`,
  `effects.no_global_writes`, `effects.no_reflect_unsafe`,
  `contract.error_last`, `contract.no_panic_only_failure`,
  `contract.receiver_read_only`, `transport.receiver_returns_self`.
- Transport-biasing now: `lifecycle.execution_profile`,
  `boundary.context_first`, `lifecycle.no_async_fork`,
  `lifecycle.long_running_loop`, `transport.handler_boundary`.
- Advisory now: `effects.no_param_escape`, `effects.no_global_reads`,
  `effects.no_param_interface_callbacks`, `effects.no_os_side_effects`,
  `contract.deterministic_under_retry`, `lifecycle.goroutine_joined`,
  `lifecycle.cancellation_honored`, `lifecycle.bounded_work`.

If a candidate cannot map cleanly onto an existing refusal code, it is not
allowed to gate admission in this sprint. That rule demotes several desirable
but not yet taxonomy-stable detectors to advisory.

## Cross-check against the v2 contract baseline

The PLOS-derived baseline in `docs/specs/monolift-v2-contract.md`
commits Monolift to bounded lifts, refusal over guessing, and no hidden global
heap-sharing semantics. This brainstorm stays within that fence:

- Properties about serialization, aliasing, callbacks, channels, and global
  mutation make the bounded-lift commitment more explicit rather than wider.
- Heuristic lifecycle and effect properties stay advisory by default, which
  preserves the contract's "refuse what cannot be proven" stance.
- No property here assumes language extensions, whole-program theorem proving,
  or non-Go concepts with no Go realization.

## Signature-only checks: retain, promote, or demote

Current signature-only checks split as follows:

- Keep as selector-only signals:
  `isHTTPHandler`, `isChannelConsumer`, `isCtxRequestResponse`,
  `isMultiDomainArgs`, and `isNoResponse`. These remain useful transport
  vocabulary but no longer gate admission on their own.
- Keep as an admission property:
  `isBuilderChain` survives as `transport.receiver_returns_self` and still maps
  to `MLV2_BUILDER_CHAIN_ROOT`.
- Re-express unsupported boundary helpers as named properties:
  today's `unsupportedEvidence` checks become `boundary.variadic_free`,
  `boundary.no_streaming_values`, `boundary.no_callable_values`, and the
  boundary/serialization properties above.
