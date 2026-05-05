# miniflux/M-14 - OAuth2 token exchange+profile (`googleProvider.Profile`)

## Header

- Trace ID: `miniflux/M-14`
- Project: `miniflux`
- Region root: `internal/oauth2/google.go:57`
- Path length: 7
- Source trace: `projects/miniflux/traces/M-14.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `Parse` | `direct-function-call` | Very-large | Serializable | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 2 | `startDaemon` | `direct-function-call` | Large | Reconstructible | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 3 | `StartWebServer` | `direct-function-call` | Large | Reconstructible | Client-reconstructible | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 4 | `newRouter` | `struct-literal-field-assignment` | Medium | Reconstructible | Client-reconstructible | Low | Needs-wrapper | Weak | Feasible |
| 5 | `Serve` | `http-handler-registration` | Medium | Reconstructible | Client-reconstructible | Low | Needs-wrapper | Strong | Feasible |
| 6 | `(*handler).oauth2Callback` | `method-value-handler-registration` | Small | Proxy-required | Config-only | Moderate | Needs-wrapper | Strong | Feasible-with-proxy |
| 7 | `(*googleProvider).Profile` | `interface-method-dispatch` | Minimal | Serializable | Client-reconstructible | 0 (confirmed) | OK | Strong | Feasible |

## Recommended Cut

Cut at step 7, `(*googleProvider).Profile`. This point keeps extraction surface at `Minimal`, avoids hard-gated boundary values, and leaves state reconstruction at `Client-reconstructible`. The inspected path reaches `internal/oauth2/google.go:57`, and this candidate is the latest feasible boundary before the region root or at the root itself.

## Tension Notes

The tradeoff is surface area versus state reconstruction. The cut avoids the large framework prefix, but the remote side must provide `Client-reconstructible` state for the extracted code.

## Observations

- At least one candidate carries proxy-required data, most often an HTTP writer, stream, channel, or queue runtime object.
- The trace contains at least one strong edge signal, but the recommendation also accounts for boundary data and state cost.
- Recommended source evidence: `func (g *googleProvider) Profile(ctx context.Context, code, codeVerifier string) (*UserProfile, error)`.
