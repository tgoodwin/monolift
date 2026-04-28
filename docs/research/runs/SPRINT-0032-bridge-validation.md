# SPRINT-0032 Bridge Validation

Date: 2026-04-28

## Command

Build:

```sh
go build -o /tmp/monolift-sprint-0032-entrypath-probe ./cmd/entrypath-probe
```

Validation:

```sh
/usr/bin/time -p env GOWORK=/Users/tgoodwin/projects/monolift/.tmp/sprint-0021-a1-go.work \
  /tmp/monolift-sprint-0032-entrypath-probe \
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
    > docs/research/runs/SPRINT-0032-bridge-phase-local.json \
    2> docs/research/runs/SPRINT-0032-bridge-phase-local.stderr
```

Summary:

```sh
jq '{wall:.stats.wallClockMillis, peak:.stats.peakRSSBytes, phases:.stats.phaseTimings, index:.stats.functionRefIndex, seeds:.stats.functionIndexSeeds, boundary:.stats.boundaryDiscovery, bridge:(.stats.bridgeDiscovery | del(.coverage)), counts:{external:(.externalSurfaces|length), registrations:(.registrationSites|length), chains:(.wrapperChains|length)}, oracle:.oracleTrace}' \
  docs/research/runs/SPRINT-0032-bridge-phase-local.json \
  > docs/research/runs/SPRINT-0032-bridge-phase-local.summary.json
```

## Result

The phase-local bridge budget recovered the main Mattermost chain with the
nominal 60s function-index budget.

| Metric | Value |
|---|---:|
| Probe wall | 154,615 ms |
| Wrapper real | 250.72s |
| Peak RSS | 9,137,803,352 bytes |
| Bridge seed discovery | 70,464 ms |
| Function-ref index | 3,533 ms |
| Indexed bridge owners | 1,676 / 1,676 |
| Skipped bridge owners | 0 |
| Indexed instructions | 124,597 |
| External surfaces | 608 |
| Registration sites | 689 |
| Wrapper chains | 3,322 |

The wrapper real time was inflated by the concurrent `go test
./pkg/compiler/entrypath` run. The important before/after fact is that the
function-ref index now starts with its own budget after bridge discovery and
indexes all admitted bridge owners.

## Oracle Recovery

| Target | Seed | Function-ref index | Final classification |
|---|---|---|---|
| `connectWebSocket` | present | present | present |
| `APIHandlerTrustRequester` | present | present | present |
| `InitWebSocket` | present | present | present |

| Relationship | Result |
|---|---|
| `connect-to-api-handler` | recovered in final classification |
| `connect-registered-at-init` | recovered in final classification |
| `init-has-http-boundary` | boundary evidence present; final relationship absent |

## Read

This confirms the SPRINT-0031 miss was a budget-semantics problem, not an owner
discovery or priority-ordering problem. Bridge discovery admitted the target
owners; phase-local bridge indexing scanned them and the existing value-flow
classification recovered the main chain.
