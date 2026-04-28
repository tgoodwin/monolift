# Per-archetype cards v1 - gpt-5.4 run

## 1. `serialized-actor`

- Pays off when: the code already wants one owner and the value of lifting is isolation, fail-stop containment, or independent placement of that owner rather than parallelism. Strong examples are PocketBase hook or batch registries and Gitea process management, where many callers already funnel through one mutable owner (`pocketbase P1` - `evaluation/pocketbase/tools/hook/hook.go:54`, `Hook[T]`; `pocketbase P5` - `evaluation/pocketbase/tools/logger/batch_handler.go:81`, `BatchHandler`; `gitea G18` - `evaluation/gitea/modules/process/manager.go:70`, `Manager`).
- Net-negative when: the mutable state is tiny, lookup frequency is high, and remote ownership would turn a cheap in-process decision into a synchronous network hop. Miniflux `ProxyRotator` is the clearest low-payoff case (`miniflux M6` - `evaluation/miniflux/internal/proxyrotator/proxyrotator.go:14`, `ProxyRotator`).
- Code-structural tells: one mutex or narrow serialized access path; receiver-owned state; a small command-style API; no need for shared-memory aliasing across callers.
- New failure modes introduced: owner outage, mailbox backlog, duplicate-owner split brain during deploy, loss of in-memory state on restart if state is not externalized.
- Operational complexity added: owner election or pinning, mailbox depth monitoring, request/reply timeouts, command schema compatibility across deploys.
- Consistency or ordering trade-offs: per-owner ordering is preserved, but formerly in-process operations now depend on network availability; cross-owner atomicity is not implied.
- Corpus regions where lifting seems plausibly useful:
- `pocketbase P1` - `evaluation/pocketbase/tools/hook/hook.go:54`, `Hook[T]`
- `pocketbase P5` - `evaluation/pocketbase/tools/logger/batch_handler.go:81`, `BatchHandler`
- `gitea G18` - `evaluation/gitea/modules/process/manager.go:70`, `Manager`
- Corpus regions where lifting seems not useful despite being liftable:
- `miniflux M6` - `evaluation/miniflux/internal/proxyrotator/proxyrotator.go:14`, `ProxyRotator`

## 2. `bounded-worker-pool`

- Pays off when: work already enters a queue, items are serializable, and the caller already tolerates eventual completion or retry. The strongest evidence is listmonk campaign delivery, Gitea queue/indexer workers, and Mattermost job workers (`listmonk L2` - `evaluation/listmonk/internal/manager/manager.go:462`, `Manager.worker`; `gitea G1` - `evaluation/gitea/modules/queue/workergroup.go:92`, `WorkerPoolQueue.doWorkerHandle`; `mattermost MM6` - `evaluation/mattermost/server/channels/app/notification_push.go:44`, `PushNotificationsHub`; `evaluation/mattermost/server/channels/jobs/server.go:17`, `JobServer`).
- Net-negative when: the queue is narrow, low-volume, or already mostly serving as an internal buffer rather than a scaling boundary. Listmonk bounce processing is liftable, but less valuable than the campaign or jobs examples because the queue is single-purpose and the callback path is already localized (`evaluation/listmonk/internal/bounce/bounce.go:118`, `Manager.Run`).
- Code-structural tells: explicit buffered channel or queue object; worker goroutines; serializable job payloads; handler body already funnels effects through external DB, callback, or API client.
- New failure modes introduced: broker outage, poison jobs, duplicate delivery, stuck backlog, visibility lag between enqueue and completion.
- Operational complexity added: broker deployment, queue naming and retention, dead-letter handling, worker autoscaling, retry policy ownership.
- Consistency or ordering trade-offs: usually shifts from in-process FIFO assumptions to at-least-once delivery with looser global ordering; idempotent handlers become more important.
- Corpus regions where lifting seems plausibly useful:
- `listmonk L2` - `evaluation/listmonk/internal/manager/manager.go:462`, `Manager.worker`
- `gitea G1` - `evaluation/gitea/modules/queue/workergroup.go:92`, `WorkerPoolQueue.doWorkerHandle`
- `mattermost MM6` - `evaluation/mattermost/server/channels/jobs/server.go:17`, `JobServer`
- Corpus regions where lifting seems not useful despite being liftable:
- `evaluation/listmonk/internal/bounce/bounce.go:118`, `Manager.Run`

