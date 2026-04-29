# SPRINT-0033 EntryPath Candidate Viability

Date: 2026-04-28

## Decision

Do not proceed to the normalized EntryPath contract in this execution pass.

The current bridge algorithm generalizes to some non-Mattermost HTTP and handler-factory shapes, but the locked candidate pass also exposed two blockers:

- The Gitea full-application corpus is too expensive for routine validation with the current probe shape. The SSE row completed but took 1,037.29s real time, with 989,852ms probe wall time and a 305,584ms bridge discovery phase despite a 60s bridge duration budget.
- Background/lifecycle/job-oriented recovery is not yet strong enough. The PocketBase autobackup fallback found the cron callback body that calls `CreateBackup`, but did not recover the bootstrap hook or `Cron.Add` registration as consumer-facing boundary evidence.

The next sprint should improve boundary-family predicates and bridge budget enforcement for non-HTTP registrations before locking a stable consumer contract.

## Probe Profile

All probes ran serially. Raw JSON and stderr were written under `/tmp` and are not intended for git.

Common profile:

```sh
--diagnostic-timings
--function-index-mode=bridge
--bridge-max-starts=1000
--bridge-max-packages=64
--bridge-max-package-functions=2000
--bridge-max-owners=2000
--bridge-max-boundary-owners=2000
--bridge-max-instructions=250000
--bridge-max-duration=60s
--function-index-budget=60s
```

Mattermost used the SPRINT-0032 oracle spec. Gitea required a Go 1.26 probe binary and a temporary module cache because the existing Go 1.26.2 toolchain cache could not list the standard library. PocketBase required a temporary Go build cache because the shared build cache had missing compiled entries.

## Summary

| Candidate | Classification | Key read |
|---|---|---|
| Mattermost WebSocket hub | Viable | Recovered the known `InitWebSocket -> connectWebSocket` registration and the wrapper link through `APIHandlerTrustRequester`; cost remains high. |
| Gitea SSE eventsource | Partial with bounded gap | Recovered `Events -> Manager.Register -> Messenger.Register` touchpoints, but no external surfaces or registration sites; bridge duration budget was not practically bounded. |
| Miniflux feed refresh | Partial with bounded gap | Recovered API, UI, worker, and CLI touchpoints plus broad HTTP boundary evidence, but did not produce a clean `refreshFeedHandler` registration. |
| Miniflux Fever handler | Viable | Recovered `newRouter`, `fever.Middleware`, and `fever.NewHandler` evidence for the object-method handler shape. |
| PocketBase autobackup fallback | Fixture-required / not currently viable | Found the cron callback closure that calls `CreateBackup`, but missed the bootstrap hook and `Cron.Add` callback registration boundary. |

## Mattermost WebSocket Hub

Command output:

- Raw JSON: `/tmp/monolift-sprint-0033-mattermost.json`
- Stderr: `/tmp/monolift-sprint-0033-mattermost.stderr`

Roots:

- `(*github.com/mattermost/mattermost/server/v8/channels/app/platform.Hub).Start` at `channels/app/platform/web_hub.go:526`
- `(*github.com/mattermost/mattermost/server/v8/channels/app/platform.WebConn).Pump` at `channels/app/platform/web_conn.go:408`

Observed counts:

- Region touchpoints: 5,525
- External surfaces: 609
- Registration sites: 690
- Wrapper chains: 3,066

Cost:

- Probe wall: 189,721ms
- `/usr/bin/time` real: 242.90s
- Peak RSS: 12,547,113,712 bytes
- Callgraph: 49,175ms
- Bridge seed discovery: 120,765ms
- Function-ref index: 2,983ms
- Function-value flow: 732ms

Budget stops:

- Bridge stopped for `boundary_owner_budget`, `duration_budget`, and `start_budget`.

Activation evidence:

- Registration site: `(*API).InitWebSocket` at `channels/api4/websocket.go:52`
- Handler: `connectWebSocket`
- Edge kind: `EdgeFunctionValueArg`
- Static sink/type evidence: `sinkKind=http-handler`, `staticParameterType=net/http.Handler`

Wrapper evidence:

- `connectWebSocket -> (*API).APIHandlerTrustRequester` with `EdgeFunctionValueArg` at `channels/api4/websocket.go:54`
- `(*API).APIHandlerTrustRequester` and `(*API).InitWebSocket` both had HTTP boundary predicate evidence.

