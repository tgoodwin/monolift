# **The Architecture of Incremental Distribution: A Comprehensive Research Report on Monolift and Compiler-Driven Network Typologies**

## **Introduction: The Distributed Systems Dichotomy**

The evolution of distributed computing over the past three decades has been defined by a persistent tension between developer ergonomics and operational reality. As applications scale beyond the capacity of a single physical machine, software engineers are forced to abandon the relative simplicity of monolithic architectures in favor of distributed systems. However, this transition is rarely seamless. The widespread adoption of microservices, service-oriented architectures (SOA), and cloud-native topologies has introduced unprecedented complexities in state management, partial failure handling, and network latency. Organizations frequently find themselves paralyzed by the massive, all-or-nothing migration costs required to decompose legacy monoliths into distributed microservices.

Recently, a novel paradigm has emerged to address this friction: treating system distribution not as a fundamental architectural rewrite, but as a compiler pass. This approach is epitomized by Monolift, a system designed to incrementally transform legacy monolithic applications into distributed deployments via transparent, code-level annotations.1 By allowing developers to maintain a single, general-purpose codebase (currently targeting the Go programming language) while delegating network communication, serialization, and placement to the compiler and runtime environment, Monolift attempts to lower the barrier to entry for distributed scalability.1

However, any framework attempting to abstract the network must grapple with historical precedents that warn against treating remote and local computing as functionally equivalent.3 This report provides an exhaustive, highly detailed examination of the theoretical and practical landscape surrounding Monolift. By analyzing the trajectory of distributed object models, multitier programming languages, choreographic programming, actor systems, automatic partitioning frameworks, and disaggregated operating systems, this analysis will contextualize Monolift’s innovations and chart a course for solving its remaining architectural challenges.

## **The Monolift Paradigm: Compiler-Driven Distribution Mechanics**

Monolift represents a departure from traditional distributed systems development by introducing a "pay-as-you-go" adoption model. Rather than requiring up-front architectural buy-in or a massive rewrite, Monolift permits legacy monoliths to be incrementally annotated.1 Critically, the application remains fully functional as a single-process monolith if compiled without the Monolift toolchain, ensuring that the introduction of distributed capabilities is completely reversible.2

### **Core Abstractions: The Lift Point and Dual Dispatch**

