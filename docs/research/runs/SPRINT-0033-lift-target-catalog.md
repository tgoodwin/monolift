# SPRINT-0033 EntryPath Lift-Target Catalog

Date: 2026-04-28

## Purpose

Catalog of candidate **lift regions** across the four corpus projects (Mattermost, Miniflux, Gitea, PocketBase), inventoried so they can be used as evaluation targets for a generalizable EntryPath activation-boundary recovery algorithm. The catalog is organized to surface structural diversity of activation shapes — not to choose distribution cut points and not to commit to algorithm design.

Each candidate carries:

- **Region root** (file:line) — the cohesive behavior we hypothetically want to lift.
- **Hypothesized activation boundary** (file:line) — where in our judgment the semantic handoff sits between broad bootstrap/dispatch machinery and the specific behavior chain.
- **Bootstrap path sketch** — a rough chain from `main` to the boundary, recorded so we can score how well an algorithm recovers it.
- **Activation family** — the structural shape, deliberately described abstractly rather than by framework name.

This catalog is downstream of `SPRINT-0033-entrypath-candidate-viability.md` and supersedes its 5-row candidate set as the evaluation target list. Known candidates are preserved as `K1`–`K5` for continuity.

## Master Table

Legend — Status: `K` = known (carried over from viability memo); `N` = newly proposed by sub-agent inventory. Difficulty: heuristic indication of how far it is from the current bridge algorithm's strengths (HTTP/function-value): `easy`, `mid`, `hard`.

