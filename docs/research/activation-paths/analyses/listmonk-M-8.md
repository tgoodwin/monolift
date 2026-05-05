# listmonk/M-8 - POP3 bounce mailbox scan (`POP.Scan`)

## Header

- Trace ID: `listmonk/M-8`
- Project: `listmonk`
- Region root: `internal/bounce/mailbox/pop.go:79`
- Path length: 3
- Source trace: `projects/listmonk/traces/M-8.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `(*Manager).Run` | `goroutine-launch` | Very-large | Proxy-required | Shared-state | Moderate | Infeasible | Anti | Infeasible |
| 2 | `(*Manager).runMailboxScanner` | `goroutine-launch` | Medium | Proxy-required | Shared-state | Moderate | Needs-wrapper | Anti | Feasible-with-proxy |
| 3 | `(*POP).Scan` | `interface-method-dispatch` | Minimal | Proxy-required | Client-reconstructible | 0 (confirmed) | OK | Strong | Feasible-with-proxy |

## Recommended Cut

Cut at step 3, `(*POP).Scan`. This is the deepest practical point on the path, but it still requires a proxy because the inspected source signature or dispatch site carries Proxy-required boundary data. The recommendation accepts that cost to avoid extracting the larger bootstrap/router surface above it.

## Tension Notes

The dominant tension is that all competitive late cuts carry a live stream, writer, channel, or request-scoped object. The recommended cut minimizes surface area while explicitly accepting a proxy requirement.

## Observations

- The path includes a goroutine launch; those rows are treated as anti-boundaries unless followed by a queue/channel payload boundary.
- At least one candidate carries proxy-required data, most often an HTTP writer, stream, channel, or queue runtime object.
- The trace contains at least one strong edge signal, but the recommendation also accounts for boundary data and state cost.
- Recommended source evidence: `func (p *POP) Scan(limit int, ch chan models.Bounce) error`.
- Listmonk's smaller codebase reduces surface-area pressure; boundary data and client reconstruction dominate the decision.
