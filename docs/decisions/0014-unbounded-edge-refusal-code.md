# ADR-0014: Unbounded-edge refusal code taxonomy

**Status:** accepted _(SPRINT-0006)_
**Date:** 2026-04-20
**Context docs:** `docs/specs/monolift-v2-contract.md` §Extraction Root and Closure; §Refusal Diagnostic Index

## Context

SPRINT-0006 introduces compiler-owned SSA closure extraction plus explicit
refusals for unresolved edges. The v2 contract already names three refusal
codes in this area:

- `MLV2_REFLECTION_DISPATCH` for unresolved reflection-driven dispatch
- `MLV2_DYNAMIC_PLUGIN` for unresolved dynamic plugin loading
- `MLV2_DISPATCH_SET_UNBOUNDED` for interface dispatch sets that are not
  statically bounded

That taxonomy is incomplete for non-dispatch unbounded edges discovered during
closure walking. In particular, `unsafe.Pointer` crossings and opaque
function-value escapes can make the extraction frontier unbounded without being
accurately described as reflection, plugin loading, or interface dispatch.

SPRINT-0006 needs one stable taxonomy decision before compiler code starts
emitting these refusals.

## Decision

SPRINT-0006 adds `MLV2_CLOSURE_UNBOUNDED` to the v2 contract as an umbrella
refusal for non-dispatch unbounded closure edges.

Scope boundaries:

- Keep `MLV2_REFLECTION_DISPATCH` for unresolved reflection-driven dispatch.
- Keep `MLV2_DYNAMIC_PLUGIN` for unresolved plugin loading.
- Keep `MLV2_DISPATCH_SET_UNBOUNDED` for interface-dispatch-set growth.
- Use `MLV2_CLOSURE_UNBOUNDED` for unresolved closure edges that make the
  frontier non-finite but are not more specifically dispatch-taxonomy failures,
  including `unsafe`-mediated crossings and opaque function-value escapes.

`MLV2_CLOSURE_UNBOUNDED` does not replace `MLV2_CLOSURE_TOO_LARGE`.
`MLV2_CLOSURE_TOO_LARGE` remains the pruning/size refusal after bounded-edge
rules have been applied. `MLV2_CLOSURE_UNBOUNDED` is the earlier taxonomy for
an extraction frontier that cannot be made finite under the selected build and
analysis precision.

## Consequences

- SPRINT-0006 can refuse `unsafe`-mediated closure escapes without overloading
  reflection or plugin-specific codes.
- Future closure-walker work inherits a stable split:
  dispatch-specific unboundedness keeps its specific codes, while other
  unbounded frontier cases converge on `MLV2_CLOSURE_UNBOUNDED`.
- The v2 contract must add `MLV2_CLOSURE_UNBOUNDED` to the Refusal Diagnostic
  Index and clarify its role in §Extraction Root and Closure.
