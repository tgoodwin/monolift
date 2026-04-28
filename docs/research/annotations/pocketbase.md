# pocketbase — cross-run composite annotation

**Corpus pin:** `evaluation/MANIFEST.yaml` as of 2026-04-19. 445 Go files.
Committed golden: `test/e2e/targets/pocketbase/golden/report.json`; `TestExtractPocketBaseRefusesForEmbeddedDBAndClosureSize` (extract_integration_test.go:78) asserts the known terminal refusal for the app root.

## Cross-run summary

Pocketbase is the **"God Object" exemplar** — the `core.App` root triggers `MLV2_EMBEDDED_DB_APP_ROOT` terminal refusal, correctly, because the embedded SQLite handle is un-externalizable at the root granularity. But **narrower roots within pocketbase are archetype-clean**: the pub/sub `Broker`, the `tools/store.Store[K,T]`, the cron subsystem, the `Hook[T]` pattern — all fit v1 archetypes. This reinforces a key research finding: **terminal at the top of a root tree does not mean terminal at narrower roots**.

gpt-5.4 emphasized root selection as a distinct research concern and proposed ADR work on root-narrowing tooling. Gemini's "God Object" framing names the phenomenon succinctly.

## Triage convergences

| region | triage | archetype | convergence |
|---|---|---|---|
| `core.App` root (embedded SQLite) | TERMINAL | — (embedded durable) | all 3 |
| `Hook[T]` (hook.go:55-57) — **P1** | AUTO | `serialized-actor` | opus + gemini (as event-bus) |
| `Cron` (cron.go:176-206) — **P2** | AUTO | `periodic-invocation` | opus + gpt-5.4 |
| `tools/store.Store[K,T]` (store.go:12-40) — **P3** | AUTO | `keyed-partitioned-state` + `ttl-cache` (when entries carry expiry) | opus |
| `Broker` (broker.go:11-65) — **P4** | AUTO | `fanout-publisher` / `connection-hub-buffer` | opus + gpt-5.4 (different lens) |
| `BatchHandler` (batch_handler.go:54-88) — **P5** | AUTO | `serialized-actor` | opus |

## Divergences and single-run findings

- **P4 lens:** opus labeled `fanout-publisher`; gpt-5.4 labeled `connection-hub-buffer` with emphasis on routing-key + register/unregister + replay semantics. Both lenses are valid for P4 — gpt-5.4's is more specific, opus's is more general. ADR-0022 territory.
- **P6 (JS VM pool)** + **P9 (S3 uploader)** — opus flagged these as SUGGEST with fallback-spawn-on-full as the missing evidence for AUTO; the proposed `bounded-pool-invariant` signal would move them to AUTO. gpt-5.4 and gemini did not enumerate at this granularity.
- **Root-narrowing as a meta-finding** — gpt-5.4 surfaced this most explicitly: the app root is too coarse, but narrower roots (cron, broker, store) have clean AUTO surfaces. This is a tooling follow-up (Bucket D) rather than an archetype, but it's a pocketbase-driven research insight.

## Pointers

- `../runs/opus/annotations/pocketbase.md` — 106 lines, P1–P9 region IDs.
- `../runs/gpt-5.4/annotations/pocketbase.md` — 91 lines, root-narrowing narrative.
- `../runs/gemini/annotations/pocketbase.md` — 30 lines, "God Object" framing.
