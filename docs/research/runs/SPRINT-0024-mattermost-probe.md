# SPRINT-0024 Mattermost EntryPath Probe

Date: 2026-04-27

## Command

The first attempt without the Mattermost workspace failed during package load
because `server/public` resolved from the module cache instead of the local
evaluation submodule. This matches the SPRINT-0023 workspace requirement.

```sh
go build -o /tmp/monolift-entrypath-probe ./cmd/entrypath-probe
GOWORK=/Users/tgoodwin/projects/monolift/.tmp/sprint-0021-a1-go.work \
  /tmp/monolift-entrypath-probe \
    --region-root '(*Hub).Start' \
    --region-root '(*WebConn).Pump' \
    /Users/tgoodwin/projects/monolift/evaluation/mattermost/server/cmd/mattermost \
    > /tmp/monolift-sprint-0024-mattermost-probe.json
```

## Outcome

Gate-A failed. With the required Mattermost `GOWORK`, the probe exceeded the
60s gate-A wall-clock ceiling before emitting JSON. The run was killed after
the budget was exceeded, and `/tmp/monolift-sprint-0024-mattermost-probe.json`
was 0 bytes.

No `ProbeResult` or `Stats` were emitted, so gates B-D were not evaluated.

## Next Probe

Split the probe timing into package load, SSA build, RTA/VTA construction,
reverse BFS, and function-value propagation. The next cheapest test is to run
only package load + SSA + RTA over the same `GOWORK` target, then add the
function-value walk with a narrowed starting set instead of seeding propagation
from every indexed function value in the program.

## Diagnostic Follow-up

After adding `--diagnostic-timings`, a five-minute diagnostic run was allowed
to exceed the original gate-A wall-clock ceiling:

```sh
go build -o /tmp/monolift-entrypath-probe ./cmd/entrypath-probe
timeout 300 env GOWORK=/Users/tgoodwin/projects/monolift/.tmp/sprint-0021-a1-go.work \
  /tmp/monolift-entrypath-probe \
    --diagnostic-timings \
    --region-root '(*Hub).Start' \
    --region-root '(*WebConn).Pump' \
    /Users/tgoodwin/projects/monolift/evaluation/mattermost/server/cmd/mattermost \
    > /tmp/monolift-sprint-0024-diagnostic.json \
    2> /tmp/monolift-sprint-0024-diagnostic.stderr
```

The run timed out after 300 seconds while still in `function_ref_index`; stdout
was 0 bytes. The completed phase timings were:

| Phase | Wall clock | Reported memory |
|---|---:|---:|
| package_load | 4.914s | 2.73 GB |
| ssa_build | 4.788s | 4.49 GB |
| root_resolution | 16.270s | 6.43 GB |
| callgraph | 39.702s | 6.68 GB |
| reverse_bfs | 0.354s | 6.68 GB |
| function_ref_index | >233s, timed out | >=6.68 GB |

This narrows the failure: callgraph construction is expensive but finished
within the five-minute diagnostic window; reverse BFS is cheap. The dominant
remaining wall-clock problem is whole-program function-reference indexing, and
the dominant memory problem appears before that, during SSA/root resolution.

The next probe should avoid enumerating every SSA function for root resolution
and should build the function-value index from a narrowed starting set:
functions discovered on reverse paths, functions flowing into HTTP-shaped sinks,
and candidate external surfaces, rather than every function value in the
program.
