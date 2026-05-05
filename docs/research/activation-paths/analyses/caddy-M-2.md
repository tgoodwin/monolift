# caddy/M-2 - Buffered template execution (`executeTemplate`)

## Header

- Trace ID: `caddy/M-2`
- Project: `caddy`
- Region root: `modules/caddyhttp/templates/templates.go:455`
- Path length: 12
- Source trace: `projects/caddy/traces/M-2.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `Main` | `direct-function-call` | Very-large | Serializable | Shared-state | 0 (estimated) | Infeasible | Anti | Infeasible |
| 2 | `cmdRun` | `function-value-in-struct-field` | Large | Serializable | Shared-state | Low | OK | Weak | Feasible |
| 3 | `Load` | `direct-function-call` | Large | Serializable | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 4 | `(*App).Start` | `interface-method-dispatch` | Large | Reconstructible | Shared-state | Low | OK | Strong | Feasible |
| 5 | `(*Server).ServeHTTP` | `interface-method-dispatch` | Large | Proxy-required | Shared-state | Low | Needs-wrapper | Strong | Feasible-with-proxy |
| 6 | `(*Server).serveHTTP` | `method-call-on-concrete-type` | Medium | Proxy-required | Shared-state | 0 (estimated) | OK | Anti | Feasible-with-proxy |
| 7 | `wrapPrimaryRoute` | `interface-method-dispatch` | Medium | Proxy-required | Shared-state | Low | Needs-wrapper | Strong | Feasible-with-proxy |
| 8 | `(*Server).enforcementHandler` | `direct-function-call` | Medium | Proxy-required | Shared-state | Moderate | OK | Anti | Feasible-with-proxy |
| 9 | `wrapRoute` | `interface-method-dispatch` | Small | Proxy-required | Shared-state | Low | Needs-wrapper | Strong | Feasible-with-proxy |
| 10 | `wrapMiddleware` | `interface-method-dispatch` | Small | Proxy-required | Shared-state | Moderate | Needs-wrapper | Strong | Feasible-with-proxy |
| 11 | `(*Templates).ServeHTTP` | `interface-method-dispatch` | Small | Proxy-required | Config-only | 0 (estimated) | OK | Strong | Feasible-with-proxy |
| 12 | `(*Templates).executeTemplate` | `method-call-on-concrete-type` | Minimal | Proxy-required | Config-only | 0 (confirmed) | OK | Anti | Feasible-with-proxy |

## Recommended Cut

Cut at step 12, `(*Templates).executeTemplate`. This is the deepest practical point on the path, but it still requires a proxy because the inspected source signature or dispatch site carries Proxy-required boundary data. The recommendation accepts that cost to avoid extracting the larger bootstrap/router surface above it.

## Tension Notes

The main tension is natural-boundary signal versus data/state cost. Step 11 has a strong `interface-method-dispatch` edge, but it scores `Proxy-required` boundary data and `Config-only` state. Step 12 is later and smaller, so the recommendation prioritizes lift feasibility over edge taxonomy.

## Observations

- At least one candidate carries proxy-required data, most often an HTTP writer, stream, channel, or queue runtime object.
- The trace contains at least one strong edge signal, but the recommendation also accounts for boundary data and state cost.
- Recommended source evidence: `func (t *Templates) executeTemplate(rr caddyhttp.ResponseRecorder, r *http.Request) error`.
