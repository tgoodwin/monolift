# Phase 3 trace synthesis prompt template

This prompt is fed to a single aggregator agent to produce the canonical activation-path trace for a single candidate, based on three independent traces and three cross-critiques. Variables in `${...}` are substituted per invocation.

---

You are producing the **canonical activation-path trace** for lift-region candidate **${CANDIDATE_ID}** (${CANDIDATE_NAME}) in project ${PROJECT_NAME}.

**Context.** Three agents independently traced the static path from a binary entrypoint to the region root at `${REGION_ROOT}`. Each then critiqued the other two traces. Your job is to synthesize these into a single, authoritative, succinct trace that a compiler engineer could use as a specification.

**Inputs (read all before writing):**

Traces:
- `${SPRINT_ROOT}/projects/${PROJECT_NAME}/traces/${CANDIDATE_M}.claude.md`
- `${SPRINT_ROOT}/projects/${PROJECT_NAME}/traces/${CANDIDATE_M}.codex.md`
- `${SPRINT_ROOT}/projects/${PROJECT_NAME}/traces/${CANDIDATE_M}.gemini.md`

Critiques:
- `${SPRINT_ROOT}/projects/${PROJECT_NAME}/traces/${CANDIDATE_M}.critique-by-claude.md`
- `${SPRINT_ROOT}/projects/${PROJECT_NAME}/traces/${CANDIDATE_M}.critique-by-codex.md`
- `${SPRINT_ROOT}/projects/${PROJECT_NAME}/traces/${CANDIDATE_M}.critique-by-gemini.md`

You may also read source code at `evaluation/${PROJECT_NAME}/` to verify or resolve disagreements.

**Synthesis rules:**

1. **Choose the minimal correct path.** Where traces agree on the path, keep it. Where they disagree, verify against source code and pick the shortest correct path. Collapse trivial forwarding steps that all critics flagged as unnecessary.

2. **Normalize edge types.** Where all three traces agree on a label, keep it. Where they disagree, prefer the label that is (a) codebase-agnostic, (b) describes the static-analysis mechanism, and (c) was endorsed by at least one critic. Apply RENAME suggestions from critiques where they improve clarity.

3. **Cite sources.** Every file:line must be verified. If a critique flagged a line cite as wrong, check it and use the correct one.

**Output format (follow exactly):**

```
# ${CANDIDATE_ID}: ${CANDIDATE_NAME}

Region root: `${REGION_ROOT}`

## Trace

| Step | From | To | Edge type |
|---|---|---|---|
| 0 (entry) | — | `<file:line>` `<func>` | `entrypoint` |
| 1 | `<file:line>` | `<file:line>` `<func>` | `<edge-type>` |
| 2 | `<file:line>` | `<file:line>` `<func>` | `<edge-type>` |
| ... | ... | ... | ... |
| N | `<file:line>` | `<file:line>` `<func>` ← region root | `<edge-type>` |

## Edge type summary

| Edge type | Count | Steps |
|---|---|---|
| `direct-function-call` | 3 | 1, 4, 5 |
| `interface-method-dispatch` | 1 | 3 |
| ... | ... | ... |

## Resolution notes

For each non-trivial edge type (anything beyond direct-function-call / method-call-on-concrete-type), one line:
- **Step N** (`<edge-type>`): <what a static analyzer needs to resolve this — e.g. "CHA or RTA to resolve Handler interface to Templates.ServeHTTP">

## Alternative paths

If the region root is reachable via a meaningfully different activation shape (e.g. HTTP handler vs. cron job), list it in one line per alternative:
- <activation shape>: `<entrypoint file:line>` → ... → region root (N steps). <How edge types differ from the primary path.>

## Trace metadata

- **Path length:** N
- **Hardest edge:** Step N (`<type>`) — <one sentence why>
- **Fully statically resolvable:** yes/no — <one sentence>
```

**Constraints:**
1. The table is the trace. No prose between steps. No narrative. Keep it scannable.
2. Every `file:line` must be one you verified against the source tree.
3. Edge type labels must be codebase-agnostic. No framework names in labels.
4. If two steps can be collapsed without losing a distinct edge type, collapse them.
5. The entire output should be under 120 lines.

**Output destination:** Write your final answer to `${OUTPUT_PATH}`. Do not write anywhere else.

**Tools available.** Standard read/grep/find against `evaluation/${PROJECT_NAME}/`. Do not modify files.
