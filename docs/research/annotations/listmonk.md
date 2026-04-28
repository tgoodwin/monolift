# listmonk — cross-run composite annotation

**Corpus pin:** `evaluation/MANIFEST.yaml` as of 2026-04-19. 92 Go files (smallest target).
No committed golden report; all three runs walked source directly.

## Cross-run summary

Listmonk is the cleanest positive case in the corpus — all three runs converged on it as the strongest AUTO surface per-target. Its distribution surface concentrates in three loci: a campaign manager that looks like a worker pool fed by `Manager.campMsgQ`/`msgQ` channels, a bounce subsystem that is a periodic scheduler feeding a single-worker queue, and an in-process pub/sub (`internal/events`) with fanout over a mutex-protected map of subscriber channels. Plus a TTL cache (`Auth.apiUsers`, `tmptokens`) that routes cleanly to `ttl-cache`.

Gemini framed listmonk's `App` as a "God Object" (large stateful struct triggering terminal refusal) and noted it can be decomposed via the Worker Pool and Event-Bus archetypes — directionally aligned with opus's finer-grained split.

## Triage convergences (all three runs agree or two-of-three)

| region | triage | archetype | convergence |
|---|---|---|---|
| `Manager.scanCampaigns` (manager.go:422-458) — **L1** | AUTO | `periodic-invocation` | all 3 |
| `Manager.worker` + `campMsgQ` (manager.go:462-559) — **L2** | AUTO | `bounded-worker-pool` | all 3 |
| `runMailboxScanner` (bounce.go:135-143) — **L3** | AUTO | `periodic-invocation` | opus + gpt-5.4; gemini high-level |
| `Events.Publish` (events.go:41-76) — **L4** | AUTO | `fanout-publisher` | opus + gemini; gpt-5.4 merges into connection-hub-buffer |
| `tmptokens` (tmptokens.go:29-42) — **L7** | AUTO | `ttl-cache` / `serialized-singleton-owner` | opus + gpt-5.4 (labels differ; same region) |

## Divergences and single-run findings

- `Manager.pipes` + `links` (manager.go:72-81) — **L5**: opus triaged AUTO as `keyed-partitioned-state`; gpt-5.4 flagged as SUGGEST with single-owner framing; gemini did not surface as distinct. Synthesis keeps AUTO with composite label — L5 is a hard ambiguity (DB owns canonical state; in-process map is cache).
- `Auth.apiUsers` + prune loop (auth.go:62-110) — **L6**: opus triaged as `ttl-cache`; gpt-5.4 as `serialized-singleton-owner`. Same region, different labels. Synthesis view: `ttl-cache` is the more-constrained archetype (matches emission gate — managed-cache adapter).
- App root as "God Object": gemini-only finding. Useful strategic framing that opus and gpt-5.4 handled implicitly by decomposing into finer archetypes rather than naming the composite.

## Pointers (per-run depth)

- `../runs/opus/annotations/listmonk.md` — 92 lines, region-level citations (L1–L7), target synthesis.
- `../runs/gpt-5.4/annotations/listmonk.md` — 127 lines, strongest narrative case for listmonk as the positive exemplar; 5 AUTO regions listed.
- `../runs/gemini/annotations/listmonk.md` — 49 lines, broader framing with "God Object" lens, 3 AUTO regions.
