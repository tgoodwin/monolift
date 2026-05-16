# ADR-0030: Focused Admission Scope

**Status:** accepted
**Date:** 2026-05-16
**Context docs:** SPRINT-0050, SPRINT-0049 coverage report

## Context

Activation admission answers whether a specific candidate cut can be lifted
under the current codegen and harness contracts. A whole-repository package
load is the wrong unit for that question: it mixes unrelated app packages,
test-only/type-erroring regions, expensive augmentation paths, and entrypoints
that are not part of the target workload. The resulting timeout or package-load
failure says more about analysis scope than candidate viability.

The confusion came from the admission-only corpus sweep introduced for manifest
drift measurement. That sweep is useful for coarse before/after counts, but its
helper originally used `./...` as the package pattern and made it too easy to
reuse a broad sweep as focused research evidence.

## Decision

Focused admission probes must not use whole-repository `./...` package scope.
They must use one of:

- reverse-import scope rooted at the target file, where the expensive
  type-check/SSA step loads only target importers and the target package;
- an explicit package set chosen for the target workload, usually the target
  package plus the command or application entry package that reaches it.

Whole-corpus admission-only sweeps are allowed only as coarse manifest drift
measurements. They can identify rows that changed status or need follow-up, but
they cannot reject a durable-resource candidate when the failure is broad
package loading, augmentation cost, or timeout.

The primary interface for research probes should be a CLI or script that writes
explicit artifacts: source root, target, package scope, augmentation mode,
timeouts, selected cut, admission verdict, demotion chain, timings, and logs.
Go tests are regression gates for stable behavior, not the main research
driver. When an exploratory finding becomes a product invariant, encode that
invariant in a Go test; until then, keep the probe command and its artifacts
inspectable outside the test harness.

## Consequences

`TestAdmission` remains as a regression-friendly wrapper around the admission
pipeline. It refuses an explicit `./...` package scope and defaults to
reverse-import scoping when no package scope is supplied. Its timeouts are
flags, so longer focused research runs can increase budgets without editing
code. This test wrapper should not be treated as the long-term research UX; a
focused admission CLI should own that role.

Sprint plans and coverage reports must distinguish sweep evidence from focused
candidate evidence. A timeout from broad scope is invalid research evidence and
should trigger a narrower probe, not a target deferral.

## References

- `pkg/codegen/admission_sweep_test.go`
- `scripts/run_activation_corpus_sweep.sh`
- `docs/sprints/SPRINT-0050.md`
- `test/e2e/README.md`
