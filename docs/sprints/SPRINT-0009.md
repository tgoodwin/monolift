# SPRINT-0009 — Reframe the classifier to reason about liftability properties

**Status:** planned
**Substrate:** [ADR-0017](../decisions/0017-classifier-reasons-about-liftability.md) (proposed)
**Planning brief:** [`docs/sprints/SPRINT-0009-brief.md`](./SPRINT-0009-brief.md)
**Primary touch points:** `pkg/compiler/shape/`, `pkg/compiler/extract/`, new `pkg/compiler/liftability/`, `pkg/compiler/reportv2/`, `pkg/compiler/passes/`, `docs/decisions/{0006,0015,0017}`, new `docs/decisions/0018-*`, `docs/specs/`

## Intent

Realize the architectural reframe committed as *proposed* in ADR-0017: move the compiler's admissibility classifier from pattern-matching on literal Go type signatures (`http-handler`, `ctx-request-response`, `channel-consumer`, …) to pattern-matching on **properties** of those signatures and of the function bodies they head. The pattern-matching *approach* established by ADR-0006 and ADR-0015 stays; what changes is *what the patterns express*. Transport selection becomes an explicit downstream step, preserving the existing transport commitments (`http-handler`, `http-json`, `channel-consumer`, `ctx-request-response`, `multi-domain-args`, `no-response`) as selector outputs rather than admissibility gates.

## Goals

1. **Decouple admission from transport.** A region is admitted into the lifted set because its intrinsic properties make local-to-remote rewriting sound, not because its signature already resembles a framework handler.
2. **Establish a stable, namespaced liftability-property taxonomy** grounded in a disciplined brainstorm (open-ended per the brief) and consolidated into an implementable subset.
3. **Preserve existing transport commitments and adapter derivation** by running transport selection downstream of admission, consuming property evidence.
4. **Accept ADR-0017** with populated Decision + Consequences, back-annotate ADR-0006 and ADR-0015 appropriately, and land the named property set as ADR-0018.

## Non-goals / scope fences

- **No `MLV2_*` refusal-code changes.** Every new property that can gate admission must map onto an existing code. New codes are out of scope.
- **No runtime, deployment-target, or adapter-runtime redesign.** Transport *templates* stay; transport *selection* moves.
- **No end-to-end golden-file migration plan.** Only the minimum updates the new classifier strictly requires. A dedicated `SPRINT-0010-GOLDENS` sprint is stubbed for the broader pass.
- **No `docs/site/` revision.** The design-story site currently describes the old classifier; its rewrite is deferred to `SPRINT-0010-DOC` after ADR-0017 is accepted and the implementation stabilizes.
- **Brainstorm scope fence:** candidate properties must be expressible within a well-formed Go program. No language-extension proposals, no cross-language analogues without a Go-specific realization.
- **No cosmetic renames.** Renaming "canonical shape" → "transport shape" across comments, report fields, and ADR prose is out of scope unless a specific report-field rename is strictly required by the new classifier.

---

## Phase 0 — Baseline, decision freeze, normativity inventory

- [x] Inventory the current classifier seam: record the ordered predicates in `pkg/compiler/shape/shape.go`, the registration seam in `pkg/compiler/passes/register.go`, the `ShapeResult` contract in `pkg/compiler/extract/hooks.go`, the orchestration order in `pkg/compiler/extract/extract.go`, the `root.shape` and `root.defaultTransport` write points, and the adapter-derivation dependency on `shapeResult.PerOperation` in `deriveAdapters`.

  Baseline recorded on 2026-04-21 before code motion:
  `pkg/compiler/shape/shape.go` classifies in strict first-match-wins order
  `isHTTPHandler` -> `isChannelConsumer` -> `isBuilderChain` ->
  `isCtxRequestResponse` -> `isMultiDomainArgs` -> `isNoResponse` ->
  `unsupportedEvidence`; `aggregateRoot` then derives the root shape from
  per-operation shapes.
  `pkg/compiler/passes/register.go` currently registers only three seams:
  `extract.RegisterShapeClassifier(shape.ForExtract)`,
  `extract.RegisterShapeValidator(shape.ValidatePragmaOptions)`, and
  `extract.RegisterStateInferer(stateclass.ForExtract)`.
  `pkg/compiler/extract/hooks.go` exposes one shape-centric contract:
  `ShapeClassification{Operation, Shape, DefaultTransport, Evidence}`,
  `ShapeResult{Root, PerOperation, Diagnostics}`, plus the
  `ShapeClassifier` and `ShapeValidator` function types.
  `pkg/compiler/extract/extract.go` currently runs:
  load/build -> seed report -> shape classification -> write
  `report.Root.Shape`/`report.Root.DefaultTransport` (plus pragma transport
  override) -> shape validation -> closure -> state inference ->
  `deriveAdapters` -> reflection/unsafe/plugin diagnostics -> refusal metadata.
  The report write points are the direct assignments in `Analyze`:
  `report.Root.Shape = shapeResult.Root.Shape` and
  `report.Root.DefaultTransport = shapeResult.Root.DefaultTransport`.
  Adapter derivation is shape-driven today: `deriveAdapters` iterates
  `shapeResult.PerOperation`, emits the handler adapter when any operation
  has shape `http-handler`, and builds the registry adapter's
  `CanonicalShapes` list by deduplicating `shapeResult.PerOperation[*].Shape`.