## 3. `periodic-invocation`

- Pays off when: the body is clearly cadence-driven, idempotence is plausible, and the loop is background work rather than part of a request path. Miniflux schedulers, listmonk mailbox scanning, pocketbase cron, and Mattermost scheduler layers all match this (`miniflux M1-M4`; `listmonk L3` - `evaluation/listmonk/internal/bounce/bounce.go:135`, `runMailboxScanner`; `pocketbase P2` - `evaluation/pocketbase/tools/cron/cron.go:20`, `Cron`; `mattermost MM8` - `evaluation/mattermost/server/channels/jobs/base_schedulers.go:15`, `PeriodicScheduler`).
- Net-negative when: the loop is only light housekeeping and moving it out creates more scheduler wiring than user-visible benefit. Caddy `stayUpdated` and `keepStorageClean` are liftable but weaker utility stories than the queue and hub examples (`caddy C1` - `sessiontickets.go:114-148`, `stayUpdated`; `caddy C2` - `tls.go:1050-1072`, `keepStorageClean`).
- Code-structural tells: ticker or sleep loop; clear body/cadence split; clean `Start`/`Stop` lifecycle; body already callable independently.
- New failure modes introduced: missed triggers, duplicate runs, overlapping runs during slow execution, scheduler outage, drift between code and schedule config.
- Operational complexity added: scheduler registration, overlap prevention, run history, alerting on missed executions, run-specific auth/credentials.
- Consistency or ordering trade-offs: typically weakens guarantees from exactly-once-in-process to skip/duplicate-tolerant execution; bodies must be idempotent or compensating.
- Corpus regions where lifting seems plausibly useful:
- `miniflux M1-M4` - feed, cleanup, watchdog, and metrics scheduler family
- `pocketbase P2` - `evaluation/pocketbase/tools/cron/cron.go:20`, `Cron`
- `mattermost MM8` - `evaluation/mattermost/server/channels/jobs/base_schedulers.go:15`, `PeriodicScheduler`
- Corpus regions where lifting seems not useful despite being liftable:
- `caddy C1` - `sessiontickets.go:114-148`, `stayUpdated`
- `caddy C2` - `tls.go:1050-1072`, `keepStorageClean`

## 4. `keyed-partitioned-state`

- Pays off when: access is already by stable key and the reason to lift is shard-local isolation or routing, not whole-map coordination. Mattermost `hubConnectionIndex` is the strongest example because user and connection identifiers are already first-class routing keys (`mattermost MM1` - `evaluation/mattermost/server/channels/app/platform/web_hub.go:812`, `hubConnectionIndex`).
- Net-negative when: the keyed map is only a local index over state that is really owned elsewhere, or when the keyed structure participates in a protocol-sensitive path. Listmonk `Manager.pipes` is liftable, but alone it is a weak payoff because campaign truth and worker execution live outside the map (`listmonk L5` - `evaluation/listmonk/internal/manager/manager.go:72-81`, `Manager.pipes`).
- Code-structural tells: mutex-protected map; input-derived key on every meaningful operation; little cross-key iteration in user-visible paths.
- New failure modes introduced: hot shards, shard-ownership bugs, rebalancing mistakes, partial shard outage, cross-shard query surprises.
- Operational complexity added: routing layer, shard membership management, rebalance tooling, per-shard observability, background repair jobs.
- Consistency or ordering trade-offs: preserves per-key locality well, but weakens or complicates whole-map iteration and cross-key invariants.
- Corpus regions where lifting seems plausibly useful:
- `mattermost MM1` - `evaluation/mattermost/server/channels/app/platform/web_hub.go:812`, `hubConnectionIndex`
- `caddy C5` - `evaluation/caddy/modules/caddyhttp/reverseproxy/streaming.go:302-324`, `Handler.connections`
- Corpus regions where lifting seems not useful despite being liftable:
- `listmonk L5` - `evaluation/listmonk/internal/manager/manager.go:72-81`, `Manager.pipes`

