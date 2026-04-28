# ADR-0022 — Composite-archetype regions

**Status:** accepted
**Date:** 2026-04-23
**Supersedes:** none
**Supersedes by:** none
**Related:** ADR-0015 (canonical-shape classifier), ADR-0016 (state-class inference), ADR-0017 (classifier reasons about liftability), ADR-0018 (liftability-property taxonomy). Planned: ADR-0019 (remediation surface), ADR-0020 (auto-lift evidence thresholds), ADR-0021 (pragmas as evidence vs. overrides).

**Origin:** SPRINT-0016 committee drafting (opus + gpt-5.4 + gemini) + cross-critique + opus synthesis. Committee drafts and critiques preserved at `docs/sprints/drafts/SPRINT-0016-*.md`.

**Terminology note (post-acceptance revision).** This ADR was originally drafted using the words *subsumption* / *subsume* and *monotone refinement*. Those terms have been renamed in the prose below to *subsumption* / *subsume* and *compatible refinement* respectively. Two reasons: "subsumption" collides with the CFG dominator-tree concept that has nothing to do with this ADR, and "monotone" was decorative jargon that gestured at order-theoretic monotonicity without formalizing one — "compatible" says the actual claim plainly (the component refinements do not conflict). Decisions are unchanged; only the words are different.

---

## Context

SPRINT-0013 established that Monolift's v1 archetype vocabulary (8 archetypes) consists of **overlapping lenses on the region space, not a partition of it**. Multiple corpus regions match more than one archetype simultaneously:

- **Caddy `Handler.connections` (C5)** — matches `serialized-actor` (mutex-protected receiver-scoped state; no pointer escape) and `keyed-partitioned-state` (connections keyed by connection ID). Both AUTO evidence sets can hold simultaneously; either transform (actor harness or keyed-shard service) is technically viable.
- **Mattermost `Hub` / `WebConn` (MM1 + MM2)** — matches `keyed-partitioned-state`, `fanout-publisher`, and `session-affinity-state` simultaneously. SPRINT-0015 identifies this region as the **single strongest PLOS §4.2 demo in the corpus** — the one region where workload-responsive placement switching is most compelling.

SPRINT-0013 recorded the leaning "the compiler picks the more-constrained archetype when multiple fit — ADR-0022 codifies the precedence" but did not resolve the decision procedure. SPRINT-0015's utility analysis elevated ADR-0022 from "later cleanup" to load-bearing for the PLOS thesis demonstration: a compiler that can only emit single-archetype transforms cannot demonstrate the mattermost hub composite case.

### The trade-off space the ADR navigates

The underlying problem is not precedence-tie-breaking but navigation of a trade-off space: for any liftable region, the classifier may discover a **set of viable candidate transforms**, some narrow and some composite. Each candidate has different preservation properties (preserves local fast path? preserves ordering? preserves ownership invariants?), different utility profiles (PLOS §4.2 fit varies per archetype per SPRINT-0015), and different runtime-selection eligibility (the `dynamic-delegate-eligible` predicate varies per archetype).

The compiler's job is to structure this navigation, not force a single scalar outcome too early. Three concrete decisions follow from this framing.

### Adjacent problem area (non-goal for this ADR)

SPRINT-0015 surfaced the adjacency of a **liftability metric** — a structured way to score regions on feasibility, utility, dynamic-placement eligibility, and composite richness. This ADR does not define that metric. It defines the decision procedure a future metric would formalize. ADR-0020 (auto-lift evidence thresholds) is the planned home for quantitative scoring work.

---

## Decision

Monolift treats composite-archetype classification as **candidate-set construction plus candidate selection**, not as forced single-label assignment. The classifier produces a match set; the compiler projects that set into a primary candidate plus alternative and composite candidates per the rules below.

### Decision 1: Precedence via region-relative subsumption

When a region's match set contains multiple archetypes, the classifier computes **subsumption** to determine the primary candidate:

