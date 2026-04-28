# SPRINT-0015 — Archetype utility analysis

Follow-up research to SPRINT-0013. SPRINT-0013 answered *"what patterns can Monolift lift?"* (8 archetypes, feasibility gates). This sprint answers the complementary question: *"when is lifting actually useful, and when is it net-negative?"*

Three parallel runs (opus, gpt-5.4, gemini), each grounded in the PLOS '25 paper and the six evaluation targets' v1 region annotations. Composite synthesis merges convergences and preserves divergences.

## Canonical composite artifacts

- **`utility-scenarios-v1.md`** — narrative research note. Primary artifact. Anchors the utility question in the PLOS paper's three-pillar framing (workload-responsive placement, pay-as-you-go, rapid exploration); develops two structural axes (local-fast-path preservability × independence); per-archetype reasoning; cross-cutting findings.
- **`per-archetype-cards-v1.md`** — one card per archetype with pays-off / net-negative / failure modes / operational cost / consistency trade-offs / code-structural tells / PLOS §4.2 fit. Cross-run attribution throughout.
- **`prioritization-implications-v1.md`** — how utility reorders the v1 feasibility-driven prioritization. Sprint-sequencing recommendations. Where the runs converged, where they disagreed.
- **`evaluation-ideas-v1.md`** — concrete demo / benchmark / paper-motivating scenarios surfaced by the research. Flagship demos, negative controls, rapid-exploration demos, thesis-stress demos. Recommended paper-motivating composition arc.

## Headline findings

1. **Usefulness reorders the v1 prioritization meaningfully.** `bounded-worker-pool` is the strongest PLOS-thesis demo (promoted from v1 feasibility-rank 2 to utility-rank 1, unanimous across runs). `ttl-cache` and `filesystem-bound-singleton` have lower §4.2 fit than their feasibility ranks suggested (one-way externalization — no local fast path).
2. **Usefulness is bimodal within archetypes.** `serialized-actor` in a coordinator role is useful; the same archetype on a hot microsecond path is actively counterproductive. The catalog label alone is insufficient for AUTO-vs-SUGGEST decisions — per-region utility heuristics needed.
3. **Composite regions are where the PLOS demo lives.** Mattermost's websocket hub (MM1+MM2) is the single corpus region where every part of the Monolift thesis applies at once. This makes ADR-0022 (composite-archetype regions) load-bearing for thesis demonstration, not a "later cleanup."
4. **Dynamic-delegate eligibility is its own predicate.** Some archetypes (`ttl-cache`, `filesystem-bound-singleton`, managed-KV-target `keyed-partitioned-state`) have one-way externalization transforms — they don't support the "local at low load, remote at high load" behavior the paper motivates. Worth carrying as an explicit catalog bit.
5. **"Liftable but not useful" is a real pattern.** Hot-path singletons, ephemeral local caches, thin wrappers over already-external state. These should route to SUGGEST via utility heuristics even when evidence gates pass.

## Source runs (preserved)

- `runs/opus/` — deepest walk. Two-axis structural model (local-fast-path preservability × independence); dynamic-placement eligibility as separate predicate; composite regions as the PLOS demo; most detailed paper-motivating evaluation composition.
- `runs/gpt-5.4/` — concise vocabulary. Operator-attention as first-class utility cost; root-narrowing as a utility enabler (pocketbase core.App terminal → narrower useful roots); sprint-wave sequencing.
- `runs/gemini/` — breakeven inequality framing. "God Object" risk for serialized-actor misapplication; strong consistency trade-off framing; scenario narratives ("Miniflux Feed Storm", "Caddy Connection Hub", "S3-backed Pocketbase") that name demo stories concretely.

## Relationship to other sprints

- **SPRINT-0013 is the feasibility-side counterpart.** This research extends it with the utility question. Neither is complete without the other.
- **SPRINT-0014 (in-flight)** will implement `periodic-invocation` end-to-end. The utility research validates this as a reasonable first implementation (exercises pragma infrastructure, broad coverage) but flags that it's *not* the flagship §4.2 demo — that's `bounded-worker-pool` (rank 1 by utility) or the mattermost composite.
- **Future sprints** should use this research's prioritization rather than v1's coverage-weighted implicit order. The most thesis-demonstrating demos are `bounded-worker-pool` on listmonk and the composite hub on mattermost.

## Sprint metadata

- **Brief:** `../../../sprints/SPRINT-0015-BRIEF.md`
- **Ledger:** `SPRINT-0015` — `Research: archetype utility analysis`
- **Framing anchor:** `inspiration/papers/monolift-plos25.pdf`
- **Scope fence:** qualitative only; no RPS / latency / user-count / cost numbers. Research only — no compiler/classifier/runtime/schema changes.
