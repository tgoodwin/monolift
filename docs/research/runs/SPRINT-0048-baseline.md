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

### miniflux/M-1 — RefreshFeed (Phase 1 attempt)

- **Corpus trace:** `miniflux/M-1` — `RefreshFeed` at `internal/reader/handler/handler.go:207`
- **Signature:** `func RefreshFeed(store *storage.Storage, userID, feedID int64, forceRefresh bool) *locale.LocalizedErrorWrapper`
- **State class:** Client-reconstructible (`*storage.Storage` wraps `*sql.DB`)
- **Boundary class:** Reconstructible

**Activation-path analysis:** Path found (5 steps): `main` → `cli.Parse` → `refreshFeeds` → closure → `handler.RefreshFeed`. Reverse-import scoping confirmed.

**Codegen admission:** Accepted. `RunLift` completes successfully — server, client stub, Dockerfiles, and K8s manifests all generated.

**E2e target:** Scaffolded at `test/e2e/targets/activation_miniflux_refreshfeed/` with `StopAtStage: 4` (verify through compilation only).

**Blocker for full e2e:** `RefreshFeed` returns `*locale.LocalizedErrorWrapper` (error-only, no data return value). The current codegen handles this for generation, but the full e2e round-trip comparison (oracle vs. lifted) requires Phase 2D multi-return/error codec to properly serialize and compare error-only responses. Oracle creation is deferred until Phase 2D.

**Status:** `manifest-skip` — admission accepts, compile verified, full e2e blocked on error-only response format.
