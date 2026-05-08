# What's changing after the initial workshop paper

## At a glance (for PLOS '25 readers)

The workshop paper demonstrated a narrow version of the idea. It showed
that Monolift could keep a Go program runnable as a monolith while
selected calls were redirected to a remote service. To make that prototype
tractable, the paper assumed a small target shape: annotated interfaces,
HTTP-handler-like functions, stateless code, and wiring close to
`main()`.

Those assumptions were useful for the demo, but they do not describe the
Go monoliths in the evaluation corpus. The current work keeps the paper's
central promises - the monolith still runs normally, and unsafe lifts are
refused - while broadening the compiler contract enough to handle real
application structure.

## What changed

| Workshop model | What the corpus showed | Current contract |
|---|---|---|
| Lift an annotated interface or handler-shaped function. | Useful targets are often methods, callbacks, queue handlers, or domain functions with no HTTP types. | The annotation names a lift target; admission is based on liftability properties, not one signature shape. |
| Treat `net/http.Handler` as the main transport shape. | Frameworks wrap or replace standard HTTP signatures. | Handler-like shapes still help choose a transport after admission, but they do not decide whether a region is liftable. |
| Require lifted code to be stateless. | Useful targets commonly carry configuration, clients, caches, locks, or channel state. | The compiler classifies state into bounded archetypes and refuses only the cases it cannot model safely. |
| Wire the lift from `main()`. | Real dispatch often happens through framework registration, callbacks, stored function values, or queues. | The compiler recovers an activation path from the program entrypoint to the lift target. |
| Refuse unsafe lifts. | The principle was right, but the prototype did not make every refusal explicit. | Refusals are stable `MLV2_*` diagnostics with rule IDs, source spans, and remediation hints. |

## Example: a Miniflux handler

Miniflux's `currentUserHandler` looks close to an ordinary HTTP handler,
but it is a method:

```go
func (h *handler) currentUserHandler(w http.ResponseWriter, r *http.Request)
```

The receiver matters. The body calls `h.store.UserByID(...)`, so lifting
the function also means accounting for the `*storage.Storage` field on
`h`. Under the prototype contract, that receiver was enough to reject
the lift: the function was not a standalone `net/http.Handler`, and the
compiler had no vocabulary for receiver state.

Under the current contract, the same function is analyzed in two stages.
First, the compiler asks whether the function's boundary and body
properties make remote execution safe. Then it classifies `h.store` as
external state and uses the HTTP-shaped parameters as transport evidence.
The source code did not become simpler; the compiler's model became more
precise.

Related decisions: [ADR-0002](https://github.com/tgoodwin/monolift/blob/main/docs/decisions/0002-renegotiate-contract-v2.md),
[ADR-0003](https://github.com/tgoodwin/monolift/blob/main/docs/decisions/0003-extraction-root-call-graph.md),
[ADR-0004](https://github.com/tgoodwin/monolift/blob/main/docs/decisions/0004-annotation-surface-generalized.md),
[ADR-0011](https://github.com/tgoodwin/monolift/blob/main/docs/decisions/0011-harness-before-compiler.md),
and [ADR-0017](https://github.com/tgoodwin/monolift/blob/main/docs/decisions/0017-classifier-reasons-about-liftability.md).
