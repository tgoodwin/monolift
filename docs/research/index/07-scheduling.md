# 07 — Microservice Scheduling and Resource Management

## Benchmark baseline

### DeathStarBench (Gan, Zhang, Delimitrou et al. — ASPLOS '19)
**Source:** `inspiration/papers/deathstarbench-asplos19.pdf`

De facto standard microservice benchmark. Six end-to-end applications (social network, media
service, hotel reservation, e-commerce, banking, "book info") implemented as 20–40 microservices
in multiple languages. Used directly in Monolift's evaluation (monolift-plos25.pdf §4, p.4–5).

**Monolift's evaluation** used the Social Network application (Go port), 4 service packages:
`user`, `post`, `timeline`, `socialgraph`. 8-node, 40-core Kubernetes cluster.

---

## Tail latency fundamentals

### The Tail at Scale (Dean & Barroso — CACM '13)
**Source:** `inspiration/papers/tail-at-scale-cacm13.pdf`

Foundational article. When a user request fans out to N services, user-observed latency =
tail of the slowest service. At fan-out of 100, even 1-in-1000 slow responses dominate.

**Implication for Monolift** (`docs/research/claude_comprehensive_research.md` line 93):
A delegate expression optimizing mean latency is **wrong**. It must target a percentile
(p99 or p99.9) and account for fan-out amplification when a single request triggers multiple
lift dispatches.

---

## ML-based microservice management

### Sage (Gan, Liang, Dev, Lo, Delimitrou — ASPLOS '21)
**Source:** `inspiration/papers/sage-asplos21.pdf`

Causal Bayesian network + GNN for root-cause analysis of QoS violations in microservice graphs.
88% root-cause accuracy on DeathStarBench. Directly applicable as an offline analysis layer
for Monolift: Sage can identify which lifts are causing SLO violations.

### Sinan (Zhang, Cheng, Delimitrou — ASPLOS '21)
**Source:** `inspiration/papers/sinan-asplos21.pdf`

CNN predicts latency from resource allocations; selects allocations minimizing cost subject to
SLO. Online learning handles workload drift. These prediction methods are templates for the
cost model inside Monolift delegate expressions.

### FIRM (Qiu, Banerjee, Jha, Kalbarczyk, Iyer — OSDI '20)
**Source:** `inspiration/papers/firm-osdi20.pdf`

Two-level RL for microservice resource management: SVM identifies the critical path, DDPG
re-allocates resources to SLO-violating services. See [11-rl-ml-systems](11-rl-ml-systems.md).

---

## Autoscaling

### Autopilot (Rzadca, Findeisen et al. — EuroSys '20)
**Source:** `inspiration/papers/autopilot-eurosys20.pdf`

Google's production autoscaler. Moving-window histogram of resource usage + recommender rules +
ML model. ML recommendations reduce slack by 23% vs. human baseline; halve OOM kill rate.
Key lesson: production autoscaling is a **hybrid of reactive control and predictive ML**.
Operator trust (histogram baseline) keeps the ML recommender honest.
`docs/research/claude_comprehensive_research.md` lines 77–78.

### Cilantro (Bhardwaj, Kim, Pavuluri, Tumanov — OSDI '23)
**Source:** `inspiration/papers/cilantro-osdi23.pdf`

Thompson-sampling bandit for resource allocation with performance feedback. Per-tenant feedback
loop with composition across tenants proven to converge under mild assumptions.
Directly instructive for Monolift: Cilantro's convergence proof is the kind of stability
argument that delegate-expression composition currently lacks.
`docs/research/claude_comprehensive_research.md` line 79.

### CherryPick (Alipourfard, Liu et al. — NSDI '17)
**Source:** `inspiration/papers/cherrypick-nsdi17.pdf`

Bayesian optimization to pick cloud VM types for recurring jobs with few trial runs.
Applicable to **offline tuning** of delegate-expression thresholds.

---

## The oscillation problem {#oscillation}

`docs/research/claude_comprehensive_research.md` lines 111–123.

Single-lift oscillation: a lift flips local→remote under load; cold-start latency causes the
delegate expression to flip back; load is still there; it flips again.

**Known mitigations** (all implemented in Kubernetes HPA):
- Schmitt trigger hysteresis (two thresholds with a gap)
- Time-windowed averaging
- Exponential smoothing
- Cooldown periods

**Multi-lift problem:** When N delegate expressions compete for shared CPU/bandwidth, independent
decisions constitute a multi-agent control problem with coupled dynamics. MARL literature
(MADDPG, COMA) finds learned policies that are locally optimal can be jointly unstable.
Centralized critics are usually necessary for convergence.

Four options for Monolift (`docs/research/claude_comprehensive_research.md` lines 119–122):
1. Central coordinator — simple, correct, non-scalable.
2. Decentralized with shared signal (global scalars like cluster load, tail latency).
3. Token-bucket admission for transitions — global rate limit on state changes.
4. Learned joint policies (MARL in simulator).

---

## SEDA

**Source:** `inspiration/papers/seda-sosp01.pdf`

Welsh, Culler, Brewer (SOSP '01). Per-stage event queues with admission control based on queue
length. Dynamic resource controllers scale threads per stage.

Key result for Monolift (`docs/research/claude_comprehensive_research.md` line 85):
*Per-stage controllers compose well as long as stages share no global resource knob*; once they
do, policy interactions produce oscillation. Exact warning for delegate expressions sharing CPU
and network on the same node.

---

## Noisy-neighbor effects

Delimitrou's Quasar (ASPLOS '14), Paragon (ASPLOS '13), Mars's Bubble-Up (MICRO '11):
compute-bound colocations can degrade each other 2×–3× through LLC and memory-bandwidth contention.
For Monolift: a delegate expression will see this as a latency spike without an obvious cause.
`docs/research/claude_comprehensive_research.md` line 97.
