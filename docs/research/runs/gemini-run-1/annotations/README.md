# Annotation Schema and Protocol

Every region in the target annotation files must use the following ten-field schema.

## Annotation Schema

1.  **subsystem**: The high-level subsystem the region belongs to (e.g., ingress, persistence, background).
2.  **owned directories**: The directory or directories containing the code for this region.
3.  **region or operation identity**: module / package / symbol / kind / span.
4.  **admitted or refused**: Status from the current compiler/extractor (ADMITTED or REFUSED).
5.  **triage**: 
    - **ADMITTED**: Already liftable today.
    - **AUTO**: Currently refused, but fits an archetype and evidence is sufficient for auto-lift.
    - **SUGGEST**: Archetype fits, but evidence is insufficient; compiler should suggest remediation.
    - **TERMINAL**: No archetype fits; refusal stands with no plausible remediation in v1.
6.  **proposed archetype**: The name of the distribution archetype (from the catalog).
7.  **proposed candidate state class**: The state class to be added to ADR-0016 to recognize this pattern.
8.  **proposed transform**: A one-line sketch of what the generated scaffolding/distribution code looks like.
9.  **competing archetypes considered**: Other archetypes that were considered but rejected.
10. **evidence signals seen**: Specific signals cited from `docs/specs/liftability-properties.md` or `pkg/compiler/stateclass/`.
11. **missing evidence**: What would move SUGGEST -> AUTO, or TERMINAL -> SUGGEST.
12. **file references**: Path and line number references.

## Triage Protocol

- **AUTO** is the primary research finding. It represents the "attainable" expansion of the lift surface.
- **TERMINAL** should be used sparingly, reserved for patterns that truly break distribution (e.g., hardware-specific pointers, deeply nested non-serializable OS state).
- **Ambiguity**: If an archetype is unclear, flag it with the specific evidence gap (e.g., "Archetype unclear pending evidence of keyed-access-only").
- **Hybrids**: If a region combines archetypes, split it into two annotation entries if possible.

## Target-level Synthesis

Every target annotation file MUST lead with a synthesis section containing:
- Dominant archetypes found.
- The **AUTO set**: regions that would become auto-liftable.
- Hardest ambiguities encountered.
- Most important evidence gaps for this target.
