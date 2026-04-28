# Target Annotation: Caddy

## Synthesis
- **Dominant archetypes**: HTTP Handler (Modules), Singleton Actor (Core/Registry), Replicated Stateless Service (Config Adapters).
- **The AUTO set**:
    - `modules/caddyhttp/reverseproxy.Handler`: already admitted.
    - `modules/caddyhttp/server.Server`: HTTP Handler.
- **Hardest ambiguities**:
    - `caddy.UsagePool`: It's a global registry for resource tracking. Classic Singleton Actor candidate, but very sensitive to latency.
- **Most important evidence gaps**:
    - Detection of "registry" patterns where objects are purely identified by string keys (Sharded Stateful Service candidate).

---

## Annotations

### Subsystem: HTTP Reverse Proxy
- **owned directories**: `modules/caddyhttp/reverseproxy/`
- **region or operation identity**: `github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy.Handler` / struct
- **admitted or refused**: ADMITTED
- **triage**: ADMITTED
- **proposed archetype**: HTTP Handler
- **proposed candidate state class**: `stateless-http`
- **proposed transform**: Standard ingress.
- **competing archetypes**: None.
- **evidence signals seen**: `context-plus-json` shape; `net/http` compatibility.
- **missing evidence**: None.
- **file references**: `modules/caddyhttp/reverseproxy/reverseproxy.go:53`

### Subsystem: Usage Pool
- **owned directories**: root
- **region or operation identity**: `github.com/caddyserver/caddy/v2.UsagePool` / struct
- **admitted or refused**: REFUSED (likely, uses sync.Map or similar)
- **triage**: AUTO
- **proposed archetype**: Singleton Actor
- **proposed candidate state class**: `singleton-mutable-mutex`
- **proposed transform**: Global resource tracker service.
- **competing archetypes**: Sharded Stateful Service (if sharded by resource name).
- **evidence signals seen**: Mutex-guarded map of resources.
- **missing evidence**: Proof that resource names are the only access path.
- **file references**: `usagepool.go:28`

### Subsystem: Config Adapters
- **owned directories**: `caddyconfig/`
- **region or operation identity**: `github.com/caddyserver/caddy/v2/caddyconfig`
- **admitted or refused**: ADMITTED (pure transforms)
- **triage**: ADMITTED
- **proposed archetype**: Replicated Stateless Service
- **proposed candidate state class**: `stateless`
- **proposed transform**: N replicas behind LB.
- **competing archetypes**: None.
- **evidence signals seen**: Pure function signatures `([]byte) -> ([]byte, error)`.
- **missing evidence**: None.
- **file references**: `caddyconfig/configadapters.go:1`
