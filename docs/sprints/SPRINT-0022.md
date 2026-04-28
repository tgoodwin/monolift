# SPRINT-0022 — Multi-root regions: pragma redesign + closure-union pipeline, validated on Mattermost Hub/WebConn

**Status:** planned
**Anchor ADRs:** ADR-0004 (annotation surface), ADR-0012 (pragma parser/diagnostics), ADR-0018 (property taxonomy, frozen unless additive amendment), ADR-0022 (composite-archetype regions), ADR-0023 (cmd-inside-host emission).
**Predecessors:** SPRINT-0017 (candidate-set + `alternative_set` machinery on Caddy), SPRINT-0019 (cmd-inside-host emission), SPRINT-0020 (real-compiler/e2e-compile path on miniflux), SPRINT-0021 (stopped on branch (C) at the region-granularity cliff documented in `docs/research/runs/SPRINT-0021-region-granularity.md`).

## Intent

SPRINT-0021 stopped because the compiler's region model assumes one root per region. The Mattermost Hub/WebConn composite is genuinely multi-root: `Hub` owns fanout + per-shard connection indexes; `WebConn` owns per-connection write-pump + sequence + dead-queue replay; they are coupled through `wc.send` (a private `chan model.WebSocketMessage` field) where Hub goroutines write and `WebConn.writePump` reads. Neither root alone covers the boundary, and a synthetic wrapper would alter the target.

This sprint lands two coupled design pieces and validates them on that exact region:

