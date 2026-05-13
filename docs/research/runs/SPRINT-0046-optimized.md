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
| caddy | 0 | true | 1.21m | 981.7ms | 1.03s | 333.9ms | 148.6ms | 1.15m | 124.6ms | 57939 | 825464 | 4 | 11 | 10 | `SPRINT-0046-optimized-profiles/caddy.json` |
| miniflux | 0 | true | 13.15s | 713.2ms | 996.7ms | 425.8ms | 2.60s | 8.06s | 10.5ms | 20967 | 260049 | 2 | 8 | 7 | `SPRINT-0046-optimized-profiles/miniflux.json` |
| gitea | 0 | true | 4.83m | 2.28s | 3.42s | 1.36s | 3.25m | 1.39m | 36.2ms | 88886 | 2082375 | 2 | 4 | 3 | `SPRINT-0046-optimized-profiles/gitea.json` |
| listmonk | 0 | true | 8.74s | 1.28s | 567.1ms | 166.6ms | 1.74s | 4.69s | 11.6ms | 17111 | 219063 | 2 | 9 | 8 | `SPRINT-0046-optimized-profiles/listmonk.json` |
| pocketbase | 0 | true | 19.60s | 462.8ms | 1.19s | 455.1ms | 4.23s | 12.79s | 24.4ms | 34163 | 469311 | 2 | 8 | 7 | `SPRINT-0046-optimized-profiles/pocketbase.json` |
| mattermost | 0 | true | 2.83m | 1.70s | 2.45s | 866.6ms | 181.1ms | 2.71m | 75.3ms | 99027 | 1469134 | 3 | 10 | 9 | `SPRINT-0046-optimized-profiles/mattermost.json` |

## Targets

| Project | Target | Log |
|---|---|---|
| caddy | `modules/caddyhttp/caddyhttp.go:279` | `SPRINT-0046-optimized-profiles/caddy.log` |
| miniflux | `internal/reader/sanitizer/sanitizer.go:217` | `SPRINT-0046-optimized-profiles/miniflux.log` |
| gitea | `modules/util/url.go:12` | `SPRINT-0046-optimized-profiles/gitea.log` |
| listmonk | `internal/utils/utils.go:41` | `SPRINT-0046-optimized-profiles/listmonk.log` |
| pocketbase | `tools/inflector/inflector.go:24` | `SPRINT-0046-optimized-profiles/pocketbase.log` |
| mattermost | `channels/app/file.go:588` | `SPRINT-0046-optimized-profiles/mattermost.log` |

## Mattermost Augment Iterations

| Iter | Struct field | Predicates | Goroutine | Package vars | Func args | Map funcs | Interface fields | Explore callees | Total |
|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 0 | 3.81s | 6.3ms | 3.0ms | 769.7ms | 104.9ms | 2.93s | 838.7ms | 33.98s | 42.45s |
| 1 | 264.4ms | 6.5ms | 67.2ms | 13.07s | 168.0ms | 13.77s | 11.55s | 2.02s | 40.92s |
| 2 | 266.4ms | 6.4ms | 69.9ms | 13.44s | 172.1ms | 13.65s | 11.54s | 1.4ms | 39.14s |
| 3 | 260.4ms | 6.4ms | 66.5ms | 13.12s | 169.7ms | 13.60s | 11.51s | - | 38.73s |
