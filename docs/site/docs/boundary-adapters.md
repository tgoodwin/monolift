# Adapting the network boundary

## Research question and result

The [previous page](cut-placement.md) treated cut placement as a search
over functions that already exist on the activation path: rank the
candidates, pick the best one, and check whether the compiler can render
it. But that search can come up empty.
A region can be the right unit of computation to run remotely and still
have no function anywhere on its path whose *signature* can support a clean
network boundary (serializable, pass-by-value call semantics). The semantic unit is correct; its shape is wrong.

This page describes the compiler's second move when that happens: _boundary adapter_ synthesis.
Rather than abandoning the lift effort, or retreating to a broader, costlier
position on the activation path (which would mean extracting a larger region of code), Monolift can **synthesize a new adapter function to provide the boundary**. It generates a local wrapper on the dataflow path that converts unserializable parameter types
into finite, wire-friendly values (i.e. dereferencing pointer types into a payload of bytes), packages these parameters and return values into data-transfer objects (DTOs), and ships only those DTOs across the network boundary.

Boundary adapter synthesis is currently designed as a **fallback** mechanism. It fires only after the
direct placement search has refused the preferred cut for a
shape-related reason. The deeper question of *whether to adapt the
semantic unit here or cut directly somewhere else* lives in a
multi-objective space the compiler does not yet optimize over — a 
future research direction.

## When a to-be lifted code region operates on unliftable types

The cut-placement ranker does a hard classification pass on whether each candidate's call params
data can cross a network. If the params aren't serializable, the candidate is rejected and the
ranker looks elsewhere on the path. Usually that works: a cleaner
boundary sits a step or two away in the call graph. Sometimes it does not, and the
compiler faces a tradeoff with no good *direct* answer.

| Candidate | Benefit | Cost |
|---|---|---|
| Broader parent (request handler, framework edge) | Signature already exchanges plain data; a natural boundary. | Pulls in application and server state, side effects, and framework context. |
| The semantic unit, as written | The correct unit of work; minimal application state. | Source signature carries awkward types — file handles, reader wrappers, multi-value returns. |
| Library internals below the unit | Some signatures are simpler locally. | The cut stops matching the application-level concept and drifts into third-party code. |

Each direct option trades one cost for another. The adapter pass adds a
fourth choice the source does not offer directly: keep the cut at the
right unit and *manufacture* the clean boundary around it. This is the
same question the cut-placement page asked — *where should the network
boundary go?* — pushed one step further. If no existing function is a
good boundary, can the compiler build one that is?

## What's a boundary adapter? 

A boundary adapter is a pair of generated functions that sit on either side of a selected cut point:

- A **host wrapper**, rendered under the original function's name, so
  every call site is unchanged by construction. It runs in the
  monolith. It drains each awkward input param into a value (to facilitate pass-by-value semantics), calls the
  remote boundary, and rehydrates the network response into the original return types on the way back.
- A **normalized remote helper**, which is the same business logic with
  its signature rewritten to finite, serializable types. This is the
  function that actually performs the network call.

The application-facing contract is preserved transparently: the call stays
synchronous, the signature is identical, and failures surface where they
did before.

That last point depends on a precondition. A remote call can fail where a
local call could not — the service may be unreachable or time out — so the
adapter needs a place to report that failure. The place is the function's
own `error` return: an adapter only preserves semantics when the original
signature *already anticipates the possibility of failure*. Because the
caller always had to handle that error, it handles a network failure the
same way. A function that never returned an error has no transparent place
to report one.

What an adapter is *not* matters as much as what it is, and three
exclusions are load-bearing:

- **Not a live proxy.** The remote side never holds a handle to a
  host-owned object. It receives finite bytes and returns finite bytes.
  This is a deliberate design line: streaming and lifecycle handles at the
  boundary are a signal to cut differently, never a "feasible with a proxy"
  deployment mode. Adapters do not reopen that category — they marshal finite
  values with local wrapper code, nothing more.
