# Download Manifest

Deduped references across all five research docs:
`GEMINI-Monolift-Research.md`, `GEMINI-Monolift-Research-II.md`,
`claude_research_notes.md`, `codex-research-report.md`, `research/RESEARCH_BRIEF.md`

Everything downloads into `inspiration/` (gitignored).

### Quick start

```bash
# requires: yq (brew install yq), curl, git
cd docs/research

./fetch.sh                   # download all 100 items (4 parallel workers)
./fetch.sh --batch 3         # only batch 3 (~10 items, great for 10-way parallelism)
./fetch.sh --type pdf        # all PDFs only
./fetch.sh --type repo       # clone all repos
./fetch.sh --id 041,042,043  # specific IDs
./fetch.sh --jobs 10 --dry-run  # preview 10-worker plan
```

Machine-readable source: [`manifest.yaml`](./manifest.yaml) — each item has `id`, `type`, `url`, `dest`, `batch`, `tags`.

---

## Batch 1 — Core + Multitier / Choreography PDFs

- [ ] `001` [Monolift: Automating Distribution (PLOS '25)](https://users.soe.ucsc.edu/~lkuper/papers/monolift-plos25.pdf) → `inspiration/papers/monolift-plos25.pdf`
- [ ] `002` [A Survey of Multitier Programming (ACM CSUR '20)](https://programming-group.com/assets/pdf/papers/2020_A-Survey-of-Multitier-Programming.pdf) → `inspiration/papers/multitier-survey-csur20.pdf`
- [ ] `003` [ScalaLoci: Distributed System Development (OOPSLA '18)](https://decomposition.al/CSE290S-2023-01/readings/scalaloci.pdf) → `inspiration/papers/scalaloci-oopsla18.pdf`
- [ ] `004` [Choreographic Programming — Montesi PhD Thesis (ITU '13)](https://pure.itu.dk/ws/files/78733848/m13_phd.pdf) → `inspiration/papers/montesi-choreo-phd13.pdf`
- [ ] `005` [Choreographic Programming — Montesi Monograph](https://www.fabriziomontesi.com/files/choreographic_programming.pdf) → `inspiration/papers/montesi-choreo-book.pdf`
- [ ] `006` [CLM '23: A Model for Choreographic Programming (Montesi et al.)](https://www.fabriziomontesi.com/files/clm23-arxiv.pdf) → `inspiration/papers/montesi-clm23.pdf`
- [ ] `007` [Choreographic Quick Changes: First-Class Location Polymorphism](https://cecchetti.sites.cs.wisc.edu/papers/lam-qc.pdf) → `inspiration/papers/choreo-quick-changes.pdf`
- [ ] `008` [Pirouette: Higher-Order Typed Functional Choreographies (MPI-SWS TR-2021-004)](https://www.mpi-sws.org/tr/2021-004.pdf) → `inspiration/papers/pirouette-mpi-sws-2021.pdf`
- [ ] `009` [The Concurrent Calculi Formalisation Benchmark](https://momigliano.di.unimi.it/papers/cbench.pdf) → `inspiration/papers/concurrent-calculi-benchmark.pdf`
- [ ] `010` [A New Architecture for Choreographic Programming Languages (UVM)](https://scholarworks.uvm.edu/bitstreams/24e2c821-9ee4-4e98-b7f2-a7edacfc394a/download) → `inspiration/papers/new-arch-choreo-uvm.pdf`

## Batch 2 — Pony + Actor Systems PDFs

- [ ] `011` [A String of Ponies (Pony language)](https://www.ponylang.io/media/papers/a_string_of_ponies.pdf) → `inspiration/papers/pony-string-of-ponies.pdf`
- [ ] `012` [Deny Capabilities for Safe, Fast Actors (Pony co-design)](https://www.ponylang.io/media/papers/codesigning.pdf) → `inspiration/papers/pony-deny-capabilities.pdf`
- [ ] `013` [A Principled Design of Capabilities in Pony (Steed, Imperial)](https://www.imperial.ac.uk/media/imperial-college/faculty-of-engineering/computing/public/GeorgeSteed.pdf) → `inspiration/papers/pony-capabilities-steed.pdf`
- [ ] `014` [A Comparison of Capability Systems: Encore, Pony, Rust](https://www.diva-portal.org/smash/get/diva2:1363822/FULLTEXT01.pdf) → `inspiration/papers/capability-systems-encore-pony-rust.pdf`
- [ ] `015` [Diva Portal Paper (diva2:2013047)](https://www.diva-portal.org/smash/get/diva2:2013047/FULLTEXT01.pdf) → `inspiration/papers/diva2-2013047.pdf`
- [ ] `016` [Ownership and Reference Counting GC in the Actor World (ICOOOLPS '15)](https://www.doc.ic.ac.uk/~scd/icooolps15_GC.pdf) → `inspiration/papers/actor-gc-icooolps15.pdf`
- [ ] `017` [Fast and Cheap Message Passing in ML (AGERE)](https://www.doc.ic.ac.uk/~scd/fast-cheap-AGERE.pdf) → `inspiration/papers/fast-cheap-msg-agere.pdf`
- [ ] `018` [Functional Choreographic Programming (escholarship)](https://escholarship.org/content/qt2620x3h4/qt2620x3h4.pdf) → `inspiration/papers/functional-choreo-programming.pdf`
- [ ] `019` [Service Weaver: A Promising Direction for Cloud-Native Systems? (SCITEPRESS '24)](https://www.scitepress.org/Papers/2024/126245/126245.pdf) → `inspiration/papers/service-weaver-analysis-scitepress24.pdf`
- [ ] `020` [Distributed Systems Lecture Notes (Cambridge ConcDisSys)](https://www.cl.cam.ac.uk/teaching/2122/ConcDisSys/dist-sys-notes.pdf) → `inspiration/papers/cambridge-dist-sys-notes.pdf`

## Batch 3 — Partitioning + Offload PDFs

- [ ] `021` [The Coign Automatic Distributed Partitioning System (OSDI '99)](https://www.microsoft.com/en-us/research/wp-content/uploads/2016/02/huntosdi99.pdf) → `inspiration/papers/coign-osdi99.pdf`
- [ ] `022` [MAUI: Making Smartphones Last Longer with Code Offload (MobiSys '10)](http://web.cs.ucla.edu/~ravi/CS219_F19/papers/maui.pdf) → `inspiration/papers/maui-mobisys10.pdf`
- [ ] `023` [Elicit: Efficiently Identify Computation-Intensive Tasks for Offloading](https://mason.gmu.edu/~mhassanb/Elicit.pdf) → `inspiration/papers/elicit-offload.pdf`
- [ ] `024` [An Efficient Code Partition Algorithm for Mobile Cloud Computing (CloudNet '12)](https://ljiao.github.io/papers/cloudnet12.pdf) → `inspiration/papers/code-partition-mobile-cloud-cloudnet12.pdf`
- [ ] `025` [Pyxis: Dependency-Guided Application Partitioning (VLDB '12)](http://vldb.org/pvldb/vol5/p1471_alvincheung_vldb2012.pdf) → `inspiration/papers/pyxis-vldb12.pdf`
- [ ] `026` [Alvin Cheung PhD Dissertation (UW — database application partitioning)](https://homes.cs.washington.edu/~akcheung/papers/dissertation.pdf) → `inspiration/papers/akcheung-dissertation.pdf`
- [ ] `027` [Enhancing Mobile Devices through Code Offload (MSR Dissertation)](https://www.microsoft.com/en-us/research/wp-content/uploads/2016/02/dissertation.pdf) → `inspiration/papers/msr-mobile-offload-dissertation.pdf`
- [ ] `028` [Toward Generating Microservice Architectures from Requirements with LLMs (SBCARS)](https://sol.sbc.org.br/index.php/sbcars/article/download/36976/36761/) → `inspiration/papers/llm-microservice-generation-sbcars.pdf`
- [ ] `029` [Verified Lifting of Stencil Computations — PLDI '16](https://csaws.cs.technion.ac.il/~shachari/dl/pldi2016.pdf) → `inspiration/papers/verified-lifting-stencil-pldi16.pdf`
- [ ] `030` [Automatic Partitioning of Database Applications (CIDR '13)](https://www.cidrdb.org/cidr2013/Papers/CIDR13_Paper117.pdf) → `inspiration/papers/auto-partition-db-apps-cidr13.pdf`

## Batch 4 — Systems / Memory / Observability PDFs

- [ ] `031` [TELEPORT: Optimizing Data-Intensive Systems in Disaggregated Data Centers (SIGMOD '22)](https://www.cis.upenn.edu/~sga001/papers/teleport-sigmod22.pdf) → `inspiration/papers/teleport-sigmod22.pdf`
- [ ] `032` [Disaggregated Database Systems — SIGMOD '23 Tutorial](https://www.cs.purdue.edu/homes/csjgwang/pubs/SIGMOD23_Tutorial_DisaggregatedDB.pdf) → `inspiration/papers/disaggregated-db-sigmod23-tutorial.pdf`
- [ ] `033` [Memory-Disaggregated DBMSs — SIGMOD '23 Slides (Far Data Lab)](https://fardatalab.org/sigmod23-tutorial-slides.pdf) → `inspiration/papers/memory-disagg-dbms-sigmod23-slides.pdf`
- [ ] `034` [The Case for Physical Memory Pools: A Vision Paper](https://www.cs.purdue.edu/homes/csjgwang/CloudNativeDB/MemoryPoolCloud19.pdf) → `inspiration/papers/physical-memory-pools-vision.pdf`
- [ ] `035` [NrOS: Effective Replication and Sharing in an OS (OSDI '21)](https://www.usenix.org/system/files/osdi21-bhardwaj.pdf) → `inspiration/papers/nros-osdi21.pdf`
- [ ] `036` [LegoOS: Disseminated Distributed OS for Hardware Disaggregation (OSDI '18)](https://www.usenix.org/system/files/osdi18-shan.pdf) → `inspiration/papers/legoos-osdi18.pdf`
- [ ] `037` [Offload Annotations: Heterogeneous Computing to Existing Libraries (ATC '20)](https://www.usenix.org/system/files/atc20-yuan.pdf) → `inspiration/papers/offload-annotations-atc20.pdf`
- [ ] `038` [Offload Annotations — ATC '20 Slides](https://www.usenix.org/system/files/atc20-paper547-slides-yuan.pdf) → `inspiration/papers/offload-annotations-atc20-slides.pdf`
- [ ] `039` [Ray: A Distributed Framework for Emerging AI Applications (OSDI '18)](https://www.usenix.org/system/files/osdi18-moritz.pdf) → `inspiration/papers/ray-osdi18.pdf`
- [ ] `040` [Hyperscale Data Processing with Network-Centric Designs — Zhang Thesis](https://netdb.cis.upenn.edu/wp-content/uploads/2022/08/Qizhen_Zhang_Thesis.pdf) → `inspiration/papers/qizhen-zhang-thesis.pdf`

## Batch 5 — arXiv Papers

- [ ] `041` arXiv:[2002.06184](https://arxiv.org/pdf/2002.06184) — ScalaLoci: Implementing a Language for Distributed Systems → `inspiration/papers/arxiv-2002.06184-scalaloci-impl.pdf`
- [ ] `042` arXiv:[2111.03701](https://arxiv.org/pdf/2111.03701) — Functional Choreographic Programming → `inspiration/papers/arxiv-2111.03701-functional-choreo.pdf`
- [ ] `043` arXiv:[2303.03983](https://arxiv.org/pdf/2303.03983) — Real-World Choreographic Programming: An Experience Report → `inspiration/papers/arxiv-2303.03983-real-world-choreo.pdf`
- [ ] `044` arXiv:[2303.00924](https://arxiv.org/pdf/2303.00924) — Choral: Object-Oriented Choreographic Programming → `inspiration/papers/arxiv-2303.00924-choral.pdf`
- [ ] `045` arXiv:[2311.11472](https://arxiv.org/pdf/2311.11472) — Portable, Efficient, and Practical Library-Level Choreographic Programming → `inspiration/papers/arxiv-2311.11472-library-choreo.pdf`
- [ ] `046` arXiv:[2107.09698](https://arxiv.org/pdf/2107.09698) — Mono2Micro: Decomposing Monolithic Java Applications → `inspiration/papers/arxiv-2107.09698-mono2micro.pdf`
- [ ] `047` arXiv:[2412.20992](https://arxiv.org/pdf/2412.20992) — *(verify title on landing)* → `inspiration/papers/arxiv-2412.20992.pdf`
- [ ] `048` arXiv:[2407.10740](https://arxiv.org/pdf/2407.10740) — *(verify title on landing)* → `inspiration/papers/arxiv-2407.10740.pdf`
- [ ] `049` arXiv:[2503.20275](https://arxiv.org/pdf/2503.20275) — *(verify title on landing)* → `inspiration/papers/arxiv-2503.20275.pdf`
- [ ] `050` [Berkeley EECS Tech Report EECS-2018-95](https://www2.eecs.berkeley.edu/Pubs/TechRpts/2018/EECS-2018-95.pdf) → `inspiration/papers/berkeley-eecs-2018-95.pdf`

## Batch 6 — GitHub Repositories

```bash
# One-liner for this whole batch:
./fetch.sh --batch 6 --jobs 8
```

- [ ] `051` `git clone --depth=1` [ServiceWeaver/weaver](https://github.com/ServiceWeaver/weaver) → `inspiration/repos/weaver`
- [ ] `052` `git clone --depth=1` [scala-loci/scala-loci](https://github.com/scala-loci/scala-loci) → `inspiration/repos/scala-loci`
- [ ] `053` `git clone --depth=1` [links-lang/links](https://github.com/links-lang/links) → `inspiration/repos/links`
- [ ] `054` `git clone --depth=1` [urweb/urweb](https://github.com/urweb/urweb) → `inspiration/repos/urweb`
- [ ] `055` `git clone --depth=1` [gshen42/HasChor](https://github.com/gshen42/HasChor) → `inspiration/repos/HasChor`
- [ ] `056` `git clone --depth=1` [WukLab/LegoOS](https://github.com/Wuklab/LegoOS) → `inspiration/repos/LegoOS`
- [ ] `057` `git clone --depth=1` [dmemsys/awesome-disaggregated-memory](https://github.com/dmemsys/awesome-disaggregated-memory) → `inspiration/repos/awesome-disaggregated-memory`
- [ ] `058` `git clone --depth=1` [docelic/awesome-urweb](https://github.com/docelic/awesome-urweb) → `inspiration/repos/awesome-urweb`

## Batch 7 — Official Docs + Project Pages (HTML)

- [ ] `059` [Service Weaver Documentation](https://serviceweaver.dev/docs.html) → `inspiration/html/serviceweaver-docs.html`
- [ ] `060` [A History of Service Weaver's Core API](https://serviceweaver.dev/blog/history.html) → `inspiration/html/serviceweaver-history.html`
- [ ] `061` [Introducing Service Weaver (Google Open Source Blog)](https://opensource.googleblog.com/2023/03/introducing-service-weaver-framework-for-writing-distributed-applications.html) → `inspiration/html/serviceweaver-announcement.html`
- [ ] `062` [Temporal Platform Documentation](https://docs.temporal.io/) → `inspiration/html/temporal-docs.html`
- [ ] `063` [Temporal: What is Durable Execution?](https://temporal.io/blog/what-is-durable-execution) → `inspiration/html/temporal-durable-execution.html`
- [ ] `064` [Temporal: Reliable Data Processing — Queues and Workflows](https://temporal.io/blog/reliable-data-processing-queues-workflows) → `inspiration/html/temporal-queues-workflows.html`
- [ ] `065` [Temporal: Durable Execution + Observability](https://temporal.io/blog/durable-execution-in-distributed-systems-increasing-observability) → `inspiration/html/temporal-observability.html`
- [ ] `066` [Ray: Serve Architecture](https://docs.ray.io/en/latest/serve/architecture.html) → `inspiration/html/ray-serve-architecture.html`
- [ ] `067` [Ray: Getting Started](https://docs.ray.io/en/latest/ray-overview/getting-started.html) → `inspiration/html/ray-getting-started.html`
- [ ] `068` [HasChor at POPL '23 SRC](https://popl23.sigplan.org/details/POPL-2023-student-research-competition/3/HasChor-Choreographic-Programming-in-Haskell) → `inspiration/html/haschor-popl23.html`

## Batch 8 — Blog Posts + Community Discussion (HTML)

- [ ] `069` [Remote Procedure Call — Meiklejohn (2016)](https://christophermeiklejohn.com/pl/2016/04/12/rpc.html) → `inspiration/html/meiklejohn-rpc.html`
- [ ] `070` [The Road We Didn't Go Down — Armstrong on Software](http://armstrongonsoftware.blogspot.com/2008/05/road-we-didnt-go-down.html) → `inspiration/html/armstrong-road-not-taken.html`
- [ ] `071` [What Did Service Weaver Miss? (Medium)](https://medium.com/@xiafan/what-did-service-weaver-miss-f1e58decfca7) → `inspiration/html/service-weaver-miss.html`
- [ ] `072` [Temporal's Internal Architecture (Medium)](https://medium.com/data-science-collective/system-design-series-a-step-by-step-breakdown-of-temporals-internal-architecture-52340cc36f30) → `inspiration/html/temporal-architecture-medium.html`
- [ ] `073` [A Note on Distributed Computing — HN Discussion](https://news.ycombinator.com/item?id=34245875) → `inspiration/html/hn-note-on-distributed-computing.html`
- [ ] `074` [HN Discussion #24396142](https://news.ycombinator.com/item?id=24396142) → `inspiration/html/hn-24396142.html`
- [ ] `075` [HN Discussion #31085216](https://news.ycombinator.com/item?id=31085216) → `inspiration/html/hn-31085216.html`
- [ ] `076` [Alice or Bob?: Process Polymorphism in Choreographies (JFP)](https://www.cambridge.org/core/journals/journal-of-functional-programming/article/alice-or-bob-process-polymorphism-in-choreographies/382AD3B58F86FF95AB59DDF0EDE96F65) → `inspiration/html/alice-or-bob-choreographies.html`
- [ ] `077` [A Model for Correlation-Based Choreographic Programming (PMC)](https://pmc.ncbi.nlm.nih.gov/articles/PMC11784539/) → `inspiration/html/correlation-choreo-pmc.html`
- [ ] `078` [Deny Capabilities for Safe Fast Actors (morning paper)](https://blog.acolyer.org/2016/02/17/deny-capabilities/) → `inspiration/html/acolyer-deny-capabilities.html`

## Batch 9 — Additional Research PDFs

- [ ] `079` [Shadaj Laddad Dissertation (Hydro / Hydroflow)](https://www.shadaj.me/papers/dissertation.pdf) → `inspiration/papers/shadaj-dissertation-hydro.pdf`
- [ ] `080` [VLDB 2020 — Chen, Ang (network-aware data processing)](https://web.eecs.umich.edu/~chenang/papers/vldb-2020.pdf) → `inspiration/papers/chenang-vldb2020.pdf`
- [ ] `081` [COORDINATION 2015 — INRIA SPADES](https://team.inria.fr/spades/files/2015/06/COORDINATION_2015.pdf) → `inspiration/papers/coordination2015-inria.pdf`
- [ ] `082` [BOC: Break-Out Containers (Kogias)](https://marioskogias.github.io/docs/boc.pdf) → `inspiration/papers/boc-kogias.pdf`
- [ ] `083` [AI-Driven Refactoring (THESAI)](https://thesai.org/Downloads/Volume17No2/Paper_83-AI_Driven_Refactoring.pdf) → `inspiration/papers/ai-driven-refactoring.pdf`
- [ ] `084` [WJARR 2025-1621](https://wjarr.com/sites/default/files/fulltext_pdf/WJARR-2025-1621.pdf) → `inspiration/papers/wjarr-2025-1621.pdf`
- [ ] `085` [WJARR 2025-1832](https://wjarr.com/sites/default/files/fulltext_pdf/WJARR-2025-1832.pdf) → `inspiration/papers/wjarr-2025-1832.pdf`
- [ ] `086` [IEEE 08318383 (IEEEXplore open access)](https://ieeexplore.ieee.org/iel7/6287639/8274985/08318383.pdf) → `inspiration/papers/ieee-08318383.pdf`
- [ ] `087` [VenkateshEmani MSR Thesis](https://www.microsoft.com/en-us/research/wp-content/uploads/2021/02/Unsigned_Long_Thesis_VenkateshEmani.pdf) → `inspiration/papers/msr-venkatesh-emani-thesis.pdf`
- [ ] `088` [MPG Pure — item 3359945](https://pure.mpg.de/rest/items/item_3359945/component/file_3362431/content) → `inspiration/papers/mpg-pure-3359945.pdf`

## Batch 10 — Reference / Index Pages (HTML)

- [ ] `089` [A Note on Distributed Computing — Semantic Scholar](https://www.semanticscholar.org/paper/A-Note-on-Distributed-Computing-Waldo-Wyant/778e4bbdea4e35f3889e48bfba7c951ed3b43b54) → `inspiration/html/note-on-distributed-computing-s2.html`
- [ ] `090` [Distributed object — Wikipedia](https://en.wikipedia.org/wiki/Distributed_object) → `inspiration/html/wikipedia-distributed-object.html`
- [ ] `091` [Mono2Micro IBM Research (toolchain)](https://research.ibm.com/publications/mono2micro-an-ai-based-toolchain-for-evolving-monolithic-enterprise-applications-to-a-microservice-architecture) → `inspiration/html/mono2micro-ibm-toolchain.html`
- [ ] `092` [Mono2Micro IBM Research (practical tool)](https://research.ibm.com/publications/mono2micro-a-practical-and-effective-tool-for-decomposing-monolithic-java-applications-to-microservices) → `inspiration/html/mono2micro-ibm-practical.html`
- [ ] `093` [PLDI '16: Verified Lifting of Stencil Computations](https://pldi16.sigplan.org/details/pldi-2016-papers/40/Verified-Lifting-of-Stencil-Computations) → `inspiration/html/pldi16-verified-lifting.html`
- [ ] `094` [Pony Language Papers Index](https://www.ponylang.io/learn/papers/) → `inspiration/html/ponylang-papers.html`
- [ ] `095` [AWS: Building Resilient Distributed Systems with Temporal](https://aws.amazon.com/blogs/apn/building-resilient-distributed-systems-with-temporal-and-aws/) → `inspiration/html/aws-temporal-resilient.html`
- [ ] `096` [LeifAndersen GitHub Gist](https://gist.github.com/LeifAndersen/12edea0b8bf62a34aebc8984eb5f3a48) → `inspiration/html/leif-andersen-gist.html`
- [ ] `097` [Martin Fowler: Research Review Rebuild](https://martinfowler.com/articles/research-review-rebuild.html) → `inspiration/html/martinfowler-research-review.html`
- [ ] `098` [Service Weaver — pkg.go.dev](https://pkg.go.dev/github.com/ServiceWeaver/weaver) → `inspiration/html/serviceweaver-pkg-go-dev.html`
- [ ] `099` [Kotlin Language Specification](https://kotlinlang.org/spec/kotlin-spec.html) → `inspiration/html/kotlin-spec.html`
- [ ] `100` [CNCF Wasm Landscape (2023)](https://www.cncf.io/blog/2023/09/06/introducing-the-wasm-landscape/) → `inspiration/html/cncf-wasm-landscape.html`

## Batch 11 — Core Prior Art: Comparison + Far Memory

- [ ] `101` [Service Weaver: Towards Modern Development of Cloud Applications (HotOS '23)](https://sigops.org/s/conferences/hotos/2023/papers/ghemawat.pdf) → `inspiration/papers/service-weaver-hotos23.pdf`
- [ ] `102` [Ignis: Scaling Distribution-Oblivious Systems with Light-Touch Distribution (PLDI '19)](https://dl.acm.org/doi/pdf/10.1145/3314221.3314586) → `inspiration/papers/ignis-pldi19.pdf`
- [ ] `103` [AIFM: High-Performance, Application-Integrated Far Memory (OSDI '20)](https://www.usenix.org/system/files/osdi20-ruan.pdf) → `inspiration/papers/aifm-osdi20.pdf`
- [ ] `104` [Infiniswap: Efficient Memory Disaggregation with RDMA (NSDI '17)](https://www.usenix.org/system/files/conference/nsdi17/nsdi17-gu.pdf) → `inspiration/papers/infiniswap-nsdi17.pdf`
- [ ] `105` [Can Far Memory Improve Job Throughput? (EuroSys '20)](https://dl.acm.org/doi/pdf/10.1145/3342195.3387522) → `inspiration/papers/far-memory-eurosys20.pdf`
- [ ] `106` [Offload Annotations: Heterogeneous Computing for Existing Libraries (ATC '20)](https://www.usenix.org/system/files/atc20-yuan.pdf) → `inspiration/papers/offload-annotations-atc20.pdf`
- [ ] `107` [TELEPORT: Pushdown to Disaggregated Data Centers (SIGMOD '22)](https://dl.acm.org/doi/pdf/10.1145/3514221.3517856) → `inspiration/papers/teleport-sigmod22.pdf`
- [ ] `108` [Coign: Automatic Distributed Partitioning of COM Applications (OSDI '99)](https://www.usenix.org/legacy/events/osdi99/full_papers/hunt/hunt.pdf) → `inspiration/papers/coign-osdi99.pdf`
- [ ] `109` [CloneCloud: Elastic Execution between Mobile Device and Cloud (EuroSys '11)](https://dl.acm.org/doi/pdf/10.1145/1966445.1966473) → `inspiration/papers/clonecloud-eurosys11.pdf`
- [ ] `110` [MAUI: Making Smartphones Last Longer with Code Offload (MobiSys '10)](https://dl.acm.org/doi/pdf/10.1145/1814433.1814441) → `inspiration/papers/maui-mobisys10.pdf`

## Batch 12 — Serverless, FaaS, and Microservice Resource Management

- [ ] `111` [SAND: Towards High-Performance Serverless Computing (ATC '18)](https://www.usenix.org/system/files/conference/atc18/atc18-akkus.pdf) → `inspiration/papers/sand-atc18.pdf`
- [ ] `112` [Nightcore: Efficient Serverless for Latency-Sensitive Microservices (ASPLOS '21)](https://www.cs.utexas.edu/~witchel/pubs/nightcore-asplos21.pdf) → `inspiration/papers/nightcore-asplos21.pdf`
- [ ] `113` [Faasm: Lightweight Isolation for Stateful Serverless Computing (ATC '20)](https://www.usenix.org/system/files/atc20-shillaker.pdf) → `inspiration/papers/faasm-atc20.pdf`
- [ ] `114` [Cilantro: Performance-Aware Resource Allocation via Online Feedback (OSDI '23)](https://www.usenix.org/system/files/osdi23-bhardwaj.pdf) → `inspiration/papers/cilantro-osdi23.pdf`
- [ ] `115` [FIRM: Fine-Grained Resource Management for SLO-Oriented Microservices (OSDI '20)](https://www.usenix.org/system/files/osdi20-qiu.pdf) → `inspiration/papers/firm-osdi20.pdf`
- [ ] `116` [Sage: ML-Based QoS Diagnosis in Cloud Microservices (ASPLOS '21)](https://www.csl.cornell.edu/~delimitrou/papers/2021.asplos.sage.pdf) → `inspiration/papers/sage-asplos21.pdf`
- [ ] `117` [Sinan: ML-Based QoS-Aware Resource Management for Cloud Microservices (ASPLOS '21)](https://www.csl.cornell.edu/~delimitrou/papers/2021.asplos.sinan.pdf) → `inspiration/papers/sinan-asplos21.pdf`
- [ ] `118` [Autopilot: Workload Autoscaling at Google (EuroSys '20)](https://dl.acm.org/doi/pdf/10.1145/3342195.3387168) → `inspiration/papers/autopilot-eurosys20.pdf`
- [ ] `119` [DeathStarBench: Open-Source Benchmark Suite for Microservices (ASPLOS '19)](https://www.csl.cornell.edu/~delimitrou/papers/2019.asplos.microservices.pdf) → `inspiration/papers/deathstarbench-asplos19.pdf`
- [ ] `120` [CherryPick: Adaptively Finding Best Cloud Configurations for Big Data (NSDI '17)](https://www.usenix.org/system/files/conference/nsdi17/nsdi17-alipourfard.pdf) → `inspiration/papers/cherrypick-nsdi17.pdf`

## Batch 13 — RL for Systems + Distributed Verification

- [ ] `121` [Pensieve: Neural Adaptive Video Streaming with RL (SIGCOMM '17)](https://dl.acm.org/doi/pdf/10.1145/3098822.3098843) → `inspiration/papers/pensieve-sigcomm17.pdf`
- [ ] `122` [Decima: Learning Scheduling Algorithms for Data Processing Clusters (SIGCOMM '19)](https://dl.acm.org/doi/pdf/10.1145/3341302.3342080) → `inspiration/papers/decima-sigcomm19.pdf`
- [ ] `123` [AuTO: Learning-Based Traffic Optimization in Data Centers (SIGCOMM '18)](https://dl.acm.org/doi/pdf/10.1145/3230543.3230551) → `inspiration/papers/auto-sigcomm18.pdf`
- [ ] `124` [Park: Open Platform for Learning-Augmented Computer Systems (NeurIPS '19)](https://proceedings.neurips.cc/paper_files/paper/2019/file/1d0832c4969f6a4cc7f1d5dc5a3bdc09-Paper.pdf) → `inspiration/papers/park-neurips19.pdf`
- [ ] `125` [SEDA: Architecture for Well-Conditioned, Scalable Internet Services (SOSP '01)](https://dl.acm.org/doi/pdf/10.1145/502034.502057) → `inspiration/papers/seda-sosp01.pdf`
- [ ] `126` [Verdi: A Framework for Implementing and Formally Verifying Distributed Systems (PLDI '15)](https://dl.acm.org/doi/pdf/10.1145/2737924.2737958) → `inspiration/papers/verdi-pldi15.pdf`
- [ ] `127` [IronFleet: Proving Practical Distributed Systems Correct (SOSP '15)](https://dl.acm.org/doi/pdf/10.1145/2815400.2815428) → `inspiration/papers/ironfleet-sosp15.pdf`
- [ ] `128` [Ivy: Safety Verification by Interactive Generalization (PLDI '16)](https://dl.acm.org/doi/pdf/10.1145/2908080.2908118) → `inspiration/papers/ivy-pldi16.pdf`
- [ ] `129` [Pirouette: Higher-Order Typed Functional Choreographies (POPL '23)](https://arxiv.org/pdf/2111.03701) → `inspiration/papers/pirouette-popl23.pdf`
- [ ] `130` [Orleans: Cloud Computing for Everyone — Virtual Actors (SoCC '11)](https://dl.acm.org/doi/pdf/10.1145/2038916.2038932) → `inspiration/papers/orleans-socc11.pdf`

## Batch 14 — Actor Systems + Language-Level Distribution

- [ ] `131` [Towards Haskell in the Cloud — Cloud Haskell (Haskell Symp. '11)](https://dl.acm.org/doi/pdf/10.1145/2096148.2034701) → `inspiration/papers/cloud-haskell-haskell11.pdf`
- [ ] `133` [Ray: A Distributed Framework for Emerging AI Applications (OSDI '18)](https://www.usenix.org/system/files/osdi18-moritz.pdf) → `inspiration/papers/ray-osdi18.pdf`
- [ ] `134` [Swift: Secure Web Applications via Automatic Partitioning (SOSP '07)](https://dl.acm.org/doi/pdf/10.1145/1294261.1294275) → `inspiration/papers/swift-sosp07.pdf`
- [ ] `135` [Fabric: Platform for Secure Distributed Computation and Storage (SOSP '09)](https://dl.acm.org/doi/pdf/10.1145/1629575.1629604) → `inspiration/papers/fabric-sosp09.pdf`
- [ ] `137` [Links: Web Programming Without Tiers (FMCO '06)](https://homepages.inf.ed.ac.uk/wadler/papers/links/links.pdf) → `inspiration/papers/links-fmco06.pdf`
- [ ] `138` [Choral: Object-Oriented Choreographic Programming (TOPLAS '24)](https://dl.acm.org/doi/pdf/10.1145/3632398) → `inspiration/papers/choral-toplas24.pdf`
- [ ] `139` [Ur/Web: A Simple Model for Programming the Web (ICFP '15)](https://dl.acm.org/doi/pdf/10.1145/2784731.2784741) → `inspiration/papers/urweb-icfp15.pdf`
- [ ] `140` [Sagas (Garcia-Molina & Salem, SIGMOD '87)](https://dl.acm.org/doi/pdf/10.1145/38713.38742) → `inspiration/papers/sagas-sigmod87.pdf`

## Batch 15 — Type Theory, Correctness, and Formal Models

- [ ] `141` [LVars: Lattice-Based Data Structures for Deterministic Parallelism (FHPC '13)](https://dl.acm.org/doi/pdf/10.1145/2502323.2502326) → `inspiration/papers/lvars-fhpc13.pdf`
- [ ] `142` [Freeze After Writing: Quasi-Deterministic Parallel Programming with LVars (POPL '14)](https://dl.acm.org/doi/pdf/10.1145/2535838.2535842) → `inspiration/papers/lvars-freeze-popl14.pdf`
- [ ] `143` [Conflict-Free Replicated Data Types — CRDTs (SSS '11)](https://inria.hal.science/inria-00555588/document) → `inspiration/papers/crdts-sss11.pdf`
- [ ] `144` [Language Primitives for Structured Communication — Session Types (ESOP '98)](https://www.di.fc.ul.pt/~vv/papers/honda.vasconcelos.kubo_language-primitives.pdf) → `inspiration/papers/session-types-esop98.pdf`
- [ ] `145` [Multiparty Asynchronous Session Types (POPL '08)](https://dl.acm.org/doi/pdf/10.1145/1328897.1328472) → `inspiration/papers/mpst-popl08.pdf`
- [ ] `146` [Static Race Detection and Mutex Safety for Go Programs (ECOOP '20)](https://drops.dagstuhl.de/opus/volltexte/2020/13186/pdf/LIPIcs-ECOOP-2020-28.pdf) → `inspiration/papers/gabet-yoshida-ecoop20.pdf`
- [ ] `147` [Keeping CALM: When Distributed Consistency Is Easy (CACM '20)](https://dl.acm.org/doi/pdf/10.1145/3369736) → `inspiration/papers/calm-cacm20.pdf`
- [ ] `148` [RIFL: Implementing Linearizability at Large Scale and Low Latency (SOSP '15)](https://dl.acm.org/doi/pdf/10.1145/2815400.2815416) → `inspiration/papers/rifl-sosp15.pdf`
- [ ] `149` [SEEC: A Framework for Self-Aware Computing (MIT-CSAIL-TR-2011-046)](https://dspace.mit.edu/bitstream/handle/1721.1/62759/MIT-CSAIL-TR-2011-046.pdf) → `inspiration/papers/seec-tr2011.pdf`
- [ ] `150` [The Tail at Scale (Dean & Barroso, CACM '13)](https://dl.acm.org/doi/pdf/10.1145/2408776.2408794) → `inspiration/papers/tail-at-scale-cacm13.pdf`

## Batch 16 — Infrastructure, Platforms, and Decomposition Tools

- [ ] `151` [Unikraft: Fast, Specialized Unikernels the Easy Way (EuroSys '21)](https://dl.acm.org/doi/pdf/10.1145/3447786.3456248) → `inspiration/papers/unikraft-eurosys21.pdf`
- [ ] `152` [Unikernels: Library Operating Systems for the Cloud — MirageOS (ASPLOS '13)](https://dl.acm.org/doi/pdf/10.1145/2451116.2451167) → `inspiration/papers/mirageos-asplos13.pdf`
- [ ] `153` [MLIR: Scaling Compiler Infrastructure for Domain-Specific Computation (CGO '21)](https://arxiv.org/pdf/2002.11054) → `inspiration/papers/mlir-cgo21.pdf`
- [ ] `154` [Hermit: Low-Latency Remote Memory via Feedback-Directed Asynchrony (NSDI '23)](https://www.usenix.org/system/files/nsdi23-qiao.pdf) → `inspiration/papers/hermit-nsdi23.pdf`
- [ ] `156` [MonoEmbed: Contrastive LLM-Based Monolith-to-Microservice Decomposition (arXiv:2502.04604)](https://arxiv.org/pdf/2502.04604) → `inspiration/papers/monoembed-arxiv2502.pdf`
- [ ] `157` [CARGO: AI-Guided Dependency Analysis for Microservice Decomposition (ASE '22)](https://dl.acm.org/doi/pdf/10.1145/3551349.3556960) → `inspiration/papers/cargo-ase22.pdf`
- [ ] `158` [Decomposition of Monolith Applications into Microservices: A Systematic Review (IEEE TSE '23)](https://arxiv.org/pdf/2305.06996) → `inspiration/papers/abgaz-tse23-decomp.pdf`
- [ ] `159` [The Rise and Fall of CORBA (Henning, ACM Queue '06)](https://dl.acm.org/doi/pdf/10.1145/1142055.1142083) → `inspiration/papers/corba-rise-fall-queue06.pdf`
- [ ] `160` [Automatic Partitioning of Database Applications — Pyxis (VLDB '12)](https://dl.acm.org/doi/pdf/10.14778/2310570.2310580) → `inspiration/papers/pyxis-vldb12.pdf`

---

## Summary

| Batch | Type | Count | Worker command |
|-------|------|-------|----------------|
| 1 | PDF — core + choreography | 10 | `./fetch.sh --batch 1` |
| 2 | PDF — Pony + actors | 10 | `./fetch.sh --batch 2` |
| 3 | PDF — partitioning + offload | 10 | `./fetch.sh --batch 3` |
| 4 | PDF — systems / memory | 10 | `./fetch.sh --batch 4` |
| 5 | arXiv + misc PDF | 10 | `./fetch.sh --batch 5` |
| 6 | GitHub repos | 8 | `./fetch.sh --batch 6` |
| 7 | HTML — official docs | 10 | `./fetch.sh --batch 7` |
| 8 | HTML — blogs + discussions | 10 | `./fetch.sh --batch 8` |
| 9 | PDF — additional research | 10 | `./fetch.sh --batch 9` |
| 10 | HTML — reference / index pages | 10 | `./fetch.sh --batch 10` |
| 11 | PDF — core prior art: far memory + partitioning | 10 | `./fetch.sh --batch 11` |
| 12 | PDF — serverless + microservice scheduling | 10 | `./fetch.sh --batch 12` |
| 13 | PDF — RL for systems + verification | 10 | `./fetch.sh --batch 13` |
| 14 | PDF — actor systems + language-level distribution | 10 | `./fetch.sh --batch 14` |
| 15 | PDF — type theory + formal correctness | 10 | `./fetch.sh --batch 15` |
| 16 | PDF — infrastructure, platforms, decomposition | 10 | `./fetch.sh --batch 16` |
| **Total** | | **160** | `./fetch.sh --jobs 16` |
