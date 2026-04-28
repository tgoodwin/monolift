# SPRINT-0032 - Bridge algorithm consolidation

**Status:** planned  
**Executor:** TBD  
**Predecessor:** SPRINT-0031 implemented deterministic bridge-index priority diagnostics. The main Mattermost chain recovered with a 65s index budget, while the 60s row missed because bridge discovery consumed the shared budget before indexing began.

## Intent

Consolidate the EntryPath bridge strategy into a stable, understandable v1 algorithm. The sprint should find the current Pareto point: preserve the recall gains from touchpoint-to-boundary bridge discovery, take only low-complexity efficiency/default improvements that are backed by evidence, and avoid further Mattermost-specific or speculative expansion. The final deliverable must include an approachable algorithm summary explaining how SSA, RTA/VTA, callgraph, reverse touchpoints, bridge owners, boundary evidence, and function-reference indexing fit together.

## Goals

- Define the bridge algorithm as a coherent v1 strategy rather than a sequence of diagnostics.
- Clarify budget semantics so bridge discovery and bridge indexing have an understandable cost envelope.
- Make at most one small implementation/default cleanup if it improves the Pareto tradeoff without adding substantial complexity.
- Validate the consolidated strategy against the Mattermost oracle and compare it to exhaustive, oracle bridge, and recent bridge rows.
- Produce a clear summary document that a new contributor can read to understand the algorithm and why it is generalizable.

## Non-goals

- No new broad graph search strategy.
- No Mattermost-specific production recognizers, package-name hacks, route-name checks, or framework-specific string matching.
- No expansion of bridge discovery budgets unless the validation proves the target owners are no longer admitted.
- No attempt to optimize away global package loading, SSA construction, or callgraph construction in this sprint.
- No reportv2, surface, transport, liftpatch, bootpath, or emission changes.
- No open-ended tuning matrix. Keep validation small and decision-oriented.

## Phase 0 - Context and Pareto Baseline

- [x] **0.1** Read SPRINT-0025 through SPRINT-0031 closeouts and reports relevant to EntryPath bridge recovery.
- [x] **0.2** Create `docs/research/runs/SPRINT-0032-bridge-pareto.md` summarizing the current cost/recall frontier.
- [x] **0.3** Include at least these rows: SPRINT-0025 exhaustive/all, SPRINT-0028 oracle bridge v2, SPRINT-0030 default, SPRINT-0030 `index180`, SPRINT-0031 default, and SPRINT-0031 `index65`.
- [x] **0.4** State the working thesis: bridge discovery is promising but only Pareto-useful if its defaults are simple and its cost envelope is honest.

## Phase 1 - Algorithm Freeze

- [x] **1.1** Write a concise algorithm sketch in `docs/research/runs/SPRINT-0032-entrypath-bridge-algorithm.md`.
- [x] **1.2** Explain each phase: package load, SSA build, RTA/VTA/callgraph, reverse BFS touchpoints, bridge start selection, local package owner scan, generic boundary-owner admission, prioritized function-ref indexing, and function-value flow/classification.
- [x] **1.3** Explain the data each phase consumes and produces.
- [x] **1.4** Explain why the approach is generalizable beyond Mattermost HTTP handlers, while naming the current limits of the boundary predicates.
- [x] **1.5** Include a small glossary for “touchpoint,” “owner,” “boundary owner,” “seed,” “function-ref index,” “wrapper chain,” and “oracle trace.”

## Phase 2 - Budget Semantics Cleanup

- [x] **2.1** Inspect current bridge budget handling and determine whether the function-index budget is effectively shared with bridge discovery.
- [x] **2.2** If the SPRINT-0031 loss is caused by confusing shared budget semantics, implement one small cleanup: make bridge discovery and function-ref indexing use explicit phase-local budgets or an explicit bridge-index reserve.
- [x] **2.3** If the current semantics are already intentionally phase-local, do not change code; document why the 60s/65s behavior happened and what default should be advertised.
- [x] **2.4** Keep the change limited to entrypath/probe configuration and diagnostics; do not change other compiler passes.
- [x] **2.5** Preserve existing non-bridge index behavior unless bridge mode is enabled.
- [x] **2.6** Add or update focused tests for the chosen budget semantics.

## Phase 3 - Pareto Validation Matrix

- [x] **3.1** Run the SPRINT-0028 oracle spec against the consolidated bridge mode with the chosen default settings.
- [x] **3.2** If code/defaults changed, run one before/after comparison row using equivalent budgets.
- [x] **3.3** If no code/defaults changed, run one confirmation row and explain why the current `index65` envelope is the recommended bridge profile.
- [x] **3.4** Produce raw JSON/stderr/summary artifacts under `docs/research/runs/SPRINT-0032-*`.
- [x] **3.5** Report target node recovery for `connectWebSocket`, `APIHandlerTrustRequester`, and `InitWebSocket`.
- [x] **3.6** Report relationship recovery for `connect-to-api-handler`, `connect-registered-at-init`, and `init-has-http-boundary`.
- [x] **3.7** Compare wall time, phase timing, peak RSS, admitted bridge owners, indexed bridge owners, and final recovery against the Pareto baseline.

## Phase 4 - Decision: Promising or Not

