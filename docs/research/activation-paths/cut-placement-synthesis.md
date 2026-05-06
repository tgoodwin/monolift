# Cut Placement Synthesis

## Executive Summary

SPRINT-0039 analyzed all 72 activation-path traces for where the network boundary should go. The corpus contains 71 reachable traces plus the accepted `mattermost/M-4` structural gap. The strongest result is simple: deep cuts dominate. Among reachable traces, mean recommended-cut depth is `0.924`, median is `1.0`, and 65/71 recommendations are at depth `>= 0.75`.

The compiler should not use a single weighted score. It should use a decision tree: apply hard boundary-data gates first, separate proxy-required cuts from ordinary RPC cuts, prefer zero callbacks, then rank state reconstruction, surface area, error semantics, and edge alignment.

Primary artifacts:

- Per-trace analyses: [`analyses/`](analyses/)
- Master recommendation table: [`analyses/recommended-cuts.md`](analyses/recommended-cuts.md)
- Corpus statistics: [`analyses/corpus-statistics.md`](analyses/corpus-statistics.md)
- Open questions: [`analyses/open-questions.md`](analyses/open-questions.md)

## Per-Codebase Findings

| Project | Main finding |
|---|---|
| Caddy | Middleware interface edges are strong but usually carry `http.ResponseWriter`; the best cuts are below the middleware shell unless the target is inherently HTTP middleware. |
| Gitea | Queue-worker traces dominate. The queue runtime is too stateful, but registered handlers expose serializable work items and retry semantics. |
| Listmonk | Small codebase size lowers surface-area pressure; boundary data and client reconstruction become the decisive axes. |
| Mattermost | `App` and server receivers carry extensive shared state. Deep cuts are still best, but shared-state reconstruction is the dominant risk. |
| Miniflux | Goroutine launches are anti-boundaries; cuts should land at named reader/parser/integration functions after job IDs or request data are extracted. |
| PocketBase | Hook/event edges are useful reachability evidence, but concrete helper cuts below hook event objects avoid continuation callbacks. |

## Corpus-Wide Archetypes

| Archetype | Representative traces | Profile |
|---|---|---|
| Pure Leaf | `caddy/M-1`, `caddy/M-3`, `gitea/M-16`, `miniflux/M-3`, `pocketbase/M-3` | Minimal surface, simple data, zero callbacks, often weak/anti edge. |
| Interface Contract | `caddy/M-4`, `gitea/M-16`, `miniflux/M-14`, `pocketbase/M-10` | Strong interface edge plus serializable/reconstructible data. |
| Queue Handler | `gitea/M-1`, `gitea/M-2`, `gitea/M-10`, `gitea/M-12`, `gitea/M-15` | Serializable queue payloads, preserved retry/batch semantics, client-reconstructible state. |
| HTTP/Request Shell Escape | `caddy/M-2`, `caddy/M-5`, `listmonk/M-5`, `gitea/M-19` | Move below request/writer objects, or accept proxy when middleware is the target. |
| Framework Callback | `gitea/M-13`, `listmonk/M-4`, `pocketbase/M-7`, `pocketbase/M-9` | Avoid sending continuation functions across the network. |
| Shared-State App Receiver | `mattermost/M-6`, `mattermost/M-7`, `mattermost/M-8`, `mattermost/M-12`, `mattermost/M-13` | Deep cut still requires coordinating app/server state. |

## Anti-Boundary Catalog

- Raw goroutine launches: 23 occurrences; never use the launch edge itself as the boundary.
- Closure captures across framework state: implicit boundary data, often request/app/handler values.
- Function factories and function-valued callbacks: hard gate when a function value must cross.
- Shallow direct bootstrap calls: huge surface and shared process state.
- Request/writer middleware shells: natural abstraction, but proxy-required HTTP data.

## Open-Question Answers

- Weighting: use gates and a decision tree, not a scalar score.
- Composite cuts: useful in roughly 10-15 traces, especially queue handler + service leaf and hook method + concrete continuation pairs.
- Feasibility model: hard-gate function values/runtime handles. Streaming types (ResponseWriter, io.Writer) at the cut point are a signal to cut deeper, not a proxy-required classification — the monolith is the gateway and handles HTTP lifecycle (ADR-0028). FeasibleWithProxy retired as a category.
- Path-local vs. graph-global: no duplicate region roots appear in this corpus, but queue families show why later graph-global merging is needed.
- Liftability integration: ADR-0018 properties align well; `boundary.no-streaming-values` and `contract.error-last` should feed cut placement directly.

## Compiler Next Steps

1. Add a cut-placement phase after activation-path discovery that consumes trace edges, function signatures, and liftability facts.
2. Implement boundary-data gates before ranking candidates.
3. Normalize edge taxonomy so variants of goroutine, callback, closure, handler registration, and interface dispatch collapse into stable scoring families.
4. Detect short composite cuts for queue handlers, template helpers, and hook continuations.
5. Treat `mattermost/M-4` as an activation-path loading gap, not a placement problem.
