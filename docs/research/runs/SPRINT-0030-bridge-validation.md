# SPRINT-0030 Bridge Validation

Date: 2026-04-28

## Result

SPRINT-0030 moved the failure from local owner discovery to index scheduling.

The default validation run did not recover the target chain, but it did scan
the target package and seed all three oracle target owners. It failed because
the function-ref index scanned 0 bridge owners under the default validation
budget.

The follow-up `index180` validation indexed 1,670 bridge owners and recovered
the important target nodes and relationships. That run demonstrates that the
generic boundary-owner discovery fix is useful: once the target owners are
indexed, the existing function-value flow can recover the chain.

## Target Status

Default validation:

| Target | Selected/scanned | Bridge seed | Function-ref index | Final classification |
|---|---:|---:|---:|---:|
| `connectWebSocket` | yes | yes | no | no |
| `APIHandlerTrustRequester` | yes | yes | no | no |
| `InitWebSocket` | yes | yes | no | no |

`index180` validation:

| Target | Selected/scanned | Bridge seed | Function-ref index | Final classification |
|---|---:|---:|---:|---:|
| `connectWebSocket` | yes | yes | yes | yes |
| `APIHandlerTrustRequester` | yes | yes | yes | yes |
| `InitWebSocket` | yes | yes | yes | yes |

Relationship recovery in `index180`:

| Relationship | Status |
|---|---|
| `connect-to-api-handler` | recovered in final classification |
| `connect-registered-at-init` | recovered in final classification |
| `init-has-http-boundary` | boundary evidence present; final relationship still absent |

## Cost

| Run | Wall time | Peak RSS | Target recovery |
|---|---:|---:|---|
| SPRINT-0025 exhaustive/all upper bound | ~206.7s | ~12.47GB | recovered |
| SPRINT-0028 oracle bridge v2 | ~78.4s | ~8.94GB | recovered |
| SPRINT-0028 frontier large | ~141.6s | ~9.95GB | missed |
| SPRINT-0029 non-oracle bridge | ~80.8s probe / ~125.4s wrapper | ~6.07GB | missed |
| SPRINT-0030 diagnostic baseline | 51.05s wrapper | ~7.08GB | missed |
| SPRINT-0030 default validation | 109.20s wrapper / 86.36s probe | ~8.85GB | missed at index budget |
| SPRINT-0030 `index180` validation | 110.87s wrapper / 83.19s probe | ~9.04GB | main chain recovered |

## Evidence

Default validation:

- bridge owners: 1,675
- bridge boundary owners: 93
- indexed bridge owners: 0
- function-ref scanned functions: 0
- function-ref skipped functions: 1,675
- first missing phase: function-ref index

`index180` validation:

- bridge owners: 1,670
- bridge boundary owners: 93
- indexed bridge owners: 1,670
- function-ref scanned functions: 1,670
- function-ref scanned instructions: 122,704
- external surfaces: 609
- registration sites: 690
- wrapper chains: 3,398

## Recommendation

Next sprint: implement bridge-index priority scheduling. Use the SPRINT-0030
default and `index180` loss table to make the function-ref index scan admitted
bridge owners, especially bridge boundary owners and oracle-adjacent owners in
selected packages, before lower-value owners consume the default budget.

Do not pursue broader local bridge discovery budgets next. The current fix
already admits the target owners; the loss has moved downstream.
