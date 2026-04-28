# SPRINT-0027 Budgeted Frontier Matrix

Date: 2026-04-28

This matrix summarizes the Mattermost budget-partitioned boundary-frontier
diagnostic ladder.

Raw artifacts live beside this file as:

- `SPRINT-0027-budgeted-frontier-small.{json,stderr,meta,summary.json}`
- `SPRINT-0027-budgeted-frontier-medium.{json,stderr,meta,summary.json}`
- `SPRINT-0027-budgeted-frontier-large.{json,stderr,meta,summary.json}`
- `SPRINT-0027-budgeted-frontier-exploratory-cut.md`

All completed rows used:

```sh
GOWORK=/Users/tgoodwin/projects/monolift/.tmp/sprint-0021-a1-go.work
--function-index-mode=http-sinks
--boundary-discovery-mode=frontier
--function-index-budget=60s
--region-root 'github.com/mattermost/mattermost/server/v8/channels/app/platform.(*Hub).Start'
--region-root 'github.com/mattermost/mattermost/server/v8/channels/app/platform.(*WebConn).Pump'
/Users/tgoodwin/projects/monolift/evaluation/mattermost/server/cmd/mattermost
```

## Ladder Rows

| Run | Exit | JSON bytes | Reverse owners | Adjacent owners | Boundary candidates | Boundary evidence | BoundarySeed owners | Final indexed owners | Boundary phase ms | Peak RSS bytes | Stop reasons | External surfaces | Registration sites | Wrapper chains | `channels/api4` | `connectWebSocket` touchpoint | `connectWebSocket` external | `APIHandlerTrustRequester` | Target registration owner | `http.Handler` sink |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---:|---:|---:|---|---|---|---|---|---|
| small: r500 / a500 / b1k / d1 / 45s | 0 | 10,271,850 | 500 | 500 | 1,000 | 272 | 48 | 48 | 15,110 | 7,084,912,328 | reverse owner, adjacent owner | 1 | 1 | 331 | yes | yes | no | no | no | yes |
| medium: r2k / a2k / b5k / d2 / 60s | 0 | 10,182,394 | 2,000 | 2,000 | 4,000 | 290 | 50 | 50 | 36,478 | 9,469,698,216 | reverse owner, adjacent owner | 1 | 1 | 360 | yes | yes | no | no | no | yes |
| large: r5k / a5k / b10k / d2 / 90s | 0 | 10,886,952 | 5,000 | 5,000 | 10,000 | 435 | 72 | 72 | 81,724 | 10,570,637,736 | reverse owner, adjacent owner | 2 | 10 | 486 | yes | yes | no | no | no | yes |
| exploratory: r5k / a10k / b20k / d3 / 120s | cut | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a | cut: no target-specific movement in prior rows | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a |

Boundary phase ms is `boundary_reverse_frontier` +
`boundary_adjacent_expansion` + `boundary_predicate_scan` +
`boundary_seed_set_assembly` + final `function_ref_index`.

The completed rows fixed the SPRINT-0026 mechanical failure:
`adjacentExpansionOwners` was nonzero in every row. Larger rows found more
generic boundary evidence, but the target-specific indicators did not move.

## Sprint Question

Does budget partitioning recover Mattermost target evidence without
whole-program boundary discovery?

**No.** Budget partitioning reserves capacity for adjacent expansion and
BoundaryPredicate scanning, and it does produce nonzero adjacent owners. It does
not recover the target registration evidence under the measured rows:

- `connectWebSocket` remains a reverse-BFS touchpoint only, not an
  ExternalSurface.
- `APIHandlerTrustRequester` is absent from touchpoints, external surfaces, and
  registration sites.
- No registration site links the target handler into the expected
  `http.Handler` sink chain.

The likely missing piece is not a larger single frontier budget; it is a more
precise way to follow function-value flow from the `connectWebSocket` touchpoint
to the registration owner.

## Recommendation

**Recommended next step: one more specific diagnostic.**

Run a bounded touchpoint-to-boundary value-flow diagnostic. Start from
reverse-BFS touchpoints and follow function values through call arguments,
stores, returns, and wrapper functions toward existing InvocationBoundary
evidence, with independent queue, depth, candidate, index, and duration budgets.
The diagnostic should answer whether the known `connectWebSocket` touchpoint can
be connected to the missing registration owner without whole-program boundary
discovery.

## Cost Gate

Keep the SPRINT-0025 split gate as the implementation baseline:

- load + SSA + root resolution + callgraph under 90s wall time and 8 GB RSS,
- incremental boundary EntryPath after callgraph under 30s wall time and
  +1.5 GB RSS.

The larger SPRINT-0027 budgets are acceptable only as diagnostic rows. They are
not implementation-ready gates.

| Run | Boundary phase ms | RSS delta after callgraph | Gate result |
|---|---:|---:|---|
| small | 15,110 | +1,998,038,768 bytes | fail memory |
| medium | 36,478 | +3,946,271,552 bytes | fail time and memory |
| large | 81,724 | +5,478,744,432 bytes | fail time and memory |

Cost-gate recommendation: keep the split gate unchanged. The next diagnostic
should aim for better target-directed selection, not larger frontier budgets.

## Suggestions

Immediate diagnostic steps:

- Shape SPRINT-0028 as a touchpoint-to-boundary value-flow bridge diagnostic.
  Evidence: all SPRINT-0027 rows already see `connectWebSocket` as a touchpoint,
  but no row promotes it to an ExternalSurface or finds
  `APIHandlerTrustRequester`.
- Keep the same independent budget reporting style. Evidence: the new stats
  proved adjacent expansion was no longer starved (`500`, `2,000`, and `5,000`
  adjacent owners), which made the remaining failure mode clearer.
- Add a target-closeness summary directly to diagnostic artifacts. Evidence:
  generic counts improved in the large row, but the meaningful target fields
  stayed false.

Larger follow-up implementation ideas, only if the next diagnostic succeeds:

- Promote the successful bridge into reusable SeedSet construction for
  invocation-boundary indexing.
- Consider report or surface wiring only after the bridge recovers the target
  registration evidence under the split cost gate.

Do not pursue:

- Do not keep increasing frontier depth or owner budgets as the next step.
  Evidence: the large row scanned 10,000 candidates, peaked above 10.5 GB RSS,
  spent 81.724s in boundary work, and still missed the target chain.
- Do not add Mattermost-specific route, package, framework, or handler-name
  recognizers. The measured failure is a generic function-value reachability
  gap, not a need for target-specific classification.
