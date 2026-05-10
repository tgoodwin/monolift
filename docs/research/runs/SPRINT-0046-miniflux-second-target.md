# SPRINT-0046 Miniflux Second Target Validation

The planned target `HasValidURIScheme` at `internal/reader/sanitizer/sanitizer.go:258` is stale for this checkout. The local function is `hasValidURIScheme` at `internal/reader/sanitizer/sanitizer.go:353`, it is unexported, and it cannot be directly imported by the e2e oracle service.

Replacement target: `StripTags` at `internal/reader/sanitizer/strip_tags.go:15`.

Why this replacement:

- Same package as `SanitizeHTML`: `internal/reader/sanitizer`.
- Package-level function, no receiver.
- Primitive signature: `(input string) string`.
- Production callsites exist in RSS/RDF parsing and reading-time paths.
- Activation analysis with reverse-import scoping reaches it from an HTTP handler path.

Profile JSON: `docs/research/runs/SPRINT-0046-miniflux-striptags-profile.json`

Validation result: found=true, path length=9, recommended cut step=8.
