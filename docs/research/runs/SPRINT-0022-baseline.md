# SPRINT-0022 Mattermost single-Hub baseline

## Command

```sh
GOWORK=$PWD/.tmp/sprint-0021-a1-go.work \
MONOLIFT_PROFILE_DIR=/tmp/monolift-s22-prof.1Rltzp \
/usr/bin/time -l ./bin/e2e-compile \
  --target=mattermost \
  --output=/tmp/monolift-s22-hub.cqvTUA \
  --source=evaluation/mattermost/server
```

The `GOWORK` setting matches SPRINT-0021 A.1 and points at the local
`evaluation/mattermost/server` and `evaluation/mattermost/server/public`
modules. Without it, `packages.Load` resolves the wrong public model surface and
fails before closure analysis.

## Result

- Status: success.
- Wall time: 70.19s.
- Max RSS: 1,886,863,360 bytes.
- Peak memory footprint: 9,621,884,416 bytes.
- Included symbols: 2,956.
- Excluded symbols: 4,838.
- Profiles:
  - `/tmp/monolift-s22-prof.1Rltzp/mattermost.cpu.pprof`
  - `/tmp/monolift-s22-prof.1Rltzp/mattermost.heap.pprof`
  - `/tmp/monolift-s22-prof.1Rltzp/mattermost.memstats.json`

This is in the same order as the SPRINT-0021 reference run: 88.22s wall,
2,152,579,072 bytes max RSS, 2,956 included symbols, and 4,838 excluded symbols.
