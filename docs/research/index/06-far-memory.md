# 06 — Far Memory, Offloading, and Disaggregation

## The Breakeven Condition {#breakeven}

`docs/research/claude_comprehensive_research.md` lines 127–128 (synthesizing MAUI, CloneCloud,
AIFM, Offload Annotations):

```
offload is profitable when:
  compute_cost(C) > (data_in + data_out) / bandwidth + RTT + serialization_overhead
```

MAUI's empirical finding: offloading saves energy at 25ms Wi-Fi RTT but *costs* energy at 220ms
3G RTT. The profitable set of methods shifts with network conditions.
**Implication for Monolift:** a delegate expression must encode this inequality. The threshold
is not static — it must be parameterized by live bandwidth and RTT measurements.

---

## AIFM — Application-Integrated Far Memory

**Source:** `inspiration/papers/aifm-osdi20.pdf`

Ruan, Schwarzkopf, Aguilera, Belay (OSDI '20). Exposes *application-integrated* semantics —
remoteable pointers and containers over the Shenango μ-threaded runtime. Up to 61× wins over
Fastswap (transparent kernel-level remote memory).

AIFM's thesis is Monolift's thesis: **application-semantic participation is what makes
distribution efficient.** At compile time, Monolift knows what AIFM learns at runtime.
`docs/research/claude_comprehensive_research.md` line 130.

---

## Infiniswap

**Source:** `inspiration/papers/infiniswap-nsdi17.pdf`

Gu, Lee, Zhang, Chowdhury, Shin (NSDI '17). Kernel RDMA swap device delivering 4×–15×
throughput gains on memory-bound workloads. Brutal cost model: every heap pointer dereference
is potentially a page fault. The anti-example: transparent remote memory is only competitive
when the unit of separation respects a natural API boundary.
`docs/research/claude_comprehensive_research.md` line 130.

---

## LegoOS

**Source:** `inspiration/papers/legoos-osdi18.pdf` (also cloned: `inspiration/repos/LegoOS`)

Shan, Huang, Chen, Zhang (OSDI '18 Best Paper). Splitkernel design — processor, memory, and
storage are separate hardware components managed by separate OS kernels. The ultimate
disaggregation: confirmed same conclusion as Infiniswap — transparent remote memory only
competitive when the separation respects a natural API boundary.

---

## Can Far Memory Improve Job Throughput?

**Source:** `inspiration/papers/far-memory-eurosys20.pdf`

Amaro, Branner-Augmon, Luo, Ousterhout et al. (EuroSys '20). Cleanest empirical breakeven
measurement. They build Fastswap + far-memory-aware scheduler CFM. Results:
- **Memory-bound** workloads: single-digit to low-double-digit throughput gains.
- **Compute-bound** workloads: zero or negative gains.

Two characterization parameters: **m2c (memory-to-compute ratio)** and **packability**.
The global scheduling problem is APX-hard. Real wins come from aggregate resource packing,
arguing for a cluster-level coordinator rather than pure per-lift policy.
`docs/research/claude_comprehensive_research.md` lines 132–133.

---

## TELEPORT

**Source:** `inspiration/papers/teleport-sigmod22.pdf`

Zhang, Chen, Sankhe et al. (SIGMOD '22). Adds `pushdown(func, arg, flags)` syscall to a
splitkernel OS. Flags choose eager/lazy synchronization and coherence relaxation.
TELEPORT is Monolift at the OS layer: the flags are the delegate-expression semantics.
`docs/research/claude_comprehensive_research.md` line 134.

---

## Offload Annotations

**Source:** `inspiration/papers/offload-annotations-atc20.pdf`
**Slides:** `inspiration/papers/offload-annotations-atc20-slides.pdf`

Yuan, Palkar, Narayanan, Zaharia (ATC '20). Annotations on types and functions in CPU libraries
(NumPy, Pandas) that map them to equivalent GPU-library implementations. Bach runtime dispatches
per-call based on estimated speedup minus transfer cost. Reported speedups up to 1200× (median 6.3×).

**Pattern**: *static annotations + runtime dispatch + per-call cost model* = exactly Monolift's
pattern. Monolift should cite OA as a direct antecedent.
`docs/research/claude_comprehensive_research.md` line 134.

---

## Hermit

**Source:** `inspiration/papers/hermit-nsdi23.pdf`

Qiao et al. (NSDI '23). Extends AIFM lineage with feedback-directed async and
application-managed soft state. Confirms: cost-model accuracy requires online measurement
not static estimates.

---

## Memory disaggregation ecosystem

- `inspiration/repos/awesome-disaggregated-memory` — curated reading list
- `inspiration/papers/disaggregated-db-sigmod23-tutorial.pdf` — SIGMOD '23 tutorial
- `inspiration/papers/memory-disagg-dbms-sigmod23-slides.pdf` — slides
- `inspiration/papers/physical-memory-pools-vision.pdf` — vision paper
- `inspiration/papers/nros-osdi21.pdf` — NrOS (OSDI '21): effective replication in an OS
- `inspiration/papers/chenang-vldb2020.pdf` — network-aware data processing (VLDB '20)
- `inspiration/papers/shadaj-dissertation-hydro.pdf` — Hydro/Hydroflow dataflow runtime

---

## CXL context

CXL 2.0+ drops interconnect RTT from ~µs (RDMA) to ~hundreds of ns. Many lifts unprofitable
on RDMA become profitable on CXL-attached memory. The delegate expression's cost model should be
**parametric in the interconnect tier**. `docs/research/claude_comprehensive_research.md` line 136.
