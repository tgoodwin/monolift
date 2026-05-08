# SPRINT-0042: Deployment artifacts & activation-path k8s e2e

**Status:** planned
**Predecessors:** SPRINT-0041 (codegen pipeline), ADR-0029 (codegen architecture), existing Kind e2e harness

## Intent

Extend `monolift lift` to emit deployment artifacts — Dockerfiles and Kubernetes deployment/service YAML for both the extracted service and the patched host — alongside the Go source it already produces, all in a single command invocation. Then validate the full activation-path lift pipeline end-to-end in Kind with 3 stateless targets across 3 different projects, starting with miniflux `SanitizeHTML`, then caddy `CleanPath`, then gitea `PathEscapeSegments`. New targets are new rows in the existing `TestE2E` table, reusing the Kind harness infrastructure.

## What exists today

`RunLift` in `pkg/codegen/pipeline.go` orchestrates: activation-path analysis → `AnalyzeCut` → extract report → `AdmitCut` → `BuildPlan` → `attachIncomingCall` → `applyLiftOptions` → `AdmitPlan` → `RenderServer` → `RenderClient` → `WriteArtifacts`. Current output:

| Artifact | Path pattern | Kind |
|---|---|---|
| Server main.go | `{output}/cmd/{service}/main.go` | `server` |
| Client stub | `{pkgDir}/monolift_lift_{ENV_SERVICE}.go` | `client_stub` |
| Manifest JSON | `{output}/monolift_lift_manifest.json` | — |

The activation-path server exposes `POST /invoke`, `GET /healthz`, and `MONOLIFT_HTTP_ADDR`. The client stub is gated by `MONOLIFT_LIFT_<SERVICE>=on`, reads `MONOLIFT_<SERVICE>_ENDPOINT`, and fail-opens unless `MONOLIFT_LIFT_FAILMODE=closed`.

The v1 compiler (`pkg/compiler/transport/emit/httpjson/templates/`) has Go templates for `dockerfile.tmpl`, `deployment.yaml.tmpl`, and `service.yaml.tmpl` that target the v1 emitter's data model. These serve as reference patterns but are not reusable directly — the activation-path codegen needs its own templates consuming `Plan` fields.

The e2e harness in `test/e2e/` is Kind-backed. `TargetCase` supports `LiftedHostBuild` (patched monolith) and `LiftedExtractedServices` (extracted servers) with per-service Dockerfiles, deployment YAMLs, service YAMLs. The harness expects extracted services to expose `/calls` and `/invocations` and logs containing `LIFT_INVOKE`. The existing miniflux target tests the v1 compiler's `EstimateReadingTime` extraction.

## Concrete targets

### Target 1: miniflux SanitizeHTML

```go
// evaluation/miniflux/internal/reader/sanitizer/sanitizer.go:217
func SanitizeHTML(baseURL, rawHTML string, sanitizerOptions *SanitizerOptions) string
```

- **Boundary params:** `baseURL string`, `rawHTML string`, `sanitizerOptions *SanitizerOptions` — all JSON-serializable
- **Return:** `string` — primitive
- **State:** None (pure function)
- **Callers:** 4 production call sites in miniflux reader
- **Why:** Exercises struct-pointer parameter serialization (`*SanitizerOptions`), HTML processing. Miniflux already has mature baseline manifests and workloads in the e2e harness.

### Target 2: caddy CleanPath

```go
// evaluation/caddy/modules/caddyhttp/caddyhttp.go:279
func CleanPath(p string, collapseSlashes bool) string
```

- **Boundary params:** `p string`, `collapseSlashes bool` — primitives
- **Return:** `string` — primitive
- **State:** None
- **Callers:** 6 production call sites
- **Why:** Already proven in v1 e2e as `monolift-extracted-cleanpath`. Provides a direct comparison between v1 compiler output and activation-path lift output. Mature Kind workload and oracle exist.

### Target 3: gitea PathEscapeSegments

```go
// evaluation/gitea/modules/util/url.go:12
func PathEscapeSegments(path string) string
```

- **Boundary params:** `path string` — primitive
- **Return:** `string` — primitive
- **State:** None (URL escaping utility)
- **Callers:** 82 — the most-called utility in the gitea evaluation corpus
- **Why:** Maximally simple signature; stress-tests the activation-path finder with many potential entry points. Different project from miniflux and caddy.

