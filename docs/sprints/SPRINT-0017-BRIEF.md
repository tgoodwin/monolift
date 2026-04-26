# SPRINT-0017-BRIEF — ADR-0022 vertical slice (Caddy `Handler.connections`)

**Status:** brief (planner not yet run). Invoke `sprint-planner` with this file as the intent when ready to draft.

## Origin

ADR-0022 (composite-archetype regions) landed accepted on 2026-04-23 with two worked examples but no implementation. Its decision space — candidate-set construction, subsumption, utility-tier fallback, composite emission, and the additive `reportv2` schema — has only been pressure-tested on paper. Before generalizing, we want one concrete vertical slice that exercises the design end-to-end on a real corpus region.

ADR-0022 itself names two natural slice candidates: Caddy `Handler.connections` (incomparable candidates → `serialized-actor` primary via utility-tier fallback) and Mattermost `Hub`/`WebConn` (the composite case). Caddy is the smaller slice — it exercises candidate-set construction, subsumption (and its incomparable outcome), utility-tier fallback, and alternatives reporting, but not composite emission. We're starting there. A second slice covering Mattermost composites is the natural follow-up sprint and should be visible in the roadmap when this sprint lands.

The terminology rename of 2026-04-25 (`dominance` → `subsumption`, `monotone refinement` → `compatible refinement`) is normative; the implementation uses the new names from the start. ADR-0022 carries a "Terminology note (post-acceptance revision)" preserving the rename rationale.

## The layered architecture context

The slice operates within the four-layer structure made explicit in ADR-0017's "Layered architecture (clarifying note)" section:

1. **Liftability properties** (`pkg/compiler/liftability/`) — named facts about the code; ADR-0018 vocabulary.
2. **Archetypes** (near `pkg/compiler/stateclass/`) — each archetype is a *subset of liftability properties* that must hold; matching is set membership.
3. **Candidate set + subsumption** — the new ADR-0022 layer; this sprint introduces it.
4. **Adapter derivation** (`pkg/compiler/extract/extract.go` `deriveAdapters`) — produces `reportv2.Adapter` records.

Layer 1 exists. Layer 2 needs a refactor to make required-property sets first-class. Layer 3 is entirely new. Layer 4 needs one new adapter kind.

## Scope intent

Run Caddy `Handler.connections` end-to-end through the new pipeline:

- The classifier produces a candidate set with `{serialized-actor, keyed-partitioned-state}`, not a single archetype label.
- Subsumption runs and returns "incomparable" for this region.
- Utility-tier fallback selects `serialized-actor` as primary; `keyed-partitioned-state` is recorded as alternative with a non-empty `rationale`.
- The `reportv2` archetype section exposes both via the new fields from ADR-0022 Decision 3 (excluding composite-specific values).
- `deriveAdapters` produces a real `Kind: "actor"` adapter record for the chosen primary, making `emittable: true` non-vacuous.
- An e2e assertion against the actual Caddy fixture validates the report shape.

## What's explicitly in scope

1. **Layer 1 cross-cutting:** introduce `liftability.PropertyID` as a typed Go constant per property. Replace bare string-literal property IDs throughout the codebase with the constants. Forbid bare strings going forward (linter or convention; planner picks).
2. **Layer 2 refactor:** materialize each archetype's required-property set as first-class data drawn from `liftability.PropertyID`. Migrate at least `serialized-actor` and `keyed-partitioned-state` for the Caddy slice. Other archetypes may be migrated incrementally; the slice does not require a full migration.
3. **Layer 3 introduction:** new candidate-set type, subsumption check (set-comparison over `PropertyID`), utility-tier fallback (the four tiers from ADR-0022 Decision 1), alternatives recording. Lives in `pkg/compiler/stateclass/` (sibling files preferred over a new package).
4. **Layer 4 extension:** new `Kind: "actor"` adapter in `deriveAdapters` with sensible `ID`, `StateEffects`, `TransportEffects`, `SerializationEffects` fields. Wired in based on the chosen primary candidate. **Adapter records are descriptive only — no runtime hosting, no Go source generation.** The runtime-truthfulness gap is a known follow-up.
5. **`reportv2` schema additions** (additive only): `archetype_kind`, `primary { ... }`, `alternatives [ ... ]`, `pragma_provenance`. Schema test updates. Composite-specific values (`archetype_kind: "composite"`, `contributing_archetypes` length > 1) are not exercised this sprint but the schema accommodates them.
6. **Tests:**
   - Unit tests for the candidate-set construction, subsumption check (Hold / Violate / Incomparable), utility-tier fallback ordering, and per-candidate flag computation.
   - E2E assertion against the actual Caddy `Handler.connections` fixture: primary is `serialized-actor`, alternative is `keyed-partitioned-state` with non-empty rationale, adapter record of kind `actor` appears in the report.

