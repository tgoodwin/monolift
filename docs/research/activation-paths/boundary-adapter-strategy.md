# Boundary Adapter Strategy

## Purpose

This note sketches a compiler strategy for cases where the semantic lift target
is good, but the source-level function signature is not already a good network
boundary. The motivating case is Listmonk `listmonk/M-4`, where the useful unit
is image thumbnail generation in `processImage`, but the current signature uses
HTTP upload and reader wrapper types:

```go
func processImage(file *multipart.FileHeader) (*bytes.Reader, int, int, error)
```

The goal is to support `AdapterPossible -> lift with adapter`: preserve
application semantics, keep the app-facing call synchronous, and synthesize a
liftable boundary without forcing the cut to climb to a broader parent.

This is different from turning the application into an async media-processing
workflow. Background processing could be a good manual redesign, but it changes
observable timing and failure behavior. The compiler path here keeps the
existing synchronous call/return contract.

## Problem

`processImage` is a strong lift candidate:

- It is CPU-bound: image decode, Lanczos resize, PNG encode.
- It is side-effect free aside from reading the uploaded bytes.
- Its output is finite data: thumbnail PNG bytes plus original dimensions.
- The caller immediately writes the returned reader to the media store.

The current source shape is awkward:

- Input is `*multipart.FileHeader`, an HTTP upload handle whose useful content
  is accessed through `Open()`.
- Output is `*bytes.Reader`, chosen because Listmonk's media store accepts
  `io.ReadSeeker`.
- The raw result shape has more than the currently supported `(T, error)` HTTP
  generator contract.

The current cut table recorded this as proxy-shaped boundary pressure in
`analyses/listmonk-M-4.md`, and the existing corpus row remains skipped rather
than proved end-to-end. Treating this as "cut at `(*App).UploadMedia` instead"
pulls in app/media-store state and loses the clean semantic target.

## Cut-Placement Tradeoff

The adapter idea matters because cut placement is not one-dimensional. Before
adapters, the compiler often sees a false choice:

- Move the cut shallower until the signature looks more like an application or
  framework boundary, but pay for more shared state, more side effects, and a
  larger extracted surface.
- Move the cut deeper toward the semantic unit, but hit source-level types that
  were not designed as network payloads.

Listmonk M4 shows the tradeoff:

| Candidate | Benefit | Cost |
|---|---|---|
| `(*App).UploadMedia` | Natural request-handler edge; caller already has filenames, content type, media store, cleanup policy. | Pulls in `*App`, media store, core DB calls, config, i18n/logging, cleanup side effects, and HTTP context reconstruction. |
| `processImage` as written | Correct semantic unit: decode, resize, encode, dimensions. Minimal app state. | Signature has `*multipart.FileHeader`, `*bytes.Reader`, and a multi-value return shape that is not directly admitted by current HTTP/JSON codegen. |
| generated `processImageBytes` adapter | Keeps the semantic unit and avoids app/media-store state. Boundary is finite bytes and scalar metadata. | Requires static proof plus generated host wrapper, DTO, and body normalization. |
| lower library internals | Some signatures may be simpler locally. | The cut stops matching the application-level concept, loses width/height/output packaging context, and can drift into third-party library code. |

With adapters, a new sweet spot emerges: cut at the semantic unit and normalize
the boundary shape, rather than climbing to a parent that is only attractive
because its signature is easier to admit. This is a real tradeoff, not a free
win. `AdapterPossible` adds compiler complexity and proof obligations, but it
can dominate both the shallow direct boundary and the deep awkward source
signature.

The ranking rule should therefore be:

1. Prefer the smallest semantic unit that preserves the original app behavior.
2. If that unit is directly liftable, use the existing direct-boundary path.
3. If it is not directly liftable, try boundary normalization before moving the
   cut to a broader parent.
4. Move to a broader parent only when the adapter proof fails, the adapter would
   need live proxy semantics, or the parent is the intended semantic unit.