## Scope boundaries

**In scope:**
- `DeployOptions` struct and deploy-related fields on `Plan`
- `RenderDockerfile` and `RenderKubernetes` in `pkg/codegen/` producing both host and extracted artifacts
- Server observability endpoints (`/calls`, `/invocations`, `LIFT_INVOKE` logs) for e2e harness compatibility
- Extending `RunLift` to collect and write infra artifacts through `WriteArtifacts`
- Extending `Manifest` to record new artifact kinds
- 3 new activation-path e2e targets as rows in the existing `TestE2E` table
- Golden-file tests for all new templates
- CLI flags for deploy artifact generation

**Out of scope:**
- gRPC, streaming, or non-HTTP transports
- Multi-service lifts (one cut point per invocation)
- Helm charts, Kustomize overlays, operators, ingress, HPA
- State-reconstruction targets (stateless only this sprint)
- CI/CD integration, production resource limits
- Replacing the v1 compiler templates or modifying existing v1 e2e targets
- Adding `sigs.k8s.io/yaml` or other new k8s dependencies — use `text/template` with golden-file tests

## Task list

### Phase 1: Plan shape and deploy contract

Stabilize the data model before any renderer touches it.

- [x] 1.1: Add `DeployOptions` struct to `pkg/codegen/types.go` with fields: `HostImage`, `ExtractedImage`, `HostServiceName`, `ExtractedServiceName`, `HostPort`, `ExtractedPort` (default 8081), `HostReadinessPath`, `HostBuildPackage`, `HostBinaryName`, `HostEnvVars []EnvVar`, `ImagePullPolicy` (default `IfNotPresent`)
- [x] 1.2: Add deploy path fields to `Plan`: `HostDockerfilePath`, `ExtractedDockerfilePath`, `HostDeploymentPath`, `HostServicePath`, `ExtractedDeploymentPath`, `ExtractedServicePath`
- [x] 1.3: Compute deploy paths in `applyLiftOptions`: Dockerfiles at `{output}/Dockerfile.{host|extracted}-{service}`, manifests at `{output}/manifests/{name}-{deployment|service}.yaml`
- [x] 1.4: Decouple Kubernetes resource names from `EnvServiceName` so `monolift-extracted-sanitizehtml` maps to `MONOLIFT_LIFT_SANITIZEHTML` and `MONOLIFT_SANITIZEHTML_ENDPOINT`
- [x] 1.5: Add CLI flags for deploy options: `--host-image`, `--extracted-image`, `--host-service-name`, `--host-build-package`, `--host-binary-name`, `--host-port`, `--host-readiness-path`, repeatable `--host-env`
- [x] 1.6: Validate generated deploy artifact paths remain under `SourceModuleRoot`
- [x] 1.7: Validate Kubernetes resource names are DNS-1123 labels
- [x] 1.8: Unit tests for deploy option defaults, explicit overrides, DNS validation, path validation
- [x] 1.9: Update `GeneratorVersion` constant from `"SPRINT-0041"` to `"SPRINT-0042"`

### Phase 2: Server observability for e2e harness

The existing harness requires `/calls`, `/invocations`, and `LIFT_INVOKE` logs. Without these, no activation-path e2e target can pass transcript comparison.

- [x] 2.1: Add invocation counter and bounded record history to the generated server template in `pkg/codegen/server.go`
- [x] 2.2: Add `GET /calls` endpoint returning `{"count": N}` — number of `POST /invoke` calls received
- [x] 2.3: Add `GET /invocations` endpoint returning `{"records": [...]}` — history of request params and results, with invocation IDs
- [x] 2.4: Add `LIFT_INVOKE service=<name>` structured log line on each invocation
- [x] 2.5: Preserve existing `POST /invoke`, `GET /healthz`, and `MONOLIFT_HTTP_ADDR` contract
- [x] 2.6: Update server golden tests and the network round-trip test (existing `SanitizeHTML` httptest test) to verify `/calls` increments and `/invocations` recording

### Phase 3: Dockerfile rendering

Both the patched host and the extracted service need Dockerfiles.

