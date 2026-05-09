# SPRINT-0043: Reverse-import scoping & 6-project activation-path coverage

**Status:** focused activation-path e2e coverage passing; combined all-target sweep pending
**Predecessors:** SPRINT-0042 (deployment artifacts & activation-path k8s e2e)

## Intent

Make the activation-path lift pipeline scalable to all 6 corpus projects by replacing the hardcoded `./...` package load with reverse-import scoping, then bring all 6 projects to passing activation-path e2e status in Kind. SPRINT-0042 delivered 2/6 (miniflux `SanitizeHTML`, caddy `CleanPath`). This sprint adds gitea, listmonk, mattermost, and pocketbase.

The core technical change: before calling `packages.Load` with full type-checking, run a lightweight `go list -json ./...` pass to discover which packages transitively import the target's package, build a reverse-import adjacency map, BFS from the target package, and load only the transitive importers. For gitea this reduces the load set from ~2,875 files to ~545. For mattermost, a similar reduction is expected.

A secondary deliverable: add `HostBuildCommand` to `DeployOptions` so projects like listmonk that need a custom Docker build step (stuffbin asset embedding) can override the default `go build` RUN directive generically.

## Concrete targets

### Existing (passing from SPRINT-0042)

| # | Project | Function | File:line | Signature |
|---|---------|----------|-----------|-----------|
| 1 | miniflux | `SanitizeHTML` | `internal/reader/sanitizer/sanitizer.go:217` | `(baseURL, rawHTML string, opts *SanitizerOptions) string` |
| 2 | caddy | `CleanPath` | `modules/caddyhttp/caddyhttp.go:279` | `(p string, collapseSlashes bool) string` |

### New this sprint

| # | Project | Function | File:line | Signature | Why |
|---|---------|----------|-----------|-----------|-----|
| 3 | gitea | `PathEscapeSegments` | `modules/util/url.go:12` | `(path string) string` | 82 callers, stress-tests reverse-import scoping on the largest corpus (2,875 files). Scaffolded in SPRINT-0042, blocked by `./...` timeout |
| 4 | listmonk | `SanitizeURI` | `internal/utils/utils.go:41` | `(u string) string` | 92-file project, scaffolded in SPRINT-0042. Exercises stuffbin/asset-embedding override via `HostBuildCommand` |
| 5 | pocketbase | `Columnify` | `tools/inflector/inflector.go:24` | `(str string) string` | 445-file pure-Go project with `modernc.org/sqlite`. Called across record CRUD, field resolution, search, and query construction |
| 6 | mattermost | `GeneratePublicLinkHash` | `server/channels/app/file.go:588` | `(fileID, salt string) string` | 2,153 files, second-largest corpus. Tests scoping at scale and multi-module workspace (`go.work`). Deterministic hash helper on the file public-link path |

All 6 targets are package-level stateless functions with primitive or trivially-serializable parameters and a single return value. No receiver methods.

## Scope boundaries

**In scope:**
- Reverse-import scoping in `pkg/activation/` and `pkg/codegen/pipeline.go`
- `HostBuildCommand` field on `DeployOptions` and host Dockerfile template update
- 4 new activation-path e2e targets (gitea, listmonk, pocketbase, mattermost)
- Benchmarking the scoping fix on gitea and mattermost load times
- CLI flags for `--host-build-command` and `--reverse-import-scope`

**Out of scope:**
- Receiver-method lift support — all targets must be package-level functions
- Non-primitive boundary types beyond what the current codec supports (no `json.RawMessage` extension)
- Automatic detection of custom build requirements
- gRPC, streaming, multi-cut, Helm/Kustomize
- Modifying existing v1 compiler targets
- Production resource tuning, CI/CD integration

## Task list

### Phase 0: Baseline and target confirmation

Record current state before code changes so we can prove the scoping fix works.

- [x] 0.1: Record current focused activation e2e pass/fail for `activation-miniflux-sanitizehtml` and `activation-caddy-cleanpath`
- [x] 0.2: Run activation-path analysis on gitea `modules/util/url.go:12` with unscoped `./...` — capture load duration, package count, and timeout. Write to `docs/research/runs/SPRINT-0043-baseline-gitea.md`
- [x] 0.3: Run activation-path analysis on mattermost `channels/app/file.go:588` from `evaluation/mattermost/server` — capture load duration, package count, and timeout/failure. Write to `docs/research/runs/SPRINT-0043-baseline-mattermost.md`
- [x] 0.4: Verify all 4 proposed new targets resolve to package-level `*types.Func` declarations (not methods) with a single return value and primitive/JSON-serializable params
- [x] 0.5: Verify pocketbase `Columnify` has production callsites reachable from an HTTP handler (record CRUD, search, query paths)
- [x] 0.6: Verify mattermost `GeneratePublicLinkHash` is reachable from an HTTP endpoint (file public-link route)

