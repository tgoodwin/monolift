# Monolift 

**Monolift is a compiler-based technique for automatically refactoring applications into distributed, cloud-native architectures.** The core abstraction of Monolift's approach is the *lift*, which is region of application code that can run locally or remotely. Developers create lifts declaratively by adding annotations to their application code. Monolift's compiler then extracts the annotated code regions into independently deployable artifacts that enable the application to run as a distributed system to more effectively leverage the compute resources available in the cloud.

The key feature of Monolift's design is that it supports *existing* applications, and does not require users to first adopt a new framework or programming model in order to reap Monolift's benefits. Consequently, Monolift cannot rely on assumptions about the structure of the code it seeks to support, creating a slew of interesting design challenges for Monolift's compiler.

We presented an early prototype of Monolift in our PLOS '25 workshop paper, which articulated these challenges but left many of them unsolved. This website serves as a follow up to our workshop paper, documenting the research process of solving these design challenges to fully realize our Monolift vision (we'll call the fully realized vision "V2").


!!! tip "Completely new here? Start with the walkthrough"
    **[How Monolift works, in one example](walkthrough.md)** presents an end-to-end 
    example of using Monolift — extracting a `processImage` function from a real-world monolithic codebase to standalone service. In the process, it introduces every term used across the rest of this site.

## The original idea, and what's changing in V2

The workshop paper's prototype (V1) demonstrated the core idea that a monolithic codebase could be transformed into a distributed architecture. The prototype implemented **lifts** and demonstrated that the "lifted" architecture could get the best of both worlds in terms of the monolith-vs-distributed tradeoff. However, the application we used to evaluate the prototype was excessively simplistic. We "de-distributed" one of the toy apps from the DeathStarBench suite to serve as a monolithic application baseline. As a result, our toy app already contained sufficient modularity, the code was largely stateless, and calling conventions at module boundaries were already friendly to wire formats (pass by value etc).

The evaluation target of V1 was insufficiently realistic, so the first step I took with V2 was to look at some [real-world Go monoliths](evaluation-targets.md). This exercise revealed that most code worth lifting is messier to extract (i.e. less modularity, complex parameter types, local state). The primary objetive of V2 is to uphold the core claims from V1, but replace its's simplifying assumptions with support for **real-world code**. Where V1 was hardcoded to a single application shape, V2 is designed around a set of general questions: can this region of computation safely cross a network, what state does it carry, how does the program
reach it, and if we want to lift it, where should we insert the network boundary? 

Each main page of this site takes one of those questions, shows the design
pressure that motivated the answer, shows the compiler code that now handles
it, and pairs it with a code excerpt from one of the open-source Go monoliths the V2 compiler
is being developed against.

