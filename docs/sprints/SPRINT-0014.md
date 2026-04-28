# SPRINT-0014 — Periodic-invocation vertical slice + ADR-0020/0021 under implementation pressure

**Status:** planned
**Shape:** one vertical slice — one archetype end-to-end, two ADRs drafted against running code, no generalized design work.
**Depends on:** SPRINT-0013 (research catalog + followups).
**Primary inputs:** `docs/research/distribution-archetypes-v1.md`, `docs/research/archetype-catalog-v1.md`, `docs/research/distribution-archetypes-followups.md`, `docs/research/annotations/README.md`, `docs/sprints/SPRINT-0013.md`. ADRs 0015/0016/0017/0018. `docs/specs/liftability-properties.md`, `docs/specs/monolift-v2-contract.md`, `pkg/compiler/stateclass/`.

## Intent

SPRINT-0013 produced eight plausible archetypes, five ripe ADRs, and one sharp unresolved tension: **when is a pragma load-bearing evidence vs. a pure override?** The next sprint's highest-leverage move is not drafting every ADR on paper, and not a generalized signal spike. It is a single **vertical slice** for `periodic-invocation` — the archetype with the broadest corpus support (6/6 targets) — that forces ADR-0021 to be written *against implementation pressure* rather than in abstract.

**Why this slice beats the alternatives.** The committee debated four framings:

- **(a) Draft all ripe ADRs first, no code** — rejected. The research already produced ~100 KB of narrative; four more documents without implementation contact risks internal-consistency drift (ADRs that read right but encode assumptions real SSA would falsify).
- **(b) Highest-leverage ADR + matching state class end-to-end** — chosen. Cheapest way to falsify the catalog's gate-pass claims.
- **(c) Pragma tension first (ADR-0021) standalone** — rejected. Design fiction without implementation grounding. But ADR-0021 *inside* a vertical slice (option b) is exactly right.
- **(d) Something else** — considered and absorbed. An end-to-end emission prototype is included as a falsification step inside (b), not as a standalone sprint.