**Subsumption relation.** Archetype candidate A *subsumes* candidate B on a region R iff:
1. Every invariant B's transform requires on R is also required by A's transform on R (A's evidence is a superset of B's).
2. A requires at least one additional invariant not required by B.
3. A's transform preserves at least the same externally observable semantics as B's on R.

When one candidate subsumes all others, it is the primary. Subsumption is a partial order: **incomparable candidates are possible and correct**, not a failure mode. The classifier must not invent a fake winner when subsumption is not decisive.

**Rationale.** A subsuming candidate's transform has strictly stronger proof obligations on the same region, meaning it makes use of more of the region's observable properties when generating the lift. Emitting the subsuming transform preserves more of the region's semantic intent. Unlike a global archetype ladder (rejected below), subsumption is **region-relative**: the same archetype may subsume another on region X and be subsumed by it on region Y. The corpus supports this; it does not support a single vocabulary-wide total order.

**Fallback when candidates are incomparable.** When the match set has multiple incomparable candidates and no registered composite covers them, the classifier selects a default candidate using utility-aware fallback tiers, in order:

1. Prefer candidates that preserve a local fast path and remain `dynamic-delegate-eligible` (the PLOS §4.2 property).
2. Prefer candidates that preserve more of the region's native state topology rather than collapsing it into a more generic owner pattern.
3. Prefer the candidate with lower operator-attention cost (fewer new external dependencies, smaller ops surface introduced).
4. If still tied, use first-seen classifier order as a deterministic implementation detail — this is a stability property, not a semantic claim.

The other incomparable candidates are **not discarded**; they are recorded as alternatives in the report.

**Caddy C5 worked example.** The match set is {`serialized-actor`, `keyed-partitioned-state`}. Applying subsumption: `serialized-actor` requires mutex-enclosure-of-store, pointer-escape-absence, and receiver-scope invariants. `keyed-partitioned-state` requires keyed-access-invariant and receiver-ownership. Neither's invariants are a strict superset of the other's, so they are **incomparable**. Falling through utility tiers: both are `dynamic-delegate-eligible` (tie on tier 1); `serialized-actor` preserves the native single-owner topology of the connections struct while `keyed-partitioned-state` collapses it into a key-based shard (tier 2 breaks in favor of `serialized-actor`). Result: `serialized-actor` is primary; `keyed-partitioned-state` is recorded as alternative. The report exposes both.

**Pragma override.** A developer-supplied `archetype=<name>` pragma (governed by ADR-0021) may designate any candidate in the match set as primary, overriding the subsumption + fallback computation. The override is recorded in the report's `pragma_provenance`.

---

### Decision 2: Composite emission

The compiler emits a **composite candidate** — a single named transform combining multiple archetypes' transforms into one coherent lift — when all of the following hold:

1. **All component archetypes independently match at AUTO** on the region. Each component's evidence conditions are satisfied as if it were the sole matching archetype.
2. **The composite is coherent.** The component transforms must be **compatible with each other**: adding one transform must refine the placement/routing/ownership strategy of another, not invalidate it. Concretely, no component transform may assume exclusive ownership of a state object that another component transform must also route through.
3. **The composite has a concrete emission sketch** demonstrating that the combined transform is writeable as code the compiler can generate.

When these three conditions hold for a region, the composite candidate becomes a member of the candidate set alongside any single-archetype candidates.

**Composite identity is compositional, not nominal.** The normative identity of a composite candidate is **the list of contributing archetypes plus the region** — not a separate named catalog entry. This preserves SPRINT-0013's scope fence against reopening the v1 archetype vocabulary. Reports may expose a human-readable alias (see Decision 3) for demo and documentation purposes, but the formal identity is compositional.

