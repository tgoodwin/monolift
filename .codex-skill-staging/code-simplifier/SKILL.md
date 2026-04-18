---
name: code-simplifier
description: Use when a user wants existing code simplified without changing behavior, especially when logic is hard to scan, conditionals are repetitive, or helper structure can be reduced safely.
---

# Code Simplifier

## Overview

Reduce complexity in existing code while preserving behavior. Prefer smaller,
clearer control flow, fewer moving parts, and names that make the remaining
logic easier to read.

This skill is for simplification, not feature work. Preserve public behavior,
existing interfaces, and the surrounding codebase style unless the user asks
for a broader rewrite.

## Workflow

1. Find the real complexity:
- Identify what makes the code hard to read: deep nesting, duplicated branches,
  temporary state, over-abstracted helpers, or unnecessary indirection.
- Read adjacent call sites and tests before rewriting.

2. Preserve behavior first:
- Keep inputs, outputs, side effects, and error handling stable.
- Avoid changing external APIs unless the user explicitly asks.
- If behavior is unclear, infer from tests and nearby usage before editing.

3. Simplify in place:
- Flatten nested conditionals with early returns or guards.
- Remove redundant variables and one-use wrappers.
- Collapse duplicated logic into one clear path when that improves readability.
- Inline trivial helpers when indirection hides more than it helps.
- Extract helpers only when a repeated concept becomes clearer by naming it.

4. Keep changes local:
- Touch the smallest surface area that meaningfully improves readability.
- Do not opportunistically reformat unrelated code or rename broadly.

5. Verify:
- Run targeted tests or the smallest relevant verification command available.
- If no automated check is available, state that behavior was preserved by local
  reasoning and unchanged call patterns, but note verification limits.

## Quick Reference

| Problem | Preferred simplification |
| --- | --- |
| Deep `if` nesting | Use guard clauses / early returns |
| Repeated branch bodies | Merge into one path with shared setup |
| Temporary variables that only rename values | Inline them |
| Tiny helper used once | Inline if it improves readability |
| Boolean flags controlling long flows | Split into clearer branches or early exits |
| Over-abstracted wrappers | Replace with direct expression of intent |

## Heuristics

- Prefer explicit code over clever code.
- Prefer one obvious control path over several partial paths.
- Prefer fewer state mutations.
- Prefer domain names over mechanical names like `tmp`, `data2`, or `resultObj`.
- Match the surrounding style when the codebase already has a clear pattern.

## Rationalization Table

| Excuse | Reality |
| --- | --- |
| "Shorter is always simpler" | Some terse rewrites reduce readability |
| "I can refactor the whole module while I'm here" | Keep the change scoped to the user's request |
| "A new abstraction will clean this up" | Extra abstraction often hides the real logic |
| "Tests pass, so behavior is obviously identical" | Confirm side effects and edge cases, not just happy paths |

## Red Flags

- Rewriting public interfaces when only internals needed cleanup.
- Replacing readable logic with dense one-liners.
- Combining unrelated cleanups into the same change.
- Removing edge-case handling because it "looks redundant" without proof.

## Common Mistakes

- Inlining so aggressively that meaning is lost.
- Extracting helpers with vague names like `processData`.
- Dropping comments or validation branches that encode real constraints.
- Simplifying control flow but changing evaluation order or side effects.

## Example Pattern

Before:

```ts
function statusLabel(item: Item) {
  let label = "unknown";

  if (item) {
    if (item.error) {
      label = "error";
    } else {
      if (item.loading) {
        label = "loading";
      } else {
        label = "ready";
      }
    }
  }

  return label;
}
```

After:

```ts
function statusLabel(item: Item) {
  if (!item) return "unknown";
  if (item.error) return "error";
  if (item.loading) return "loading";
  return "ready";
}
```
