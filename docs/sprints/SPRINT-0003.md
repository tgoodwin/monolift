# SPRINT-0003 — Monolift v2 Contract Specification

**Status:** completed · **Scope:** spec-only, no code · **Timebox:** none (quality-first)
**Primary deliverable:** `docs/specs/monolift-v2-contract.md` (single versioned markdown doc, ≥ v1.0 on close)
**Primary input:** `docs/evaluation/generalization-analysis-2026-04-19.md`

---

## Why this sprint exists

The April 19 generalization audit showed Monolift's v1 input contract fits **1–4 out of 8** dimensions on six real Go monoliths. Every target violates at least four of: interface-annotated services, unique implementer, `New<Iface>` constructor, `main`-reconstructable wiring, uniform `(ctx, req) → (resp, err)` methods, statelessness, package-equals-service. The demo at `demo/monolith/` is the *only* app that fits — because it was written to the compiler.

Before we write any more compiler code, we fix the contract. This sprint produces one document that tells a future compiler implementer, a Monolift researcher, and a candidate-application developer exactly what Monolift v2 promises, refuses, and defers. It is spec-only. The next sprint implements against it.

## Goals

- A single versioned normative document at `docs/specs/monolift-v2-contract.md` that resolves all seven audit axes.
- For each axis: a normative rule, worked examples, refusal diagnostics, and evidence from ≥1 real target.
- A validation walk-through for **all six** evaluation targets — miniflux, listmonk, caddy, gitea, mattermost, pocketbase — including pocketbase as the intentional negative case.
- An explicit PLOS '25 delta: each baseline claim marked preserved / revised / retired with rationale.
- A rejection-rationale table for discarded designs, every row linked to audit or research-brief evidence.
- A handoff to SPRINT-0005: implementation epics derived from the spec, not contained in this sprint.

## Non-goals

- No compiler, runtime, codegen, manifest, or test code changes. Nothing under `pkg/`, `cmd/`, `demo/`, `output/`. No CI changes, benchmarks, or executable tests.
- No attempt to retrofit v1 onto any evaluation target.
- No new language, IDL, framework, or application-rewrite requirement (these are explicitly rejected alternatives, not goals).
- No solution to the global transition-function / cross-lift placement-optimization problem. Per-lift policy only; cross-lift orchestration is deferred with written rationale.
- No transport performance claim requiring measurement.
- No commitment to a second language or serverless backend beyond reserving extension points.

## Scope boundaries

**In scope:** the normative v2 contract text; glossary and normative-language conventions; pragma grammar and examples; validation matrices and per-target walk-throughs; refusal diagnostics at the spec level; compatibility commitments; reserved extension points.

**Out of scope:** all changes under `pkg/`, `cmd/`, `demo/`, `output/`; CI changes and executable tests; benchmark campaigns; production readiness (auth, TLS, retries, circuit breakers, HPA, observability) except where the spec reserves extension points; v2 compiler internals beyond the *closure report* contract interface.

## Design axes the spec MUST resolve

1. **Annotation surface** — what syntactic forms accept a pragma (interface decl, struct method, package-level func, struct type, …).
2. **Extraction root** — how the compiler finds what to lift once the annotation is located; how the closure terminates.
3. **State semantics** — which in-process state travels with the lift; which patterns are refused.
4. **Transport** — HTTP/JSON RPC vs. shape-preserving handler forwarding vs. gRPC vs. reserved future; default per signature class.
5. **Dispatch granularity** — per-interface vs. per-method vs. per-call-site decision point, and how `local | remote | dynamic` composes.
6. **Multi-implementer handling** — how annotations disambiguate registries, plugin sets, mocks, build-tagged variants.
7. **Pragma syntax** — v2 grammar, defaults, error categories, migration story from `// @monolift` / `//monolift:offload`.

Open PLOS '25 revisions in play (pre-dispositions to be confirmed or rejected during drafting):

