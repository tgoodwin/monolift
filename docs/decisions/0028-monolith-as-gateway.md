# ADR-0028: Monolith as gateway — the lifting model

**Status:** accepted
**Date:** 2026-05-06
**Context docs:** `docs/research/activation-paths/cut-placement-evaluation.md`, `docs/research/activation-paths/cut-placement-synthesis.md`, SPRINT-0039, SPRINT-0040

## Context

SPRINT-0040 implemented a cut-placement analyzer that classifies candidate
network boundaries as Feasible, FeasibleWithProxy, or Infeasible. During
corpus evaluation, 6 traces diverged because the analyzer preferred a shallow
ordinary-Feasible cut over a deep FeasibleWithProxy cut (the ground truth).
These traces are all HTTP middleware functions carrying `http.ResponseWriter`.

Investigating the divergence surfaced a question: what does "FeasibleWithProxy"
mean in practice? The answer depends on the lifting model — specifically, what
role the monolith plays after a function is lifted.

## Decision

**The monolith is the gateway.** After lifting, the original application
continues to serve its public API surface. Clients still hit the monolith.
At the cut point, the monolith makes an RPC to the lifted service instead
of a local function call. Only the cut-point function's parameters and
return values cross the network boundary.

This is not a new decision — it is the model described in the PLOS paper.
This ADR makes it explicit because it has concrete implications for the
cut-placement analyzer.

**FeasibleWithProxy is retired as a cut-placement category.** The
three-way classification (Feasible / FeasibleWithProxy / Infeasible)
collapses to two (Feasible / Infeasible):

- **`http.ResponseWriter` at the cut point** means the cut is too shallow.
  The monolith handles the HTTP request/response lifecycle; the
  ResponseWriter should never reach the cut-point function's signature.
  The fix is to cut deeper, not to add a proxy.

- **`io.Reader`/`io.Writer` at the cut point** is a transport-selection
  question (buffer the data and send as `[]byte`, or use a streaming RPC),
  not a cut-placement question. The liftability property
  `boundary.no-streaming-values` (ADR-0018) already covers this.

- **Channels at the cut point** do not appear at deep cuts in practice.
  The channel-send-receive edge is above the cut (the dispatch mechanism);
  the function below receives the work item as a regular parameter.

## Consequences

1. **Cut-placement analyzer simplifies.** The decision tree no longer needs
   to maintain separate Feasible and FeasibleWithProxy pools, compare across
   feasibility classes, or decide when a proxy candidate should win. A
   candidate is either feasible (its boundary data can be serialized for RPC)
   or infeasible.

2. **Streaming boundary values become a "cut deeper" signal.** If a candidate
   function's parameters include `http.ResponseWriter`, `io.Writer`, or
   similar streaming types, the analyzer should prefer a deeper candidate
   rather than accepting the proxy. If no deeper candidate exists, the
   streaming type is an infeasibility signal — the function may not be a
   realistic lift target.

3. **Transport selection remains downstream.** How the RPC is implemented
   (gRPC, Connect, HTTP, etc.) and whether streaming is needed are transport
   decisions made after cut placement, informed by the cut-point function's
   signature and ADR-0018 liftability properties.

4. **The 6 proxy-preference corpus divergences are reclassified.** The Caddy
   and PocketBase traces where the ground truth recommended FeasibleWithProxy
   middleware cuts are cases where the developer would realistically target
   the domain function *below* the middleware, not the middleware itself.
