# caddy annotation — SPRINT-0013 (opus run)

**Corpus pin:** 2026-04-19. 306 Go files.
Golden report: `test/e2e/targets/caddy/golden/report.json` (35k lines);
walked source for this note rather than the report, per plan.

## Target synthesis

Caddy's refusal surface concentrates in **systems-level background
tasks** (TLS maintenance, connection lifecycle) and **caching/pooling**
(BasicAuth cache, Handler.connections), *not* in admitted handlers
themselves. This aligns with the sprint premise that handlers are
admitted but their state dependencies are refused. The three dominant
refused archetypes are:

1. **`periodic-scheduler` loops** — TLS storage cleaner
   (`keepStorageClean`), SessionTicketService key rotation
   (`stayUpdated`) — goroutine + ticker + stop channel, lifecycle bound
   to provisioning phase.
2. **`pipeline-stage` goroutines** — STEK providers (distributed and
   standard) each spawn `go rotate(doneChan, keysChan)`: a one-stage
   input→output channel transform.
3. **`singleton-actor` with deferred cleanup** — reverse-proxy
   `Handler.connections` map + `connectionsMu` + optional
   `time.AfterFunc` cleanup timer; BasicAuth cache with
   `singleflight.Group`.

**Hardest ambiguity.** The reverse-proxy `Handler` is *admitted* (via
`//monolift:lift`), but its reachable state (`connections`, cleanup
timer) is refused. Does the state lift follow the handler, or does it
become a separate managed service the handler reaches? The
archetype-first framing suggests the latter: handler stays stateless
and replicated, connections state becomes a managed-session service.

**Evidence gaps.**
- STEK goroutines are spawned inside `Next()` — a method call from
  within Provision. The spawn point is internal, so current
  signature-level checks miss it. Need `lifecycle.no-async-fork`
  inspection inside provisioning paths, not just at handler boundary.
- No `MLV2_CHANNEL_BOUNDARY` markers are visible in caddy source; the
  channels are entirely internal (not exposed on lifted boundaries),
  which means the refusal is firing on internal state, not boundary
  shape. Makes this a state-class problem, not a boundary problem.

## AUTO set

| # | subsystem | region (file:line) | archetype | candidate state class | transform | evidence signals | missing evidence |
|---|---|---|---|---|---|---|---|
| C1 | caddytls | `SessionTicketService.stayUpdated`, sessiontickets.go:114-148 | `periodic-scheduler` | `periodic-invocation` | scheduled key rotation: cron → load-keys-from-source → write-keys | `go s.stayUpdated()` from `start()`; select on `Next(stopChan)`; mutates `s.keys` under `s.mu`; `lifecycle.long-running-loop` bias | is rotation-timing (stekConfig.RotationInterval) a coordination constraint across replicas? |
| C2 | caddytls | `TLS.keepStorageClean`, tls.go:1050-1072 | `periodic-scheduler` | `periodic-invocation` | distributed lock + scheduled cleanup job | `go func()` + `time.NewTicker`; select on stopChan; panic-recover | cross-replica coordination: need leader election or per-bucket sharding? |
| C3 | caddytls/distributedstek | `Provider.rotate`, distributedstek.go:209-236 | `pipeline-stage` | `bounded-worker-pool` (degenerate, 1 worker) | timer→storage-lock→rotate-keys→emit pipeline as scheduled job with storage lock as distributed-mutex primitive | `go s.rotate(doneChan, keysChan)` in `Next()`; storage.Lock/Unlock pair; sends keys down chan | storage lock API semantics under partition (blocking vs. error)? |
| C4 | caddytls/standardstek | `standardSTEKProvider.rotate`, stek.go:73-98 | `pipeline-stage` | `bounded-worker-pool` (degenerate) | same as C3 but local-only (crypto/rand), no external coordination | same shape; no storage dependency | `rotate` is spawned per Next() call not once — ephemeral goroutine |
| C5 | caddyhttp/reverseproxy | `Handler.connections` + `cleanupConnections` + `time.AfterFunc`, streaming.go:302-324 | `singleton-actor` | `keyed-partitioned-state` | connections map + cleanup timer → managed session store with TTL-based eviction | `connectionsMu.Lock/Unlock` around map; `time.AfterFunc(delay, closeConnections)`; cleanup timer optional (StreamCloseDelay>0) | graceful-shutdown-only? or required for protocol correctness? |
| C6 | caddyhttp/reverseproxy | `handleUpgradeResponse`, streaming.go:147-159 | `ephemeral-worker` (into session-scoped) | `session-scoped-state` | bidirectional stream broker as per-connection service | `go func()` + select on ctx.Done / backConnCloseCh; per-request hijack | session-scoped by WS-upgrade contract; clean boundary |
| C7 | caddyauth | `HTTPBasicAuth.Cache`, basicauth.go:105-110 | `ttl-cache-managed` | `ttl-cache` (new) | hash compare results cached in managed cache service w/ LRU | `mu *sync.RWMutex` + cache map + `singleflight.Group` for dedup | eviction policy (random 1/10) needs to match target cache semantics |

