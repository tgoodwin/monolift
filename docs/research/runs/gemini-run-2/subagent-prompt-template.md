# Subagent Delegation Template: Corpus Walk

## Task
Perform an exhaustive walk of the Go source files in the assigned subsystem bundle and annotate distribution archetypes for Monolift auto-lifting.

## Assigned Bundle
- **Target**: {{target}}
- **Subsystem**: {{subsystem}}
- **Owned Directories**: {{directories}}
- **File Count**: {{file_count}}

## Requirements
1. **Exhaustiveness**: You must read EVERY Go file in the assigned directories. Sampling is forbidden.
2. **Annotation Schema**: For every region of interest (stateful code, goroutines, channels, sync primitives), produce an entry with exactly these fields:
    - `subsystem`: {{subsystem}}
    - `owned directories`: {{directories}}
    - `region or operation identity`: module/package/symbol/kind/span
    - `admitted or refused`: current compiler status (check existing reports if available)
    - `triage`: AUTO / SUGGEST / TERMINAL
    - `proposed archetype`: from the catalog (singleton-actor, worker-pool, etc.)
    - `proposed candidate state class`: (e.g., singleton-mutable)
    - `proposed transform`: (one-line sketch)
    - `competing archetypes considered`: (list)
    - `evidence signals seen`: (cite liftability-properties or stateclass)
    - `missing evidence`: (what moves SUGGEST -> AUTO)
    - `file references`: (path:line)
3. **AUTO vs SUGGEST vs TERMINAL**:
    - **AUTO**: Refused today, but fits a known archetype with sufficient evidence for an automated transform.
    - **SUGGEST**: Archetype fits, but evidence is weak or needs user confirmation/pragma.
    - **TERMINAL**: No distribution pattern fits; must remain local.
4. **No Promotion**: Do not create new archetype names in the v1 catalog. If you find a pattern that fits none of the v0 archetypes, label it `proposed archetype: [FLAG] <name>` and explain why.

## Return Format
- A list of annotations following the schema.
- A summary of the dominant patterns found in this bundle.
- Any ambiguities or evidence gaps that prevent a firm AUTO classification.
