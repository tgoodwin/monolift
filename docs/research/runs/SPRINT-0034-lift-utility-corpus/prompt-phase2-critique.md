# Phase 2a prompt template — cross-critique

This is the prompt fed to each Phase 2a critic agent. Variables in `${...}` are substituted per invocation.

---

You are reviewing two peer agents' lift-region candidate sets for the project ${PROJECT_NAME}, in support of the Monolift research project.

Background: see `docs/research/runs/SPRINT-0034-lift-utility-corpus/README.md` and `docs/research/runs/SPRINT-0034-lift-utility-corpus/rubric.md` for the methodology and selection criteria. Phase 1 of this sprint produced three independent candidate drafts per project; you are entering the cross-critique phase.

You are the critic ${REVIEWER}. Your own Phase 1 draft is at:
  `docs/research/runs/SPRINT-0034-lift-utility-corpus/projects/${PROJECT_NAME}/${REVIEWER}.md`

The two drafts you are critiquing are at:
  `docs/research/runs/SPRINT-0034-lift-utility-corpus/projects/${PROJECT_NAME}/${OTHER1}.md`
  `docs/research/runs/SPRINT-0034-lift-utility-corpus/projects/${PROJECT_NAME}/${OTHER2}.md`

Read all three drafts and the rubric before producing your critique. Your own draft anchors your judgment about what is and is not a useful lift region; do not re-review it.

For every candidate in each of the two foreign drafts, produce one of three verdicts:

- KEEP — the candidate passes the rubric and should remain in the merged set. State which of the five rubric criteria it most clearly satisfies.
- DROP — the candidate fails the rubric (cite which criterion) or is structurally a worse pick than an alternative already in your own draft (name the alternative).
- MODIFY — the candidate identifies a real lift target but the framing is wrong: incorrect line cite, conflated scope, marginal pick where the adjacent region is the real lift. Specify the modification concretely.

After the per-draft verdict tables, produce an OVERLOOKED section listing any region(s) that all three drafts missed but that you now believe belong in the merged set. Use the full per-candidate format from rubric.md for each.

Close with an "Overall observations" section (2–4 sentences) noting where the two foreign drafts converge cleanly, where they meaningfully diverge, and whether one model is systematically more or less rigorous on a particular rubric criterion.

**Constraints:**
1. Be terse. One paragraph max per verdict.
2. It is fine for many verdicts to be KEEP — honesty over thoroughness.
3. Cite file:line for every MODIFY and every OVERLOOKED.
4. Do not modify the foreign drafts; only critique them.
5. You may read the source tree at `evaluation/${PROJECT_NAME}/` to verify file:line citations and check rubric claims against the actual code.

**Output format:** a single Markdown document structured as follows:

```
# Critique of ${PROJECT_NAME} drafts by ${REVIEWER}

## Verdicts on ${OTHER1}'s draft

| Candidate ID | Verdict | One-paragraph reasoning |
|---|---|---|
| C-1 (...) | KEEP | rubric criterion satisfied: ... |
| ... | ... | ... |

## Verdicts on ${OTHER2}'s draft

| Candidate ID | Verdict | One-paragraph reasoning |
|---|---|---|
| ... | ... | ... |

## Overlooked

(Regions all three drafts missed. Use the per-candidate format from rubric.md. May be empty.)

## Overall observations

(2–4 sentences)
```

**Output destination:** Write your final answer to `${OUTPUT_PATH}`. Do not write anywhere else.

**Tools available.** Standard read/grep/find. You may run `go list`, `grep`, etc. against the source tree. Do not modify the source tree. Do not run the project. Do not run tests.
