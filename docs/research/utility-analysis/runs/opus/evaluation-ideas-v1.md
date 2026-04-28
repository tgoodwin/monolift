# Evaluation ideas v1 — opus run

**Status:** run artifact. Parallel with gpt-5.4 and gemini.

Qualitative eval scenarios surfaced by the usefulness analysis. Each entry names: the archetype demonstrated, the target and region, the workload character, what the demo shows, and why it beats alternatives. No numeric targets — this is a catalog of experiment shapes, not a benchmark spec.

## Organizing principle

The PLOS '25 paper's evaluation has two goals: (§4.1) *show that different decompositions give different performance profiles and that Monolift lets you explore cheaply*, and (§4.2) *show that dynamic placement gives monolith-floor latency at low load and microservice-shoulder scalability at high load*. Future Monolift evaluation should extend both, and the candidates below are graded against that template.

## 1. Flagship demo candidates (one end-to-end story per archetype cluster)

### 1.1 `bounded-worker-pool` — listmonk campaign send under burst workload

- **Region:** listmonk L2, `manager.worker` + `campMsgQ`.
- **Why it is the strongest demo:** listmonk is the smallest corpus target (92 Go files), so the demo setup is cheapest. The existing pattern is already "fixed worker pool consuming from a channel for campaign email delivery" — the prose Monolift-story is natural: "at low load, local workers handle the queue; under campaign-launch burst, the runtime offloads to a broker-backed pool that scales horizontally."
- **Workload character:** burst. A campaign launch concentrates a large email set into a short time window. This exactly matches the §4.2 load-pattern shape.
- **What the demo shows:** end-to-end demonstration of dynamic delegate on a real archetype. Three conditions: (a) local-only baseline (all work in-process), (b) broker-only static (Service-Weaver-like), (c) Monolift dynamic (local at idle, broker on burst). The §4.2 "crossover" story is visible: (c) matches (a) at low load and (b) at high load.
- **Why it beats alternatives:** listmonk's queue-shape is cleaner than mattermost's hub, simpler to instrument than gitea's queue infrastructure, and — crucially — the campaign-launch workload is synthesizable without user simulation infrastructure (compare mattermost, which needs websocket client swarm).

### 1.2 `session-affinity-state` composite — mattermost websocket hub under connection growth

- **Region:** mattermost MM1 (`web_hub.go`) + MM2 (`web_conn.go`), as a `connection-hub-buffer` composite per the v1 catalog.
- **Why it is worth the technical cost:** it is the one corpus region where every part of the Monolift thesis applies at once: per-user affinity (session-affinity-state), per-user broadcast (fanout-publisher), per-connection send queues (keyed-partitioned-state). At low connection count, the hub runs in-process; at high count, the hub partitions across replicas with consistent-hash on user ID and the API to the handler layer is unchanged.
- **Workload character:** long-tail growth of concurrent connections. Not a request-rate-burst but a connection-count-growth story.
- **What the demo shows:** composite-archetype lift preserving API, scaling beyond single-replica's connection limit. This is the "multi-tenant / large deployment" story that Service Weaver's abandonment post-mortem implied was the real target.
- **Why it is harder:** requires ADR-0022 composite support and a runtime session-affinity router. Would be the largest single implementation spike, but is the strongest paper-motivating artifact.

### 1.3 `periodic-invocation` — miniflux feed refresh as a "fan-out scheduled job" story

- **Region:** miniflux M1 feedScheduler (+ M2 cleanup, M3 watchdog, M4 metrics as secondary).
- **Why it is a good demo:** miniflux's value *is* its feeds-refreshing-in-the-background story. Each feed is independent; refresh is idempotent; interval is config-driven. Lifting it to a platform scheduler means each feed-refresh becomes a scalable scheduled unit.
- **Workload character:** steady background workload scaling with feed count, not request rate.
- **What the demo shows:** scaling a background workload independent of the synchronous request-serving replicas. Shows the "operational packing" utility of `periodic-invocation` — not a §4.2 latency story, but a genuine resource-separation story.
- **Caveat:** this is not a §4.2 headline. Pair with (1.1) if presenting both.

### 1.4 `ttl-cache` as a "scale-out read path" story — mattermost session cache

- **Region:** mattermost MM4 / MM5 session and status caches.
- **Workload character:** steady high RPS with high cache-hit rate; bottleneck is session-lookup replication across replicas.
- **What the demo shows:** lifting the per-replica cache to a shared managed cache allows replica scaling without linear increase in session-DB load.
- **Why qualified:** this is *not* a dynamic-placement story — once the cache is external, every access is external. Frame as "automation of a hand-port" rather than as a PLOS §4.2 artifact.

## 2. Tradeoff / negative-control demos (show the archetype isn't always worth lifting)

### 2.1 Hot-path microsecond actor — miniflux ProxyRotator regression

- **Region:** miniflux M6 `ProxyRotator`.
- **What the demo shows:** applying the `serialized-actor` transform degrades feed-fetch latency because the archetype's RPC cost exceeds the actor's work cost per call. Intentional regression.
- **Purpose:** negative control that validates the SUGGEST-by-utility-heuristic claim in `prioritization-implications-v1.md` §6. Without this, reviewers will ask "but you said the archetype was AUTO?" This demo answers "feasibility ≠ utility."
- **Pair with** the listmonk L2 positive demo: same archetype, same transform, opposite utility outcome; the difference is call-site character (sync hot path vs. async queue).

### 2.2 Low-preservability externalization — gitea EphemeralCache anti-demo

