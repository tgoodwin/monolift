# Monolift

Monolift is a Go-based compiler that helps modernize monolithic Go services into Kubernetes-ready, service-oriented deployments. It scans your Go module for annotated interfaces, extracts the implementation and dependency graph, generates standalone services plus an updated entrypoint, and writes Kubernetes manifests for the resulting artifacts.

**Vision paper:** [Monolift: Monolith Modernization](https://dl.acm.org/doi/10.1145/3764860.3768327)

## How it works

1. **Discover and analyze** Go packages within a module (`go/packages`).
2. **Find annotated interfaces** using `// @monolift` pragmas on interface types.
3. **Resolve dependency graphs** starting from the constructor call found in `main`.
4. **Generate services** and an updated entrypoint application.
5. **Build artifacts and manifests** (Docker images + Kubernetes YAML).

## Key capabilities

- Interface-driven extraction (`// @monolift`) for service boundaries.
- Dependency resolution and code generation for service implementations.
- Entrypoint regeneration that wires extracted services and delegates.
- Kubernetes manifest generation with environment extraction from the original deployment.

## Pragma syntax

Annotate interface declarations with a Monolift pragma. The compiler currently supports CPU and memory triggers with a normalized threshold (0.0–1.0).

```go
// Service defines the interface for user-related operations.
// @monolift trigger=CPU threshold=0.5
// @monolift trigger=MEM threshold=0.7
```

Notes:
- Only the first pragma on an interface is currently processed.
- Empty pragmas are allowed for pure extraction without triggers: `// @monolift`.

## Build

```bash
make build
```

This builds the `monolift` binary from `./cmd/main.go`.

## Usage

```bash
./monolift \
  --dirname /path/to/your/go/module \
  --output output \
  --docker-registry localhost:5000 \
  --manifest /path/to/monolith-deployment.yaml
```

### CLI flags

- `--dirname`, `-d` (**required**): Go module directory to parse.
- `--output`, `-o`: Output directory for generated code and manifests (default: `output`).
- `--docker-registry`, `-r`: Docker registry to push images to (default: `localhost:5000`).
- `--manifest`, `-m`: Path to the original Kubernetes Deployment manifest (used to extract env vars and namespace).

## Output layout

A successful run writes:

- `output/<service>/` — generated service binaries (one per extracted interface).
- `output/entrypoint/` — regenerated entrypoint Go module.
- `output/k8s/` — Kubernetes manifests for each extracted service and the entrypoint.

## Demo

A demo monolith and manifests live under `demo/`.

```bash
kind create cluster --config demo/k8s/kind-config.yaml
make build
```

See `demo/README.md` for the full demo workflow.

## Repository layout

- `cmd/` — CLI entrypoint.
- `pkg/compiler/` — package loading, pragma parsing, extraction, and manifest generation.
- `pkg/lift/` — code generation templates and helper types.
- `pkg/pragma/` — pragma parsing and controller logic.
- `demo/` — example monolith and Kubernetes manifests.
- `loadgen.go` — load generator for performance experiments.