- [x] **4.1** Add a “Read” section to `SPRINT-0032-bridge-pareto.md` that classifies the approach as promising, marginal, or not promising.
- [x] **4.2** Define the classification criteria explicitly: recall, cost relative to exhaustive, implementation complexity, generalizability, and remaining risk.
- [x] **4.3** State the recommended stop/continue point for EntryPath bridge work.
- [x] **4.4** Identify exactly one future optimization hypothesis that would be worth pursuing only if EntryPath remains a priority.
- [x] **4.5** Identify at least one optimization path not worth pursuing next.

## Phase 5 - Final Explainer

- [x] **5.1** Create `docs/research/runs/SPRINT-0032-entrypath-bridge-summary.md`.
- [x] **5.2** Write it for a new contributor: approachable, plain-language, and diagram-friendly, without assuming prior sprint history.
- [x] **5.3** Include a step-by-step narrative of how a lifted region becomes external entrypath candidates.
- [x] **5.4** Explain SSA, RTA, VTA, and callgraph roles in practical terms.
- [x] **5.5** Explain where cost comes from and why this strategy improves over exhaustive without being a silver bullet.
- [x] **5.6** Include a short “When this should work / when it may fail” section.

## Phase 6 - Closeout

- [x] **6.1** Update this sprint file closeout with phases run/cut, code/default changes, final Pareto read, artifacts, tests, and recommendation.
- [x] **6.2** Add a concise “Algorithm v1” section pointing to the summary doc.
- [x] **6.3** Add a “Do next / do not do next” recommendation.

## Guardrails

- [x] **G.1** Run `go test ./pkg/compiler/entrypath`.
- [x] **G.2** Run `go test ./cmd/entrypath-probe`.
- [x] **G.3** Keep forbidden-string lint passing against `pkg/compiler/entrypath/*.go` non-test sources. Banned strings: `"websocket"`, `"Mux"`, `"HandleFunc"`, `"mattermost"`, `"gorilla"`, `"chi"`, `"echo"`, `"gin"`. Allowlist remains `"http.Handler"`, `"ServeHTTP"`, `"net/http"`.
- [x] **G.4** Verify no intentional changes under `pkg/compiler/reportv2/`, `pkg/compiler/surface/`, `pkg/compiler/transport/emit/`, `pkg/compiler/extract/bootpath/`, `evaluation/mattermost/`, or `docs/decisions/`.
- [x] **G.5** If executing with a subagent, use local filesystem and shell only; do not initialize or use MCP/connectors/web/plugin tools.

## Acceptance Criteria

- [x] Bridge algorithm v1 is documented clearly enough for a new contributor.
- [x] The summary explains SSA, RTA, VTA, callgraph, reverse BFS, bridge discovery, boundary owners, and indexing.
- [x] The Pareto report compares cost/recall against recent baselines and gives a clear promising/marginal/not-promising read.
- [x] At most one small implementation/default cleanup is made, and only if evidence supports it.
- [x] Mattermost oracle validation confirms the final recommended bridge profile.
- [x] Tests and forbidden-string lint pass, or failures are recorded with relevance.
- [x] Closeout recommends whether to stop, ship/consolidate, or pursue exactly one future optimization.

## Closeout

### Phases Run

All planned phases were run. Phase 3 used the SPRINT-0028 Mattermost oracle spec
against bridge mode with the SPRINT-0031 bridge discovery/profile settings and
a 60s phase-local function-index budget.

### Phases Cut

None. Phase 2.3 was not applicable because the inspected bridge semantics were
not already phase-local; SPRINT-0032 changed bridge mode accordingly.

### Code or Default Changes

Changed `FunctionIndexModeBridge` so bridge seed discovery no longer subtracts
elapsed time from `FunctionRefIndexBudget`. Bridge discovery remains bounded by
the explicit bridge budgets; the function-reference index gets the configured
index budget when it starts. Non-bridge modes keep their previous behavior.

Added a focused budget-semantics test in `pkg/compiler/entrypath`.

### Pareto Read

Promising, with a clear stop point. The cleaned-up bridge row indexed all 1,676
admitted bridge owners under a nominal 60s index budget and recovered the main
Mattermost oracle chain. The approach remains much smaller than exhaustive
indexing, but package load, SSA, callgraph, and bridge discovery still dominate
large-target wall time.

### Algorithm v1

Algorithm sketch:
`docs/research/runs/SPRINT-0032-entrypath-bridge-algorithm.md`

Contributor summary:
`docs/research/runs/SPRINT-0032-entrypath-bridge-summary.md`

### Recommendation

Stop expanding EntryPath bridge for now and consolidate this as v1. The one
future optimization worth considering, only if EntryPath remains a priority, is
more selective bridge start/package derivation from registration evidence. Do
not pursue broad graph-search expansion or framework/package-name special cases
next.

### Pointers

- Pareto report: `docs/research/runs/SPRINT-0032-bridge-pareto.md`
- Validation report: `docs/research/runs/SPRINT-0032-bridge-validation.md`
- Raw validation artifacts:
  `docs/research/runs/SPRINT-0032-bridge-phase-local.{json,stderr,summary.json}`
- Tests: `go test ./pkg/compiler/entrypath`, `go test ./cmd/entrypath-probe`
