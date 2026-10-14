# Monolift research agenda for semantic partitioning

October 13, 2026 — working research direction

Monolift will investigate how a programming framework can expose semantically justified partitioning choices and make those choices available to different deployment policies. A possible partition is an admissible network boundary: a place where an application can be split under explicit semantic rules and assumptions. The framework should establish which choices are available and what each requires, while allowing developers and deployment policies to choose among them.

The central research question is:

> How can we characterize possible partitions of an application from its program semantics, and use those partitions to support different development and deployment objectives?

This agenda connects three concerns: the semantics that make a partition admissible, the alternative decompositions those partitions permit, and the operational choices available within that space. The form of a partition remains an open design question; functions, components with owned state, and larger regions may require different rules.

## Why the framing is changing

The workshop framing emphasized the expense of refactoring a monolith into distributed services. Its evaluation began with a highly modular application that was readily partitionable and explored performance-oriented distribution strategies. The spring 2026 work, referred to here as v2, pursued production applications such as Gitea and Mattermost, requiring substantial compiler and analysis work to handle less convenient program structure.

The working motivation for this refocusing is that agentic coding reduces the effort of producing refactorings while leaving their reliability an important open problem. A research contribution centered on saving manual edits is therefore less compelling than an account of what distribution is allowed to change, which boundaries support it, and what evidence justifies those conclusions.

Extending support to increasingly complicated applications remains useful engineering. The research agenda needs questions whose answers explain more than how many application shapes a compiler supports. Both existing settings can contribute: the workshop application exposes deployment choices when modularity is already present; v2 exposes dependencies that constrain those choices. Neither setting needs to be discarded or treated as the sole model of success.

There is also substantial overlap with prior work on automated distribution. Ignis already combines module-boundary transformations, selective scale-out, and developer-supplied recipes governing distribution semantics. Its treatment of state and ordering makes a distinction based simply on “preserving behavior” insufficient. Monolift's prospective contribution must be established through the particular semantic account, analysis, and deployment freedoms it provides; using a compiler alone does not establish novelty. See [Ignis, sections 1–3](https://nikos.vasilak.is/p/ignis:pldi:2019.pdf).

## Semantic foundations of possible partitions

The first question is what makes a network boundary admissible. Admissibility should be defined relative to an explicit execution model and contract. Relevant concerns include ownership and aliasing, mutable state, external effects, values crossing the boundary, ordering, resource lifetimes, and observable failures. The analysis must account for relevant dependencies beyond a function's signature.

Go remains a reasonable prototype target. Its explicit error-return convention provides a natural place to report failures, but the existence of an `error` result does not establish that callers correctly handle new distributed outcomes. A lost response can leave the caller uncertain whether an effect occurred. Retrying or falling back to local execution may require additional justification.

Monolift should make these differences explicit. It does not aim to make RPCs indistinguishable from local calls. A promising scope restriction is to decline responsibility for automatically synchronizing shared mutable state across a boundary. This can still permit components with privately owned state or explicit access to external storage. Remote placement, replication, and migration may each require different obligations.

The formal work should identify a relevant subset of Go semantics and define admissibility and transformation guarantees against it. A complete semantics of Go is not an assumed prerequisite. The connection between the formal model and supported Go programs must nevertheless be explicit, including assumptions about libraries and external services.

The key questions are which application properties remain invariant across a partition, which additional behaviors are allowed by its distribution contract, and what the compiler can establish about each. The appropriate formal relationship—such as equivalence under restricted conditions or refinement of an explicitly distributed specification—remains to be determined.

## From possible boundaries to application partitionings

An admissible boundary identifies a possible partition. A collection of such boundaries may offer several ways to decompose an application. The framework should investigate how to represent this space and the constraints within it.

Individual admissibility may not be sufficient for joint admissibility. Multiple cuts can interact through ownership, effects, ordering, and failure propagation. One research question is whether local obligations compose into guarantees about an entire partitioning, and when a broader analysis is necessary.

