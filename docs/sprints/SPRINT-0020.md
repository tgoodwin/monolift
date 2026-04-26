# SPRINT-0020 — Miniflux real-compiler bring-up + retire stub-fixture path

**Status:** planned
**Predecessor:** SPRINT-0019 (cmd-inside-host emission, two-symbol Caddy lift, recursion-safety dual gate).
**Anchor ADRs:** ADR-0017, ADR-0018 (untouched), ADR-0023 (additive amendment for cmd-inside-host oracle pattern + non-`error`-returning fail-closed semantics).
**Drafts:** `docs/sprints/drafts/SPRINT-0020-{CODEX,GEMINI,CLAUDE}.md` plus `-critique.md`.

## Intent

Two coupled goals:

1. **Bring miniflux onto the real compiler.** It is the last OSS target on the stub-fixture path (`test/e2e/stubcompiler/main.go:70` `if !usesRealCompiler { copyTree(...) }`). Lift one in-closure miniflux symbol via the SPRINT-0019 cmd-inside-host emitter and verify with the four-layer stack (per-request `/calls` delta, `/invocations` oracle equality, transcript parity, fail-closed + fail-open).
2. **Delete the stub-fixture path.** Once miniflux is real-compiler-driven, `usesRealCompiler` is a tautology: every target uses the real compiler. Delete the function, delete the fixture-copy branch, delete `test/e2e/stubcompiler/fixtures/`, and rename the binary to reflect what it now does. After this sprint the binary is a pure real-compiler driver.

## Audit of current state (verified)

- `test/e2e/stubcompiler/main.go:346` `usesRealCompiler(target)` returns true for: `caddy`, `pocketbase`, `shape-transport-handler-mismatch`, `state-decl-conflict-stateless-global-store`. All other targets fall through to a fixture-copy path at `:70-79`. The lifted-artifact branch at `:90-99` also still gates on `usesRealCompiler`.
- `test/e2e/stubcompiler/fixtures/` contains only `caddy/` and `miniflux/` subtrees. The `caddy/` entry is dead — caddy is in `usesRealCompiler` and the fixture-copy path no longer runs for it.
- The pragma sub-targets (`shape-transport-handler-mismatch`, etc.) live in `test/e2e/targets/pragma/fixtures/` (NOT in stubcompiler/fixtures/) and are *input source* to the real compiler via `SourceDirs:`. They are not stub fixtures.
- **miniflux is the only OSS target on the stub-fixture path.** Once it flips, `usesRealCompiler` becomes dead code.
- `test/e2e/targets/miniflux/golden/report.json` has empty `closure.includedSymbols` — placeholder, never compiled.
- `test/e2e/targets/miniflux/target.go:10` `SkipReason: "deferred pending v2 compiler FeedProcessor lift — SPRINT-0005"` is a stale reference; SPRINT-0019 mechanism A solved the `internal/` blocker that prevented this work.
- `test/e2e/harness/env.go:11` holds `DefaultCompilerPath = "./bin/stubcompiler"`.
- `pkg/compiler/extract_transport.go:34` calls `caddyEmitContexts` (Caddy-specific). Miniflux needs an explicit context path or generalised target wiring.

## Settled choices

### Symbol pick (Block B)

**Pick:** `miniflux.app/v2/internal/reader/readingtime.EstimateReadingTime(content string, defaultReadingSpeed, cjkReadingSpeed int) int`.

**File:** `evaluation/miniflux/internal/reader/readingtime/readingtime.go:17`.

**Why:**
- **Internal symbol — exercises mechanism A on a non-Caddy module.** SPRINT-0019 closed the `internal/` rule on Caddy; this is the portability claim under non-`github.com/...` modules.
- **Marshalable signature `(string, int, int) → int`.** A strict superset of SanitizeMethod's `(string) → string` rendering shape — proves the SPRINT-0019 forward-design claim ("future basic-typed signature with three params would render without code changes") empirically.
- **Pure function.** No I/O, no DB, no goroutines, no receiver. Trivial oracle, trivial fail-mode.
- **Verified call sites:**
  - `evaluation/miniflux/internal/reader/processor/processor.go:216` (single-entry web-page processing)
  - `evaluation/miniflux/internal/reader/processor/reading_time.go:105` (feed-processing cycle, gated by `user.ShowReadingTime`)
  - `evaluation/miniflux/internal/api/entry_handlers.go:320`, `:405` (API entry update / import — **per-request** firing)

