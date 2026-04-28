# Monolift v2 Contract — Systems / Research Review

**Target:** `docs/specs/monolift-v2-contract.md` v0.1-draft (2026-04-19)
**Reviewers (merged):** codex / gemini / claude (via `sprint-planner`-style fan-out; critiques cross-cut)
**Merge-author:** Opus 4.7 (this session)
**Perspective:** skeptical PL/systems PC reviewer; Waldo 1994, Weisenburger CSUR '20,
ScalaLoci, HasChor/Pirouette, Orleans, Pony, Service Weaver retrospective, CALM/LVars,
Offload Annotations, AIFM, Coign/MAUI are shared baseline.

---

## Executive summary

**Verdict: major revisions required for research-grade.** The spec is a substantial
engineering improvement over v1 — honest about the network boundary where v1
was silent, principled in refusing unsafe lifts. But as a research/specification
artifact it (a) has no thesis a PC could evaluate against, (b) leans on the word
"bounded" 12+ times without defining it as a predicate, (c) treats the seven
state classes as given when several are not carved at joints, (d) gives only a
partial Waldo account (failure and zero-values covered; call outcomes, pointer
mutation, memory-model visibility, reordering are silent), and (e) cites no
external work. The PocketBase negative case is the most honest part of the
document and the strongest evidence of research-grade thinking. The rest needs
a Waldo-delta appendix, a bounded-closure predicate, a re-derived state
taxonomy, a remote-call-outcome model, a prior-art section, and a one-sentence
thesis before it reads as a research contribution rather than a pragma reference.

**Top five concerns:**
1. **No thesis.** The document needs a research claim.
2. **"Bounded" is not a predicate.** The central invariant is decorative.
3. **Waldo awareness is partial.** Call outcomes, pointer mutation, memory
   model, reordering all unspecified.
4. **State taxonomy confuses ontology with predicate.** Seven classes, several
   collapsible.
5. **Zero external citations.** PC-indefensible for a research artifact.

---

## Blocking issues (block research-grade acceptance)

### B1. No thesis statement

The spec does not state, in one sentence, what research claim it defends. The
closest is L95 ("A compiler that cannot prove... MUST refuse") — a design rule,
not a thesis. Without a thesis the document reads as engineering notes on how
to build a pragma parser.

**Proposed thesis (pick one or synthesize):**

> "A Go monolith can be incrementally distributed by annotating declarations
> with a bounded-closure contract whose violations are compile-time refusals,
> and whose accepted lifts preserve local semantics modulo a named set of
> Waldo-delta axes."

That sentence commits the spec to (a) bounded-closure as a predicate, (b)
refusal as the failure mode (not best-effort generation), (c) Waldo-delta as
an enumerated set rather than an unqualified disclaimer. Add to spec between
L10 and L26.

### B2. Bounded-closure as formal predicate (cross-cutting; L34, L79, L292, L354, …)

"Bounded" appears 12+ times as load-bearing. It is never defined. EC-PRUNE-2
(L356) enumerates smells, not a predicate. If a reviewer asks "is this closure
bounded?" the spec's answer is "the compiler said so." That is an escape hatch,
not a research invariant.

**Required:** new rule after L295: "A closure is *bounded* iff the set of
reachable external edges is finite under the termination rules of EC-TERM-*
and no refusal condition in §3–§5 applies." Cross-reference from every current
use of "bounded". `MLV2_CLOSURE_TOO_LARGE` becomes a named refusal when the
predicate does not hold; the smell list in EC-PRUNE-2 becomes non-normative
implementer guidance.

### B3. Waldo awareness is partial (CP-3 at L103; §SS-WALDO-1..6 at L414–L426)

CP-3 admits remote execution "MAY add latency, serialization limits,
independent failure modes, timeout behavior, and scheduling effects". That is
a list, not a semantic commitment. SS-WALDO-1..6 cover failure and zero-value
paths but are silent on:

- **Call outcome taxonomy** (see B4 below)
- **Pointer mutation / aliasing** (see B5)
- **Go memory-model visibility** — receiver fields read by a lifted method
  and written by a non-lifted goroutine in the local impl; Go's
  happens-before relation does not extend across the network
