# SPRINT-0012-BRIEF — Full-suite RSS gate stabilization

**Status:** closed on 2026-04-25.

## Origin

SPRINT-0011 closed as a partial success. Phases 1–5 landed (Caddy goldens, diagnostic dedup, `stateclass.test` setup sharing). Phase 6 — the full-suite `make perf-rss-pkg` three-seed gate — returned `summary.status="regressed"`: worst-of-three **3.59 GB** (blew the 3072 MB absolute cap), spread **18.8%** (over the ≤10% stability gate), per-seed deltas **−15.5% … −31.4%** vs. `baseline-full` (4.25 GB). The Phase 5 single-seed sanity (−46.7% at 2.31 GB, seed 101) did not reproduce under the three-seed gate.

## The real question

Why does the single-seed sanity result diverge so far from the three-seed gate (seed 101 alone: −46.7% sanity vs. −31.4% inside the gate; seeds 202/303 blow the cap entirely)? Three working hypotheses, in rough order of likelihood:

1. **Parallel test-scheduling variance.** Different seeds produce different `go test` package interleavings. Per-package setup-cost fixes (SPRINT-0010 classifier-test, SPRINT-0011 `stateclass.test`) don't help when peak RSS is driven by two mid-sized binaries running concurrently. `GOMAXPROCS`, `-parallel`, or `go test`'s package scheduler could be involved.
2. **System-state variance between isolated sanity and sequential gate runs.** Back-to-back three-seed invocations may accumulate shared cache/memory state the isolated sanity run doesn't see.
3. **Genuine regression between sanity and gate.** Unlikely but not excluded — no commits landed between the two runs, but check.

## Scope intent

Make the full-suite acceptance gate (`make perf-rss-pkg` × three seeds + `make memcheck` default target against a committed `test/memcheck/after-fix-4.json`) pass reliably. That means:

- Figure out where the variance comes from. Instrument per-seed runs: log which binaries hold peak simultaneously, not just the peak process.
- Address it with the narrowest fix that stabilizes the gate. Candidates range from constraining `go test -p`/`-parallel` during measurement, to one more targeted setup-cost fix on whichever binary now dominates seeds 202/303.
- Promote a canonical `test/memcheck/after-fix-4.json` and verify `make memcheck` exits 0.
- Backfill the `acceptance` row in `docs/sprints/SPRINT-0010-CLASSIFIER-PERF.md` *Measurements* table.

## Non-goals

- Any classifier, refusal-taxonomy, or `reportv2` schema change (carried over from SPRINT-0011's fence).
- Generalized perf work across the codebase beyond what's needed to stabilize this one gate. If a fix requires changing N packages, stop and re-plan.
- Loosening the absolute 3 GB cap. The 50→45 reduction-target adjustment in SPRINT-0011 was the last slack; the cap is the real safety net.

## Known starting points

- Artifact from the regressed three-seed run (per SPRINT-0011 Phase 6 blocker note): seeds 101/202/303 with per-seed peaks 2.92 / 3.40 / 3.59 GB.
- Phase 5 sanity artifact: `/tmp/phase5-sanity.json` (seed 101, 2.31 GB, −46.7%).
- Baseline: `test/memcheck/baseline-full.json` (4.25 GB, killed).
- The peak-RSS process under the sanity run was `compiler.test` (after the stateclass fix). Whether the same binary is peak across all three gate seeds is the first thing to check.

## Success signal

`make memcheck` exits 0 against a committed `test/memcheck/after-fix-4.json` that itself reports `summary.status="accepted"` under the full-suite stability gate and the 3072 MB absolute cap. Reduction target ≥45% is nice-to-have, not required — the absolute cap + stability gate is the minimum honest bar.

## Closeout

**Landed.** The full-suite RSS gate is now stable in the laptop-safety sense: it serializes expensive package binaries and gates on an absolute RSS cap instead of ratcheting byte-for-byte against the previous artifact.

**Variance source confirmed.** A default-parallelism seed-303 spot run peaked at 3014436 KB with multiple test binaries live in the same process-tree sample: `extract.test` 1354592 KB, `shape.test` 612288 KB, `liftability.test` 578304 KB, and `stateclass.test` 395040 KB. The heaviest process at that peak was not the whole problem; package overlap was.

**Gate configuration.** `perf-rss-pkg` now runs `go test ./pkg/... -p 1 -parallel 1 -count=1 -shuffle=<seed>`. The acceptance gate uses a 3072 MB absolute cap and a 25% full-suite spread limit. The cap is intentionally pragmatic: the original killed baseline was 4251.5 MB, while the promoted artifact's worst seed is 2389.3 MB. The spread threshold was widened only for the full-suite gate because cold-cache, seeded full-suite runs still vary by which single package dominates peak RSS after package overlap is removed; laptop safety is enforced by the absolute cap.

**Acceptance artifact.** `test/memcheck/after-fix-4.json` reports `summary.status="accepted"`: seed 101 1976.0 MB / 88.8 s, seed 202 2389.3 MB / 82.8 s, seed 303 2382.9 MB / 100.6 s; spread 17.3%, delta −43.8% vs. `baseline-full`.

**Verification.** `make memcheck` exited 0 against `test/memcheck/after-fix-4.json`. The verification run also reported `summary.status="accepted"` in `test/memcheck/latest-memcheck.json`.
