# pocketbase/M-11 - Email template resolution (`resolveEmailTemplate`)

## Header

- Trace ID: `pocketbase/M-11`
- Project: `pocketbase`
- Region root: `mails/record.go:251`
- Path length: 9
- Source trace: `projects/pocketbase/traces/M-11.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `(*PocketBase).Start` | `method-call-on-concrete-type` | Very-large | Trivial | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 2 | `RunE` | `function-value-in-struct-field` | Large | Serializable | Shared-state | Low | Needs-wrapper | Weak | Feasible |
| 3 | `Serve` | `direct-function-call` | Large | Trivial | Config-only | 0 (estimated) | OK | Anti | Feasible |
| 4 | `NewRouter` | `direct-function-call` | Large | Trivial | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 5 | `bindSettingsApi` | `direct-function-call` | Medium | Serializable | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 6 | `settingsTestEmail` | `http-handler-registration` | Medium | Trivial | Config-only | Low | OK | Strong | Feasible |
| 7 | `(*TestEmailSend).Submit` | `method-call-on-concrete-type` | Small | Trivial | Config-only | 0 (estimated) | OK | Anti | Feasible |
| 8 | `SendRecordPasswordReset` | `direct-function-call` | Small | Trivial | Config-only | 0 (estimated) | OK | Anti | Feasible |
| 9 | `resolveEmailTemplate` | `direct-function-call` | Minimal | Serializable | Config-only | 0 (confirmed) | OK | Anti | Feasible |

## Recommended Cut

Cut at step 9, `resolveEmailTemplate`. This point keeps extraction surface at `Minimal`, avoids hard-gated boundary values, and leaves state reconstruction at `Config-only`. The inspected path reaches `mails/record.go:251`, and this candidate is the latest feasible boundary before the region root or at the root itself.

## Tension Notes

The main tension is natural-boundary signal versus data/state cost. Step 6 has a strong `http-handler-registration` edge, but it scores `Trivial` boundary data and `Config-only` state. Step 9 is later and smaller, so the recommendation prioritizes lift feasibility over edge taxonomy.

## Observations

- The trace contains at least one strong edge signal, but the recommendation also accounts for boundary data and state cost.
- Recommended source evidence: `func resolveEmailTemplate( app core.App, authRecord *core.Record, emailTemplate core.EmailTemplate, placeholders map[string]any, ) (subject string, body string, err error)`.
- PocketBase hook/router edges are natural dispatch points, but app and request event receivers often carry shared framework state.
