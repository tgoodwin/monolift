# listmonk/M-4 - Image thumbnail generation (`processImage`)

## Header

- Trace ID: `listmonk/M-4`
- Project: `listmonk`
- Region root: `cmd/media.go:212`
- Path length: 4
- Source trace: `projects/listmonk/traces/M-4.synthesis.md`
- Decision: ADR-0032 boundary-adapter recovery

## Candidate Cut-Point Table

| Step | Candidate cut point | Incoming edge | Extraction surface | Boundary data | Adapter class | State reconstruction | Callbacks | Error semantics | Selected |
|---:|---|---|---|---|---|---|---|---|---|
| 1 | `initHTTPServer` | `direct-function-call` | Very-large | Reconstructible | AdapterUnknown | Shared-state | 0 estimated | Needs-wrapper | No |
| 2 | `initHTTPHandlers` | `direct-function-call` | Medium | Reconstructible | AdapterUnknown | Shared-state | Low | Needs-wrapper | No |
| 3 | `(*App).UploadMedia` | `callback-registration` | Small | Reconstructible | DirectBoundary | Client-reconstructible | Low | OK | No |
| 4 | `processImage` | `direct-function-call` | Minimal | Reconstructible | AdapterPossible | Client-reconstructible | 0 confirmed | OK | Yes |

> **Footnote — `Reconstructible` vs. the `missing_reconstructor` admission
> refusal.** Phase 0's flag-on admission sweep records `processImage` refusing
> *direct* codegen with `missing_reconstructor`, which can look at odds with the
> `Reconstructible` boundary-data column above. They describe different axes and
> both are correct. `Reconstructible` is the `BoundaryDataClass` of the *source
> value* — the bytes behind `*multipart.FileHeader` are finite and can be
> reconstructed in principle. `missing_reconstructor` is an *admission* refusal
> about whether the current **direct** pipeline has a registered reconstructor
> for that specific awkward Go type; it does not. That refusal is precisely the
> shape-compatible signal that triggers the adapter recovery branch (ADR-0032),
> which then supplies the reconstructor via the `multipart_file_read_all` /
> `bytes_reader_return` patterns. The source of truth for "is this liftable" is
> the orthogonal `AdapterClass` axis (`AdapterPossible` here), not
> `BoundaryDataClass` alone — the data being reconstructible is necessary but
> not sufficient for *direct* admission, and the adapter pass closes the gap.

## Adapted Semantic Unit

The selected cut is `processImage`, not `(*App).UploadMedia`.

The host wrapper preserves the application signature:

```go
func processImage(file *multipart.FileHeader) (*bytes.Reader, int, int, error)
```

It drains the multipart file locally into `[]byte`, calls the extracted
normalized helper, and rehydrates the thumbnail with `bytes.NewReader`.

The extracted boundary is finite:

```go
func monoliftNormalizedprocessImage(input []byte) ([]byte, int, int, error)
```

The first result is thumbnail PNG bytes; the integer results are the original
image width and height.

## Proof Summary

`multipart_file_read_all` discharges finite input, local lifecycle, and
use-shape checks: the helper opens the `*multipart.FileHeader` once and does
not inspect filename, header, or mutable file state.

`bytes_reader_return` discharges return rehydration: the helper returns a
`*bytes.Reader` only via `bytes.NewReader` on thumbnail bytes.

`adapter_error_order` records the accepted move of file read errors to the
host side before RPC. `adapter_call_site` is bounded by the Listmonk call-site
scan: `processImage` is called directly from `UploadMedia`.

## Stage Result

`activation-listmonk-processimage` reaches stage 10 on CloudLab through the
4 -> 5 -> 6 -> 7 -> 8 -> 9 -> 10 ladder. The oracle policy is direct PNG byte
comparison for thumbnail bytes plus original dimension comparison.

Retired terminology: this row no longer uses "Proxy-required" or
"Feasible-with-proxy". `AdapterPossible` is finite local marshaling, not a live
proxy.
