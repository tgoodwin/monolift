# SPRINT-0028 Oracle Trace

Date: 2026-04-28

## Oracle Spec

Structured oracle input:
`docs/research/runs/SPRINT-0028-mattermost-oracle.json`

Target nodes:

| ID | Package | Object |
|---|---|---|
| `connect` | `github.com/mattermost/mattermost/server/v8/channels/api4` | `connectWebSocket` |
| `api-handler-trust-requester` | `github.com/mattermost/mattermost/server/v8/channels/api4` | `(*API).APIHandlerTrustRequester` |
| `init-websocket` | `github.com/mattermost/mattermost/server/v8/channels/api4` | `(*API).InitWebSocket` |

Target relationships:

| ID | Kind | Expected evidence |
|---|---|---|
| `connect-to-api-handler` | function-value edge | `connectWebSocket` passed to `(*API).APIHandlerTrustRequester` with `EdgeFunctionValueArg` |
| `connect-registered-at-init` | registration | `connectWebSocket` reaches `(*API).InitWebSocket` as `net/http.Handler` sink evidence |
| `init-has-http-boundary` | boundary predicate | `(*API).InitWebSocket` has `net/http.Handler` boundary evidence |

## Exhaustive Oracle Upper Bound

Source artifact:
`docs/research/runs/SPRINT-0025-entrypath-index-budget-120s-v3.json`

This is the known-good upper bound used by SPRINT-0028. It completed the
whole-program function-reference scan but hit the 120s budget during
finalization/sorting; downstream flow still recovered the target evidence.

| Metric | Value |
|---|---:|
| Probe wall clock | 206,689 ms |
| Peak RSS | 12,470,170,336 bytes |
| Function-index phase | 120,038 ms |
| Function-value-flow phase | 23,954 ms |
| Scanned functions | 140,801 |
| Scanned instructions | 5,625,718 |
| External surfaces | 2,956 |
| Registration sites | 3,982 |
| Wrapper chains | 64,314 |

Recovered target evidence from the upper bound:

| Evidence | Status |
|---|---|
| `connectWebSocket` external surface | recovered via `EdgeFunctionValueArg` at `(*Router).Handle(...)` |
| `connectWebSocket -> APIHandlerTrustRequester` | recovered in wrapper-chain evidence with `EdgeFunctionValueArg` at `channels/api4/websocket.go:54` |
| `connectWebSocket -> InitWebSocket -> net/http.Handler` | recovered as a registration site owned by `(*API).InitWebSocket`, sink kind `http-handler`, static type `net/http.Handler` |

## Runs

Raw artifacts:

- `SPRINT-0028-oracle-all.{json,stderr,summary.json}`
- `SPRINT-0028-oracle-frontier-large.{json,stderr,summary.json}`
- `SPRINT-0028-oracle-bridge-v2.{json,stderr,summary.json}`
- Earlier bridge row before the explicit seed-discovery phase:
  `SPRINT-0028-oracle-bridge.{json,stderr,summary.json}`

All rows used exact region roots:

```sh
--region-root 'github.com/mattermost/mattermost/server/v8/channels/app/platform.(*Hub).Start'
--region-root 'github.com/mattermost/mattermost/server/v8/channels/app/platform.(*WebConn).Pump'
```

Cost comparison:

| Run | Probe wall ms | Wrapper real | Peak RSS | Key work | Target result |
|---|---:|---:|---:|---|---|
| SPRINT-0025 all upper bound | 206,689 | not used | 12,470,170,336 | all functions, 120s index budget | recovered |
| SPRINT-0028 all + oracle | 245,943 | 311.99s | 14,622,893,264 | all functions, oracle trace | recovered |
| SPRINT-0028 frontier large | 141,600 | 188.87s | 9,951,744,280 | 5k reverse + 5k adjacent + 10k candidates | missed |
| SPRINT-0028 oracle bridge v2 | 78,380 | 103.62s | 8,935,415,848 | package-local bridge, 94 indexed owners | recovered |

The reproduced oracle all-mode row is more expensive than the SPRINT-0025
artifact, mostly after function-indexing, but it preserves the same target
answer. Use the SPRINT-0025 row as the cost upper bound and the SPRINT-0028 row
as the phase-presence oracle output.

## Loss Table

Legend: `P` present, `A` absent, `NR` not run.

### Node Presence

| Run | Node | SSA | Touchpoint | Reverse owner | Adjacent owner | Boundary candidate | Boundary evidence | Seed | Function-ref index | Final |
|---|---|---|---|---|---|---|---|---|---|---|
| all + oracle | `connect` | P | P | NR | NR | NR | NR | NR | P | P |
| all + oracle | `api-handler-trust-requester` | P | A | NR | NR | NR | NR | NR | P | P |
| all + oracle | `init-websocket` | P | A | NR | NR | NR | NR | NR | P | P |
| frontier large | `connect` | P | P | P | A | P | A | A | A | A |
| frontier large | `api-handler-trust-requester` | P | A | A | A | A | A | A | A | A |
| frontier large | `init-websocket` | P | A | A | A | A | A | A | A | A |
| oracle bridge v2 | `connect` | P | P | NR | NR | NR | A | P | P | P |
| oracle bridge v2 | `api-handler-trust-requester` | P | A | NR | NR | NR | P | P | P | P |
| oracle bridge v2 | `init-websocket` | P | A | NR | NR | NR | P | P | P | P |

### Relationship Presence

