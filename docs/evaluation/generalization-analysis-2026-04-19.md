# Compiler Generalization Analysis

_2026-04-19. Cross-cutting synthesis of per-target audits (see `targets/NN-*.md` for the raw per-repo data once filled in)._

## Headline finding

Monolift's current input contract holds in **1–2 of 8 dimensions** on real Go monoliths.
The test demo at `demo/monolith/` is a greenfield app *built to the compiler*; none of the
candidate evaluation targets were. The compiler's biggest assumptions — interface-annotated
services, unique implementers, constructor-by-convention, wiring-in-main, stateless
services with uniform RPC-shaped methods — are each violated by most or all targets.

## Scorecard

| Contract item                             | gitea | mattermost | caddy | listmonk | pocketbase | miniflux |
|-------------------------------------------|:-:|:-:|:-:|:-:|:-:|:-:|
| **A.** Annotations on interfaces          | ❌ | ❌ | ❌ | ❌ | ❌ | ⚠️ |
| **B.** Unique implementer per interface   | ⚠️ | ⚠️ | ❌ | ✅ | ✅ | ❌ |
| **C.** `New<InterfaceName>` constructor   | ❌ | ⚠️ | ⚠️ | ⚠️ | ✅ | ✅ |
| **D.** Wiring reconstructible from `main` | ❌ | ❌ | ❌ | ❌ | ⚠️ | ✅ |
| **E.** `(ctx, req) → (resp, err)` methods | ❌ | ⚠️ | ⚠️ | ❌ | ✅ | ⚠️ |
| **F.** Stateless services                 | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ |
| **G.** Package = service boundary         | ⚠️ | ⚠️ | ✅ | ⚠️ | ❌ | ⚠️ |
| **H.** Single main executable             | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| **Total fit**                             | 1/8 | 1/8 | 2/8 | 2/8 | 3/8 | 4/8 |

Miniflux is the closest; pocketbase's fit is misleading (methods match but SQLite-embedded
monolith is architecturally unliftable).

## What consistently breaks

### 1. Wiring doesn't live in `main` (6/6 targets)
This is the universal failure. The compiler's `resolveDependencies()` walk over main's
variable declarations (compiler.go:952) finds nothing in real codebases. Instead, wiring is
done via:
- **`init()` chains** — gitea's `routers/init.go:InitWebInstalled()` imperatively calls
  `mailer.NewContext(ctx)`, `cache.Init()`, …
- **Options builder** — mattermost's `app.NewServer(options...)`
- **Plugin registry** — caddy's blank-import `_ "modules/standard"` + `init()` calls
  `caddy.RegisterModule(instance)` keyed by string IDs
- **Lifecycle hooks** — pocketbase's `OnBootstrap`, `OnServe` fire after App construction
- **App-struct god object** — listmonk assembles an `App{…}` literal with 14 state fields
- **CLI delegation** — miniflux's `main()` just calls `cli.Parse()`

### 2. Service "interface" is rare (6/6)
Gitea, listmonk, pocketbase, and most of mattermost/miniflux expose business logic through
**concrete structs with methods**, not interfaces. Interfaces, when present, are for
*adapters* (mailer backends, OAuth providers, storage drivers) — not for carving service
boundaries. Monolift's interface-centric annotation site doesn't exist.

### 3. Multi-implementer is common (3/6 ❌, 2/6 ⚠️)
Caddy's plugin model is built on many-impls-per-interface. Miniflux has two `Provider`
impls (Google, OIDC). Gitea has multiple `Sender` impls (SMTP, Sendmail, Dummy).
Mattermost has auto-generated mocks alongside prod impls. The
`findSingleImplementer()` heuristic (compiler.go:735) is fragile.

### 4. Method shapes are heterogeneous (5/6 not strict)
Real shapes observed:
- `(c echo.Context) error` — listmonk
- `(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error` — caddy
- `(rctx request.CTX, user *model.User, opts UserCreateOptions) (*model.User, error)` — mattermost
- `(ctx, u *user_model.User, newName string, doer *user_model.User) error` — gitea
- Builder chains, package-level functions, goroutine loops reading channels
The compiler's `panic` in clientgen.go:110 would fire on virtually all of these.

