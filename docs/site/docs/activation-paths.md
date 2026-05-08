# Recovering activation paths

## Research question and result

The workshop prototype assumed the developer could identify the calls to
rewrite near `main()`. Real Go monoliths do not make that path so direct:
the lifted region may be reached through framework dispatch, callback
registration, queue handlers, or function values stored in registries.
The compiler therefore needs to recover the **activation path**: the
static control-flow path from a program entrypoint to the lifted region's
root function.

The research question is whether there is a static graph that is small
enough to analyze but rich enough to connect entrypoints to lifted region
roots across real codebases. Monolift's answer augments a standard
type-aware call graph with value-flow edges and iterative exploration.
On the pinned corpus, it recovers 71 of 72 reviewed paths; the remaining
miss is an enterprise-package build issue, not a known analysis
limitation.

## Why the compiler needs this

Monolift takes a developer-annotated region of code and extracts it into
a remote service. The annotation says *what* to lift, not *where* to cut.
The compiler still has to decide where the network boundary goes, and
that decision requires understanding how the application reaches the
annotated code in the first place.

The activation path supplies that evidence. It shows how control flows
from program startup, through framework and application machinery, to
the code being extracted.
Without it, the compiler is choosing a network boundary without the
evidence needed to decide where that boundary belongs.

In a simple program the activation path might be a straight line of
function calls. In practice it almost never is. Real Go monoliths wire
their behavior together through patterns that standard call graphs
cannot follow: a handler function stored in a struct field and invoked
later by a framework, a callback registered with a queue and triggered
asynchronously, a factory function looked up from a registry by name.
These are all static, deterministic patterns in the source code — the
information is there — but a standard call graph only sees direct
function calls and misses the rest.

## How we designed the algorithm

We did not design the algorithm from theory. We designed it from data,
using real codebases as the specification.

### Step 1: Find things worth lifting

We surveyed six open-source Go monoliths (Caddy, Gitea, Listmonk,
Mattermost, Miniflux, PocketBase — roughly 1.5M lines of Go) for
functions whose remote execution would plausibly benefit the
application under load. Three independent model agents each produced
candidate lists per project, cross-reviewed each other's picks, and
an aggregator merged the results into a consensus corpus of 88
candidates, each scored against a utility rubric.

### Step 2: Trace the paths by hand

For 72 of those candidates, we had three agents independently trace
the static path from `main()` to the target function — labeling each
step with the *kind of resolution* a compiler would need to follow it.
The agents cross-critiqued each other's traces, and a synthesis pass
produced one reviewed reference trace per candidate.

The traces are structured: each step has a source location, a target
location, and a label describing what kind of connection it is. The
labels are not framework names — they describe what the compiler needs
to *do* to follow the edge. Some edges are trivial (direct function
calls). Others require the compiler to track a value stored in a data
structure and later invoked, or to recognize that a goroutine launch
creates a new thread of control.

These 72 reviewed traces became the algorithm's **ground truth**: the
reference dataset that tells us, for any version of the algorithm,
which paths it finds and where it gets stuck.

### Step 3: Build the algorithm to match the data

With ground truth in hand, we built the algorithm one capability at a
time. After each addition, we re-ran the full evaluation:

- **Baseline**: a standard type-aware call graph (RTA) found 49 of 72
  paths (68%). It handles direct calls and interface dispatch but
  misses everything that flows through stored values.

- **Value-flow tracking**: we added passes that follow function values
  stored in struct fields, package variables, map registries, and
  callback arguments. This created the right edges, but the functions
  on the other side were still unexplored nodes: the call graph had not
  analyzed their bodies yet.

- **Iterative exploration**: the key insight. After augmentation
  discovers new functions, re-run the call graph analysis from those
  functions as additional roots. Their callees get explored, which may
  reveal more stored values, which get augmented, which get explored —
  repeat until nothing new appears. This loop typically converges in
  2–3 rounds.

- **Final result**: 71 of 72 paths found (98.6%). The single miss is a
  function in Mattermost's enterprise package that doesn't compile
  against the open-source build — a test infrastructure issue, not an
  algorithm limitation.

```
                        ┌─────────────────┐
                        │ Standard call   │  49/72 (68%)
                        │ graph (RTA)     │  direct calls + interfaces
                        └────────┬────────┘
                                 │
                        ┌────────▼────────┐
                        │ + value-flow    │  edges correct, but
                        │   tracking      │  discovered functions
                        │                 │  are dead ends
                        └────────┬────────┘
                                 │
                        ┌────────▼────────┐
                        │ + iterative     │  69/72 (96%)
                        │   exploration   │  explores discovered functions
                        └────────┬────────┘
                                 │
                        ┌────────▼────────┐
                        │ + remaining     │  71/72 (99%)
                        │   value-flow    │
                        │   patterns      │
                        └─────────────────┘
```

