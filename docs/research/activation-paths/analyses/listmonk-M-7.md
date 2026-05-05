# listmonk/M-7 - Campaign template compilation (`CompileTemplate`)

## Header

- Trace ID: `listmonk/M-7`
- Project: `listmonk`
- Region root: `models/campaigns.go:138`
- Path length: 4
- Source trace: `projects/listmonk/traces/M-7.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `(*Manager).Run` | `goroutine-launch-of-concrete-method` | Very-large | Proxy-required | Shared-state | Moderate | Infeasible | Anti | Infeasible |
| 2 | `(*Manager).scanCampaigns` | `goroutine-launch-of-concrete-method` | Medium | Proxy-required | Shared-state | Moderate | Infeasible | Anti | Infeasible |
| 3 | `(*Manager).newPipe` | `method-call-on-concrete-type` | Small | Trivial | Client-reconstructible | 0 (estimated) | OK | Anti | Feasible |
| 4 | `(*Campaign).CompileTemplate` | `method-call-on-concrete-type` | Minimal | Trivial | Config-only | 0 (confirmed) | OK | Anti | Feasible |

## Recommended Cut

Cut at step 4, `(*Campaign).CompileTemplate`. This point keeps extraction surface at `Minimal`, avoids hard-gated boundary values, and leaves state reconstruction at `Config-only`. The inspected path reaches `models/campaigns.go:138`, and this candidate is the latest feasible boundary before the region root or at the root itself.

## Tension Notes

The recommended cut has no major Pareto conflict: it is late in the path, has preserveable errors, and does not require proxying live request or stream state.

## Observations

- The path includes a goroutine launch; those rows are treated as anti-boundaries unless followed by a queue/channel payload boundary.
- At least one candidate carries proxy-required data, most often an HTTP writer, stream, channel, or queue runtime object.
- Recommended source evidence: `func (c *Campaign) CompileTemplate(f template.FuncMap) error`.
- Listmonk's smaller codebase reduces surface-area pressure; boundary data and client reconstruction dominate the decision.
