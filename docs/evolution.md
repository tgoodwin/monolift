# Monolift — Design Evolution

A chronological narrative of how studying real-world Go monoliths reshaped
Monolift's compiler design. Each entry links to the ADR that formalizes the
decision and the evidence docs that motivated it.

The ADRs in `decisions/` are the primary sources; this document is the
reader's entry point and tells the story that no single ADR can.

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

## Pending

- **SPRINT-0004 harness closed.** `make e2e` now builds the stub compiler,
  runs the Kind-backed e2e table, passes Caddy through stages 0–10, passes
  Pocketbase through refusal stages 0–4, and cleanly skips Miniflux,
  Listmonk, Gitea, and Mattermost with SPRINT-0005 pointers. The harness
  owns `test/e2e/`, shared closure-report types live in
  `pkg/compiler/reportv2/`, and failure messages carry
  `[stage=N target=X kind=...]` prefixes for agent triage.

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
