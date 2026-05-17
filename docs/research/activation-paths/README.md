# Activation-path research

**Research question:** What is the smallest static graph whose edges connect a binary entrypoint to a lift-region root, and what compiler algorithm generalizes across codebases to produce it?

## Data sources

### Lift-region candidate corpus

88 lift-region candidates across 6 Go codebases, selected by multi-model consensus (3 independent drafts → cross-critique → merged set per project).

- **Candidate manifest:** [`CANDIDATE-MANIFEST.md`](../runs/SPRINT-0034-lift-utility-corpus/CANDIDATE-MANIFEST.md) — master index with global IDs, region roots (linked to source), confidence tiers, and trace status.
- **Merged candidate sets:** one per project, with full rubric scoring, provenance, and exclusion rationale.
  - [`caddy/merged.md`](../runs/SPRINT-0034-lift-utility-corpus/projects/caddy/merged.md) (12 candidates)
  - [`gitea/merged.md`](../runs/SPRINT-0034-lift-utility-corpus/projects/gitea/merged.md) (21 candidates)
  - [`listmonk/merged.md`](../runs/SPRINT-0034-lift-utility-corpus/projects/listmonk/merged.md) (11 candidates)
  - [`mattermost/merged.md`](../runs/SPRINT-0034-lift-utility-corpus/projects/mattermost/merged.md) (17 candidates)
  - [`miniflux/merged.md`](../runs/SPRINT-0034-lift-utility-corpus/projects/miniflux/merged.md) (15 candidates)
  - [`pocketbase/merged.md`](../runs/SPRINT-0034-lift-utility-corpus/projects/pocketbase/merged.md) (12 candidates)
- **Selection rubric:** [`rubric.md`](../runs/SPRINT-0034-lift-utility-corpus/rubric.md)

### Activation-path traces

72 synthesized traces (medium+ confidence candidates). Each trace is a tabular path from binary entrypoint to region root, with typed edges representing the static-analysis resolution required at each step.

Synthesis files (one per candidate):
- [`caddy/traces/M-*.synthesis.md`](../runs/SPRINT-0034-lift-utility-corpus/projects/caddy/traces/) (6 traces)
- [`gitea/traces/M-*.synthesis.md`](../runs/SPRINT-0034-lift-utility-corpus/projects/gitea/traces/) (18 traces)
- [`listmonk/traces/M-*.synthesis.md`](../runs/SPRINT-0034-lift-utility-corpus/projects/listmonk/traces/) (10 traces)
- [`mattermost/traces/M-*.synthesis.md`](../runs/SPRINT-0034-lift-utility-corpus/projects/mattermost/traces/) (15 traces)
- [`miniflux/traces/M-*.synthesis.md`](../runs/SPRINT-0034-lift-utility-corpus/projects/miniflux/traces/) (12 traces)
- [`pocketbase/traces/M-*.synthesis.md`](../runs/SPRINT-0034-lift-utility-corpus/projects/pocketbase/traces/) (11 traces)

Each synthesis file contains: a tabular trace (From → To → Edge type), edge type summary, resolution notes for non-trivial edges, alternative paths, and metadata.

Raw data (3 independent traces + 3 cross-critiques per candidate) lives alongside the synthesis files in the same `traces/` directories.

**Machine-readable corpus:** [`traces/`](traces/) — 72 per-candidate JSON files (e.g. `miniflux-M-1.json`, `caddy-M-3.json`), parsed from the synthesis markdown. Each file contains the structured trace (steps with from/to file:line, edge type, function name), edge summary, region root, path length, and metadata. This is the ground truth for algorithm evaluation.

### Evaluation codebases

Pinned Go source trees at [`evaluation/`](../../../evaluation/) (SHAs in [`evaluation/MANIFEST.yaml`](../../../evaluation/MANIFEST.yaml)).

## Edge-type inventory

Extracted from all 72 synthesis traces. These are the static-analysis resolution categories the compiler must handle.

### Core types (~90% of all edges)

| Edge type | Count | Description | Analysis technique |
|---|---|---|---|
| `direct-function-call` | 240 | Static call, fully resolved at compile time | Standard call graph |
| `method-call-on-concrete-type` | 106 | Receiver type known statically | Standard call graph |
| `function-value-in-struct-field` | 70 | Func stored in a struct field, set at init/config time | Field-store tracking |
| `interface-method-dispatch` | 47 | Caller holds interface; resolved by type analysis | CHA / RTA / VTA |
| `goroutine-launch` | 29 | `go f()` — async boundary | Go-statement tracking |
| `http-handler-registration` | 19 | `mux.Handle("/path", handler)` — route wiring | Pattern recognition |

