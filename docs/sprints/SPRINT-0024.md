# SPRINT-0024 — Invocation-trace probe + report-first invocation pass (entrypath)

**Status:** in-progress
**Predecessors:** SPRINT-0021 (Mattermost composite slice), SPRINT-0022 (multi-root pragma + closure union), SPRINT-0023 (boot-path extraction + RegionPatchRequest + stream-proxy emitter).
**Touches:** new package `pkg/compiler/entrypath/`; additive changes to `pkg/compiler/reportv2/{schema.json,report.go}` and `pkg/compiler/surface/surface.go`; additive call site in `pkg/compiler/extract/extract.go`. **No** changes to `pkg/compiler/extract/bootpath/`, no new `docs/decisions/` files.

## Intent

The compiler today characterises what is *inside* a region (closure union, surface derivation, boot-path) but cannot answer the question that gates session-surface emission on real apps: **how is the region externally invoked?** `surface.Derive` (`pkg/compiler/surface/surface.go:53`) only inspects root-method bodies; `bootpath.Walk` (`pkg/compiler/extract/bootpath/walk.go:13`) is forward-only union ∪ {main}. There is no reverse edge from a region root to its caller, and no function-value flow tracking. On the Mattermost overlay, `(*Hub).Start` and `(*WebConn).Pump` do not explain the external surface by themselves: the concrete link is the package-level function value `connectWebSocket` passed to `APIHandlerTrustRequester`, stored in `web.Handler.HandleFunc`, and invoked from `Handler.ServeHTTP`.

SPRINT-0024 lands a falsifiable probe in a new sibling package `pkg/compiler/entrypath/`, gates further work on the probe recovering the Mattermost ground-truth chain without string matching or framework recognisers, and (only if the gate passes) produces an additive `InvocationTrace` artifact in reportv2 plus a `surface.DeriveWithTrace` consumer. No emission. No new ADR. No replacement of `bootpath`.

## Goals

- New `pkg/compiler/entrypath/` package implementing reverse callgraph reachability + forward function-value flow. Sibling to `bootpath`, not a rewrite.
- Phase 1 gate: probe names `connectWebSocket` as the external surface for the Hub/WebConn region and recovers the chain `api4.Init → InitWebSocket → Router.Handle(... APIHandlerTrustRequester(connectWebSocket))` using only generic SSA / callgraph evidence, within the existing Mattermost budget (~55s wall, 2.4 GB RSS — same envelope as SPRINT-0023).
- Phase 2 (only if gate passes): additive `invocationTrace` section in reportv2 (`schemaVersion` `"1.0"` → `"1.1"`); `surface.DeriveWithTrace` consumer wired so the Mattermost overlay classifies as `Session`/`streamproxy` *via the trace*, not via root-method `exposesSession` heuristics.
- Negative fixture proves the analyser is not string-matching framework names: two handlers in the same program both contain `"websocket"` as a substring; only the one structurally reachable to the region root appears in the trace.
- Toy fixtures cover each generic edge kind: function-value-as-arg, struct-field-stored handler, global-stored handler, returned wrapper, `http.Handler` interface dispatch, goroutine-launched handler, closure registration, route-builder chaining (recognised structurally — fluent chain on the same receiver where one method takes an `http.Handler`-typed arg).

## Non-goals

- **No emission.** Boot-path / stream-proxy codegen stays deferred. Phase 2 produces a report artifact and a `surface.Derive` consumer only.
- **No new ADR this sprint.** Decisions go in `docs/evolution.md`. (User explicit — do not draft ADR-0028. Do not call gate-failure notes "ADR-stubs".)
- **No taxonomy.** No registry of app patterns, no framework recognisers (`gorilla`, `chi`, `echo`, `gin`, `mux`, `mattermost`). The only ecosystem knowledge allowed is `net/http` boundary semantics: `http.Handler`, `ServeHTTP`, `http.Server.Handler`.
- **No replacement of bootpath.** `pkg/compiler/extract/bootpath/walk.go` stays forward-only and byte-identical.
- **No Mattermost-specific code paths in the analyser.** Mattermost is the *forcing fixture*, not a code-path target. If a fix would only be defensible with "because Mattermost," it does not land.
- **No string matching on framework names.** Enforced by P1.15 lint test with explicit allowlist (`"http.Handler"`, `"ServeHTTP"`, `"net/http"`).

## Sequencing — Phase 1 → gate → Phase 2

Phase 1 is the kill: if reverse callgraph + function-value flow cannot recover the registration chain on the Mattermost overlay within budget, no Phase 2 implementation will save it. Phase 2 is shape work — schema, consumer wiring, fixtures — and only spends effort once Phase 1 has shown the underlying analysis is sound.

