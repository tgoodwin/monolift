# 10 — Compiler Infrastructure and IRs

## MLIR

**Source:** `inspiration/papers/mlir-cgo21.pdf`

Lattner, Amini, Bondhugula et al. (CGO '21). Multi-level IR with user-defined dialects.

**For Monolift** (`docs/research/claude_comprehensive_research.md` line 184):
A Monolift dialect is plausible: the lift annotation becomes an MLIR op with attributes
for the delegate expression and deployment target; lowering passes produce either in-process
Go or gRPC-plus-Kubernetes manifests. Benefit: non-Go source languages could share the same
Monolift backend. Cost: reimplementing the Go SSA frontend against MLIR.

---

## Go toolchain

`docs/research/claude_comprehensive_research.md` line 188:

- **`golang.org/x/tools/go/ssa`** — SSA form suitable for dataflow analysis
- **`go/analysis`** — standard framework for pluggable analyzers
- **`gopls`** — exposes the same analyses to editors

Key gaps for Monolift:
1. A production-quality **escape analysis** beyond what the Go runtime does internally.
2. A **points-to analysis** robust to Go's interface dispatch and reflection.

Both are required to reliably detect when a function argument aliases shared mutable state
and therefore cannot cross a lift boundary.

---

## GraalVM and Truffle

`docs/research/claude_comprehensive_research.md` line 186:

Language-agnostic compilation and partial evaluation across multiple source languages.
Closest thing to a production language-portable substrate.

---

## TinyGo and WASM backend

`docs/research/claude_comprehensive_research.md` lines 186–188, 240:

- **TinyGo** compiles a useful subset of Go to WASM (also via LLVM).
- **WebAssembly Component Model** (WASI Preview 3 / Wasmtime 24+, 2024–25):
  - Lift compiled to WASM component is small (KB vs. MB for container images)
  - Cold start sub-10 ms (Faasm-class runtimes)
  - Platform-agnostic: Wasmtime, WasmEdge, browsers, Cloudflare Workers, Fastly Compute@Edge
- **Research question**: can Monolift's lift IR lower to a WASM component? Requires solving
  the async/goroutine-to-WASM mapping — active in WASI Preview 3 proposal.
- A WASM-backed Monolift extends naturally to edge and multi-cloud deployments.

See also [08-serverless](08-serverless.md) §Faasm and §WebAssembly.

---

## Verified lifting of stencils

**Source:** `inspiration/papers/verified-lifting-stencil-pldi16.pdf`

Kamil, Cheung et al. (PLDI '16). Lifts stencil computations from low-level C/Fortran to
high-level array operators with a machine-checkable proof of equivalence.

**For Monolift**: The "verified lifting" framing (lift + proof of equivalence) is the
correctness template. If Monolift's compiler pass is cast as endpoint projection
(see [07-choreography](02-choreography.md)), the proof obligation reduces to "generated
RPC matches local call's contract" — the same shape as a verified lift.

---

## Hydro / Shadaj dissertation

**Source:** `inspiration/papers/shadaj-dissertation-hydro.pdf`

Shadaj Laddad (PhD thesis, UC Berkeley, ~2024). Hydro: dataflow runtime for distributed
systems compiled from a high-level specification. Hydroflow is the runtime layer.

**For Monolift**: Hydro's compilation strategy — high-level specification → dataflow IR →
distributed deployment — is the closest architecture to a mature version of Monolift's
compiler pass. The dissertation is a detailed reference for how to structure that pipeline.

---

## Verified lifting and formal proof

`docs/research/claude_comprehensive_research.md` lines 252–253:

If the compiler pass is cast as endpoint projection, the correctness obligation reduces to
*"the generated RPC matches the local call's contract"* — small enough to be proof-tractable
for a bounded lift. The full verification stack (Verdi/IronFleet/Ivy in
[09-correctness](09-correctness.md)) is a multi-year project; the single-lift obligation
is a tractable near-term target.

---

## Static analysis frameworks (JVM reference)

`docs/research/claude_comprehensive_research.md` line 188:

**Soot** and **WALA**: JVM-side frameworks for call graph construction, pointer analysis,
and dataflow. Monolift's Go analysis mirrors Soot's inter-procedural capabilities but
starts from `go/ssa` rather than Jimple.

**CodeQL** and **Infer** (Meta): query-based and abstract-interpretation-based analyses.
CodeQL's query language is a useful template for the "find all lift-unsafe patterns"
pass Monolift needs.
