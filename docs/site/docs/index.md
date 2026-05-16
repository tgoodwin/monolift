# Monolift — design story

**The vision.** The initial workshop paper, *Monolift: automating
distribution with the tools you have at home*, pitched Monolift as a Go
compiler and runtime that lets a developer annotate ordinary function
definitions and have selected call sites turn into remote invocations
automatically — without restructuring the code, adopting a new
framework, or switching languages. The "tools you have at home" are
the Go toolchain and the Kubernetes cluster the developer already uses.
The program continues to build and run as a plain monolith when the
Monolift compiler is not in the loop, and distribution decisions can
be expressed as policies the runtime evaluates per call.

**What the initial prototype actually did.** The prototype that shipped
alongside the paper was developed against a small synthetic demo and
implemented a deliberately narrow slice of the vision: one annotation
surface (interfaces), one function signature (functions that looked
like HTTP handlers), one state model (stateless functions), and one
deployment target (Kubernetes). Inspecting [real-world Go
monoliths](evaluation-targets.md) made clear that several of those
simplifying assumptions do not hold in practice — almost every
function worth lifting sat outside one of them.

**What this site explains.** Each of the main pages takes one of those
simplifying assumptions or open design questions, shows the design
pressure that motivated the answer, and shows the compiler code that now
handles it — paired with an excerpt from one of the open-source Go
monoliths the compiler is being developed against.

## The reboot thesis

The workshop paper demonstrated the core idea: ordinary Go code could
remain a monolith by default while selected calls became remote
invocations under a compiler/runtime policy. The reboot keeps that core
claim, but replaces the prototype assumptions that do not hold in real-world
application codebases.

The prototype mostly knew how to lift stateless HTTP-handler-shaped
functions wired near `main()`. Real Go monoliths are not that regular:
useful lift targets may be ordinary domain functions, methods on
stateful receivers, callbacks registered with frameworks, or handlers
hidden behind application-specific dispatch. The reboot therefore moves
Monolift from recognizing one hardcoded shape to asking a more general
set of questions: can this region of computation safely cross a network, what state
does it carry, how does the program reach it, and where should the
network boundary actually go?

The result is a stricter
compiler contract for making the workshop paper's claims work against production
Go monoliths.

## Terms used throughout

| Term | Meaning |
|---|---|
| **Lift target** | The region the developer wants to run remotely. |
| **Region root** | The root function of that region. |
| **Activation path** | The static path from program entrypoint to the region root. |
| **Cut point** | The function where Monolift inserts the network boundary. |
| **Network boundary** | The split between monolith-local execution and remote-service execution. |
| **Admission** | The decision that a region is safe enough to lift. |
| **Transport selection** | The later choice of adapter shape, such as an HTTP-oriented adapter. |
| **State archetype** | A named model for stateful code, with explicit preconditions. |
| **Refusal code** | A stable `MLV2_*` diagnostic explaining why a lift is rejected. |

## How to read this site

If you are returning from the workshop paper, start with
[What's changing after the initial workshop paper](v1-to-v2.md). That
page explains which paper commitments were preserved, revised, or
retired. Then read [Evaluation targets](evaluation-targets.md), which
describes the set of real-world Go monolith codebases referenced throughout the site. The technical pages then move from admission, to state, to refusals, to the two path
problems: recovering how the program reaches the lift target and deciding
where the network boundary belongs.


## Sections

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
- [**Code extraction**](extraction.md) — once a cut point is chosen,
  how the compiler pulls the function body out of the monolith and
  renders the boundary scaffolding around it to produce a *lift*;
  which reconstructors and receiver policies make a candidate
  admissible; and how admission feeds back into placement. The page
  describes the current long-running-pod backend, but the extraction
  phase is runtime-agnostic.
- [**Stages of evidence**](validation-ladder.md) — the 0–10 e2e ladder
  the harness uses to grade a lift, with each rung tied to a specific
  claim (compile, deploy, reach, transcript-compare, fail-mode).
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
