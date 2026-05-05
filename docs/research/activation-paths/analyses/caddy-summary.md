# Caddy Summary

## Scope

- Traces analyzed: `caddy/M-1`, `M-2`, `M-3`, `M-4`, `M-5`, `M-7`
- Codebase size: ~93k LOC
- Dominant path shape: command bootstrap into app/module startup, then either HTTP middleware dispatch or TLS automation.

## Architecture Pattern

Caddy's HTTP traces share a long prefix through `Server.ServeHTTP`, `serveHTTP`, route wrapping, and middleware wrapping. Those edges often look attractive because they are interface dispatches or handler-field dispatches, but the source signatures consistently carry `http.ResponseWriter`, `*http.Request`, or `caddyhttp.Handler`. As a result, the middleware-chain boundary is usually `Feasible-with-proxy`, not an ordinary RPC cut.

## Cut Placement Findings

| Trace | Recommended pattern | Summary |
|---|---|---|
| `caddy/M-1` | Pure Leaf | `funcMarkdown` is a stateless markdown renderer reached through template function dispatch. |
| `caddy/M-2` | Middleware/Template Split | `executeTemplate` is the deepest target but still carries response recorder and request state. |
| `caddy/M-3` | Pure Leaf | `correctPassword` avoids the HTTP writer that appears at the authentication interface. |
| `caddy/M-4` | Reconstructible TLS Leaf | `InternalIssuer.Issue` carries CSR/context data and reconstructs CA issuer state. |
| `caddy/M-5` | Middleware Split | `Encode.ServeHTTP` is a natural middleware boundary but requires a response-writer proxy. |
| `caddy/M-7` | Filesystem Leaf | `loadDirectoryContents` is later than the HTTP handler and avoids proxying the writer. |

## Shared Prefix Impact

The shared HTTP prefix inflates shallow-cut surface area and repeatedly introduces proxy-required request/response values. `interface-method-dispatch` edges in the middleware chain reliably signal replaceable components, but they do not reliably signal serializable boundaries because the handler contract itself is stream/request oriented.

## Synthesis Notes

- Caddy confirms the `Middleware Split` archetype: strong edge alignment, but proxy-required HTTP data.
- The best Caddy cuts are usually below middleware once the code turns into algorithmic work (`funcMarkdown`, `correctPassword`) or concrete resource work (`loadDirectoryContents`, `Issue`).
- Edge alignment alone would choose too high in the middleware chain; boundary-data complexity is the decisive tiebreaker.
