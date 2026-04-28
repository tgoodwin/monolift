# ADR-0025: Compiler-derived distribution shape

**Status:** accepted
**Date:** 2026-04-27
**Related:** ADR-0017 (classifier reasons about liftability), ADR-0018 (liftability property taxonomy), ADR-0022 (composite-archetype regions), ADR-0023 (sidecar emission and real-symbol execution), ADR-0024 (multi-root region pragma)

## Context

The user-facing pragma surface today already includes `transport=` (`http-json`, `handler`, `grpc`) as an optional key. SPRINT-0019/0020 lifts (caddy `CleanPath`, miniflux `EstimateReadingTime`) used `http-json` because they had pure-function surfaces that marshal cleanly. The Mattermost Hub/WebConn investigation (SPRINT-0021/0022/follow-up) surfaced a different surface shape: HTTP routes that take ownership of the underlying TCP connection for a long-lived session (websocket upgrade), where no marshaling boundary exists at all — frames must be tunneled, not encoded.

A natural design question arose: should the user write `transport=stream-proxy` to opt into the websocket-tunnel emission path? Two related questions came with it: should the lifted application's external API surface change (i.e., does `host:443/api/v4/websocket` still work for clients after the lift)? And are transport choices interchangeable — could a user override one with another?

A further question, raised during ADR drafting: today's `transport=http-json | handler | grpc` muddles two distinct axes. `http-json` is a wire protocol for synchronous request-response. `grpc` could be unary (still synchronous request-response) or streaming (session-shaped); the value doesn't tell you which. A function like `func(in <-chan A, out chan<- B)` is neither call-shaped nor session-shaped — it's an asynchronous producer-consumer that might map to a message queue. The pragma key conflates *what the surface admits semantically* with *which wire protocol implements that semantic*.

This ADR records four coupled decisions that emerged from those discussions.

## Decision

### 1. The compiler derives distribution shape from the region's surface; the user states intent, not implementation

The user's annotation footprint is **what to lift** — `//monolift:lift name=<region>` on the declaration(s) that root the region. The compiler is responsible for **how** the lift is realized: which surface category applies, what the dialer stub looks like, how the extracted service's boot sequence is reconstructed, what the failure modes are.

The compiler's job is to derive the shape of distribution this code admits, given its surface. This framing is consistent with the PLOS '25 minimal-annotation contract: developers declare regions; the compiler does the figuring out.

Pragma keys describing *implementation* (e.g., `transport=`) are not primary inputs. They become **forcing overrides** the user can supply only to demand a specific compiler-derived choice when multiple choices are admissible (see decision 4). In the typical case, no user-facing transport key is needed.

### 2. Surface categories and wire protocols are two distinct axes

The compiler reasons about distribution along two axes that the existing `transport=` pragma key has been conflating:

- **Surface category** — the semantic shape of distribution the region's external surface admits. *Derived from code, not a user choice.* A region with a hijack-shaped external surface cannot be lifted as if it were function-shaped, and vice versa: these are different distribution semantics, not different encodings.
- **Wire protocol within a category** — the concrete on-the-wire implementation of that semantic. Multiple wire protocols may implement the same semantic with different cost/latency/durability tradeoffs.

The v0 surface categories are:

