# ADR-0013: SSA closure precision policy

**Status:** accepted
**Date:** 2026-04-20
**Context docs:** `docs/specs/monolift-v2-contract.md` §Extraction Root and Closure; `docs/decisions/0011-harness-before-compiler.md`

## Context

SPRINT-0006 replaces fixture-backed Caddy and Pocketbase reports with real
compiler-owned SSA extraction and refusal diagnostics. The extraction contract
requires a conservative, deterministic call/value-graph analysis, but it does
not mandate a specific algorithm.

Three precision/cost facts matter for this sprint:

- CHA is sound on partial programs and gives a deterministic whole-program
  over-approximation with comparatively low implementation complexity.
- RTA can reduce interface-dispatch fanout once a root and reachable
  instantiation set are known, which is especially relevant for registry-keyed
  roots such as Caddy modules.
- VTA and pointer analysis could further reduce false positives, but they add
  materially more implementation cost, tuning surface, and runtime expense than
  SPRINT-0006 is prepared to absorb while also landing the red-first harness
  flips and the new refusal-diagnostic framework.

Phase-0 probe data:

- Local spike (`pkg/compiler/testdata/ssaspike`) showed `packages.LoadAllSyntax`
  + SSA + CHA succeeding with deterministic file selection under build-tag and
  `CGO_ENABLED` changes.
- Caddy reverse-proxy probe reported `ssaFunctions=78289`, `chaNodes=78290`,
  and root-level interface-dispatch fanout `22` at `(*Handler).ServeHTTP`,
  which is large enough to justify targeted refinement but not so large that
  CHA is unusable as the default base algorithm.

## Decision

SPRINT-0006 uses a two-level SSA precision policy:

- CHA is the default dispatch approximation for lifted roots.
- Registry-keyed roots may refine invoke edges through RTA when the annotated
  root and compiled package graph make the implementation set finite.
- The emitted analysis marker for this sprint is `ssa-cha+rta`, reflecting that
  CHA remains the baseline and RTA is an opt-in refinement, not a separate
  compiler mode.
- Precision triggers are recorded deterministically so widened or narrowed
  dispatch sites remain debuggable across repeated runs.

VTA and pointer analysis are explicitly deferred. They stay out of the
SPRINT-0006 implementation even when they could reduce over-approximation,
because the sprint is simultaneously landing the real Caddy harness flip, the
Pocketbase refusal framework, and the new diagnostic seam. Adding a more
expensive precision stack would materially widen implementation scope, runtime
cost, and tuning surface without being required to make the current roots pass.

The Phase-0 probe data supports this cutoff:

- The local SSA spike stayed comfortably sub-second even under build-tag and
  `CGO_ENABLED` changes.
- The Caddy probe found `ssaFunctions=78289`, `chaNodes=78290`, and root fanout
  `22` at the reverse-proxy invoke site. That is high enough to justify
  registry-keyed refinement, but not high enough to require VTA or pointer
  analysis for SPRINT-0006 acceptance.

## Consequences

- The compiler guarantees deterministic ordering for closure members, precision
  triggers, and refusal diagnostics under identical inputs. This is part of the
  shipped policy, not an implementation accident.
- Expected false positives remain on high-fanout interface sites outside the
  registry-keyed subset. Those are accepted for now as the cost of keeping the
  analysis simple, stable, and sprint-sized.
- A dynamic-plugin-like site may be accepted only when a registry key and the
  compiled package graph make the implementation set finite. Otherwise the site
  remains a refusal, not a reason to widen into whole-program analysis.
- Pointer-sensitive precision work is postponed to a measured follow-up. The
  next experiment is to quantify VTA false-positive reduction on the current
  Caddy and Pocketbase roots before any broader rollout or algorithm change.