Classification: viable. This preserves the known SPRINT-0032 control chain, but cost is high and bridge discovery exceeded the nominal duration budget.

## Gitea SSE Eventsource

Command output:

- Raw JSON: `/tmp/monolift-sprint-0033-gitea-sse.json`
- Stderr: `/tmp/monolift-sprint-0033-gitea-sse.stderr`

Roots:

- `(*code.gitea.io/gitea/modules/eventsource.Manager).Register` at `modules/eventsource/manager.go:33`
- `(*code.gitea.io/gitea/modules/eventsource.Messenger).Register` at `modules/eventsource/messenger.go:24`

Observed counts:

- Region touchpoints: 3
- External surfaces: 0
- Registration sites: 0
- Wrapper chains: 15

Cost:

- Probe wall: 989,852ms
- `/usr/bin/time` real: 1,037.29s
- Peak RSS: 7,592,652,616 bytes
- Callgraph: 246,798ms
- Bridge seed discovery: 305,584ms
- Function-ref index: 83ms
- Function-value flow: 16ms

Budget stops:

- Bridge stopped for `duration_budget`, but only after a 305,584ms bridge discovery phase.

Recovered evidence:

- `routers/web/events.Events -> (*eventsource.Manager).Register` with `EdgeStaticCall`
- `(*eventsource.Manager).Register -> (*eventsource.Messenger).Register` with `EdgeStaticCall`
- Wrapper/direct-invoke facts around `Events`, `GetManager`, `Manager.Register`, `Messenger.Register`, and the event writer loop.

Missing evidence:

- No `/user/events` route registration boundary was classified.
- No consumer-facing registration site or external surface was produced.

Gap class:

- Boundary-family coverage and budget/cost. The direct call path is present, but the current boundary predicates and bridge owner selection do not recover the web route registration as stable activation evidence. The Gitea full-app probe is too expensive for repeated corpus validation without stronger budget enforcement or a narrower harness.

Classification: partial with bounded gap.

## Miniflux Feed Refresh

Command output:

- Raw JSON: `/tmp/monolift-sprint-0033-miniflux-refresh.json`
- Stderr: `/tmp/monolift-sprint-0033-miniflux-refresh.stderr`

Roots:

- `miniflux.app/v2/internal/reader/handler.RefreshFeed` at `internal/reader/handler/handler.go:207`
- `miniflux.app/v2/internal/reader/processor.ProcessFeedEntries` at `internal/reader/processor/processor.go:27`

Observed counts:

- Region touchpoints: 218
- External surfaces: 29
- Registration sites: 53
- Wrapper chains: 760

Cost:

- Probe wall: 7,640ms
- `/usr/bin/time` real: 14.87s
- Peak RSS: 2,132,483,064 bytes
- Callgraph: 5,255ms
- Bridge seed discovery: 1,436ms
- Function-ref index: 99ms
- Function-value flow: 61ms

Budget stops:

- None.

Recovered evidence:

- HTTP/API touchpoints including `(*api.handler).refreshFeedHandler`, API auth middleware closures, `net/http` server functions, and `internal/api.NewHandler`.
- Background/job-style touchpoints including `(*worker.worker).Run` and CLI refresh goroutine/direct invoke paths.
- HTTP boundary evidence and registrations were produced, but many are unrelated Google Reader/UI handlers over-associated with the broad region.

Missing evidence:

- No clean registration site was produced for `refreshFeedHandler` itself.
- Background scheduler/worker bootstrap was visible as touchpoints, but not as a distinct activation-boundary registration.

Gap class:

- Boundary-family coverage and root/result precision. The algorithm finds useful surrounding evidence quickly, but the consumer contract would need explicit unsupported/noisy-gap evidence to avoid presenting broad registrations as a precise refresh endpoint.

Classification: partial with bounded gap.

## Miniflux Fever Handler

Command output:

- Raw JSON: `/tmp/monolift-sprint-0033-miniflux-fever.json`
- Stderr: `/tmp/monolift-sprint-0033-miniflux-fever.stderr`

Root:

- `(*miniflux.app/v2/internal/fever.feverHandler).serve` at `internal/fever/handler.go:31`

Observed counts:

