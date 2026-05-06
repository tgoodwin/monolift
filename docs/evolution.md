# Monolift — Design Evolution

A chronological narrative of how studying real-world Go monoliths reshaped
Monolift's compiler design. Each entry links to the ADR that formalizes the
decision and the evidence docs that motivated it.

The ADRs in `decisions/` are the primary sources; this document is the
reader's entry point and tells the story that no single ADR can.

---

## 2026-05-05 — Cut-placement analyzer: where the network boundary goes

SPRINT-0039 established the research question — given an activation path from
`main()` to a lift target, where should the compiler introduce the network
boundary? — and answered it empirically across 72 traces from 6 codebases.
Six scoring dimensions (extraction surface area, boundary-data complexity,
state reconstruction cost, callback frequency, error semantics, edge-type
alignment) and a decision-tree ranking emerged from the data. The strongest
finding: deep cuts dominate (mean recommended depth 0.924, median 1.0).

SPRINT-0040 implemented the analyzer (`AnalyzeCut` in `pkg/activation/`).
Corpus-driven iteration surfaced three design corrections that would not have
been visible from the research alone:

- **Surface area must rank first among soft dimensions.** The initial
  implementation ranked callbacks first, causing shallow bootstrap functions
  (stateless, zero callbacks, VeryLarge surface) to beat deep service
  functions. The corpus showed this immediately — PocketBase picked
  `log.Fatal` at step 1 for all 11 traces.

- **Receivers are state, not boundary data.** Classifying receiver types
  as boundary data hard-gated most Mattermost candidates as infeasible
  (the `*App` struct contains mutexes and func fields). But the receiver
  is reconstructed on the remote side, not serialized across the wire.
  Excluding it from boundary-data classification and handling it under
  state reconstruction eliminated the false rejections.

- **Stdlib and framework internals are not lift candidates.** The
  activation path traverses functions from `fmt`, `net/http`, `protobuf`,
  and other dependencies that have simple boundary data but are never the
  code a developer wants to extract. Filtering to the project's Go module
  eliminated these false positives.

The evaluation taxonomy revealed four categories of divergence between the
automated analyzer and human judgment: step-numbering misalignment (13 cases,
innocuous — same function, different path structure), known-type refinements
needed (18 cases, fixable with type-walker overrides), proxy preference
design questions (6 cases, whether to accept HTTP streaming proxies for
middleware targets), and legitimate algorithmic differences (18 cases, the
analyzer optimizes a different dimension than the human reviewer prioritized).

**Primary artifacts:** [`cut-placement-brief.md`](research/activation-paths/cut-placement-brief.md) · [`cut-placement-synthesis.md`](research/activation-paths/cut-placement-synthesis.md) · [`cut-placement-evaluation.md`](research/activation-paths/cut-placement-evaluation.md) · `pkg/activation/cut.go` · `docs/sprints/SPRINT-0039.md` · `docs/sprints/SPRINT-0040.md`

---

## 2026-04-22 — Classifier-test performance + callgraph reuse

SPRINT-0010-CLASSIFIER-PERF landed the two test-memory fixes deferred from SPRINT-0009 (`shape.test` at 12 GB RSS; `extract.test` OOM on the PocketBase corpus lane) and built the verification substrate that unblocks every future perf-sensitive change.

- **Fix 3 — shape-test SSA sharing.** `pkg/compiler/shape/shape_test.go`'s `classifyFixture` / `classifyFixtureForExtract` helpers now back onto a `sync.Once`-guarded shared `*extract.LoadedModule` + `*ssa.Program` + liftability `Context` mirroring the `pkg/compiler/liftability/test_helpers_test.go` pattern. Measured result: worst-run peak RSS on `go test ./pkg/compiler/shape` dropped from 1820 MB (killed baseline) to **635 MB (−65.1%)** with 7.3% spread across seeds 101/202/303.
- **Fix 4 — callgraph reuse on the fast path.** `liftability.NewContext` now accepts a pre-built `*callgraph.Graph`; `extract.buildProgram` flows the CHA graph downstream; the registry-keyed RTA closure path is rationalized to avoid a third build. A new structural-invariant test in `pkg/compiler/extract/` fails if callgraph construction fires more than once per `*ssa.Program` per pass — direct assertion of Fix 4's claim, with RSS as downstream evidence.
- **Verification harness.** `cmd/memcheck/main.go` + `test/memcheck/{schema.md,run.sh,README.md,_kill_smoketest/}` + Makefile `perf-rss-{shape,pocketbase,pkg}` targets. Whole-process-tree peak-RSS polling, whole-tree SIGKILL-on-budget-trip, per-tick JSON flush, five-state `summary.status` (`working | regressed | accepted | killed_rss | killed_time`), fixed shuffle seeds `101/202/303`, cold-cache per run, worst-run gating, `spread_pct ≤ 10` stability gate. All three baselines (shape/pocketbase/full) are committed under `test/memcheck/`.

