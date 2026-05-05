# listmonk/M-2 - HTTP webhook delivery (`Postback.Push`)

## Header

- Trace ID: `listmonk/M-2`
- Project: `listmonk`
- Region root: `internal/messenger/postback/postback.go:97`
- Path length: 4
- Source trace: `projects/listmonk/traces/M-2.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `(*Manager).Run` | `goroutine-launch-of-concrete-method` | Very-large | Proxy-required | Shared-state | Moderate | Infeasible | Anti | Infeasible |
| 2 | `(*Manager).worker` | `goroutine-launch-of-concrete-method` | Medium | Proxy-required | Shared-state | Moderate | Infeasible | Anti | Infeasible |
| 3 | `receive` | `channel-typed-flow` | Small | Serializable | Client-reconstructible | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 4 | `(*Postback).Push` | `map-lookup-interface-method-dispatch` | Minimal | Trivial | Client-reconstructible | 0 (confirmed) | OK | Strong | Feasible |

## Recommended Cut

Cut at step 4, `(*Postback).Push`. This point keeps extraction surface at `Minimal`, avoids hard-gated boundary values, and leaves state reconstruction at `Client-reconstructible`. The inspected path reaches `internal/messenger/postback/postback.go:97`, and this candidate is the latest feasible boundary before the region root or at the root itself.

## Tension Notes

The tradeoff is surface area versus state reconstruction. The cut avoids the large framework prefix, but the remote side must provide `Client-reconstructible` state for the extracted code.

## Observations

- The path includes a goroutine launch; those rows are treated as anti-boundaries unless followed by a queue/channel payload boundary.
- At least one candidate carries proxy-required data, most often an HTTP writer, stream, channel, or queue runtime object.
- The trace contains at least one strong edge signal, but the recommendation also accounts for boundary data and state cost.
- Recommended source evidence: `func (p *Postback) Push(m models.Message) error`.
- Listmonk's smaller codebase reduces surface-area pressure; boundary data and client reconstruction dominate the decision.