**Workload firing path: API import-entry (primary), feed-refresh (fallback).** The API import-entry path at `entry_handlers.go:405` is per-request and deterministic. Feed-refresh is scheduler/cache-sensitive and goes through Postgres + RSS feed server. Block B prefers API import-entry for the four-layer stack; if API setup proves too brittle we fall back to feed-refresh and switch the counter unit from per-request to per-cycle, recorded explicitly.

**Hidden firing-path precondition (caught by Claude self-critique):** `EstimateReadingTime` is gated on `user.ShowReadingTime` in `reading_time.go:105`. Workload setup must seed an admin user with `ShowReadingTime=true`. Block B.4 is explicit about this.

**Block A stop/go gate on `int`-result rendering (Codex-flagged):** SPRINT-0019's lift path was string-only (`CleanPath: string→string`, `SanitizeMethod: string→string`). `EstimateReadingTime` returns `int`. The httpjson template's parametrization (`ResultFields []FieldSpec`) and the patcher's fail-closed sentinel rendering must support `int` today — or this sprint stops, and we either land an additive emitter improvement or re-pick a string-returning miniflux symbol (`internal/reader/sanitizer.StripTags(string) string` is the documented fallback). **This is a real gate**, not a footnote — Block A.5 proves it before Block B starts.

### Oracle approach

**Decision: cmd-inside-host oracle binary.** Emit `cmd/monolift-oracle-estimatereadingtime/main.go` *inside* the patched miniflux module, alongside `cmd/monolift-extracted-estimatereadingtime/`. The oracle binary directly imports `miniflux.app/v2/internal/reader/readingtime` (legal — same module). The harness invokes it as a Pod (mirrors the extracted-service pattern) and compares HTTP responses for value equality.

This fixes the SPRINT-0019 verification weakness (the SanitizeMethod oracle was a hand-mirror of the symbol's logic in `test/e2e/targets/caddy/oracle.go` because the test package couldn't import `internal/...`). Generalises for any future internal/ symbol. ADR-0023 amended in C.6.

### Fail-closed semantics for non-`error`-returning symbols

`EstimateReadingTime` returns `int`, not `error`. The Caddy-style "sentinel cascade through path matchers to 404" doesn't apply — the request still returns 200; only the *value* of `reading_time` is degraded. **Fail-closed sentinel for `int` is `-1`** (or another distinguished value); the request HTTP status remains 200. Verification observes the sentinel via the `/invocations` oracle equality (which won't match a real `EstimateReadingTime` invocation) and via the response body's `reading_time` field. Recorded in ADR-0023 amendment as a generalisation across return types.

### Binary rename

**Decision: `bin/stubcompiler` → `bin/e2e-compile`.** Punchier than `monolift-e2e-compiler`; clearly describes what the binary now does. Atomic rename in C.5 (final block) so it is scope-cuttable if the blast radius proves larger than the audit. Fallback: keep `bin/stubcompiler` with a one-line comment in `main.go` noting the historical name; documented in C.5 as an in-flight escape hatch.

**Blast radius (enumerated; covered by C.5):**
- `Makefile` build target.
- `test/e2e/harness/env.go:11` `DefaultCompilerPath`.
- Source directory `test/e2e/stubcompiler/` → `test/e2e/e2ecompile/`.
- Test invocations `go test ./test/e2e/stubcompiler/...` → `./test/e2e/e2ecompile/...` (CI, scripts, docs).
- Lockfile path `os.TempDir()/monolift-stubcompiler.lock` → `monolift-e2e-compile.lock`.
- Test file `test/e2e/stubcompiler/stubcompiler_test.go` → renamed file.
- Historical sprint docs SPRINT-0009..SPRINT-0019: read-only history; no rewrites.

## Hard requirements (carried)