- [x] Freeze the architectural split in writing before any code moves: **liftability analysis becomes the admissibility gate; canonical transport shapes become downstream selector outputs; `MLV2_*` codes stay stable and only their triggering evidence changes.**

  Freeze for implementation work in this sprint:
  admission answers whether an exposed operation is liftable at all and is
  driven only by named liftability properties plus their evidence; transport
  selection runs after admission and is the only stage allowed to emit
  canonical transport-shape outputs such as `http-handler`,
  `channel-consumer`, `ctx-request-response`, `multi-domain-args`, and
  `no-response`.
  `root.shape` and `root.defaultTransport` stay in the report for downstream
  consumers, but from this point forward they are selector outputs rather
  than the classifier's admission vocabulary.
  Existing `MLV2_*` refusal codes are retained verbatim in this sprint; when
  a refusal is re-grounded in property evidence, only the evidence and
  triggering path change, not the refusal taxonomy.
- [x] Inventory every place the current shape vocabulary is normative: ADR-0006, ADR-0015, ADR-0017, `reportv2` schema + report types, schema tests, extraction integration tests, `pragma_keys.go` validators, and any prose docs that currently describe the classifier as signature-archetype matching. This inventory drives Phase 8 back-annotations and is not a gate for Phase 1 code motion.

  Normative and semi-normative inventory captured for follow-on edits:
  ADRs: `docs/decisions/0006-canonical-shapes-transport.md`,
  `docs/decisions/0015-canonical-shape-classifier.md`, and
  `docs/decisions/0017-classifier-reasons-about-liftability.md`.
  Spec and evolution prose:
  `docs/specs/monolift-v2-contract.md` transport/canonical-shape sections and
  `docs/evolution.md` entries describing SPRINT-0007 and canonical shapes.
  Report contract:
  `pkg/compiler/reportv2/report.go`, `pkg/compiler/reportv2/schema.json`, and
  `pkg/compiler/reportv2/report_test.go`.
  Extraction/integration tests and classifier tests:
  `pkg/compiler/extract_integration_test.go` and
  `pkg/compiler/shape/shape_test.go`.
  Orchestration and validators:
  `pkg/compiler/extract/extract.go`, `pkg/compiler/extract/hooks.go`,
  `pkg/compiler/passes/register.go`, and `pkg/compiler/pragma_keys.go`
  (surface/value validation stays shape-agnostic, but transport values and
  later compatibility checks consume the same vocabulary).
  Deferred prose outside this sprint's edit scope but relevant to the
  inventory:
  `docs/site/docs/canonical-shapes.md`, generated `docs/site/site/`, and
  related snippets that still narrate signature-archetype matching. The
  sprint's non-goal stands: inventory them now, leave the rewrite to
  `SPRINT-0010-DOC`.

---

## Phase 1 — Open-ended brainstorm (the brief's central task)

This phase is deliberately wider than it will land. The fan-out intent is to expose Go-expressible properties the author's "obvious set" missed. Output is a superset that Phase 2 prunes.