### 5. Stateful services are the norm (5/6)
Every target holds meaningful in-process state: worker pools, template/link caches,
WebSocket hubs, broadcast channels, subscription brokers, connection pools, POP3 sessions,
scheduled-task tables. Monolift's stateless-only assumption (F) is the single biggest
architectural mismatch — it rules out lifting things like mattermost's WebHub or
listmonk's campaign worker entirely.

### 6. Package ≠ service (4/6 ⚠️ or ❌)
Handlers + business logic + persistence routinely share packages or cross-call each other
directly. Pocketbase is the pathological case: `core.App` has 190+ methods in one interface.
Even gitea, which has a `services/` tree, has routers calling `models/` directly,
sidestepping the service layer.

## Implications for compiler redesign

### Must-haves to support any of these targets

1. **Annotation site generalization.** Move beyond "pragma on interface decl." Support
   pragmas on:
   - function declarations (listmonk's `worker()`, miniflux's `ProcessFeedEntries`)
   - struct methods (mattermost's `(us *UserService) CreateUser`)
   - struct-type decls with an inferred "public method surface"
   Pragma-on-interface becomes the *convenient* case, not the *only* case.

2. **Call-graph-driven extraction instead of main-walk.** Replace
   `resolveDependencies()` with a `go/ssa`-based closure: starting from the annotated
   symbol, compute the transitive set of values and functions it reads/calls. This makes
   the compiler indifferent to *where* wiring happens — `init()`, hooks, Options, registries
   all contribute to the final SSA program.

3. **Method-shape adapter generation.** Replace the strict `(ctx, req) → (resp, err)`
   check with a signature classifier that generates the right serialization/dispatch
   adapter for each category: HTTP handler, gRPC-shaped, domain-object-args,
   channel-consumer, builder-chain. Each category gets its own transport template.

4. **Stateful lift.** Treat in-process state as part of the extracted deployable.
   If an annotated method reads from a package-level queue/cache/goroutine, the compiler
   must move those with it — generating a single-instance (or sticky-routed) service, not
   a stateless replica. This is the core conceptual shift away from the current
   demo-shaped assumption.

5. **Non-single-implementer handling.** Require the annotation to disambiguate
   (`//monolift:lift impl=SMTPSender`) or lift the interface *dispatch point*
   (switch local/remote per-impl, not per-interface).

### Nice-to-haves

6. **Pattern library.** Recognize common wiring idioms and extract dependency graphs from
   them: Options-builder, Provide-style DI (wire/fx), registry `Register*` calls,
   hook-based `OnFoo(func(...))`. Treat each as a first-class wiring backend.

7. **State-class inference.** Classify state into {ephemeral per-request, process-local
   cache, durable-via-external-store, replica-coordinated} to decide whether a lift
   produces a stateless replica, a singleton, or requires session affinity.

8. **Handler transport preservation.** If the annotated symbol is an HTTP/echo handler,
   the lifted service should *still* be an HTTP handler — just in a separate process
   behind the same route. No RPC round-trip through JSON/gRPC/HTTP re-encoding.

## Target ordering for iteration

Based on architectural fit and LOC:

| Priority | Target | Why |
|----------|--------|-----|
| **1** | **miniflux** | Highest contract fit (4/8); clean cli→daemon bootstrap; feed-fetcher is a plausible first lift once multi-impl and stateful-worker generalizations exist |
| **2** | **listmonk** | Small (92 files); campaign worker is the natural target; forces us to solve stateful-worker + channel-consumer method shape |
| **3** | **caddy** | Plugin model forces "call-graph extraction" generalization; TLS cert issuance is the cleanest lift candidate |
| **4** | **gitea** | Larger, stresses init()-chain wiring; mailer service is the most isolated candidate |
| **5** | **mattermost** | Largest; useful only once the first 4 are handled; UserService is the best entry |
| **6** | **pocketbase** | Architecturally unliftable without a rewrite; useful as a *negative* case — document what Monolift can't do |

## Recommended next step

Pick **miniflux's feed fetcher** or **listmonk's campaign worker** as the first
"second-target" experiment and use it to drive compiler generalization. Both force
three of the five must-haves (annotation-on-function, stateful lift, non-stdlib method
shape) without requiring the plugin-system or hook-based wiring work. The design choices
made for that first lift will shape how the harder targets can eventually be supported.

The alternative framing: **don't try to make the current compiler work on these.**
Treat this audit as input to a compiler v2 design pass — the current codebase is
evidence that the contract is over-specialized to the demo. Fix the contract first,
then implement.
