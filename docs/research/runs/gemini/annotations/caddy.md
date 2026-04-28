# Caddy Annotations

## Target Synthesis
Caddy is the flagship target for the **Singleton Actor** archetype. Its module system (registry-based) creates clearly bounded stateful objects (Handlers, Transports, Upstreams) that are currently refused due to shared mutable state (mutexes, atomics) and channel usage. However, because each instance is a well-defined "module," it can be lifted into a remote service where its state is local and serialized, while `ServeHTTP` is exposed as the distributed interface.

## Annotations

### reverseproxy.Handler
- **Subsystem**: `ingress`
- **Owned Directories**: `modules/caddyhttp/reverseproxy`
- **Region or Operation Identity**: `github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy.Handler:type`
- **Admitted or Refused**: Refused
- **Triage**: AUTO
- **Proposed Archetype**: Singleton Actor
- **Proposed Transform**: Registry-keyed singleton service; `ServeHTTP` forwarded.
- **Evidence Signals Seen**: `syncPrimitiveRule`, `registry-key:http.handlers.reverse_proxy`, `boundary.no-streaming-values`.
- **Missing Evidence**: Static proof of registry mapping.
- **File References**: `modules/caddyhttp/reverseproxy/reverseproxy.go:101`

### UpstreamPool (via Host)
- **Subsystem**: `ingress`
- **Owned Directories**: `modules/caddyhttp/reverseproxy`
- **Region or Operation Identity**: `github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy.Host:type`
- **Admitted or Refused**: Refused
- **Triage**: AUTO
- **Proposed Archetype**: Singleton Actor
- **Proposed Transform**: Shared host-state service (circuit breaker / health check status).
- **Evidence Signals Seen**: `atomic.Int64` for request counts and failures.
- **Missing Evidence**: None.
- **File References**: `modules/caddyhttp/reverseproxy/hosts.go:34`
