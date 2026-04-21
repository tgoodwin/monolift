# SPRINT-0004 — Monolift v2 E2E Test Harness

**Status:** planned · **Scope:** test infrastructure only, no v2 compiler code
**Primary deliverable:** `test/e2e/` Go harness + make targets + Caddy green + Pocketbase refusal green
**Primary input:** `docs/specs/e2e-test-strategy.md` v1.0 (the strategy this sprint implements)
**Prerequisite for:** SPRINT-0005+ (v2 compiler implementation epics)

---

## Why this sprint exists

The v2 contract spec (SPRINT-0003) is content-complete at v1.0. Implementation
of the v2 compiler could start — but agents implementing compiler code have
no feedback loop without an e2e harness. They would write SSA extraction,
canonical-shape classification, state-class inference etc. without any way
to see whether their changes work against a real Go monolith.

The e2e test strategy (`docs/specs/e2e-test-strategy.md`) specifies the
harness. This sprint **builds** that harness — with a stub compiler standing
in for the real v2 compiler so the harness can go green *before* any v2
compiler code is written.

When this sprint closes, a coding agent can run `make e2e`, see
red-vs-green signal per target, observe `[stage=N target=X kind=(...)]`
diagnostics, and iterate. SPRINT-0005+ replaces the stub compiler with real
compiler epics, flipping skipped/stubbed targets to green as each epic
lands.

## Goals

- `test/e2e/` Go harness that runs `make e2e` end-to-end against Kind.
- Caddy target fully green through all 10 stages (against stub compiler).
- Pocketbase refusal target green through stages 0–4 (compile + report assertion).
- Miniflux target scaffolded (Postgres sidecar manifests, workload stub), `t.Skip`ped.
- Listmonk / Gitea / Mattermost declared with expected verdicts, `t.Skip`ped.
- Stub compiler lives under `test/e2e/stubcompiler/` (test-only; not polluting `pkg/`).
- `MONOLIFT_E2E=1`, `MONOLIFT_E2E_KEEP=1`, `MONOLIFT_COMPILER=<path>` env gates all functional.
- Closure-report Go struct under `pkg/compiler/reportv2/` + JSON Schema validator.
- README covering: run, interpret-failures, add-target, update-goldens.

## Non-goals

