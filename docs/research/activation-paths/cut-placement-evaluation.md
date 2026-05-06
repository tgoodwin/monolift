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
| Miniflux | 12 | 7 | 4 | 1 | 0 | 0.8 |
| Listmonk | 10 | 3 | 1 | 6 | 0 | 2.8 |
| PocketBase | 11 | 1 | 2 | 5 | 3 | 4.5 |
| Gitea | 18 | 3 | 3 | 11 | 1 | 3.9 |
| Mattermost | 14 | 1 | 1 | 12 | 0 | 3.7 |
| **Total** | **71** | **16** | **13** | **38** | **4** | **3.5** |

Zero infeasible recommendations for traces marked feasible in the ground
truth, with the exception of one PocketBase trace (M-6) where all project
candidates were gated by boundary-data infeasibility.

## Divergence taxonomy

The 55 non-exact results fall into four categories:

### Step-numbering misalignment (13 cases, all acceptable)

The analyzer picked the **same function** as the ground truth but at a
different step number because the activation-path algorithm found a
different path than the hand-traced ground truth.

| Trace | Expected step | Got step | Δ | Function |
|---|---|---|---|---|
| caddy/M-1 | 13 | 12 | -1 | funcMarkdown |
| caddy/M-4 | 10 | 8 | -2 | (InternalIssuer).Issue |
| miniflux/M-1 | 5 | 4 | -1 | RefreshFeed |
| miniflux/M-3 | 6 | 7 | +1 | SanitizeHTML |
| miniflux/M-5 | 7 | 5 | -2 | UpdateOrCreateFeedIcon |
| miniflux/M-9 | 8 | 4 | -4 | SendEntry |
| listmonk/M-2 | 4 | 3 | -1 | Push |
| pocketbase/M-1 | 11 | 6 | -5 | CreateThumb |
| pocketbase/M-4 | 9 | 7 | -2 | Create |
| gitea/M-7 | 6 | 4 | -2 | checkPullRequestMergeable |
| gitea/M-11 | 7 | 4 | -3 | InitIssueIndexer |
| gitea/M-17 | 12 | 8 | -4 | RenderFullFile |
| mattermost/M-14 | 12 | 11 | -1 | Hash |

**Implication:** These are not algorithm errors. The activation-path algorithm
and the manual traces find different (but valid) paths through the same
codebase. The cut function is correct; only the step numbering differs.

### Known-type refinements needed (18 cases: 16 acceptable, 2 disagreements)

The boundary-data classifier gates out intermediate candidates (reporting
"no competing feasible candidate remained"), pushing the algorithm to a
fallback. The gated candidates have parameter or return types that are
actually serializable or reconstructible but are not recognized by the
current type-walker.

Representative cases:
- **listmonk/M-4** (delta=+5): expected `(*App).UploadMedia` at step 3,
  got `processImage` at step 8. The `*App` receiver's parameter types
  include types the classifier doesn't yet handle.
- **gitea/M-8** (disagreement): no recommendation — all project candidates
  gated as infeasible due to unrecognized types in the diff-rendering pipeline.
- **pocketbase/M-6** (disagreement): no recommendation — all project
  candidates gated as infeasible.

**Implication:** The Phase 7 type-walker refinements (named interface
walking, known-type overrides for `*sql.DB`, `*http.Client`, etc.) would
resolve most of these. This is the clearest improvement target.

### Proxy preference (6 cases: 4 acceptable, 2 disagreements)

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

**Implication:** This is a design question, not a bug. The research
identified the "HTTP/Request Shell Escape" archetype — when the lift
target is inherently HTTP middleware, accepting a streaming proxy is
the right engineering choice. The analyzer currently prefers ordinary
feasibility, which avoids proxy complexity but misses the intended
cut. A future refinement could allow proxy-required candidates to win
when the target function itself is in the HTTP handler chain.

### Algorithm chose a different nearby function (18 cases, all acceptable)

The algorithm applied its heuristics and picked a different feasible
function that scores better on one or more dimensions. These are
legitimate differences in judgment, not errors.

Common patterns:
- **Error semantics preference:** Gitea M-1, M-2, M-5, M-10, M-16 — the
  analyzer picks a function that returns `error` over a deeper function
  that returns a non-error type. The research recommended the deeper
  function because it matched the lift target more closely, but the
  analyzer's choice is defensible: error-returning functions are easier
  to wrap for network failure.
- **Callback avoidance:** Gitea M-12, mattermost/M-13 — the analyzer
  picks a function with zero confirmed callbacks over one with Low
  callbacks. Both are feasible; the analyzer is more conservative.
- **Surface minimization:** Mattermost M-1, M-3, M-6, M-12 — the
  analyzer picks a mid-path closure or anonymous function with Minimal
  surface over a deeper named function with Small surface. These closures
  are real functions in the SSA but were not listed in the hand-traced
  ground truth.

**Implication:** Many of these divergences reflect that the automated
analyzer sees the full SSA call graph (including anonymous closures and
compiler-generated functions) while the ground truth was traced from
source. The analyzer's picks are defensible — they just optimize a
different dimension than the human reviewer prioritized.

## Design questions surfaced

1. **Should the analyzer accept proxy-required cuts for HTTP middleware
   targets?** The current policy prefers ordinary feasibility. The
   research says both are valid — the "HTTP Shell Escape" archetype
   exists because sometimes the proxy is worth accepting.

2. **Should anonymous closures and compiler-generated functions be
   candidate cut points?** The SSA representation includes them, and
   they sometimes score well, but they are not functions a developer
   would recognize as lift targets.

3. **How should the corpus harness handle step-numbering misalignment?**
   Matching by function name rather than step index would reclassify
   13 acceptable divergences as exact matches, bringing the total to
   29/71 exact.

## Iteration history

| Iteration | Fix | Exact | Acceptable | Disagree |
|---|---|---|---|---|
| 1 | Initial implementation | 12 | 56 | 3 |
| 2 | Surface-first ranking | 13 | 55 | 3 |
| 3 | Receiver exclusion from boundary data | 15 | 55 | 1 |
| 4 | Project-locality filter | 16 | 51 | 4 |
| — | + function-name matches | 29 | 38 | 4 |
