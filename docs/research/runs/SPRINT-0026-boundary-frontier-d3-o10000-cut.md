# SPRINT-0026 Boundary Frontier Depth 3 / 10k Cut

Date: 2026-04-27

The depth 3 / 10k owner-budget Mattermost row was not run.

Reason: the prior ladder rows did not show movement toward the target chain.
Depth 1 / 5k and depth 2 / 5k had identical frontier shape. Depth 2 / 10k
increased BoundarySeed owners from 47 to 70, but still reported:

- `channels/api4` reached: no
- `connectWebSocket` found: no
- `APIHandlerTrustRequester` found: no
- Adjacent expansion owners: 0
- Stop reasons: owner budget and duration budget

The frontier is spending the budget in reverse-frontier owners before
callgraph-adjacent expansion can contribute. Running depth 3 with the same 10k
owner budget would not test a meaningfully different frontier.
