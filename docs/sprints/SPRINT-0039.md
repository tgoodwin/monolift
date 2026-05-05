# SPRINT-0039: Cut-point analysis across the 72-trace corpus

**Status:** done
**Predecessor:** SPRINT-0035 through SPRINT-0038 (activation-path algorithm, 71/72 reachable at 98.6%)

## Intent

The activation-path algorithm answers *how does the application reach the code I want to lift?* This sprint answers the next question: *where on that path should the network boundary go?* Every node in a trace is a candidate cut point. Three model agents independently analyze all 72 traces, enumerate candidate cuts, score them on six dimensions, and produce per-trace analyses plus a cross-corpus synthesis. The multi-agent design surfaces scoring disagreements as data rather than hiding them behind single-agent consensus.

## Scoring dimensions (reference)

Each candidate cut is scored on:

| # | Dimension | Scale |
|---|-----------|-------|
| 1 | Extraction surface area | Minimal / Small / Medium / Large / Very-large |
| 2 | Boundary data complexity | Trivial / Serializable / Reconstructible / Proxy-required / Infeasible |
| 3 | State reconstruction cost | Stateless / Config-only / Client-reconstructible / Shared-state |
| 4 | Callback frequency | 0 / Low / Moderate / Many |
| 5 | Error semantics preservation | OK / Needs-wrapper / Infeasible |
| 6 | Edge-type alignment | Strong / Weak / Anti |

Edge-type classification rules:
- **Strong:** `interface-method-dispatch`, `http-handler-registration`, `callback-registration`, `channel-send-receive`
- **Weak:** `function-value-in-struct-field`, `struct-literal-field-assignment`, `function-value-as-argument`
- **Anti:** `direct-function-call`, `method-call-on-concrete-type`, `closure-capture`, `goroutine-launch`

Type-classification heuristics for boundary data:
- Primitives, strings, byte slices, structs of exported trivials → **Trivial**
- Structs with exported fields, slices/maps of serializables → **Serializable**
- DB clients, HTTP clients, loggers, config objects (rebuildable from config on remote side) → **Reconstructible**
- `io.Reader`/`Writer`, `http.ResponseWriter`, channels → **Proxy-required**
- Function values, mutexes, runtime-internal types → **Infeasible**

For `context.Context`: classify as **Serializable** (deadline + values can be propagated; cancellation requires a proxy channel but is standard practice).

For callbacks: when uncertain whether code below the cut calls back above, default to conservative (assume callbacks exist). Distinguish "0 (confirmed)" vs. "0 (estimated)" in output.

## Scope boundaries

**In scope:**
- Reading and analyzing all 72 trace JSON files at `docs/research/activation-paths/traces/`
- Inspecting source code in pinned evaluation codebases at `evaluation/` to determine function signatures, parameter types, return types, receiver state
- Per-trace analysis files with candidate-cut scoring tables
- Per-project summaries highlighting architecture-specific patterns
- Cross-corpus synthesis with named archetypes, anti-boundary catalog, and dimension statistics
- Answering the brief's open questions where corpus evidence supports it

**Out of scope:**
- Any changes to `pkg/`, `cmd/`, or `evaluation/`
- Automated scoring tooling or compiler integration
- Transport selection or canonical-shape mapping
- mattermost/M-4 (target-not-found; document as a gap, do not attempt analysis)

## Per-trace analysis format

Each analysis file goes to `docs/research/activation-paths/analyses/<trace-id>.md`. Consistent format across all files so synthesis can aggregate.

Required sections:
1. **Header:** trace ID, region root function, path length, project, source file
2. **Candidate cut-point table:** one row per path node (skip step 0 / `main`), all six scoring columns, plus a composite feasibility class (Feasible / Feasible-with-proxy / Infeasible)
3. **Recommended cut:** the preferred node(s), one-paragraph rationale citing the decisive dimensions
4. **Tension notes:** dimensions that conflict and how the recommendation resolves the tradeoff
5. **Observations:** anything noteworthy for synthesis (unusual types, callback patterns, composite-cut opportunities, multi-path implications)

Volume management: steps 0-1 (entrypoint and framework bootstrap) are universally poor cuts — score them in one line. Focus analytical depth on the 2-4 genuinely competitive cut points per trace.

## Task list

### Phase 0: Calibration