- "Lifts are stateless" → **revise**: lifts may be stateless-replica, singleton, or affinity-routed; compiler-enforced classification.
- "Wiring lives in `main`" → **retire**: extraction is call-graph-driven from the annotated root; wiring source is irrelevant.
- "Annotation site is an interface" → **revise**: interfaces are one of several annotation surfaces; no longer privileged.
- "Dual dispatch at interface granularity" → **revise**: dispatch point is the lift-point expression; granularity depends on annotation surface.
- "Pay-as-you-go, monolith still runs un-compiled" → **preserve**: the single non-negotiable invariant.

## Working method

Draft → target-validate → revise. The spec is not done when the normative sections are written; it is done when a complete pass through all six targets uncovers no rule the spec fails to handle (or handles with a refusal the team accepts). Budget **two full Phase 9 revision passes**. A third pass is triggered automatically if reviewer feedback (Phase 11) invalidates a rule.

Open a **running rejection log** in Phase 0 so alternatives discarded mid-drafting don't vanish before Phase 10.

---

## Tasks

All concrete work is a checkbox. Phases are ordered; validation is interleaved starting Phase 2 (annotation-surface cross-check) and becomes mandatory in Phase 9.

### Phase 0 — Setup & evidence baseline

- [x] Create `docs/specs/` and the empty `docs/specs/monolift-v2-contract.md` stub with frontmatter (title, status=draft, version=0.1-draft, date, authors, change log section). Record rationale in sprint notes if the path changes.
- [x] Write an "evidence index" subsection linking: `docs/evaluation/generalization-analysis-2026-04-19.md`, `docs/codebase-state-2026-04-19.md`, `docs/evaluation/README.md`, `research/RESEARCH_BRIEF.md`, `research/monolift_PLOS.pdf`.
- [x] Extract the PLOS '25 load-bearing claims (annotated code segments as lifts, lift points at call sites, transparent monolith execution, dynamic delegation, bounded lift model, Kubernetes as one backend) into a "conceptual-model baseline" subsection.
- [x] Build the **traceability table**: one row per broken v1 assumption from the audit (the A–H scorecard plus the 5 "consistently breaks" items), one column per planned spec section. Empty cells at sprint close = unresolved = blocking.
- [x] Name the three intended readers (compiler implementer, Monolift researcher, candidate-adopter application developer) and write a one-line reading guide for each.
- [x] Open a running **rejection log** in the sprint scratch area so mid-drafting discards are captured before Phase 10 formalization.

### Phase 1 — Terminology & framing

- [x] Draft the normative-language conventions (MUST / SHOULD / MAY / implementation-defined) and the normative-vs-example split.
- [x] Draft the glossary: *lift*, *lift root*, *lift point*, *extraction closure*, *closure report*, *lifted deployable*, *local impl*, *remote impl*, *adapter*, *dispatch policy*, *state class*, *refusal diagnostic*. Each term defined exactly once.
- [x] Draft the v1→v2 delta narrative (≤1 page): what v1 assumed, what broke, what v2 commits to.
- [x] Draft the **compatibility promise** as a normative non-negotiable: an annotated monolith MUST build and run under ordinary `go build` without Monolift.

### Phase 2 — Annotation surface

- [x] Draft normative rule for pragma on **interface declaration** (v1 carryover, now one case among several).
- [x] Draft normative rule for pragma on **package-level function declaration** (targets: miniflux `ProcessFeedEntries`, listmonk `worker`).
- [x] Draft normative rule for pragma on **method with concrete receiver** (targets: mattermost `UserService.CreateUser`, gitea service funcs).
- [x] Draft normative rule for pragma on **struct type declaration** (inferred public-method surface; targets: caddy modules).
- [x] Decide per-form: accepted / deferred / refused for interface method, function value in var, anonymous func, generic instantiation, whole package. Record rationale inline; log rejections to Phase 10.
- [x] Write one minimal Go example per accepted form; one refusal example per rejected form with the intended diagnostic.
- [x] **Cross-check surface rules** against each target's most plausible lift site (listmonk worker, miniflux processor, mattermost UserService method, caddy module struct, gitea mailer, pocketbase `core.App`). Surface-level mismatches must be fixed here, not deferred to Phase 9.
- [x] Sketch a **provisional pragma surface-syntax** (keys only, not EBNF) sufficient to express the surface rules — finalized in Phase 8, but visible early so subsequent phases can reference it.

