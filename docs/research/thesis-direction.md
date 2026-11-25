# Monolift thesis direction

The [full thesis proposal, *Kubernetes Should Be Easy!* (November 2026 PDF)](https://github.com/tgoodwin/monolift/blob/main/docs/research/thesis-proposal-2026-11-11.pdf) covers both Kamera and Monolift. Start with §4 for the Monolift experiments and §§5–6 for the proposed compiler and control plane.

**Monolift asks which deployment boundaries can be derived from an existing Go application, and when a Kubernetes control plane should use them.** Today, teams choose those boundaries when they structure and package the application. Kubernetes can place and scale the resulting components, but it cannot choose to run a particular operation in-process instead of across the network. Monolift aims to expose that choice where it is valid, without requiring a new application framework. Developers can introduce candidate boundaries incrementally and still run the original program without the compiler.

Separating application structure from deployment matters because independent scaling and isolation come with communication overhead and backpressure. The [Segment](https://www.infoq.com/news/2020/04/microservices-back-again/) and [Prime Video monitoring](https://archive.is/h788m) cases illustrate the costs of particular boundaries, not a general case for monoliths. [Service Weaver](https://github.com/ServiceWeaver/weaver#readme) illustrates a separate problem: moving an existing application into a component framework can require substantial rewriting.

## What the workshop experiments showed (proposal §4)

The [PLOS ’25 paper](https://dl.acm.org/doi/10.1145/3764860.3768327) used a Go port of [DeathStarBench](https://dl.acm.org/doi/10.1145/3297858.3304013) Social Network as a monolithic baseline. The packages (`user`, `post`, `timeline`, and `socialgraph`) were already organized by function. The port replaced synchronous RPCs with direct interface calls that preserved the call graph, and kept asynchronous message formats while moving queues into memory. The prototype used annotations to extract selected interfaces as services and generated clients for remote calls. A *lift* is a region of computation that can run in-process or as an independently deployed network service. A *lift point* is a call site where a generated proxy dispatches to the local implementation or the remote service. Both experiments ran on an eight-node Kubernetes cluster of 40-core servers with the application initialized from the `Reed98` social graph.

### Static boundaries under a fixed CPU budget

We generated five deployments from the same source and compared them with the monolith under a workload of post creation requests. Each variant had 32 CPU cores in total, with no autoscaling. Each extracted component ran on a separate node.

| Configuration | Entrypoint CPU cores | Extracted components |
| --- | ---: | --- |
| Monolith | 32 | None |
| All four lifts | 16 | 4 cores per lift |
| One lift (four separate runs: user, post, timeline, or socialgraph) | 16 | 16 cores for the selected lift |

![Average latency versus throughput for six deployment configurations](https://github.com/tgoodwin/monolift/raw/main/docs/research/thesis-figures/monolift-boundaries.png)

*Proposal Figure 5; PLOS ’25 Figure 3. TL = timeline; SG = socialgraph.*

**Timeline-only extraction gave a better throughput and average latency tradeoff than either the monolith or full extraction.** The other single-component cuts gave different results. This shows that the boundary matters for this workload even at equal total CPU. It does not establish that timeline is the best cut under other workloads or allocations.

### Local versus remote execution as demand rises

Using the timeline boundary from the first experiment, a developer-written *delegate expression* sent timeline operations to a dedicated service when their rate exceeded **100 requests per second**. The experiment increased input load over time and compared this policy with the monolith and the original microservice implementation. The threshold applies to **timeline operations**, not to the overall input rate plotted on the right axis.

![Tail latency over time for the monolith, Monolift, and the original microservices as input load rises](https://github.com/tgoodwin/monolift/raw/main/docs/research/thesis-figures/monolift-changing-load.png)

*Proposal Figure 6; PLOS ’25 Figure 4. The dashed curve is input load. The paper reports tail latency without specifying the percentile.*

The monolith had low tail latency until it saturated. The microservice implementation paid a latency cost at low load but held up as demand rose. Monolift ran timeline operations locally at low load, with latency close to the monolith, and remotely at higher load, with tail latency about **20% lower** than the microservice implementation. This was one hand-tuned policy on one application. It shows why adaptation may help, not how to choose a boundary or threshold in general. It also shows a change in where calls execute, not live migration of program state.

## Proposed work: valid partitions and sound transformations (proposal §5)

A *partition* is a region of code and the state it owns. A *boundary operation* transfers control and data between partitions. The research question is which partitions Go programs admit under an explicit model, how individual cuts compose into a valid partitioning, and what the compiler can establish about the resulting program.

The model deliberately excludes distributed shared memory, synchronization of mutable state across instances, migration of running computations, and added durability for local state. State that needs to survive a restart must already live in an external durable store. Network failure remains visible to the application.

- **Ownership:** Only the owning partition directly reads or writes a mutable location. Other partitions go through boundary operations. Analysis must account for aliases through receivers, closures, globals, and dependencies, not only the types in a signature. An ephemeral singleton counter still needs one authoritative instance; permission to reset on restart does not make independent replicas interchangeable.
- **Value passing:** Arguments and results cross as copies, with supported values preserved by encoding and decoding. Copying a Go value that contains pointers or shares backing storage cannot silently remove mutations the caller depends on.
- **Call replacement:** In normal operation, extraction and generated RPCs should preserve results, state changes, and external effects under stated assumptions. The compiler must bring along the required code and dependencies, distinguish owned memory from access to external resources, and refuse cases whose preconditions it cannot establish. Cuts may overlap or nest; individually plausible cuts need not work together.
- **Failure semantics:** The proposal requires boundary operations to expose an explicit failure channel in their return signature; the compiler would refuse candidates without one. A timeout or lost response can leave completion uncertain. An error result lets the proxy surface that failure, but does not prove the caller handles it correctly. The invocation contract must say what each outcome means and when another attempt is permitted. Neither retry nor local fallback is safe by default, and exactly-once execution is not assumed.

The goal is a characterization of admissible Go partitions, soundness arguments for supported transformations, and a compiler that implements them. Candidate boundaries would come from lightweight annotations or be inferred at package boundaries. The compiler would diagnose candidates it cannot establish as admissible and generate standalone artifacts and client proxies for accepted ones. The catalog of resulting deployment options is the input to the controller.

For evaluation, it is useful to separate conditions established by analysis from assumptions about deployment and runtime behavior. Pure computations, singleton services with ephemeral state, and operations on external resources are useful cases for exercising accepted and refused boundaries, generated calls, lost requests or responses, and restarts. Coverage and empirical comparisons complement the soundness argument; they do not replace it.

The workshop provided a modular setting for testing placement. In this repository, the [v2 compiler work](https://github.com/tgoodwin/monolift/blob/main/docs/site/docs/v1-to-v2.md) and [real Go application corpus](https://github.com/tgoodwin/monolift/blob/main/docs/evaluation/README.md) expose harder boundaries and dependencies. The [semantic partitioning agenda](https://github.com/tgoodwin/monolift/blob/main/docs/research/2026-10-13-partitioning-research-agenda-v1.md) records the broader research questions. Support for arbitrary Go is not a prerequisite.

## Proposed work: adaptive deployment control (proposal §6)

**The compiler and runtime define which deployments are supported; the controller chooses when to use them.** A centralized Kubernetes control plane would replace independent delegate expressions, which see only local metrics and cannot coordinate competing decisions across lifts. The controller would combine live application and cluster telemetry with offline profiling to select a topology and deploy or scale extracted services. The policy cannot make an unsupported boundary valid.

The proposal outlines three kinds of profiling: find hotspots in the original application, benchmark generated deployments under representative workloads, and measure the overhead of individual boundaries. Those results could inform a transition matrix or a Pareto frontier of candidate topologies. The research question is how to use those measurements to adapt a deployment, not how to devise a universal performance model.

### Transitions under active workloads

A valid target deployment does not imply a valid transition from the current deployment. The proposal targets stateless and database-backed lifts for dynamic adaptation. The controller would provision remote capacity and progressively shift new invocations between local and remote paths. Keeping capacity warm reduces switching latency but consumes resources; reclaiming it saves resources but can make a later switch slower.

Lifts that own local mutable state may be extracted as standalone services, but their placement is fixed at startup: the proposal does not dynamically move or replicate them. The runtime does not migrate in-flight invocations. Calls continue on the path they started on, with network errors or unready endpoints surfaced through the boundary’s explicit failure contract rather than hidden by transparent failover.

An integrated evaluation could vary demand and cluster capacity, compare adaptive behavior with fixed deployments, and measure both steady-state performance and switching costs. Evidence about the resulting deployments is distinct from evidence about the switches themselves.

Resource managers such as [Cilantro](https://www.usenix.org/conference/osdi23/presentation/bhardwaj), [Sinan](https://dl.acm.org/doi/10.1145/3445814.3446693), and [Sage](https://dl.acm.org/doi/10.1145/3445814.3446700) are policy context; Monolift’s focus is expanding the set of deployment choices those kinds of policies could act on. The proposal also identifies an exploratory role for agents: they could use the compiler and profiling harness to investigate performance, then choose from supported deployment options rather than rewrite application code. Agent-driven operation is not required for the core compiler and controller work.

## Relationship to Kamera

The thesis treats Kamera and Monolift as complementary threads on adaptive cloud systems. Kamera makes multi-controller executions explorable and testable; Monolift expands the deployment choices exposed to those controllers. This repository documents the managed-application side: how to derive valid alternatives and what a controller may safely ask the runtime to do. It does not claim that the compiler solves controller coordination, or that the controller can make an invalid transformation valid.

## Scope and milestones

The core deliverable is an integrated compiler and controller with stated guarantees and evaluated behavior. The compiler’s validity rules and transformation preconditions inform the controller interface; controller scaffolding and workload preparation can proceed in parallel. Broader transformation coverage is optional. The proposal targets a single Monolift paper for OOPSLA in spring 2027 and dissertation completion in fall 2027. These are provisional targets, not release commitments.

*This page summarizes the proposal’s research direction, not an inventory of implemented features or a benchmark reproduction guide. The plots are rendered from the proposal’s copies of the published workshop figures.*
