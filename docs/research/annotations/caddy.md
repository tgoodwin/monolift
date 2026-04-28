# caddy — cross-run composite annotation

**Corpus pin:** `evaluation/MANIFEST.yaml` as of 2026-04-19. 306 Go files.
Committed golden: `test/e2e/targets/caddy/golden/report.json`; also reference `pkg/compiler/extract_integration_test.go:12` (Caddy reverseproxy Handler).

## Cross-run summary

Caddy is the **negative control** as much as the positive surface. Its reverseproxy hot path (the pattern that drove `MLV2_CHANNEL_BOUNDARY`) is correctly classified TERMINAL by all three runs — the existence of channels and sync primitives does not automatically imply a transform when the sync primitives encode wire-protocol-level behavior (HTTP/2 multiplexing, hijacked connections). Beyond the hot path, caddy contains a rich archetype surface across periodic maintenance (STEK rotation, storage cleanup), TTL caches (BasicAuth password cache), and keyed state (the connections map).

gpt-5.4 emphasized this negative-control framing most strongly; opus enumerated finer-grained regions; gemini framed the Handler itself as a Singleton Actor candidate (aligned with opus's `serialized-actor` for adjacent state but not the hot-path codepath that stays TERMINAL).

## Triage convergences

| region | triage | archetype | convergence |
|---|---|---|---|
| reverseproxy Handler hot path (post-SPRINT-0011 refusal) | TERMINAL | — (negative control) | all 3 |
| `stayUpdated` (sessiontickets.go:114-148) — **C1** | AUTO | `periodic-invocation` | opus + gpt-5.4 |
| `keepStorageClean` (tls.go:1050-1072) — **C2** | AUTO | `periodic-invocation` | opus + gpt-5.4 |
| `Handler.connections` + `connectionsMu` (streaming.go:302-324) — **C5** | AUTO | `serialized-actor` + `keyed-partitioned-state` composite | opus |
| `handleUpgradeResponse` per-request hijack (streaming.go:147-159) — **C6** | AUTO | `session-affinity-state` | opus |
| `HTTPBasicAuth.Cache` + `mu *sync.RWMutex` (basicauth.go:105-110) — **C7** | AUTO | `ttl-cache` (opus) / `serialized-actor` (alternative) | opus |
| filestorage subsystem — **C-FS** | AUTO | `filesystem-bound-singleton` | gemini |

## Divergences and single-run findings

- **`filesystem-bound-singleton` in filestorage** — gemini-only. Opus folded filesystem-bound state into `serialized-actor`; gpt-5.4 did not enumerate. The synthesis keeps this as A8 in the catalog because the transform (object-store adapter) is distinct from other archetypes' transforms.
- **C5's composite nature** — opus split into `serialized-actor` + `keyed-partitioned-state`; gpt-5.4 would compress into `serialized-singleton-owner`. Synthesis note: when a region fits multiple archetypes simultaneously, ADR-0022 composite-archetype-regions will specify precedence.
- **Gemini's Caddy triage was the narrowest** (30 lines, 2 AUTO). This reflects gemini's terser per-target depth in run-3 rather than a substantive disagreement with the other runs' findings.

## Pointers

- `../runs/opus/annotations/caddy.md` — 103 lines, C1–C11 region IDs.
- `../runs/gpt-5.4/annotations/caddy.md` — 56 lines, strongest negative-control framing.
- `../runs/gemini/annotations/caddy.md` — 30 lines, surfaces filesystem-bound-singleton.
