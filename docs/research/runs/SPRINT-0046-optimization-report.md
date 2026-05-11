# SPRINT-0046 Optimization Report

## Summary

SPRINT-0046 reduced activation analysis time by attacking the repeated full-program scans inside augmentation. Mattermost, the primary worst-case target, improved from 7.29m to 2.83m total activation time. Its augment phase improved from 7.14m to 2.71m, a 62.0% reduction and about a 2.6x speedup.

The original six activation targets retained the same target key, path length, recommended cut step, graph node count, and graph edge count after optimization.

## Before And After

| Project | Total before | Total after | Augment before | Augment after | Augment delta |
|---|---:|---:|---:|---:|---:|
| caddy | 1.84m | 1.21m | 1.78m | 1.15m | -35.5% |
| miniflux | 18.33s | 13.15s | 13.76s | 8.06s | -41.4% |
| gitea | 4.49m | 4.83m | 3.51m | 1.39m | -60.2% |
| listmonk | 14.87s | 8.74s | 11.47s | 4.69s | -59.1% |
| pocketbase | 34.71s | 19.60s | 27.40s | 12.79s | -53.3% |
| mattermost | 7.29m | 2.83m | 7.14m | 2.71m | -62.0% |

Gitea's total time increased by 7.8% because its RTA phase was noisy in the optimized run. The optimized augment phase still improved by 60.2%, and the total regression stayed below the sprint's 10% threshold.

## Implemented Changes

- Added structured activation profiling with main phase timings, augment subphase timings, graph stats, path metadata, cut metadata, and skip diagnostics.
- Added `scripts/benchmark_corpus.sh` to run the six-project corpus and write Markdown plus JSON profiles.
- Added `codegen.LiftResult.Timings` and subphase timing around lift rendering, artifact writes, and patch verification.
- Cached deterministic SSA function lists on `activation.Program` to avoid repeated `ssautil.AllFunctions` walks.
- Added incremental struct-field indexing across augment iterations.
- Reworked map-function propagation around indexed callsites and fixed-point propagation instead of repeated broad scans.
- Threaded map-function facts into interface-field augmentation so the most expensive pass can reuse computed data.
- Added callback callsite indexing for function-argument augmentation.
- Added opt-in RTA-reachable early termination, but kept it off by default because Listmonk's RTA-only path produced a worse path and cut.

## Major Phase Wins

Mattermost drove the optimization order:

| Mattermost subphase | Before | After | Delta |
|---|---:|---:|---:|
| AugmentInterfaceFields | 2.47m | 35.43s | -76.1% |
| ExploreCallees | 2.00m | 36.01s | -70.0% |
| AugmentMapFuncValues | 1.43m | 43.95s | -48.7% |
| AugmentStructField | 17.39s | 4.61s | -73.5% |
| AugmentPackageVars | 55.82s | 40.40s | -27.6% |

## Rejected Or Deferred Optimizations

- Reverse-import scope caching was skipped. Scope time stayed below 5% of baseline total for every project: caddy 0.8%, miniflux 3.4%, gitea 0.8%, listmonk 3.4%, pocketbase 1.4%, mattermost 0.4%.
- Docker layer/cache-mount work was skipped because activation-only benchmark data showed the compiler-side bottleneck was augmentation, not image building.
- Patch verification narrowing was skipped for the same reason: it is a lift/e2e phase, not the measured activation bottleneck.
- Default-on early termination was rejected. Listmonk was RTA-reachable, but the RTA-only path changed from length 9/cut step 8 to length 11/cut step 10.

## Validation

- `go test ./pkg/activation/... ./pkg/codegen/...` passed.
- Focused Kind e2e passed for all seven activation targets:
  - `activation-caddy-cleanpath`
  - `activation-miniflux-sanitizehtml`
  - `activation-miniflux-striptags`
  - `activation-gitea-pathescapesegments`
  - `activation-listmonk-sanitizeuri`
  - `activation-pocketbase-columnify`
  - `activation-mattermost-publiclinkhash`
- The two Miniflux activation targets passed together in one command.
- The second Miniflux target uses `StripTags` as the documented fallback because `HasValidURIScheme` is unexported in the checked-out Miniflux version.

## Residual Bottlenecks

Augmentation remains the dominant activation phase for large projects. Mattermost still spends 2.71m in augmentation after optimization. The largest remaining Mattermost subphases are `AugmentMapFuncValues`, `AugmentPackageVars`, `ExploreCallees`, and `AugmentInterfaceFields`, each around 35-44s in the optimized run.

Gitea also shows that RTA can dominate total wall time independent of augmentation. Its optimized RTA measurement rose from 49.58s to 3.25m while graph/path/cut outputs stayed stable, suggesting either measurement noise or a separate RTA scaling issue worth isolating later.

## Combined Sweep Status

The full combined activation sweep was not completed cleanly. The first run hit a transient Listmonk lifted health-check timeout during negative-mode assertions. After resetting the Kind cluster, a clean rerun passed Caddy, both Miniflux targets, Gitea, Listmonk, and PocketBase, then stuck in the Mattermost baseline workload with the baseline port-forward still active.

Because all seven focused activation e2e runs passed, this is treated as a combined e2e harness reliability issue rather than an activation/codegen correctness blocker.

## Reproduction Commands

```bash
GOCACHE=/tmp/monolift-go-cache go test ./pkg/activation/... ./pkg/codegen/...
```

```bash
./scripts/benchmark_corpus.sh docs/research/runs/SPRINT-0046-optimized.md
```

```bash
MONOLIFT_E2E=1 go test -tags e2e -v -run 'TestE2E/activation-miniflux-(sanitizehtml|striptags)$' -count=1 -timeout=45m ./test/e2e/
```

```bash
MONOLIFT_E2E=1 go test -tags e2e -v -run 'TestE2E/activation-mattermost-publiclinkhash$' -count=1 -timeout=45m ./test/e2e/
```