## 5. `fanout-publisher`

- Pays off when: the publisher already has a clear subscriber set and usefulness comes from isolating slow or independent consumers behind a broker boundary. PocketBase `Broker` and Gitea `Messenger` are the cleanest examples (`pocketbase P4` - `evaluation/pocketbase/tools/subscriptions/broker.go:11`, `Broker`; `gitea G7` - `evaluation/gitea/modules/eventsource/messenger.go:9`, `Messenger`).
- Net-negative when: delivery semantics are intentionally process-local, best-effort, or not yet explicit enough to survive transport changes. Listmonk `Events.Publish` is the cautionary case because queue-full and drop behavior are part of the current meaning (`listmonk L4` - `evaluation/listmonk/internal/events/events.go:41`, `Events.Publish`).
- Code-structural tells: subscriber registry, publish loop over channels or listeners, event payloads already serializable, minimal coupling between subscribers.
- New failure modes introduced: broker unavailability, message replay, slow-consumer backlog, leaked subscriptions, redelivery after reconnect.
- Operational complexity added: topic management, subscription lifecycle, replay/retention policy, consumer lag monitoring, schema compatibility.
- Consistency or ordering trade-offs: subscriber independence usually improves, but ordering across subscribers and exactly-once delivery usually weaken unless explicitly rebuilt.
- Corpus regions where lifting seems plausibly useful:
- `pocketbase P4` - `evaluation/pocketbase/tools/subscriptions/broker.go:11`, `Broker`
- `gitea G7` - `evaluation/gitea/modules/eventsource/messenger.go:9`, `Messenger`
- `mattermost MM7` - `evaluation/mattermost/server/channels/app/platform/cluster.go:189`, `PlatformService.Publish`
- Corpus regions where lifting seems not useful despite being liftable:
- `listmonk L4` - `evaluation/listmonk/internal/events/events.go:41`, `Events.Publish`

## 6. `ttl-cache`

- Pays off when: cache contents are derivative, serialization is straightforward, and a shared cache or centralized eviction policy removes duplicated work across replicas. Mattermost session/status caches and gitea ephemeral cache fit that story (`mattermost MM4-MM5` - `evaluation/mattermost/server/channels/app/platform/session.go:45`, `PlatformService.AddSessionToCache`; `evaluation/mattermost/server/channels/app/platform/status.go:19`, `PlatformService.AddStatusCache`; `gitea G10` - `evaluation/gitea/modules/cache/ephemeral.go:18`, `EphemeralCache`).
- Net-negative when: the cache exists to keep a request-hot path in local memory, so lifting converts a local hit into a network hop. Caddy's BasicAuth cache is the clearest example (`caddy C7` - `evaluation/caddy/modules/caddyhttp/caddyauth/basicauth.go:105-110`, `HTTPBasicAuth.Cache`).
- Code-structural tells: map or `sync.Map`; TTL metadata or cleanup goroutine; obvious source of truth elsewhere; narrow `Get`/`Set`/`Clean` surface.
- New failure modes introduced: cache outage, stale reads, eviction mismatch across deploys, thundering-herd fallback to source of truth.
- Operational complexity added: managed cache deployment, TTL policy, serialization/versioning of values, hit-rate and eviction observability.
- Consistency or ordering trade-offs: usually weakens read freshness in exchange for wider sharing; invalidation becomes explicit instead of incidental.
- Corpus regions where lifting seems plausibly useful:
- `mattermost MM4-MM5` - `evaluation/mattermost/server/channels/app/platform/session.go:45`, `PlatformService.AddSessionToCache`; `evaluation/mattermost/server/channels/app/platform/status.go:19`, `PlatformService.AddStatusCache`
- `gitea G10` - `evaluation/gitea/modules/cache/ephemeral.go:18`, `EphemeralCache`
- Corpus regions where lifting seems not useful despite being liftable:
- `caddy C7` - `evaluation/caddy/modules/caddyhttp/caddyauth/basicauth.go:105-110`, `HTTPBasicAuth.Cache`

