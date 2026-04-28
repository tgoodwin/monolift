# SPRINT-0026 Boundary-Frontier Baseline

Date: 2026-04-27

## Diagnostic Question

Can a protocol-agnostic invocation-boundary frontier search recover the
Mattermost HTTP registration chain without whole-program boundary scanning?

The target evidence remains the chain from `connectWebSocket` through
`APIHandlerTrustRequester` into an `http.Handler` registration sink. This sprint
is diagnostic only: it should determine whether bounded frontier discovery can
find useful BoundarySeed evidence from region roots and BoundaryPredicate
matches before any whole-program scan is required.

## SPRINT-0025 Result Summary

SPRINT-0025 measured the existing EntryPath modes against the Mattermost target
using region roots `(*Hub).Start` and `(*WebConn).Pump`.

| Mode | Result | Cost shape | Mattermost target evidence |
|---|---|---|---|
| `all` | Recovered the target chain | Too expensive: function index took about 120s with budget and peaked near 12.5 GB RSS | `connectWebSocket`, `APIHandlerTrustRequester`, and `http.Handler` sink evidence found |
| `reverse-path` | Completed cheaply | Function index took about 7.1s and peaked near 6.2 GB RSS | Missed `connectWebSocket` and `APIHandlerTrustRequester`; found only a small set of `http.Handler` sinks |
| `http-sinks` | Did not reach useful seeded scanning | Spent the 60s index budget during HTTP seed discovery | No target evidence recovered |
| `targeted` | Did not reach useful seeded scanning | Spent 60s to 120s budgets before final seeded scan | No target evidence recovered |

The key SPRINT-0025 conclusion was that reverse-path search is cheap but too
narrow, while whole-program function-reference or HTTP seed discovery is too
expensive. The open question is therefore not whether the target chain exists in
the analyzable program: `all` mode proved that it does. The open question is
whether a smaller SeedSet assembled from an invocation-boundary frontier can
recover that evidence under an incremental cost gate.

## Cost Baseline

Use the SPRINT-0025 split gate as the comparison point:

- Baseline package load, SSA build, root resolution, and callgraph should remain
  under 90s wall time and 8 GB RSS.
- Incremental boundary EntryPath work after callgraph should stay under 30s wall
  time and add no more than 1.5 GB RSS.

Whole-program `all` and current whole-program HTTP seed discovery fail the
incremental gate. `reverse-path` fits the incremental gate but misses the target
evidence.

## Scope Guard

This sprint should introduce boundary vocabulary and a bounded frontier
diagnostic without report schema changes, surface trace artifacts, emission
work, Mattermost/framework recognizers, or package-pruning behavior that changes
analysis semantics.
