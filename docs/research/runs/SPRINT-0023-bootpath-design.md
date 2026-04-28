# SPRINT-0023 Boot-path Design Hypothesis

## Hypothesis

A bounded SSA scan can recover enough startup context for a lifted region without changing admission. The pass runs after surface derivation and records deployability evidence in `BootSpec`; refusal means emission is not currently possible, not that the region boundary is semantically inadmissible.

## Walk algorithm

v0 starts from the built SSA program and the region union closure. It inspects reachable functions and the package `main.main` entry path when available, collecting:

- config-source calls (`os.Getenv`, `os.LookupEnv`, `flag.String`, `flag.Var`, `os.Open`, `os.ReadFile`);
- string literals as report-only `LiteralSource` evidence;
- goroutine launches whose callee or captured function is in the region closure;
- dependency-init calls that appear on the path and return values consumed by region functions.

The first implementation is deliberately bounded: it scans functions once, keys visited functions by `*ssa.Function`, and sorts all output before returning. It does not recursively chase arbitrary heap aliases in v0.

## Config-source mapping

- `os.Getenv("X")` / `os.LookupEnv("X")` becomes `EnvSource{Name: "X"}`.
- `flag.String("name", default, ...)` and `flag.Var(..., "name", ...)` become `FlagSource{Name: "name"}`.
- `os.Open("/path")` and `os.ReadFile("/path")` become `FileSource{Path: "/path"}`.
- string constants reachable on the path become `LiteralSource`.
- SQL query reads are recorded as `DBSource` when they are visible at the call boundary.

## Goroutine overlap

A `*ssa.Go` is recorded when its static callee is in the union closure, or when the launched closure is itself in the union. The extracted service main replays these launches; the host stub does not.

## Bounds

The pass is single-program, single-region, and single-visit per SSA function. It does not walk full callgraph transitive closure beyond the compiler-provided union. Mattermost scale is tested separately under B.gate-1 with a 30 minute / 16 GiB budget.

## Refusal framing

Unportable literals, required DB-backed config, or unclassified required dependency-init shapes produce `BootPathRefusal`. These are emission refusals and do not modify ADR-0017 admission rules or ADR-0018 Layer-1 properties.
