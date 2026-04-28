# Prioritization implications v1 — opus run

**Status:** run artifact. Parallel with gpt-5.4 and gemini.

## 1. The v1 ordering the research this extends proposed

`distribution-archetypes-followups.md` (Bucket A) lists the 8 archetypes without an explicit priority, but the narrative in `distribution-archetypes-v1.md` §12 recommends `bounded-worker-pool` + `periodic-invocation` as the first implementation pair, justified by "strongest coverage, most externalization-affine, lowest evidence-gap risk." Implicit ordering: corpus-coverage-weighted feasibility.

| v1 rank (feasibility) | archetype | basis |
|---|---|---|
| 1 | `periodic-invocation` | 6-target coverage, pragma-bridgeable evidence, clean transform |
| 2 | `bounded-worker-pool` | 4-target coverage, clean broker transform |
| 3 | `serialized-actor` | 5-target coverage, broadest state-class |
| 4 | `keyed-partitioned-state` | 5-target coverage, borderline evidence |
| 5 | `ttl-cache` | 5-target coverage, managed-cache transform |
| 6 | `fanout-publisher` | 4-target coverage |
| 7 | `session-affinity-state` | 3-target coverage, contract-dependent evidence |
| 8 | `filesystem-bound-singleton` | 2-target coverage, distinct transform |

## 2. Reordering by PLOS-utility (this research)

Using the two axes from `usefulness-scenarios-v1.md` §2 (local-fast-path preservability × independence) plus the paper's workload-responsive-placement criterion, the ordering changes meaningfully.

