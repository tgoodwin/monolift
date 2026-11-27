# GoLean liftability experiment

This opt-in experiment explores the proposal's §5 contract for value-only
boundaries and explicit communication failures. It is research for a newer
Monolift, independent of the existing compiler and runtime.

## What it checks

`testdata/negate/main.go` is a real Go value-only operation:

```go
negate(bool) (bool, error)
```

The error result is explicit and is `nil` for both observations. The experiment
runs the source with Go (`false` and `true`), lowers the same source with
GoLean's native frontend, decodes that wire in Lean, and executes the decoded
program with GoLean's interpreter. The runner compares the two observations
line by line, including whether the Go error result is nil.

`MonoliftLean.SingleAttempt` contains the deliberately small transport model.
It has one attempt only:

| delivery event | caller sees | modelled callee executions |
| --- | --- | ---: |
| delivered reply | the decoded result | 1 |
| request lost | the communication error `lost` | 0 |
| reply lost | the same communication error `lost` | 1 |

There is no retry or local-fallback transition. The proved theorem
`delivered_preserves_direct` compares one delivered attempt with direct
execution under explicit request and reply codec round-trip assumptions.
`negate_delivered_preserves_direct` specializes that theorem to the GoLean
interpreter entry point and identity codecs. For each input, the runner checks
all three outcomes against the actual freshly decoded callee. The loss theorem
assumes a completed callee for the lost-reply case; evaluator stops or unexpected
readouts fail the experiment rather than masquerading as communication errors.

The Go comparison is an observation, not a proof about the frontend or
interpreter. This experiment inherits trust in both, does not model non-nil Go
error values or codec corruption, and says nothing about arbitrary Go,
production RPC, ownership, exactly-once effects, or compiler soundness.

## Run from a clean checkout

Requirements:

- Lean via `elan`; `lean-toolchain` pins `leanprover/lean4:v4.35.0`.
- Go via the pinned upstream expectation `go1.26.5`; the script uses
  `GOTOOLCHAIN=go1.26.5` and may download that toolchain through Go's normal
  toolchain mechanism.
- Git, Bash, and a C compiler for Lake's native executable build.
- Network access on the first run so Lake can fetch the pinned GoLean commit.
  No Mathlib or other Lean package is used.

From the repository root:

```sh
cd lean
./run-experiment.sh
```

Expected output includes:

```text
Go and GoLean agree (input, output, error == nil):
false true true
true false true
Single-attempt checks passed: delivered, request lost, reply lost.
```

The script runs only this experiment. It does not run Monolift suites or the
GoLean upstream corpus gate.

## Provenance and regeneration

- GoLean source is fetched by `lakefile.toml` at commit
  `9b84b590de541bf0b675badabc000fbb0f2e12b7` from
  `https://github.com/OathTech/golean.git`; `lake-manifest.json` records the
  same revision.
- GoLean declares Lean `v4.35.0` and no external Lean dependencies at that
  revision. The experiment's `lean-toolchain` repeats that pin.
- The native frontend is built from that fetched checkout. It lowers the
  checked-in `testdata/negate/main.go` with target `linux/amd64` and writes a
  temporary wire (removed at script exit). The Go observation runs on the host;
  the fixture uses only Booleans and a nil error, not platform-specific features.
- The wire is regenerated on every run; no lowered fixture is checked in. A
  clean checkout therefore needs only the source, pins, and the reproducible
  runner.