This also separates semantic possibility from implementation support and operational preference. A boundary can fit the semantic model while lacking a generator implementation; an implemented partition can be valid yet unattractive for a particular workload. The existing v2 distinction between cut placement and code-generation admission provides a starting point for refining these concepts, rather than a settled formal definition. See [Drawing the network boundary](../site/docs/cut-placement.md).

## Deployment policies over the available choices

Partitioning can support objectives beyond distributing bottlenecks for higher throughput. A control plane could choose among admissible decompositions and placements according to resource availability, cost, latency requirements, or isolation needs.

Resource-constrained clusters provide one promising setting. When peak performance is unnecessary, could finer placement choices reduce the resources required to meet a service objective? Could some components be consolidated while others remain separately placed? The benefit is a hypothesis: communication, serialization, duplicated runtime state, and coordination can consume or outweigh any savings.

The workshop's distribution strategies can be reconsidered as policies operating over available partitions. Further experiments should ask what additional choices the framework enables and when those choices matter relative to whole-application placement or replication. This keeps the analysis and the control plane connected while allowing them to answer distinct questions.

Choosing a partitioning at deployment time and changing it during execution are separate scopes. Live changes introduce obligations concerning state transfer and in-flight work. The agenda does not yet commit to dynamic repartitioning or migration.

## Local development of distributed applications

Another potential benefit is allowing developers to design applications with explicit distribution boundaries and exercise them locally. This includes intentionally modular applications as well as existing applications analyzed by the compiler.

The research question is what a local execution can establish about executions under other placements. A local mode that uses ordinary calls may preserve accidental pointer sharing or omit transport failures. A mode that enforces boundary protocols could provide stronger correspondence, while still requiring an account of failures and concurrency that local testing actually exercises.

The desired result is a clear relationship between development modes and deployment semantics: which properties transfer, under which assumptions, and which require additional testing. Convenient local execution is valuable, but its guarantees should follow from the same boundary contracts used by the partitioning analysis.

## Diagnostics and targeted refactoring

V2's refusal codes offer a supporting application of the semantic account. They can explain why a requested partition cannot currently be admitted and identify obligations that a targeted refactoring might resolve. The existing [refusal-guided refactoring discussion](../site/docs/evaluation-questions/refusal-codes-guide-refactoring.md) already considers agents as consumers of this feedback.

Diagnostics should distinguish an established semantic conflict, a condition the analysis cannot establish, and an implementation limitation. Those findings require different responses; refusal is relative to the framework's contract and capabilities, rather than a universal impossibility claim.

An agent could use structured evidence to propose a change, then rerun the analysis. Establishing that the resulting program admits a partition and establishing that the refactoring preserves intended application behavior remain separate obligations. This direction can demonstrate the value of the framework without making agent integration the central research contribution.

## How the existing work supports the agenda

The workshop application provides a setting for studying deployment policies with comparatively straightforward boundaries. V2 provides concrete analysis machinery, boundary adapters, refusal diagnostics, and experience with dependencies in production applications. Together they can ground the semantic model and its evaluation.

The primary measure of progress should be the explanatory power and utility of the framework: whether it defines meaningful guarantees, identifies useful deployment choices, and makes their constraints understandable. Corpus coverage and compiler scalability remain important evidence of applicability. They do not by themselves answer the central research questions.

## Questions for subsequent research sessions

The next sessions should sharpen the following questions before turning the agenda into implementation sprints:

1. What execution and failure model should define admissibility, and what properties should hold across supported placements?
2. What forms can a partition take, and what semantic information is required to characterize each?
3. How do admissible boundaries compose into a valid application partitioning?
4. Which subset of Go semantics supports the desired claims, and how will the implementation connect to that model?
5. What does this account add relative to prior distribution frameworks, their semantic guarantees, and their developer obligations?
6. Which operational or development use case best demonstrates the value of exposing a space of possible partitions?

These questions define a research program. The eventual paper scope can select a coherent subset once the semantic claim and its practical consequences become clearer.