- [x] 3.1: Create `pkg/codegen/docker.go` with `RenderDockerfiles(plan *Plan) (map[string][]byte, error)` returning both extracted and host Dockerfiles keyed by output path
- [x] 3.2: Extracted Dockerfile template: multi-stage build with `golang:{goVersion}` builder compiling `./cmd/{service}`, distroless runtime exposing port 8081, entrypoint `/{service}`. Read Go version from the target module's `go.mod` with a fallback to `1.24`
- [x] 3.3: Host Dockerfile template: multi-stage build compiling the configured `HostBuildPackage`, runtime stage exposing `HostPort`, with `HostEnvVars` as ENV directives
- [x] 3.4: Support explicit host asset copies (e.g., caddy's Caddyfile, static assets) via `HostAssetCopies []AssetCopy` on `DeployOptions`
- [x] 3.5: Golden-file test: render Dockerfiles for a SanitizeHTML-shaped plan, compare against `pkg/codegen/testdata/sanitizehtml_dockerfile_extracted.golden` and `sanitizehtml_dockerfile_host.golden`
- [x] 3.6: Unit test: verify `RenderDockerfiles` output contains expected `FROM`, `COPY`, `EXPOSE`, `ENTRYPOINT` with correct service name and Go version substitution

### Phase 4: Kubernetes YAML rendering

- [x] 4.1: Create `pkg/codegen/kubernetes.go` with `RenderKubernetes(plan *Plan) (map[string][]byte, error)` returning deployment + service YAML for both host and extracted, keyed by output path
- [x] 4.2: Extracted Deployment: single replica, labels `app: {ExtractedServiceName}`, container `extracted`, image `{ExtractedImage}`, port 8081, readiness probe `httpGet /healthz :8081 periodSeconds:2`, `imagePullPolicy: IfNotPresent`. Must NOT contain any `MONOLIFT_LIFT_*` env vars (recursion safety)
- [x] 4.3: Extracted Service: ClusterIP, port 8081 named `http`, selector matching deployment labels
- [x] 4.4: Host Deployment: single replica, labels `app: {HostServiceName}`, image `{HostImage}`, port `{HostPort}`, readiness probe on `HostReadinessPath`, env vars including `MONOLIFT_LIFT_<ENV>=on`, `MONOLIFT_LIFT_FAILMODE=closed`, `MONOLIFT_<ENV>_ENDPOINT=http://{ExtractedServiceName}:{ExtractedPort}/invoke`, plus any `HostEnvVars`
- [x] 4.5: Host Service: ClusterIP, port `{HostPort}`, selector matching deployment labels
- [x] 4.6: Golden-file test: render k8s manifests for SanitizeHTML plan, compare against `pkg/codegen/testdata/sanitizehtml_extracted_deployment.yaml.golden`, `sanitizehtml_extracted_service.yaml.golden`, `sanitizehtml_host_deployment.yaml.golden`, `sanitizehtml_host_service.yaml.golden`
- [x] 4.7: Unit test: parse generated YAML and assert kind, apiVersion, metadata.name, selectors, ports, env vars, readiness probes
- [x] 4.8: Verify extracted Deployment YAML never contains string `MONOLIFT_LIFT_` (static assertion)

### Phase 5: Pipeline integration and manifest

- [x] 5.1: Update `RunLift` in `pipeline.go` to call `RenderDockerfiles(plan)` and `RenderKubernetes(plan)` after existing `RenderServer`/`RenderClient`. Collect artifacts with kinds: `dockerfile_extracted`, `dockerfile_host`, `k8s_deployment_extracted`, `k8s_service_extracted`, `k8s_deployment_host`, `k8s_service_host`
- [x] 5.2: Verify Dockerfiles and YAML skip the `withGeneratedHeader` Go-file logic in `writeArtifactFiles` (already gated by `.go` suffix)
- [x] 5.3: Add deploy metadata to the manifest: Kubernetes resource names, image tags, ports, env var prefix, endpoint URL, readiness paths
- [x] 5.4: Add `RunLiftWithResult(ctx, opts) (*LiftResult, error)` returning structured `Report`, `Cut`, `Plan`, `Manifest`, and patched-file path for e2e programmatic consumption. Keep existing `RunLift` as a CLI wrapper
- [x] 5.5: Verify deterministic output: repeated `monolift lift` runs produce byte-identical artifacts
- [x] 5.6: Integration test: run the full pipeline on miniflux SanitizeHTML, verify Go source + Dockerfiles + k8s YAML all written, manifest lists all artifact kinds

### Phase 6: E2e harness activation-path support

- [x] 6.1: Add `ActivationLiftSpec` to `harness.TargetCase` with fields: target `file:line`, service name, deploy options, expected env var prefix, direct invocation probe payload
- [x] 6.2: Update the e2e compile step: when `ActivationLiftSpec` is set, copy the source module into the compile output, call `codegen.RunLiftWithResult` with `--write-monolith-stub` semantics, and write the report from the returned `LiftResult`
- [x] 6.3: Point `TargetCase.LiftedHostBuild` and `LiftedExtractedServices` at the generated Dockerfiles and manifests using paths from the returned manifest
- [x] 6.4: Add target-driven metadata for `invokePayload`, `invocationPayload`, `invocationResult`, and `symbolForService` so activation targets don't require new branches in the existing hardcoded helpers. Keep v1 caddy/miniflux branches unchanged
- [x] 6.5: Verify activation targets participate in the full harness flow: baseline deploy, lifted deploy, transcript comparison, env-off, fail-open/fail-closed, `/calls` deltas, `/invocations` oracle comparison, extracted-service log assertions

### Phase 7: E2e targets

Targets are sequenced: miniflux first (shakes out the pipeline), caddy second (mature workload, direct v1 comparison), gitea third (different project, stress test).

#### 7A: miniflux SanitizeHTML

- [x] 7A.1: Create `test/e2e/targets/activation_miniflux_sanitizehtml/target.go` defining `Target() harness.TargetCase`. Name: `activation-miniflux-sanitizehtml`. Source dirs: `["evaluation/miniflux"]`. Target: `internal/reader/sanitizer/sanitizer.go:217`. Reuse existing postgres fixture and rss-feed-server fixture for baseline
- [x] 7A.2: Configure host build: package `.`, binary `miniflux`, port 8080, readiness `/healthcheck`, env vars `DATABASE_URL`, `RUN_MIGRATIONS=1`, `CREATE_ADMIN=1`, `ADMIN_USERNAME=admin`, `ADMIN_PASSWORD=test123`, `LISTEN_ADDR=0.0.0.0:8080`
- [x] 7A.3: Configure extracted service: name `monolift-extracted-sanitizehtml`, env prefix `SANITIZEHTML`, port 8081
- [x] 7A.4: Create `workload.go` — exercise the miniflux feed-entry retrieval API path that transitively calls `SanitizeHTML` (feed ingestion with HTML payloads). `Setup` creates a user and subscribes to the RSS feed server; `Action` fetches entries and records sanitized content
- [x] 7A.5: Create `oracle.go` — `SymbolInvoker` that directly calls `sanitizer.SanitizeHTML(baseURL, rawHTML, opts)` for result comparison
- [ ] 7A.6: Register in `e2e_test.go`. Run `MONOLIFT_E2E=1 go test -v -run "TestE2E/activation-miniflux-sanitizehtml" ./test/e2e/` until all stages pass
- [ ] 7A.7: Verify: transcript match, `/calls` delta ≥ 1 per request, `/invocations` oracle match, `LIFT_INVOKE` in logs, env-off produces zero extracted calls, fail-closed returns error sentinel, fail-open falls back to local

#### 7B: caddy CleanPath

- [x] 7B.1: Create `test/e2e/targets/activation_caddy_cleanpath/target.go`. Name: `activation-caddy-cleanpath`. Source dirs: `["evaluation/caddy", "test/e2e/targets/caddy"]`. Target: `modules/caddyhttp/caddyhttp.go:279`
- [x] 7B.2: Configure host build: package `./cmd/caddy`, binary `caddy`, port 8080, Caddyfile mount, static asset copies for existing workload compatibility
- [x] 7B.3: Reuse existing caddy baseline manifests (caddyfile-configmap, echo-upstream, deployment, service)
- [x] 7B.4: Reuse or adapt the existing caddy workload pattern. Create `oracle.go` calling `caddyhttp.CleanPath(p, collapseSlashes)` directly
- [ ] 7B.5: Register in `e2e_test.go` and validate all stages

#### 7C: gitea PathEscapeSegments

- [x] 7C.1: Create `test/e2e/targets/activation_gitea_pathescapesegments/target.go`. Name: `activation-gitea-pathescapesegments`. Source dirs: `["evaluation/gitea"]`. Target: `modules/util/url.go:12`
- [x] 7C.2: Configure host build for gitea. Gitea requires a database; reuse the postgres fixture. Create minimal gitea deployment manifest with env vars for database, `GITEA__security__INSTALL_LOCK=true`
- [x] 7C.3: Create `workload.go` — exercise gitea web routes that trigger URL path construction (repository browsing, file views). `PathEscapeSegments` is called on every URL that involves repository paths
- [x] 7C.4: Create `oracle.go` — directly calls `util.PathEscapeSegments(path)` for result comparison
- [ ] 7C.5: Register in `e2e_test.go` and validate all stages
- [ ] 7C.6: If gitea activation-path analysis times out (2,875 Go files, 82 callers — see R1), substitute with listmonk `SanitizeURI` (`internal/utils/utils.go:41`, 92 Go files, simpler graph) or gitea `ShellEscape` (13 callers, shorter paths). Document the blocker

### Phase 8: Verification and closeout

- [x] 8.1: Run `go test ./pkg/codegen/...` — all unit and golden tests pass
- [x] 8.2: Run `go vet ./pkg/codegen/...` — clean
- [ ] 8.3: Run all 3 activation targets together: verify no cross-target interference
- [ ] 8.4: Run existing v1 e2e rows (caddy, miniflux, pocketbase) — no regressions
- [ ] 8.5: Verify `monolift_lift_manifest.json` for each target lists Go, Dockerfile, and k8s artifacts with correct kinds
- [ ] 8.6: Verify extracted Deployments are dormant (no `MONOLIFT_LIFT_*`, direct `/invoke` still works)
- [ ] 8.7: Verify env-off host execution produces zero extracted `/calls` delta
- [ ] 8.8: Verify fail-closed and fail-open behavior after scaling extracted services to 0 and back
- [ ] 8.9: The three passing activation rows cover three different projects

## Sequencing

```
Phase 1 (plan shape) ← GATE: stable deploy contract before renderers
    │
    ├──→ Phase 2 (server observability) ──┐
    │                                      │
    ├──→ Phase 3 (Dockerfiles) ───────────┤
    │                                      ├──→ Phase 5 (pipeline integration)
    └──→ Phase 4 (k8s YAML) ─────────────┘          │
                                                      ↓
                                              Phase 6 (harness support)
                                                      │
                                                      ↓
                              Phase 7A (miniflux) → Phase 7B (caddy) → Phase 7C (gitea)
                                                                              │
                                                                              ↓
                                                                      Phase 8 (verification)
```

Phase 1 must complete first — the deploy contract is the data model everything else consumes. Phases 2–4 can proceed in parallel once Plan is stable. Phase 5 integrates all renderers. Phase 6 wires the harness. Targets are sequential: miniflux first to shake out the pipeline, caddy second because it has mature fixtures, gitea third as the generalizability proof. Phase 8 runs the full verification matrix.

## Risks

**R1: Activation-path timeout on large codebases.** Gitea has 2,875 Go files and `PathEscapeSegments` has 82 callers. The activation-path analyzer may explore many branches and exceed the 120s timeout. *Mitigation:* if gitea times out, substitute listmonk `SanitizeURI` (92 Go files, 10 callers) or gitea `ShellEscape` (13 callers). Task 7C.6 makes the fallback explicit.

**R2: SanitizeHTML's `*SanitizerOptions` pointer parameter.** The struct has one field (`OpenLinksInNewTab bool`), so it's trivially serializable. But the pointer type means codegen must handle nil. *Mitigation:* verify that SPRINT-0041's `CodecJSON` codec handles pointer-to-struct params and that the admission check accepts `*SanitizerOptions`. If not, extend the type mapper.

**R3: E2e harness assumes v1 compiler.** `harness.Compiler` calls the v1 compiler binary. Activation-path targets need `monolift lift` via `RunLiftWithResult`. *Mitigation:* Phase 6 adds `ActivationLiftSpec` to `TargetCase` and a dispatch branch in the compile step. Existing v1 rows are unaffected.

**R4: Workload design for internal functions.** `SanitizeHTML`, `CleanPath`, and `PathEscapeSegments` are not HTTP endpoints — they're called internally. The e2e workload must exercise user-facing routes that transitively call the target function. *Mitigation:* each target's workload.go exercises a known API path (miniflux: feed entry retrieval; caddy: proxied requests; gitea: repository URL construction).

**R5: Dockerfile Go version.** The v1 template uses a future Go version. The activation-path Dockerfile template should use the Go version from the target module's `go.mod`. *Mitigation:* read `go.mod`'s go directive and use it as the builder image tag. Default to `golang:1.24` if parsing fails.

**R6: Generated path validation.** `writeArtifactFiles` rejects paths outside `Plan.SourceModuleRoot`. Dockerfiles and k8s YAML must live under the module root. *Mitigation:* compute all deploy paths relative to the output directory which is under the module root. Unit test path validation for all new artifact types.

**R7: Recursive remote invocation.** If a generated extracted Deployment carries `MONOLIFT_LIFT_*` env vars, it could call itself. *Mitigation:* task 4.2 explicitly excludes lift env vars from extracted Deployments. Task 4.8 adds a static assertion. Task 8.6 verifies at runtime.

**R8: Server/harness contract mismatch.** The activation server from SPRINT-0041 lacks `/calls`, `/invocations`, and `LIFT_INVOKE` logs that the e2e harness expects. *Mitigation:* Phase 2 adds these before any e2e target runs. Phase 2 is sequenced as a prerequisite.

## Acceptance criteria

- [x] `monolift lift` emits Go source, Dockerfiles (host + extracted), k8s YAML (host + extracted deployment/service), and manifest JSON in one command
- [x] All generated deploy artifacts are written under the source module root and pass path validation
- [x] Manifest records every artifact with distinct kinds (`server`, `client_stub`, `dockerfile_extracted`, `dockerfile_host`, `k8s_deployment_extracted`, `k8s_service_extracted`, `k8s_deployment_host`, `k8s_service_host`) and deploy metadata
- [x] Generated extracted Deployments contain no `MONOLIFT_LIFT_*` env vars
- [x] Generated host Deployments contain `MONOLIFT_LIFT_<ENV>=on`, `MONOLIFT_LIFT_FAILMODE`, and `MONOLIFT_<ENV>_ENDPOINT` pointing at the extracted service
- [ ] `activation-miniflux-sanitizehtml` passes all e2e stages in Kind: baseline deploy, lifted deploy, transcript comparison, env-off, fail-mode assertions
- [ ] `activation-caddy-cleanpath` passes all e2e stages
- [ ] A third activation target from gitea (or documented substitute) passes all e2e stages
- [ ] Three passing targets cover three different projects
- [ ] Existing v1 e2e rows (caddy, miniflux) still pass or have documented unrelated failures
- [x] `go test ./pkg/codegen/...` passes including golden-file tests for Dockerfiles and k8s manifests
- [x] Output is deterministic across repeated runs

## Blockers

- 2026-05-07: Resolved 2026-05-08 by using the oracle pod pattern. `test/e2e/targets/activation_miniflux_sanitizehtml/oracle.go` could not directly call `sanitizer.SanitizeHTML` because that symbol is in `miniflux.app/v2/internal/reader/sanitizer`, so the oracle is now generated inside the copied target module under `cmd/monolift-oracle-*`.
- 2026-05-08: Blocked on 7A.6+ runtime validation. `MONOLIFT_E2E=1 go test -tags e2e -v -run "TestE2E/activation-miniflux-sanitizehtml" -count=1 ./test/e2e/` fails in stage 0 before compile/deploy because Kind cannot talk to Docker: `permission denied while trying to connect to the docker API at unix:///Users/tgoodwin/.docker/run/docker.sock`. Codegen tests and e2e compile-only tests pass with `GOCACHE=/tmp/monolift-go-cache`, but Kind e2e validation requires Docker socket access.
