# Monolift Codebase Research Report

_Generated 2026-04-19 after ~9 months of inactivity. Codebase snapshot: d44e2e2 (2025-07-21)._

## Executive Summary

Monolift is a Go→Kubernetes compiler that transforms annotated monolithic applications into distributed microservices through compile-time code extraction. The codebase (~6.8K LOC across compiler + runtime) enables pay-as-you-go service migration: developers annotate interfaces with `@monolift` pragmas, and the compiler lifts them into separately-scaled services while maintaining backward compatibility. Two major feature branches merged recently (#7 Dapr integration, #8 timeline worker pool).

---

## 1. Top-Level Layout

```
monolift/
├── cmd/main.go                      [entry point: CLI parser + orchestrator]
├── pkg/                             [~3.5K LOC - core compiler & runtime]
│   ├── compiler/                    [~1.5K LOC - AST analysis, code extraction]
│   │   ├── compiler.go              [main Compiler struct, Compile() orchestrator]
│   │   ├── pragma.go                [pragma parsing from // @monolift comments]
│   │   ├── pragma_test.go           [unit tests for pragma parsing]
│   │   ├── artifacts.go             [Docker build orchestration]
│   │   ├── manifests.go             [K8s manifest utilities]
│   │   └── util.go                  [AST helper functions]
│   ├── lift/                        [~1.5K LOC - code generation, K8s templates]
│   │   ├── types.go                 [Dependency, InstantiationPlan data structures]
│   │   ├── clientgen.go             [RPC client generation]
│   │   ├── servicegen.go            [extracted service scaffold]
│   │   ├── delegategen.go           [dual-dispatch delegate generation]
│   │   ├── k8s.go                   [manifest generation]
│   │   └── templates/               [embedded Go templates]
│   ├── pragma/                      [~400 LOC - runtime delegation controller]
│   │   ├── pragma.go                [Pragma struct, trigger/threshold parsing]
│   │   └── controller.go            [ThresholdController: hysteresis-based decider]
│   ├── metrics/                     [~400 LOC - cgroup-based resource monitoring]
│   │   ├── monitor.go               [Monitor: CPU/memory usage polling]
│   │   ├── cgroup.go                [cgroup reader for container limits]
│   │   └── k8s.go                   [K8s resource limit parsing]
│   └── util/
├── demo/monolith/                   [full social-network monolith test app, ~3.4K LOC]
├── go.mod, go.sum                   [Dapr, Prometheus, gRPC, K8s client]
├── Makefile
└── run_all.sh
```

**Total: ~6,800 LOC** (excluding tests, inspiration, evaluation, docs).

---

## 2. Compiler Architecture

### Entry Point
`cmd/main.go:20–51` — Cobra CLI:
- `-d, --dirname`: Go program directory (required)
- `-o, --output`: output directory
- `-r, --docker-registry`: Docker registry URL
- `-m, --manifest`: original K8s deployment manifest (for env var extraction)

### Main Pipeline
`pkg/compiler/compiler.go:122–184` — `Compile()` orchestrates five stages:
1. Extract code (`c.extractCode()` L135) — discover interfaces with `@monolift`
2. Generate entrypoint (`c.generateEntrypoint()` L168) — rewrite main to use clients/delegates
3. Build Docker artifacts (`builder.build()` L175)
4. Generate K8s manifests (`generateK8sManifests()` L180)
5. Return with `output/` containing all artifacts

### Go Toolchain
- `go/ast`, `go/packages` (NeedTypes, NeedTypesInfo), `go/token`, `go/types`
- `golang.org/x/tools/go/ast/astutil` — import manipulation

### Key Functions
| Function | Location | Purpose |
|----------|----------|---------|
| `New()` | compiler.go:51 | Load all application packages |
| `extractCode()` | compiler.go:291 | Walk AST for `@monolift` interfaces |
| `findSingleImplementer()` | compiler.go:735 | Resolve interface → unique struct |
| `findConstructorCallInMain()` | compiler.go:854 | Locate `New<Service>()` in main |
| `resolveDependencies()` | compiler.go:952 | Topologically sort dependency graph |
| `resolveAssignment()` | compiler.go:1079 | Recursively analyze RHS of assignments |
| `resolveExpr()` | compiler.go:1334 | Resolve variable refs and constants |

### Pragma Parsing
- `pkg/compiler/pragma.go` — extract `// @monolift trigger=CPU threshold=0.5`, split by whitespace
- `pkg/pragma/pragma.go:32–64` — schema validation: `trigger` ∈ {CPU, MEM}, `threshold` ∈ (0,1]

### Generated Code Patterns

Plain `@monolift` (no trigger):
```go
userService = NewuserserviceClient("http://userservice.default")
```

With trigger (e.g. `trigger=CPU threshold=0.5`):
```go
localSvc := userservice.NewService(...)
remoteSvc := NewuserserviceClient("http://userservice.default")
decider := pragma.NewCPUDecider(monoliftMetricsMonitor, 0.5)
userService = NewuserserviceClientDelegate(localSvc, remoteSvc, decider)
```

### Known Gaps / TODOs
| Location | Issue | Severity |
|----------|-------|----------|
| compiler.go:69 | Type errors during package loading not handled | Medium |
| compiler.go:161 | Manifest parsing via byte manipulation (not YAML schema) | Low |
| compiler.go:358 | Constructor naming hardcoded to `New<InterfaceName>` | High |
| compiler.go:364–365 | Auto-resolution failure handling stubbed | Medium |
| compiler.go:433 | Extracted code naming can collide | High |
| compiler.go:953 | `c.rootStmt` mutated globally (thread-unsafe) | High |
| clientgen.go:110 | `panic` on unhandled response zero-value type | Critical |

---

## 3. Runtime & Lifted-Service Scaffold

### Dual-Dispatch Delegate
At call time: decider evaluates metrics → local in-process OR remote RPC. Transparent to caller.

### Client Generation (`pkg/lift/clientgen.go`)
HTTP/JSON RPC client. Method signature requirements (L67): exactly `(ctx, req) → (resp, error)`. Otherwise generation errors.

### Service Generation (`pkg/lift/servicegen.go`)
For each extracted service: copy methods, add HTTP handler wrappers (`/methodname`), embed dependencies, listen on `:8080`.

### Delegate Generation (`pkg/lift/delegategen.go`)
Wraps local + remote impls; `ShouldDelegate()` picks.

### Metrics Monitor (`pkg/metrics/monitor.go`)
Polls cgroups at configurable interval (default 1s). Reads limits from `REQUESTS_CPU`, `REQUESTS_MEMORY` env vars (from K8s downward API).

### Threshold Controller (`pkg/pragma/controller.go`) ⚠️ **STUBBED**
`updateState()` L109–117 is unimplemented. Design: hysteresis (trigger/reset), async polling, thread-safe `ShouldDelegate()`. **~50 LOC to complete.**

### Transport
Plain HTTP/JSON. No TLS/auth/retry/gRPC yet.

### State Store: Dapr
Demo uses Dapr for state abstraction (Redis backend). Optimistic concurrency via ETag matching.

---

## 4. Kubernetes Integration

### Manifest Generation (`pkg/lift/k8s.go`)
- **Service** (L95–130): ClusterIP, port 8080
- **Deployment** (L95–130): replicas=1 (hardcoded), resource limits 2 CPU / 2 GiB (compiler.go:33–34), env copied from original manifest
- **ServiceMonitor** (optional): Prometheus scraping

### Entrypoint Deployment
Original monolith main rewritten; deployed alongside extracted services; communicates via cluster DNS (`http://<svc>.<ns>`).

### Templates (`pkg/lift/templates/`)
`deployment.yaml.tmpl`, `service.yaml.tmpl`, `entrypoint-deployment.yaml.tmpl`, `servicemonitor.yaml.tmpl`.

### PR #8 (timeline-workerpool)
Bounded worker pool (500 workers, 1000 buffer). No autoscaling — manual HPA still required.

### Dapr Sidecar
K8s manifests need `dapr.io/enabled: "true"`. Sidecar listens on `localhost:3500`.

---

## 5. Test/Demo Application

Located at `demo/monolith/`. A **social-network backend**:

| Service | Interface | Location | Methods | Pragma |
|---------|-----------|----------|---------|--------|
| UserService | `Service` | `userservice/service.go:28` | Register, Login | `trigger=CPU threshold=0.5` |
| PostService | `Service` | `postservice/service.go` | CreatePost, GetPost | — |
| SocialGraphService | `Service` | `socialgraph/service.go` | Follow, GetFollowers | — |
| TimelineService | `Service` | `timelineservice/service.go:39` | ReadTimeline, UpdateTimeline | `trigger=CPU threshold=0.5` |

All services use Dapr state store. Frontend (`frontend/handlers.go`): `/save`, `/home`, `/register`, `/login`, `/metrics`.

### Compile & Run
- `make build` — compiles monolift CLI
- `go run ./demo/monolith` — monolith on `:8080`
- Full compile: `./monolift -d demo/monolith -o output -r ghcr.io/tgoodwin -m demo/monolith/k8s/monolith-deployment.yaml`
- **No automated E2E test** that runs: compile → deploy → functional verify.

---

## 6. Tests

### Unit
`pkg/compiler/pragma_test.go` (236 LOC) — only covers pragma parsing.

### Load
`demo/monolith/test/throughput_test.py` (237 LOC) — async Python load generator. Geometric RPS ramp, latency percentiles. Not functional.

### Missing
E2E, functional correctness, dependency-graph correctness, codegen/template, manifest-valid-YAML, delegate switching under metric changes.

---

## 7. Completeness Assessment

### Works End-to-End ✅
Pragma parsing · interface extraction · constructor resolution · dependency analysis · code generation · Docker build · K8s manifest generation · metrics collection.

### Partial ⚠️
- **Delegate dispatch** — skeleton but `updateState()` stubbed (controller.go:109–117)
- **Auto-scaling** — Deployment hardcoded to replicas=1; no HPA
- **Service discovery** — DNS only; no health checks / circuit breakers

### Not Implemented ❌
Automatic annotation inference · stateful service lifting · failure recovery · TLS/auth · gRPC transport · function-level migration · adaptive placement · operator mode.

---

## 8. Recent Activity

### Last 15 Commits
```
d44e2e2 (HEAD) Merge PR #8 timeline-workerpool
a0d542d update comments ii
e07107f clean up comments
3062103 Merge PR #7 cloudlab-dapr-run-all
8dbdb38 Merge branch 'main' into cloudlab-dapr-run-all
2555036 clean up more old scripts
b7271d9 update gitignore
b11c5a6 clean up 1 more
c002aa5 remove generated code from git tracking
5e9ab80 update gitignore
d1c8213 merge main
9ded63f fix container repo
6adfdca update deployment
e00c449 update code
4cf836a add go load generator script
```

### PR #8 `timeline-workerpool` (merged 2025-07-21)
- TimelineService worker pool (500 workers, 1000 buffer)
- Async UpdateTimeline enqueue/background processing
- K8s pod toleration/taint annotations

### PR #7 `cloudlab-dapr-run-all` (merged 2025-07-16)
- Replace direct Redis with Dapr abstraction
- Add `throughput_test.py` (237 LOC)
- CloudLab bootstrap script
- `run_all.sh` automated deployment script

### Timeline
- **Aug 2024–Jul 2025:** active development (Dapr, workerpool, perf)
- **Jul 2025–Apr 2026 (9 months):** paused on main; evaluation/research branches hold experimental data

---

## 9. Dependencies

- **Go 1.23.6** (toolchain 1.23.10)
- `github.com/dapr/go-sdk` v1.12.0
- `github.com/prometheus/client_golang` v1.22.0
- `golang.org/x/tools` v0.34.0 (compiler backbone)
- `k8s.io/api`, `k8s.io/apimachinery` v0.31.x
- `sigs.k8s.io/yaml` v1.4.0
- `google.golang.org/grpc` v1.70.0

---

## 10. 2–3 Hour Re-Orientation Plan

### Quick (30 min)
1. Read this report
2. Skim `pkg/compiler/compiler.go:1–120`
3. Run throughput test: `cd demo/monolith && go run . &` then `python3 test/throughput_test.py --ip 127.0.0.1 --port 8080`

### Medium (1 hr)
4. Trace full compilation: `./monolift -d demo/monolith -o /tmp/output -r ghcr.io/tgoodwin -m demo/monolith/k8s/monolith-deployment.yaml` → review `/tmp/output/`
5. Read `pkg/pragma/controller.go:109–117` (stubbed logic)
6. Read `demo/monolith/userservice/service.go:28–42` (annotated example)

### Deep (1 hr)
7. **Implement the TODO in `pragma/controller.go:updateState()`** — poll metrics, apply hysteresis, set `isOffloading` with proper locking. Payoff: placement decisions actually work.
8. Add E2E test: compile → build → K8s deploy → trigger metric change → verify delegation switch.

### Open Questions
- Generalize constructor naming beyond `New<InterfaceName>`?
- Plan for stateful services?
- Async/await placement directives beyond CPU threshold?
- Roadmap: gRPC, CRD operator, multi-language?
