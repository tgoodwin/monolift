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

The workshop paper demonstrated the mechanism: ordinary Go code could
remain a monolith by default while selected calls became remote
invocations under a compiler/runtime policy. The reboot keeps that core
claim, but replaces the prototype assumptions that did not survive real
code.

The prototype mostly knew how to lift stateless HTTP-handler-shaped
functions wired near `main()`. Real Go monoliths are not that regular:
useful lift targets may be ordinary domain functions, methods on
stateful receivers, callbacks registered with frameworks, or handlers
hidden behind application-specific dispatch. The reboot therefore moves
Monolift from recognizing one favored shape to asking a more general
set of questions: can this region safely cross a network, what state
does it carry, how does the program reach it, and where should the
network boundary actually go?

The result is not a new project in place of the paper. It is a stricter
compiler contract for making the paper's claim work against production
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
defines the pinned corpus used throughout the site. The technical pages
then move from admission, to state, to refusals, to the two path
problems: recovering how the program reaches the target and deciding
where the network boundary belongs.

Most pages open with either an **"At a glance"** paper-delta section or
a **"Research question and result"** section. Readers who want the
high-level argument can stop there; readers who want the implementation
details can keep going into the paired compiler and corpus examples.

## Sections

- [**What's changing after the initial workshop paper**](v1-to-v2.md) —
  the contract renegotiation, traced on a single function that was
  refused under v1 and is admitted under v2.
- [**Evaluation targets**](evaluation-targets.md) — the pinned
  open-source Go monoliths the compiler is developed against.
- [**Reasoning about liftability**](canonical-shapes.md) —
  why the paper's assumption that lift admission could be recognized
  from HTTP-handler-like signatures was replaced by a named
  liftability-property vocabulary.
- [**Pattern matching on stateful code**](state-class-inference.md) —
  why the paper's rule that lifts must be stateless was relaxed, and
  how state archetypes build on liftability properties, form candidate
  sets, and resolve overlapping matches.
- [**Making the compiler opinionated**](refusal-diagnostics.md) — how
  the paper's commitment to refusing lifts the compiler cannot
  distribute reliably is kept, and made concrete through a named set of
  refusal codes.
- [**Recovering activation paths**](activation-paths.md) —
  how the compiler recovers the path from `main()` to a lifted
  region, so it knows where to place the network boundary.
  Designed empirically: 72 reviewed traces across 6 codebases
  guided incremental algorithm development to 71/72 coverage.
- [**Drawing the network boundary**](cut-placement.md) — how the compiler
  decides where on the activation path to insert the network boundary.
  The lift target and the cut point are not always the same function;
  a decision tree over six dimensions picks the best candidate.
- [**Finding your way around the code**](reading-guide.md) — how to map
  the narrative pages back to `pkg/compiler/` and the ADR log.

## External references

- Initial workshop paper: [*Monolift: automating distribution with the
  tools you have at home*](https://dl.acm.org/doi/10.1145/3764860.3768327),
  PLOS '25.
- [ADR directory](https://github.com/tgoodwin/monolift/tree/main/docs/decisions)
  — architectural decision records.
- [`docs/evolution.md`](https://github.com/tgoodwin/monolift/blob/main/docs/evolution.md)
  — narrative timeline linking ADRs to sprints.