## 7. `session-affinity-state`

- Pays off when: there is a stable session or connection key, state lifetime is bounded by that key, and sticky routing isolates noisy clients cleanly. Mattermost `WebConn` is the best evidence; Caddy upgrade handling is the smaller but structurally similar case (`mattermost MM2` - `evaluation/mattermost/server/channels/app/platform/web_conn.go:88`, `WebConn`; `caddy C6` - `evaluation/caddy/modules/caddyhttp/reverseproxy/streaming.go:147-159`, `handleUpgradeResponse`).
- Net-negative when: the durable source of truth already lives in DB or Redis and the in-process code is a thin wrapper, so lifting mainly inserts another service hop. Gitea's session providers are useful only if session routing or isolation matters more than the already-externalized backing store (`gitea G11-G13` - `evaluation/gitea/modules/session/db.go:93`, `DBProvider`; `evaluation/gitea/modules/session/redis.go:96`, `RedisProvider`).
- Code-structural tells: explicit session or connection ID; bounded lifetime; per-session mutable state; register/unregister or open/close lifecycle.
- New failure modes introduced: affinity key misrouting, reconnect storms, stranded session ownership after deploy, uneven shard load from sticky users.
- Operational complexity added: affinity-aware load balancing, ownership drain or handoff, per-session debugging, reconnection policy tuning.
- Consistency or ordering trade-offs: strengthens per-session ordering but does not solve cross-session or cross-device consistency; multi-connection users remain tricky.
- Corpus regions where lifting seems plausibly useful:
- `mattermost MM2` - `evaluation/mattermost/server/channels/app/platform/web_conn.go:88`, `WebConn`
- `caddy C6` - `evaluation/caddy/modules/caddyhttp/reverseproxy/streaming.go:147-159`, `handleUpgradeResponse`
- Corpus regions where lifting seems not useful despite being liftable:
- `gitea G11-G13` - `evaluation/gitea/modules/session/db.go:93`, `DBProvider`; `evaluation/gitea/modules/session/redis.go:96`, `RedisProvider`

## 8. `filesystem-bound-singleton`

- Pays off when: local disk placement is itself the reason a subsystem cannot move, and replacing that assumption with object-store or sidecar access opens a real deployment option. Caddy file storage and Gitea local object storage are the main corpus anchors (`caddy C-FS` - `evaluation/caddy/modules/filestorage/filestorage.go:29`, `FileStorage`; `gitea G-FS` - `evaluation/gitea/modules/storage/local.go:23`, `LocalStorage`).
- Net-negative when: local POSIX semantics are part of the intended deployment and the transform becomes mostly a storage migration. Gitea local storage is still the best cautionary example because many installations may prefer a single-node local-disk model (`gitea G-FS` - `evaluation/gitea/modules/storage/local.go:23`, `LocalStorage`).
- Code-structural tells: structs own paths or file handles; methods call `os`, `filepath`, or direct file operations; little non-filesystem state between calls.
- New failure modes introduced: object-store outage, eventual-consistency surprises, stale reads after write, partial upload failure, credentials drift.
- Operational complexity added: bucket provisioning, credentials, lifecycle/retention policy, migration from existing on-disk data, larger observability surface.
- Consistency or ordering trade-offs: often trades strong local filesystem behavior for object-store semantics, especially around rename, directory walking, and visibility timing.
- Corpus regions where lifting seems plausibly useful:
- `caddy C-FS` - `evaluation/caddy/modules/filestorage/filestorage.go:29`, `FileStorage`
- `gitea G-FS` - `evaluation/gitea/modules/storage/local.go:23`, `LocalStorage`
- Corpus regions where lifting seems not useful despite being liftable:
- `gitea G-FS` - `evaluation/gitea/modules/storage/local.go:23`, `LocalStorage`
