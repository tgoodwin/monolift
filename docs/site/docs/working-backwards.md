# Working backwards from real code

## Research question and result

Building a Go-monolith-to-microservices compiler in the abstract is a research
project with boundless scope. There is no end to the language features, framework
patterns, and runtime contracts a fully general compiler would have to
handle. The workshop paper sidestepped that by shipping a deliberately
narrow prototype. Now the compiler needs to grow into a tool that
handles production codebases without trying to handle every language
feature at once.

The strategy the project actually runs on is the inverse of the naive
prototype direction. With the high-level design goals already in place, we now pick real lift candidates from six open-source Go monoliths and lets those candidates dictate what compiler features need to be developed next. Each candidate is a forcing function for a specific capability. Some land. Some prove infeasible on closer inspection, and we update our design goals accordingly.

## What becomes of a candidate

We start with a corpus of 72 hand-traced lift candidates drawn from six
open-source Go monoliths (roughly 1.5M lines of Go in total). See
[Evaluation targets](evaluation-targets.md) for the survey itself.

Once a candidate is in the corpus, it sits in one of four states at
any given time.

- **Landed.** The compiler produces a lift that builds, deploys, and
  serves real workload traffic against the host.
- **In progress.** The compiler takes the candidate part of the way.
  Analysis may succeed while extraction is still blocked, or
  extraction may succeed while the lifted service cannot yet stand up
  against a real dependency. The current ceiling is recorded against
  the candidate.
- **Deferred.** The candidate is plausible but waits on a compiler
  capability not scheduled for the next R&D sprints. The block is
  recorded so the backlog stays ranked by what is actually in the
  way.
- **Infeasible.** Inspection surfaced something the compiler should
  not try to handle: a cross-process plugin boundary, a
  runtime-conditional registration, a mutable shared cache. The note
  stays with the candidate so the same ground is not re-tilled
  later.

The mix matters more than the totals. A corpus where every
candidate had landed would mean the candidates were chosen after the
fact to flatter the compiler. A corpus with no infeasibles would
mean the selection prior was too tight. The real shape, with all
four states present, is what gives the strategy traction and
direction.

## Selecting a corpus of candidates 

The candidates were not picked at random. Three independent model
agents per project produced candidate lists scored against a utility
rubric, cross-reviewed each other's picks, and a synthesis pass
merged the result. The rubric implicitly carried a feasibility
prior. No candidate is a whole subsystem; every candidate is a
single function or a small cluster of methods. The corpus entries
represent regions of code that seem plausibly liftable and where
there is plausible benefit in doing so. Iterative development of
the compiler will reveal which ones actually are.


## Stages as a per-target capability marker

The work involved in lifting a candidate is divided into tiered
stages. Each candidate is tagged with the highest stage the compiler
currently takes it to. That tag is the project's standing claim
about what the compiler can do for that specific symbol.

Aggregated across the corpus, those per-candidate stage values form
a capability map. The map is what the project diffs sprint-over-sprint: each sprint's
coverage report is the difference between the snapshot before it and the
snapshot after.

## New capability lands, targets unlock

Most sprints raise existing targets through the rungs that already
exist, rather than adding new rungs to the ladder.

Miniflux's `RefreshFeed` is the canonical example. Earlier in
development the compiler could take it through admission and produce
compile-clean extracted code, but no further. The blocker was a
missing piece of compiler work: the SQL-wrapper reconstructor needed
to know how to rebuild `*storage.Storage` on the lifted side from a
`DATABASE_URL` environment variable, and the lifted deployment
needed that env var propagated to it. Once the capability landed,
the same target's ceiling moved up to a Ready deployment, and a
focused run confirmed the lifted pod came up against a real Postgres.
The capability itself was not built in the abstract. The existence
of the target, plus another roughly fourteen queued candidates
blocked on the same family, is what made it worth building.

Adding a new rung to the ladder is rarer. Transcript comparison with
declared substitutions — matching a lifted run against the original — is
one example: it sets a new ceiling above the prior best, which was a
deployed lift handling real workload traffic. Adding a rung is a bigger commitment because
it changes the meaning of the capability map for every target at
once. The default move is the smaller one.

??? abstract "Sprint breadcrumbs (for maintainers)"
    The milestones above map to the project's sprint history: the
    capability-map diff is recomputed each sprint (the SPRINT-0048 →
    SPRINT-0049 snapshot pair is one such diff); the SQL-wrapper
    reconstruction that unblocked miniflux's `RefreshFeed` landed in
    SPRINT-0049; and the transcript-comparison rung was added in SPRINT-0050.

## Refusal as evidence

The same strategy that ranks candidates by what they unlock also
turns refusal into evidence. Every refusal carries a named code
(see [Refusal diagnostics](refusal-diagnostics.md)), and an
admission-only sweep across the corpus produces a histogram of
those codes for almost no cost. The histogram is the cheapest
signal the project has about which compiler capability to build
next. Capabilities with no consumer in the corpus stay in the
backlog.
