# SPRINT-0016 — ADR-0022 committee drafting (composite-archetype regions)

**Status:** done
**Executor:** opus+gpt-5.4+gemini (three-way committee + opus synthesis)
**Brief:** `docs/sprints/SPRINT-0016-BRIEF.md`
**Deliverable:** `docs/decisions/0022-composite-archetype-regions.md`

## Intent

Draft ADR-0022 — composite-archetype regions — via three-way committee. SPRINT-0013 flagged ADR-0022 as ripe to draft; SPRINT-0015's utility analysis elevated it to load-bearing for the PLOS §4.2 demo. The ADR had to resolve three decisions (precedence among competing archetype matches, composite emission criteria, report format) against the trade-off space SPRINT-0013 surfaced (archetypes as overlapping lenses; multiple viable decompositions per region) and the utility framing SPRINT-0015 established (dynamic-delegate eligibility, bimodal within-archetype utility).

## Process

Same pattern as SPRINT-0013 and SPRINT-0015 research sprints: three parallel independent drafts, three cross-critiques, opus synthesis. Opus (orchestrating session) wrote the canonical ADR; per-run drafts and critiques preserved for transparency.

1. **Phase 1 — drafts.** Three parallel drafts at `docs/sprints/drafts/SPRINT-0016-{CLAUDE,CODEX,GEMINI}.md`. Each model read the full required-reading list from the brief (SPRINT-0013 and SPRINT-0015 outputs, ADRs 0015-0018, v2 contract, mattermost/caddy annotations) and formulated its own mental model for navigating the trade-off space.
2. **Phase 2 — cross-critiques.** Three parallel critiques at `docs/sprints/drafts/SPRINT-0016-{CLAUDE,CODEX,GEMINI}-critique.md`. Each critiquing model evaluated the other two against: defensibility of mental model, internal consistency across decisions, mattermost-demo handling, runtime-selection over/under-claiming, credibility of rejected alternatives.
3. **Phase 3 — opus synthesis.** Opus (this session) read all six files, identified convergences and disagreements, and wrote the canonical ADR at `docs/decisions/0022-composite-archetype-regions.md` + `docs/evolution.md` entry.

## Closeout

### Committee convergences adopted in the canonical ADR

