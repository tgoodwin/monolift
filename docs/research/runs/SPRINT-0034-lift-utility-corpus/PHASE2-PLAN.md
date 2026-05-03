# SPRINT-0034 Phase 2 Plan — cross-critique and aggregation

**Status:** drafted, pending user review before scripts/runners are written.
**Predecessor:** Phase 1 produced 18 independent draft candidate sets (3 models × 6 projects); see `README.md`.

This plan mirrors the `sprint-planner` skill's pattern: *independent drafts → cross-critiques → merged final*. Phase 1 produced the drafts. Phase 2a has each model critique the other two's drafts. Phase 2b has a single aggregator merge each project's drafts + critiques into a final candidate set.

## File layout (additions)

```
projects/<project>/
├── claude.md                # Phase 1 draft (existing)
├── codex.md                 # Phase 1 draft (existing)
├── gemini.md                # Phase 1 draft (existing)
├── critique-by-claude.md    # Phase 2a: claude reviews codex + gemini drafts
├── critique-by-codex.md     # Phase 2a: codex reviews claude + gemini drafts
├── critique-by-gemini.md    # Phase 2a: gemini reviews claude + codex drafts
└── merged.md                # Phase 2b: aggregator-produced final candidate set
```

24 new files total: 18 critiques (3 per project × 6 projects) + 6 merged sets.

---

## Phase 2a — Cross-critique

### Methodology

For each project, each of the three models reviews the **other two** drafts (it does not re-review its own — its own draft already encodes its judgment). The critic produces a structured per-candidate verdict (KEEP / DROP / MODIFY) for every candidate in the two foreign drafts, plus an OVERLOOKED section nominating regions all three drafts missed.

The critic must read:

- The rubric: `docs/research/runs/SPRINT-0034-lift-utility-corpus/rubric.md`
- Its own draft (for self-anchoring)
- The two foreign drafts (the ones being critiqued)

### Verdict types

- **KEEP** — candidate passes the rubric and should remain in the merged set.
- **DROP** — candidate fails the rubric, or is structurally weaker than something else (cite which alternative is better).
- **MODIFY** — candidate identifies a real lift target but the framing is wrong: incorrect line cite, conflated scope, marginal pick where the adjacent region is the real lift, etc. Specify the modification concretely.

The critic also produces an **OVERLOOKED** section listing regions all three drafts missed that the critic believes should be in the merged set, with the full per-candidate rubric format from `rubric.md`.

### Output format

```markdown
# Critique of <project> drafts by <reviewer>

## Verdicts on <other1>'s draft

| Candidate ID | Verdict | One-paragraph reasoning |
|---|---|---|
| C-1 (...) | KEEP | rubric criterion satisfied: ... |
| C-2 (...) | MODIFY | line cite drift: code is at file:NNN not file:MMM ... |
| C-3 (...) | DROP | fails state-independence: ... |
| ... | ... | ... |

## Verdicts on <other2>'s draft

[same table format]

## Overlooked

(Regions all three drafts missed. Use the per-candidate format from the rubric. May be empty.)

### O-1: <short name>
- (full per-candidate format from rubric.md)

## Overall observations

(2–4 sentences: where do the two drafts converge cleanly? where do they meaningfully diverge? is one model systematically more rigorous on a particular criterion?)
```

### Phase 2a prompt template (will be saved as `prompt-phase2-critique.md`)

```
You are reviewing two peer agents' lift-region candidate sets for the project ${PROJECT_NAME}, in support of the Monolift research project.

Background: see ${SPRINT_ROOT}/README.md and ${SPRINT_ROOT}/rubric.md for the methodology and selection criteria. Phase 1 of this sprint produced three independent candidate drafts per project; you are entering the cross-critique phase.

You are the critic ${REVIEWER}. Your own Phase 1 draft is at:
  ${SPRINT_ROOT}/projects/${PROJECT_NAME}/${REVIEWER}.md

The two drafts you are critiquing are at:
  ${SPRINT_ROOT}/projects/${PROJECT_NAME}/${OTHER1}.md
  ${SPRINT_ROOT}/projects/${PROJECT_NAME}/${OTHER2}.md

Read all three drafts and the rubric before producing your critique. Your own draft anchors your judgment about what is and is not a useful lift region; do not re-review it.

For every candidate in each of the two foreign drafts, produce one of three verdicts:

- KEEP — the candidate passes the rubric and should remain in the merged set. State which of the five rubric criteria it most clearly satisfies.
- DROP — the candidate fails the rubric (cite which criterion) or is structurally a worse pick than an alternative already in your own draft (name the alternative).
- MODIFY — the candidate identifies a real lift target but the framing is wrong: incorrect line cite, conflated scope, marginal pick where the adjacent region is the real lift. Specify the modification concretely.

After the per-draft verdict tables, produce an OVERLOOKED section listing any region(s) that all three drafts missed but that you now believe belong in the merged set. Use the full per-candidate format from rubric.md for each.

Close with an "Overall observations" section (2–4 sentences) noting where the two foreign drafts converge cleanly, where they meaningfully diverge, and whether one model is systematically more or less rigorous on a particular rubric criterion.

Constraints:
1. Be terse. One paragraph max per verdict.
2. It is fine for many verdicts to be KEEP — honesty over thoroughness.
3. Cite file:line for every MODIFY and every OVERLOOKED.
4. Do not modify the foreign drafts; only critique them.

Output format: a single Markdown document, structure exactly as shown in PHASE2-PLAN.md §"Output format" for Phase 2a.

Output destination: write your final answer to ${OUTPUT_PATH}. Do not write anywhere else.
```