**Why `periodic-invocation` over `bounded-worker-pool`.** `periodic-invocation` has 6/6 corpus coverage (vs. bounded-worker-pool's 3–4) — success or failure generalizes, not just teaches about one niche. Its transform (platform scheduler) is the cheapest emission sketch to falsify (no actor runtime, no session routing, no composite semantics). Most importantly: `idempotent=true` is load-bearing evidence in the research, not optional metadata. Picking periodic-invocation **forces** ADR-0021 into implementation contact this sprint; picking bounded-worker-pool sidesteps the research's sharpest tension, which the next sprint would pay the same cost for with one fewer data point. Claude's draft argued for deferral; Claude's self-critique conceded this draft's deferral is "defensive, not ambitious."

The research question this sprint answers: **does the `periodic-invocation` archetype survive implementation?** If yes, the catalog's four-gate discipline is validated and generalizes; if not, we learn precisely which gate is weak before drafting three more ADRs on a shaky foundation.

## Goals

1. **ADR-0021 drafted** — pragmas as load-bearing evidence vs. overrides. Evidence pragmas (e.g., `idempotent=true`) supply facts the classifier uses; override pragmas waive a specific refusal code. Explicit precedence, negative-validation rule ("if source contradicts the pragma's claim, the pragma does not save the region"), provenance expectations.
2. **ADR-0020 drafted** — auto-lift evidence thresholds. Only `periodic-invocation` is *normative*; other seven archetypes are stubs citing the catalog, explicitly marked "pending implementation contact." Two-axis structural model (evidence-locality × externalization-affinity) as explanatory framework.
3. **ADR-0016 amendment** adding `periodic-invocation` state class (appended note, not a new ADR).
4. **Pragma surface** — minimal support for `monolift:idempotent=true` as load-bearing evidence on functions/methods. No `monolift:periodic` sugar pragma (scope creep — archetype recognition is the classifier's job, not the author's).
5. **Classifier signal `time-loop-detector`** — SSA pattern recognition for `for { select { case <-ticker.C: ... } }` and `for { time.Sleep(d); ... }`. Specified in `docs/specs/liftability-properties.md`; implemented in `pkg/compiler/stateclass/`.
6. **`periodic-invocation` rule** — promotes qualifying regions from their current refusal codes to AUTO, requires `time-loop-detector` + `idempotent=true` pragma + `effects.no-global-writes` on the loop body.
7. **Emission prototype** for one corpus region (miniflux feedScheduler as canonical target). Produces a standalone scheduler-entry Go file + patched call-site stub. Not wired into `test/e2e/`. Not a runtime library. The output is `.go` for human inspection and a findings companion documenting what the ≤30-line sketch missed.
8. **`reportv2` schema update** — minimal field for `archetype` label + pragma-provenance on the decision. This is the one in-sprint schema change: without it, the pragma-as-evidence distinction isn't observable in the report, which makes ADR-0021 un-testable.
9. **False-refusal regression harness** — per-target test pinning the current ADMITTED set across all six evaluation targets; fails if any region drops out of ADMITTED under the new signal or rule.
10. **Numeric corpus-validation target** — convert ≥4 miniflux periodic regions (feedScheduler, cleanupScheduler, watchdog, metrics) from current refusal to AUTO. Framed as **falsification anchor**, not completion goal: if fewer than 4 convert, the sprint surfaces why, which is the research output.
11. **Closeout** answering three gate-validation questions in writing: did the evidence gate hold, did the emission gate hold, did the boundary gate hold.

## Non-goals

- No ADR-0019 (remediation surface), no ADR-0022 (composite archetypes), no ADR-0023 (lifecycle-state-machine).
- No other archetype state classes (serialized-actor, bounded-worker-pool, keyed-partitioned-state, fanout-publisher, ttl-cache, session-affinity-state, filesystem-bound-singleton). Each is a future-sprint vertical slice.
- No runtime library / harness for periodic-invocation transforms. Emission is prototype-only.
- No plumbing the emitted transform into `test/e2e/`. The prototype's output is inspected, not executed under the harness.
- No additional classifier evidence signals beyond `time-loop-detector` (other four proposed signals each wait for their future slice).
- No additional pragmas beyond `monolift:idempotent=true`. No `monolift:periodic` or archetype-declaration sugar.
- No re-opening the SPRINT-0013 catalog vocabulary. Take the eight archetypes as inputs.

## Scope boundaries

**In scope:**
- `docs/decisions/0020-auto-lift-evidence-thresholds.md` (ADR-0020 draft)
- `docs/decisions/0021-pragmas-as-evidence-vs-overrides.md` (ADR-0021 draft)
- Appended amendment note to `docs/decisions/0016-state-class-inference.md` for the `periodic-invocation` state class
- `docs/evolution.md` entries for ADR-0020 and ADR-0021 (per repo convention)
- `docs/specs/liftability-properties.md` update for `time-loop-detector`
- `pkg/pragma/` minimal surface for `monolift:idempotent`
- `pkg/compiler/stateclass/` new signal + new rule
- `pkg/compiler/reportv2/` minimal schema additions for archetype label + pragma-provenance
- `pkg/compiler/emit/prototype/` (new package, compiler-internal) for the emission prototype
- New findings doc at `docs/research/periodic-invocation-emission-findings.md`
- Regression test for the six-target ADMITTED pinning harness
- Closeout appended to this sprint file

**Out of scope (each becomes a followup, not a scope expansion):**
- Every item in SPRINT-0013 followups beyond ADR-0020-periodic, ADR-0021, state-class-A3, signal `time-loop-detector`, prototype emission for periodic-invocation, and the regression harness.
- `reportv2` schema work beyond the minimum (archetype label + pragma-provenance field). Rich SUGGEST-triage payload remains ADR-0019 territory.
- Any pragma surface beyond `monolift:idempotent=true`.
- The four other proposed evidence signals.

**Halt rule.** If the emission prototype against miniflux feedScheduler reveals the ≤30-line sketch is materially incomplete (requires a runtime harness, or the loop-body extraction requires captured-state work the sketch cannot express), **stop before merging the rule into the main rule stack**. Record as gate failure in the closeout; ADR-0020's `periodic-invocation` threshold stays unvalidated; the catalog's emission-gate discipline gets a counter-example entry.

**Blocker rule.** If `time-loop-detector` turns out to need pragma-sourced evidence (e.g., static analysis cannot distinguish the periodic pattern from general long-running loops), record the gap; drop `periodic-invocation` AUTO claim; keep ADR-0020 draft alive but mark the threshold as unvalidated pending classifier work.

**ADR halt rule.** If implementation reveals ADR-0021's evidence-vs-override split doesn't carve at the joints — e.g., a single pragma turns out to need both roles, or the negative-validation rule conflicts with override semantics — **do not merge ADR-0021**. Close it as "draft, pending further implementation contact." Document the mismatch in the closeout.

## Tasks

### Phase 0 — Lock the decision frame (evidence matrix before ADR text)

This phase is strict. No ADR text is written until the anchors exist.

- [ ] Build a `periodic-invocation` evidence matrix at `docs/research/periodic-invocation-evidence-matrix.md` (scratch, not committed to `docs/decisions/`). Include: the 6 corpus targets' periodic regions (citations from annotations), what static evidence exists today (`lifecycle.long-running-loop`, `time.Ticker`/`time.Sleep` patterns), what pragma evidence is required (`idempotent=true`), what negative controls prove the rule doesn't over-fire.
- [ ] Pick three implementation anchors from the corpus:
  - **Positive anchor** (should become AUTO): miniflux `feedScheduler` with `monolift:idempotent=true` applied.
  - **Missing-evidence anchor** (should stay SUGGEST/TERMINAL): same region without the pragma.
  - **Contradictory-evidence anchor** (pragma must *not* act as override): a synthetic fixture where `monolift:idempotent=true` is applied to a body that writes to a global — must fail classification or hard-refuse, not promote to AUTO.
- [ ] Write the sprint's three halt conditions into this file's *Halt rule* / *Blocker rule* / *ADR halt rule* sections **before any Phase 1 work begins**. (Already written above — re-read and confirm applicability against the evidence matrix.)
- [ ] Declare deliberate deferrals explicitly — ADR-0019 (remediation surface), ADR-0022 (composites), ADR-0023, other archetype threshold sections, other classifier signals, extra pragmas. These will be captured inside the ADR-0020 and ADR-0021 text as "pending" rather than silently omitted.

### Phase 1 — ADR drafting (grounded in the evidence matrix)

- [ ] Draft `docs/decisions/0021-pragmas-as-evidence-vs-overrides.md`. Separate the two roles: (a) **evidence pragmas** supply facts the classifier combines with static evidence to reach a decision; (b) **override pragmas** waive a specific refusal code with an explicit waiver. Spell out: precedence (when both roles could apply), negative-validation rule (pragma does not save a region whose source contradicts the pragma's claim), provenance expectation (every pragma use is traceable in the report), and what counts as "contradicting the pragma's claim" for `idempotent=true` specifically.
- [ ] Draft `docs/decisions/0020-auto-lift-evidence-thresholds.md`. Normative only for `periodic-invocation`: AUTO iff (`time-loop-detector` Hold) ∧ (`idempotent=true` pragma present) ∧ (`effects.no-global-writes` on loop body) ∧ (interval config-driven); SUGGEST iff body mutates local state persisting across invocations and idempotency pragma is absent; TERMINAL iff interval is self-tuning. Other seven archetypes appear as stubs citing the catalog, each tagged "pending implementation contact — threshold not committed." Structural two-axis model (evidence-locality × externalization-affinity) included as explanatory framework.
- [ ] Append a one-paragraph amendment note to `docs/decisions/0016-state-class-inference.md` introducing `periodic-invocation` as a new state class recognized by the rule stack. Reference ADR-0020 for threshold semantics.
- [ ] Update `docs/evolution.md` with entries for ADR-0020 and ADR-0021 per repo convention.
- [ ] Cross-check both ADR drafts against ADR-0015/0016/0017/0018 and `docs/specs/monolift-v2-contract.md` for naming or semantic collisions. Record the check.

**Gate between Phase 1 and Phase 2: all three anchors (positive, missing-evidence, contradictory) are expressible in the ADR text. If they aren't, the ADR is wrong — revise before coding.**

### Phase 2 — Signal: `time-loop-detector`

- [ ] Read existing `pkg/compiler/stateclass/` signal implementations; identify the nearest analog (any SSA-on-loop or SSA-on-goroutine-spawn work already present).
- [ ] Specify `time-loop-detector` in `docs/specs/liftability-properties.md`: definition, SSA patterns it matches (`for { select { case <-t.C: ... } }`, `for { time.Sleep(d); ... }`, `for range time.Tick(d) { ... }`), outcome classes (gate/bias/advisory per ADR-0018), examples of Hold / Violate / Unknown.
- [ ] Implement the signal as a new file under `pkg/compiler/stateclass/` following existing per-signal conventions.
- [ ] Unit tests: synthetic SSA fixtures for (a) ticker-driven loop — Hold; (b) sleep-driven loop — Hold; (c) non-periodic `for range ch` — Violate; (d) timer-driven but body calls `doWork()` whose cadence self-tunes — Unknown. Tests live under the signal's package.
- [ ] Run the signal against the six corpus extract reports; record per-target outcomes and verify they match the catalog's periodic-invocation citations (listmonk L1/L3, caddy C1/C2, pocketbase P2, miniflux M1–M4, gitea G14, mattermost MM8).

### Phase 3 — Pragma surface: `monolift:idempotent=true`

- [ ] Read `pkg/pragma/` current implementation; identify how to add a new evidence-class pragma without disrupting existing override-class pragmas.
- [ ] Add parsing, validation, and representation for `monolift:idempotent=true`. No `=false` form in this sprint (keep the surface minimal; if someone wants to assert non-idempotency, the default absence already encodes that).
- [ ] Plumb the pragma into the region-level metadata the classifier consumes. Pragma-provenance must be preserved through to the classifier's decision (needed for ADR-0021's traceability claim and for the reportv2 provenance field).
- [ ] Unit tests: (a) pragma parses correctly; (b) pragma on a function makes that function's regions carry the evidence; (c) pragma on a method carries only to that method's regions (not siblings); (d) malformed pragma produces a diagnostic, not silent acceptance.

### Phase 4 — Rule: `periodic-invocation` state class

- [ ] Read `pkg/compiler/passes/register.go` and the existing rule stack; understand how a new state-class rule wires in.
- [ ] Implement the `periodic-invocation` rule: conjunction of (a) `time-loop-detector` Hold, (b) `monolift:idempotent=true` pragma present on the enclosing function, (c) `effects.no-global-writes` Hold on the loop body, (d) interval is visibly config-driven (not derived from in-process mutable state).
- [ ] Wire into `passes/register.go`. If a feature-flag convention exists, land behind the flag; otherwise land behind the Phase 5 regression harness as the safety net.
- [ ] Run the classifier against the six corpus extract reports with `monolift:idempotent=true` applied to the known positive regions. Verify the rule promotes the expected regions (listmonk L1/L3, miniflux M1–M4, etc.) and does not promote any ADMITTED region.
- [ ] **Negative-validation test (critical, per ADR-0021).** Construct a synthetic fixture where `monolift:idempotent=true` is applied to a function that writes to a global (`effects.no-global-writes` Violate). Verify the rule does *not* promote to AUTO — the pragma does not save a region whose source contradicts it. This is the ADR-0021 contract under test.
- [ ] **Contradictory-evidence test.** A fixture where the pragma is present but `time-loop-detector` is Violate (non-periodic loop). Verify the rule does not promote — requires *both* the static signal and the pragma, pragma alone is insufficient.
- [ ] Diff against committed golden reports at `test/e2e/targets/*/golden/report.json`; land expected golden changes from promoting the periodic regions; document the diffs.

### Phase 5 — False-refusal regression harness

- [ ] Add a per-target regression test that, for each of the six extract-report goldens, asserts the set of regions currently classified as ADMITTED (`replicated` or `immutable-captured-config`) remains *exactly pinned* after Phase 4's rule lands. Delta tolerated only in the known promotion set (the periodic-invocation regions listed above).
- [ ] Test location: `pkg/compiler/` unit test against fixture reports (runs on every CI pass, doesn't need e2e).
- [ ] Run it; fix any regression the rule introduced.
- [ ] Comment at the top of the test file notes that this is load-bearing for SPRINT-0013 open question C5 and should be extended with each future archetype landing.

### Phase 6 — Emission prototype (for falsification, not shipping)

- [ ] Create `pkg/compiler/emit/prototype/` (new package, compiler-internal). Not imported by `test/e2e/`. Not a runtime library.
- [ ] Write the ≤30-line Go emission for miniflux `feedScheduler`: reads the classifier's region output, emits (a) a new file expressing the loop-body extracted as a standalone handler function `Handler(ctx, ScheduledTrigger) error`, (b) a registration stub that references an abstract `platform.Schedule(name, cronExpr, handler)` interface — no commitment to k8s CronJob, serverless, etc. The prototype is about emission correctness, not runtime choice.
- [ ] Verify the emitted `.go` files pass `go vet` and compile (`go build -o /dev/null`) against a vendored stub `platform` interface. Do not run them.
- [ ] Write `docs/research/periodic-invocation-emission-findings.md` (≤1 page): what the catalog's ≤30-line sketch assumed; what the prototype had to add (error handling, context plumbing, loop-body captured-state handling, interval-derivation-from-config handling, Start/Stop lifecycle replacement); whether additions violate the ≤30-line claim; how much of what was added is generic (reusable across archetypes) vs. archetype-specific.

**Halt rule activation point.** If Phase 6 shows the sketch is materially incomplete (ballooned to >60 lines, required runtime-library package creation, couldn't handle loop-body captures without a runtime harness), stop merging Phase 4's rule into the main stack. Record gate failure. ADR-0020's periodic-invocation threshold stays unvalidated.

### Phase 7 — `reportv2` schema additions (minimal, for observability)

- [ ] Extend `pkg/compiler/reportv2/schema.json` with an optional `archetype` field (string) at the region level.
- [ ] Extend with an optional `pragma_provenance` field at the decision level (which pragmas contributed evidence to the classification).
- [ ] Update `pkg/compiler/reportv2/report.go` to populate both fields when the `periodic-invocation` rule fires.
- [ ] Update `pkg/compiler/reportv2/schema.json` consumers' tests; verify existing reports still validate.

### Phase 8 — Validation run + closeout

- [ ] Run the full classifier against all six corpus targets with `monolift:idempotent=true` applied to the known positive regions. Record which regions promoted to AUTO, which stayed SUGGEST, which remained TERMINAL.
- [ ] **Numeric falsification anchor.** Expect ≥4 miniflux periodic regions (feedScheduler, cleanupScheduler, watchdog, metrics) to convert. If fewer convert, the sprint's research output is the reason — don't hide it.
- [ ] Answer the three gate-validation questions at the end of this sprint file in writing, with citations:
  - **Evidence gate.** Did `time-loop-detector` cleanly distinguish periodic from non-periodic loops via SSA, or did it need pragma-sourced evidence beyond `idempotent`?
  - **Emission gate.** Did the ≤30-line sketch survive contact with miniflux feedScheduler, or did it balloon? (Link the findings companion.)
  - **Boundary gate.** Did the AUTO / SUGGEST / TERMINAL thresholds in ADR-0020 hold up under the six-target corpus run, or did any region violate the stated conditions?
- [ ] For each "no" answer above, open a followup entry in `docs/research/distribution-archetypes-followups.md` (Bucket D for spikes; Bucket B for additional ADR work; Bucket C for re-opened empirical questions).
- [ ] Update the ADR-0020 draft's threshold stubs for the other seven archetypes with risk notes learned this sprint ("state-class X likely has the same emission-gate concern," "archetype Y's pragma dependency is unclear without its own slice").
- [ ] Recommend the next sprint's slice based on what this one taught. Candidates: `bounded-worker-pool` (if pragma semantics are settled and need a pragma-free archetype for contrast); `session-affinity-state` (if the pragma surface wants another load-bearing-evidence case); back up a level to ADR-0019 if the remediation-surface gap became painful to work without.

## Sequencing

1. **Phase 0 → Phase 1 strict.** No ADR text is written until the evidence matrix and the three anchors exist. This is Codex's discipline; Claude's self-critique conceded it.
2. **Phase 1 → Phase 2 strict.** ADR text must be complete enough to cite in implementation code comments before signal work begins.
3. **Phase 2 → Phase 3 → Phase 4 strict.** Signal must Hold on real corpus regions before the pragma surface consumes it; pragma surface must be functional before the rule consumes both.
4. **Phase 4 → Phase 5.** Regression harness must land before the rule merges — silent ADMITTED-drop would be a regression the sprint could ship without this guard. (Codex and Gemini both missed this; Claude carried it in.)
5. **Phase 5 → Phase 6.** Emission prototype is the Halt-rule checkpoint. If it fails, Phase 4's rule does not merge; the sprint converts to "gate failed, research output recorded."
6. **Phase 6 → Phase 7.** Schema additions are cheap; land them after the rule is validated to avoid schema churn from halted work.
7. **Phase 8 is closeout.** Gate questions answered with evidence; followups opened; next-sprint recommendation recorded.

## Risks

| Risk | Mitigation |
|---|---|
| ADR-0021 drafted on paper without enough implementation contact; the evidence-vs-override split looks clean but falsifies under real pragma. | Phase 0 anchors (positive / missing-evidence / contradictory) must be writeable in ADR-0021 text before Phase 2 starts. ADR halt rule: if implementation contradicts the ADR's split, do not merge ADR-0021. |
| `time-loop-detector` needs pragma-sourced evidence because SSA cannot distinguish idiomatic periodic loops from general long-running loops. | Blocker rule: drop periodic-invocation AUTO claim, mark ADR-0020 threshold unvalidated, open followup. |
| Emission prototype balloons beyond ≤30 lines — sketch is fiction. | Halt rule: do not merge Phase 4 rule. Findings companion documents exactly what ballooned; this is the catalog's first gate-failure and is a legitimate sprint outcome. |
| Loop-body captured-state becomes a runtime-harness dependency. | Emission prototype is compiler-internal (`pkg/compiler/emit/prototype/`); no imports from `test/e2e/`; no `runtime/` package created this sprint. If captured state forces a runtime, that's halt-rule territory. |
| `monolift:idempotent` gets misused as an override ("trust me") — developers slap it on non-idempotent code. | ADR-0021's negative-validation rule is the contract; Phase 4's negative-validation test is the enforcement. If source contradicts the pragma's claim (e.g., writes to a global), the pragma does not save the region. |
| `reportv2` schema additions drift into ADR-0019 territory (rich SUGGEST payload). | Strict minimum: only `archetype` label and `pragma_provenance` field. Rich remediation output stays ADR-0019 work. If the sprint wants more, it's a followup. |
| ADR-0020 drifts into "write threshold stubs for all eight archetypes." | Fence: only `periodic-invocation` is normative; other stubs cite catalog, say "pending implementation contact," do not commit thresholds. |
| Regression harness introduces test-flake due to golden-file non-determinism. | Phase 5 specifies unit test against fixture reports, not e2e extraction — deterministic input. If flake appears, it's a real signal, not a test-harness issue. |
| Scope creep into "while I'm in stateclass, let me also add `keyed-access-invariant` / `bounded-pool-invariant`". | Non-goal fence: one signal this sprint. The other four evidence signals each wait for their archetype's future slice. |
| ADR-0021's evidence/override split generalizes poorly to other pragmas (ordering, partitioning), forcing ADR-0021 to re-open. | Accept this risk. ADR-0021 is drafted from one archetype's pragma pressure; if a future archetype's pragma needs different semantics, the ADR will need amendment. This is better than designing the ADR cold. |
| Archetype naming in `reportv2` drifts from ADR-0016 vocabulary. | Use the exact string `periodic-invocation` (hyphenated, lowercase). Document the string contract in the amended ADR-0016 note. |

## Acceptance criteria

- [ ] `docs/research/periodic-invocation-evidence-matrix.md` exists with the three anchors (positive / missing-evidence / contradictory) written before ADR text.
- [ ] `docs/decisions/0021-pragmas-as-evidence-vs-overrides.md` exists; separates evidence pragmas from override pragmas; specifies negative-validation rule; includes a worked example for `idempotent=true`.
- [ ] `docs/decisions/0020-auto-lift-evidence-thresholds.md` exists; `periodic-invocation` threshold is fully normative; other seven archetypes are present as stubs marked "pending implementation contact"; two-axis structural model documented.
- [ ] `docs/decisions/0016-state-class-inference.md` has an appended amendment note for `periodic-invocation` state class referencing ADR-0020.
- [ ] `docs/evolution.md` has entries for ADR-0020 and ADR-0021.
- [ ] `docs/specs/liftability-properties.md` documents `time-loop-detector` with definition, SSA patterns, outcome classes, Hold/Violate/Unknown examples.
- [ ] `time-loop-detector` signal implemented in `pkg/compiler/stateclass/` with unit tests covering Hold, Violate, Unknown per the spec.
- [ ] `pkg/pragma/` supports `monolift:idempotent=true`; pragma-provenance is preserved through to classifier decisions; malformed pragmas produce diagnostics.
- [ ] `periodic-invocation` rule lands in `pkg/compiler/stateclass/` and is wired into the rule stack; corpus run shows expected periodic regions promote to AUTO with the pragma applied.
- [ ] **Negative-validation test passes:** `monolift:idempotent=true` applied to a function with a global write does NOT promote to AUTO (pragma does not act as override).
- [ ] **Contradictory-evidence test passes:** pragma present without `time-loop-detector` Hold does NOT promote (requires both).
- [ ] False-refusal regression harness in tree, passing, pinning the current ADMITTED set across six target goldens.
- [ ] Emission prototype under `pkg/compiler/emit/prototype/` compiles (`go build`) against a stubbed `platform.Schedule` interface for miniflux feedScheduler.
- [ ] `docs/research/periodic-invocation-emission-findings.md` answers what the ≤30-line sketch missed; no imports from `test/e2e/`; no runtime-library package created.
- [ ] `pkg/compiler/reportv2/schema.json` adds `archetype` (region) and `pragma_provenance` (decision) fields; existing reports still validate.
- [ ] Corpus validation run shows ≥4 miniflux periodic regions convert from refusal to AUTO — or the closeout explains precisely which gate blocked fewer conversions.
- [ ] Closeout answers the three gate-validation questions with evidence citations.
- [ ] For each gate that failed, a Bucket-D or Bucket-B followup is added to `docs/research/distribution-archetypes-followups.md`.
- [ ] ADR-0020's threshold stubs for the other seven archetypes carry updated risk notes based on what this sprint taught.
- [ ] Next-sprint recommendation recorded at the bottom of this file.

## Open questions this sprint may sharpen (not resolve)

Carried from SPRINT-0013 followups; this sprint will report its current best characterization as part of the closeout.

- **C1 (boundary-is-threshold-per-archetype-or-structural).** `periodic-invocation`'s threshold surviving (or not) implementation is direct evidence for the structural two-axis model.
- **C2 (user-facing API changes).** `Start`/`Stop` → scheduler-registration is a minimal API change; the prototype's findings companion documents whether this is genuinely zero-cost or whether callers break.
- **C3 (pragma roles).** The whole sprint is testing this. The closeout restates ADR-0021's current position.
- **C5 (false-refusal regression risk).** Phase 5 regression harness is the concrete answer for `time-loop-detector`. Methodology generalizes.

## Closeout (to be written at sprint end)

- [ ] Gate 1 — evidence: did `time-loop-detector` cleanly distinguish periodic from non-periodic via SSA?
- [ ] Gate 2 — emission: did the ≤30-line sketch survive contact with miniflux feedScheduler?
- [ ] Gate 3 — boundary: did AUTO/SUGGEST/TERMINAL thresholds hold across the six-target corpus?
- [ ] Numeric anchor: how many miniflux regions converted vs. the 4-region expectation
- [ ] Negative-validation enforcement: contradictory pragma + global write refused as expected? (ADR-0021 contract verified)
- [ ] Golden diffs landed (list)
- [ ] Followups opened (list)
- [ ] SPRINT-0015 recommendation
