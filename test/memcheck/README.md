# Memcheck Workflow

The `test/memcheck/` harness is the load-bearing RSS gate for
`SPRINT-0010`. Use the Makefile targets instead of ad hoc `go test`
invocations when you need a measured artifact.

## Secondary Debug Recipe

When a stage artifact lands in `regressed`, `killed_rss`, or
`killed_time`, drill into the worst-offender package before changing code
again.

Shape package:

```bash
go test ./pkg/compiler/shape -run . -count=1 -shuffle=303 -memprofile=mem.out
go tool pprof -top mem.out
```

PocketBase extract lane:

```bash
MONOLIFT_CORPUS_TESTS=1 \
  go test ./pkg/compiler/extract \
    -run TestAnalyzeDetectsPocketBaseRefusals \
    -count=1 \
    -shuffle=303 \
    -memprofile=mem.out
go tool pprof -top mem.out
```

Full package lane:

```bash
go test ./pkg/... -count=1 -shuffle=303 -memprofile=mem.out
go tool pprof -top mem.out
```

Use the worst failing seed from the last artifact if it is not `303`.

## Non-Gating Spot Check

Use `/usr/bin/time` only as corroborating data. It is not the sprint gate
because it does not reliably capture the whole `go test` process tree.

macOS:

```bash
/usr/bin/time -l go test ./pkg/... -count=1 -shuffle=303
```

Linux:

```bash
/usr/bin/time -v go test ./pkg/... -count=1 -shuffle=303
```