Full-suite acceptance is deferred to SPRINT-0010-GOLDENS. Two integration tests (`TestExtractCaddyReverseProxyProducesNonEmptyValidatedReport`, `TestAnalyzeDetectsPocketBaseRefusals`) now fail because of the same downstream diagnostic-duplication bug — every `MLV2_*` diagnostic is emitted twice. The Caddy test's stale expectations are genuine golden-drift from the SPRINT-0009 reframe (the new classifier correctly refuses reverseproxy's closure over `sync.Mutex`/`sync.Once`/`sync/atomic.*`/channels/function values/`unsafe.Pointer`). Both routed to SPRINT-0010-GOLDENS with grounded diagnostic captures.

**Primary artifacts:** `cmd/memcheck/main.go` · `test/memcheck/` · `pkg/compiler/shape/shape_test.go` · `pkg/compiler/extract/{ssa,closure}.go` · `pkg/compiler/liftability/detector.go` · `docs/sprints/SPRINT-0010-CLASSIFIER-PERF.md`

---

## 2026-04-22 — Liftability-first classifier lands

SPRINT-0009 moved admissibility off literal canonical-shape matching and onto
named liftability properties with structured evidence. The compiler now writes
root-level admission verdicts and property evidence into `reportv2`, keeps the
existing `MLV2_*` refusal taxonomy, and uses canonical shapes only as
downstream transport-selection outputs for reports, pragmas, and adapters.
ADR-0017 records the admission/transport split; ADR-0018 freezes the named
property set and IDs.

---

## 2026-04-21 — SPRINT-0007 closed: canonical shape + state inference

SPRINT-0007 split semantic interpretation out of SSA extraction into two
dedicated passes: `pkg/compiler/shape/` classifies lifted roots into canonical
shapes and default transports, while `pkg/compiler/stateclass/` infers captured
state classes from SSA evidence and developer declarations. The compiler now
records `root.shape` and `root.defaultTransport`, validates shape-aware pragma
options after parsing, and derives adapters from classifier output instead of
the retired `ServeHTTP` suffix heuristic.

The sprint also removed the off-spec `registry-keyed-module` adapter label,
deleted `pkg/compiler/extract/pocketbase.go`, fixed the no-filter
`resolveExposedOperations` bug, and replaced the synthetic Pocketbase
`BaseApp.db` refusal with per-field rows discovered from the corpus. Two new
pragma fixtures now exercise the new refusal paths, and ADR-0015 plus ADR-0016
record the classifier and state-inference decisions that make SPRINT-0008
adapter code generation possible.

**Primary artifacts:** `pkg/compiler/shape/` · `pkg/compiler/stateclass/` ·
`pkg/compiler/extract/extract.go` ·
`test/e2e/targets/caddy/golden/report.json` ·
`test/e2e/targets/pocketbase/golden/report.json` · ADR-0015 · ADR-0016

---

## 2026-04-19 — Returning to the project after 9 months

After ~9 months away from the project, we took stock: the v1 compiler was
developed against a greenfield demo at `demo/monolith/`, and the runtime
controller at `pkg/pragma/controller.go` was known to be incomplete. Before
finishing the runtime, a more fundamental question needed an answer: would
the v1 compiler even apply to real Go monoliths?

**Primary artifact:** `docs/codebase-state-2026-04-19.md` — research report
re-orienting to the codebase after the gap. Established that ~85% of the
compiler's happy path works but key runtime state-machine logic is stubbed.

---

## 2026-04-19 — Adopting the six-target evaluation corpus · [ADR-0001](decisions/0001-evaluation-corpus.md)

The obsidian "Monolift" doc listed six candidate Go monoliths for evaluation:
gitea, mattermost, caddy, listmonk, pocketbase, miniflux. We pulled them down
locally as a structured corpus — `evaluation/` for clones (gitignored),
`docs/evaluation/` for the committed semantic index, `MANIFEST.yaml` pinning
SHAs for reproducibility. The pattern mirrors the pre-existing `inspiration/`
research corpus.

This wasn't yet a decision about Monolift's design — it was the *apparatus*
for making future decisions evidence-backed instead of intuition-backed.

**Primary artifact:** `docs/evaluation/README.md` + per-target dossiers.

---

## 2026-04-19 — The generalization audit · context for [ADR-0002](decisions/0002-renegotiate-contract-v2.md)

Parallel Explore agents audited each of the six targets against Monolift's v1
input contract — eight dimensions: interface-annotated services, unique
implementer, `New<Iface>` constructor, wiring-in-`main`, `(ctx, req) →
(resp, err)` method shape, statelessness, package-as-service, single main.

Result: v1 fits **1–4 out of 8** on every target. The demo at `demo/monolith/`
is the only app that fits because it was written to the compiler's
conventions, not the other way around.

Five universal failures emerged, each documented in the audit:

1. Wiring never lives in `main` (6/6) — real patterns are `init()` chains,
   Options builders, plugin registries, lifecycle hooks, god-App structs.
