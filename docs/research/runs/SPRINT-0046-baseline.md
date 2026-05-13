# SPRINT-0046 Baseline Benchmark

## Machine And Cache State

- Go: `go version go1.26.2 darwin/arm64`
- OS/arch: `darwin/arm64`
- Docker: `
Docker version 29.4.2, build 055a478`
- GOCACHE: `/tmp/monolift-go-cache` (present)
- GOMODCACHE: `/Users/tgoodwin/go/pkg/mod` (36 top-level entries)

## Summary

| Project | Status | Found | Total | Scope | Load | SSA | RTA | Augment | BFS | Nodes | Edges | Iter | Path | Cut | Profile |
|---|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|
| caddy | 0 | true | 1.84m | 882.3ms | 1.06s | 324.1ms | 162.6ms | 1.78m | 55.7ms | 57939 | 825464 | 4 | 11 | 10 | `SPRINT-0046-baseline-profiles/caddy.json` |
| miniflux | 0 | true | 18.33s | 623.9ms | 837.6ms | 317.2ms | 2.33s | 13.76s | 15.8ms | 20967 | 260049 | 2 | 8 | 7 | `SPRINT-0046-baseline-profiles/miniflux.json` |
| gitea | 0 | true | 4.49m | 2.04s | 3.08s | 1.16s | 49.58s | 3.51m | 50.5ms | 88886 | 2082375 | 2 | 4 | 3 | `SPRINT-0046-baseline-profiles/gitea.json` |
| listmonk | 0 | true | 14.87s | 510.3ms | 655.6ms | 188.2ms | 1.78s | 11.47s | 11.3ms | 17111 | 219063 | 2 | 9 | 8 | `SPRINT-0046-baseline-profiles/listmonk.json` |
| pocketbase | 0 | true | 34.71s | 469.6ms | 1.25s | 490.2ms | 4.53s | 27.40s | 26.7ms | 34163 | 469311 | 2 | 8 | 7 | `SPRINT-0046-baseline-profiles/pocketbase.json` |
| mattermost | 0 | true | 7.29m | 1.78s | 2.71s | 1.20s | 308.3ms | 7.14m | 85.1ms | 99027 | 1469134 | 3 | 10 | 9 | `SPRINT-0046-baseline-profiles/mattermost.json` |

## Targets

| Project | Target | Log |
|---|---|---|
| caddy | `modules/caddyhttp/caddyhttp.go:279` | `SPRINT-0046-baseline-profiles/caddy.log` |
| miniflux | `internal/reader/sanitizer/sanitizer.go:217` | `SPRINT-0046-baseline-profiles/miniflux.log` |
| gitea | `modules/util/url.go:12` | `SPRINT-0046-baseline-profiles/gitea.log` |
| listmonk | `internal/utils/utils.go:41` | `SPRINT-0046-baseline-profiles/listmonk.log` |
| pocketbase | `tools/inflector/inflector.go:24` | `SPRINT-0046-baseline-profiles/pocketbase.log` |
| mattermost | `channels/app/file.go:588` | `SPRINT-0046-baseline-profiles/mattermost.log` |

## Mattermost Augment Iterations

| Iter | Struct field | Predicates | Goroutine | Package vars | Func args | Map funcs | Interface fields | Explore callees | Total |
|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 0 | 5.38s | 8.8ms | 5.0ms | 7.98s | 17.5ms | 21.03s | 1.23m | 1.96m | 3.76m |
| 1 | 3.72s | 8.0ms | 66.6ms | 15.38s | 231.7ms | 20.40s | 25.67s | 2.43s | 1.13m |
| 2 | 3.92s | 6.3ms | 76.0ms | 16.20s | 207.2ms | 21.53s | 25.64s | 2.1ms | 1.13m |
| 3 | 4.37s | 6.4ms | 72.5ms | 16.26s | 296.3ms | 22.70s | 23.50s | - | 1.12m |

## Phase Bottleneck Ranking

| Project | Top phases |
|---|---|
| caddy | augment 1.78m; resolve-target 1.08s; load 1.06s |
| miniflux | augment 13.76s; rta 2.33s; load 837.6ms |
| gitea | augment 3.51m; rta 49.58s; load 3.08s |
| listmonk | augment 11.47s; rta 1.78s; load 655.6ms |
| pocketbase | augment 27.40s; rta 4.53s; load 1.25s |
| mattermost | augment 7.14m; resolve-target 3.10s; load 2.71s |

## Optimization Decision

Mattermost's top augment costs are `AugmentInterfaceFields` at 2.47m, `ExploreCallees` at 2.00m, and `AugmentMapFuncValues` at 1.43m. The data points to repeated full-program scan cost more than raw iteration count: iterations 1-3 each spend about 1.12-1.13m after the initial exploration-heavy iteration.

Optimization order for Phase 1:

1. Reuse `MapFuncValues` indexes and thread them into `AugmentInterfaceFields`, because those two passes account for about 3.90m of Mattermost's 7.14m augment time.
2. Deduplicate `ExploreCallees` roots across iterations, because initial exploration alone costs about 1.96m and total exploration costs about 2.00m.
3. Cache package-var and struct-field scans after map/interface reuse, because package vars are the next material repeated scan at 55.82s and struct-field scans are smaller at 17.39s.
