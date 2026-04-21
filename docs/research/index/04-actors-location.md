# 04 — Actor Systems and Location Transparency

## The Waldo Critique {#waldo}

**Source:** Referenced throughout; see `inspiration/html/note-on-distributed-computing-s2.html`
and `docs/research/claude_comprehensive_research.md` lines 15–19.

Waldo, Wyant, Wollrath, Kendall (Sun TR-94-29, 1994): four irreducible differences between
local and remote invocation:
1. **Latency** — network is orders of magnitude slower than in-process.
2. **Memory access semantics** — no shared address space.
3. **Partial failure** — remote calls can fail in ways local calls cannot.
4. **Concurrency** — remote calls introduce new concurrency hazards.

Any system promising to paper over these differences will eventually force the programmer to
confront them at the worst possible time. CORBA, DCOM, Java RMI, and .NET Remoting all did this.

**Monolift's structural answer** (`docs/research/claude_comprehensive_research.md` lines 18–19):
Monolift does *not* claim transparency. It claims opt-in per lift. The delegate expression DSL
is the surface where the four Waldo differences re-surface as **first-class inputs**.

---

## The CORBA Postmortem

**Source:** `inspiration/papers/corba-rise-fall-queue06.pdf`
Henning, ACM Queue 2006.

CORBA hid distribution *aspirationally*: failure modes were worse than the problems it replaced.
Proxies, `oneway` modifiers, apartment models — all were reinventions of the differences Waldo
enumerated. The root cause: conflating logical and physical boundaries.
`docs/research/claude_comprehensive_research.md` line 17.

---

## Actor Model Lineage

### Pony (Clebsch, Drossopoulou et al. — AGERE! 2015)
**Sources:**
- `inspiration/papers/pony-string-of-ponies.pdf`
- `inspiration/papers/pony-deny-capabilities.pdf` — *Deny Capabilities for Safe, Fast Actors*
- `inspiration/papers/capability-systems-encore-pony-rust.pdf`
- `inspiration/papers/actor-gc-icooolps15.pdf`
- `inspiration/papers/fast-cheap-msg-agere.pdf`
- `inspiration/html/ponylang-papers.html`

The formal high-water mark for actor systems. Six capability qualifiers —
`iso`, `val`, `ref`, `box`, `tag`, `trn` — statically rule out data races without GC synchronization.
An `iso` reference is the only mutable reference to its object; a `val` is immutable and freely shareable.

**Relevance to Monolift:** The capability lattice *is* the condition under which "no shared state" holds.
Go has no analogous discipline, so Monolift's compiler must reconstruct Pony-style sendability
via escape analysis (p.3 §3.1 of monolift-plos25.pdf: compiler refuses lifts with heap operations).
`docs/research/claude_comprehensive_research.md` line 29.

Also: `inspiration/papers/diva2-2013047.pdf` — comparative capabilities study (Encore, Pony, Rust).

### Orleans (Bykov, Geller, Kliot et al. — SoCC 2011)
**Source:** `inspiration/papers/orleans-socc11.pdf`

*Virtual actor* or *grain*: an actor identified by a stable logical key, materialized on some server
on demand. Turn-based single-threaded execution. The "where does the actor live?" question answered
with "don't ask; ask what it is."

Closest in spirit to Monolift's runtime-directed placement — a lift is a piece of code whose
location is a runtime property, not a source-level property. The key difference: Orleans grains
are long-lived stateful entities; Monolift lifts are stateless call-site reinterpretations.
`docs/research/claude_comprehensive_research.md` lines 25–26.

### Ray (Moritz, Nishihara et al. — OSDI '18)
**Source:** `inspiration/papers/ray-osdi18.pdf`
**HTML docs:** `inspiration/html/ray-serve-architecture.html`, `inspiration/html/ray-getting-started.html`

`@ray.remote` decorator converts a Python function/class into a remotely-invokable task or actor.
Plasma object store separates data movement from task dispatch. Ownership model: every remote
object has exactly one owner; failures propagate via fate-sharing.

Most successful modern actor-adjacent system. Closest existing analog to Monolift's annotation +
runtime model. Ownership discipline is a design pattern Monolift should adopt when a lift's
result outlives the caller's scope.
`docs/research/claude_comprehensive_research.md` lines 27–28.

### Cloud Haskell (Epstein, Black, Peyton Jones — Haskell Symp. 2011)
**Source:** `inspiration/papers/cloud-haskell-haskell11.pdf`

Solved actor-style distribution with static closures and serializable `Closure a` values.
The lesson: crossing a process boundary is a **typed operation**.
`docs/research/claude_comprehensive_research.md` line 29.

---

## Temporal and Durable Execution

**HTML sources:**
- `inspiration/html/temporal-docs.html`
- `inspiration/html/temporal-durable-execution.html`
- `inspiration/html/temporal-queues-workflows.html`
- `inspiration/html/temporal-observability.html`
- `inspiration/html/temporal-architecture-medium.html`
- `inspiration/html/aws-temporal-resilient.html`

Temporal/Cadence: checkpointed workflow execution with automatic retry/replay on failure.
Durable execution is the highest rung of Monolift's correctness escape-hatch ladder
(2PC → sagas → durable execution). Most expressive; most expensive.
`docs/research/claude_comprehensive_research.md` line 166.

---

## Dapr

`docs/research/claude_comprehensive_research.md` line 39:
Dapr (Microsoft) is a sidecar runtime exposing distributed-systems primitives over gRPC/HTTP.
Zero compile-time work but substantial runtime cost (every call through the sidecar).
Monolift inverts the tradeoff: compile-time cost, minimal runtime overhead.