- **Match-set mental model.** Classification produces a candidate set per region; the compiler projects that set into primary + alternative + composite candidates. Proposed by Claude's draft; Codex concurred explicitly in its critique.
- **Region-relative subsumption for precedence** (not a global ladder, not condition-counting). Codex's formulation adopted. Claude's self-critique conceded that its own condition-counting rule was weaker. Gemini's self-critique conceded that its global precedence ladder was arbitrary.
- **Composite coherence via compatible refinement** (adding one component transform must refine another's placement/ownership strategy, not invalidate it). Codex's formulation, adopted directly.
- **AND rule for dynamic-delegate-eligibility inheritance** on composites. Unanimous across all three drafts.
- **Compositional composite identity** (contributing-archetype list + region), with informal alias permitted for human-readable reporting. Codex's primary position; adopted as the narrow-safe middle between Claude's registered-catalog proposal and Gemini's promotion-to-first-class-archetype.
- **Concrete `reportv2` schema** exposing the candidate set with orthogonal fields for *exists* / *emittable* / *runtime-selectable*. Claude's spine refined per Codex's "recognized vs. emitted vs. runtime-selectable" distinction.

### Committee disagreements resolved in the canonical ADR

- **Condition-counting vs. subsumption.** Canonical adopts **subsumption** per Codex. Claude's cardinality-count was an approximation that could misfire; subsumption is the semantically correct criterion.
- **`connection-hub-buffer` as registered name vs. informal alias.** Canonical adopts **informal alias**. The report may expose it for demo storytelling, but formal identity stays compositional until a future ADR promotes it based on ≥2-corpus-instance evidence (the SPRINT-0013 coverage-gate criterion applied to composite promotion).
- **Runtime-selection specificity.** Canonical specifies the **reporting contract** (what `runtime_selectable` means) but defers the **mechanism** and pragma vocabulary to ADR-0021 and future ADRs. Claude's `alternatives=all` pragma proposal is noted but not adopted in the normative text.

### Claims explicitly rejected by the canonical ADR

- **Global archetype precedence ladder** (Gemini's Decision 1). Corpus does not support a total order; precedence is region-relative.
- **Numeric `utility_heuristic` scores in the report** (Gemini's Decision 3). Scope violation — liftability-metric work is ADR-0020 territory.
- **Condition-counting formalization of "most constrained"** (Claude's Decision 1). Semantic strength ≠ checklist cardinality.
- **Promotion of `connection-hub-buffer` to first-class archetype** (Gemini's implicit position). Fails the ≥2-target coverage gate; promotion awaits a second corpus instance.
- **Partial composite emission** (implicit in Gemini's framing). If N-1 of N component archetypes match, no composite fires; falls through to single-archetype primary over the N-1 satisfied archetypes.
- **Pragma-defined composites.** New composites require a future ADR; pragma does not extend the catalog.

### Per-draft attribution

| Contribution | Primary source | Notes |
|---|---|---|
| Match-set mental model | Claude | Codex concurred in critique |
| Region-relative subsumption | Codex | Claude and Gemini self-critiques conceded |
| Monotone-refinement coherence | Codex | Adopted with Claude's worked-example tracings |
| Caddy C5 subsumption tracing | Codex analysis + Claude's explicit primary output | Merged |
| AND rule for eligibility | Unanimous | Triple convergence; strong signal this is a real joint |
| Concrete `reportv2` schema | Claude | Refined per Codex's three-axis distinction |
| `connection-hub-buffer` as informal alias | Codex | Adopted over Claude's registered-catalog proposal |
| `runtime_selectable` as per-candidate conditional | Codex | Claude's pragma proposal deferred |
| ADR-per-composite-promotion governance | Claude | Adopted for future composite promotion, not v1 registration |
| Operator-attention cost as fallback tier | Codex | Folded into utility-tier fallback in Decision 1 |

### What this ADR unblocks

1. **Mattermost Hub composite as PLOS §4.2 demo.** The second-wave flagship SPRINT-0015 identified. A future implementation sprint can proceed with a concrete compiler contract: the classifier produces the match set, the coherence check validates the composite, the emission sketch generates the sharded-hub transform, the report exposes the composite under the `connection-hub-buffer` alias.
2. **Caddy C5 deterministic resolution.** Dominance is incomparable; tier-2 utility fallback selects `serialized-actor` primary with `keyed-partitioned-state` alternative.
3. **ADR roadmap dependencies.** ADR-0019 (remediation surface) has a stable candidate-set representation to consume. ADR-0020 (auto-lift evidence thresholds) can extend the report additively with scoring fields. ADR-0021 (pragmas) coordinates on `archetype=<name>` as an **override** pragma (position recorded in the canonical ADR).

### Files produced

- `docs/decisions/0022-composite-archetype-regions.md` — the canonical ADR.
- `docs/evolution.md` — entry added per repo convention.
- `docs/sprints/SPRINT-0016-BRIEF.md` — research brief.
- `docs/sprints/drafts/SPRINT-0016-{CLAUDE,CODEX,GEMINI}.md` — committee drafts (preserved).
- `docs/sprints/drafts/SPRINT-0016-{CLAUDE,CODEX,GEMINI}-critique.md` — cross-critiques (preserved).

### Files not produced (explicit non-goals met)

- No classifier code changes.
- No runtime code changes.
- No `reportv2` schema changes (ADR specifies the contract; implementation is a future sprint).
- No new v1 archetype vocabulary entries.
- No liftability metric definition.
- No ADR-0019, ADR-0020, ADR-0021, or ADR-0023 drafting.
