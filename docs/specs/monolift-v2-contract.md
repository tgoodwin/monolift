---
title: Monolift v2 Contract Specification
status: accepted
version: 1.0
date: 2026-04-19
authors:
  - Monolift team
---

# Monolift v2 Contract Specification

## Change Log

- 1.0 (2026-04-19): Applied Option 1a revision from merged reviews: 19 edits applied, Category A contract blockers resolved, and Category B research-narrative items deferred to a future revision. Compiler-facing review: Tim Goodwin (manual audit) plus AI-merged review in `docs/specs/reviews/compiler-review.md`. Systems/research-facing review: AI-merged review in `docs/specs/reviews/systems-review.md`.
- 0.1-draft (2026-04-19): Created SPRINT-0003 specification stub.

## Evidence Index

This specification is grounded in the following evidence set:

- [Compiler generalization analysis](../evaluation/generalization-analysis-2026-04-19.md): cross-target audit of the v1 input contract and the primary source for v2 requirements.
- [Codebase state report](../codebase-state-2026-04-19.md): snapshot of the current compiler, runtime, v1 pragma surface, and known code-level limitations.
- [Evaluation index](../evaluation/README.md): target roster and dossier structure for the six real-world Go monoliths used to validate this contract.
- [Research brief](../../research/RESEARCH_BRIEF.md): prior-art map and research constraints, especially the pay-as-you-go claim, Waldo warning, actor distinction, and unresolved placement function.
- [PLOS '25 paper](../../research/monolift_PLOS.pdf): baseline Monolift conceptual model and prototype claims revised by this v2 contract.

## Conceptual-Model Baseline

PLOS '25 defines the load-bearing Monolift model that v2 inherits or revises:

- **Annotated code segments become lifts.** A lift is an annotated program segment that can execute locally or remotely. The paper names functions, classes, and interfaces as possible source constructs; the Go prototype primarily implemented function and interface cases.
- **Lift points live at call sites.** The compiler identifies calls to lifted functionality and replaces them with wrappers that decide local versus remote execution while preserving the source-level signature exposed to surrounding code.
- **Ordinary monolith execution remains transparent.** Lift annotations are comments. Without the Monolift compiler, the annotated Go program continues to build, test, and run as the original monolith.
- **Dynamic delegation is policy-directed.** Delegate expressions in annotations can choose static remote/local behavior or runtime decisions based on metrics such as CPU utilization, memory utilization, or invocation rate.
- **The lift model is bounded.** PLOS '25 explicitly rejects arbitrary distribution, migration, global heap sharing, and expensive synchronization as default semantics. It assumes the compiler refuses lifts it cannot distribute reliably.
- **Kubernetes is one backend.** The prototype emits standalone lift artifacts and Kubernetes deployments, but the concept treats an external orchestration platform as a compiler target rather than as Monolift's semantic core.

## Intended Readers

- **Compiler implementer:** read normative rules and diagnostics as the source contract the v2 compiler must enforce; only the closure report is an implementation-facing interface.
- **Monolift researcher:** read the PLOS delta, Waldo semantics, rejected alternatives, and validation verdicts as the research boundary between v2 commitments and deferred work.
- **Candidate-adopter application developer:** read annotation surfaces, pragma examples, state dispositions, and refusal diagnostics to decide whether a real Go component is liftable.

## Normative Language

This document uses the following conventions:

- **MUST**, **MUST NOT**, and **REQUIRED** define mandatory v2 source-contract behavior. A compiler that violates one of these rules is not implementing this specification.
- **SHOULD** and **SHOULD NOT** define recommended behavior. A compiler MAY diverge only when it documents the reason and preserves all MUST-level rules.
- **MAY** defines optional behavior. Optional behavior MUST NOT change the meaning of accepted pragmas or the compatibility promise.
- **Implementation-defined** means the compiler may choose the mechanism, but it MUST document the choice and surface deterministic diagnostics when the choice affects whether a lift is accepted.

Normative rules appear in sections labeled **Rule** or in tables whose cells use MUST-level language. Code blocks, target walk-throughs, and paragraphs labeled **Example**, **Rationale**, or **Non-normative note** illustrate the rules but do not add new requirements.

## Glossary

- **Lift:** an annotated unit of Go source functionality that Monolift may compile into a lifted deployable while preserving an ordinary local implementation in the monolith.
- **Lift root:** the declaration or registration entry selected by a pragma as the starting point for extraction analysis.
- **Lift point:** a compiler-inserted invocation site that chooses the local impl or remote impl according to dispatch policy.
- **Extraction closure:** the bounded transitive set of functions, methods, types, package variables, initialization effects, and state needed for a lifted deployable to implement the lift root.
- **Closure report:** the required compiler analysis artifact that lists the extraction closure, captured state, external dependencies, selected adapters, and refusal diagnostics.
- **Lifted deployable:** the remote artifact produced for a lift, such as a long-running service, singleton worker, handler process, or future backend-specific artifact.
- **Local impl:** the original in-process code path used when a lift point chooses local execution or when the program is built without Monolift.
- **Remote impl:** the lifted deployable plus generated client or handler forwarding path used when a lift point chooses remote execution.
- **Adapter:** generated boundary code that maps a Go signature or handler shape to the selected transport without changing the source declaration.
- **Dispatch policy:** the rule attached to a lift point that selects local impl, remote impl, or a runtime decision per invocation.
- **State class:** the compiler-visible category assigned to a state edge or facet of receiver fields, package globals, captured closures, goroutines, channels, caches, pools, sessions, and external stores touched by an extraction closure.
- **Refusal diagnostic:** a named compile-time error that explains why Monolift refuses a pragma or lift candidate.

## v1 to v2 Delta

Monolift v1 proved that a Go compiler pass can turn comment annotations into remote deployables while leaving the source monolith runnable. Its working contract was much narrower than the research vision: annotate an interface, find exactly one implementation, find a `New<InterfaceName>` constructor from `main`, assume the service is stateless, generate HTTP/JSON RPC for `(ctx, req) -> (resp, error)` methods, and dispatch at interface call sites.

The April 19 audit shows that contract fails on real Go monoliths. Business logic usually appears as package functions, concrete receiver methods, worker loops, or framework handlers. Wiring lives in `init` chains, options builders, registries, lifecycle hooks, app structs, and CLI delegation. Interfaces frequently represent adapters with multiple implementations, not service boundaries. Stateful workers, caches, hubs, channels, connection pools, and embedded databases are normal. Handler signatures and domain-object methods do not fit one RPC shape.

Monolift v2 keeps the core pay-as-you-go model and replaces the demo assumptions with a source contract. Annotations may target a bounded set of Go declarations. Extraction starts from the annotated root and computes a bounded call/value closure, not a `main` reconstruction. State is classified and either replicated, singleton-owned, affinity-routed, externalized, or refused. Signature handling is organized by canonical shapes with one default adapter per shape. Multi-implementer cases are explicit through `impl`, `registry`, direct concrete annotations, or lifting the dispatch point itself. A compiler that cannot prove a required contract condition MUST refuse the lift with a named diagnostic instead of panicking, guessing, or generating partial code.

## Compatibility Promise

**Rule CP-1:** A Go program annotated for Monolift v2 MUST build and run under ordinary `go build`, `go test`, and normal developer tooling without invoking Monolift.

**Rule CP-2:** Monolift annotations MUST remain Go comments. v2 MUST NOT require a new IDL, generated source checked into the monolith, language fork, framework base class, import solely for annotation parsing, or source rewrite before the uncompiled monolith can run.

**Rule CP-3:** For every accepted lift, the local impl remains the semantic baseline. Remote execution MAY add latency, serialization limits, independent failure modes, timeout behavior, and scheduling effects, but it MUST NOT require application callers to use a different source-level API.

## Annotation Surface

### Interface Declaration Pragmas

**Rule AS-IFACE-1:** A `//monolift:lift` pragma MAY annotate a named interface type declaration. The lift root is the interface type, and the exposed surface is the set of interface methods after embedded-interface expansion.

**Rule AS-IFACE-2:** Interface declaration pragmas MUST NOT require a unique implementation as a prerequisite. If production implementation resolution is not provably unique, the pragma MUST provide `impl=<ConcreteName>`, `registry=<key>`, or `dispatch=lift-point`; otherwise the compiler MUST refuse with `MLV2_IMPL_AMBIGUOUS`.

**Rule AS-IFACE-3:** Interface declaration pragmas are appropriate when callers already depend on the interface or when lifting the interface dispatch point is intentional. They SHOULD NOT be used to force an artificial service boundary into code that naturally exposes a concrete function, method, or handler.

Example:

```go
//monolift:lift name=mailer mode=dynamic impl=SMTPSender state=external transport=http-json policy="trigger=CPU threshold=0.70"
type Sender interface {
    Send(ctx context.Context, msg Message) error
}
```

### Package-Level Function Pragmas

**Rule AS-FUNC-1:** A `//monolift:lift` pragma MAY annotate a named package-level function declaration. The lift root is the function symbol, and the exposed surface is exactly that function's signature.

**Rule AS-FUNC-2:** A function lift MUST classify into one canonical signature shape in [Transport and Adapter Contract](#transport-and-adapter-contract). If no shape applies, the compiler MUST refuse with `MLV2_SHAPE_UNSUPPORTED`.

**Rule AS-FUNC-3:** Function lifts MAY capture package-level variables and initialization effects only through the extraction closure and state-class rules. The compiler MUST NOT reconstruct a function's dependencies by searching for a constructor convention.

**Rule AS-FUNC-4:** Generic function declarations are refused in v2 unless every instantiation reachable under the selected build configuration is enumerable and the closure report records each type-substituted symbol identity. If the reachable instantiation set is not enumerable, the compiler MUST refuse with `MLV2_SURFACE_DEFERRED_GENERIC_DECL`.

Examples include Miniflux feed processing functions and Listmonk campaign worker functions whose boundaries are real package functions rather than service interfaces.

```go
//monolift:lift name=feed-processor mode=dynamic state=external transport=http-json policy="trigger=CPU threshold=0.75"
func ProcessFeedEntries(ctx context.Context, feedID int64, force bool) error {
    // existing implementation
}
```

### Concrete Receiver Method Pragmas

**Rule AS-METHOD-1:** A `//monolift:lift` pragma MAY annotate a method declaration whose receiver type is a named concrete type or pointer to a named concrete type. The lift root is the method symbol, and the exposed surface is exactly that method's receiver plus parameter and result signature.

**Rule AS-METHOD-2:** Receiver fields reachable from the method body are captured state and MUST be classified under [State Semantics](#state-semantics). If receiver construction depends on options builders, lifecycle hooks, or registries, those wiring effects are inputs to extraction closure, not reasons to refuse by themselves.

**Rule AS-METHOD-3:** A concrete receiver method pragma SHOULD be preferred over an interface pragma when the target application's stable boundary is a concrete service object, as in `UserService.CreateUser`, or a concrete mailer/notification service method.

```go
type UserService struct {
    store Store
    hub   *WebHub
}

//monolift:lift name=user-create mode=remote state=external transport=http-json
func (s *UserService) CreateUser(rctx request.CTX, user *model.User, opts UserCreateOptions) (*model.User, error) {
    // existing implementation
}
```

### Struct Type Pragmas

**Rule AS-STRUCT-1:** A `//monolift:lift` pragma MAY annotate a named struct type declaration. The lift root is the struct type, and the exposed surface is the set of exported methods declared on the struct or pointer-to-struct receiver after applying any `methods=` filter in the pragma.

**Rule AS-STRUCT-2:** If no `methods=` filter is provided, every exported method on the receiver surface MUST independently classify into an accepted canonical shape and state disposition. One unsupported method MUST refuse the struct lift with `MLV2_STRUCT_SURFACE_UNSUPPORTED`, unless the pragma explicitly excludes it.

**Rule AS-STRUCT-3:** Struct type pragmas MAY be registry-keyed. When a framework registers concrete module structs by ID, `registry=<id>` identifies the production entry and disambiguates the lifted deployable.

```go
//monolift:lift name=cert-issuer mode=remote state=singleton transport=handler registry="tls.issuance.acme" methods=Provision,ServeHTTP
type ACMEIssuer struct {
    cache Cache
}
```

### Deferred and Refused Annotation Forms

| Form | v2 disposition | Rationale | Diagnostic when rejected |
|---|---|---|---|
| Interface method declaration | **Refused** | A method line inside an interface has no implementation body, receiver state, or independent production identity. Annotate the interface with `methods=` or annotate the concrete implementer method. | `MLV2_SURFACE_INTERFACE_METHOD` |
| Function value in `var` declaration | **Deferred** | Function-valued vars require value-flow identity, reassignment analysis, and closure capture semantics beyond v2's declaration-root contract. Annotate the named function assigned to the var when available. | `MLV2_SURFACE_DEFERRED_FUNCTION_VALUE` |
| Anonymous function literal | **Refused** | Anonymous funcs do not provide a stable declaration name, reusable lift root, or clear external deployable identity. | `MLV2_SURFACE_ANON_FUNC` |
| Generic named declaration | **Deferred unless enumerable** | Generic declarations need a finite selected-build instantiation set before adapter shapes and symbol identities are stable. | `MLV2_SURFACE_DEFERRED_GENERIC_DECL` |
| Generic instantiation expression | **Deferred** | v2 MAY lift a generic named function declaration after type substitution is modeled in the closure report, but a single instantiation expression is not a declaration root. | `MLV2_SURFACE_DEFERRED_GENERIC_INSTANTIATION` |
| Whole package | **Refused** | Package boundaries do not equal service boundaries in the audit; package pragmas are too coarse and risk absorbing the monolith. Annotate a declaration inside the package. | `MLV2_SURFACE_WHOLE_PACKAGE` |

### Annotation Surface Examples

Accepted interface declaration:

```go
//monolift:lift name=sender mode=dynamic impl=SMTPSender
type Sender interface { Send(context.Context, Message) error }
```

Accepted package-level function:

```go
//monolift:lift name=render-campaign mode=remote state=external
func RenderCampaign(ctx context.Context, id int64) error { return nil }
```

Accepted concrete receiver method:

```go
//monolift:lift name=user-create mode=remote state=external
func (s *UserService) CreateUser(ctx context.Context, user *User) (*User, error) { return user, nil }
```

Accepted struct type:

```go
//monolift:lift name=caddy-module mode=remote state=singleton registry="http.handlers.reverse_proxy" methods=ServeHTTP
type ReverseProxy struct{}
```

Refused interface method:

```go
type Sender interface {
    //monolift:lift name=send
    Send(context.Context, Message) error // diagnostic: MLV2_SURFACE_INTERFACE_METHOD
}
```

Deferred function-valued var:

```go
//monolift:lift name=worker
var worker = func(ctx context.Context) error { return nil } // diagnostic: MLV2_SURFACE_DEFERRED_FUNCTION_VALUE
```

Refused anonymous function:

```go
go func() {
    //monolift:lift name=anon
    runWork() // diagnostic: MLV2_SURFACE_ANON_FUNC
}()
```

Deferred generic instantiation expression:

```go
//monolift:lift name=int-parser
var parseInt = Parse[int] // diagnostic: MLV2_SURFACE_DEFERRED_GENERIC_INSTANTIATION
```

Refused whole-package annotation:

```go
//monolift:lift name=models
package models // diagnostic: MLV2_SURFACE_WHOLE_PACKAGE
```

### Annotation Surface Cross-Check

| Target | Plausible lift site | Surface rule | Surface verdict |
|---|---|---|---|
| Miniflux | `ProcessFeedEntries` or equivalent feed fetch/process function | `AS-FUNC-*` | Accept: package-level function root fits without requiring a service interface. |
| Listmonk | campaign `worker` and render/template flow | `AS-FUNC-*`; handler routes use accepted handler shapes later | Accept: named worker function is a declaration root; channel state is deferred to state semantics, not a surface mismatch. |
| Mattermost | `(*UserService).CreateUser` | `AS-METHOD-*` | Accept: concrete receiver method captures real service boundary and avoids generated mock ambiguity. |
| Caddy | concrete module struct registered by module ID | `AS-STRUCT-*` with `registry=` | Accept: struct root plus registry key matches plugin architecture. |
| Gitea | mailer/notification sender or service method | `AS-METHOD-*` or `AS-IFACE-*` with `impl=` / dispatch point | Accept: concrete annotation is preferred; interface sender remains possible with explicit implementation. |
| Pocketbase | `core.App` | `AS-STRUCT-*` initially finds a surface but will refuse later on state and closure criteria | Surface-only partial: struct surface exists, but 190+ method god-object is expected to fail closure/state rules. |

### Provisional Pragma Surface Syntax

Until [Pragma Syntax v2](#pragma-syntax-v2) finalizes EBNF, later sections use this key sketch:

```go
//monolift:lift name=<id> mode=<local|remote|dynamic> state=<stateless|singleton|affinity|external> transport=<http-json|handler|grpc> impl=<ConcreteName> registry=<key> methods=<A,B> policy="<delegate expression>"
```

Required keys are surface-dependent. `name` is always REQUIRED. `impl` is REQUIRED only when an interface annotation has multiple production implementations and does not lift the dispatch point. `registry` is REQUIRED when a registry/plugin system supplies production identity. `methods` is allowed only for interface and struct surfaces. Missing `mode`, `state`, or `transport` MAY be inferred by later rules when inference is deterministic.

## Extraction Root and Closure

### Extraction Root by Surface

**Rule EC-ROOT-1:** For an interface declaration pragma, the extraction root is the interface type plus the selected implementation set: the concrete `impl`, the registry-selected implementation, or the dispatch point when `dispatch=lift-point`.

**Rule EC-ROOT-2:** For a package-level function pragma, the extraction root is the named function symbol after build tags, GOOS/GOARCH, and module resolution select the compiled package instance.

**Rule EC-ROOT-3:** For a concrete receiver method pragma, the extraction root is the method symbol and its receiver type. Receiver construction values are inputs to the closure report.

**Rule EC-ROOT-4:** For a struct type pragma, the extraction root is the struct type plus the selected exposed method set. If `registry=` is present, the registry entry is part of the root identity.

**Rule EC-ROOT-5:** A lift root MUST be stable under ordinary Go name resolution. If the root cannot be named deterministically in the compiled package graph, the compiler MUST refuse with `MLV2_ROOT_UNSTABLE`.

### Closure Computation

**Rule EC-CLOSURE-1:** The compiler MUST compute the extraction closure as a transitive call-graph and value-graph closure from the extraction root. It MUST NOT depend on finding constructor calls, variable declarations, or assignment order in `main`.

**Rule EC-CLOSURE-2:** Closure computation MUST operate on the compiled Go program after module loading, type checking, build constraint selection, and initialization ordering. `golang.org/x/tools/go/ssa` is one valid non-normative analysis substrate; this specification does not require a particular internal representation.

**Rule EC-CLOSURE-3:** When call-graph precision is insufficient to prove a bounded closure, the compiler MUST either require disambiguating pragma keys or refuse with a named diagnostic. It MUST NOT silently include the whole program as a fallback.

**Rule EC-CLOSURE-4:** Closure computation MUST use a conservative call/value-graph analysis with precision-triggered refusals. The specification does not mandate CHA, RTA, VTA, pointer analysis, or another algorithm, but the selected algorithm, precision limits, and refusal triggers MUST be disclosed in the closure report. Two runs over the same package graph, build configuration, and disclosed algorithm MUST produce identical accept/refuse verdicts.

### Closure Includes

**Rule EC-INCLUDE-1:** The extraction closure MUST include all application functions and methods reachable from the extraction root through direct calls, resolved interface calls, selected function values, and registered callbacks that execute as part of the root behavior.

**Rule EC-INCLUDE-2:** The extraction closure MUST include reachable package-level variables that the root reads or writes, receiver fields accessed by included methods, captured closure variables, and initialization effects needed to construct those values.

**Rule EC-INCLUDE-3:** The extraction closure MUST include reachable named types, type aliases, constants, interface definitions, struct field types, method sets, and adapter-visible parameter/result types needed to compile the lifted deployable.

**Rule EC-INCLUDE-4:** External services such as databases, object stores, queues, Dapr components, SMTP servers, and Kubernetes APIs are not copied into the lifted deployable. They MUST appear in the closure report as external dependencies with their access path and configuration source.

### Closure Excludes and Termination

**Rule EC-TERM-1:** The closure terminates at Go standard-library package boundaries. Standard-library symbols are imported by the lifted deployable, not copied.

**Rule EC-TERM-2:** Source inclusion terminates at external module boundaries by default. External-module symbols are imported as module dependencies unless the pragma explicitly opts into vendored source inclusion and the compiler can preserve licenses and build constraints. State/effect analysis does not terminate at the module boundary: imported-module summaries MUST still be considered when the imported API exposes process-local mutable state, init effects, background goroutines, cgo, file handles, or serialization-visible types. If the required state/effect summary is unavailable or ambiguous, the compiler MUST refuse with `MLV2_EXTERNAL_STATE_UNRESOLVED`.

**Rule EC-TERM-3:** Calls through `cgo` are allowed only as external dependencies with stable runtime availability. If a cgo dependency requires process-local state that cannot be reproduced in the lifted deployable, the compiler MUST refuse with `MLV2_CGO_UNLIFTABLE`.

**Rule EC-TERM-4:** Reflection-driven dispatch is refused unless the compiler can resolve the concrete target set statically or through an explicit registry key. Unresolved reflection MUST refuse with `MLV2_REFLECTION_DISPATCH`.

**Rule EC-TERM-5:** Build-tag-gated code is analyzed only for the selected build configuration. Alternative build-tag implementations MUST NOT create ambiguity for the selected lift.

**Rule EC-TERM-6:** Dynamic plugin loading is refused unless the plugin identity is represented by `registry=` and the loaded implementation is present in the compiled package graph. Unresolved plugins MUST refuse with `MLV2_DYNAMIC_PLUGIN`.

**Rule EC-TERM-7:** Any non-dispatch closure edge that makes the reachable frontier non-finite MUST refuse with `MLV2_CLOSURE_UNBOUNDED`. This includes `unsafe.Pointer`-mediated crossings and opaque function-value escapes that cannot be reduced to a finite target set. This rule is distinct from the dispatch-specific refusals `MLV2_REFLECTION_DISPATCH`, `MLV2_DYNAMIC_PLUGIN`, and `MLV2_DISPATCH_SET_UNBOUNDED`; see [ADR-0014](../decisions/0014-unbounded-edge-refusal-code.md).

**Rule EC-TERM-8:** Generated code included in the selected build is treated as ordinary code for closure purposes. Generated mocks, fakes, and test-only artifacts MUST be ignored during production implementation resolution unless the pragma explicitly names them under a test build.

### Wiring Idioms

**Rule EC-WIRE-1:** The source location of wiring MUST NOT determine liftability. `init()` chains, options builders, registry `Register(...)` calls, dependency-injection helpers, lifecycle hooks, and app-struct literals are all inputs to the same program-initialization value graph.

**Rule EC-WIRE-2:** The compiler MUST report the initialization path that supplies each captured receiver field, package variable, registry entry, and external dependency in the closure report.

**Rule EC-WIRE-3:** Registry calls are accepted when registration IDs and concrete values are statically visible after build selection. Lifecycle hooks are accepted when hook registration is visible and hook execution order is deterministic for the selected root.

**Rule EC-WIRE-4:** Supported wiring patterns are limited to top-level `var` initializers whose expressions are constants, composite literals, or calls whose callees do not write package globals; `init()` functions containing only package-global writes and registry calls; blank imports whose selected package `init()` functions match those forms; and framework registries described by adapter metadata. Any other initialization order, late mutation, callback registration, dependency-injection helper, lifecycle hook, or app-struct construction that cannot be reduced to those patterns MUST refuse with `MLV2_WIRING_UNRESOLVED`.

### Closure Report

**Rule EC-REPORT-1:** For every accepted or refused pragma, the compiler MUST be able to produce a closure report. This report is the named interface between this specification and SPRINT-0005 implementation work.

**Rule EC-REPORT-2:** The closure report MUST be JSON conforming to [Appendix A: Closure Report JSON Schema](#appendix-a-closure-report-json-schema). All arrays of symbols, diagnostics, adapters, external dependencies, and state facets MUST be emitted in stable lexicographic order by symbol identity and then rule or diagnostic name.

### Boundary Pruning and Oversized Closures

**Rule EC-PRUNE-1:** The compiler MUST apply boundary pruning before accepting a lift. Pruning MUST stop inclusion at stdlib and external-module boundaries, external service clients, serialization boundaries, framework handler interfaces, and registry entries outside the selected root identity.

**Rule EC-PRUNE-2:** A closure is bounded iff the reachable external-edge frontier is finite under `EC-TERM-*` and no refusal condition in [Extraction Root and Closure](#extraction-root-and-closure), [State Semantics](#state-semantics), or [Transport and Adapter Contract](#transport-and-adapter-contract) applies.

**Rule EC-PRUNE-3:** When pruning cannot produce a bounded deployable for the annotated root, the compiler MUST refuse with `MLV2_CLOSURE_TOO_LARGE`. The diagnostic MUST name the symbol or state edge that caused unbounded growth and SHOULD suggest a narrower function, method, method filter, `impl`, or `registry` annotation.

Non-normative implementer guidance: common oversized-closure smells include inclusion of an unrelated application subsystem, an entrypoint package, multiple independent registries, a persistence layer plus routing layer with no bounded adapter, or a state object whose public method surface is broader than the selected lift behavior.

Recommended default calibration, non-normative: an implementation should consider warning before refusal when a candidate closure reaches 10 external-module packages, 20 exposed methods on a root surface, or more than one independent registry namespace. These numbers are indicative starting points for SPRINT-0005 corpus calibration, not acceptance criteria.

## State Semantics

### State Taxonomy

| State class | Definition | Examples |
|---|---|---|
| `stateless` | No mutable state survives across invocations except stack-local values and immutable constants. | Pure helpers, deterministic render functions. |
| `immutable-captured-config` | Values captured at initialization and read thereafter without mutation. | Parsed config structs, template sets loaded once. |
| `externalized-durable` | Durable state already owned by an external system with stable client semantics. | SQL DB, KV store, Dapr state store, SMTP service. |
| `process-local-cache` | Mutable local cache whose contents may be recomputed or lost without correctness loss. | Template cache, link cache, in-memory lookup cache. |
| `singleton-mutable` | Mutable process state that has one logical owner. | Goroutine+channel worker, worker pool, subscription hub. |
| `shared-mutable-across-callers` | Mutable state concurrently shared by unrelated callers with correctness depending on synchronization across owners. | Global maps with cross-request invariants, shared counters requiring consensus. |
| `connection-session` | State tied to a specific client connection, protocol session, websocket, POP3/IMAP session, or similar affinity boundary. | WebSocket hub membership, POP3 sessions, long-lived streaming connection. |

### State Disposition

| State class | v2 disposition | Normative rule |
|---|---|---|
| `stateless` | replicated | May run as any number of remote replicas. |
| `immutable-captured-config` | replicated | Config is copied into each lifted deployable at startup or supplied through equivalent config injection. |
| `externalized-durable` | replicated | Remote replicas may share the external dependency; connection credentials and config appear in the closure report. |
| `process-local-cache` | replicated or singleton | Replication is allowed only when cache loss/divergence does not affect correctness; otherwise singleton is required. |
| `singleton-mutable` | singleton | Exactly one logical remote owner is required unless the developer externalizes the state. |
| `shared-mutable-across-callers` | externalize-required or refused | Accepted only if rewritten to an external durable/concurrency service; otherwise refused. |
| `connection-session` | affinity-routed or refused | Accepted only when the lift point can preserve stable session affinity; otherwise refused. |

**Rule SS-DISP-1:** Every captured state edge or facet MUST have one or more state classes in the closure report. Its disposition is the most restrictive class relevant to correctness, and the report MUST show composite state when one value has multiple correctness-relevant facets.

**Rule SS-DISP-2:** Any state requiring consensus, distributed locking, hidden heap sharing, or cross-node mutation coherence without an explicit external system MUST be refused with `MLV2_SHARED_MUTABLE_STATE`.

### Lifted State Meaning

**Rule SS-LIFT-1:** Receiver fields read or written by the extraction closure are state owned by the lifted deployable according to their state class. The local impl keeps its ordinary receiver fields for local execution.

**Rule SS-LIFT-2:** Package globals read or written by the extraction closure are captured state. Immutable globals may be copied; mutable globals require singleton, affinity, externalization, or refusal.

**Rule SS-LIFT-3:** Closure variables captured by accepted named functions are treated like receiver fields. Captures of anonymous function literals are refused at the annotation surface.

**Rule SS-LIFT-4:** Long-lived goroutines and channels inside the closure are part of the lifted deployable only under `singleton-mutable` disposition. Channels MUST NOT be serialized across a remote call boundary.

**Rule SS-LIFT-5:** Caches may be replicated only when the compiler can prove or the developer declares that cache divergence is not correctness-observable. Otherwise they require singleton or refusal.

**Rule SS-LIFT-6:** Connection pools are not copied as live connections. The lifted deployable MAY create its own pool from captured configuration. Session-bound connections require affinity or refusal. If connection or session state crosses invocations and no stable affinity key exists at the lift point, the compiler MUST refuse with `MLV2_SESSION_AFFINITY_UNAVAILABLE`.

### State Inference and Declarations

**Rule SS-CLASS-1:** The compiler MUST infer state classes for every captured state edge or facet when possible. Inference evidence includes mutability, synchronization, escape paths, channel/goroutine use, external client types, connection/session APIs, and writes to package globals or receiver fields.

**Rule SS-CLASS-2:** The pragma MAY declare `state=stateless|singleton|affinity|external`. A declaration selects the intended disposition, not a waiver of analysis.

**Rule SS-CLASS-3:** A developer declaration MAY narrow an inferred safe disposition, such as forcing a replicable cache to singleton. A declaration MUST NOT widen an unsafe disposition. For example, `state=stateless` on code that writes a package global MUST refuse with `MLV2_STATE_DECL_CONFLICT`.

**Rule SS-CLASS-4:** When inference is ambiguous but a developer declaration plus closure evidence is sufficient to choose a safe disposition, the compiler MAY accept and mark the item as developer-declared in the closure report. When ambiguity remains correctness-relevant, it MUST refuse with `MLV2_STATE_UNKNOWN`.

### Failure, Cancellation, Deadline, Panic, and Zero-Value Semantics

**Rule SS-WALDO-1:** A remote lift invocation is not semantically identical to a local call. The generated adapter MUST make network failure, serialization failure, remote process failure, and timeout observable as ordinary Go errors when the source signature can carry an error.

**Rule SS-WALDO-2:** If the source signature cannot carry an error and the selected transport can fail independently, the compiler MUST refuse with `MLV2_NO_ERROR_CHANNEL` unless the transport shape has framework-defined failure semantics, such as HTTP handler status responses.

**Rule SS-WALDO-3:** `context.Context` cancellation and deadlines MUST be propagated across supported transports when a context parameter is present or framework handler context exposes one. Context values are not automatically propagated unless serialization rules explicitly accept them.

**Rule SS-WALDO-4:** Panics in a remote impl MUST be converted to transport failure and returned through the shape's error channel. The remote stack trace MAY be logged or surfaced in diagnostics, but the compiler MUST NOT rely on panic equivalence with local stack unwinding.

**Rule SS-WALDO-5:** For signatures with non-error return values, failure adapters MUST return the Go zero value for non-error results only alongside a non-nil error. The v1 `clientgen.go` panic-on-unhandled-zero-value case is replaced by compile-time refusal `MLV2_SHAPE_UNSUPPORTED`.

**Rule SS-WALDO-6:** At-least-once, at-most-once, retry, and idempotency semantics are transport policy extensions. v2 makes no hidden retry guarantee.

### Remote Call Outcomes

**Rule SS-OUTCOME-1:** Every remote invocation outcome MUST classify as one of: `success`, `local-serialization-failure`, `pre-exec-transport-failure`, `remote-maybe-executed`, `completed-but-reply-lost`, `timeout-or-cancellation`, or `remote-panic`.

**Rule SS-OUTCOME-2:** For `remote-maybe-executed`, `completed-but-reply-lost`, `timeout-or-cancellation`, and `remote-panic` outcomes, remote side effects MAY have occurred unless adapter metadata declares stronger semantics such as deduplication, transactional rollback, or exactly-once completion.

**Rule SS-OUTCOME-3:** After a failed remote attempt, generated code MUST NOT automatically fall back to the local impl unless the operation is declared idempotent or adapter metadata supplies deduplication for the operation. Unsafe fallback MUST refuse with `MLV2_REMOTE_FALLBACK_UNSAFE`.

### State Refusals

Monolift v2 MUST refuse lifts with any of the following criteria:

| Diagnostic | Criteria | Typical remediation |
|---|---|---|
| `MLV2_SHARED_MUTABLE_STATE` | Correctness depends on mutable state shared across unrelated callers without an external concurrency authority. | Move state to DB/KV/queue or narrow the root. |
| `MLV2_EMBEDDED_DB_APP_ROOT` | The root owns an embedded database handle plus broad lifecycle hooks and application-wide method surface. | Lift a narrower function or externalize database ownership. |
| `MLV2_SESSION_AFFINITY_UNAVAILABLE` | Connection/session state crosses invocations but no stable affinity key exists at the lift point. | Add an affinity key or keep local. |
| `MLV2_CHANNEL_BOUNDARY` | A channel, goroutine scheduler, or worker queue must be serialized across the remote boundary. | Use singleton worker lift or external queue. |
| `MLV2_STATE_DECL_CONFLICT` | Developer-declared state disposition contradicts compiler evidence. | Change `state=` or refactor state. |

PocketBase's `core.App` is the canonical negative example for `MLV2_EMBEDDED_DB_APP_ROOT`: its broad app object, embedded SQLite ownership, lifecycle hooks, and 190+ method surface make the whole object an application runtime rather than a bounded lift.
This refusal is a composite state refusal: it records the embedded durable-state ownership constraint from `SS-LIFT-6` together with the shared-mutation refusal from `SS-DISP-2`.

### State Cross-Check

| Target/state example | State class | Disposition | Contract result |
|---|---|---|---|
| Mattermost WebHub | `connection-session` plus `singleton-mutable` | affinity-routed or singleton | Deferred unless stable session affinity is available; otherwise `MLV2_SESSION_AFFINITY_UNAVAILABLE`. |
| Listmonk campaign-worker queue | `singleton-mutable` | singleton | Accepted as singleton worker; channels stay inside deployable. |
| Gitea mailer context/cache | `externalized-durable` plus `process-local-cache` | replicated or singleton | Accepted if cache divergence is non-correctness-observable and sender config is external. |
| Caddy cert/issuer state | `singleton-mutable` plus `externalized-durable` | singleton | Accepted as registry-keyed singleton issuer. |
| Miniflux feed-worker concurrency | `singleton-mutable` or `externalized-durable` depending queue ownership | singleton | Accepted when worker concurrency stays inside lift or external queue owns scheduling. |
| PocketBase embedded SQLite app | `shared-mutable-across-callers` plus embedded DB app root | refused | Refused with `MLV2_EMBEDDED_DB_APP_ROOT` and possibly `MLV2_CLOSURE_TOO_LARGE`. |

## Transport and Adapter Contract

### Adapter Metadata

**Rule TA-ADAPTER-1:** Adapter metadata is a first-class contract input. A compiler MAY accept framework, serialization, context-value, cgo, reflection, registry, or generic-substitution behavior only when built-in rules or adapter metadata make the behavior deterministic for the selected build.

**Rule TA-ADAPTER-2:** Adapter metadata MUST declare adapter kind, matched Go types or functions, accepted canonical shapes, state effects, transport effects, serialization effects, and closure-report fields. Malformed or incomplete metadata that affects acceptance MUST refuse with `MLV2_ADAPTER_METADATA_INVALID`.

| Adapter kind | Matches | Accepted canonical shapes | State/effect declaration | Required report fields |
|---|---|---|---|---|
| `handler` | Framework handler types/functions such as `http.Handler`, Echo handlers, Caddy middleware | `http-handler` | Request/response ownership, context propagation, unsupported handler capabilities | adapter id, matched symbol, framework, propagated context fields |
| `registry` | Registry `Register` functions, plugin factory tables, module IDs | Any shape selected by registered value | Registry namespace, key extraction, registration side effects | namespace, key, registered factory/value symbol |
| `serialization` | Named types, interface dynamic type sets, custom encoders | Domain and no-response shapes | Encoding format, identity/alias behavior, error envelope support | encoded type, encoder symbol, unsupported fields |
| `context-value` | Context key/value pairs read inside closure | Handler or domain shapes with context | Key identity, value serializer, drop policy | key symbol, value type, propagated or dropped |
| `cgo` | cgo calls and external native handles | Any shape whose state rules pass | Native dependency availability and process-local state requirements | library identity, required runtime config |
| `reflection` | Reflection-based registration or dispatch allowlists | Registry, handler, or domain shapes | Resolved target set and data-vs-dispatch use | resolved targets, unresolved operations |
| `generic-substitution` | Type-substituted generic declarations | Any shape after substitution | Finite instantiation set and substituted type identities | generic symbol, type arguments, instantiated symbol |

**Rule TA-ADAPTER-3:** Adapter metadata MUST NOT waive a refusal rule. It may only provide the bounded target sets, state effects, serialization semantics, or transport semantics required to evaluate the same rules.

### Transport Taxonomy

| Transport | v2 status | Use |
|---|---|---|
| `http-json` | First-class default | Domain-function and domain-method shapes whose parameters/results are JSON-serializable or adapter-serializable. |
| `handler` | First-class default for HTTP-shaped roots | Shape-preserving forwarding for `net/http`, Echo, Caddy middleware, and equivalent framework handler signatures. |
| `grpc` | Reserved extension in v2 | Allowed syntax for future typed RPC, but not required for v2 compiler acceptance. |
| `in-proc` | Reserved future | Local optimization backend; does not define remote semantics. |
| `serverless` | Reserved future | Function-lift deployment target with cold-start and lifecycle semantics left to future specs. |
| `shared-memory` | Reserved future | Node-local optimization only; MUST NOT be assumed by source contracts. |

**Rule TA-TRANSPORT-1:** Every accepted exposed operation MUST map to exactly one default transport unless the pragma selects another first-class transport that is valid for the canonical shape.

### Canonical Shapes

**Rule TA-SHAPE-1:** The compiler MUST classify every exposed operation into one canonical shape before adapter generation. Shape classification order is: HTTP handler, channel consumer, builder chain, `(ctx, req) -> (resp, err)`, domain-argument method/function, no-response method/function, unsupported.

| Shape | Matches | Default transport | Result |
|---|---|---|---|
| `ctx-request-response` | `func(context.Context, Req) (Resp, error)` and method equivalents | `http-json` | Accepted when `Req` and `Resp` serialize. |
| `multi-domain-args` | Context plus multiple domain args and `(..., error)` or `error` result | `http-json` | Accepted through generated request envelope. |
| `no-response` | Returns only `error` or no results with framework-defined failure path | `http-json` when error exists; refused otherwise | No-error remote failures require `MLV2_NO_ERROR_CHANNEL`. |
| `http-handler` | `http.Handler`, `http.HandlerFunc`, `func(http.ResponseWriter,*http.Request)`, Echo handler, Caddy middleware | `handler` | Accepted shape-preserving. |
| `channel-consumer` | Long-running loop consuming a channel or queue inside the closure | singleton worker over `http-json` control plane or external queue | Accepted only when channels do not cross boundary. |
| `builder-chain` | Fluent options/config builder returning receiver/config for later use | none | Refused as lift root with `MLV2_BUILDER_CHAIN_ROOT`; may be included in closure wiring. |
| `unsupported` | Variadic unserializable args, function args without registered adapter, unsafe pointers, channels across boundary | none | Refused with `MLV2_SHAPE_UNSUPPORTED` or more specific diagnostic. |

### Shape-Preserving Handler Transport

**Rule TA-HANDLER-1:** When the lift root is classified as `http-handler`, the lifted deployable MUST preserve the framework's request/response shape. The adapter forwards the incoming HTTP request to the remote handler endpoint and returns the remote handler's status, headers, and body through the original framework path.

**Rule TA-HANDLER-2:** Handler lifts MUST NOT decode the request into an intermediate Monolift JSON-RPC envelope and then re-encode it as HTTP unless the application handler itself already does so.

**Rule TA-HANDLER-3:** Framework context objects such as Echo context or Caddy middleware state are adapter-managed values. They MUST be represented by explicit handler adapters, not by generic JSON serialization.

This rule exists for Listmonk Echo handlers and Caddy middleware, where the application boundary is already HTTP-shaped.

### Serialization Rules

**Rule TA-SER-1:** Parameters and return values for `http-json` MUST be serializable through the compiler's accepted encoding set: Go primitives, strings, byte slices, structs, slices, maps with string-compatible keys, pointers to serializable acyclic object graphs, and named types whose exported representation is serializable.

**Rule TA-SER-2:** Errors MUST cross the boundary as structured error envelopes containing at least a message, remote type name when available, and retryability/transport category when known. Exact concrete error identity is not preserved unless a registered adapter states otherwise.

**Rule TA-SER-3:** Pointer graphs MUST be acyclic and ownership-transferred for the duration of the call. v2 refuses mutable pointer arguments or results that require caller-visible alias preservation across the boundary unless adapter metadata defines copy-in/copy-out with alias restoration for that exact type graph. Without that metadata, the compiler MUST refuse with `MLV2_POINTER_ALIAS_UNSUPPORTED`. Cross-invocation pointer identity is not preserved.

**Rule TA-SER-4:** Interface-typed parameters or results are accepted only when the concrete dynamic type set is statically bounded or declared through adapter metadata. Otherwise the compiler MUST refuse with `MLV2_INTERFACE_SERIALIZATION`.

**Rule TA-SER-5:** Generic named declarations are accepted only when `AS-FUNC-4` has produced an enumerable selected-build instantiation set and generic-substitution adapter metadata records the type-substituted symbols. Generic instantiation expressions remain deferred annotation surfaces.

**Rule TA-SER-6:** `context.Context` deadlines and cancellation are propagated. Context values are propagated only if keys and values are explicitly adapter-serializable; otherwise they are dropped and MUST be reported in the closure report.

**Rule TA-SER-7:** Channels MUST NOT be serialized as parameters, returns, receiver fields crossing the boundary, or context values. Channel consumers are accepted only when the channel remains wholly inside a singleton lifted deployable or is replaced by an external queue.

### gRPC Decision

**Rule TA-GRPC-1:** gRPC/protobuf is a reserved v2 transport extension, not a first-class required transport. A v2 compiler MAY reject `transport=grpc` with `MLV2_TRANSPORT_RESERVED` while still conforming to this specification.

Rationale: the audit failure is not a lack of protobuf performance; it is annotation surface, closure, state, heterogeneous shapes, and multi-implementer handling. Mandatory gRPC would add IDL/schema and adoption work before those contract issues are solved. The syntax reserves `grpc` so a later compiler can add typed RPC without changing source annotations.

### Context Propagation

**Rule TA-CTX-1:** When an accepted shape includes `context.Context` or framework-equivalent request context, the adapter MUST propagate cancellation and deadline metadata to the remote impl.

**Rule TA-CTX-2:** The remote impl MUST observe cancellation no later than the selected transport can detect client disconnect, deadline expiry, or explicit cancellation. v2 does not guarantee preemption of CPU-bound code that ignores context.

**Rule TA-CTX-3:** Context values are not part of the default compatibility promise. Dropped values MUST be reported in the closure report when they are read inside the extraction closure.

### Unresolvable Signature Rule

**Rule TA-REFUSE-1:** If the compiler cannot classify a signature into a supported canonical shape, cannot construct zero values for non-error results on the failure path, or cannot serialize any adapter-visible type, it MUST refuse at compile time with a named diagnostic. It MUST NOT generate code that can panic during client generation or at first remote invocation.

The v1 `clientgen.go:110` panic-on-unhandled-return behavior is replaced by `MLV2_SHAPE_UNSUPPORTED`, `MLV2_NO_ERROR_CHANNEL`, `MLV2_SERIALIZATION_UNSUPPORTED`, or a more specific diagnostic.

## Dispatch Granularity and Placement Policy

### Lift Point

**Rule DP-POINT-1:** A lift point is the compiler-inserted invocation site that selects local impl, remote impl, or runtime-decided impl for one exposed lift operation.

**Rule DP-POINT-2:** Lift points MUST preserve the source-level call shape visible to surrounding application code. Any adapter, client, monitor, or policy machinery is internal generated code.

**Rule DP-POINT-3:** Each lift point MUST reference exactly one dispatch policy after composition. If policy composition is ambiguous, the compiler MUST refuse with `MLV2_POLICY_CONFLICT`.

### Dispatch Granularity Matrix

| Annotation surface | Default dispatch granularity | Override |
|---|---|---|
| Interface declaration | Per interface method at each call through the annotated interface value | `methods=` can select subset; `dispatch=lift-point` lifts interface-switch itself when multiple impls are intended. |
| Package-level function | Per call site to the named function | No broader default; each function pragma creates lift points for calls to that function. |
| Concrete receiver method | Per method call site | Receiver-level policy may be inherited from a struct pragma when present. |
| Struct type declaration | Per exposed method of the struct surface | `methods=` narrows the exposed surface; method pragmas override outer policy. |

**Rule DP-GRAN-1:** v2 dispatch granularity is never implicitly "whole package" or "whole application." It is derived only from accepted annotation surfaces.

### Policy Composition

**Rule DP-COMP-1:** Outer policies from interface or struct pragmas apply to every exposed method unless a method-specific pragma or `method:<Name>` keyed option overrides them.

**Rule DP-COMP-2:** Inner method policy wins over outer policy when both are valid and compatible with the method's state class and transport shape.

**Rule DP-COMP-3:** A policy is invalid when it conflicts with state disposition. For example, `mode=remote` with inferred `connection-session` state and no affinity key MUST refuse with `MLV2_POLICY_STATE_CONFLICT`.

**Rule DP-COMP-4:** Two policies conflict when they assign incompatible modes to the same lift point without an explicit override. The compiler MUST refuse with `MLV2_POLICY_CONFLICT` and report both source spans.

### Policy Modes

| Mode | Meaning | Required behavior |
|---|---|---|
| `local` | Never call remote impl. | Compiler may still analyze/report the lift, but generated lift points always use local impl. |
| `remote` | Always call remote impl. | Requires accepted transport, adapter, and state disposition. |
| `dynamic` | Decide at runtime per invocation. | Requires a valid policy expression and both local and remote impls available. |

**Rule DP-MODE-1:** Singleton, affinity, and externalized placement are state dispositions, not dispatch modes. `state=singleton` MUST route remote execution to one logical owner or a stable singleton service identity and MUST NOT be lowered to a replicated deployment without an external state authority.

**Rule DP-MODE-2:** `mode=dynamic` is accepted only for `stateless`, `immutable-captured-config`, `externalized-durable`, or explicitly divergence-tolerant `process-local-cache` state. `singleton-mutable`, `connection-session`, and `shared-mutable-across-callers` state require `mode=remote` with singleton or affinity placement and no local/remote alternation unless an external authority owns the state. Violations MUST refuse with `MLV2_POLICY_STATE_CONFLICT`.

### Policy Expressions

**Rule DP-POLICY-1:** v2 policy expressions are the source-level successor to PLOS '25 delegate expressions. They describe per-lift runtime selection inputs, not a global optimizer.

**Rule DP-POLICY-2:** The baseline v2 expression form is a whitespace-separated key/value expression inside `policy="..."`. The concrete required instantiation is threshold-based metrics, for example `policy="trigger=CPU threshold=0.70"` or `policy="trigger=MEM threshold=0.80"`.

**Rule DP-POLICY-3:** A dynamic policy MUST identify a metric trigger and threshold supported by the compiler/runtime pair. Unsupported metrics MUST refuse with `MLV2_POLICY_UNSUPPORTED_TRIGGER`.

**Rule DP-POLICY-4:** Policy expressions MAY be extended later to invocation rate, latency, SLO, or learned controllers, but such extensions MUST preserve the per-lift contract and MUST NOT require source programs to solve cross-lift placement.

### Deferred Cross-Lift Optimization

**Rule DP-DEFER-1:** v2 does not define global or cross-lift placement optimization. The compiler/runtime may evaluate each lift's policy independently or with implementation-defined coordination, but the source contract exposes only per-lift policy.

Rationale: PLOS '25 identifies the composition of delegate expressions into a global state transition function as challenging and unsolved. Solving the order, hysteresis, and workload interaction among multiple dynamic lifts belongs in a future runtime/control sprint, not in this contract.

## Multi-Implementer Handling

### Unique Implementer as Optimization

**Rule MI-UNIQUE-1:** Finding a single production implementation for an interface is an optimization, not a prerequisite for v2 interface pragmas.

**Rule MI-UNIQUE-2:** If exactly one production implementation remains after build tags, generated mocks, tests, and ignored fakes are filtered, the compiler MAY infer `impl=<ConcreteName>` and record the inference in the closure report.

**Rule MI-UNIQUE-3:** If more than one production implementation remains, the compiler MUST use one of the explicit fallback paths: `impl=`, `registry=`, direct concrete annotation, or `dispatch=lift-point`. If none is present, it MUST refuse with `MLV2_IMPL_AMBIGUOUS`.

### `impl=` Disambiguation

**Rule MI-IMPL-1:** `impl=<ConcreteName>` on an interface pragma selects the named concrete type as the remote implementation for that interface lift.

**Rule MI-IMPL-2:** The named implementation MUST satisfy the annotated interface under the selected build configuration. If it does not, the compiler MUST refuse with `MLV2_IMPL_NOT_ASSIGNABLE`.

**Rule MI-IMPL-3:** `impl=` MUST resolve to exactly one concrete type in the package graph or through a package-qualified name. Ambiguous names MUST refuse with `MLV2_IMPL_NAME_AMBIGUOUS`.

### Direct Concrete Annotation

**Rule MI-CONCRETE-1:** When the desired lift boundary is a concrete type or method, developers SHOULD annotate the concrete struct or receiver method directly instead of annotating an interface plus `impl=`.

**Rule MI-CONCRETE-2:** Direct concrete annotation bypasses interface implementation search. Interface mocks, alternative backends, and adapters do not affect lift identity unless they are reachable inside the extraction closure.

This is the preferred path for Mattermost `UserService.CreateUser` and many Gitea service functions.

### Registry and Plugin Implementations

**Rule MI-REG-1:** When production identity is supplied by a registry or plugin table, the pragma MUST provide `registry=<key>` unless the compiler can infer exactly one registry key for the annotated concrete type.

**Rule MI-REG-2:** The compiler MUST trace the selected registry key to the registered concrete value or factory in the selected build. If the registration is hidden behind unresolved reflection or dynamic plugin loading, it MUST refuse with `MLV2_REGISTRY_UNRESOLVED`.

**Rule MI-REG-3:** The lifted deployable identity for registry roots is the pair `(registry namespace, registry key)`, not merely the Go type name.

This rule exists for Caddy modules registered by blank imports and `init()` calls.

### Adapter Wrapper Implementations

**Rule MI-WRAP-1:** The compiler MUST distinguish wrapping adapters from independent production implementations when resolving an interface lift.

**Rule MI-WRAP-2:** Type `W` is a wrapper of interface `I` iff `W` has one field, direct or promoted, whose type is assignable to `I`, and every method of `W` implementing `I` makes at least one call on that field with a matching method name. If this syntactic predicate does not hold and wrapper status affects implementation selection, the compiler MUST fall through to `MLV2_IMPL_WRAPPER_AMBIGUOUS`.

**Rule MI-WRAP-3:** Wrapping adapters MAY be included in the extraction closure around the selected implementation. They MUST NOT be counted as independent implementations for `MLV2_IMPL_AMBIGUOUS` unless the wrapper itself is the annotated or named `impl`.

**Rule MI-WRAP-4:** If wrapper detection is ambiguous and affects behavior, the compiler MUST require `impl=` or refuse with `MLV2_IMPL_WRAPPER_AMBIGUOUS`.

### Mocks and Build-Tagged Implementations

**Rule MI-FILTER-1:** Generated mocks, test-only fakes, and files excluded by the selected build tags MUST be ignored during production implementation resolution.

**Rule MI-FILTER-2:** A generated file is ignored for implementation resolution only when it is test-only by filename/package/build tag or matches an implementation-defined generated-mock detector recorded in the closure report. Non-test generated production code remains eligible.

**Rule MI-FILTER-3:** Build-tagged alternative production implementations are considered only if active in the selected build. Inactive variants MUST NOT create `MLV2_IMPL_AMBIGUOUS`.

### Lift the Dispatch Point

**Rule MI-DISPATCH-1:** `dispatch=lift-point` on an interface pragma means the lifted root is the production implementation-selection expression plus the selected implementation set, not one concrete implementation.

**Rule MI-DISPATCH-2:** This mode is accepted only when the implementation set is statically bounded and every selected implementation independently satisfies closure, state, transport, and adapter rules. Otherwise the compiler MUST refuse with `MLV2_DISPATCH_SET_UNBOUNDED`.

**Rule MI-DISPATCH-3:** The closure report MUST list every implementation in the dispatch set and the condition or registry key that selects it.

This mode covers Miniflux Google/OIDC provider selection and Gitea SMTP/Sendmail/Dummy sender selection when the user wants remote dispatch to preserve the original backend choice.

## Pragma Syntax v2

### v1 Syntax Inventory

The current codebase and paper expose two v1-era forms:

```go
// @monolift trigger=CPU threshold=0.5
type Service interface { /* demo compiler form */ }
```

```go
//monolift:offload metric=CPU threshold=75%
func hashPassword(pw string) (string, error) { /* paper form */ }
```

v1 parsing is whitespace key/value parsing over comments, with CPU/MEM threshold support in the codebase and paper examples using `metric=`/`threshold=`. v2 replaces these with one canonical pragma prefix and one grammar while preserving the comment-based compatibility promise.

### Canonical v2 Grammar

**Rule PS-GRAMMAR-1:** The canonical v2 pragma form is `//monolift:lift` followed by whitespace-separated key/value options.

```ebnf
pragma        = line-comment ws? "monolift:lift" { ws option } ;
line-comment  = "//" ;
option        = key "=" value ;
key           = ident { "." ident | ":" ident } ;
value         = bare-value | quoted-value ;
bare-value    = bare-char { bare-char } ;
quoted-value  = '"' { quoted-char | escape } '"' ;
ident         = letter { letter | digit | "_" | "-" } ;
bare-char     = letter | digit | "_" | "-" | "." | "/" | ":" | "," ;
quoted-char   = ? any non-quote, non-newline character ? ;
escape        = "\\" ( "\"" | "\\" | "n" | "t" ) ;
ws            = " " | "\t" ;
```

**Rule PS-GRAMMAR-2:** Unknown keys MUST be parseable but validation-defined. The prefix `x-` is globally reserved for implementation-defined extension keys across all conforming compilers. `MLV2_PRAGMA_UNKNOWN_KEY` MUST NOT fire for keys beginning with `x-`; unknown non-extension keys MUST produce `MLV2_PRAGMA_UNKNOWN_KEY`.

**Rule PS-GRAMMAR-3:** The pragma grammar MUST NOT require an import, generated schema file, IDL, or source-level wrapper type.

**Rule PS-ATTACH-1:** A v2 pragma MUST appear as a doc comment attached to the annotated declaration, represented by `ast.Decl.Doc` after Go parsing. A trailing comment, line-end comment, or separated comment group MUST refuse with `MLV2_PRAGMA_MISATTACHED`.

**Rule PS-ATTACH-2:** A declaration MUST NOT have more than one `//monolift:lift` pragma. Multiple lift pragmas attached to one declaration MUST refuse with `MLV2_PRAGMA_DUPLICATE`.

**Rule PS-ATTACH-3:** Comments beginning with `monolift:` but using verbs other than `lift`, such as `//monolift:retire`, MUST refuse with `MLV2_PRAGMA_UNKNOWN_VERB`.

### Keys and Requirements

| Key | Values | Required when | Notes |
|---|---|---|---|
| `name` | stable identifier | Always | Used for report/deployable identity. |
| `mode` | `local`, `remote`, `dynamic` | Optional; default `remote` | Must satisfy state disposition. |
| `state` | `stateless`, `singleton`, `affinity`, `external` | Optional when inference is deterministic; required to resolve ambiguity | Selects intended disposition, not a waiver. |
| `transport` | `http-json`, `handler`, `grpc` | Optional when canonical shape has one valid default | `grpc` may refuse as reserved. |
| `impl` | concrete type name or package-qualified name | Interface pragma with multiple impls unless `registry` or `dispatch=lift-point` is used | Disambiguates implementation. |
| `registry` | registry key string | Registry/plugin roots | Required for Caddy-style keyed module identity unless inferred. |
| `methods` | comma-separated method names | Optional on interface/struct pragmas | Narrows exposed surface. |
| `policy` | quoted policy expression | Required for `mode=dynamic` | Baseline supports `trigger=CPU|MEM threshold=<float>`. |
| `dispatch` | `impl`, `lift-point` | Optional on interface pragmas | `impl` is default. |
| `affinity` | context/request/session key | Required for `state=affinity` unless adapter supplies a framework key | Defines routing key. |

**Rule PS-KEY-1:** Keys not meaningful for the annotation surface MUST produce `MLV2_PRAGMA_INVALID_KEY_FOR_SURFACE`.

### Defaults and Validation

**Rule PS-DEFAULT-1:** `mode` defaults to `remote`. State disposition controls placement, so `state=singleton` does not introduce a separate dispatch mode.

**Rule PS-DEFAULT-2:** `transport` defaults from canonical shape: `handler` for HTTP handler shapes and `http-json` for accepted domain function/method shapes.

**Rule PS-DEFAULT-3:** `state` defaults from compiler inference. If inference is ambiguous, `state` is required.

**Rule PS-ERROR-1:** Parse errors are lexical or grammar failures in the pragma comment and MUST use `MLV2_PRAGMA_PARSE`.

**Rule PS-ERROR-2:** Validation errors are syntactically valid pragmas that violate a surface, implementation, state, transport, dispatch, or serialization rule. They MUST use the most specific named diagnostic.

Invalid combinations include: `mode=dynamic` without `policy`; `mode=dynamic` with singleton/session/shared mutable state lacking an external authority; `mode=local` with `transport=grpc`; `state=affinity` without an affinity key or handler-provided session key; `impl` on a function pragma; `methods` on a function pragma; `transport=handler` on a non-handler canonical shape; and `transport=grpc` on a compiler that has not implemented the reserved extension.

### Worked Pragma Examples

Interface with dynamic policy:

```go
//monolift:lift name=profile-store mode=dynamic state=external transport=http-json policy="trigger=CPU threshold=0.70"
type ProfileStore interface { Load(context.Context, int64) (*Profile, error) }
```

Function singleton worker:

```go
//monolift:lift name=campaign-worker mode=remote state=singleton transport=http-json
func worker(ctx context.Context, jobs <-chan CampaignJob) error { return nil }
```

Method with reserved gRPC transport:

```go
//monolift:lift name=user-create mode=remote state=external transport=grpc
func (s *UserService) CreateUser(ctx context.Context, u *User) (*User, error) { return u, nil }
// A compiler without gRPC support emits: MLV2_TRANSPORT_RESERVED.
```

Struct with shape-preserving handler transport:

```go
//monolift:lift name=reverse-proxy mode=remote state=external transport=handler methods=ServeHTTP
type ReverseProxy struct{}
```

Interface with explicit implementation:

```go
//monolift:lift name=mailer mode=remote impl=SMTPSender transport=http-json
type Sender interface { Send(context.Context, Message) error }
```

Registry-keyed module:

```go
//monolift:lift name=acme-issuer mode=remote state=singleton registry="tls.issuance.acme" methods=Provision,Issue
type ACMEIssuer struct{}
```

Local-only analysis mode:

```go
//monolift:lift name=expensive-render mode=local state=stateless
func Render(ctx context.Context, req RenderRequest) (RenderResponse, error) { return RenderResponse{}, nil }
```

Refusal diagnostic:

```go
//monolift:lift name=app mode=remote state=stateless
type App struct { db *sqlite.DB }
// Writes global lifecycle state and owns embedded DB: MLV2_EMBEDDED_DB_APP_ROOT.
```

### v1 Migration

**Rule PS-MIGRATE-1:** A v2 compiler MUST recognize known v1 `// @monolift ...` and `//monolift:offload ...` comment prefixes only to emit migration diagnostics. It MUST NOT silently treat them as v2 pragmas.

**Rule PS-MIGRATE-2:** When a v1 pragma is recognized, the compiler MUST emit `MLV2_PRAGMA_V1_DEPRECATED` as a warning with a suggested v2 rewrite using `//monolift:lift`, `name=<required>`, `mode=dynamic`, and `policy="trigger=<metric> threshold=<value>"` when the v1 keys are sufficient.

**Rule PS-MIGRATE-3:** Automatic rewriting is tooling-defined and outside this source contract. The canonical v2 input is the grammar above.

Rationale: silent acceptance would hide important v2 decisions such as annotation surface, state disposition, implementation identity, and transport shape. Warning and rewrite guidance preserve pay-as-you-go compatibility without carrying ambiguous v1 semantics forward.

### Phase 2-7 Representation Audit

| Decision | Representation |
|---|---|
| Accepted annotation surface | Inferred from pragma placement on interface, function, method, or struct declaration. |
| Interface implementation identity | `impl=`, `registry=`, or `dispatch=lift-point`; otherwise `MLV2_IMPL_AMBIGUOUS`. |
| Struct/interface exposed method subset | `methods=`. |
| Extraction root | Inferred from annotated declaration and selected `impl`/`registry`. |
| State disposition | `state=` or compiler inference recorded in closure report. |
| Transport | `transport=` or canonical-shape default. |
| Dispatch mode | `mode=`. |
| Dynamic policy | `policy=`. |
| Registry/plugin identity | `registry=`. |
| Session affinity | `affinity=` or handler adapter-provided key. |
| Unsupported/deferred surfaces | Deliberately no pragma representation; they refuse with named diagnostics. |
| Closure report contents | Deliberately inferred; no source key controls report schema. |
| Cross-lift optimization | Deliberately no pragma representation in v2; deferred by `DP-DEFER-1`. |

## Cross-Target Validation

### Miniflux

Plausible annotation:

```go
//monolift:lift name=miniflux-feed-processor mode=dynamic state=external transport=http-json policy="trigger=CPU threshold=0.75"
func ProcessFeedEntries(ctx context.Context, feedID int64, force bool) error
```

- **Extraction root:** package-level feed fetch/process function under `AS-FUNC-*` and `EC-ROOT-2`.
- **Closure sketch:** feed fetcher, parser, persistence client calls, worker concurrency helpers, provider selection used by the feed path, and reachable domain types. Stdlib and external modules terminate under `EC-TERM-*`.
- **State classification:** the dynamic example assumes scheduling is externalized, so durable feed/user data and queue ownership are `externalized-durable`. If feed-worker concurrency remains process-local, the lift must use `mode=remote state=singleton` instead of `mode=dynamic`.
- **Transport:** `multi-domain-args` or `no-response` shape over `http-json`; errors carry remote failure.
- **Dispatch granularity:** per call site to the feed-processing function under `DP-GRAN-1`.
- **Implementer handling:** Google/OIDC provider ambiguity uses `dispatch=lift-point` if provider selection itself is lifted; otherwise it remains an included bounded dispatch set.
- **Verdict:** accept. The target exercises function pragmas, stateful worker rules, multi-implementation provider handling, and non-`(ctx, req)` method shape without requiring application rewrites beyond annotations.

### Listmonk

Plausible annotation:

```go
//monolift:lift name=listmonk-campaign-worker mode=remote state=singleton transport=http-json
func worker(ctx context.Context, jobs <-chan CampaignJob) error
```

Alternative handler annotation:

```go
//monolift:lift name=listmonk-preview mode=remote transport=handler
func previewCampaign(c echo.Context) error
```

- **Extraction root:** named worker function under `AS-FUNC-*`; Echo handlers classify under `http-handler`.
- **Closure sketch:** campaign job loop, template/render helpers, DB/list repositories, app-config values, and mailer/external provider clients. `App{...}` wiring is accepted as initialization value graph under `EC-WIRE-*`.
- **State classification:** worker queue and goroutine loop are `singleton-mutable`; render/template caches are `process-local-cache`; database and mail transport are `externalized-durable`.
- **Transport:** singleton worker uses `http-json` control/invocation adapter only if channel remains internal or is represented by an external queue; Echo handlers use `handler`.
- **Dispatch granularity:** per function call site for worker entry; per handler method/function for HTTP route.
- **Implementer handling:** no unique-interface dependency required; mail/render adapters are included or disambiguated only if they become interface roots.
- **Verdict:** candidate accepted under conditions. The channel is valid only if the worker owns it wholly inside the lifted deployable or the queue is externalized; raw channels cannot cross the boundary.

### Caddy

Plausible annotation:

```go
//monolift:lift name=caddy-acme-issuer mode=remote state=singleton transport=handler registry="tls.issuance.acme" methods=Provision,Issue
type ACMEIssuer struct{}
```

- **Extraction root:** registry-keyed struct type under `AS-STRUCT-*`, `EC-ROOT-4`, and `MI-REG-*`.
- **Closure sketch:** selected module struct, registered factory, provisioning path, issuer/cert storage clients, handler or middleware adapter methods, and reachable Caddy module interfaces. Blank-import `init()` registration is wiring evidence under `EC-WIRE-*`.
- **State classification:** cert issuance state and rate-limit/coordinator state are `singleton-mutable`; durable cert storage is `externalized-durable`; handler request state is adapter-managed.
- **Transport:** Caddy middleware/handler signatures use shape-preserving `handler`; non-handler issuance calls use `http-json` only if they classify as domain methods.
- **Dispatch granularity:** per exposed method selected by `methods=`.
- **Implementer handling:** registry key supplies identity; many implementations per interface are expected and do not cause ambiguity.
- **Verdict:** candidate accepted under conditions for the static-registry subset: registry-keyed module or cert-issuer roots are accepted when the registry value is statically visible. Dynamic module loading remains deferred and refuses with `MLV2_DYNAMIC_PLUGIN` or `MLV2_REGISTRY_UNRESOLVED`.

### Gitea

Plausible annotation:

```go
//monolift:lift name=gitea-mailer mode=remote state=external transport=http-json impl=SMTPSender
type Sender interface { Send(context.Context, Message) error }
```

Alternative concrete annotation:

```go
//monolift:lift name=gitea-notify mode=remote state=external transport=http-json
func (s *NotifyService) Notify(ctx context.Context, req NotifyRequest) error
```

- **Extraction root:** interface plus `impl=` under `MI-IMPL-*`, or concrete receiver method under `AS-METHOD-*`.
- **Closure sketch:** mailer context initialization, cache/config reads, sender backend, message templates, and external SMTP/sendmail clients. `init()` chains are wiring evidence, not a `main` dependency.
- **State classification:** mailer config and sender clients are `externalized-durable` or `immutable-captured-config`; caches are `process-local-cache`.
- **Transport:** mailer/domain operations classify as `multi-domain-args` or `no-response` over `http-json`.
- **Dispatch granularity:** per interface method or concrete method call.
- **Implementer handling:** SMTP/Sendmail/Dummy require `impl=` or `dispatch=lift-point`; inactive build-tag alternatives and test mocks are filtered.
- **Verdict:** accept for mailer/notification. Router code that directly interleaves HTTP routing, model mutation, and persistence without a bounded adapter is refused as `MLV2_CLOSURE_TOO_LARGE` or `MLV2_SHARED_MUTABLE_STATE`; that boundary should not be lifted wholesale.

### Mattermost

Plausible annotation:

```go
//monolift:lift name=mattermost-user-create mode=remote state=external transport=http-json
func (s *UserService) CreateUser(rctx request.CTX, user *model.User, opts UserCreateOptions) (*model.User, error)
```

- **Extraction root:** concrete receiver method under `AS-METHOD-*` and `EC-ROOT-3`.
- **Closure sketch:** `UserService` receiver fields reachable from `CreateUser`, store calls, options-builder initialized dependencies, request context adapter, validation helpers, and model types. Generated mocks are ignored for implementation resolution.
- **State classification:** primary user/store state is `externalized-durable`; receiver config is `immutable-captured-config`; request context is adapter-managed; WebHub is `connection-session`/`singleton-mutable` if included.
- **Transport:** `multi-domain-args` over `http-json` with generated request envelope. `request.CTX` requires an explicit adapter; if unavailable, refusal is `MLV2_SERIALIZATION_UNSUPPORTED`.
- **Dispatch granularity:** per method call site.
- **Implementer handling:** direct concrete method annotation avoids interface/mocks ambiguity.
- **Verdict:** candidate accepted under conditions for `UserService.CreateUser`: `request.CTX` requires explicit context-value or serialization adapter metadata. Websocket hub lifting is deferred unless an affinity key and singleton/affinity policy are specified; otherwise `MLV2_SESSION_AFFINITY_UNAVAILABLE`.

### PocketBase

Candidate annotation that must refuse:

```go
//monolift:lift name=pocketbase-app mode=remote state=external
type App interface { /* core.App broad application surface */ }
```

- **Extraction root:** `core.App`-like broad app object initially appears as interface/struct surface but fails closure and state rules.
- **Closure sketch:** app lifecycle hooks, router/server state, embedded SQLite ownership, collection/model APIs, auth, subscriptions, and broad method surface. Pruning cannot isolate a bounded deployable for the whole app object.
- **State classification:** embedded SQLite app runtime is `shared-mutable-across-callers` plus `externalized-durable` unavailable because the DB is owned in-process; lifecycle hooks are app-wide initialization state.
- **Transport:** no single canonical shape for the whole 190+ method surface; some methods may individually classify, but the app root does not.
- **Dispatch granularity:** broad app/interface dispatch would become whole-runtime dispatch, which v2 forbids.
- **Implementer handling:** unique implementation does not help; fit on constructor/method shape is misleading because state and closure dominate.
- **Verdict:** refuse. Required diagnostics are `MLV2_EMBEDDED_DB_APP_ROOT` and `MLV2_CLOSURE_TOO_LARGE`; additional methods may also trigger `MLV2_SHARED_MUTABLE_STATE`. This is not future-work hand-waving: PocketBase defines the lower bound of v2's refusal contract.

### Validation Matrix

| Design axis | Miniflux | Listmonk | Caddy | Gitea | Mattermost | PocketBase |
|---|---|---|---|---|---|---|
| Annotation surface | `AS-FUNC-1` | `AS-FUNC-1`, `TA-HANDLER-1` | `AS-STRUCT-1` | `AS-METHOD-1`, `AS-IFACE-1` | `AS-METHOD-1` | `AS-STRUCT-2` then refusal |
| Extraction root | `EC-ROOT-2` | `EC-ROOT-2` | `EC-ROOT-4`, `MI-REG-1` | `EC-ROOT-3`, `EC-WIRE-1` | `EC-ROOT-3` | `EC-PRUNE-3` |
| State semantics | `SS-DISP-1` singleton worker | `SS-LIFT-4` singleton queue | `SS-DISP-1` singleton issuer | `SS-LIFT-5` cache/external | `SS-CLASS-1`, `SS-WALDO-1` | `MLV2_EMBEDDED_DB_APP_ROOT` |
| Transport | `TA-SHAPE-1` multi-domain/no-response | `TA-HANDLER-1`, `TA-SHAPE-1` channel-consumer | `TA-HANDLER-1` | `TA-SHAPE-1` multi-domain/no-response | `TA-SHAPE-1`, `TA-SER-4` | `TA-REFUSE-1` |
| Dispatch granularity | `DP-GRAN-1` function call site | `DP-GRAN-1` function/handler | `DP-GRAN-1` struct methods | `DP-GRAN-1` interface/method | `DP-GRAN-1` method | `DP-GRAN-1` refuses whole runtime |
| Multi-implementer | `MI-DISPATCH-1` | `MI-CONCRETE-2` | `MI-REG-1` | `MI-IMPL-1`, `MI-DISPATCH-1` | `MI-FILTER-1` | `MI-UNIQUE-3` not sufficient |
| Pragma syntax | `PS-KEY-1`, `policy=` | `PS-KEY-1`, `state=singleton` | `registry=`, `methods=` | `impl=` or `dispatch=` | concrete method keys | refusal diagnostics |

### Target Dossier Follow-Ups

- `docs/evaluation/targets/01-gitea.md`: add mailer/notification architecture notes, sender implementation list, and router-to-models refusal example.
- `docs/evaluation/targets/02-mattermost.md`: add `UserService.CreateUser` dependency sketch, `request.CTX` adapter needs, and WebHub affinity notes.
- `docs/evaluation/targets/03-caddy.md`: add module registry flow, ACME issuer state ownership, and handler signature examples.
- `docs/evaluation/targets/04-listmonk.md`: add campaign worker queue ownership, `App{...}` field map, and Echo handler examples.
- `docs/evaluation/targets/05-pocketbase.md`: add concrete `core.App` method-surface size, SQLite ownership notes, and lifecycle-hook refusal evidence.
- `docs/evaluation/targets/06-miniflux.md`: add feed processor candidate path, worker concurrency notes, and Google/OIDC provider implementation sketch.

### Revision History

| Revision | Result |
|---|---|
| 1.0 review integration | Category A contract blockers changed the closure-report schema, closure analysis baseline, bounded-closure predicate, wiring whitelist, adapter metadata, wrapper predicate, generic declaration handling, dispatch modes, pragma extension keys, pragma attachment, v1 migration warnings, remote outcome model, pointer aliasing rule, dynamic-state invariant, composite state classification, external-module state/effect summaries, conditional target verdicts, revision history, and traceability handling. Category B research-narrative items remain deferred. |

## Appendix A: Closure Report JSON Schema

This schema is normative for the v1.0 closure report. Fields not relevant to a refused pragma are present as empty arrays or `null` where the schema allows it. Arrays whose items contain `symbol` or `identity` objects are sorted lexicographically by `module_path`, `package_path`, `object_name`, `kind`, and serialized `instantiation`.

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://monolift.dev/schemas/closure-report-v1.0.json",
  "title": "Monolift v2 Closure Report",
  "type": "object",
  "required": [
    "schemaVersion",
    "buildConfig",
    "analysis",
    "pragma",
    "root",
    "closure",
    "state",
    "adapters",
    "externalDependencies",
    "pruning",
    "diagnostics"
  ],
  "properties": {
    "schemaVersion": { "const": "1.0" },
    "buildConfig": { "$ref": "#/$defs/buildConfig" },
    "analysis": {
      "type": "object",
      "required": ["algorithm", "precisionTriggers", "deterministic"],
      "properties": {
        "algorithm": { "type": "string" },
        "precisionTriggers": { "type": "array", "items": { "type": "string" } },
        "deterministic": { "const": true }
      },
      "additionalProperties": false
    },
    "pragma": {
      "type": "object",
      "required": ["name", "surface", "span", "options"],
      "properties": {
        "name": { "type": "string" },
        "surface": { "enum": ["interface", "function", "method", "struct"] },
        "span": { "$ref": "#/$defs/sourceSpan" },
        "options": { "type": "object", "additionalProperties": { "type": "string" } }
      },
      "additionalProperties": false
    },
    "root": {
      "type": "object",
      "required": ["identity", "registryKey", "exposedOperations"],
      "properties": {
        "identity": { "$ref": "#/$defs/symbolIdentity" },
        "registryKey": { "type": ["string", "null"] },
        "exposedOperations": {
          "type": "array",
          "items": { "$ref": "#/$defs/symbolIdentity" }
        }
      },
      "additionalProperties": false
    },
    "closure": {
      "type": "object",
      "required": ["includedSymbols", "excludedSymbols", "wiringPaths"],
      "properties": {
        "includedSymbols": { "type": "array", "items": { "$ref": "#/$defs/symbolEntry" } },
        "excludedSymbols": { "type": "array", "items": { "$ref": "#/$defs/symbolEntry" } },
        "wiringPaths": { "type": "array", "items": { "$ref": "#/$defs/wiringPath" } }
      },
      "additionalProperties": false
    },
    "state": {
      "type": "array",
      "items": { "$ref": "#/$defs/stateFacet" }
    },
    "adapters": {
      "type": "array",
      "items": { "$ref": "#/$defs/adapter" }
    },
    "externalDependencies": {
      "type": "array",
      "items": { "$ref": "#/$defs/externalDependency" }
    },
    "pruning": {
      "type": "object",
      "required": ["bounded", "frontier"],
      "properties": {
        "bounded": { "type": "boolean" },
        "frontier": { "type": "array", "items": { "$ref": "#/$defs/symbolEntry" } }
      },
      "additionalProperties": false
    },
    "diagnostics": {
      "type": "array",
      "items": { "$ref": "#/$defs/diagnostic" }
    }
  },
  "additionalProperties": false,
  "$defs": {
    "buildConfig": {
      "type": "object",
      "required": ["GOOS", "GOARCH", "CGO_ENABLED", "buildTags", "moduleRoot", "workspaceMode", "tests", "dependencyManifest"],
      "properties": {
        "GOOS": { "type": "string" },
        "GOARCH": { "type": "string" },
        "CGO_ENABLED": { "type": "boolean" },
        "buildTags": { "type": "array", "items": { "type": "string" } },
        "moduleRoot": { "type": "string" },
        "workspaceMode": { "type": "string" },
        "tests": { "type": "boolean" },
        "dependencyManifest": {
          "type": "array",
          "items": {
            "type": "object",
            "required": ["module_path", "version", "sum"],
            "properties": {
              "module_path": { "type": "string" },
              "version": { "type": "string" },
              "sum": { "type": "string" }
            },
            "additionalProperties": false
          }
        }
      },
      "additionalProperties": false
    },
    "symbolIdentity": {
      "type": "object",
      "required": ["module_path", "package_path", "object_name", "kind"],
      "properties": {
        "module_path": { "type": "string" },
        "package_path": { "type": "string" },
        "object_name": { "type": "string" },
        "kind": { "enum": ["function", "method", "type", "interface", "field", "variable", "constant", "registry-entry", "package", "adapter"] },
        "instantiation": {
          "type": ["array", "null"],
          "items": { "type": "string" }
        }
      },
      "additionalProperties": false
    },
    "sourceSpan": {
      "type": "object",
      "required": ["file_relative_path", "byte_offset_start", "byte_offset_end", "line_start", "line_end"],
      "properties": {
        "file_relative_path": { "type": "string" },
        "byte_offset_start": { "type": "integer", "minimum": 0 },
        "byte_offset_end": { "type": "integer", "minimum": 0 },
        "line_start": { "type": "integer", "minimum": 1 },
        "line_end": { "type": "integer", "minimum": 1 }
      },
      "additionalProperties": false
    },
    "symbolEntry": {
      "type": "object",
      "required": ["identity", "span", "ruleIds"],
      "properties": {
        "identity": { "$ref": "#/$defs/symbolIdentity" },
        "span": { "$ref": "#/$defs/sourceSpan" },
        "ruleIds": { "type": "array", "items": { "type": "string" } }
      },
      "additionalProperties": false
    },
    "wiringPath": {
      "type": "object",
      "required": ["target", "steps"],
      "properties": {
        "target": { "$ref": "#/$defs/symbolIdentity" },
        "steps": { "type": "array", "items": { "$ref": "#/$defs/symbolEntry" } }
      },
      "additionalProperties": false
    },
    "stateFacet": {
      "type": "object",
      "required": ["symbol", "classes", "disposition", "evidence", "developerDeclared"],
      "properties": {
        "symbol": { "$ref": "#/$defs/symbolIdentity" },
        "classes": {
          "type": "array",
          "items": { "enum": ["stateless", "immutable-captured-config", "externalized-durable", "process-local-cache", "singleton-mutable", "shared-mutable-across-callers", "connection-session"] },
          "minItems": 1
        },
        "disposition": { "enum": ["replicated", "singleton", "affinity-routed", "externalize-required", "refused"] },
        "evidence": { "type": "array", "items": { "type": "string" } },
        "developerDeclared": { "type": "boolean" }
      },
      "additionalProperties": false
    },
    "adapter": {
      "type": "object",
      "required": ["kind", "id", "matchedSymbols", "canonicalShapes", "stateEffects", "transportEffects", "serializationEffects"],
      "properties": {
        "kind": { "enum": ["handler", "registry", "serialization", "context-value", "cgo", "reflection", "generic-substitution"] },
        "id": { "type": "string" },
        "matchedSymbols": { "type": "array", "items": { "$ref": "#/$defs/symbolIdentity" } },
        "canonicalShapes": { "type": "array", "items": { "type": "string" } },
        "stateEffects": { "type": "array", "items": { "type": "string" } },
        "transportEffects": { "type": "array", "items": { "type": "string" } },
        "serializationEffects": { "type": "array", "items": { "type": "string" } }
      },
      "additionalProperties": false
    },
    "externalDependency": {
      "type": "object",
      "required": ["identity", "accessPath", "configurationSource", "stateEffectSummary"],
      "properties": {
        "identity": { "$ref": "#/$defs/symbolIdentity" },
        "accessPath": { "type": "string" },
        "configurationSource": { "type": "string" },
        "stateEffectSummary": { "type": "array", "items": { "type": "string" } }
      },
      "additionalProperties": false
    },
    "diagnostic": {
      "type": "object",
      "required": ["code", "severity", "span", "ruleIds", "message"],
      "properties": {
        "code": { "type": "string" },
        "severity": { "enum": ["error", "warning"] },
        "span": { "$ref": "#/$defs/sourceSpan" },
        "ruleIds": { "type": "array", "items": { "type": "string" } },
        "message": { "type": "string" },
        "remediation": { "type": ["string", "null"] }
      },
      "additionalProperties": false
    }
  }
}
```

## Alternatives Rejected

### Rejection-Rationale Table

| Rejected alternative | Rationale | Evidence checkbox |
|---|---|---|
| Keep interface-only annotation | The audit says service interfaces are rare and business logic is usually concrete functions/methods. | [x] Audit §Service "interface" is rare |
| Continue `main`-walk extraction | All six targets wire through `init`, options builders, registries, lifecycle hooks, app structs, or CLI delegation. | [x] Audit §Wiring doesn't live in `main` |
| Require stateless-only lifts | Stateful services are the norm in five of six targets. | [x] Audit §Stateful services are the norm |
| Force HTTP/JSON on HTTP handler lifts | HTTP-shaped handlers already have a request/response protocol; JSON-RPC re-encoding adds adapter cost and semantic risk. | [x] Audit §Method shapes are heterogeneous |
| Add a custom IDL | A new IDL violates the pay-as-you-go compatibility promise and the research brief's no-new-language claim. | [x] Research brief §0 claim 1 |
| Make gRPC mandatory | Mandatory gRPC adds schema and adoption tax before v2 solves the audited source-contract failures. | [x] Research brief §8 communication options |
| Promise full transparent distribution | Waldo-style location transparency is a known antipattern; v2 must expose failure and deadline differences. | [x] Research brief §2 distributed object caution |
| Adopt an actor framework wholesale | A lift is a compiler-selected source segment, not an actor API or supervision framework. | [x] Research brief §2 actor distinction |
| Interface method pragma | No implementation body or independent production identity; interface `methods=` or concrete method annotations cover the valid use cases. | [x] Audit §Service "interface" is rare |
| Function-valued var pragma | Value-flow and reassignment semantics exceed v2 declaration-root scope. | [x] Research brief §0 low-commitment annotations |
| Anonymous function pragma | No stable declaration name or deployable identity. | [x] Research brief §0 annotations on code constructs |
| Generic instantiation pragma | Instantiation expressions are not declaration roots; generic declaration support needs closure-report type substitution first. | [x] Audit implication: method-shape adapter generation |
| Whole-package pragma | Package boundaries do not equal service boundaries and risk absorbing the monolith. | [x] Audit item G |

## Review and Revision

### Internal Consistency Pass

- Normative terms are defined in [Glossary](#glossary) and referenced elsewhere without duplicate term-definition sections.
- MUST/SHOULD/MAY statements are attached to named rules, normative tables, JSON schema fields, or explicit diagnostics.
- v1.0 review edits were checked for singleton mode/state consistency, generic declaration handling, adapter metadata, and closure-report schema references.

### Traceability Pass

- The original Phase 0 traceability table was deleted in v1.0 because it linked every audit item to the same broad section set rather than identifying resolving rules.
- Traceability is now carried by the validation matrix, refusal diagnostic index, and cross-target subsections, each of which names concrete rule IDs or diagnostics.

### Target-Coverage Pass

- All six required targets have validation subsections.
- The validation matrix has seven design-axis rows and six target columns with no empty cells.
- PocketBase is a concrete refusal using `MLV2_EMBEDDED_DB_APP_ROOT` and `MLV2_CLOSURE_TOO_LARGE`.

### PLOS '25 Alignment Pass

| Baseline claim | v2 status | Rationale |
|---|---|---|
| Annotated code segments become lifts | Revised | Still true, but accepted Go surfaces are explicitly bounded to interface declarations, package functions, concrete receiver methods, and struct types. |
| Lift points live at call sites | Preserved | v2 keeps compiler-inserted invocation sites and defines per-surface dispatch granularity. |
| Ordinary monolith execution remains transparent | Preserved | Compatibility promise CP-1/CP-2 is non-negotiable. |
| Dynamic delegation via delegate expressions | Revised | v2 keeps per-lift dynamic policies but defers global transition-function composition. |
| Bounded lift model | Revised | v2 relaxes stateless-only into classified state dispositions while preserving refusal for hidden shared mutable state and embedded app runtimes. |
| Kubernetes as compiler backend | Preserved | v2 keeps Kubernetes-compatible deployables but treats backend details as outside the source contract. |
| Lifts are stateless | Revised | Stateless remains one state class; singleton and affinity dispositions are accepted under explicit rules. |
| Wiring lives in `main` | Retired | Extraction is call/value-graph driven from the annotated root; wiring source is irrelevant. |
| Annotation site is an interface | Revised | Interfaces are one accepted surface among several. |
| Dual dispatch at interface granularity | Revised | Dispatch granularity follows annotation surface and exposed operation. |

### Refusal Diagnostic Index

All diagnostics below are compile-time refusals unless marked as a warning.

| Diagnostic | Meaning |
|---|---|
| `MLV2_ADAPTER_METADATA_INVALID` | Adapter metadata is malformed or incomplete for an acceptance decision. |
| `MLV2_BUILDER_CHAIN_ROOT` | Builder chain selected as lift root. |
| `MLV2_CGO_UNLIFTABLE` | cgo dependency cannot be reproduced remotely. |
| `MLV2_CHANNEL_BOUNDARY` | Channel must cross remote boundary. |
| `MLV2_CLOSURE_UNBOUNDED` | Non-dispatch closure frontier is not statically bounded; see [ADR-0014](../decisions/0014-unbounded-edge-refusal-code.md). |
| `MLV2_CLOSURE_TOO_LARGE` | Closure cannot be pruned to a bounded deployable. |
| `MLV2_DISPATCH_SET_UNBOUNDED` | Interface dispatch set is not statically bounded. |
| `MLV2_DYNAMIC_PLUGIN` | Plugin loading is dynamic/unresolved. |
| `MLV2_EMBEDDED_DB_APP_ROOT` | Embedded DB app runtime selected as root. |
| `MLV2_EXTERNAL_STATE_UNRESOLVED` | Imported module state/effect summary is unavailable or ambiguous. |
| `MLV2_IMPL_AMBIGUOUS` | Multiple production implementations without disambiguation. |
| `MLV2_IMPL_NAME_AMBIGUOUS` | `impl=` name resolves ambiguously. |
| `MLV2_IMPL_NOT_ASSIGNABLE` | `impl=` type does not satisfy interface. |
| `MLV2_IMPL_WRAPPER_AMBIGUOUS` | Wrapper versus independent implementation is ambiguous. |
| `MLV2_INTERFACE_SERIALIZATION` | Interface-typed boundary value has unbounded concrete type set. |
| `MLV2_NO_ERROR_CHANNEL` | Remote failure cannot be represented by the source shape. |
| `MLV2_POINTER_ALIAS_UNSUPPORTED` | Pointer argument/result alias preservation cannot be represented remotely. |
| `MLV2_POLICY_CONFLICT` | Multiple policies target the same lift point incompatibly. |
| `MLV2_POLICY_STATE_CONFLICT` | Policy conflicts with state disposition. |
| `MLV2_POLICY_UNSUPPORTED_TRIGGER` | Dynamic policy trigger is unsupported. |
| `MLV2_PRAGMA_DUPLICATE` | Multiple `monolift:lift` pragmas attach to one declaration. |
| `MLV2_PRAGMA_INVALID_KEY_FOR_SURFACE` | Key is invalid for the annotated surface. |
| `MLV2_PRAGMA_MISATTACHED` | Pragma is not attached as the declaration doc comment. |
| `MLV2_PRAGMA_PARSE` | Pragma text fails grammar parsing. |
| `MLV2_PRAGMA_REGION_CONFLICT` | Shared-name peer pragmas disagree on a region-wide option after defaults. |
| `MLV2_PRAGMA_UNKNOWN_KEY` | Unknown non-extension key. |
| `MLV2_PRAGMA_UNKNOWN_VERB` | `monolift:` comment uses an unsupported verb. |
| `MLV2_PRAGMA_V1_DEPRECATED` | Warning for recognized v1 syntax. |
| `MLV2_REFLECTION_DISPATCH` | Reflection target set is unresolved. |
| `MLV2_REGISTRY_UNRESOLVED` | Registry key cannot be traced to concrete value. |
| `MLV2_REMOTE_FALLBACK_UNSAFE` | Failed remote attempt would automatically fall back locally without idempotency or deduplication. |
| `MLV2_ROOT_UNSTABLE` | Lift root cannot be named deterministically. |
| `MLV2_SERIALIZATION_UNSUPPORTED` | Adapter-visible value cannot be serialized. |
| `MLV2_SESSION_AFFINITY_UNAVAILABLE` | Session state exists without stable affinity. |
| `MLV2_SHAPE_UNSUPPORTED` | Signature does not match an accepted canonical shape. |
| `MLV2_SHARED_MUTABLE_STATE` | Hidden shared mutable state requires distributed coherence. |
| `MLV2_STATE_DECL_CONFLICT` | Declared state contradicts compiler evidence. |
| `MLV2_STATE_UNKNOWN` | State class remains correctness-relevant and ambiguous. |
| `MLV2_STRUCT_SURFACE_UNSUPPORTED` | Struct exposed method surface includes unsupported method. |
| `MLV2_SURFACE_ANON_FUNC` | Anonymous function selected as pragma root. |
| `MLV2_SURFACE_DEFERRED_GENERIC_DECL` | Generic declaration selected without enumerable selected-build instantiations. |
| `MLV2_SURFACE_DEFERRED_FUNCTION_VALUE` | Function-valued var selected as pragma root. |
| `MLV2_SURFACE_DEFERRED_GENERIC_INSTANTIATION` | Generic instantiation expression selected as pragma root. |
| `MLV2_SURFACE_INTERFACE_METHOD` | Interface method line selected as pragma root. |
| `MLV2_SURFACE_WHOLE_PACKAGE` | Whole package selected as pragma root. |
| `MLV2_TRANSPORT_RESERVED` | Reserved transport requested but not implemented. |
| `MLV2_WIRING_UNRESOLVED` | Initialization/value graph cannot be represented deterministically. |

Refusal-rules pass result: every refusal in this specification names a diagnostic in this index or uses a more specific diagnostic already listed here.

### Waldo Pass

- The spec explicitly states that remote lift invocation is not semantically identical to a local call.
- Failure, cancellation, deadline, panic, and zero-value behavior are specified in `SS-WALDO-*`, `TA-CTX-*`, and `TA-REFUSE-1`.
- The only preserved transparency claim is ordinary uncompiled monolith execution under `go build`, not network transparency.
