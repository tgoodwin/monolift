# Annotation Schema

Every target annotation in this directory must conform to this schema.

## Schema Fields

1.  **subsystem**: The architectural subsystem (e.g., ingress, persistence, domain service).
2.  **owned directories**: List of directories associated with this region.
3.  **region or operation identity**: module / package / symbol / kind / span.
4.  **admitted or refused**: Whether the compiler currently admits or refuses this region.
5.  **triage**: `ADMITTED` / `AUTO` / `SUGGEST` / `TERMINAL`.
    *   **ADMITTED**: Already liftable today.
    *   **AUTO**: Currently refused, but could be auto-lifted with an archetype transform.
    *   **SUGGEST**: Archetype fits but evidence is insufficient for safe auto-application; remediation suggestion.
    *   **TERMINAL**: No archetype fits; refusal stands.
6.  **proposed archetype**: The distribution archetype name (from the catalog).
7.  **proposed candidate state class**: For ADR-0016 (if different from existing).
8.  **proposed transform**: One-line sketch of what the compiler would generate.
9.  **competing archetypes considered**: Other candidates that were ruled out.
10. **evidence signals seen**: Cited to `docs/specs/liftability-properties.md` or `pkg/compiler/stateclass/`.
11. **missing evidence**: What would move `SUGGEST` → `AUTO`, or `TERMINAL` → `SUGGEST`.
12. **file references**: List of specific file paths and line numbers.

## Ambiguity and Classification Rules

- **Ambiguity**: If a region is ambiguous, flag the specific evidence gap.
- **Terminal Refusal**: Use when no distribution pattern fits and remediation is unlikely in v1.
- **Archetype Unclear**: If it fits multiple or needs a split, note this in competing archetypes.
- **Hybrid Archetype**: If a region needs to be split across archetypes, indicate the split point.