The gate is a **kill-ladder, cheapest first**. Stop at the first failure:

1. **gate-A** — RTA over the Mattermost overlay completes within 55s wall / 2.4 GB RSS at all.
2. **gate-B** — Reverse BFS from `(*Hub).Start` / `(*WebConn).Pump` reaches *some* registration site (not necessarily the right one).
3. **gate-C** — Forward function-value flow tracks at least one `*ssa.MakeClosure` / `*ssa.Function` reference through a struct-field store → invocation. (If RTA edges already give the answer without function-value flow, prefer that simplicity — but the toy fixtures still need to fire.)
4. **gate-D** — Full chain recovery on Mattermost: probe names `connectWebSocket`, recovers the four-link path including `APIHandlerTrustRequester(connectWebSocket)`, uses zero framework-name strings.

If any of A/B/C/D fails: stop. Append a brief gate-failure narrative to `docs/evolution.md` (which gate, what the probe returned instead, one concrete next thing to test). **Do not** proceed to Phase 2. **Do not** call the note an ADR-stub.

Phase 2 tasks are blocked on **gate-D passing**. If Phase 1 lands but gate-D doesn't fire, Phase 2 does not start.

**Cut policy if scope balloons** (in priority order, drop from the bottom):

1. P2.16 (goroutine fixture) and P2.17 (route-builder fixture).
2. Confidence-tier diagnostics (P2.2 `Confidence`/`MissingEdges` fields). Land the structural fields only; add confidence in a follow-up sprint.
3. **Do not cut**: gate ladder (P1.10–P1.15), negative fixture (P2.13), forbidden-string lint test (P1.15 / gate-D-5), golden-diff scoping (P2.5), `cmd/entrypath-probe/`, `evolution.md` closeout (P2.20). These are the falsifiability surface. VTA fallback is **architecturally required** per spec — do not cut.

---

## Phase 1 — Falsifiable probe

Goal: stand up `pkg/compiler/entrypath/` with the smallest analysis that can falsifiably answer "does reverse callgraph + function-value flow recover the Mattermost ground-truth chain on this overlay?" Tasks live entirely in `pkg/compiler/entrypath/` and the e2e harness. **Nothing in `pkg/compiler/extract/bootpath/`, `pkg/compiler/surface/surface.go`, or `pkg/compiler/reportv2/` changes during Phase 1.**

### P1 — Package scaffolding

- [x] **P1.1** Create `pkg/compiler/entrypath/` package. Files: `entrypath.go` (public `Probe` entry point + result types), `callgraph.go` (RTA/VTA construction + reverse BFS), `funcvalue.go` (forward function-value def-use flow), `boundary.go` (`net/http` predicates only — no `gorilla.go`, no `mattermost.go`), `confidence.go` (confidence-tier scoring; may be deferred per cut policy), `types.go` (result types).
- [x] **P1.2** Result types in `entrypath/types.go`. Fields chosen to match the Phase 2 schema so the artifact lands without renames: `RegionRoots`, `ExternalSurfaces`, `RegistrationSites`, `WrapperChains`, `RegionTouchpoints`, `BootStartCandidates`, `Diagnostics`, `Stats`. Stable node identity uses `reportv2.SymbolIdentity` (package path, receiver, function name) plus source position.
- [x] **P1.3** Probe driver `entrypath.Probe(prog *ssa.Program, mainPkg *ssa.Package, regionRoots []*ssa.Function) (ProbeResult, error)`. No reportv2 schema dependency in Phase 1 except the `SymbolIdentity` value type.

### P1 — Callgraph + reverse BFS

- [x] **P1.4** Build whole-program callgraph using `golang.org/x/tools/go/callgraph/rta`. Seed from application entrypoints: `mainPkg.Func("main")` plus relevant package init roots required for the loaded main package. Do **not** seed region roots or their transitive callees as RTA roots — that would make the region reachable by assumption and defeat the "how is this invoked from the application?" question. Cache the constructed `*callgraph.Graph` so subsequent reverse-BFS calls don't rebuild.
- [x] **P1.5** VTA fallback in `entrypath/callgraph.go`. **Trigger is structural, not a name match**: when an RTA node has zero out-edges and its static signature accepts a parameter of static type `http.Handler` (or any interface with `ServeHTTP`), build a second graph using `golang.org/x/tools/go/callgraph/vta` over the same SSA program and merge. Diagnostic `{Kind: "vta_fallback_used", reason: "rta_indirect_collapse"}` is recorded. Tests for the trigger live in `entrypath/callgraph_test.go` against a toy interface-dispatch fixture.
- [x] **P1.6** Reverse BFS in `entrypath/callgraph.go`. Inputs: callgraph + region roots. Output: per root, the deduplicated set of caller functions in deterministic order (sort by `*ssa.Function.String()`). Bound: ≤4096 visited nodes per root before refusing with `Diagnostic{Kind: "reverse_bfs_bound_exceeded"}`.
- [x] **P1.7** Telemetry in `Stats`: function count, static edge count, dynamic edge count, unresolved-dynamic-site count, callgraph algorithm used (`rta` / `rta+vta`), wall-clock millis, peak RSS bytes. Emitted alongside `ProbeResult`.