| ID | Project | Short name | Region root | Hypothesized boundary | Activation shape | Difficulty |
|---|---|---|---|---|---|---|
| K1 | Mattermost | WebSocket hub | `(*Hub).Start` @ `channels/app/platform/web_hub.go:526` | `(*API).InitWebSocket` @ `channels/api4/websocket.go:52` | HTTP route → WebSocket upgrade | easy |
| K2 | Miniflux | Fever handler | `(*feverHandler).serve` @ `internal/fever/handler.go:31` | `internal/http/server.newRouter` registration | Object-method handler under sub-mux | easy |
| K3 | Miniflux | Feed refresh | `RefreshFeed` @ `internal/reader/handler/handler.go:207` | multiple (HTTP, worker, CLI, scheduler) | One region, multiple activation paths | mid |
| K4 | Gitea | SSE eventsource | `(*Manager).Register` @ `modules/eventsource/manager.go:33` | `/user/events` route registration (not promoted today) | Fluent router, non-`http.Handler` typed | hard |
| K5 | PocketBase | Autobackup | `(*BaseApp).CreateBackup` @ `core/base_backup.go:44` | `OnBootstrap().BindFunc` + `Cron.Add(...)` (not promoted today) | Lifecycle hook + cron registration | hard |
| N-MM-1 | Mattermost | `/echo` slash command | `(*EchoProvider).DoCommand` @ `channels/app/slashcommands/command_echo.go:45` | `(*App).tryExecuteBuiltInCommand` @ `channels/app/command.go:377` | Init() registry + string-keyed dispatch | mid |
| N-MM-2 | Mattermost | Incoming webhook ingest | `(*App).HandleIncomingWebhook` (called from `web/webhook.go:96`) | `APIHandlerTrustRequester` factory @ `web/handlers.go:544` | HTTP route on a sibling router with different wrapper factory | mid |
| N-MM-3 | Mattermost | ExpiryNotify scheduled job | `notifySessionsExpired` (worker callback) @ `jobs/expirynotify/worker.go:21` | `(*JobServer).RegisterJobType` @ `jobs/server.go:68` | Dual registry (workers + schedulers) keyed by JobType + leader gate | hard |
| N-MM-4 | Mattermost | Token cleanup recurring task | `doTokenCleanup` @ `channels/app/server.go:1286` | `model.CreateRecurringTask` `go func()` @ `public/model/scheduled_task.go:46` | Function value in struct field invoked by ticker goroutine | hard |
| N-MM-5 | Mattermost | Cluster publish handler | `(*PlatformService).ClusterPublishHandler` @ `app/platform/cluster_handlers.go:32` | `RegisterClusterMessageHandler` @ `app/platform/cluster_handlers.go:28` | Pub-sub callback registry, dispatch via interface | hard |
| N-MM-6 | Mattermost | Shared-channel sync receiver | `(*Service).onReceiveSyncMessage` @ `services/sharedchannel/sync_recv.go:27` | `rcs.AddTopicListener(TopicSync, ...)` @ `services/sharedchannel/service.go:142` | Topic-keyed RPC listener, cross-package, license/config-gated | hard |
| N-MM-7 | Mattermost | Plugin `OnActivate` | `sup.Hooks().OnActivate()` @ `public/plugin/environment.go:298` | `(*Environment).startPluginServer` @ `:283` | Cross-process RPC + hashicorp/go-plugin handshake | hard |
| N-MF-1 | Miniflux | Feed batch scheduler tick | `feedScheduler` @ `internal/cli/scheduler.go:33` | `go feedScheduler(...)` launch @ `internal/cli/scheduler.go:18` | Periodic ticker goroutine | mid |
| N-MF-2 | Miniflux | Worker pool consumer | `for job := range c` @ `internal/worker/worker.go:31` | channel receive @ `worker.go:31`; producers at `pool.go:20` | Channel consumer with multi-producer fan-in | mid |
| N-MF-3 | Miniflux | Google Reader `editTagHandler` | `(*greaderHandler).editTagHandler` @ `internal/googlereader/handler.go` (registered :51) | `withApiKeyAuth(h.editTagHandler)` @ `handler.go:51` | Per-route closure wrapper on nested mux | mid |
| N-MF-4 | Miniflux | API saveEntry + integration fork | `(*handler).saveEntryHandler` @ `internal/api/entry_handlers.go` | triple wrap `withCORSHeaders(validateAPIKeyAuth(validateBasicAuth(...)))` @ `internal/api/api.go:76` | Triple-stacked middleware + inline `go integration.SendEntry` fork | mid |
| N-MF-5 | Miniflux | Integration fan-out region | `integration.SendEntry` @ `internal/integration/integration.go:41` | 4 distinct call sites (UI, Fever, GReader, API) | Many-to-one fan-in + flag-gated runtime fan-out | hard |
| N-MF-6 | Miniflux | Cleanup tasks (dual activation) | `runCleanupTasks` @ `internal/cli/cleanup_tasks.go:16` | flag branch @ `cli/cli.go:243` AND `cleanupScheduler` ticker @ `cli/scheduler.go:54` | Same region reachable via flag-switch and scheduler | mid |
| N-MF-7 | Miniflux | Signal-driven shutdown | shutdown block @ `internal/cli/daemon.go:77` | `signal.Notify(stop, ...)` @ `daemon.go:27` (boundary ambiguous) | OS-signal → channel receive, no caller edge | hard |
| N-GT-1 | Gitea | Smart-HTTP `git-upload-pack` | `repo.ServiceUploadPack` @ `routers/web/repo/githttp.go:432` | `m.Methods("POST,OPTIONS", "/git-upload-pack", ...)` @ `routers/web/githttp.go:13` | Fluent router via helper-fn vararg middleware | hard |
| N-GT-2 | Gitea | LFS BatchHandler (multi-mount) | `lfs.BatchHandler` @ `services/lfs/server.go:187` | `m.Post("/objects/batch", ...)` @ `routers/common/lfs.go:18` (mounted by both web + private) | Shared registration helper used by multiple parent routers | hard |
| N-GT-3 | Gitea | Webhook delivery worker | `webhook.handler` @ `services/webhook/webhook.go:78` | `CreateUniqueQueue("webhook_sender", handler)` + `go RunWithCancel` @ `services/webhook/deliver.go:330,334` | Queue (named handler symbol) + separate goroutine kick | hard |
| N-GT-4 | Gitea | Cron `update_mirrors` | closure @ `services/cron/tasks_basic.go:39` | `RegisterTaskFatal(...)` @ `tasks_basic.go:31` | Closure → `Task.fun` field → gocron scheduler (triple indirection) | hard |
| N-GT-5 | Gitea | Mirror sync queue handler | `queueHandler` @ `services/mirror/queue.go:31` | `CreateUniqueQueue("mirror", queueHandler)` @ `:43` | Queue with discriminated-union payload (handler internally branches) | hard |
| N-GT-6 | Gitea | SSH session dispatcher | `sessionHandler` @ `modules/ssh/ssh.go:102` | `ssh.Server{ ..., Handler: sessionHandler, ... }` struct literal @ `:341` | Function value stored in 3rd-party server struct field | hard |
| N-GT-7 | Gitea | ConnectRPC `RunnerService.FetchTask` | `(*Service).FetchTask` @ `routers/api/actions/runner/runner.go:38` | two-stage: `NewRunnerServiceHandler` @ `:29` then `m.Post(path+"*", handler.ServeHTTP)` @ `actions/actions.go:20` | Codegen-driven RPC + fluent router mount | hard |
| N-GT-8 | Gitea | `gitea serv` CLI subcommand | `runServ` @ `cmd/serv.go:135` | `Action: runServ` field @ `cmd/serv.go:45`, dispatched via `app.Run` | urfave/cli v3 string-keyed subcommand dispatch | mid |
| N-PB-1 | PocketBase | Realtime broadcast on create | `realtimeBroadcastRecord` @ `apis/realtime.go:487` | `OnModelAfterCreateSuccess().Bind(&hook.Handler{...})` in `bindRealtimeEvents` @ `apis/realtime.go:270` | Hook `Bind` (typed handler) + downstream broker fan-out | hard |
| N-PB-2 | PocketBase | Realtime SSE connect | `realtimeConnect` @ `apis/realtime.go:40` | `sub.GET("", realtimeConnect)` @ `apis/realtime.go:34` | Echo-typed handler + side-effect broker registration | mid |
| N-PB-3 | PocketBase | Password-reset email send | `mails.SendRecordPasswordReset` @ `mails/record.go:128` | `sub.POST("/request-password-reset", ...)` @ `apis/record_auth.go:46` + `routine.FireAndForget` @ `apis/record_auth_password_reset_request.go:56` | Route → typed event hook → goroutine → cross-package mailer | hard |
| N-PB-4 | PocketBase | Migration runner Up | `(*MigrationsRunner).Up` @ `core/migrations_runner.go:122` | import side-effect `_ "...pocketbase/migrations"` @ `pocketbase.go:21` + `RunAllMigrations` call @ `apis/serve.go:67` | `init()`-time registry consumed by runner loop | hard |
| N-PB-5 | PocketBase | OTP cleanup cron | closure @ `core/otp_model.go:122` | `app.Cron().Add("__pbOTPCleanup__", ...)` @ `core/otp_model.go:122`; started via `OnServe` hook @ `core/base.go:1349` | Minimal `Cron.Add` (autobackup-shape variant, simpler) | mid |
| N-PB-6 | PocketBase | OAuth2 callback exchange | `recordAuthWithOAuth2` @ `apis/record_auth_with_oauth2.go:30` | `sub.POST("/auth-with-oauth2", ...)` @ `record_auth.go:35` + broker rendezvous @ `record_auth_with_oauth2_redirect.go:51` | Two cooperating routes + non-router rendezvous + re-entrant internal request | hard |
| N-PB-7 | PocketBase | Superuser CLI subcommand | `RunE` closure @ `cmd/superuser.go:37` | `pb.RootCmd.AddCommand(cmd.NewSuperuserCommand(pb))` @ `pocketbase.go:168` | Cobra subcommand tree | mid |

