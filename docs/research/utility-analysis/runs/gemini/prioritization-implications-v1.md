# Prioritization implications v1 — SPRINT-0015 (Gemini run)

This document re-evaluates the prioritization of Monolift's distribution archetypes based on their qualitative utility analysis, contrasting it with the "corpus breadth" prioritization from SPRINT-0013.

## 1. Comparison of Prioritization Models

| Rank | SPRINT-0013 (Coverage-Based) | SPRINT-0015 (Utility-Based) | Rationale for Change |
|---|---|---|---|
| 1 | `periodic-invocation` | `periodic-invocation` | Highest utility for isolation; lowest risk. |
| 2 | `serialized-actor` | `bounded-worker-pool` | More universal scaling win; clearer transform. |
| 3 | `keyed-partitioned-state` | `ttl-cache` | Faster path to immediate performance wins. |
| 4 | `ttl-cache` | `fanout-publisher` | High decoupling value; standard architecture. |
| 5 | `bounded-worker-pool` | `serialized-actor` | High risk of bottlenecking if misapplied. |
| 6 | `fanout-publisher` | `keyed-partitioned-state` | High complexity/consistency trade-offs. |
| 7 | `session-affinity-state` | `session-affinity-state` | Essential but niche (WebSockets/long-conns). |
| 8 | `filesystem-bound-singleton`| `filesystem-bound-singleton`| High latency risk; niche coverage. |

## 2. Key Re-prioritization Arguments

### 2.1 The Case for `periodic-invocation` at #1
It remains the strongest candidate. It solves a real "noisy neighbor" problem in almost every target (Miniflux, Caddy, Gitea) with minimal consistency risk and a very clean transform (Platform Scheduler). It is the perfect "demonstration of utility" for the compiler.

### 2.2 Promoting `bounded-worker-pool` to #2
While it had slightly lower coverage (4 targets) than `serialized-actor` (5 targets), its utility is more robust. Every modern distributed system relies on worker queues for throughput. The transform is well-understood, and the code-structural tells are clear. It provides a more "standard" scaling story than the actor model.

### 2.3 Promoting `ttl-cache` to #3
Lifting a cache to Redis/Memcached is a "day 1" optimization for most scaling monoliths. The utility-analysis shows it as a low-risk way to reduce DB load and improve latency. It is often the first thing a developer would manually extract, making it a high-value auto-lift target.

### 2.4 Demoting `serialized-actor` to #5
Despite high coverage, it is the most dangerous archetype to auto-lift. If the compiler incorrectly identifies a global "God Object" as a serialized actor (e.g., Pocketbase's core app), it will create a massive performance bottleneck. Its utility is highly conditional on the state being truly "entity-scoped." It should follow the simpler, more robust archetypes.

### 2.5 Demoting `keyed-partitioned-state` to #6
The complexity of consistent hashing and the risk of breaking cross-key invariants make this a "high effort, high risk" archetype. While its potential utility for massive scale is high, the "Monolift sweet spot" (mid-scale monoliths) is likely better served by caches and worker pools first.

## 3. Implications for Future Sprints

- **Sprint 0016 Implementation:** Focus exclusively on the "High Utility / Low Risk" cluster: `periodic-invocation` and `bounded-worker-pool`. These provide the best "before-and-after" stories for a paper or demo.
- **Reporting & Remediation (ADR-0019):** For risky archetypes like `serialized-actor`, the compiler should lean heavily into `SUGGEST` rather than `AUTO`, providing clear warnings about potential bottlenecking on global state.
- **Pragma Strategy (ADR-0021):** Use pragmas to allow users to "opt-in" to the riskier archetypes (`keyed-partitioned-state`, `serialized-actor`) where the compiler's utility analysis is uncertain.

## 4. Conclusion

Utility-based prioritization shifts the focus from "what can we lift?" to "what *should* we lift?" This reordering favors archetypes that align with standard cloud-native scaling patterns and provide clear, measurable benefits with minimal risk of performance regression or consistency breakage.