- **Not a general rewrite.** The helper body is transformed by bounded,
  pattern-matched prologue replacement, not a whole-function SSA rewrite
  (SSA — static single assignment — is the compiler's internal form of the
  code). The pass is deliberately kept from sliding into a refactoring compiler.
- **Not an async redesign.** Background processing could be a sound
  manual redesign of a media pipeline, but it changes observable timing
  and failure behavior. The adapter keeps the existing synchronous
  call/return contract.

## Packaging values into DTOs

Turning an awkward signature into a finite wire format takes two
mechanisms, and they are kept separate on purpose.

**Multi-result DTO packing is generic codegen.** Any boundary whose
function returns more than the small `(T, error)` shape — say
`(A, B, error)` — needs its non-error returns packed into one JSON
object. The compiler synthesizes a `<FuncName>Result` struct, gives each
field a JSON tag, and routes the boundary through it. This runs for
*every* admitted boundary with multiple returns; it is not gated on the
adapter pass, and it belongs to the [extraction](extraction.md) codec
table rather than to adapters.

**Type normalization is the adapter pass.** Packing assumes the values
are already JSON-codable. When they are not — `*multipart.FileHeader`
in, `*bytes.Reader` out — the adapter pass converts them via named
patterns before packing. Each pattern owns two transforms: a host-side
*extraction* that drains the awkward input to a finite value, and a
host-side *rehydration* that rebuilds the awkward return from finite
bytes. The pattern library is small and explicit — `multipart_file_read_all`
(`*multipart.FileHeader → []byte`) and `bytes_reader_return`
(`[]byte → *bytes.Reader`) are the two shipped today — and new shapes are
added as registry entries, not as branches in the renderer.

Together, the parameters and return values the developer wrote become
DTOs that cross the wire, while the source-level signature stays exactly
as it was.

## A concrete example: listmonk's `processImage`

