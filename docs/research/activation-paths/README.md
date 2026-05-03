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

## Next steps

1. **Edge-type taxonomy normalization** — collapse the long tail into ~8 canonical types with formal definitions. Output: `edge-type-taxonomy.md`.
2. **Algorithm prototype** — Go program that builds an augmented call graph (RTA + struct-field tracking + pattern recognition) and finds shortest paths from entrypoints to annotated regions. Test against the 72 traces as ground truth.
3. **Coverage measurement** — for each trace, does the algorithm reproduce the path? Where does it fail? Each failure maps to a specific edge type, guiding what analysis to add next.