- **Ordering** — two calls to the same lifted method from one goroutine are
  serialized locally; SS-WALDO-6 (L426) is silent on remote serialization
- **Panic propagation** — SS-WALDO-4 converts panics to errors correctly, but
  caller-side `defer recover()` no longer sees the remote stack; this is a
  semantic drift worth naming near CP-3 rather than buried in state semantics
- **Partial failure across multiple lifts in one request**
- **Cancellation best-effort vs. semantic** — TA-CTX-2 (L516–L519) is a
  transport-delivery statement, not a semantic one

**Required:** Waldo-delta appendix enumerating axes with per-axis rules.
Cross-reference from CP-3. Rewrite CP-3 as a positive statement of the
observable differences v2 commits to rather than a disclaimer.

### B4. No remote-call-outcome model (SS-WALDO-6 at L426)

"v2 makes no hidden retry guarantee" is an abdication, not a semantics.
Absence of retries is not at-most-once. A single HTTP request can still be
maybe-executed under process crash, disconnect, timeout, load-balancer
failure. For side-effecting operations, this is the central distributed-systems
fact.

**Required:** new subsection defining outcomes:
- success
- local serialization failure
- remote transport failure before execution
- remote maybe-executed failure (started, result unknown)
- remote completed-but-reply-lost
- timeout / cancellation
- remote panic (post SS-WALDO-4 conversion)

State that side effects MAY have occurred for maybe-executed outcomes unless
the transport/adapter provides stronger semantics.

**Required companion rule:** forbid automatic local fallback after a failed
remote attempt unless the operation is declared idempotent or the adapter
supplies deduplication / exactly-once semantics. Fallback after
maybe-executed remote = double-apply side effects or switched state
authority.

### B5. Pointer mutation through arguments is the most direct Waldo violation (TA-SER-3 at L498)

Local Go calls can mutate through pointer parameters or receiver fields that
alias caller-visible state. Remote calls generally cannot, unless the adapter
does copy-in/copy-out and alias restoration. TA-SER-3's "ownership is
transferred for the duration of the call" does not address mutation of aliased
state.

**Required:** either refuse mutable pointer arguments/results that require
caller-visible alias preservation (with a named diagnostic), or define
copy-in/copy-out semantics with alias restoration. Without this the spec
silently converts local reference semantics to remote value semantics — the
canonical Waldo violation.

### B6. Dynamic mode + mutable state = state forking (DP-MODE at L559–L568; SS-LIFT-1 at L390)

`mode=dynamic` requires both local and remote impls to be available.
`SS-LIFT-1` says receiver fields read/written by the closure are owned by the
lifted deployable; the local impl keeps receiver fields for local execution.
These two rules are incompatible for mutable state unless the state is
externalized, immutable, or explicitly copy-in/copy-out with a consistency
story. Under `mode=dynamic` the state can fork between local and remote
execution.

**Required:** top-level invariant — `mode=dynamic` is accepted only for
stateless, immutable-captured-config, externalized-durable, or explicitly
divergence-tolerant process-local-cache state. Singleton / session / shared
mutable state requires singleton or affinity placement and no local/remote
alternation unless an external authority owns the state.

### B7. State taxonomy not carved at joints (L360–L451)

`SS-DISP-1` requires every captured state item to have exactly one class
(L386) — but the target cross-check (L446–L451) uses "plus" combinations
(Mattermost WebHub = singleton-mutable + connection/session). **Internal
spec contradiction.**