34 candidates total (5 known + 29 new).

## Coverage by activation shape

| Activation shape | Candidates |
|---|---|
| HTTP route via fluent router (non-`http.Handler` typed handler) | K4, N-GT-1, N-GT-2, N-PB-2 |
| HTTP route via middleware-wrapper chain | K1, N-MM-2, N-MF-3, N-MF-4 |
| Object-method handler (method-value into a registry) | K2, N-MF-3 |
| WebSocket / SSE / streaming endpoint | K1, K4, N-PB-2 |
| Background goroutine launched from bootstrap | N-MM-4, N-MF-1, N-PB-3 |
| Scheduled / cron / periodic job (registered task) | K5, N-MM-3, N-MM-4, N-GT-4, N-PB-5 |
| Queue / channel-driven worker consumer | N-MF-2, N-GT-3, N-GT-5 |
| Lifecycle hook / `BindFunc` / `Bind` callback | K5, N-MM-7, N-PB-1, N-PB-3 |
| Pub-sub / event subscription / topic listener | N-MM-5, N-MM-6 |
| CLI subcommand handler | N-MF-6, N-GT-8, N-PB-7 |
| Plugin entry point / cross-process RPC | N-MM-7, N-GT-7 |
| Slash command / interactive message | N-MM-1 |
| gRPC-style codegen-driven RPC | N-GT-7 |
| Signal handler / OS-signal-driven activation | N-MF-7 |
| Function value stored in struct field, invoked later | N-MM-4, N-GT-6, N-GT-7, N-PB-1 |
| Init()-time global registry, consumed at runtime | N-MM-1, N-PB-4 |
| Multi-path activation onto a single region (one region, multiple paths) | K3, N-MF-5, N-MF-6 |
| Multi-mount: same handler reachable through multiple routers | N-GT-2 |
| Cross-package boundary chain (registration site ≠ dispatch site) | N-MM-5, N-MM-6, N-GT-7, N-PB-1 |

### Gaps in coverage

