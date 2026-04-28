# Prioritization implications v1 - gpt-5.4 run

## Bottom line

Usefulness does reorder the v1 landscape. Coverage still matters, but the paper's utility thesis is about low-commitment placement leverage under changing workload conditions, not about how many regions share a shape. On that axis, the first implementation targets should be the archetypes that already expose a queue, cadence, or sticky-routing boundary in source.

## Suggested usefulness-first ordering

| Rank | Archetype | Why it moves here |
|---|---|---|
| 1 | `bounded-worker-pool` | Strongest fit to the paper's utility story. The queue is already the placement boundary, the failure model is well understood, and the payoff is easy to demonstrate in listmonk, gitea, and Mattermost (`evaluation/listmonk/internal/manager/manager.go:462`, `Manager.worker`; `evaluation/gitea/modules/queue/workergroup.go:92`, `WorkerPoolQueue.doWorkerHandle`; `evaluation/mattermost/server/channels/jobs/server.go:17`, `JobServer`). |
| 2 | `periodic-invocation` | Also highly aligned to incremental adoption: it turns process-owned loops into scheduler-owned work without forcing callers to change. It has the broadest corpus spread, but more importantly it creates a clean "local when simple, external when needed" story (`evaluation/listmonk/internal/bounce/bounce.go:135`, `runMailboxScanner`; `evaluation/pocketbase/tools/cron/cron.go:20`, `Cron`; `evaluation/mattermost/server/channels/jobs/base_schedulers.go:15`, `PeriodicScheduler`). |
| 3 | `fanout-publisher` | Worth prioritizing ahead of some broader archetypes because it maps directly to existing broker infrastructure and provides visible isolation benefits for slow subscribers. PocketBase `Broker` and Gitea `Messenger` make good implementation anchors (`evaluation/pocketbase/tools/subscriptions/broker.go:11`, `Broker`; `evaluation/gitea/modules/eventsource/messenger.go:9`, `Messenger`). |
| 4 | `session-affinity-state` | Smaller corpus breadth than several other archetypes, but higher demo value. Mattermost `WebConn` and Caddy upgrade handling give a concrete story about isolating long-lived connection state with sticky routing (`evaluation/mattermost/server/channels/app/platform/web_conn.go:88`, `WebConn`; `evaluation/caddy/modules/caddyhttp/reverseproxy/streaming.go:147-159`, `handleUpgradeResponse`). |
| 5 | `ttl-cache` | Often useful, but less central to the paper's dynamic-distribution thesis. Many cache regions are liftable but only conditionally valuable because a remote cache can add latency on hits (`evaluation/mattermost/server/channels/app/platform/session.go:45`, `PlatformService.AddSessionToCache`; `evaluation/mattermost/server/channels/app/platform/status.go:19`, `PlatformService.AddStatusCache`; `evaluation/caddy/modules/caddyhttp/caddyauth/basicauth.go:105-110`, `HTTPBasicAuth.Cache`). |
| 6 | `serialized-actor` | Real surface area, but usefulness is much more conditional. Many regions are liftable because they already have one owner; fewer are worth lifting because they create enough new leverage to justify a remote owner (`evaluation/pocketbase/tools/hook/hook.go:54`, `Hook[T]`; `evaluation/gitea/modules/process/manager.go:70`, `Manager`; `evaluation/miniflux/internal/proxyrotator/proxyrotator.go:14`, `ProxyRotator`). |
| 7 | `keyed-partitioned-state` | Important, but riskier as an early utility bet because cross-key invariants and scan behavior make payoff highly workload-dependent. Mattermost shows why it matters; listmonk shows why it is easy to overclaim (`evaluation/mattermost/server/channels/app/platform/web_hub.go:812`, `hubConnectionIndex`; `evaluation/listmonk/internal/manager/manager.go:72-81`, `Manager.pipes`). |
| 8 | `filesystem-bound-singleton` | Keep in the catalog, but late in implementation order. It is often a storage migration or deployment-architecture change more than a Monolift runtime-placement win (`evaluation/caddy/modules/filestorage/filestorage.go:29`, `FileStorage`; `evaluation/gitea/modules/storage/local.go:23`, `LocalStorage`). |

## What this means for sprint sequencing

### First wave

- `bounded-worker-pool`
- `periodic-invocation`

These are the best first pair because they best match the paper's claim that the same source should support both simpler and more distributed placements depending on workload. They also have the cleanest operational story: broker or scheduler is already a familiar external substrate, and the source regions already look like queues or loops.

### Second wave

- `fanout-publisher`
- `session-affinity-state`

These have strong demo value, especially when paired. Mattermost and Gitea both suggest that once the compiler can lift a fanout publisher and a sticky session owner, the most convincing realtime demos become available. I would still stage them after the queue and scheduler work because their routing and replay semantics are less forgiving.

### Third wave

- `ttl-cache`
- `serialized-actor`

These are better treated as conditional utilities. The compiler should probably present them as stronger suggestions before it aggressively auto-applies them, except in the clearest regions. The cache and actor shapes are common, but common does not mean valuable.

### Last wave

- `keyed-partitioned-state`
- `filesystem-bound-singleton`

Both matter, but both need stricter usefulness discipline. `keyed-partitioned-state` is easy to misread when the map is only a local index. `filesystem-bound-singleton` often asks the user to accept a new storage substrate, which is a larger operational decision than the paper's canonical queue/scheduler lifts.

## Specific reorderings relative to a breadth-first reading of v1

- `session-affinity-state` should move up. Its corpus breadth is smaller than `serialized-actor` or `ttl-cache`, but its payoff is more legible and more aligned with the paper's "placement follows workload" argument because long-lived connections are exactly where placement matters.
- `serialized-actor` should move down. The v1 research surfaced many actor-like regions, but the usefulness work shows that many are thin ownership wrappers rather than compelling distributed services.
- `keyed-partitioned-state` should move down unless paired with a stronger demo surface. Mattermost's hub index is convincing; listmonk's keyed maps are not.
- `filesystem-bound-singleton` should stay catalogued but late. It is a good exception archetype, not a good first utility demo.

## Auto-apply vs suggest implications

- `bounded-worker-pool` and `periodic-invocation` look like the safest early auto-apply candidates once their existing evidence thresholds are met.
- `fanout-publisher` can likely auto-apply in the cleanest broker-shaped regions, but only after replay and subscriber semantics are explicit enough.
- `serialized-actor`, `ttl-cache`, and `keyed-partitioned-state` are the archetypes most likely to justify a suggestion-first posture in borderline cases, because the usefulness penalty for unnecessary lifting is high.
- `filesystem-bound-singleton` should probably be suggestion-first by default even if the classifier can recognize it, because the operational substrate decision is more consequential than for queue or scheduler lifts.

## Alignment with the paper

This ranking stays aligned with the paper's view of utility. The paper argues for rapid exploration and dynamic placement under changing run-time conditions. That naturally favors queues, schedulers, and some session-affinity surfaces over archetypes that mostly amount to "make this singleton remote" or "replace local disk with object storage."
