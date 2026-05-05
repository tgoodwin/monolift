# listmonk/M-3 - SMTP message send (`Emailer.Push`)

## Header

- Trace ID: `listmonk/M-3`
- Project: `listmonk`
- Region root: `internal/messenger/email/email.go:111`
- Path length: 3
- Source trace: `projects/listmonk/traces/M-3.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `(*Manager).Run` | `goroutine-launch` | Very-large | Proxy-required | Shared-state | Moderate | Infeasible | Anti | Infeasible |
| 2 | `(*Manager).worker` | `goroutine-launch` | Medium | Proxy-required | Shared-state | Moderate | Needs-wrapper | Anti | Feasible-with-proxy |
| 3 | `(*Emailer).Push` | `interface-method-dispatch` | Minimal | Reconstructible | Config-only | 0 (confirmed) | OK | Strong | Feasible |

## Recommended Cut

Cut at step 3, `(*Emailer).Push`. This point keeps extraction surface at `Minimal`, avoids hard-gated boundary values, and leaves state reconstruction at `Config-only`. The inspected path reaches `internal/messenger/email/email.go:111`, and this candidate is the latest feasible boundary before the region root or at the root itself.

## Tension Notes

The recommended cut has no major Pareto conflict: it is late in the path, has preserveable errors, and does not require proxying live request or stream state.

## Observations

- The path includes a goroutine launch; those rows are treated as anti-boundaries unless followed by a queue/channel payload boundary.
- At least one candidate carries proxy-required data, most often an HTTP writer, stream, channel, or queue runtime object.
- The trace contains at least one strong edge signal, but the recommendation also accounts for boundary data and state cost.
- Recommended source evidence: `func (e *Emailer) Push(m models.Message) error`.
- Listmonk's smaller codebase reduces surface-area pressure; boundary data and client reconstruction dominate the decision.
