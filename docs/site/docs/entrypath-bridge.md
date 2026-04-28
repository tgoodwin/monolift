# Finding entrypoints with a bridge search

## At a glance

When Monolift lifts a region of code, it needs to know what external
code can call into that region. The direct call graph answers part of
that question: it can show functions that call, or are called by, the
lifted region. Real Go services often hide the important edge somewhere
else, though. A handler might be passed into a router, wrapped by
middleware, stored in a registry, or attached to a service object before
traffic ever reaches it.

The **EntryPath bridge** is the compiler's current answer for that
case. It is a bounded search that starts from the lifted region, finds
nearby code that already reaches it, then looks for statically visible
registration evidence that connects that code to an external boundary.
It is more expansive than a pure call-path search, but much smaller than
scanning every function in the program.

The important qualifier is that this is not a universal entrypoint
solver. It is designed for **statically visible registration patterns**:
HTTP handlers registered with routers, methods attached to generated
service registries, callback tables, worker queues, command trees, and
similar structures. The current implementation has the strongest
boundary predicates for HTTP-shaped Go code, but the algorithm itself is
not tied to Mattermost or to any one router package.

## The problem: call paths miss registration paths

A lifted region has **region roots**: the functions the developer asked
Monolift to extract. From those roots, the compiler can walk backward
through the call graph to find functions that already reach the region.
Those functions are useful, so we call them **touchpoints**.

But a touchpoint is not always the public entrypoint. Consider a common
shape:

```mermaid
flowchart LR
    B["external boundary<br/>router / service registry / queue"] --> W["wrapper or registration owner"]
    W --> H["handler or callback"]
    H --> R["lifted region root"]
```

The call graph can often find `H -> R`. The harder question is how `H`
became reachable from the boundary. That edge might be a function
argument, a method value, a struct field, a table entry, or a wrapper
closure. The bridge search exists to recover that registration path
without falling back to an exhaustive whole-program reference scan.

## Terminology

**SSA** is the compiler's inspection format. Go source is lowered into
typed instructions, so Monolift can reason about calls, stores, returns,
closures, method values, and interface values mechanically.

**Call graph** means the graph of possible calls between functions. It
includes direct static calls and type-informed edges from RTA/VTA-style
analysis, which helps when calls go through interfaces or function
values.

**Reverse BFS** is a backwards graph walk from the lifted region roots.
It finds functions that can already reach the region.

**Touchpoint** means a function found by reverse BFS. It is not
necessarily an external entrypoint; it is a known point near the lifted
region.

**Owner** means the function whose SSA instructions contain the evidence
we care about. If a function stores a callback in a table, passes a
handler to a router, or returns a wrapper closure, that function is the
owner of that evidence.

**Boundary owner** means an owner with generic evidence that it is near
an external boundary. Today that evidence is strongest for HTTP-like
boundaries, such as `net/http` handler interfaces or `ServeHTTP`-shaped
values. The same role could be filled by gRPC service registration
evidence or queue-handler registration evidence.

**Function-reference index** means a scoped "find references" pass for
functions. It records where function values are created, passed, stored,
returned, or used. Function values are not the whole algorithm; they are
one important evidence channel for registration-shaped Go code.

**Bridge owner** means an owner admitted into the bounded bridge search.
It may be admitted because it sits in a selected touchpoint package,
because it directly references a touchpoint, or because it carries
boundary evidence.

## The algorithm

```mermaid
flowchart TD
    A["load packages + build SSA"] --> B["build call graph<br/>static + type-informed edges"]
    B --> C["reverse BFS from region roots"]
    C --> D["touchpoints"]
    D --> E["select bounded bridge starts<br/>near touchpoint packages"]
    E --> F["scan local owners<br/>for registration evidence"]
    F --> G["admit bridge + boundary owners"]
    G --> H["prioritized function-reference index"]
    H --> I["function-value flow<br/>and entrypath classification"]
    I --> J["external surfaces<br/>registration sites<br/>wrapper chains"]
```

