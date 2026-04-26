# SPRINT-0017 — ADR-0022 vertical slice (Caddy `Handler.connections`)

**Status:** completed
**Brief:** [`SPRINT-0017-BRIEF.md`](./SPRINT-0017-BRIEF.md)
**Anchor ADRs:** ADR-0017 (layered architecture), ADR-0018 (property taxonomy), ADR-0022 (composite-archetype regions, post-2026-04-25 `subsumption`/`compatible refinement` rename).
**Drafts:** `docs/sprints/drafts/SPRINT-0017-{CODEX,GEMINI,CLAUDE}.md` and corresponding critiques.

## Intent

Run Caddy `Handler.connections` end-to-end through the new four-layer pipeline so that the report exhibits a complete `alternative_set` outcome: candidate-set construction yields `{serialized-actor, keyed-partitioned-state}`, subsumption returns `Incomparable`, utility-tier fallback selects `serialized-actor` as primary on tier 2 (native state topology), `keyed-partitioned-state` is recorded as alternative with non-empty rationale carrying the tier tag, and `deriveAdapters` produces a real `Kind: "actor"` record so `emittable: true` is non-vacuous. The slice exercises every part of ADR-0022 *except* composite emission, which is SPRINT-0018 (Mattermost `Hub`/`WebConn`).

## Goals

1. Layer 1 property IDs are typed (`liftability.PropertyID`); bare-string usage is forbidden by an AST-based test.
2. Layer 2 archetypes carry first-class required-property sets for `serialized-actor` and `keyed-partitioned-state` (other archetypes stay on the legacy path).
3. Layer 3 (new) constructs a candidate set, runs subsumption, applies utility-tier fallback, and records alternatives — with forward-design seams in place for composite emission.
4. Layer 4 emits a `Kind: "actor"` adapter for the chosen primary, descriptive only.
5. `reportv2` gains additive ADR-0022 Decision 3 fields (`archetype_kind`, `primary`, `alternatives`, `pragma_provenance`).
6. The Caddy e2e fixture asserts the full chain.

## Non-goals

- Composite emission, compatible-refinement coherence check, AND-rule eligibility inheritance, composite catalog (all SPRINT-0018).
- Runtime hosting of the actor adapter; real Go source generation for the actor wrapper.
- Full migration of the archetype catalog (only the two Caddy archetypes).
- Any change to `MLV2_*` refusal codes.
- ADR-0019 / ADR-0020 / ADR-0021 work.
- `docs/site/` revisions (handled by separate docs-site subagent).
- Quantitative scoring or liftability-metric work (ADR-0020 territory).

## Planner decisions on the six open questions

These are settled before execution begins; do not re-litigate.

1. **Where the candidate-set logic lives.** Sibling files under `pkg/compiler/stateclass/`: `archetype.go`, `candidates.go`, `subsumption.go`, `tiers.go`, `selection.go`. No new package.
2. **Bare-string property IDs: enforcement form.** A Go test at `pkg/compiler/liftability/property_lint_test.go` that walks `pkg/` and `cmd/` AST string literals via `go/parser` and fails on any matching the property-ID regex outside `pkg/compiler/liftability/property.go`. Per-file opt-out via `// liftability:allow-string-literals` comment for the rare legitimate case (URLs, file paths). Whitelist constant declarations, JSON struct tags, and explicitly-named test golden files. No external linter dependency.
3. **Test fixture form.** Both: hand-rolled minimal fixtures in `pkg/compiler/stateclass/testdata/candidates/` exercise each subsumption / tier / flag branch; the Caddy e2e provides the single real-corpus assertion. SPRINT-0009 OOM lessons: unit tests stay fast; e2e gates honesty.
4. **`Kind: "actor"` adapter field shape.**
   - `Kind: "actor"`
   - `ID: "serialized-actor"` (stable archetype-derived ID; multi-region disambiguation comes from `MatchedSymbols`, not the ID)
   - `MatchedSymbols`: the receiver type identity (e.g., `Handler`) plus `Handler.connections` when the candidate evidence identifies it
   - `CanonicalShapes`: the existing root canonical shape (typically `http-handler` for this target) — preserves the Layer 4 transport-shape continuity
   - `StateEffects: ["serialized-owner", "mutex-serialized-state"]`
   - `TransportEffects: ["rpc-command-mailbox"]`
   - `SerializationEffects: ["command-envelope"]` — descriptive, does *not* commit to a wire format (no `gob`, no `json`)