### P1 — Function-value flow

- [x] **P1.8** Function-reference index in `entrypath/funcvalue.go`. Build a whole-program `FunctionRefIndex` for each `*ssa.Function` and `*ssa.MakeClosure` value: operand references, call arguments, stores, returns, captures, direct invokes, and goroutine launches. The index is queried from three starting sets: function values discovered along reverse paths, function values that reference external-surface candidates, and function values flowing into `http.Handler`-shaped sinks.
- [x] **P1.9** Forward def-use walk in `entrypath/funcvalue.go`. For each indexed function value selected by P1.8, follow def-use until termination at one of:
  - stored into a struct field (`*ssa.Store` whose target is `*ssa.FieldAddr`)
  - stored into a global (`*ssa.Store` whose target is `*ssa.Global`)
  - stored into a map / slice / array element where SSA exposes the value operand (covers route-builder map sinks)
  - passed as a call argument whose static parameter type is an interface (recorded as **registration-via-interface**)
  - returned from a function (recorded as **returned wrapper**)
  - captured by another `*ssa.MakeClosure` (recorded as **closure capture**)
  - invoked directly via `*ssa.Call` whose `CallCommon.Value` is the function value
  - launched via `*ssa.Go`
  Termination produces a `RegistrationSite` or `WrapperChain` link with the corresponding edge kind.
- [x] **P1.10** Wrapper-chain assembly: when a function-value flow visits an intermediate `*ssa.MakeClosure` whose `Bindings` include another `*ssa.Function` already classified as an `ExternalSurface` candidate, or when the `FunctionRefIndex` finds a package-level function value passed through wrapper calls before reaching a sink, record the link as a wrapper edge. The chain is the ordered sequence handler → outermost wrapper → registration site.
- [x] **P1.11** "Long-lived sink" predicate in `entrypath/boundary.go`. A `RegistrationSite` is HTTP-shaped iff it terminates at a parameter of static type `http.Handler` *or* a struct field whose owning type has a `ServeHTTP` method. The `net/http` recognition is the **only** ecosystem-aware predicate. No lifetime heuristics beyond this — keep it cheap and structural.

### P1 — Mattermost gate