Reproduce the brief's three worked examples to validate scoring methodology before scaling to the full corpus.

- [x] Read `caddy-M-3.json` and inspect source at all 11 path nodes in `evaluation/caddy/`. Score every candidate cut. Compare against the brief's table (steps 4, 9, 10, 11). Document any disagreements and why.
- [x] Read `gitea-M-1.json` and inspect source at all 13 path nodes in `evaluation/gitea/`. Score every candidate cut. Compare against the brief's table (steps 5, 8, 12, 13).
- [x] Read `miniflux-M-1.json` and inspect source at all 5 path nodes in `evaluation/miniflux/`. Score every candidate cut. Compare against the brief's table (steps 2, 3, 5).
- [x] Write a calibration summary: confirm methodology matches the brief, record decision rules for ambiguous cases (how to score `context.Context`, how to count callbacks without full call-graph data, how to estimate surface area without exact callee counts, how to distinguish `Reconstructible` from `Shared-state` for receiver types). This summary is the rubric for all subsequent analysis.

### Phase 1: Per-trace analysis — Caddy (6 traces, 93k LOC)

- [x] Analyze caddy-M-1 through caddy-M-7 (6 traces): enumerate cuts, score 6 dimensions, recommend cut, write `analyses/caddy-M-<N>.md`
- [x] Write `analyses/caddy-summary.md`: Caddy's middleware-chain architecture means most traces share a `Server.ServeHTTP` → `serveHTTP` → handler-chain prefix. Document how this shared prefix affects cut placement. Do the `interface-method-dispatch` edges in the middleware chain reliably signal strong boundaries, or do they all carry `http.ResponseWriter` (proxy-required) through?

### Phase 2: Per-trace analysis — Gitea (18 traces, 456k LOC)

- [x] Analyze gitea-M-1 through gitea-M-19 (18 traces): enumerate cuts, score 6 dimensions, recommend cut, write `analyses/gitea-M-<N>.md`
- [x] Write `analyses/gitea-summary.md`: Focus on queue-worker patterns (M-1 archetype — how many traces share it?), HTTP handler registration chains, callback-argument-stored-in-field patterns, and whether 456k LOC means shallow cuts always score Very-large on surface area.

### Phase 3: Per-trace analysis — Listmonk (10 traces, 20k LOC)

- [x] Analyze listmonk-M-1 through listmonk-M-10 (10 traces): enumerate cuts, score 6 dimensions, recommend cut, write `analyses/listmonk-M-<N>.md`
- [x] Write `analyses/listmonk-summary.md`: Listmonk is the smallest codebase. Characterize whether surface-area pressure is uniformly low, whether that eliminates surface area as a discriminating dimension, and what becomes the dominant tradeoff axis.

### Phase 4: Per-trace analysis — Mattermost (15 traces, 761k LOC)