5. **Utility-tier rationale surface.** Both a structured tag *and* short prose. Reportv2 gains `rationale_tier` (validated enum: `[PLOS-EL]`, `[TOPOLOGY]`, `[OPS-COST]`, `[STABILITY]` — these are also the human-grep tags surfaced inline). The existing free-text `rationale` (≤140 chars) carries one sentence of prose. Tags map to ADR-0022 Decision 1 tiers in order. Schema validates the enum.
6. **`archetype_kind` value for Caddy.** `"alternative_set"`. Subsumption was incomparable; the report is honest about there being multiple non-subsumed candidates, with `primary` chosen by tier fallback. `"single"` is reserved for cases where subsumption itself collapses the set to one. This reading needs to be recorded as an ADR-0022 clarification appendix in this sprint (not a full ADR-0023 — the ambiguity is one sentence).

## Architecture overview

The work fits ADR-0017's layered architecture:

- **Layer 1** (`pkg/compiler/liftability/`) — exists; this sprint adds typed constants + lint test only.
- **Layer 2** (`pkg/compiler/stateclass/archetype.go`, new) — refactor: archetype as first-class data with required-property set drawn from Layer 1 vocabulary.
- **Layer 3** (`pkg/compiler/stateclass/{candidates,subsumption,tiers,selection}.go`, new) — entirely new code.
- **Layer 4** (`pkg/compiler/extract/extract.go` `deriveAdapters`) — extended to recognize the new `actor` kind.

## Sequencing

Three blocks; within a block, items are roughly parallelizable; across blocks, strictly serial. Validation gates between blocks are explicit.

**Block A — substrate + region evidence (do first).** Unblocks both new-logic and integration work.
1. Layer 1 typed `PropertyID` migration + lint test.
2. Layer 2 archetype property-set type + the two Caddy entries + new evidence-property IDs.
3. Caddy region evidence emission (the seed-harvesting / fact-derivation work for `Handler.connections`).
4. Layer 4 `reportv2` schema additions (types + JSON schema only; wiring in Block C).

Validation gate: `go test ./pkg/compiler/liftability/...` and `go test ./pkg/compiler/reportv2/...` green.

**Block B — new logic.** Depends on A.
5. Layer 3 implementation (candidates / subsumption / tiers / selection).
6. Unit-test fixtures in `pkg/compiler/stateclass/testdata/candidates/` developed alongside.
7. Layer 4 actor adapter derivation.

Validation gate: `go test ./pkg/compiler/stateclass/...` green; the `Kind: "actor"` adapter unit test passes.

**Block C — integration.** Depends on A and B.
8. Wire `ClassifyRegion` into the pass currently invoking `stateclass.Infer`; translate `Classification` into the new `reportv2` archetype fields.
9. Caddy e2e fixture extensions and golden regeneration.
10. ADR-0022 clarification appendix; `docs/evolution.md` entry.

Validation gate: full `go test ./pkg/...` green; Caddy e2e assertions pass.

## Tasks

### Block A — substrate + region evidence

#### A.1 — Layer 1 typed `PropertyID` discipline

- [x] Audit `pkg/compiler/liftability/property.go`. Confirm `PropertyID` is `type PropertyID string`. Enumerate every existing constant. Add any property referenced anywhere in `pkg/` or `cmd/` that lacks a corresponding constant.
- [x] Grep `pkg/` and `cmd/` for bare property-ID string literals (regex `^[a-z]+(\.[a-z_-]+)+$` anchored). Replace each with the corresponding `liftability.PropertyID` constant. Touch-points expected to include: `pkg/compiler/extract/extract.go`, `pkg/compiler/stateclass/stateclass.go`, `pkg/compiler/diagnostics/translate.go`, `pkg/compiler/reportv2/report.go`, every `*_test.go` under those paths, `test/e2e/targets/*/target.go`. Land this as a single mechanical commit so reviewers can skim the diff fast.
- [x] Add `pkg/compiler/liftability/property_lint_test.go`: walks `pkg/` and `cmd/` with `go/parser`, visits string literals (not comments), fails on any matching the property-ID regex outside `property.go`. Allowlist: constant declarations, JSON struct tags. Per-file opt-out via `// liftability:allow-string-literals` directive. Each opt-out grep-able for review.
- [x] Add a one-line note in `docs/decisions/0018-liftability-property-taxonomy.md` Implementation Notes section: "PropertyID is a typed Go constant; bare-string property IDs are forbidden, enforced by `property_lint_test.go`."