**Call surface** — synchronous request-response. Function shape: `func(args) (result, error)`. Marshalable arguments in, marshalable result out, discrete round-trip per invocation. Wire protocols within this category include HTTP+JSON (today's `http-json`), HTTP+protobuf, gRPC unary, JSON-RPC. Lifts in SPRINT-0019/0020 (caddy `CleanPath`, miniflux `EstimateReadingTime`) live here.

**Session surface** — long-lived bidirectional byte stream owned by the handler. The handler takes ownership of the underlying connection at handshake time and runs until the session ends. There is no marshalable result; the wire bytes belong to whatever session protocol the application speaks. Wire protocols within this category include raw TCP proxy (host bridges client connection ↔ extracted-service connection, copies bytes both ways), gRPC bidi streams (with framing), SSE-plus-POST (asymmetric pairing). Mattermost's `/api/v4/websocket` route is here.

**Async producer-consumer surface** — channel-passing. Function shape: takes input channels and/or returns output channels (e.g., `func(in <-chan A, out chan<- B)`); does not block on a single marshalable result. The function consumes and produces asynchronously over time. Wire protocols within this category include durable message queues (Kafka, NATS, Redis Streams), event buses, gRPC streaming used as a queue. The semantic gap from in-process channels (unbounded ordering, lossless, single-process) to a distributed analog (bounded reordering tolerance, possible loss, multi-replica) is real and adapter-specific.

The compiler's surface-derivation pass picks a category by inspecting the region's external entry points:

- If every external entry point is function-shaped with marshalable arguments and a marshalable result, the category is **call surface**.
- If any external entry point takes ownership of the underlying connection (Go-specific detection signal: `http.Hijacker.Hijack()`, `(*websocket.Upgrader).Upgrade()`, raw `net.Conn` exposure; analogous primitives in other languages), the category is **session surface**.
- If any external entry point's signature is channel-passing in the function-argument or return position, the category is **async producer-consumer**.
- If the surface mixes incompatible categories or matches none of the above, refuse with a diagnostic naming the surface shape and the recognized categories.

"Hijack" is one (common) Go implementation detail of session surfaces, not the category. The category is defined by the semantic shape — long-lived session ownership — and the same shape exists in other languages under different primitives (`HttpServletRequest.upgrade()` in Java, `socket.upgrade()` in Node, hyper's upgrade machinery in Rust).

The seam-shape admission check from ADR-0024 / SPRINT-0022 lives at a different layer than surface-category derivation. Internal seams (channel coupling *inside* the region's union closure) are admission concerns about whether the lifted boundary preserves region semantics. External surface category is what determines transport. A region can have async-producer-consumer-shaped internal seams (Hub's `h.broadcast` channel) while having a session-surface external entry point (the websocket route); the seam admits in-region, the external surface drives transport selection.

### 3. The lifted application's external API surface is preserved

After a lift, existing callers and clients see the same external API as before. `host:443/api/v4/websocket` continues to accept websocket upgrades from the same clients with the same authentication and the same wire protocol. `host:80/cleanpath?p=...` continues to return the same response. The lift is internal restructuring, not an external contract change.

Mechanism: the host process continues to bind the same external ports and serve the same HTTP routes. The compiler's emission generates a **dialer stub** at each external entry point that internally forwards to the extracted service. The stub's behavior depends on the surface category:

- **Call surface:** stub marshals arguments, performs a request-response round-trip to the extracted service, unmarshals the result.
- **Session surface:** stub completes the connection handshake on the host side, dials the extracted service's matching internal endpoint, and bidirectionally bridges bytes between the two connections for the life of the session.
- **Async producer-consumer surface:** stub forwards channel operations onto the chosen wire protocol (e.g., enqueues onto a message queue when the host writes; consumes and delivers when the host reads).

In all cases, the client cannot tell a lift has occurred.

This is a contract guarantee, not an implementation detail. A future emission strategy that broke external-API equivalence (redirecting clients to a different hostname, requiring a protocol upgrade, changing authentication semantics) would not be admissible without superseding this ADR.

### 4. Categories are semantically distinct; wire protocols within a category may be interchangeable

Surface categories are *fundamentally different distribution semantics*, not encodings. A region cannot be lifted across categories: a session-surface region cannot be lifted via a call-surface adapter (no marshalable boundary exists), and a call-surface region cannot be lifted via a session-surface adapter (no connection to proxy). The category is determined by what the code admits, not by user preference.

Within a category, wire protocols may be legitimately swappable. A call-surface region could in principle be lifted via HTTP+JSON, HTTP+protobuf, or gRPC unary — these implement the same synchronous request-response semantics with different encoding costs and dependency surfaces. A session-surface region could in principle be lifted via raw TCP proxy or gRPC bidi streams. An async-producer-consumer region could be lifted onto Kafka, NATS, or Redis Streams.

The compiler's v0 implementation does **not** support swappability. Each surface category has one default wire protocol:

- Call surface → `http-json` (today's adapter).
- Session surface → `stream-proxy` (raw TCP byte-for-byte bridging via the host).
- Async producer-consumer → not yet implemented; admission refuses this category until an adapter lands.

The pragma `transport=` key, when supplied, is treated as a forcing constraint. The compiler must either honor it (when the requested wire protocol is admissible for the derived category) or refuse with a diagnostic. The key never selects a category — only a wire protocol within a category. v0 effectively ignores the key in normal flow because each category has only one admissible wire protocol; the key matters once a category has alternatives.

The future framing, when swappability lands, will be: **the surface-derivation pass produces a set of admissible (category, wire-protocol) pairs for the region; selection within that set is by cost model or user override.** This ADR does not commit to that selection mechanism — it commits only to the framing that makes it possible.

### 5. Cleaning up the existing `transport=` pragma key

Today's `transport=http-json | handler | grpc` is a casualty of decisions 1, 2, and 4 not having been made yet. `http-json` is a wire protocol within the call-surface category. `handler` is a Caddy-specific adapter shape adjacent to the call-surface category. `grpc` is ambiguous — could be unary (call-surface) or streaming (session-surface), and the value alone doesn't disambiguate. Cleanup follows from this ADR but is not an action item *of* this ADR — it lands in the implementation sprint that adds session-surface emission and any future surface-derivation pass.

## Consequences

- The compiler grows a **surface-derivation pass** that runs after admission and before emission. Its inputs are the region's external surface (set of exported entry points + their signatures) and SSA-level evidence about session-ownership and channel-passing patterns. Its output is the region's surface category and (in v0) the single admissible wire protocol within that category.
- Adapter implementations live as siblings under `pkg/compiler/transport/emit/`. Today: `httpjson/` (call-surface). Future: `streamproxy/` (session-surface), `messagequeue/` or per-broker adapters (async producer-consumer). Each adapter declares the surface category it implements; the derivation pass dispatches based on category match.
- The pragma surface stays minimal: `name=` is the load-bearing key; `mode=`, `state=`, `policy=` describe lift intent (when/whether to dispatch). `transport=` is optional and defensive — a forcing constraint within an already-derived category, never a category selector.
- Surface preservation gives a stable verification target. The four-layer e2e (counter delta + oracle equality + transcript parity + fail modes) extends naturally to each category. Transcript parity for a session-surface workload is "the same frames flow to clients in the same order," provable by a recording oracle. Transcript parity for an async producer-consumer is "the same messages are observable on the output channel/queue in an admissible order."
- A region whose surface matches no registered category is refused with a diagnostic that names what the surface looks like and what categories are recognized. Refusal here is a research finding (this surface needs a new category or adapter), not a soft failure — analogous to admission refusal under ADR-0017.
- This ADR does not commit to specific adapter implementations (session-surface stream-proxy, async-producer-consumer message-queue), the boot-path extraction pass, or the multi-symbol patcher. Those are implementation decisions that follow from this framing and will land in their own ADRs as they ship.

## Alternatives considered

- **User specifies surface category or wire protocol as primary input.** Rejected. Inconsistent with the PLOS '25 minimal-annotation contract; pushes implementation choices onto the user; creates user-error modes where the user demands a category the code doesn't admit.
- **One axis: "transport" conflates surface category and wire protocol.** Rejected — this is the status-quo ante that this ADR cleans up. The conflation is what made `transport=grpc` ambiguous (call-shaped or session-shaped?) and what made it tempting to write `transport=stream-proxy` as a primary input. Splitting into two axes resolves both problems.
- **External API may change after a lift.** Rejected. Breaks the contract with existing clients; violates the principle that lifting is a deployment-shape change, not a redesign. A user who wants to redesign their external API is doing something else, not lifting.
- **Implement swappability within categories now.** Rejected for v0. Each category has obvious work to do landing the first adapter; supporting multiple wire protocols per category before any of them ship is premature. The framing reserves room for swappability cleanly when it's earned.

## References

- `pkg/compiler/transport/emit/httpjson/` — existing adapter for function-shaped surfaces.
- `pkg/compiler/surface/` — landed surface-derivation pass. Call surfaces select
  HTTP/JSON; session surfaces select stream-proxy; async producer-consumer
  surfaces refuse with `MLV2_SURFACE_ASYNC_UNSUPPORTED`.
- `pkg/compiler/transport/emit/streamproxy/` — v0 session-surface wire protocol:
  host-side hijack-and-tunnel with raw byte bridging. `gorilla/websocket` is
  used only by tests for byte-parity coverage, not by emitted host stubs.
- `pkg/compiler/pragma_keys.go` — current pragma key validation; `transport=` is `Allowed`, not `Required`.
- ADR-0023 §"Boundary admission rule v0" — the six boundary properties + `lifecycle.execution-profile=sync-short` describing a function-shaped surface.
- ADR-0022 — composite-archetype regions; multi-root regions complicate but do not change the surface-derives-transport rule (each region still has one derived transport for its union surface).
