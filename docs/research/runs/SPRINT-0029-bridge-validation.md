# SPRINT-0029 Bridge Validation

Date: 2026-04-28

## Command

Build:

```sh
go build -o /tmp/monolift-sprint-0029-entrypath-probe ./cmd/entrypath-probe
```

Validation run:

```sh
/usr/bin/time -p env GOWORK=/Users/tgoodwin/projects/monolift/.tmp/sprint-0021-a1-go.work \
  /tmp/monolift-sprint-0029-entrypath-probe \
    --diagnostic-timings \
    --function-index-mode=bridge \
    --bridge-max-starts=1000 \
    --bridge-max-packages=64 \
    --bridge-max-package-functions=2000 \
    --bridge-max-owners=2000 \
    --bridge-max-boundary-owners=2000 \
    --bridge-max-instructions=250000 \
    --bridge-max-duration=60s \
    --function-index-budget=60s \
    --oracle-spec docs/research/runs/SPRINT-0028-mattermost-oracle.json \
    --region-root 'github.com/mattermost/mattermost/server/v8/channels/app/platform.(*Hub).Start' \
    --region-root 'github.com/mattermost/mattermost/server/v8/channels/app/platform.(*WebConn).Pump' \
    /Users/tgoodwin/projects/monolift/evaluation/mattermost/server/cmd/mattermost \
    > docs/research/runs/SPRINT-0029-bridge.json \
    2> docs/research/runs/SPRINT-0029-bridge.stderr
```

Summary extraction:

```sh
jq '{wall:.stats.wallClockMillis, peak:.stats.peakRSSBytes, phases:.stats.phaseTimings, index:.stats.functionRefIndex, seeds:.stats.functionIndexSeeds, boundary:.stats.boundaryDiscovery, bridge:.stats.bridgeDiscovery, counts:{external:(.externalSurfaces|length), registrations:(.registrationSites|length), chains:(.wrapperChains|length)}, oracle:.oracleTrace}' \
  docs/research/runs/SPRINT-0029-bridge.json \
  > docs/research/runs/SPRINT-0029-bridge.summary.json
```

Artifacts:

- `docs/research/runs/SPRINT-0029-bridge.json`
- `docs/research/runs/SPRINT-0029-bridge.stderr`
- `docs/research/runs/SPRINT-0029-bridge.summary.json`

## Result

The Mattermost target chain was not recovered without oracle-provided bridge
start names.

The bridge did select and index `connectWebSocket` from reverse-BFS touchpoints,
but it did not discover the local owners that make the function value useful:

- `connectWebSocket`: present in SSA, reverse-BFS touchpoints, bridge seed set,
  and function-ref index; absent from final classification.
- `APIHandlerTrustRequester`: present in SSA only; absent from bridge seed set,
  function-ref index, and final classification.
- `InitWebSocket`: present in SSA only; absent from bridge seed set,
  function-ref index, boundary evidence, and final classification.

Relationship recovery:

- `connect-to-api-handler`: not recovered.
- `connect-registered-at-init`: not recovered.
- `init-has-http-boundary`: not recovered.

First missing phase: bridge local owner discovery after start selection. In the
oracle trace this appears as `function_ref_index` absence for
`APIHandlerTrustRequester`, `InitWebSocket`, and the two target relationships.
The more specific implementation miss is that those owners never entered the
bridge seed set, so the final function-ref index never scanned them.

## Costs

| Metric | SPRINT-0029 bridge |
|---|---:|
| Probe wall clock | 80,766 ms |
| Wrapper real time | 125.41 s |
| Peak RSS | 6,066,748,888 bytes |
| Callgraph phase | 54,733 ms |
| Reverse BFS phase | 513 ms |
| Bridge seed discovery | 5,421 ms |
| Function-ref index phase | 1,301 ms |
| Function-value flow phase | 595 ms |
| Indexed owners | 1,074 |
| Indexed instructions | 94,909 |
| External surfaces | 56 |
| Registration sites | 58 |
| Wrapper chains | 2,570 |

Bridge seed stats:

| Metric | Value |
|---|---:|
| Reverse-BFS touchpoints | 5,630 |
| Start candidates | 4,584 |
| Selected starts | 1,000 |
| Start packages | 109 |
| Scanned packages | 46 |
| Scanned package functions | 4,685 |
| Bridge owners / seed owners | 1,074 |
| Indexed bridge owners | 1,074 |
| Boundary candidate owners | 2,000 |
| Boundary owners | 0 |
| Boundary evidence | 0 |
| Duplicate owner suppressions | 211 |

Budget stops:

- `start_budget`
- `boundary_owner_budget`
- `instruction_budget`

## Comparison

| Run | Probe wall ms | Wrapper real | Peak RSS | Key work | Target result |
|---|---:|---:|---:|---|---|
| SPRINT-0025 exhaustive upper bound | 206,689 | not used | 12,470,170,336 | 140,801 indexed owners, 5,625,718 indexed instructions | recovered |
| SPRINT-0028 frontier large | 141,600 | 188.87 s | 9,951,744,280 | 5k reverse + 5k adjacent + 10k candidates, 72 indexed owners | missed |
| SPRINT-0028 oracle bridge v2 | 78,380 | 103.62 s | 8,935,415,848 | oracle start, package-local bridge, 94 indexed owners | recovered |
| SPRINT-0029 bridge | 80,766 | 125.41 s | 6,066,748,888 | reverse-touchpoint starts, 1,074 indexed owners | missed |

Compared with SPRINT-0028 oracle bridge v2, the generic bridge used less peak
memory and much less bridge seed-discovery time, but it indexed many more owners
and failed to recover the target because the needed local owner package/member
scan did not reach `APIHandlerTrustRequester` or `InitWebSocket`.

Compared with SPRINT-0028 frontier large, the generic bridge was cheaper and
found `connectWebSocket` as a seed/source, but it still missed the owner bridge
from the selected start to the boundary owners.

Compared with the SPRINT-0025 exhaustive upper bound, the generic bridge was far
cheaper, but the exhaustive run remains the only non-oracle upper bound in this
comparison that recovered the target.

## Loss-Table Delta

| Target | SPRINT-0028 frontier large | SPRINT-0029 bridge |
|---|---|---|
| `connectWebSocket` seed/index | absent | present |
| `connectWebSocket` final | absent | absent |
| `APIHandlerTrustRequester` seed/index/final | absent | absent |
| `InitWebSocket` seed/index/final | absent | absent |
| `InitWebSocket` boundary evidence | absent | absent |
| `connect-to-api-handler` final | absent | absent |
| `connect-registered-at-init` final | absent | absent |

The bridge improved the first half of the SPRINT-0028 failure: the reverse-BFS
touchpoint became a seed and function-ref source. It did not solve the second
half: discovering the nearby owners that pass the touchpoint into a typed
boundary.

## Diagnostic Recommendation

Run one targeted diagnostic next: record bridge package scheduling and per-start
package scan coverage, including whether each selected start's package was
scanned before `instruction_budget` or `boundary_owner_budget` stopped
discovery.

Do not broaden the start, owner, instruction, or boundary-owner budgets as the
next step. The failed row already selected `connectWebSocket`; the missing fact
is why the selected start's local owners were not reached and admitted.

