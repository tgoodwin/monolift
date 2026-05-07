# Cut-placement analyzer evaluation

## Context

SPRINT-0039 produced 72 per-trace cut-point analyses as ground truth —
human-recommended network boundaries scored across six dimensions. SPRINT-0040
implemented an automated cut-placement analyzer (`AnalyzeCut` in
`pkg/activation/`) that consumes an activation path and selects the optimal
single-node cut using a decision-tree approach.

This document records the evaluation of the automated analyzer against the
ground truth, the divergence patterns discovered during iteration, and the
design questions that emerged.

## Algorithm overview

The analyzer applies a decision tree rather than a weighted score:

1. **Hard gate:** reject candidates with infeasible boundary data (function
   values, mutexes, runtime handles).
2. **Classify:** separate proxy-required candidates (http.ResponseWriter,
   io.Writer, channels) from ordinary RPC candidates.
3. **Project filter:** exclude candidates from packages outside the project's
   Go module (stdlib and third-party framework internals are not lift
   candidates).
4. **Rank** surviving candidates lexicographically: surface area (smaller
   first), then callbacks (zero first), state reconstruction cost, error
   semantics, edge-type alignment.
5. **Tiebreak:** prefer deeper path step, then node key.

Three corpus-driven corrections reshaped the algorithm during iteration:

- **Surface-first ranking.** The initial implementation ranked callbacks before
  surface area, causing shallow bootstrap functions (stateless, zero callbacks,
  VeryLarge surface) to beat deep service functions. Reordering surface before
  callbacks matched the corpus finding that mean recommended depth is 0.924.

- **Receiver exclusion from boundary data.** The boundary-data classifier
  initially walked receiver types alongside parameters, causing receivers with
  sync primitives or func-typed fields (common in `*App` / `*Server` structs)
  to hard-gate the candidate as infeasible. Receivers are reconstructed
  on the remote side — they are state, not boundary data. Excluding them
  eliminated the "no competing feasible candidate remained" pattern that
  dominated Mattermost and Listmonk.

- **Project-locality filter.** The activation path traverses framework
  internals (Go stdlib, protobuf, goja) that happen to have simple boundary
  data but are not lift candidates. Filtering candidates to the project's
  module eliminated picks like `fmt.Printf` and `goja.AssertFunction`.

## Corpus results

71 reachable traces across 6 codebases (mattermost/M-4 skipped as a
structural target-loading gap).

| Project | Traces | Exact | Step-name match | Other acceptable | Disagreements | Mean step distance |
|---|---|---|---|---|---|---|
| Caddy | 6 | 1 | 2 | 3 | 0 | 5.2 |
| Miniflux | 12 | 5 | 5 | 2 | 0 | 0.9 |
| Listmonk | 10 | 2 | 1 | 5 | 0 | 2.9 |
| PocketBase | 11 | 1 | 5 | 3 | 1 | 4.4 |
| Gitea | 18 | 4 | 3 | 10 | 1 | 3.5 |
| Mattermost | 14 | 1 | 10 | 2 | 0 | 2.6 |
| **Total** | **71** | **14** | **26** | **25** | **2** | **3.1** |

Including step-name matches (same function, different path structure),
the effective agreement rate is **40/71 (56%)**. Zero infeasible
recommendations for traces marked feasible in the ground truth, with two
remaining disagreements (pocketbase/M-9 and gitea/M-8) where all project
candidates are gated by boundary-data types not yet in the known-type table.

## Divergence taxonomy

The 57 non-exact results (55 acceptable + 2 disagreements) fall into four categories:

### Step-numbering misalignment (26 cases, all acceptable)

The analyzer picked the **same function** as the ground truth but at a
different step number because the activation-path algorithm found a
different (often shorter) path than the hand-traced ground truth.