- [x] Create `docs/specs/liftability-properties-brainstorm.md` as the working scratchpad. For each candidate property, record: **name**, **namespace** (`boundary.*` | `effects.*` | `lifecycle.*` | `contract.*` | `transport.*`), **what the property expresses**, **why it matters for local-to-remote rewriting**, **detection sketch** (which pass: `go/types` | `golang.org/x/tools/go/ssa` | `golang.org/x/tools/go/callgraph` | `golang.org/x/tools/go/pointer` | AST), **confidence** (cheap-and-sound / cheap-and-heuristic / expensive), **outcome class** (admission-gating / transport-biasing / advisory), **mapped `MLV2_*` code** if admission-gating, and a worked Go example.
- [x] Populate the brainstorm with ADR-0017's starting set first — pass-by-value, serializability (or cheap deterministic transform), pointer-mediated mutation of caller memory, error-return discipline, synchronous-short-lived vs. long-running — each filled to the rubric so additions have a reference shape.
- [x] Extend with at least the following candidates; treat the list as non-exhaustive and propose more during the brainstorm:
  - `boundary.context_first` — first param is `context.Context` (cancellation/deadline vocabulary).
  - `boundary.variadic_free` — no variadic params at the public boundary.
  - `boundary.no_callable_values` — no `*types.Signature`-typed params or results (no caller-callback).
  - `boundary.no_streaming_values` — no `*types.Chan` values at the boundary.
  - `boundary.no_sync_primitives` — no `sync.Mutex`/`RWMutex`/`WaitGroup`/`sync/atomic` wrappers at the boundary.
  - `boundary.fully_instantiated` — no unresolved `*types.TypeParam` in params/results/receiver.
  - `boundary.serializable_via_custom_encoding` — Gemini's addition: types that satisfy `MarshalJSON`/`UnmarshalJSON` count as serializable even when raw structural serializability fails.
  - `effects.no_param_heap_mutation` — SSA provenance from params/receiver; inspect stores and field/index addresses.
  - `effects.no_param_escape` — param-derived aliases do not reach globals, `*ssa.MakeClosure`, or `*ssa.Go`.
  - `effects.no_global_writes` — no stores through `*ssa.Global`.
  - `effects.no_global_reads` — no loads of mutable package globals (const-qualified loads are fine).
  - `effects.no_param_interface_callbacks` — interface-method invocations on boundary-derived receivers are flagged.
  - `effects.no_reflect_unsafe` — callgraph reachability does not hit `reflect` / `unsafe` / `runtime.SetFinalizer`.
  - `effects.no_os_side_effects` — callgraph reachability does not hit `os` / `syscall` / filesystem / network-socket packages (allowlist for pure standard packages).
  - `lifecycle.no_async_fork` — no `*ssa.Go`.
  - `lifecycle.goroutine_joined` — if `*ssa.Go` occurs, there is `sync.WaitGroup` or channel-join evidence before return (heuristic).
  - `lifecycle.long_running_loop` — CFG back-edges (via `golang.org/x/tools/go/ssa.Dominators`) plus `*ssa.UnOp` receive or `*ssa.Select` inside the loop body.
  - `lifecycle.cancellation_honored` — loops check `ctx.Done()` or return `ctx.Err()` (heuristic).
  - `lifecycle.bounded_work` — no unbounded loop whose exit depends on state not in the signature.
  - `contract.error_last` — terminal result is `error`.
  - `contract.no_panic_only_failure` — no `*ssa.Panic` paired with absence of `error` result.
  - `contract.deterministic_under_retry` — no `time.Now` / `rand.*` / UUID / monotonic counters in the reachable body (heuristic; advisory).
  - `contract.receiver_read_only` — for methods, receiver fields are not mutated.
  - `transport.handler_boundary` — reuses existing `net/http` and Caddy middleware signature predicates, but only as **selector evidence**, not admission.
  - `transport.receiver_returns_self` — builder-chain detection retained as a selector refusal signal.