### Secondary types (~8% of all edges)

| Edge type | Count | Description |
|---|---|---|
| `channel-send-receive` | 8 | Value passed through a channel (queue-worker pattern) |
| `struct-literal-field-assignment` | 6 | Func/interface assigned in a struct literal |
| `closure-capture` | ~12 | Closure binds over a function/method, passed or stored |
| `callback-registration` | ~7 | Function stored for later invocation (hooks, observers) |

### Long tail (~2% of all edges)

One-off variants that may collapse into the categories above during normalization: `reflective-call-via-string-keyed-map`, `type-asserted-method-call`, `promoted-method-call-through-embedded-field`, etc. See [`edge-type-taxonomy.md`](edge-type-taxonomy.md) (to be written) for the full normalized taxonomy.

## Prior work in this repo

Sprints 25–33 explored entrypath algorithms with different approaches (frontier search, bridge algorithm, oracle-guided). Key prior artifacts:
- [`SPRINT-0029-bridge-algorithm.md`](../runs/SPRINT-0029-bridge-algorithm.md) — bridge-based path finding
- [`SPRINT-0032-entrypath-bridge-algorithm.md`](../runs/SPRINT-0032-entrypath-bridge-algorithm.md) — refined bridge approach
- [`SPRINT-0033-lift-target-catalog.md`](../runs/SPRINT-0033-lift-target-catalog.md) — structural diversity catalog (predecessor to the utility corpus)

## Sprint history

### SPRINT-0035: RTA baseline (done)

Built `cmd/activation-path/` and `pkg/activation/` from scratch. RTA call graph + BFS shortest-path + tiered evaluation harness.

**Result:** 49/72 reachable (68%). miniflux/listmonk/pocketbase at 100%. caddy 0%, mattermost 0% — all blocked by `StructFieldFuncValue` (cobra/urfave `RunE`/`Action` dispatch). All 6 codebases feasible for RTA.

- [`SPRINT-0035-rta-baseline.md`](../runs/SPRINT-0035-rta-baseline.md)
- [`SPRINT-0035-rta-baseline.json`](../runs/SPRINT-0035-rta-baseline.json)

### SPRINT-0036: Struct-field tracking + framework predicates (done)

Added struct-field function-value tracking (SSA `FieldAddr`+`Store`/`Load` scan), framework predicate registry (cobra `RunE`, urfave/cli v3 `Action`), goroutine-launch edges, and partial-path emission.