- `cmd/main.go` untouched.
- `evaluation/miniflux/` byte-identical pre/post stubcompiler. `make verify-evaluation-untouched` extended to include miniflux.
- Patcher API in `pkg/compiler/transport/emit/liftpatch/` **FROZEN** — no API changes.
- Admission rule in `pkg/compiler/transport/admission.go` unchanged.
- ADR-0018 untouched.
- SPRINT-0018 + SPRINT-0019 e2e (`MONOLIFT_E2E=1 go test -tags=e2e ./test/e2e -run TestE2E/caddy`) **STILL PASSES** unchanged.
- `syscall.Flock` startup guard in stubcompiler (commit `969f0d2`) stays.
- No new Layer-1 liftability property.

## Non-goals

- Multi-symbol miniflux lift in one sprint.
- Receiver-bearing symbols (Cliff 2 still deferred).
- Switching host-build to `go build -overlay`.
- Touching pocketbase, the pragma sub-targets, or shape/state-decl conflict targets — they are already on the real compiler.
- Mattermost (next sprint candidate).
- Multi-symbol lift in one extracted-service binary.

## Sequencing — three blocks

### Block A — Boot miniflux + prove the emitter on `int` results

Goal: prove the harness can boot Postgres + RSS feed server + miniflux + extracted-service in kind, prove the closure regenerates with `EstimateReadingTime` in `closure.includedSymbols`, and prove the existing emitter machinery handles the `(string,int,int)→int` shape. This is the stop/go gate before any lift work.

