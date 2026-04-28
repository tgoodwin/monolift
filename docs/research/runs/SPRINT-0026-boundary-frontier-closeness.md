# SPRINT-0026 Boundary Frontier Closeness Indicators

Date: 2026-04-27

The staged Mattermost ladder used `--function-index-mode=http-sinks` with
`--boundary-discovery-mode=frontier`. The frontier reached `channels/api4` and
`connectWebSocket` as reverse-BFS touchpoint evidence, but none of the frontier
rows recovered `connectWebSocket` as an ExternalSurface, found
`APIHandlerTrustRequester`, or recovered the target registration chain.

| Run | Candidate owners | Packages | BoundarySeed owners | Stop reasons | `channels/api4` reached | `connectWebSocket` touchpoint | `connectWebSocket` external | `APIHandlerTrustRequester` | Any registration owner | `http.Handler` sink | Shortest observed wrapper chain |
|---|---:|---:|---:|---|---|---|---|---|---|---|---:|
| depth 1 / 500 | 500 | 32 | 45 | owner budget | yes | yes | no | no | yes | yes | 1 |
| depth 1 / 5k | 5,000 | 272 | 47 | duration budget, owner budget | yes | yes | no | no | yes | no | 1 |
| depth 2 / 5k | 5,000 | 272 | 47 | duration budget, owner budget | yes | yes | no | no | yes | no | 1 |
| depth 2 / 10k | 10,000 | 439 | 70 | duration budget, owner budget | yes | yes | no | no | yes | no | 1 |

Top missing edge: the frontier never reached
`APIHandlerTrustRequester` or a registration site that links the target handler
into an `http.Handler` sink. The dominant stop reason is structural for this
implementation: the reverse frontier consumes the owner budget before
callgraph-adjacent expansion contributes any owners.
