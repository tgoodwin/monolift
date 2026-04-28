# Evaluation ideas v1 - gpt-5.4 run

These are research scenarios, not measurements. Each is a candidate demo, benchmark story, or paper-motivating example surfaced by the usefulness analysis.

## 1. Queue burst absorber

- Archetypes: `bounded-worker-pool`
- Best anchor regions: `listmonk L2` - `evaluation/listmonk/internal/manager/manager.go:462`, `Manager.worker`; `gitea G1` - `evaluation/gitea/modules/queue/workergroup.go:92`, `WorkerPoolQueue.doWorkerHandle`; `mattermost MM6` - `evaluation/mattermost/server/channels/jobs/server.go:17`, `JobServer`
- Why it is a good utility demo: these regions already expose the queue boundary in source, so the demo can show Monolift moving from in-process buffering to remote workers without rewriting the worker body.
- What to observe qualitatively: whether bursts stay contained in the queue boundary, whether the web-facing part of the app remains simpler when demand is quiet, and whether retries/backpressure stay legible after lifting.

## 2. Scheduler extraction without application surgery

- Archetypes: `periodic-invocation`
- Best anchor regions: `miniflux M1-M4`; `listmonk L3` - `evaluation/listmonk/internal/bounce/bounce.go:135`, `runMailboxScanner`; `pocketbase P2` - `evaluation/pocketbase/tools/cron/cron.go:20`, `Cron`; `mattermost MM8` - `evaluation/mattermost/server/channels/jobs/base_schedulers.go:15`, `PeriodicScheduler`
- Why it is a good utility demo: this is the cleanest way to show the paper's incremental-adoption thesis. The same loop body can remain in source while ownership of cadence moves to the platform.
- What to observe qualitatively: whether the main service sheds always-on housekeeping responsibility, whether duplicate or skipped runs remain acceptable, and whether the transform preserves developer-visible control flow.

## 3. Realtime connection isolation

- Archetypes: `session-affinity-state`, `fanout-publisher`, `keyed-partitioned-state`
- Best anchor regions: `mattermost MM1` - `evaluation/mattermost/server/channels/app/platform/web_hub.go:812`, `hubConnectionIndex`; `mattermost MM2` - `evaluation/mattermost/server/channels/app/platform/web_conn.go:88`, `WebConn`; `gitea G7` - `evaluation/gitea/modules/eventsource/messenger.go:9`, `Messenger`; `caddy C6` - `evaluation/caddy/modules/caddyhttp/reverseproxy/streaming.go:147-159`, `handleUpgradeResponse`
- Why it is a good utility demo: it shows a case where usefulness is not just scaling. Sticky ownership and fanout isolation reduce blast radius for long-lived connections, which is a strong complement to the queue-and-scheduler story.
- What to observe qualitatively: whether slow or noisy clients stay isolated, whether reconnect/replay semantics remain understandable, and whether the lift introduces an operationally acceptable affinity story.

## 4. Root narrowing as a utility enabler

- Archetypes: `periodic-invocation`, `fanout-publisher`, `serialized-actor`
- Best anchor regions: PocketBase `core.App` terminal root - `evaluation/pocketbase/core/app.go:29`, `App`; narrower useful regions `pocketbase P2` - `evaluation/pocketbase/tools/cron/cron.go:20`, `Cron`; `pocketbase P4` - `evaluation/pocketbase/tools/subscriptions/broker.go:11`, `Broker`; `pocketbase P5` - `evaluation/pocketbase/tools/logger/batch_handler.go:81`, `BatchHandler`
- Why it is a good utility demo: it shows that Monolift utility is not "split the whole monolith." The useful compiler behavior is often selecting or suggesting narrower roots inside an otherwise terminal application root.
- What to observe qualitatively: whether the compiler's explanation steers the user away from a bad root and toward a narrow, high-payoff lift; whether the transform preserves the rest of the app unchanged.

## 5. Shared-cache payoff versus local-cache penalty

- Archetypes: `ttl-cache`
- Best anchor regions: `mattermost MM4-MM5` - `evaluation/mattermost/server/channels/app/platform/session.go:45`, `PlatformService.AddSessionToCache`; `evaluation/mattermost/server/channels/app/platform/status.go:19`, `PlatformService.AddStatusCache`; `gitea G10` - `evaluation/gitea/modules/cache/ephemeral.go:18`, `EphemeralCache`; negative anchor `caddy C7` - `evaluation/caddy/modules/caddyhttp/caddyauth/basicauth.go:105-110`, `HTTPBasicAuth.Cache`
- Why it is a good utility demo: this scenario can make a subtle point the paper needs. Some liftable cache regions are worth sharing across replicas; others should stay local because the remote hit path becomes worse than the duplicate local cache.
- What to observe qualitatively: whether the lifted cache reduces duplicated warmup or cross-replica inconsistency, and whether request-hot lookups become observably more fragile or operationally heavier.

## 6. Intentional refusal as part of utility

- Archetypes: negative control
- Best anchor regions: reverse-proxy hot path in Caddy - `evaluation/caddy/modules/caddyhttp/reverseproxy/reverseproxy.go:50-51`, package-global `inFlightRequests`; `evaluation/caddy/modules/caddyhttp/reverseproxy/reverseproxy.go:102`, `Handler`
- Why it is a good utility demo: the usefulness story is incomplete if Monolift only shows successes. Caddy demonstrates that saying "do not lift this" is part of delivering utility, because protocol-critical coordination can be harmed by distribution.
- What to observe qualitatively: whether the explanation clearly distinguishes a real negative control from a missed compiler opportunity, and whether nearby positive regions such as `stayUpdated` or file storage help show that the refusal is selective rather than conservative everywhere.

## 7. Thin-wrapper cautionary demo

- Archetypes: `serialized-actor`, `session-affinity-state`
- Best anchor regions: `miniflux M6` - `evaluation/miniflux/internal/proxyrotator/proxyrotator.go:14`, `ProxyRotator`; `gitea G11-G13` - `evaluation/gitea/modules/session/db.go:93`, `DBProvider`; `evaluation/gitea/modules/session/redis.go:96`, `RedisProvider`
- Why it is a good utility demo: it surfaces a useful "not every liftable region is worth lifting" case. That is important for prioritization and for user trust in future compiler suggestions.
- What to observe qualitatively: whether a remote owner or session service creates any new placement leverage beyond what the existing external store already provides, and whether the operator cost dominates the benefit.

## Recommended demo sequence

1. Start with `bounded-worker-pool` on listmonk or gitea.
2. Follow with `periodic-invocation` on miniflux or pocketbase.
3. Use Mattermost realtime state as the advanced composite demo.
4. Include the Caddy negative control to show discrimination, not just transform generation.

That sequence tracks the paper's utility thesis closely: start with the easiest cases where dynamic placement is obviously useful, then widen into more conditional but more visually compelling connection-state stories.
