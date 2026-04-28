# SPRINT-0013 Annotations Protocol - gpt-5.4 run

This run writes only under `docs/research/runs/gpt-5.4/`.

## Frozen Schema

Each region entry in every target note uses this field set:

1. `subsystem`
2. `owned directories`
3. `region or operation identity (module / package / symbol / kind / span)`
4. `admitted or refused`
5. `triage (ADMITTED / AUTO / SUGGEST / TERMINAL)`
6. `proposed archetype`
7. `proposed candidate state class (if different from existing ADR-0016 classes)`
8. `proposed transform (one-line sketch)`
9. `competing archetypes considered`
10. `evidence signals seen (cited to liftability-properties or stateclass)`
11. `missing evidence (what would move SUGGEST -> AUTO, or TERMINAL -> SUGGEST)`
12. `file references`

## Triage Protocol

- `ADMITTED`: already liftable today; carried for coverage and contrast.
- `AUTO`: currently refused or currently unlifted surface where the evidence is closed-form enough for compiler-owned transform application.
- `SUGGEST`: archetype fit is credible, but at least one load-bearing condition is not statically closed.
- `TERMINAL`: no honest v1 archetype fit or no v1 remediation surface.

## Ambiguity Handling

- `terminal refusal` means no v1 archetype survives after competing fits are considered.
- `archetype unclear pending evidence X` keeps the region out of `TERMINAL`; the missing evidence must be named.
- `hybrid archetype needing split` means one source region hid more than one transform shape.

## Coverage Rule

Every subsystem bundle in a target ledger ends in one of two states:

- explicit findings
- explicit `no relevant archetype surface observed` note with a reason

Silence is not coverage.

## Large-Target Delegation Notes

- `gitea` and `mattermost` both used owned-directory bundle registration before dispatch.
- First-pass large-target fanout suffered two distinct failures:
  - over-broad prompts that did not terminate in a reasonable interval
  - one Mattermost pass that omitted the `evaluation/` prefix and returned the thin e2e fixture instead of the corpus checkout
- Those failed passes are logged in target dispatch tables rather than treated as evidence.
- Parent-written syntheses are grounded in raw source spot-checks even where a bundle return was thin, invalid, or irrelevant.

## Scratch Extract Note

I attempted scratch `extract-report` runs for `listmonk`, `gitea`, and `mattermost` by copying source to `/tmp` and injecting temporary pragmas. The extractor could not produce usable artifacts in this environment:

- `listmonk` and `gitea` require Go 1.26.x while the session toolchain is Go 1.25.4.
- `mattermost` failed under `packages.Load` with a large unresolved-type surface in the available environment.

I treated that as a tooling follow-up, not as a reason to stop source analysis, because the run-local deliverables are research notes rather than regenerated reports.

## Thin-Return Check

A bundle return is thin and must be re-dispatched, downgraded, or ignored if any of the following fail:

- region count clearly undershoots the source surface in the owned bundle
- file and line citations are missing
- triage is not applied consistently
- `AUTO` entries omit transform sketch or candidate state class
- the return collapses distinct websocket, queue, session, timer, or singleton surfaces into one vague label
