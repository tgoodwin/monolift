# SPRINT-0029 Bridge Algorithm

Date: 2026-04-28

## Intent

The bridge seed source is a diagnostic EntryPath mode that starts from
reverse-BFS touchpoints and looks locally for the owners that make those
touchpoints useful to function-value flow. It does not use oracle-provided start
names, service-specific route names, package-name rules, or report/emission
wiring.

The problem it addresses is the SPRINT-0028 loss pattern: a region-adjacent
function can be found by reverse BFS, but the owners that pass, wrap, register,
or accept that function value are not necessarily reverse-reachable callgraph
owners. The bridge adds those local owners as explicit function-ref index seeds.

## Inputs

- SSA program and application callgraph.
- Region roots.
- Reverse-BFS touchpoints already produced by EntryPath.
- Existing boundary predicates, currently `net/http`.
- Bridge budgets:
  - maximum selected starts;
  - maximum packages scanned;
  - maximum functions scanned per package;
  - maximum bridge owners;
  - maximum boundary owners;
  - maximum scanned instructions;
  - maximum elapsed duration.

## Plain-Language Algorithm

1. Run the existing callgraph and reverse-BFS phases.
2. Treat reverse-BFS touchpoints as bridge-start candidates.
3. Select stable, named function starts using generic metadata only:
   - the touchpoint must resolve to an SSA function;
   - it must have a package and object identity;
   - duplicate touchpoints collapse to one start;
   - starts are sorted deterministically by package/object identity;
   - the start budget skips the remaining candidates.
4. Group selected starts by package. Each package is scanned at most once.
5. For each selected start, seed the start itself so it can become a
   function-value source.
6. For each scanned local owner in a selected start package:
   - scan instructions for direct references to any selected start in that
     package;
   - add owners that pass, store, return, wrap, capture, register, or directly
     invoke the start value;
   - when a start is passed to a static callee, add that callee as a bridge
     owner too, because it may be the typed boundary owner;
   - run boundary predicates on bounded candidate owners and add matching
     boundary owners/evidence to the same seed set.
7. Build the normal function-ref index over the bridge seed set.
8. Run the existing function-value flow over that index.
9. Report bridge-specific counts, budget stops, and diagnostics without changing
   default EntryPath behavior.

## Pseudocode

```text
touchpoints = reverse_bfs(region_roots)
starts = select_bridge_starts(touchpoints, budgets.max_starts)
seeds = new_seed_set()

for start in starts:
    seeds.add(start, "bridge")

for package in packages_for(starts):
    if package_budget_exceeded:
        stop("package_budget")
        break

    local_starts = starts_in_package(package)
    scanned = 0

    for owner in sorted_functions(package):
        if duration_budget_exceeded:
            stop("duration_budget")
            break
        if package_function_budget_exceeded(scanned):
            stop("package_function_budget")
            break
        if instruction_budget_exceeded:
            stop("instruction_budget")
            break

        scanned += 1

        evidence = boundary_predicates.match(owner)
        if evidence:
            add_boundary_owner(owner, evidence)
            seeds.add(owner, "bridge")

        refs = refs_for_owner(owner)
        for ref in refs:
            if ref.operand in local_starts:
                seeds.add(owner, "bridge")

                if ref is call_arg and ref.call has static_callee:
                    callee = ref.call.static_callee
                    seeds.add(callee, "bridge")
                    if boundary_predicates.match(callee):
                        add_boundary_owner(callee, evidence)

        enforce_owner_budget()
        enforce_boundary_owner_budget()

index = build_function_ref_index(seeds)
flow = analyze_function_value_flow(index, region_roots)
```

## Seed Assembly

The bridge seed set contains:

- selected bridge starts;
- local owners that reference selected starts;
- static callees that receive selected starts as arguments;
- owners with boundary predicate evidence discovered while scanning the local
  owner set.

Owners can have multiple reasons. A boundary owner discovered by the bridge is
seeded with both bridge and boundary/http-sink reasons when applicable. This
keeps bridge statistics separate while preserving existing function-ref and
boundary evidence behavior.

## Implemented Behavior

