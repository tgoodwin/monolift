# 09 — Distributed Correctness, Types, and Proofs

Monolift's bounded model: no global state, no heap sharing, no migration.
This section covers (a) what rules out problematic sharing statically, (b) what makes
sharing safe by construction, and (c) formal verification of distributed systems.

## The CALM Theorem {#calm}

**Source:** `inspiration/papers/calm-cacm20.pdf`

Hellerstein & Alvaro (CACM '20). A program has a coordination-free distributed implementation
**if and only if** it is expressible in monotonic logic.

**For Monolift** (`docs/research/claude_comprehensive_research.md` line 162):
This cleanly partitions the lift-safety question:
- **Monotone lifts** — free; no coordination needed; safe to distribute.
- **Non-monotone lifts** — require explicit coordination; opt-in; more expensive.

The CALM framing gives a principled characterization of when "no shared state" is the right
restriction. A medium-term paper: CALM-based static analysis that categorizes each candidate
lift and picks the cheapest coordination rung for non-monotone ones.

---

## LVars — lattice-based shared state

### LVars (Kuper & Newton — FHPC '13)
**Source:** `inspiration/papers/lvars-fhpc13.pdf`

### Freeze After Writing (Kuper, Turon, Krishnaswami, Newton — POPL '14)
**Source:** `inspiration/papers/lvars-freeze-popl14.pdf`

An LVar is a shared location whose state moves monotonically up a lattice. Reads are threshold
reads that block until a bound is reached. Provides deterministic parallelism without locks.

**For Monolift** (`docs/research/claude_comprehensive_research.md` line 160):
Monolithic patterns that look like shared state — counters, sets, caches with union semantics —
are often LVars in disguise. Monolift should recognize them as **safe to distribute**.

---

## CRDTs

**Source:** `inspiration/papers/crdts-sss11.pdf`

Shapiro, Preguiça, Baquero, Zawirski (SSS '11). Replicas converge via lattice join or
commutative operations. When shared state is genuinely mergeable, CRDTs are the right tool.
Most monolith shared state is not — but when it is, CRDTs make a lift safe.

---

## Session Types

### Binary Session Types (Honda, Vasconcelos, Kubo — ESOP '98)
**Source:** `inspiration/papers/session-types-esop98.pdf`

Original binary session types. Establishes the basic two-party communication calculus.

### Multiparty Asynchronous Session Types (Honda, Yoshida, Carbone — POPL '08)
**Source:** `inspiration/papers/mpst-popl08.pdf`

Global types project to local types per participant; well-typed participants cannot deadlock.
Every lift's RPC edge has an implicit session type — the pre/post conditions of the call.
MPST is the apparatus for checking that bi-directional streams stay in protocol.

### Go analysis (Gabet & Yoshida — ECOOP '20)
**Source:** `inspiration/papers/gabet-yoshida-ecoop20.pdf`

Session-type-inspired static analysis for Go channels and mutexes. Directly handles Go's
concurrency primitives. Monolift can reuse this analysis to validate generated RPC stubs.
`docs/research/claude_comprehensive_research.md` line 164.

---

## Monolift's anti-pattern table

From `docs/research/claude_comprehensive_research.md` lines 168–180 (Table in §8):

| Pattern | Problem on lift | Resolution |
|---------|----------------|------------|
| `sync.Map`, custom LRU (in-memory cache) | Cache shared across handlers | Distributed cache (Redis) or recognize as LVar |
| `sync.Mutex` | No shared address space | Rework as idempotent/monotone |
| `context.Context` with non-serializable values | Request-scoped pointers to DB handles | Re-inject on remote side |
| Goroutine closures capturing heap pointers | Captures dangle across processes | Materialize as explicit serializable args |
| Package-level `init()` state | Not replicated to lifted process | Pin to one host or deterministic init |
| `chan T` shared across lifted code | Channels are in-process | Generate queue/stream; check via MPST |
| `time.Now()`, `rand.Rand` | Non-deterministic across hosts | Seeded RNG or tag as non-replay-safe |

---

## Correctness escape-hatch ladder

`docs/research/claude_comprehensive_research.md` line 166:

1. **2PC** — atomicity at the cost of availability.
2. **Sagas** — long-running transactions with programmer-supplied compensations.
   Source: `inspiration/papers/sagas-sigmod87.pdf` (Garcia-Molina & Salem, SIGMOD '87).
3. **Durable execution** — Temporal/Cadence: checkpointed workflows with automatic retry/replay.
   Sources: `inspiration/html/temporal-*.html`

---

## RIFL

**Source:** `inspiration/papers/rifl-sosp15.pdf`

Lee et al. (SOSP '15). Implementing linearizability at large scale and low latency. Protocol
for making RPCs exactly-once semantics practical in distributed systems.

---

## Formal verification of distributed systems

### Verdi (Wilcox, Woos, Panchekha et al. — PLDI '15)
**Source:** `inspiration/papers/verdi-pldi15.pdf`

Framework for implementing and formally verifying distributed systems in Coq. Transformers
model network semantics (packet loss, duplication, etc.) and are verified compositionally.

### IronFleet (Hawblitzel, Howell, Kapritsos et al. — SOSP '15)
**Source:** `inspiration/papers/ironfleet-sosp15.pdf`

Proving practical distributed systems correct in Dafny. IronRSL (Paxos-based RSM) and IronKV
(key-value store) verified end-to-end. Demonstrates that verification of realistic systems
is tractable but expensive.

### Ivy (Padon, McMillan et al. — PLDI '16)
**Source:** `inspiration/papers/ivy-pldi16.pdf`

Safety verification by interactive generalization. Inductive invariants found by the tool;
user provides protocol structure.

**For Monolift** (`docs/research/claude_comprehensive_research.md` lines 252–253):
If the compiler pass is cast as endpoint projection, the correctness obligation reduces to
*"the generated RPC matches the local call's contract"* — small enough to be proof-tractable
for a bounded lift. A multi-year project but the clearest theoretical payoff.
