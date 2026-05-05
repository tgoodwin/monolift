# Corpus Statistics

## Surface-Area Analysis

Depth was normalized as `recommended step / region-root step`; when a trace's declared `path_length` differed from the largest step number, the largest step number was used as the denominator so the region root remains `1.0`.

| Project | Reachable traces | Mean depth | Median depth | Traces at depth >= 0.75 | Recommended surface classes |
|---|---:|---:|---:|---:|---|
| Caddy | 6 | 1.000 | 1.000 | 6 | 6 Minimal |
| Gitea | 18 | 0.885 | 0.906 | 15 | 6 Minimal, 9 Small, 3 Medium |
| Listmonk | 10 | 0.860 | 0.875 | 9 | 5 Minimal, 4 Small, 1 Medium |
| Mattermost | 14 | 0.887 | 1.000 | 12 | 8 Minimal, 4 Small, 2 Medium |
| Miniflux | 12 | 0.988 | 1.000 | 12 | 10 Minimal, 2 Small |
| PocketBase | 11 | 0.983 | 1.000 | 11 | 9 Minimal, 2 Small |

Across the 71 reachable traces, recommended cuts are deep: mean depth is `0.924`, median depth is `1.0`, and 65/71 recommendations occur at depth `>= 0.75`. Deep cuts dominate because request/framework/bootstrap code tends to carry live handlers, app receivers, or queue runtimes.

The candidate-row surface bands show the practical jump points: `Small` candidates cluster around relative depth `0.71-1.0` with median `0.83`, while `Large` candidates cluster around `0.12-0.69` with median `0.33`. In this corpus, surface area usually stops being `Large` only after the path passes roughly the halfway point; codebase size raises the cost of shallow cuts, but architecture predicts the threshold better than LOC alone.

## Boundary-Data Feasibility Map

Candidate boundary-data counts across reachable traces:

| Boundary data class | Candidate rows | Recommended cuts |
|---|---:|---:|
| Trivial | 121 | 19 |
| Serializable | 219 | 29 |
| Reconstructible | 119 | 19 |
| Proxy-required | 122 | 4 |
| Infeasible | 16 | 0 |

The shallowest feasible non-proxy cut exists for 70/71 reachable traces; the only exception is the structural `mattermost/M-4` gap. The median shallowest feasible depth is `0.143`, because many bootstrap functions have simple argument lists even though their surface and state costs are terrible. There is therefore no universal type-only feasibility cliff. The real cliff is architectural: request/response, queue runtime, and callback objects usually disappear only after the handler or worker has converted them into domain values.

Proxy-required candidates are concentrated around interface dispatch, goroutine launch, direct/concrete calls, and handler registration. Source inspection shows the recurring proxy sources are `http.ResponseWriter`/response recorders in HTTP middleware, `io.Reader`/`io.Writer` and filesystem handles in archive/upload paths, and channels or queue runtime objects in worker paths. Function-valued callbacks account for most hard `Infeasible` rows.

## State-Reconstruction Clustering

Recommended cuts by state class:

| Project | Stateless | Config-only | Client-reconstructible | Shared-state |
|---|---:|---:|---:|---:|
| Caddy | 1 | 3 | 0 | 2 |
| Gitea | 1 | 1 | 16 | 0 |
| Listmonk | 0 | 2 | 7 | 1 |
| Mattermost | 1 | 0 | 6 | 7 |
| Miniflux | 2 | 0 | 10 | 0 |
| PocketBase | 2 | 5 | 4 | 0 |
| **Total** | **7** | **11** | **43** | **10** |

Architecture predicts state class strongly. Gitea and Miniflux cluster around `Client-reconstructible` because the recommended cuts are service, queue-handler, DB, Git, HTTP, or feed-processing functions. Mattermost is the outlier: its `App` receiver keeps many recommended cuts in `Shared-state`. PocketBase splits between config-only helpers and client-reconstructible app services, while Caddy splits between pure/configured leaves and HTTP middleware cuts that still carry shared server state.

## Callback Prevalence

Every reachable trace has at least one zero-callback candidate. Recommended-cut callback counts are:

| Callback class | Recommended cuts |
|---|---:|
| `0 (confirmed)` | 44 |
| `0 (estimated)` | 8 |
| `Low` | 19 |
| `Moderate` / `Many` | 0 |

