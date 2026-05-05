# Open Questions

## Weighting

Default priority ordering from the 72-trace corpus:

1. Boundary data gates: reject `Infeasible`; mark `Proxy-required` as a separate transport class.
2. Callback frequency: prefer zero-callback cuts before comparing softer dimensions.
3. State reconstruction: prefer `Stateless`, then `Config-only`, then `Client-reconstructible`, then `Shared-state`.
4. Extraction surface: prefer the latest feasible cut unless it breaks queue/hook semantics.
5. Error semantics: prefer existing error-returning contracts; synthesize wrappers for boolean/string/localized returns.
6. Edge alignment: use strong edges as evidence, not as an override for bad boundary data.

Boundary data most often acts as the decisive tiebreaker, especially in Caddy middleware and HTTP handler paths. A single composite score is not sufficient because hard gates, proxy classes, and semantic-preservation choices are categorical. The compiler should use a decision tree: gate first, classify proxy vs. ordinary RPC, prefer zero callbacks, then rank state/surface/error/edge tradeoffs.

## Composite Cuts

Composite cuts are useful when two adjacent functions together preserve a semantic contract that either single cut weakens. The strongest examples are:

| Trace family | Composite candidate | Why it beats single-node alternatives |
|---|---|---|
| `caddy/M-3` | `HTTPBasicAuth.Authenticate` + `correctPassword` with a synthesized verification-only entry | Keeps the authentication provider contract while avoiding `http.ResponseWriter` at the external boundary. |
| `gitea/M-1`, `M-2`, `M-10`, `M-15` | queue handler + first service leaf | Preserves retry/batch semantics and keeps queue runtime out of the extracted unit. |
| `caddy/M-1`, `caddy/M-2` | template context buffer execution + specific template function | Keeps template helper registration local while extracting deterministic rendering work. |
| `pocketbase/M-7`, `pocketbase/M-9` | hook-dispatched method + concrete continuation target | Avoids sending continuation callbacks over the network while preserving hook semantics inside the monolith. |

Composite cuts are not the majority case, but they are common enough to matter: roughly 10-15 traces have a better engineering shape if the compiler extracts a short contiguous sub-path rather than exactly one node. Correlated edge sequences are `function-value-in-struct-field -> direct-function-call`, `interface-method-dispatch -> direct-function-call`, and hook/closure callback dispatch followed by a concrete leaf.

## Feasibility Gates vs. Scoring

Hard gates:

- Function values and continuation callbacks crossing the boundary.
- Closures with captured app/request state that must be serialized.
- Mutexes, wait groups, process managers, cancellation functions, and runtime lifecycle handles.
- Function factories where the returned function is the boundary value.

Soft proxy scores:

- `http.ResponseWriter`, response recorders, and request bodies.
- `io.Reader`/`io.Writer`, archive streams, filesystem handles, multipart streams.
- Channels and queue runtime objects when the channel itself, not just the payload, crosses.

Ordinary scoring inputs:

- `context.Context`, primitives, strings, byte slices, exported/domain structs, slices/maps of serializable values.
- DB, Git, HTTP, mailer, S3, filesystem, and indexer clients that can be reconstructed from config.

The proposed compiler model is two-tiered: first apply hard gates and proxy classification, then score remaining candidates. Proxy-required candidates should remain selectable only when the target shape is inherently streaming or middleware-based.

## Path-Local vs. Graph-Global

No region root appears as the target of multiple traces in this 72-trace corpus; every `region_root` value is unique. The corpus therefore does not provide direct evidence that the optimal cut differs across multiple activation paths to the same target.

Even so, the Gitea and Miniflux queue families show why graph-global reasoning will matter in larger corpora: a single worker handler or queue runtime can dominate several targets. The compiler should keep the path-local recommendation as the first pass, then merge recommendations by common target, common handler, and common reconstructed state before emitting an extraction plan.

## Integration With Liftability

The visible ADR-0018 properties align strongly with cut placement:

- `boundary.no-streaming-values` predicts the Caddy middleware and HTTP handler cases where `http.ResponseWriter`, response recorders, request bodies, or stream writers force `Feasible-with-proxy` instead of ordinary RPC.
- `contract.error-last` aligns with the 49 region roots that already return `error` or an error-like wrapper. These cuts are easier because network failure can be expressed without changing broad caller control flow.
- The absence of `contract.error-last` does not always contradict liftability: password/hash helpers and boolean validators can be lifted with generated wrappers, but the wrapper becomes part of the required boundary contract.
- State-class evidence from ADR-0016/ADR-0022 complements ADR-0018: Mattermost `App` receivers remain cut-placement risks even when the boundary data is serializable.

Cut placement should consume liftability facts rather than duplicate them. The placement phase can use ADR-0018 gates to remove candidates, then use the remaining dimensions to choose where along the activation path the admitted boundary should sit.
