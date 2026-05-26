# Monolift — design story

**Monolift takes a piece of a Go monolith and runs it as its own service —
without rewriting the program.** Suppose your app has a CPU-heavy image-resize
helper buried inside a web handler, and you would like it to run on its own
machines. Doing that by hand means writing a network layer, serializing the
arguments, deploying a service, and threading failures back through the call
site. Monolift does it for you: you mark the code, and the compiler generates the
network boundary while the call site — and the rest of the program — stays
exactly as it was.

!!! tip "New here? Start with the walkthrough"
    **[How Monolift works, in one example](walkthrough.md)** follows one real
    function — listmonk's `processImage` — from monolith to lifted service, and
    introduces every term used across the rest of this site.

## The idea, and what changed

The initial workshop paper demonstrated the core idea: ordinary Go code could
stay a monolith by default while selected calls became remote invocations under a
compiler-and-runtime policy — using only the Go toolchain and the Kubernetes
cluster the developer already has. The prototype that shipped with the paper
proved this on a narrow slice: stateless, HTTP-handler-shaped functions wired
close to `main()`.

Inspecting [real-world Go monoliths](evaluation-targets.md) showed that most code
worth lifting falls outside that slice. It may be ordinary domain functions,
methods on stateful receivers, callbacks registered with frameworks, or handlers
hidden behind application-specific dispatch. The reboot keeps the core claim but
replaces those simplifying assumptions, moving Monolift from recognizing *one
hardcoded shape* to asking a general set of questions: can this region of
computation safely cross a network, what state does it carry, how does the program
reach it, and where should the network boundary actually go? The result is a
stricter compiler contract for making the paper's claims hold against production
Go monoliths.

Each main page of this site takes one of those questions, shows the design
pressure that motivated the answer, and shows the compiler code that now handles
it — paired with an excerpt from one of the open-source Go monoliths the compiler
is developed against.

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
