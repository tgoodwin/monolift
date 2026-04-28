# Prioritization implications — composite SPRINT-0015

Cross-run synthesis of how utility analysis reorders the v1 archetype catalog's implicit feasibility-first priority.

## Headline

Usefulness does reorder the v1 landscape. **Three claims the runs converged on unanimously:**

1. `bounded-worker-pool` is the strongest first-implementation target by utility (promoted from v1 feasibility-rank ~2 to utility-rank 1).
2. `filesystem-bound-singleton` is last by utility (unchanged from v1 feasibility — both orderings agree it's niche).
3. Usefulness is **bimodal within archetypes**, not just across them — `serialized-actor` in a coordinator role is useful; the same archetype on a hot path is actively counterproductive. This means per-archetype prioritization alone is insufficient; the compiler also needs per-region utility heuristics.

**The runs disagreed on exact mid-tier positions**, particularly whether `session-affinity-state` (which has only 3-target coverage but houses the single strongest composite demo in mattermost) deserves rank 2 (opus), rank 4 (gpt-5.4), or rank 7 (gemini). This disagreement is real and recorded; the composite leans toward opus's promotion on thesis-demonstration grounds.

## v1 feasibility ordering (baseline)

| v1 feasibility rank | archetype | basis |
|---|---|---|
| 1 | `periodic-invocation` | 6-target coverage, clean transform, pragma-bridgeable |
| 2 | `bounded-worker-pool` | 4-target coverage, clean broker transform |
| 3 | `serialized-actor` | 5-target coverage, broadest state-class |
| 4 | `keyed-partitioned-state` | 5-target coverage, borderline evidence |
| 5 | `ttl-cache` | 5-target coverage, managed-cache transform |
| 6 | `fanout-publisher` | 4-target coverage |
| 7 | `session-affinity-state` | 3-target coverage, contract-dependent |
| 8 | `filesystem-bound-singleton` | 2-target coverage |

This is the implicit ordering from `distribution-archetypes-followups.md` Bucket A, weighted by corpus breadth.

## Utility reordering (this research)

Composite reordering, noting where runs disagreed:

| utility rank | archetype | primary rationale | run agreement |
|---|---|---|---|
| **1** | `bounded-worker-pool` | Highest PLOS §4.2 fit — enqueue-site preserves local fast path exactly; hits listmonk L2 / gitea G1 / mattermost MM6 cleanly; delegate DSL can express "offload when queue depth > N" naturally | **unanimous** |
| **2** | `periodic-invocation` *(first pair with #1)* | Easiest win; exercises pragma infrastructure via `idempotent=true`; strong coverage; low risk. Opus flags it is not the flagship §4.2 demo (no user-visible latency story) but all three agree it's a reasonable early win paired with #1 | unanimous on pair, disagree on priority among pair |
| **3** | `session-affinity-state` (for composite demo value) | Opus ranks 2 — the single strongest PLOS-thesis demo via mattermost MM1+MM2 composite (connection-hub-buffer). gpt-5.4 ranks 4 — promotes on demo value but stages after queue/scheduler work. gemini ranks 7 — treats as niche | **disagreement** — opus/gpt-5.4 promote; gemini treats as niche |
| **4** | `fanout-publisher` | Cleanest broker-shaped regions (pocketbase Broker P4; gitea Messenger G7) give visible isolation benefits for slow subscribers. Works ahead of broader archetypes because brokers are existing infra | moderate agreement (opus #5, gpt-5.4 #3, gemini #4) |
| **5** | `serialized-actor` *(coordinator subset only)* | **Bifurcated.** Coordinator-shaped instances (gitea queue/eventsource/process managers) are useful; hot-path actors (miniflux M6) are net-negative. The subset-gate is load-bearing — do not auto-apply broadly. | **unanimous on bifurcation**, disagreement on rank |
| **6** | `ttl-cache` | Useful automation; low PLOS §4.2 fit (one-way externalization, no local fast path). gemini ranks 3 (day-1 optimization); opus ranks 7 (no monolith floor); gpt-5.4 ranks 5 | **strongest disagreement** — gemini promotes, opus demotes |
| **7** | `keyed-partitioned-state` | Standalone regions thinner than feasibility claimed. Strong value only *inside composites* (mattermost hub). Carries cross-key invariant risk | moderate agreement on demotion from v1 rank 4 |
| **8** | `filesystem-bound-singleton` | Narrow corpus impact; often a storage-migration decision more than a Monolift runtime-placement win | **unanimous** |

## Why corpus-breadth-first prioritization was misleading

Three reasons the v1 coverage-weighted ordering skewed wrong:

1. **Coverage counts feasibility instances, not utility instances.** A `serialized-actor` fit in caddy's connections map and in miniflux's ProxyRotator both count as "cross-target coverage," but only one repays the transform.
2. **Utility concentrates in composites.** Mattermost MM1+MM2 is the single highest-utility region in the corpus, but it requires co-lifting three archetypes simultaneously. Counting each archetype independently undercounts this.
3. **Externalization-affinity and utility don't line up.** The v1 catalog used externalization-affinity as an evidence-confidence heuristic ("archetypes targeting managed substrates have more reliable transforms"). But the same property — "target is a managed substrate" — is precisely what *weakens* the PLOS §4.2 story, because it forecloses the local fast path.

## Sprint-sequencing recommendation

Based on the composite reordering, a plausible implementation sequence:

### First wave (the "easy wins" pair)
- **`bounded-worker-pool`** — hit listmonk L2 as the first end-to-end demo. Ship alongside a workload harness that shows campaign-email latency under static-broker, static-local, and dynamic-delegate modes.
- **`periodic-invocation`** — exercises pragma infrastructure via `idempotent=true`, clean test bed for ADR-0021's evidence-vs-override distinction (this is already SPRINT-0014's planned work).

### Second wave (the composite flagship)
- **`session-affinity-state` composite** (mattermost MM1+MM2 via `connection-hub-buffer` lens) — **second-order demo**, the strongest PLOS §4.2 story in the corpus. Largest technical lift (requires ADR-0022 composite support) but highest thesis-demonstration value.

### Third wave (broker-shaped archetype)
- **`fanout-publisher`** — pocketbase Broker and gitea Messenger as anchors. Lower technical complexity than session-affinity composites.

### Fourth wave (conditional / bifurcated)
- **`serialized-actor`** — coordinator subset only; SUGGEST for everything else.
- **`ttl-cache`** — SUGGEST-first for ephemeral local caches; AUTO for genuinely shared session caches.

### Last (narrow / late)
- **`keyed-partitioned-state`** (standalone) — mostly via composite support landing in the second wave.
- **`filesystem-bound-singleton`** — when a demo target specifically needs it.

## Auto-apply vs. suggest implications

The utility analysis surfaces a strategic position not explicit in SPRINT-0013: **SUGGEST is the right default for an archetype on a new region, even when evidence is sufficient, until there is corroborating signal that lifting is useful for that specific region.** The v1 AUTO thresholds assume "if feasible, apply." A utility-weighted reading would make AUTO conditional on either explicit user opt-in (pragma) or a utility-prior heuristic.

Three region classes where v1 AUTO threshold would pass but utility argues for SUGGEST:

- **Hot-path `serialized-actor` regions** on user-synchronous paths (miniflux M6). Gate should include something like "not reachable from an ADMITTED request handler without crossing an async boundary."
- **Ephemeral local `ttl-cache` regions** (gitea G10 EphemeralCache). Gate should include "cache is used by multiple replicas' requests in a single deployment."
- **Low-cardinality `keyed-partitioned-state`** (gitea G2 baseChannel set) — size/cardinality is not a compile-time fact; best expressed as a pragma.

Maps onto ADR-0021's evidence-pragma territory and ADR-0019's SUGGEST remediation surface.

## Strategic changes this reordering implies

1. **ADR-0022 (composite-archetype regions) moves from "later" to "load-bearing for the PLOS demo."** Mattermost hub is the single strongest thesis-demonstration region, and it can't ship without composite support.
2. **The catalog should carry a `dynamic-delegate-eligible` bit per archetype.** Archetypes whose transform forecloses the local fast path (`ttl-cache`, `filesystem-bound-singleton`, managed-KV-target `keyed-partitioned-state`) should be marked as such so reports can explain that the lifted region does not participate in runtime delegate decisions the way the paper describes.
3. **Utility-prior heuristics belong in the classifier roadmap.** A cheap "is this region on a user-synchronous path?" analysis would let AUTO be more surgical without full measurement. This is a proposed classifier signal not yet in the v1 spec; flag for future research.
4. **Per-region utility gate, not just per-archetype.** Opus's strongest recommendation, echoed by gpt-5.4 and gemini: the compiler's AUTO decision on a specific region should incorporate utility heuristics even after the archetype's evidence gates pass.

## Cross-run attribution for the reordering

- **Unanimous** (all three runs agree): `bounded-worker-pool` to #1; `filesystem-bound-singleton` to #8; bifurcation of `serialized-actor`; `periodic-invocation` + `bounded-worker-pool` as first-wave pair.
- **Opus-primary**: promotion of `session-affinity-state` via composite lens; dynamic-placement-eligibility as separate predicate; composite regions as the PLOS demo; per-region utility heuristic proposal.
- **GPT-5.4-primary**: operator-attention as utility cost; root-narrowing as utility enabler (pocketbase core.App terminal → narrower useful regions); sprint-wave sequencing.
- **Gemini-primary**: breakeven-inequality formulation; "God Object" risk for serialized-actor misapplication; promotion of `ttl-cache` as day-1 scaling optimization.

The composite leans toward opus/gpt-5.4 convergence where gemini diverges (e.g., `ttl-cache` demoted per opus's PLOS-fit argument; `session-affinity-state` promoted per opus's composite argument) — but gemini's alternate positions are documented as legitimate alternative readings, not dismissed.
