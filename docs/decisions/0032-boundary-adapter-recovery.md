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

The pass discharges six named obligations. Each is checked exactly as far as the
implementation can prove it; no claim below exceeds what is actually verified:

| Obligation | What is actually checked | Where |
|---|---|---|
| `adapter_finite_input` | Summary proof: every awkward input parameter is matched to a registered input pattern that declares its host-side extraction renderer; an unmatched parameter refuses here. | `adapter_pass.go` (`dischargeGenericObligations`) + pattern registry |
| `adapter_local_lifecycle` | SSA scan over each awkward input parameter's referrers; refuses if the value escapes the helper via any of: a `defer` capturing it, a `Close()` call (static or interface-dispatch), interface boxing (`MakeInterface` of the parameter), or a store into a package-level global. Defense-in-depth across all adapter inputs. | `adapter_pass.go` (`dischargeLocalLifecycle`) |
| `adapter_use_shape` | Pattern-owned allowlist predicate: the input pattern proves the value is consumed only in the bounded shape it recognizes (e.g. `multipart_file_read_all` refuses closure/goroutine capture of the file). | pattern `Discharge` (`adapter_patterns.go`) |
| `adapter_return_rehydration` | Pattern-owned producer scan: the output pattern proves the return value can be reconstructed host-side from the wire value. | pattern `Discharge` (`adapter_patterns.go`) |
| `adapter_error_order` | Accepted-with-divergence record (spec §5): host-side extraction errors occur before the RPC and helper read-errors move to a host-side `ReadAll`. Recorded, not refused. | `adapter_pass.go` (`dischargeGenericObligations`) |
| `adapter_call_site` | Reverse-import scan over the activation-path scope (`ssautil.AllFunctions`): direct calls pass; any function-value, address-of, reflective, or interface-dispatch reference disqualifies. An unexported helper with no references passes (bounded by its own package); an **exported** helper with no observed references refuses, since the scan cannot observe references outside the scope. | `adapter_callsite.go` + `adapter_pass.go` (`dischargeCallSite`) |

Policy and classification refusals use `adapter_payload_too_large`,
`adapter_unknown`, `adapter_impossible`, and `live_proxy_required`.

Multi-result DTO normalization is generic codegen behavior, not adapter
behavior. It applies to every admitted boundary whose multiple non-error
returns can be packed into JSON-codable fields.

Transport is inline JSON/base64 only. The payload ceiling is **plan-configurable**
via `Plan.MaxInlinePayloadBytes`, which the generated client's size guard reads
(`adapter_client.go`) rather than a hardcoded literal. It defaults to 8 MiB when
unset (`defaultInlinePayloadBytes`, applied to the plan in `cut_admit.go`). A
size-sensitive deployment can lower the ceiling by setting `MaxInlinePayloadBytes`
on the plan before rendering. The `staged_object` enum is reserved for future
work and has no renderer.

`MONOLIFT_BOUNDARY_ADAPTER`'s sole behavior is to gate the adapter recovery
branch in admission. It has no other effect — in particular it no longer
influences `callable_boundary_values` emission, which is now always emitted
independent of the flag. It defaults on for local development; flag-off parity
is required against the SPRINT-0050 admission baseline. Remove the flag no
earlier than SPRINT-0053 after two clean releases.

## Consequences

`listmonk/M-4` now selects `processImage`, not `(*App).UploadMedia`, and reaches
stage 10 with direct PNG byte comparison for thumbnail output. The broader parent
is excluded by a **structural** admission rule, not a function-name match: a
candidate that is a strict ancestor of a deeper non-`DirectBoundary` candidate
refuses with `adapter_parent_forbidden` (`adapterParentForbiddenForCandidate` in
`cut_admit.go`). The predicate names no function and no type — it keys solely on
the `activation.AdapterClass` label of the deeper candidate, so new adapter
patterns extend the forbidden set automatically.

Adapters do not support live proxies, streaming transports, general SSA
rewrites, or cost-model ranking. Future patterns must add their own proof
checks and keep `AdapterClass` independent from `BoundaryDataClass`.
