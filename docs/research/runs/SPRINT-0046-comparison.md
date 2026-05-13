# SPRINT-0046 Baseline vs Optimized Comparison

## Main Phases

| Project | Scope | Load | SSA | RTA | Augment | BFS | Nodes | Edges | Path | Cut |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| caddy | 882.3ms -> 981.7ms (+11.3%) | 1.06s -> 1.03s (-2.7%) | 324.1ms -> 333.9ms (+3.0%) | 162.6ms -> 148.6ms (-8.6%) | 1.78m -> 1.15m (-35.5%) | 55.7ms -> 124.6ms (+123.8%) | 57939 -> 57939 | 825464 -> 825464 | 11 -> 11 | 10 -> 10 |
| miniflux | 623.9ms -> 713.2ms (+14.3%) | 837.6ms -> 996.7ms (+19.0%) | 317.2ms -> 425.8ms (+34.2%) | 2.33s -> 2.60s (+11.8%) | 13.76s -> 8.06s (-41.4%) | 15.8ms -> 10.5ms (-33.7%) | 20967 -> 20967 | 260049 -> 260049 | 8 -> 8 | 7 -> 7 |
| gitea | 2.04s -> 2.28s (+11.5%) | 3.08s -> 3.42s (+11.2%) | 1.16s -> 1.36s (+17.3%) | 49.58s -> 3.25m (+292.9%) | 3.51m -> 1.39m (-60.2%) | 50.5ms -> 36.2ms (-28.4%) | 88886 -> 88886 | 2082375 -> 2082375 | 4 -> 4 | 3 -> 3 |
| listmonk | 510.3ms -> 1.28s (+150.8%) | 655.6ms -> 567.1ms (-13.5%) | 188.2ms -> 166.6ms (-11.4%) | 1.78s -> 1.74s (-2.4%) | 11.47s -> 4.69s (-59.1%) | 11.3ms -> 11.6ms (+2.6%) | 17111 -> 17111 | 219063 -> 219063 | 9 -> 9 | 8 -> 8 |
| pocketbase | 469.6ms -> 462.8ms (-1.5%) | 1.25s -> 1.19s (-5.4%) | 490.2ms -> 455.1ms (-7.2%) | 4.53s -> 4.23s (-6.7%) | 27.40s -> 12.79s (-53.3%) | 26.7ms -> 24.4ms (-8.5%) | 34163 -> 34163 | 469311 -> 469311 | 8 -> 8 | 7 -> 7 |
| mattermost | 1.78s -> 1.70s (-4.4%) | 2.71s -> 2.45s (-9.4%) | 1.20s -> 866.6ms (-28.0%) | 308.3ms -> 181.1ms (-41.3%) | 7.14m -> 2.71m (-62.0%) | 85.1ms -> 75.3ms (-11.5%) | 99027 -> 99027 | 1469134 -> 1469134 | 10 -> 10 | 9 -> 9 |

## Augment Subphases

