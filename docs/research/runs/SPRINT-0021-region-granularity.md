# SPRINT-0021 region-granularity cliff

## Summary

SPRINT-0021 stopped at A.2. The Mattermost Hub/WebConn candidate is a real composite region, but the current compiler region model is rooted at a single parsed pragma/root declaration. The planned boundary spans at least two independent lifecycle roots:

- Hub fanout and connection-index ownership rooted at `Hub` methods in `channels/app/platform/web_hub.go`.
- Per-connection write-pump, sequence, and dead-queue replay rooted at `WebConn.Pump` / `WebConn.writePump` in `channels/app/platform/web_conn.go`.

The A.1 closure can cover the Hub side and several WebConn filtering helpers, but it does not include `(*WebConn).writePump`, and the report schema does not expose field-level symbols such as `WebConn.send`, `WebConn.deadQueue`, `WebConn.Sequence`, or `WebConn.connectionID` as independent `closure.includedSymbols`.

This is not an OOM. It is a region-granularity contract collision: the sprint's intended composite boundary is a multi-root region, while the current classifier/closure/report path can represent only one root surface with selected operations.

## Reproduction

The Mattermost module path is:

```text
github.com/mattermost/mattermost/server/v8
```

The A.1 probe used a temporary workspace to make the local `server/public` module visible without modifying `evaluation/mattermost`:

```sh
cat > .tmp/sprint-0021-a1-go.work <<EOF
go 1.25.8

use (
	$PWD/evaluation/mattermost/server
	$PWD/evaluation/mattermost/server/public
)
EOF

GOWORK=$PWD/.tmp/sprint-0021-a1-go.work \
MONOLIFT_PROFILE_DIR=.tmp/sprint-0021-a1-profiles \
/usr/bin/time -l bin/e2e-compile \
  --target=mattermost \
  --output=.tmp/sprint-0021-a1-output \
  --source=evaluation/mattermost/server
```

The synthetic e2e root was `Hub` with methods `Start,Broadcast,Register,Unregister,CheckConn`.

## A.1 resource numbers

- Wall time: 88.22s.
- Maximum resident set size: 2,152,579,072 bytes.
- Peak memory footprint: 10,268,152,640 bytes.
- Runtime memstats: heap_alloc 4098.69 MiB, heap_sys 9650.81 MiB, sys 9841.48 MiB.
- Closure size: 2,956 included symbols and 4,838 excluded symbols.
- Profiles:
  - `.tmp/sprint-0021-a1-profiles/mattermost.cpu.pprof`
  - `.tmp/sprint-0021-a1-profiles/mattermost.heap.pprof`
  - `.tmp/sprint-0021-a1-profiles/mattermost.memstats.json`
- Report:
  - `.tmp/sprint-0021-a1-output/closure-report.json`

## Boundary status

The intended Hub/WebConn boundary from A.2 is:

- `Hub`
- `Hub.Start`
- `Hub.Broadcast`
- `Hub.Register`
- `Hub.Unregister`
- `hubConnectionIndex`
- `hubConnectionIndex.Add`
- `hubConnectionIndex.Remove`
- `hubConnectionIndex.ForUser`
- `hubConnectionIndex.ForChannel`
- `WebConn`
- `WebConn.send`
- `WebConn.deadQueue`
- `WebConn.Sequence`
- `WebConn.connectionID`
- `WebConn.writePump`
- `CheckWebConn`
- `PlatformService.GetHubForUserId`

Observed in `closure.includedSymbols` for the A.1 report:

```text
present Hub
present (*Hub).Start
present (*Hub).Broadcast
present (*Hub).Register
present (*Hub).Unregister
present hubConnectionIndex
present (*hubConnectionIndex).Add
present (*hubConnectionIndex).Remove
present (*hubConnectionIndex).ForUser
present (*hubConnectionIndex).ForChannel
present WebConn
missing (*WebConn).writePump
present (*PlatformService).GetHubForUserId
```

Field-level members `send`, `deadQueue`, `Sequence`, and `connectionID` are visible in source and SSA type information, but they are not modeled as individual closure symbols in the report.

## Why this is a cliff

The planned composite is not just "Hub.Broadcast plus callees." Its semantics require the interaction between:

- user-keyed hub shard selection (`PlatformService.GetHubForUserId`),
- hub-local connection indexes (`hubConnectionIndex` maps),
- fanout delivery from `Hub.Start` / `Hub.Broadcast`,
- and per-connection replay state in `WebConn.writePump`.

The current compiler accepts one root pragma and builds one closure from that root's exposed operations. A `Hub` root reaches the hub fanout/index side but not the WebConn write-pump lifecycle. A `WebConn` root would reach the connection replay side but not the Hub fanout boundary as the same candidate region. Creating a fake wrapper root would modify the target or synthesize a region not present in the source, which is out of scope for this sprint.

## Follow-up shape

A future sprint needs a first-class multi-root region model before this Mattermost composite can be classified honestly. The likely shape is:

- a region declaration or detector output that can name multiple source roots as one candidate region,
- closure union with provenance per root,
- stateclass evidence aggregation across the union,
- report schema support for contributing root symbols distinct from contributing archetypes,
- and admission that can evaluate the complete boundary without pretending one lifecycle root owns all behavior.

No Mattermost-specific carve-out is recommended. The general compiler gap is support for real regions whose semantics are distributed across cooperating actor roots.