2. Business logic lives on concrete structs, not interfaces.
3. Multi-implementer is common (plugin systems, OAuth providers, mocks).
4. Method shapes are heterogeneous.
5. Every target holds in-process state.

**Primary artifact:** `docs/evaluation/generalization-analysis-2026-04-19.md`.

---

## 2026-04-19 — Renegotiating the contract · [ADR-0002](decisions/0002-renegotiate-contract-v2.md)

The audit made it obvious: patching v1 indefinitely would never converge on
real-world applicability, because the v1 *contract itself* was wrong, not its
implementation. We committed to a v2 specification as the next milestone —
code freeze on the compiler until the contract is resolved.

The sprint to produce the v2 spec used a multi-model planning pattern
(codex / gemini / claude drafts + Opus merge; see `docs/sprints/SPRINT-0003.md`),
with the generalization analysis as the primary input. The spec itself is
being executed at `docs/specs/monolift-v2-contract.md`.

Sub-decisions inside v2 became ADR-0003 through ADR-0008. The framing for how
the v2 spec relates to the PLOS '25 paper — preserve / revise / retire —
became [ADR-0009](decisions/0009-plos-claims-preserve-revise-retire.md).

---

## 2026-04-19 — Extraction root: retire `main`-walk, adopt call-graph · [ADR-0003](decisions/0003-extraction-root-call-graph.md)

The single biggest lever from the audit. v1 reconstructs the dependency graph
by walking variable declarations in `func main()`; this found nothing in 6/6
real targets. v2 uses call-graph / SSA transitive closure from the annotated
root — indifferent to whether wiring lives in `init()`, Options, plugin
registries, or hooks.

This introduced the **closure report** as the one sanctioned interface
between spec and implementation: what symbols are included, what state is
captured, what external deps, what refusals. It's also the thing that makes
the spec implementable: you can't hand-wave extraction if the spec mandates a
concrete output artifact.

---

## 2026-04-19 — Annotation surface: generalize beyond interfaces · [ADR-0004](decisions/0004-annotation-surface-generalized.md)

The audit showed Go monoliths use concrete structs and package-level
functions far more than interfaces. v2 accepts pragmas on interface decls,
function decls, methods with concrete receivers, and struct types. Interfaces
aren't privileged anymore.

This required generalizing the pragma parser and the compiler's "what's being
lifted?" entry point. It also demoted v1's unique-implementer assumption — see
the spec's §Multi-implementer handling.

---

## 2026-04-19 — State semantics: bounded taxonomy, not stateless-only · [ADR-0005](decisions/0005-state-semantics-bounded-taxonomy.md)