The foundational abstraction within this system is the "lift point".1 Through transparent comments embedded directly into the source code (e.g., //monolift:offload), developers indicate specific boundaries or functions within the application that are eligible for remote execution. During the compilation phase, the Monolift compiler transforms these lift points into a dual-dispatch mechanism. At runtime, the system evaluates dynamic conditions to determine whether the function should execute locally within the current process or be serialized and offloaded to a remote node across the network.1

This dynamic placement is governed by "delegate expressions"—a domain-specific language (DSL) embedded within the annotations that couples placement decisions to live performance signals.1 For example, a delegate expression might dictate that a compute-heavy function should only be offloaded if the local application's CPU utilization exceeds a certain threshold (e.g., 50%) or if the request queue reaches a specific depth.2 By evaluating these delegate expressions continuously, Monolift avoids the rigid, static topologies characteristic of standard microservices. The physical deployment of the application can expand or contract elastically based on real-time load, which has been shown to reduce tail latency in performance benchmarks, such as simulated social network architectures under increasing input loads.2

### **Infrastructure Backend and the Bounded State Model**

Unlike previous academic frameworks that required bespoke runtime environments, Monolift explicitly targets existing infrastructure backends.1 By compiling down to artifacts compatible with container platforms like Kubernetes, Monolift leverages the robust orchestration, scheduling, and networking capabilities already present in modern enterprise environments.1

However, to simplify the complex realities of distributed memory and avoid the pitfalls of remote pointer chasing, Monolift imposes a "bounded model" on the application.1 This model restricts shared state, prohibits heap sharing across lift boundaries, and severely limits object migration.1 While this bounded model guarantees a high degree of safety by eliminating distributed data races, it represents a significant constraint on the types of applications that can be fully lifted. Overcoming this limitation without sacrificing safety remains one of Monolift's primary research directives.

## **Contrasting Methodologies: The Rise and Fall of Service Weaver**

To understand the significance of Monolift's incremental approach, it must be contrasted with recent industry attempts at achieving similar goals, most notably Google's Service Weaver framework.6 Introduced as a programming framework for Go, Service Weaver allowed developers to write applications as modular monoliths, utilizing standard Go interfaces to define component boundaries.6

### **The Service Weaver Architecture**

Service Weaver abstracted away the boilerplate of distributed systems. Developers did not write protocol buffers, RPC stubs, or network routing logic.8 Instead, components interacted via standard method calls. When executing locally (via go run.), the application operated as a single binary. When deployed to a cluster (via weaver gke deploy or weaver kube deploy), the Service Weaver deployer dissected the binary along the defined component boundaries.6 The runtime handled the replication, autoscaling, and co-location of these distributed components, injecting synthetic network boundaries only when components were placed on separate machines.6

Despite its conceptual elegance and robust library support for logging, metrics, and tracing, Service Weaver ultimately failed to gain traction.6 Effective December 5, 2024, Google transitioned Service Weaver into maintenance mode, ceasing active feature development.10

### **The Cost of Migration and Polyglot Realities**

The primary cause of Service Weaver's demise was its immense migration cost.2 Although it was marketed as a simplified framework, adopting Service Weaver required developers to significantly rewrite existing applications to conform to its specific component lifecycle and interface requirements.2 The framework's API evolved rapidly—transitioning from constructor-based object initialization to interface-based component injection (weaver.Implements)—which created churn and instability for early adopters.12

Furthermore, Service Weaver struggled to integrate into the heterogeneous realities of enterprise architecture. It exclusively supported the Go programming language.13 In modern cloud-native systems, a single business workflow might traverse a Go-based API gateway, a Python-based machine learning inference service, and a Rust-based financial transaction engine.11 By forcing the entire logical monolith into Go, Service Weaver alienated organizations reliant on polyglot architectures.11

Monolift directly addresses the primary failure mode of Service Weaver by prioritizing zero-friction incremental adoption. Its use of transparent code comments ensures that no massive refactoring is required, allowing legacy code to be modernized piecemeal without locking the organization into a proprietary component framework.1

## **Historical Precedents: The Fallacy of Transparent RPC**

If Monolift claims to dynamically switch an application between local and distributed modes via a compiler pass, it bears the burden of proving that it will not repeat the catastrophic failures of 1990s distributed object middleware.1 To evaluate this, one must engage with the foundational text of distributed systems theory: the 1994 paper *A Note on Distributed Computing* by Jim Waldo, Geoff Wyant, Ann Wollrath, and Sam Kendall.3

### **The Four Fault Lines of Distributed Computing**

Waldo et al. argued that the industry periodically attempts to build "unified object models" that hide the network from the developer, and these attempts inevitably fail because local and distributed computing differ fundamentally across four axes 3:

1. **Latency:** Remote calls are orders of magnitude slower than local calls. If a compiler simply wraps a local function in an RPC without exposing this latency to the developer, performance profiles become highly unpredictable. An application expecting microsecond local execution will suffer from catastrophic thread starvation when faced with millisecond network round-trips.4  
2. **Memory Access:** In a local address space, pointers are valid, and memory is inherently shared. In a distributed system, memory is disjointed. Passing pointers across a network is fundamentally meaningless without complex, overhead-heavy distributed shared memory simulations.4  
3. **Concurrency:** Distributed systems are inherently concurrent. Components execute simultaneously, leading to race conditions, deadlocks, and synchronization challenges that do not exist in single-threaded local execution models.15  
4. **Partial Failure:** This is the most critical distinction. A local system either runs or crashes entirely. A distributed system can experience partial failures—a network link may sever, a remote switch may drop packets, or a remote node may panic while the caller remains active.3 It is impossible for the caller to distinguish between a slow network, a dead remote processor, or a crashed remote application.4

### **The Collapse of CORBA and DCOM**

Historically, middleware like CORBA (Common Object Request Broker Architecture), Microsoft's DCOM, and Java RMI attempted to provide network transparency.1 They allowed distributed objects to be invoked exactly as if they were local objects, generating extensive stub code to serialize data across the wire.14 These systems ultimately collapsed under their own weight because they successfully masked the network during the "happy path" but entirely failed to expose the realities of partial failure.4 When a network partitioned, the application-level logic was completely unprepared to handle the resulting timeouts, leading to inconsistent distributed state.16

Monolift’s architectural design implicitly acknowledges these historical failures. By enforcing a "bounded model" that prohibits shared state and heap sharing across lift points, Monolift actively prevents the pointer-chasing disasters that plagued CORBA.1 Furthermore, by relying on dynamic placement tied to real-time performance metrics, Monolift maintains a degree of operational awareness that early transparent RPC systems lacked.1 However, as Monolift evolves, it must explicitly expose partial failure mechanisms—perhaps through typed errors, required fallback closures, or promise pipelining 18—so that developers remain aware of the network boundary.

## **Multitier and Tierless Programming Architectures**

Monolift’s philosophical lineage is closely tied to multitier (or tierless) programming languages, an area of research focused on reducing the cognitive overhead of developing distributed applications by unifying them into a single compilation unit.1

In traditional software development, particularly web systems, functionality is artificially scattered across different tiers. This often requires multiple languages: JavaScript in the browser, Java or Go on the application server, and SQL in the database.20 This scattering leads to brittle communication protocols, immense serialization boilerplate, and a complete lack of holistic, cross-tier type safety.20 Tierless languages solve this by allowing the entire application to be written in one cohesive language, delegating the physical separation and communication to the compiler.19

### **Prominent Tierless Paradigms**

Several prominent tierless languages provide valuable insights into how Monolift might refine its placement guarantees and static analysis capabilities.

| Language | Paradigm | Core Innovation and Architecture | Relevance to Monolift |
| :---- | :---- | :---- | :---- |
| **ScalaLoci** 21 | Object-Oriented / Functional | Uses "placement types" (e.g., Local\[Client\]) to statically verify distributed data flows. Uses Scala macros (5.5K lines of code) to perform AST transformations and generate network code. | Provides a blueprint for statically verifying Monolift's dynamic delegate expressions via robust type systems. |
| **Links** 24 | Functional | Designed specifically for web programming. A single OCaml-based compiler translates code simultaneously into JavaScript (client), bytecode (server), and SQL (database). | Demonstrates the viability of multi-target compilation, directly relevant to Monolift's ambitions to compile to WASM or generic IRs. |
| **Ur/Web** 1 | Functional | Enforces rigorous type safety to mathematically prevent SQL injection, dead intra-application links, and XSS. Compiles to highly optimized, garbage-collection-free C code and JS. | Highlights the potential for compilers to enforce rigorous security and memory efficiency at distribution boundaries. |

The critical differentiator between these tierless languages and Monolift is the barrier to entry.1 Languages like Links and Ur/Web require greenfield development; an engineering team must abandon their existing codebase and learn a niche functional language.1 Monolift's innovation lies in retrofitting tierless concepts onto an existing, mainstream language (Go) using standard code comments, thereby eliminating the greenfield requirement and allowing for incremental, risk-free adoption.1

## **Formal Correctness: Choreographic Programming and Endpoint Projection**

If Monolift claims to dynamically switch an application between local and distributed modes based on delegate expressions, it bears the burden of mathematical proof that the distributed execution is semantically equivalent to the local monolith. This problem space is heavily researched under the umbrella of Choreographic Programming.1

In choreographic programming, developers write a "choreography"—a global, top-down description of the entire distributed system's behavior, specifying exactly how messages are exchanged between endpoints.29 A compiler then takes this unified choreography and automatically generates the local, executable code for each individual node. This generation process is known as Endpoint Projection (EPP).30

### **The EPP Theorem: Progress and Preservation**

The foundational guarantee of choreographic programming is the EPP Theorem, which mathematically proves that the projected endpoints behave exactly as described by the global choreography, without introducing deadlocks or unmatched messages.31 The EPP Theorem is generally established via two core lemmas 32:

1. **Type Preservation:** If a choreography is well-typed in the global view, its endpoint projections will also be perfectly well-typed. As the distributed system transitions through states and exchanges messages, these types are preserved, ensuring no unexpected data structures are received.33  
2. **Progress:** A well-typed choreography (and its resulting projected endpoints) will never get "stuck." It will either evaluate to a final value or continually make valid execution steps.32 This provides a mathematical guarantee of deadlock-freedom, as every send operation is guaranteed to have a corresponding receive operation.32

### **Mechanization and Implementations**

To ensure that the EPP compiler itself is devoid of bugs, researchers formalize these theories in theorem provers and advanced type systems:

* **Pirouette:** A higher-order functional choreographic language fully formalized in the Coq theorem prover.33 Pirouette mathematically proves deadlock freedom for its endpoint projections using a "select-and-merge" strategy, ensuring that different nodes stay in lock-step by projecting explicit synchronization messages.36  
* **HasChor:** An embedded domain-specific language (eDSL) in Haskell that leverages freer monads to express choreographies.30 HasChor proves that choreographic programming does not require a bespoke, standalone compiler; it can be implemented entirely at the library level by relying on the host language's monadic abstractions to handle dependency injection (EPP-as-DI) and message passing.30  
* **ChoRus & Choral:** ChoRus brings these concepts to Rust, utilizing "choreographic enclaves" to limit knowledge propagation and improve efficiency.40 Choral explores object-oriented choreographies, demonstrating how state and objects can be passed safely through higher-order choreographic parameters.41

For Monolift to move from an experimental prototype to an enterprise-grade infrastructure compiler, it must adopt the rigorous proofs of the EPP Theorem. By treating the un-annotated Go monolith as the "global choreography," the Monolift compiler's AST transformation phase is essentially an Endpoint Projection.1 If the Monolift team can mathematically model their Go AST transformations using the Coq techniques pioneered by Pirouette 37, they can offer users a strict mathematical guarantee: lifting a function will never introduce a distributed deadlock.

## **Actor Systems and Distributed State: The Pony Paradigm**

As Monolift looks to mature its runtime backend and relax its highly restrictive "bounded model," it must evaluate existing distributed architectures that handle state securely. The Actor Model represents the dominant, battle-tested paradigm for "run-it-distributed" systems.1 In the actor model, isolated computational entities (actors) communicate exclusively via asynchronous message passing, eliminating shared memory and, consequently, data races.44

Classical systems like Erlang/OTP utilize supervision trees for extreme fault tolerance, expecting failure and providing seamless recovery mechanisms.1 Akka brought these concepts to the JVM, heavily relying on location transparency, while Orleans pioneered the "virtual actor" model, where the runtime dynamically manages actor lifecycles, paging them into memory when messaged and out to storage when idle.1

### **Reference Capabilities and ORCA Garbage Collection**

To solve Monolift's state restrictions without inviting the concurrency disasters warned of by Waldo, the system should look to the Pony programming language.1 Pony is an actor-model, capability-secure language uniquely designed to be completely data-race-free without relying on locks.43 It achieves this via Reference Capabilities—a form of deny guarantees encoded directly into the type system.43

Pony defines strict mutability guarantees 48:

* iso (Isolated): The reference is mutable, but it is the *only* reference to that object. It can be safely consumed and passed to another actor over the network without copying.  
* val (Value): The reference is globally immutable. It can be shared across the distributed network infinitely because no actor can ever mutate it.  
* trn (Transition): A unique reference that allows local mutation but prevents other actors from reading or writing, useful for building data structures before freezing them into val.

Furthermore, Pony features the ORCA (Ownership and Reference Counting based Garbage Collection) protocol.47 Traditional distributed garbage collection requires massive synchronization overhead (stop-the-world pauses or complex read/write barriers). ORCA avoids this entirely.49 It operates by associating a "weight" with reference counts. When an actor sends an object in a message, it splits the reference weight, sending a portion to the receiving actor.50 The system simply tracks when the global weight reaches zero, allowing actors to independently collect their own objects without global synchronization barriers.49

Integrating Pony's capability semantics into Monolift's static analyzer could provide the theoretical foundation needed to allow complex object sharing across lift points. If Monolift can prove an object is iso or val, it can serialize it across the network safely, expanding the bounded model while maintaining mathematical safety.

## **Modern Dynamic Placement and Orchestration Runtimes**

If Monolift is to abstract placement effectively, it can compile its deployments to utilize modern, sophisticated clustered runtimes rather than relying entirely on raw Kubernetes pod management.1

### **Ray: Clustered Scheduling and Autoscaling**

Ray is an open-source framework designed for scaling AI and complex Python workloads, treating a cluster of machines as a single unified compute resource.51 Ray's architecture relies on a Controller (managing the control plane), HTTP/gRPC Proxies for routing, and Worker replicas that execute the actual tasks or actor methods.51

Critically for Monolift, Ray features a highly sophisticated scheduling engine.54 Ray categorizes nodes as "feasible" or "infeasible" based on strict resource requirements (e.g., CPU, GPU, memory).54 It then applies scheduling strategies 54:

* **PACK Strategy:** Ray attempts to colocate tasks onto the same node to minimize network latency and maximize data locality.  
* **SPREAD Strategy:** Ray distributes tasks across all available nodes to maximize resource usage and parallelism.  
* **NodeAffinitySchedulingStrategy:** Provides hard (soft=False) or soft (soft=True) placement constraints based on specific node IDs or labels.

Ray’s Autoscaler seamlessly requests resources from the underlying infrastructure (like Kubernetes or Spark), scaling up worker nodes when tasks are pending and scaling down when idle.56 Monolift could vastly simplify its deployment topology by compiling its delegate expressions directly into Ray Placement Group API calls, offloading the physical placement optimization entirely to Ray's global scheduler.

### **Temporal: Durable Execution and History Replay**

To address the partial failure vulnerabilities inherent in distributed systems (as highlighted by Waldo), Monolift could target Temporal.1 Temporal is a durable execution platform that renders applications functionally crash-proof.58

Temporal separates execution into two paradigms 61:

1. **Workflows:** Deterministic orchestration logic that defines the business process.  
2. **Activities:** Side-effecting operations (e.g., charging a credit card, updating a database).

Temporal relies on an event-sourcing backend managed by a centralized server cluster.63 Workers pull tasks from heavily partitioned Task Queues.61 As an activity completes, its result is durably written to the event history. If a worker node crashes mid-execution due to a partial failure, the Temporal server simply dispatches the workflow to a new worker. The new worker replays the event history, skipping the already-completed activities, and resumes execution exactly where the previous node failed.58 Compiling Monolift lift points into Temporal Activities would entirely shield the developer from network timeouts and partial failures, providing true fault-tolerance.

## **Automated Program Partitioning: From Heuristics to Machine Learning**

A critical unresolved fault line in Monolift is the balance between manual annotation and automated inference.1 Currently, a developer must explicitly annotate a lift point using //monolift:offload.1 The holy grail of compiler-driven distribution is the ability of the system to infer exactly *what* should be partitioned and *where* it should be placed without human intervention.1

### **Legacy Heuristics: Coign and MAUI**

The pursuit of automatic partitioning spans decades, with several prominent milestones informing current approaches:

* **Coign (1999):** Coign was an Automatic Distributed Partitioning System (ADPS) that operated directly on COM binaries without requiring source code access.66 By manipulating the dynamic link library (DLL) import table to load its runtime, Coign extensively profiled the application's network and inter-component communication during simulated execution scenarios.67 It built an abstract profile analysis graph and applied a graph-cutting algorithm to minimize network communication delay, automatically relocating components between clients and servers.66  
* **MAUI (2010):** MAUI was designed to maximize smartphone battery life through fine-grained code offloading.69 MAUI continuously utilized device, program, and network profilers to measure battery state, execution duration, and network latency (WiFi vs. 3G RTT).70 To determine which methods to offload to an edge server, MAUI formulated the placement decision as a **0-1 Integer Linear Programming (ILP)** problem.1 Because solving NP-hard ILP equations at runtime is computationally expensive, MAUI utilized solvers to approximate the optimal energy-saving global configuration dynamically.72

### **Modern Machine Learning Approaches: IBM Mono2Micro**

In the modern enterprise space, IBM's Mono2Micro represents the state-of-the-art for decomposing massive monolithic Java applications.74 Mono2Micro eschews simple graph cutting in favor of AI-driven, spatio-temporal decomposition.76

The process begins by using a tool called Bluejay to instrument the monolith's source code, generating static analysis metadata.78 Next, the application is executed through standard business test cases (using a tool called Flicker) to capture dynamic runtime execution traces.78 Utilizing unsupervised machine learning clustering algorithms, the AI engine processes this data to generate two distinct types of partition recommendations 74:

1. **Business-logic-seams-based partitions:** Clustered based on functional cohesion and use-case execution.  
2. **Natural-seams-based partitions:** Clustered to strictly respect underlying data containment and inheritance dependencies, preventing massive database refactoring.74

**Insight Synthesis for Monolift:** Monolift's current reliance on "delegate expressions" requires the developer to hardcode the threshold for distribution. This leads to the "global transition function" problem: how does the system balance a network of hundreds of lift points simultaneously without causing cascading placement thrashing?.1 By synthesizing Coign's graph theory, MAUI's 0-1 ILP solvers, and Mono2Micro's ML clustering, Monolift could build an offline profiling daemon. This daemon would profile the application in a staging environment, solve the ILP optimization equations offline, and automatically inject mathematically optimized delegate expressions into the AST during compilation, achieving truly zero-touch, optimized distribution.

## **Far-Memory Offloading and Disaggregated Infrastructure**

Beyond distributing workloads across a standard Kubernetes cluster, Monolift's compiler architecture makes it uniquely suited to target next-generation hardware paradigms, particularly resource disaggregation and near-data computing.1 In disaggregated data centers (DDCs), compute, memory, and storage are not housed in monolithic servers; they are physically separated into distinct hardware pools connected by ultra-high-speed networks.81

### **The TELEPORT Abstraction and LegoOS**

In a DDC, pulling massive amounts of data from the memory pool to the compute pool to perform simple analytical filtering creates severe network bottlenecks.84

* **TELEPORT:** This framework addresses the bottleneck by providing a "compute pushdown" mechanism.81 TELEPORT introduces a novel system call: pushdown(fn, arg, flags).81 When invoked, TELEPORT migrates the entire execution context of a user process—including the page table mappings, program stack, and text segment—directly to the remote controller of the memory pool.81 This allows arbitrary C functions to execute directly adjacent to the data in memory, entirely eliminating the cost of data movement over the network, achieving massive speedups on in-memory databases like MonetDB.81 TELEPORT is unique in that it maintains unfettered access to the original virtual address space.81  
* **LegoOS:** Taking disaggregation to the operating system level, LegoOS splits traditional monolithic OS functionalities into loosely coupled, network-communicating monitors tailored for specific hardware (e.g., CPU monitors, Memory monitors).82 LegoOS treats the network as the primary interconnect, proving that highly distributed internal OS components can maintain the illusion of a cohesive execution environment.87

### **Offload Annotations for Heterogeneous Hardware**

Similar logic applies to hardware accelerators. Proposed at USENIX ATC 2020, Offload Annotations (OAs) allow developers to annotate standard CPU-bound Python libraries (like Numpy) with equivalents tailored for heterogeneous hardware accelerators (like GPUs or TPUs).89 The OA runtime automatically manages the partitioning of input data arrays (NdArraySplit), memory transfers across the PCIe bus, and execution scheduling, allowing legacy code to achieve hardware-accelerated speeds with minimal modification.91

**Insight Synthesis:** Monolift's //monolift:offload annotation is conceptually identical to the Offload Annotations model.1 By extending its compiler backends, Monolift could treat a TELEPORT-enabled memory pool or a remote GPU cluster as just another backend placement target. A Monolift delegate expression could evaluate the size of a data payload; if the payload exceeds a specific megabyte threshold, the compiler could translate the lift point into a TELEPORT pushdown() syscall rather than a standard HTTP RPC, flawlessly blending application-level service distribution with hardware-level memory disaggregation.

## **Conclusions and Future Directives**

The pursuit of distributed computing has perpetually vacillated between the allure of network transparency and the brutal realities of partial failure and state management. From the catastrophic pointer-chasing of CORBA to the rigid, boilerplate-heavy topologies of modern microservices, the industry has struggled to find a middle ground that balances developer ergonomics with robust, scalable infrastructure.

Monolift represents a highly promising synthesis of decades of systems research. By treating distribution as an incrementally adoptable compiler pass, it circumvents the fatal migration costs that doomed predecessors like Service Weaver. Its use of dynamic delegate expressions provides the operational elasticity required by modern cloud-native environments, allowing applications to organically scale and contract across existing infrastructure.

However, to evolve from a compelling research prototype into an industry-standard architecture, Monolift must aggressively resolve its remaining fault lines by integrating the broader ecosystem's innovations:

1. **Mathematical Correctness:** It must look to choreographic programming, utilizing Coq formalizations to mathematically guarantee deadlock-freedom and type preservation during AST projection.  
2. **State Management:** It must incorporate the strict reference capabilities (iso, val) and weight-based ORCA garbage collection of the Pony language to safely navigate the boundaries of shared state, moving beyond its current overly restrictive bounded model.  
3. **Automated Placement Optimization:** It must transition from manual heuristic annotations to AI-driven, 0-1 ILP-optimized automated inference, borrowing the spatio-temporal clustering algorithms pioneered by Mono2Micro and MAUI to solve the global transition function.  
4. **Platform Agnosticism:** Finally, Monolift should shift its compilation target from native Go binaries to WebAssembly (WASM) or a similar Intermediate Representation (IR). By compiling lift points into WASM modules, Monolift could seamlessly transition workloads across heterogeneous environments, fulfilling the promise of a truly "write once, run anywhere... distributed" ecosystem.

## ---

**Reference and Artifact Directory**

As specifically requested, the following directory details the foundational papers, artifacts, and GitHub repositories synthesized within this research report. This index serves as a comprehensive resource for downloading source materials, analyzing historical precedents, and accessing the open-source implementations driving compiler-driven distribution and placement logic.

| Research Domain | Artifact / Paper Title | Primary URLs / Repository Context | Description |
| :---- | :---- | :---- | :---- |
| **Compiler-Driven Distribution** | Monolift: Automating Distribution With the Tools You Have at Home (PLOS '25) | 2, 2, 2 | Introduces the Monolift architecture, dual-dispatch lift points via //monolift:offload, and dynamic delegate expressions for Go programs. |
| **Modular Monoliths** | Service Weaver Framework | github.com/ServiceWeaver/weaver 6 | Google's Go framework utilizing interfaces for distributed modularity. Transitioned to maintenance mode Dec 2024\. |
| **Distributed Systems Theory** | A Note on Distributed Computing (1994) | waldo.scholars.harvard.edu/publications 3 | Foundational text by Waldo et al. identifying the four fault lines: latency, memory access, concurrency, and partial failure. |
| **Tierless Programming** | A Survey of Multitier Programming (ACM CSUR 2020\) | \[8, 19, 20\] | Comprehensive taxonomy of tierless languages. |
| **Tierless Programming** | ScalaLoci | github.com/scala-loci/scala-loci \[23, 93\] | Scala-embedded language using macros and placement types for distributed data flow verification. |
| **Tierless Programming** | Links & Ur/Web | github.com/links-lang/links \[24\], github.com/urweb/urweb \[27\] | Functional languages compiling to JS, bytecode, C, and SQL simultaneously, guaranteeing web security. |
| **Choreographic Programming** | Pirouette | \[36, 37, 38\] | Higher-order functional choreographies formalized in Coq to prove the EPP Theorem (deadlock-freedom). |
| **Choreographic Programming** | HasChor | github.com/gshen42/HasChor 30 | Haskell library implementing choreographies via freer monads and EPP-as-Dependency-Injection. |
| **Choreographic Programming** | ChoRus & Choral | \[40, 41, 42\] | Implementations exploring choreographic enclaves in Rust and object-oriented parameterization. |
| **Actor Models & State** | Pony Language & ORCA GC Protocol | github.com/ponylang/ponyc, \[44, 47, 49, 50\] | Capability-secure actor language utilizing iso/val references and completely concurrent weight-based garbage collection. |
| **Dynamic Placement** | Ray Framework | docs.ray.io, github.com/ray-project/ray \[51, 55, 56\] | Distributed compute framework featuring PACK/SPREAD placement groups and a dedicated autoscaler. |
| **Durable Execution** | Temporal | docs.temporal.io, github.com/temporalio/temporal \[58, 61, 62\] | Platform utilizing event-sourcing task queues and history replay to ensure crash-proof workflow execution. |
| **Automated Partitioning** | Coign | \[66, 67, 68\] | 1999 system for automatic distributed partitioning of COM binaries via graph-cutting. |
| **Automated Partitioning** | MAUI | \[69, 70, 73\] | Smartphone code offload system utilizing 0-1 Integer Linear Programming (ILP) solvers to optimize battery life. |
| **Automated Partitioning** | IBM Mono2Micro | ibm.com/docs/en/mono2micro \[74, 79, 94\] | AI-driven toolkit leveraging static analysis and runtime traces to cluster Java monoliths into microservices. |
| **Resource Disaggregation** | TELEPORT | github.com/eniac/TELEPORT \[81, 84, 85\] | Memory disaggregation primitive providing the pushdown() syscall for zero-copy far-memory execution. |
| **Resource Disaggregation** | LegoOS | github.com/Wuklab/LegoOS 82 | A disseminated operating system splitting kernel monitors across network-attached hardware resources. |
| **Heterogeneous Compute** | Offload Annotations | github.com/stanford-futuredata/offload-annotations \[89, 91, 95\] | Python decorators automatically partitioning data and routing compute to GPU/TPU accelerators. |

#### **Works cited**

1. RESEARCH\_BRIEF.md  
2. Monolift: Automating Distribution With the Tools You Have at Home, accessed April 18, 2026, [https://users.soe.ucsc.edu/\~lkuper/papers/monolift-plos25.pdf](https://users.soe.ucsc.edu/~lkuper/papers/monolift-plos25.pdf)  
3. Publications | Jim Waldo \- Harvard University, accessed April 18, 2026, [https://waldo.scholars.harvard.edu/publications](https://waldo.scholars.harvard.edu/publications)  
4. Remote Procedure Call \- Christopher Meiklejohn, accessed April 18, 2026, [https://christophermeiklejohn.com/pl/2016/04/12/rpc.html](https://christophermeiklejohn.com/pl/2016/04/12/rpc.html)  
5. Introduction \- Kotlin language specification, accessed April 18, 2026, [https://kotlinlang.org/spec/kotlin-spec.html](https://kotlinlang.org/spec/kotlin-spec.html)  
6. Service Weaver Docs, accessed April 18, 2026, [https://serviceweaver.dev/docs.html](https://serviceweaver.dev/docs.html)  
7. Towards Modern Development of Cloud Applications \- ResearchGate, accessed April 18, 2026, [https://www.researchgate.net/publication/371791360\_Towards\_Modern\_Development\_of\_Cloud\_Applications](https://www.researchgate.net/publication/371791360_Towards_Modern_Development_of_Cloud_Applications)  
8. Introducing Service Weaver: A Framework for Writing Distributed Applications, accessed April 18, 2026, [https://opensource.googleblog.com/2023/03/introducing-service-weaver-framework-for-writing-distributed-applications.html](https://opensource.googleblog.com/2023/03/introducing-service-weaver-framework-for-writing-distributed-applications.html)  
9. weaver package \- github.com/ServiceWeaver/weaver \- Go Packages, accessed April 18, 2026, [https://pkg.go.dev/github.com/ServiceWeaver/weaver](https://pkg.go.dev/github.com/ServiceWeaver/weaver)  
10. GitHub \- ServiceWeaver/weaver: Programming framework for writing and deploying cloud applications., accessed April 18, 2026, [https://github.com/ServiceWeaver/weaver](https://github.com/ServiceWeaver/weaver)  
11. What Did Service Weaver Miss \- Medium, accessed April 18, 2026, [https://medium.com/@xiafan/what-did-service-weaver-miss-f1e58decfca7](https://medium.com/@xiafan/what-did-service-weaver-miss-f1e58decfca7)  
12. A History of Service Weaver's Core API, accessed April 18, 2026, [https://serviceweaver.dev/blog/history.html](https://serviceweaver.dev/blog/history.html)  
13. Service Weaver: A Promising Direction for Cloud-Native Systems? \- SciTePress, accessed April 18, 2026, [https://www.scitepress.org/Papers/2024/126245/126245.pdf](https://www.scitepress.org/Papers/2024/126245/126245.pdf)  
14. Distributed object \- Wikipedia, accessed April 18, 2026, [https://en.wikipedia.org/wiki/Distributed\_object](https://en.wikipedia.org/wiki/Distributed_object)  
15. \[PDF\] A Note on Distributed Computing \- Semantic Scholar, accessed April 18, 2026, [https://www.semanticscholar.org/paper/A-Note-on-Distributed-Computing-Waldo-Wyant/778e4bbdea4e35f3889e48bfba7c951ed3b43b54](https://www.semanticscholar.org/paper/A-Note-on-Distributed-Computing-Waldo-Wyant/778e4bbdea4e35f3889e48bfba7c951ed3b43b54)  
16. The 40-Year Evolution of RPC: From Simple Procedure Calls to Modern Microservices, accessed April 18, 2026, [https://medium.com/@amazing\_gs/the-40-year-evolution-of-rpc-from-simple-procedure-calls-to-modern-microservices-7ac410bf5df2](https://medium.com/@amazing_gs/the-40-year-evolution-of-rpc-from-simple-procedure-calls-to-modern-microservices-7ac410bf5df2)  
17. Distributed Systems \- Department of Computer Science and Technology |, accessed April 18, 2026, [https://www.cl.cam.ac.uk/teaching/2122/ConcDisSys/dist-sys-notes.pdf](https://www.cl.cam.ac.uk/teaching/2122/ConcDisSys/dist-sys-notes.pdf)  
18. A Note on Distributed Computing (1994) \[pdf\] | Hacker News, accessed April 18, 2026, [https://news.ycombinator.com/item?id=34245875](https://news.ycombinator.com/item?id=34245875)  
19. A Survey of Multitier Programming | Request PDF \- ResearchGate, accessed April 18, 2026, [https://www.researchgate.net/publication/342761110\_A\_Survey\_of\_Multitier\_Programming](https://www.researchgate.net/publication/342761110_A_Survey_of_Multitier_Programming)  
20. A Survey of Multitier Programming \- Programming Group, accessed April 18, 2026, [https://programming-group.com/assets/pdf/papers/2020\_A-Survey-of-Multitier-Programming.pdf](https://programming-group.com/assets/pdf/papers/2020_A-Survey-of-Multitier-Programming.pdf)  
21. Distributed system development with ScalaLoci, accessed April 18, 2026, [https://decomposition.al/CSE290S-2023-01/readings/scalaloci.pdf](https://decomposition.al/CSE290S-2023-01/readings/scalaloci.pdf)  
22. Implementing a Language for Distributed Systems: Choices and Experiences with Type Level and Macro Programming in Scala \- arXiv, accessed April 18, 2026, [https://arxiv.org/pdf/2002.06184](https://arxiv.org/pdf/2002.06184)  
23. GitHub \- scala-loci/scala-loci: A programming language for distributed applications, accessed April 18, 2026, [https://github.com/scala-loci/scala-loci](https://github.com/scala-loci/scala-loci)  
24. The Links Programming Language \- GitHub, accessed April 18, 2026, [https://github.com/links-lang](https://github.com/links-lang)  
25. links/dune-project at master · links-lang/links · GitHub, accessed April 18, 2026, [https://github.com/links-lang/links/blob/master/dune-project](https://github.com/links-lang/links/blob/master/dune-project)  
26. links-lang/links: Links: Linking Theory to Practice for the Web \- GitHub, accessed April 18, 2026, [https://github.com/links-lang/links](https://github.com/links-lang/links)  
27. GitHub \- urweb/urweb: The Ur/Web programming language, accessed April 18, 2026, [https://github.com/urweb/urweb](https://github.com/urweb/urweb)  
28. docelic/awesome-urweb: Collection of awesome Ur/Web libraries, components, and projects · GitHub, accessed April 18, 2026, [https://github.com/docelic/awesome-urweb](https://github.com/docelic/awesome-urweb)  
29. "Choreographic Programming", accessed April 18, 2026, [https://pure.itu.dk/ws/files/78733848/m13\_phd.pdf](https://pure.itu.dk/ws/files/78733848/m13_phd.pdf)  
30. arXiv:2303.00924v2 \[cs.PL\] 19 Jul 2023, accessed April 18, 2026, [https://arxiv.org/pdf/2303.00924](https://arxiv.org/pdf/2303.00924)  
31. arXiv:2303.03972v1 \[cs.PL\] 7 Mar 2023 \- Fabrizio Montesi, accessed April 18, 2026, [https://www.fabriziomontesi.com/files/clm23-arxiv.pdf](https://www.fabriziomontesi.com/files/clm23-arxiv.pdf)  
32. Choreographic Quick Changes: First-Class Location (Set) Polymorphism \- Ethan Cecchetti, accessed April 18, 2026, [https://cecchetti.sites.cs.wisc.edu/papers/lam-qc.pdf](https://cecchetti.sites.cs.wisc.edu/papers/lam-qc.pdf)  
33. A model for correlation-based choreographic programming \- PMC \- NIH, accessed April 18, 2026, [https://pmc.ncbi.nlm.nih.gov/articles/PMC11784539/](https://pmc.ncbi.nlm.nih.gov/articles/PMC11784539/)  
34. Functional Choreographic Programming \- arXiv, accessed April 18, 2026, [https://arxiv.org/pdf/2111.03701](https://arxiv.org/pdf/2111.03701)  
35. "Choreographic Programming" \- Fabrizio Montesi, accessed April 18, 2026, [https://www.fabriziomontesi.com/files/choreographic\_programming.pdf](https://www.fabriziomontesi.com/files/choreographic_programming.pdf)  
36. Technical Report MPI-SWS-2021-004 November 4, 2021Pirouette: Higher-Order Typed Functional Choreographies \- Max Planck Institute for Software Systems, accessed April 18, 2026, [https://www.mpi-sws.org/tr/2021-004.pdf](https://www.mpi-sws.org/tr/2021-004.pdf)  
37. The Concurrent Calculi Formalisation Benchmark \- Alberto Momigliano, accessed April 18, 2026, [https://momigliano.di.unimi.it/papers/cbench.pdf](https://momigliano.di.unimi.it/papers/cbench.pdf)  
38. A New Architecture for Choreographic Programming Languages \- UVM ScholarWorks, accessed April 18, 2026, [https://scholarworks.uvm.edu/bitstreams/24e2c821-9ee4-4e98-b7f2-a7edacfc394a/download](https://scholarworks.uvm.edu/bitstreams/24e2c821-9ee4-4e98-b7f2-a7edacfc394a/download)  
39. gshen42/HasChor: Functional choreographic programming in Haskell \- GitHub, accessed April 18, 2026, [https://github.com/gshen42/HasChor](https://github.com/gshen42/HasChor)  
40. \[2311.11472\] Portable, Efficient, and Practical Library-Level Choreographic Programming \- arXiv, accessed April 18, 2026, [https://arxiv.org/abs/2311.11472](https://arxiv.org/abs/2311.11472)  
41. Real-World Choreographic Programming: An Experience Report \- arXiv, accessed April 18, 2026, [https://arxiv.org/pdf/2303.03983](https://arxiv.org/pdf/2303.03983)  
42. Alice or Bob?: Process polymorphism in choreographies | Journal of Functional Programming | Cambridge Core, accessed April 18, 2026, [https://www.cambridge.org/core/journals/journal-of-functional-programming/article/alice-or-bob-process-polymorphism-in-choreographies/382AD3B58F86FF95AB59DDF0EDE96F65](https://www.cambridge.org/core/journals/journal-of-functional-programming/article/alice-or-bob-process-polymorphism-in-choreographies/382AD3B58F86FF95AB59DDF0EDE96F65)  
43. Pony: Co-Designing a Type System and a Runtime \- YouTube, accessed April 18, 2026, [https://www.youtube.com/watch?v=R6T8ytKV6dc](https://www.youtube.com/watch?v=R6T8ytKV6dc)  
44. A String of Ponies, accessed April 18, 2026, [https://www.ponylang.io/media/papers/a\_string\_of\_ponies.pdf](https://www.ponylang.io/media/papers/a_string_of_ponies.pdf)  
45. A Principled Design of Capabilities in Pony, accessed April 18, 2026, [https://www.imperial.ac.uk/media/imperial-college/faculty-of-engineering/computing/public/GeorgeSteed.pdf](https://www.imperial.ac.uk/media/imperial-college/faculty-of-engineering/computing/public/GeorgeSteed.pdf)  
46. The Road we didn't go down \- armstrong on software, accessed April 18, 2026, [http://armstrongonsoftware.blogspot.com/2008/05/road-we-didnt-go-down.html](http://armstrongonsoftware.blogspot.com/2008/05/road-we-didnt-go-down.html)  
47. Papers \- Pony, accessed April 18, 2026, [https://www.ponylang.io/learn/papers/](https://www.ponylang.io/learn/papers/)  
48. A Comparison of the Capability Systems of Encore, Pony and Rust \- Diva-Portal.org, accessed April 18, 2026, [https://www.diva-portal.org/smash/get/diva2:1363822/FULLTEXT01.pdf](https://www.diva-portal.org/smash/get/diva2:1363822/FULLTEXT01.pdf)  
49. Orca: GC and type system co-design for actor languages | Request PDF \- ResearchGate, accessed April 18, 2026, [https://www.researchgate.net/publication/345056846\_Orca\_GC\_and\_type\_system\_co-design\_for\_actor\_languages](https://www.researchgate.net/publication/345056846_Orca_GC_and_type_system_co-design_for_actor_languages)  
50. Ownership and Reference Counting based Garbage Collection in the Actor World \- Imperial College London, accessed April 18, 2026, [https://www.doc.ic.ac.uk/\~scd/icooolps15\_GC.pdf](https://www.doc.ic.ac.uk/~scd/icooolps15_GC.pdf)  
51. Architecture — Ray 2.54.1, accessed April 18, 2026, [https://docs.ray.io/en/latest/serve/architecture.html](https://docs.ray.io/en/latest/serve/architecture.html)  
52. Getting Started — Ray 2.55.0, accessed April 18, 2026, [https://docs.ray.io/en/latest/ray-overview/getting-started.html](https://docs.ray.io/en/latest/ray-overview/getting-started.html)  
53. Ray: A Distributed Framework for Emerging AI Applications \- USENIX, accessed April 18, 2026, [https://www.usenix.org/system/files/osdi18-moritz.pdf](https://www.usenix.org/system/files/osdi18-moritz.pdf)  
54. Scheduling — Ray 2.55.0 \- Ray Docs \- Ray.io, accessed April 18, 2026, [https://docs.ray.io/en/latest/ray-core/scheduling/index.html](https://docs.ray.io/en/latest/ray-core/scheduling/index.html)  
55. Placement Groups — Ray 2.55.0, accessed April 18, 2026, [https://docs.ray.io/en/latest/ray-core/scheduling/placement-group.html](https://docs.ray.io/en/latest/ray-core/scheduling/placement-group.html)  
56. What's Ray Core? — Ray 2.55.0 \- Ray Docs, accessed April 18, 2026, [https://docs.ray.io/en/latest/ray-core/walkthrough.html](https://docs.ray.io/en/latest/ray-core/walkthrough.html)  
57. Announcing Ray Autoscaling support on Databricks and Apache Spark, accessed April 18, 2026, [https://www.databricks.com/blog/announcing-ray-autoscaling-support-databricks-and-apache-sparktm](https://www.databricks.com/blog/announcing-ray-autoscaling-support-databricks-and-apache-sparktm)  
58. Temporal Docs | Temporal Platform Documentation, accessed April 18, 2026, [https://docs.temporal.io/](https://docs.temporal.io/)  
59. Understanding Temporal | Temporal Platform Documentation, accessed April 18, 2026, [https://docs.temporal.io/evaluate/understanding-temporal](https://docs.temporal.io/evaluate/understanding-temporal)  
60. The definitive guide to Durable Execution \- Temporal, accessed April 18, 2026, [https://temporal.io/blog/what-is-durable-execution](https://temporal.io/blog/what-is-durable-execution)  
61. What is a Temporal Worker?, accessed April 18, 2026, [https://docs.temporal.io/workers](https://docs.temporal.io/workers)  
62. Reliable data processing: Queues and Workflows \- Temporal, accessed April 18, 2026, [https://temporal.io/blog/reliable-data-processing-queues-workflows](https://temporal.io/blog/reliable-data-processing-queues-workflows)  
63. Building Resilient Distributed Systems with Temporal and AWS, accessed April 18, 2026, [https://aws.amazon.com/blogs/apn/building-resilient-distributed-systems-with-temporal-and-aws/](https://aws.amazon.com/blogs/apn/building-resilient-distributed-systems-with-temporal-and-aws/)  
64. System Design: A Breakdown of Temporal's Internal Architecture by Sanil Khurana | Data Science Collective \- Medium, accessed April 18, 2026, [https://medium.com/data-science-collective/system-design-series-a-step-by-step-breakdown-of-temporals-internal-architecture-52340cc36f30](https://medium.com/data-science-collective/system-design-series-a-step-by-step-breakdown-of-temporals-internal-architecture-52340cc36f30)  
65. Mastering Durable Execution in Distributed Systems \- Temporal, accessed April 18, 2026, [https://temporal.io/blog/durable-execution-in-distributed-systems-increasing-observability](https://temporal.io/blog/durable-execution-in-distributed-systems-increasing-observability)  
66. The Coign Automatic Distributed Partitioning System \- USENIX, accessed April 18, 2026, [https://www.usenix.org/conference/osdi-99/coign-automatic-distributed-partitioning-system](https://www.usenix.org/conference/osdi-99/coign-automatic-distributed-partitioning-system)  
67. The Coign Automatic Distributed Partitioning System \- Microsoft, accessed April 18, 2026, [https://www.microsoft.com/en-us/research/wp-content/uploads/2016/02/huntosdi99.pdf](https://www.microsoft.com/en-us/research/wp-content/uploads/2016/02/huntosdi99.pdf)  
68. (PDF) The Coign Automatic Distributed Partitioning System \- ResearchGate, accessed April 18, 2026, [https://www.researchgate.net/publication/2392926\_The\_Coign\_Automatic\_Distributed\_Partitioning\_System](https://www.researchgate.net/publication/2392926_The_Coign_Automatic_Distributed_Partitioning_System)  
69. MAUI: Making Smartphones Last Longer with Code Offload \- UCLA Computer Science Department, accessed April 18, 2026, [http://web.cs.ucla.edu/\~ravi/CS219\_F19/papers/maui.pdf](http://web.cs.ucla.edu/~ravi/CS219_F19/papers/maui.pdf)  
70. (PDF) MAUI: Making smartphones last longer with code offload \- ResearchGate, accessed April 18, 2026, [https://www.researchgate.net/publication/221234509\_MAUI\_Making\_smartphones\_last\_longer\_with\_code\_offload](https://www.researchgate.net/publication/221234509_MAUI_Making_smartphones_last_longer_with_code_offload)  
71. MAUI: Making Smartphones Last Longer With Code Offload \- Semantic Scholar, accessed April 18, 2026, [https://pdfs.semanticscholar.org/4545/22172d080946d490017f631267001658cd47.pdf](https://pdfs.semanticscholar.org/4545/22172d080946d490017f631267001658cd47.pdf)  
72. Elicit: Efficiently Identify Computation-intensive Tasks in Mobile Applications for Offloading, accessed April 18, 2026, [https://mason.gmu.edu/\~mhassanb/Elicit.pdf](https://mason.gmu.edu/~mhassanb/Elicit.pdf)  
73. an efficient code partition algorithm for mobile cloud computing \- Lei Jiao, accessed April 18, 2026, [https://ljiao.github.io/papers/cloudnet12.pdf](https://ljiao.github.io/papers/cloudnet12.pdf)  
74. Mono2Micro: An AI-based toolchain for evolving monolithic enterprise applications to a microservice architecture for ESEC/FSE 2020 \- IBM Research, accessed April 18, 2026, [https://research.ibm.com/publications/mono2micro-an-ai-based-toolchain-for-evolving-monolithic-enterprise-applications-to-a-microservice-architecture](https://research.ibm.com/publications/mono2micro-an-ai-based-toolchain-for-evolving-monolithic-enterprise-applications-to-a-microservice-architecture)  
75. Mono2Micro: A Practical and Effective Tool for Decomposing Monolithic Java Applications to Microservices for ESEC/FSE 2021 \- IBM Research, accessed April 18, 2026, [https://research.ibm.com/publications/mono2micro-a-practical-and-effective-tool-for-decomposing-monolithic-java-applications-to-microservices](https://research.ibm.com/publications/mono2micro-a-practical-and-effective-tool-for-decomposing-monolithic-java-applications-to-microservices)  
76. (PDF) Mono2Micro: a practical and effective tool for decomposing monolithic Java applications to microservices \- ResearchGate, accessed April 18, 2026, [https://www.researchgate.net/publication/354057927\_Mono2Micro\_a\_practical\_and\_effective\_tool\_for\_decomposing\_monolithic\_Java\_applications\_to\_microservices](https://www.researchgate.net/publication/354057927_Mono2Micro_a_practical_and_effective_tool_for_decomposing_monolithic_Java_applications_to_microservices)  
77. Mono2Micro: A Practical and Effective Tool for Decomposing Monolithic Java Applications to Microservices \- arXiv, accessed April 18, 2026, [https://arxiv.org/pdf/2107.09698](https://arxiv.org/pdf/2107.09698)  
78. Modernization Playbook – Getting Started with IBM Mono2Micro, accessed April 18, 2026, [https://ibm-cloud-architecture.github.io/modernization-playbook/applications/m2m/](https://ibm-cloud-architecture.github.io/modernization-playbook/applications/m2m/)  
79. Downloading and installing IBM Mono2Micro, accessed April 18, 2026, [https://www.ibm.com/docs/en/mono2micro?topic=mono2micro-downloading-installing](https://www.ibm.com/docs/en/mono2micro?topic=mono2micro-downloading-installing)  
80. Migrating to Liberty \- IBM, accessed April 18, 2026, [https://www.ibm.com/docs/en/websphere-hybrid?topic=cloud-migrating-liberty](https://www.ibm.com/docs/en/websphere-hybrid?topic=cloud-migrating-liberty)  
81. Optimizing Data-intensive Systems in Disaggregated Data Centers with TELEPORT \- Computer and Information Science, accessed April 18, 2026, [https://www.cis.upenn.edu/\~sga001/papers/teleport-sigmod22.pdf](https://www.cis.upenn.edu/~sga001/papers/teleport-sigmod22.pdf)  
82. WukLab/LegoOS: Disseminated, Distributed OS for Hardware Resource Disaggregation. USENIX OSDI 2018 Best Paper. \- GitHub, accessed April 18, 2026, [https://github.com/Wuklab/LegoOS](https://github.com/Wuklab/LegoOS)  
83. A collection of awesome researchers and papers about disaggregated memory. \- GitHub, accessed April 18, 2026, [https://github.com/dmemsys/awesome-disaggregated-memory](https://github.com/dmemsys/awesome-disaggregated-memory)  
84. HYPERSCALE DATA PROCESSING WITH NETWORK-CENTRIC DESIGNS Qizhen Zhang A DISSERTATION in Computer and Information Science Presente \- NetDB@Penn, accessed April 18, 2026, [https://netdb.cis.upenn.edu/wp-content/uploads/2022/08/Qizhen\_Zhang\_Thesis.pdf](https://netdb.cis.upenn.edu/wp-content/uploads/2022/08/Qizhen_Zhang_Thesis.pdf)  
85. Disaggregated Database Systems \- CS@Purdue, accessed April 18, 2026, [https://www.cs.purdue.edu/homes/csjgwang/pubs/SIGMOD23\_Tutorial\_DisaggregatedDB.pdf](https://www.cs.purdue.edu/homes/csjgwang/pubs/SIGMOD23_Tutorial_DisaggregatedDB.pdf)  
86. Memory-disaggregated DBMSs \- Far Data Lab, accessed April 18, 2026, [https://fardatalab.org/sigmod23-tutorial-slides.pdf](https://fardatalab.org/sigmod23-tutorial-slides.pdf)  
87. The Case for Physical Memory Pools: A Vision Paper \- CS@Purdue, accessed April 18, 2026, [https://www.cs.purdue.edu/homes/csjgwang/CloudNativeDB/MemoryPoolCloud19.pdf](https://www.cs.purdue.edu/homes/csjgwang/CloudNativeDB/MemoryPoolCloud19.pdf)  
88. NrOS: Effective Replication and Sharing in an Operating System | USENIX, accessed April 18, 2026, [https://www.usenix.org/system/files/osdi21-bhardwaj.pdf](https://www.usenix.org/system/files/osdi21-bhardwaj.pdf)  
89. USENIX ATC '20 Technical Sessions, accessed April 18, 2026, [https://www.usenix.org/conference/atc20/technical-sessions](https://www.usenix.org/conference/atc20/technical-sessions)  
90. Offload Annotations: Bringing Heterogeneous Computing to Existing Libraries and Workloads \- USENIX, accessed April 18, 2026, [https://www.usenix.org/system/files/atc20-paper547-slides-yuan.pdf](https://www.usenix.org/system/files/atc20-paper547-slides-yuan.pdf)  
91. Offload Annotations: Bringing Heterogeneous Computing to Existing Libraries and Workloads \- USENIX, accessed April 18, 2026, [https://www.usenix.org/system/files/atc20-yuan.pdf](https://www.usenix.org/system/files/atc20-yuan.pdf)  
92. Heterogeneous CPU-GPU Epsilon Grid Joins: Static and Dynamic Work Partitioning Strategies | NSF Public Access Repository, accessed April 18, 2026, [https://par.nsf.gov/biblio/10215704-heterogeneous-cpu-gpu-epsilon-grid-joins-static-dynamic-work-partitioning-strategies](https://par.nsf.gov/biblio/10215704-heterogeneous-cpu-gpu-epsilon-grid-joins-static-dynamic-work-partitioning-strategies)