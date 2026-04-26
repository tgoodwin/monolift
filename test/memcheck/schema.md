# Memcheck JSON Schema

`cmd/memcheck` writes one JSON artifact per measured stage. The artifact is
updated on every poll tick and rewritten once more on process exit so a crashed
watchdog still leaves the latest observed peak-RSS evidence on disk.

## Top-Level Fields

- `label`: stage label such as `baseline-shape`, `after-fix-3-shape`, or
  `acceptance`.
- `command`: measured command argv as an array.
- `sample_ms`: RSS poll interval in milliseconds. Default: `250`.
- `rss_limit_mb`: whole-process-tree RSS kill-switch budget in megabytes.
- `wall_limit_sec`: wall-clock kill-switch budget in seconds.
- `runs`: one entry per cold-cache run. The wrapper captures seeds `101`,
  `202`, and `303`.
- `summary`: aggregated gate result for the artifact.
- `host`: execution host metadata (`os`, `arch`, `ncpu`, `go_version`,
  `gomaxprocs`).

## Per-Run Fields

- `seed`: fixed shuffle seed for this run.
- `exit_code`: measured command exit code. `0` means success.
- `killed`: whether the watchdog killed the process tree.
- `kill_reason`: `rss_limit`, `wall_limit`, or empty string.
- `elapsed_sec`: wall time for the run.
- `peak_tree_rss_kb`: maximum summed RSS across the live process tree.
- `peak_process_rss_kb`: maximum RSS for the heaviest single process observed.
- `peak_process_comm`: command name for the heaviest single process.
- `peak_process_pid`: pid for the heaviest single process.
- `peak_tree_processes`: largest live processes from the sample where
  `peak_tree_rss_kb` was observed, sorted by RSS descending.
- `last_observed_tree_rss_kb`: most recent tree RSS sample flushed to disk.

## Summary Fields

- `status`: one of `working`, `regressed`, `accepted`, `killed_rss`,
  `killed_time`.
- `baseline_artifact`: baseline artifact path used for ratcheting, if any.
- `candidate_artifact`: path to the artifact being written.
- `worst_peak_tree_rss_kb`: worst-run tree RSS across `runs`.
- `delta_pct`: percentage change vs. the baseline artifact, where negative means
  improvement.
- `spread_pct`: `(max_peak - min_peak) / max_peak * 100` across the runs.
- `stability_limit_pct`: maximum allowed `spread_pct` for this aggregate.
- `stability_ok`: `true` when `spread_pct <= stability_limit_pct`.

## Status Vocabulary

- `accepted`: all required runs exited `0`, no kill-switch fired, and the stage
  gate passed. When an absolute peak limit is configured with no reduction
  target, the stage is accepted by the cap and stability checks rather than by
  a byte-for-byte ratchet against the baseline artifact.
- `working`: candidate improved materially versus baseline but missed the
  stage-specific target.
- `regressed`: candidate got worse, improved too little, exited non-zero, or
  failed the `spread_pct <= 10` stability gate.
- `killed_rss`: the watchdog killed the process tree because summed RSS
  exceeded `rss_limit_mb`.
- `killed_time`: the watchdog killed the process tree because wall time
  exceeded `wall_limit_sec`.

## Kill-Switch Semantics

- The measured command runs in its own process group via
  `SysProcAttr{Setpgid: true}`.
- Every `sample_ms`, the watchdog runs `ps -o pid=,rss=,comm= -g <pgid>` and
  sums the process-tree RSS.
- If the summed RSS exceeds `rss_limit_mb`, the watchdog sends `SIGKILL` to the
  negative process-group id so the whole tree dies together.
- If the wall clock exceeds `wall_limit_sec`, the watchdog kills the same
  process group with `SIGKILL`.
- The final artifact preserves the last observed peak even when the run was
  killed.
