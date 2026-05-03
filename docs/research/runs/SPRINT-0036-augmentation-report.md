# SPRINT-0036 Augmentation Report

## Summary

SPRINT-0036 added augmentation hooks, struct-field store/read indexing, framework predicates, goroutine-launch edges, and partial-path reporting. The RTA-only baseline was reproduced at 49/72 using `GOTOOLCHAIN=go1.26.2+auto`.

Final Tier 1 reachability stayed flat at 49/72. The new predicates do create the intended first-hop command-dispatch edges, for example Caddy reaches `cmdRun` through `(*cobra.Command).execute`, but the full missed traces still hit later unsupported or path-shape gaps.

## Phase Progression

| Phase | Reachable | Rate | Mean T2 Exact | Mean T2 Fuzzy |
|---|---:|---:|---:|---:|
| SPRINT-0035 baseline | 49/72 | 68.1% | 0.173 | 0.199 |
| Phase 1 struct-field | 49/72 | 68.1% | 0.173 | 0.199 |
| Phase 2 predicates | 49/72 | 68.1% | 0.173 | 0.199 |
| Phase 3 goroutine | 49/72 | 68.1% | 0.173 | 0.199 |
| Final | 49/72 | 68.1% | 0.173 | 0.199 |

## Final Per Project

| Project | Traces | Reachable | Rate | Mean T2 Exact | Mean T2 Fuzzy | Mean T3 |
|---|---:|---:|---:|---:|---:|---:|
| caddy | 6 | 0 | 0.0% | 0.000 | 0.000 | 0.000 |
| gitea | 18 | 16 | 88.9% | 0.133 | 0.159 | 0.000 |
| listmonk | 10 | 10 | 100.0% | 0.565 | 0.576 | 0.000 |
| mattermost | 15 | 0 | 0.0% | 0.000 | 0.000 | 0.000 |
| miniflux | 12 | 12 | 100.0% | 0.356 | 0.451 | 0.000 |
| pocketbase | 11 | 11 | 100.0% | 0.015 | 0.028 | 0.000 |

## Original Struct-Field Blockers

None of the 22 SPRINT-0035 `StructFieldFuncValue`-blocked full trace targets became reachable. Several first blockers moved to later non-struct-field patterns in the final scoring, but partial-path walking still often reports the trace-level direct struct-field step because the ground-truth trace skips framework-internal dispatch frames such as `ExecuteC -> execute`.

| Trace | Final status | First blocker | Partial gap |
|---|---|---|---|
| caddy/M-1 | still blocked | Unsupported: method-value-into-keyed-map | 2/14, struct-field-not-resolved |
| caddy/M-2 | still blocked | target-unreachable | 2/13, struct-field-not-resolved |
| caddy/M-3 | still blocked | CallbackRegistration: library-callback-through-interface-field | 2/12, struct-field-not-resolved |
| caddy/M-5 | still blocked | ClosureCapture: closure-capture + interface-method-dispatch | 2/11, struct-field-not-resolved |
| caddy/M-7 | still blocked | target-unreachable | 2/10, struct-field-not-resolved |
| gitea/M-13 | still blocked | CallbackRegistration: closure-callback-registration | 1/7, struct-field-not-resolved |
| gitea/M-16 | still blocked | Unsupported: map-indexed-function-value-call | 2/9, struct-field-not-resolved |
| mattermost/M-1 | still blocked | HTTPHandlerRegistration: http-handler-registration | 2/11, struct-field-not-resolved |
| mattermost/M-10 | still blocked | ChannelFlow: channel-receive-type-switch | 2/10, struct-field-not-resolved |
| mattermost/M-11 | still blocked | target-unreachable | 2/6, unknown-unreachable |
| mattermost/M-12 | still blocked | HTTPHandlerRegistration: http-handler-registration | 1/13, struct-field-not-resolved |
| mattermost/M-13 | still blocked | target-unreachable | 2/12, struct-field-not-resolved |
| mattermost/M-14 | still blocked | HTTPHandlerRegistration: http-handler-registration | 2/13, unknown-unreachable |
| mattermost/M-15 | still blocked | target-unreachable | 2/5, struct-field-not-resolved |
| mattermost/M-2 | still blocked | HTTPHandlerRegistration: http-handler-registration | 2/10, struct-field-not-resolved |
| mattermost/M-3 | still blocked | target-unreachable | 2/15, struct-field-not-resolved |
| mattermost/M-4 | target not found | target-not-found | 2/12, target-not-loaded |
| mattermost/M-5 | still blocked | target-unreachable | 2/5, unknown-unreachable |
| mattermost/M-6 | still blocked | target-unreachable | 2/14, struct-field-not-resolved |
| mattermost/M-7 | still blocked | HTTPHandlerRegistration: http-handler-registration | 2/14, struct-field-not-resolved |
| mattermost/M-8 | still blocked | Unsupported: tagged-union-dispatch | 2/10, struct-field-not-resolved |
| mattermost/M-9 | still blocked | ClosureCapture: closure-capture-into-struct-field | 2/15, struct-field-not-resolved |

## Partial Paths

| Gap reason | Count |
|---|---:|
| struct-field-not-resolved | 18 |
| unknown-unreachable | 3 |
| string-keyed-registry-deferred | 1 |
| target-not-loaded | 1 |

`caddy/M-4` now emits `string-keyed-registry-deferred` for the init-populated command registry gap. `mattermost/M-4` remains `target-not-loaded` because the enterprise target package is not loaded.

## Remaining First Blockers

| Edge kind | Count | Traces |
|---|---:|---|
| HTTPHandlerRegistration | 5 | mattermost/M-1, M-2, M-7, M-12, M-14 |
| Unsupported | 4 | caddy/M-1, caddy/M-4, gitea/M-16, mattermost/M-8 |
| CallbackRegistration | 2 | caddy/M-3, gitea/M-13 |
| ClosureCapture | 2 | caddy/M-5, mattermost/M-9 |
| ChannelFlow | 1 | mattermost/M-10 |

Additional `target-unreachable` traces without a first unsupported edge are caddy/M-2, caddy/M-7, mattermost/M-3, M-5, M-6, M-11, M-13, and M-15.

## Follow-Up Findings

Closure-capture work needs to handle closures that capture interface values and then invoke concrete or interface methods inside route/middleware wrappers, as seen in Caddy routes and templates. It also needs closure callbacks stored in package-level variables or service structs, such as Gitea mailer `send`, and Mattermost worker structs where `SimpleWorker.execute` is captured into a worker container and invoked later from `DoJob`.

Channel-flow work needs to connect goroutine-launched receive loops to values sent through queue channels and type switches. Mattermost/M-10 is the clearest pattern: `(*Service).Start` launches `sendLoop`, `sendLoop` receives a typed request, then dispatches to `sendFile`.

HTTP registration is now the largest concrete blocker class. Mattermost repeatedly registers API handlers through web handler wrappers, then later reaches `Handler.ServeHTTP` and function fields such as `createPost`, `getPost`, and `executeCommand`.

Unsupported map/function-value patterns remain distinct: Caddy template FuncMap style keyed method values, Gitea map-indexed password hasher constructors, and Mattermost tagged-union dispatch.