## What's explicitly out (but designed-for)

Per forward-design constraints — the implementation must not foreclose these:

- **Composite emission, coherence check, compatible-refinement check.** Mattermost's territory. The candidate type must accommodate `contributing_archetypes` as a list from day one (Caddy slice always sets it to length 1). The seam where composite candidates would be inserted into the match set must exist as a no-op extension point.
- **The `archetype_kind: "composite"` value.** Schema accommodates it; this slice never emits it.
- **AND-rule eligibility inheritance** for composites. Per-candidate flag computation should be a per-candidate function (not a property baked into the candidate type), so composite inheritance can plug in later.
- **The composite catalog data structure.** Out of scope; not even stubbed.
- **Runtime hosting of the actor adapter.** The adapter record describes the boundary; nothing on the runtime side actually hosts a serialized actor yet. This gap is named explicitly in the closeout doc as a follow-up.
- **Real Go source generation for the actor wrapper.** The adapter record is metadata, not generated code.
- **A full sweep of the archetype catalog into the new property-set form.** Only the two archetypes the Caddy slice needs must be migrated. Other archetypes can stay in their current matching form for now and migrate incrementally.

## Forward-design constraints (load-bearing)

The implementer must satisfy these so the Mattermost composite slice doesn't force a rewrite:

1. **Candidate-set construction is a discrete pass returning a collection**, not a function returning a scalar with alternatives bolted on. Composites consume the same collection.
2. **Match-set construction and selection are separate phases.** Composite candidates will be inserted *after* construction and *before* selection — that seam exists.
3. **The candidate type accommodates `contributing_archetypes` as a list and `alias` as a string from day one**, even if Caddy only ever sets length 1 and empty string.
4. **`emittable`, `runtime_selectable`, `dynamic_delegate_eligible` are per-candidate computation functions, not stored fields.** AND-rule for composites plugs in later.
5. **No `if N candidates then dominance/subsumption else single` branch in orchestration.** N=1 is the degenerate case of the same code path.
6. **The new `Kind: "actor"` adapter produced by `deriveAdapters` reads from the candidate-set output, not from a legacy single-shape field.** No back-channel.

## Required reading

For the three drafting models:

1. ADR-0017 (`docs/decisions/0017-classifier-reasons-about-liftability.md`) — especially the new "Layered architecture (clarifying note)" section.
2. ADR-0018 (`docs/decisions/0018-liftability-property-taxonomy.md`) — the property vocabulary archetypes will consume.
3. ADR-0022 (`docs/decisions/0022-composite-archetype-regions.md`) — the full ADR including the post-acceptance terminology note. **Note:** wherever the ADR or supporting docs say `dominance`/`monotone`, this sprint's implementation uses `subsumption`/`compatible`.
4. `docs/research/archetype-catalog-v1.md` — the v1 archetype vocabulary; especially the entries for `serialized-actor` and `keyed-partitioned-state`.
5. `docs/research/runs/*/annotations/caddy.md` (if present) and `test/e2e/targets/caddy/` for the Caddy fixture.
6. `pkg/compiler/liftability/` — Layer 1 today; understand the current property-ID surface (currently strings; this sprint introduces typed constants).
7. `pkg/compiler/stateclass/` — Layer 2 today; the refactor target.
8. `pkg/compiler/extract/extract.go` `deriveAdapters` — Layer 4; the extension point.
9. `pkg/compiler/reportv2/{report.go,schema.json,report_test.go}` — the schema additions land here.