| Trace | Expected step | Got step | Δ | Function |
|---|---|---|---|---|
| caddy/M-1 | 13 | 12 | -1 | funcMarkdown |
| caddy/M-4 | 10 | 8 | -2 | (InternalIssuer).Issue |
| miniflux/M-1 | 5 | 4 | -1 | RefreshFeed |
| miniflux/M-3 | 6 | 7 | +1 | SanitizeHTML |
| miniflux/M-5 | 7 | 5 | -2 | UpdateOrCreateFeedIcon |
| miniflux/M-7 | 6 | 7 | +1 | ScrapeWebsite |
| miniflux/M-8 | 6 | 7 | +1 | ScrapeWebsite |
| miniflux/M-9 | 8 | 4 | -4 | SendEntry |
| listmonk/M-2 | 4 | 3 | -1 | Push |
| pocketbase/M-1 | 11 | 6 | -5 | CreateThumb |
| pocketbase/M-2 | 8 | 6 | -2 | recordAuthWithOAuth2 |
| pocketbase/M-4 | 9 | 7 | -2 | Create |
| pocketbase/M-5 | 9 | 5 | -4 | SendRecordPasswordReset |
| pocketbase/M-6 | 8 | 7 | -1 | setValue |
| pocketbase/M-11 | 9 | 6 | -3 | resolveEmailTemplate |
| gitea/M-7 | 6 | 4 | -2 | checkPullRequestMergeable |
| gitea/M-11 | 7 | 4 | -3 | InitIssueIndexer |
| gitea/M-17 | 12 | 8 | -4 | RenderFullFile |
| mattermost/M-3 | 13 | 7 | -6 | handleWebhookEvents |
| mattermost/M-5 | 3 | 5 | +2 | bulkExportCmdF |
| mattermost/M-6 | 13 | 11 | -2 | getLinkMetadataForURL |
| mattermost/M-7 | 13 | 11 | -2 | DoCommandRequest |
| mattermost/M-8 | 9 | 12 | +3 | sendPushNotificationToAllSessions |
| mattermost/M-11 | 3 | 5 | +2 | bulkImportCmdF |
| mattermost/M-12 | 12 | 7 | -5 | sendNotificationEmail |
| mattermost/M-14 | 12 | 11 | -1 | Hash |
| mattermost/M-15 | 2 | 5 | +3 | slackImportCmdF |

**Implication:** These are not algorithm errors. The activation-path algorithm
and the manual traces find different (but valid) paths through the same
codebase. The cut function is correct; only the step numbering differs.
This category doubled from 13 to 26 after the Pattern A/B/D known-type
fixes, because previously-gated candidates are now feasible and the
algorithm reaches the correct function via a different path.

### Boundary classifier still too aggressive (8 acceptable + 2 disagreements)

After the Pattern A/B/D fixes (sync.Mutex skip, framework context overrides,
cobra.Command override), 10 traces still hit "no competing feasible candidate
remained." These represent types not yet in the known-type table or edge
cases in the struct/interface walkers.

- **caddy/M-2, M-5, M-7** (acceptable): deep candidates carry streaming
  types per ADR-0028 (ResponseWriter in the HTTP handler chain). Only
  `cmd.Main` survives. These are the HTTP middleware targets that a
  developer would not realistically annotate for lifting.
- **pocketbase/M-7, M-8** (acceptable): hook closure functions survive but
  the deep named targets have types the classifier doesn't handle.
- **gitea/M-3, M-11, M-17** (acceptable): shallow candidates survive.
- **pocketbase/M-9** (disagreement): all project candidates gated.
- **gitea/M-8** (disagreement): all project candidates gated — the
  diff-rendering pipeline has types not yet in the known-type table.

**Implication:** The remaining 3 Caddy cases are design-correct (HTTP
middleware is not a realistic lift target). The other 7 need additional
known-type overrides or more sophisticated type-walker heuristics. See
`known-type-investigation.md` for the root-cause analysis.

### Proxy preference (6 cases: 4 acceptable, 2 disagreements) — resolved by ADR-0028

The ground truth recommends a FeasibleWithProxy cut (typically an HTTP
middleware function carrying `http.ResponseWriter`), but the analyzer
prefers an ordinary Feasible candidate at a shallower point.

