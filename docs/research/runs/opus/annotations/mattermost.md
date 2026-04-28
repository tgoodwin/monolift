# mattermost annotation — SPRINT-0013 (opus run)

**Corpus pin:** 2026-04-19. 2153 Go files.
No committed golden report; corpus walked via mandatory subagent
delegation per sprint plan.

## Target synthesis

Mattermost's refusal surface concentrates in **two subdirectories**
(`server/channels/app` at 477 files and `server/platform` at 106 files,
and the `jobs` subtree at 72) that together carry the bulk of
long-lived concurrency. The other 1,500+ files are request-scoped
handlers (`api4`, `web`, `wsapi`), data access (`store`, `db`), or CLI
bootstrap (`cmd`, `config`) with no distribution archetype surface.

**Hypothesis-prime disposition.** The sprint plan primed the subagent
with the hypothesis that mattermost stresses `event-bus-publisher/subscriber`
and `session-scoped-state`. The research refines that: the primary
stress is **`websocket-fanout-hub`** — a hub-per-node, user→hash→hub,
connections→per-conn-send-queue pattern that is a *specialized form*
of `sharded-keyed-state` + `event-bus-publisher`, but with two
additional invariants (node-local connection affinity, connection-scoped
send queues as backpressure) that the v0 archetype names do not
capture. This suggests a catalog entry either (a) as a separate
archetype with a clear emission sketch, or (b) as a composite.

**Competing-archetype boundaries surfaced:**
- Session-to-hub affinity (GetHubForUserId) enforces singleton
  ownership per node; cluster messages break that boundary. Two
  archetypes (`session-scoped-state` + `event-bus-subscriber`) both
  fit the hub-invalidation path.
- Cluster event dispatch (`Publish`, `PublishSkipClusterSend`) is
  ADMITTED as stateless event dispatch — this is `event-bus-publisher`
  in its purest form. Does *not* compete with the hub fanout.

**Hard ambiguities / data races found during walk:**
- **Email batching** (`email_batching.go:141-158, 162`): confirmed
  data race — `Add()` sends to `newNotifications` channel while
  `CheckPendingEmails()` drains channel into `pendingNotifications`
  map without locking during map iteration. TERMINAL in its current
  form, but AUTO-eligible after a safe actor-model transform.
- **Session cache scan-then-delete** (`session.go:48-93`): Scan →
  GetMulti → RemoveMulti sequence assumes external `cache.Cache`
  interface atomicity, which the compiler cannot verify. SUGGEST in
  v1 pending an external-contract mechanism.

**Evidence gaps.**
- `ps.Go()` semantics — is it a bounded thread-pool or unbounded
  spawn? Affects whether cluster-leader listeners are AUTO or
  TERMINAL.
- Hub `SendMessage` call sites — always from hub's select loop (safe)
  or from external context (race)?

## Owned-directory bundle file counts

| bundle | file count | refusable density |
|---|---|---|
| server/channels/api4 | 157 | low |
| server/channels/web | 21 | low |
| server/channels/wsapi | 5 | low |
| server/channels/app | 477 | **high** |
| server/channels/jobs | 72 | medium |
| server/channels/store | 316 | low (delegates to DB) |
| server/channels/db | 1 | n/a |
| server/platform | 106 | **high** |
| server/cmd | 165 | low |
| server/config | 26 | low |

**Tracked total: 1,346; full corpus: 2,153. Residual ~800 files are
tests, utilities, shared libs — no archetype surface expected.**

## AUTO set (per bundle)

### server/platform (websocket hub, cluster, session caching)

