# Annotation protocol (runs/opus, SPRINT-0013)

This directory holds per-target annotation notes for the six evaluation
corpus targets pinned in `evaluation/MANIFEST.yaml` as of 2026-04-19.
Each note uses the schema frozen below so cross-target comparison is
mechanical. The notes in this directory are one of three parallel runs
(opus / gpt-5.4 / gemini) that will later be merged by a synthesis step.

## Annotation schema (ten fields)

For each region identified in a target:

1. **subsystem** — which bundle of the target this region belongs to
   (e.g. `routers/api` for gitea, `server/channels/app` for mattermost).
2. **owned directories** — filesystem paths relative to the evaluation
   corpus root that the region lives in.
3. **region or operation identity** — module path, package path, symbol
   name, kind (function / method / type), and span when available.
4. **admitted or refused** — "admitted" if the current classifier/extract
   accepts it; "refused" if it carries an `MLV2_*` code. Pragma-targeted
   regions are what extracts see today; uninstrumented regions are
   categorized by walking source.
5. **triage** — one of `ADMITTED` / `AUTO` / `SUGGEST` / `TERMINAL`.
   - `ADMITTED`: already liftable today; kept for completeness.
   - `AUTO`: currently-refused, but an archetype + transform + evidence
     conditions would let the compiler lift it automatically.
   - `SUGGEST`: archetype fits but static evidence is insufficient;
     compiler should surface a remediation suggestion.
   - `TERMINAL`: no archetype fits in v1; refusal stands.
6. **proposed archetype** — the named distribution archetype from the
   v1 catalog. For subagent returns, archetypes are *flagged* candidates;
   promotion happens in Phase 4.
7. **proposed candidate state class** — if the region would require a
   new ADR-0016 state class to be recognized, name it. Otherwise cite
   an existing class or `n/a`.
8. **proposed transform (one-line sketch)** — what distributed code
   this region becomes after the lift.
9. **competing archetypes considered** — other archetype labels that
   plausibly fit; brief reason for choosing the primary one.
10. **evidence signals seen** — cited to
    `docs/specs/liftability-properties.md` property IDs or
    `pkg/compiler/stateclass/` rules, plus the `missing evidence`
    signal that would move the region one step up the triage
    (SUGGEST → AUTO, or TERMINAL → SUGGEST).
11. **file references** — path[:line] citations.

## Ambiguity flagging

- `terminal refusal` — no archetype fits; the refusal is the correct
  answer in v1.
- `archetype unclear pending evidence X` — a specific missing
  evidence signal (named) would let the region be classified; this
  routes to the follow-up "candidate state-class additions" bucket.
- `hybrid archetype needing split` — two archetypes both cover the
  region and the catalog should split them. Flag with both labels.

No generic "unclassified" bucket is allowed.

## AUTO is the primary finding

Every target's synthesis at the top of its file leads with the **AUTO
set**: the currently-refused regions that would become auto-liftable
if the classifier learned the archetype.

## Target file order

Each target note opens with:

1. **Target synthesis** (parent-written, not stitched): dominant
   archetypes, hardest ambiguities, most important evidence gaps.
2. **AUTO set** — currently-refused regions that this research
   argues become auto-liftable with a named archetype.
3. **SUGGEST set** — archetypes that fit but need user evidence.
4. **TERMINAL set** — archetypes that do not fit; refusal stands.
5. **ADMITTED set** — already liftable today, kept for completeness.
6. **Subsystem coverage ledger** — every bundle in the target with
   either findings or a "no relevant archetype surface observed" note.
7. **Subagent dispatch log** (gitea / mattermost only).