??? abstract "Background: the workshop paper's pitch and the prototype's limits"
    The initial workshop paper, *Monolift: automating distribution with the tools
    you have at home* (PLOS '25), pitched Monolift as a Go compiler and runtime
    that lets a developer annotate ordinary function definitions and have selected
    call sites turn into remote invocations automatically — without restructuring
    the code, adopting a new framework, or switching languages. The "tools you have
    at home" are the Go toolchain and the Kubernetes cluster the developer already
    uses. The program continues to build and run as a plain monolith when the
    Monolift compiler is not in the loop, and distribution decisions can be
    expressed as policies the runtime evaluates per call.

    The prototype that shipped alongside the paper was developed against a small
    synthetic demo and implemented a deliberately narrow slice of that vision: one
    annotation surface (interfaces), one function signature (functions that looked
    like HTTP handlers), one state model (stateless functions), and one deployment
    target (Kubernetes). Almost every function worth lifting in a real codebase sat
    outside one of those assumptions — which is what motivated the reboot described
    above.

## Terms used throughout

| Term | Meaning |
|---|---|
| **Lift target** | The region the developer wants to run remotely. |
| **Region root** | The root function of that region. |
| **Activation path** | The static path from program entrypoint to the region root. |
| **Cut point** | The function where Monolift inserts the network boundary. |
| **Network boundary** | The split between monolith-local execution and remote-service execution. |
| **Boundary adapter** | A synthesized local wrapper plus normalized remote helper that manufactures a clean boundary at a cut whose source signature is awkward. |
| **Admission** | The decision that a region is safe enough to lift. |
| **Transport selection** | The later choice of adapter shape, such as an HTTP-oriented adapter. |
| **State archetype** | A named model for stateful code, with explicit preconditions. |
| **Refusal code** | A stable `MLV2_*` diagnostic explaining why a lift is rejected. |

## How to read this site

Start with [How Monolift works, in one example](walkthrough.md) for the
end-to-end story on a single function. If you are returning from the workshop
paper, [What's changing after the initial workshop paper](v1-to-v2.md) explains
which paper commitments were preserved, revised, or retired, and
[Evaluation targets](evaluation-targets.md) describes the real-world Go monolith
codebases referenced throughout the site. The remaining technical pages then move
from admission, to state, to refusals, to the two path problems: recovering how
the program reaches the lift target and deciding where the network boundary belongs.


## Sections

- [**How Monolift works, in one example**](walkthrough.md) — the end-to-end
  story, following listmonk's `processImage` from monolith to lifted service.
  The recommended starting point.
- [**What's changing after the initial workshop paper**](v1-to-v2.md) —
  the contract renegotiation, traced on a single function that was
  infeasible under v1 and is admitted under v2.
- [**Evaluation targets**](evaluation-targets.md) — the pinned
  open-source Go monoliths the v2 compiler is being developed against.
- [**Reasoning about liftability**](canonical-shapes.md) —
  describes a named liftability-property vocabulary the compiler uses to evaluate what code regions can be extracted for distribution.
- [**Pattern matching on stateful code**](state-class-inference.md) —
  why the paper's rule that lifts must be stateless was relaxed, and
  how state archetypes build on liftability properties, form candidate
  sets, and resolve overlapping matches.
- [**Making the compiler opinionated**](refusal-diagnostics.md) — how
  the paper's commitment to refusing lifts the compiler cannot
  distribute reliably is implemented and made concrete through a named set of
  refusal codes.
- [**Recovering activation paths**](activation-paths.md) —
  how the compiler recovers the control flow path from `main()` to a lifted
  region, so it can reason about where to place the network boundary.
- [**Drawing the network boundary**](cut-placement.md) — how the compiler
  decides where on the activation path to insert the network boundary.
  The lift target and the cut point are not always the same function;
  a decision tree over six dimensions picks the best candidate.
- [**Adapting the network boundary**](boundary-adapters.md) — what the
  compiler does when a region is the right unit to lift but no function
  on activation path presents a clean boundary: synthesize a local wrapper and
  a normalized remote helper, packaging parameters and returns into
  DTOs. A recovery fallback today, with the cut-here-vs-adapt-there
  tradeoff left as open work.
- [**Code extraction**](extraction.md) — once a cut point is chosen,
  how the compiler pulls the function body out of the monolith and
  renders the boundary scaffolding around it to produce a *lift*;
  which reconstructors and receiver policies make a candidate
  admissible; and how admission feeds back into placement. The page
  describes the current long-running-pod backend, but the extraction
  phase is runtime-agnostic.
- [**Working backwards from real code**](working-backwards.md) — the
  research-and-development strategy: pick lift candidates from six
  real codebases, let those candidates dictate which compiler
  capabilities to build next, and treat per-target progress as a
  capability map the project diffs sprint-over-sprint.
- [**Finding your way around the code**](reading-guide.md) — how to map
  the narrative pages back to `pkg/compiler/`, `pkg/activation/`,
  `pkg/codegen/`, and the ADR log.

## External references

- Initial workshop paper: [*Monolift: automating distribution with the
  tools you have at home*](https://dl.acm.org/doi/10.1145/3764860.3768327),
  PLOS '25.
- [ADR directory](https://github.com/tgoodwin/monolift/tree/main/docs/decisions)
  — architectural decision records.
- [`docs/evolution.md`](https://github.com/tgoodwin/monolift/blob/main/docs/evolution.md)
  — narrative timeline linking ADRs to sprints.