- **gRPC service methods (true `.proto`-generated)** — only N-GT-7 (ConnectRPC) approximates; no genuine `grpc-go` server registration in any of the four projects. Probably out of scope for the corpus.
- **fsnotify / file-watch callback** — not present in any project; Mattermost config reload uses `AddConfigListener`, which falls under the pub-sub family (N-MM-5).
- **Mid-request goroutine fork** is only lightly represented (N-MF-4 forks `go integration.SendEntry`, N-PB-3 forks via `routine.FireAndForget`). If we want stronger coverage of "request handler spawns a deferred activation chain," we should add fixtures.
- **Producer side of a queue** — N-GT-3 and N-MF-2 capture the consumer side; the *producer* registration is not currently surfaced as a region (Gitea's webhook notifier is the producer for N-GT-3 but is itself activation-shape pub-sub, registered via `notify.RegisterNotifier`).

## Recommended initial evaluation subset

For a first algorithm-comparison pass that exercises diversity without running the full 34, we suggest 8 targets that span the matrix and include both "known viable" and "known hard" cases:

1. **K1 Mattermost WS hub** — regression baseline (current algorithm passes).
2. **K2 Miniflux Fever handler** — second baseline (current algorithm passes; non-Mattermost).
3. **N-PB-5 PocketBase OTP cleanup cron** — *minimal* `Cron.Add` shape, easier sibling of K5; shows whether the cron-family fix lands.
4. **N-GT-4 Gitea `update_mirrors` cron** — triple-indirected closure into a `Task.fun` struct field; harder cron variant.
5. **N-GT-3 Gitea webhook delivery queue** — queue + named handler symbol + separate goroutine kick; tests two-step activation.
6. **N-MM-1 Mattermost `/echo` slash command** — `init()` registry + string-keyed dispatch through `tryExecuteBuiltInCommand`.
7. **N-GT-6 Gitea SSH session dispatcher** — function value stored in 3rd-party struct literal; tests struct-field-as-registration.
8. **N-MF-5 Miniflux integration fan-out** — many-to-one fan-in (4 caller chains). Tests whether the algorithm enumerates all activation paths or collapses to one.

This subset covers: HTTP+middleware, cron (easy + hard variants), queue worker, init-registry slash command, struct-field handler, multi-path fan-in. It deliberately omits hardest cases (cross-process plugin RPC, license-gated topic listener) until the easier shapes are working.

## Notes & caveats

- **Miniflux router correction.** The brief and prior memos state Miniflux uses gorilla/mux. The current source uses Go 1.22 `net/http.ServeMux` with `"METHOD /pattern"` syntax (see `internal/api/api.go:25`, `internal/googlereader/handler.go:49`). The "fluent-router" candidate categorization in earlier docs should be revised: in this Miniflux, all HTTP routes are stdlib mux. Update SPRINT-0033 brief and any prose that says "gorilla/mux" → "Go 1.22 ServeMux."
- **Mattermost N-MM-5 dispatch site unverified.** Registration site is in `channels/app/platform/cluster_handlers.go`; the actual call site of `ClusterPublishHandler` is behind `einterfaces.ClusterInterface` (likely in `server/enterprise/`), which the inventory did not open. Marked as a known limitation rather than an evaluation blocker — the registration-side activation path is still useful evidence.
- **Mattermost N-MM-7 (plugin `OnActivate`) is intentionally cross-process.** The boundary is observable up to the RPC stub (`sup.Hooks().OnActivate()`); the actual region root lives in a separate plugin binary. Useful as the explicit "limit case" — any algorithm that requires single-binary call edges should report this as a partial-gap.
- **Gitea is structurally the richest corpus.** 8 candidates not because we forced it but because the project genuinely combines a custom fluent router, two queue framings, a cron registry with closure-into-struct-field, a 3rd-party SSH server, ConnectRPC codegen, and urfave/cli — almost every activation idiom the brief asked about, in a single repo.
- **PocketBase N-PB-1 uses `Bind(&hook.Handler{...})` (typed) rather than `BindFunc`.** The autobackup case (K5) uses `BindFunc`. Both should be recognizable by the same boundary predicate; if our predicate keys on the method name `BindFunc`, it will miss N-PB-1. Worth noting for predicate design.
- **Path-format consistency.** All file paths above are relative to each project's repo root under `evaluation/<project>/`. When configuring probes, prefix accordingly.
- **Source freshness.** Sub-agents read the trees as they exist on disk on 2026-04-28. The viability memo's existing K-row line numbers (e.g. `(*Hub).Start` at `web_hub.go:526`) were measured against the same tree (or close). If `evaluation/mattermost` is a different commit than `.tmp/monolift-sprint-0013/mattermost-src` (used by some earlier probes), file:line numbers may differ; verify before pinning a probe oracle.

## Next steps

1. **Fold this catalog into the brief.** Update `docs/sprints/drafts/SPRINT-0033-generalizable-entrypath-brief.md` to point at this catalog as the evaluation target list, and correct the Miniflux router language.
2. **Score each candidate against the existing bridge algorithm.** Pick the 8-target subset above and run probes; record what the current algorithm recovers vs. the hypothesized boundary in this catalog. The score table becomes the gap analysis input for the algorithm-direction memo.
3. **Decide which structural predicates are worth fixturing first.** From the score table, identify the smallest set of new boundary-evidence kinds that closes the largest number of gaps. Cron variants (K5 + N-GT-4 + N-PB-5) and struct-field-handler (N-GT-6, N-MM-4, N-PB-1) look like the highest-leverage families to fixture first.