| # | region | archetype | state class | transform | evidence | gap |
|---|---|---|---|---|---|---|
| MM1 | `Hub` struct (14 chan fields + hubConnectionIndex), web_hub.go:77-120, 527-763 | `websocket-fanout-hub` (proposed specialization of `sharded-keyed-state` + `event-bus-publisher`) | `sharded-keyed-state` w/ session-affinity + `fanout-publisher` composite | replace 14 channels + select loop with command-queue actor; broadcast as async task-spawn to per-conn send queues | 14 chan fields; `hubSemaphore chan struct{}`; select loop at 527-763; hubConnectionIndex map nested access | per-hub connection isolation under cluster sync (cluster nodes may have different connection sets for same user) |
| MM2 | `WebConn` session state + deadQueue, web_conn.go:88-149 | `session-scoped-state` | `session-affinity-state` | session → explicit session-store with generation counter; deadQueue → request-scoped buffer; connection lifetime → actor (register/event-process/unregister) | 6 atomic fields (sessionToken, session, connectionID, ...); send channel + deadQueue circular buffer; reuseCount | HA cluster spawning multiple WebConns for same UserId — session-affinity invariant violation |
| MM3 | Hub invalidation (cluster sync), web_hub.go:459-464, cluster_handlers.go:97-102 | `event-bus-subscriber` (inbound) | `serialized-actor` | fold 6-way select into single command-queue enum; InvalidateUserCmd, UpdateActivityCmd, ... | non-blocking send to `invalidateUser` chan; connIndex.ForUser mutation during iteration | connIndex race on cluster message during unregister |
| MM4 | Session cache, session.go:44-97 (AddSessionToCache, ClearUserSessionCacheLocal) | `ttl-cache-managed` | `ttl-cache` | external `cache.Cache` → actor-managed TTL state with explicit eviction commands | SetWithExpiry; Scan/GetMulti/RemoveMulti sequence; cluster-reliable invalidation message | external-cache atomicity contract (compiler cannot verify) |
| MM5 | Status cache + cluster handler, cluster_handlers.go:42-51 | `event-bus-subscriber` | `serialized-actor` | status-actor owns cache + ClusterMessage command queue | SetWithDefaultExpiry from handler; `additionalClusterHandlers` map written at init | cache eviction model: TTL or explicit invalidation? |

### server/channels/app (notifications, event publishing)

| # | region | archetype | state class | transform | evidence | gap |
|---|---|---|---|---|---|---|
| MM6 | `PushNotificationsHub`, notification_push.go:44-52, 393-482 | `worker-pool-consumer` | `bounded-worker-pool` | replace `notificationsChan` + `sema chan struct{}` with explicit worker-pool actor; bounded queue + overflow fallback | `notificationsChan` buffered + `sema` for NumCPU*8 concurrency; Add-to-semaWg + spawn-goroutine + defer-release pattern; drain+close shutdown | buffer-full fallback (line 59: "buffer was full" → immediate mail) must be preserved by transform |
| MM7 | Event publishing, cluster.go:189-234 (Publish, PublishSkipClusterSend) | `event-bus-publisher` | (already ADMITTED) | keep as-is; cluster dispatch is idempotent command receiver | already stateless; ToJSON + ClusterMessage.SendType branching | SharedChannelSyncHandler blocking behavior? |
| MM8 | Notification batching, email/email_batching.go:71-159 | `periodic-scheduler` + `ttl-cache-managed` composite | `periodic-invocation` + `ttl-cache` | replace `taskMutex` + task-swap with actor-owned task handle; replace unlocked `pendingNotifications` map with actor-owned per-user keyed queue; timer-driven CheckPendingEmails → actor wakeup | **CONFIRMED DATA RACE**: line 141-158 drains channel into map; line 162 iterates map without lock; concurrent Add+CheckPending races | current code is refused correctly; lift moves it to AUTO by making the map actor-owned |

### server/channels/jobs

| # | region | archetype | state class | transform | evidence | gap |
|---|---|---|---|---|---|---|
| MM9 | Job scheduler base, base_schedulers.go:14-90 | `periodic-scheduler` | `periodic-invocation` | ADMITTED shape; verify JobServer.Start does not use channels | no internal state mutation between ticks; callback style | JobServer invocation loop — must confirm single-threaded per job instance |
| MM10 | Worker pattern (example: delete_expired_posts/worker.go:29-80) | per-job worker | `periodic-invocation` | ADMITTED — callback-driven factory, no channel/goroutine | execute is callback; time.Sleep OK for blocking job worker | SimpleWorker.Do() semantics |

### server/platform (cluster, listeners)

| # | region | archetype | state class | transform | evidence | gap |
|---|---|---|---|---|---|---|
| MM11 | Cluster leader listeners, cluster.go:164-187 | `event-bus-subscriber` (fanout to callbacks) | `serialized-actor` | listener registry as actor; InvokeListeners as command; replaces `ps.Go(...)` unbounded spawn | `sync.Map` for listener registry; `ps.Go` to invoke | is `ps.Go` bounded or unbounded? |