### Phase 3 — Extraction root & closure

- [x] Define *extraction root* formally for each accepted annotation surface.
- [x] Specify closure computation as **call-graph / SSA-based transitive closure from the root** — explicitly replacing v1's `main`-walk. Reference `golang.org/x/tools/go/ssa` as one candidate analysis substrate; keep internal data structures non-normative.
- [x] Specify what closure *includes*: reachable functions, reachable package-level vars the closure reads/writes, reachable types.
- [x] Specify what closure *excludes* and the **termination rules**: stdlib boundary, external-module boundary, cgo, reflection-driven dispatch (refused), build-tag-gated code, dynamic plugin loading, generated code.
- [x] Specify handling of wiring idioms that feed values into the closure: `init()` chains, Options builders, registry `Register(...)` calls, lifecycle hooks (`OnBootstrap`). The rule: wiring source doesn't matter — the value graph at program-init is the input.
- [x] Specify the **closure report** — the required output of extraction analysis (included symbols, captured state, external deps, refusals). This is *the named interface* between the spec and SPRINT-0005. The one place implementation detail is required in the spec.
- [x] Specify boundary-pruning rules so a small lift cannot accidentally absorb the whole monolith; define the **"closure too large" refusal diagnostic** by name.

### Phase 4 — State semantics