#### A.2 — Layer 2 archetype required-property sets

- [x] Add `pkg/compiler/stateclass/archetype.go`:
  - `type ArchetypeID string` with constants `ArchetypeSerializedActor` and `ArchetypeKeyedPartitionedState`.
  - `type Archetype struct { ID ArchetypeID; Name string; Required map[liftability.PropertyID]liftability.Verdict }` — map form handles "must hold" and "must not hold" cases. Subsumption operates on the *keys* (the property IDs being demanded); `ConstructCandidates` checks the *values* (the verdicts demanded) at match time.
  - Package-level `archetypes` registry keyed by `ArchetypeID`.
- [x] Define new typed `liftability.PropertyID` constants for archetype evidence not yet in ADR-0018: at minimum `state.mutex-encloses-store-invariant`, `state.receiver-owned-state`, `state.keyed-access-invariant`. Mark these as **archetype-evidence properties** (non-gating outcome class) in their docstrings. Append a one-paragraph note to ADR-0018 covering the new IDs and that they are non-gating.
- [x] Populate `serialized-actor` required map: `effects.no-param-heap-mutation: Hold`, `effects.no-param-escape: Hold`, `effects.no-global-writes: Hold`, `state.mutex-encloses-store-invariant: Hold`, `state.receiver-owned-state: Hold`.
- [x] Populate `keyed-partitioned-state` required map: `effects.no-global-writes: Hold`, `state.receiver-owned-state: Hold`, `state.keyed-access-invariant: Hold`.
- [x] Leave the existing stateclass `Class*` enum and `inferClass` rules in place; they continue to serve other call sites. The new candidate machinery reads from `archetypes`, not from `Class*`. Note this in the file header. Migration of the other archetypes is **explicitly out of scope** — flag in `docs/evolution.md` as a follow-up.
- [x] Add tests that the two catalog entries expose deterministic required-property sets and stable IDs.

#### A.3 — Caddy region evidence

This is the sprint's hidden cliff: if the property evidence doesn't currently exist for `Handler.connections`, the Layer 3 machinery has nothing to chew on. Validating evidence first is cheaper than discovering the gap at the e2e gate.

- [x] Inspect the existing Caddy report (`test/e2e/targets/caddy/golden/report.json` or equivalent) for the region containing `Handler.connections`. Identify which evidence properties already fire and which are missing.
- [x] Extend stateclass seed harvesting or derived facts to identify `Handler.connections`, `Handler.connectionsMu`, and the lock-protected map updates in Caddy's `streaming.go` (or wherever the connections map mutates). The receiver is `Handler`, the reachable method is `ServeHTTP`; today's seed prepopulation may not surface the connections field as a distinct state region.
- [x] Add focused stateclass fixtures in `pkg/compiler/stateclass/testdata/fixtures/`:
  - `mutex-keyed-map/`: a mutex-protected map with keyed add/delete/iteration mirroring the Caddy connections pattern. Should produce both `serialized-actor` *and* `keyed-partitioned-state` candidate matches.
  - `mutex-only-store/`: a mutex-protected single-value store (no keyed access). Should produce only `serialized-actor` — exercises the N=1 candidate-set path with no alternative.
  - `keyed-no-mutex/`: a keyed-access store without serialization. Should produce only `keyed-partitioned-state`.
- [x] Add candidate-construction tests against these fixtures. The N=1 fixtures (single-archetype matches) flow through the same code path as the N=2 case — there must be no `if len(candidates) == 1` short-circuit.

#### A.4 — `reportv2` schema additions

- [x] In `pkg/compiler/reportv2/report.go`, extend `Root` with four new optional fields:
  ```go
  ArchetypeKind     string             `json:"archetype_kind,omitempty"`
  Primary           *ArchetypeChoice   `json:"primary,omitempty"`
  Alternatives      []ArchetypeChoice  `json:"alternatives,omitempty"`
  PragmaProvenance  *PragmaProvenance  `json:"pragma_provenance,omitempty"`
  ```
