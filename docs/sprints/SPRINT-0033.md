# SPRINT-0033 - EntryPath contract and corpus shape validation

**Status:** retired / superseded after Phase 0  
**Executor:** Lagrange  
**Predecessor:** SPRINT-0032 froze the EntryPath bridge as algorithm v1, clarified phase-local bridge/index budgets, and validated the Mattermost oracle chain without adding Mattermost-specific search expansion.

## Retirement Summary

SPRINT-0033 began with a reasonable premise: before downstream compiler
passes reason about activation handoffs and distribution cut points,
EntryPath should expose a normalized, consumer-facing contract instead
of raw diagnostic `ProbeResult` data.

The sprint intentionally put a viability checkpoint first. That
checkpoint showed the premise was premature. The current bridge can find
useful evidence for some request-shaped cases, but it does not yet
generalize well enough across activation shapes to justify freezing a
stable downstream contract.

Rather than forcing an API around incomplete evidence, this sprint stops
after Phase 0 and records the pivot. The next research problem is not
"normalize the current probe output." It is:

> What is the smallest static graph whose edges are meaningful enough
> that a path from application roots to region roots corresponds to a
> real activation path?

That reframing moves the work toward activation graphs, activation
handoffs, and root-linked callable-transfer evidence. The normalized
EntryPath contract should wait until that graph model is credible across
the candidate corpus.

## Original Intent

The original plan was to:

- run the current EntryPath bridge against a locked candidate set;
- use those observations to define a stable EntryPath producer contract;
- separate consumer evidence from diagnostics, oracle traces, stats, and
  budget instrumentation;
- produce a corpus catalog of concrete entry-path examples; and
- validate that downstream cut-point reasoning would have enough
  structured evidence to consume.

That contract implementation did not proceed. The Phase 0 evidence
showed that the current abstraction was still too shaped by known
registration mechanisms and not yet grounded in a general activation
graph.

## Executed Scope

Only Phase 0 was executed.

- [x] Treat the initial candidate matrix as locked unless a row failed to
  load or was technically impossible.
- [x] Run the current EntryPath algorithm against each candidate with
  existing `ProbeResult` output before designing a normalized contract.
- [x] Classify recovery for each candidate as viable, partial, or not
  currently viable.
- [x] Record observed roots, touchpoints, activation evidence,
  registration/bootstrap evidence, wrapper links, missing edges, timing,
  RSS, and budget stops.
- [x] Run candidate probes serially rather than in parallel.
- [x] Stop before contract implementation because the current approach
  did not generalize across the locked candidate set.

Primary evidence:

- `docs/research/runs/SPRINT-0033-entrypath-candidate-viability.md`
- `docs/research/runs/SPRINT-0033-lift-target-catalog.md`

## Candidate Findings

| Candidate | Result | Read |
|---|---|---|
| Mattermost WebSocket hub | Viable | Recovered the known `InitWebSocket -> APIHandlerTrustRequester(connectWebSocket)` chain. Useful control case, but still close to the Mattermost/request-registration shape that motivated the bridge. |
| Miniflux Fever handler | Viable | Best compact non-Mattermost success. The bridge found handler factory, middleware, and object-method evidence. |
| Gitea SSE eventsource | Partial / expensive | Found `Events -> Manager.Register -> Messenger.Register`, but did not cleanly connect framework route/context dispatch to that behavior chain as activation-handoff evidence. The full probe was also very expensive. |
| Miniflux feed refresh | Partial / noisy | Found useful API and worker touchpoints, but did not produce a clean activation path for the refresh behavior. This row exposed the multiple-activator problem. |
| PocketBase autobackup | Not viable yet | Found the callback body calling `CreateBackup`, but missed the `OnBootstrap().BindFunc` and `Cron.Add(...)` bootstrap/scheduled-callback handoff. |

## Pivot

The failed assumption was that the current bridge output was close enough
to a stable compiler contract. It was not. The evidence suggests the
analysis needs a clearer graph model first.

The concepts that supersede the original Sprint 33 plan are:

- **Activation graph:** the static graph between application roots and
  region roots.
- **Activation path:** one recovered route through that graph explaining
  how control reaches the lifted region.
- **Activation handoff:** the semantic transition where broad
  application machinery becomes the specific behavior chain that reaches
  the region.
- **Distribution cut point:** a later compiler decision about where to
  introduce the generated remote boundary. EntryPath should preserve
  evidence for this decision, but should not choose it.

The next sprint should focus on defining and validating activation graph
edges, not implementing the normalized downstream contract yet.

## Deferred Work

The following original phases are explicitly retired from this sprint:

- current `ProbeResult` contract audit;
- `EntryPathResult` / `EntryPathTraceSet` implementation;
- CLI normalized-output mode;
- fixture coverage for the normalized contract;
- downstream import tests for the normalized type; and
- cut-point selector readiness validation.

These are still likely useful, but only after the activation graph model
can represent the candidate set without overfitting to Mattermost or to
request-handler registration.

## Recommended Next Sprint

Plan the next sprint as a research/design sprint around activation graph
definition and evidence.

It should answer:

- What are the graph nodes: functions, closures, method values, storage
  slots, fields, bootstrap sites, goroutine launches, or something else?
- What edge kinds mean real activation transfer rather than mere
  co-location in the same owner?
- Can root-linked callable-transfer edges explain the candidate set?
- How should EntryPath represent partial knowledge, missing dynamic
  links, and multiple activation paths?
- Which small fixtures should be built before rerunning expensive corpus
  probes?

Only after that work should Monolift revisit a stable EntryPath producer
contract for downstream activation-handoff and distribution-cut-point
reasoning.
