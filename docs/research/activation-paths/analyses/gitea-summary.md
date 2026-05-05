# Gitea Summary

## Scope

- Traces analyzed: 18 (`gitea/M-1` through `M-19`, with no `M-18` trace in the corpus)
- Codebase size: ~456k LOC
- Dominant path shapes: web bootstrap, HTTP handler registration, queue-worker registration, and service/database leaf functions.

## Queue-Worker Pattern

The queue-worker archetype appears repeatedly in `M-1`, `M-2`, `M-5`, `M-6`, `M-10`, `M-12`, and `M-15`. The attractive architectural boundary is the `WorkerPoolQueue` dispatch, but source inspection shows that the queue receiver owns channels, cancellation, worker counters, and handler function fields. The better cut is usually the registered handler or the first service function under it, where queue payloads are serializable and the remote side can reconstruct DB clients.

## Handler and Callback Patterns

HTTP handler registration traces (`M-3`, `M-4`, `M-9`, `M-14`, `M-17`, `M-19`) have a different shape: the handler edge is strong, but the cleanest cut is often one or two calls below it, after request/response values have been converted into repository IDs, package metadata, diff options, or file paths. Function-value-stored-in-field edges are useful evidence of delayed dispatch, but they are weak boundaries unless the stored handler's call signature is itself serializable.

## Surface Area

Gitea's 456k LOC size makes shallow cuts almost always `Very-large` or `Large`. That does not mean every deep cut is automatically best: queue handlers with retry return values can beat the absolute leaf because they preserve batch semantics. Still, surface area is a strong pressure against cuts above `InitWebInstalled`, router registration, or queue runtime startup.

## Synthesis Notes

- Queue-worker traces are the clearest Gitea archetype and provide several representative examples for the corpus synthesis.
- HTTP handler registration is a strong edge signal, but `context.Context`, DB access, and domain structs are usually more liftable than request/response objects.
- Gitea has many client-reconstructible cuts: DB, Git command, indexer, mailer, package, and webhook clients can be rebuilt remotely from config rather than serialized.
