# pocketbase/M-1 - Image thumbnail generation (`CreateThumb`)

## Header

- Trace ID: `pocketbase/M-1`
- Project: `pocketbase`
- Region root: `tools/filesystem/filesystem.go:489`
- Path length: 11
- Source trace: `projects/pocketbase/traces/M-1.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `(*PocketBase).Start` | `method-call-on-concrete-type` | Very-large | Trivial | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 2 | `closure / dispatch site` | `struct-literal-field-assignment` | Large | Serializable | Shared-state | Low | Needs-wrapper | Weak | Feasible |
| 3 | `closure / dispatch site` | `function-value-via-struct-field` | Large | Serializable | Shared-state | Low | Needs-wrapper | Weak | Feasible |
| 4 | `Serve` | `direct-function-call` | Large | Trivial | Config-only | 0 (estimated) | OK | Anti | Feasible |
| 5 | `NewRouter` | `direct-function-call` | Medium | Trivial | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 6 | `bindFileApi` | `direct-function-call` | Medium | Serializable | Shared-state | 0 (estimated) | Needs-wrapper | Anti | Feasible |
| 7 | `closure / dispatch site` | `method-value-as-callback` | Medium | Infeasible | Shared-state | Low | OK | Weak | Infeasible |
| 8 | `(*fileApi).download` | `function-value-via-struct-field` | Small | Serializable | Client-reconstructible | Low | OK | Weak | Feasible |
| 9 | `(*fileApi).createThumb` | `method-call-on-concrete-type` | Small | Reconstructible | Config-only | 0 (estimated) | OK | Anti | Feasible |
| 10 | `closure / dispatch site` | `closure-capture` | Small | Reconstructible | Config-only | Low | OK | Anti | Feasible |
| 11 | `(*System).CreateThumb` | `method-call-on-concrete-type` | Minimal | Reconstructible | Client-reconstructible | 0 (confirmed) | OK | Anti | Feasible |

## Recommended Cut

Cut at step 11, `(*System).CreateThumb`. This point keeps extraction surface at `Minimal`, avoids hard-gated boundary values, and leaves state reconstruction at `Client-reconstructible`. The inspected path reaches `tools/filesystem/filesystem.go:489`, and this candidate is the latest feasible boundary before the region root or at the root itself.

## Tension Notes

The tradeoff is surface area versus state reconstruction. The cut avoids the large framework prefix, but the remote side must provide `Client-reconstructible` state for the extracted code.

## Observations

- Recommended source evidence: `func (s *System) CreateThumb(originalKey string, thumbKey, thumbSize string) error`.
- PocketBase hook/router edges are natural dispatch points, but app and request event receivers often carry shared framework state.
