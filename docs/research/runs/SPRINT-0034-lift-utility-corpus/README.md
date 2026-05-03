# SPRINT-0034 — Lift-utility candidate corpus

**Status:** in progress (research only; no implementation yet)
**Started:** 2026-05-02
**Predecessor:** SPRINT-0033 retired after Phase 0; the new research question is *"What is the smallest static graph whose edges are meaningful enough that a path from application roots to region roots corresponds to a real activation path?"* (see `docs/sprints/SPRINT-0033.md`).

## Goal

Build an empirical corpus of **useful lift regions** across all six pinned evaluation targets, then — in a follow-up phase — answer the smallest-static-graph question independently for each region. The aggregate of those independent answers is the evidence base for a generalizable activation-graph schema.

This corpus differs from SPRINT-0033's 34-candidate catalog in one critical way: that catalog selected for **structural diversity of activation shapes** (algorithm-test inputs). This corpus selects for **lift utility** — regions whose remote-execution would plausibly benefit the application under load.

## Methodology

### Phase 1 — Per-project candidate drafts (trio fan-out)

For each of the six projects, three independent model agents (claude, codex, gemini) read the project's source tree under `evaluation/<project>/` and produce a draft candidate set, scored against the rubric in `rubric.md`.

- 6 projects × 3 models = **18 draft documents**
- Each draft proposes 5–12 candidate regions, ranked by lift-utility.
- Each candidate must include: name, region root file:line, why-useful reasoning grounded in the rubric, state-coupling note, and a confidence/risk flag.

Outputs land at `projects/<project>/{claude,codex,gemini}.md`.

### Phase 2 — Cross-review within each trio

For each project, each model reviews the *other two* drafts and flags candidates that are bad picks (violate the rubric, are not real bottlenecks, or are tightly state-coupled). 6 projects × 3 reviewers = **18 cross-review documents** at `projects/<project>/cross-review-{claude,codex,gemini}.md`.

### Phase 3 — Per-project merged candidate set

Consolidate the three drafts plus the three cross-reviews into one ranked candidate set per project: `projects/<project>/merged.md`. Candidates that all three models picked, or that survived all three cross-reviews, ranked highest. Disputed candidates kept with the dispute recorded.

### Phase 4 (deferred) — Smallest-graph analysis per candidate

Once the merged candidate set is stable, kick off a separate fan-out where agents reason about the smallest static graph that connects each candidate to the binary entrypoint. The aggregate of those independent graph derivations is the input to the eventual activation-graph schema design.

## File layout

```
SPRINT-0034-lift-utility-corpus/
├── README.md                 — this file
├── rubric.md                 — selection criteria for "useful lift region"
├── prompt-template.md        — the prompt fed to each model in Phase 1
└── projects/
    ├── caddy/
    │   ├── claude.md         — Phase 1 draft
    │   ├── codex.md          — Phase 1 draft
    │   ├── gemini.md         — Phase 1 draft
    │   ├── cross-review-claude.md  — Phase 2
    │   ├── cross-review-codex.md   — Phase 2
    │   ├── cross-review-gemini.md  — Phase 2
    │   └── merged.md         — Phase 3
    ├── gitea/      — same structure
    ├── listmonk/   — same
    ├── mattermost/ — same
    ├── miniflux/   — same
    └── pocketbase/ — same
```

## Status table

| Project | Phase 1 (drafts) | Phase 2 (cross-review) | Phase 3 (merged) |
|---|---|---|---|
| caddy | claude ✓ · gemini ✓ · codex pending | — | — |
| gitea | claude ✓ · gemini ✓ · codex pending | — | — |
| listmonk | claude ✓ · gemini ✓ · codex pending | — | — |
| mattermost | claude ✓ · gemini ✓ · codex pending | — | — |
| miniflux | claude ✓ · gemini ✓ · codex pending | — | — |
| pocketbase | claude ✓ · gemini ✓ · codex pending | — | — |

Codex round deferred to ≥2026-05-04 22:35 due to upstream usage limit on the primary machine.

## Running the codex round on a different machine

The codex Phase 1 runs are portable. On a machine with codex quota:

```bash
git clone git@github.com:tgoodwin/monolift.git
cd monolift

# Hydrate evaluation/<project>/ at the pinned SHAs from evaluation/MANIFEST.yaml.
docs/research/runs/SPRINT-0034-lift-utility-corpus/clone-evaluation-targets.sh

# Fire the codex round (one project per invocation).
for proj in caddy gitea listmonk mattermost miniflux pocketbase; do
    docs/research/runs/SPRINT-0034-lift-utility-corpus/run-phase1.sh codex "$proj"
done

# Commit and push the resulting drafts.
git add docs/research/runs/SPRINT-0034-lift-utility-corpus/projects/*/codex.md
git commit -m "Add codex Phase 1 drafts for SPRINT-0034"
git push origin main
```

Then pull on the primary machine and resume with Phase 2.

(Cells become ✓ as outputs land.)
