# SPRINT-0046 Mattermost MapFunc Propagation Profile

Command:

`GOWORK=/tmp/monolift-mattermost-sprint0046.work /tmp/monolift-activation-path --packages ./... --target channels/app/file.go:588 --timeout 30m --augmentations all --reverse-import-scope --profile --profile-output /tmp/SPRINT-0046-mapfunc-propagation.json --format text --verbose`

Profile JSON: `docs/research/runs/SPRINT-0046-mapfunc-propagation.json`

Mattermost target remained `GeneratePublicLinkHash` at `channels/app/file.go:588`; path length remained 10 and recommended cut step remained 9.

## Propagation Loop

The worklist propagation loop converged in 3 iterations and did not hit the cap of 20.

| Iter | Functions scanned | Callsites scanned | New stores | New parameter-store facts |
|---:|---:|---:|---:|---:|
| 0 | 22 | 32 | 23 | 8 |
| 1 | 4 | 4 | 3 | 1 |
| 2 | 0 | 0 | 0 | 0 |

The diagnostic appears once per augment pass because the shared `mapFuncIndex` is reused across the fixed-point loop; the underlying propagation stats are identical each time.

## Explore Dedup Validation

After adding the global `exploredRoots` set, Mattermost was rerun and saved to `docs/research/runs/SPRINT-0046-mattermost-1e.json`.

| Profile | Found | Path length | Cut step | Graph nodes | Graph edges | Augment time |
|---|---|---:|---:|---:|---:|---:|
| Before 1E | true | 10 | 9 | 99027 | 1469134 | 3m34.40s |
| After 1E | true | 10 | 9 | 99027 | 1469134 | 3m28.42s |

No graph edges were dropped by root deduplication in the Mattermost validation run.

## Early Termination Validation

The unit fixture `pkg/activation/testdata/simple` is RTA-reachable and now skips augmentation while preserving the RTA-only path length and recommended cut step. The fixture `pkg/activation/testdata/mapfunc/direct` still enters augmentation, proving augmented targets are not skipped.

Caddy `CleanPath` was also rerun and saved to `docs/research/runs/SPRINT-0046-caddy-early.json`. In this checkout it is not pure-RTA reachable under the current entrypoint set, so it correctly did not skip augmentation. The result still matched the baseline target properties: found=true, path length=11, recommended cut step=10, graph nodes=57939, graph edges=825464.

The full six-project early-termination check is saved to `docs/research/runs/SPRINT-0046-early-check.md`. It showed that listmonk was RTA-reachable but the RTA-only path changed from length 9/cut step 8 to length 11/cut step 10. Per SPRINT-0046 1F.5, early termination is therefore opt-in via `--skip-augment-if-rta-reachable` / `Config.SkipAugmentWhenRTAReachable` rather than default-on.
