# Monolift archetype utility analysis — composite research note v1

**Status:** v1. Synthesized from three parallel independent runs:
- `runs/opus/` — deep walk, two-axis structural model, dynamic-placement-eligibility as separate predicate
- `runs/gpt-5.4/` — concise, introduces operator-attention cost as first-class utility input
- `runs/gemini/` — breakeven inequality framing, "God Object" risk framing, strong consistency trade-off lens

This note is the cross-run composite. Where runs converged, the finding is the claim. Where they diverged, both framings are presented with attribution.

## 1. Why this research exists

SPRINT-0013 answered *what can Monolift lift?* — 8 archetypes, a four-gate vocabulary, candidate classifier additions. That mapped the **feasibility** space.

This sprint answers the complementary question: *given a liftable region, when does lifting it actually produce value, and when is it net-negative?* The goal is to hold utility as a north star for future compiler work, so Monolift does not invest in sophisticated classifier/transform techniques for archetypes whose payoff is marginal.

The user's framing: *"at the end of the day, we need to demonstrate utility."* This research is the evidence base.

## 2. Framing anchor — what "utility" means in the PLOS '25 paper

All three runs converged on this: Monolift's notion of utility is specifically different from a generic "when to microservice" argument, in three ways:

1. **Workload-responsive placement, not scaling per se.** §4.2 positions Monolift against both the monolith (best at low load, tail-latency spike at saturation) and the static microservice (worse at low load, stable under load). The claim is "best of both worlds": near-monolith latency at low load *and* microservice scalability at high load. Utility is the area under that Pareto improvement, not the top-end scaling ceiling alone.
2. **Pay-as-you-go.** §1 cites Service Weaver's abandonment — *"it required rewriting large parts of existing applications."* Monolift's argument rests on being worth using *for a handful of lifts* in an otherwise-monolithic app. An archetype that is only useful when 90% of the application is already lifted does not fit the thesis.
3. **Rapid exploration, not a priori "correct" decomposition.** §4.1's timeline-only result is the headline: the ideal decomposition was not obvious before measurement, and finding it required generating multiple architectures cheaply from the same source.

**Implication.** An archetype's usefulness in the Monolift sense is highest when its lift (a) preserves a local-execution fast path the runtime can take at low load, (b) can be introduced for a small region without co-lifting neighbors, and (c) delivers a credible win under some workload the application actually experiences. An archetype is *less* useful when its transform is "all or nothing" (state moves to a managed substrate and cannot go back), even if the transform is technically clean.

This single framing does more work for per-archetype reasoning than any of the individual trade-off lists below.

## 3. Cross-archetype structural axes

Two structural axes predict usefulness across the 8 archetypes. These came from the opus run, but map onto claims the other two runs independently surfaced.

### 3.1 Local-fast-path preservability

Does the lifted region retain a cheap, in-process implementation that the Monolift runtime can use at low load, with remote execution engaging only under pressure? This is the PLOS §4.2 property directly.

- **High preservability:** `periodic-invocation`, `bounded-worker-pool` (at the enqueue site), `serialized-actor` (at the method site), `fanout-publisher` (at the Publish site), `session-affinity-state` (at the routing decision). Local signature and remote signature are swap-compatible.
- **Low preservability — one-way externalization:** `ttl-cache` (once the cache becomes Redis, every Get/Set pays the network cost even at 1 RPS), `filesystem-bound-singleton` (once paths become object keys, the local disk is structurally gone), `keyed-partitioned-state` when the target is a managed KV.

Low-preservability archetypes aren't useless — they still scale — but their *Monolift-specific* advantage over writing a direct Redis client is smaller. The usefulness is primarily in the automation (no hand-porting), not in the dynamic-placement property. **This is worth calling out because the v1 catalog treats all 8 archetypes as if they participate equally in the PLOS thesis; they don't.**

### 3.2 Independence of the lifted unit — the pay-as-you-go property

Can the archetype's region be lifted alone, or does the transform pull a constellation of neighbors with it?