## SUGGEST set

| # | region | archetype | why SUGGEST |
|---|---|---|---|
| MMS1 | Session expiry invalidation, session.go:239-250, web_conn.go:89 | `event-bus-subscriber` | coupled atomic.Load/Store across WebConn + platform; no explicit TTL callback |
| MMS2 | Plugin event hooks, notification_push.go:95-106 RunMultiHook | `event-bus-subscriber` (async) | synchronous hook invocation blocks publisher; plugins may be untrusted |
| MMS3 | Busy-state cluster events, cluster_handlers.go:83-95 | `serialized-actor` | Busy field mutated by cluster + local; no lock visible, race possible |
| MMS4 | Direct-message fanout, web_hub.go:481-487 SendMessage / directMsg | `session-scoped-state` | call-site verification: always from hub's select loop or external? |

## TERMINAL set

| # | region | reason |
|---|---|---|
| MMT1 | `server/channels/api4`, `web`, `wsapi` | request-scoped handlers; stateless; ADMITTED baseline |
| MMT2 | `server/channels/store` | DB drivers + query builders; delegation; not runtime concurrency |
| MMT3 | `server/cmd`, `server/config` | one-time initialization; outside runtime distribution scope |
| MMT4 | `email_batching` **as currently written** | data race (see MM8); cannot lift without actor-transform first. AUTO-eligible only under the proposed transform. |
| MMT5 | session cache scan-then-delete **as currently written** | depends on external `cache.Cache` interface atomicity the compiler cannot verify. Move to AUTO (MM4) only with an external-contract mechanism (pragma or trusted-lib allowlist). |

## ADMITTED set

- Event publishing (`cluster.Publish*`) — stateless dispatcher;
  idempotent.
- Job worker invocation (callback-driven factory pattern).
- Team/channel service (`teams/service.go` WebHub interface) — call-back
  based Publish.

## Per-bundle coverage ledger

| bundle | files | finding |
|---|---|---|
| server/channels/api4 | 157 | no relevant archetype surface observed — stateless request handlers |
| server/channels/web | 21 | no relevant archetype surface observed — api4 wrappers |
| server/channels/wsapi | 5 | no relevant archetype surface observed — Hub delegators |
| server/channels/app | 477 | 3 AUTO (MM6, MM7 ADMITTED, MM8), 2 SUGGEST (MMS1, MMS2), 1 TERMINAL-convertible (MMT4) |
| server/channels/jobs | 72 | 2 AUTO/ADMITTED (MM9, MM10) |
| server/channels/store | 316 | no relevant archetype surface observed — abstraction layer, delegates to sqlstore |
| server/channels/db | 1 | no relevant archetype surface observed — schema |
| server/platform | 106 | 5 AUTO (MM1, MM2, MM3, MM4, MM5), 1 AUTO (MM11), 2 SUGGEST (MMS3, MMS4), 1 TERMINAL-convertible (MMT5) |
| server/cmd | 165 | no relevant archetype surface observed — bootstrap |
| server/config | 26 | no relevant archetype surface observed — data structures |

## Subagent dispatch log

| dispatch | subsystems | prompt version | return summary | re-dispatch? |
|---|---|---|---|---|
| #1 | ALL bundles (via `rg`-based fanout discovery) | v1 (Phase 0 template, hypothesis-prime framing, AUTO-focus) | 11 AUTO, 4 SUGGEST, 5 TERMINAL (including 2 data-race-noted regions); file counts recorded; every bundle covered | no — return met schema, every AUTO has transform + state class, hypothesis-prime disposition surfaced (websocket-fanout-hub refines event-bus-publisher) |

Parent-agent spot check: verified MM1 (web_hub.go select loop width),
MM8 (email_batching.go race at line 141-158 vs. line 162), MM6
(notification_push.go semaphore pattern). All claims reproduce. The
`websocket-fanout-hub` proposed archetype is the right *specialization*
but under the v1 discipline it should be expressed as a composite
(`sharded-keyed-state` + `fanout-publisher` + per-connection
send-queue) rather than a new single archetype name.

**Net:** 11 AUTO, 4 SUGGEST, 5 TERMINAL. New archetype candidate
(`websocket-fanout-hub`) proposed but the v1 discipline handles it as
a composite of existing archetypes — see catalog retirement note.
