# caddy/M-5 - HTTP response compression (`Encode.ServeHTTP`)

## Header

- Trace ID: `caddy/M-5`
- Project: `caddy`
- Region root: `modules/caddyhttp/encode/encode.go:154`
- Path length: 10
- Source trace: `projects/caddy/traces/M-5.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `Main` | `direct-function-call` | Very-large | Serializable | Shared-state | 0 (estimated) | Infeasible | Anti | Infeasible |
| 2 | `cmdRun` | `function-value-in-struct-field` | Large | Serializable | Shared-state | Low | OK | Weak | Feasible |
| 3 | `Load` | `direct-function-call` | Large | Serializable | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 4 | `(*App).Start` | `interface-method-dispatch` | Large | Reconstructible | Shared-state | Low | OK | Strong | Feasible |
| 5 | `(*Server).ServeHTTP` | `struct-literal-field-assignment` + `goroutine-launch` + `interface-method-dispatch` | Medium | Proxy-required | Shared-state | Moderate | Infeasible | Strong | Infeasible |
| 6 | `(*Server).serveHTTP` | `method-call-on-concrete-type` | Medium | Proxy-required | Shared-state | 0 (estimated) | OK | Anti | Feasible-with-proxy |
| 7 | `wrapPrimaryRoute` | `interface-method-dispatch` | Medium | Proxy-required | Shared-state | Low | Needs-wrapper | Strong | Feasible-with-proxy |
| 8 | `wrapRoute` | `closure-capture` + `interface-method-dispatch` | Small | Proxy-required | Shared-state | Moderate | Needs-wrapper | Strong | Feasible-with-proxy |
| 9 | `wrapMiddleware` | `closure-capture` + `interface-method-dispatch` | Small | Proxy-required | Shared-state | Moderate | Needs-wrapper | Strong | Feasible-with-proxy |
| 10 | `(*Encode).ServeHTTP` | `interface-method-dispatch` | Minimal | Proxy-required | Shared-state | 0 (estimated) | OK | Strong | Feasible-with-proxy |

## Recommended Cut

Cut at step 10, `(*Encode).ServeHTTP`. This is the deepest practical point on the path, but it still requires a proxy because the inspected source signature or dispatch site carries Proxy-required boundary data. The recommendation accepts that cost to avoid extracting the larger bootstrap/router surface above it.

## Tension Notes

The dominant tension is that all competitive late cuts carry a live stream, writer, channel, or request-scoped object. The recommended cut minimizes surface area while explicitly accepting a proxy requirement.

## Observations

- The path includes a goroutine launch; those rows are treated as anti-boundaries unless followed by a queue/channel payload boundary.
- At least one candidate carries proxy-required data, most often an HTTP writer, stream, channel, or queue runtime object.
- The trace contains at least one strong edge signal, but the recommendation also accounts for boundary data and state cost.
- Recommended source evidence: `func (enc *Encode) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error`.