The specific composite covering mattermost MM1+MM2 — `keyed-partitioned-state` + `fanout-publisher` + `session-affinity-state` — is the first observed instance satisfying the three conditions above. Its transform (consistent-hash routing on connection/user ID → per-shard hub replica with broadcast via pub/sub and per-connection state on the routed replica) is the one the PLOS §4.2 demo requires. The report may surface this composite under the alias `connection-hub-buffer` (gpt-5.4's proposed name from SPRINT-0013 cross-run analysis), but that alias is a reporting convenience, not a catalog-level vocabulary addition. Future corpus evidence of the same composite pattern in a second target may justify promoting the alias to a governed name via a future ADR; until then, the alias is informal.

**Coherence check — mattermost MM1+MM2 worked example.** `keyed-partitioned-state` refines ownership (per-user keying). `session-affinity-state` refines routing (sticky-connection-to-replica). `fanout-publisher` refines delivery (broadcast to subscribers of this user). These three refinements are compatible: each one strictly narrows a routing/ownership decision made by the others rather than contradicting. The coherence test passes. The composite transform is `connection-hub-buffer`; the report alias applies.

**Coherence check — caddy C5 counter-example.** `serialized-actor` claims exclusive ownership of the connections struct for serialized mutation; `keyed-partitioned-state` would shard the struct by key, implying non-exclusive ownership of the same struct across shards. The two are not compatible — sharding invalidates single-owner serialization. **No composite is emitted for caddy C5**. The region resolves to `serialized-actor` primary (per Decision 1) with `keyed-partitioned-state` as alternative.

**Partial composite forbidden.** When a candidate composite requires all N component archetypes at AUTO and only N-1 are satisfied, **no composite is emitted**. The region falls through to Decision 1's single-archetype primary selection over the N-1 satisfied archetypes. There is no "partial composite" mode.

**Governance for additional composites.** If future corpus evidence supports promoting a composite pattern to a governed named identity (stable report label, consistent cross-region recognition), that promotion requires a future ADR — not pragma or inline annotation. The criterion SPRINT-0013 established for archetype promotion (≥2 instances across ≥2 targets) applies to composite promotion as well. ADR-0022 itself registers one composite *observation* (`connection-hub-buffer` in mattermost); promotion to a stable catalog entry awaits a second observation.

---

### Decision 3: Report format — exposing the candidate set

The `reportv2` archetype section must represent the full candidate set. A scalar `archetype` field, as introduced by SPRINT-0014, is insufficient for composite-capable regions. The following fields extend SPRINT-0014's baseline additively:

```json
{
  "archetype": "string",
  "archetype_kind": "single" | "composite" | "alternative_set",
  "primary": {
    "archetype": "string",
    "contributing_archetypes": ["string"],
    "alias": "string | null",
    "emittable": true | false,
    "runtime_selectable": true | false,
    "dynamic_delegate_eligible": true | false
  },
  "alternatives": [
    {
      "archetype": "string",
      "contributing_archetypes": ["string"],
      "alias": "string | null",
      "verdict": "AUTO" | "SUGGEST",
      "emittable": true | false,
      "runtime_selectable": true | false,
      "dynamic_delegate_eligible": true | false,
      "rationale": "string"
    }
  ],
  "pragma_provenance": "string | null"
}
```

**Field semantics.**

- `archetype` — headline label the developer sees. For single-archetype primaries, it equals `primary.archetype`. For composites, it equals the composite's alias (when registered) or a hyphen-joined contributing-archetype list.
- `archetype_kind` — `"single"` when one candidate subsumes; `"composite"` when the primary is a composite candidate per Decision 2; `"alternative_set"` when subsumption was not decisive and multiple incomparable candidates exist in the match set. These three *kinds* are describing the shape of the candidate set, not modes on a single axis.
- `primary.contributing_archetypes` — ordered list. Length 1 for single; length ≥2 for composite.
- `primary.alias` — human-readable composite name when one has been assigned (e.g., `"connection-hub-buffer"`). Null for single-archetype primaries and for composites without a registered alias.
- `primary.emittable` — whether the compiler can emit a transform for this candidate with the current classifier/emitter stack. Separates "candidate exists" from "candidate is generable."
- `primary.runtime_selectable` — whether this candidate can participate in runtime delegate selection (requires `dynamic_delegate_eligible` *and* a compatible call boundary with other runtime-selectable candidates in the alternative set). Separates "static emit" from "runtime-selectable."
- `primary.dynamic_delegate_eligible` — whether the candidate's transform preserves a local fast path that the PLOS §4.2 delegate DSL can switch between. For composites, inherited by AND rule over contributing archetypes.
- `alternatives[*]` — same shape as `primary`, plus `verdict` (whether the alternative itself passes AUTO or only SUGGEST gates) and `rationale` (one-line explanation for why this alternative was not selected as primary).
- `pragma_provenance` — records any developer pragma that overrode the default classification.

**The three orthogonal facts about a candidate** the schema preserves:
- *Candidate exists* (appears in `primary` or `alternatives`).
- *Candidate is statically emittable* (`emittable: true`).
- *Candidate participates in runtime selection* (`runtime_selectable: true`).

These are separately surfaced because the research distinguishes them as independent axes. A candidate may exist without being emittable (no emission sketch yet); a candidate may be emittable without being runtime-selectable (ineligible for dynamic delegation, or call-boundary incompatible with other candidates).

**SPRINT-0014 compatibility.** SPRINT-0014 introduces `archetype` and `pragma_provenance` as minimal fields to `reportv2`. This ADR extends additively: implementations predating full composite support should treat `archetype_kind` as defaulting to `"single"`, `primary.contributing_archetypes` as a single-element list containing `archetype`, `alternatives` as `[]`, and all boolean fields as computed from the single-archetype's catalog entry. No breaking changes to SPRINT-0014's field semantics.

---

### Decision 4: Dynamic-delegate eligibility inheritance (folded)

A composite candidate is `dynamic-delegate-eligible` iff **all** contributing archetypes are individually `dynamic-delegate-eligible`. The rule is AND (not OR) because the local fast path is only preserved when all component transforms can simultaneously revert to in-process execution.

Unanimous across the committee. If any contributing transform is one-way externalization (`ttl-cache`, `filesystem-bound-singleton`, managed-substrate `keyed-partitioned-state`), the composite is not eligible even if other components are. The composite's `dynamic_delegate_eligible` field in the report surfaces this explicitly so developers understand why the composite does not participate in §4.2 runtime switching.

The `dynamic_delegate_eligible` bit is a per-candidate property (not a per-region property). A region may legitimately have a mixed candidate set — one primary that is eligible, one alternative that is not — and the report preserves this.

---

## Consequences

### What this decision enables

1. **The mattermost Hub composite becomes the PLOS §4.2 demo.** ADR-0022 provides the coherence check, the compositional transform identity, the `connection-hub-buffer` alias for reporting, and the report schema to expose runtime-selection eligibility. SPRINT-0015's recommended second-wave flagship (`session-affinity-state` composite on mattermost) can now proceed with a concrete compiler contract.
2. **Caddy C5 has a deterministic primary without ad hoc judgment.** Subsumption is incomparable; utility-tier fallback (tier 2: preserve native ownership topology) selects `serialized-actor` as primary. `keyed-partitioned-state` is recorded as alternative. A future implementation sprint can execute this without reopening the precedence question.
3. **Runtime selection has a defined reporting contract.** The `runtime_selectable` field per candidate tells the runtime which alternatives it may switch among. ADR-0022 specifies *what the report exposes* to enable runtime selection; the runtime mechanism itself (pragma vocabulary, delegate DSL extensions) remains in ADR-0021 and future ADRs. This ADR does not over-commit.
4. **Report format is stable for the ADR roadmap.** ADR-0019 (remediation surface) can cite this ADR for how composite regions appear in reports. ADR-0020 (auto-lift evidence thresholds) can assume the candidate-set representation when specifying per-archetype threshold outputs. ADR-0021 (pragmas) can coordinate on `pragma_provenance` and any future `archetype=<name>` override semantics.
5. **Compositional identity prevents premature vocabulary creep.** Composite candidates are identified by contributing-archetype lists; named aliases are informal report conveniences. This preserves SPRINT-0013's scope fence. Future ADRs can promote observed composites to stable named entries when corpus evidence warrants.

### What this decision forecloses

1. **No global archetype precedence ladder.** The corpus does not support a total order. Subsumption is region-relative.
2. **No partial composite.** If a candidate composite requires all N components and only N-1 are satisfied, the composite does not fire. The region falls through to single-archetype treatment over the N-1 satisfied archetypes.
3. **No compiler-invented composite names.** If a region matches two archetypes without a pre-defined composite (e.g., caddy C5's `serialized-actor` + `keyed-partitioned-state`), the compiler does not synthesize a name. It emits the primary single-archetype transform and records alternatives.
4. **No pragma-defined composites.** New composites require a future ADR. Pragmas do not extend the composite catalog.
5. **Composite dynamic-delegate-eligibility is not overridable by pragma.** The AND rule expresses a physical constraint (local fast path requires all components locally executable); pragma override is semantically incoherent.
6. **No utility scoring in the report.** The `verdict` and `rationale` fields surface the classifier's decision; they do not carry numeric scores. A future ADR (likely ADR-0020) may extend `reportv2` with quantitative fields if the liftability metric work produces them.

### Implementation work implied

- **Classifier** must produce the full archetype match set, not a scalar winner. This is a new output shape for the stateclass pass — the classifier runs all applicable archetype rules and collects the match set before selecting a primary.
- **Subsumption computation** requires comparing archetype invariant sets per-region. The catalog entries must make the "required invariants" explicit enough to compare; some catalog entries may need addenda to clarify invariant sets.
- **Composite catalog** must be a queryable structure (likely a registered list in `pkg/compiler/stateclass/` or an adjacent package) mapping contributing-archetype tuples to coherence checks and emission sketches. At v1 registration, this catalog contains only the mattermost composite observation.
- **`reportv2` schema** must be extended with the fields in Decision 3. SPRINT-0014's additions form the baseline; the composite-specific fields are additive.
- **Pragma coordination** with ADR-0021 for the `archetype=<name>` override semantics. This ADR's `pragma_provenance` field is the foundation; ADR-0021 governs the pragma vocabulary itself.

---

## Alternatives considered and rejected

### Alternative 1: Global archetype precedence ladder

Define a single vocabulary-wide total order (e.g., "composites > externalization-specific > lifecycle-structured > partitioned > general serialization"). Simple to implement, deterministic, easy to document.

**Rejected** because the corpus does not support a total order. The same `serialized-actor` label is sometimes the subsuming narrow reading (gitea queue.Manager as coordinator) and sometimes subsumed by a stronger structural reading (caddy C5 where session/key properties are also present). A fixed ladder encodes arbitrary judgment that region-relative subsumption avoids. The SPRINT-0013 research explicitly found that precedence is region-relative.

### Alternative 2: Cardinality-of-satisfied-conditions as subsumption proxy

Count the number of independent AUTO conditions satisfied by each candidate and take the maximum. Computationally simpler than true subsumption.

**Rejected** because condition counting confuses number with strength. A transform with three strong invariants may be more specific than one with five weak invariants. The caddy C5 resolution under condition-counting would incorrectly prefer the archetype with more checklist items rather than the one with stronger constraint structure. True subsumption (superset-of-invariants relation) avoids this failure mode. (Noted: this alternative appeared in an early committee draft and was self-corrected during cross-critique.)

### Alternative 3: Named composites as first-class v1 archetypes

Promote `connection-hub-buffer` (and any future composites) to full archetype status in the v1 vocabulary with their own state classes and catalog entries.

**Rejected** for three reasons: (a) SPRINT-0013's scope fence against reopening the v1 vocabulary; (b) the coverage-gate criterion (≥2 corpus instances across ≥2 targets) — mattermost is the only observed `connection-hub-buffer` instance; (c) promoting the composite to a first-class archetype would create naming tension with the three contributing archetypes (`keyed-partitioned-state`, `fanout-publisher`, `session-affinity-state`), each of which remains a first-class archetype in non-composite regions. The compositional identity preserves the component archetypes' independent utility.

*Note: if future corpus evidence shows a second instance of the `connection-hub-buffer` pattern in another target, the coverage argument weakens and this alternative should be reconsidered via a future ADR.*

### Alternative 4: Emit every viable candidate; let runtime always choose

Generate N lift extractions per region whenever multiple candidates exist; the runtime's delegate DSL selects at execution time.

**Rejected** because many alternatives are not runtime-switch compatible (different call boundaries), some candidates exist without being emittable (no emission sketch), and emitting uniformly imposes unnecessary implementation cost for cases where a single primary is clearly correct. The ADR's position is: *the report exposes the candidate set; runtime selection is available when the candidates are mutually compatible and all are eligible; default behavior emits only the primary.*

### Alternative 5: User-prompted selection for all composites (pragma required)

Never emit a composite transform without an explicit `archetype=<composite>` pragma. Even when all composite component archetypes are at AUTO and the coherence check passes, require developer opt-in.

**Rejected** because the auto-lift premise of Monolift is that the compiler recognizes patterns without annotation. The pragma-override path in Decision 1 preserves developer control when their judgment differs from the classifier. But the default should be: if the compiler observes the pattern, the compiler reports and (when emittable) emits it.

### Alternative 6: Composite dynamic-delegate-eligibility via OR

Inherit eligibility from any eligible contributing archetype rather than requiring all.

**Rejected** because the local fast path requires all component transforms to be simultaneously locally executable. If one component has moved state to a managed external substrate (`ttl-cache`, `filesystem-bound-singleton`), the composite's "local" form is incoherent — the composite's local behavior cannot be restored without the externalized state returning to in-process. The AND rule expresses this physical constraint.

### Alternative 7: Liftability-metric scoring in the report

Include a numeric `utility_heuristic` score per candidate in the report, driving runtime selection by score comparison.

**Rejected** on scope grounds: SPRINT-0013's followups planned ADR-0020 as the home for auto-lift evidence thresholds (the quantitative work). Introducing scoring in ADR-0022 would pre-empt ADR-0020 and couple the composite-decision contract to a scoring system that has not been designed. The `verdict` + `rationale` fields preserve the classifier's reasoning without committing to a metric.

---

## Naming-collision check

Cross-checked against ADR-0015, ADR-0016, ADR-0017, ADR-0018, `docs/specs/monolift-v2-contract.md`, and `docs/specs/liftability-properties.md`:

| Term introduced | Status | Notes |
|---|---|---|
| `candidate set` | **Safe** | No prior use. New normative term. |
| `subsumption` / `subsume` | **Safe** | Not used in any prior ADR or spec. Renamed from `dominance` / `dominate` post-acceptance to avoid collision with the CFG dominator-tree concept; see Terminology note above. |
| `composite candidate` | **Safe** | "Composite" appears in ADR-0016 only as a pass name (composite post-pass), not as a vocabulary term. No conflict. |
| `connection-hub-buffer` (alias) | **Safe, informal** | Informal alias only; normative identity is compositional. First formal registration here, as an observed-but-not-promoted composite. |
| `archetype_kind` | **Safe** | New `reportv2` field. |
| `primary` (as report section) | **Safe** | New field structure in `reportv2`. |
| `alternatives` (as report section) | **Safe** | New field structure in `reportv2`. |
| `contributing_archetypes` | **Safe** | New field. |
| `emittable` | **Safe** | New field. No conflict with existing "emission" terminology (which refers to the transform generation act). |
| `runtime_selectable` | **Safe** | New field. Related concept to PLOS §4.2 delegate DSL; not a prior ADR term. |
| `dynamic_delegate_eligible` | **Extends SPRINT-0015** | Introduced as a concept in `docs/research/utility-analysis/utility-scenarios-v1.md` §5.2; first formal ADR registration here. |
| `pragma_provenance` | **Extends SPRINT-0014** | SPRINT-0014 is adding this field. ADR-0022 extends its usage to composite/alternative contexts. Additive; no collision. |

ADR-0019, ADR-0020, ADR-0021, and ADR-0023 are planned but not yet written. No terms introduced here conflict with the described scope of those future ADRs. Coordination points for future ADRs:

- **ADR-0021** must settle whether `archetype=<name>` is an evidence-pragma or override-pragma. This ADR's position: it is an **override** (the developer is asserting authoritative intent, not supplying missing classifier evidence).
- **ADR-0019** will specify how the candidate set appears in SUGGEST-surface remediation text. The `rationale` field on alternatives is the foundation.
- **ADR-0020** may extend `reportv2` with numeric scoring fields. They are additive to this ADR's schema.

## Clarification (2026-04)

`archetype_kind` describes how the candidate set was reduced, not how many primary candidates are reported. `single` means subsumption selected one survivor; `alternative_set` means multiple candidates were incomparable and the utility tiers selected the primary; `composite` means a composite candidate was emitted.

## Addendum: SPRINT-0022 multi-root regions

SPRINT-0022 extends this ADR from composite classification into the multi-root analysis pipeline needed to observe the Mattermost Hub/WebConn region.

The closure pass runs once per declared root, unions the included and excluded symbol sets, and records reachability-based provenance per symbol. Provenance is report metadata only; it does not assign goroutine roles. In mutually referenced Go types, a symbol may correctly carry multiple roots in provenance.

Inter-root seams are detected from SSA, not from closure symbol entries. The committed load-bearing seam kind is a channel-typed struct field: `*ssa.Send`, `*ssa.UnOp{Op: token.ARROW}`, and `*ssa.Select` states on field operands record writer and reader roots. Mutex-field and atomic-field seams are recorded as structural metadata but do not participate in SPRINT-0022 admission.

Region admission is per-root transport admission ANDed with a region-level channel-seam shape check: if every writer and reader root of a channel seam is a member of the lifted region, the channel remains in-process when the region is emitted as one service and the seam admits.

---

## Committee notes

Three-way committee draft + cross-critique preserved at:
- `docs/sprints/drafts/SPRINT-0016-{CLAUDE,CODEX,GEMINI}.md`
- `docs/sprints/drafts/SPRINT-0016-{CLAUDE,CODEX,GEMINI}-critique.md`

**Committee convergences adopted in this ADR:**
- Match-set mental model (Claude framing; Codex concurrence).
- Region-relative subsumption rejecting global ladder (Codex formulation; Claude and Gemini self-critiques conceded).
- AND rule for composite dynamic-delegate-eligibility (unanimous).
- Compositional identity for composites with informal alias (Codex primary position; adopted as the narrow-safe middle between Claude's registered-catalog proposal and Gemini's promotion-to-first-class).
- Concrete `reportv2` schema (Claude spine, refined per Codex's "recognized vs. emitted vs. runtime-selectable" distinction).
- Dropping Gemini's `utility_heuristic` scores as out-of-scope (ADR-0020 territory).

**Committee disagreements, decisions recorded:**
- **Condition-counting vs. subsumption for "most constrained":** this ADR adopts subsumption per Codex's formulation. Claude's cardinality-count is a simpler approximation that can misfire; subsumption is the semantically correct criterion.
- **`connection-hub-buffer` as registered name vs. informal alias:** this ADR adopts informal alias. Claude argued for registered naming citing SPRINT-0014's string-stability requirement; Codex argued for compositional-only. The narrow middle: the report may expose the alias for human-readable demo storytelling, but the formal identity stays compositional until a future ADR promotes it based on ≥2-corpus-instance evidence.
- **Runtime-selectable alternatives and pragma semantics:** this ADR specifies the *reporting contract* (what `runtime_selectable` means) but defers the mechanism and pragma vocabulary to ADR-0021 and future ADRs. Claude's `alternatives=all` pragma proposal is noted but not adopted in this ADR's normative text.