No trace has unavoidable callbacks under the path-local analysis. Callback risk is nevertheless common in candidate rows: it clusters around `function-value-in-struct-field`, `interface-method-dispatch`, `goroutine-launch`, and `http-handler-registration` edges. The recommended cuts usually move below those dispatch shells, leaving callbacks only when the selected cut intentionally preserves queue, handler, or hook semantics.

## Edge-Type Alignment Statistics

Boundary-affinity ratio is `recommended occurrences / total occurrences`.

| Edge type | Total occurrences | Recommended occurrences | Affinity |
|---|---:|---:|---:|
| `http-handler-registration` | 19 | 6 | 0.316 |
| `interface-method-dispatch` | 44 | 9 | 0.205 |
| `function-value-in-struct-field` | 70 | 10 | 0.143 |
| `method-call-on-concrete-type` | 105 | 15 | 0.143 |
| `direct-function-call` | 239 | 18 | 0.075 |
| `goroutine-launch` | 23 | 1 | 0.043 |
| `channel-send-receive` | 6 | 0 | 0.000 |

The Strong/Weak/Anti classification mostly holds, but with an important correction: direct and concrete calls are often selected when they are deep leaf functions with trivial data. Strong edges are disproportionately selected when they are also late and domain-shaped, especially HTTP handler registrations and interface dispatches. Goroutine launches are strongly disfavored. Channel edges had zero recommendations because the adjacent post-receive function was usually a smaller equivalent cut.

## Error-Semantics Survey

Among the 71 reachable region roots, 49 return `error` or a project-specific error-like value that the caller already handles. The remaining 22 are non-error-returning roots, usually boolean/hash helpers, void goroutine work, or framework callbacks that report failure through side effects.

For non-error-returning roots with an error-capable ancestor, the nearest error-handling ancestor is usually close:

| Distance to nearest error-capable ancestor | Trace count |
|---:|---:|
| 1 | 7 |
| 2 | 7 |
| 5 | 3 |
| 6 | 2 |
| 7 | 1 |
| 9 | 1 |

Error-semantics tension exists in 22 reachable traces, but it is decisive in fewer cases than boundary data or state. The common mitigation is a generated wrapper that maps network failure into the caller's existing localized error, boolean failure, or retry path.

## Pareto Frontier Characterization

Using the rubric, 43/71 reachable traces have a clearly dominant recommendation: a `Minimal` or `Small` feasible cut with zero callbacks and no proxy-required boundary data. The remaining 28 reachable traces have genuine tradeoffs.

The tradeoff axes are not mutually exclusive, but they cluster as follows:

| Tension axis | Typical traces | Character |
|---|---|---|
| Surface vs. state | Mattermost `App` methods, Gitea service workers, Miniflux feed processing | Deep cut avoids framework surface but requires DB/app/client reconstruction. |
| Boundary data vs. edge alignment | Caddy middleware, HTTP handlers, PocketBase hooks | Strong edge exists higher up, but request/writer/event objects make the natural boundary expensive. |
| Queue semantics vs. leaf purity | Gitea webhook/archive/push/indexer queues | Handler cut preserves retry/batch semantics; leaf cut is smaller but loses queue contract. |
| Error wrapper vs. pure leaf | password/hash/render helpers | Leaf is tiny but network failure must be mapped into boolean/string/localized-return contracts. |

Genuine Pareto tensions are common enough that a single weighted sum would be brittle. The compiler should treat hard feasibility gates first, then use a decision tree that can choose a queue handler over a leaf when preserving batch semantics is more important than absolute minimal surface.

## Cut-Point Archetypes

