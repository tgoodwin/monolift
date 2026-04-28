# SPRINT-0022 emission gap

SPRINT-0022 lands on branch (R) at G.gate-1. The multi-root analysis pipeline can describe the Mattermost Hub/WebConn region, but the current liftpatch API cannot describe a single extracted service that replaces multiple methods across multiple receiver types.

## Required shape

- One extracted service binary: `cmd/monolift-extracted-connection-hub-buffer/main.go`.
- One oracle binary for the same union region.
- Host-side dialer stubs on every external entry point on every root: Hub methods and WebConn methods.
- Intra-region channels, including `WebConn.send`, remain ordinary in-process Go channels inside the extracted binary.

## Missing primitives by API field

### `PatchRequest`

- `FuncName string` supports one function name only. Multi-root emission needs a list of replacement symbols with independent receiver types, names, signatures, and source files.
- `PackageImportPath string` and `PackageDir string` support one package. Mattermost Hub and WebConn happen to share a package, but the region model does not require that, and the API has no package-per-symbol shape.
- `ExpectedSignature string` supports one signature. The region needs one expected signature per replaced method.
- `PreludeSpec` is request-wide. Multi-root stubs need per-symbol preludes because each method has a different receiver, parameter list, result list, endpoint environment variable, and fail-open fallback.
- `GeneratedFiles []GeneratedFile` is request-wide. The renderer needs per-package generated files plus one shared region client/runtime file without collisions.
- `SentinelIdent string` is request-wide. Multiple patched symbols need deterministic, collision-free sentinels per package or a shared package-level sentinel with coordinated ownership.

### `PatchResult`

- `PatchedFile string` records one patched file. Multi-root host patching can patch several files.
- `OriginalSHA256` and `PatchedSHA256` record one file hash pair. Multi-root patching needs a per-file manifest.
- `AddedImports []string` is not keyed by file. Multiple files need separate import deltas.
- `GeneratedFiles []string` is flat and cannot distinguish shared region files from per-symbol siblings.

### `PatchSymbolBody`

- The implementation rejects methods via `DiagnosticMethodReceiver`. Mattermost external entry points are methods on `*Hub` and `*WebConn`.
- Target lookup is by bare function name, so overloaded method names on distinct receivers cannot be represented.
- The prelude insertion path assumes one target body and one generated sibling file. It cannot coordinate one service endpoint across many replaced symbols.

## Classification

This is tooling immaturity, not a fundamental distribution refusal. The region admission hypothesis accepts the in-region `WebConn.send` channel seam because no wire boundary crosses that channel; the missing piece is a host patch representation capable of replacing multiple receiver methods with one shared extracted-service client.

## Follow-up sketch

Introduce a `RegionPatchRequest` with `Symbols []PatchSymbolRequest`, per-symbol package/file/signature/prelude metadata, and a `SharedGeneratedFiles` section. Return `RegionPatchResult{Files []PatchedFileResult, GeneratedFiles []GeneratedFileResult}`. Keep `PatchSymbolBody` as the single-function adapter used by legacy targets, and route multi-root emission through the region patcher.