- [x] Analyze mattermost-M-1 through mattermost-M-15, skipping M-4 (14 traces): enumerate cuts, score 6 dimensions, recommend cut, write `analyses/mattermost-M-<N>.md`
- [x] Document mattermost-M-4 as a gap: trace target is not found in the call graph. Note what cut analysis would require if the target were reachable.
- [x] Write `analyses/mattermost-summary.md`: Focus on shared-state prevalence (Mattermost's `App` struct carries extensive server state), callback frequency in plugin/hook chains, whether the enterprise-package boundary creates natural cuts, and whether any trace has zero feasible cuts.

### Phase 5: Per-trace analysis — Miniflux (12 traces, 76k LOC)

- [x] Analyze miniflux-M-1 through miniflux-M-14 (12 traces): enumerate cuts, score 6 dimensions, recommend cut, write `analyses/miniflux-M-<N>.md`
- [x] Write `analyses/miniflux-summary.md`: Characterize the goroutine + channel-send-receive pattern from M-1 and how many traces share it. Confirm or refute: goroutine launch is always an anti-boundary in Miniflux.

### Phase 6: Per-trace analysis — PocketBase (11 traces, 122k LOC)

- [x] Analyze pocketbase-M-1 through pocketbase-M-11 (11 traces): enumerate cuts, score 6 dimensions, recommend cut, write `analyses/pocketbase-M-<N>.md`
- [x] Write `analyses/pocketbase-summary.md`: PocketBase uses a hook/event-driven architecture. Do hooks create natural boundaries (the hook interface defines a contract), or do they force callbacks because the handler needs to call back into the app for data access?

### Phase 7: Cross-corpus synthesis

This phase cannot begin until all 72 per-trace analyses and 6 per-project summaries are complete.

- [x] **Master recommendation table.** Compile `analyses/recommended-cuts.md` with one row per trace: trace ID, path length, recommended cut step, recommended cut function, edge type at cut, boundary data class, state class, callback count, feasibility class.

- [x] **Surface-area analysis.** For each trace, record the recommended-cut depth as a fraction of path length (0.0 = entrypoint, 1.0 = region root). Answer: do deep cuts dominate? At what relative depth does surface area jump from Small to Large? Does codebase size predict the depth threshold?

- [x] **Boundary-data feasibility map.** For each trace, identify the shallowest feasible cut (no proxy-required or infeasible types). Answer: is there a consistent "feasibility cliff" where boundary data transitions? Do `http.ResponseWriter` and `io.Writer` account for most infeasibilities?

- [x] **State-reconstruction clustering.** Group all 72 recommended cuts by state class. Cross-tabulate with project. Answer: does codebase architecture predict state class? (e.g., Miniflux mostly client-reconstructible, Mattermost mostly shared-state?)

- [x] **Callback prevalence.** For each trace, record whether a zero-callback cut exists. Count traces with no zero-callback option. Answer: how common are unavoidable callbacks? Which edge types are associated with callbacks?

- [x] **Edge-type alignment statistics.** For each edge type, count: (a) total occurrences across all 72 traces, (b) occurrences at recommended cut points. Compute the boundary-affinity ratio (b/a). Answer: which edge types are disproportionately selected as cut points? Does the Strong/Weak/Anti classification hold empirically?

- [x] **Error-semantics survey.** Count region-root functions that return error vs. those that do not. For non-error-returning roots, record the distance to nearest error-returning ancestor. Answer: how many traces have error-semantics tension?

- [x] **Pareto frontier characterization.** Count traces where the recommended cut clearly dominates vs. traces with genuine multi-objective tradeoffs. For tradeoff traces, categorize the tension axis (surface vs. state, boundary data vs. edge alignment, etc.). Answer: how common are genuine Pareto tensions, and which dimension pairs create them?

- [x] **Cut-point archetypes.** Define and name recurring patterns. Hypothesized archetypes to confirm, refute, or refine:
  - *Pure Leaf:* deep cut at a stateless function with trivial boundary data and weak/anti edge signal. Wins on every dimension except edge alignment.
  - *Interface Proxy:* cut at an interface-dispatch edge with serializable boundary data. Strong edge signal; the interface already defines the replacement contract.
  - *Queue Worker:* cut after a channel/queue dispatch boundary with client-reconstructible state. Strong edge signal from the queue abstraction.
  - *Middleware Split:* cut mid-chain in an HTTP middleware stack. Proxy-required boundary data, but strong edge signal from the handler interface.
  - *Framework Callback:* cut at a framework-registered callback (cobra RunE, hook handler). Weak edge signal from struct-field dispatch, potentially clean boundary data.
  
  Document each confirmed archetype with 3+ representative traces and the typical dimension profile.

- [x] **Anti-boundary catalog.** List edge types that should never be cut points, with corpus counts and representative failure cases.

- [x] **Falsifiable pattern claims.** Test these hypotheses against the data:
  - "Interface-dispatch edges at depth >3 always have ≤Medium surface and zero callbacks"
  - "Goroutine-launch boundaries are anti-boundaries: always prefer the next deeper non-goroutine cut"
  - "HTTP-handler-registration edges are strong natural boundaries despite shallow depth"
  - "Reconstructible-state traces cluster around database-client patterns"
  - "A deep cut with trivial boundary types beats a shallow interface cut unless callbacks force the shallow cut"
  - "Zero-callback cuts exist for ≥90% of traces"

### Phase 8: Open questions from the brief

- [x] **Weighting.** Based on all 72 analyses, propose a default priority ordering for the six dimensions. Which dimension most often acts as the decisive tiebreaker? Is a single composite score feasible, or does the compiler need a decision tree?

- [x] **Composite cuts.** Identify traces where extracting a contiguous sub-path (two or more adjacent functions as a unit) produces a strictly better tradeoff than any single-node cut. How common? Which edge-type sequences correlate?

- [x] **Feasibility gates vs. scoring.** Enumerate boundary-data types that are never feasible (hard gates) vs. feasible-with-proxy (soft scores). Propose a two-tier model: gate check first, then score remaining candidates.

- [x] **Path-local vs. graph-global.** Check whether any region root appears as the target of multiple traces (different paths to the same function). If so, does the optimal cut differ across paths?

- [x] **Integration with liftability.** For traces where ADR-0018 liftability properties are visible in source (error contracts, boundary predicates), note whether those properties align with or contradict the cut-placement recommendation.

### Phase 9: Documentation and linking

- [x] Write `docs/research/activation-paths/cut-placement-synthesis.md`: executive summary, per-codebase findings, corpus-wide archetypes with representative traces, anti-boundary catalog with counts, answers to open questions, recommended next steps for compiler integration.
- [x] Update `docs/research/activation-paths/README.md` sprint history section with SPRINT-0039 entry and links to `analyses/` and `cut-placement-synthesis.md`.

## Sequencing

```
Phase 0 (calibration on 3 worked examples)
    │
    ▼
Phases 1–6 (per-trace analysis, parallelizable across projects)
    │
    ▼
Phase 7 (cross-corpus synthesis — requires all 72 analyses + 6 summaries)
    │
    ▼
Phase 8 (open questions — requires synthesis)
    │
    ▼
Phase 9 (documentation and linking)
```

Within Phases 1-6, individual trace analyses are independent. The per-project summary within each phase should be written last, after all traces for that project are complete.

Phase 0 must complete before any Phase 1-6 work begins — the calibration summary establishes the rubric.

## Risks

1. **Boundary-data classification in large codebases.** Gitea (456k LOC) and Mattermost (761k LOC) have deep type hierarchies and extensive interface wrapping. Determining whether a receiver type is Reconstructible vs. Shared-state requires reading multiple source files. *Mitigation:* budget more time per trace for these codebases; use package-level reasoning (type in `services/` wrapping a DB pool → Client-reconstructible) rather than exhaustive field-by-field analysis. Pre-cache frequently-used source files (HTTP libraries, queue interfaces). Use the trace JSON `to_raw` field as a hint for where to look.

2. **Surface-area estimation without call-graph data.** The trace JSON contains the activation path but not the full call graph below each node. *Mitigation:* use relative scales (Minimal through Very-large) based on the function's package, its architectural position, and visible direct callees. Note confidence level when uncertain. Do not fall back to "unknown" — make a judgment call and record the reasoning.

3. **Callback detection without full graph.** Detecting reverse edges requires the full call graph, not just the activation path. *Mitigation:* inspect source of functions at and below each candidate cut for obvious calls to functions above the cut. Accept that some callbacks will be missed. Default conservative: assume callbacks exist if unclear.

4. **Scoring subjectivity.** State reconstruction cost and boundary data complexity involve judgment. Different agents may score the same cut differently. *This is by design:* disagreements surface ambiguities in the framework. The synthesis should record disagreements rather than force artificial consensus.

5. **Volume management.** 72 traces × ~8 average path length = ~576 candidate cuts across 6 dimensions. *Mitigation:* steps 0-1 (entrypoint and main dispatch) are universally poor cuts and can be scored in one line. Focus analytical depth on the 2-4 genuinely competitive cut points per trace.

## Acceptance criteria

- [x] 72 per-trace analysis files exist at `docs/research/activation-paths/analyses/<trace-id>.md` (71 with full scoring, 1 structural gap note for mattermost-M-4)
- [x] Each analysis file contains: candidate cut-point table with all 6 dimension columns, recommended cut with rationale, tension notes, and observations section
- [x] 6 per-project summary files at `docs/research/activation-paths/analyses/<project>-summary.md`
- [x] `analyses/recommended-cuts.md` aggregates all 72 recommendations into a single table
- [x] `docs/research/activation-paths/cut-placement-synthesis.md` covers: executive summary, per-codebase findings, at least 3 named cut-point archetypes with representative traces, anti-boundary catalog with corpus counts, and answers to the brief's open questions
- [x] `docs/research/activation-paths/README.md` updated with SPRINT-0039 entry and links
- [x] No modifications to `pkg/`, `cmd/`, or `evaluation/` directories
