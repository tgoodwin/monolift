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
alongside the paper was developed against a contrived toy demo and
implemented a deliberately narrow slice of the vision: one annotation
surface (interfaces), one function signature (functions that looked
like HTTP handlers), one state model (stateless functions), and one
deployment target (Kubernetes). Inspecting [real-world Go
monoliths](evaluation-targets.md) made clear that several of those
simplifying assumptions do not hold in practice — almost every
function worth lifting sat outside one of them.

**What this site explains.** Each of the four main pages takes one of
those simplifying assumptions, shows the design pressure that broke
it, and shows the compiler code that now handles the revised case —
paired with an excerpt from one of the open-source Go monoliths the
compiler is being developed against.

## How to read this site

Each main page opens with an **"At a glance"** section that names the
paper's claim in the paper's own vocabulary, explains why the claim did
not hold up on real-world Go monoliths, and tags it as *preserve*,
*revise*, or *retire*. The rest of the page is the implementation
close-up: the compiler code that realizes the revised claim, paired
with an excerpt from the real-world project that forced the revision.
Readers who want the delta from the paper can stop after each "At a
glance" block; readers who want the implementation can keep going.

## Sections

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
- [**Recovering activation paths**](entrypath-bridge.md) —
  why the compiler needs an activation graph between application roots
  and region roots before it can choose a distribution cut point.
- [**What's changing after the initial workshop paper**](v1-to-v2.md) —
  the contract renegotiation, traced on a single function that was
  refused under v1 and is admitted under v2.

## External references

- Initial workshop paper: [*Monolift: automating distribution with the
  tools you have at home*](https://dl.acm.org/doi/10.1145/3764860.3768327),
  PLOS '25.
- [ADR directory](https://github.com/tgoodwin/monolift/tree/main/docs/decisions)
  — architectural decision records.
- [`docs/evolution.md`](https://github.com/tgoodwin/monolift/blob/main/docs/evolution.md)
  — narrative timeline linking ADRs to sprints.
- [Finding your way around the code](reading-guide.md) — how to navigate
  `pkg/compiler/`.
- [Evaluation targets](evaluation-targets.md) — the open-source Go
  monoliths the compiler is developed against, with a one-line summary
  of each.