- [x] Add new types:
  ```go
  type ArchetypeChoice struct {
      Archetype                string
      ContributingArchetypes   []string  // length 1 this sprint; ≥2 reserved for composites
      Alias                    string    `json:",omitempty"`
      Verdict                  string    `json:",omitempty"` // "AUTO" | "SUGGEST" — alternatives only
      Emittable                bool
      RuntimeSelectable        bool
      DynamicDelegateEligible  bool
      RationaleTier            string    `json:",omitempty"` // [PLOS-EL] | [TOPOLOGY] | [OPS-COST] | [STABILITY]
      Rationale                string    `json:",omitempty"`
  }
  type PragmaProvenance struct {
      File string
      Line int
  }
  ```
- [x] Update `pkg/compiler/reportv2/schema.json`:
  - `archetype_kind` enum: `["single", "alternative_set", "composite"]` (composite present in schema even though never emitted this sprint).
  - `rationale_tier` enum: `["[PLOS-EL]", "[TOPOLOGY]", "[OPS-COST]", "[STABILITY]"]`.
  - All new fields optional; pre-sprint goldens must continue to validate.
- [x] Update `pkg/compiler/reportv2/report_test.go`:
  - Sample report builders gain helpers for the new fields.
  - Add a test confirming the four new root fields are optional.
  - Add a round-trip test for an `alternative_set` report with two `ArchetypeChoice` entries and an `actor` adapter.
  - Add a **pre-sprint golden regression test**: load the existing pre-sprint Caddy/Pocketbase goldens (without the new fields) and validate against the new schema. Catches accidental non-additive changes.

### Block B — new logic

#### B.1 — Layer 3 implementation

- [x] `pkg/compiler/stateclass/candidates.go`:
  - `type Candidate struct { Archetype ArchetypeID; ContributingArchetypes []ArchetypeID; Alias string; SatisfiedProperties map[liftability.PropertyID]liftability.Verdict }` (`ContributingArchetypes` always length 1 this sprint; `Alias` always `""` — both exist for forward-compat).
  - `type CandidateSet []Candidate`.
  - `func ConstructCandidates(props []liftability.Evidence) CandidateSet` — iterates the `archetypes` registry, tests each archetype's required map against `props`, includes the candidate iff every required `(PropertyID, Verdict)` pair is satisfied by the corresponding `Evidence`. Returns the satisfied atomic candidates only.
  - `func ExtendWithComposites(set CandidateSet, props []liftability.Evidence) CandidateSet` — no-op identity function this sprint, with a `// SPRINT-0018: composite construction lands here` marker. The seam exists; it just doesn't do anything yet.

- [x] `pkg/compiler/stateclass/subsumption.go`:
  - `type SubsumptionOutcome int` with `OutcomeEmpty`, `OutcomeSingle`, `OutcomeSubsumed`, `OutcomeIncomparable`.
  - `func Subsume(set CandidateSet) (CandidateSet, SubsumptionOutcome)`. Compares the *keys* of each candidate's `Required` map (the property IDs the archetype demands). If candidate A's required-key set is a strict superset of candidate B's, A subsumes B; B drops out of the returned set. If multiple candidates remain incomparable, returns `OutcomeIncomparable` with all of them. `OutcomeSingle` for length 1 input. `OutcomeSubsumed` when subsumption collapses N>1 to length 1. **Note in file header:** subsumption operates on *which properties are demanded*, not on *which verdicts are demanded* — verdicts are checked at construction time.
  - File-level naming discipline: every comment, identifier, exported docstring uses `subsumption`/`compatible`, never `dominance`/`monotone`. Validated by `TestNoLegacyTerminology` (see B.2).

- [x] `pkg/compiler/stateclass/tiers.go`:
  - `type RationaleTier string` with constants `TierPLOSEL = "[PLOS-EL]"`, `TierTopology = "[TOPOLOGY]"`, `TierOpsCost = "[OPS-COST]"`, `TierStability = "[STABILITY]"`.
  - The four tiers map directly to ADR-0022 Decision 1 in order.
  - `func SelectPrimary(set CandidateSet) (primary Candidate, alternatives []Candidate, tier RationaleTier, prose string)`. Walks the four tiers in order; first one to break the tie wins.
  - **Per-archetype tier-2 priority table** (small `map[ArchetypeID]int` — entries only for the two Caddy archetypes this sprint): `serialized-actor: 100` (preserves single-owner topology), `keyed-partitioned-state: 50` (collapses to keyed shards). For Caddy, both are tier-1 eligible (PLOS-EL holds for both), so tier-2 (TOPOLOGY) breaks the tie in favor of `serialized-actor`. Comment links each entry to ADR-0022 Decision 1's worked example. Add a unit test that enumerates the table so additions are visible in diffs.