**Result:** Tier 1 stayed at 49/72. The augmentation passes create the correct first-hop edges (e.g., `cobra.execute → cmdRun`), but newly-discovered functions are **dead ends in the graph** because RTA ran before augmentation and never explored their call trees. See [Key Finding](#key-finding-rta-augmentation-ordering) below.

- [`SPRINT-0036-augmentation-report.md`](../runs/SPRINT-0036-augmentation-report.md)
- [`SPRINT-0036-final.json`](../runs/SPRINT-0036-final.json)

### SPRINT-0037: Re-rooted RTA after augmentation (done)

Fixed the RTA-augmentation ordering bug by iterating augmentation with re-rooted RTA exploration from newly added functions only. Augmentation-discovered command handlers now have their transitive callees explored, so dead-end nodes such as caddy `cmdRun` are no longer terminal.

**Result:** 69/72 reachable (95.8%), up from 49/72. Caddy improved from 0/6 to 6/6 and Mattermost from 0/15 to 14/15. RTA-only mode still reproduces the 49/72 SPRINT-0035 baseline, and all 49 previously reachable traces remain reachable. All projects converged in 2-3 exploration rounds.

Remaining misses are now small and concrete: gitea callback registration, gitea map-indexed function-value dispatch, and mattermost/M-4 target package loading.

- [`SPRINT-0037-report.md`](../runs/SPRINT-0037-report.md)
- [`SPRINT-0037-full.json`](../runs/SPRINT-0037-full.json)

### SPRINT-0038: Callback arguments + map-keyed dispatch (done)

Closed the two remaining Gitea algorithm gaps by adding augmentation passes for package-level function/interface variables, function values passed as callback arguments, map-keyed function registries, and the embedded-interface field propagation needed by Gitea's password hash factory.

**Result:** 71/72 reachable (98.6%), up from 69/72. Gitea is now 18/18; `gitea/M-13` resolves through `queue.CreateSimpleQueue` callback registration and package-level `sender_service.Send`, and `gitea/M-16` resolves through `availableHasherFactories` to `NewArgon2Hasher` and then to `(*Argon2Hasher).HashWithSaltBytes`. The only remaining corpus miss is `mattermost/M-4`, still categorized as target-not-found.

- [`SPRINT-0038-final.json`](../runs/SPRINT-0038-final.json)

### SPRINT-0039: Cut-placement analysis (done)

Scored every activation-path node as a candidate network boundary across the 72-trace corpus, using six dimensions: extraction surface, boundary data, state reconstruction, callbacks, error semantics, and edge alignment. The analysis includes 71 full per-trace cut tables plus the accepted `mattermost/M-4` gap note, six project summaries, a master recommendation table, corpus statistics, and open-question answers.

**Result:** Deep cuts dominate: 65/71 reachable recommendations are at depth `>= 0.75`, and the median recommended depth is the region root. Boundary data is the decisive gate; strong edges are useful evidence only after proxy-required and infeasible values are handled.

- [`analyses/`](analyses/)
- [`analyses/recommended-cuts.md`](analyses/recommended-cuts.md)
- [`cut-placement-synthesis.md`](cut-placement-synthesis.md)
- [`boundary-adapter-strategy.md`](boundary-adapter-strategy.md) — follow-on
  strategy for `AdapterPossible` cuts where the semantic target is good but
  the source signature needs a generated network-boundary adapter.

## Key finding: RTA-augmentation ordering

**Discovered during SPRINT-0036 investigation.** This is the central architectural issue for the next sprint.

### The problem

The current pipeline is:

```
BuildRTAGraph() → AugmentStructField() → ApplyPredicates() → AugmentGoroutine() → ShortestPath()
```

RTA builds the call graph from `main`, exploring only functions reachable through the RTA-visible call graph. Then the struct-field pass adds edges like `cobra.(*Command).execute → cmdRun`. But `cmdRun` was never RTA-reachable, so **its own callees were never explored**. The node exists in the graph with an incoming edge but zero outgoing edges. BFS reaches `cmdRun` and stops — there's nowhere to go.

### Concrete evidence

```
$ activation-path --target cmd/commandfuncs.go:172 --augmentations all
found: 6 steps
main → Main → Execute → ExecuteC → execute → [StructFieldFuncValue] → cmdRun ✓

$ activation-path --target caddy.go:115 --augmentations all
miss: target-unreachable
```

`cmdRun` calls `caddy.Load` at `commandfuncs.go:262` — a direct function call. But `caddy.Load` is not in the graph because RTA never visited `cmdRun`'s body.

### The fix

After augmentation adds new functions to the graph, their call trees must be transitively explored. Options:

1. **Re-run RTA** with the augmentation-discovered functions as additional roots. This is the simplest — treat newly-added target functions as extra entrypoints and re-run RTA from them.
2. **Incremental call-graph extension** — when `AddNode` adds a function not previously in the graph, walk its SSA body and add all its static/interface callees recursively. More surgical than re-running full RTA but requires careful handling of interface dispatch.
3. **Interleave augmentation with RTA** — run struct-field scanning during RTA's worklist iteration so newly-discovered functions are immediately added to the RTA worklist. Most precise but requires modifying the RTA algorithm.

Option 1 is the pragmatic first step. The struct-field pass discovers ~20 new functions across the corpus; re-running RTA from those roots adds their transitive callees to the graph. The cost is one additional RTA pass (cheap — the bulk of the program is already loaded).

### Impact estimate

This fix would likely unblock most of the 22 `StructFieldFuncValue` traces, since the struct-field edges themselves are already correct — they just lead to dead-end nodes. How many traces remain blocked after the fix depends on whether the *subsequent* edges in each trace are RTA-resolvable (direct calls, interface dispatch) or require additional augmentation (HTTP registration, closure capture, channel flow).

## Next steps

1. **Mattermost/M-4 target loading** — the remaining corpus miss is target-not-found rather than an unresolved activation edge.
2. **Path quality** — Tier 1 reachability is effectively saturated; remaining work should focus on shorter or more trace-faithful paths.
3. **Taxonomy cleanup** — fold the new package-var and map-dispatch edge kinds into the written normalized taxonomy.