| Run | Relationship | SSA endpoints | Boundary evidence | Seed | Function-ref index | Final | First missing in failed row |
|---|---|---|---|---|---|---|---|
| all + oracle | `connect-to-api-handler` | P | NR | NR | A | P | recovered |
| all + oracle | `connect-registered-at-init` | P | NR | NR | A | P | recovered |
| frontier large | `connect-to-api-handler` | P | A | A | A | A | function-ref index / source seed |
| frontier large | `connect-registered-at-init` | P | A | A | A | A | function-ref index / registration owner seed |
| frontier large | `init-has-http-boundary` | P | A | A | A | A | boundary candidate owner selection |
| oracle bridge v2 | `connect-to-api-handler` | P | A | A | A | P | recovered |
| oracle bridge v2 | `connect-registered-at-init` | P | A | A | A | P | recovered |
| oracle bridge v2 | `init-has-http-boundary` | P | P | A | A | A | recovered as boundary evidence |

The relationship-level function-ref phase is conservative: it records direct
index evidence only when the relationship itself can be explained from indexed
refs. The all and bridge rows still recover the target relationships at final
classification, which is the decisive oracle result.

### First Missing Mechanism

| Target | Frontier-large first meaningful miss | Likely mechanism |
|---|---|---|
| `connect` node | Present as touchpoint, reverse owner, and boundary candidate, but absent from seed set and function-ref index | seed/source selection after owner discovery |
| `api-handler-trust-requester` node | Loaded SSA only; absent from reverse/frontier/candidate/seed/index | owner selection and missing function-value bridge |
| `init-websocket` node | Loaded SSA only; absent from reverse/frontier/candidate/seed/index | owner selection and missing call-site bridge |
| `connect-to-api-handler` | endpoints loaded, final absent, no indexed source relationship | function-value bridge missing |
| `connect-registered-at-init` | endpoints loaded, final absent, registration owner absent | function-value bridge missing |
| `init-has-http-boundary` | predicate evidence absent because `init-websocket` is never a boundary candidate | owner selection, not predicate rejection |

## Bounded Bridge Experiment

Mode: `--function-index-mode=oracle-bridge`

Bridge parameters:

```sh
--oracle-bridge-max-package-functions=2000
--oracle-bridge-max-owners=500
--oracle-bridge-max-duration=30s
--function-index-budget=60s
```

The bridge starts from oracle `bridgeStarts` and scans bounded same-package
owners for direct function-value refs plus existing boundary predicate evidence.
It then builds the normal function-ref index over that seed set.

Bridge v2 result:

| Metric | Value |
|---|---:|
| Probe wall ms | 78,380 |
| Wrapper real | 103.62s |
| Peak RSS | 8,935,415,848 bytes |
| Bridge seed discovery | 38,687 ms |
| Indexed owners | 94 |
| Scanned instructions | 11,279 |
| Boundary evidence | 4,261 |
| External surfaces | 591 |
| Registration sites | 670 |
| Wrapper chains | 788 |

Oracle target result: recovered.

The bridge found:

- `connect` as an oracle bridge seed and indexed source.
- `api-handler-trust-requester` as boundary/http-sink/oracle-bridge seed,
  indexed source, and final chain participant.
- `init-websocket` as boundary/http-sink/oracle-bridge seed, indexed source,
  final registration owner, and `net/http.Handler` boundary evidence.

## Findings

1. SPRINT-0027 frontier did not fail because reverse BFS missed
   `connectWebSocket`. It touched it and even admitted it as a reverse owner and
   boundary candidate in the large row.
2. The bounded frontier fails after owner discovery: `connect` never becomes a
   seed or function-ref-index source, and the call-site/wrapper owners
   `InitWebSocket` and `APIHandlerTrustRequester` never enter the candidate set.
3. The boundary predicate is capable of recognizing the missing HTTP evidence
   once the right owner is scanned. The bridge row finds boundary evidence for
   both `InitWebSocket` and `APIHandlerTrustRequester`.
4. The successful bridge is generic in shape: start from a known touchpoint,
   scan a bounded local owner set for function-value refs and boundary evidence,
   then run existing function-value flow. The oracle spec contains Mattermost
   identities, but the mechanism does not require Mattermost-specific
   recognizers.
5. The bridge is not cost-ready. Seed discovery took 38.687s and raised RSS by
   roughly 3.8 GB after reverse BFS. This sprint was diagnostic, so no memory
   optimization was attempted.

## Recommendation

Recommended next step: an implementation sprint for a generic
touchpoint-to-boundary bridge seed source.

Scope it to reusable mechanics only:

- derive bridge starts from reverse-BFS touchpoints, not service-specific names;
- find nearby function-value ref owners and boundary predicate owners without a
  whole-program sort/scan;
- preserve independent bridge discovery, seed, index, and flow stats;
- keep the result behind EntryPath diagnostic/seed construction until it meets
  the split cost gate.

## Suggestions

Immediate next sprint shape: implement the generic bridge as a bounded seed
source with package/member indexes and cost accounting, then rerun the same
oracle spec without oracle-provided start names.

Do not pursue next: deeper frontier rows, larger owner budgets, package
pruning, or Mattermost-specific recognizers. The loss table shows the missing
transition is a function-value bridge, not lack of raw frontier budget.

Generic bridge assessment: yes. The oracle indicates a generic bridge
mechanism rather than a Mattermost workaround; the same pattern should apply to
callback registration chains and other typed boundary sinks such as gRPC once
the boundary predicate set is extended.
