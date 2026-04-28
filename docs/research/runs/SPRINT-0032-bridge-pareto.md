# SPRINT-0032 Bridge Pareto Read

Date: 2026-04-28

## Working Thesis

Bridge discovery is promising, but only Pareto-useful if its defaults are
simple and its cost envelope is honest. The useful shape is not "search harder";
it is "use reverse-BFS touchpoints to find a small owner set, then spend a
clearly bounded function-reference index budget on that set."

SPRINT-0031 showed the previous default was misleading: bridge discovery
consumed the shared `FunctionRefIndexBudget`, so the index phase could start
with effectively no time left. SPRINT-0032 changes only bridge mode so bridge
seed discovery keeps its own explicit budget and `FunctionRefIndexBudget`
applies to the bridge index phase itself.

## Cost / Recall Frontier

| Run | Probe wall / wrapper real | Peak RSS | Indexed owners | Target result | Read |
|---|---:|---:|---:|---|---|
| SPRINT-0025 exhaustive/all upper bound | ~206.7s | ~12.47 GB | 140,801 | recovered | Correct but too expensive for the desired default path. |
| SPRINT-0028 oracle bridge v2 | ~78.4s / 103.62s | ~8.94 GB | 94 | recovered | Shows the bridge shape can work if the right owners are known. |
| SPRINT-0030 default | 86.36s / 109.20s | ~8.85 GB | 0 bridge owners | missed | Target owners admitted, then lost at function-ref indexing. |
| SPRINT-0030 `index180` | 83.19s / 110.87s | ~9.04 GB | 1,670 bridge owners | recovered main chain | Discovery was sufficient; index coverage was the missing step. |
| SPRINT-0031 default | 79.96s / 97.32s | ~8.85 GB | 0 / 1,676 bridge owners | missed | Deterministic priority existed, but shared budget left no index time. |
| SPRINT-0031 `index65` | 87.66s / 106.27s | ~9.17 GB | 1,676 / 1,676 bridge owners | recovered main chain | Best pre-SPRINT-0032 evidence for the consolidated bridge profile. |
| SPRINT-0032 phase-local bridge budget | 154.62s / 250.72s | ~9.14 GB | 1,676 / 1,676 bridge owners | recovered main chain | Confirms the cleaned-up 60s index budget works; wrapper wall was inflated by concurrent `go test`. |

## Budget Semantics

Before this sprint, bridge mode used one elapsed timer for bridge seed discovery
and the later function-reference index. That made a nominal 60s
`FunctionRefIndexBudget` behave like "60s for bridge discovery plus indexing."
When bridge discovery used the whole allowance, the admitted bridge owners were
all skipped by `index_budget`.

After this sprint, bridge mode has two visible budgets:

- `BridgeMaxDuration` bounds bridge seed discovery.
- `FunctionRefIndexBudget` bounds the function-reference index over admitted
  bridge owners.

Other index modes keep their previous semantics.

## SPRINT-0032 Validation Row

Artifacts:

- `docs/research/runs/SPRINT-0032-bridge-phase-local.json`
- `docs/research/runs/SPRINT-0032-bridge-phase-local.stderr`
- `docs/research/runs/SPRINT-0032-bridge-phase-local.summary.json`

Key measurements:

- probe wall: 154,615 ms
- wrapper real: 250.72s, inflated by concurrent package tests
- peak RSS: 9,137,803,352 bytes
- bridge seed discovery: 70,464 ms
- function-ref index: 3,533 ms
- bridge owners: 1,676
- indexed bridge owners: 1,676
- skipped bridge owners: 0
- indexed instructions: 124,597
- output counts: 608 external surfaces, 689 registration sites, 3,322 wrapper chains

Target node recovery:

| Target | Seed | Function-ref index | Final classification |
|---|---|---|---|
| `connectWebSocket` | present | present | present |
| `APIHandlerTrustRequester` | present | present | present |
| `InitWebSocket` | present | present | present |

Relationship recovery:

| Relationship | Result |
|---|---|
| `connect-to-api-handler` | recovered in final classification |
| `connect-registered-at-init` | recovered in final classification |
| `init-has-http-boundary` | boundary evidence present; final relationship absent |

## Read

Classification: promising, with a clear stop point.

Criteria:

- Recall: the approach recovers the known Mattermost chain once admitted bridge
  owners are indexed.
- Cost relative to exhaustive: the successful bridge rows are roughly half the
  wall time and several GB below the exhaustive upper bound.
- Implementation complexity: the v1 algorithm is now a small sequence of
  generic phases plus deterministic owner priority. SPRINT-0032 only clarifies
  budget semantics.
- Generalizability: the graph logic is generic. The current boundary evidence
  predicates are HTTP-heavy, but they are isolated and can be generalized for
  other registration families.
- Remaining risk: package load, SSA, and callgraph still dominate large-target
  cost; bridge mode is not a solution for those shared costs.

Recommended stop/continue point: stop expanding the algorithm for now after the
SPRINT-0032 validation row. Treat bridge v1 as the best current Pareto point:
use reverse touchpoints, local owner discovery, generic boundary evidence, and
priority indexing, with phase-local budgets.

One future optimization worth pursuing only if EntryPath remains a priority:
derive bridge starts from owner/package registration evidence more selectively,
then test whether the same oracle chain is recovered with fewer selected starts
and scanned package functions.

Do not pursue next: broad graph-search expansion or framework/package-name
special cases. Those add complexity and overfit risk without addressing the
observed SPRINT-0031 loss.
