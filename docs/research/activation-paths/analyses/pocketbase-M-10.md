# pocketbase/M-10 - Record relation expansion (`ExpandRecords`)

## Header

- Trace ID: `pocketbase/M-10`
- Project: `pocketbase`
- Region root: `core/record_query_expand.go:34`
- Path length: 9
- Source trace: `projects/pocketbase/traces/M-10.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `(*PocketBase).Start` | `method-call-on-concrete-type` | Very-large | Trivial | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 2 | `closure / dispatch site` | `function-value-in-struct-field` | Large | Serializable | Shared-state | Low | Needs-wrapper | Weak | Feasible |
| 3 | `Serve` | `direct-function-call` | Large | Trivial | Config-only | 0 (estimated) | OK | Anti | Feasible |
| 4 | `NewRouter` | `direct-function-call` | Large | Trivial | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 5 | `bindRecordCrudApi` | `direct-function-call` | Medium | Serializable | Client-reconstructible | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 6 | `recordsList` | `http-handler-registration` | Medium | Trivial | Client-reconstructible | Low | OK | Strong | Feasible |
| 7 | `EnrichRecords` | `closure-passed-as-callback-arg` | Small | Serializable | Client-reconstructible | Low | OK | Anti | Feasible |
| 8 | `defaultEnrichRecords` | `closure-passed-as-callback-arg` | Small | Serializable | Client-reconstructible | Low | OK | Anti | Feasible |
| 9 | `(*BaseApp).ExpandRecords` | `interface-method-dispatch` | Minimal | Reconstructible | Client-reconstructible | 0 (confirmed) | Needs-wrapper | Strong | Feasible |

## Recommended Cut

Cut at step 9, `(*BaseApp).ExpandRecords`. This point keeps extraction surface at `Minimal`, avoids hard-gated boundary values, and leaves state reconstruction at `Client-reconstructible`. The inspected path reaches `core/record_query_expand.go:34`, and this candidate is the latest feasible boundary before the region root or at the root itself.

## Tension Notes

The tradeoff is surface area versus state reconstruction. The cut avoids the large framework prefix, but the remote side must provide `Client-reconstructible` state for the extracted code.

## Observations

- The trace contains at least one strong edge signal, but the recommendation also accounts for boundary data and state cost.
- Recommended source evidence: `func (app *BaseApp) ExpandRecords(records []*Record, expands []string, optFetchFunc ExpandFetchFunc) map[string]error`.
- PocketBase hook/router edges are natural dispatch points, but app and request event receivers often carry shared framework state.
