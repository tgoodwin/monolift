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
| caddy | 0 | true | 1.07m | 934.5ms | 1.06s | 305.5ms | 149.5ms | 1.01m | 53.4ms | 57939 | 825464 | 4 | 11 | 10 | `SPRINT-0046-early-check-profiles/caddy.json` |
| miniflux | 0 | true | 4.82s | 609.4ms | 759.7ms | 316.3ms | 2.70s | 0.0ms | 146.6ms | 19408 | 248082 | 0 | 8 | 7 | `SPRINT-0046-early-check-profiles/miniflux.json` |
| gitea | 0 | true | 33.71s | 2.16s | 4.22s | 2.60s | 22.64s | 0.0ms | 33.8ms | 79635 | 1970910 | 0 | 4 | 3 | `SPRINT-0046-early-check-profiles/gitea.json` |
| listmonk | 0 | true | 3.81s | 1.13s | 551.5ms | 157.6ms | 1.73s | 0.0ms | 11.3ms | 16513 | 211281 | 0 | 11 | 10 | `SPRINT-0046-early-check-profiles/listmonk.json` |
| pocketbase | 0 | true | 6.65s | 427.8ms | 1.15s | 400.4ms | 4.17s | 0.0ms | 22.3ms | 33601 | 459935 | 0 | 8 | 7 | `SPRINT-0046-early-check-profiles/pocketbase.json` |
| mattermost | 0 | true | 2.86m | 1.66s | 2.43s | 852.1ms | 183.4ms | 2.74m | 85.1ms | 99027 | 1469134 | 3 | 10 | 9 | `SPRINT-0046-early-check-profiles/mattermost.json` |

## Targets

| Project | Target | Log |
|---|---|---|
| caddy | `modules/caddyhttp/caddyhttp.go:279` | `SPRINT-0046-early-check-profiles/caddy.log` |
| miniflux | `internal/reader/sanitizer/sanitizer.go:217` | `SPRINT-0046-early-check-profiles/miniflux.log` |
| gitea | `modules/util/url.go:12` | `SPRINT-0046-early-check-profiles/gitea.log` |
| listmonk | `internal/utils/utils.go:41` | `SPRINT-0046-early-check-profiles/listmonk.log` |
| pocketbase | `tools/inflector/inflector.go:24` | `SPRINT-0046-early-check-profiles/pocketbase.log` |
| mattermost | `channels/app/file.go:588` | `SPRINT-0046-early-check-profiles/mattermost.log` |

## Mattermost Augment Iterations

| Iter | Struct field | Predicates | Goroutine | Package vars | Func args | Map funcs | Interface fields | Explore callees | Total |
|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 0 | 2.46s | 6.3ms | 4.0ms | 777.0ms | 113.3ms | 2.91s | 838.9ms | 42.36s | 49.47s |
| 1 | 263.7ms | 6.1ms | 76.4ms | 11.95s | 208.4ms | 11.63s | 11.95s | 2.99s | 39.08s |
| 2 | 287.0ms | 6.8ms | 79.2ms | 12.18s | 188.0ms | 11.74s | 11.80s | 1.3ms | 36.28s |
| 3 | 266.0ms | 6.8ms | 67.0ms | 11.80s | 172.5ms | 11.65s | 12.17s | - | 36.13s |
