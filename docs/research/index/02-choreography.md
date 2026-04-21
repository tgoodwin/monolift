# 02 — Choreographic Programming

Choreographies describe a multi-party protocol from a **global viewpoint**; *endpoint projection*
mechanically derives each participant's local process. Deadlock-freedom follows from a theorem
relating the global spec to its projections.

## Foundational works

### Montesi PhD Thesis (2013)
**Source:** `inspiration/papers/montesi-choreo-phd13.pdf`
The canonical reference. Defines Chor, the first full choreographic programming language with
a verified endpoint projection. The metatheory that everything else cites.

### Montesi Monograph
**Source:** `inspiration/papers/montesi-choreo-book.pdf`
Book-length treatment of choreographic programming theory and practice.

### CLM '23
**Source:** `inspiration/papers/montesi-clm23.pdf`
"A Model for Choreographic Programming" — the most recent Montesi et al. formal model, extending
Chor with correlation sets and more expressive projection.

### Multiparty Session Types (Honda, Yoshida, Carbone — POPL '08)
**Source:** `inspiration/papers/mpst-popl08.pdf`
The session-type formalism underlying most choreographic correctness proofs. Global types project
to local types per participant; well-typed participants cannot deadlock.
`docs/research/claude_comprehensive_research.md` line 150: cited as the apparatus for checking
Monolift's RPC edges as implicit session types.

### Session Types Origin (Honda, Vasconcelos, Kubo — ESOP '98)
**Source:** `inspiration/papers/session-types-esop98.pdf`
Original binary session types. Establishes the basic calculus.

---

## Functional / typed formulations

### Pirouette (Hirsch & Garg, POPL '23)
**Source:** `inspiration/papers/pirouette-popl23.pdf` (arXiv 2111.03701)
Dependently-typed λ-calculus for choreographies. Fully formalized endpoint projection with a
proof that projected processes simulate the choreography. The formal high-water mark.
If Monolift's compiler pass is cast as endpoint projection, Pirouette provides the proof template.
`docs/research/claude_comprehensive_research.md` line 150.

Also see earlier MPI-SWS tech report: `inspiration/papers/pirouette-mpi-sws-2021.pdf`

### Functional Choreographic Programming
**Sources:**
- `inspiration/papers/functional-choreo-programming.pdf` (escholarship)
- `inspiration/papers/arxiv-2111.03701-functional-choreo.pdf`

### Choral (Giallorenzo, Montesi, Peressotti — TOPLAS '24)
**Source:** `inspiration/papers/choral-toplas24.pdf`
Object-oriented choreographic programming embedded in Java. Roles = type parameters.
Endpoint projection to plain Java via type erasure. The most industrially-accessible formulation.

### HasChor
**Source:** `inspiration/repos/HasChor` (cloned)
Choreographic programming library in Haskell. POPL '23 SRC entry.
See also `inspiration/html/haschor-popl23.html`.

---

## Application and experience reports

### Real-World Choreographic Programming
**Source:** `inspiration/papers/arxiv-2303.03983-real-world-choreo.pdf`

### Choreographic Quick Changes
**Source:** `inspiration/papers/choreo-quick-changes.pdf`
First-class location polymorphism in choreographies. Relevant to Monolift's per-lift placement:
if a lift's role can be polymorphic over location, it maps naturally to choreographic projection.

### A New Architecture for Choreographic Programming Languages (UVM)
**Source:** `inspiration/papers/new-arch-choreo-uvm.pdf`

### Library-Level Choreographic Programming
**Source:** `inspiration/papers/arxiv-2311.11472-library-choreo.pdf`

### COORDINATION 2015 — INRIA SPADES
**Source:** `inspiration/papers/coordination2015-inria.pdf`

### Concurrent Calculi Formalisation Benchmark
**Source:** `inspiration/papers/concurrent-calculi-benchmark.pdf`

---

## Relevance to Monolift {#endpoint-projection}

`docs/research/claude_comprehensive_research.md` lines 148–152:

> *"The question this literature asks of Monolift is: can the Monolift compiler pass be
> characterized as endpoint projection from an implicit choreography? If yes, Monolift inherits
> a correctness theorem for free… The answer is probably 'yes, for the subset of lifts whose
> state is isolated, and no, for lifts that share state through channels or shared data structures.'"*

A workshop paper casting Monolift's IR as a choreography calculus with projection into Go + RPC
is identified as one of the cleanest theoretical contributions available.

**Go-specific:** `inspiration/papers/gabet-yoshida-ecoop20.pdf` — session-type-inspired static
analysis of Go channels and mutexes (ECOOP '20). Directly reusable for validating Monolift's
generated RPC stubs against their implicit session type.