---

## Phase 2b — Aggregation

### Methodology

For each project, **one aggregator model** reads all three Phase 1 drafts and all three Phase 2a critiques, then produces a single merged candidate set. The aggregator does not re-vote on candidates; it applies a deterministic merge rule grounded in cross-model consensus and critic verdicts.

### Aggregator selection

Default: **claude** is the aggregator across all six projects (consistency between projects matters more than per-project model variety; mixing aggregators makes the merged sets harder to compare).

Open question to user: do you want a single aggregator (default claude), or do you want to rotate (e.g., 2 projects per model)?

### Inclusion rules (deterministic)

| Phase 1 picks | Phase 2a critiques | Action |
|---|---|---|
| 3 of 3 picked | All 3 critics KEEP (or self) | Include — strongest consensus. |
| 2 of 3 picked | Third critic KEEPs | Include. |
| 2 of 3 picked | Third critic DROPs | Include with a "disputed" annotation; aggregator decides. |
| 1 of 3 picked | At least one critic KEEPs | Include with "weak consensus" annotation. |
| 1 of 3 picked | All other critics DROP | Exclude unless aggregator actively defends. |
| OVERLOOKED by ≥2 critics | n/a | Include. |
| OVERLOOKED by 1 critic | n/a | Include only if rubric scoring is unambiguous (all 5 criteria yes/maybe). |

A candidate marked MODIFY by any critic gets the modification applied (corrected line cite, narrowed scope, etc.) before inclusion.

### Output format

```markdown
# <project> — merged lift-region candidate set (Phase 2b final)

## Methodology note

Phase 1 drafts: claude, codex, gemini.
Phase 2a critics: each model reviewed the other two.
Phase 2b aggregator: ${AGGREGATOR}.

## Merged candidates (ranked strongest → weakest)

### M-1: <short name>

- **pick_provenance:** "claude+codex+gemini (3/3)" or similar
- **critique_status:** "KEEP from all 3 critics" / "MODIFY from gemini (corrected line cite)" / etc.
- **Region root:** (per-candidate format from rubric.md)
- **Caller(s):** ...
- **Why useful (rubric scoring):** ...
- **Activation shape:** ...
- **Confidence:** ...
- **Risk notes:** ...

### M-2: ...

[continue ranked]

## Discrepancies

For each candidate where the critics significantly disagreed, summarize the disagreement and note which side the aggregator sided with and why.

## Excluded candidates

Brief list of candidates that appeared in Phase 1 drafts but were excluded from the merged set, with one-line reason each.
```

### Phase 2b prompt template (will be saved as `prompt-phase2-aggregate.md`)