[`processImage`](https://github.com/tgoodwin/monolift/blob/main/docs/research/activation-paths/analyses/listmonk-M-4.md)
(corpus trace `listmonk/M-4`) is a strong lift candidate semantically: it
decodes an uploaded image, runs a CPU-bound Lanczos resize and PNG
encode, and is otherwise side-effect free. Its output is finite —
thumbnail bytes plus the original dimensions. But its source shape is
awkward at both ends.

<div class="code-pair" markdown="1">

<div markdown="1">
<div class="pair-caption">listmonk — <code>cmd/media.go</code> (the semantic unit, as written)</div>

```go
--8<-- "docs/site/snippets/external/listmonk/process-image-signature.go.txt"
```

The input is a `*multipart.FileHeader` — a live HTTP-upload handle whose
only useful content is reached through `Open()`. The output is a
`*bytes.Reader` (listmonk's media store wants an `io.ReadSeeker`) plus
two integers. Neither end is directly serializable, and the multi-value
return exceeds the direct generator's `(T, error)` contract.
</div>

<div markdown="1">
<div class="pair-caption">Monolift — the synthesized normalized boundary</div>

```go
// Host wrapper: rendered under the original name. Call sites unchanged.
func processImage(file *multipart.FileHeader) (*bytes.Reader, int, int, error)
// drains file → []byte, calls the remote boundary, rehydrates *bytes.Reader

// Normalized remote helper: the same logic, finite signature.
func monoliftNormalizedprocessImage(input []byte) ([]byte, int, int, error)

// Multi-result DTO: how the three non-error returns travel on the wire.
type processImageResult struct {
    Result0 []byte `json:"result0"` // thumbnail PNG bytes
    Result1 int    `json:"result1"` // original width
    Result2 int    `json:"result2"` // original height
}
```
</div>

</div>

The broader parent on this path, `(*App).UploadMedia`, is a natural
request-handler edge whose signature already exchanges plain data — but
cutting there would pull in `*App`, the media store, core DB calls,
config, and request cleanup. The adapter lets the compiler keep the cut
at `processImage`, the minimal unit, and the only state the remote helper
sees is the bytes it was handed.

A note on faithfulness: the original computes width and height from the
*source* image (`img.Bounds()` before resize), not the thumbnail. The
adapter preserves that exactly, because the helper body is the same code;
only the input and output edges are rewritten.

## An orthogonal classification axis

The compiler does not express "this needs an adapter" by overloading the
boundary-data verdict — it adds a separate axis. `BoundaryDataClass` answers
*how hard are the source values to serialize?* A second axis, `AdapterClass`,
answers *can compiler-owned wrapper code bridge the gap?* The two are never
folded into one score, and keeping them apart is what makes the recovery branch
auditable: the compiler can state both facts at once instead of hiding them in a
single verdict.

??? info "Reference: the `AdapterClass` values"

    ```go
    --8<-- "docs/site/snippets/internal/adapter-class.go.txt"
    ```

    Why the orthogonality matters in practice: a `processImage` cut is
    `Reconstructible` on the data axis (its bytes are finite) yet refuses
    *direct* codegen with `missing_reconstructor` (no registered reconstructor
    for `*multipart.FileHeader`). That pairing is not a contradiction — it is
    precisely the shape-compatible signal that the cut is a candidate for
    adaptation. The source of truth for "is this liftable" becomes
    `AdapterClass = AdapterPossible`, not the data class alone.

## A recovery branch, not a ranking

The adapter pass sits in a fixed place: it runs as a recovery branch *after*
primary admission refuses the preferred semantic cut for a shape-compatible
reason. It is fallback, not ranking. If the direct cut already admits, the
adapter pass never fires and does not compete. If the adapter proof fails, the
existing demotion chain continues to a deeper candidate.

One structural rule keeps the fallback honest: the compiler cannot quietly
settle for the broad parent (`UploadMedia`) when the real target is an
adaptable leaf below it.

??? info "Reference: the parent-forbidden rule"
    Once a deeper candidate is labeled non-`DirectBoundary`, any strict ancestor
    of it refuses with `adapter_parent_forbidden`. The predicate names no
    function and no type; it keys solely on the deeper candidate's
    `AdapterClass`, so new adapter patterns extend the forbidden set
    automatically.

This phase ordering was a deliberate choice between two designs, and the
alternative is the more interesting one:

- **Phase ordering (today).** The adapter pass is a strict fallback. It
  runs the expensive proof obligations only when the primary pass cannot
  place a good cut, and it is easy to explain — adapters are a recovery
  mechanism, not a parallel option.
- **Ranking (future work).** Adapter-normalized candidates compete with
  direct candidates in a single ordering, and the compiler picks the best
  under a unified cost model.

The known gap in the fallback framing is exactly a multi-objective
tradeoff. A *feasible but clearly suboptimal* direct cut wins simply
because it admits, and the adapter pass never fires — even when a deeper
cut would adapt cleanly into a better boundary. Choosing between "cut
here directly" and "cut there with an adapter" trades surface area,
shared state, callback pressure, and the cost of discharging the adapter
proofs against one another. The compiler has no cost model for that
comparison yet, and building one is deferred until there is corpus
evidence that fallback systematically misses good adapter-enabled cuts.
That evidence does not exist today — across the 72 reviewed traces, only
one candidate classifies as `AdapterPossible`, so there is not yet a case
where a direct cut is chosen over a clearly better adapted one.

## What the pass must prove

Before the compiler will manufacture a boundary it discharges — proves — six
named obligations, each checked as far as the implementation can verify it on
the candidate's SSA and its callers. A failure downgrades the cut to
`AdapterUnknown`, `LiveProxyRequired`, or `AdapterImpossible` rather than
silently shipping a partial solution.

??? info "Reference: the six proof obligations"
    | Obligation | What is proved |
    |---|---|
    | `adapter_finite_input` | Every awkward input parameter matches a registered pattern that declares its host-side extraction. |
    | `adapter_local_lifecycle` | The awkward value does not escape the host: no `defer` capture, `Close()`, interface boxing, or store into a package global. |
    | `adapter_use_shape` | The pattern proves the value is consumed only in the bounded shape it recognizes (e.g. a single open-and-read of the upload, no filename/header dependence). |
    | `adapter_return_rehydration` | Each awkward return can be reconstructed host-side from the finite wire value. |
    | `adapter_error_order` | Errors stay on the same side of the cut as before; host-side extraction errors are recorded, not hidden. |
    | `adapter_call_site` | A reverse-import scan over the activation-path scope confirms the function is only called directly — never taken by address, passed as a value, or reached reflectively. |

For `processImage`, `multipart_file_read_all` discharges the input,
lifecycle, and use-shape checks (the helper opens the file once and never
inspects its name or header); `bytes_reader_return` discharges
rehydration (the reader is rebuilt only via `bytes.NewReader`); and the
call-site scan confirms the single direct call from `UploadMedia`. The
one recorded divergence is benign: a file read error now surfaces from
the host-side `io.ReadAll` rather than from inside `imaging.Decode` —
still host-side, still before the RPC.

## Current state

`processImage` reaches the full end-to-end pipeline — image build, deploy,
lifted execution, output comparison — with the test oracle doing a direct
byte-for-byte comparison of the produced PNG thumbnail against the local
helper's output. The selected cut is `processImage`, not `(*App).UploadMedia`;
the structural parent rule held.

To test whether the mechanism was general or a one-off, two further lifts were
landed at the same stage — miniflux's `ExtractContent` (a streaming-bytes input
plus a multi-result DTO) and pocketbase's `S256Challenge` (a plain
single-return primitive transform) — using only the *generic* DTO and codec
machinery, with no new adapter pattern. That is the honest scope of the
evidence: the pattern library itself is exercised by a single corpus example so
far, while the generic machinery around it generalizes. Streaming/chunked
transport, a staged payload object for values above the inline size ceiling, and
the cost-model ranking discussed above all remain future work.

