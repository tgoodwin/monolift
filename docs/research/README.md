# Monolift research — index

Research artifacts accumulate here as sprints and inspiration work produce them. This file is the map: scan the sections below to find where a given effort's outputs live. Append a new section when a new research effort lands; don't file things at the root without updating this index.

Convention: canonical outputs (the ones future sprints cite) live at the top level under stable names. Source artifacts (individual model runs, intermediate drafts) live in subdirectories. Everything under `docs/research/` is tracked; per-sprint scratch that shouldn't live forever belongs in `/tmp/` or a sprint drafts directory.

---

## Inspiration research (pre-sprint, Apr 21 2026)

Long-form research notes from multiple models, produced during Monolift v2 scoping. These drove the paper-corpus manifest below and are the background reading behind ADRs 0015–0018. Not sprint outputs; kept as reference.

- `GEMINI-Monolift-Research.md` — Gemini's first pass on Monolift research directions.
- `GEMINI-Monolift-Research-II.md` — Gemini's second pass.
- `claude_comprehensive_research.md` — Claude's long-form survey.
- `claude_research_notes.md` — Claude's working notes.
- `codex-research-report.md` — Codex's research report.

## Paper-corpus infrastructure (built from inspiration research)

Bibliographic scaffolding for downloading and organizing the external research papers the inspiration notes referenced. Not research output itself; infrastructure for consuming related work.

- `manifest.yaml` — paper download manifest; batches, URLs, destination paths, tags.
- `fetch.sh` — download script; `./fetch.sh --batch N` for parallel workers.
- `download.md` — notes on the download workflow.
- `acm-manual-downloads.md` — papers requiring manual download (ACM paywall).
- `index/` — downloaded papers organized by category.

## SPRINT-0007 — canonical-shape classifier + state-class inference

State probes and spec-rules extracted during SPRINT-0007's classifier work. Referenced by ADR-0015 and ADR-0016.

- `SPRINT-0007-caddy-state-probe.json` — Caddy extract probe.
- `SPRINT-0007-pocketbase-state-probe.json` — PocketBase extract probe.
- `SPRINT-0007-spec-rules.md` — spec rules derived from the probes.

## SPRINT-0015 — archetype utility analysis (Apr 23 2026)

Follow-up research to SPRINT-0013 answering the complementary question: *given a liftable region, when does lifting it actually produce value, and when is it net-negative?* Three parallel runs (opus + gpt-5.4 + gemini) grounded in the PLOS '25 paper and v1 region annotations. See `utility-analysis/README.md` for the sprint-specific index.

**Canonical outputs:**

- `utility-analysis/utility-scenarios-v1.md` — narrative composite; primary artifact.
- `utility-analysis/archetype-catalog-v1.md` does not exist for this sprint (vocabulary unchanged from v1); see `utility-analysis/per-archetype-cards-v1.md` for per-archetype utility analysis.
- `utility-analysis/per-archetype-cards-v1.md` — utility cards per archetype.
- `utility-analysis/prioritization-implications-v1.md` — how utility reorders v1 prioritization.
- `utility-analysis/evaluation-ideas-v1.md` — concrete demo / benchmark scenarios.

**Source artifacts (preserved for transparency):**

- `utility-analysis/runs/opus/` — deep walk; two-axis structural model; dynamic-placement-eligibility as separate predicate.
- `utility-analysis/runs/gpt-5.4/` — operator-attention as utility cost; root-narrowing demo.
- `utility-analysis/runs/gemini/` — breakeven inequality framing; named scenario narratives.

## SPRINT-0013 — distribution-archetype transforms (Apr 22 2026)

Multi-model parallel research on distribution archetypes, the auto-lift-vs-suggest boundary, and candidate ADR-0016 state-class additions. The large composite effort — ~200 KB across canonical + source artifacts. See `../sprints/SPRINT-0013.md` for the plan and closeout.

**Canonical outputs (merged composite across three parallel runs):**

- `distribution-archetypes-v1.md` — narrative research note; primary artifact.
- `archetype-catalog-v1.md` — the 8-archetype catalog with per-gate pass records, retirements, cross-target citation matrix.
- `distribution-archetypes-followups.md` — four buckets (state-class additions for ADR-0016, ADRs ripe to draft, still-open empirical questions, implementation spikes).
- `annotations/README.md` + `annotations/{caddy,pocketbase,miniflux,listmonk,gitea,mattermost}.md` — per-target composite summaries with cross-run attribution.

**Source artifacts (individual runs, preserved for transparency):**

- `runs/opus/` — deepest walk; 7-archetype vocabulary; structural two-axis boundary model; 5 evidence-signal proposals.
- `runs/gpt-5.4/` — concise 4-archetype vocabulary; per-archetype threshold framing; `connection-hub-buffer` composite lens.
- `runs/gemini/` — broad bundle-coverage walk; contributed `filesystem-bound-singleton` archetype.
- `runs/gemini-run-1/` — first gemini attempt; sampled rather than exhaustive (violated the sprint's no-sampling fence). Preserved for transparency.
- `runs/gemini-run-2/` — second gemini attempt; MCP tool-infrastructure failure blocked subagent delegation. Preserved for transparency.

## Inspiration research (post-sprint, May 2026)

Targeted research into the industry shift back toward modular monoliths, focusing on economic and operational virtues identified in the 2016–2026 decade. These findings ground the Monolift "extraction path" philosophy.

- `modular-monolith-virtues-v1.md` — Synthesis of virtues, case studies (Amazon, Segment), and DDD technical blueprints.

---

## How to add a new research effort

When a sprint or research pass produces outputs here:

1. Decide which outputs are canonical (future sprints will cite them by name) vs. source (individual runs, intermediate drafts). Canonical at the top level, source in a subdirectory.
2. Append a section to this README grouping the new artifacts, one line per file with what-it-is. Include the sprint pointer if applicable.
3. Use stable filenames for canonical artifacts (`<topic>-v1.md`, not `<topic>-final-v2-real-final.md`) so cross-references stay stable.
4. If an effort gets a follow-up (v2 research, a later sprint that extends it), append the new version as its own section and keep v1 in place; don't overwrite.
