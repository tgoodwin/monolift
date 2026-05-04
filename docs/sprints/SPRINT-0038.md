# SPRINT-0038 — Close remaining gitea gaps: callback arguments + map-keyed dispatch

**Status:** planned
**Executor:** TBD
**Predecessor:** SPRINT-0037 (69/72 reachable, 95.8%)

## Intent

Close the last 2 algorithm gaps in the activation-path corpus: gitea/M-13 (closure passed as callback argument to a queue constructor) and gitea/M-16 (map-keyed factory function registry). Both are focused augmentation passes that extend the existing struct-field tracking pattern to new value-flow shapes.

## The 2 patterns

### gitea/M-13: callback argument + package-level variable

The trace expects:
```
Step 5: services/mailer/mailer.go:52 → closure-callback-registration
Step 6: services/mailer/sender/sender.go:17 → call-through-package-level-function-variable
```

What happens in the code:
1. `NewContext()` in `mailer.go:25` creates a closure that calls `sender_service.Send(sender, msg)`
2. The closure is passed as an argument to `queue.CreateSimpleQueue(ctx, "mail", handlerFunc)`
3. The queue worker later invokes this closure
4. Inside the closure, `sender` is a **package-level variable** set to `&sender_service.SMTPSender{}` at `mailer.go:43`

The analyzer needs: (a) track function values passed as arguments to callees and recognize when the callee stores/invokes them, and (b) track package-level variable stores of function/interface values.

### gitea/M-16: map-indexed function-value dispatch

The trace expects:
```
Step 6: modules/auth/password/hash/hash.go:134 → map-indexed-function-value-call
```

What happens in the code:
1. `init()` in `argon2.go:16` calls `MustRegister("argon2", NewArgon2Hasher)`
2. `Register()` stores a wrapper factory into `availableHasherFactories["argon2"]` (a `map[string]func(string) PasswordSaltHasher`)
3. `Parse(algorithmSpec)` at `hash.go:134` looks up `availableHasherFactories[name]` and calls the returned factory

The analyzer needs: track function values stored into maps via assignment/store, and connect map index+call sites to the stored functions.

## Task list

### Phase 0 — Package-level variable tracking

Package-level variables holding function/interface values are a simpler version of struct-field tracking. Gitea/M-13 needs this for `var sender sender_service.Sender`.

- [ ] **0.1** Implement `AugmentPackageVars(graph *Graph, program *Program)` in `pkg/activation/pkgvar.go`. Scan all `init` functions and package-level `*ssa.Store` instructions for stores to `*ssa.Global` where the stored value has a function or interface type. Record `global → []*ssa.Function` mappings.
- [ ] **0.2** Scan for loads from the same globals followed by calls (method calls on the loaded interface value, or direct calls if it's a function type). Add edges from the calling function to each stored concrete function/implementation.
- [ ] **0.3** Fixture test: package-level `var handler func()` set in `init()`, called from `main()`. Verify edge is found.
- [ ] **0.4** Update `rtaRepresents()` to accept `CallbackRegistration` edge kind (used for this pattern).

### Phase 1 — Function-value-as-argument tracking

When a function value is passed as an argument to a callee, and the callee stores or invokes it. Gitea/M-13 needs this for `queue.CreateSimpleQueue(ctx, "mail", handlerFunc)`.

- [ ] **1.1** Implement `AugmentFuncArgs(graph *Graph, program *Program)` in `pkg/activation/funcarg.go`. Scan for `*ssa.Call` instructions where one of the arguments has a function type (is a `*ssa.Function`, `*ssa.MakeClosure`, or has `func` type). For each such call, check if the callee stores the argument into a struct field or invokes it. If so, add an edge from the callee (or a function that later invokes the stored value) to the argument function.
- [ ] **1.2** Simpler alternative if 1.1 is too complex: scan for function-typed arguments passed to any call, and add a `CallbackRegistration` edge from the callee to the passed function. This over-approximates but is sufficient for queue-handler patterns.
- [ ] **1.3** Fixture test: `registerHandler(myFunc)` where `registerHandler` stores and later invokes the function. Verify edge is found.

### Phase 2 — Map-keyed function-value tracking

Function values stored in maps and dispatched by key lookup. Gitea/M-16 needs this for `availableHasherFactories`.

- [ ] **2.1** Implement `AugmentMapFuncValues(graph *Graph, program *Program)` in `pkg/activation/mapfunc.go`. Scan for `*ssa.MapUpdate` instructions where the value operand has a function type. Record `(mapType) → []*ssa.Function` mappings.
- [ ] **2.2** Scan for `*ssa.Lookup` on the same map type followed by a `*ssa.Call` where the looked-up value is the callee. Add edges from the calling function to each function stored in that map.
- [ ] **2.3** Handle the `Register` wrapper pattern: when the stored value is a closure created inside `Register[T]()` that wraps the `newFn` parameter, trace through the closure to the original argument passed to `Register`/`MustRegister`.
- [ ] **2.4** Fixture test: `registry["key"] = myFunc` then `f := registry["key"]; f()`. Verify edge is found.
- [ ] **2.5** Update `rtaRepresents()` to accept any new edge kinds used for map dispatch.

### Phase 3 — Wire into augmentation loop + evaluate

- [ ] **3.1** Add the new passes to `Augment()` in `augment.go`, called within the iterative convergence loop so newly-explored code feeds back into discovery.
- [ ] **3.2** Run evaluation on gitea only. Verify M-13 and M-16 are now reachable.
- [ ] **3.3** Run full 72-trace evaluation with all augmentations. Save to `docs/research/runs/SPRINT-0038-final.json`. Target: 71/72 (mattermost/M-4 remains target-not-found).
- [ ] **3.4** Verify no regressions: all 69 previously-reachable traces must stay reachable.
- [ ] **3.5** Verify determinism: run twice, diff JSONs.
- [ ] **3.6** Run `go test ./pkg/activation/...` — all pass.
- [ ] **3.7** Update `docs/research/activation-paths/README.md` with SPRINT-0038 summary.

## Acceptance criteria

1. gitea/M-13 is reachable.
2. gitea/M-16 is reachable.
3. Corpus reachability ≥ 71/72 (98.6%).
4. No regressions on the 69 previously-reachable traces.
5. `go test` passes on `pkg/activation/...`.
