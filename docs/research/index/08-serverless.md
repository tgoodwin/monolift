# 08 — Serverless, FaaS, and Cold Start

A Monolift lift deployed as a Kubernetes pod is structurally a FaaS invocation.
The cold-start cost is the lower bound on the delegate expression's flip threshold.

## Cold-start cost reference numbers

`docs/research/claude_comprehensive_research.md` lines 103–104:

| Platform | Cold-start latency |
|----------|--------------------|
| AWS Lambda (interpreted) | 100–500 ms |
| AWS Lambda (JVM) | 500–2000 ms |
| AWS Lambda (Go/Rust) | tens of ms |
| Knative (fresh pod) | 2–5 s |
| Knative (pre-warmed) | sub-second |
| Faasm (WASM faaslet) | < 10 ms |

**Implication:** The delegate expression flip threshold must be larger than the transition cost.
In the Kubernetes/Knative regime this is at least hundreds of milliseconds.
**Pre-provisioning a "remote" replica before flipping any lift to remote** is the single
strongest near-term feature Monolift needs to add.

---

## SAND

**Source:** `inspiration/papers/sand-atc18.pdf`

Akkus, Chen, Rimac-Drlje et al. (ATC '18). Intra-application sandboxing and fast
function-to-function communication via a message bus. Warm-start overhead sub-millisecond.
Design insight: co-location of functions within a shared sandbox eliminates cold-start for
same-application invocations.

---

## Nightcore

**Source:** (PDF download failed — UT Austin URL 404)
Jia & Witchel (ASPLOS '21). Co-locates microservice functions on a shared host runtime,
achieving 1.36–2.93× throughput over containerized baselines with 69–85% lower tail latency.

---

## Faasm

**Source:** `inspiration/papers/faasm-atc20.pdf`

Shillaker & Pietzuch (ATC '20). WebAssembly "faaslets" with shared memory for fast communication.
Cold start < 10 ms. Key enabling technology: WASM isolation is cheap enough that per-call
instantiation becomes practical.

**For Monolift:** Faasm is an alternative backend. If Kubernetes pod latency is too coarse (2–5s
cold start), a WASM-backed lift could be instantiated in <10ms. See [10-compiler-infra](10-compiler-infra.md)
for WASM/TinyGo context.

---

## Unikernels

### Unikraft (Kuenzer et al. — EuroSys '21)
**Source:** `inspiration/papers/unikraft-eurosys21.pdf`

Compiles an application + just the needed OS functionality into a single unikernel image.
Startup latency: tens of milliseconds. Image size: single-digit MB.

### MirageOS (Madhavapeddy et al. — ASPLOS '13)
**Source:** `inspiration/papers/mirageos-asplos13.pdf`

Library OS for OCaml. The intellectual ancestor of Unikraft's approach.

**For Monolift** (`docs/research/claude_comprehensive_research.md` lines 246–247):
If a lift were deployed as a Unikraft instance rather than a Kubernetes pod, cold-start drops
by an order of magnitude — the delegate expression flip threshold could be correspondingly lower.
A direct engineering win with no theoretical extension required.

---

## WebAssembly as a lift backend

`docs/research/claude_comprehensive_research.md` lines 186–187:
WASM Component Model (WASI Preview 3, Wasmtime 24+) — a lift compiled to a WASM component is:
- Small (KB vs. MB for container images)
- Fast to start (single-digit ms)
- Platform-agnostic (Wasmtime, WasmEdge, browsers, edge)

TinyGo already compiles a useful subset of Go to WASM. See `inspiration/html/cncf-wasm-landscape.html`.

A WASM-backed Monolift would naturally extend to edge and multi-cloud deployments.