Several classes collapse on inspection:
- `immutable-captured-config` is a subclass of `stateless` plus a
  deep-immutability predicate; keeping it separate only makes sense if v2
  actually enforces freeze semantics (which it doesn't — see S4 below).
- `process-local-cache` is not a state class but a *correctness predicate*
  ("divergence is not correctness-observable") applied to shared-mutable
  data. Splitting it out conflates ontology with predicate.
- `connection-session` is a transport/affinity concern; the state stored
  *for* a session is `singleton-mutable` keyed by session — the refusal at
  L384 is really about transport affinity.

**Required:** adopt *facet-classification* — each captured state **edge/facet**
has one or more classes; the disposition is the most restrictive class
relevant to correctness. Closure report shows composite state. Re-derive the
taxonomy from two orthogonal axes as a cross-check:
- *who may mutate* (none / single owner / multiple owners)
- *what loses correctness on loss* (nothing / session-scoped / application-scoped)

Derive the classes (or fewer) from the cross-product. Seven becomes four or
five with the predicate axis orthogonal.

### B8. Prior-art positioning (zero external citations)

The spec cites PLOS '25 and the audit docs; it cites no external work.
Indefensible for a research-oriented specification. Audit-level gaps:

- **Waldo et al. 1994, "A Note on Distributed Computing"** — cited
  implicitly at L968; SS-WALDO-* takes its name; must be referenced
- **Weisenburger, Wirth, Salvaneschi (CSUR '20)** — multitier survey; v2
  operates directly in this axis space
- **ScalaLoci (OOPSLA '18)** — closest prior art; tier-typed fields are
  stronger version of v2's state classes
- **HasChor (POPL '23), Pirouette (POPL '23), Choral (TOPLAS '24)** —
  choreographic programming; v2's lift points are endpoint projections.
  Either claim this framing or explicitly reject it
- **Service Weaver (HotOS '23) + 2024 shutdown retrospective** — cited in
  research brief as the cautionary tale motivating pay-as-you-go; not in
  spec. The failure mode Weaver hit (developers won't rewrite to a new
  framework) is the motivating constraint for CP-1/CP-2
- **Orleans, Akka Typed** — virtual actor model solves singleton affinity
  by construction. v2's `singleton` reinvents a weaker version; either
  claim the connection or justify the divergence
- **Pony (AGERE '15) / Rust Send+Sync** — reference capabilities answer
  "when is state safely sendable" with a type system; v2's state inference
  is a manual approximation
- **Coign (OSDI '99), MAUI (MobiSys '10), CloneCloud (EuroSys '11)** —
  automatic partitioning; v2 defers (DP-DEFER-1) but never cites the
  tradition it's deferring into
- **Offload Annotations (ATC '20), AIFM (OSDI '20)** — the closest
  published prior art to `//monolift:lift`
- **CALM / Bloom / LVars (Hellerstein, Alvaro, Kuper)** — principled
  answer to "which shared-mutable state can be safely distributed"; v2
  refuses the question at SS-DISP-2
- **Go memory model** — required for any treatment of SS-LIFT-1's
  receiver-field semantics

**Required:** References section. Minimum: the 10 items above.

### B9. Pay-as-you-go has two senses (CP-1/CP-2 at L99–L101)

CP-1/CP-2 promise annotated code builds under `go build` without Monolift.
This is preserved *syntactically*. But the pragma surface at L266–L274 now
carries state/transport/policy/impl/registry/methods keys — a developer
reading `//monolift:lift name=x state=singleton` cannot understand it
without reading this spec. The *semantic* commitment of a v2-annotated
program has grown substantially.

**Required:** state explicitly that pay-as-you-go is preserved at the
build-and-run level, and source-comprehension cost is not zero. The
adoption promise is weaker than CP-1/CP-2 suggest — many accepted lifts
require adapters, state assertions, affinity keys, or external queues.
Candidate adopters reading L57–L61 will expect more than v2 delivers.

### B10. External-module state/effect laundering (EC-TERM-2 at L310–L314)

The closure terminates at external module boundaries, importing symbols as
dependencies. Third-party Go modules can have package globals, init-time
side effects, caches, file descriptors, background goroutines, connection
pools, singletons, process-local state. Treating them as an import
boundary hides exactly the state the taxonomy is meant to expose.

**Required:** distinguish *source inclusion* termination (fine at module
boundary) from *state/effect analysis* termination (unsound at module
boundary when imported modules have globals / init effects / goroutines
/ cgo / serialization-visible types). State/effect summaries for
imported modules must be considered.

---

## Serious concerns (should land before v1.0)

### On state semantics

- **S1. `dispatch=lift-point` state-consistency contract missing** (L642–L646).
  The spec requires the closure report to list implementation conditions but
  not the state-consistency contract for the *dispatch condition itself*. If
  that expression reads mutable config, feature flags, session state, the
  local and remote semantics diverge. Name this.
- **S2. Wrappers that add retry or authorization are not transparent.**
  MI-WRAP-2..4 treats wrappers as non-implementations because they forward.
  Retry changes distributed-failure semantics; authz reads request/session
  state. Distinction should be "decorator in selected production value
  graph" vs. "alternative root", not "independent implementation" vs.
  "wrapper".
- **S3. `process-local-cache` needs proof-vs-assertion distinction.** L400
  says "compiler proves OR developer declares" divergence is not
  correctness-observable. The assertion path is unsound if wrong. Many
  caches encode authz, rate limits, idempotency. The `state=` syntax
  should distinguish *compiler proved* from *developer asserted,
  unsound if wrong*; closure report should record the proof basis or
  lack thereof.
- **S4. `immutable-captured-config` needs deep-immutability semantics** (L367).
  "Read without mutation" requires deep immutability, not just no direct
  writes: maps, slices, pointers, `atomic.Value`, methods-that-mutate can
  all violate it. Commit to a freeze semantic or narrow the class.
- **S5. `shared-mutable-across-callers` vs `singleton-mutable` distinction is
  placement, not data-shape.** A global counter is `shared-mutable` if
  multiple owners exist, `singleton-mutable` if routed to one owner.
  The real distinction is "does the selected placement give all
  correctness-relevant operations a single serialization authority". The
  current taxonomy hides this.

### On transport and handler

- **S6. Handler transport capability gaps** (TA-HANDLER-1 at L482–L492).
  Streaming responses, request-body backpressure, trailers, flush,
  hijack, WebSockets, HTTP/2 server push, client disconnect, framework-
  specific context mutation are all unpreserved by the sketched forwarder.
  Add `MLV2_HANDLER_CAPABILITY_UNSUPPORTED` for features the proxy can't
  preserve.
- **S7. Listmonk channel-consumer example contradicts TA-SER-7** (L831–L851
  vs L504–L506). Annotated `func worker(ctx, jobs <-chan CampaignJob) error`
  has `jobs` supplied by the monolith — but channels cannot cross the remote
  boundary. Either rewrite the example to show internally-owned queue or
  external-queue substitution, and state explicitly that this lift
  requires application change (an adoption cost, not annotation-only).
- **S8. Stable session affinity mentioned three places without a protocol.**
  L384 disposition says "affinity-routed or refused"; L704 pragma `affinity`
  key says "context/request/session key"; L906 Mattermost row says
  "deferred unless stable session affinity is available". Three mentions,
  no single rule specifying key extraction, per-invocation vs per-connection
  scope, or interaction with the HTTP request lifecycle. Define it once,
  normatively.
- **S9. Module version pinning in closure report.** EC-TERM-2 terminates at
  external module, but if the monolith uses `v1.2.0` and the lifted
  deployable resolves to `v1.2.1` with different `init()` side effects,
  pay-as-you-go equivalence is silently broken. Require closure report to
  record the exact dependency manifest (module path + version + sum) at
  extraction time.
- **S10. Adapter versioning and deployment skew.** Even with Kubernetes as
  backend (not semantic core), generated adapters still need a compatibility
  story: monolith version A calling lift version B, rolling deploys with
  mixed schema, drained-but-serving pods. Add a named failure mode.

### On dispatch and policy

- **S11. Dynamic policy is trigger predicate, not placement policy.**
  DP-POLICY-3 at L574–L576 defines `trigger=CPU threshold=0.70` as the
  baseline. The research brief explicitly warns this is myopic and oscillates
  without hysteresis/damping. The spec should specify sampling granularity,
  hysteresis, stickiness, in-flight request behavior during transitions,
  and whether decisions are per-call / per-window / per-deployable-epoch —
  OR state explicitly that the concrete baseline is *non-normative example*
  and v2 makes no semantic guarantee about placement stability.
- **S12. Offloading cost model missing.** A CPU threshold cannot decide
  remote profitability without argument size, serialization cost, RTT,
  bandwidth, queueing, state-affinity cost. MAUI, CloneCloud, AIFM, and
  Offload Annotations all define this. The spec can defer optimization
  but should acknowledge the inputs and cite the literature.

### On the conceptual-model baseline and alternatives

- **S13. PLOS retirements missing from alignment table (L997–L1008).**
  The table retires "wiring in main" and revises state/annotation-site
  assumptions. It should also mark as revised or retired:
  - "Functions become ephemeral tasks; classes/interfaces become network
    services" (PLOS §§) — v2 admits singleton workers and channel
    consumers, a different mapping
  - "Minor code changes required to support lifts" — v2's state taxonomy
    admits that externalizing state, adding affinity keys, or replacing
    channels with external queues may be nontrivial
  - Strong source-level transparency ("no modifications to surrounding
    program") — v2 softens but never explicitly retires
- **S14. Actor rejection is thin.** L969 rejects "actor framework wholesale"
  in one sentence. v2 *does* inherit the hard actor questions once it
  accepts singleton + affinity-routed state: activation, mailbox ordering,
  reentrancy, supervision, crash recovery, identity. The rejection
  paragraph should enumerate which actor concerns v2 refuses, which it
  leaves to the backend, and which it adopts in reduced form. Cite Orleans
  and Pony.

### On the validation section

- **S15. Cross-target verdicts overclaim (L810–L954).** "Accept" language is
  used for conditional cases:
  - Listmonk (L845–L851) depends on channel staying internal or being
    replaced by external queue — code change
  - Mattermost (L906) depends on `request.CTX` adapter that doesn't exist
    yet in the spec
  - Caddy (L862–L868) accepts static-registry subset; dynamic module
    loading is common enough to deserve a framing of "accepts a static
    Caddy subset"
  Reclassify as "candidate accepted under conditions", not successes.
- **S16. Validation Pass Log is self-congratulation (L949–L954).** "Pass 1
  handled all six targets. Pass 2 re-ran... no rule invalidation." If
  two passes produced zero changes, the passes were not adversarial. A
  real validation log records rule revisions forced by target validation.
  Remove or replace.
- **S17. Traceability Table is decorative (L37–L55).** Every row points at
  the same nine sections. Either prune columns to the one or two rules
  that actually resolve each audit item, or delete.
- **S18. PocketBase refusal too tailored.** `MLV2_EMBEDDED_DB_APP_ROOT`
  names PocketBase's specific shape. The class is general: SQLite, Bolt,
  Badger, local file-backed indexes, spool directories, raft logs, WAL
  segments, persistent queues. Keep PocketBase as the canonical example,
  generalize the diagnostic class.

---

## Cross-cutting themes

### T1. Diagnostic-driven design is not a semantics

Thirty-seven refusal diagnostics (L1014–L1052); a program is "accepted" iff no
diagnostic fires. Operationally clear, semantically empty — there is no
positive statement of what acceptance means. Compare Pirouette: endpoint
projection is an operator with a theorem (projection preserves behavior modulo
failure). Fix: short semantics appendix giving the (lifted, local) pair and
their precise relation under the enumerated Waldo-delta axes. One page of
denotational/operational semantics upgrades the document from style guide to
specification.

### T2. The "shadow actor" problem

Once v2 admits singleton-mutable + affinity-routed state, it has imported the
hard actor questions without importing actor discipline: activation, mailbox
ordering, reentrancy, supervision, crash recovery, identity. L969's "a lift
is a compiler-selected source segment, not an actor API" rhetoric doesn't
address this. Name the pattern, cite Orleans/Pony, and state per-question
whether v2 refuses / defers-to-backend / adopts-in-reduced-form.

### T3. Pay-as-you-go survives as tooling compatibility, not zero-refactoring adoption

The pay-as-you-go promise is preserved only in the narrow sense that comments
don't break ordinary builds. The broader adoption promise is weaker — many
accepted lifts require adapters, state assertions, affinity keys, external
queues, or state externalization. The spec should embrace the distinction. It
is more credible than pretending all real targets remain annotation-only.

### T4. Dynamic placement + mutable state needs a hard boundary

The single most dangerous combination. If local and remote paths both remain
available, state can fork. If fallback-to-local is ever allowed after remote
errors, same problem. Top-level invariant: dynamic local/remote selection
accepted only for stateless / immutable / externalized / explicitly
divergence-tolerant state.

---

## Suggested spec edits (concrete, priority-ordered)

1. **Thesis section** between L10 and L26. One paragraph committing to the
   research claim. (B1)
2. **Bounded-closure predicate** after EC-CLOSURE-3 at L295. Cross-reference
   from every current use of "bounded". (B2)
3. **Waldo-delta appendix** enumerating: latency, failure, partial failure,
   cancellation, deadline, panic, memory-model visibility, ordering,
   context-value loss, pointer mutation. Per-axis: v2 commitment. Cross-reference
   from CP-3 and SS-WALDO-*. (B3)
4. **Remote call outcome model subsection.** Define success, local-serialization-failure,
   pre-exec transport failure, maybe-executed, completed-but-reply-lost, timeout,
   remote panic. (B4)
5. **Pointer-mutation / copy-in-copy-out rule** in TA-SER-3 at L498. (B5)
6. **Dynamic-state invariant** — `mode=dynamic` is accepted only for
   stateless / immutable / externalized / explicitly divergence-tolerant
   state. (B6)
7. **Re-derive state taxonomy** from orthogonal axes (who-may-mutate ×
   correctness-observable-on-loss); adopt facet-classification; fix
   SS-DISP-1 "exactly one class" contradiction. (B7)
8. **References section** with ≥10 prior-art citations. (B8)
9. **Pay-as-you-go two-senses clarification** in CP-1/CP-2 at L99–L101. (B9)
10. **Source-inclusion vs state/effect-analysis termination** in EC-TERM-2
    at L310–L314. (B10)
11. **Forbid automatic local fallback** after failed remote attempt unless
    idempotent-declared. (B4 companion)
12. **Handler-capability refusal family** `MLV2_HANDLER_CAPABILITY_UNSUPPORTED`
    covering streaming/hijack/WS/HTTP2-push/trailers/flush/backpressure. (S6)
13. **Stable session affinity protocol** — one normative rule, replacing
    the three current mentions. (S8)
14. **Rewrite Listmonk channel-consumer example** to not imply a
    monolith-owned Go channel crosses the lift boundary. (S7)
15. **Policy stability / cost-model** clarification or explicit non-normative
    example marker. (S11, S12)
16. **PLOS retirements table expansion** (function-as-task, minor-code-
    changes, strong transparency). (S13)
17. **Actor rejection expansion** — enumerate per-concern disposition;
    cite Orleans/Pony. (S14)
18. **Downgrade cross-target verdicts** from "accept" to "candidate accepted
    under conditions" where adapters / code changes are required. (S15)
19. **Replace or delete Validation Pass Log** (L949–L954) with a real
    revision history. (S16)
20. **Prune or delete Traceability Table** (L37–L55). (S17)
21. **Generalize `MLV2_EMBEDDED_DB_APP_ROOT`** to a class diagnostic;
    PocketBase as canonical example. (S18)
22. **Dependency-manifest identity in closure report** (S9).
23. **Adapter versioning / deployment-skew diagnostic** (S10).
24. **Wrapper semantics distinction** — decorator-in-production vs
    alternative-root, not transparent-vs-independent. (S2)
25. **Cache proof-vs-assertion distinction** in `state=` syntax. (S3)

---

## Points of reviewer disagreement + merge resolutions

| Disagreement | Positions | Merge decision |
|---|---|---|
| Bounded-closure numeric vs predicate | GEMINI: max-depth / symbol-count threshold; CLAUDE: frontier predicate; CODEX: "measurable cut" | Predicate is the spec-level invariant; numeric thresholds are non-normative implementer defaults |
| Pay-as-you-go "lie" vs "equivocation" | GEMINI: Monolift "chooses to lie"; CLAUDE/CODEX: overstates | Frame as two senses (build-and-run preserved; source-comprehension cost nonzero). Drop "lie" framing |
| Hard error-checking refusal | GEMINI: refuse lifts whose call sites ignore error | Warning-level only (`MLV2_LIFT_ERROR_IGNORED`); not refusal |
| Cycle-tolerant encoding | CLAUDE: soften TA-SER-3 acyclic rule or add cycle-tolerant encoding | Real issue is aliasing/mutation (B5), not cycles; cycle concern is secondary |
| State taxonomy reform | CLAUDE: re-derive from 2 orthogonal axes; CODEX: facet-classification; GEMINI: coherence warnings | Facet-classification (rule change) + 2-axis re-derivation (structural) |
| Actor-system relationship | GEMINI: shadow-actor framing; CLAUDE: Orleans prior-art; CODEX: actor-rejection too thin | Combine: acknowledge singleton/affinity are actor-shaped; cite Orleans/Pony; enumerate per-concern disposition |
| Thesis | CLAUDE: blocking absence; CODEX: gentle proposal; GEMINI: paraphrase of spec's own framing | Blocking — add explicit thesis section |

---

## What the spec gets right (do not unwind)

- **Refusal-first posture.** Compiler that refuses unsafe annotations rather
  than generating unsafe code is the right research default. v1's panic-on-
  unhandled-return is correctly retired.
- **PocketBase as named negative example** (L911–L926). Unusually honest;
  materially strengthens credibility.
- **EC-TERM-* termination rules** (L310–L322). Correctly cut at every place
  Go programs escape analysis (with caveats on module state in B10).
- **`dispatch=lift-point` primitive** (L640–L647). Genuinely interesting —
  admits some lifts are selection expressions, not implementations. Small step
  toward choreographic projection. Claim it.
- **Separating annotation surface from extraction root from closure report.**
  Right factoring.
- **Demoting unique-implementer to optimization** (L588–L595). Matches
  real-world multi-impl patterns exposed by the audit.
- **`//go:build` / selected build configuration as part of the lift root.**
  Mostly right; just needs ambiguity-refusal (see compiler-review S24).

---

## Research-narrative assessment

### Credible follow-up paper

**Partial.** This spec supports a paper like "Monolift v2: A Bounded-Closure
Source Contract for Pay-as-you-go Distribution in Go" — venue-defensible at
PLOS, Onward!, or SOSP workshop venues. Not yet a PLDI / OOPSLA / OSDI paper,
because:

- State-class taxonomy is not derived from a principle
- Bounded-closure invariant is not a predicate
- Waldo-awareness is incomplete
- No formal relation between accepted programs and lifted executions

Any one of those, closed, lifts the paper into a stronger venue. Most
tractable: the bounded-closure predicate (B2). Define it precisely, prove
EC-TERM-* imply it, and the core contribution becomes a decidable safety
property — a PL result rather than a tool description.

### One-sentence thesis (proposed)

> "Monolift v2 turns the original transparent-offload prototype into a
> conservative liftability contract for legacy monoliths: lift boundaries
> are accepted only when closure, state, serialization, and failure
> semantics can be made explicit; otherwise the compiler refuses with
> actionable diagnostics."

Better paper than "automatic distribution with comments." Less glamorous:
research contribution becomes the *contract and its empirical grounding*,
not the compiler trick.

### Positioning

**Engineering-forward, research-weak.** A PL venue asks for a semantics.
A systems venue asks for empirical evaluation (acceptance/refusal rates on
the six-target corpus with quantified closure sizes) or a deployment story
(what does lifted Caddy look like in production). The spec punts on both by
pointing at SPRINT-0005 and the evaluation-corpus ADR. A cross-reference is
not a deferral — it's a promise the paper must eventually keep.

The strongest available narrative is *negative-results-informed design
revision*: the audit shows v1 fits 1–4 of 8 dimensions across the six
targets; the evolution doc says the demo fit because it was written to
compiler conventions. Real monoliths forced a renegotiation of the
abstraction. That's a story worth telling, and it's credible because the
spec admits refusals and names a lower bound (PocketBase). The danger is
overclaiming acceptance in the validation section; S15/S16 above address
that.

---

## One-paragraph verdict

The spec has moved the project from "how does the v1 compiler not generalize?"
to "what exactly will a v2 compiler accept?" — real progress. As a research
contribution it needs the structural edits above: a thesis, a bounded-closure
predicate, a Waldo-delta appendix, a remote-call-outcome model, a re-derived
state taxonomy, pointer-mutation rules, and a prior-art section. With those,
the follow-up paper writes itself around the contract and the six-target
empirical grounding. Without them, the document is a well-organized pragma
reference, and a PC will say so. Estimated effort for the full edit pass: 2–3
focused revision sessions, including a re-run of Phase 9 target validation
against the revised state rules.
