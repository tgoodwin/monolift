# ADR-0017: Classifier should reason about liftability, not transport

**Status:** accepted
**Date:** 2026-04-21
**Context docs:** `docs/decisions/0006-canonical-shapes-transport.md`, `docs/decisions/0015-canonical-shape-classifier.md`, `docs/specs/monolift-v2-contract.md`, `docs/evolution.md`

## Context

The canonical-shape classifier committed by ADR-0006 and implemented
per ADR-0015 established the right *approach* — pattern-match code
regions to decide admissibility. What this ADR calls into question is
the *patterns* that approach matches on. Today's predicates fall into
two groups, and both work at the level of literal type signatures:

- Transport-oriented archetypes — `http-handler`, `channel-consumer` —
  match on framework-specific type signatures and reward code that
  already has a network-facing shape. This is backwards relative to
  the paper's vision. A real monolith that the developer has not yet
  distributed is unlikely to present its liftable regions as HTTP
  handlers in the first place; if the classifier depends on
  HTTP-handler idioms, the compiler ends up chasing already-distributed
  code instead of distributing what was local.
- Generic signature archetypes — `multi-domain-args`, `no-response`,
  `builder-chain` — come closer to a liftability check but still
  match on literal signatures rather than on the properties those
  signatures express.

A liftability-first classifier would keep the pattern-matching
discipline and replace what it matches on. Instead of literal type
signatures, it would match on *properties* of those signatures and
their function bodies — the properties that actually determine whether
a local call can be rewritten as a remote one:

- whether parameters are passed by value;
- whether parameter and return-value types are serializable across a
  network boundary, or can be made so by a cheap, deterministic
  transform the compiler can generate;
- whether the function body does direct heap access or pointer-mediated
  mutation of the caller's memory;
- whether the function's signature anticipates failure — an error
  return is evidence that the caller already has vocabulary for the
  failure modes a remote call would introduce;
- whether the function is synchronous and short-lived, or open-ended
  (a long-running worker).

The error-return property connects directly to Waldo et al.'s *A Note
on Distributed Computing* (1994): Monolift has never intended to
pretend that remote calls behave like local ones. A function whose
signature already returns an error is a function whose caller has
accepted that the call may fail — which is a prerequisite any
distribution-aware rewrite must respect. Matching on this property is
how the classifier makes that prerequisite visible in the admissibility
decision rather than asserting it after the fact.

Transport selection then becomes a downstream step: given a region
that is liftable and classified by its intrinsic properties, the
compiler picks a transport template that matches. HTTP, gRPC, channel
over a message broker, or anything else is a packaging decision, not
a gate for admission.

## Decision

Admission is now decided by named liftability properties and their evidence,
not by literal canonical-shape matches. The compiler evaluates each exposed
operation against the gating property set from ADR-0018, aggregates those
operation-level results at the root, and treats transport-shape predicates as
downstream selector signals instead of admissibility gates.

The implemented split is:

- liftability analysis decides `liftable` / `refused` / `unsupported`
- transport selection consumes admitted operations plus selector signals and
  writes `root.shape` / `root.defaultTransport`
- existing `MLV2_*` refusal codes stay stable; only the triggering evidence
  changes

Property-to-refusal mapping stays inside the existing taxonomy:

- boundary incompatibilities such as variadics and callable boundary values
  continue to route to `MLV2_SHAPE_UNSUPPORTED`
- channel-typed boundary crossings continue to route to
  `MLV2_CHANNEL_BOUNDARY`
- non-serializable boundary types continue to route to
  `MLV2_SERIALIZATION_UNSUPPORTED`
- unresolved type parameters continue to route to
  `MLV2_SURFACE_DEFERRED_GENERIC_DECL`
- parameter/receiver mutation through aliases continues to route to
  `MLV2_POINTER_ALIAS_UNSUPPORTED`
- shared mutable global writes continue to route to
  `MLV2_SHARED_MUTABLE_STATE`
- reflect/unsafe reachability continues to route to
  `MLV2_REFLECTION_DISPATCH` or `MLV2_UNSAFE_CODE`
- missing error-channel vocabulary continues to route to
  `MLV2_NO_ERROR_CHANNEL`
- builder/self-return surfaces continue to route to
  `MLV2_BUILDER_CHAIN_ROOT`

ADR-0018 is the frozen named set of properties, IDs, and outcome classes that
this decision relies on.

## Consequences

- Admission now matches the semantics Monolift actually cares about:
  liftability of a local boundary under remote rewriting, not resemblance to
  an existing framework transport.
- The analysis is more complex because it combines `go/types`, SSA, and
  selective callgraph evidence, but the heuristic-containment rule keeps that
  complexity from producing false refusals: sound detectors may gate;
  heuristic detectors default to `Unknown` and stay advisory.
- The `MLV2_*` refusal-code taxonomy is preserved. Reports and tests change
  because the evidence is richer, not because the refusal vocabulary was
  replaced.
- Canonical transport shapes still survive as downstream selector outputs for
  reports, pragmas, and adapter derivation; ADR-0006 remains live for that
  narrower role.
- The design-story site still describes the old classifier and remains
  intentionally deferred until the post-landing docs sprint.

## Layered architecture (clarifying note)

This ADR drew the line between admission (driven by intrinsic code properties)
and transport selection (downstream of admission). Combined with ADR-0016
(state-class inference) and ADR-0022 (composite-archetype regions), the
analyses Monolift performs over a region now compose as a layered structure
worth stating explicitly:

1. **Liftability properties** — the named vocabulary of facts about the code
   (boundary, effects, lifecycle, contract). ADR-0018 freezes this set.
   Detectors live in `pkg/compiler/liftability/`. The output is a per-region
   property-fact set: which properties hold, which are violated, which are
   unknown. This layer also gates admission per this ADR.
2. **Archetypes** — each archetype is *defined as a particular subset of those
   liftability properties*. An archetype "matches" a region iff its required
   property subset is satisfied by that region's property-fact set. The
   archetype catalog lives alongside `pkg/compiler/stateclass/`.
3. **Candidate set + subsumption** — when a region matches more than one
   archetype, ADR-0022 governs how the compiler navigates the resulting set:
   subsumption (compared over the underlying property subsets), composite
   emission, alternatives reporting.
4. **Transport selection and adapter derivation** — downstream of the chosen
   primary candidate. This is the layer ADR-0006 still organizes.

The key invariant: archetypes do not invent their own vocabulary of code
facts; they consume the liftability-property vocabulary from ADR-0018. This
is what makes subsumption (Layer 3) a clean set-comparison operation —
candidates are comparable because their required-invariant sets are drawn
from the same shared vocabulary.

## References

- ADR-0006 — Canonical shapes as the transport/adapter organizing concept
- ADR-0015 — Canonical-shape classifier
- ADR-0016 — State-class inference
- ADR-0018 — Liftability property taxonomy (the shared vocabulary)
- ADR-0022 — Composite-archetype regions (the layer-3 navigation rules)
- `docs/specs/monolift-v2-contract.md` §Conceptual-Model Baseline