- [x] `pkg/compiler/stateclass/selection.go`:
  - Top-level entry point `func ClassifyRegion(props []liftability.Evidence) Classification`.
  - `Classification` carries `ArchetypeKind string`, `Primary *Candidate`, `Alternatives []Candidate`, `RationaleTier RationaleTier`, `RationaleProse string`.
  - `ArchetypeKind` derivation: `OutcomeEmpty → ""`; `OutcomeSingle → "single"`; `OutcomeSubsumed → "single"` (one survivor); `OutcomeIncomparable → "alternative_set"`. (Composite branch leaves a `// SPRINT-0018: composite kind set here` marker.)
  - Per-candidate flag computation lives here as **functions, not stored fields**:
    - `Emittable(c Candidate) bool` — returns true for `serialized-actor` (the actor adapter exists for it). Returns false for `keyed-partitioned-state` (no adapter wired this sprint).
    - `RuntimeSelectable(c Candidate) bool` — returns false for both this sprint (no runtime hosting yet). Surface this in the alternative's rationale prose so the report is honest about the gap.
    - `DynamicDelegateEligible(c Candidate) bool` — returns false for both this sprint.
  - No `if len(candidates) == 1 { ... } else { ... }` branch anywhere in selection or orchestration. `OutcomeSingle` is the degenerate path through the same code.

#### B.2 — Unit test matrix (`pkg/compiler/stateclass/testdata/candidates/` and `pkg/compiler/stateclass/*_test.go`)

- [x] Fixture `single-archetype-hold/`: one archetype, all required properties Hold → `OutcomeSingle`, `archetype_kind: "single"`, no alternatives.
- [x] Fixture `subsumed/`: two archetypes, one's required-key set strictly supersets the other's, both satisfied → `OutcomeSubsumed`, smaller drops, kind `"single"`.
- [x] Fixture `incomparable-tier-2/`: two archetypes, neither subsumes the other, tier 1 ties, tier 2 (`[TOPOLOGY]`) breaks → `OutcomeIncomparable`, kind `"alternative_set"`, primary chosen, rationale tier `[TOPOLOGY]`. **Mirrors the Caddy worked example.**
- [x] Fixture `incomparable-tier-fallthrough/`: forces the tie to fall through to `[STABILITY]`; asserts the rationale tier name surfaces correctly.
- [x] Fixture `empty/`: no archetype's required set is satisfied → `OutcomeEmpty`, kind `""`, no primary.
- [x] `TestExtendWithCompositesIsNoOp` — calls `ExtendWithComposites` on each of the above and asserts identity. Protects the SPRINT-0018 seam.
- [x] `TestNoLegacyTerminology` — greps `pkg/compiler/stateclass/*.go` and `pkg/compiler/liftability/*.go` for `dominance`, `dominate`, `monotone`. Fails if found. Protects the 2026-04-25 rename.
- [x] `TestTierTableEnumeration` — enumerates the per-archetype tier-2 priority table in `tiers.go`; any addition must update the test.
- [x] Negative test: confirm no `if N == 1` branch by greppling selection/orchestration source for `len(candidates) == 1` and `len(set) == 1` patterns.

#### B.3 — Layer 4 actor adapter

- [x] In `pkg/compiler/extract/extract.go` `deriveAdapters`:
  - Read `Classification` from the new stateclass output. Plumb it via the existing `ShapeResult` pipeline or add a sibling result type — pick whichever requires fewer signature changes; document the choice in a one-line comment at the call site.
  - When `Classification.Primary.Archetype == ArchetypeSerializedActor` and `Emittable` is true, append:
    ```go
    reportv2.Adapter{
        Kind:                 "actor",
        ID:                   "serialized-actor",
        MatchedSymbols:       []reportv2.SymbolIdentity{rootSymbol, stateFieldSymbol},  // both when evidence identifies them
        CanonicalShapes:      []string{rootShape},  // typically "http-handler" for Caddy
        StateEffects:         []string{"serialized-owner", "mutex-serialized-state"},
        TransportEffects:     []string{"rpc-command-mailbox"},
        SerializationEffects: []string{"command-envelope"},
    }
    ```
  - **No back-channel:** the adapter MUST be derived from `Classification.Primary`, not from any legacy `Class*` field or shape-only signal. Add a unit test that asserts the adapter reads from `Classification.Primary` (e.g., by varying primary while holding shape constant).
