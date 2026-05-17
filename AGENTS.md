# Instructions for AI agents

## Run heavy work on the CloudLab build node, not locally

This repo's compiler, e2e suite, and corpus sweeps are memory-intensive (gitea SSA construction, mattermost analysis, full Kind clusters). Do **not** run them on the developer's macbook. SSH into the CloudLab build node and execute there.

```sh
# Find the current experiment + SSH command
cl status <name|id>

# Drop into a shell on the build node
cl ssh <name|id>
```

`cl` is the local CLI wrapper at `cloudlab/cl` (already on PATH). It reads `~/Downloads/cloudlab.jwt` via the repo's `.env`. If `cl ls` shows no experiments, ask the user to spin one up from the `monolift-buildserver` profile.

The build node already has Go, Docker, kind, kubectl, k9s, and the pinned evaluation/* clones (provisioned by `cloudlab/setup.sh`). Stage all command output, sweep results, and coverage reports under `.moab/runs/` on the node, then surface paths back to the user.

What this means in practice:

- `go test ./pkg/...`, `make e2e`, the `scripts/run_activation_corpus_sweep.sh` runner, and any `cmd/activation-path` invocation against real corpus targets - run on the build node.
- Codegen unit tests that don't touch evaluation/* (golden tests, small fixtures) can run locally.
- When in doubt, run remotely.

## Research mode: do not let timeout caps decide viability

During research phases, timeouts and internal caps are instrumentation, not
admission facts. Treat them as evidence about cost and where the analyzer or
generator spends time, but do **not** use them as the reason to reject a
research direction.

What this means in practice:

- If an internal cap fires, rerun with the cap disabled or widened before
  classifying the candidate.
- Record timing breakdowns under `.moab/runs/` so future agents can see whether
  cost is in scope, package load, SSA, RTA, augmentation, extraction report,
  admission, build-plan, image build, Kind deploy, or workload runtime.
- Defer a candidate only for semantic or scope reasons: wrong selected cut,
  app/shared-state receiver, unsupported boundary/result shape, missing
  reconstructor family, generated build failure, workload/fixture requirements
  outside sprint scope, or runtime behavior that violates the declared contract.
- When reporting a candidate, separate "cost profile" from "blocker". A slow
  targeted run can be important research evidence without being a refusal.
- Do not compensate for timeout pressure by broadening to whole-repository
  admission. Focused research should use reverse-import scope or an explicit
  target/importer package set.
