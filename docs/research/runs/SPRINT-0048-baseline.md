# SPRINT-0048 Baseline: Existing Activation Targets vs Corpus Trace IDs

**Date:** 2026-05-12
**Purpose:** Audit the 7 existing activation e2e targets and record whether any correspond exactly to SPRINT-0039 corpus trace IDs.

## Existing Activation Targets

| # | E2E Target | Function | Target `file:line` | Corpus Match | Notes |
|---|---|---|---|---|---|
| 1 | `activation-caddy-cleanpath` | `cleanpath` | `modules/caddyhttp/caddyhttp.go:279` | **None** | Not in 72-trace matrix |
| 2 | `activation-gitea-pathescapesegments` | `PathEscapeSegments` | `modules/util/url.go:12` | **None** | Not in 72-trace matrix |
| 3 | `activation-listmonk-sanitizeuri` | `SanitizeURI` | `internal/utils/utils.go:41` | **None** | Not in 72-trace matrix |
| 4 | `activation-mattermost-publiclinkhash` | `generatePublicLinkHash` | `channels/app/file.go:588` | **None** | Not in 72-trace matrix |
| 5 | `activation-miniflux-sanitizehtml` | `SanitizeHTML` | `internal/reader/sanitizer/sanitizer.go:217` | **miniflux/M-3** | Exact match: same function and file:line |
| 6 | `activation-miniflux-striptags` | `StripTags` | `internal/reader/sanitizer/strip_tags.go:15` | **None** | Not in 72-trace matrix (different file from M-3) |
| 7 | `activation-pocketbase-columnify` | `Columnify` | `tools/inflector/inflector.go:24` | **None** | Not in 72-trace matrix |

## Summary

- **1 of 7** existing targets maps to a corpus trace ID: `activation-miniflux-sanitizehtml` ↔ `miniflux/M-3`
- **6 of 7** targets were hand-picked for their simplicity (stateless, single-return, no receiver) and do not appear in the 72-row SPRINT-0039 corpus matrix
- The corpus matrix focuses on the top activation paths by execution frequency, while the existing targets were chosen to prove the codegen pipeline end-to-end on the simplest possible functions

## Corpus Trace Correspondence Detail

### miniflux/M-3 ↔ activation-miniflux-sanitizehtml

- **Corpus trace:** `miniflux/M-3` — `SanitizeHTML` at `internal/reader/sanitizer/sanitizer.go:217`
- **E2E target:** `activation-miniflux-sanitizehtml` — same function, same `file:line`
- **Status in matrix:** `generator-eligible` (already proven by existing codegen)
- **Boundary:** Trivial (primitive/JSON params)
- **State:** Stateless
- **Edge type:** `direct-function-call`

This target already provides corpus coverage for `miniflux/M-3`. The manifest should mark it as `pass` with the existing e2e package.