- [x] Update report validation to accept `Kind: "actor"`.
- [x] Add a unit test that confirms no `actor` adapter appears when the selected primary is something other than `serialized-actor`.

### Block C — integration

#### C.1 — Wire into the existing pass

- [x] Find the call site of the current `stateclass.Infer` (likely `pkg/compiler/passes/register.go` or an extract-side caller). Additionally invoke `stateclass.ClassifyRegion(props)` and stash the result on the per-region report struct. Both old `Inference` and new `Classification` coexist this sprint; the old call site stays untouched to avoid scope creep.
- [x] Translate `Classification` into the new `reportv2` archetype fields in the report-emit path. Preserve existing state rows and diagnostics — additive only.
- [x] Define an extract-facing plain struct (in `extract` or `reportv2`) that stateclass converts to, mirroring the existing `StateResult` pattern. Avoids creating a new direct dependency from `extract` on `stateclass` types beyond what already exists.

#### C.2 — Caddy e2e

- [x] Inspect `test/e2e/targets/caddy/target.go` and the golden report. Identify the region that classifies as `Handler.connections`.
- [x] Add to `target.go` (extending `harness/target.go` if needed):
  - `ExpectedArchetypeKind: "alternative_set"`.
  - `ExpectedPrimary`: archetype `serialized-actor`, `contributing_archetypes: ["serialized-actor"]`, `alias: ""`, `emittable: true`, `runtime_selectable: false`.
  - `ExpectedAlternatives`: exactly one entry, archetype `keyed-partitioned-state`, `RationaleTierEqual: "[TOPOLOGY]"`, `RationaleNonEmpty: true` (assert non-empty prose rather than exact string to avoid brittleness).
  - `ExpectedAdapterKind: "actor"`, `ExpectedAdapterID: "serialized-actor"` for the corresponding region.
- [x] Confirm the existing `RequiredRootFacts` and HTTP `Invariants` still pass — additive only.
- [x] Regenerate the Caddy golden report (`go test ./test/e2e/... -run Caddy -update` if that flag exists; otherwise hand-edit and document the regen command in the test comment).

#### C.3 — ADR clarification + closeout

- [x] Append a "Clarification (2026-04)" section to `docs/decisions/0022-composite-archetype-regions.md` documenting the §6 reading: `archetype_kind` reflects *how the candidate set was reduced*, not *how many primaries are reported*. Specifically: `single` ≡ subsumption-decided; `alternative_set` ≡ incomparable + tier-decided; `composite` ≡ composite emitted. Cross-link from `docs/evolution.md`.
- [x] Add `docs/evolution.md` entry summarizing the slice landing.
- [x] Add a closeout note (in the SPRINT-0017 doc itself or a sibling closeout file) naming the actor runtime-hosting gap: the actor adapter is descriptive metadata only. The runtime cannot host an actor yet.
- [x] Add a closeout note that Mattermost composite emission is the intended SPRINT-0018 follow-up; record full archetype-catalog migration as another future un-numbered sprint.

## Forward-design sanity check

Before marking the sprint done, the executor must confirm:

