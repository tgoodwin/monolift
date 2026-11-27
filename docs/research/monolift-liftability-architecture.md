# Monolift liftability architecture

**Status:** exploratory design.

The compiler should gather program facts, while Lean owns the semantic reasoning
about ownership and placement.

```text
Go source
  ├─ go/types ──────── signatures, types, error channels
  ├─ Go SSA ─────────── loads, stores, aliases, calls, control flow
  └─ GoCore ─────────── concrete execution semantics (later)

Go SSA
  -> Liftability IR
  -> Lean abstract semantics
  -> satisfying placements
```

## Component boundaries

- **`go/types`** describes values and call boundaries. It identifies parameter,
  receiver, result, and error-channel shapes.
- **Go SSA** exposes operations and data-flow relationships. It should provide
  evidence, not placement verdicts.
- **Liftability IR** is a small normalized representation of relevant SSA facts.
  It preserves origins, effects, calls, source locations, and unknown cases.
- **Lean** interprets the IR, applies ownership assumptions, composes effects,
  and determines which placements satisfy the contract.
- **GoCore** is a possible concrete semantic foundation. Later work can relate
  GoCore executions to the abstract effects and prove that accepted programs
  preserve caller-owned state. It is not required by the first SSA bridge.

The intended query is set-valued:

```text
pure operation                 -> {stateless, singleton, external-state}
receiver-owned mutation        -> {singleton}
explicit external-state write -> {external-state}
caller mutation or alias       -> {}
unresolved effect              -> unknown; refuse admission
```

The Go side may normalize representation, but should not independently decide
that an operation is singleton-safe or external-state-safe.

## Small boundary examples

These examples are deliberately schematic. They define the observations that a
future SSA-to-IR adapter should produce and the placement question Lean should
answer.

### 1. Pure value operation

```go
func Add(x, y int) (int, error) {
    return x + y, nil
}
```

```text
go/types:       value parameters, value result, error channel
SSA:            arithmetic and return; no memory effects
Liftability IR: return value; no ownership effects
Lean:           all placements satisfy the effect contract
GoCore later:   concrete execution can witness the same value behavior
```

### 2. Caller-owned mutation

```go
func SetFirst(xs []int) error {
    xs[0] = 1
    return nil
}
```

```text
go/types:       slice parameter, error channel
SSA:            store through parameter-derived backing storage
Liftability IR: write(parameter 0)
Lean:           no placement; the copied boundary cannot preserve the caller's mutation
```

### 3. Singleton-owned receiver mutation

```go
type Counter struct { n int }

func (c *Counter) Inc() error {
    c.n++
    return nil
}
```

```text
SSA:            store through receiver-derived field
Liftability IR: write(receiver)
Lean:           singleton only, assuming the receiver is region-owned
```

The same effect is not admissible under stateless placement.

### 4. Explicit external state

```go
func Put(db *Database, key, value string) error {
    return db.Put(key, value)
}
```

```text
SSA:            call that may write through db
Liftability IR: write(parameter 0), plus an ownership assumption
Lean:           external-state only if db is explicitly classified as external
```

A parameter named `db` is not enough to establish external ownership.

### 5. Caller alias escape

```go
func Head(xs []byte) ([]byte, error) {
    return xs[:1], nil
}
```

```text
SSA:            result remains rooted in parameter backing storage
Liftability IR: escape(parameter 0)
Lean:           no placement unless a future contract explicitly permits that alias
```

### 6. Unknown dynamic effect

```go
func Run(x *Thing, f func(*Thing) error) error {
    return f(x)
}
```

```text
SSA:            indirect call with parameter-derived argument
Liftability IR: unresolved call / unknown effect
Lean:           unknown; the compiler refuses rather than treating it as pure
```

## Roadmap

1. Define the smallest Liftability IR for these examples.
2. Make Lean compute satisfying placements over that IR.
3. Add a mechanical Go SSA lowering for direct, well-understood cases.
4. Represent unsupported alias and call shapes explicitly as unknown.
5. Later, relate the abstract effects to GoCore executions and prove the frame
   property for a restricted Go fragment.
