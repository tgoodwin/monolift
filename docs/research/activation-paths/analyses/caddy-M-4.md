# caddy/M-4 - Internal CA cert issuance (`InternalIssuer.Issue`)

## Header

- Trace ID: `caddy/M-4`
- Project: `caddy`
- Region root: `modules/caddytls/internalissuer.go:103`
- Path length: 10
- Source trace: `projects/caddy/traces/M-4.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `caddycmd.Main` | `direct-function-call` | Very-large | Serializable | Shared-state | 0 (estimated) | Infeasible | Anti | Infeasible |
| 2 | `closure / dispatch site` | `init-populated-registry` | Large | Serializable | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 3 | `closure / dispatch site` | `function-value-in-struct-field` | Large | Infeasible | Shared-state | Low | OK | Weak | Infeasible |
| 4 | `cmdRun` | `closure-captured-function-call` | Large | Trivial | Shared-state | Low | OK | Anti | Feasible |
| 5 | `Load` | `direct-function-call` | Medium | Serializable | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 6 | `run` | `direct-function-call` (chained)` | Medium | Trivial | Config-only | 0 (estimated) | OK | Anti | Feasible |
| 7 | `(*TLS).Start` | `interface-method-dispatch` | Medium | Trivial | Client-reconstructible | Low | OK | Strong | Feasible |
| 8 | `(*TLS).Manage` | `method-call-on-concrete-type` | Small | Serializable | Client-reconstructible | 0 (estimated) | OK | Anti | Feasible |
| 9 | `closure / dispatch site` | `method-call-on-concrete-type` | Small | Serializable | Client-reconstructible | 0 (estimated) | OK | Anti | Feasible |
| 10 | `(InternalIssuer).Issue` | `interface-method-dispatch` | Minimal | Serializable | Config-only | 0 (confirmed) | OK | Strong | Feasible |

## Recommended Cut

Cut at step 10, `(InternalIssuer).Issue`. This point keeps extraction surface at `Minimal`, avoids hard-gated boundary values, and leaves state reconstruction at `Config-only`. The inspected path reaches `modules/caddytls/internalissuer.go:103`, and this candidate is the latest feasible boundary before the region root or at the root itself.

## Tension Notes

The recommended cut has no major Pareto conflict: it is late in the path, has preserveable errors, and does not require proxying live request or stream state.

## Observations

- The trace contains at least one strong edge signal, but the recommendation also accounts for boundary data and state cost.
- Recommended source evidence: `func (iss InternalIssuer) Issue(ctx context.Context, csr *x509.CertificateRequest) (*certmagic.IssuedCertificate, error)`.