The phases are deliberately separated so their costs and failure modes
are understandable.

1. **Load packages and build SSA.** This is the shared setup cost for
   static analysis. The bridge algorithm does not make this part cheap;
   it depends on it.

2. **Build the call graph.** The call graph gives the compiler a
   directional map between functions. It is enough to find code near the
   lifted region, but not enough to explain every registration edge.

3. **Run reverse BFS from the region roots.** This produces touchpoints:
   functions that already reach the lifted region.

4. **Select bridge starts.** The compiler chooses a bounded set of
   functions and packages near those touchpoints. This is the first place
   the algorithm chooses not to be exhaustive.

5. **Scan local owners.** Within the selected budget, the compiler scans
   SSA instructions for evidence that an owner moves executable behavior
   toward a boundary. That evidence can include function arguments,
   method values, stores, returns, closures, and boundary-shaped types.

6. **Admit bridge and boundary owners.** Owners with useful evidence
   become seeds for the next phase. Owners with boundary evidence get
   special priority because they are more likely to explain how external
   traffic enters the lifted region.

7. **Build a prioritized function-reference index.** The index runs over
   admitted owners, not the entire program. Owners are ordered so boundary
   owners are scanned first, then owners with direct touchpoint
   references, then the rest of the selected-package owners.

8. **Classify entrypath evidence.** The existing function-value flow
   uses the index to recover external surfaces, registration sites, and
   wrapper chains.

## Why this is a bridge

The bridge connects two views of the program:

- the **call graph view**, which is good at finding code that reaches a
  lifted region; and
- the **reference/registration view**, which is good at explaining how a
  function or method became attached to an external boundary.

Neither view is enough alone. A pure call path can stop too close to the
region and miss the registration site. A pure reference scan can recover
the path but may be too expensive on a large monolith. The bridge uses
the call graph to choose where to look, then uses reference evidence to
explain the registration path.

## What makes it generalizable, and what does not

The general part is the shape of the search:

1. Start from region roots.
2. Find nearby touchpoints with the call graph.
3. Look for registration evidence near those touchpoints.
4. Prioritize owners that look like external boundaries.
5. Index only the admitted owners.

The current implementation's strongest evidence channel is
function-value movement because Go registrations often pass functions,
methods, or closures into framework code. That should not be read as
"this only works for one Mattermost function." It should be read as
"this works best when the entrypath is statically visible as executable
behavior being registered somewhere."

For a gRPC service, the boundary evidence might be generated
`RegisterXServer` calls, service descriptors, or interface
implementations. For a queue system, it might be handler registration in
a job table. For a CLI command tree, it might be command structs with
callable run hooks. Those would require more boundary predicates, but
the bridge pipeline would stay the same.

The approach may fail when:

- the entrypoint is created mostly through reflection or strings;
- generated code hides the useful registration evidence;
- dependency injection makes the static owner unclear;
- the relevant owner is outside the bridge budgets; or
- the boundary family has no predicate yet.

## Budget model

Sprint 32 clarified the cost envelope. Bridge discovery and bridge
indexing now have separate budgets:

- bridge discovery is bounded by package, owner, instruction, start, and
  duration limits; and
- function-reference indexing has its own phase-local budget over the
  admitted bridge owners.

That split matters. Before the cleanup, bridge discovery could consume
the function-index budget before indexing began. The compiler could find
the right owners, then skip all of them at the indexing phase. With
phase-local indexing, the budget now describes what it says: time spent
indexing admitted bridge owners.

## Current status

On the Mattermost diagnostic chain, the consolidated bridge search
recovered the target path while indexing roughly 1.7k bridge owners
instead of doing the much larger exhaustive function-reference scan. The
result is promising enough to keep as the current EntryPath bridge
strategy, but it should not be treated as finished general entrypoint
recovery.

The next useful validation step is not to make the Mattermost case more
clever. It is to test the same pipeline on a non-Mattermost registration
shape, such as a gRPC-style service registration or a typed job-handler
registry, and add only the boundary predicates needed for that family.
