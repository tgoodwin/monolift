# Evaluation / demonstration scenarios — composite SPRINT-0015

Cross-run synthesis of demo, benchmark, and paper-motivating scenarios surfaced by the utility analysis. **Qualitative only** — these are experiment *shapes*, not benchmark specs; turning any of them into a measured benchmark requires workload thresholds and infrastructure outside this research.

**Organizing principle (from PLOS '25 paper evaluation):**
- §4.1: show that different decompositions give different performance profiles and that Monolift lets you explore cheaply.
- §4.2: show that dynamic placement gives monolith-floor latency at low load and microservice-shoulder scalability at high load.

Future Monolift evaluation should extend both axes.

---

## 1. Flagship demos — one per archetype cluster

### 1.1 `bounded-worker-pool` — listmonk campaign burst

- **Region:** listmonk L2, `manager.worker` + `campMsgQ` (`evaluation/listmonk/internal/manager/manager.go:462`, `Manager.worker`).
- **Why it's the strongest demo:** listmonk is the smallest corpus target (92 Go files), so demo setup is cheapest. The pattern is already "fixed worker pool consuming from a channel for campaign email delivery" — the prose is natural: *at low load, local workers handle the queue; under campaign-launch burst, the runtime offloads to a broker-backed pool that scales horizontally.*
- **Workload character:** burst. Campaign launches concentrate a large email set into a short window. Exactly matches the §4.2 load-pattern shape.
- **Three conditions to compare:** (a) local-only baseline (all work in-process), (b) broker-only static (Service-Weaver-like), (c) Monolift dynamic (local at idle, broker on burst). The §4.2 crossover story is visible: (c) matches (a) at low load and (b) at high load.
- **Why it beats alternatives:** cleaner queue-shape than mattermost's hub, simpler than gitea's queue infrastructure, campaign-launch workload is synthesizable without user simulation infrastructure.
- **Unanimous** across all three runs as #1 demo candidate.

### 1.2 `session-affinity-state` composite — mattermost websocket hub under connection growth

- **Region:** mattermost MM1 (`web_hub.go:77-120`, `Hub.hubConnectionIndex`) + MM2 (`web_conn.go:88-149`, `WebConn`), as `connection-hub-buffer` composite.
- **Why it's worth the technical cost:** the single corpus region where every part of the Monolift thesis applies at once — per-user affinity (session-affinity-state), per-user broadcast (fanout-publisher), per-connection send queues (keyed-partitioned-state). At low connection count, hub runs in-process; at high count, hub partitions across replicas via consistent-hash on user ID, and the API to handler layer is unchanged.
- **Workload character:** long-tail growth of concurrent connections. Not a request-rate-burst but a connection-count-growth story.
- **What the demo shows:** composite-archetype lift preserving API, scaling beyond single-replica connection limits. This is the "multi-tenant / large deployment" story that Service Weaver's post-mortem implied was the real target.
- **Technical cost:** requires ADR-0022 composite support and a runtime session-affinity router. **Largest single implementation spike in the near-term roadmap; also the strongest paper-motivating artifact.**
- **Run agreement:** opus and gpt-5.4 both surface this as the top composite demo; gemini names it as "Caddy Connection Hub" scenario (with a subtle misattribution — the canonical region is mattermost's hub, not caddy's).

### 1.3 `periodic-invocation` — miniflux feed-refresh storm

- **Region:** miniflux M1 `feedScheduler` (+ M2 `cleanupScheduler`, M3 watchdog, M4 metrics).
- **Scenario:** miniflux with 10,000 active RSS feeds. Baseline: all feeds fetched by the main process; during a fetch storm CPU/RAM spikes and the web UI becomes unresponsive. Lifted: `feedScheduler` as platform scheduler; `worker.Pool` (already ADMITTED) scales horizontally.
- **Workload character:** steady background workload scaling with feed count, not request rate.
- **What the demo shows:** scaling a background workload independent of synchronous request-serving replicas. Shows the "operational packing" utility of `periodic-invocation` — not a §4.2 latency story, but genuine resource-separation.
- **Caveat:** not a §4.2 headline. Best paired with (1.1) as the "second act" of a narrative that shows both background-workload scaling and request-path dynamic-placement.

### 1.4 `ttl-cache` as "scale-out read path" — mattermost session cache

- **Region:** mattermost MM4 / MM5 session and status caches (`evaluation/mattermost/server/channels/app/platform/session.go:45`).
- **Workload character:** steady high-RPS with high cache-hit rate; bottleneck is session-lookup replication across replicas.
- **What the demo shows:** lifting the per-replica cache to a shared managed cache allows replica scaling without linear increase in session-DB load.
- **Framing caveat:** this is *not* a dynamic-placement story — once the cache is external, every access is external. Frame as "automation of a hand-port" rather than a §4.2 artifact.

---

## 2. Tradeoff / negative-control demos

These show the archetype isn't always worth lifting. Critical for validating utility-weighted AUTO/SUGGEST decisions.

### 2.1 Hot-path microsecond actor — miniflux `ProxyRotator` regression

- **Region:** miniflux M6 `ProxyRotator` (`evaluation/miniflux/internal/proxyrotator/proxyrotator.go:14`).
- **What the demo shows:** applying `serialized-actor` transform degrades feed-fetch latency because the archetype's RPC cost exceeds the actor's work cost per call. Intentional regression.
- **Purpose:** negative control validating the SUGGEST-by-utility-heuristic claim. Reviewers will ask "but you said the archetype was AUTO?" This demo answers "feasibility ≠ utility."
- **Pair with 1.1:** same archetype (`serialized-actor` is in listmonk L2's enqueue path; actor in listmonk wins, actor in miniflux loses). The difference is call-site character (sync hot path vs. async queue).

### 2.2 Low-preservability externalization — gitea `EphemeralCache` anti-demo

- **Region:** gitea G10 `EphemeralCache` (`evaluation/gitea/modules/cache/ephemeral.go:18`).
- **What the demo shows:** `ttl-cache` transform applied to a local ephemeral cache degrades *every* call without load-relief benefit.
- **Purpose:** validates the SUGGEST-by-utility heuristic for ephemeral caches specifically.

### 2.3 Composite-without-neighbors — caddy C5 connections map as competing archetypes

- **Region:** caddy C5 `Handler.connections` (`evaluation/caddy/modules/caddyhttp/reverseproxy/streaming.go:302-324`).
- **What the demo shows:** same region, different archetype label (`serialized-actor` vs. `keyed-partitioned-state` vs. `session-affinity-state`), different emitted transform, different performance profile.
- **Purpose:** visualizes "archetypes are overlapping lenses" as a performance decision, not just a classifier decision. Supports ADR-0022 composite work.

### 2.4 Caddy reverse-proxy hot path — intentional refusal

- **Region:** `evaluation/caddy/modules/caddyhttp/reverseproxy/reverseproxy.go:50-51`, package-global `inFlightRequests`; `reverseproxy.go:102`, `Handler`.
- **What the demo shows:** protocol-critical coordination with channels / mutexes / maps that the compiler correctly refuses to lift. Distribution would harm it.
- **Purpose:** demonstrates that saying "do not lift this" is part of delivering utility. The usefulness story is incomplete if Monolift only shows successes.
- **Unanimous** across all three runs as the canonical negative control.

---

## 3. Rapid-exploration demos (§4.1-style)

### 3.1 "Listmonk under five lifts" — reproduce §4.1's decomposition-exploration on a new target

- **Target:** listmonk (smallest, cleanest).
- **Setup:** apply annotations to L1 (scanCampaigns), L2 (worker), L3 (mailbox scanner), L4 (events.Publish), L5 (pipes). Generate five compiled configurations, each lifting one region, plus baseline and fully-lifted.
- **What the demo shows:** not every decomposition is best. Exploring the space reveals the right decomposition is not a priori obvious. Direct replication of the paper's rapid-exploration claim on a target *not* written for the paper.
- **Why it matters:** the paper's §4.1 used DeathStarBench's Social Network (already-distributed). Running the same experiment on an unmodified open-source monolith is a stronger validation.

### 3.2 "Gitea under queue pressure" — decomposition choice on a larger target

- **Target:** gitea.
- **Setup:** annotate G1 (WorkerPoolQueue), G4 (queue.Manager), G14 (cron). Compare static "all lifted" vs. dynamic delegate on CI-heavy workload (CI hooks trigger queue load).
- **What the demo shows:** composite decomposition's effect on a larger target with richer co-dependence than listmonk.

---

## 4. Thesis-stress demos

Scenarios where the paper's claim is hardest to support — run these to find the limits honestly.

### 4.1 Very-small-scale deployment (graceful degradation)

- **Target:** any single-replica self-hosted deployment (gitea or pocketbase on one machine, moderate user count).
- **What the demo shows:** lifts produce no benefit, and the delegate DSL correctly keeps execution local across the workload's full range. Shows Monolift degrades gracefully to "the original application" when no benefit is available.
- **Purpose:** answers the "why wouldn't I just not use Monolift?" pushback honestly — the answer is "use Monolift if you *might* scale; it costs you nothing if you don't."

### 4.2 API-breaking transforms on preserve-required contracts

- **Region:** any `bounded-worker-pool` where per-caller enqueue is on the request's critical path and the caller expects "returned → durable."
- **What the demo shows:** the compiler correctly routes to SUGGEST (not AUTO), emits structured remediation, and if the user overrides via pragma, the resulting behavior degrades as predicted.
- **Purpose:** validates ADR-0019 (remediation surface) and ADR-0021 (pragmas-as-evidence-or-override) on a real corpus example.

### 4.3 Root-narrowing as a utility enabler (gpt-5.4's contribution)

- **Region:** pocketbase `core.App` terminal root (`evaluation/pocketbase/core/app.go:29`, `App`) contrasted with narrower useful regions P2 (Cron), P4 (Broker), P5 (BatchHandler).
- **What the demo shows:** Monolift utility is not "split the whole monolith." The useful compiler behavior is often **selecting or suggesting narrower roots inside an otherwise terminal application root.**
- **Purpose:** the demo argument gpt-5.4 surfaced — show the user a terminal root + adjacent liftable narrower roots; the compiler's explanation steers toward the narrower high-payoff lift.

---

## 5. Paper-motivating composition — a specific narrative arc

If a future Monolift paper wants a single evaluation section, the composite research recommends this arc (largely opus's proposal, validated by gpt-5.4's sequencing and gemini's scenarios):

1. **Reproduce §4.1's rapid-exploration claim on listmonk** (unmodified real app, five decomposition configs, show the best one is not obvious — §3.1 above).
2. **Reproduce §4.2's dynamic-placement claim on listmonk L2 campaign workload** (§1.1 above), extended: (a) monolith floor, (b) full-distribution ceiling, (c) dynamic crossover, (d) negative-control on miniflux M6 for the same archetype — proving the archetype isn't a silver bullet but is useful when aligned with workload shape.
3. **Scale demo on mattermost MM1/MM2 composite** (§1.2 above), showing the composite lift enables connection-count scaling impossible in a single-replica monolith.
4. **Anti-demo on gitea G10 EphemeralCache** (§2.2 above), showing the compiler keeps the region at SUGGEST and the utility heuristic caught a feasibility-only-liftable region.

**Four claims, four regions, three targets (no more) — feasible for a single evaluation section.** Covers rapid-exploration utility, dynamic-placement utility, scale-out utility, and disciplined refusal.

---

## 6. Recommended demo sequence for early implementation sprints

Cross-run convergence on ordering:

1. **Start with `bounded-worker-pool` on listmonk.** Easiest demo infrastructure, cleanest PLOS §4.2 story, smallest target.
2. **Follow with `periodic-invocation` on miniflux or pocketbase.** Exercises pragma infrastructure (idempotent=true), broad corpus applicability.
3. **Use mattermost realtime state as the advanced composite demo.** Largest implementation spike but highest thesis-demonstration value.
4. **Include Caddy negative control to show discrimination.** Round out the story — Monolift isn't "lift everything."

This sequence tracks the paper's utility thesis: start with easiest cases where dynamic placement is obviously useful, then widen into more conditional but more visually compelling connection-state stories.

---

## 7. Evaluation infrastructure that would unlock these

Not a demo list but the support needed to run them (opus's contribution, gpt-5.4 and gemini absorbed it implicitly):

- **Standardized workload harness per corpus target.** listmonk campaign-burst, mattermost websocket-swarm, miniflux feed-set-loader, gitea git-push + CI-webhook. None exist today.
- **Dynamic-delegate wiring test.** Apply delegate expression `metric=IPS threshold=X` and verify the lift-point flips local↔remote at runtime.
- **Regression harness for PLOS-thesis claims.** Lock in the "monolith floor + distributed ceiling" Pareto shape as a property test per archetype; fail if a future classifier change regresses it.
- **Archetype-to-region-to-demo traceability matrix.** A table that for every AUTO region cites the nearest demo scenario demonstrating its utility. Gaps in the table surface "SUGGEST-for-lack-of-utility-evidence" regions.

---

## 8. What this catalog does not cover

- **Measurement thresholds.** Everything is qualitative. Turning any of these into a benchmark requires RPS / latency / replica-count targets that this sprint explicitly forbade.
- **Cost / economic tradeoffs.** Infrastructure-cost calculus (broker cost vs. compute savings) is out of scope.
- **Multi-region / geo-distributed scenarios.** The PLOS '25 paper is single-cluster; so is this catalog.
- **Failure-injection eval.** Broker-down, partition, cold-start scenarios are important (see `per-archetype-cards-v1.md` "New failure modes") but need their own eval sprint.

---

## 9. Cross-run attribution summary

| scenario / theme | opus | gpt-5.4 | gemini |
|---|---|---|---|
| listmonk L2 campaign burst as flagship | ✓ (most detail) | ✓ | — (implicit in scenario 3) |
| mattermost composite as paper-motivating | ✓ (primary) | ✓ | partial (misattributed to caddy) |
| miniflux M1-M4 as scheduler demo | ✓ | ✓ | ✓ ("Miniflux Feed Storm") |
| Caddy as negative control | ✓ (explicit) | ✓ (explicit) | — |
| Negative-control demo pairings | ✓ (primary) | ✓ | — |
| Rapid-exploration reproduction on listmonk | ✓ (§3.1 primary) | — | — |
| Root-narrowing as utility enabler | — | ✓ (primary) | — |
| Very-small-scale graceful-degradation | ✓ (primary) | — | — |
| Paper-motivating composition arc | ✓ (§5 primary) | partial | — |
| Evaluation infrastructure requirements | ✓ (primary) | partial | — |
| Scenario-narrative framing ("Feed Storm") | — | — | ✓ (primary) |
| S3/object-store pocketbase migration demo | — | — | ✓ (filesystem-bound-singleton scenario) |
| Gitea multi-tenant sharding demo | — | — | ✓ (keyed-partitioned-state scenario) |
