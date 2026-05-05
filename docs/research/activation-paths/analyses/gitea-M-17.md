# gitea/M-17 - Syntax highlighting (`RenderFullFile`)

## Header

- Trace ID: `gitea/M-17`
- Project: `gitea`
- Region root: `modules/highlight/highlight.go:124`
- Path length: 12
- Source trace: `projects/gitea/traces/M-17.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `RunMainApp` | `direct-function-call` | Very-large | Trivial | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 2 | `runWeb` | `function-value-in-struct-field` | Very-large | Serializable | Shared-state | Low | OK | Weak | Feasible |
| 3 | `serveInstalled` | `direct-function-call` | Very-large | Trivial | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 4 | `NormalRoutes` | `direct-function-call` | Large | Serializable | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 5 | `Routes` | `direct-function-call` | Large | Serializable | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 6 | `registerWebRoutes` | `direct-function-call` | Medium | Serializable | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 7 | `Home` | `http-handler-registration` | Medium | Serializable | Client-reconstructible | Low | Needs-wrapper | Strong | Feasible |
| 8 | `prepareToRenderDirOrFile` | `direct-function-call` | Medium | Infeasible | Client-reconstructible | 0 (estimated) | Needs-wrapper | Anti | Infeasible |
| 9 | `closure / dispatch site` | `function-value-in-slice` | Small | Infeasible | Client-reconstructible | Low | Needs-wrapper | Weak | Infeasible |
| 10 | `prepareFileView` | `direct-function-call` | Small | Serializable | Client-reconstructible | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 11 | `handleFileViewRenderSource` | `direct-function-call` | Small | Proxy-required | Client-reconstructible | 0 (estimated) | Needs-wrapper | Anti | Feasible-with-proxy |
| 12 | `RenderFullFile` | `direct-function-call` | Minimal | Serializable | Config-only | 0 (confirmed) | Needs-wrapper | Anti | Feasible |

## Recommended Cut

Cut at step 12, `RenderFullFile`. This point keeps extraction surface at `Minimal`, avoids hard-gated boundary values, and leaves state reconstruction at `Config-only`. The inspected path reaches `modules/highlight/highlight.go:124`, and this candidate is the latest feasible boundary before the region root or at the root itself.

## Tension Notes

The main tension is natural-boundary signal versus data/state cost. Step 7 has a strong `http-handler-registration` edge, but it scores `Serializable` boundary data and `Client-reconstructible` state. Step 12 is later and smaller, so the recommendation prioritizes lift feasibility over edge taxonomy.

## Observations

- At least one candidate carries proxy-required data, most often an HTTP writer, stream, channel, or queue runtime object.
- The trace contains at least one strong edge signal, but the recommendation also accounts for boundary data and state cost.
- Recommended source evidence: `func RenderFullFile(fileName, language string, code []byte) ([]template.HTML, string)`.
