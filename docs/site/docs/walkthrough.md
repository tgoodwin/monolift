# How Monolift works, in one example

**Monolift takes code inside a Go monolith and enables it to run as its own service — without you rewriting the program.** This page follows a single real
example, `processImage` from the [listmonk](https://listmonk.app) newsletter
app, all the way from "it's an ordinary function in a big program" to "it runs
as a separate service the program calls over the network."

This page describes the overall process, start to finish. Every other page on this site explores one part of this journey in greater detail. 

## The starting point: a function buried in a monolith

A **monolith** is an application that builds and runs as a single program.
listmonk is one — a Go web app for sending email newsletters. When a user
uploads an image for a campaign, listmonk's upload handler calls a small helper,
`processImage`, to make a thumbnail:

```go
--8<-- "docs/site/snippets/external/listmonk/process-image-signature.go.txt"
```

`processImage` decodes the uploaded image, resizes it with a
[Lanczos](https://en.wikipedia.org/wiki/Lanczos_resampling) filter, and
re-encodes it as a PNG. It is **CPU-bound** and **self-contained**: the same
bytes in always produce the same bytes out, and it touches nothing else in the
program — no database, no shared state.

That makes it a tempting thing to pull out. Image resizing is exactly the kind
of work you might want on its own machines: scale it independently, give it more
CPU, keep it from competing with the web server for resources.

## The goal: *lift* it, don't rewrite it

To **lift** a function is to make selected calls to it run on a remote service
instead of in-process, while the rest of the program is left alone. The call
site still reads as an ordinary function call; underneath, Monolift has replaced
the body with a call across the network. The program still compiles and runs as
a plain monolith when Monolift is not involved.

The catch is that "just call it remotely" hides a pile of questions. The next
four steps are the questions Monolift has to answer to lift `processImage`
safely.

## Step 1 — How does an application workload even reach this function?

Before you can move a function, you need to know how the program gets to it.
Monolift recovers the **activation path**: the chain of calls from the program's
entry point (`main`) down to the function you want to lift.

For `processImage`, the path runs roughly:

```
main()  →  (HTTP routing)  →  (*App).UploadMedia  →  processImage
```

Knowing the call chain matters because the function you want to lift and the place you *insert a network call* need not be the same (lifting certain code may require adding the network call in a neighboring region) and you cannot reason about where to "cut" until you can see the whole path.


??? note "Go deeper: recovering activation paths"
    Real programs rarely reach their handlers through plain function calls —
    they go through routers, callbacks registered with frameworks, goroutines,
    and lookup tables. Recovering the path through all of that is its own
    analysis problem. See **[Recovering activation paths](activation-paths.md)**.

## Step 2 — Where should we split the program?

Now the key decision: *where on that path do we draw the line between "runs in
the monolith" and "runs in the remote service"?* That line is the **network
boundary**, and the function where Monolift inserts it is the **cut point**.

Two natural candidates sit on `processImage`'s path:

- **Cut at `UploadMedia`** (the broader web handler). Its inputs and outputs are
  already simple web data, so it makes an easy boundary. But cutting here drags
  the whole handler across: the application object `*App`, the database connection, the
  media store client, config, request cleanup. The remote service would have to
  reconstruct half the app.
- **Cut at `processImage`** (the small helper). It is the *right unit of work* —
  pure computation, no shared state — so almost nothing has to travel with it.

Monolift would prefer to cut directly at `processImage`, producing a small and self-contained piece of extracted code. It weighs
candidates like these along several dimensions: how much code has to move off the monolith, how much
state must be rebuilt on the far side, whether call parameters are serializable, failures can still be reported,
and more.

??? note "Go deeper: drawing the network boundary"
    The choice among candidate cut points is made by a ranked decision tree over
    six dimensions, designed so every decision is auditable. See **[Drawing the
    network boundary](cut-placement.md)**.

## Step 3 — The snag: this function can't cross a network as-is

There is a problem. To call a function over a network, its inputs and outputs
have to be *serialized*. `processImage`'s signature resists that at both ends:

```go
func processImage(file *multipart.FileHeader) (*bytes.Reader, int, int, error)
```

- The **input** `*multipart.FileHeader` is a *live handle* to an in-progress
  HTTP upload — not data, but a thing you `Open()` to reach the data. You cannot
  ship a live handle to another machine.
- The **output** `*bytes.Reader` is a reader object wrapped around some bytes —
  again not plain data.

One *good* aspect of this signature is that `processImage` returns an `error`, i.e. the code anticipates the possibility of failure. This is an important semantic detail when reasoning about [liftability](canonical-shapes.md): a remote call can fail where a local one could not (the service may be unreachable), and an existing `error` return gives that failure a place to surface — so callers handle it the way they already did.

So we have identified the right function to lift, but its *shape* is wrong. An
earlier version of Monolift would give up here and settle for a worse cut. But Monolift can be smarter!

## Step 4 — Manufacture a clean boundary (the new part)

If no function on the path has a liftable shape, Monolift can add a new one. This is a **boundary adapter**: a small amount of generated code added to the activation path that provides a clean, serialization-friendly place to make the network cut. It comes in two halves:

- A **host wrapper** stays in the monolith, *under the original function's
  name*, so every existing call site is unchanged. It drains the awkward inputs
  into plain bytes, calls the remote service, and rebuilds the awkward return
  values from the bytes that come back.
- A **normalized remote helper** is the same image-resizing logic, but with its
  signature rewritten to use only plain, shippable types. This is what runs in
  the remote service.

The plain data that travels between them rides in **DTOs** (data-transfer
objects) — ordinary structs whose only job is to be turned into bytes and back:

```go
// Host wrapper — stays in the monolith, keeps the original name and signature.
func processImage(file *multipart.FileHeader) (*bytes.Reader, int, int, error)
// (drains file → []byte, calls the remote helper, rebuilds *bytes.Reader from the reply)

// Normalized remote helper — same logic, but only plain types cross the wire.
func monoliftNormalizedprocessImage(input []byte) ([]byte, int, int, error)

// DTO — how the three non-error return values travel as one JSON object.
type processImageResult struct {
    Result0 []byte `json:"result0"` // thumbnail PNG bytes
    Result1 int    `json:"result1"` // original width
    Result2 int    `json:"result2"` // original height
}
```

The important property: nothing about listmonk changes. The call site still
calls `processImage` with a file handle and gets back a reader and two ints.
Monolift manufactured the clean boundary *around* the function instead of asking
the developer to rewrite it.

??? note "Go deeper: adapting the network boundary"
    Adapters are a careful, bounded mechanism — not a general code rewriter, and
    not a way to ship live objects across the wire. The compiler builds one only
    after proving a short list of safety conditions. See **[Adapting the network
    boundary](boundary-adapters.md)**.

## Step 5 — The result: a service the monolith calls, with a safety net

From all of the above, Monolift generates both sides of the boundary plus the
glue that runs them. The generated client-side code (shown here for the
wire-normalized entry point) is where the per-call behavior lives:

```go
func ProcessImage(srcData []byte, typ string) ([]byte, int, int, error) {
    if os.Getenv("MONOLIFT_LIFT_PROCESSIMAGE") != "on" {
        return monoliftOriginalProcessImage(srcData, typ) // lift is off: run locally
    }
    r0, r1, r2, appErr, transportErr := monoliftRemoteProcessImage(srcData, typ)
    if transportErr != nil {
        if os.Getenv("MONOLIFT_LIFT_FAILMODE") == "closed" {
            return nil, 0, 0, fmt.Errorf("monolift: extracted service unavailable")
        }
        return monoliftOriginalProcessImage(srcData, typ) // remote down: fall back to local
    }
    return r0, r1, r2, appErr
}
```

Three behaviors are worth pointing out, because they are what make lifting safe
to turn on in production:

1. **The lift is a switch.** A single environment variable decides, per call,
   whether to run locally or remotely. Lifting is something you turn on, not a
   one-way rewrite.
2. **It fails soft by default.** If the remote service is unreachable, the
   wrapper falls back to running the original function locally — the feature
   keeps working.
3. **…unless you ask it not to.** In *fail-closed* mode it surfaces the error
   instead, for cases where silently running locally would be wrong.

To confirm the lifted service is actually correct, the test harness runs the
original and lifted versions on the same image and compares the output
**byte-for-byte**: the lifted thumbnail must be identical to the local one.

??? note "Go deeper: code extraction"
    Pulling the function body out of the monolith, generating the server, and
    rebuilding values that cannot cross the wire (database handles, loggers) on
    the far side is the *extraction* phase. See **[Code extraction](extraction.md)**.

## The whole arc, in one line

> Find how the program reaches the function (**activation path**) → decide where
> to split it (**cut point** / **network boundary**) → if the split point's
> shape cannot cross a network, **build an adapter** around it → **generate**
> both sides plus the safety net.

Everything else on this site is one of these steps, in depth. The
[glossary on the home page](index.md#terms-used-throughout) collects the terms
introduced above.
