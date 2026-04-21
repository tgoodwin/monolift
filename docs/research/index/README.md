# Monolift Research Index

Semantic index over `inspiration/` — papers, repos, and HTML snapshots.
Each topic file cites specific files and page/line numbers.

## Topics

| # | Topic | Key files |
|---|-------|-----------|
| [01](01-monolift-core.md) | **Monolift + Closest Comparisons** | monolift-plos25, service-weaver-hotos23, ignis-pldi19 |
| [02](02-choreography.md) | **Choreographic Programming** | montesi-choreo-*, pirouette-popl23, choral-toplas24, HasChor |
| [03](03-multitier.md) | **Multitier and Tierless Programming** | multitier-survey-csur20, scalaloci-oopsla18, urweb, links |
| [04](04-actors-location.md) | **Actor Systems and Location Transparency** | pony-*, orleans-socc11, ray-osdi18, corba-rise-fall-queue06 |
| [05](05-partitioning.md) | **Automated Partitioning and Decomposition** | coign-osdi99, ignis-pldi19, maui-mobisys10, monoembed, cargo |
| [06](06-far-memory.md) | **Far Memory, Offloading, and Disaggregation** | aifm-osdi20, infiniswap-nsdi17, teleport-sigmod22, hermit-nsdi23 |
| [07](07-scheduling.md) | **Microservice Scheduling and Resource Management** | deathstarbench, sage, sinan, autopilot, cilantro, firm |
| [08](08-serverless.md) | **Serverless, FaaS, and Cold Start** | sand-atc18, faasm-atc20, unikraft-eurosys21, mirageos-asplos13 |
| [09](09-correctness.md) | **Distributed Correctness, Types, and Proofs** | calm-cacm20, crdts, lvars, session-types, ironfleet, verdi |
| [10](10-compiler-infra.md) | **Compiler Infrastructure and Program Analysis** | mlir-cgo21, verified-lifting, shadaj-dissertation-hydro |
| [11](11-rl-ml-systems.md) | **RL and ML for Systems** | pensieve, decima, auto, park, cilantro |

## Cross-cutting themes

**The annotation spectrum** — from fully automatic (Coign, Ignis) to fully manual (OpenMP, Jif).
Monolift sits in the middle: `//monolift:offload` per function, runtime-directed via delegate expressions.
See [05-partitioning](05-partitioning.md) §annotation-burden and [01-monolift-core](01-monolift-core.md) §design-space.

**The location-transparency trap** — Waldo et al. TR-94-29 → CORBA → DCOM → RMI all promised
transparency and failed. Monolift concedes Waldo's critique structurally: the delegate expression
*is* the API surface for latency, partial failure, and concurrency.
See [04-actors-location](04-actors-location.md) §waldo.

**The breakeven condition** — every offloading system (MAUI, CloneCloud, AIFM, Offload Annotations)
converges on: offload is profitable iff `compute_cost > (data_in + data_out)/bandwidth + RTT + serialization`.
Delegate expressions must encode this. See [06-far-memory](06-far-memory.md) §breakeven.

**The composition/oscillation problem** — per-lift delegate expressions that are locally stable
can be jointly unstable when they share CPU/bandwidth. SEDA, Kubernetes HPA, and MARL literature
all confirm this. The hardest open problem in Monolift's current design.
See [07-scheduling](07-scheduling.md) §oscillation and [11-rl-ml-systems](11-rl-ml-systems.md).

**Endpoint projection** — choreographic programming's correctness theorem says: if your
distributed system is the projection of a global choreography, it is deadlock-free by construction.
Monolift lifts may be castable as projections for the monotone-state subset.
See [02-choreography](02-choreography.md) §endpoint-projection and [09-correctness](09-correctness.md) §calm.

## Source documents

Research notes in `docs/research/`:
- `claude_comprehensive_research.md` — deep survey, sections 1–17
- `claude_research_notes.md` — earlier notes
- `GEMINI-Monolift-Research.md` / `GEMINI-Monolift-Research-II.md` — Gemini survey
- `codex-research-report.md` — Codex report
