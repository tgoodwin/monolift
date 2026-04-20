---
title: Monolift v2 E2E Test Strategy
status: accepted
version: 1.0
date: 2026-04-19
scope: spec for the e2e test harness that SPRINT-0004 (v2 compiler implementation) iterates against
---

# Monolift v2 E2E Test Strategy

## Purpose

Before SPRINT-0004 (v2 compiler implementation) begins, establish a
Kubernetes-based e2e test harness that coding agents can run with one
command, observe specific-stage failures, iterate, and re-run. The harness is
the primary correctness feedback loop for the v2 compiler.

This is a **strategy doc**, not the sprint plan. The execution plan lives in
`docs/sprints/SPRINT-0004.md` (separately).

## Core decisions

| Decision | Choice | Rationale |
|---|---|---|
| Harness language | **Go** | Shares types with compiler; `go test` integrates with existing `pragma_test.go`; `client-go`/`controller-runtime` are idiomatic; typed closure-report assertions |
| Location | `test/e2e/` | New top-level; `e2e` build tag keeps unit-test runs fast |
| K8s backend | **local Kind** | Already used by `make deploy-demo`; no cloud, no k3d/minikube |
| Image registry | `kind load docker-image` | No external registry |
| Test style | Data-driven; one row per evaluation target | Cheap to add targets |
| Env gate | `MONOLIFT_E2E=1` required | `go test ./...` at root stays cheap |
| Debug escape | `MONOLIFT_E2E_KEEP=1` preserves namespaces on failure | Post-mortem |
| Parallel targets | **Deferred to SPRINT-0005** (serial in v1) | Parallelism masks non-determinism during compiler churn |

## Architecture

```
test/e2e/
├── README.md
├── e2e_test.go                   # table-driven TestE2E; ranges over targets
├── harness/                      # reusable; Go package
│   ├── cluster.go                # Kind lifecycle; reuses demo/k8s/kind-config.yaml
│   ├── compiler.go               # shells out to `monolift compile`; captures stdout/stderr
│   ├── imagebuild.go             # docker build + `kind load docker-image` (cluster: monolift-e2e)
│   ├── deployer.go               # client-go apply + wait-for-ready; per-target namespace
│   ├── workload.go               # WorkloadExecutor interface; HTTP driver
│   ├── report.go                 # closure-report schema validator + golden comparator
│   ├── verdict.go                # refusal-diagnostic assertions
│   ├── compare.go                # baseline-vs-lifted transcript diff
│   └── diagnostics.go            # constants mirrored from pkg/compiler; failure pretty-printer
├── targets/
│   ├── caddy/                    # FIRST positive — no DB
│   │   ├── target.go             # TargetCase declaration
│   │   ├── baseline/*.yaml       # k8s manifests for un-lifted caddy
│   │   ├── golden/report.json    # expected closure report (normative subset)
│   │   └── workload.go           # WorkloadExecutor impl
│   ├── pocketbase/               # negative — refuse-blocking
│   │   ├── target.go
│   │   ├── golden/report.json    # expected refusal: MLV2_EMBEDDED_DB_APP_ROOT + MLV2_CLOSURE_TOO_LARGE
│   │   └── workload.go           # minimal — no lifted deploy; baseline optional
│   ├── miniflux/                 # SECOND positive — Postgres-backed (stretch for SPRINT-0004)
│   │   ├── target.go
│   │   ├── baseline/*.yaml       # includes postgres sidecar
│   │   ├── golden/report.json
│   │   └── workload.go
│   ├── listmonk/  gitea/  mattermost/   # scaffold + t.Skip("deferred to SPRINT-0005")
│   │   └── target.go             # still declares expected verdict + diagnostics
├── fixtures/
│   ├── kind-config.yaml          # symlink or copy of demo/k8s/kind-config.yaml
│   ├── postgres.yaml             # shared PG for miniflux/listmonk/mattermost
│   └── rss-feed-server.yaml      # in-cluster RSS fixture for miniflux workload
└── Makefile.include              # make e2e, make e2e-reset, make e2e-update-golden
```