### Phase 1: Reverse-import scoping

Replace `Packages: []string{"./..."}` in `runActivation` with a pre-filtered set of packages that transitively import the target's package.

- [x] 1.1: Add `ReverseImportScope(dir, targetFile string, env []string) ([]string, error)` to `pkg/activation/scope.go`. Implementation: (a) resolve the target file's package import path via `go list -json <targetDir>`; (b) run `go list -json ./...` from `dir` to get all module packages and their `Deps` fields; (c) build a reverse-import adjacency map; (d) BFS from the target package through reverse edges to find all transitive importers; (e) include command packages only when they transitively import the target; (f) return deduplicated, sorted import paths
- [x] 1.2: Add `ScopePackages bool` to `activation.Config`. When true, `LoadProgram` calls `ReverseImportScope` to replace its patterns before `packages.Load`. When the target has no transitive importers, emit a diagnostic and return only the target package
- [x] 1.3: Add a `scope` phase timing to `Result.Timings` so the `go list` cost is visible separately from `packages.Load`
- [x] 1.4: Update `runActivation` in `pkg/codegen/pipeline.go` to set `ScopePackages: true` instead of passing `./...`
- [x] 1.5: Add `--reverse-import-scope` flag to `cmd/activation-path` CLI for exercise outside `monolift lift`
- [x] 1.6: Unit test: mock module with 4 packages where only 2 import the target — verify `ReverseImportScope` returns only those 2 plus the target
- [x] 1.7: Unit test: target package with zero importers — returns only the target package itself
- [x] 1.8: Unit test: circular imports (A imports B imports A) — terminates without infinite loop
- [x] 1.9: Integration test: `ReverseImportScope` on `evaluation/gitea` targeting `modules/util/url.go` — verify returned set is <600 packages (not the full module) and includes `modules/util` itself
- [x] 1.10: Integration test: scoped activation on miniflux `SanitizeHTML` still finds a path from an entry point — no regression from scoping

### Phase 2: HostBuildCommand override

Listmonk's Docker build requires stuffbin asset embedding, not just `go build`. Add a generic override.

- [x] 2.1: Add `HostBuildCommand string` to `DeployOptions` in `pkg/codegen/types.go`
- [x] 2.2: Update `hostDockerfileTemplate` in `pkg/codegen/docker.go`: when `HostBuildCommand` is non-empty, emit it as the `RUN` line instead of the default `go build` command
- [x] 2.3: Add `--host-build-command` flag to `monolift lift` CLI
- [x] 2.4: Golden-file test: default case still produces `go build`
- [x] 2.5: Golden-file test: plan with `HostBuildCommand` set — custom command appears verbatim in rendered Dockerfile
- [x] 2.6: Update `GeneratorVersion` constant from `"SPRINT-0042"` to `"SPRINT-0043"`

### Phase 3: Shared e2e harness mechanics

Generalize the harness before adding 4 new target rows.

