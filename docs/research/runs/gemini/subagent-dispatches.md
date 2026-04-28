# Subagent Dispatches — Monolift SPRINT-0013

| Dispatch ID | Target | Bundle | Directories | Status | Notes |
|---|---|---|---|---|---|
| DISPATCH-01 | Gitea | boot/lifecycle | `cmd/`, `routers/install`, etc. | SUCCESS | Identified web server and CLI archetypes. |
| DISPATCH-02 | Gitea | ingress | `routers/api`, `routers/web`, etc. | SUCCESS | Exhaustive walk of API handlers (Stateless). |
| DISPATCH-03 | Gitea | domain services | `services/auth`, `services/user`, etc. | SUCCESS | Identified signin and mirror archetypes. |
| DISPATCH-04 | Gitea | background/async | `services/mailer`, `modules/queue`, etc. | SUCCESS | Strong Worker Pool / Queue Consumer evidence. |
| DISPATCH-05 | Gitea | infra/runtime | `modules/cache`, `modules/storage`, etc. | SUCCESS | Identified Singleton Actor for storage/cache. |
| DISPATCH-06 | Gitea | persistence | `models/` | SUCCESS | Mostly Terminal; identified AppState singleton. |
| DISPATCH-07 | Mattermost | ingress | `server/channels/api4`, etc. | SUCCESS | api4 (Stateless), wsapi (Session-scoped). |
| DISPATCH-08 | Mattermost | app/service logic | `server/channels/app` | SUCCESS | CreatePost (Stateless), dndTask (Singleton). |
| DISPATCH-09 | Mattermost | long-lived / fanout | WebSocket Hubs | SUCCESS | WebHub (Sharded Singleton Actor). |
| DISPATCH-10 | Mattermost | jobs/workers | `server/channels/jobs` | SUCCESS | JobServer (Worker Pool). |
| DISPATCH-11 | Mattermost | persistence/search | `server/channels/store`, etc. | SUCCESS | searchlayer (Ephemeral Worker). |
| DISPATCH-12 | Mattermost | platform/bootstrap | `server/platform`, etc. | SUCCESS | Config Store (Singleton Actor). |
