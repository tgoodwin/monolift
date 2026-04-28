# Per-target annotations — composite across three parallel runs

This directory contains cross-run composite summaries for each of the six evaluation targets walked in SPRINT-0013. Each `<target>.md` file is a compact synthesis of what the three parallel runs (opus, gpt-5.4, gemini) found in that target — convergences, divergences, and single-run-only findings — with pointers to the per-run depth.

**Where to go for depth:** the detailed per-region walks live in the run directories under `../runs/<agent>/annotations/<target>.md`. Those are preserved as source artifacts. The composite here points into them rather than duplicating 200+ KB of prose.

## Annotation schema (frozen in SPRINT-0013 Phase 0)

Every region in every run's annotation uses:

- `subsystem`, `owned directories`
- `region / operation identity` (module / package / symbol / kind / span)
- `admitted or refused` (classifier's current verdict)
- `triage` — one of: `ADMITTED` (already liftable today), `AUTO` (primary research finding — currently refused but would be auto-liftable with a named archetype + transform), `SUGGEST` (archetype fits but evidence insufficient for auto-apply), `TERMINAL` (no archetype fits, refusal stands)
- `proposed archetype`
- `proposed candidate state class for ADR-0016` (where different from an existing state class)
- `proposed transform` (one-line sketch)
- `competing archetypes considered`
- `evidence signals seen` (cited to `docs/specs/liftability-properties.md` or `pkg/compiler/stateclass/`)
- `missing evidence` (what would move SUGGEST → AUTO, or TERMINAL → SUGGEST)
- `file references`

## Composite structure per target file

Each per-target composite follows the same shape:

1. **Cross-run summary** — one-paragraph target-level view combining all three runs.
2. **Triage convergences** — regions where all three runs agree on AUTO / SUGGEST / TERMINAL (or where only opus+gpt-5.4 found a region; gemini was terser overall).
3. **Divergences and single-run findings** — regions where runs labeled differently, or where only one run caught something distinctive. Worth surfacing because the parallel-runs design is specifically meant to capture these.
4. **Pointers** — direct paths to each per-run file for depth.

## Run provenance

- `../runs/opus/annotations/<target>.md` — deepest walks, most line-level citations, 7-archetype vocabulary.
- `../runs/gpt-5.4/annotations/<target>.md` — concise walks, aggressive merging into 4-archetype vocabulary, strong per-archetype boundary framing.
- `../runs/gemini/annotations/<target>.md` — broad coverage especially for gitea/mattermost bundle enumeration; `filesystem-bound-singleton` contributions.
- `../runs/gemini-run-1/` and `../runs/gemini-run-2/` — earlier gemini attempts preserved. Run 1 was sampled (explicitly forbidden); run 2 hit an MCP tool-infrastructure failure. Run 3 (the current `gemini/`) was dispatched with MCP fully disabled and covers all six targets with all 12 gitea+mattermost bundles enumerated.
