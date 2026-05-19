# Boundary Adapter Strategy

## Purpose

This note sketches a general compiler strategy for **boundary normalization
via adapters**: when the chosen semantic-lift target is the right unit of
work but its source signature does not present a clean network boundary,
the compiler may synthesize a local wrapper plus a normalized remote
helper rather than abandoning the cut or moving it to a broader parent.

The strategy preserves application semantics, keeps the app-facing call
synchronous, and avoids forcing the cut to climb to a wider boundary. It
is distinct from turning the application into an async media-processing
workflow: background processing could be a sound manual redesign, but it
changes observable timing and failure behavior. The compiler path keeps
the existing synchronous call/return contract.

The note has two parts. Part 1 develops the strategy abstractly. Part 2
applies it to `listmonk/M-4` (`processImage`) as a worked example.

---

## Part 1 — Strategy

### The Cut-Placement Tradeoff

Cut placement is not one-dimensional. Without an adapter pass, the
compiler often sees a false choice:

- Move the cut shallower until the signature looks more like an
  application or framework boundary, but pay for more shared state, more
  side effects, and a larger extracted surface.
- Move the cut deeper toward the semantic unit, but hit source-level
  types that were not designed as network payloads (HTTP handles, reader
  wrappers, callback closures).

Abstractly, four cut placements typically compete:

| Candidate | Benefit | Cost |
|---|---|---|
| Broader parent (request handler, framework edge) | Natural boundary; signature already exchanges plain data. | Pulls in app/server state, side effects, and framework context. |
| Semantic unit (as written) | Correct unit of computation; minimal app state. | Source signature carries awkward types (file handles, readers, multi-result returns). |
| Adapted semantic unit | Keeps the semantic unit; finite serialized boundary. | Requires static proof plus generated host wrapper, DTO, and body normalization. |
| Library internals below the unit | Some signatures simpler locally. | Cut stops matching the application concept; may drift into third-party code. |

With adapters, a new sweet spot emerges: cut at the semantic unit and
normalize the boundary shape, rather than climbing to a parent that is
only attractive because its signature is easier to admit.
`AdapterPossible` adds compiler complexity and proof obligations, but it
can dominate both the shallow direct boundary and the deep awkward source
signature.

The ranking rule should therefore be:

1. Prefer the smallest semantic unit that preserves the original app
   behavior.
2. If that unit is directly liftable, use the existing direct-boundary
   path.
3. If it is not directly liftable, try boundary normalization before
   moving the cut to a broader parent.
4. Move to a broader parent only when the adapter proof fails, the
   adapter would need live-proxy semantics, or the parent is the intended
   semantic unit.

### Phase Ordering vs Ranking

The rules above can be operationalized two ways, and it is not yet clear
which is better:

- **As fallback (phase ordering).** Primary cut placement runs first.
  The adapter pass fires only when admission refuses the preferred
  semantic unit. Its output, when it succeeds, is preferred to the
  broader-parent alternative the primary pass would otherwise have
  chosen.
- **As ranking (cost comparison).** Adapter-normalized candidates
  compete with direct candidates in a single ordering. The compiler
  considers all options together and picks the best under a unified
  cost model.