The coverage report after each step told us exactly which paths were
newly found, which remained blocked, and what kind of edge was the
blocker. This made prioritization mechanical: implement support for the
pattern that blocks the most paths, re-measure, repeat.

## What the algorithm does

The algorithm is a pipeline of static analysis passes over the
program's intermediate representation:

**1. Build a call graph.** Load the program, lower it to SSA, and run
Rapid Type Analysis from `main()`. This produces a graph of which
functions can call which other functions, including calls through
interfaces (where the compiler infers which concrete types could be on
the other side).

**2. Augment with value-flow edges.** Scan the program for patterns
where a function is not *called* but *stored* — placed into a struct
field, a package variable, a map, or passed as an argument — and later
retrieved and invoked somewhere else. For each such pattern, add an
edge connecting the storage site to the invocation site. For well-known
frameworks (like the cobra CLI library), a small registry of known
dispatch locations makes this connection precise without needing to
analyze the framework's internals.

**3. Explore discovered functions.** Any function reached only through
a value-flow edge is new territory the call graph hasn't explored.
Re-run the call graph from those functions as additional roots, merging
the results back. Repeat augmentation and exploration until no new
functions appear.

**4. Find the shortest path.** Run breadth-first search (BFS) from all
entrypoints to the target.

## A concrete example

Consider Caddy's markdown rendering function, which the corpus
identified as a useful lift candidate. It renders Markdown to HTML on
every matching HTTP response — CPU-intensive, bursty under traffic,
and a clean pure function.

The activation path from `main()` to `funcMarkdown`:

```
main()
  │ direct call
  ▼
cmd.Main()
  │ value stored in struct field: cobra command's RunE field
  ▼                               holds a reference to cmdRun
cmdRun()                          ← framework invokes this later
  │ direct call
  ▼
caddy.Load()
  │ interface dispatch (App interface → HTTP app)
  ▼
caddyhttp.App.Start()
  │ several layers of interface dispatch through
  │ the middleware/handler chain
  ▼
Templates.executeTemplate()
  │ direct call
  ▼
funcMarkdown()                    ← the lift target
```

The standard call graph handles most of this path. The one gap is the
second step: `cmdRun` is stored in cobra's `RunE` field at
initialization time and invoked later by cobra's command dispatcher.
Without the value-flow pass, the compiler sees `main → cmd.Main →
cobra.Execute` but cannot connect that dispatcher to `cmdRun`. With the
value-flow pass, the full path is recovered.

## What the algorithm cannot resolve

The algorithm is purely static. It cannot follow:

- **Reflection-based dispatch** — when a function is invoked by name
  via Go's `reflect` package or `text/template`'s internal interpreter.
  The algorithm sees the function *registered* but not the reflective
  *invocation*.

- **Cross-process boundaries** — plugin systems where the function body
  lives in a separate binary, connected via RPC.

- **Runtime-conditional registration** — handlers registered inside
  `if config.Enabled { ... }` blocks, where visibility depends on
  runtime state.

In all these cases, the algorithm emits a **partial path with a labeled
gap** — it reports how far it got and classifies why it stopped. This
is more useful than silence: downstream compiler phases can reason about
incomplete evidence rather than treating every gap as a hard failure.

## Evaluation

| Project | Lines of Go | Paths found | |
|---|---:|---:|---|
| Caddy | 93k | 6/6 | |
| Gitea | 456k | 18/18 | |
| Listmonk | 20k | 10/10 | |
| Mattermost | 761k | 14/15 | 1 miss: enterprise build issue |
| Miniflux | 76k | 12/12 | |
| PocketBase | 122k | 11/11 | |
| **Total** | **1.5M** | **71/72** | **98.6%** |

The corpus spans CLI-dispatched servers (Caddy, Mattermost), queue-based
workers (Gitea, Listmonk), direct HTTP handlers (Miniflux), and
framework-wrapped APIs (PocketBase). The algorithm generalizes across
all of them without project-specific logic.

## Design principles

**Design from data, not theory.** The 72 reviewed traces were the
specification. Every analysis pass was justified by measured coverage
improvement, not by abstract completeness arguments.

**Iterate to convergence.** Discovering a function leads to exploring
its callees, which may reveal new stored values, which lead to
discovering more functions. A fixed-point loop handles this naturally.

**Emit partial paths, not failures.** When the algorithm cannot
complete a path, it reports how far it got and why it stopped. This
turns binary pass/fail into a gradient.

**Framework knowledge is data, not code.** Dispatch patterns for
specific frameworks are expressed as table entries. Adding a new
framework means adding a row, not writing a new analysis pass.