## Design principles

**A fallback, not a parallel option.** Adapters fire only when the direct
search refuses a good cut for a shape reason. This keeps the expensive
proof obligations off the common path and keeps the story simple:
adapters recover boundaries, they do not compete for them.

**Refuse, don't proxy.** A boundary value that is more than a finite
payload — an `http.ResponseWriter`, a channel, a transaction closure — is
`LiveProxyRequired`, which is a reason to cut elsewhere, not a deployment
mode. The remote side only ever sees finite bytes.

**Patterns own their proofs.** Each input/output pattern carries its own
match predicate, its host-side render, and its refusal reasons. The pass
discharges named obligations against SSA; it does not bake one target's
quirks into the renderer. Adding a shape is a registry entry.

**Two axes, never one score.** `AdapterClass` stays orthogonal to
`BoundaryDataClass`. The compiler can always say both *how hard the
values are to serialize* and *whether wrapper code can bridge them*,
rather than collapsing the two into an opaque verdict.

**The plan is an explicit artifact.** A boundary-normalized cut carries an
`AdapterPlan` — host signature, remote signature, per-argument transforms, the
body rewrite, and the discharged proofs — emitted as explicit data rather than
buried in rendering code, so admission can explain why a cut was accepted and
end-to-end tests have a stable artifact to inspect.

## Provenance

??? abstract "Sprint and decision-record breadcrumbs (for maintainers)"
    These pointers track how the design landed; they are not needed to follow
    the story above.

    - **Refuse, don't proxy** — finite bytes only, never a live handle to a
      host-owned object — is the line set by
      [ADR-0028 (monolith as gateway)](https://github.com/tgoodwin/monolift/blob/main/docs/decisions/0028-monolith-as-gateway.md).
    - The **recovery-branch placement** (fallback, not ranking) is fixed by
      [ADR-0032 (boundary-adapter recovery)](https://github.com/tgoodwin/monolift/blob/main/docs/decisions/0032-boundary-adapter-recovery.md).
    - `processImage` (corpus trace
      [`listmonk/M-4`](https://github.com/tgoodwin/monolift/blob/main/docs/research/activation-paths/analyses/listmonk-M-4.md))
      was the first lift to exercise the adapter pattern library end-to-end;
      SPRINT-0052 then added the `ExtractContent` and `S256Challenge` lifts to
      confirm the generic machinery generalizes beyond it.