## Terminology

Use five distinct boundary classes:

| Class | Meaning | Example |
|---|---|---|
| `DirectBoundary` | Source signature is already serializable/reconstructible enough for codegen. | `Hash(password, salt []byte) ([]byte, error)` |
| `AdapterPossible` | Compiler can synthesize a local wrapper plus normalized remote payloads. | `*multipart.FileHeader -> []byte`, `[]byte -> *bytes.Reader` |
| `AdapterUnknown` | A boundary might be adaptable, but the compiler has not proved the required transforms. | custom stream wrapper with unrecognized methods |
| `LiveProxyRequired` | Remote code would need to interact with a host-owned live object. | `http.ResponseWriter`, callbacks, channel protocols |
| `AdapterImpossible` | Static proof fails or preserving semantics would require changing the app. | mutable write-back object with aliasing, transaction closure |

`AdapterPossible` should not be called "proxy-required". The adapter is local
code that marshals finite values before/after the RPC. A live proxy is a
runtime protocol between extracted code and the monolith.

## ProcessImage Normalization

Current call site:

```go
thumbFile, wi, he, err := processImage(file)
if err != nil {
    cleanUp = true
    return echo.NewHTTPError(http.StatusInternalServerError, ...)
}
width = wi
height = he

tf, err := a.media.Put(thumbPrefix+fName, contentType, thumbFile)
```

Current helper:

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
    Thumbnail []byte `json:"thumbnail"`
    Width     int    `json:"width"`
    Height    int    `json:"height"`
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
        Thumbnail: out.Bytes(),
        Width:     b.X,
        Height:    b.Y,
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
    return bytes.NewReader(out.Thumbnail), out.Width, out.Height, nil
}
```

The caller still sees the same signature and writes the returned reader to the
same media store. The remote service only sees finite bytes and returns finite
bytes plus scalar metadata.

## Static Feasibility Obligations

An adapter pass should prove these before classifying a cut as
`AdapterPossible`:

1. **Finite input extraction.** The awkward input can be converted to a bounded
   value before the remote call. For `multipart.FileHeader`, this is
   `Open() -> io.ReadAll`.
2. **Local-only lifecycle.** Temp files, close operations, request context, and
   cleanup stay in the host wrapper. The remote function receives no handle
   whose lifetime it must preserve.
3. **Use-shape compatibility.** The target uses the awkward value only through
   operations the adapter models. For this case: one `Open`, read-only decode,
   no mutation, no reflection, no filename/header dependence in the extracted
   body.
4. **Return rehydration.** Awkward return values can be rebuilt locally from
   finite values. For this case: `[]byte -> bytes.NewReader`.
5. **Error order preservation.** Errors introduced by extraction occur at the
   same logical point as the original code, or are equivalent to existing
   helper errors. `file.Open` and `ReadAll` errors remain host-side before the
   remote call; decode/resize/encode errors remain helper errors.
6. **Call-site compatibility.** All observed callers can continue using the
   original signature. The adapter must not require source callers to change.

If any proof fails, the pass should downgrade to `AdapterUnknown`,
`LiveProxyRequired`, or `AdapterImpossible` rather than selecting a broad parent
by default.

## Compiler Pipeline

Add a boundary-normalization pass between cut selection/admission and codegen:

```text
activation path
  -> cut candidate ranking
  -> boundary data classification
  -> boundary normalization
       DirectBoundary       -> existing codegen path
       AdapterPossible      -> synthesize adapter plan, then codegen normalized target
       LiveProxyRequired    -> refuse unless a live-proxy transport exists
       AdapterImpossible    -> refuse
       AdapterUnknown       -> refuse or keep searching for another cut
  -> codegen admission
  -> generated host patch + extracted service
