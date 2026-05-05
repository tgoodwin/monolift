# listmonk/M-1 - Per-recipient campaign message render

## Header

- Trace ID: `listmonk/M-1`
- Project: `listmonk`
- Region root: `internal/manager/message.go:13`
- Path length: 4
- Source trace: `projects/listmonk/traces/M-1.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `(*Manager).Run` | `goroutine-launch` | Very-large | Proxy-required | Shared-state | Moderate | Infeasible | Anti | Infeasible |
| 2 | `(*pipe).NextSubscribers` | `channel-receive-to-concrete-method-call` | Medium | Trivial | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 3 | `(*pipe).newMessage` | `method-call-on-concrete-type` | Small | Trivial | Client-reconstructible | 0 (estimated) | OK | Anti | Feasible |
| 4 | `(*Manager).NewCampaignMessage` | `method-call-on-concrete-type` | Minimal | Trivial | Client-reconstructible | 0 (confirmed) | OK | Anti | Feasible |

## Recommended Cut

Cut at step 4, `(*Manager).NewCampaignMessage`. This point keeps extraction surface at `Minimal`, avoids hard-gated boundary values, and leaves state reconstruction at `Client-reconstructible`. The inspected path reaches `internal/manager/message.go:13`, and this candidate is the latest feasible boundary before the region root or at the root itself.

## Tension Notes

The tradeoff is surface area versus state reconstruction. The cut avoids the large framework prefix, but the remote side must provide `Client-reconstructible` state for the extracted code.

## Observations

- The path includes a goroutine launch; those rows are treated as anti-boundaries unless followed by a queue/channel payload boundary.
- At least one candidate carries proxy-required data, most often an HTTP writer, stream, channel, or queue runtime object.
- Recommended source evidence: `func (m *Manager) NewCampaignMessage(c *models.Campaign, s models.Subscriber) (CampaignMessage, error)`.
- Listmonk's smaller codebase reduces surface-area pressure; boundary data and client reconstruction dominate the decision.