- [x] **A.1** Remove `SkipReason` from `test/e2e/targets/miniflux/target.go:10`. Add a SPRINT-0020 regen note.
- [x] **A.2** Boot-and-curl smoke: stand up the existing baseline manifests (`test/e2e/fixtures/postgres.yaml`, `test/e2e/fixtures/rss-feed-server.yaml`, `test/e2e/targets/miniflux/baseline/{deployment,service}.yaml`) in kind. Confirm Postgres `Ready`, `RUN_MIGRATIONS=1` initialised the schema, miniflux `/healthcheck` returns 200, and from inside the namespace `curl http://rss-feed-server/index.xml` returns the deterministic RSS corpus. If any step fails, fix bitrot before continuing.
- [x] **A.3** Pin Postgres → RSS feed server → miniflux → extracted-service readiness ordering in the harness. Sized for Postgres startup (longer than Caddy's). Deployer's parallel readiness loop must handle the multi-pod set without the Caddy-shaped timeout.
- [x] **A.4** Regenerate `test/e2e/targets/miniflux/golden/report.json` from the real compiler against `evaluation/miniflux`. Closure must include `{module_path: miniflux.app/v2, package_path: miniflux.app/v2/internal/reader/readingtime, object_name: EstimateReadingTime, kind: function, file: internal/reader/readingtime/readingtime.go, line_start: 17}`. Add a closure-pin regression test alongside the SPRINT-0019 caddy pin.
- [x] **A.5** **`int`-result emitter sanity test** (the stop/go gate). Render-only AST test: synthesise a `Context` for `EstimateReadingTime`, invoke `httpjson.Render`, parse the result, assert (a) `main.go` imports `miniflux.app/v2/internal/reader/readingtime`; (b) calls `readingtime.EstimateReadingTime(in.Content, in.DefaultReadingSpeed, in.CjkReadingSpeed)` literally; (c) response struct has an `int` `ReadingTime` field; (d) `Dockerfile.extracted` build context paths resolve for non-`github.com/...` modules. Run `go build` against the rendered tree to prove compilation. **If any of (a)–(d) fail, stop the sprint and decide: (i) land an additive emitter fix as a Block A pre-step; (ii) re-pick a string-returning miniflux symbol (`internal/reader/sanitizer.StripTags(string) string` is the documented fallback). Do not proceed to Block B with a workaround.**
- [x] **A.6** Verify `evaluation/miniflux/` is byte-identical pre/post stubcompiler. Extend `make verify-evaluation-untouched` (or its check) to cover miniflux. The patcher must NOT mutate `evaluation/miniflux/` in place; it operates against the host-patch staging directory only.
- [x] **A.7** Frozen-API audit: confirm no changes to `pkg/compiler/transport/emit/liftpatch/`, `pkg/compiler/transport/admission.go`, or `pkg/compiler/liftability/property.go`. Block A's emitter proof must work without touching these.

**Block A gate:** kind cluster boots all baseline manifests; closure regen contains `EstimateReadingTime`; the `int`-result render+build test passes; `evaluation/miniflux/` byte-identical; SPRINT-0019 actor-adapter assertions for caddy still pass.

### Block B — Lift `EstimateReadingTime` and verify the four-layer stack

Goal: lift the symbol via the cmd-inside-host emitter, deploy lifted miniflux + extracted-service Pod + oracle Pod, drive the per-request firing path, and verify all four layers.

- [x] **B.1** Generalise the transport-context wiring. `pkg/compiler/extract_transport.go:34` currently calls `caddyEmitContexts`; generalise to dispatch on target identity. Add a miniflux context that emits one extracted service for `EstimateReadingTime` (service `monolift-extracted-estimatereadingtime`, env prefix `MONOLIFT_LIFT_ESTIMATEREADINGTIME`, package `miniflux.app/v2/internal/reader/readingtime`, params `content/default_reading_speed/cjk_reading_speed`, result `reading_time int`). Pragma sub-targets keep their existing path.
- [x] **B.2** Generalise stubcompiler lifted-tree materialisation in `test/e2e/stubcompiler/main.go` from caddy-only to caddy + miniflux. Each target gets its own `<output>/lifted/host-patch/` (the patcher's frozen API doesn't change). Miniflux gets `Dockerfile.host`, extracted-service Dockerfile + Deployment + Service, `MANIFEST.json`, and `LIFTPATCH.json`. `packageDirFor` (or its equivalent) gets a small `modulePath → directoryName` map covering `github.com/caddyserver/caddy/v2`, `github.com/pocketbase/pocketbase`, and `miniflux.app/v2`.
- [x] **B.3** Build the cmd-inside-host oracle binary template. Emit `cmd/monolift-oracle-estimatereadingtime/main.go` inside the patched miniflux module: imports `miniflux.app/v2/internal/reader/readingtime` directly, serves `POST /invoke {p, default_reading_speed, cjk_reading_speed}` returning `{reading_time: int}`. Build a Docker image and Deployment + Service following the SPRINT-0019 extracted-service pattern. Stubcompiler emits all of this alongside the extracted-service artifacts.
- [x] **B.4** Implement `test/e2e/targets/miniflux/workload.go`:
  - `Setup`: authenticate against miniflux via the bootstrap admin (per `RUN_MIGRATIONS=1` initialised user), set `ShowReadingTime=true` on the admin's user-prefs (the firing-path precondition), create one feed pointing at `http://rss-feed-server/index.xml`.
  - `Action`: drive the API import-entry path — `POST /v1/feeds/{id}/entries` (or whichever route maps to `entry_handlers.go:405`) with one entry payload containing fixed HTML content. Each call invokes `EstimateReadingTime` once.
  - `Verify`: returned entry has non-zero `reading_time` (when lifted) AND the `/invocations` record matches the in-cluster oracle binary's response for the same args.
- [x] **B.5** Lifted miniflux deployment env: `MONOLIFT_LIFT_ESTIMATEREADINGTIME=on`, `MONOLIFT_LIFT_FAILMODE=closed` (default), `MONOLIFT_LIFT_ESTIMATEREADINGTIME_ENDPOINT=http://monolift-extracted-estimatereadingtime:8081/invoke`. Extracted-service Deployment env: omit all `MONOLIFT_LIFT_*` (recursion safety).
- [x] **B.6** Recursion-safety dual gate (carry from SPRINT-0019.C.1/C.2):
  - **Static:** harness asserts the rendered extracted-service Deployment YAML grep-clean for `MONOLIFT_LIFT_[A-Z_]+:` in the env block.
  - **Runtime:** post-deploy, port-forward the extracted Pod, send one `POST /invoke` with no `MONOLIFT_LIFT_*` env on the extracted container, verify `/calls` increments by exactly 1 (proves no recursion through the dormant patched body).
- [x] **B.7** Per-request counter delta + aggregate: harness reads the extracted Pod's `/calls` before each workload request, asserts `>= 1` delta, accumulates total. Aggregate `<= 50` total per workload run (catches recursion / accidental client loops). If the workload necessarily uses feed-refresh (fallback path), document the counter as per-cycle and assert `>= 1` per cycle instead.
- [x] **B.8** Per-invocation oracle equality: for each `/invocations` record on the extracted Pod, harness invokes the cmd-inside-host oracle Pod via HTTP with the same args, asserts response equality on `reading_time`. Load-bearing falsifiability check.
- [x] **B.9** Transcript parity: capture baseline transcript (env-off deployment, same image), capture lifted transcript (env-on), assert response equality on the workload requests.
- [ ] **B.10** Negative test: re-deploy lifted miniflux with `MONOLIFT_LIFT_ESTIMATEREADINGTIME` *unset*; assert `/calls` delta = 0; transcripts identical to env-on case.
- [ ] **B.11** Fail-mode tests:
  - **Fail-closed (default).** Scale `monolift-extracted-estimatereadingtime` to 0 replicas, fire workload, assert response status is 200 (request succeeds) AND `reading_time` field equals the sentinel `-1` (degraded value visible). `/calls` stays at 0. Scale back to 1, run again, assert real values resumed.
  - **Fail-open.** Re-deploy lifted miniflux with `MONOLIFT_LIFT_FAILMODE=open`, scale extracted to 0, fire workload, assert 200 + real (locally-computed) `reading_time` (degraded but available, original body executed). Counter stays at 0. Restore replicas, counter increments.

**Block B gate:** `MONOLIFT_E2E=1 go test -tags=e2e ./test/e2e -run TestE2E/miniflux -count=1` green: per-request `/calls` delta `>= 1`, aggregate `<= 50`, oracle equality on every `/invocations` record, transcript parity, env-off zero counter, fail-closed sentinel `-1`, fail-open real value. SPRINT-0018 + SPRINT-0019 caddy e2e (`MONOLIFT_E2E=1 go test -tags=e2e ./test/e2e -run TestE2E/caddy`) STILL passes unchanged.

### Block C — Delete the stub-fixture path + atomic rename

Goal: now that miniflux is on the real compiler, `usesRealCompiler` is dead code. Remove it cleanly. Then atomically rename the binary.

- [ ] **C.1** Delete `func usesRealCompiler(target string) bool` at `test/e2e/stubcompiler/main.go:346`. Delete the `if !usesRealCompiler { copyTree(...) }` fixture-copy branch at `:70-79`. Delete the `else if … copyLiftedArtifacts` branch at `:90-99` that also gated on `usesRealCompiler`. Verify `git grep usesRealCompiler` returns zero matches after this commit.
- [ ] **C.2** Delete `test/e2e/stubcompiler/fixtures/` entirely (both `caddy/` dead subtree and `miniflux/` subtree). Verify `git grep test/e2e/stubcompiler/fixtures` returns zero matches.
- [ ] **C.3** Sweep orphaned helpers: if `copyTree` / `copyFile` / `copyLiftedArtifacts` no longer have callers after C.1+C.2, delete them. Verify `go build ./...` succeeds.
- [ ] **C.4** Full per-target matrix gate **before** the rename (so bisect is clean): run caddy + pocketbase + miniflux + all pragma sub-targets through stubcompiler, assert each produces a valid report and the expected verdict. `go test ./test/e2e/stubcompiler/...` passes. `MONOLIFT_E2E=1 go test -tags=e2e ./test/e2e -count=1` passes for every non-skipped target.
- [ ] **C.5** **Atomic rename** in a single commit (so it can be reverted independently if the blast radius blows up):
  - `bin/stubcompiler` → `bin/e2e-compile` in `Makefile` build target.
  - `test/e2e/harness/env.go:11` `DefaultCompilerPath` → `"./bin/e2e-compile"`.
  - Source directory `test/e2e/stubcompiler/` → `test/e2e/e2ecompile/`.
  - Test file `stubcompiler_test.go` → `e2ecompile_test.go` (or equivalent inside the renamed dir).
  - Lockfile path: `monolift-stubcompiler.lock` → `monolift-e2e-compile.lock` in the syscall.Flock guard.
  - All references in `Makefile`, scripts, doc strings, sprint plans being updated this sprint, and CI configs.
  - `git grep -i stubcompiler` returns zero matches after the rename commit (excluding historical sprint docs SPRINT-0009..SPRINT-0019, which stay as read-only history).
  - **Scope-cut fallback (only if the rename's blast radius exceeds estimate):** keep `bin/stubcompiler` with a one-line comment in `main.go` ("`historical name; binary is now a real-compiler driver — see SPRINT-0020`"). C.5 then becomes a no-op rename + comment-only commit. Decision recorded in C.7.
- [ ] **C.6** Add ADR-0023 amendment titled "Internal-rule compliance for oracle binaries via cmd-inside-host" recording: (a) the cmd-inside-host oracle binary pattern as the load-bearing fix to SPRINT-0019's mirror-in-harness fragility, (b) the non-`error`-returning fail-closed sentinel semantics (sentinel value at the result-type level; HTTP status remains 200), (c) the closure of the stub-fixture path across all OSS targets. ADR-0018 unchanged.
- [ ] **C.7** Append `docs/evolution.md` paragraph summarising the SPRINT-0020 landing. Record the rename decision (kept rename / scope-cut comment).
- [ ] **C.8** Verify `cmd/main.go` unchanged.

**Block C gate (sprint acceptance):** `usesRealCompiler` gone; `test/e2e/stubcompiler/fixtures/` gone; full per-target matrix passes pre-rename; rename committed atomically (or scope-cut to comment) and post-rename matrix still passes; `git grep usesRealCompiler` and `git grep test/e2e/stubcompiler/fixtures` both zero; `git grep stubcompiler` zero outside historical docs.

## Acceptance criteria

All must hold at sprint close:

- [ ] `test/e2e/targets/miniflux/target.go` is no longer skipped.
- [ ] `test/e2e/targets/miniflux/golden/report.json` `closure.includedSymbols` contains `EstimateReadingTime` at line 17.
- [ ] `evaluation/miniflux/` byte-identical pre/post stubcompiler. `make verify-evaluation-untouched` (or equivalent) passes for caddy and miniflux.
- [ ] Stubcompiler emits `<output>/lifted/host-patch/` for miniflux containing the patched `internal/reader/readingtime/readingtime.go` (CleanPath-style prepended `*ast.IfStmt`), the sibling `monolift_lift_estimatereadingtime.go` dialer, the `cmd/monolift-extracted-estimatereadingtime/main.go` extracted-service binary, AND the `cmd/monolift-oracle-estimatereadingtime/main.go` oracle binary. Zero string-substituted symbol bodies; AST tests assert the real selector call.
- [ ] `MONOLIFT_E2E=1 go test -tags=e2e ./test/e2e -run TestE2E/miniflux -count=1` green: per-request `/calls` delta `>= 1`, aggregate `<= 50`, oracle equality, transcript parity, env-off zero counter, fail-closed sentinel `-1` (HTTP 200), fail-open real value (HTTP 200).
- [ ] SPRINT-0018 + SPRINT-0019 caddy e2e still passes unchanged.
- [ ] `usesRealCompiler` deleted from `test/e2e/stubcompiler/main.go`. `if !usesRealCompiler` and `else if … copyLiftedArtifacts` branches deleted. `git grep usesRealCompiler` zero matches.
- [ ] `test/e2e/stubcompiler/fixtures/` deleted. `git grep test/e2e/stubcompiler/fixtures` zero matches.
- [ ] Pre-rename per-target matrix passes (caddy, pocketbase, miniflux, all pragma sub-targets).
- [ ] Binary renamed to `bin/e2e-compile` (or kept with historical comment if scope-cut). `harness/env.go:11`, `Makefile`, source directory, lockfile path, test invocations all updated atomically. `git grep stubcompiler` returns zero matches outside historical sprint docs.
- [ ] ADR-0023 contains "Internal-rule compliance for oracle binaries via cmd-inside-host" amendment.
- [ ] ADR-0018 unchanged. `pkg/compiler/transport/admission.go` unchanged. `pkg/compiler/transport/emit/liftpatch/` API unchanged. No new Layer-1 properties.
- [ ] `cmd/main.go` unchanged.
- [ ] `syscall.Flock` startup guard from SPRINT-0019 commit `969f0d2` preserved (renamed lockfile path).

## Forward-design sanity check

- [ ] `httpjson` template renders `(string,int,int) → int` symbols without code changes — proven by Block A.5.
- [ ] Patcher API remains symbol-agnostic; any future `(...basic...) → (...basic...)` symbol works without re-opening the API.
- [ ] cmd-inside-host oracle generalises across modules (`github.com/...` and `miniflux.app/...`). Future targets reuse the pattern.
- [ ] Fail-closed sentinel mechanism is type-aware: cascade-to-404 for `string`-result symbols (matcher-driven), sentinel-value for non-`error`-returning symbols. ADR-0023 records both.
- [ ] No target fingerprinting by import-path prefix in the patcher or admission rule. Targets dispatch only at the e2e harness layer.

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| **Block A.5 fails: `int`-result rendering not supported by current emitter.** | Stop the sprint at Block A. Decide (a) land additive emitter fix; (b) re-pick `internal/reader/sanitizer.StripTags(string) string`. Do not proceed with a synthetic workaround. |
| **Miniflux baseline manifests bitrotted since SPRINT-0005.** | Block A.2 boot-and-curl smoke before any lift work. If bitrot exceeds reasonable fix scope, surface and decide whether SPRINT-0020 closes the bring-up gap or punts. |
| **`ShowReadingTime` user-flag precondition.** | Block B.4 explicit setup step. Workload Verify also asserts `reading_time != 0` on baseline (env-off) responses to confirm the precondition holds. |
| **API import-entry path fails / not deterministic.** | Fall back to feed-refresh path with per-cycle counter delta. Counter unit change recorded explicitly. |
| **`miniflux.app/v2` non-`github.com/...` module path breaks cmd-inside-host emission.** | Block A.5 isolates this as a render-only test before the full pipeline runs. Fast-fail. |
| **Postgres + RSS + miniflux + extracted + oracle = 5 pods, kind cluster slow.** | Parallel readiness wait (SPRINT-0019.B.8 pattern); sized for Postgres startup. If image-build or kind-load times out, observe + extend the existing 10-min context budget. Do not paper over with retries. |
| **Recursion: extracted-service binary contains the patched body.** | Static YAML grep + runtime single-increment test (B.6). Belt and suspenders. |
| **Oracle Pod and extracted-service Pod confused / the same binary by accident.** | They are different `cmd/` targets — `monolift-oracle-estimatereadingtime` and `monolift-extracted-estimatereadingtime`. Different images, different deployments, different Services. B.3 emits both; harness asserts both Ready before workload. |
| **Stub-removal accidentally deletes pragma-fixture sources.** | Pragma fixtures live at `test/e2e/targets/pragma/fixtures/` (different path). C.2 deletes only `test/e2e/stubcompiler/fixtures/`. Test C.4 runs all pragma sub-targets to confirm. |
| **Rename blast radius exceeds estimate.** | C.5 scope-cut fallback: keep `bin/stubcompiler`, add historical comment, defer rename. Decision recorded in C.7. |
| **Concurrent stubcompiler invocations OOM-kill (SPRINT-0019 lesson).** | `syscall.Flock` startup guard from `969f0d2` preserved. Lockfile path renames with the binary in C.5. |
| **`make verify-evaluation-untouched` doesn't currently cover miniflux.** | A.6 extends it. If the make target is too entangled with caddy-only paths, generalise minimally — do not re-open admission or patcher APIs. |

## Resolved blockers

- **A.4 Go 1.26 target requirement:** `evaluation/miniflux` declares `go 1.26.0`, and a `stubcompiler` binary built with the repo default Go 1.25.4 fails `go/packages` loading with "package requires newer Go version go1.26". Resolved by building the e2e compile driver with `GOTOOLCHAIN=go1.26.0` for e2e runs and test-spawned driver invocations.
- **B.2 int fail-closed sentinel:** the liftpatch client template still rendered the original string sentinel for every result type. Resolved with additive type-aware sentinel rendering; `int` results now use `-1` without changing the frozen patcher API.

## Roadmap follow-ups

- **SPRINT-0021 (provisional):** Mattermost target bring-up. Ambitious: real-time WebSocket-heavy server, larger codebase, harder closure. Earns the right to start now that the real-compiler path is proven on caddy + miniflux + pocketbase + pragma.
- **Future un-numbered:** Receiver-bearing symbol slice (Cliff 2 in earnest).
- **Future un-numbered:** Multi-symbol lift in one extracted-service binary.
- **Future un-numbered:** Replace HTTP/JSON with a typed-codec template (gRPC, capnp).
- **Conditional follow-up:** if Block A.5 surfaces `int`-result emitter limitations and we additively land a fix, document the fix in ADR-0023.

## Committee notes

Drafts and critiques preserved at `docs/sprints/drafts/SPRINT-0020-{CODEX,GEMINI,CLAUDE}.md` and `-critique.md`.

**Convergences adopted across drafts/critiques:**
- Symbol pick `EstimateReadingTime` — unanimous.
- cmd-inside-host oracle binary (not in-harness mirror) — unanimous.
- Stub purge: delete `usesRealCompiler` + fixture-copy branch + `test/e2e/stubcompiler/fixtures/` — unanimous.
- Block A `int`-result emitter sanity test as a real stop/go gate — Codex framing, validated by both critiques.
- API import-entry primary, feed-refresh fallback — Codex framing, validated by both critiques.
- Postgres + RSS + miniflux + extracted-service + oracle multi-pod readiness ordering — Claude framing.
- Recursion-safety dual gate carried verbatim from SPRINT-0019.C.1/C.2 — Claude framing, validated.
- Patcher API frozen, admission rule unchanged, ADR-0018 untouched — unanimous.

**Disagreements resolved (committee + critiques):**
- **Workload firing path:** API import-entry primary (Codex), feed-refresh fallback. Claude draft had it backwards; both critiques caught this.
- **`int`-result fail-closed semantics:** sentinel `int` (e.g. `-1`), HTTP status remains 200. NOT 404 cascade — that's Caddy/string-specific. Both critiques caught Gemini's wrong "404/error on refresh" framing.
- **Binary rename:** `bin/e2e-compile` (Claude's pick) over `bin/monolift-e2e-compiler` (Gemini) — punchier, source-dir-friendly. Atomic rename in C.5 with documented scope-cut fallback (Codex's instinct). Codex's "default defer" softened — the merged plan commits to the rename and only falls back if blast radius proves larger.
- **`ShowReadingTime` precondition:** Claude self-critique caught it; merged plan adds explicit setup step in B.4. None of the original drafts had this.
- **Closure-pin sequencing:** closure regen in Block A (before lift work), pin alongside the SPRINT-0019 caddy pin. Unanimous via critiques.
- **Stub-removal surgical precision:** Claude's line-number-precise deletions (70-79, 90-99, 346) over Codex/Gemini's general deletion language. Critiques validated.
- **Multi-target context dispatch:** generalise `caddyEmitContexts` at `pkg/compiler/extract_transport.go:34` to a target-dispatched function (Codex caught this); add a `modulePath → directoryName` map (Gemini's `packageDirFor` framing).

**Items rejected from drafts:**
- Gemini's wrong "404/error on refresh" fail-closed semantics for `int` results.
- Gemini's `monolift-e2e-compiler` rename name (verbose, inconsistent with `test/e2e/e2e-compiler/` directory which has hyphen issues for Go package paths).
- Gemini's hand-waved workload `Verify` ("entries exist and have non-zero reading_time" without oracle comparison).
- Codex's "default defer rename" — softened to scope-cut fallback only, not the default outcome.
- Claude's "decide in flight" rename straddle — replaced with commit-to-rename plus documented scope-cut.
- Claude's feed-refresh-as-primary workload — replaced with API import-entry as primary.
- Any draft's omission of the `ShowReadingTime` precondition.
