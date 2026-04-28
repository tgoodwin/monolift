# gitea — cross-run composite annotation

**Corpus pin:** `evaluation/MANIFEST.yaml` as of 2026-04-19. 2875 Go files (largest target alongside mattermost). Mandatory subagent delegation per sprint plan; all three runs applied the owned-directory bundle ledger (boot/lifecycle, ingress, domain services, background/async, infra/runtime, persistence).

## Cross-run summary

Gitea's archetype surface **concentrates in seven infrastructure bundles**: `modules/queue`, `modules/eventsource`, `modules/indexer`, `modules/cache`, `modules/session`, `services/cron`, `services/actions` — plus a `lifecycle-state-machine` flavor in `modules/graceful` and `modules/process` that is TERMINAL for v1. The 1,660+ domain-service files (`routers/api`, `routers/web`, `services/{auth,user,org,…}`, `models/`) overwhelmingly exhibit **no archetype surface at all** — they are request-scoped handlers operating on DB-backed models with context propagation, which is the admitted baseline. This is a load-bearing finding: **large targets do not inflate archetype vocabulary; they concentrate AUTO findings in a small number of infrastructure bundles**.

All three runs converged on the infrastructure-bundle concentration. Opus enumerated deepest (G1–G18 region IDs). gpt-5.4 picked out the strongest candidates (queue manager, indexer queues, eventsource manager, cron). Gemini covered all 6 bundles with terser per-bundle depth.

## Triage convergences (dominant archetypes)

| region family | archetype | convergence |
|---|---|---|
| `modules/queue.WorkerPoolQueue` — **G1** | AUTO `bounded-worker-pool` | all 3 |
| `queue.baseChannel.set` (uniqueness guard) — **G2** | AUTO `keyed-partitioned-state` (opus) / composite (gpt-5.4) | opus + gpt-5.4 |
| `queue.Manager` registry (manager.go:18) — **G4** | AUTO `serialized-actor` + `keyed-partitioned-state` | opus + gemini |
| `eventsource.Manager` (manager.go:11) — **G6** | AUTO `serialized-actor` | all 3 |
| `eventsource.Messenger` (messenger.go:9) — **G7** | AUTO `fanout-publisher` / `connection-hub-buffer` (gpt-5.4 lens) | all 3 |
| `EphemeralCache` (ephemeral.go:20) — **G10** | AUTO `ttl-cache` | opus |
| `session.DBStore`/`RedisStore`/`VirtualStore` — **G11–G13** | AUTO `session-affinity-state` | opus + gemini |
| `services/cron.Task` + gocron (tasks.go:36) — **G14** | AUTO `periodic-invocation` | all 3 |
| `services/cron` task registry — **G15** | AUTO `serialized-actor` | opus |
| `modules/process.Manager` (manager.go:70-71) — **G18** | AUTO `serialized-actor` + `keyed-partitioned-state` composite | opus |
| `graceful.Manager` (init → running → shutdown → terminate) | TERMINAL v1 | all 3 (opus named `lifecycle-state-machine` candidate, retired for v1) |
| `modules/storage` local storage — **G-FS** | AUTO `filesystem-bound-singleton` | gemini |

## Divergences and single-run findings

- **Gitea concentrates on 7 infrastructure bundles; the 1,660+ domain-service files are ADMITTED baseline** — this framing was strongest in opus, aligned with gpt-5.4's "concentrated, not everywhere" reading.
- **`lifecycle-state-machine` as a proposed-but-retired archetype** — opus-only deep treatment. Flagged as ADR-0023 territory; gemini and gpt-5.4 left the graceful/process pattern as TERMINAL without naming it.
- **`filesystem-bound-singleton` for local storage** — gemini-only. Opus would fold into `serialized-actor`; synthesis keeps as distinct A8.
- **Bundle-coverage ledgers:** gemini's run-3 subagent-dispatches log shows all 6 bundles covered (G-BOOT, G-ASYNC, G-INFRA, G-DB, plus ingress and domain-services). opus covered all 6 at greater depth; gpt-5.4 focused on the highest-yield bundles.

## Pointers

- `../runs/opus/annotations/gitea.md` — 237 lines (deepest), G1–G18 region IDs.
- `../runs/gpt-5.4/annotations/gitea.md` — 156 lines, strong narrative on concentration framing.
- `../runs/gemini/annotations/gitea.md` — 49 lines, complete bundle enumeration at terse depth.
