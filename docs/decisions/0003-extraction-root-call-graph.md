# ADR-0003: Extraction root is call-graph-driven, not `main`-walk

**Status:** accepted _(v2 spec v1.0, 2026-04-19)_
**Date:** 2026-04-19
**Context docs:** `docs/specs/monolift-v2-contract.md` §Extraction Root, `docs/evaluation/generalization-analysis-2026-04-19.md` §"What consistently breaks" #1

## Context

v1's `pkg/compiler/compiler.go:952` (`resolveDependencies()`) reconstructs the
service dependency graph by walking variable declarations in `func main()` —
assuming the developer has assembled services there with `New<Iface>(...)`
constructor calls.

The audit found this holds in **0 / 6** real targets. Real wiring lives in:

- `init()` chains (gitea's `routers/init.go:InitWebInstalled(ctx)`)
- Functional-Options builders (mattermost's `app.NewServer(options...)`)
- Plugin registries driven by blank imports + `init()` (caddy's `_ "modules/standard"`)
- Lifecycle hooks fired post-construction (pocketbase's `OnBootstrap`)
- Direct `App{...}` struct-literal assembly (listmonk)
- CLI-delegating `main()` (miniflux's `cli.Parse()`)

No single wiring convention covers real Go monoliths. Patching the
`main`-walker to handle each pattern would be an endless whack-a-mole.

## Decision

Replace `main`-walk extraction with **call-graph / SSA-based transitive
closure from the annotated root**. The compiler ignores *where* wiring happens
— it only cares about the reachable value graph at program start.

- Expected analysis substrate: `golang.org/x/tools/go/ssa` (non-normative; the
  spec names the substrate as a candidate but keeps internal data structures
  out of the contract).
- The closure includes reachable functions, package-level vars the closure
  reads/writes, and reachable types.
- The closure excludes stdlib, external modules, cgo, reflection-driven
  dispatch (refused), build-tag-gated code, dynamic plugin loading, generated code.
- Wiring idioms (`init()`, Options, `Register(...)`, hooks) all resolve to
  values in the same program-init value graph — the compiler treats them uniformly.
- Boundary-pruning rules + a "closure too large" refusal diagnostic prevent a
  small lift from accidentally absorbing the whole monolith.

## Consequences

- `resolveDependencies()` and its supporting machinery are retired in v2.
- Defines a new **closure report** artifact (ADR referenced by v2 spec §Extraction Root):
  the required output of extraction analysis — included symbols, captured state,
  external deps, refusals. This is the one sanctioned interface between the
  v2 spec and the v2 compiler implementation.
- The compiler becomes indifferent to wiring-source code style — enabling
  targets (caddy, pocketbase) whose wiring is structurally different from the demo's.
- Shifts the "how do I know what to extract?" question from syntactic
  (where's the constructor call?) to semantic (what's reachable from the annotated symbol?).
- Opens a new failure mode — closures that balloon past usable size — which
  the spec addresses via pruning rules and an explicit refusal diagnostic.

## References

- `pkg/compiler/compiler.go:952` — v1's `resolveDependencies()`, to be retired.
- `docs/specs/monolift-v2-contract.md` §Extraction Root, §Closure Report.
- `docs/evaluation/generalization-analysis-2026-04-19.md` — §"Wiring doesn't live in main" (6/6 violations).
- ADR-0002 (renegotiate contract) — parent decision.