1. **Multi-root pipeline.** Per-root closure analysis run N times → closure union with per-symbol provenance tags → inter-root seam detection (channel-coupled seams via private fields are the load-bearing case) → stateclass classification over the union (ADR-0022's composite machinery falls out naturally) → per-root + region-level admission with a new seam-shape check → single-service emission via the SPRINT-0019 cmd-inside-host path (the union closure becomes one extracted-service binary; channel seams stay in-process Go channels because the region was lifted as a unit; dialer code is emitted on every external entry point on every root).

2. **Pragma redesign — shared-name peer roots.** Multiple `//monolift:lift` doc-comment pragmas on different declarations that share the same `name=` value coalesce into one `Region{Roots: []*Pragma}`. Cross-pragma consistency is validated on `mode`, `transport`, `policy`, `dispatch`, `affinity` with a new `MLV2_PRAGMA_REGION_CONFLICT` diagnostic. Discovery (auto-detecting peers by seam analysis) is explicitly deferred — declaration is the path.

The validating target is the Mattermost Hub/WebConn region at `evaluation/mattermost/server/channels/app/platform/web_hub.go` + `web_conn.go`. Expected composite `{keyed-partitioned-state, fanout-publisher, session-affinity-state}` with alias `connection-hub-buffer`. Acceptance is the four-layer e2e (counter delta + `/invocations` oracle equality + transcript parity + fail-closed/open) on the SPRINT-0021 N=8 conn / M=4 user / shard-split + dead-queue workload.

## Pragma redesign — shared-name peer pragmas

All three committee drafts converged on this choice. Shape:

```go
//monolift:lift name=connection-hub-buffer mode=remote transport=http-json
func (h *Hub) Broadcast(msg *model.WebSocketEvent) { ... }

//monolift:lift name=connection-hub-buffer mode=remote transport=http-json
func (wc *WebConn) Pump() { ... }
```

Two pragmas, two declarations, one shared `name`. The regrouping pass turns parsed `[]*Pragma` into `[]*Region{Roots: []*Pragma}` keyed by `name`. New `MLV2_PRAGMA_REGION_CONFLICT` diagnostic on disagreement of `mode` / `transport` / `policy` / `dispatch` / `affinity` (post-default values).

**Why this over the alternatives.** Region-level external pragma (file/package-level pragma referencing multiple decls) breaks doc-comment-on-decl locality and adds a new attachment site the parser doesn't scan. Explicit `roots=Type1.Method1|Type2.Method2` inside one pragma is asymmetric (whichever decl you attach to becomes "primary"), creates a mini-language inside an option value, and goes stale on rename. Method-extension keys on a single primary root (e.g., `include=WebConn.writePump` on the Hub pragma) models WebConn as an appendix to Hub, which is exactly the framing SPRINT-0021's cliff doc rejected — Hub and WebConn are *peers*, neither is logically primary.

The cost: a region with N roots requires N near-identical pragmas. Acceptable. If duplication becomes painful after more targets, a future sprint can add region-level pragma syntax as sugar that desugars to N peer pragmas.

## Multi-root pipeline shape

1. **Per-root closure.** Run the existing closure analysis once per root. No algorithmic change.
2. **Closure union with provenance.** `union = ⋃ closure_i` plus a `provenance: Symbol → bitset(RootID)` map. A symbol reachable only from Hub gets `{Hub}`; a helper reachable from both gets `{Hub, WebConn}`. Strict superset — no symbol added that no root reaches. Both *included* and *excluded* symbol sets union; provenance is sorted deterministically by root name; root IDs are stable, derived from declaration identity (qualified type+method name), not source order.
3. **Inter-root seam detection.** A *seam* is a (writer-roots, reader-roots, medium) triple where one root writes and another reads through a shared medium. The load-bearing medium for Mattermost is a private channel field on a struct in the union (`WebConn.send`). Detector reads SSA directly (`*ssa.Send` / `*ssa.UnOp{Op: token.ARROW}` on struct field operands) — *not* the closure-symbol report — because SPRINT-0021 A.2 documented that field-level members aren't modeled as closure symbols. Other seam media (shared `sync.Mutex`, atomic field) are detected the same way and recorded but not required to admit in this sprint — channel-typed seams are the only kind we commit to a verdict for, because that's what Mattermost exercises.
4. **Stateclass over the union.** Existing classifier; ADR-0022 composite machinery handles the candidate set. Provenance is metadata on the report, not classifier input.
5. **Admission + emission.** Per-root admission AND'd at region level, plus a seam-shape check (see §Admission). Single-service emission via cmd-inside-host: union closure becomes one extracted-service binary; intra-union channel seams stay Go channels in-process; dialer code on every external entry point on every root.

## Admission posture

Admission rules in `pkg/compiler/transport/admission.go` are research artifacts, not contracts. The agent may refine an admission rule in-sprint **iff** the discipline checklist (a)-(e) is satisfied:

(a) hypothesis is documented before the change in `docs/research/runs/SPRINT-0022-admission-characterization.md`;
(b) hypothesis grounds in prior ADRs (ADR-0017 / 0018 / 0022 / 0023);
(c) change is principled — applies regardless of Mattermost being on the desk (test: would I make this same change if the region were a hand-rolled Go test fixture?);
(d) all prior e2e (Caddy, miniflux, pocketbase, pragma) byte-identical after the change;
(e) if the hypothesis is weak, default to (R) characterization branch.

**The expected admission concern is the inter-root channel seam.** Hypothesis to write before changing anything: *a `chan T` field whose writer-roots and reader-roots are all members of the union closure of the region admits trivially, because lifting the region as one extracted-service binary keeps the channel in-process; the channel is preserved verbatim, no wire crosses, no serialization or backpressure-shape change is introduced.* If that holds up, the seam-shape check is an additive admission rule, not a relaxation. If a stronger refinement is needed (e.g., the seam crosses out of the union), default to (R).

**A new Layer-1 `liftability.PropertyID` constant remains a high bar.** If the seam-shape check needs a new property, write the ADR-0018 amendment first, get property-lint to accept it explicitly, then add the constant. Default expectation: no new Layer-1 properties this sprint; provenance and seam metadata are stateclass-internal structural facts.

## Frozen boundaries

- `cmd/main.go` byte-identical (no exceptions).
- `evaluation/mattermost/` byte-identical pre/post compile. Pragmas live in a sidecar overlay under `test/e2e/targets/mattermost/`.
- `syscall.Flock` startup guard at `test/e2e/e2ecompile/main.go` preserved.
- SPRINT-0017 Caddy `alternative_set` goldens byte-identical after any rule changes.
- SPRINT-0019/0020 e2e (caddy, miniflux, pocketbase, pragma) byte-identical after any rule changes.
- `make verify-evaluation-untouched` extended to cover Mattermost; runs in Block B and Block G.

May move under documented hypothesis: pragma parser, closure analysis, stateclass internals, admission rules, the liftpatch API, and ADR-0018 (additive-only).

## Anticipated cliffs

| # | Cliff | Symptom | Stop criterion |
|---|---|---|---|
| 1 | **Pragma parser cannot represent shared-name peer roots without rewrite.** Current `FromDecl` rejects multiple pragmas on one decl and assumes 1 pragma → 1 region; downstream consumers may silently rely on this. | A.gate-1 fixture or downstream consumer breaks on `[]*Region` shape requiring API rewrites spanning >2 packages. | Sprint lands (C) with `docs/research/runs/SPRINT-0022-pragma-cliff.md` enumerating contract collisions and the API rewrites needed. |
| 2 | **Closure-union OOMs or blows wall budget.** SPRINT-0021 A.1 measured 88s wall / 2.15GB max RSS for the Hub-rooted closure; the union with WebConn must stay in similar order. | Closure-only union run > **30 min wall / 16 GiB RSS**. | Sprint lands (C). Reuses SPRINT-0021 A.1 OOM cliff doc shape. |
| 3 | **Closure-union surfaces graph algorithms not present in the pipeline** (cycle detection across roots; iterative fixed-point for provenance). | Implementing C.x reveals the union step needs more than two passes over the per-root closures. | Not necessarily a stop. If bounded (e.g., fixed second pass to propagate provenance), implement it. If unbounded (full SCC analysis), stop at (C). |
| 4 | **Seam detector misses `wc.send` because field-level symbols are not first-class.** SPRINT-0021 A.2 documented this. | D.x seam-detector run on the Mattermost union returns zero `chan T` seams. | Not a stop. Build the detector against SSA directly (`*ssa.Send` / `*ssa.UnOp{Op: token.ARROW}`); do not require fields to be closure-report-listed. |
| 5 | **Admission has no representation for the inter-root channel seam.** Existing predicates key off per-root surface properties; "seam shape" is a region-level concept. | F.x admission probe surfaces per-root verdicts but no place to record/check the seam result. | Expected. The seam-shape check is the additive refinement (§Admission). Land it under the discipline checklist; if hypothesis is weak, sprint lands (R). |
| 6 | **Defaulting-reorder cliff.** Parser fills `mode`/`transport`/`policy` defaults *after* per-pragma parse; consistency check runs *before* defaulting → semantically-equivalent pragmas flagged as conflicting. | A.x consistency tests fail on inputs that should be equivalent. | Not a stop. Move defaulting *before* `RegroupPragmas`. Diagnostic compares post-default values. |
| 7 | **Fanout/session-affinity stateclass evidence still fails on the union.** SPRINT-0021 deferred archetype-registry work when the region cliff hit; those checks must pass against the union closure here. | E.x archetype run does not produce both new archetypes independently. | Stop at (C). Document the evidence-model gap. Do not invent ADR-0018 properties. |
| 8 | **Liftpatch API has no shape for "single service from multiple roots."** Each root needs a host-side dialer stub; the patcher today assumes one host stub per `PatchRequest` keyed on one root. | G.x emission can't describe multi-root host patching as a single coherent payload. | Stop emission. `docs/research/runs/SPRINT-0022-emission-gap.md` enumerates missing primitives by `PatchRequest`/`PatchResult`/`PatchSymbolBody` field. Sprint lands (R). |
| 9 | **Mattermost baseline boot still flaky.** Carried from SPRINT-0021 Cliff 8. | G.x baseline can't reach `/api/v4/users/me` 200. | Pin minimal config from SPRINT-0021 C.2; if still fails, stop at (C). |
| 10 | **`maphash` non-determinism in shard placement.** Carried from SPRINT-0021 Cliff 10. | Workload picks userIDs that don't reliably split across shards. | Pre-compute shard assignments at workload-init time with fixed seed. |

## Sequencing

`A → B → C → D → E → F → G → H`, strict. Within blocks, the ordering below is prescribed.

The cheapest kills, in order: **A.gate-1** (parser regrouping cliff — wire `RegroupPragmas` into the downstream consumer; if the contract collision spans too much API surface, stop before any closure or seam work). Then **C.gate-1** (closure-union OOM probe — same shape as SPRINT-0021 A.1). Then **F.gate-1** (admission verdict — branches the sprint between (S) attempt and (R) characterization).

### Block A — Pragma parser: shared-name regrouping + conflict diagnostic

Goal: shared-`name` peer-pragma model lands in the parser, regrouping produces `[]*Region`, conflict diagnostic fires correctly, all existing pragma fixtures pass byte-identical. **First gate of the sprint.**

- [x] **A.1** Add `Region` and `RegionRoot` types in `pkg/compiler/pragma.go` (or a new `pkg/compiler/region.go`). `Region{Name, Roots []*Pragma, Span, Mode, Transport, Policy, Dispatch, Affinity}`. Stable root IDs derived from declaration identity (qualified type+method name), not source order.
- [x] **A.2** Add `MLV2_PRAGMA_REGION_CONFLICT` to the diagnostic code constants and `knownPragmaDiagnosticCodes` slice.
- [x] **A.3** Move pragma defaulting (mode/transport/policy/dispatch/affinity fill-in) to *before* `RegroupPragmas` runs. Cliff 6 mitigation. If defaults currently happen later in the pipeline, lift them up.
- [x] **A.4** Implement `RegroupPragmas([]*Pragma) ([]*Region, []Diagnostic)`. Bucket by non-empty `Name`; emit `MLV2_PRAGMA_REGION_CONFLICT` for regions whose pragmas disagree on `mode`/`transport`/`policy`/`dispatch`/`affinity` (post-default values); preserve deterministic ordering by `(first Span.Filename, first Span.Line)`; empty-`Name` falls back to one Region per Pragma (legacy single-root case).
- [x] **A.gate-1** **Pragma-cliff probe.** Wire `RegroupPragmas` into the call site that today consumes `[]*Pragma`. If that consumer's contract assumes one Region per Pragma in a way that does not survive the `[]*Region` change without API rewrites spanning >2 packages, stop at Cliff 1: write `docs/research/runs/SPRINT-0022-pragma-cliff.md` and land (C).
- [x] **A.5** Diagnostic-fixture tests: (a) two pragmas same `name`, identical other keys → one Region, two Roots, no diagnostic. (b) two pragmas same `name`, conflicting `mode` → `MLV2_PRAGMA_REGION_CONFLICT`. (c) two pragmas different `name` → two Regions. (d) pragma with empty `name` → one Region per Pragma (legacy path). (e) three pragmas same `name`, all consistent → one Region, three Roots, sorted deterministically. (f) duplicate pragma on same decl still fires `MLV2_PRAGMA_DUPLICATE`. (g) two pragmas same `name`, equivalent post-default values → no diagnostic (Cliff 6 regression test).
- [x] **A.6** SPRINT-0019/0020 pragma e2e fixtures (caddy, miniflux, pocketbase, pragma target) byte-identical. Empty-name / unique-name fallback must produce the exact same downstream artifacts.
- [x] **A.7** Update `docs/decisions/0012-pragma-parser-diagnostics.md` with an additive amendment describing shared-name regrouping and the new diagnostic. Per project convention, don't rewrite the original ADR text. Decide whether the multi-root pragma decision warrants a new ADR-0024 (vs. amendment to ADR-0012); default to additive amendment + brief new ADR pointing back if the change is load-bearing.

**Block A gate:** `RegroupPragmas` correct on all fixtures; `MLV2_PRAGMA_REGION_CONFLICT` fires on conflicts and only on conflicts; SPRINT-0019/0020 pragma artifacts byte-identical. If A.gate-1 fired Cliff 1, sprint lands (C).

### Block B — Mattermost pragma overlay + baseline recon

Goal: stage the Mattermost Hub/WebConn pragmas without modifying `evaluation/mattermost/`; refresh e2e target metadata; carry SPRINT-0021's A.1/A.2 findings forward.

- [x] **B.1** Decide the pragma-overlay mechanism. `evaluation/mattermost/` is frozen. Choose between (i) sidecar overlay file the e2e-compile driver merges into the parsed AST view, or (ii) `test/e2e/targets/mattermost/pragma_overlay.go` declaring pragmas against re-exported aliases of the real symbols. Default: (ii). Document the resolver behavior the e2e-compile loader must honor (alias → original symbol identity). If the alias resolver turns out to be non-trivial, fall back to (i).
- [x] **B.2** Write the chosen overlay file. Both Hub and WebConn declarations decorated with `//monolift:lift name=connection-hub-buffer mode=remote transport=http-json` (and consistent values for any other region-wide keys). Build-tag-gated to compile only under e2e.
- [x] **B.3** Refresh `test/e2e/targets/mattermost/target.go`: replace SPRINT-0021 placeholders, set expected roots covering Hub's full external surface (`Broadcast`, `Register`, `Unregister`, `Start`, `CheckConn`, ...) and WebConn's (`Pump`, `writePump`). Leave skipped in CI until G gates pass.
- [x] **B.4** Re-run the SPRINT-0021 A.1 closure-only probe under the **single-Hub-root** configuration. Confirm baseline did not regress vs. SPRINT-0021's reference numbers (88.22s wall / 2.15GB max RSS / 2956 included / 4838 excluded). Capture into `docs/research/runs/SPRINT-0022-baseline.md`. Regression check, not a new measurement.
- [x] **B.5** Run `make verify-evaluation-untouched` after B.1–B.2.

**Block B gate:** Mattermost root declaration exists without modifying `evaluation/mattermost/`; baseline numbers in same order as SPRINT-0021; evaluation source byte-identical.

### Block C — Multi-root closure analysis: per-root, union, provenance

Goal: closure analyzer accepts multiple roots, produces a union with provenance; OOM probe completes under budget; closure-pin assertion shows the SPRINT-0021 cliff cleared.

- [x] **C.1** Refactor closure analyzer entry point to accept `[]Root` and return `Closure` + `Provenance: map[Symbol]bitset(RootID)`. Internally: run the existing per-root algorithm N times, union included and excluded symbol sets, OR provenance bitsets per symbol. No change to the per-root algorithm itself.
- [x] **C.2** Provenance round-trip in `pkg/compiler/reportv2/`: each `closure.includedSymbols[i]` gets a `provenance: ["Hub", "WebConn"]` (sorted) field. Update schema + add fixture round-trip test. Provenance ordering deterministic (sorted by root name).
- [x] **C.3** Closure-union OOM probe. Run `bin/e2e-compile` against the Mattermost overlay region (Hub + WebConn roots) in closure-only union mode. SPRINT-0021 A.1 workspace + flock guard. Capture wall, max RSS, peak memory, profiles into `docs/research/runs/SPRINT-0022-union-probe.md`.
- [x] **C.gate-1** **Stop budget: 30 min wall / 16 GiB RSS.** If exceeded (Cliff 2), stop. Sprint lands (C).
- [x] **C.4** Closure-pin assertions on the union report: includes the SPRINT-0021 A.2 symbol set (`Hub`, `(*Hub).Start`, `(*Hub).Broadcast`, `(*Hub).Register`, `(*Hub).Unregister`, `hubConnectionIndex` + methods, `WebConn`, `(*PlatformService).GetHubForUserId`) **plus** `(*WebConn).writePump` (the SPRINT-0021 missing symbol — its presence is the central "we cleared the cliff" signal). Provenance is reachability-based per the §"Multi-root pipeline shape" definition: assert `(*WebConn).writePump` provenance includes `{WebConn}` (it must be reachable from the WebConn root); assert provenance is non-empty on every union symbol; assert provenance bitsets are sorted deterministically. **Do not assert exclusive single-root provenance on cross-referenced types** — Go's mutual type references between Hub and WebConn mean both roots' closure walks transitively reach each other's methods, and reachability provenance correctly records that. Symbol-level provenance is metadata; goroutine-level writer/reader attribution lives in Block D's seam detector, not here.

**Block C gate:** union closure completes under budget; provenance round-trips through the report; `(*WebConn).writePump` present in `closure.includedSymbols`. If C.gate-1 fired, sprint lands (C).

### Block D — Inter-root seam detection

Goal: SSA-level seam detector finds `wc.send` on the Mattermost union; toy fixtures cover the shape; recorded-only entries for non-channel seams.

- [x] **D.1** Define `Seam` model: `{Type, Field, ElemType, Writers []RootID, Readers []RootID, Span, Evidence string}` in `pkg/compiler/stateclass/seams.go`. Deterministic ordering for report output.
- [x] **D.2** Implement SSA-based channel-field seam detection. For every struct type `T` in the union with `chan U` field `f`, walk SSA `*ssa.Send` (writes `*T.f <- ...`) and `*ssa.UnOp{Op: token.ARROW}` (reads `<-*T.f`); tag each instruction with the root(s) whose closure reaches the enclosing function; emit a `Seam{Type: ChannelField, ...}` whenever writer-roots ≠ reader-roots. Cliff 4 mitigation: read SSA directly, not the closure-symbol report.
- [x] **D.3** Toy-fixture unit tests under `pkg/compiler/stateclass/testdata/seams/`: (a) one struct, one `chan int` field, root-A writes / root-B reads → one seam emitted, writers `{A}` readers `{B}`. (b) both roots write *and* read same channel → no inter-root seam (intra-root). (c) `*sync.Mutex` field touched by two roots → `Seam{Type: MutexField}` emitted, recorded only, not required to admit. (d) atomic field touched by two roots → `Seam{Type: AtomicField}`, recorded only.
- [x] **D.4** Mattermost regression: assert the union surfaces exactly one channel seam on `WebConn.send` with writers `{Hub}` and readers `{WebConn}`.
- [x] **D.5** Seam data round-trips through the report deterministically.

**Block D gate:** `WebConn.send` detected without requiring field-level closure symbols; seam evidence round-trips in reports; toy fixtures exercise positive and negative cases.

### Block E — Stateclass + composite recognition

Goal: ADR-0022 composite-archetype machinery wires through the union; `connection-hub-buffer` is primary; SPRINT-0017 Caddy `alternative_set` goldens stay byte-identical; no new Layer-1 properties.

- [x] **E.1** Register `ArchetypeFanoutPublisher` in `pkg/compiler/stateclass/archetype.go` using existing ADR-0018 properties only. Document each chosen property and why it's evidence of fanout.
- [x] **E.2** Register `ArchetypeSessionAffinityState` using existing ADR-0018 properties only.
- [x] **E.3** Update `harvestSeeds` in `stateclass.go` to detect `fanout-publisher` (Hub broadcast over connection-channel pattern) and `session-affinity-state` (WebConn `connectionID`, `Sequence`, `deadQueue`, write-pump ownership) evidence on the union.
- [x] **E.4** Update `topologyTierPriority` in `tiers.go` with values for the two new archetypes.
- [x] **E.5** Mattermost-specific fixtures under `pkg/compiler/stateclass/testdata/fixtures/`: three fixtures, one per component archetype, each verifies AUTO-match independently.
- [x] **E.6** Property-lint: assert no new `liftability.PropertyID` constants. Mechanical check; fail the sprint test suite if violated.
- [x] **E.7** Create `pkg/compiler/stateclass/composites.go`: `Composite{Components []ArchetypeID, Alias string, CoherenceCheck func(...) bool}`. Component order canonical (sorted by `ArchetypeID`).
- [x] **E.8** Register `{fanout-publisher, keyed-partitioned-state, session-affinity-state}` (sorted) with alias `"connection-hub-buffer"`.
- [x] **E.9** Create `pkg/compiler/stateclass/coherence.go` codifying ADR-0022's "compatible refinement" predicate: components must refine **disjoint axes** (ownership / routing / delivery) **and** agree on the keying dimension (same key drives partition placement, sticky routing, and fanout-recipient selection). Codify keying-agreement: same `PropertyID` carries the partition key across all three components' evidence.
- [x] **E.10** Coherence unit tests: 3 positive cases + 4 negative cases (two-of-three; mismatched key dimension; same axis claimed twice; SPRINT-0017 Caddy `serialized-actor + keyed-partitioned-state` does not produce `connection-hub-buffer`).
- [x] **E.11** Implement `ExtendWithComposites` at the SPRINT-0017 seam in `pkg/compiler/stateclass/candidates.go`. Iterate registered composites; check all components present + coherence holds.
- [x] **E.12** Composite subsumption: composite preferred over its components when present; components remain in `Alternatives` with non-empty rationales.
- [x] **E.13** AND-rule eligibility in `selection.go`: composite is `dynamic_delegate_eligible` iff every component is. Same for `runtime_selectable` and `emittable`. Unit test: composite eligibility is `false` when at least one component is ineligible. Expected outcome on Hub/WebConn: `dynamic_delegate_eligible: false` (sticky session-affinity breaks any-replica dispatch). Assert explicitly.
- [x] **E.14** Wire `archetypeKindForOutcome` in `selection.go` to return `"composite"` when primary is a composite. Populate `Primary.ContributingArchetypes` (sorted) and `Primary.Alias = "connection-hub-buffer"`.
- [x] **E.15** Schema validation: confirm `pkg/compiler/reportv2/schema.json` enum includes `"composite"`; add hand-rolled composite report fixture round-trip test.
- [x] **E.16** SPRINT-0017 Caddy `alternative_set` goldens byte-identical against pre-sprint.

**Block E gate:** all unit tests pass; SPRINT-0017 Caddy goldens byte-identical; coherence predicate codified (not prose); no new Layer-1 properties.

### Block F — Admission: per-root + region-level + seam-shape check

Goal: region-level admission verdict produced; if seam-shape check landed, all SPRINT-0019/0020 e2e byte-identical; if refused, characterization doc has one entry per refused property/seam with classification + follow-up sketch.

- [x] **F.1** Per-root admission probe: run existing `pkg/compiler/transport/admission.go` against `Hub` root surface and `WebConn` root surface independently. Capture per-property verdicts (Hold / Violate / NoEvidence) for both roots into `docs/research/runs/SPRINT-0022-admission-characterization.md`.
- [x] **F.2** Region-level admission predicate: `RegionAdmits(region, perRootVerdicts, seams)` returns a region verdict. Initial rule: AND of per-root verdicts, plus for each seam in `seams` the seam-shape check fires. Document the rule in the characterization doc.
- [x] **F.3** **Seam-shape check hypothesis** (must be written into the characterization doc *before* coding the check): *a `chan T` seam whose writer-roots and reader-roots are all members of the union closure of the region admits trivially, because lifting the region as one extracted-service binary keeps the channel in-process; the channel is preserved verbatim, no wire crosses, no serialization or backpressure-shape change is introduced.*
- [x] **F.4** Implement the channel-seam admission check per the F.3 hypothesis. For `chan T` seams whose writers or readers escape the union, refuse with rationale; on Mattermost the writers are Hub-side and readers are WebConn-side, both inside the union.
- [x] **F.5** **Discipline checklist** before merging the seam-shape check (per-task subtasks):
    - [x] (a) hypothesis doc committed *before* the rule change.
    - [x] (b) change cited against ADR-0022 (composite emission as a unit) and ADR-0023 (cmd-inside-host emission).
    - [x] (c) change is principled — applies to any region whose channel-seam endpoints are both lifted as one service. Test: would I make this change if the region were a hand-rolled Go test fixture?
    - [x] (d) re-run SPRINT-0019/0020 e2e (caddy, miniflux, pocketbase, pragma) — all byte-identical.
    - [x] (e) if the hypothesis is weak or any of (a)-(d) fail, defer to (R).
- [x] **F.6** Run region admission on the Mattermost union with the seam-shape check active. Capture verdict per property, per seam.
- [x] **F.gate-1** **Sprint branch decision.** If region admission **accepts**: proceed to Block G (S branch attempt). If it **refuses** any property or any seam (Cliff 5 — possible outcome): mark composite `emittable: false`, fill out per-property and per-seam refusal entries in the characterization doc following the SPRINT-0021 fundamental-vs-tooling-immaturity classification (each entry: verdict, triggering code shape, distribution-feasibility analysis, classification, follow-up sketch), write the synthesis paragraph naming the dominant gap shape, skip Block G emission tasks. Sprint lands (R).
- [x] **F.7** **In-sprint rule refinement, optional, gated** (SPRINT-0021 C.9.action analog): if F.6 surfaces a refusal but the agent has a clear, ADR-grounded hypothesis for an additional principled refinement, follow F.5 discipline to land it; otherwise default (R).

**Block F gate:** region-level admission verdict produced; if seam-shape check landed, all SPRINT-0019/0020 e2e byte-identical; if refused, characterization doc complete.

### Block G — Multi-root emission + four-layer e2e (skipped on R/C)

Skipped if F.gate-1 fired (R) or any earlier block landed (C).

- [x] **G.1** Emission sketch as design note appended to this sprint file: union closure becomes one extracted-service binary `cmd/monolift-extracted-connection-hub-buffer/main.go`; channel seams stay Go channels (in-process); host-side dialer stubs emitted on every external entry point on every root (Hub.Broadcast, Hub.Register, Hub.Unregister, Hub.Start, WebConn.Pump, etc.); oracle binary `cmd/monolift-oracle-connection-hub-buffer/main.go` per ADR-0023.
- [x] **G.2** Liftpatch fit check: enumerate every `PatchRequest` / `PatchResult` / `PatchSymbolBody` field needed for "one host patch, multiple replaced symbols across multiple receiver types." Concrete artifact, not yes/no.
- [x] **G.gate-1** If sketch does **not** fit (Cliff 8): write `docs/research/runs/SPRINT-0022-emission-gap.md` enumerating missing primitives by field. Skip remaining G tasks. Sprint lands (R).
- [ ] **G.3** Add Mattermost emit contexts in `pkg/compiler/extract_transport.go` for the multi-root composite. No admission/patcher API changes (unless G.gate-1 hypothesis-driven amendment landed under F.5 discipline).
- [ ] **G.4** Extend `test/e2e/e2ecompile/main.go` lifted-tree materialization for Mattermost: emit `<output>/lifted/host-patch/server/` with patched Hub + WebConn regions, single extracted-service `cmd/`, oracle `cmd/`, Dockerfiles, manifests.
- [ ] **G.5** Static recursion-safety assertion: extracted-service and oracle Deployment YAMLs grep-clean for `MONOLIFT_LIFT_[A-Z_]+:` env keys.
- [ ] **G.6** Build host, extracted-service, and oracle from `lifted/host-patch/server/`; assert `go build` succeeds for all three.
- [ ] **G.7** Mattermost baseline manifests + minimal config (`FileSettings.DriverName=local`, `EmailSettings.SendEmailNotifications=false`, `EmailSettings.RequireEmailVerification=false`, no Elasticsearch, `PluginSettings.Enable=false`, `RateLimitSettings.Enable=false`). Carry SPRINT-0021 C.1–C.4 forward.
- [ ] **G.8** Phased readiness: Postgres → migrations → Mattermost → `/api/v4/system/ping` 200 → `/api/v4/users/me` 200 with bootstrap admin token. Hard cap 8 min. If fails (Cliff 9), stop at (C).
- [ ] **G.9** Implement workload `test/e2e/targets/mattermost/workload/` (Go, not bash). Pre-compute shard-split user IDs offline with fixed `maphash` seed (Cliff 10). Open N=8 WebSocket clients across M=4 users; users land on at least two different hub shards.
- [ ] **G.10** Workload action — decomposed:
    - [ ] Wait for Mattermost hello/connected event on each conn.
    - [ ] Post N×K messages via REST → triggers `Hub.Broadcast`.
    - [ ] Verify each message received by every subscribed conn **exactly once** (fanout-publisher assertion).
    - [ ] Drop a configurable subset of WS connections after a drain point.
    - [ ] Reconnect with the same `connectionID` + last seqNum.
    - [ ] Assert `deadQueue` replay returns the missed events (session-affinity-state assertion).
- [ ] **G.11** Drain-then-reconnect protocol on the reconnect/replay subset: workload waits for server ack of message N before triggering disconnect.
- [ ] **G.12** Pin baseline workload reference trace at `test/e2e/targets/mattermost/golden/workload-trace.json` (event counts per user / per shard / replay counts).
- [ ] **G.13 — Layer 1 (evidence).** Property-lint passes; no new ADR-0018 properties.
- [ ] **G.14 — Layer 2 (catalog).** Atomic candidate set on the union is exactly `{fanout-publisher, keyed-partitioned-state, session-affinity-state}` (sorted) before composite insertion.
- [ ] **G.15 — Layer 3 (composite + report).** Composite is primary, `archetype_kind: "composite"`, alias `"connection-hub-buffer"`, contributing archetypes sorted, alternatives present with non-empty rationales, `dynamic_delegate_eligible: false`. Provenance present on union symbols. Seam list present and includes `WebConn.send`.
- [ ] **G.16 — Layer 4, branch (S).** Deploy lifted Mattermost + extracted-service + oracle against Postgres baseline; run G.10 workload; per-request `/calls` delta `>= 1`; aggregate `<= 50`; oracle equality on every `/invocations` record; transcript parity vs G.12 reference trace; recursion-safety runtime test (direct POST to extracted `/invoke` with no lift env increments `/calls` exactly once); fail-closed test (extracted scaled to 0 → expected degraded signal); fail-open test (`MONOLIFT_LIFT_FAILMODE=open`, extracted scaled to 0, workload succeeds, `/calls` stays 0).
- [x] **G.17 — Layer 4, branch (R).** If `emittable: false`: assert no lifted artifacts emitted; report records `emittable: false`; G.12 baseline trace remains valid; F characterization doc exists with one entry per refused property/seam.
- [x] **G.18** `go test ./pkg/compiler/stateclass/... ./pkg/compiler/reportv2/... ./pkg/compiler/...`
- [x] **G.19** `go test ./test/e2e/e2ecompile/...` serially.
- [ ] **G.20** If lifted: `MONOLIFT_E2E=1 go test -tags=e2e ./test/e2e -run TestE2E/mattermost -count=1 -timeout 45m` serially.
- [x] **G.21** Full-matrix regression: `MONOLIFT_E2E=1 go test -tags=e2e ./test/e2e -timeout 60m -count=1`. Caddy + miniflux + pocketbase + pragma + mattermost (whatever branch). SPRINT-0017/0019/0020 byte-identical to pre-sprint.
- [x] **G.22** Run `make verify-evaluation-untouched` post-emission. `evaluation/mattermost/` byte-identical.

**Block G gate:** assertions for the active branch (S or R) all pass; full-matrix regression green; `evaluation/mattermost/` byte-identical.

### Block H — Documentation, ADR addenda, ledger

- [x] **H.1** Sprint closeout section in this file: which branch (S, R, C); what landed; admission verdict; closure-union compile metrics (wall, RSS) recorded regardless of branch.
- [x] **H.2** ADR-0012 additive amendment: shared-name regrouping, `MLV2_PRAGMA_REGION_CONFLICT`, `Region` type, deferred discovery rationale.
- [x] **H.3** ADR-0022 additive amendment: multi-root region model — closure union with provenance, inter-root seam taxonomy (channel-field load-bearing, mutex-field/atomic recorded only), region-level admission with seam-shape check, single-service emission rule.
- [ ] **H.4** ADR-0023 additive amendment **only on (S)**: multi-root cmd-inside-host emission shape.
- [x] **H.5** New ADR `docs/decisions/0024-multi-root-region-pragma.md` if the multi-root pragma decision warrants it (vs. additive amendment to ADR-0012). Default: additive amendment + brief new ADR pointing back if H.2 amendment outgrows additive scope.
- [x] **H.6** No amendment to ADR-0018 (no new Layer-1 properties).
- [x] **H.7** Update `docs/evolution.md` narrative entry.
- [x] **H.8** Create `docs/evaluation/targets/03-mattermost.md` with: pinned multi-root region, pragmas, candidate set, branch outcome, compile metrics, seam list, refusal characterization (if R), or working emission shape (if S).
- [x] **H.9** `docs/sprints/ledger.yaml`: status `done` (branch R landing).
- [x] **H.10** `test/e2e/targets/mattermost/target.go` to final form: non-skipped on (S); skipped with `SkipReason` pointing to characterization or cliff doc on (R)/(C).
- [x] **H.11** Verify `cmd/main.go`, ADR-0018, the patcher API (unless F.7 hypothesis-driven amendment landed), and `evaluation/mattermost/` all unchanged via `git diff` against sprint base.

## Acceptance criteria

The sprint accepts iff one of (S), (R), or (C) holds and all gates for that branch pass. **A sprint that lands none of these cleanly is a process failure.**

### (S) — Successful multi-root lifted demo
- All Block A–G gates pass. Branch (S) of G.16.
- Pragma parser accepts shared-`name` peer pragmas; `RegroupPragmas` emits one Region with two Roots for the Mattermost overlay; `MLV2_PRAGMA_REGION_CONFLICT` fires on conflict fixtures.
- Closure analyzer produces a union with provenance; `(*WebConn).writePump` is in `closure.includedSymbols` (the SPRINT-0021 cliff signal cleared).
- Seam detector finds `WebConn.send` as the inter-root channel seam with writers `{Hub}` / readers `{WebConn}`.
- Classifier emits `archetype_kind: "composite"`, alias `connection-hub-buffer`, contributing archetypes sorted, `dynamic_delegate_eligible: false`.
- Region admission (per-root AND + seam-shape check) accepts. Seam-shape check has documented hypothesis grounded in ADR-0022 + ADR-0023, change is principled, SPRINT-0017/0019/0020 e2e byte-identical post-change.
- `lifted/host-patch/server/` contains single extracted-service `cmd/`, oracle `cmd/`, host stubs on every external entry point on every root. `evaluation/mattermost/` byte-identical.
- Lifted Mattermost passes all four e2e layers on the N=8 / M=4 / shard-split + dead-queue workload.
- ADR-0012 + ADR-0022 + ADR-0023 additive amendments landed; `cmd/main.go` unchanged; ADR-0018 unchanged.

### (R) — Multi-root pipeline lands; Mattermost stops at a characterized gap
- Blocks A, B, C, D, E pass. Block F characterization doc exists with one entry per refused property and per refused seam (verdict, triggering code shape, distribution-feasibility analysis, fundamental-vs-tooling-immaturity classification, follow-up sketch, plus synthesis paragraph naming dominant gap). Block G either skipped (F.gate-1 refused) or aborted at G.gate-1 (sketch doesn't fit liftpatch API).
- Pragma/closure-union/seam machinery lands as in (S) and is exercised by tests; Mattermost classification produces the composite report with `emittable: false`.
- Baseline Mattermost workload trace pinned at `test/e2e/targets/mattermost/golden/workload-trace.json`.
- ADR-0012 + ADR-0022 additive amendments landed. No frozen contract modified beyond the documented-hypothesis discipline.
- Either `docs/research/runs/SPRINT-0022-admission-characterization.md` (admission refusal) or `docs/research/runs/SPRINT-0022-emission-gap.md` (sketch gap) is the load-bearing deliverable.

### (C) — Hard cliff before machinery completes
- Stop point at one of: A.gate-1 (pragma parser rewrite wider than additive), C.gate-1 (closure-union OOM), E (evidence-model gap for new archetypes), G.8 (Mattermost baseline boot — SPRINT-0021 Cliff 8 carryover).
- Cliff doc at `docs/research/runs/SPRINT-0022-<cliff>.md` records reproduction steps + resource numbers (RSS, wallclock) + log excerpts.
- All other targets (caddy, miniflux, pocketbase, pragma) still pass — cliff did not regress anything.

## Resolved decisions (in-flight)

- **C.4 provenance semantics** — *resolved 2026-04-27, mid-sprint.* Codex's reachability-based provenance is the spec as written. Go's mutual type references between Hub and WebConn mean both roots' closure walks transitively reach each other's methods; that's correctly recorded as `{Hub, WebConn}` provenance on `(*Hub).Broadcast`. The load-bearing signal — clearing SPRINT-0021's cliff — is `(*WebConn).writePump` in `closure.includedSymbols` with `{WebConn}` ⊆ provenance. Confirmed (3025 included / 4889 excluded; 127.96s wall / 2.07GB max RSS, well under C.gate-1 budget). Goroutine-level writer/reader attribution lives in Block D's SSA-based seam detector, not in symbol-level provenance. C.4 updated to assert non-empty provenance + sorted determinism + `(*WebConn).writePump` reachable from `{WebConn}`, no exclusive-single-root expectation on cross-referenced types.

## Emission design note (G.1/G.2)

The intended multi-root emission shape is one extracted-service binary at `cmd/monolift-extracted-connection-hub-buffer/main.go` plus one oracle binary for the same region. The union closure is emitted as one Go process; channel seams, including `WebConn.send`, remain ordinary in-process Go channels. Host-side dialer stubs are needed on each external entry point across both receiver types: Hub methods (`Broadcast`, `Register`, `Unregister`, `Start`, `CheckConn`, `SendMessage`, `ProcessAsync`, `Stop`) and WebConn methods (`Pump`, `writePump`).

Liftpatch does not fit this shape today. `PatchRequest` names one `FuncName`, one `PackageDir`, one `ExpectedSignature`, and one request-wide prelude; `PatchSymbolBody` rejects methods through `DiagnosticMethodReceiver`; `PatchResult` records one patched file and one hash pair. The concrete missing primitives are documented in `docs/research/runs/SPRINT-0022-emission-gap.md`. G.gate-1 therefore lands branch (R), not branch (S).

## Closeout

Branch: **(R)**.

Blocks A through F landed: shared-name pragma regrouping, multi-root closure union with sorted reachability provenance, SSA seam detection, composite candidate recognition, and region admission with the in-region channel seam check. Mattermost admission accepts the `WebConn.send` seam under the single-service hypothesis.

The sprint stops at G.gate-1 because the current liftpatch API cannot patch multiple receiver methods across the Hub/WebConn root set into one extracted service. This is classified as tooling immaturity rather than a fundamental distribution refusal. Union probe metrics: 3025 included / 4889 excluded, 127.96s wall, 2.07GB max RSS.
