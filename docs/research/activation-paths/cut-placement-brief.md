# Cut placement: where the network boundary goes

## The problem

The activation-path algorithm (SPRINT-0035 through SPRINT-0038) answers
*how does the application reach the code I want to lift?* at 98.6%
coverage. But knowing the path doesn't tell you where to cut it. Every
node along the path is a candidate cut point — everything above stays in
the monolith, everything at and below moves to the remote service. The
compiler needs to choose.

This is a multi-objective optimization problem. There is no single best
cut; there is a Pareto frontier where improving one dimension worsens
another. The sprint should define the dimensions, build a scoring model,
and validate it against the 72-trace corpus.

## The tradeoff space

Six dimensions interact. Each candidate cut point in an activation path
gets a score along each.

### 1. Extraction surface area

How much code moves to the remote side.

A shallow cut (near `main`) extracts a large subtree: framework
dispatch, middleware chains, and the target function. A deep cut (near
the region root) extracts just the target and its direct dependencies.

**Why it matters:** In a serverless deployment model, extraction surface
directly affects cold-start time, container image size, memory footprint,
and attack surface. A cut that drags 200k LOC of framework code into a
Lambda defeats the purpose of lifting.

**Measurable:** Count of transitive callees below the cut, or sum of
SSA instruction count in those callees. The activation path already has
this information — the call graph below any node is known.

### 2. Boundary data complexity

What types cross the network at the cut point.

The function at the cut point has parameters (the data flowing in) and
return values (the data flowing out). These must be serializable. Some
types serialize trivially (primitives, structs with exported fields),
some require work (interfaces need concrete-type registries), and some
are effectively impossible to serialize (channels, `io.Writer`,
`*os.File`, mutexes, function values).

**Why it matters:** This is the hard constraint. A cut where
`http.ResponseWriter` must cross the wire is infeasible without a
streaming proxy — the writer is a live connection to the client. A cut
where `context.Context` crosses is feasible but requires careful
deadline/cancellation propagation. A cut where `([]byte, error)` crosses
is trivial.

**Measurable:** Classify each parameter and return type of the cut-point
function:
- **Trivial:** primitives, strings, byte slices, structs of trivials
- **Serializable:** structs with exported fields, slices/maps of serializables
- **Reconstructible:** types that can be rebuilt on the remote side
  from config (DB connections, HTTP clients, loggers)
- **Proxy-required:** streaming types (io.Reader/Writer, channels,
  http.ResponseWriter) that need a streaming proxy protocol
- **Infeasible:** function values, mutexes, runtime-internal types

The worst parameter type determines the boundary complexity class.

### 3. State reconstruction cost

What state needs to exist on the remote side for the extracted code to
run.

The extracted function may close over or receive through its receiver:
database connections, configuration objects, caches, connection pools.
These cannot cross the wire as serialized values — they must be
reconstructed on the remote side. The state-class analysis (ADR-0005,
ADR-0016, ADR-0022) already classifies these patterns.

**Categories by cost:**
- **Stateless:** No reconstruction needed. The function takes all inputs
  as arguments and returns all outputs. (e.g., `correctPassword(hash,
  plaintext) bool`)
- **Config-only:** Remote side needs read-only configuration. Inject at
  startup, no runtime coordination. (e.g., template rendering with a
  config-provided template set)
- **Client-reconstructible:** Remote side needs its own client to an
  external service (DB, cache, queue). Requires connection management
  but no state synchronization with the monolith. (e.g., `RefreshFeed`
  with its own DB pool)
- **Shared-state:** Remote side needs access to state that the monolith
  also reads/writes. Requires distributed coordination (distributed
  lock, event sourcing, or accepting eventual consistency). (e.g.,
  in-memory session cache)

The deeper the cut, the more likely the function is stateless or
config-only. The shallower the cut, the more state gets dragged across.

### 4. Callback frequency (fan-in to monolith)

Does the extracted code need to call back into the monolith?

If the code below the cut calls functions that remain above the cut,
each such call becomes either: (a) a reverse network call back to the
monolith (adding latency), or (b) additional code that must also be
extracted (expanding the surface area).

**Why it matters:** A cut that produces N callbacks per request
multiplies latency by N and couples the remote service back to the
monolith's internal API. Zero-callback cuts are strongly preferred.

**Measurable:** For a candidate cut at node C, count the edges from
nodes below C to nodes above C in the call graph. These are the
"callbacks" that would cross the boundary in reverse.

### 5. Error semantics preservation

Does introducing a network failure at the cut point violate the
original error contract?