```

The adapter plan should be explicit IR, not an implicit special case buried in
rendering:

```text
AdapterPlan
  SourceFunction: processImage
  HostSignature:  *multipart.FileHeader -> *bytes.Reader, int, int, error
  RemoteSignature: []byte -> processImageResult, error
  InputTransforms:
    file: multipart_file_read_all
  BodyRewrite:
    file.Open/decode(src) -> decode(bytes.NewReader(input))
  OutputTransforms:
    processImageResult.Thumbnail -> bytes.NewReader(...)
    processImageResult.Width -> int
    processImageResult.Height -> int
  Proofs:
    finite_input
    local_lifecycle
    read_only_use_shape
    return_rehydration
    error_order_preserved
```

This lets admission explain why a cut is accepted or refused, and gives e2e
tests a stable artifact to inspect.

## Pattern Library

Start with a small library of recognized adapters:

| Pattern | Input/Output Shape | Status |
|---|---|---|
| `multipart_file_read_all` | `*multipart.FileHeader -> []byte` | Candidate for Listmonk M4 |
| `reader_read_all` | `io.Reader` / `io.ReadCloser` input -> `[]byte` | Already conceptually supported for bounded readers |
| `bytes_reader_return` | `[]byte -> *bytes.Reader` return | Candidate for Listmonk M4 |
| `multi_result_dto` | `A, B, ..., error -> struct{A; B; ...}, error` | Needed for HTTP/JSON generator compatibility |

Do not generalize immediately to arbitrary method-call traces. Each pattern
should carry its own proof matcher and refusal reasons.

## Refusal Examples

The pass should refuse or classify as `LiveProxyRequired` when the awkward value
is not just a finite payload wrapper:

- `http.ResponseWriter`: remote code writes headers/body with ordering and
  lifecycle tied to the active request.
- `io.Writer` output parameters: remote code would stream back to a host-owned
  sink unless the entire output can be captured as a value.
- Channels: send/receive order and goroutine scheduling are part of the
  semantics.
- Transaction closures: callback execution owns database transaction lifetime.
- Mutable write-back objects with aliasing: local aliases may observe mutation
  order.

These are not Listmonk M4-like adapters.

## Implementation Sketch

1. Add `AdapterClass` and `AdapterPlan` to the cut/admission model.
2. Teach boundary classification to return `AdapterUnknown` rather than forcing
   all non-direct shapes into infeasible/proxy buckets.
3. Implement the `multipart_file_read_all`, `bytes_reader_return`, and
   `multi_result_dto` patterns against SSA signatures and function bodies.
4. Extend admission so `AdapterPossible` candidates can pass when every
   required transform has a codegen implementation.
5. Render the host wrapper under the original function name and render the
   normalized helper as the extracted target.
6. Add a focused e2e target for `listmonk/M-4` that uploads an image, asserts
   the thumbnail object is written, checks width/height metadata, and verifies
   extracted-service call count.

## Open Questions

- How large can a bounded reader payload be before the adapter should require a
  staging object rather than inline JSON/base64?
- Should body rewriting be source-to-source, SSA-to-source, or implemented by
  cloning the function and replacing recognized operations?
- Can the pass prove all call sites are compatible cheaply, or should it
  require the original function wrapper to preserve compatibility by
  construction?
- Should `AdapterPossible` candidates outrank broader direct boundaries, or
  should the cut strategy require an explicit semantic-target preference?
- How should transcript comparison represent DTO-normalized returns when the
  app-facing return shape is unchanged?

## Success Criteria

For Listmonk M4, success means:

- The selected semantic target remains image thumbnail generation, not
  `(*App).UploadMedia`.
- The source-facing function still behaves synchronously and returns
  `(*bytes.Reader, int, int, error)`.
- The extracted service receives only finite serialized values.
- The thumbnail written through `a.media.Put` is byte-equivalent to the local
  helper's output for the test fixture.
- Failure behavior remains local where it was local (`file.Open`) and remote
  where it was helper compute (`Decode`, `Resize`, `Encode`).