- [x] `ConstructCandidates` returns a `CandidateSet`, never a single value.
- [x] `ExtendWithComposites` exists as a no-op seam between construction and selection, with `// SPRINT-0018:` marker.
- [x] `Candidate` has `ContributingArchetypes []ArchetypeID` and `Alias string` from day one.
- [x] `Emittable`, `RuntimeSelectable`, `DynamicDelegateEligible` are functions on the selection package, not struct fields.
- [x] No `if len(candidates) == 1 { ... } else { ... }` branch exists in selection or orchestration. `OutcomeSingle` is the degenerate path.
- [x] `deriveAdapters` reads from `Classification.Primary`, not from any legacy `Class*` field. Verified by unit test.
- [x] `archetype_kind` enum in the schema accepts `"composite"` even though this sprint never emits it.
- [x] Property-ID references in `pkg/` and `cmd/` use the typed `liftability.PropertyID` constant; bare strings caught by `property_lint_test.go`.
- [x] `TestNoLegacyTerminology` passes; no `dominance`/`monotone` in the new code.

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| **Caddy region evidence may not currently emit both candidates.** The single most likely scope-blower. The Layer 3 machinery has nothing to chew on if `Handler.connections` doesn't surface as a state region with `mutex-encloses-store`/`keyed-access` evidence. | Block A.3 inspects the existing report **first**, surfaces the gap before any Layer 3 work, and adds the missing seed harvesting / fact derivation up front. The N=1 fixtures in `mutex-only-store/` and `keyed-no-mutex/` sanity-check the path even if the Caddy region misbehaves. |
| **ADR-0018 doesn't yet contain `mutex-encloses-store-invariant` / `keyed-access-invariant`.** The required-property sets reference IDs that don't exist. | Block A.2 explicitly adds these as new **archetype-evidence properties** with non-gating outcome class. ADR-0018 gets a one-paragraph note appended, not a re-open. |
| **`PropertyID` migration touches every test file.** | A.1 lands as a single mechanical commit — no behavior changes — so reviewers can skim the diff fast. |
| **`extract.Analyze` plumbing creates an awkward `extract → stateclass` dependency.** | C.1 defines an extract-facing plain struct mirroring `StateResult`; stateclass converts to it. |
| **Schema additions break existing golden reports.** | A.4 includes the pre-sprint golden regression test that loads old goldens and validates against the new schema. All new fields are `omitempty`. |
| **Per-archetype tier-2 priority table is hidden config.** | Two entries only this sprint. Each entry comment-linked to ADR-0022 Decision 1's worked example. `TestTierTableEnumeration` makes additions visible in diffs. |
| **`Emittable`/`RuntimeSelectable` semantics drift between this sprint and SPRINT-0018.** | Defined as functions with single-line docstrings now; SPRINT-0018 amends, doesn't redefine. Cross-references to ADR-0022 Decision 2. |
| **Bare-string lint test produces false positives** on URLs, file paths, doc strings. | Anchored regex (`^...$`); per-file `// liftability:allow-string-literals` opt-out, each occurrence grep-able for review. |
| **Adapter `ID` collisions in multi-region reports.** | `ID: "serialized-actor"` is stable; multi-region disambiguation via `MatchedSymbols`. Consistent with `handler` and `registry` adapter records that also use stable IDs. |
| **`actor` adapter `SerializationEffects: ["command-envelope"]` reads as a runtime commitment.** | Closeout note explicitly states the adapter is descriptive metadata only; no runtime hosting is implied. The string `command-envelope` is intentionally generic (does not name a wire format). |

## Acceptance criteria

All must be true at sprint close:

- [x] `go test ./pkg/...` is green on the sprint branch.
- [x] `pkg/compiler/liftability/property_lint_test.go` passes; no bare property-ID strings remain in `pkg/` or `cmd/` (except constant declarations).
- [x] `pkg/compiler/stateclass/` contains `archetype.go`, `candidates.go`, `subsumption.go`, `tiers.go`, `selection.go` with the surfaces named in B.1.
- [x] All five testdata fixtures in B.2 exist and are exercised; `TestNoLegacyTerminology`, `TestExtendWithCompositesIsNoOp`, and `TestTierTableEnumeration` pass.
- [x] `pkg/compiler/reportv2/schema.json` validates pre-sprint goldens (no new fields), reports with `archetype_kind: "single"`, and reports with `archetype_kind: "alternative_set"` plus one alternative.
- [x] Caddy e2e (`go test ./test/e2e/... -run Caddy`) produces a report whose `Handler.connections` region contains:
  - `archetype_kind: "alternative_set"`,
  - `primary.archetype: "serialized-actor"`, `primary.contributing_archetypes: ["serialized-actor"]`, `primary.emittable: true`, `primary.runtime_selectable: false`,
  - exactly one `alternatives` entry for `keyed-partitioned-state` with non-empty `rationale` and `rationale_tier: "[TOPOLOGY]"`,
  - one adapter record of `kind: "actor"`, `id: "serialized-actor"`, with the field shape from §Planner-decisions #4.
