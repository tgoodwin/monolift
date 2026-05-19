# ADR-0032: Boundary-adapter recovery

**Status:** accepted
**Date:** 2026-05-19
**Context docs:** ADR-0028, ADR-0029, ADR-0030, `docs/research/activation-paths/boundary-adapter-strategy.md`, SPRINT-0051

## Context

Some good semantic cut points have finite data at the boundary but expose it
through awkward local API shapes. `listmonk/M-4` is the first implemented case:
`processImage(*multipart.FileHeader) (*bytes.Reader, int, int, error)` is the
right unit, but direct HTTP/JSON codegen cannot ship a `*multipart.FileHeader`
or a `*bytes.Reader`.

ADR-0028 retired live proxy cuts. This decision does not reopen that category:
the monolith remains the gateway, and adapters only marshal finite values using
local wrapper code.

## Decision

Add an orthogonal `AdapterClass` axis with five values:
`DirectBoundary`, `AdapterPossible`, `AdapterUnknown`, `LiveProxyRequired`, and
`AdapterImpossible`. This axis remains separate from `BoundaryDataClass`.
`BoundaryDataClass` describes source boundary values; `AdapterClass` describes
whether compiler-owned wrapper code can normalize them.

Run the adapter pass as a recovery branch after primary admission refuses the
preferred semantic cut for shape-compatible reasons. This is fallback, not
ranking. If the direct cut already admits, the adapter pass does not compete.
If recovery fails, the existing demotion chain continues.

The pass discharges six named obligations:

- `adapter_finite_input`
- `adapter_local_lifecycle`
- `adapter_use_shape`
- `adapter_return_rehydration`
- `adapter_error_order`
- `adapter_call_site`

Policy and classification refusals use `adapter_payload_too_large`,
`adapter_unknown`, `adapter_impossible`, and `live_proxy_required`.

Multi-result DTO normalization is generic codegen behavior, not adapter
behavior. It applies to every admitted boundary whose multiple non-error
returns can be packed into JSON-codable fields.

Transport is inline JSON/base64 only with an 8 MiB payload ceiling. The
`staged_object` enum is reserved for future work and has no renderer.

`MONOLIFT_BOUNDARY_ADAPTER` gates the recovery branch. It defaults on for local
development; flag-off parity is required against the SPRINT-0050 admission
baseline. Remove the flag no earlier than SPRINT-0053 after two clean releases.

## Consequences

`listmonk/M-4` now selects `processImage`, not `(*App).UploadMedia`, and reaches
stage 10 with direct PNG byte comparison for thumbnail output.

Adapters do not support live proxies, streaming transports, general SSA
rewrites, or cost-model ranking. Future patterns must add their own proof
checks and keep `AdapterClass` independent from `BoundaryDataClass`.