A function call that was infallible becomes fallible at a network
boundary. If the original caller doesn't handle errors (common in
Go's `must`-style patterns, or void functions), introducing network
failure requires changing the caller's error handling — which may
propagate up the activation path.

**Related liftability properties:** `contract.error-last`,
`boundary.no-streaming-values`. These already exist in the property
taxonomy.

**Measurable:** Does the function at the cut point return an error?
Does the caller handle errors? If not, what's the distance to the
nearest error-handling ancestor in the path?

### 6. Edge-type alignment (natural boundary signal)

Does the cut land at a point where the program already abstracts over
behavior?

The activation-path edge taxonomy is itself a signal. Some edge types
represent places where the application is already treating the
downstream code as a replaceable unit:

- **Strong natural boundary:** `http-handler-registration`,
  `interface-method-dispatch`, `function-value-as-argument` (callback
  registration), `channel-send-receive` (queue worker pattern)
- **Weak natural boundary:** `function-value-in-struct-field` (framework
  dispatch — the framework already treats this as pluggable)
- **Anti-boundary:** `direct-function-call`, `method-call-on-concrete-type`,
  `closure-capture` (tight coupling, not designed for replacement)

Cutting at a strong natural boundary means the program's existing
abstraction layer does most of the work. The framework already treats
the code below the cut as a unit that could be swapped or relocated.

## Concrete examples from the corpus

### Caddy M-3: `correctPassword` (path length 11)

```
main → Main → [struct-field] → cmdRun → [interface] → App.Start
→ [callback] → Server.ServeHTTP → serveHTTP → [struct-field]
→ enforcementHandler → [closure] → wrapRoute → [closure]
→ wrapMiddleware → [interface] → Authentication.ServeHTTP
→ [interface] → HTTPBasicAuth.Authenticate → correctPassword
```

Candidate cuts and their profiles:

| Cut at | Surface | Boundary data | State | Callbacks | Edge signal |
|--------|---------|---------------|-------|-----------|-------------|
| `Server.ServeHTTP` (step 4) | Large (entire HTTP stack) | `http.Request` + `ResponseWriter` → proxy-required | Full app state | 0 | Strong (HTTP handler) |
| `Authentication.ServeHTTP` (step 9) | Medium (auth chain) | `http.ResponseWriter` + `*Request` → proxy-required | Auth providers | 0 | Strong (interface dispatch) |
| `HTTPBasicAuth.Authenticate` (step 10) | Small (auth logic) | `*Request` → serializable, returns `(User, bool, error)` | Hash comparator | 0 | Strong (interface dispatch) |
| `correctPassword` (step 11) | Minimal | `(hash, plaintext []byte)` → trivial | Stateless | 0 | Weak (concrete method call) |

**Observation:** The deepest cuts (steps 10-11) dominate on surface area,
boundary data, and state cost, with no penalty on callbacks. Step 11 is
nearly a pure function. The tension here is minimal — deep cuts win on
every dimension. But the edge signal at step 11 is weak (concrete method
call), meaning the program doesn't already treat `correctPassword` as a
replaceable unit. Step 10 (interface dispatch) may be a better
engineering choice because the interface already defines the replacement
contract.

### Gitea M-1: `Deliver` (webhook delivery, path length 13)

```
main → RunMainApp → [struct-field] → runWeb → serveInstalled
→ InitWebInstalled → [func-arg] → webhook.Init
→ [func-arg-stored-field] → queue registration → [goroutine]
→ RunWithCancel → [interface] → WorkerPoolQueue.Run → doRun
→ [goroutine] → worker closure → [struct-field] → safeHandler
→ [closure-capture] → handler → Deliver
```

| Cut at | Surface | Boundary data | State | Callbacks | Edge signal |
|--------|---------|---------------|-------|-----------|-------------|
| `webhook.Init` (step 5) | Very large | Init args → complex | Full webhook subsystem | Many | Weak |
| `WorkerPoolQueue.Run` (step 8) | Large | Queue interface → proxy-required | Queue + DB | Moderate | Strong (interface) |
| `handler` (step 12) | Small | `(items []int64)` → trivial | DB connection | Low | Moderate (closure) |
| `Deliver` (step 13) | Minimal | `(ctx, *HookTask)` → serializable | DB + HTTP client | 0 | Weak (direct call) |

**Observation:** The queue boundary (step 8) is the strongest *natural*
boundary — the program already treats the worker as an independent unit
dispatched through an interface with serializable work items. But it
extracts the entire worker pool mechanism. Step 13 (`Deliver`) has the
cleanest boundary data but requires the remote side to reconstruct DB
and HTTP client connections. The interesting middle ground is step 12
(`handler`), which inherits the queue's natural boundary while keeping
extraction surface small.

