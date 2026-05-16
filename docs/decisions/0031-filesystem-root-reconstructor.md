# ADR-0031: Filesystem Root Reconstructor

**Status:** accepted
**Date:** 2026-05-16
**Context docs:** SPRINT-0050, SPRINT-0050 candidates

## Context

SPRINT-0050 needs a filesystem/object-store proof that reconstructs a durable
resource in the extracted service without reconstructing the whole application.
The selected primary candidate is PocketBase `(*filesystem.System).CreateThumb`,
whose receiver owns a blob bucket backed by a local root. The earlier admission
result refused that receiver because `*filesystem.System` has non-serializable
fields, which is the correct refusal unless the compiler has an explicit
reconstructor for that state family.

This target must not become a general PocketBase or app-root reconstructor. The
proof value is narrower: the extracted service can rebuild access to the same
durable filesystem root as the host, handle root-relative object keys, and fail
early when the root is missing or unsafe.

## Decision

Add a filesystem-root reconstructor family for local filesystem/object-store
handles. For the PocketBase primary, `*filesystem.System` is reconstructed with
`filesystem.NewLocal(root)` from an explicit environment variable and closed
during extracted-service shutdown.

The reconstructor registry entry owns the generated imports, initialization
code, close code, extracted-service environment variables, startup checks, and
deployment mount requirements. A lift that uses this reconstructor must mount a
shared durable root into both the host and extracted service; per-pod
`emptyDir` volumes do not satisfy this contract.

Payload paths that address objects inside the root are root-relative keys.
Generated or target-specific request handling must reject unsafe absolute paths
and `..` traversal before calling the original function.

## Consequences

Receiver reconstruction is admitted only for known durable-resource
reconstructors. Whole-app reconstructors such as `core.App` or `PocketBase.App`
remain out of scope.

Generated deployments must carry the durable-root environment variable and a
shared mount shape that both pods can use. Generated server startup must check
that the root exists and is usable, so stage 7 proves more than syntactically
rendered code.

Filesystem/object-store target tests should verify reconstructor detection,
constructor metadata, server init/probe/close rendering, root-relative path
validation, env propagation, mount propagation, and admission refusal when the
metadata is missing.

## References

- `docs/sprints/SPRINT-0050.md`
- `docs/research/runs/SPRINT-0050-candidates.md`
- `evaluation/pocketbase/tools/filesystem/filesystem.go`
- `pkg/codegen/recon.go`
