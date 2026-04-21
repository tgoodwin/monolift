# ADR-0004: Generalized annotation surface beyond interfaces

**Status:** accepted _(v2 spec v1.0, 2026-04-19)_
**Date:** 2026-04-19
**Context docs:** `docs/specs/monolift-v2-contract.md` §Annotation Surface, `docs/evaluation/generalization-analysis-2026-04-19.md` §"What consistently breaks" #2

## Context

v1 requires pragmas on interface declarations:

```go
// @monolift trigger=CPU threshold=0.5
type Service interface { ... }
```

The audit found that real Go monoliths mostly don't use interfaces for their
service layer. Interfaces, when present, are for *adapters* — multiple mailer
backends, OAuth providers, storage drivers — not for carving service
boundaries. Business logic lives on **concrete structs with methods** or on
**package-level functions**.

Keeping v1's interface-only annotation means almost no real code can be
annotated without being refactored first — which violates the pay-as-you-go
adoption promise.

## Decision

Accept pragmas on all of:

- **Interface declarations** (v1 carryover; now one case among several, no longer privileged)
- **Package-level function declarations** (targets: miniflux `ProcessFeedEntries`, listmonk campaign `worker()`)
- **Methods with concrete receivers** (targets: mattermost `(us *UserService) CreateUser`, gitea `services/user/RenameUser`)
- **Struct type declarations** (inferred public-method surface; targets: caddy module structs)

Explicit per-form decisions (accept / defer / refuse) covering interface
methods, function values in vars, anonymous functions, generic instantiations,
and whole packages are recorded in the v2 spec.

## Consequences

- The compiler's entry-point to "what is being lifted?" becomes any of the
  four annotation surfaces, not just interfaces.
- Combined with ADR-0003 (call-graph extraction), this means a developer can
  annotate a single function deep in a codebase and the compiler can extract
  it regardless of where it's wired.
- Pragma syntax must encode which surface is being annotated, or infer it
  from the annotated symbol's declaration.
- The "unique implementer" requirement from v1 is demoted — see ADR-0005 for
  state semantics and the spec's §Multi-implementer handling for disambiguation.
- Some surfaces (generic instantiations, anonymous functions) are deliberately
  refused in v2 to bound compiler complexity.

## References

- `docs/specs/monolift-v2-contract.md` §Annotation Surface.
- `docs/evaluation/generalization-analysis-2026-04-19.md` — §"Service interface is rare" (6/6 either ❌ or ⚠️).
- `pkg/compiler/pragma.go` — v1's interface-only parser, to be generalized.
- ADR-0002 (renegotiate contract) — parent decision.