```
You are aggregating Phase 1 drafts and Phase 2 critiques for project ${PROJECT_NAME} into a single merged lift-region candidate set, in support of the Monolift research project.

Background: see ${SPRINT_ROOT}/README.md, ${SPRINT_ROOT}/rubric.md, and ${SPRINT_ROOT}/PHASE2-PLAN.md for the full methodology.

Read these inputs:
- Rubric: ${SPRINT_ROOT}/rubric.md
- Phase 1 drafts:
    ${SPRINT_ROOT}/projects/${PROJECT_NAME}/claude.md
    ${SPRINT_ROOT}/projects/${PROJECT_NAME}/codex.md
    ${SPRINT_ROOT}/projects/${PROJECT_NAME}/gemini.md
- Phase 2a critiques:
    ${SPRINT_ROOT}/projects/${PROJECT_NAME}/critique-by-claude.md
    ${SPRINT_ROOT}/projects/${PROJECT_NAME}/critique-by-codex.md
    ${SPRINT_ROOT}/projects/${PROJECT_NAME}/critique-by-gemini.md

Apply the deterministic inclusion rules from PHASE2-PLAN.md §"Inclusion rules":

1. A candidate picked by all 3 drafts and KEEPed by all critics → include without question.
2. A candidate picked by 2 drafts and KEEPed by the third's critique → include.
3. A candidate picked by 2 drafts but DROPped by the third's critique → include with a "disputed" annotation; you decide based on rubric defensibility.
4. A candidate picked by 1 draft and KEEPed by at least one other critic → include with "weak consensus" annotation.
5. A candidate picked by 1 draft and DROPped by all other critics → exclude unless you actively defend it (defense must be grounded in the rubric, not in your own opinion).
6. A region in the OVERLOOKED section of ≥2 critiques → include.
7. A region in the OVERLOOKED section of exactly 1 critique → include only if the rubric scoring is unambiguous (all 5 criteria are yes or yes/maybe).

Apply MODIFY corrections (line cite drift, scope narrowing) before producing the merged entry.

For each merged candidate, use the per-candidate format from the rubric, prefixed with two new lines:
- pick_provenance: which of the three drafts originally picked this region
- critique_status: a one-line summary of critic verdicts

Sort candidates from strongest (highest cross-model consensus + cleanest rubric scoring) to weakest.

After the merged candidates, produce two sections:
- "Discrepancies" — for any candidate where critics significantly disagreed, summarize the disagreement and note which side you sided with.
- "Excluded candidates" — brief list of Phase 1 picks that did not make the merged set, with one-line reason each.

Constraints:
1. Be a faithful aggregator. Apply the inclusion rules deterministically; do not silently add or drop candidates outside the rules.
2. When you do exercise judgment (rule 3 disputed cases, rule 5 active defense), be explicit about doing so and cite the rubric criterion that justifies it.
3. Cite file:line for every kept candidate. Apply MODIFY corrections from critiques where applicable.

Output destination: write your final answer to ${OUTPUT_PATH}. Do not write anywhere else.
```

---

## Workflow

Total invocations: **18 critiques + 6 aggregations = 24 model runs.**

Critique runs are independent and can be parallelized within and across models. Aggregation runs depend on all 3 critiques for the same project being complete; they should run sequentially per project after Phase 2a finishes.

### Recommended order

1. Pull latest from origin (to get codex Phase 1 outputs).
2. Run all 18 Phase 2a critique invocations. Within a model, these can be parallel; across models, also parallel (subject to per-account rate limits).
3. Wait for Phase 2a to finish for all six projects.
4. Run the 6 Phase 2b aggregation invocations (one per project). These can be parallel — they only depend on the project's own critiques being done.
5. Commit the 24 new files. Push.

### Where to run

If running entirely on the codex-capable machine: all 24 invocations there, sequentially within model (codex), parallel across models.

If splitting across machines (faster end-to-end):
- codex critiques (6) and codex aggregations (0–6, depending on aggregator choice) on the codex machine.
- claude + gemini critiques (12) on the primary machine.
- Aggregator runs on whichever machine the chosen aggregator lives on.

For simplicity I recommend running the full 24 on the codex machine in one batch.

---

## Open questions for the user

1. **Aggregator choice.** Default is claude across all six projects. Alternatives: rotate (2 projects per model), or run all three aggregators per project (18 merged sets, then we pick). Recommendation: claude default.
2. **Critique format.** Structured table (KEEP/DROP/MODIFY per candidate) vs. discursive prose. The plan above is structured because it makes the aggregator's job mechanical. Alternative: discursive critique, aggregator does more interpretive work.
3. **OVERLOOKED granularity.** Currently the prompt asks critics to nominate regions all three drafts missed. We could also ask them to flag candidates one draft picked but the others missed in the wrong direction (i.e., should have been picked but wasn't). The current plan folds this implicitly into MODIFY for the picking draft and KEEP/silence for the non-picking ones; a more explicit "should-have-been-picked" verdict could be added.
4. **Sequencing of Phase 2a and Phase 2b.** The plan assumes 2a fully completes before 2b starts. If you'd rather pipeline (start aggregating projects as their critiques land), say so — the runner scripts can be structured either way.

---

## Notes on execution

The two prompt templates above (§"Phase 2a prompt template", §"Phase 2b prompt template") are self-contained — they reference rubric.md and the relevant draft/critique paths and tell the agent where to write its output. Drive them through whichever CLI / orchestration you prefer; they do not require the runner scripts used in Phase 1.
