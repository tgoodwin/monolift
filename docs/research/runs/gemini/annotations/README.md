# Annotation Schema and Protocol

All annotations in this research sprint MUST follow the schema below. This ensures mechanical comparability across targets and subsystems.

## Annotation Schema (Ten Fields)

1.  **Subsystem**: The logical bundle or component (e.g., `ingress`, `persistence`).
2.  **Owned Directories**: List of directories covered by this entry.
3.  **Region or Operation Identity**: Module/Package/Symbol/Kind/Span (e.g., `services/user.CreateUser`).
4.  **Admitted or Refused**: Current status in the Monolift compiler.
5.  **Triage**: `ADMITTED`, `AUTO`, `SUGGEST`, or `TERMINAL`.
6.  **Proposed Archetype**: The candidate distribution archetype (from `archetype-catalog-v1.md`).
7.  **Proposed Transform**: A one-line sketch of the distribution transform.
8.  **Evidence Signals Seen**: Cited to `liftability-properties.md` or `stateclass`.
9.  **Missing Evidence**: What would move `SUGGEST` → `AUTO` or `TERMINAL` → `SUGGEST`.
10. **File References**: Path and line number citations.

## Triage Definitions

- **ADMITTED**: Already liftable today under existing rules.
- **AUTO**: Currently refused, but fits an archetype and evidence is sufficient for auto-lift.
- **SUGGEST**: Archetype fits, but evidence is insufficient for safety; surfaces as a remediation suggestion.
- **TERMINAL**: No archetype fits; refusal stands with no plausible remediation in v1.

## Protocol for Ambiguity

- **Flagging**: If a region fits multiple archetypes or none perfectly, flag it.
- **Hybrid Archetypes**: If a region needs to be split to be lifted, note the split point.
- **Evidence Gaps**: If an archetype is "unclear pending evidence X", name X explicitly.