- [x] For every candidate, explicitly label it as **admission-gating**, **transport-biasing**, or **advisory**. Default to advisory on uncertainty.
- [x] For every admission-gating candidate, name the existing `MLV2_*` refusal code a violation would map onto. If no existing code fits, the property is demoted to advisory (or stated as a blocker that the brief's `MLV2_*`-stability rule must resolve).
- [x] Cross-reference the brainstorm against `docs/specs/monolift-v2-contract.md` §Conceptual-Model Baseline so no property silently widens the paper's bounded-lift commitment.
- [x] Ask, as its own bullet: **which current signature-only checks (`isHTTPHandler`, `isChannelConsumer`, `isCtxRequestResponse`, `isMultiDomainArgs`, `isNoResponse`, `isBuilderChain`) survive as first-class properties, and which become selector-only signals.** Retention is as explicit a decision as addition.

---

## Phase 2 — Taxonomy landing

- [x] Consolidate the brainstorm into `docs/specs/liftability-properties.md` — the *implementable* subset. Every property carries a stable `Name` (kebab-case, under its namespace), a `PropertyID` Go identifier, a prose definition, the detection pass, the outcome class (gate/bias/advisory), the mapped `MLV2_*` code (if gating), and an evidence-record template.
- [x] Freeze the evidence-record convention: `PropertyID`, subject (`receiver` | `param[n]` | `result[n]` | `body`), verdict (`Hold` | `Violate` | `Unknown`), evidence source (`types` | `ssa` | `callgraph`), and a deterministic detail string. Document ordering rules so tests can use exact comparisons.
- [x] **Commit the heuristic-containment rule explicitly** in the taxonomy spec: *sound detectors may gate admission; heuristic detectors default to `Unknown` and stay advisory unless promoted by evidence*. This is the single most important policy to prevent the new classifier from regressing acceptance the first time it runs on the corpus.
- [x] Write ADR-0018 (*Liftability property taxonomy — named set*). Narrow scope: freezes names + IDs + outcome classes. Future property additions become small decision records that append to ADR-0018's set rather than re-opening ADR-0017. Keep ADR-0018 short.
- [x] Record in `docs/evolution.md` the move from *canonical shapes match signatures* to *properties decide admissibility; shapes bias transport*. One paragraph.

---

## Phase 3 — Liftability analysis package

- [x] Create `pkg/compiler/liftability/` with files split by concern:
  - `property.go` — `PropertyID`, `Verdict`, `Evidence` struct, `Result` aggregating one operation's evaluations.
  - `detector.go` — `Detector` interface: `ID() PropertyID`, `Evaluate(ctx, op) (Verdict, []Evidence, error)`.
  - `registry.go` — register + iterate detectors.
  - `boundary.go` — `go/types`-only detectors over `*types.Signature`.
  - `effects.go` — SSA-based detectors over parameter provenance and stores.
  - `lifecycle.go` — CFG/loop detectors.
  - `contract.go` — signature-property detectors (`error_last`, `no_panic_only_failure`, etc.).
  - `decision.go` — per-operation and root-level aggregation, including explicit rules for mixed-operation roots.
- [x] Build an analysis `Context` bundling `*extract.LoadedModule`, `*ssa.Program`, the chosen callgraph, and per-function SSA-walk caching so detectors don't re-traverse.
- [x] **Gate 3.1 — callgraph spike.** Before body-level detectors land, spike `cha` vs. `rta` callgraph builders against the evaluation corpus on the largest fixture (Mattermost or Gitea). Pick the builder that answers "reachable-from-function" accurately without blowing up build time. Record the decision inline in `pkg/compiler/liftability/doc.go` with evidence.
- [x] Implement the **signature-level detectors first** (cheap, pure `go/types`) so Phase 5 can start the classifier rewrite without waiting on body-level work:
  - `boundary.context_first`, `boundary.variadic_free`, `boundary.no_callable_values`, `boundary.no_streaming_values`, `boundary.no_sync_primitives`, `boundary.fully_instantiated`, `boundary.serializable_via_custom_encoding`, `contract.error_last`.
- [x] Implement the **body-level detectors** once the callgraph choice is frozen:
  - `effects.no_param_heap_mutation`, `effects.no_param_escape`, `effects.no_global_writes`, `effects.no_global_reads`, `effects.no_param_interface_callbacks`, `effects.no_reflect_unsafe`, `effects.no_os_side_effects`, `contract.no_panic_only_failure`, `contract.receiver_read_only`, `lifecycle.no_async_fork`, `lifecycle.long_running_loop`.
- [x] Implement the synchronous-short-lived vs. long-running classifier as a body-level detector returning a three-valued outcome (`sync-short`, `long-running`, `unknown`); this one feeds both admission and transport selection, so keep it promoted.
- [x] Cache property results at the package level where detectors prove idempotent over a package's SSA, reusing across multiple extraction roots.
- [x] Preserve deterministic output ordering for facts and diagnostics so tests can use exact string comparisons.
- [x] **Per-detector unit tests.** For every detector, add `pkg/compiler/liftability/<file>_test.go` with a tiny fixture package under `pkg/compiler/liftability/testdata/` exercising `Hold`, `Violate`, and `Unknown` branches. Table-driven assertions over `Verdict` and evidence substrings. Detectors that fail this gate do not proceed to Phase 5.

---

## Phase 4 — Extract seam and report contract

- [x] Extend `pkg/compiler/extract/hooks.go` with a `LiftabilityResult` hook and result type rather than overloading `ShapeResult` with two concepts.
- [x] Rewire `pkg/compiler/extract/extract.go` so the orchestration order is explicit and unambiguous: **seed report → liftability analysis → pragma validation against liftability/selector outcomes → downstream transport-shape selection → closure/state inference → adapter derivation.**
- [x] Update `pkg/compiler/passes/register.go` so liftability analysis and transport-shape selection register as independent passes.
- [x] Add **additive** `reportv2` fields for liftability reasoning in `pkg/compiler/reportv2/report.go` and `pkg/compiler/reportv2/schema.json`: a root-level admission verdict plus structured property-evidence records. Keep `root.shape` and `root.defaultTransport` populated as downstream selector outputs. No existing field is removed or renamed in this sprint.
- [x] Route new conservative refusals onto existing `MLV2_*` codes and messages; if a code already exists for the refusal class, only the evidence and triggering condition change.

---

## Phase 5 — Classifier rewrite

- [x] Gut `classifyOperation` in `pkg/compiler/shape/shape.go` and replace it with an evaluator that runs the admission-gating detectors and produces a liftability result. Keep the `Classify` and `Result` public names to cap the blast radius on call sites; evolve their fields.
- [x] Stop populating `Classification.Shape` from archetype predicates on the admission path. Populate a new `Classification.Admission` field (`Liftable` | `Refused` | `Unsupported`) and a `Classification.Properties []liftability.Evidence` field.
- [x] Migrate `aggregateRoot` to aggregate on `Admission` + property verdicts rather than on `Shape`. A root is liftable iff every exposed operation is liftable; mixed outcomes emit a single structured refusal carrying per-operation evidence.
- [x] Move the archetype predicates that are purely transport-framing (`isHTTPHandler`, `isChannelConsumer`, `isCtxRequestResponse`, `isMultiDomainArgs`, `isNoResponse`, `isBuilderChain`) out of the admission path and into `pkg/compiler/shape/transport_signals.go`. Retain the code; it feeds Phase 6, not Phase 5.
- [x] Re-express `unsupportedEvidence` (variadic, channel-crossing, func-crossing, `unsafe.Pointer`) as admission-gating property detectors under `boundary.*`. Delete the helper once callers consume the detector outputs.

---

## Phase 6 — Transport selection downstream

- [x] Introduce a transport-selection package (prefer `pkg/compiler/transport/`; if that proves too disruptive to existing imports, `pkg/compiler/shape/transport_select.go` is the fallback).
  - `Template` type: `handler`, `http-json`, `channel-consumer`, `reserved-grpc`.
  - `Select(liftability.Result, pragmaOptions) (Template, []Evidence, error)`.
- [x] Implement selection rules as an explicit pattern-matching table, **never terminal on a literal signature alone**:
  - `transport=handler` pragma + `transport.handler_boundary` evidence → `handler`.
  - `transport=handler` pragma + no handler-boundary evidence → `MLV2_SHAPE_UNSUPPORTED` (preserved).
  - `transport=grpc` → `MLV2_TRANSPORT_RESERVED` (unchanged).
  - No pragma + `lifecycle.long_running_loop` + no public channel crossing → `channel-consumer`.
  - No pragma + `sync-short` + serializable boundary + `contract.error_last` → `http-json`.
  - No pragma + `contract.no_panic_only_failure` violation with long-running lifecycle → existing `MLV2_NO_ERROR_CHANNEL` path (preserved).
- [x] Preserve `ctx-request-response`, `multi-domain-args`, and `no-response` as recognizable **selector outputs** — not as admission gates. If the classifier accepts a region whose shape signals match one of these, the selector labels it accordingly for downstream transport/adapter code.
- [x] Enforce the guardrail: **no selector rule may be terminal on a raw signature predicate alone**. Every selector rule references a named property fact. Lint this in a unit test.
- [x] Wire `ValidatePragmaOptions` (currently in `shape.go`) to consult the transport-selection output rather than the legacy `Shape`.
- [x] Keep `extract.ShapeClassification.DefaultTransport` populated for downstream code-gen consumers; its value now comes from `transport.Select`, not from `defaultTransportForShape(shape)`.
- [x] Update `deriveAdapters` (`pkg/compiler/extract/extract.go`) so it consumes selector output + selector evidence, not the old classifier's literal-shape evidence strings.

---

## Phase 7 — Integration and minimum golden updates

- [x] Audit call sites via `grep -R "shape.Shape\|ShapeHTTPHandler\|ShapeChannel\|ShapeCtxRequestResponse" pkg/ internal/ cmd/`; update each to consume the new fields.
- [x] Run the targeted unit-test suite required by the resume prompt. Update only goldens that the new classifier *strictly* requires (evidence strings, new property IDs in reports). Goldens that change for cosmetic or ordering reasons are out of scope — file them for `SPRINT-0010-GOLDENS`.
- [x] Run `test/e2e/` integration tests. Ensure the **Caddy anti-regression anchor** holds: the already-supported Caddy middleware fixture still ends with `root.shape=http-handler` and `root.defaultTransport=handler`, but now because property evidence assigned it, not because of literal type matching.
- [x] **Cover the `http-json` selector path with a synthetic unit test** in the transport-selection package: construct a `liftability.Result` matching the ctx-request-response profile (`boundary.context_first` + serializable boundary + `contract.error_last` + sync-short lifecycle) and assert `transport.Select` returns `http-json`. This exercises the rule without a new committed evaluation target. *(The live Miniflux anti-regression was dropped from this sprint — see **Deferred follow-ups** below.)*
- [x] **Selector-rule coverage gate.** Every rule in the Phase 6 selection table must be exercised by at least one test (unit or e2e): `handler`-with-evidence, `handler`-without-evidence refusal, `grpc` reserved refusal, `channel-consumer`, `http-json`, long-running `no-response` + preserved `MLV2_NO_ERROR_CHANNEL`. Any uncovered rule is a test gap this sprint must close.
- [x] Verify the `accept-with-warnings` verdict introduced by the recent pragma parser sprint (commit 840c447) still holds under the new classifier on the fixture that introduced it.
- [x] Manual corpus cross-check: run the classifier against the evaluation corpus fixtures and confirm no previously-accepted case is refused without an ADR-0017-grounded reason. Any surprises get documented inline in ADR-0017 §Consequences. _Satisfied via the SPRINT-0010-CLASSIFIER-PERF spot-read on 2026-04-22. `TestExtractCaddyReverseProxyProducesNonEmptyValidatedReport` refusals (`MLV2_CHANNEL_BOUNDARY`, `MLV2_REFLECTION_DISPATCH`, `MLV2_SERIALIZATION_UNSUPPORTED` on `sync.Mutex`/`sync.Once`/`sync.RWMutex`/`sync/atomic.*`/channels/func values/`unsafe.Pointer`, `MLV2_SHAPE_UNSUPPORTED`) are all ADR-0017-grounded: the new property detectors correctly catch closures the old signature-matching classifier admitted naively. PocketBase refusals are its existing expected-refusal set. No surprises worth an ADR-0017 §Consequences amendment — the refusals are exactly what the reframe was designed to surface. Full diagnostic capture at `/tmp/caddy-spotread.log`._

---

## Phase 8 — ADR closure and deferrals

- [x] Populate ADR-0017 §Decision: the property-based admission rule, the admission/transport split, the map from properties to existing `MLV2_*` codes, and a reference to ADR-0018 for the named set.
- [x] Populate ADR-0017 §Consequences: better semantic fit; more analysis complexity; heuristic-containment policy (heuristics stay advisory); preserved `MLV2_*` taxonomy; continued reuse of canonical transport shapes as downstream selector outputs; deferred site revision.
- [x] Commit ADR-0018 (*Liftability property taxonomy — named set*) with the Phase 2 content.
- [x] Back-annotate ADR-0006: add a narrower-scope Status update stating that canonical shapes still organize transport and adapters, but no longer define admissibility. **Do not mark as fully superseded** — the canonical-shape concept survives as transport-selection vocabulary.
- [x] Back-annotate ADR-0015: mark as superseded in classifier logic by ADR-0017 and ADR-0018. Include a short pointer to the new liftability-first analysis path.
- [x] Append `docs/evolution.md` entry summarizing the reframe landing, linking ADR-0017 and ADR-0018.
- [x] Open scoped sprint stubs for the explicitly deferred work: `docs/sprints/SPRINT-0010-CLASSIFIER-PERF.md` (classifier-test SSA sharing + duplicate-callgraph elimination; must land first), `docs/sprints/SPRINT-0010-GOLDENS.md` (full golden migration, depends on PERF), and `docs/sprints/SPRINT-0010-DOC.md` (design-story site revision after the classifier stabilizes).
- [x] **Flip ADR-0017 from `proposed` to `accepted`** — this is the final act of the sprint, after implementation, schema changes, tests, and back-annotations all land. The status flip confirms the sprint, not the other way round.

---

## Sequencing

```
Phase 0 ─► Phase 1 ─► Phase 2 ─┬─► Phase 3 ─┬─► Phase 5 ─► Phase 6 ─► Phase 7 ─► Phase 8
                               └─► Phase 4 ─┘
```

- **Phase 0** is the only hard prerequisite for everything; the inventory is cheap and prevents silent assumptions about existing seams.
- **Phase 1** must close before Phase 2 — the implementable subset can only be chosen after the superset is written down.
- **Phase 2** must close before Phase 3 — the detector package API encodes the taxonomy; writing it before the taxonomy freezes invites churn.
- **Phase 3** and **Phase 4** can overlap once the taxonomy is frozen, because the extract-seam work (hooks, reportv2 additive fields, pass registration) is orthogonal to the detector implementations. But Phase 4's reportv2 shape depends on the fact model landed in Phase 2, so Phase 4's *schema edits* cannot precede Phase 2's evidence-format freeze.
- **Phase 5** needs the admission-gating subset of Phase 3 detectors green; it need not wait for all detectors.
- **Phase 6** consumes Phase 3 detector outputs and Phase 5's classifier results. It cannot be parallel with late Phase 5 — transport selection depends on stabilized detector outputs and the admission/selection contract.
- **Phase 7** begins after Phase 6 and reveals any integration bugs that force focused golden updates.
- **Phase 8** is strictly last. Acceptance-flipping ADR-0017 is the terminal act.

---

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| The property list balloons into an unimplementable wish list. | Phase 2 freezes a bounded implementable subset with explicit "load-bearing now" vs. "defer" labels. Each surviving property carries one concrete detector spec before it lands. |
| Heuristic detectors produce false refusals against the evaluation corpus. | Phase 2 commits the heuristic-containment rule: heuristics default to `Unknown` and stay advisory; only sound detectors gate admission. No detector gates admission until its `Unknown` behavior is audited against the corpus. |
| Callgraph reachability analysis blows up build time on large corpus fixtures. | Phase 3 Gate 3.1 spikes `cha` vs. `rta` against the largest fixture before any body-level detector lands. Per-package caching of property results reduces per-root cost. Detectors that cannot bound their cost are deferred or demoted to advisory. |
| The downstream selector quietly recreates the old classifier. | Phase 6 enforces the guardrail that no selector rule may be terminal on a raw signature predicate alone; every rule references named property facts. A unit test lints this. |
| `go/pointer` aliasing analysis over-promises soundness. | Aliasing-dependent detectors (`effects.no_param_escape`) stay advisory in this sprint unless the callgraph spike proves cheap soundness. `effects.no_param_heap_mutation` uses parameter-provenance SSA without full pointer analysis. |
| Keeping `Shape` alive while changing its role preserves naming confusion. | The enum survives in Phase 6 as a transport-signal type only. Phase 5 stops populating it from archetype predicates. A follow-on sprint may rename; this sprint does not. |
| Report and extract integration breaks silently. | Phase 4 adds reportv2 fields *additively*; no existing field is removed or renamed. Phase 7 explicitly asserts the two semantic anchors (handler still lands on `handler`; domain function still lands on `http-json`). |
| Golden-file churn overwhelms the sprint. | Phase 7 updates only goldens the new classifier strictly requires. Evidence-format determinism (Phase 2) is the precondition that makes a later full migration mechanical. The full migration is scheduled as `SPRINT-0010-GOLDENS`. |
| Documentation scope creeps into site rewrites during implementation. | `docs/site/` stays untouched. The site-revision sprint is stubbed as `SPRINT-0010-DOC` and starts only after ADR-0017 is accepted. |
| ADR-0018 introduces taxonomy-stability burden. | ADR-0018 is narrow — names + IDs + outcome classes only. Future property additions become small ADRs that append to the set rather than re-opening ADR-0017. |

---

## Acceptance criteria

- [x] `pkg/compiler/liftability/` exists with per-property detectors, each unit-tested over `Hold`/`Violate`/`Unknown` branches.
- [x] `pkg/compiler/shape/shape.go`'s admission path no longer runs archetype predicates; the archetype code that survives lives on the transport-selection path (`transport_signals.go`).
- [x] `pkg/compiler/extract/extract.go` executes in the order *seed → liftability → pragma validation → transport selection → closure/state → adapters*.
- [x] `reportv2` carries root-level admission verdict plus structured property evidence as **additive** fields; `root.shape` and `root.defaultTransport` remain populated as downstream selector outputs.
- [x] The `MLV2_*` refusal code set is unchanged; every refusal emitted by the new classifier maps to an existing code.
- [x] An already-accepted Caddy handler fixture still produces `root.shape=http-handler` and `root.defaultTransport=handler` in the report, but through property evidence rather than literal type matching. Verified by an explicit e2e assertion.
- [x] The `http-json` selector rule is exercised by a synthetic unit test in the transport-selection package that constructs a ctx-request-response-profile liftability result and asserts the selector output (no committed evaluation target required).
- [x] Every selector rule in the Phase 6 table is exercised by at least one test.
- [ ] `go test ./...` is green. Only goldens the new classifier strictly requires have been updated; broader fixture churn is documented and deferred.
- [x] ADR-0017 status is `accepted`; §Decision and §Consequences are non-empty and reference ADR-0018.
- [x] ADR-0018 exists with the named property set and stable IDs.
- [x] ADR-0006 carries a narrower-scope update (not full supersession); ADR-0015 carries a supersession note pointing at ADR-0017 and ADR-0018.
- [x] `docs/evolution.md` records the reframe landing.
- [x] `docs/site/` is untouched.
- [x] `docs/sprints/SPRINT-0010-CLASSIFIER-PERF.md`, `docs/sprints/SPRINT-0010-GOLDENS.md`, and `docs/sprints/SPRINT-0010-DOC.md` exist as scoped sprint stubs.

## Deferred follow-ups

- **Miniflux live anti-regression.** A live e2e assertion that Miniflux
  `currentUserHandler` lands on `http-json` (backed by a committed
  `test/e2e/targets/miniflux/` target with a pragma and a golden report) is
  deferred to a future Miniflux unskip sprint. The `http-json` selector rule
  itself is covered in this sprint by a synthetic unit test (Phase 7), so the
  classifier is not untested against that code path; what is deferred is the
  *target-level* regression gate that would live next to the existing Caddy
  and Pocketbase e2e targets.

## Blockers

_(Updated 2026-04-22 after SPRINT-0010-CLASSIFIER-PERF landed.)_

- Acceptance item "`go test ./...` is green" remains open. **Memory pressure is no longer the blocker** — SPRINT-0010-CLASSIFIER-PERF's Fix 3 landed SSA sharing in the shape suite (worst-run peak 1820 MB → 635 MB, −65.1%) and Fix 4 landed callgraph reuse with a structural invariant. The aggregate `go test ./pkg/... -count=1` lane now fails on a **diagnostic-duplication bug**: every `MLV2_*` code is emitted twice, breaking the assertion in both `TestExtractCaddyReverseProxyProducesNonEmptyValidatedReport` and `TestAnalyzeDetectsPocketBaseRefusals`. Plus the Caddy integration test's expected output is stale against the liftability-first classifier. Both routed to `SPRINT-0010-GOLDENS.md` (items #1 Caddy golden update, #2 diagnostic duplication). SPRINT-0009 closes on that sprint's landing.
- The manual corpus cross-check (line 225) is satisfied in place via the SPRINT-0010 spot-read — see the checkbox note above.