| Archetype | Status | Representative traces | Typical dimension profile |
|---|---|---|---|
| Pure Leaf | Confirmed | `caddy/M-1`, `caddy/M-3`, `gitea/M-16`, `miniflux/M-3`, `pocketbase/M-3` | Minimal surface, trivial/serializable data, stateless/config-only state, zero callbacks, often weak/anti edge. |
| Interface Contract | Confirmed/refines Interface Proxy | `caddy/M-4`, `gitea/M-16`, `miniflux/M-14`, `pocketbase/M-10` | Strong interface edge, serializable/reconstructible data, zero callbacks; the interface supplies a replacement contract. |
| Queue Handler | Confirmed/refines Queue Worker | `gitea/M-1`, `gitea/M-2`, `gitea/M-10`, `gitea/M-12`, `gitea/M-15` | Small handler surface, serializable queue payloads, client-reconstructible DB/Git/indexer state, low callbacks for retry semantics. |
| HTTP/Request Shell Escape | Confirmed/refines Middleware Split | `caddy/M-2`, `caddy/M-5`, `listmonk/M-5`, `gitea/M-19` | Strong handler or middleware edge exists, but the recommended cut either accepts a proxy or moves below request/writer state. |
| Framework Callback | Confirmed | `gitea/M-13`, `listmonk/M-4`, `pocketbase/M-7`, `pocketbase/M-9` | Callback registration proves reachability, but the recommended cut avoids sending continuation functions across the boundary. |
| Shared-State App Receiver | Confirmed | `mattermost/M-6`, `mattermost/M-7`, `mattermost/M-8`, `mattermost/M-12`, `mattermost/M-13` | Deep cut still carries app/server state; feasibility depends on reconstructing or coordinating service state. |

The hypothesized `Middleware Split` is real in Caddy, but across the corpus the broader pattern is escaping the HTTP/request shell before cutting. The hypothesized `Queue Worker` is also real, but the best cut is usually the registered handler rather than the queue runtime itself.

## Anti-Boundary Catalog

| Anti-boundary | Corpus count | Representative failure cases | Why it fails |
|---|---:|---|---|
| Raw goroutine launch | 23 | `miniflux/M-1` step 3, `gitea/M-1` step 7, `mattermost/M-8` step 6 | Launch edges capture scheduler/lifecycle state and do not define a request/response contract. Prefer the next named worker function or payload handler. |
| Closure capture across framework state | 10+ closure-capture variants | `caddy/M-3` steps 7-8, `pocketbase/M-1` step 10, `gitea/M-1` step 12 when treated as captured field rather than handler | Captured values are implicit boundary data; if the captured value is a handler/app/event, callbacks or proxies follow. |
| Function factory / function value as data | 16 infeasible boundary rows | `gitea/M-11` `getIssueIndexerQueueHandler`, `pocketbase/M-8` `Uploader.Upload` opt callbacks | Function values cannot cross as ordinary data. Cut at the created handler invocation or below the callback-bearing API. |
| Shallow direct bootstrap calls | 239 direct-call occurrences, only 18 selected | `RunMainApp`, `InitWebInstalled`, `runServer`, `Serve` bootstrap paths | Direct calls are only good at the leaf. Near the top they imply huge surface and shared process state. |
| Request/writer middleware shell | 122 proxy-required candidate rows | `caddy/M-5`, `caddy/M-2`, Caddy route wrappers | The handler abstraction is natural but carries live HTTP streams; it is a proxy boundary, not a serializable RPC boundary. |

These are not syntax bans. A direct call or goroutine-launched named function can be selected when the source signature is clean and the launch/call edge itself is not the semantic boundary. The anti-catalog applies when the edge's runtime mechanism must cross the network.

## Falsifiable Pattern Claims

| Claim | Verdict | Evidence |
|---|---|---|
| Interface-dispatch edges at depth >3 always have <=Medium surface and zero callbacks | False | 48 such candidate rows exist; 6 still score `Large`, and 36 have nonzero callback risk because middleware, hooks, or app interfaces remain involved. |
| Goroutine-launch boundaries are anti-boundaries: always prefer the next deeper non-goroutine cut | Mostly true | Raw goroutine launches are never preferred. Two recommendations are launched named functions (`miniflux/M-4`, `miniflux/M-9`), where the selected function has a clean signature and the launch mechanism is not the boundary contract. |
| HTTP-handler-registration edges are strong natural boundaries despite shallow depth | Partly true | 6/19 HTTP-handler-registration edges are recommended. They work when request state has already been shaped into domain inputs; they are poor when `ResponseWriter` or request event state would cross. |
| Reconstructible-state traces cluster around database-client patterns | True | 43 recommendations are `Client-reconstructible`, heavily concentrated in Gitea and Miniflux DB/Git/feed/service paths. |
| A deep cut with trivial boundary types beats a shallow interface cut unless callbacks force the shallow cut | Mostly true | Pure leaves dominate, but queue retry semantics and hook continuation semantics can also justify a shallower handler cut even when callbacks are only `Low`. |
| Zero-callback cuts exist for >=90% of traces | True | 71/71 reachable traces have at least one zero-callback candidate; 52/71 recommendations are zero-callback cuts. |