### Miniflux M-1: `RefreshFeed` (path length 5)

```
main → Parse → refreshFeeds → [goroutine] → worker closure
→ [channel] → job dispatch → RefreshFeed
```

| Cut at | Surface | Boundary data | State | Callbacks | Edge signal |
|--------|---------|---------------|-------|-----------|-------------|
| `refreshFeeds` (step 2) | Large | CLI args → trivial | Full app context | 0 | Weak (direct call) |
| goroutine body (step 3) | Medium | Worker ID → trivial | App + channel | Channel proxy needed | Anti (goroutine) |
| `RefreshFeed` (step 5) | Small | `(ctx, feedID)` → trivial | DB + HTTP | 0 | Weak (direct call) |

**Observation:** The goroutine boundary (step 3) is an anti-boundary —
splitting a goroutine across a network is architecturally wrong. The
channel between steps 3-4 is already a work-dispatch mechanism, which
suggests the cut should land *after* the channel receive, not before.
`RefreshFeed` is the clear winner: trivial boundary types, zero
callbacks, small surface. The tradeoff is that the remote side needs
its own DB and HTTP connections — but `RefreshFeed` is exactly the kind
of work that benefits from independent scaling, so dedicated connections
are a feature, not a cost.

## Key observations from the examples

1. **Deep cuts often dominate.** For compute-heavy leaf functions
   (password hashing, feed parsing, markdown rendering), the deepest
   cut wins on almost every dimension. The "interesting" multi-objective
   tradeoff only emerges for functions embedded in stateful middleware
   chains or framework dispatch loops.

2. **Edge type is the strongest natural-boundary signal.** Interface
   dispatch and callback registration edges mark points where the
   program already expects substitutability. These are not always the
   deepest cuts, but they are the easiest to implement correctly.

3. **The "reconstructible state" question is the real differentiator.**
   For most corpus traces, the boundary data at deep cuts is simple
   (primitives, small structs). What distinguishes easy from hard is
   whether the remote side needs to reconstruct application state (DB
   pools, caches, framework contexts).

4. **Anti-boundaries are real.** Goroutine launches and closure captures
   are terrible cut points because the abstraction on either side wasn't
   designed for separation. The algorithm should penalize these heavily.

5. **Callbacks are the multiplier.** A cut with zero callbacks is
   fundamentally different from a cut with even one callback. The
   first is a clean extraction; the second is a distributed system.

## Open questions for the sprint

1. **Weighting.** How should the six dimensions be weighted? Should this
   be configurable by the developer (who knows their deployment model),
   or should the compiler choose a single Pareto-optimal point?

2. **Composite cuts.** Can the compiler suggest cutting at multiple
   points simultaneously — e.g., extracting both `Authenticate` and its
   callee `correctPassword` as a unit, with the cut between
   `Authentication.ServeHTTP` and `Authenticate`?

3. **Feasibility gates vs. scoring.** Some dimensions are hard
   constraints (infeasible boundary types), while others are soft
   preferences (smaller surface area). The model needs to distinguish
   gates (binary reject) from scores (ranked preference).

4. **Corpus validation.** For each of the 72 traces, what cut point
   would a human choose? Collecting this ground truth would let us
   measure the algorithm's quality the same way we measured path-finding
   quality.

5. **Integration with liftability analysis.** The liftability property
   taxonomy already answers some of these questions for a given function.
   Should cut-placement consume liftability facts, or is it a separate
   phase that feeds into liftability?

6. **Path-local vs. graph-global.** Should the algorithm consider each
   activation path independently, or should it reason about all paths to
   a target simultaneously? (A target reachable through 3 paths might
   have a different optimal cut for each.)

## Relationship to existing work

- **Activation paths (SPRINT-0035–0038):** Provide the path and edge
  taxonomy that cut placement operates over. No changes needed to the
  path-finding algorithm.

- **Liftability properties (ADR-0018):** Boundary and effect properties
  partially answer dimensions 2, 3, and 5. Cut placement should consume
  these facts where available.

- **State-class inference (ADR-0016, ADR-0022):** Archetype classification
  (serialized-actor, keyed-partitioned-state, etc.) directly informs
  dimension 3.

- **Bridge/entrypath infrastructure (`pkg/compiler/entrypath/`):** The
  older bridge algorithm already identifies boundary evidence (HTTP
  handler types, boundary predicates) and touchpoint references. Some of
  this machinery is relevant to dimension 6 (natural boundary signals).

- **Canonical shapes (ADR-0006):** Transport selection is downstream of
  cut placement. The cut determines the boundary function; canonical
  shapes determine how to transport its calls.
