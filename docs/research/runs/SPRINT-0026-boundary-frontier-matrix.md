# SPRINT-0026 Boundary-Frontier Matrix

Date: 2026-04-27

This matrix summarizes the Mattermost boundary-frontier diagnostic ladder.
Raw artifacts live beside this file as
`SPRINT-0026-boundary-frontier-*.{json,stderr,meta,summary.json}`.

## Ladder Rows

All rows used:

```sh
--function-index-mode=http-sinks
--boundary-discovery-mode=frontier
--boundary-frontier-max-duration=30s
--function-index-budget=60s
--region-root '(*Hub).Start'
--region-root '(*WebConn).Pump'
```

| Run | Exit | JSON bytes | Owners | Packages | BoundarySeed owners | Boundary evidence | Boundary predicate scan ms | Final index ms | Final scanned funcs | Peak RSS bytes | Stop reasons | External surfaces | Registration sites | Wrapper chains | `channels/api4` reached | `connectWebSocket` touchpoint | `connectWebSocket` external | `APIHandlerTrustRequester` | `http.Handler` sink |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---:|---:|---:|---|---|---|---|---|
| depth 1 / 500 | 0 | 10,064,891 | 500 | 32 | 45 | 259 | 9,079 | 69 | 45 | 7,046,127,208 | owner budget | 1 | 1 | 312 | yes | yes | no | no | yes |
| depth 1 / 5k | 0 | 10,461,559 | 5,000 | 272 | 47 | 278 | 29,720 | 45 | 47 | 8,778,088,456 | duration budget, owner budget | 1 | 9 | 389 | yes | yes | no | no | no |
| depth 2 / 5k | 0 | 10,603,261 | 5,000 | 272 | 47 | 278 | 29,660 | 91 | 47 | 8,741,908,472 | duration budget, owner budget | 1 | 9 | 389 | yes | yes | no | no | no |
| depth 2 / 10k | 0 | 10,698,555 | 10,000 | 439 | 70 | 383 | 29,351 | 119 | 70 | 9,300,734,072 | duration budget, owner budget | 1 | 9 | 445 | yes | yes | no | no | no |

The conditional depth 3 / 10k row was cut; see
[`SPRINT-0026-boundary-frontier-d3-o10000-cut.md`](SPRINT-0026-boundary-frontier-d3-o10000-cut.md).

## Sprint Question

Can boundary-frontier discovery recover the Mattermost target evidence without
whole-program boundary scanning?

**No, not in the measured implementation.** The frontier mode avoids
whole-program boundary scanning and keeps the final seeded index tiny
(45-70 scanned functions), but it does not recover the target HTTP registration
chain. `channels/api4` and `connectWebSocket` appear as reverse/touchpoint
evidence, yet no row recovers `connectWebSocket` as an ExternalSurface, no row
finds `APIHandlerTrustRequester`, and no row links the target handler into the
desired `http.Handler` registration chain.

The failure mode is structural: reverse-frontier collection consumes the owner
budget before callgraph-adjacent expansion can add owners. Every measured row
has `adjacentExpansionOwners=0`; increasing depth therefore does not change the
searched frontier under the tested budgets.

## Recommendation

**Recommended next step: one more specific diagnostic.**

Run a budget-partitioned frontier diagnostic that reserves separate owner
budgets for:

- reverse-frontier owners,
- callgraph-adjacent expansion owners,
- BoundaryPredicate scan candidates.

The diagnostic should stream predicate scanning during expansion instead of
collecting thousands of reverse owners first. The only question for that follow
up should be whether reserved adjacent expansion can reach the target
registration evidence under the same split cost gate. Do not start report
wiring or surface classification from the current result.

## Cost Gate

Keep SPRINT-0025's split gate as the baseline:

- load + SSA + root resolution + callgraph under 90s wall time and 8 GB RSS,
- incremental boundary EntryPath after callgraph under 30s wall time and
  +1.5 GB RSS.

The baseline analysis stage passed this gate in every SPRINT-0026 row. The
incremental boundary stage only passed at depth 1 / 500:

| Run | Incremental boundary wall ms | RSS delta after callgraph | Gate result |
|---|---:|---:|---|
| depth 1 / 500 | 9,223 | 4,128,768 | pass |
| depth 1 / 5k | 30,110 | 2,871,845,456 | fail |
| depth 2 / 5k | 30,130 | 2,124,755,360 | fail |
| depth 2 / 10k | 30,244 | 2,373,181,904 | fail |

Cost-gate recommendation: keep the split gate unchanged, and require the next
diagnostic to reserve adjacent-expansion budget without increasing the
incremental memory delta beyond +1.5 GB. The current 5k and 10k rows are not
eligible for implementation because they fail the incremental memory gate while
still missing the target chain.
