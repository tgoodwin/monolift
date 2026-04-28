# Subagent Delegation Prompt Template — Monolift SPRINT-0013

**Objective**: Walk the provided bundle of directories in the `<target>` repository and produce exhaustive region-by-region annotations for Monolift distribution archetypes.

## Context
You are an expert Go developer and distributed systems researcher. You are contributing to the Monolift research project, which aims to automatically lift monolith components into distributed services.

## Input
- **Target**: <target>
- **Bundle Name**: <bundle_name>
- **Directories**: <directories>
- **Reference Archetypes**: Use the vocabulary in `docs/research/runs/gemini/archetype-catalog-v1.md`.
- **Annotation Schema**: Use the ten-field schema defined in `docs/research/runs/gemini/annotations/README.md`.

## Task
1. **Walk every Go file** in the provided directories.
2. **Identify regions or operations** (functions, methods, structs) that are either already admitted by Monolift or currently refused.
3. **Apply the AUTO / SUGGEST / TERMINAL triage** to every refused region.
4. **For every AUTO region**, propose a named **archetype**, a **transform** (one-line sketch), and a **candidate state class** for ADR-0016.
5. **Cite specific evidence** from `docs/specs/liftability-properties.md` or `pkg/compiler/stateclass/`.
6. **Flag ambiguities** where an archetype fit is unclear.

## Constraints
- **Exhaustiveness**: Every directory in the bundle must be covered. Thin returns will be rejected.
- **Accuracy**: Cite exact file paths and line numbers.
- **Discipline**: Use the v0 vocabulary; only flag new candidate archetypes, do not promote them yourself.
- **Format**: Return a Markdown list of annotations following the ten-field schema.

## Return Format
For each region:
1. Subsystem:
2. Owned Directories:
3. Region or Operation Identity:
4. Admitted or Refused:
5. Triage (ADMITTED / AUTO / SUGGEST / TERMINAL):
6. Proposed Archetype:
7. Proposed Transform:
8. Evidence Signals Seen:
9. Missing Evidence:
10. File References:
