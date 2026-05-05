# caddy/M-3 - Bcrypt/Argon2 password verify (`correctPassword`)

## Header

- Trace ID: `caddy/M-3`
- Project: `caddy`
- Region root: `(HTTPBasicAuth).correctPassword` at `modules/caddyhttp/caddyauth/basicauth.go:165`
- Path length: 11
- Source trace: `projects/caddy/traces/M-3.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `caddycmd.Main` | `direct-function-call` | Very-large | Trivial | Shared-state | 0 (estimated) | Infeasible | Anti | Infeasible |
| 2 | `cmdRun` | `function-value-in-struct-field` | Very-large | Serializable | Shared-state | Low | OK | Weak | Feasible |
| 3 | `(*App).Start` | `interface-method-dispatch` | Very-large | Reconstructible | Shared-state | Moderate | OK | Strong | Feasible |
| 4 | `(*Server).ServeHTTP` | `library-callback-through-interface-field` | Large | Proxy-required | Shared-state | Low | Needs-wrapper | Strong | Feasible-with-proxy |
| 5 | `(*Server).serveHTTP` | `method-call-on-concrete-type` | Large | Proxy-required | Shared-state | Low | OK | Anti | Feasible-with-proxy |
| 6 | `(*Server).enforcementHandler` | `function-value-in-struct-field` | Medium | Proxy-required | Shared-state | Moderate | OK | Weak | Feasible-with-proxy |
| 7 | `wrapRoute` closure | `closure-capture-of-interface-value` | Medium | Proxy-required | Shared-state | Moderate | OK | Anti | Feasible-with-proxy |
| 8 | `wrapMiddleware` closure | `closure-capture-of-interface-value` | Medium | Proxy-required | Shared-state | Moderate | OK | Anti | Feasible-with-proxy |
| 9 | `(Authentication).ServeHTTP` | `interface-method-dispatch` | Medium | Proxy-required | Client-reconstructible | Low | OK | Strong | Feasible-with-proxy |
| 10 | `(HTTPBasicAuth).Authenticate` | `interface-method-dispatch` | Small | Proxy-required | Client-reconstructible | 0 (estimated) | OK | Strong | Feasible-with-proxy |
| 11 | `(HTTPBasicAuth).correctPassword` | `method-call-on-concrete-type` | Minimal | Serializable | Config-only | 0 (confirmed) | OK | Anti | Feasible |

## Recommended Cut

Cut at step 11, `(HTTPBasicAuth).correctPassword`, if the cache is either disabled or reconstructed per remote instance. The boundary can be represented as account hash material plus plaintext bytes returning `(bool, error)`, the surface is minimal, and no HTTP connection object crosses the network. Step 10 has the stronger interface-dispatch signal, but the actual source signature includes `http.ResponseWriter`, and failure handling writes the authentication challenge through that writer, so it is proxy-required unless the compiler synthesizes a narrower verification-only boundary.

## Tension Notes

The main tension is edge alignment versus boundary data. Step 10 is the natural authentication-provider contract, but it carries `http.ResponseWriter` and request state. Step 11 is a concrete method call with weaker architectural signal, yet it eliminates the HTTP proxy and leaves only hasher configuration plus optional cache state to reconstruct. That boundary is materially easier to lift.

## Observations

- Calibration disagreement with the brief: the brief scores step 10 boundary data as `*Request` plus `(User, bool, error)`, but `Authenticate(w http.ResponseWriter, req *http.Request)` uses the writer in `promptForCredentials`; I therefore score it `Proxy-required`.
- The deepest method is almost a pure leaf, but `HTTPBasicAuth` owns a `Comparer` interface and optional `Cache` containing a mutex, map, and `singleflight.Group`. The clean extraction shape is to reconstruct the hasher and keep cache local to the remote side.
- Caddy's middleware chain supplies several strong or weak dispatch points, but the shared `http.ResponseWriter`/`*http.Request` pair makes most mid-chain cuts proxy boundaries rather than ordinary RPC boundaries.
