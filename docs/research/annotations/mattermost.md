# mattermost — cross-run composite annotation

**Corpus pin:** `evaluation/MANIFEST.yaml` as of 2026-04-19. 2153 Go files. Mandatory subagent delegation per sprint plan.

## Cross-run summary

Mattermost is the **richest single source of connection-hub and worker-runtime evidence** in the corpus. Its websocket hub (`server/channels/app/web_hub.go`) holds 14 channel fields, a `hubConnectionIndex` map keyed per-user, a broadcast path with send-queue-per-connection, and a reliable-queue replay buffer. This is the archetype-composite case gpt-5.4 names `connection-hub-buffer` — and also the case opus expresses as `keyed-partitioned-state` + `fanout-publisher` + `session-affinity-state` simultaneously. Both framings are valid; they are the reason ADR-0022 (composite-archetype-regions) is in the follow-ups.

Beyond the hub, mattermost contains a conventional `bounded-worker-pool` in `PushNotificationsHub`, session and status caches that are `ttl-cache` candidates, and a periodic `email_batching` loop that becomes `periodic-invocation` after lift.

gpt-5.4 emphasized mattermost as the strongest `connection-hub-buffer` evidence; opus enumerated the hub as a composite of narrower archetypes; gemini identified the websocket hub as a "Sharded Singleton Actor" which is directionally consistent.

## Triage convergences

| region | triage | archetype | convergence |
|---|---|---|---|
| `Hub.hubConnectionIndex` (web_hub.go:77-120) — **MM1** | AUTO | `keyed-partitioned-state` + `fanout-publisher` + `session-affinity-state` composite = `connection-hub-buffer` | all 3 (different labels, same region) |
| `WebConn` session state (web_conn.go:88-149) — **MM2** | AUTO | `session-affinity-state` | opus + gpt-5.4 + gemini (as wsapi) |
| session / status cache (session.go:44-97) — **MM4, MM5** | AUTO | `ttl-cache` | opus |
| `PushNotificationsHub` (notification_push.go:44-52) — **MM6** | AUTO | `bounded-worker-pool` | opus + gpt-5.4 |
| cluster `Publish` (cluster.go:189-234) — **MM7** | ADMITTED | validates `fanout-publisher` shape | opus |
| `email_batching` (email_batching.go:71-159) — **MM8** | AUTO (post-transform) | `periodic-invocation` | opus + gpt-5.4 |
| cluster-leader-listeners composite (cluster.go:164-187) — **MM11** | AUTO | `serialized-actor` | opus |
| `searchlayer indexPost` — **MM-SEARCH** | AUTO | `ephemeral-worker` (gemini) / fissioned → `bounded-worker-pool` (synthesis) | divergence |

## Divergences and single-run findings

- **`connection-hub-buffer` as composite vs. three narrow archetypes:** gpt-5.4's composite framing is the strongest argument for ADR-0022. Opus expressed the same region via three narrow archetype labels. Neither is wrong — they're different resolution levels. Synthesis keeps both: the catalog has the three narrow archetypes; the composite has a named entry (`connection-hub-buffer`) as a pattern to report on when all three co-occur.
- **`searchlayer indexPost` as ephemeral-worker vs. fissioned:** gemini kept `ephemeral-worker` as an archetype; opus retired it (fissioned to `session-affinity-state` lifecycled OR TERMINAL fire-and-forget). For mattermost's search indexing, the synthesis view routes it to `bounded-worker-pool` — it's lifecycled job execution, not anonymous spawn.
- **Session-across-cluster** — opus flagged as out-of-v1-scope (C7 open question). Mattermost's cluster model with a single user across connections stresses `session-affinity-state`'s scope and becomes a candidate for a future `user-affinity-state` archetype. gpt-5.4 flagged similar concern under "subscriber semantics" for `connection-hub-buffer`.

## Pointers

- `../runs/opus/annotations/mattermost.md` — 163 lines, MM1–MM11 region IDs.
- `../runs/gpt-5.4/annotations/mattermost.md` — 174 lines (longest per-target narrative), strongest `connection-hub-buffer` case.
- `../runs/gemini/annotations/mattermost.md` — 49 lines, WebHub as Sharded Singleton Actor, JobServer as Worker Pool.