- Region touchpoints: 94
- External surfaces: 29
- Registration sites: 53
- Wrapper chains: 641

Cost:

- Probe wall: 13,002ms
- `/usr/bin/time` real: 22.51s
- Peak RSS: 2,322,353,192 bytes
- Callgraph: 4,689ms
- Bridge seed discovery: 6,306ms
- Function-ref index: 400ms
- Function-value flow: 134ms

Budget stops:

- None.

Activation evidence:

- Registration site: `internal/http/server.newRouter`
- Handler: `internal/http/server.newRouter$1`
- Edge kind: `EdgeFunctionValueArg`
- Static sink/type evidence: `sinkKind=http-handler`, `staticParameterType=net/http.Handler`

Wrapper/bootstrap evidence:

- `fever.NewHandler` direct invoke at `internal/http/server/routes.go:27`
- `fever.Middleware` direct invoke at `internal/http/server/routes.go:27`
- `fever.Middleware$1$1 -> fever.Middleware$1` with `EdgeFunctionValueReturned` at `internal/fever/middleware.go:19`
- `newRouter -> server.middleware` and `newRouter -> net/http.StripPrefix` wrapper edges at `internal/http/server/routes.go`

Classification: viable. This is the strongest non-Mattermost corpus row and demonstrates method-value/object-handler and middleware-return evidence.

## PocketBase Autobackup Fallback

Command output:

- Raw JSON: `/tmp/monolift-sprint-0033-pocketbase-autobackup.json`
- Stderr: `/tmp/monolift-sprint-0033-pocketbase-autobackup.stderr`

Root:

- `(*github.com/pocketbase/pocketbase/core.BaseApp).CreateBackup` at `core/base_backup.go:44`

Observed counts:

- Region touchpoints: 1
- External surfaces: 0
- Registration sites: 0
- Wrapper chains: 12

Cost:

- Probe wall: 77,310ms
- `/usr/bin/time` real: 89.81s
- Peak RSS: 1,910,418,376 bytes
- Callgraph: 25,109ms
- Bridge seed discovery: 3,845ms
- Function-ref index: 3ms
- Function-value flow: 2ms

Budget stops:

- None.

Recovered evidence:

- Touchpoint: `(*BaseApp).registerAutobackupHooks$1$1 -> (*BaseApp).CreateBackup` with `EdgeStaticCall`
- Direct invoke wrapper at `core/base_backup.go:313`

Missing evidence:

- `registerAutobackupHooks` bootstrap binding through `OnBootstrap().BindFunc`
- `Cron.Add(jobId, rawSchedule, func(){ ... CreateBackup(...) })` callback registration
- Boundary family for cron/job/lifecycle activation

Gap class:

- Boundary-family coverage. The callback body is found, but the framework/lifecycle registration that makes it an activation boundary is not classified.

Classification: fixture-required / not currently viable.

## Evidence Shapes Needed By A Future Contract

If a normalized consumer contract is implemented after the gaps above are addressed, Phase 0 suggests it must carry:

- Region roots with source positions and symbol identity.
- Touchpoints from reverse callgraph evidence, even when no boundary was classified.
- Activation boundary candidates with boundary family/type evidence, not only endpoint terminology.
- Registration/bootstrap sites with source positions, static sink type, sink kind, and edge kind.
- Wrapper/path links with edge kind and source position.
- Explicit unsupported or partial-gap evidence for cases like Gitea SSE route registration and PocketBase cron/bootstrap.
- A precision/confidence marker so broad registrations from large shared regions do not masquerade as exact endpoint paths.

It should not carry oracle traces, bridge coverage, phase timings, RSS, raw stats, budget stops, or diagnostics as consumer data.

## Recommended Next Sprint

Run a boundary-predicate sprint before cut-point selection or consumer integration:

- Add structural predicates for lifecycle hooks, cron/job registration, queue worker handlers, and route registration patterns not expressed as direct `net/http.Handler` sinks.
- Fix bridge duration enforcement so large package scans cannot exceed the configured duration by minutes.
- Add a smaller Gitea harness or fixture for `Events -> Manager.Register` and issue-indexer queue worker shapes before making Gitea a required full-corpus validation target.
- Then rerun this candidate set and implement the normalized contract only if Gitea-like and background/job rows can produce stable evidence or explicit first-class unsupported-gap records.
