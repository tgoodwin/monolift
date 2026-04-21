# 03 — Multitier and Tierless Programming

Systems where a single program spans multiple execution tiers (client/server, browser/DB, local/remote),
with the compiler handling tier separation and communication.

## Survey

### A Survey of Multitier Programming (Weisenburger, Wirth, Salvaneschi — ACM CSUR '20)
**Source:** `inspiration/papers/multitier-survey-csur20.pdf`

The authoritative taxonomy. Three axes:
- **Placement strategy**: explicit annotation, inference, or mixed.
- **Communication style**: remote call, reactive signal, or channel.
- **Consistency model**: none, eventual, or transactional.

Against this: Monolift = *explicit annotation* + *remote call* + *no shared consistency*.
`docs/research/claude_comprehensive_research.md` line 142.

**Monolift's gap:** Every multitier system requires upfront adoption (write Links, port to ScalaLoci,
commit to Ur/Web). Monolift's genuine novelty against multitier is **legacy-compatibility** — it
adds lifts to unmodified Go rather than requiring a new language.
`docs/research/claude_comprehensive_research.md` line 146: *"a previously unoccupied cell"* in
the Weisenburger taxonomy.

---

## Major systems

### ScalaLoci (Weisenburger, Köhler, Salvaneschi — OOPSLA '18)
**Source:** `inspiration/papers/scalaloci-oopsla18.pdf`
**Repo:** `inspiration/repos/scala-loci`

First-class placement types in Scala. Peers declared with `@multitier`, placements via `on[Peer]`.
Reactive propagation across placement boundaries via reactive values.
Implementation details: `inspiration/papers/arxiv-2002.06184-scalaloci-impl.pdf`

### Links (Cooper, Lindley, Wadler, Yallop — FMCO '06)
**Source:** `inspiration/papers/links-fmco06.pdf`
**Repo:** `inspiration/repos/links`

Founding example. Single functional language for three tiers (server, client, DB), with location
annotations and type-directed call-site transformation. `docs/research/claude_comprehensive_research.md` line 144.

### Ur/Web (Chlipala — ICFP '15)
**Source:** `inspiration/papers/urweb-icfp15.pdf`
**Repo:** `inspiration/repos/urweb`
**Ecosystem:** `inspiration/repos/awesome-urweb`

Type-safe web programming with server-client tier split. Strongest static guarantees in the tierless
lineage. Ur/Web's type system proves tier-safety at compile time.

### Swift (Chong, Liu, Myers et al. — SOSP '07)
**Source:** `inspiration/papers/swift-sosp07.pdf`

Partitions a web application between client and server using Jif information-flow labels. Places
each operation where its label permits; inserts minimum replication for correctness.
Cautionary lesson: beautiful but requires the *entire program* to be labeled. The annotation burden
that Monolift explicitly refuses to impose.
`docs/research/claude_comprehensive_research.md` lines 49–50.

### Fabric (Liu, George, Vikram et al. — SOSP '09)
**Source:** `inspiration/papers/fabric-sosp09.pdf`

Extends Swift's discipline to distributed storage with explicit declassification. Full information-flow
types across the distributed system. Jif-class annotation cost.

---

## Monolift's position in the taxonomy

```
                    Placement Strategy
                    ┌──────────────┬──────────────┬──────────────┐
Communication       │  Explicit    │  Inferred    │  Mixed       │
Style               │  Annotation  │              │              │
────────────────────┼──────────────┼──────────────┼──────────────│
Remote Call         │  Monolift ✓  │  Coign       │  Ignis       │
                    │  Swift       │              │              │
────────────────────┼──────────────┼──────────────┼──────────────│
Reactive Signal     │  ScalaLoci   │              │              │
────────────────────┼──────────────┼──────────────┼──────────────│
Channel             │  Links       │              │              │
                    │  Ur/Web      │              │              │
└──────────────────────────────────────────────────────────────────
```

Monolift is unique in the "Explicit + Remote Call" cell for having **legacy-compatible** adoption
(existing Go codebase, no rewrite required) and **runtime-directed** placement (not fixed at deploy).