The SPRINT-0029 implementation adds an explicit `bridge` function-index mode.
Default EntryPath behavior and the existing `oracle-bridge` mode are unchanged
unless this mode is selected.

Implementation details:

- Reverse BFS now preserves internal SSA functions for each emitted touchpoint.
- Bridge start selection uses those reverse-BFS touchpoint functions directly;
  it does not read `OracleSpec.BridgeStarts`.
- Starts with missing package/object identity are skipped, duplicate starts are
  collapsed, and selected starts are sorted by receiver rank, package path,
  object name, and full function string.
- Selected starts are grouped by package. The implementation scans selected
  packages in deterministic package-name order.
- Each selected package is scanned at most once for:
  - boundary predicate evidence;
  - refs whose operand is a selected start in that package;
  - static callees that receive a selected start as a call/go argument.
- Bridge owners are added with the explicit `bridge` seed reason.
- Boundary owners discovered during bridge scanning also keep the existing
  boundary/http-sink seed reasons.
- Bridge discovery reports touchpoints, start candidates, selected/skipped
  starts, skip reasons, scanned packages, scanned package functions, scanned
  instructions, bridge owners, boundary candidates, boundary owners, seed
  owners, indexed bridge owners, duplicate suppressions, budget stops, and stop
  reasons.

Validation showed an ordering-sensitive miss: the run selected and indexed
`connectWebSocket`, but package/member discovery stopped on `instruction_budget`
and `boundary_owner_budget` before admitting `APIHandlerTrustRequester` or
`InitWebSocket`. The next diagnostic should record per-start package scheduling
and whether each selected start package was scanned before budgets stopped.

## Function-Ref Indexing

The bridge does not introduce a new flow engine. After seeds are assembled,
EntryPath calls the existing seeded function-ref index builder. The normal
value-flow pass then recovers source-to-registration relationships through
existing reference kinds: operands, call arguments, stores, returns, closure
captures, direct invokes, and supported passthrough values.

## Why The Algorithm Is Generic

The bridge starts from graph facts and typed SSA facts, not framework names.
HTTP works today because the repository already has a `net/http` boundary
predicate. The same owner-discovery shape applies to:

- HTTP registration, where a function reaches an `http.Handler` sink;
- gRPC registration, once a predicate recognizes generated service/server
  boundary shapes;
- callback registration, where a function value is passed, stored, wrapped, or
  captured before invocation;
- other typed boundary sinks, as long as a predicate can identify the owner or
  call signature that makes the sink externally reachable.

The bridge logic does not need to know the protocol. It only needs starts from
reverse BFS, local SSA references to those starts, and boundary predicates that
can explain typed sinks.

## Stop Conditions And Diagnostics

The bridge stops or partially stops on independent budgets:

- start budget: candidates remain after the selected-start cap;
- package budget: selected starts span more packages than allowed;
- package-function budget: a package has more local functions than allowed;
- owner budget: too many bridge owners would be added;
- boundary-owner budget: too many boundary owners would be predicate-scanned or
  admitted;
- instruction budget: local scanning exceeds the instruction cap;
- duration budget: elapsed bridge discovery time is exhausted;
- duplicate suppression: a candidate owner was already admitted and is counted
  as suppressed rather than added again.

Each budget stop is reported with a budget name and stop reason. The final
function-ref index budget remains separate and is reported as an index stop.

## Expected Failure Modes

- Missing refs: SSA reference extraction does not expose the relevant
  function-value use.
- Too-broad owner discovery: many touchpoint packages consume budgets before the
  needed owner package is scanned.
- Predicate rejection: the local owner is scanned, but no boundary predicate
  recognizes the sink type.
- Ordering issues: deterministic start/package ordering reaches budget before a
  useful start.
- Duplicate seeds: multiple touchpoints or local owners collapse to the same
  seed, which is expected but can hide weak signal if counts are not reported.
- Budget exhaustion: start, package, owner, boundary-owner, instruction,
  duration, or final index budgets stop before recovery.
- False-positive bridge owners: local references to a touchpoint can add owners
  that are not true externally reachable registration sites.
