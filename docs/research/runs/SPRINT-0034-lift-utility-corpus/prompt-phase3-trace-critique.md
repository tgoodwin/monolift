# Phase 3 trace critique prompt template

This prompt is fed to a single agent to critique the other two agents' activation-path traces for a single candidate. Variables in `${...}` are substituted per invocation.

---

You are reviewing two peer agents' activation-path traces for the lift-region candidate **${CANDIDATE_ID}** (${CANDIDATE_NAME}) in project ${PROJECT_NAME}.

**Context.** Each trace describes the minimal static path from a binary entrypoint to the region root at `${REGION_ROOT}`. The traces label each step with an edge type — a category of static-analysis resolution a compiler would need to perform. We are building toward a general graph schema and compiler algorithm, so consistency and accuracy in path shape and edge-type labeling matter more than prose.

You are the critic **${REVIEWER}**. Your own trace is at:
  `${SPRINT_ROOT}/projects/${PROJECT_NAME}/traces/${CANDIDATE_M}.${REVIEWER}.md`

The two traces you are critiquing are at:
  `${SPRINT_ROOT}/projects/${PROJECT_NAME}/traces/${CANDIDATE_M}.${OTHER1}.md`
  `${SPRINT_ROOT}/projects/${PROJECT_NAME}/traces/${CANDIDATE_M}.${OTHER2}.md`

Read all three traces before producing your critique. You may also read source code at `evaluation/${PROJECT_NAME}/` to verify claims.

**For each of the two foreign traces, assess:**

1. **Path correctness.** Does the trace follow a real, verifiable static path from entrypoint to region root? Are any steps fabricated, skipped, or in the wrong order? Are file:line citations accurate?

2. **Path minimality.** Is this the shortest path, or does it include unnecessary intermediate steps? A step that a compiler could skip (e.g. a trivial wrapper that just forwards arguments) inflates the path without adding resolution complexity. Note which steps could be collapsed.

3. **Edge-type accuracy.** For each non-trivial edge (anything beyond `direct-function-call`), is the label correct? Would a different label better describe what a static analyzer needs to do? Propose corrections with reasoning.

4. **Edge-type granularity.** Are the labels codebase-agnostic and at the right level of abstraction? Flag any label that names a framework concept instead of a language/pattern-level mechanism.

**Output format:**

```
# Trace critique for ${CANDIDATE_ID} by ${REVIEWER}

## Critique of ${OTHER1}'s trace

### Path
- **Correct:** yes/no/partially — <1-2 sentences>
- **Minimal:** yes/no — <steps to collapse or add, if any>

### Edge types
| Step | ${OTHER1}'s label | Verdict | Suggested label (if different) | Reasoning |
|---|---|---|---|---|
| 1 | ... | OK | — | — |
| 3 | ... | RENAME | `<better-label>` | <why> |
| ... | ... | ... | ... | ... |

(Only include rows for non-trivial edges or where you disagree. Skip `direct-function-call` rows where you agree.)

### Other notes
<anything else: missed alternative paths, static-analysis boundary observations, etc.>

## Critique of ${OTHER2}'s trace

[same format]

## Cross-trace observations

2-3 sentences: where do the traces converge? where do they meaningfully diverge on path or edge types? which trace is closest to the canonical minimal path?
```

**Constraints:**
1. Be terse. Focus on disagreements and corrections.
2. Do not rewrite the traces; only critique them.
3. Verify file:line citations against the source tree when you suspect drift.

**Output destination:** Write your final answer to `${OUTPUT_PATH}`. Do not write anywhere else.

**Tools available.** Standard read/grep/find against `evaluation/${PROJECT_NAME}/`. Do not modify files.
