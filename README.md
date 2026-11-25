# monolift

Monolift is a research compiler for deriving alternative Kubernetes deployments from an existing Go application. The goal is to establish which boundaries are valid and let a control plane choose among supported deployments as demand and available resources change.

- [Thesis direction](docs/research/thesis-direction.md) — the PLOS ’25 experiments and plots, the proposed compiler guarantees, and the adaptive deployment control plan.
- [Design and implementation tour](https://tgoodwin.github.io/monolift/) — walkthrough, compiler internals, and design history.

## Dev workflow — test memory budget

Classifier-test memory pressure is gated by a committed watchdog harness. The default `make memcheck` (full-suite) target will be reliable once SPRINT-0010-GOLDENS lands the Caddy integration-test golden update + diagnostic-duplication fix. Until then, use the stage-specific targets:

- `make perf-rss-shape` — shape-package stage.
- `MONOLIFT_CORPUS_TESTS=1 make perf-rss-pocketbase` — PocketBase corpus stage.

See `test/memcheck/README.md` and `docs/sprints/SPRINT-0010-CLASSIFIER-PERF.md` for the harness contract and measurement history.
