# SPRINT-0027 Budgeted Frontier Baseline

Date: 2026-04-28

## Diagnostic Question

Can a budget-partitioned invocation-boundary frontier reserve enough search
capacity for adjacent expansion and BoundaryPredicate scanning to recover the
Mattermost registration evidence without falling back to whole-program boundary
discovery?

The target evidence is the same chain used in SPRINT-0025 and SPRINT-0026:
`connectWebSocket -> APIHandlerTrustRequester -> http.Handler` registration.
This sprint remains diagnostic only. It should measure whether separately
bounded reverse owners, adjacent owners, boundary candidates, final indexing,
and duration move the bounded search closer to that chain.

## SPRINT-0026 Matrix Summary

SPRINT-0026 tested boundary-frontier discovery with `--function-index-mode=http-sinks`,
`--boundary-discovery-mode=frontier`, a 30s boundary-frontier duration, and
region roots `(*Hub).Start` and `(*WebConn).Pump`.

| Row | Owners | BoundarySeed owners | Boundary evidence | Stop reasons | Target movement |
|---|---:|---:|---:|---|---|
| depth 1 / 500 | 500 | 45 | 259 | owner budget | `channels/api4` and `connectWebSocket` touchpoint present; no `connectWebSocket` external surface or `APIHandlerTrustRequester` |
| depth 1 / 5k | 5,000 | 47 | 278 | duration budget, owner budget | same target gap |
| depth 2 / 5k | 5,000 | 47 | 278 | duration budget, owner budget | same target gap |
| depth 2 / 10k | 10,000 | 70 | 383 | duration budget, owner budget | same target gap |

The positive result was that frontier discovery avoided whole-program boundary
scanning and kept the final seeded function-reference index small. The negative
result was that no row recovered `connectWebSocket` as an ExternalSurface, no
row found `APIHandlerTrustRequester`, and no row linked the target handler into
the desired `http.Handler` registration chain.

## Budget Partitioning Question

SPRINT-0026 used a single owner budget for reverse-frontier and adjacent
expansion work. That budget shape left no measured capacity for adjacent
expansion in the Mattermost rows. SPRINT-0027 asks a narrower question: if
reverse owners, adjacent owners, BoundaryPredicate scan candidates, final
function-reference indexing, and elapsed boundary duration are budgeted
independently, does adjacent expansion contribute useful owners or boundary
evidence closer to the target chain?

This sprint should answer that with bounded ladder rows before considering any
implementation or schema work.

## SPRINT-0026 Failure Mode

The observed SPRINT-0026 failure mode was:

- reverse-frontier owners exhausted the shared owner budget,
- adjacent expansion contributed zero owners in every Mattermost row
  (`adjacentExpansionOwners=0`),
- the target chain was not recovered.

Increasing depth did not help because there was no remaining owner budget for
adjacent expansion to use. SPRINT-0027 must therefore report reverse-owner,
adjacent-owner, and boundary-candidate consumption separately.

## Boundary Terminology

This diagnostic keeps the SPRINT-0026 vocabulary:

- **InvocationBoundary:** semantic place where external control reaches owned
  code.
- **BoundaryPredicate:** pluggable detector for a boundary family.
- **BoundarySeed:** owner or instruction selected because a BoundaryPredicate
  matched.
- **RegistrationSite:** source location or instruction where a callable is
  registered.
- **ValueSink:** internal value-flow endpoint where a function value lands.
- **SeedSet:** bounded worklist input for function-reference indexing.
