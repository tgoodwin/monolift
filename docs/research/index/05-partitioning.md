# 05 — Automated Partitioning and Decomposition

The thirty-year literature on automatically distributing programs. Monolift sits toward the
"annotation-heavy, automation-light" end by choice.

## Classical automated partitioning

### Coign (Hunt & Scott — OSDI '99)
**Source:** `inspiration/papers/coign-osdi99.pdf`

Founding example. Profiles a running COM application, builds a weighted call graph, solves a
graph-cut problem to place each component on client or server. Key insight: the right partition
is discoverable from runtime behavior. Key limitation: only works because COM has explicit
coarse-grained component boundaries; the optimum changes with workload in ways a one-shot solver
never re-solves.
`docs/research/claude_comprehensive_research.md` line 47.

### Pyxis (Cheung, Arden, Madden, Myers — VLDB '12)
**Source:** `inspiration/papers/pyxis-vldb12.pdf`
**Also:** `inspiration/papers/auto-partition-db-apps-cidr13.pdf`

Statically partitions a Java+SQL application between app server and database to minimize round
trips, using information-flow types to guarantee semantic equivalence.

### Alvin Cheung dissertation
**Source:** `inspiration/papers/akcheung-dissertation.pdf`

---

## Mobile / edge offloading

### MAUI (Cuervo, Balasubramanian et al. — MobiSys '10)
**Source:** `inspiration/papers/maui-mobisys10.pdf`

Method-level annotations (`[Remoteable]`) plus an ILP solver selects which methods to offload
to the cloud at runtime. Reports order-of-magnitude energy savings on compute-heavy workloads
but *negative* gains on I/O-bound or interactive workloads at high RTT (220ms 3G).
`docs/research/claude_comprehensive_research.md` line 47.

### CloneCloud (Chun, Ihm, Maniatis et al. — EuroSys '11)
**Source:** `inspiration/papers/clonecloud-eurosys11.pdf`

VM-level thread migration: a running thread can be checkpointed and resumed on the cloud.
No annotations — fully automatic partitioning at thread-clone granularity.

### Elicit
**Source:** `inspiration/papers/elicit-offload.pdf`

### Code Partition (CloudNet '12)
**Source:** `inspiration/papers/code-partition-mobile-cloud-cloudnet12.pdf`

### MSR Mobile Offload Dissertation
**Source:** `inspiration/papers/msr-mobile-offload-dissertation.pdf`

**Key empirical finding across MAUI, CloneCloud** (`docs/research/claude_comprehensive_research.md` line 128):
Only 30–60% of candidate methods are profitable to offload, and the profitable set shifts with
network conditions. **Any statically-chosen lift boundary is wrong a measurable fraction of the time.**
This is the core empirical argument for runtime-directed placement.

---

## Annotation burden {#annotation-burden}

`docs/research/claude_comprehensive_research.md` lines 63–67: annotation burden is acceptable when
annotations are **local, optional, and monotonic**. Monolift satisfies all three:
- Local: an annotation affects only its function.
- Optional: the program compiles and runs without them.
- Monotonic: adding a lift does not force rewrites elsewhere.

The risk: if delegate expressions acquire dependencies on each other, they become non-local.

**Cautionary examples:** Jif (`inspiration/papers/swift-sosp07.pdf`) — per-value information-flow
labels are too expensive for a monolith retrofit; Swift-style annotation never saw wide adoption.

---

## LLM and ML-based decomposition (2020–2025)

### Mono2Micro (Kalia, Xiao et al. — FSE '21, IBM Research)
**Source:** `inspiration/papers/arxiv-2107.09698-mono2micro.pdf`
**HTML:** `inspiration/html/mono2micro-ibm-toolchain.html`, `inspiration/html/mono2micro-ibm-practical.html`

Most-cited industrial baseline. Combines runtime call traces with hierarchical clustering to
propose microservice partitions for Java monoliths.

### CARGO (Nitin, Asthana, Ray, Krishna — ASE '22)
**Source:** `inspiration/papers/cargo-ase22.pdf`

AI-guided dependency analysis for microservice decomposition.

### MonoEmbed (Sellami & Saied — arXiv 2502.04604, Empirical SE '25)
**Source:** `inspiration/papers/monoembed-arxiv2502.pdf`

Contrastive learning + LoRA fine-tuning of LLMs to produce embeddings of monolithic components,
then clustered into microservice boundaries. State of the art for offline decomposition.
`docs/research/claude_comprehensive_research.md` line 59.

### Systematic Review (Abgaz et al. — IEEE TSE '23)
**Source:** `inspiration/papers/abgaz-tse23-decomp.pdf`

Best entry point to the decomposition literature. Reviews ~50 systems.

### LLM-guided annotation synthesis paper
**Source:** `inspiration/papers/llm-microservice-generation-sbcars.pdf`
**Source:** `inspiration/papers/ai-driven-refactoring.pdf`

### Key distinction from Monolift (`docs/research/claude_comprehensive_research.md` lines 60–61):

> *"These systems do offline, one-shot decomposition — they propose a partition, a human approves it,
> and a rewrite happens. None of them do what Monolift does: keep a single source tree with
> annotations that can be toggled, and make placement decisions at runtime."*

A plausible 2026 paper: use an LLM-based decomposer as an **annotation suggester** for Monolift.
