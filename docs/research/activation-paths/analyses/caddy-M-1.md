# caddy/M-1 - Goldmark markdown render (`funcMarkdown`)

## Header

- Trace ID: `caddy/M-1`
- Project: `caddy`
- Region root: `modules/caddyhttp/templates/tplcontext.go:350`
- Path length: 13
- Source trace: `projects/caddy/traces/M-1.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `Main` | `direct-function-call` | Very-large | Serializable | Shared-state | 0 (estimated) | Infeasible | Anti | Infeasible |
| 2 | `cmdRun` | `function-value-in-struct-field` | Large | Serializable | Shared-state | Low | OK | Weak | Feasible |
| 3 | `a.Start()` | `direct-function-call` | Large | Trivial | Config-only | 0 (estimated) | OK | Anti | Feasible |
| 4 | `(*App).Start` | `interface-method-dispatch` | Large | Reconstructible | Shared-state | Low | OK | Strong | Feasible |
| 5 | `(*Server).ServeHTTP` | `interface-typed-struct-field-write` | Large | Proxy-required | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Feasible-with-proxy |
| 6 | `wrapPrimaryRoute` | `interface-method-dispatch` | Medium | Proxy-required | Shared-state | Low | Needs-wrapper | Strong | Feasible-with-proxy |
| 7 | `wrapRoute` | `closure-captured-interface-dispatch` | Medium | Proxy-required | Shared-state | Moderate | Needs-wrapper | Anti | Feasible-with-proxy |
| 8 | `wrapMiddleware` | `closure-captured-interface-dispatch` | Medium | Proxy-required | Shared-state | Moderate | Needs-wrapper | Anti | Feasible-with-proxy |
| 9 | `(*Templates).ServeHTTP` | `interface-method-dispatch` | Medium | Proxy-required | Config-only | Low | OK | Strong | Feasible-with-proxy |
| 10 | `(*Templates).executeTemplate` | `method-call-on-concrete-type` | Small | Proxy-required | Config-only | 0 (estimated) | OK | Anti | Feasible-with-proxy |
| 11 | `(*TemplateContext).executeTemplateInBuffer` | `method-call-on-concrete-type` | Small | Trivial | Config-only | 0 (estimated) | OK | Anti | Feasible |
| 12 | `"markdown": c.funcMarkdown` | `method-value-into-keyed-map` | Small | Trivial | Stateless | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 13 | `(TemplateContext).funcMarkdown` | `reflective-call-via-string-keyed-map` | Minimal | Trivial | Stateless | 0 (confirmed) | OK | Anti | Feasible |

## Recommended Cut

Cut at step 13, `(TemplateContext).funcMarkdown`. This point keeps extraction surface at `Minimal`, avoids hard-gated boundary values, and leaves state reconstruction at `Stateless`. The inspected path reaches `modules/caddyhttp/templates/tplcontext.go:350`, and this candidate is the latest feasible boundary before the region root or at the root itself.

## Tension Notes

The main tension is natural-boundary signal versus data/state cost. Step 9 has a strong `interface-method-dispatch` edge, but it scores `Proxy-required` boundary data and `Config-only` state. Step 13 is later and smaller, so the recommendation prioritizes lift feasibility over edge taxonomy.

## Observations

- At least one candidate carries proxy-required data, most often an HTTP writer, stream, channel, or queue runtime object.
- The trace contains at least one strong edge signal, but the recommendation also accounts for boundary data and state cost.
- Recommended source evidence: `func (TemplateContext) funcMarkdown(input any) (string, error)`.
