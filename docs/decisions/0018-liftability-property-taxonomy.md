# ADR-0018: Liftability property taxonomy

**Status:** accepted
**Date:** 2026-04-21
**Context docs:** `docs/decisions/0017-classifier-reasons-about-liftability.md`, `docs/specs/liftability-properties.md`

## Context

ADR-0017 reframes admissibility around liftability properties instead of
literal canonical-shape matches. To keep that reframe stable in code and
reports, the property names and Go IDs need their own narrow decision record.

## Decision

The SPRINT-0009 classifier uses the named property set defined in
`docs/specs/liftability-properties.md`. ADR-0018 freezes:

- property names
- Go `PropertyID` identifiers
- outcome classes (`gate`, `bias`, `advisory`)

This ADR does not freeze detector internals, evidence wording, or selector
policy beyond what the spec records. Future properties append to the set in
new decision records rather than reopening ADR-0017.

## Consequences

- The compiler, reports, tests, and docs can share one property vocabulary.
- Transport-shape signals remain available, but they no longer substitute for
  the named admissibility properties.
- Future taxonomy growth is incremental instead of destabilizing the whole
  classifier rewrite.

## Implementation Notes

PropertyID is a typed Go constant; bare-string property IDs are forbidden, enforced by `property_lint_test.go`.

SPRINT-0017 adds archetype-evidence property IDs for ADR-0022 candidate construction: `state.mutex-encloses-store-invariant`, `state.receiver-owned-state`, and `state.keyed-access-invariant`. These IDs describe evidence used to match state archetypes; they are non-gating and do not directly change root liftability admission.