Every target holds in-process state. Refusing to lift it rules out 5/6
targets; lifting it naively produces broken distributed systems (replicated
WebSocket hubs don't work). v2 adopts a seven-class state taxonomy:
stateless, immutable config, externalized durable, process-local cache,
singleton mutable, connection/session state, and shared-mutable-across-
unrelated-callers (the last of which is refused).

The compiler can either infer class from the closure or accept an explicit
declaration. Failure/cancellation/deadline semantics are specified at the
contract level — v2 does *not* pretend remote is local (honoring Waldo).

---

## 2026-04-19 — Canonical shapes for transport · [ADR-0006](decisions/0006-canonical-shapes-transport.md)

Method shapes in the wild are heterogeneous. A naive per-shape adapter would
explode the compiler's template surface; a strict one-shape rule rules out
most targets. The v2 compromise — surfaced originally in Gemini's sprint
draft, kept through the Opus merge — is a small bounded set of canonical
shapes (RPC req/resp, multi-domain-arg, no-response, HTTP handler, channel
consumer, builder chain), each with a default transport or an explicit
refusal. Classification is deterministic so no method matches two shapes.

This replaces v1's `panic()` at `pkg/lift/clientgen.go:110` with a
contract-level refusal diagnostic.

---

## 2026-04-19 — Shape-preserving transport for HTTP handlers · [ADR-0007](decisions/0007-shape-preserving-transport.md)

The natural consequence of one canonical shape — HTTP handlers. If the lift
root is already HTTP-shaped (echo, caddy middleware, `http.Handler`), forcing
it through a JSON/gRPC wrapper doubles the round-trip and loses
header/streaming fidelity. v2 preserves the HTTP shape across the boundary:
the lifted deployable is still an HTTP handler, the lift point forwards the
request. One transport hop, not two.

---

## 2026-04-19 — Pocketbase as intentional refusal · [ADR-0008](decisions/0008-pocketbase-negative-case.md)

The audit's hardest case. Pocketbase's `core.App` is a 190-method god object
with embedded SQLite and bootstrap-time-only configuration. Lifting any piece
requires refactoring first; that's a user problem, not a compiler problem. We
documented four named refusal criteria (god-object interface, embedded
persistent state, bootstrap-only config, monolithic lifecycle coupling) — the
concrete lower bound of v2.

Pocketbase's role in the evaluation matrix is to define what v2 *won't* do,
grounded in named diagnostics. That definition is blocking for the spec's
Phase 9 close — no vague "future work" verdict allowed.

---

## 2026-04-19 — PLOS '25 alignment: preserve / revise / retire · [ADR-0009](decisions/0009-plos-claims-preserve-revise-retire.md)

The paper's conceptual model needed a principled relation to v2 — not
preserved wholesale, not retired wholesale. Each load-bearing claim gets
tagged: the pay-as-you-go compatibility promise is preserved, statelessness
is revised, main-walk wiring is retired, and so on. Every change from paper
to spec is annotated with a rationale in the spec; no silent retirements.

This framing is useful infrastructure for a PLOS follow-up narrative that
tells the story of how evaluation evidence forced model revision.

---

## 2026-04-19 — Fan-out review, Category A/B triage, v1.0 landing · [ADR-0010](decisions/0010-spec-review-triage.md)

Phase 11 of SPRINT-0003 needed compiler-facing and systems/research-facing
reviewers, but sourcing external humans is slow. We extended the sprint-planner
multi-model fan-out pattern to reviewing: two parallel tracks (compiler /
systems), each with three drafts (codex/claude/gemini), cross-critiques, and
Opus-merged review docs. Tim's own manual audit rounded out the compiler lens.
Outputs landed at `docs/specs/reviews/compiler-review.md` and `systems-review.md`.

The merged reviews produced ~25 blocking items. Rather than either accept-all
or defer-all, we adopted a triage discipline: **Category A** (contract-affecting
— blocks the next implementation sprint) vs **Category B** (research-narrative
— blocks paper, not implementation). Option 1a = Category A + three editorial honesty edits
(verdict downgrades where targets required code changes; replace the
self-congratulatory Validation Pass Log; prune the decorative Traceability
Table).

Codex applied the 19 Option-1a edits. The spec is now v1.0 with status
`accepted`. Notable contract-affecting edits: JSON-schema appendix for the
closure report; EC-WIRE-4 wiring-pattern whitelist replacing "cannot be
represented deterministically"; AS-FUNC-4 refusing non-enumerable generic
declarations; SS-DISP-1 moved to facet-classification (fixing an internal
contradiction with the Mattermost WebHub row); TA-SER-3 now refuses mutable
pointer aliases with `MLV2_POINTER_ALIAS_UNSUPPORTED`; EC-WIRE distinguished
source-inclusion termination from state/effect termination; new rule block
for Remote Call Outcomes (success / maybe-executed / completed-but-reply-lost /
timeout / panic); first-class Adapter Metadata section.

ADRs 0003–0007 moved from `proposed` to `accepted` now that v1.0 is the
reference implementation of those decisions. SPRINT-0003 closed (status
`done`, all 103 checkboxes checked). Follow-on-sprints section seeds six
compiler implementation epics at the bottom of the sprint file.

**Primary artifacts:** `docs/specs/monolift-v2-contract.md` v1.0 · `docs/specs/reviews/{compiler,systems}-review.md` · ADR-0010

---

## 2026-04-19 — E2E test strategy + harness-before-compiler discipline · [ADR-0011](decisions/0011-harness-before-compiler.md)

Before SPRINT-0005+ compiler implementation epics begin, we recognized that
coding agents need a concrete feedback loop to iterate against. Wrote an
e2e test strategy (`docs/specs/e2e-test-strategy.md`) through the same
multi-model fan-out pattern used for the spec (3 drafts from codex/claude/
gemini → 3 cross-critiques → Opus merge). Merge picked Caddy as the first
positive target (no DB, shortest path), Pocketbase as the refusal case
(asserts both `MLV2_EMBEDDED_DB_APP_ROOT` + `MLV2_CLOSURE_TOO_LARGE`), and
Miniflux as second positive (Postgres-backed; stretch for the harness sprint).

The harness ships **before** the real v2 compiler: it runs against a stub
compiler that emits hard-coded golden closure reports matching the shape the
real v2 compiler will eventually emit. As each SPRINT-0005+ compiler epic
lands, it replaces the stub target-by-target until the stub is deleted.
This is now a process invariant for the project — ADR-0011.

Also refactored the sprint-planner skill's ledger to be per-project:
`ledger.py` now anchors on the nearest `.git` ancestor of CWD, so each
project keeps its own `docs/sprints/ledger.yaml` with an independent
counter. (Previously the ledger was accidentally user-home-scoped, which
put Monolift and Tractor sprints into one sequence with gaps — confusing.)
Registered SPRINT-0004 for the harness sprint; SPRINT-0005+ for the
compiler-epic sequence.

**Primary artifacts:** `docs/specs/e2e-test-strategy.md` v1.0 · `docs/sprints/SPRINT-0004.md` · ADR-0011 · per-project ledger at `docs/sprints/ledger.yaml`

---

## 2026-04-19 — SPRINT-0004 closed: e2e harness live

Codex executed SPRINT-0004 end-to-end against the plan: 89 / 89 checkboxes
green, no blockers. `test/e2e/` now holds a working Go harness that runs
`make e2e` in ~50s against a local Kind cluster. Caddy exercises the full
10-stage pipeline (baseline deploy → compile → report-assert → image build
→ kind load → lifted deploy → workload → compare), Pocketbase asserts both
expected refusal diagnostics (stages 0, 3, 4 only), and Miniflux / Listmonk
/ Gitea / Mattermost cleanly skip with SPRINT-0005 pointers. The stub
compiler driver at `test/e2e/e2ecompile/` emits hard-coded golden closure
reports so the harness is green before any real v2 compiler code exists.
Smoke run confirmed: `TestE2E` passes, `TestE2ECompileTargetsValidate`
passes, cluster-lifecycle test is opt-in.

The harness carries failure messages with `[stage=N target=X kind=...]`
prefixes so SPRINT-0005+ agents can triage compiler-vs-harness-vs-artifact
failures without reading logs.

**Primary artifacts:** `test/e2e/` · `pkg/compiler/reportv2/` · `make e2e`

---

## 2026-04-20 — SPRINT-0006 closed: SSA extraction + refusal framework flip

SPRINT-0006 replaced the remaining fixture-backed compiler reports for Caddy
and Pocketbase with a real SSA-backed extraction path in `pkg/compiler`.
`compiler.Extract` now consumes SPRINT-0005 parser output, loads the annotated
module with `packages.LoadAllSyntax`, builds SSA, walks a deterministic closure,
records external dependencies, and emits CHA-based analysis with RTA refinement
for registry-keyed roots.

The Caddy target now passes stages 0–10 against real compiler output with the
report fixture retired. The Pocketbase target now passes stage 4 against real
compiler refusals, reproducing `MLV2_EMBEDDED_DB_APP_ROOT`,
`MLV2_CLOSURE_TOO_LARGE`, and the compatibility `BaseApp.db=refused` state row
without relying on a fixture report.

The `compiler.Diagnostic -> reportv2.Diagnostic` seam moved out of
`test/e2e/e2ecompile/main.go` into the new `pkg/compiler/diagnostics`
package, which now owns rule IDs, remediation text, UTF-8-aware byte-offset
formatting, unknown-code failure, and the import-boundary guard.

ADR-0013 records the shipped SSA precision policy: CHA as the default dispatch
approximation, RTA refinement for registry-keyed roots, and explicit deferral
of VTA/pointer analysis pending a measured follow-up. ADR-0014 records the
unbounded-edge refusal taxonomy, including `MLV2_CLOSURE_UNBOUNDED` as the
non-dispatch umbrella alongside the dispatch-specific reflection and plugin
codes.

**Primary artifacts:** `pkg/compiler/extract/` · `pkg/compiler/diagnostics/` ·
`test/e2e/targets/caddy/golden/report.json` ·
`test/e2e/targets/pocketbase/golden/report.json` · ADR-0013 · ADR-0014

---

## 2026-04-20 — SPRINT-0005 closed: v2 pragma parser + harness seam

SPRINT-0005 replaced the v1 parser in `pkg/compiler/pragma.go` in place with
the v2 pragma grammar from the contract. The parser now recognizes the four
annotation surfaces, validates surface-specific keys, emits parser-internal
`MLV2_PRAGMA_*` diagnostics, honors `x-*` extension keys, and treats v1 forms
as warning-only migration diagnostics with rewrite suggestions.

The e2e harness now includes seven source-only pragma micro-fixtures, one for
each `MLV2_PRAGMA_*` code. `test/e2e/e2ecompile/` preserves existing
fixture-copy behavior for Caddy/Pocketbase/Miniflux, but falls through to the
real parser for pragma source directories and translates parser diagnostics
to `reportv2.Diagnostic` at that test seam only. ADR-0012 records the
parser-internal diagnostic boundary, required-key mapping, duplicate-key
decision, and import-boundary guard.

The old v1 demo parser path is intentionally broken by the in-place
replacement; restoration or retirement is deferred to SPRINT-0006+.

**Primary artifacts:** `pkg/compiler/pragma.go` · `pkg/compiler/pragma_keys.go` · `test/e2e/targets/pragma/` · ADR-0012

---

## SPRINT-0008: Educational static site (2026-04-21)

SPRINT-0008 shipped the advisor-facing design-story site at
`https://tgoodwin.github.io/monolift/`, an MkDocs + Material build rooted
at `docs/site/` with `mkdocs.yml` at the repo root. Six pages: index,
four load-bearing design-story pages (canonical shapes, state-class
inference, refusal diagnostics, v1→v2 arc), and a single reading-guide
appendix. Each load-bearing page follows a fixed "one paragraph + mermaid
+ side-by-side Monolift/corpus pairing + ≤3-sentence why" structure;
ADRs are linked, never restated.

Snippet discipline is marker-based: every `pkg/compiler/**` reference is
bracketed by `// site:begin NAME` / `// site:end NAME` comments and
drift-checked against the Markdown include's line range in CI. Vendored
external excerpts live under `docs/site/snippets/external/` with
provenance headers naming the upstream repo, pinned SHA from
`evaluation/MANIFEST.yaml`, path, line range, SPDX license, and fetch
date; `scripts/refresh-external-snippets.py` regenerates them from a
cached bare mirror independent of the gitignored `evaluation/` clones.
A single `.github/workflows/docs-site.yml` enforces policy, drift,
strict build, and internal linkcheck on every PR, and deploys to Pages
on pushes to `main` with `concurrency: { group: pages, cancel-in-progress: false }`.

The site is tooling around the decision log, not a decision about the
Monolift system itself, so no ADR was added for it; this chronological
entry is the only record. ADR-0016 was tightened during this sprint to
enumerate all six precedence rules explicitly and to frame the composite
embedded-DB post-pass and the `MLV2_STATE_UNKNOWN` ambiguity fallback as
distinct from the precedence stack.

**Primary artifacts:** `mkdocs.yml` · `docs/site/` · `.github/workflows/docs-site.yml` · `scripts/{check-docs-policy,check-snippet-drift,fix-snippet-drift,refresh-external-snippets}.py`

---

## 2026-04-23 — Composite-archetype regions: candidate-set classification · [ADR-0022](decisions/0022-composite-archetype-regions.md)

**Context.** SPRINT-0013's v1 archetype vocabulary (8 archetypes) is a set of
*overlapping lenses* on the region space, not a partition. Multiple corpus
regions match more than one archetype simultaneously — caddy `Handler.connections`
matches `serialized-actor` + `keyed-partitioned-state`; mattermost's websocket
hub (MM1+MM2) matches `keyed-partitioned-state` + `fanout-publisher` +
`session-affinity-state`. SPRINT-0013 flagged ADR-0022 as "ripe to draft" but
deferred the decision. SPRINT-0015's utility analysis elevated it to load-bearing
for the PLOS §4.2 demo: the mattermost hub composite is the single strongest
thesis-demonstration region in the corpus, and a compiler that can only emit
single-archetype transforms cannot demonstrate it.

**Decision.** Composite-archetype classification is candidate-set construction
plus candidate selection, not forced single-label assignment. The classifier
produces a match set per region; the compiler projects that set into a primary
candidate plus alternative and composite candidates. Precedence is computed via
**region-relative subsumption** (A subsumes B iff A's transform invariants are a
strict superset of B's on the same region), rejecting a global archetype ladder.
Composite candidates emit when all components independently match AUTO, the
composite passes a **compatible-refinement coherence check**, and a concrete
emission sketch exists. Composite identity is **compositional** (contributing-archetype
list plus region), not nominal — named aliases like `connection-hub-buffer` are
informal reporting conveniences, not catalog additions. Dynamic-delegate
eligibility on composites inherits by **AND over contributing archetypes**. The
report format exposes the candidate set with orthogonal fields for *candidate
exists*, *candidate is emittable*, and *candidate participates in runtime
selection*.

**Committee drafting.** ADR-0022 was drafted via three-way committee (opus +
gpt-5.4 + gemini), cross-critique, and opus synthesis — the same pattern that
produced the SPRINT-0013 and SPRINT-0015 composite research notes. Committee
drafts and critiques preserved at `docs/sprints/drafts/SPRINT-0016-*.md`.

**Primary artifacts:** `docs/decisions/0022-composite-archetype-regions.md` · `docs/sprints/SPRINT-0016-BRIEF.md` · `docs/sprints/SPRINT-0016.md` · `docs/sprints/drafts/SPRINT-0016-*.md`

---

## 2026-04-25 — ADR-0022 vertical slice: Caddy actor alternative set · [ADR-0022](decisions/0022-composite-archetype-regions.md)

SPRINT-0017 lands the first end-to-end ADR-0022 slice on Caddy
`Handler.connections`. The compiler now builds candidate sets for
`serialized-actor` and `keyed-partitioned-state`, reduces them by
subsumption plus utility-tier fallback, reports `archetype_kind:
"alternative_set"` with a tier-tagged alternative rationale, and emits a
descriptive `actor` adapter for the selected `serialized-actor` primary.

ADR-0022 now includes a 2026-04 clarification that `archetype_kind` describes
how the set was reduced: `single` for subsumption-decided, `alternative_set`
for incomparable plus tier-decided, and `composite` for emitted composites.

**Primary artifacts:** `pkg/compiler/stateclass/{archetype,candidates,subsumption,tiers,selection}.go` · `pkg/compiler/extract/extract.go` · `pkg/compiler/reportv2/report.go` · `test/e2e/targets/caddy/golden/report.json`

---

## 2026-04-26 — Real-symbol sidecar execution: Caddy CleanPath · [ADR-0023](decisions/0023-sidecar-emission-and-real-symbol-execution.md)

SPRINT-0018 lands the first extract-to-sidecar execution slice. The compiler
admits a basic synchronous boundary, emits an HTTP/JSON extracted service that
imports and calls the real `caddyhttp.CleanPath` symbol, and builds a lifted
Caddy image from a patched copy of the host source tree.

The host patch prepends an AST-generated env-gated prelude to `CleanPath` while
keeping all imports in a generated sibling file. The lifted Caddy deployment can
run lifted or unlifted from the same image, records structured invocation data,
and verifies every remote result against an in-process oracle. The e2e harness
now checks per-request counter deltas, aggregate bounds, `/invocations` oracle
equality, extracted-service logs, transcript parity, env-off zero calls,
fail-closed 404 behavior, and fail-open 200 behavior.

The slice is intentionally narrow: `CleanPath` has no receiver state and a
simple `(string, bool) -> string` signature. ADR-0023 records the mechanism
tradeoffs and the next pressure points: receiver-bearing symbols, `internal/`
import legality, and non-Caddy host layouts.

**Primary artifacts:** `pkg/compiler/transport/admission.go` · `pkg/compiler/transport/emit/` · `test/e2e/e2ecompile/main.go` · `test/e2e/e2e_test.go` · `docs/decisions/0023-sidecar-emission-and-real-symbol-execution.md`

---

## 2026-04-26 — Internal-symbol lift via cmd-inside-host emission · [ADR-0023](decisions/0023-sidecar-emission-and-real-symbol-execution.md)

SPRINT-0019 removes the separate-module extracted-service layout from
SPRINT-0018 and emits extracted binaries as `cmd/monolift-extracted-*`
packages inside the patched host module. The Caddy proof now runs two
extracted-service pods in parallel from one shared `host-patch/` tree:
`caddyhttp.CleanPath` remains the exported-symbol regression target, while
`internal/metrics.SanitizeMethod` proves Go `internal/` imports are legal under
cmd-inside-host emission. The e2e harness verifies both pods with per-request
counter deltas, aggregate bounds, oracle equality over `/invocations`,
transcript parity, env-off zero calls, static dormant-env checks, runtime
single-increment recursion checks, and per-symbol fail-mode behavior.

**Primary artifacts:** `pkg/compiler/transport/emit/httpjson/` · `test/e2e/e2ecompile/main.go` · `test/e2e/e2e_test.go` · `test/e2e/targets/caddy/` · `docs/decisions/0023-sidecar-emission-and-real-symbol-execution.md`

---

## 2026-04-26 — Miniflux real-compiler lift and e2e compile driver rename · [ADR-0023](decisions/0023-sidecar-emission-and-real-symbol-execution.md)

SPRINT-0020 moves miniflux off the legacy generated-output fixture path and
onto the real compiler. The proof lifts
`internal/reader/readingtime.EstimateReadingTime(string, int, int) int`,
verifying `int` result rendering, type-aware fail-closed sentinel behavior,
cmd-inside-host oracle binaries for internal-package legality, per-request
counter deltas, oracle equality, transcript parity, env-off behavior, and
fail-open/fail-closed recovery.

The old fixture-copy branch and generated fixtures are gone. The test compile
driver was renamed atomically to `bin/e2e-compile` with source under
`test/e2e/e2ecompile/`; the scope-cut fallback was not used.

**Primary artifacts:** `pkg/compiler/extract_transport.go` · `pkg/compiler/transport/emit/httpjson/` · `pkg/compiler/transport/emit/liftpatch/` · `test/e2e/e2ecompile/main.go` · `test/e2e/e2e_test.go` · `test/e2e/targets/miniflux/` · `docs/decisions/0023-sidecar-emission-and-real-symbol-execution.md`

---

## 2026-04-27 — Mattermost composite probe surfaces multi-root region gap · branch (C)

SPRINT-0021 attempted the ADR-0022 composite vertical slice on Mattermost Hub/WebConn. A.1 closure analysis succeeded under budget (88s wall, 2.15GB max RSS, 4.1GB Go heap; 2956 included / 4838 excluded symbols). A.2 region pinning surfaced the cliff: the intended composite boundary is genuinely multi-root — Hub fanout/index ownership at one root, per-connection write-pump/replay state at another — and the current closure model accepts one pragma root per region. `WebConn.writePump` does not appear under a Hub-rooted closure; field-level state (`send`, `deadQueue`, `Sequence`, `connectionID`) is not modeled as closure symbols at all.

The sprint stopped on branch (C) per design. The cliff doc captures reproduction steps, resource numbers, observed-vs-intended boundary diff, and a follow-up shape: first-class multi-root region declaration + closure union with per-root provenance + stateclass evidence aggregation across the union + report schema for contributing roots distinct from contributing archetypes. The framing is generalizable, not Mattermost-specific.

SPRINT-0022 implemented that multi-root analysis path and landed branch (R). Shared-name pragmas now regroup into one region; the Mattermost overlay declares Hub and WebConn as peer roots without touching `evaluation/mattermost/`. The union closure includes `(*WebConn).writePump`, every union symbol carries sorted reachability provenance, and SSA seam detection records the `WebConn.send` channel seam as Hub writers to WebConn readers. Admission accepts in-region channel seams under the single-service emission hypothesis. Emission stops at a characterized tooling gap: the current liftpatch API patches one free function per request and cannot replace multiple receiver methods across the Hub/WebConn root set with one extracted service.

ADR-0022 specified composite *archetypes* but tacitly assumed one region = one root. Mattermost surfaced that the region model itself is the missing piece — that's the research output.

**Primary artifacts:** `docs/research/runs/SPRINT-0021-region-granularity.md` · `test/e2e/e2ecompile/main.go` (additive: Mattermost packageDirFor + synthetic Hub pragma helper) · `test/e2e/targets/mattermost/target.go` (SkipReason updated)

---

## 2026-04-27 — Boot-path, RegionPatchRequest, and stream-proxy machinery · branch (R)

SPRINT-0023 lands the next tranche of Mattermost-forcing machinery: surface
derivation, additive `RegionPatchRequest`, a bounded boot-path pass, config
manifest rendering, and the stream-proxy emitter/test harness for session
surfaces. The Mattermost boot-path probe completed under budget at 55.41s wall
and 2.35GB max RSS with the required Mattermost `GOWORK`.

Mattermost lands branch (R), not (S). The new machinery passes toy stream-proxy
and multi-root fixtures, but Mattermost still needs route-to-region surface
derivation and true reverse boot-chain reconstruction before host/extracted
artifacts can be emitted honestly. The characterized gaps are tooling
immaturity, not fundamental distribution blockers.

**Primary artifacts:** `pkg/compiler/surface/` · `pkg/compiler/extract/bootpath/` · `pkg/compiler/transport/emit/liftpatch/` · `pkg/compiler/transport/emit/manifest/` · `pkg/compiler/transport/emit/streamproxy/` · `docs/research/runs/SPRINT-0023-mattermost-attempt.md` · `docs/decisions/0026-bootpath-extraction.md` · `docs/decisions/0027-region-patch-request.md`

---

## 2026-04-27 — EntryPath invocation probe stops at gate-A

SPRINT-0024 added the `pkg/compiler/entrypath` probe package, a debug
`cmd/entrypath-probe` binary, toy fixtures for reverse reachability and
function-value flow, and e2e harness wiring that can request a Mattermost probe
artifact without consuming it. The Mattermost run did not reach chain-quality
checks: with the required Mattermost `GOWORK`, the probe exceeded the 60s
gate-A wall-clock budget before emitting JSON and was killed. Without that
workspace, package loading fails earlier due the known local `server/public`
module skew.

No Phase 2 reportv2 or surface consumer work landed. The next probe should
separate package load, SSA build, RTA/VTA, reverse BFS, and function-value walk
timing, then narrow function-value propagation to reverse-path and HTTP-sink
candidate starts instead of every indexed function value.

**Primary artifacts:** `pkg/compiler/entrypath/` · `cmd/entrypath-probe/` · `docs/research/runs/SPRINT-0024-mattermost-probe.md`

---

## 2026-04-27 — EntryPath search matrix redirects Mattermost work to seeded diagnostics

SPRINT-0025 turned the broad EntryPath probe into measured search modes. The
full `all` index can recover `connectWebSocket` and the
`APIHandlerTrustRequester` chain, but only at whole-program scale. Reverse-path
mode is cheap and bounded but too narrow; current HTTP-sink and targeted modes
spend their budgets in seed discovery before indexed flow reaches Mattermost
evidence.

The next sprint should be another diagnostic, not report wiring: make
HTTP-shaped seed discovery incremental from reverse-path owners and nearby
callgraph structure, then require recovery inside a split gate for baseline
loader/SSA/callgraph cost and incremental EntryPath cost.

**Primary artifacts:** `docs/research/runs/SPRINT-0025-entrypath-baseline.md` · `docs/research/runs/SPRINT-0025-entrypath-search-matrix.md`

---

## Pending

- **Full archetype catalog migration.** SPRINT-0017 migrates only
  `serialized-actor` and `keyed-partitioned-state` to first-class required
  property sets for the Caddy ADR-0022 vertical slice. The remaining archetypes
  stay on the legacy path until a future un-numbered sprint.

- **Category B backlog** (from `docs/specs/reviews/systems-review.md` §§B1/B3/B8/S13/S14/T1/T2):
  thesis statement, Waldo-delta appendix, prior-art References section (≥10
  citations), PLOS retirement table expansion, actor-rejection expansion,
  shadow-actor framing, semantics appendix. Deferred to a pre-paper revision
  sprint; not blocking for the compiler implementation sprints.

- **SPRINT-0005+ compiler implementation epics.** The follow-on-sprints section at the bottom of
  `docs/sprints/SPRINT-0003.md` seeds six implementation epics: call-graph /
  SSA extraction pass, canonical-shape signature classifier, state-class
  inference + singleton codegen, v2 pragma parser, refusal-diagnostic
  framework, end-to-end miniflux smoke test. These will each produce their
  own ADRs as implementation decisions land.