## SUGGEST set

| # | subsystem | region | archetype | why SUGGEST | missing evidence |
|---|---|---|---|---|---|
| C8 | caddytls/distributedstek | `Provider.getSTEK` storage.Lock/Unlock, distributedstek.go:150-178 | distributed-lock primitive | lock is infrastructure service (certmagic.Storage), not application state; not lift target per se | whether the lock API has timeout/deadlock detection the transform can rely on |
| C9 | caddyhttp/app | `App.allCertDomains`, app.go:209 | `keyed-partitioned-state` | transient map used only across phase 1 → phase 2; no runtime concurrency | is phase ordering a sufficient barrier, or can runtime reload violate it? |
| C10 | caddytls | `TLS.serverNames`, tls.go:143-144 | `singleton-actor` (name registry) | coarse mutex; set is post-provision static | how often is the set read/written during request serving? |
| C11 | caddy/admin | `adminHandler` routes + RemoteAdmin state, admin.go | `singleton-actor` | module reload path may exercise mutex contention | whether config reload is bounded to a single node's admin socket |

## TERMINAL set

| # | region | reason |
|---|---|---|
| C12 | `caddy.exiting` atomic.Int32, caddy.go:1254-1265 | process-global shutdown signal; kernel-level primitive, not a distribution archetype |
| C13 | `caddyhttp/app.App.Servers` provisioning | declarative config unmarshal, not runtime mutable shared state |
| C14 | `caddyhttp/reverseproxy.Handler.logger` | immutable after Provision; infrastructure service; no archetype candidate |
| C15 | OCSP stapling (delegated to certmagic) | refusal lives in dependency, outside evaluated corpus |

## ADMITTED set

- `reverseproxy.Handler.ServeHTTP` — pragma-admitted handler; stateless per request (connections map is out-of-band).
- `caddyhttp.App` Provision/Start/Stop — sequential phases, admitted once.
- `certmagic.Storage` operations — delegation to pluggable interface.
- `basicauth.HTTPBasicAuth.ServeHTTP` — handler pattern; per-request auth compute.
- `caddy.Listeners` via `UsagePool` — intentional reference-counted pool pattern.

## Subsystem coverage ledger

| subsystem | file count | finding |
|---|---|---|
| modules/ | 196 | Rich archetype surface in caddyhttp/caddytls/caddyauth — 5 AUTO (C1-C5, C7 — C6 session-scoped boundary case), 2 SUGGEST (C9, C10). Most modules (encode, headers, fileserver, etc.) are stateless request handlers — no archetype surface observed, reason: pure per-request transform. |
| caddyconfig/ | 33 | no relevant archetype surface observed — pure stateless parsing; input → Config struct, no runtime concurrency |
| cmd/ | 13 | no relevant archetype surface observed — CLI orchestration; low-level process control |
| caddytest/ | 20 | no relevant archetype surface observed — test harness, not production |
| internal/ | 10 | 1 SUGGEST (logbuffer — transient per-logger) — no hard archetype surface |
| root caddy/ | 29 | caddy.go / admin.go / listeners.go. 1 SUGGEST (C11 admin), 1 TERMINAL (C12 exiting). UsagePool is admitted. |

**Net:** 7 AUTO, 4 SUGGEST, 4 TERMINAL.

## Cross-cutting note

Caddy is the canonical instance of the "admitted handler + refused
state dependency" tension. The research-level finding: handlers do not
need to carry their state in the admission envelope. A handler + named
sharded state service (with the state lifted under an archetype
transform like C5/C7) is a cleaner separation than pulling the whole
struct into the lift region and refusing on its mutex.
