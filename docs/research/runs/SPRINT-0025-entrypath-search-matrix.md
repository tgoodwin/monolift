# SPRINT-0025 EntryPath Search Matrix

Date: 2026-04-27

This matrix summarizes the measured SPRINT-0025 EntryPath diagnostics. Raw
commands, stderr phase lines, and JSON artifacts are linked from
[`SPRINT-0025-entrypath-baseline.md`](SPRINT-0025-entrypath-baseline.md).

## Matrix

The table columns cover the required command, budget, function-index wall time,
peak RSS, scan counts, recovered result counts, target-symbol recovery checks,
stop reason, and confidence.

| Mode / run | Command | Budget | Function-index wall ms | Peak RSS | Scanned functions | Scanned instructions | External surfaces | Registration sites | Wrapper chains | `connectWebSocket` recovered | `APIHandlerTrustRequester` recovered | `http.Handler` sink reached | Stop reason | Confidence |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|---|---|---|
| `all` | `--function-index-budget=120s` with two exact roots | 120s index | 120,038 | 12,458,504,928 | 140,801 | 5,625,718 | 2,956 | 3,982 | 64,314 | yes | yes (28 links) | yes (702 sites) | `function_ref_index_budget_exceeded` during finalization/sort | high |
| `reverse-path` | `--function-index-mode=reverse-path --function-index-budget=60s` with two exact roots | 60s index | 7,081 | 6,193,044,984 | 11,115 | 538,869 | 126 | 252 | 12,211 | no | no | yes (3 sites) | completed | high |
| `http-sinks` | `--function-index-mode=http-sinks --function-index-budget=60s` with two exact roots | 60s index | 60,027 | 7,867,346,744 | 0 | 0 | 0 | 0 | 0 | no | no | no | budget spent during HTTP seed discovery; `function_ref_index_budget_exceeded` | high |
| `targeted-default` | `--function-index-mode=targeted --function-index-budget=60s` with default targeted bounds | 60s index, 30s expansion | 61,032 | 7,971,815,240 | 0 | 0 | 0 | 0 | 0 | no | no | no | `targeted_index_budget_exceeded` before final seeded scan | high |
| `targeted-expanded` | `--function-index-mode=targeted --function-index-budget=120s --targeted-max-depth=2 --targeted-max-duration=90s --targeted-max-functions=50000 --targeted-max-queue=500000` | 120s index, 90s expansion | 120,804 | 8,109,514,600 | 0 | 0 | 0 | 0 | 0 | no | no | no | `targeted_index_budget_exceeded` before final seeded scan | high |
| `reverse-path` one-root scaling | `--function-index-mode=reverse-path --region-root '...(*Hub).Start'` | 60s index | 294 | 5,680,001,392 | 244 | 13,898 | 16 | 21 | 769 | no | no | yes (3 sites) | completed | high |
| `reverse-path` synthetic fixture | `--function-index-mode=reverse-path --region-root root testdata/reverse_path_seed` | none | 0 | 764,431,784 | 4 | 9 | 1 | 1 | 3 | n/a | n/a | yes (1 site) | completed | high |

## Hypotheses

- **H1 confirmed:** Reverse BFS is cheap. Mattermost reverse BFS stayed below
  1s across full and seeded runs.
- **H2 confirmed:** Whole-program function-reference indexing is the dominant
  incremental EntryPath cost after loader/SSA/callgraph. The all-mode index
  consumed 120s with budget and 168s in the no-budget baseline.
- **H3 confirmed:** Reverse-path seeding alone is too narrow for Mattermost.
  It completed cheaply but did not recover `connectWebSocket`.
- **H4 falsified for the current implementation:** HTTP-sink seeding plus
  targeted expansion did not reach final seeded scanning before budget
  exhaustion. An optimized non-whole-program HTTP seed source remains unmeasured.
- **H5 confirmed:** The old 2.5 GB memory gate is not a useful single gate.
  Loader/SSA/callgraph exceeded it before function indexing, and seeded modes
  still peaked above 5 GB on Mattermost.

## Recommended Next Sprint Shape

**Another diagnostic with a precise next question.**

Next question: can HTTP-shaped seed discovery be made incremental from the
reverse-path owner set and callgraph-adjacent functions, avoiding the current
whole-program HTTP seed scan while still recovering `connectWebSocket` and the
`APIHandlerTrustRequester` registration chain?

## Do Not Pursue Next

- Do not wire `all` mode into report or surface classification. It recovers the
  target chain but has unacceptable scan volume and memory pressure.
- Do not broaden reverse-path mode with framework or Mattermost-specific
  recognizers. It is cheap, but the miss is structural.
- Do not keep the current whole-program HTTP seed discovery as the targeted
  entrypoint. It spent the budget before final seeded scanning.
- Do not treat a larger single budget as the fix. Larger targeted budgets
  increased seed counts but still recovered no Mattermost target evidence.

## Proposed Cost Gate

Use a split gate for future Mattermost EntryPath work:

- **Baseline analysis gate:** package load + SSA + root resolution + callgraph
  must stay under 90s wall and 8 GB RSS. The measured runs already reached
  roughly 5.1-7.2 GB before seeded indexing, so a 2.5 GB total-process gate is
  not meaningful for Mattermost.
- **Incremental EntryPath gate:** the selected seeded mode must add no more
  than 30s wall and 1.5 GB RSS after callgraph. Reverse-path met this
  incremental shape (7.1s index phase, ~0.6 GB RSS increase) but missed the
  target chain. All-mode and current HTTP/targeted modes fail this gate.

The gate should be applied only after the next diagnostic identifies a seeded
mode that recovers the Mattermost target evidence.
