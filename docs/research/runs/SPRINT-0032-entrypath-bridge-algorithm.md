# SPRINT-0032 EntryPath Bridge Algorithm

Date: 2026-04-28

## Algorithm v1

The bridge algorithm starts from the lifted region and works outward until it
finds code that registered or passed a function value into an external entry
boundary. It is not a route-table recognizer. It is a generic static-analysis
pipeline that uses value-flow evidence to connect region touchpoints to
boundary-shaped owners.

1. Load packages and build SSA.
   - Input: the target main package and region root specs.
   - Output: SSA functions, instructions, types, and source positions.

2. Build RTA/VTA-backed callgraph evidence.
   - Input: SSA program.
   - Output: application callgraph with static and type-informed call edges.

3. Reverse BFS from the region roots.
   - Input: callgraph and region roots.
   - Output: touchpoint functions that already reach the lifted region.

4. Select bridge starts.
   - Input: reverse-BFS touchpoints.
   - Output: a bounded set of functions/packages worth scanning locally.
   - Budget controls: start count, package count, package-function count,
     instruction count, and bridge duration.

5. Scan local package owners.
   - Input: selected start packages.
   - Output: owner functions that may assign, pass, return, store, or wrap
     touchpoint-adjacent function values.

6. Admit generic boundary owners.
   - Input: scanned owners and type/value evidence.
   - Output: bridge seeds and boundary evidence. Today the strongest predicates
     are HTTP-shaped (`net/http` interfaces and `ServeHTTP`-style signatures),
     but the phase is intentionally isolated from the graph algorithm.

7. Prioritize bridge function-reference indexing.
   - Input: bridge seed set and bridge diagnostics.
   - Output: deterministic owner order:
     boundary bridge owners first, selected-package owners with direct
     touchpoint references second, remaining selected-package bridge owners
     third, and other bridge owners last.

8. Run function-value flow and classification.
   - Input: indexed function references.
   - Output: external surfaces, registration sites, wrapper chains, and oracle
     trace phase evidence.

## Data Flow

| Phase | Consumes | Produces |
|---|---|---|
| SSA build | Go packages | Typed SSA functions and instructions |
| Callgraph | SSA program | Caller/callee edges |
| Reverse BFS | Callgraph, region roots | Region touchpoints |
| Bridge start selection | Touchpoints | Selected packages and starts |
| Local owner scan | Selected packages | Candidate bridge owners |
| Boundary admission | Owners, SSA types, instructions | Boundary evidence and seed owners |
| Function-ref index | Seed owners, priority context | Function source/use references |
| Function-value flow | Function-ref index | Surfaces, registrations, wrapper chains |

## Generalizability

The durable idea is registration-based, not Mattermost-specific:

- Start from code that reaches the lifted region.
- Look nearby for owners that move function values.
- Give extra priority to owners with generic boundary evidence.
- Index only admitted owners, then let the existing function-value flow connect
  wrappers, arguments, stores, returns, and registrations.

For an HTTP service, boundary evidence often appears as handler interfaces or
handler-shaped functions. For a gRPC service, a similar boundary phase could be
implemented around generated service descriptors, registration methods, or
server interface implementations. The bridge pipeline should not need to know
which service framework is involved; only the boundary predicate library should
grow.

Current limits:

- Existing boundary predicates are strongest for HTTP-shaped code.
- Bridge start selection still scans a small number of selected packages, so it
  is bounded but not deeply semantic.
- Package load, SSA, and callgraph cost are still shared with every EntryPath
  mode and are not optimized by this algorithm.

## Glossary

- Touchpoint: a function discovered by reverse BFS that already reaches a
  lifted region root through the callgraph.
- Owner: the SSA function whose instructions contain a function-value movement
  or boundary-shaped operation.
- Boundary owner: an owner with generic evidence that it participates in an
  external entry boundary.
- Seed: an owner admitted into the function-reference index worklist.
- Function-ref index: a scoped scan that records where function values are
  created, passed, stored, returned, and used.
- Wrapper chain: a recovered chain of function-value movement from one owner to
  another.
- Oracle trace: a diagnostic spec describing the known target nodes and
  relationships so each phase can report where the chain is present or missing.
