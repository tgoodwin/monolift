# SPRINT-0023 Liftpatch API Extension Hypothesis

## Hypothesis

SPRINT-0022 stopped at a tooling gap: `PatchRequest`/`PatchResult`/`PatchSymbolBody` can replace one free function, but a single lifted region can require multiple host stubs across receiver methods and packages. The additive `RegionPatchRequest` sibling can cover that shape without changing the legacy single-symbol API.

## Proposed shape

`RegionPatchRequest` contains:

- `RegionName`
- `Symbols []PatchSymbolRequest`
- `SharedGeneratedFiles []GeneratedFile`

Each `PatchSymbolRequest` carries package identity, package directory, optional file hint, function name, optional receiver type, expected signature, prelude, sentinel identifier, and per-symbol generated files. `RegionPatchResult` returns per-file hashes and generated-file records plus an optional structured refusal.

## Additive-only boundary

`PatchSymbolBody(req PatchRequest) (PatchResult, error)` remains the legacy contract for SPRINT-0019/0020 single-function fixtures. Its method-receiver rejection and result shape stay intact. `PatchRegion` is a sibling entry point with receiver support and multi-symbol coordination; routing to it is opt-in for multi-root or receiver-shaped regions.

## Required invariants

- Legacy caddy/miniflux/pocketbase compile fixtures must remain byte-identical.
- `PatchSymbolBody` must continue refusing receiver methods with `DiagnosticMethodReceiver`.
- Region sentinels must be deterministic and package-scoped: `monolift_<hash(regionName, packageImportPath)>_sentinel`.
- Duplicate symbol identities, signature mismatches, generated-file collisions, and receiver mismatches must refuse before partially writing source files.

## Discipline checklist

(a) The moved boundary is the liftpatch API only.

(b) The observed gap is documented in `docs/research/runs/SPRINT-0022-emission-gap.md`.

(c) The change is additive and keeps the old API surface intact.

(d) Tests cover both the new multi-symbol shape and legacy refusal behavior.

(e) If wiring the result into extract/e2e consumers spills beyond two packages outside `pkg/compiler/transport/emit/`, the sprint stops at Cliff 1.