- [x] **P1.12** Probe binary `cmd/entrypath-probe/main.go` (mirrors `cmd/extract-report/`). Loads SSA + callgraph for a target package, accepts `--region-root pkg.Type.Method` flags, prints `ProbeResult` as deterministic JSON to stdout. Used both by tests and for hand-debugging when the gate fails.
- [x] **P1.13** Wire the probe into `test/e2e/harness/target.go` so the existing Mattermost target requests a probe run during the existing compile flow but does not yet consume the result. Mattermost overlay bytes remain identical to SPRINT-0023; only an additional artifact is written to the run directory.
- [ ] **P1.14** Run the probe on the SPRINT-0023 Mattermost overlay with `--region-root '(*Hub).Start' --region-root '(*WebConn).Pump'`. Capture `ProbeResult` + `Stats` into `docs/research/runs/SPRINT-0024-mattermost-probe.md`.
- [ ] **P1.15** **Gate test** in `entrypath/mattermost_gate_test.go`, build-tagged behind `MONOLIFT_E2E=1`. Asserts the gate ladder:
  - **gate-A**: `Stats.WallClockMillis ≤ 60000` and `Stats.PeakRSSBytes ≤ 2.5 * 1024 * 1024 * 1024` (slack on SPRINT-0023's measured envelope).
  - **gate-B**: at least one `RegistrationSite` reaches a parameter of static type `http.Handler` from any region root.
  - **gate-C**: at least one toy fixture (P2.14–P2.18 land in Phase 2; for Phase 1 a minimal struct-field-stored fixture lives at `entrypath/testdata/struct_field_handler/` and is exercised here) flows a function value through a `*ssa.FieldAddr` store into an `EdgeFunctionValueStoredField` registration.
  - **gate-D-1**: `connectWebSocket` (qualified `github.com/mattermost/.../api4.connectWebSocket`) appears in `ProbeResult.ExternalSurfaces`.
  - **gate-D-2**: at least one `WrapperChain` contains the ordered link `connectWebSocket → APIHandlerTrustRequester → registration site whose static parameter type is http.Handler`. Source-location anchors: handler at `evaluation/mattermost/server/channels/api4/websocket.go:57`, registration at `websocket.go:52`, wrapper at `evaluation/mattermost/server/channels/web/handlers.go:544`.
  - **gate-D-3**: at least one `BootStartCandidate` resolves to `api4.Init`, anchored at `evaluation/mattermost/server/channels/api4/api.go:185`.
  - **gate-D-5** (forbidden-string lint): a Go test that runs `go list` plus a regex over `pkg/compiler/entrypath/*.go` non-test sources confirms zero occurrences of `"websocket"`, `"Mux"`, `"HandleFunc"`, `"mattermost"`, `"gorilla"`, `"chi"`, `"echo"`, `"gin"`. Allowlist: `"http.Handler"`, `"ServeHTTP"`, `"net/http"`.
- [x] **P1.16** **If any gate-X fails**: append a closeout paragraph to *this file* under `## Phase 1 outcome` naming the failed gate, the observed evidence, and one concrete next probe to try. Add a corresponding short paragraph to `docs/evolution.md` under the entrypath / SPRINT-0024 entry. **Do not proceed to Phase 2.**

### Phase 1 acceptance (the gate)

- [ ] All P1.1–P1.15 land.
- [ ] `entrypath/mattermost_gate_test.go` passes under `MONOLIFT_E2E=1`.
- [ ] `go test ./pkg/compiler/...` green; existing surface/bootpath/transport tests untouched.
- [ ] `evaluation/mattermost/` byte-identical (`make verify-evaluation-untouched`).
- [ ] `pkg/compiler/extract/bootpath/`, `pkg/compiler/surface/surface.go`, `pkg/compiler/reportv2/` unchanged.
- [ ] `cmd/entrypath-probe/` builds and produces deterministic JSON for a fixed seed.

---

## Phase 2 — Report-first invocation pass (only if gate-D passes)

Goal: turn the probe into an `InvocationTrace` artifact in reportv2 and feed it to a new `surface.DeriveWithTrace` so the Mattermost overlay classifies as `Session`/`streamproxy` *because of the trace*. The artifact is the deliverable — there is no emission consumer this sprint.

### P2 — InvocationTrace schema (reportv2, additive)

- [ ] **P2.1** Bump `pkg/compiler/reportv2/report.go` `SchemaVersion` from `"1.0"` → `"1.1"` per existing convention. Add a one-line comment ("1.1: added invocationTrace section").
- [ ] **P2.2** Add `InvocationTrace` Go type in `pkg/compiler/reportv2/report.go` mirroring `entrypath.ProbeResult`: `ExternalSurfaces`, `RegistrationSites`, `WrapperChains`, `RegionTouchpoints`, `BootStartCandidates`, `Confidence` (enum `high|partial|missing-edges`, computed in `entrypath/confidence.go`), `MissingEdges` (one per probe `Diagnostic` whose Kind is in `{rta_indirect_collapse, vta_fallback_used, reverse_bfs_bound_exceeded, funcvalue_terminated_at_unknown_sink}`).
- [ ] **P2.3** Extend `pkg/compiler/reportv2/schema.json` additively. New top-level optional property `invocationTrace`; JSON-schema definitions for each sub-type. Every existing required field stays required; no existing field changes type. Update schema's `schemaVersion` const to `"1.1"`.
- [ ] **P2.4** Round-trip + validation tests in `pkg/compiler/reportv2/report_test.go`:
  - an existing 1.0 report (without `invocationTrace`) still decodes successfully through the report reader, while newly emitted reports use schemaVersion 1.1;
  - a 1.1 report with a fully populated `invocationTrace` round-trips through the encoder/decoder byte-identically;
  - a 1.1 report whose `invocationTrace.Confidence` is not in the enum fails validation.
- [ ] **P2.5** Goldens for SPRINT-0017 / 0019 / 0020 (caddy + miniflux + pocketbase + pragma) update to include `"schemaVersion": "1.1"` and an empty/null `"invocationTrace"`. Pick one representation (empty struct or null) and document; assert that the diff contains **only** those two lines per fixture. Unbounded golden churn is a smell — fix encoder determinism before re-asserting.

### P2 — Promote probe → pass

- [ ] **P2.6** Promote `entrypath.Probe` → `entrypath.Pass`. `Pass` returns `reportv2.InvocationTrace` directly; `entrypath/types.go` private types become aliases for the reportv2 types. The probe binary `cmd/entrypath-probe/main.go` becomes a thin CLI wrapper around `Pass`.
- [ ] **P2.7** Generic edge taxonomy in `entrypath/edges.go`. **Eight kinds, no framework knowledge** (route-builder chaining is a *fixture*, not a top-level edge kind):
  - `EdgeStaticCall`
  - `EdgeDynamicInterface` (RTA/VTA)
  - `EdgeMethodCall`
  - `EdgeFunctionValueArg`
  - `EdgeFunctionValueStoredField`
  - `EdgeFunctionValueStoredGlobal`
  - `EdgeFunctionValueReturned`
  - `EdgeClosureCapture`
  - `EdgeGoroutineLaunch`
- [ ] **P2.8** Wire `entrypath.Pass` into the existing extract pipeline at the same point `bootpath.Walk` is called today (sibling, not replacement). New invocation in `pkg/compiler/extract/extract.go` runs `entrypath.Pass` after `bootpath.Walk` and before reportv2 finalisation. The result is written into `Report.InvocationTrace`.

### P2 — surface.DeriveWithTrace consumer

- [ ] **P2.9** Add `surface.DeriveWithTrace(root reportv2.Root, reachable []*ssa.Function, trace reportv2.InvocationTrace) (RegionSurface, error)` as an **additive sibling** to `surface.Derive`. Keep `surface.Derive` callable for tests that have no trace. Rationale: changing `Derive`'s signature would churn every call site and the SPRINT-0019/0020 goldens for one sprint where the trace is still being shaken out.
- [ ] **P2.10** Consumer logic in `DeriveWithTrace`. The trace proves that an external HTTP surface is attached to the region; it does **not** by itself prove the surface is session-shaped. If `trace.RegionRoots` matches the requested `root` AND `trace.ExternalSurfaces` contains an entry whose edge-kind set intersects `{EdgeFunctionValueStoredField, EdgeFunctionValueStoredGlobal, EdgeFunctionValueArg}` AND any wrapper chain ends at a sink whose static parameter type is `http.Handler`, inspect the external surface function body for session evidence (`Upgrade`, `Hijack`, or raw `net.Conn` exposure). Only then classify as `SurfaceSession` / `WireProtocolStreamProxy`; otherwise classify the traced surface as `SurfaceCall` / `WireProtocolHTTPJSON` or fall back to the existing body walk when the trace is empty.
- [ ] **P2.11** **Existing `exposesSession` heuristic stays** as fallback for regions where the trace produced no entry (low-confidence or empty). Phase 2 deliberately preserves both signals; a follow-up sprint can audit whether the heuristic is still load-bearing once the trace is consumed elsewhere.
- [ ] **P2.12** Wire the call site in `pkg/compiler/extract/extract.go` (the place currently calling `surface.Derive` for each root) to use `DeriveWithTrace` when the just-computed `InvocationTrace` is non-empty for that region; keep the bare `Derive` path on the empty-trace branch.

### P2 — Negative + toy fixtures

- [ ] **P2.13** **Negative fixture** at `pkg/compiler/entrypath/testdata/negative_unrelated_websocket/`. Two completely independent handlers, **both containing the substring `"websocket"` in their package or symbol names**. One uses `(*websocket.Upgrader).Upgrade()`; the other is the only one structurally reachable from the lifted region's root via a function-value flow. `entrypath.Pass` must emit `ExternalSurfaces` containing only the second handler. The first must not appear in any wrapper chain attached to the region. **This is the regression guard for the "no string matching" non-goal** — both handlers share the substring; only the structurally attached one is recorded.
- [ ] **P2.14** Toy fixture at `entrypath/testdata/wrapper_callback/`. A `func(http.Handler) http.Handler` middleware applied to a handler then registered. Expected `WrapperChain` with two links: handler → middleware → registration.
- [ ] **P2.15** Toy fixture at `entrypath/testdata/struct_field_handler/`. `type Server struct { handler http.Handler }`; constructor stores into the field; `(s *Server) Run()` invokes `s.handler.ServeHTTP(w,r)`. Expected `RegistrationSite` with `Edge: EdgeFunctionValueStoredField`. (Doubles as the gate-C fixture in P1.15.)
- [ ] **P2.16** Toy fixture at `entrypath/testdata/interface_dispatch/`. `var h http.Handler = myHandler{}; srv := &http.Server{Handler: h}`. Expected `EdgeDynamicInterface` edge into `myHandler.ServeHTTP`.
- [ ] **P2.17** Toy fixture at `entrypath/testdata/goroutine_handler/`. `go srv.ListenAndServe()`. Expected `EdgeGoroutineLaunch` flagged in the chain.
- [ ] **P2.18** Toy fixture at `entrypath/testdata/route_builder_chain/`. Synthetic `Router` type with `Handle(path string, h http.Handler) *Router; Methods(...) *Router`. The probe must recognise the registration **structurally** (function-value flowing into an `http.Handler`-typed parameter on a fluent chain) without any string match on `"Mux"` or `"Handle"`.
- [ ] **P2.19** All toy fixtures live under `entrypath/testdata/` and are exercised by `entrypath/pass_test.go`. Each test asserts the `InvocationTrace` shape, not the textual report.

### P2 — Mattermost end-to-end + closeout

- [ ] **P2.20** Mattermost report assertion in the e2e harness: `InvocationTrace.ExternalSurfaces` includes `connectWebSocket`, `RegistrationSites` includes the route registration, `WrapperChains` includes `APIHandlerTrustRequester(connectWebSocket)`, `RegionTouchpoints` includes `(*Hub).Start` and `(*WebConn).Pump`.
- [ ] **P2.21** Mattermost surface assertion: `surface.DeriveWithTrace` on the Mattermost trace returns `Category: SurfaceSession`, `WireProtocol: WireProtocolStreamProxy`, with evidence pointing at `InvocationTrace`-derived edges (not `exposesSession` heuristic).
- [ ] **P2.22** Budget regression guard: re-run the Mattermost probe under the existing e2e budget envelope. Fail on regression beyond ~55s wall / 2.4 GB RSS. Record final metrics in `docs/research/runs/SPRINT-0024-mattermost-probe.md`.
- [ ] **P2.23** Append an entry to `docs/evolution.md` titled "SPRINT-0024 — invocation-trace probe + report-first invocation pass". Cover: what landed, gate-D outcome and metrics, why no ADR (decision deferred until consumer is real), the open question for the next sprint (does emission consume `InvocationTrace` directly, or do we route it through a refactor of `bootpath` first?). Narrative entry, not a normative decision — flag any normatively constraining decision for ADR-isation in a *future* sprint.
- [ ] **P2.24** Sprint closeout section appended to *this file* under `## Phase 1 outcome` and `## Phase 2 outcome`: which gates fired, probe metrics, fixture pass list. Mirrors SPRINT-0023's closeout shape.
- [ ] **P2.25** Update `docs/sprints/ledger.yaml` SPRINT-0024 entry to `done` with executor + branch tag. **No new ADR file** under `docs/decisions/`.

### Phase 2 acceptance

- [ ] All P2.1–P2.25 land.
- [ ] `pkg/compiler/reportv2/schema.json` `schemaVersion` is `"1.1"`; round-trip + 1.0 decode-compat tests green.
- [ ] SPRINT-0017/0019/0020 goldens diff is **only** the `schemaVersion` line and the empty `invocationTrace` field. No bytes change in fixture-derived report sections.
- [ ] Mattermost overlay run produces a non-empty `InvocationTrace` whose `ExternalSurfaces` contains `connectWebSocket`. `surface.DeriveWithTrace` on that trace returns `Category: SurfaceSession`, `WireProtocol: WireProtocolStreamProxy`.
- [ ] Negative fixture (`negative_unrelated_websocket`): trace contains zero region-attached entries for the unrelated WebSocket handler.
- [ ] All toy fixtures (P2.14–P2.18) green.
- [ ] `evaluation/mattermost/` byte-identical (`make verify-evaluation-untouched`).
- [ ] `cmd/main.go`, `pkg/compiler/extract/bootpath/walk.go`, `PatchSymbolBody` API, `RegionPatchRequest` API, ADR-0018 — all unchanged.
- [ ] Forbidden-string lint test (P1.15 gate-D-5) still passes against `pkg/compiler/entrypath/` non-test sources.
- [ ] No file under `docs/decisions/` created or modified.

---

## Risks and mitigations

| # | Risk | Trigger | Mitigation |
|---|---|---|---|
| 1 | **RTA imprecision through `chi.Mux` / `gorilla/mux`-style indirection.** RTA can lose type info at interface dispatch; `(*Mux).Handle` accepts `http.Handler`, and RTA may either generate spurious edges to every `ServeHTTP` in the program or collapse to none. | gate-A passes but gate-B reaches dozens of unrelated handlers, or out-edges from a registration node collapse to zero. | VTA fallback per P1.5, **structurally triggered** (zero out-edges from a node whose static signature takes `http.Handler`). If VTA fixes precision → trace records `algo: rta+vta`. If VTA also fails → gate-D fails honestly; document in `evolution.md`, stop. |
| 2 | **Budget blow-up.** Whole-program RTA on Mattermost may exceed 2.4 GB or 55s. SPRINT-0023 already ran the overlay close to that bound. | gate-A: `WallClockMillis > 60000` or `PeakRSSBytes > 2.5 GB`. | (a) Seed RTA narrowly per P1.4 (`main` + relevant init roots), not region roots or every package init. (b) If still over budget, build the callgraph lazily over the reachable subset from `main`. **Do not** expand the budget envelope to make the test pass — that defeats falsifiability. **Do not** exclude `net/http` from the search — that severs the boundary the gate depends on. |
| 3 | **Function-value flow false positives.** Forward def-use over `*ssa.Function` values can reach sinks that have nothing to do with HTTP handling (e.g., a `func()` cleanup callback). | Negative fixture P2.13 attaches the wrong handler; or Mattermost trace contains 50 `ExternalSurfaces`. | Sink classification is `net/http`-specific (P1.11): a registration is HTTP-shaped only if it terminates at an `http.Handler`-typed parameter or a struct field whose owning type has `ServeHTTP`. Non-HTTP sinks become `Diagnostic`s, not `RegistrationSite`s. Negative fixture P2.13 is the regression guard. |
| 4 | **Function-value flow false negatives.** Wrapper or stored-handler path loses `connectWebSocket` before registration. | gate-D-2 fails: no `WrapperChain` reaches the upgrader. | P1.8 enumerates termination conditions including map/slice/container stores where SSA exposes the value operand. Missing-edge diagnostics name the lost SSA instruction precisely so the next probe iteration knows what to add. |
| 5 | **Scope creep into emission.** Tempting to "just" wire the trace into stream-proxy emission once `DeriveWithTrace` returns Session. | A task description starts referencing `pkg/compiler/transport/emit/`. | **Cut on sight.** If a task touches `pkg/compiler/transport/emit/` it is out of scope. Phase 2 deliverable is the artifact + the `surface` consumer; emitter consumption is a follow-up. P2.23 explicitly names the next-sprint hand-off. |
| 6 | **Surface-derivation regression on caddy/miniflux/pocketbase.** `DeriveWithTrace` plus an empty trace must behave exactly as `Derive` today. | SPRINT-0019/0020 goldens churn beyond the `schemaVersion` + `invocationTrace` lines. | The "additive sibling" choice in P2.9 keeps the empty-trace path identical to current behavior. P2.5 asserts goldens diff is minimal. CI failure here is a hard stop, not a golden-update sprint. |
| 7 | **Framework recognizer creep.** Analyser starts recognising `gorilla`/`chi`/`mux` names to pass Mattermost. | `entrypath/*.go` non-test source contains a forbidden string. | P1.15 gate-D-5 lint test enforces the allowlist (`"http.Handler"`, `"ServeHTTP"`, `"net/http"` only). All other ecosystem semantics live in fixtures. |
| 8 | **`evolution.md` becomes a junk drawer for "decisions we should have ADR-ed".** | Phase 2 closeout adds three "decision" paragraphs that look like ADR stand-ins. | `evolution.md` entries are *narrative*, not normative. If a decision is normatively constraining (e.g., "all session-surface emission must consume `InvocationTrace`"), flag it in P2.24 closeout for ADR-isation in a *future* sprint when the constraint actually binds future work. User is explicit: no ADR-0028 this sprint. |
| 9 | **Coupling between `entrypath` and `bootpath`.** Reverse semantics leak into the forward-only boot-path package. | `bootpath/walk.go` import list grows; or shared types appear in `bootpath` that only entrypath needs. | Keep all reverse / function-flow code in `pkg/compiler/entrypath`. Shared identity goes through `reportv2.SymbolIdentity`. P1 acceptance asserts `bootpath/walk.go` byte-identical. |

## Anticipated cliffs

| # | Cliff | Symptom | Stop |
|---|---|---|---|
| 1 | RTA whole-program build OOMs on Mattermost overlay. | gate-A: peak RSS > 2.5 GB. | Try seed-narrowing (Risk #2(a)). If still over budget, **stop at Phase 1**, write `evolution.md` entry, do not proceed. |
| 2 | RTA precision insufficient even with VTA fallback through `chi`-style indirection. | gate-D-1 fails: `connectWebSocket` not reachable, or every `http.Handler` in the program is recorded as a candidate. | **Stop at Phase 1.** Note in `evolution.md` what the probe *did* recover and which structural pattern defeated it. Next sprint's design constraint is "what additional analysis would close the precision gap." |
| 3 | Function-value flow misses the upgrader path because the Mattermost route builder stores closures into a map keyed by string at runtime. | gate-D-2 fails: no `WrapperChain` reaches `connectWebSocket`. P1.8 map/slice tracking caught the easy cases but the route-builder uses a more exotic shape. | **Stop at Phase 1.** Adding deeper map-flow tracking risks defeating falsifiability discipline. Document and defer. |
| 4 | Schema version bump cascades into more golden churn than expected because reportv2 fields are encoded with `omitempty` inconsistencies. | P2.5: goldens diff includes more than the `schemaVersion` + `invocationTrace` lines. | This is a Phase 2 *blocker*, not a cliff. Fix encoder determinism (one task: enforce `omitempty` consistency for the new field) before re-asserting goldens. If determinism requires structural reportv2 changes, **stop Phase 2** and land Phase 1 only. |
| 5 | `surface.DeriveWithTrace` consumer accidentally re-classifies caddy/miniflux/pocketbase regions as Session. | SPRINT-0019/0020 wire-protocol selection diffs. | The `DeriveWithTrace` predicate (P2.10) requires *both* the function-value-flow edge-kind set AND the `http.Handler` sink AND the trace's `RegionRoots` matching the requested root. Caddy's middleware setup chain doesn't cross a region root in those targets, so the trace is empty for them. If this still misfires, narrow the predicate before relaxing it. |

## Frozen boundaries

- `cmd/main.go` byte-identical.
- `pkg/compiler/extract/bootpath/walk.go` byte-identical (forward-only stays forward-only).
- `evaluation/mattermost/` byte-identical pre/post compile.
- ADR-0018 Layer-1 properties unchanged; no new `liftability.PropertyID` constants.
- `PatchSymbolBody` and `RegionPatchRequest` API surfaces unchanged (additive-only changes elsewhere).
- SPRINT-0017/0019/0020/0022/0023 e2e fixtures byte-identical except for the `schemaVersion` + empty `invocationTrace` line bump (P2.5).
- No file under `docs/decisions/` created or modified.

**May move under documented hypothesis** (per the discipline checklist used since SPRINT-0021): `pkg/compiler/entrypath/` (new package, all internals fluid), `pkg/compiler/reportv2/{report.go,schema.json}` (additive `invocationTrace` only), `pkg/compiler/surface/surface.go` (additive `DeriveWithTrace` sibling, original `Derive` unchanged), `pkg/compiler/extract/extract.go` (one new pass invocation between `bootpath.Walk` and `reportv2` finalisation), `cmd/entrypath-probe/` (new debug binary), `test/e2e/harness/target.go` (probe wiring only).

## Closeout (filled in at sprint end)

### Phase 1 outcome

Phase 1 stopped at gate-A. The toy probe fixtures pass for VTA fallback, reverse
BFS, function-value-as-argument indexing, struct-field handler storage, and
wrapper callback propagation. The Mattermost probe required the existing
SPRINT-0021/SPRINT-0023 `GOWORK` file; without it, package loading resolved the
wrong `server/public` module and failed on model symbol skew. With
`GOWORK=/Users/tgoodwin/projects/monolift/.tmp/sprint-0021-a1-go.work`, the
probe exceeded the 60s gate-A wall-clock ceiling before emitting JSON and was
killed; `/tmp/monolift-sprint-0024-mattermost-probe.json` was 0 bytes.

No `ProbeResult` or `Stats` were produced, so gates B-D were not evaluated and
Phase 2 did not start. The next concrete probe is to split package load, SSA
build, RTA/VTA, reverse BFS, and function-value propagation timing, then rerun
Mattermost with function-value propagation seeded only from reverse-path and
HTTP-sink candidates instead of every indexed function value.

Follow-up diagnostic: after adding `cmd/entrypath-probe --diagnostic-timings`,
a five-minute Mattermost run completed package load (~4.9s), SSA build (~4.8s),
root resolution (~16.3s), callgraph construction (~39.7s), and reverse BFS
(~0.35s), then timed out while still building the whole-program
function-reference index. Reported memory had already exceeded the original
gate-A RSS budget during package load/SSA/root resolution. The next optimization
target is therefore not Phase 2 wiring; it is avoiding all-function root
resolution and replacing whole-program function-reference indexing with a
narrowed index rooted in reverse paths, HTTP-shaped sinks, and candidate
external surfaces.

### Phase 2 outcome

Not started. Gate-D did not pass because Phase 1 stopped at gate-A.
