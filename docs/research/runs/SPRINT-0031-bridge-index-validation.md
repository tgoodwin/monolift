# SPRINT-0031 Bridge Index Validation

Date: 2026-04-28

## Policy

The implemented bridge-index scheduler uses one generic deterministic policy:

1. boundary bridge owners;
2. selected-package bridge owners with direct touchpoint references;
3. remaining selected-package bridge owners;
4. other bridge owners.

Ties are sorted by boundary evidence count descending, direct touchpoint refs
descending, package path, object name, function string, and seed reasons. The
policy uses no oracle IDs, route names, application package names, or framework
strings.

## Artifacts

- `docs/research/runs/SPRINT-0031-bridge-validation-default.{json,stderr,summary.json}`
- `docs/research/runs/SPRINT-0031-bridge-validation-index65.{json,stderr,summary.json}`
- `docs/research/runs/SPRINT-0031-bridge-index-priority.md`

Both rows used the SPRINT-0028 oracle spec:
`docs/research/runs/SPRINT-0028-mattermost-oracle.json`.

## Matrix

Only the function-index budget varied. Bridge discovery budgets and owner caps
were unchanged.

| Row | Function-index budget | Probe wall | Wrapper real | Peak RSS | Indexed bridge owners | Skipped bridge owners | Target result |
|---|---:|---:|---:|---:|---:|---:|---|
| default | 60s | 79,961 ms | 97.32s | 8,849,690,648 | 0 / 1,676 | 1,676 | missed |
| index65 | 65s | 87,658 ms | 106.27s | 9,171,816,536 | 1,676 / 1,676 | 0 | recovered main chain |

Default diagnostics show the miss was not bridge discovery regression. The
target owners were admitted to the bridge seed set, but all admitted bridge
owners were skipped by `index_budget` before scanning began.

Priority class counts in the default row:

| Class | Owners | Indexed | Skipped |
|---|---:|---:|---:|
| boundary_bridge | 93 | 0 | 93 |
| touchpoint_ref_bridge | 618 | 0 | 618 |
| selected_package_bridge | 964 | 0 | 964 |
| other_bridge | 1 | 0 | 1 |

The `index65` row indexed all priority classes. It still recorded an index
budget stop during post-scan sorting/finalization, but no bridge owner was
skipped and downstream classification recovered.

## Target Nodes

| Target | Default selected/scanned | Default seed | Default indexed | Default final | index65 indexed | index65 final | Priority class/rank in index65 |
|---|---:|---:|---:|---:|---:|---:|---|
| `connectWebSocket` | yes | yes | no | no | yes | yes | selected_package_bridge / 1178 |
| `APIHandlerTrustRequester` | yes | yes | no | no | yes | yes | boundary_bridge / 73 |
| `InitWebSocket` | yes | yes | no | no | yes | yes | boundary_bridge / 87 |

## Relationships

| Relationship | Default | index65 |
|---|---|---|
| `connect-to-api-handler` | absent | recovered in final classification |
| `connect-registered-at-init` | absent | recovered in final classification |
| `init-has-http-boundary` | boundary evidence present; final relationship absent | boundary evidence present; final relationship absent |

## Cost Comparison

| Run | Probe wall / wrapper real | Peak RSS | Indexed owners | Target result |
|---|---:|---:|---:|---|
| SPRINT-0025 exhaustive/all upper bound | ~206.7s | ~12.47 GB | 140,801 | recovered |
| SPRINT-0028 oracle bridge v2 | ~78.4s / 103.62s | ~8.94 GB | 94 | recovered |
| SPRINT-0030 default validation | 86.36s / 109.20s | ~8.85 GB | 0 bridge owners | missed |
| SPRINT-0030 `index180` | 83.19s / 110.87s | ~9.04 GB | 1,670 bridge owners | recovered main chain |
| SPRINT-0031 default | 79.96s / 97.32s | ~8.85 GB | 0 bridge owners | missed |
| SPRINT-0031 index65 | 87.66s / 106.27s | ~9.17 GB | 1,676 bridge owners | recovered main chain |

## Findings

The selected policy adds the required owner-level diagnostics and deterministic
bridge-specific ordering without changing non-bridge index behavior. It does
not improve recall at the unchanged 60s budget because bridge seed discovery
can consume the entire function-index budget before the index phase starts.

The smallest observed useful budget increase was 65s. That is enough for this
run to scan all admitted bridge owners and recover the main chain, while keeping
the same bridge discovery budgets and owner caps.

## Recommendation

Next step: make bridge mode reserve or configure a small dedicated index slice
equivalent to the observed 65s function-index budget floor, rather than raising
bridge discovery budgets.