- [x] ADR-0022 carries a "Clarification (2026-04)" section recording the `archetype_kind` reading; `docs/evolution.md` cross-links it.
- [x] No runtime actor hosting and no generated actor wrapper code introduced. Closeout notes record both gaps as known follow-ups.
- [x] Roadmap follow-ups recorded: SPRINT-0018 Mattermost composite, runtime-hosting sprint (un-numbered), full archetype-catalog migration sprint (un-numbered).

## Roadmap follow-ups (visible from this sprint)

- **SPRINT-0018:** Mattermost `Hub`/`WebConn` composite slice. Exercises composite emission, compatible-refinement coherence check, AND-rule eligibility inheritance, full `archetype_kind: "composite"` path. The `ExtendWithComposites` no-op seam from this sprint becomes load-bearing.
- **Future sprint (un-numbered):** Runtime hosting of actor adapters; closes the gap that this sprint's `emittable: true` semantics points at.
- **Future sprint (un-numbered):** Full archetype catalog migration to the property-set form. Mechanical once this slice proves the pattern.

## Closeout Notes

- The actor adapter emitted in this sprint is descriptive metadata only. The runtime cannot host an actor yet, and no actor wrapper source is generated.
- Mattermost `Hub`/`WebConn` composite emission is the intended SPRINT-0018 follow-up.
- Full archetype-catalog migration remains a future un-numbered sprint.

## Committee notes

Drafts and critiques preserved at `docs/sprints/drafts/SPRINT-0017-{CODEX,GEMINI,CLAUDE}.md` and corresponding `-critique.md` files.

**Convergences adopted in this plan:**
- Sibling files under `pkg/compiler/stateclass/` (unanimous on no new package).
- AST-based property-ID lint test (Claude framing; both critiques validated as the strongest implementation).
- Both unit tests + e2e (unanimous).
- `archetype_kind: "alternative_set"` for Caddy (unanimous).
- Forward-design seams + sanity check (Claude formulation; both critiques validated).
- `TestNoLegacyTerminology` and `ExtendWithComposites` no-op seam test (Claude unique additions; both critiques adopted).
- Caddy region evidence as a first-class scope item (Codex framing; both critiques validated as critical).
- Adding archetype-evidence `PropertyID` constants and the ADR-0018 amendment paragraph (Codex unique catch; both critiques validated).
- Pre-sprint golden regression test (Claude unique addition; both critiques validated).

**Disagreements resolved in this plan:**
- **Tier ordering for Caddy:** ADR-0022's worked example explicitly says **tier 2 (`[TOPOLOGY]`)** breaks the tie. Claude's draft mistakenly used tier 1; Codex correctly used tier 2. Plan adopts tier 2.
- **Actor adapter shape:** Codex's stable `ID: "serialized-actor"` over Claude's `serialized-actor-<package>.<receiver>` (more golden-stable; multi-region disambiguation via `MatchedSymbols`). Codex's effects vocabulary (`serialized-owner`/`mutex-serialized-state`/`rpc-command-mailbox`/`command-envelope`) over Gemini's `actor-harness`/`gob` (which leaks runtime commitments). `CanonicalShapes` includes the existing `http-handler` per Codex critique of Claude.
- **Subsumption semantics:** Claude's "compare required-key sets" framing adopted, with Codex's clarification that this is paired with construction-time verdict checking. Subsumption operates on which properties are demanded; verdicts are checked at construction.
- **Archetype storage:** Claude's `map[PropertyID]Verdict` over Codex's `[]PropertyID` because the map handles "must not hold" cases that may surface in future archetypes.
- **Rationale surface:** Combined Claude's validated enum schema with Gemini's ADR-anchored tag names (`[PLOS-EL]`, `[TOPOLOGY]`, `[OPS-COST]`, `[STABILITY]`). Best of both: schema-validated *and* grep-friendly *and* ADR-traceable.
- **ADR clarification form:** Codex's lightweight clarification appendix to ADR-0022 over Claude's full ADR-0023 — the ambiguity is one sentence; full ADR is process overkill.
- **Sequencing:** Claude's Block A/B/C structure with validation gates, but with Codex's Caddy region evidence pulled into Block A (instead of deferred to Block C) to surface the seed-visibility risk before Layer 3 is written.
