# miniflux — cross-run composite annotation

**Corpus pin:** `evaluation/MANIFEST.yaml` as of 2026-04-19. 407 Go files.
Committed golden: `test/e2e/targets/miniflux/golden/report.json`.

## Cross-run summary

Miniflux is the **strongest before-vs-after contrast** in the corpus. Its worker-pool (`worker.Pool` / `internal/worker`) is already ADMITTED baseline — the admitted-state-with-externalized-persistence pattern. But the surrounding code — four distinct periodic loops (`feedScheduler`, `cleanupScheduler`, watchdog, metrics) plus a ProxyRotator singleton — sits just outside that baseline and shows exactly what happens when a scheduler moves to a platform trigger.

Miniflux's integration dispatches (fever, googlereader) use anonymous `go func(){}()` fire-and-forget — a TERMINAL pattern the research specifically identifies as outside v1 (no archetype for anonymous spawn over mutable closure without lifecycle).

## Triage convergences

| region | triage | archetype | convergence |
|---|---|---|---|
| `feedScheduler`, `cleanupScheduler`, watchdog, metrics — **M1–M4** | AUTO | `periodic-invocation` | opus (all 4); gpt-5.4 (scheduled-reconciler, batch); gemini (implicit) |
| `ProcessFeedEntries` (worker baseline) | ADMITTED | `replicated-stateless-service` | all 3 |
| `ProxyRotator` (proxyrotator.go:20-51) — **M6** | AUTO | `serialized-actor` | opus + gpt-5.4 (as serialized-singleton-owner) |
| fever / googlereader fire-and-forget dispatches | TERMINAL | — (fire-and-forget without lifecycle) | opus |

## Divergences and single-run findings

- **M1–M4 as four distinct periodic regions vs. compressed count:** opus enumerated all four; gpt-5.4 compressed into a single "timer-loop family" finding; gemini gestured high-level. All three agree on the AUTO triage at the category level.
- **Fire-and-forget dispatches as TERMINAL** — opus-only explicit finding. This is load-bearing for the research's TERMINAL-set characterization: v1 has no archetype for anonymous spawn over mutable closure without lifecycle vocabulary.
- gemini enumerated only 2 AUTO regions; gpt-5.4 1; opus 6. Depth difference, not substance difference — all three converged on the worker-pool-as-ADMITTED-baseline framing.

## Pointers

- `../runs/opus/annotations/miniflux.md` — 108 lines, M1–M6 region IDs, fire-and-forget TERMINAL finding.
- `../runs/gpt-5.4/annotations/miniflux.md` — 78 lines, before-vs-after contrast framing.
- `../runs/gemini/annotations/miniflux.md` — 30 lines.