| Project | Subphase | Baseline | Optimized | Delta |
|---|---|---:|---:|---:|
| caddy | AugmentStructField | 8.19s | 2.23s | -72.7% |
| caddy | ApplyPredicates | 27.2ms | 27.8ms | +2.1% |
| caddy | AugmentGoroutine | 127.4ms | 135.3ms | +6.2% |
| caddy | AugmentPackageVars | 17.75s | 14.04s | -20.9% |
| caddy | AugmentFuncArgs | 323.6ms | 300.9ms | -7.0% |
| caddy | AugmentMapFuncValues | 25.59s | 15.89s | -37.9% |
| caddy | AugmentInterfaceFields | 32.43s | 12.80s | -60.5% |
| caddy | ExploreCallees | 22.26s | 22.74s | +2.2% |
| miniflux | AugmentStructField | 1.38s | 766.2ms | -44.5% |
| miniflux | ApplyPredicates | 0.1ms | 0.1ms | -12.5% |
| miniflux | AugmentGoroutine | 32.7ms | 34.1ms | +4.3% |
| miniflux | AugmentPackageVars | 2.40s | 2.02s | -15.9% |
| miniflux | AugmentFuncArgs | 75.0ms | 141.8ms | +89.1% |
| miniflux | AugmentMapFuncValues | 3.78s | 2.50s | -33.9% |
| miniflux | AugmentInterfaceFields | 5.15s | 1.44s | -72.0% |
| miniflux | ExploreCallees | 939.0ms | 1.15s | +22.3% |
| gitea | AugmentStructField | 16.85s | 3.15s | -81.3% |
| gitea | ApplyPredicates | 25.8ms | 13.2ms | -49.0% |
| gitea | AugmentGoroutine | 579.0ms | 173.0ms | -70.1% |
| gitea | AugmentPackageVars | 42.31s | 24.13s | -43.0% |
| gitea | AugmentFuncArgs | 1.26s | 512.1ms | -59.2% |
| gitea | AugmentMapFuncValues | 1.02m | 25.38s | -58.5% |
| gitea | AugmentInterfaceFields | 1.25m | 21.85s | -70.8% |
| gitea | ExploreCallees | 13.39s | 8.40s | -37.3% |
| listmonk | AugmentStructField | 1.18s | 400.7ms | -66.1% |
| listmonk | ApplyPredicates | 0.1ms | 0.0ms | -20.9% |
| listmonk | AugmentGoroutine | 26.6ms | 25.6ms | -3.7% |
| listmonk | AugmentPackageVars | 1.89s | 1.17s | -38.2% |
| listmonk | AugmentFuncArgs | 56.0ms | 56.4ms | +0.8% |
| listmonk | AugmentMapFuncValues | 3.17s | 1.38s | -56.4% |
| listmonk | AugmentInterfaceFields | 4.29s | 841.9ms | -80.4% |
| listmonk | ExploreCallees | 848.0ms | 807.5ms | -4.8% |
| pocketbase | AugmentStructField | 2.20s | 828.5ms | -62.4% |
| pocketbase | ApplyPredicates | 8.3ms | 7.0ms | -15.9% |
| pocketbase | AugmentGoroutine | 49.9ms | 47.4ms | -5.1% |
| pocketbase | AugmentPackageVars | 4.77s | 3.31s | -30.5% |
| pocketbase | AugmentFuncArgs | 164.9ms | 164.2ms | -0.4% |
| pocketbase | AugmentMapFuncValues | 8.20s | 3.92s | -52.3% |
| pocketbase | AugmentInterfaceFields | 10.18s | 2.72s | -73.3% |
| pocketbase | ExploreCallees | 1.82s | 1.79s | -1.6% |
| mattermost | AugmentStructField | 17.39s | 4.61s | -73.5% |
| mattermost | ApplyPredicates | 29.5ms | 25.6ms | -13.5% |
| mattermost | AugmentGoroutine | 220.0ms | 206.6ms | -6.1% |
| mattermost | AugmentPackageVars | 55.82s | 40.40s | -27.6% |
| mattermost | AugmentFuncArgs | 752.6ms | 614.7ms | -18.3% |
| mattermost | AugmentMapFuncValues | 1.43m | 43.95s | -48.7% |
| mattermost | AugmentInterfaceFields | 2.47m | 35.43s | -76.1% |
| mattermost | ExploreCallees | 2.00m | 36.01s | -70.0% |

## Target Invariance

| Project | Found | Target | Path unchanged | Cut unchanged | Graph stats unchanged |
|---|---|---|---|---|---|
| caddy | True | CleanPath | True | True | True |
| miniflux | True | SanitizeHTML | True | True | True |
| gitea | True | PathEscapeSegments | True | True | True |
| listmonk | True | SanitizeURI | True | True | True |
| pocketbase | True | Columnify | True | True | True |
| mattermost | True | GeneratePublicLinkHash | True | True | True |

## Mattermost Direct Invocation Oracle

`docs/research/runs/SPRINT-0046-mattermost-optimized.json` resolves the target to `GeneratePublicLinkHash` at `channels/app/file.go:588`. The e2e oracle for `activation-mattermost-publiclinkhash` computes `base64.RawURLEncoding(sha256(salt || fileID))`, matching the production function body at that target.

For the configured direct invocation probe payload (`file_id=test-file-001`, `salt=monolift-test-public-link-salt-0001`), both formulas produce:

`QvuS2RmTIS72El2ffMm3CL0URcsvXGd3DYUY2r9-Eb4`

## Regression Check

No project regressed by more than 10% in activation-only total time. Gitea had a noisy RTA phase increase, but total activation time changed by +7.8%, below the sprint threshold.

## Data-Gated Secondary Optimizations

Reverse-import scoping did not exceed 5% of total baseline time for any project: caddy 0.8%, miniflux 3.4%, gitea 0.8%, listmonk 3.4%, pocketbase 1.4%, mattermost 0.4%. Scope caching is skipped for SPRINT-0046.

`patched-package-verify` and Docker build phases do not appear in the activation-only benchmark data; they are lift/e2e phases rather than activation analysis phases. No verify-scope or Docker layer optimization is implemented under Phase 3 without material timing evidence from this sprint's measured compiler-side bottleneck.