| utility rank | archetype | rationale |
|---|---|---|
| 1 | `bounded-worker-pool` | highest on both axes; enqueue-site preserves local channel fast path exactly; self-contained region; hits listmonk L2 / gitea G1 / mattermost MM6 cleanly; delegate DSL can express "offload when queue-depth > N" naturally |
| 2 | `session-affinity-state` | best fit to PLOS §4.2 scale-out-on-load story when load grows; the mattermost MM1/MM2 composite is the single strongest corpus demo for the Monolift thesis |
| 3 | `serialized-actor` (coordinator subset only) | gitea queue/eventsource/process managers cleanly benefit; but explicit subset-gate needed — hot-path actors like miniflux M6 ProxyRotator are net-negative |
| 4 | `periodic-invocation` | clean transform, high coverage, but the benefit is operational / resource packing rather than workload-responsive latency; solid pick if the compiler wants an easy early win that exercises pragma infrastructure |
| 5 | `fanout-publisher` | medium — API-clean transform but limited dynamic-placement capability; worth it when the subscriber graph is already service-oriented (gitea G7, listmonk L4) |
| 6 | `keyed-partitioned-state` | high value *only* in composite with session-affinity-state (mattermost hub); standalone use cases in the corpus are thinner than feasibility suggested |
| 7 | `ttl-cache` | useful automation but low PLOS fit — one-way externalization, no local fast path; implement because it's easy, not because it demonstrates the thesis |
| 8 | `filesystem-bound-singleton` | narrow corpus impact (only 2 targets, only caddy's filestorage is a clean win); implement last, or spike only if a demo target needs it |

## 3. What changed and why

**Promotions:**
- `bounded-worker-pool`: feasibility rank 2 → utility rank 1. It is the single archetype where the PLOS §4.2 dynamic-offload story maps onto a single code signature — the enqueue call. It delivers a demonstrable "local-at-low-load, remote-at-high-load" story with the delegate DSL as-is.
- `session-affinity-state`: feasibility rank 7 → utility rank 2. Low corpus coverage hides that the single big region (mattermost MM1+MM2 composite) is *the* textbook Monolift scale-out demo. Corpus breadth underweights impact here.

**Demotions:**
- `periodic-invocation`: feasibility rank 1 → utility rank 4. Strongest coverage and easiest transform, but the transform doesn't demonstrate workload-responsive placement — periodic ticks are not on the user hot path, so offloading them changes ops but not user-visible latency. Still worth shipping early, just not the flagship story.
- `ttl-cache`: feasibility rank 5 → utility rank 7. Once the cache is externalized, every access pays the network cost; the in-process fast path is structurally gone. Useful automation, but the Monolift thesis gets less mileage here than from `bounded-worker-pool`.
- `keyed-partitioned-state`: feasibility rank 4 → utility rank 6. Standalone regions in the corpus are thinner than feasibility claimed — listmonk L5 pipes iterates, gitea G2 is small-cardinality, pocketbase P3 is more `ttl-cache`-shaped. The strong corpus evidence for this archetype is almost entirely inside composites.

## 4. Why corpus-breadth-first prioritization was misleading

SPRINT-0013 prioritized by "cross-target coverage," which assumed coverage-at-the-archetype-level correlates with coverage-at-the-useful-region-level. Three reasons this assumption broke:

1. **Coverage counts feasibility instances, not utility instances.** A `serialized-actor` fit in caddy C5's connections map and in miniflux M6's ProxyRotator both count as "cross-target coverage," but only one repays the transform.
2. **Utility concentrates in composites.** The mattermost hub region accounts for the single highest-utility liftable unit in the corpus, but it does so by *co-lifting* three archetypes. Counting each archetype independently undercounts this.
3. **Externalization-affinity and utility don't line up.** The catalog used externalization-affinity as an evidence-confidence heuristic (archetypes that target managed substrates have more reliable transforms). But the same property — "target is a managed substrate" — is precisely what *weakens* the PLOS-utility story, because it forecloses the local fast path.

**Recommendation:** the composite synthesis should either (a) reorder Bucket A explicitly by utility, or (b) add an explicit "utility rank" alongside the existing evidence rank in the follow-ups doc, so future implementation sprints have both signals. The synthesis should not silently rely on feasibility order to also be utility order.

## 5. What to implement first, under PLOS-thesis lens

If the goal is "ship the state class that most clearly demonstrates the Monolift thesis on a real workload," the ordering is:

1. **`bounded-worker-pool`** — hit listmonk L2 as the first end-to-end demo. Ship alongside a workload harness that shows campaign-email latency under static-broker, static-local, and dynamic-delegate modes.
2. **`session-affinity-state` composite (MM1+MM2 lens)** — second-order demo, hit the mattermost websocket hub. This is the hardest corpus region technically (requires `connection-hub-buffer` composite support — ADR-0022 territory) but is the most compelling demo.
3. **`periodic-invocation`** — third, as an "easy win" that exercises pragma infrastructure (idempotent=true) without being the flagship.

Everything else follows corpus opportunity / user demand rather than thesis-demonstration.

## 6. What to *not* auto-apply even when evidence is strong

Three region classes where the v1 AUTO threshold passes but the utility reading argues for SUGGEST:

- **Hot-path serialized-actors** on user-synchronous paths (miniflux M6 ProxyRotator, caddy basicauth C7 as actor). Gate should include something like "not reachable from an ADMITTED request handler without crossing an async boundary."
- **Ephemeral local ttl-caches** (gitea G10 EphemeralCache). Gate should include "cache is used by multiple replicas' requests in a single deployment" — a hand-pragma until static analysis can catch this.
- **Low-cardinality keyed-partitioned-state** where the in-process mutex-map is already fine (gitea G2 baseChannel set). Size/cardinality is not a compile-time fact; best expressed as a pragma.

These map to ADR-0021's "evidence pragma" territory and ADR-0019's SUGGEST remediation surface. They also reveal a finding worth surfacing: **SUGGEST is the right default for an archetype on a new region, even when evidence is sufficient, until there is corroborating signal that lifting is useful for that specific region.** The catalog's AUTO thresholds currently assume "if feasible, apply." A utility-weighted reading would make AUTO conditional on either explicit user opt-in (pragma) or a utility-prior heuristic.

## 7. Meta: does this reordering change strategic direction?

Not dramatically — `bounded-worker-pool` was already in the v1 top-two, and `periodic-invocation` is still a reasonable early win. The interesting strategic change is:

- **Composite support becomes more urgent.** ADR-0022 (composite-archetype regions) moves from "followup after the first single-archetype state class lands" to "load-bearing for the PLOS demo." The mattermost hub composite is where the thesis shows best, and it can't ship without composite support.
- **Dynamic-delegate eligibility should be an archetype property.** Archetypes whose transform forecloses the local fast path (ttl-cache, filesystem-bound-singleton, managed-KV-target keyed-partitioned-state) should be marked as such so reports can explain that the lifted region will not participate in runtime delegate decisions the way the paper describes.
- **Utility-prior heuristics belong in the classifier roadmap.** A cheap "is this region on a user-synchronous path?" analysis would let AUTO be more surgical without needing full measurement. This is a proposed signal not yet in the classifier spec; flagged for future research.