**Default to the fallback framing for now.** It is simpler, runs the
more expensive proof obligations only when the primary pass cannot
place a good cut, and is easier to explain ("adapters are a recovery
mechanism, not a parallel option"). The known risk is missing cases
where a *feasible* but clearly suboptimal direct cut should lose to an
adapted semantic unit — under fallback, the direct cut wins simply
because it was admissible, and the adapter pass never fires. This
scenario is currently hypothetical; we do not have a cost model for
comparing the two kinds of cuts, and we have not yet run the pass on
enough cases to know how often it appears. Revisit once there is
evidence.

### Boundary Classes

Replace the existing two-axis classification (proxy-required vs
reconstructible) with five distinct boundary classes:

| Class | Meaning | Example |
|---|---|---|
| `DirectBoundary` | Source signature is already serializable/reconstructible enough for codegen. | `Hash(password, salt []byte) ([]byte, error)` |
| `AdapterPossible` | Compiler can synthesize a local wrapper plus normalized remote payloads. | `*multipart.FileHeader -> []byte`, `[]byte -> *bytes.Reader` |
| `AdapterUnknown` | A boundary might be adaptable, but the compiler has not proved the required transforms. | Custom stream wrapper with unrecognized methods. |
| `LiveProxyRequired` | Remote code would need to interact with a host-owned live object. **Refuse** — this is a reason to look for another cut, not a deployment mode. | `http.ResponseWriter`, callbacks, channel protocols. |
| `AdapterImpossible` | Static proof fails or preserving semantics would require changing the app. | Mutable write-back object with aliasing; transaction closure. |

`AdapterPossible` is not "proxy-required". The adapter is local code
that marshals finite values before and after the RPC. A live proxy is a
runtime protocol between extracted code and the monolith; this strategy
never ships one.

### Static Feasibility Obligations

Before classifying a cut as `AdapterPossible`, an adapter pass must
prove:

1. **Finite input extraction.** Each awkward input value can be converted
   to a bounded, serializable value before the remote call.
2. **Local-only lifecycle.** Temporary resources, close operations,
   request context, and cleanup remain on the host side. The remote
   function receives no handle whose lifetime it must preserve.
3. **Use-shape compatibility.** The target body uses each awkward value
   only through operations the adapter models — typically a single
   open + read, with no mutation, no reflection, no filename/header
   dependence in the extracted body.
4. **Return rehydration.** Awkward return values can be rebuilt locally
   from finite values returned by the remote helper.
5. **Error-order preservation.** Errors introduced by host-side
   extraction occur on the same side of the cut as in the original code,
   or are equivalent to existing helper errors. Errors moved from inside
   helper compute to host-side extraction are acceptable only if
   observable ordering is preserved.
6. **Call-site compatibility.** The wrapper renders under the original
   function name, so ordinary call sites are unchanged by construction.
   The pass must additionally verify that the function is not passed as
   a function value, taken by address, or accessed reflectively in a way
   that would observe a changed implementation.

Proofs run on the cut-candidate's SSA plus its callers along the
activation path; no whole-program analysis is required except for
obligation 6.

If any proof fails, the pass downgrades to `AdapterUnknown`,
`LiveProxyRequired`, or `AdapterImpossible` rather than selecting a
broad parent by default.

### Compiler Pipeline Placement

Add a boundary-normalization pass between cut selection/admission and
codegen:

```text
activation path
  -> cut candidate ranking
  -> boundary data classification
  -> boundary normalization
       DirectBoundary       -> existing codegen path
       AdapterPossible      -> synthesize adapter plan, then codegen normalized target
       LiveProxyRequired    -> refuse; ranking should choose a different cut
       AdapterImpossible    -> refuse
       AdapterUnknown       -> refuse, or keep searching for another cut
  -> codegen admission
  -> generated host patch + extracted service
```

The adapter plan is explicit IR, not an implicit special case buried in
rendering:

```text
AdapterPlan
  SourceFunction:   <name>
  HostSignature:    <original signature, preserved>
  RemoteSignature:  <normalized signature>
  InputTransforms:  per-argument adapter pattern
  BodyRewrite:      mapping from awkward-typed operations to finite-input equivalents
  OutputTransforms: per-return adapter pattern
  Proofs:           the obligations above, named and individually checked
```

This lets admission explain why a cut is accepted or refused, and gives
e2e tests a stable artifact to inspect.

### Pattern Library

The pass recognizes a small library of input/output adapters. Each
pattern carries its own proof matcher and refusal reasons. Do not
generalize to arbitrary method-call traces.

| Pattern | Input/Output Shape | Purpose |
|---|---|---|
| `multipart_file_read_all` | `*multipart.FileHeader -> []byte` | Drain an HTTP-upload handle into a finite payload host-side. |
| `reader_read_all` | `io.Reader` / `io.ReadCloser` input -> `[]byte` | Drain a bounded reader host-side. |
| `bytes_reader_return` | `[]byte -> *bytes.Reader` return | Rehydrate a reader from finite remote bytes. |

A related concern — wrapping multi-value returns into a single DTO
(`A, B, ..., error -> struct{...}, error`) — is a generic codegen
normalization that should run for every boundary, not gate behind an
`AdapterPossible` proof. It belongs in the codegen support matrix, not
in this library.

### Refusal Classes

The pass refuses, or classifies as `LiveProxyRequired`, when the awkward
value is more than a finite payload wrapper:

- `http.ResponseWriter`: remote code writes headers/body with ordering
  and lifecycle tied to the active request.
- `io.Writer` output parameters: remote code would stream back to a
  host-owned sink unless the entire output can be captured as a value.
- Channels: send/receive order and goroutine scheduling are part of the
  semantics.
- Transaction closures: callback execution owns database transaction
  lifetime.
- Mutable write-back objects with aliasing: local aliases may observe
  mutation order.

### Implementation Sketch

1. Add `AdapterClass` and `AdapterPlan` to the cut/admission model.
2. Teach boundary classification to return `AdapterUnknown` rather than
   forcing all non-direct shapes into infeasible/proxy buckets.
3. Implement individual patterns against SSA signatures and function
   bodies; each pattern owns its proof matcher.
4. Extend admission so `AdapterPossible` candidates pass when every
   required transform has a codegen implementation.
5. Render the host wrapper under the original function name; render the
   normalized helper as the extracted target.

### Open Questions

- How large can a bounded reader payload be before the adapter should
  require a staging object rather than inline JSON/base64? Transport
  choice probably belongs in `AdapterPlan`, not in codegen.
- Should body rewriting be source-to-source, SSA-to-source, or
  implemented by cloning the function and replacing recognized
  operations? Leaning toward pattern-matched prologue replacement rather
  than a general rewrite, to keep the pass from sliding into a
  refactoring compiler.
- Should `AdapterPossible` candidates outrank broader direct boundaries
  even when the broader boundary is also semantically meaningful, or
  should the cut strategy require an explicit semantic-target
  preference?
- Phase ordering vs ranking (see above): the current default runs the
  adapter pass as a strict fallback. Resolving whether to move to a
  unified ranking needs a cost model for comparing direct cuts against
  adapter-normalized cuts, plus enough empirical runs to see whether
  fallback systematically misses good adapter-enabled cuts.
- How should transcript comparison represent DTO-normalized returns
  when the app-facing return shape is unchanged?

---

## Part 2 — Applied to `listmonk/M-4` (`processImage`)

### Why this case motivates the framework

`processImage` is a strong lift candidate semantically:

- CPU-bound: image decode, Lanczos resize, PNG encode.
- Side-effect free aside from reading the uploaded bytes.
- Output is finite data: thumbnail PNG bytes plus original dimensions.
- The caller immediately writes the returned reader to the media store.

The source shape is awkward:

- Input is `*multipart.FileHeader`, an HTTP upload handle whose useful
  content is accessed through `Open()`.
- Output is `*bytes.Reader`, chosen because Listmonk's media store
  accepts `io.ReadSeeker`.
- The raw result shape exceeds the currently supported `(T, error)` HTTP
  generator contract.

The existing cut table in `analyses/listmonk-M-4.md` recorded the deep
cut as proxy-shaped, and the corpus row remains skipped rather than
proved end-to-end. Treating this as "cut at `(*App).UploadMedia`
instead" pulls in app/media-store state and loses the clean semantic
target.

Cut placements compared:

| Candidate | Benefit | Cost |
|---|---|---|
| `(*App).UploadMedia` | Natural request-handler edge; caller already has filenames, content type, media store, cleanup policy. | Pulls in `*App`, media store, core DB calls, config, i18n/logging, cleanup side effects, and HTTP context reconstruction. |
| `processImage` as written | Correct semantic unit. Minimal app state. | Signature has `*multipart.FileHeader`, `*bytes.Reader`, and a multi-value return shape not admitted by current codegen. |
| Generated `processImageBytes` adapter | Keeps the semantic unit and avoids app/media-store state. Boundary is finite bytes and scalar metadata. | Requires static proof plus generated host wrapper, DTO, and body normalization. |
| Lower library internals | Some signatures may be simpler locally. | Cut stops matching the application-level concept, loses width/height/output packaging context, drifts into third-party library code. |

The adapted semantic unit is the right answer.

### Boundary classification

Classify the `processImage` cut as `AdapterPossible`, satisfying the six
obligations:

1. **Finite input extraction.** `*multipart.FileHeader` drains to
   `[]byte` via `Open() + io.ReadAll`.
2. **Local-only lifecycle.** `Open()`/`Close()`, temp files, and request
   context stay host-side.
3. **Use-shape compatibility.** The helper opens the file once, passes
   it to `imaging.Decode`, and never references filename or header
   fields.
4. **Return rehydration.** `[]byte` rehydrates to `*bytes.Reader` via
   `bytes.NewReader`. `int` returns pass through.
5. **Error-order preservation.** `file.Open` and `io.ReadAll` errors
   remain host-side before the remote call. `imaging.Decode`, `Resize`,
   and `Encode` errors remain helper errors. Minor divergence: read
   errors previously surfaced inside `Decode`; they now surface from
   `ReadAll`. Same side of the cut, slightly earlier logically.
6. **Call-site compatibility.** The single call site in `cmd/media.go`
   calls `processImage` directly; no function-value or reflective use.

### Normalized helper and host wrapper

Original helper:

```go
func processImage(file *multipart.FileHeader) (*bytes.Reader, int, int, error) {
    src, err := file.Open()
    if err != nil {
        return nil, 0, 0, err
    }
    defer src.Close()

    img, err := imaging.Decode(src)
    if err != nil {
        return nil, 0, 0, err
    }

    var out bytes.Buffer
    thumb := imaging.Resize(img, thumbnailSize, 0, imaging.Lanczos)
    if err := imaging.Encode(&out, thumb, imaging.PNG); err != nil {
        return nil, 0, 0, err
    }

    b := img.Bounds().Max
    return bytes.NewReader(out.Bytes()), b.X, b.Y, nil
}
```

Normalized remote boundary:

```go
type processImageResult struct {
    Thumbnail      []byte `json:"thumbnail"`
    OriginalWidth  int    `json:"original_width"`
    OriginalHeight int    `json:"original_height"`
}

func processImageBytes(input []byte) (processImageResult, error) {
    img, err := imaging.Decode(bytes.NewReader(input))
    if err != nil {
        return processImageResult{}, err
    }

    var out bytes.Buffer
    thumb := imaging.Resize(img, thumbnailSize, 0, imaging.Lanczos)
    if err := imaging.Encode(&out, thumb, imaging.PNG); err != nil {
        return processImageResult{}, err
    }

    b := img.Bounds().Max
    return processImageResult{
        Thumbnail:      out.Bytes(),
        OriginalWidth:  b.X,
        OriginalHeight: b.Y,
    }, nil
}
```

Generated host-side wrapper:

```go
func processImage(file *multipart.FileHeader) (*bytes.Reader, int, int, error) {
    src, err := file.Open()
    if err != nil {
        return nil, 0, 0, err
    }
    defer src.Close()

    input, err := io.ReadAll(src)
    if err != nil {
        return nil, 0, 0, err
    }

    out, err := monoliftProcessImageBytes(input)
    if err != nil {
        return nil, 0, 0, err
    }
    return bytes.NewReader(out.Thumbnail), out.OriginalWidth, out.OriginalHeight, nil
}
```

The caller still sees the same signature and writes the returned reader
to the same media store. The remote service only sees finite bytes and
returns finite bytes plus scalar metadata.

The original computes width/height from the *source* image
(`img.Bounds()` before resize), not the thumbnail. The normalized field
names make this explicit so the adapter does not appear to silently
change semantics.

### AdapterPlan instance

```text
AdapterPlan
  SourceFunction:  processImage
  HostSignature:   *multipart.FileHeader -> *bytes.Reader, int, int, error
  RemoteSignature: []byte -> processImageResult, error
  InputTransforms:
    file: multipart_file_read_all
  BodyRewrite:
    file.Open + decode(src) -> decode(bytes.NewReader(input))
  OutputTransforms:
    processImageResult.Thumbnail      -> bytes.NewReader(...)
    processImageResult.OriginalWidth  -> int
    processImageResult.OriginalHeight -> int
  Proofs:
    finite_input
    local_lifecycle
    read_only_use_shape
    return_rehydration
    error_order_preserved
    call_site_compatibility
```

### Focused e2e target

Add an e2e target for `listmonk/M-4` that uploads an image, asserts the
thumbnail object is written, checks original-width/height metadata
against the fixture, and verifies the extracted-service call count.

### Success criteria

For Listmonk M-4, success means:

- The selected semantic target remains image thumbnail generation, not
  `(*App).UploadMedia`.
- The source-facing function still behaves synchronously and returns
  `(*bytes.Reader, int, int, error)`.
- The extracted service receives only finite serialized values.
- The thumbnail written through `a.media.Put` is byte-equivalent to the
  local helper's output for the test fixture.
- Failure behavior remains on the same side of the cut as before:
  `file.Open` and the new `io.ReadAll` extraction remain local; `Decode`,
  `Resize`, `Encode` remain remote helper errors.
