# ADR-0012: Pragma parser diagnostic boundaries

**Status:** accepted _(SPRINT-0005)_
**Date:** 2026-04-20
**Context docs:** `docs/specs/monolift-v2-contract.md` §Pragma Syntax v2 and §Refusal Diagnostic Index; `docs/decisions/0011-harness-before-compiler.md`

## Context

SPRINT-0005 replaces the v1 pragma parser in `pkg/compiler/pragma.go` with a
v2 parser. The parser is the first production-facing component to emit
`MLV2_PRAGMA_*` diagnostics, but the closure-report diagnostic schema lives in
`pkg/compiler/reportv2`.

The v2 contract defines the pragma grammar and diagnostic index, but it leaves
two parser-edge cases implementation-defined for this sprint:

- Missing required keys, such as `name`, do not have a dedicated
  `MLV2_PRAGMA_MISSING_REQUIRED_KEY` code.
- Duplicate option keys within one pragma line are not assigned an explicit
  diagnostic code.

## Decision

SPRINT-0005 uses these decisions:

- The parser owns a parser-internal `Diagnostic` type. It does not construct
  or import `reportv2.Diagnostic`.
- The stub compiler is the only SPRINT-0005 translation seam from
  parser-internal diagnostics into closure-report diagnostics.
- Missing required keys emit `MLV2_PRAGMA_INVALID_KEY_FOR_SURFACE`, with the
  missing key named in the message.
- Duplicate option keys within a single pragma line emit `MLV2_PRAGMA_PARSE`.
- A mechanical import-boundary test prevents `pkg/compiler/pragma*` files from
  importing `pkg/compiler/reportv2`.

## Consequences

- `pkg/compiler/pragma*` can be tested without depending on the closure-report
  schema package.
- Spec drift for `MLV2_PRAGMA_*` codes is caught by a parser-package test that
  compares parser constants against the v2 contract's Refusal Diagnostic Index.
- Parser diagnostics carry enough span and suggestion information for the
  harness seam, but they remain independent of report serialization details.
- Required-key failures and duplicate in-line keys have stable, documented
  diagnostic mappings until the v2 contract introduces more specific codes.
- Production movement of the translation seam belongs to the later
  refusal-diagnostic framework epic; SPRINT-0005 keeps it in the stub compiler
  only.