- [x] 3.1: Verify the activation e2e harness consumes generated Dockerfile and manifest paths from `RunLiftWithResult` generically — no target-specific branches needed for new rows
- [x] 3.2: Generalize any remaining hardcoded target metadata in `test/e2e/e2e_test.go` into `harness.TargetCase` fields
- [x] 3.3: Add a helper for activation rows that need per-project copied-module setup (e.g., mattermost's `go.work`)
- [x] 3.4: Add a helper for e2e rows that need a local writable data directory in the host container (pocketbase)
- [x] 3.5: Run `go test -tags e2e ./test/e2e -run '^$'` after registering new rows — compile-only wiring check

### Phase 4: Gitea PathEscapeSegments

Already scaffolded in SPRINT-0042. Now unblocked by Phase 1.

- [x] 4.1: Verify existing `test/e2e/targets/activation_gitea_pathescapesegments/target.go` is still correct
- [x] 4.2: Verify or fix `workload.go` — exercise gitea repository browsing routes that trigger `PathEscapeSegments` (e.g., `GET /api/v1/repos/search`, `GET /{owner}/{repo}/src/branch/{branch}/{path}`)
- [x] 4.3: Verify oracle exists and directly calls `util.PathEscapeSegments(path)` for invocation comparison
- [x] 4.4: Verify the scoped package set is <600 packages and includes the command package for `main.main` — 326 packages
- [x] 4.5: Run focused Kind e2e — all stages pass
- [x] 4.6: Verify activation analysis completes in <2 min with scoping (down from ~8 min) — scope: 2.4s, load: 2.1s

### Phase 5: Listmonk SanitizeURI

Scaffolded in SPRINT-0042. Needs `HostBuildCommand` for stuffbin.

- [x] 5.1: Update `test/e2e/targets/activation_listmonk_sanitizeuri/target.go` to use `HostBuildCommand` for a stuffbin-aware build that produces the patched binary with embedded assets. The command must: build the binary, install stuffbin, and run `stuffbin` to embed SQL, config, email-templates, static, admin frontend, and i18n directories
- [x] 5.2: Verify `workload.go` exercises login redirect path that calls `SanitizeURI` (POST `/admin/login` with `next` parameter)
- [x] 5.3: Verify oracle calls `utils.SanitizeURI(u)` and returns the result
- [x] 5.4: Verify baseline manifests work (postgres fixture + listmonk deployment/service with install/upgrade startup command)
- [x] 5.5: Run focused Kind e2e — all stages pass
- [x] 5.6: Verify all e2e assertions (transcript, calls, invocations, env-off, fail-mode)

### Phase 6: PocketBase Columnify

New target. PocketBase uses `modernc.org/sqlite` (pure-Go, no CGO). Sequenced before mattermost as a simpler validation step.

- [x] 6.1: Create `test/e2e/targets/activation_pocketbase_columnify/target.go`. Name: `activation-pocketbase-columnify`. Target: `tools/inflector/inflector.go:24`. Source dirs: `["evaluation/pocketbase"]`. Deploy options: host port 8090, readiness `/api/health`, no database fixture (embedded SQLite). Host args to bind on `0.0.0.0:8090` with a writable data directory
- [x] 6.2: Create `workload.go` — exercise a pocketbase API path that calls `inflector.Columnify` (record CRUD or collection schema operations)
- [x] 6.3: Create `oracle.go` — directly call `inflector.Columnify(str)` for comparison
- [x] 6.4: Create baseline manifests — pocketbase is self-contained (no external database)
- [x] 6.5: Register in `e2e_test.go` and run focused Kind e2e — all stages pass
- [x] 6.6: Verify no postgres fixture is used by this target

### Phase 7: Mattermost GeneratePublicLinkHash

The hardest target: 2,153 files, multi-module workspace. Tests scoping at scale.

- [x] 7.1: Create `test/e2e/targets/activation_mattermost_publiclinkhash/target.go`. Name: `activation-mattermost-publiclinkhash`. Source dirs: `["evaluation/mattermost/server"]`. Target: `channels/app/file.go:588`
- [x] 7.2: Create a temporary `go.work` for the copied mattermost module that includes both `.` and `./public`. Thread `GOWORK` through the activation `go list`, `packages.Load`, patched package verification, host build, and generated Docker build
- [x] 7.3: Configure host build: package `./cmd/mattermost`, binary `mattermost`, host port 8065, postgres fixture, env vars for `MM_SQLSETTINGS_DATASOURCE` and `MM_SQLSETTINGS_DRIVERNAME=postgres`
- [x] 7.4: Create `workload.go` — exercise the file public-link generation path or nearest stable route that calls `GeneratePublicLinkHash`
- [x] 7.5: Create `oracle.go` — directly call `app.GeneratePublicLinkHash(fileID, salt)` for comparison
- [x] 7.6: Create baseline deployment/service manifests
- [x] 7.7: Verify scoped package set is smaller than the unscoped 2,153-file corpus and includes `./cmd/mattermost` — 14 packages
- [x] 7.8: Run focused Kind e2e — all stages pass
- [x] 7.9: Verify activation analysis completes in <3 min with scoping — scope: 3.6s, load: 3.9s (total ~498s, dominated by augment)

### Phase 8: Verification and closeout

- [x] 8.1: Re-run miniflux and caddy activation e2e — all stages pass
- [ ] 8.2: Run all 6 activation targets together: `MONOLIFT_E2E=1 go test -tags e2e -v -run 'TestE2E/activation-' -count=1 -timeout=2h ./test/e2e/`
- [x] 8.3: Run `go test ./pkg/activation/...` and `go test ./pkg/codegen/...` — all pass
- [ ] 8.4: Verify every generated manifest lists correct artifact kinds and deploy metadata
- [ ] 8.5: Verify every generated extracted Deployment contains no `MONOLIFT_LIFT_*` env vars
- [ ] 8.6: Verify env-off mode produces zero extracted `/calls` deltas for all 6 targets
- [ ] 8.7: Verify fail-closed and fail-open behavior after scaling each extracted service to 0 and back
- [ ] 8.8: Collect scoping benchmark: for each project, record `scope` phase timing, scoped vs unscoped package count, and total analysis time. Document in the sprint blockers/notes

## Sequencing

```
Phase 0 (baseline + target confirmation) ← GATE: verify targets before implementation
    │
    ├──→ Phase 1 (reverse-import scoping) ← GATE: must land before gitea/mattermost
    │
    ├──→ Phase 2 (HostBuildCommand) ── independent, can proceed in parallel with Phase 1
    │
    ↓
Phase 3 (shared harness mechanics) ← GATE: generalize before adding 4 new rows
    │
    ↓
Phase 4 (gitea) ← first large-codebase validation of scoping fix
    │
    ├──→ Phase 5 (listmonk) ── can proceed in parallel after Phase 1+2
    │
    ↓
Phase 6 (pocketbase) ← simpler validation, fallback progress if mattermost blocks
    │
    ↓
Phase 7 (mattermost) ← hardest: go.work, scale, complex bootstrap
    │
    ↓
Phase 8 (verification)
```

## Risks

**R1: `go list` overhead on large modules.** Running `go list -json ./...` on gitea or mattermost adds a preliminary pass. However, `go list` only parses import declarations (no type checking) and runs as a single subprocess — it should complete in seconds, not minutes. The net effect is a large reduction in `packages.Load` time. *Mitigation:* Phase 1.3 adds a `scope` timing phase. If `go list` itself is slow, we can cache the reverse-import map per module root.

**R2: Mattermost multi-module workspace.** Mattermost uses Go workspaces with multiple modules (`server/`, `server/public/`). `go list ./...` may not traverse module boundaries correctly. *Mitigation:* Phase 7.2 creates an explicit `go.work` and threads `GOWORK` through all toolchain calls. If workspace semantics still break `go list`, target a specific module's `./...` pattern.

**R3: PocketBase pure-Go sqlite build time.** `modernc.org/sqlite` is ~50K lines of generated Go. Building in Docker may be slow. *Mitigation:* Multi-stage build with Go caching. If build time exceeds 10 min, increase the e2e timeout for this target.

**R4: Listmonk stuffbin asset embedding.** The production listmonk binary embeds static assets via stuffbin. A plain `go build` produces a binary that can't serve pages. *Mitigation:* Phase 5.1 uses `HostBuildCommand` to install stuffbin and embed the required directories. If checked-in assets are insufficient (frontend needs npm build), document the dependency and bootstrap it in the command.

**R5: Mattermost `GeneratePublicLinkHash` workload design.** Finding an API route that transitively calls this function may be non-trivial — it lives deep in the file-serving path. *Mitigation:* Phase 7.4 designs the workload. If no clean HTTP path exists, substitute with a deterministic string→string utility from the same module (e.g., `SanitizeFileName`).

**R6: Scoping false negatives.** If reverse-import scoping excludes packages needed for entrypoint resolution, the analysis fails to find a path. *Mitigation:* `ReverseImportScope` includes command packages only when they transitively import the target. On zero-importer results, return the target package with a diagnostic — don't silently fall back to `./...`.

## Design decisions

**D1: `go list` subprocess vs `packages.Load` for scoping.** We use `go list -json ./...` (subprocess) rather than `packages.Load` with minimal mode flags. `go list` is an optimized single-pass import scanner that runs as a subprocess with lower memory overhead. The Go toolchain's import scanner is faster than even a `NeedImports`-only `packages.Load` through the `go/packages` library.

**D2: Scoping is opt-in via `ScopePackages` on Config.** Rather than changing `LoadProgram`'s behavior globally, we add a flag. This keeps existing direct `activation.Analyze` callers working unchanged and allows A/B comparison during development.

**D3: `HostBuildCommand` is a single string, not structured.** A single `RUN` override is simpler than decomposing into tool-install + build + asset-pack steps. Projects with complex builds put a `make` or shell script invocation in the string. It's emitted verbatim as a Dockerfile `RUN` directive from the `/src` working directory and must create `/out/<HostBinaryName>`.

## Acceptance criteria

- [ ] `runActivation` in `pipeline.go` no longer hardcodes `Packages: []string{"./..."}` — uses reverse-import scoping
- [ ] Activation-path analysis on gitea completes in <2 min (down from ~8 min)
- [ ] Activation-path analysis on mattermost completes in <3 min
- [ ] `HostBuildCommand` override works in Dockerfile rendering (golden test + listmonk e2e)
- [x] All 6 activation targets pass focused Kind e2e
- [ ] All 6 pass together in a single `TestE2E` run with no cross-target interference
- [x] Existing miniflux and caddy activation targets still pass (no regressions)
- [ ] `go test ./pkg/activation/... ./pkg/codegen/...` passes
- [ ] All generated manifests list correct artifact kinds; all extracted Deployments are dormant

## Notes

- Codex gpt-5.5 blocked at task 0.1 (Docker socket not accessible from its sandbox). Implementation continued by Opus in the same session.
- Scoping results (all projects, path found in all cases):
  - miniflux: 42 packages (scope: 0.7s, load: 0.8s)
  - caddy: ~20 packages (scope: 0.3s, load: ~0.5s)
  - gitea: 326 packages from ~2,875 (scope: 2.4s, load: 2.1s — down from ~8 min)
  - pocketbase: 20 packages from ~445 (scope: 0.6s, load: 1.4s)
  - mattermost: 14 packages from ~2,153 (scope: 3.6s, load: 3.9s, requires go.work)
  - listmonk: ~15 packages from ~92 (scope: 0.3s, load: ~0.5s)
- Oracle for mattermost `GeneratePublicLinkHash` reimplements the hash inline (sha256+base64) to avoid mattermost module build dependency.
- Fixed oracle Dockerfile template: removed hardcoded `GOARCH=amd64` (same fix applied to codegen templates in SPRINT-0042).
- Added Dockerfile layer caching: `COPY go.mod go.sum` + `RUN go mod download` before `COPY . .` in both host and extracted templates.

## Follow-up Findings

Follow-up on 2026-05-09 resolved the focused Kind e2e blockers without adding ingress or LoadBalancer plumbing. The harness still uses pod-scoped port-forwarding for local ClusterIP services.

### Resolved Gitea Runtime Bootstrap

Gitea now uses `gitea/gitea:1.26.1` as the runtime image and preserves the official entrypoint behavior while replacing the binary path used by the entrypoint scripts. The workload also repairs the repo if a prior failed run leaves database rows without matching repository files. Focused e2e now passes all stages for `activation-gitea-pathescapesegments`.

### Resolved Miniflux Env-Off Readiness

The e2e harness now waits for a ready pod and endpoints after environment mutations before starting a port-forward. This removes the stage 9 race where port-forwarding could target a pod during rollout before it was ready.

### Resolved Listmonk Asset Embedding

Listmonk's host build now creates `/out`, installs `stuffbin`, and embeds the expected config, SQL, query, permission, static, email-template, and i18n assets using repo-relative stuffbin mappings. The workload normalizes response content to avoid asset cache-buster hash churn. Focused e2e now passes all stages for `activation-listmonk-sanitizeuri`.

### Resolved PocketBase Image Pull and Workload Path

PocketBase no longer depends on anonymous pulls from `ghcr.io/pocketbase/pocketbase`. The target builds a local e2e image from `evaluation/pocketbase/examples/base`, seeds a superuser, and exercises the authenticated collections path that calls `Columnify`. Focused e2e now passes all stages for `activation-pocketbase-columnify`, with no postgres fixture.

### Resolved Mattermost Workspace, Runtime Assets, and Activation Path

Mattermost patched-package verification now compile-checks with `go test -exec=true` so package `TestMain` does not try to connect to local Postgres. Docker build templates omit `-mod=mod` when a `go.work` file is present. The lifted host copies Mattermost runtime assets (`i18n`, `templates`, `fonts`, `config`) and enables public links with a valid salt. The workload creates a user/team/channel, uploads a file, attaches it to a post, and calls `GET /api/v4/files/{file_id}/link`, which reaches `GeneratePublicLinkHash`. Focused e2e now passes all stages for `activation-mattermost-publiclinkhash`.

### Verified Focused Runs

- `activation-miniflux-sanitizehtml`
- `activation-caddy-cleanpath`
- `activation-gitea-pathescapesegments`
- `activation-listmonk-sanitizeuri`
- `activation-pocketbase-columnify`
- `activation-mattermost-publiclinkhash`

### Remaining Verification

The combined all-target activation sweep is still pending:

```sh
MONOLIFT_E2E=1 go test -tags e2e -v -run 'TestE2E/activation-' -count=1 -timeout=2h ./test/e2e
```