- **Region:** gitea G10 `EphemeralCache`.
- **What the demo shows:** `ttl-cache` transform applied to local ephemeral cache degrades *every* call without load-relief benefit.
- **Purpose:** argues for the SUGGEST-by-utility heuristic for ephemeral caches specifically.

### 2.3 Composite-without-neighbors — caddy C5 connections as actor vs. partitioned

- **Region:** caddy C5 `Handler.connections`.
- **What the demo shows:** same region, different archetype label (`serialized-actor` vs. `keyed-partitioned-state` vs. `session-affinity-state`), different emitted transform, different performance profile.
- **Purpose:** visualizes `distribution-archetypes-v1.md` §8.1 ("archetypes are overlapping lenses") as a performance decision rather than a classifier decision. Supports ADR-0022.

## 3. Rapid-exploration demos (§4.1-style, multiple archetypes, single target)

### 3.1 "listmonk under five lifts" — reproduce §4.1's decomposition-exploration idea on a new target

- **Target:** listmonk (smallest, cleanest).
- **Setup:** apply annotations to L1 (scanCampaigns), L2 (worker), L3 (mailbox scanner), L4 (events.Publish), L5 (pipes). Generate five compiled configurations (each lifting one region) plus baseline and full.
- **What the demo shows:** not every decomposition is best. Like §4.1's "timeline-only wins," exploring the space reveals the right decomposition is not a priori obvious. This is a direct replication of the paper's rapid-exploration claim on a target *not* written for the paper.
- **Why it matters:** the paper's §4.1 used DeathStarBench's Social Network, an already-distributed benchmark. Running the same experiment on an unmodified open-source monolith is a stronger validation.

### 3.2 "gitea under queue pressure" — decomposition choice under a real workload shape

- **Target:** gitea.
- **Setup:** annotate G1 (WorkerPoolQueue), G4 (queue.Manager), G14 (cron). Compare static "all lifted" vs. dynamic delegate on CI-heavy workload (CI hooks trigger queue load).
- **What the demo shows:** composite decomposition's effect on a larger target with richer co-dependence than listmonk.

## 4. Thesis-stress demos (scenarios where the paper's claim is hardest to support)

### 4.1 Very-small-scale deployment

- **Target:** any single-replica self-hosted deployment (gitea or pocketbase on one machine, moderate user count).
- **What the demo shows:** lifts produce no benefit at all, and the delegate DSL correctly keeps execution local across the workload's full range. Shows that Monolift degrades gracefully to "the original application" when no benefit is available.
- **Purpose:** answers the "why wouldn't I just not use Monolift?" pushback honestly — the answer is "use Monolift if you *might* scale; it costs you nothing if you don't."

### 4.2 API-breaking transforms on preserve-required contracts

- **Region:** any `bounded-worker-pool` where per-caller enqueue is on the request's critical path and the caller expects "returned → durable."
- **What the demo shows:** the compiler correctly routes to SUGGEST (not AUTO), emits a structured remediation, and if the user overrides via pragma, the resulting behavior degrades as predicted.
- **Purpose:** validates ADR-0019 (remediation surface) and ADR-0021 (pragmas as evidence-or-override) on a real corpus example.

## 5. Paper-motivating composition (a specific narrative arc)

If a future Monolift paper wants a single evaluation section, the strongest composition is:

1. **Reproduce §4.1's rapid-exploration claim on listmonk** (unmodified real app, five decomposition configs, show the best one is not obvious — candidate 3.1).
2. **Reproduce §4.2's dynamic-placement claim on listmonk L2 campaign workload** (candidate 1.1), extended to show: (a) monolith floor, (b) full-distribution ceiling, (c) dynamic crossover, (d) negative-control on miniflux M6 for the same archetype — proving the archetype is not a silver bullet but is useful when aligned with workload shape.
3. **Scale demo on mattermost MM1/MM2 composite** (candidate 1.2), showing that the composite lift enables connection-count scaling impossible in a single-replica monolith.
4. **Anti-demo on gitea G10 EphemeralCache** (candidate 2.2), showing the compiler keeps the region at SUGGEST and the utility heuristic caught a feasibility-only-liftable region.

This covers: rapid-exploration utility, dynamic-placement utility, scale-out utility, and disciplined refusal. Four claims, four regions, three targets (no more) — feasible for a single evaluation section.

## 6. Evaluation infrastructure that would unlock these

Not a demo list but the support needed to run them:

- **A standardized workload harness for each corpus target.** listmonk campaign-burst, mattermost websocket-swarm, miniflux feed-set-loader, gitea git-push + CI-webhook. None exist today.
- **A dynamic-delegate wiring test.** Apply delegate expression `metric=IPS threshold=X` and verify the lift-point flips local↔remote at runtime.
- **A regression harness for PLOS-thesis claims.** Lock in the "monolith floor + distributed ceiling" Pareto shape as a property test per archetype; fail if a future classifier change regresses it.
- **Archetype-to-region-to-demo traceability matrix.** A table that for every AUTO region cites the nearest demo scenario demonstrating its utility. If the table has gaps, those are SUGGEST-for-lack-of-utility-evidence regions per `prioritization-implications-v1.md` §6.

## 7. What this catalog does not cover

- Measurement thresholds. Everything here is qualitative; turning any of these into a benchmark requires RPS / latency / replica-count targets which this sprint explicitly forbade.
- Cost / economic tradeoffs. Infrastructure-cost calculus (broker cost vs. compute savings) is out of scope.
- Multi-region / geo-distributed scenarios. The PLOS '25 paper is single-cluster; so is this catalog.
- Failure-injection eval. Broker-down, partition, cold-start scenarios are important (see `per-archetype-cards-v1.md` "New failure modes") but need their own eval sprint.