- **No v2 compiler implementation.** That's SPRINT-0005+. This sprint uses a stub.
- No changes under `pkg/compiler/{compiler,pragma,artifacts,manifests,util}.go` (v1 stays frozen).
- No changes under `pkg/lift/*` (v1 codegen stays).
- No changes under `pkg/pragma/` or `pkg/metrics/` (runtime untouched).
- No changes under `demo/monolith/*` (the existing demo keeps working).
- No CI integration / GitHub Actions (that's SPRINT-0005+).
- No performance regression testing (`throughput_test.py` owns that).
- No chaos injection, no multi-cluster, no parallel target execution.
- No full §EC-REPORT equality — normative subset only.

## Prerequisites

- `kind` CLI installed locally (`brew install kind` or equivalent).
- Docker daemon running.
- `kubectl` CLI.
- `go` 1.23.6+.

## Scope boundaries

**In scope:** `test/e2e/` Go package + subpackages + target fixtures; `pkg/compiler/reportv2/` closure-report types; test-only stub compiler at `test/e2e/stubcompiler/`; Makefile additions for e2e targets; per-target baseline k8s manifests (hand-written where upstream doesn't ship them); the Caddy/Pocketbase/Miniflux fixtures; README.

**Out of scope:** anything that implements v2 compiler contract rules; changes to v1 compiler or runtime; changes to the v2 spec itself; performance or security testing.

---

## Tasks

All work is checkboxed. Phases ordered; most tasks within a phase can overlap.

### Phase 0 — Scaffold & prereqs

- [x] Create `test/e2e/` directory with `README.md` stub, `Makefile.include`, `e2e_test.go` skeleton (build tag `e2e`).
- [x] Add `//go:build e2e` to all test files so `go test ./...` from root stays fast.
- [x] Add Makefile targets: `make e2e`, `make e2e-reset`, `make e2e-update-golden`, `make e2e-clean`. Include `test/e2e/Makefile.include` from root Makefile.
- [x] Add `MONOLIFT_E2E=1` gate — unset = all cases `t.Skip("MONOLIFT_E2E=1 required")`.
- [x] Add `MONOLIFT_E2E_KEEP=1` env var — preserves namespaces on failure.
- [x] Add `MONOLIFT_COMPILER=<path>` env var — selects compiler binary (default: stub path).
- [x] Update `.gitignore` for `/tmp/monolift-e2e/` artifact dumps and `test/e2e/targets/*/bin/` builds.

### Phase 1 — Closure-report types & schema

- [x] Create `pkg/compiler/reportv2/report.go` with Go structs for §EC-REPORT normative subset: `Report`, `BuildConfig`, `Analysis`, `Pragma`, `Root`, `Closure`, `StateItem`, `Adapter`, `ExternalDep`, `Pruning`, `Diagnostic`.
- [x] Generate JSON Schema fragment from the Go structs (via `jsonschema` tag annotations) OR write it inline; commit to `pkg/compiler/reportv2/schema.json`.
- [x] Add `reportv2.Validate(data []byte) error` that validates JSON against the schema.
- [x] Add `reportv2.Parse(data []byte) (*Report, error)` that unmarshals + validates.
- [x] Unit test in `pkg/compiler/reportv2/report_test.go`: parse 1 valid accept-verdict report, 1 valid refuse-verdict report, 1 invalid (missing required field).
- [x] Document: "This is the normative v1.0 schema per docs/specs/monolift-v2-contract.md §EC-REPORT. Additions are backwards-compatible; renames require schemaVersion bump."

### Phase 2 — Kind cluster lifecycle (`test/e2e/harness/cluster.go`)

- [x] `Cluster.Ensure(ctx) error` — create Kind cluster `monolift-e2e` from copy of `demo/k8s/kind-config.yaml` if absent; no-op if present and healthy.
- [x] `Cluster.Reset(ctx) error` — destroy + recreate (invoked by `make e2e-reset`).
- [x] `Cluster.LoadImage(ctx, imageRef) error` — wraps `kind load docker-image` with logging.
- [x] `Cluster.WaitNodesReady(ctx, timeout) error` — polls via client-go, not `kubectl wait`.
- [x] Integration test in `harness/cluster_test.go` (behind `e2e` tag): ensure → load a tiny image → teardown.

### Phase 3 — Stub compiler (`test/e2e/stubcompiler/`)

- [x] `main.go` implementing CLI contract: `stubcompiler --target=<name> --output=<dir>` → writes `closure-report.json` + generated manifests under `<dir>`.
- [x] Per-target fixtures: `fixtures/caddy/` (accept verdict report + extracted service manifests), `fixtures/pocketbase/` (refuse verdict, no manifests), `fixtures/miniflux/` (accept report, Postgres-aware manifests).
- [x] Fixture format: mirror the exact normative-subset JSON shape that the real v2 compiler would emit for each target.
- [x] Integration test in `stubcompiler_test.go`: invoke stub for each target, verify report parses via `reportv2.Validate`.
- [x] Add `make build-stubcompiler` target that compiles to `./bin/stubcompiler`. Harness defaults `MONOLIFT_COMPILER` to this path.

### Phase 4 — Compile seam (`test/e2e/harness/compiler.go`)

- [x] `Compiler.Run(ctx, target) (CompileResult, error)` — shell out to `$MONOLIFT_COMPILER --target=<name> --output=<dir>`; captures stdout/stderr/exit code.
- [x] `CompileResult` exposes: artifacts dir, parsed `reportv2.Report`, raw stderr, exit code.
- [x] Failure formatter: `[stage=3 target=X kind=compiler] compile exit=N verdict=got_V want_W stderr: <tail>`.

### Phase 5 — Image build + kind load (`test/e2e/harness/imagebuild.go`)

- [x] `ImageBuilder.Build(ctx, dockerfile, contextDir, tag) error` — shells `docker build`; captures build log tail on failure.
- [x] `ImageBuilder.LoadToKind(ctx, tag) error` — wraps `Cluster.LoadImage`.
- [x] Cache: skip rebuild if `target source sha + Dockerfile hash` unchanged since last build (write to `/tmp/monolift-e2e/.cache/`).
- [x] Failure format: `[stage=5|6 target=X kind=artifact] docker build failed: <last 20 lines>` / `kind load failed for image <tag>`.

### Phase 6 — Deployer (`test/e2e/harness/deployer.go`)

- [x] `Deployer.Apply(ctx, ns, manifests []string) error` — client-go apply with server-side apply.
- [x] `Deployer.WaitReady(ctx, ns, timeout) error` — poll pod status until all Ready or timeout; dump describe + logs on fail.
- [x] `Deployer.CreateNamespace(ctx, ns)` — idempotent.
- [x] `Deployer.DeleteNamespace(ctx, ns, timeout)` — respects `MONOLIFT_E2E_KEEP` env var; no-op if set.
- [x] Namespace naming: `mlv2-baseline-<target>-<runid>`, `mlv2-lifted-<target>-<runid>`. Runid = nanosecond timestamp captured once per `TestE2E` run.

### Phase 7 — WorkloadExecutor interface (`test/e2e/harness/workload.go`)

- [x] Define `WorkloadExecutor` interface: `Setup(ctx, host) error`, `Action(ctx, host) (Transcript, error)`, `Verify(ctx, host, expected Transcript) error`.
- [x] Define `Transcript` struct: ordered `[]Step{Method, Path, Status, Headers, BodyJSON}`; normalization hooks for timestamps/IDs.
- [x] `Workload.RunBoth(ctx, baselineURL, liftedURL, exec)` — runs Setup+Action on both, returns paired transcripts.
- [x] `Transcript.Compare(baseline, lifted, invariants []Invariant)` — structured diff; invariants declare which fields matter (status vs body shape vs persisted state).

### Phase 8 — Report comparator & verdict assertor (`test/e2e/harness/report.go`, `verdict.go`)

- [x] `Report.CompareNormativeSubset(golden, got *Report) error` — compare only the normative fields from strategy doc §Golden-file discipline (schemaVersion, analysis.algorithm, root identity, pragma.verdict, closure.boundedPruning, state[].disposition, adapters[].kind, externalDependencies[].access_path, diagnostics[].code). Return structured diff.
- [x] `Verdict.AssertAccept(report *Report)` — verdict is "accept" and diagnostics contain no refusal codes.
- [x] `Verdict.AssertRefuse(report *Report, required []DiagnosticCode)` — verdict is "refuse-blocking" and ALL listed diagnostics fire.
- [x] `-update-golden` flag hook in `e2e_test.go`: regenerates golden JSON from current-run `report.json`; prints diff; requires manual commit.

### Phase 9 — Caddy target (first complete positive)

- [x] `test/e2e/targets/caddy/target.go` — declares `TargetCase{Name: "caddy", ExpectedVerdict: "accept", StopAtStage: 10, ...}`.
- [x] `test/e2e/targets/caddy/Dockerfile` — build Caddy binary from `evaluation/caddy/` + minimal config (no upstream Dockerfile exists).
- [x] `test/e2e/targets/caddy/baseline/deployment.yaml`, `service.yaml`, `caddyfile-configmap.yaml` — reverse proxy + static file serving.
- [x] `test/e2e/targets/caddy/baseline/echo-upstream.yaml` — tiny echo server that Caddy proxies to.
- [x] `test/e2e/targets/caddy/workload.go` — implements `WorkloadExecutor`. Sequence: `GET /static/hello.txt` (static), `GET /proxy?x=1` (reverse proxy), `GET /headers` (asserts injected X-Caddy header). Transcript asserts status + selected headers + body.
- [x] `test/e2e/targets/caddy/golden/report.json` — expected closure report: verdict=accept, root identity=`caddy.Module`-style, adapters include registry+handler kinds, bounded-pruning true.
- [x] Stub compiler fixture at `fixtures/caddy/` emits this exact golden report + extracted service manifests under `lifted/`.
- [x] Integration test: all 10 stages pass against stub compiler.

### Phase 10 — Pocketbase target (refusal)

- [x] `test/e2e/targets/pocketbase/target.go` — `TargetCase{Name: "pocketbase", ExpectedVerdict: "refuse-blocking", StopAtStage: 4, RequiredDiagnostics: []{"MLV2_EMBEDDED_DB_APP_ROOT", "MLV2_CLOSURE_TOO_LARGE"}}`.
- [x] No baseline deploy needed by default (skip if `MONOLIFT_E2E_POCKETBASE_BASELINE=1` unset).
- [x] `test/e2e/targets/pocketbase/golden/report.json` — expected refusal report with both required diagnostics.
- [x] Stub compiler fixture emits the refusal report.
- [x] Integration test: stages 0→3 run, stage 4 asserts verdict + both diagnostics present, pipeline exits clean (stages 5-10 skipped).

### Phase 11 — Miniflux scaffold (Postgres sidecar; `t.Skip` active)

- [x] `test/e2e/fixtures/postgres.yaml` — shared Postgres statefulset + service (16Mi RAM, tiny).
- [x] `test/e2e/targets/miniflux/target.go` — `TargetCase{Name: "miniflux", ExpectedVerdict: "accept", t.Skip("deferred pending v2 compiler FeedProcessor lift — SPRINT-0005")}`.
- [x] `test/e2e/targets/miniflux/baseline/*.yaml` — Miniflux deployment + Postgres dependency declaration. Committed but not exercised until skip removed.
- [x] `test/e2e/targets/miniflux/workload.go` — workload stub. Not compiled or run while `t.Skip` active.
- [x] `test/e2e/fixtures/rss-feed-server.yaml` — in-cluster RSS fixture pod; stubbed with deterministic XML.
- [x] `test/e2e/targets/miniflux/golden/report.json` — expected accept report for feed-fetcher lift. Placeholder; sharpened in SPRINT-0005.

### Phase 12 — Remaining targets (Listmonk, Gitea, Mattermost) declared & skipped

- [x] `test/e2e/targets/listmonk/target.go` — `TargetCase{ExpectedVerdict: "accept-with-state-rules", t.Skip("deferred")}` with expected-diagnostics empty + expected root `CampaignWorker`.
- [x] `test/e2e/targets/gitea/target.go` — `TargetCase{ExpectedVerdict: "accept-mailer-subset", t.Skip("deferred")}` with expected root `MailerService`.
- [x] `test/e2e/targets/mattermost/target.go` — `TargetCase{ExpectedVerdict: "accept-UserService", t.Skip("deferred")}` with expected root `UserService`.
- [x] Each target declaration cites the corresponding spec §Cross-Target Validation subsection for traceability.

### Phase 13 — Make targets, README, handoff

- [x] `make e2e` → `MONOLIFT_E2E=1 go test -tags=e2e -v ./test/e2e/... -timeout=30m`.
- [x] `make e2e-reset` → destroys Kind cluster; next `make e2e` recreates it.
- [x] `make e2e-update-golden` → runs with `MONOLIFT_E2E_UPDATE_GOLDEN=1`; prints diffs; exits non-zero if any golden changed (forces human review).
- [x] `make e2e-clean` → removes `/tmp/monolift-e2e/*`.
- [x] `test/e2e/README.md` covering: prereqs (kind, docker, kubectl), one-command run, env vars, failure-message taxonomy, how to add a target, how to update goldens, how to debug a failing target.
- [x] Dry-run: execute `make e2e` from a clean checkout + clean Kind state; all 6 target rows run; Caddy + Pocketbase green; Miniflux/Listmonk/Gitea/Mattermost skip cleanly.

### Phase 14 — Handoff to SPRINT-0005

- [x] At bottom of this sprint file, add **## SPRINT-0005 Seed Epics** section listing the six v2 compiler epics (from SPRINT-0003's original handoff). Each epic references the specific harness target that will flip from red to green when the epic lands.
- [x] Commit the final harness + strategy doc + sprint close notes.
- [x] Update `docs/evolution.md` with harness-sprint-closed entry.

---

## Sequencing

Strict: **Phase 0 → 1 → {2, 3, 4, 5, 6, 7, 8 parallel} → {9, 10 parallel} → 11 → 12 → 13 → 14.**

Phase 1 (closure-report types) blocks Phases 4 (compile seam), 8 (report comparator), 3 (stub compiler output format). Everything else can overlap. Phase 9 (Caddy) and Phase 10 (Pocketbase) are parallelizable by different agents once scaffolding is in place.

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| Caddy has no upstream Dockerfile | Write minimal one in `test/e2e/targets/caddy/Dockerfile` (Caddy is a single Go binary — trivial) |
| Pocketbase baseline deploy complexity | Make it opt-in (`MONOLIFT_E2E_POCKETBASE_BASELINE=1`); default skip |
| Kind + docker on Apple silicon flakiness | Cluster reuse; retry-once on wait-for-Ready; `make e2e-reset` documented |
| `kind load` slow (~30s per image) | Dedicated stage 6 failure signal; sha-based cache |
| Stub compiler diverging from what real v2 will emit | Ground stub output in the exact v2 spec §EC-REPORT format; any schema change breaks both sides loudly |
| Scope creep into real compiler work | Non-goals explicitly exclude `pkg/compiler/*`; reviewer gate |
| Golden-file thrash | Normative-subset only; `-update-golden` gated |
| Agent confused whose fault a failure is | `[stage=N target=X kind=(harness|compiler|artifact|workload)]` prefix on every failure |
| Miniflux Postgres sidecar churn | Keep as `t.Skip` this sprint; sharpen when SPRINT-0005 starts |
| `reportv2` struct in `pkg/compiler/` implies compiler code | Document: struct is shared type between harness and compiler; belongs in `pkg/compiler/reportv2/` for import convenience, not because v2 compiler code exists yet |

## Acceptance criteria

- [x] `MONOLIFT_E2E=1 make e2e` runs without panic and completes in ≤10 minutes on a 16GB M-series Mac.
- [x] Caddy target: all 10 stages pass against stub compiler output.
- [x] Pocketbase target: stages 0–4 pass; asserts both `MLV2_EMBEDDED_DB_APP_ROOT` + `MLV2_CLOSURE_TOO_LARGE` present.
- [x] Miniflux target: `t.Skip` active with explicit SPRINT-0005 pointer; Postgres sidecar manifest committed.
- [x] Listmonk, Gitea, Mattermost: `t.Skip` active; verdict + expected root declared.
- [x] `pkg/compiler/reportv2/` compiles; `Report` struct matches §EC-REPORT normative subset; JSON Schema validates.
- [x] Stub compiler compiles to `./bin/stubcompiler`; invocation for all 3 active targets produces valid reports.
- [x] `make e2e-reset` destroys and recreates Kind cluster successfully.
- [x] `make e2e-update-golden` regenerates goldens with visible diff and non-zero exit on change.
- [x] `MONOLIFT_E2E_KEEP=1` preserves namespaces on failure.
- [x] Every failure message carries `[stage=N target=X kind=...]` prefix.
- [x] `test/e2e/README.md` covers prereqs, run, interpret, add-target, update-goldens, debug.
- [x] No changes under `pkg/compiler/{compiler,pragma,artifacts,manifests,util}.go`, `pkg/lift/*`, `pkg/pragma/`, `pkg/metrics/`, or `demo/*` (v1 untouched).
- [x] This sprint file ends with a `## SPRINT-0005 Seed Epics` section.

---

## SPRINT-0005 Seed Epics

- **SSA-based extraction pass** consuming root declarations and emitting the
  §EC-REPORT v1.0 closure report. Harness flip: Pocketbase refusal report
  moves from stub to real first, then Miniflux report generation.
- **Canonical-shape signature classifier + per-shape adapter templates.**
  Harness flip: Caddy handler/registry adapter golden moves from stub to real;
  unlocks Gitea mailer and Mattermost service method shape rows.
- **State-class inference + singleton/affinity deployable codegen.** Harness
  flip: Miniflux Postgres-backed state dispositions move from placeholder to
  real; enables Listmonk state-rules acceptance.
- **v2 pragma parser and attachment validator.** Harness flip: all active
  reports get real `pragma` options/spans; skipped Listmonk/Gitea/Mattermost
  rows can validate expected roots without fixture-only declarations.
- **Refusal-diagnostic framework with source spans and remediation text.**
  Harness flip: Pocketbase required diagnostics
  `MLV2_EMBEDDED_DB_APP_ROOT` + `MLV2_CLOSURE_TOO_LARGE` come from the real
  compiler instead of the stub fixture.
- **End-to-end Miniflux smoke test.** Harness flip: remove Miniflux `t.Skip`,
  exercise Postgres + RSS fixtures through stages 0–10, and compare baseline
  versus lifted feed-processing transcripts.
- **v2 pragma parser with EBNF-validated keys** (including `x-*` reserved prefix, `Doc`-comment attachment, `MLV2_PRAGMA_V1_DEPRECATED` warning on v1 syntax).
- **Refusal-diagnostic framework** with every §Refusal Diagnostic Index entry raisable at compile time. Flips: pocketbase from stub-assertion to real-compiler-raised.
- **End-to-end miniflux smoke via real compiler** — removes miniflux `t.Skip`; real compiler emits the closure report that matches miniflux golden.

Each epic should land as its own ADR once implementation decisions are made.
