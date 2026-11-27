# Lean liftability model

This directory contains the first formal model for the thesis's liftability
contract. It is an independent Lean model, not a Go compiler and not a network
transport model.

## What it models

`MonoliftLean.Liftability` defines:

- placement modes: stateless, singleton-owned state, and external state;
- an ownership vocabulary for caller, region, and external locations;
- abstract reads, writes, and alias escapes;
- an explicit error-channel requirement;
- a safety predicate for traces;
- a frame theorem showing that an admitted trace preserves caller-owned state.

The examples cover a pure value operation, caller-owned slice mutation,
caller-alias escape, singleton-owned receiver mutation, external state,
and a missing error channel. The trace examples are deliberately written in
Lean today. The Go fixtures in `testdata/liftability/main.go` are the small
programs that the future SSA-to-liftability adapter should summarize.

The current model proves properties of the abstract summary. It does not yet
prove that a Go SSA analysis produces a sound summary for arbitrary Go. That
is a separate integration and soundness step.

## Run

Requirements:

- Lean via `elan`; `lean-toolchain` pins `leanprover/lean4:v4.35.0`.
- Bash and a C compiler for Lake's native executable build.

From the repository root:

```sh
cd lean
./run-liftability-check.sh
```

Expected output:

```text
Liftability checks passed: pure, aliasing, singleton state, external state, and error channel.
```

The check is intentionally self-contained. It does not analyze Go source yet,
run a Go interpreter, or model network serialization.
