# pocketbase/M-4 - Backup archive zip writer (`archive.Create`)

## Header

- Trace ID: `pocketbase/M-4`
- Project: `pocketbase`
- Region root: `tools/archive/create.go:18`
- Path length: 10
- Source trace: `projects/pocketbase/traces/M-4.synthesis.md`

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | State reconstruction | Callbacks | Error semantics | Edge alignment | Feasibility |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `(*PocketBase).Execute` | `method-call-on-concrete-type` | Very-large | Trivial | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 2 | `closure / dispatch site` | `goroutine-launched-closure` | Large | Proxy-required | Shared-state | Moderate | OK | Anti | Feasible-with-proxy |
| 3 | `RunE` | `function-value-in-struct-field` | Large | Serializable | Shared-state | Low | Needs-wrapper | Weak | Feasible |
| 4 | `(*plugin).update` | `method-call-on-concrete-type` | Large | Trivial | Shared-state | 0 (estimated) | OK | Anti | Feasible |
| 5 | `(*BaseApp).CreateBackup` | `interface-method-dispatch` | Medium | Reconstructible | Client-reconstructible | Low | OK | Strong | Feasible |
| 6 | `closure / dispatch site` | `closure-passed-as-variadic-argument` | Medium | Reconstructible | Client-reconstructible | Low | OK | Anti | Feasible |
| 7 | `closure / dispatch site` | `interface-method-dispatch` + `closure-passed-as-argument` | Medium | Reconstructible | Client-reconstructible | Low | OK | Strong | Feasible |
| 8 | `closure / dispatch site` | `interface-method-dispatch` + `closure-passed-as-argument` | Small | Reconstructible | Client-reconstructible | Low | OK | Strong | Feasible |
| 9 | `Create` | `direct-function-call` | Small | Trivial | Client-reconstructible | 0 (estimated) | OK | Anti | Feasible |

## Recommended Cut

Cut at step 9, `Create`. This point keeps extraction surface at `Small`, avoids hard-gated boundary values, and leaves state reconstruction at `Client-reconstructible`. The inspected path reaches `tools/archive/create.go:18`, and this candidate is the latest feasible boundary before the region root or at the root itself.

## Tension Notes

The main tension is natural-boundary signal versus data/state cost. Step 8 has a strong `interface-method-dispatch` + `closure-passed-as-argument` edge, but it scores `Reconstructible` boundary data and `Client-reconstructible` state. Step 9 is later and smaller, so the recommendation prioritizes lift feasibility over edge taxonomy.

## Observations

- The path includes a goroutine launch; those rows are treated as anti-boundaries unless followed by a queue/channel payload boundary.
- At least one candidate carries proxy-required data, most often an HTTP writer, stream, channel, or queue runtime object.
- The trace contains at least one strong edge signal, but the recommendation also accounts for boundary data and state cost.
- Recommended source evidence: `func Create(src string, dest string, skipPaths ...string) error`.
- PocketBase hook/router edges are natural dispatch points, but app and request event receivers often carry shared framework state.