- **High independence:** `periodic-invocation`, `bounded-worker-pool`, `ttl-cache`, `filesystem-bound-singleton`, single-region `serialized-actor` instances.
- **Low independence:** `fanout-publisher` pulls all subscribers onto the broker at once. Composite `connection-hub-buffer` regions (mattermost MM1 + MM2) co-lift `keyed-partitioned-state` + `fanout-publisher` + `session-affinity-state` simultaneously. `keyed-partitioned-state` with cross-key iteration in hot paths drags iteration-protocol redesign.

The most "Monolift-native" archetypes are the ones that are high on both axes: `periodic-invocation`, and `bounded-worker-pool` at the single-region level.

### 3.3 GPT-5.4's third axis: operator attention

GPT-5.4 added a cost lens the other runs absorbed implicitly: *if a lift mostly creates a new service to manage without creating a new placement or isolation lever, it is low-payoff even when technically liftable*. This tightens the pay-as-you-go argument — not just "small commitment at lift time" but "small commitment in ongoing ops."

### 3.4 Gemini's breakeven inequality framing

Gemini formalized the trade-off:

> Benefit(Scaling + Isolation + Fault Tolerance) > Cost(Network Latency + Operational Complexity + Consistency Relaxation)

Not a measurable inequality given this sprint's no-numbers discipline, but a useful structure for per-archetype reasoning. Opus's two-axis model explains *why* the inequality skews for certain archetypes; gpt-5.4's operator-attention input adds a third cost term.

## 4. Per-archetype reasoning

Summary form; deep per-archetype cards (including corpus anchors per region) live in `per-archetype-cards-v1.md`.

### `serialized-actor`