## Success signal

`go test ./pkg/...` is green. The Caddy `Handler.connections` e2e fixture produces a report whose archetype section contains:
- `archetype_kind: "alternative_set"` (because subsumption was incomparable),
- `primary.archetype: "serialized-actor"`, `primary.contributing_archetypes: ["serialized-actor"]`, `primary.emittable: true`,
- exactly one entry in `alternatives` for `keyed-partitioned-state` with non-empty `rationale`,
- a corresponding `Kind: "actor"` adapter record in the report's adapters section.

Plus: every property-ID reference in `pkg/` and `cmd/` uses a typed `liftability.PropertyID` constant; bare property-ID string literals fail review (or a linter, planner picks).

## Non-goals

- Composite emission of any kind (Mattermost slice).
- Runtime-side hosting or Go source generation for the actor adapter.
- Full migration of the archetype catalog to the property-set form (only the two Caddy needs).
- Any change to the `MLV2_*` refusal-code taxonomy. Out of scope per ADR-0017's stability commitment.
- ADR-0019, ADR-0020, ADR-0021 work. Reserved-number placeholders, not this sprint.
- Any quantitative scoring or liftability-metric work. ADR-0020 territory.
- `docs/site/` revisions. Handled separately by the docs subagent that ran in parallel with this sprint's planning.

## Open questions for the planner

The drafting models should reach a decision on each:

1. **Where exactly does the candidate-set logic live?** Sibling files under `pkg/compiler/stateclass/` is the recommendation in this brief, but the planner may propose a new sibling package if there's a reason. New top-level package only with strong justification.
2. **Linter vs. convention** for forbidding bare property-ID strings. Pick one; document.
3. **Test fixture form for the candidate-set machinery:** unit tests against minimal hand-rolled fixtures in `pkg/compiler/stateclass/testdata/` plus an e2e assertion on the live Caddy target, or just one or the other? Default: both, per the OOM lessons from SPRINT-0009 (unit tests on tiny fixtures; e2e for the real-corpus assertion).
4. **The exact field names for the `Kind: "actor"` adapter record.** ADR-0022 doesn't specify these; pick something defensible and consistent with the existing `handler` and `registry` adapter records.
5. **How to surface the four utility-tier fallback rules in the rationale.** A short structured tag per tier, free-text prose, or both. Reportv2 readers will see this; should be human-legible without being noisy.
6. **`archetype_kind` value for the Caddy case.** ADR-0022 Decision 3 says `"alternative_set"` applies when "subsumption was not decisive and multiple incomparable candidates exist in the match set" — which is exactly the Caddy case. But the same Decision 3 also says `"single"` applies "when one candidate subsumes" the others, and in the Caddy case the report does designate a single primary (via utility-tier fallback, not via subsumption). The ADR text leaves it ambiguous which `archetype_kind` value applies when subsumption is incomparable but a primary is still chosen. Pick one and document the rationale; the success signal in this brief tentatively says `"alternative_set"` but the committee should overrule if the other reading is cleaner. This ambiguity is itself a useful artifact for ADR-0022 — flag it as a candidate for a small ADR clarification.

## Roadmap follow-ups (visible from this brief)

- **SPRINT-0018 (or next available):** Mattermost `Hub`/`WebConn` composite slice. Exercises composite emission, compatible-refinement coherence check, AND-rule eligibility inheritance, full `archetype_kind: "composite"` path.
- **Future sprint (un-numbered):** Runtime hosting of actor adapters; closes the runtime-truthfulness gap that this slice's `emittable: true` semantics points at.
- **Future sprint (un-numbered):** Full archetype catalog migration to the property-set form. Mechanical once the slice proves the pattern.
