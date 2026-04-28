# EntryPath Bridge Summary

Date: 2026-04-28

## Plain-Language Model

EntryPath is trying to answer this question: "If this region is lifted, what
external code can call into it?"

The exhaustive answer is to scan every function in the program for function
values and registrations. That works, but it is expensive. The bridge strategy
keeps the useful part of exhaustive mode while narrowing the scan:

1. Find code that already reaches the lifted region.
2. Treat that code as a set of touchpoints.
3. Look in nearby packages for functions that move those touchpoint-related
   function values.
4. Prefer owners that look like external boundary registrations.
5. Index those owners and run the existing function-value flow.

The result is a smaller, registration-oriented search. It is more expansive
than a pure call path, but much smaller than scanning every function.

## How a Lifted Region Becomes EntryPath Candidates

First, Monolift loads the target application and builds SSA. SSA turns the Go
program into a typed instruction graph so the compiler can inspect calls,
stores, returns, closures, and interface values without relying on source text.

Next, EntryPath builds callgraph evidence. Static calls are direct. RTA and VTA
help with calls that go through interfaces or function values by using type and
reachability information to estimate possible callees.

Then EntryPath runs reverse BFS from the lifted region roots. This finds
touchpoints: functions that can reach the region through the callgraph. A cheap
reverse-path mode stops around here, but that misses registrations where the
real entry function is passed around outside the direct call path.

Bridge mode continues from the touchpoints. It selects a bounded set of nearby
packages and owners, scans their SSA instructions, and admits owners that either
move relevant function values or have boundary evidence. Boundary evidence is
generic evidence that an owner participates in an external entry boundary, such
as accepting or storing a handler-shaped value.

Those admitted owners become seeds for the function-reference index. The index
records the facts the later flow engine needs: where function values came from,
where they were passed, where they were stored, and where they were returned.
The flow engine then classifies external surfaces, registration sites, and
wrapper chains.

## Roles of SSA, RTA, VTA, and Callgraph

SSA is the inspection format. It makes function values and typed operands
explicit enough to analyze mechanically.

The callgraph gives EntryPath a directional map between functions. Reverse BFS
uses that map to find region touchpoints.

RTA helps approximate reachable methods from instantiated types. VTA helps
resolve value-flow-through-types cases, especially interface and function-value
calls. Neither is perfect, but together they make the callgraph more useful
than direct static calls alone.

The function-reference index is separate from the callgraph. It is closer to an
IDE "find all references" pass for function values: where is this function
mentioned, passed, stored, wrapped, or returned?

Bridge mode combines both graphs. The callgraph finds touchpoints; the
function-reference index explains registration and wrapper movement around
those touchpoints.

## Cost Shape

The expensive shared phases are package loading, SSA construction, and
callgraph construction. Bridge mode does not solve those costs.

The improvement is in the later function-reference scan. Exhaustive mode indexes
about 140k owners in the Mattermost diagnostic. The successful bridge rows index
about 1.7k bridge owners and still recover the main known chain.

SPRINT-0032 also makes the budget easier to reason about:

- Bridge discovery has its own bridge duration and owner/package/instruction
  caps.
- Bridge indexing has its own function-index budget.

That split matters because the previous shared timer could admit the right
owners but leave no time to index them.

## When This Should Work

Bridge mode should work when the external entrypath is registration-shaped:
some owner near a region touchpoint passes, stores, returns, or wraps a function
value that eventually reaches an external boundary.

It should generalize beyond HTTP if the boundary predicate library can recognize
the relevant registration family. The core graph strategy does not depend on
route names, package names, or a specific framework.

## When It May Fail

Bridge mode may fail when the boundary family has no predicate yet, when the
relevant owner is outside the selected package/start budgets, or when the
function movement is too indirect for the current function-value flow.

It may also look less impressive on targets where package loading and callgraph
construction dominate total cost. In those cases the bridge scan can be more
efficient than exhaustive indexing while total wall time remains high.
