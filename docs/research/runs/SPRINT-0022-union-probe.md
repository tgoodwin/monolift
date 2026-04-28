# SPRINT-0022 Mattermost closure-union probe

## Command

```sh
GOWORK=$PWD/.tmp/sprint-0021-a1-go.work \
MONOLIFT_PROFILE_DIR=/tmp/monolift-s22-union-prof.Scgirt \
/usr/bin/time -l ./bin/e2e-compile \
  --target=mattermost \
  --output=/tmp/monolift-s22-union.L7it5X \
  --source=evaluation/mattermost/server \
  --source=test/e2e/targets/mattermost
```

The `test/e2e/targets/mattermost` source directory supplies the shared-name
Hub/WebConn pragma overlay. The e2e-compile loader resolves the overlay roots to
the real Mattermost source files before extraction.

## Result

- Status: success.
- Wall time: 127.96s.
- Max RSS: 2,071,986,176 bytes.
- Peak memory footprint: 12,667,446,592 bytes.
- Included symbols: 3,025.
- Excluded symbols: 4,889.
- Profiles:
  - `/tmp/monolift-s22-union-prof.Scgirt/mattermost.cpu.pprof`
  - `/tmp/monolift-s22-union-prof.Scgirt/mattermost.heap.pprof`
  - `/tmp/monolift-s22-union-prof.Scgirt/mattermost.memstats.json`

## Budget

The probe stayed below the C.gate-1 stop budget of 30 minutes wall time and
16 GiB max RSS.
