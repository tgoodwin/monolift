# SPRINT-0027 Budgeted Frontier Closeness

Date: 2026-04-28

Rows used exact root specs for the same roots:

- `github.com/mattermost/mattermost/server/v8/channels/app/platform.(*Hub).Start`
- `github.com/mattermost/mattermost/server/v8/channels/app/platform.(*WebConn).Pump`

| Row | `channels/api4` reached | `connectWebSocket` touchpoint | `connectWebSocket` external | `APIHandlerTrustRequester` | Target registration owner | `http.Handler` sink | Shortest observed edge chain | Top missing edge / stop reason |
|---|---|---|---|---|---|---|---:|---|
| small | yes | yes | no | no | no | yes | 1 | Missing function-value path from `connectWebSocket` to boundary registration; stopped on reverse and adjacent owner budgets |
| medium | yes | yes | no | no | no | yes | 1 | Same target gap; stopped on reverse and adjacent owner budgets |
| large | yes | yes | no | no | no | yes | 1 | Same target gap; generic registration sites increased, but target owner remained absent; stopped on reverse and adjacent owner budgets |

The shortest observed edge chain is generic wrapper-chain evidence, not the
target registration chain. The target chain remained absent in every completed
row.

## Adjacent Expansion Contribution

| Row | Adjacent owners | Boundary candidates | Boundary evidence | BoundarySeed owners | Evidence closer to target chain? |
|---|---:|---:|---:|---:|---|
| small | 500 | 1,000 | 272 | 48 | No target-specific movement |
| medium | 2,000 | 4,000 | 290 | 50 | No target-specific movement |
| large | 5,000 | 10,000 | 435 | 72 | Generic boundary evidence increased, but `APIHandlerTrustRequester` and target registration owner remained absent |

Budget partitioning fixed the SPRINT-0026 failure mode where
`adjacentExpansionOwners=0`. Adjacent expansion contributed nonzero owners in
every completed row. The added owners did not include boundary evidence close
enough to recover the target chain under the measured budgets.