- **Pays off when** the actor is a coordinator and single-ownership is the real semantic constraint. Gitea's `queue.Manager` (G4), `eventsource.Manager` (G6), `process.Manager` (G18) all fit: low-frequency coordination where in-process mutex is already a known serialization point.
- **Net-negative when** the actor sits on a user-synchronous hot path and the method is microsecond-level. Miniflux's `ProxyRotator` (M6) is the clearest miss — the actor's "pick next proxy" method is called synchronously on every feed fetch. RPC round-trip would regress every fetch; the proxy list isn't worth centralizing.
- **Within-archetype variance matters.** All three runs converge on this: usefulness of `serialized-actor` is bimodal (coordinator-shaped useful; hot-path-shaped net-negative). The catalog label does not distinguish these. Opus proposes a "reachability from admitted handler" heuristic; gemini flags "God Object" risk; gpt-5.4 frames it as "ownership wrapper vs. distributed service."
- **New failure modes.** Remote actor unavailability blocks all calls; retries may double-apply mutations (in-process version's mutex *was* the dedup); actor is structurally a single-point-of-failure.

### `bounded-worker-pool`

- **Pays off when** work is async-completable, per-job ordering isn't user-visible, and the existing channel is already a coarse buffer between ingress and background processing. Listmonk `manager.worker` (L2) is canonical — campaign email delivery where per-email ordering doesn't matter. Gitea `WorkerPoolQueue` (G1) and mattermost `PushNotificationsHub` (MM6) similar.
- **Net-negative when** the caller is on a synchronous hot path expecting "enqueue returned → job is durable," *or* when per-key FIFO is load-bearing (broker-dedup can't cheaply preserve it).
- **PLOS fit: highest of any archetype.** All three runs agree. The enqueue call site is the single code signature where the §4.2 dynamic-offload story maps exactly: channel send ↔ broker publish at the same shape.

### `periodic-invocation`

- **Pays off when** the body is genuinely background maintenance — no request is waiting on it, duplicate or skipped ticks are tolerable. Caddy `stayUpdated` (C1), `keepStorageClean` (C2). Miniflux M1-M4 scheduler family. Gitea cron G14. Pocketbase cron P2. Listmonk mailbox scanner L3.
- **Net-negative when** the "periodic" loop is a control loop with cross-tick state (backoff counter, self-tuning interval, watermark pointer). Also net-negative on primary control loops where pause/duplicate has user-visible impact — listmonk's `scanCampaigns` (L1) despite fitting the shape, because it's the campaign distributor.
- **PLOS fit: high but thinner than headline.** API cost is zero (Start/Stop become no-ops), local-fast-path preservation is trivial. But there is no workload condition where running it remotely vs. locally affects user-visible latency — periodic ticks aren't on the user hot path by definition. Utility is operational / resource-packing, which is real but not §4.2.
- **Cross-run disagreement.** Gemini puts this at #1 ("solves real noisy-neighbor problem in almost every target, minimal consistency risk, very clean transform"). Opus demotes it ("doesn't demonstrate workload-responsive placement"). GPT-5.4 splits the difference ("incremental-adoption hero, not flagship demo"). Honest composite: **first implementation win, not the flagship thesis demo.**

### `keyed-partitioned-state`

- **Pays off when** key space is naturally partitionable, per-key load is skewed enough that sharding relieves hot shards, no consumer depends on a global view. Mattermost's `hubConnectionIndex` (MM1) is the strongest example — user/connection key is operationally meaningful.
- **Net-negative when** (a) iteration across keys in hot path (listmonk L5 pipes — campaign manager iterates to dispatch; lifting turns O(shards) aggregation into a distributed operation), (b) map encodes cross-key invariants, (c) cardinality is small enough that the in-process mutex map is already fine.
- **Utility concentrates in composites.** All three runs converge: standalone `keyed-partitioned-state` regions in the corpus are thinner than feasibility claimed. The strong utility case is inside composites like the mattermost hub.

### `fanout-publisher`

- **Pays off when** subscribers are already logically independent services that happen to co-reside with the producer only for convenience. Mattermost cluster `Publish` (MM7) is ADMITTED for exactly this reason. Gitea `eventsource.Messenger` (G7), pocketbase `Broker` (P4) fit.
- **Net-negative when** the fanout encodes a distributed transaction (every subscriber must succeed), or subscribers read back shared state that is also being lifted (creates read-your-writes issues across broker boundary). Listmonk `Events.Publish` (L4) is borderline — non-blocking send + queue-full behavior suggest local policy that broker semantics would obscure.
- **PLOS fit: medium.** API preservation is clean, but dynamic swap between local fanout and broker fanout is not cleanly expressible — subscribers are either in-process or on the broker, not both.

### `ttl-cache`

- **Pays off when** the cache is genuinely shared read-only state that every replica would otherwise rebuild independently. Mattermost session cache (MM4/MM5) fits — sessions hit on every authenticated request. Listmonk `apiUsers` (L6). Pocketbase `tools/store` (P3) with TTL entries.
- **Net-negative when** the cache is local memoization of local computation (gitea `EphemeralCache` G10), or the in-process cache defends against source-of-truth latency (replacing it with another network call is circular). Caddy `HTTPBasicAuth.Cache` (C7) is borderline — authentication lookups are user-visible so remote cache on miss is risky.
- **PLOS fit: low — finding worth recording.** Once externalized, every Get/Set pays the network hop; there is no "local at low load" regime. The archetype is useful, but its usefulness argument is "automated hand-port to managed cache," not the §4.2 dynamic-placement story.
- **Cross-run disagreement.** Gemini promotes to #3 ("day-1 optimization for scaling monoliths"). Opus demotes to #7 (low PLOS-fit). GPT-5.4 mid at #5 (mixed utility). The honest composite view: `ttl-cache` is *easy* to lift and *useful* in the hand-port sense, but it's the archetype where "is this demonstrating Monolift's specific thesis" is weakest.

### `session-affinity-state`

- **Pays off when** per-connection state is bounded by connection lifetime, many concurrent connections, and per-connection work substantial enough to benefit from cross-replica load-spreading. Mattermost WebConn (MM2) and the composite hub region (MM1) fit: millions of concurrent websockets is canonical.
- **Net-negative when** session lifetime is sub-request, sessions routinely migrate across connections (mobile reconnects), or cross-session invariants (mattermost cluster) break the per-replica sticky-routing story.
- **PLOS fit: high at the top end of the load curve.** This is exactly the "scale-out when load increases" scenario §4.2 motivates. Low-load benefit is zero, but the delegate DSL (`metric=CPU threshold=75%`) can express this cleanly.

### `filesystem-bound-singleton`

- **Pays off when** local disk state is *data* (not lifecycle configuration), the deployment needs horizontal scaling of the service that reads/writes it, and path/object semantics match object-store primitives. Caddy's `filestorage` for certificates fits precisely.
- **Net-negative when** the filesystem is used for local lifecycle state the process needs on its own machine (lock files signaling "I am running here"). Also when filesystem access encodes crash-safety via fsync/rename-dance that object store doesn't preserve.
- **PLOS fit: low** (same reason as `ttl-cache` — one-way externalization, no monolith floor). **High-value automation but outside §4.2.**

## 5. Cross-cutting findings

### 5.1 A PLOS-framing finding not in the v1 catalog

All three runs independently surfaced this, though in different language: **the v1 catalog ranks archetypes by feasibility; PLOS utility ranks them differently, and these orderings disagree for at least three archetypes.**

| archetype | v1 rank (feasibility) | utility rank (this research) | direction |
|---|---|---|---|
| `periodic-invocation` | 1 (6-target coverage) | ~4 | down — no user-visible latency story |
| `bounded-worker-pool` | 2 (4-target coverage) | 1 | up — best §4.2 fit |
| `serialized-actor` | 3 (5-target coverage) | bifurcated (coordinator up, hot-path down) | split |
| `ttl-cache` | 5 (5-target coverage) | 7 | down — one-way externalization |
| `session-affinity-state` | 7 (3-target coverage) | 2 (opus) to 4 (gpt-5.4) to 7 (gemini) | up (mattermost composite drives it) |
| `filesystem-bound-singleness` | 8 | 8 | same |

(Full reordering in `prioritization-implications-v1.md`. The runs disagreed on exact positions; convergence points are recorded there.)

### 5.2 Dynamic-placement eligibility is its own predicate

The v1 catalog records AUTO/SUGGEST/TERMINAL per archetype. It does not carry a "does this archetype support the delegate DSL's local ↔ remote runtime switching?" bit.

From the PLOS paper that predicate is the headline property. Only some archetypes support it:

- **Dynamic-delegate-eligible:** `periodic-invocation`, `bounded-worker-pool`, `serialized-actor`, `fanout-publisher`, `session-affinity-state`.
- **Not dynamic-delegate-eligible:** `ttl-cache`, `filesystem-bound-singleton`, and `keyed-partitioned-state` when the target is a managed KV. These have one-way "externalize to managed substrate" transforms.

**Suggested finding:** either the catalog adds a `dynamic-delegate-eligible` bit per archetype, or ADR-0019's remediation surface surfaces the distinction. Without this, the compiler can auto-lift an archetype whose transform structurally cannot deliver §4.2 behavior, and the paper's utility claim doesn't cover those regions.

### 5.3 Usefulness is uneven *within* an archetype

`serialized-actor` in gitea's `queue.Manager` is usefully lifted; `serialized-actor` in miniflux's `ProxyRotator` is actively counterproductive. The archetype label does not distinguish these. This surfaced in every run. Opus proposes a lightweight "SSA reachability from an ADMITTED handler entry" heuristic to catch hot-path actors. Gemini flags via "God Object" risk for centralizing tightly-coupled state.

**Implication:** the catalog's AUTO thresholds distinguish *feasibility*, not *utility*. Even within AUTO regions, the compiler's decision to auto-apply should probably gate on a utility-prior signal.

### 5.4 Composite regions are where the PLOS demo lives

The v1 composite `connection-hub-buffer` (mattermost MM1 + MM2) is low-independence (co-lifting required) but high-preservability (local hub ↔ distributed cluster at the same API). It is the single region in the corpus that best demonstrates Monolift's §4.2 story: low load → hub runs in-process; high connection count → hub splits across nodes via consistent-hash on user-id, and the API to callers is unchanged.

Every other region individually makes a weaker version of this argument. **This makes ADR-0022 (composite-archetype regions) more urgent than SPRINT-0013's followups implied** — it's load-bearing for the PLOS demo, not just a future cleanup.

### 5.5 "Liftable but not useful" patterns worth flagging

Three recurring patterns across the corpus that pass feasibility gates but fail utility:

1. **Hot-path microsecond singletons** — fits `serialized-actor` shape, does not repay RPC round-trip. Miniflux `ProxyRotator` (M6); caddy `HTTPBasicAuth.Cache` (C7) if request-synchronous.
2. **Ephemeral caches of local computations** — fits `ttl-cache` shape, does not repay managed-cache overhead. Gitea `EphemeralCache` (G10).
3. **Thin in-process wrappers over already-external state** — lifting creates a second source of truth without retiring the first. Listmonk `Manager.pipes` (L5 — DB is SoT, pipes map is a routing convenience); gitea session providers (G11-G13) already on DB/Redis.

These three should be called out as "liftable-but-SUGGEST-for-utility-reasons" in ADR-0019's remediation text, not just "liftable-but-SUGGEST-for-evidence-reasons." The catalog currently conflates the two.

## 6. Answers to the brief's meta-questions

**Does ease-of-auto-lift correlate with usefulness?** Weakly, and not in the direction feasibility suggests. The easiest archetypes to lift (`periodic-invocation`, `ttl-cache`) have the thinnest PLOS-utility stories. The ones with the strongest utility (`bounded-worker-pool`, session-affinity composites) are mid-difficulty. **Implementation order driven by corpus-coverage feasibility is misleading.**

**Are there archetypes whose usefulness is conditional enough that auto-apply is wrong even with strong evidence?** Yes. Hot-path `serialized-actor` regions, ephemeral `ttl-cache` regions, low-cardinality `keyed-partitioned-state` regions. These should stay SUGGEST even when the evidence gates close, with the SUGGEST rationale citing utility-heuristic rather than evidence-gap.

**Does prioritizing by usefulness reorder the v1 list?** Yes, meaningfully — see §5.1 and `prioritization-implications-v1.md`. Not a dramatic reordering but a consequential one: `bounded-worker-pool` → first, `session-affinity-state` (composite lens) → second flagship, `ttl-cache` / `filesystem-bound-singleton` → later (automation, not thesis).

**Are there evaluation / demonstration scenarios this research surfaces?** Yes — four flagship demos surfaced by all three runs (worker-pool on listmonk campaign burst; session-affinity composite on mattermost hub; periodic on miniflux feed storm; caddy as negative control). Plus tradeoff / negative-control demos that show the archetype isn't always worth lifting. Full catalog in `evaluation-ideas-v1.md`.

## 7. Cross-run attribution summary

| finding | opus | gpt-5.4 | gemini |
|---|---|---|---|
| PLOS framing anchors utility | ✓ (explicit 3-pillar) | ✓ (3 advantages) | ✓ (breakeven inequality) |
| Two-axis structural model | **primary contribution** | — | — |
| Operator-attention as cost | — | **primary contribution** | implicit |
| Breakeven inequality formulation | — | — | **primary contribution** |
| `bounded-worker-pool` = top utility | ✓ | ✓ | ✓ #2 |
| Caddy hot-path as negative control | ✓ | ✓ (explicit) | ✓ |
| Composite regions = PLOS demo | **surfaced** | partial | partial |
| Dynamic-placement eligibility as separate predicate | **surfaced** | — | — |
| "Liftable but not useful" patterns | ✓ | ✓ (thin wrappers) | ✓ (God Object) |
| Usefulness reorders feasibility | ✓ | ✓ | ✓ |
| Usefulness uneven within archetype | ✓ | ✓ | ✓ |

## 8. Cross-links

- `per-archetype-cards-v1.md` — per-archetype detail (pays-off / net-negative / structural tells / failure modes / ops cost / consistency trade-offs / corpus regions).
- `prioritization-implications-v1.md` — how utility reorders the v1 prioritization; what to implement first and why.
- `evaluation-ideas-v1.md` — concrete demo / benchmark / paper-motivating scenarios surfaced by the research.
- `runs/{opus,gpt-5.4,gemini}/` — per-run source artifacts.