The harness shells out to the `monolift` CLI (not in-process) to exercise the
developer entrypoint, stderr diagnostics, and artifact paths together. The
closure-report Go struct under `pkg/compiler/reportv2/` is the shared source
of truth between compiler and harness; the §EC-REPORT JSON Schema validates
it in CI.

**Target table source of truth:** `evaluation/MANIFEST.yaml` (already pins
SHAs for all 6 targets). Harness reads the manifest; duplicate pin-files per
target are prohibited.

## Staging table

Each target row runs these stages. Refusal targets (pocketbase) exit cleanly
after stage 4.

| # | Stage | Assertion | Failure signal |
|---|---|---|---|
| 0 | **Setup** | Kind cluster `monolift-e2e` exists (create if absent); nodes Ready ≤60s; namespaces created | `kind cluster monolift-e2e not ready: worker NotReady after 60s` |
| 1 | **Baseline deploy** | `mlv2-baseline-<target>` ns: all pods Ready; health endpoint 200 | `baseline deploy failed: deployment/caddy unavailable`; dumps `kubectl describe` + pod logs |
| 2 | **Baseline workload** | WorkloadExecutor.Setup + Action succeeds against baseline; transcript recorded | `baseline workload failed at step create-feed: HTTP 500`; request/response log |
| 3 | **Compile** | `monolift compile <target>` exit 0 iff verdict≠refuse; stderr contains expected refusal codes iff refuse | `compile verdict mismatch for pocketbase: got accept, want refuse-blocking with MLV2_EMBEDDED_DB_APP_ROOT + MLV2_CLOSURE_TOO_LARGE` |
| 4 | **Report** | Closure report exists; schema-valid per §EC-REPORT; normative-subset fields match golden | unified JSON diff on normative subset; artifact path; `-update-golden` hint. **Pocketbase terminates here on PASS** |
| 5 | **Artifact build** | Docker build of extracted service images + rewritten entry | build log tail; image name; Dockerfile path |
| 6 | **Image load** | `kind load docker-image` for each built image; images present in Kind node | `crictl images` dump from Kind node (this is the #1 flaky step) |
| 7 | **Lifted deploy** | `mlv2-lifted-<target>` ns: all pods Ready | describe + logs |
| 8 | **Lifted workload** | Same WorkloadExecutor against lifted URL; transcript recorded | as stage 2 |
| 9 | **Compare** | Baseline transcript ≡ lifted transcript on declared invariants (status codes, response-shape, persisted-state readback). Timestamps/IDs normalized | side-by-side response diff, first divergent step |
| 10 | **Cleanup** | Namespaces deleted unless `MONOLIFT_E2E_KEEP=1`; cluster persists | warnings only |

**Failure categorization for agents:** each message prefixes with
`[stage=N target=X kind=(harness|compiler|artifact|workload)]` so the agent
can immediately see *whose fault* the failure is, not just *where* it hit.

## Workload interface

```go
type WorkloadExecutor interface {
    Setup(ctx context.Context, host string) error                        // arrange
    Action(ctx context.Context, host string) (Transcript, error)         // act
    Verify(ctx context.Context, host string, expected Transcript) error  // assert
}
```

The Setup/Verify split matters for stateful targets where arrange and assert
need different lifetimes. Transcripts are structured request/response logs;
the comparator diffs them.

### Per-target workloads

- **Caddy** (first positive, no DB): deploy an echo upstream pod + Caddy
  reverse proxy. Sequence: `GET /static/hello.txt` (static file), `GET /proxy?x=1`
  (reverse proxy to echo), `GET /headers` (asserts injected header). Assert
  status, selected headers, body bytes. Exercises the `accept-static-subset`
  + registry-keyed module path from the v2 contract.
- **Pocketbase** (refusal): pipeline terminates at stage 4. Baseline deploy
  optional (skip by default); workload is synthetic closure-report assertion
  asserting both `MLV2_EMBEDDED_DB_APP_ROOT` **and** `MLV2_CLOSURE_TOO_LARGE`
  fire on the `core.App` root.
- **Miniflux** (second positive, Postgres-backed, stretch): deploy Postgres
  sidecar + Miniflux. Sequence: wait for `/healthz`, seed admin via init job,
  `POST /v1/feeds {feed_url: <in-cluster fixture>}`, poll
  `GET /v1/entries?feed_id=…` until N≥1, compare entry bodies across
  namespaces. Fixture RSS is served by an in-cluster pod, not an host-side
  `httptest.Server` (Kind pods can't always reach the host).
- **Listmonk / Gitea / Mattermost** (deferred): declared with expected
  verdict + expected diagnostics, but `t.Skip("deferred to SPRINT-0005")`.

## Initial target scope (SPRINT-0004)

**Caddy (complete) + Pocketbase (refusal) + Miniflux (scaffold, stretch for green).**

Rationale:
- Caddy has no database, no auth, no admin seed — the shortest path from
  baseline-deploy to a meaningful functional-equivalence check. It exercises
  the contract's registry-keyed-module + handler-adapter path.
- Pocketbase is the cheapest refusal-branch coverage: no deploy needed; all
  work lives in compile + report-assertion. Validates that the compiler
  produces correctly-named refusal diagnostics instead of crashing.
- Miniflux adds the Postgres sidecar pattern and stateful-worker lift
  path. It's in scope for SPRINT-0004 if green on Caddy/Pocketbase arrives
  early, but not blocking.
- Listmonk, Gitea, Mattermost ship as scaffolded rows with
  `t.Skip("deferred")` — their verdicts and expected diagnostics are still
  recorded in the target declaration so governance drift is prevented.

## Golden-file discipline

Assert only on the **normative §EC-REPORT subset**, not the full report:

- `schemaVersion`
- `analysis.algorithm` (the call-graph algorithm disclosure)
- `root` identity (module_path, package_path, object_name, kind)
- `pragma` verdict
- `closure.boundedPruning` (did the bounded-closure predicate hold?)
- `state[].disposition` (replicated / singleton / affinity / externalize / refused)
- `adapters[].kind`
- `externalDependencies[].access_path`
- `diagnostics[].code` (refusal diagnostics only; warnings are non-asserted)

Ignored (volatile): timestamps, binary hashes, artifact paths, pointer
addresses, symbol-span byte offsets (within ±5).

Golden-update flag: `make e2e-update-golden` regenerates the golden JSON
from the current run. Implementers must review the diff before committing.

## Determinism and cleanup

- **Pinned inputs:** target source SHAs come from `evaluation/MANIFEST.yaml`
  (no per-target COMMIT files).
- **Namespace isolation:** `mlv2-baseline-<target>-<runid>` and
  `mlv2-lifted-<target>-<runid>`; runid is nanosecond timestamp. No
  cross-run bleed.
- **Cluster reuse by default;** `make e2e-reset` destroys and recreates.
- **Timeouts:** deploy wait 180s, workload 60s, compile 120s, hard test
  timeout 10m. No unbounded waits.
- **Frozen inputs:** workload requests include deterministic request-IDs.
  Fixture RSS feeds are version-pinned XML.
- **In-cluster fixtures** (not host-side): Kind pod networking doesn't
  reliably reach host-bound listeners.
- **Artifact dump:** failed runs leave artifacts under
  `/tmp/monolift-e2e/<target>/<runid>/` for post-hoc inspection.

## Handoff shape for coding agents

Agent loop:

```
$ make e2e
--- FAIL: TestE2E/caddy (42.1s)
    [stage=4 target=caddy kind=compiler] closure report mismatch
      expected: test/e2e/targets/caddy/golden/report.json
      got:      /tmp/monolift-e2e/caddy/019da-abc/report.json
      diff (normative subset):
        - adapters[1].kind: "registry"
        + adapters[1].kind: "handler"
      hint: compiler did not classify the static-file module as a registry
            adapter; see pkg/compiler/adapters/registry.go
      rerun with MONOLIFT_E2E_UPDATE_GOLDEN=1 if intentional
--- PASS: TestE2E/pocketbase (1.3s)
--- SKIP: TestE2E/miniflux (deferred until Postgres sidecar lands)
```

Three properties make this iterable:
1. **First failure names the stage and the "kind"** (harness/compiler/artifact/workload) so the agent knows whose fault it is.
2. **Failure messages point at the compiler file** to edit (`hint:` string curated in target declarations).
3. **Golden-update mode** is explicit and requires human review before commit.

## SPRINT-0004 delivery shape

The harness must land BEFORE the compiler. SPRINT-0004 should deliver:

1. This strategy doc (committed).
2. Skeletal harness: `test/e2e/harness/*` stubs, Kind automation, closure-report parser, refusal-verdict assertor. Runs with zero target cases and exits cleanly.
3. **Caddy e2e fully green** — baseline through compare. Uses stub compiler output (since v2 isn't implemented yet) — harness verified before compiler exists.
4. **Pocketbase refusal e2e green** — compile + report-assertion only.
5. Miniflux scaffolded (target dir + expected golden + Postgres sidecar manifest) but `t.Skip` until v2 compiler can handle it.
6. Listmonk, Gitea, Mattermost declared with verdicts + `t.Skip`.
7. `make e2e`, `make e2e-reset`, `make e2e-update-golden` targets.
8. README covering: how to run, how to interpret failures, how to add a target, how to update goldens.

The stub-compiler step matters: the harness must prove red/green signal
*before* v2 lands, because it's the thing SPRINT-0005+ agents iterate against.

## Non-goals

- No performance / latency gates (`demo/monolith/test/throughput_test.py` owns that).
- No chaos / failure injection.
- No parallel case execution until the flake profile is known (SPRINT-0005+).
- No external registry; Kind image-load only.
- No multi-arch images; linux/amd64 only.
- No CI integration (hook to GitHub Actions in SPRINT-0005).
- No TLS / ingress testing.
- No full-spec-report equality (normative subset only).
- No cross-lift composition (multi-pragma per target OK; multi-target per cluster isolated).

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| `kind load docker-image` slowness (~30s per image) | Dedicated stage 6 with its own failure signal; cache layers via `docker buildx`; skip rebuild when target source unchanged |
| Golden-file thrash during compiler dev | Normative subset only; `-update-golden` gated; diff shown in failure message |
| Host↔Kind fixture-network ambiguity | In-cluster fixture pods, not `httptest.Server` on host |
| Closure-report schema drift | Shared struct in `pkg/compiler/reportv2/`; JSON Schema validates in CI; harness breaks loudly on field rename |
| Stateful nondeterminism (timestamps, generated IDs) | Explicit normalization in `compare.go`; deterministic request-IDs; version-pinned fixtures |
| Kind flakiness on Apple silicon | Cluster reuse by default; `make e2e-reset` for clean slate; retry-once on wait-for-Ready |
| Baseline-vs-lifted namespace collision | Per-run nanosecond runid suffix on namespace names |
| Upstream target API drift between extraction-time manifest pin and runtime | `evaluation/MANIFEST.yaml` SHA pinning; vendored fallback if upstream deletes |
| Refusal target path regression (pocketbase silently passes) | Assert both `MLV2_EMBEDDED_DB_APP_ROOT` AND `MLV2_CLOSURE_TOO_LARGE`; require both to be present |
| `t.Parallel()` masking flakes | Explicit non-goal in v1; serial execution until profile stable |
| Agent iteration loop confused by "whose fault" | `kind=(harness|compiler|artifact|workload)` prefix in every failure message |

## Acceptance criteria for the harness (SPRINT-0004 close)

- [ ] `test/e2e/` compiles; `MONOLIFT_E2E=1 go test ./test/e2e -v` runs without panic.
- [ ] `make e2e` produces a human-readable pass/fail table.
- [ ] Caddy target: all 10 stages pass against stub compiler output.
- [ ] Pocketbase target: stages 0–4 pass, asserts both expected refusal diagnostics.
- [ ] Miniflux target: declared with Postgres sidecar and expected verdict, `t.Skip` active.
- [ ] Listmonk/Gitea/Mattermost: declared with verdict + expected diagnostics, `t.Skip` active.
- [ ] README explains: run, interpret, add-target, update-goldens.
- [ ] `MONOLIFT_E2E_KEEP=1` preserves namespaces on failure.
- [ ] `make e2e-reset` destroys and recreates the Kind cluster.
- [ ] `make e2e-update-golden` regenerates golden JSON with visible diff.
- [ ] Every failure message carries `[stage=N target=X kind=...]` prefix.
