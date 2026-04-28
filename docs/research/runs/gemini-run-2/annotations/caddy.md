# Caddy Annotation & Coverage Ledger

## Target Information
- **Name**: Caddy
- **Total Go Files**: 306

## Coverage Ledger

| Bundle | Subsystem | Directories | File Count | Status |
| :--- | :--- | :--- | :--- | :--- |
| **C-ALL** | full | `.` | 306 | DONE |

## Annotations

### C-RP-001: Caddy Reverse Proxy Handler
- **subsystem**: reverseproxy
- **owned directories**: `modules/caddyhttp/reverseproxy`
- **region or operation identity**: `github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy:Handler` (type)
- **admitted or refused**: refused
- **triage**: AUTO
- **proposed archetype**: singleton-actor
- **proposed candidate state class**: immutable-captured-config
- **proposed transform**: Wrap `Handler` in a service; expose `ServeHTTP` via HTTP/gRPC; serialize/proxy hijacked connections.
- **competing archetypes considered**: stateless (rejected because it carries state like `Upstreams`)
- **evidence signals seen**: `sync.Mutex`, `sync.RWMutex`, `sync.Once`, `http.Handler` interface compliance.
- **missing evidence**: Automated handling of `net.Conn` hijacking and `http.ResponseWriter` wrapper serialization.
- **file references**: `evaluation/caddy/modules/caddyhttp/reverseproxy/reverseproxy.go:101`
