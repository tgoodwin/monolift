# Phase 2b prompt template — aggregation

This is the prompt fed to the aggregator agent for each project. Variables in `${...}` are substituted per invocation.

---

You are aggregating Phase 1 drafts and Phase 2a critiques for project ${PROJECT_NAME} into a single merged lift-region candidate set, in support of the Monolift research project.

Background: see `docs/research/runs/SPRINT-0034-lift-utility-corpus/README.md`, `docs/research/runs/SPRINT-0034-lift-utility-corpus/rubric.md`, and `docs/research/runs/SPRINT-0034-lift-utility-corpus/PHASE2-PLAN.md` for the full methodology.

Read these inputs:
- Rubric: `docs/research/runs/SPRINT-0034-lift-utility-corpus/rubric.md`
- Phase 1 drafts:
    `docs/research/runs/SPRINT-0034-lift-utility-corpus/projects/${PROJECT_NAME}/claude.md`
    `docs/research/runs/SPRINT-0034-lift-utility-corpus/projects/${PROJECT_NAME}/codex.md`
    `docs/research/runs/SPRINT-0034-lift-utility-corpus/projects/${PROJECT_NAME}/gemini.md`
- Phase 2a critiques:
    `docs/research/runs/SPRINT-0034-lift-utility-corpus/projects/${PROJECT_NAME}/critique-by-claude.md`
    `docs/research/runs/SPRINT-0034-lift-utility-corpus/projects/${PROJECT_NAME}/critique-by-codex.md`
    `docs/research/runs/SPRINT-0034-lift-utility-corpus/projects/${PROJECT_NAME}/critique-by-gemini.md`

Apply the deterministic inclusion rules from PHASE2-PLAN.md §"Inclusion rules":

1. A candidate picked by all 3 drafts and KEEPed by all critics → include without question.
2. A candidate picked by 2 drafts and KEEPed by the third's critique → include.
3. A candidate picked by 2 drafts but DROPped by the third's critique → include with a "disputed" annotation; you decide based on rubric defensibility.
4. A candidate picked by 1 draft and KEEPed by at least one other critic → include with "weak consensus" annotation.
5. A candidate picked by 1 draft and DROPped by all other critics → exclude unless you actively defend it (defense must be grounded in the rubric, not in your own opinion).
6. A region in the OVERLOOKED section of ≥2 critiques → include.
7. A region in the OVERLOOKED section of exactly 1 critique → include only if the rubric scoring is unambiguous (all 5 criteria are yes or yes/maybe).

Apply MODIFY corrections (line cite drift, scope narrowing) before producing the merged entry.

You may read the source tree at `evaluation/${PROJECT_NAME}/` to verify file:line citations and resolve ambiguities.

For each merged candidate, use the per-candidate format from the rubric, prefixed with two new lines:
- **pick_provenance:** which of the three drafts originally picked this region (e.g. "claude+codex+gemini (3/3)")
- **critique_status:** a one-line summary of critic verdicts (e.g. "KEEP from all 3 critics", "MODIFY from gemini (corrected line cite)")

Sort candidates from strongest (highest cross-model consensus + cleanest rubric scoring) to weakest.

After the merged candidates, produce two sections:
- **Discrepancies** — for any candidate where the critics significantly disagreed, summarize the disagreement and note which side you sided with and why.
- **Excluded candidates** — brief list of candidates that appeared in Phase 1 drafts but were excluded from the merged set, with one-line reason each.

**Output format:**

```
# ${PROJECT_NAME} — merged lift-region candidate set (Phase 2b final)

## Methodology note

Phase 1 drafts: claude, codex, gemini.
Phase 2a critics: each model reviewed the other two.
Phase 2b aggregator: claude (opus).

## Merged candidates (ranked strongest → weakest)

### M-1: <short name>

- **pick_provenance:** ...
- **critique_status:** ...
- **Region root:** ...
- **Caller(s):** ...
- **Why useful (rubric scoring):**
  - Compute envelope: ...
  - Load profile: ...
  - Coherent unit: ...
  - State independence: ...
  - Latency / failure: ...
- **Activation shape:** ...
- **Confidence:** ...
- **Risk notes:** ...

### M-2: ...

[continue ranked]

## Discrepancies

...

## Excluded candidates

...
```

**Constraints:**
1. Be a faithful aggregator. Apply the inclusion rules deterministically; do not silently add or drop candidates outside the rules.
2. When you do exercise judgment (rule 3 disputed cases, rule 5 active defense), be explicit about doing so and cite the rubric criterion that justifies it.
3. Cite file:line for every kept candidate. Apply MODIFY corrections from critiques where applicable.

**Output destination:** Write your final answer to `${OUTPUT_PATH}`. Do not write anywhere else.

**Tools available.** Standard read/grep/find. You may run `go list`, `grep`, etc. against the source tree. Do not modify the source tree. Do not run the project. Do not run tests.