- [x] Draft the **state taxonomy**: (a) stateless, (b) immutable-captured config, (c) externalized durable (DB, KV, Dapr), (d) process-local cache, (e) singleton mutable (goroutine + channel, worker pool, subscription hub), (f) shared mutable across unrelated callers, (g) connection/session state.
- [x] For each class, specify lift disposition: replicated / singleton / affinity-routed / externalize-required / refused.
- [x] Define what "state lifted with the lift" means concretely for receiver fields, package globals, closures, long-lived goroutines, channels, caches, connection pools.
- [x] Decide: is state class **inferred** by the compiler, **declared** in the pragma, or both-with-override? Specify developer obligations.
- [x] Draft failure / cancellation / deadline / panic / zero-value semantics at the contract level. Honor Waldo: state plainly which invariants break at the network boundary.
- [x] Write the refusal section — what v2 will never lift (e.g., pocketbase's SQLite-embedded `core.App`) — with concrete criteria and named diagnostics.
- [x] Cross-check state classes against mattermost WebHub, listmonk campaign-worker queue, gitea mailer context/cache, caddy cert/issuer state, miniflux feed-worker concurrency, pocketbase embedded SQLite.

### Phase 5 — Transport & adapter

- [x] Draft the **transport taxonomy**: HTTP/JSON RPC (default), shape-preserving HTTP-handler forwarding, gRPC/protobuf, reserved future (in-proc, serverless, shared-memory).
- [x] Adopt **"canonical shapes"** as the organizing concept for signature classification: group method signatures into a small bounded set of canonical shapes, each with a shared adapter template, to prevent transport-code explosion. Specify the shapes: `(ctx, req) → (resp, err)`, multi-domain-arg methods, no-response methods, HTTP handlers (`http.Handler`, `echo.HandlerFunc`, caddy middleware), channel consumers, builder chains. Each shape maps to exactly one default transport or a refusal.
- [x] Specify **shape-preserving transport**: when the lift root is an HTTP-shaped handler, the lifted deployable stays an HTTP handler — no JSON/gRPC re-encoding round-trip. Targets: listmonk echo handlers, caddy middleware.
- [x] Specify serialization rules for parameters, return values, errors, pointer graphs, interfaces-in-parameters, generics, context values. Channels across the boundary: refused or specified.
- [x] Decide: is gRPC a first-class v2 transport or a reserved extension? Record rationale either way.
- [x] Specify how `context.Context` cancellation and deadlines cross the boundary — what's preserved, what's not.
- [x] Write the contract-level rule that replaces `clientgen.go:110`'s panic-on-unhandled-return: unresolvable signature → compile-time refusal with a named diagnostic.

### Phase 6 — Dispatch granularity & placement policy

- [x] Define the **lift point** — the compiler-inserted site that chooses local vs. remote per invocation.
- [x] Decide dispatch granularity *per annotation surface*: interface-pragma → per-interface or per-method? function-pragma → per-call-site? method-pragma → per-method? struct-pragma → per-method of exposed surface? Write the matrix.
- [x] Specify policy composition: outer (struct/interface) policy + inner (method) override; which wins; how conflicts diagnose.
- [x] Specify policy modes: `local` (never remote), `remote` (always remote), `dynamic` (runtime-decided via policy expression); singleton placement is driven by state disposition.
- [x] Specify how the v2 policy-expression DSL relates to PLOS '25 delegate expressions. Keep CPU/MEM threshold triggers as one concrete instantiation; do not re-solve the control-theoretic problem here.
- [x] Explicitly **defer** global / cross-lift optimization with written rationale citing the paper's unsolved transition function.

### Phase 7 — Multi-implementer handling

- [x] Demote unique-implementer detection to an **optimization** — not a prerequisite. Rewrite the fallback path.
- [x] Specify **disambiguation syntax**: `impl=ConcreteName` on interface pragmas.
- [x] Specify the alternative: annotate the concrete implementer directly (often the cleaner move).
- [x] Specify registry/plugin handling: lift keyed by registration ID (caddy modules).
- [x] Specify adapter-wrapper handling: distinguish wrapping adapters from independent implementers.
- [x] Specify that generated mocks and build-tagged alt impls are *ignored* during impl resolution.
- [x] Specify the "lift the dispatch point" mode: when there are multiple prod impls and the user wants the interface-switch itself lifted (miniflux Google/OIDC providers, gitea senders).

### Phase 8 — Pragma syntax v2

- [x] Inventory current syntax: `// @monolift trigger=CPU threshold=0.5` (demo) and `//monolift:offload` (paper). Record the v1 state as a baseline.
- [x] Draft v2 grammar (EBNF) with one canonical form. Goals: readable, stable parse surface, extensible via keyed options, no new IDL.
- [x] Specify keys: `name`, `mode` (local|remote|dynamic), `state` (stateless|singleton|affinity|external), `transport` (http-json|handler|grpc), `impl`, `registry`, `policy` (e.g., `trigger=CPU threshold=0.5`). Specify which are required vs. optional per annotation surface.
- [x] Define defaults, invalid combinations, parse-error vs. validation-error categories.
- [x] Write ≥8 worked pragma examples covering: interface+dynamic, function+singleton+worker, method+remote+grpc, struct+shape-preserving, interface+impl=X, registry-keyed, local-only, refusal diagnostic.
- [x] Decide migration: v1 `@monolift` syntax is accepted / warned / rewritten / rejected by future v2 compiler. Record rationale.
- [x] Gate: do not close this phase until every Phase 2–7 decision has a pragma representation **or a deliberately inferred rule** documented.

### Phase 9 — Cross-target validation (all six required)

For each target, produce one subsection with: plausible annotation, expected extraction root, closure sketch, state classification, transport choice, dispatch granularity, implementer handling, verdict (accept / partial / refuse-with-rationale). When validation breaks a rule, revise the rule and redo prior targets under the revised rule.

- [x] **miniflux** — lift feed fetcher (`ProcessFeedEntries` or equivalent). Exercises: function-pragma, stateful worker, multi-impl provider (Google/OIDC), method shape diverging from `(ctx,req)→(resp,err)`. Expected verdict: **accept**.
- [x] **listmonk** — lift campaign worker + template/render flow. Exercises: channel-consumer signature, `App{...}` god-object wiring, echo-handler shape-preserving transport, singleton worker. Expected verdict: **candidate accepted under internal-ownership or external-queue conditions**.
- [x] **caddy** — lift TLS/ACME cert issuance or a module. Exercises: blank-import registry wiring, multi-implementer-by-design, caddy-handler signature, `init()`-based registration. Expected verdict: **candidate accepted for static registry subset; dynamic module loading deferred**.
- [x] **gitea** — lift mailer/notification. Exercises: `init()`-chain wiring, multiple `Sender` impls, concrete-struct annotation, routers-calling-models boundary blur. Expected verdict: **accept for mailer; document the router-to-models boundary as a refusal class**.
- [x] **mattermost** — lift `UserService.CreateUser` (websocket hub as stretch). Exercises: method-pragma with domain-object args, Options-builder wiring, auto-generated mocks, larger closure, `request.CTX` adapter needs. Expected verdict: **candidate accepted when `request.CTX` adapter metadata is supplied; websocket hub deferred with written rationale**.
- [x] **pocketbase** — **negative case, blocking**. Exercises: `core.App` 190-method god-object, SQLite embedding, lifecycle hooks. Expected verdict: **refuse with concrete, named refusal criteria**. This target defines the lower bound of v2; a vague "future work" verdict does not close the phase.
- [x] Assemble the **validation matrix**: rows = 7 axes, columns = 6 targets, cells = rule-id from the spec that applies. No empty cells.
- [x] For each target, record a one-line follow-up against `docs/evaluation/targets/NN-<name>.md` if validation surfaces missing architecture notes.
- [x] Budget: two full passes through Phase 9 are expected before close; each pass that invalidates a Phase 2–8 rule triggers a revision of that phase before the pass continues.

### Phase 10 — Alternatives rejected

- [x] Produce a **rejection-rationale table** with ≥8 rows, each row linked to audit or research-brief evidence via an explicit checkbox. Seed rows:
  - keeping interface-only annotation (contradicts §"Service interface is rare")
  - continuing `main`-walk extraction (contradicts §"Wiring doesn't live in main")
  - requiring stateless-only lifts (rules out 5/6 targets)
  - forcing HTTP/JSON on HTTP-handler lifts (pointless re-encoding)
  - custom IDL (violates pay-as-you-go compatibility promise)
  - mandatory gRPC (adoption tax, unnecessary for simple lifts)
  - full-transparency Waldo-style distribution (cited antipattern, research-brief §2)
  - actor-framework wholesale adoption (research-brief §2: lift ≠ actor)
- [x] Fold in any rows accumulated in the Phase 0 running rejection log.

### Phase 11 — Review & revision

- [x] Internal consistency pass: every normative term defined exactly once; every MUST/SHOULD is checkable.
- [x] Traceability pass: the decorative Phase 0 table was deleted; rule-level traceability now lives in the validation matrix and refusal index.
- [x] Target-coverage pass: validation matrix complete; pocketbase is a concrete refusal.
- [x] PLOS '25 alignment pass: each baseline claim marked preserved / revised / retired with rationale.
- [x] Refusal-rules pass: every refusal in the spec has a named compile-time diagnostic.
- [x] Waldo pass: spec nowhere claims remote-is-local; failure/cancel/deadline/panic/zero-value semantics present.
- [x] Review completed via merged AI fan-out plus Tim Goodwin manual audit; reviewer names recorded in the spec change log.
- [x] Convert reviewer comments to tracked checklist items in merged review artifacts and this closeout revision.
- [x] Integrate reviewer feedback. Category A contract changes landed; deferred research-narrative items remain out of v1.0.
- [x] Bump spec version to 1.0; update change log with the revisions that landed.
- [x] Final editorial pass: stable section anchors, no accidental implementation commitments, examples compile in the reader's head.
- [x] Extract **SPRINT-0005 implementation epics** into a handoff section at the bottom of this sprint file (not the spec). Epics only, no code tasks.

---

## Sequencing

Phase ordering: **0 → 1 → 2–8 → 9 → 10 → 11.**

Within 2–8, expect iteration. Annotation-surface decisions (Phase 2) constrain extraction root (Phase 3) and pragma surface syntax (which is sketched early in Phase 2 and finalized in Phase 8). Transport (5), dispatch (6), multi-impl (7), and pragma keys (8) form a coupled loop — plan on rewriting one after Phase 9 target validation.

Phase 9 is a **revision trigger**, not a terminal phase — failing any target validation sends work back to the relevant Phase 2–8 section. Reviewer feedback in Phase 11 can trigger a third Phase 9 pass.

### Gating rules

- [x] Do not close Phase 2 until every accepted surface has a Phase 3 extraction-root rule and a provisional pragma surface-syntax sketch.
- [x] Do not close Phase 3 until the closure report is named and the "closure too large" refusal diagnostic is defined.
- [x] Do not close Phase 4 until every state class has a disposition (replicated | singleton | affinity | externalize | refused).
- [x] Do not close Phase 5 until every canonical shape has a default transport or a refusal.
- [x] Do not close Phase 8 until every Phase 2–7 decision has a pragma representation **or a deliberately inferred rule** documented.
- [x] Do not close Phase 9 until pocketbase has a concrete written refusal verdict.
- [x] Do not close Phase 11 until reviewer feedback is integrated and, if any rule was invalidated, Phase 9 re-validated the affected targets.

## Risks and mitigations

- **Wishlist spec.** Mitigation: every normative rule must be validated against a target or carry a refusal. Phase 9 matrix must have no empty cells.
- **SSA closure absorbs the whole monolith.** Mitigation: explicit boundary-pruning rules and the "closure too large" refusal diagnostic (Phase 3).
- **State semantics drift toward a distributed-actor system (Orleans/Akka-scale).** Mitigation: bounded taxonomy in Phase 4; anything requiring consensus or cross-node shared mutable state is refused, not designed.
- **Shape-preserving + JSON-RPC produce conflicting semantics for the same lift.** Mitigation: deterministic canonical-shape classification ordering; each shape has exactly one default transport.
- **Transport-code explosion across heterogeneous signatures.** Mitigation: canonical shapes with shared adapter templates (Phase 5).
- **Dynamic dispatch inherits the unsolved transition-function problem.** Mitigation: Phase 6 scopes to per-lift policy; cross-lift orchestration deferred with rationale.
- **Multi-implementer rules too implicit for plugin systems.** Mitigation: explicit `impl=` / `registry=` syntax required whenever resolution is not provably unique.
- **Validation overfits to miniflux/listmonk (the easy targets).** Mitigation: all six targets required; pocketbase negative verdict is blocking.
- **Distribution hazards hidden behind local-looking calls (Waldo violation).** Mitigation: failure/cancel/deadline/panic/zero-value semantics specified in Phase 4; Waldo-pass in Phase 11.
- **Spec leaks implementation details.** Mitigation: closure report is the *only* sanctioned spec/implementation interface; all other compiler algorithms remain non-normative. Editorial pass in Phase 11.
- **Scope creep into implementation design.** Mitigation: any "how it works" material not needed to define the source contract goes to the SPRINT-0005 handoff, not the spec.
- **Reviewer feedback remains untracked or informal.** Mitigation: convert reviewer comments to tracked checklist items; re-run Phase 9 if feedback invalidates a rule.
- **Path / artifact churn.** Mitigation: the deliverable path is fixed at Phase 0; any change requires a sprint-notes rationale entry.

## Acceptance criteria

- [x] `docs/specs/monolift-v2-contract.md` exists as one versioned markdown file at version ≥ 1.0.
- [x] Each of the seven design axes is resolved in a named section with a normative rule and at least one worked example.
- [x] Validation matrix covers all six targets; every cell references a spec rule-id or a refusal.
- [x] miniflux and the internal-queue Listmonk candidate are accepted without requiring application-code rewrites beyond adding annotations; external-queue Listmonk remains a conditional variant.
- [x] pocketbase is documented as refused with concrete, named criteria (not hand-waved as "future work").
- [x] caddy, gitea, mattermost each have an accept/partial verdict with extraction-root, state, transport, and dispatch rules named.
- [x] PLOS '25 conceptual-model baseline has each claim marked preserved / revised / retired with rationale.
- [x] Rejection-rationale table contains ≥8 discarded alternatives, each linked to audit or research-brief evidence.
- [x] Every refusal has a named compile-time diagnostic.
- [x] Pragma grammar is given as EBNF with ≥8 worked examples covering every accepted dispatch mode.
- [x] Compatibility promise (unmodified `go build` still works) stated normatively and non-negotiable.
- [x] `clientgen.go:110` panic-on-unhandled-return is addressed at the spec level as a contract rule, not left as code-level behavior.
- [x] Spec reviewed by ≥1 compiler-facing and ≥1 systems/research-facing reviewer; reviewer names in change log; review comments tracked and integrated.
- [x] Sprint file ends with a SPRINT-0005 implementation-epics handoff section — epic-level only, no code tasks inside SPRINT-0003.

---

## Sprint Scratch Area

### Running Rejection Log

Rows captured here are folded into the formal rejection-rationale table during Phase 10.

| Alternative | Why it is being rejected | Evidence |
|---|---|---|
| Interface method pragma | No implementation body or independent production identity; use interface `methods=` or concrete method annotation. | Audit: service interface is rare; method bodies live on concrete structs. |
| Function-valued var pragma | Requires value-flow and reassignment semantics beyond v2 declaration roots. | Audit: heterogeneous shapes; research brief: keep annotation burden low without adding a new language. |
| Anonymous function pragma | No stable declaration name or deployable identity. | Research brief §0: annotations must remain lightweight and transparent. |
| Generic instantiation pragma | Instantiation expressions are not declaration roots; generic declaration lifting is deferred until closure reports model type substitution. | Audit: v2 must avoid guessing and refuse unsupported shapes. |
| Whole-package pragma | Packages do not match service boundaries and can absorb whole monoliths. | Audit item G: package != service boundary in 4/6 targets. |

---

## Follow-on sprints

### SPRINT-0004 — E2E test harness (prerequisite for compiler work)

Before v2 compiler implementation begins, build the e2e harness that coding agents iterate against. Uses a stub compiler so the harness can go green before any real compiler code is written. Plan: `docs/sprints/SPRINT-0004.md`. Strategy: `docs/specs/e2e-test-strategy.md`.

### SPRINT-0005+ — v2 compiler implementation epics

Each epic lands as its own sprint and flips one or more harness targets from red to green. Seed list:

- SSA-based extraction pass: compute the root-driven call/value closure and emit the v1.0 JSON closure report.
- Canonical-shape signature classifier: classify exposed operations and select per-shape adapter templates.
- State-class inference and singleton codegen: infer composite state facets and generate singleton or affinity placement where required.
- v2 pragma parser: implement doc-comment attachment, EBNF parsing, key validation, extension-key handling, and v1 migration warnings.
- Refusal-diagnostic framework: centralize named refusals and warnings with source spans, rule IDs, and remediation text.
- End-to-end miniflux smoke test: validate a feed-fetcher lift through parsing, extraction, classification, codegen, and refusal/report output.
