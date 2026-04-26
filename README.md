# monolift
monolith modernizer

Design story and ADR tour: <https://tgoodwin.github.io/monolift/>.

## Dev workflow — test memory budget

Classifier-test memory pressure is gated by a committed watchdog harness. The default `make memcheck` (full-suite) target will be reliable once SPRINT-0010-GOLDENS lands the Caddy integration-test golden update + diagnostic-duplication fix. Until then, use the stage-specific targets:

- `make perf-rss-shape` — shape-package stage.
- `MONOLIFT_CORPUS_TESTS=1 make perf-rss-pocketbase` — PocketBase corpus stage.

See `test/memcheck/README.md` and `docs/sprints/SPRINT-0010-CLASSIFIER-PERF.md` for the harness contract and measurement history.