| Trace | Ground truth | Analyzer pick | Reason |
|---|---|---|---|
| caddy/M-2 | `executeTemplate` (proxy) | `cmd.Main` (ordinary) | Prefers ordinary feasibility |
| caddy/M-5 | `(*Encode).ServeHTTP` (proxy) | `cmd.Main` (ordinary) | Prefers ordinary feasibility |
| caddy/M-7 | `loadDirectoryContents` (proxy) | `cmd.Main` (ordinary) | Prefers ordinary feasibility |
| gitea/M-13 | `send` (proxy) | `NewContext$1` (ordinary) | Prefers ordinary feasibility |
| pocketbase/M-7 | `send` (feasible) | `send` (proxy, -5) | Classified as proxy |
| pocketbase/M-9 | `safeFileFromURL` (feasible) | `safeFileFromURL` (proxy, -8) | Classified as proxy |

**Resolution (ADR-0028):** The monolith is the gateway — it handles
HTTP lifecycle, and only the cut-point function's parameters/returns
cross the network. FeasibleWithProxy is retired as a category. Streaming
types at the cut point are a signal to cut deeper. These 6 traces are
reclassified as cases where the developer would target the domain
function below the middleware, not the middleware itself.

### Algorithm chose a different nearby function (19 cases, all acceptable)

The algorithm applied its heuristics and picked a different feasible
function that scores better on one or more dimensions. These are
legitimate differences in judgment, not errors. The decisive dimensions
break down as:

| Decisive dimension | Count | Pattern |
|---|---|---|
| Callbacks (zero > low) | 7 | Algorithm prefers confirmed-zero-callback function over deeper function with low callback pressure |
| Surface (smaller wins) | 5 | Algorithm picks mid-path function with smaller surface area |
| Error semantics (OK > NeedsWrapper) | 5 | Algorithm prefers function returning `error` over one returning non-error type |
| Depth tiebreaker | 1 | Same scores, deeper step wins |
| Edge alignment (Strong > Anti) | 1 | Interface dispatch edge wins over direct call |

The callback and error-semantics categories reveal a systematic difference
between the algorithm and human judgment: **the algorithm is more
conservative**. Human reviewers preferred the "right" domain function
even when it had low callbacks or non-error returns. The algorithm treats
those as ranking dimensions and picks the nearby alternative that scores
better.

This is arguably correct behavior — a function with zero callbacks and
error returns is genuinely easier to lift than one with low callbacks and
boolean returns. Whether the algorithm should defer to the developer's
intent (the annotated lift target) over its own scoring is a design
question for the liftability integration phase.

**Implication:** The automated analyzer sees the full SSA call graph
(including anonymous closures and compiler-generated functions) while the
ground truth was traced from source. The analyzer's picks are defensible —
they just optimize a different dimension than the human reviewer
prioritized.

## Design questions surfaced

1. **FeasibleWithProxy is retired (ADR-0028).** The monolith serves as
   the gateway after lifting — clients still hit the original API surface,
   and only the cut-point function's parameters/returns cross the network.
   Streaming types at the cut point (`http.ResponseWriter`, `io.Writer`)
   are a signal to cut deeper, not to add a proxy. The three-way
   classification collapses to Feasible / Infeasible. The 6 proxy-preference
   traces are reclassified as cases where the developer would target the
   domain function below the middleware, not the middleware itself.

2. **Should anonymous closures and compiler-generated functions be
   candidate cut points?** The SSA representation includes them, and
   they sometimes score well, but they are not functions a developer
   would recognize as lift targets. Possibly addressable via a sourcemap
   approach that presents the decision in terms of the original source.

3. **How should the corpus harness handle step-numbering misalignment?**
   Matching by function name rather than step index would reclassify
   13 acceptable divergences as exact matches, bringing the total to
   29/71 exact. The current delta-reporting approach is adequate for now.

## Iteration history

| Iteration | Fix | Exact | Acceptable | Disagree | Mean step dist |
|---|---|---|---|---|---|
| 1 | Initial implementation | 12 | 56 | 3 | — |
| 2 | Surface-first ranking | 13 | 55 | 3 | — |
| 3 | Receiver exclusion from boundary data | 15 | 55 | 1 | — |
| 4 | Project-locality filter | 16 | 51 | 4 | 3.5 |
| 5 | ADR-0028 + Pattern A/B/D known-type fixes | 14 | 55 | 2 | 3.1 |
| — | + function-name matches | **40** | 25 | 2 | — |
