# Distribution Archetypes v1 - gpt-5.4 run

Cross-links: [catalog](./archetype-catalog-v1.md), [follow-ups](./distribution-archetypes-followups.md), [annotations](./annotations/README.md).

## Question

The sprint question was not "where is the auto-lift vs suggest line in the abstract?" It was narrower and more useful: which currently refused patterns in the corpus already have enough structure that the compiler could name a transform and apply it automatically?

This run's answer is that the new AUTO surface is real, but it is not broad taxonomy. Four survivor terms carried the corpus without inflating the vocabulary:

- `queue-backed worker`
- `scheduled-reconciler`
- `serialized-singleton-owner`
- `connection-hub-buffer`

Everything else either merged into one of those four or stayed terminal.

## Main Finding

The strongest new AUTO surface comes from places where the code already tells us who owns time, who owns work items, or who owns a connection. The compiler is weakest not on "stateful code" in general, but on stateful code whose transform shape is already common and named:

- queue loops whose items are already serializable and whose handlers already externalize their real state
- timer loops whose cadence can move to a scheduler without changing the work body
- singleton-owned maps and registries where a lock is standing in for serialized ownership
- websocket and broker-like hubs where routing keys, replay buffers, and ownership surfaces are explicit

The control sample matters too. Caddy's reverse proxy hot path is exactly the kind of stateful code that should stay terminal in v1. The existence of a mutex or channel is not enough.

## Cross-Target Matrix

`A` means AUTO regions, `S` means SUGGEST regions, `T` means TERMINAL regions.

| Archetype | Caddy | PocketBase | Miniflux | Listmonk | Gitea | Mattermost | Currently refused but shown to be auto-liftable |
|---|---|---|---|---|---|---|---|
| `queue-backed worker` | `0` | `0` | `A1` | `A3` | `A1` | `A1` | Campaign workers, feed refresh workers, Gitea indexer queues, Mattermost job runtime |
| `scheduled-reconciler` | `0` | `A1` | `0` | `A1` | `A1` | `A1` | Bounce mailbox scans, PocketBase cron, Gitea event refresh, Mattermost schedulers |
| `serialized-singleton-owner` | `T1` | `S1` | `S1` | `A1` | `S1` | `S1` | Token tables and selected owner registries; broader session/runtime owners still need evidence closure |
| `connection-hub-buffer` | `0` | `A1` | `0` | `S1` | `A1` | `A2` | PocketBase broker, Gitea eventsource, Mattermost websocket/replay hub |

Per-target headline triage counts from this run:

- Caddy: `ADMITTED 1`, `AUTO 0`, `SUGGEST 0`, `TERMINAL 1`
- PocketBase: `ADMITTED 0`, `AUTO 2`, `SUGGEST 1`, `TERMINAL 1`
- Miniflux: `ADMITTED 1`, `AUTO 1`, `SUGGEST 1`, `TERMINAL 0`
- Listmonk: `ADMITTED 0`, `AUTO 5`, `SUGGEST 1`, `TERMINAL 0`
- Gitea: `ADMITTED 0`, `AUTO 3`, `SUGGEST 1`, `TERMINAL 1`
- Mattermost: `ADMITTED 0`, `AUTO 3`, `SUGGEST 1`, `TERMINAL 2`

## Per-Archetype Boundary Model

The boundary is per-archetype, not one scalar threshold.

- `queue-backed worker`: AUTO when item independence and retry semantics are already visible; SUGGEST when ordering or idempotence remains partly conventional.
- `scheduled-reconciler`: AUTO when the cadence is clearly separate from the work body; SUGGEST when skipped or duplicate tick semantics are not obvious.
- `serialized-singleton-owner`: AUTO when there is one identifiable owner and no meaningful alias leakage; SUGGEST when lifecycle or alias semantics are only inferred.
- `connection-hub-buffer`: AUTO when routing, replay, and queue ownership are explicit; SUGGEST when fanout loss and replay semantics are only partially encoded.

That is the central practical result: "auto-lift vs suggest" is not a universal rule. It is a per-archetype evidence threshold.

## Target Highlights

### Small Targets

Listmonk is the cleanest positive case. Its queue loops, timers, and token table all read like named transforms waiting for the classifier to recognize them.

Miniflux gives the strongest before/after contrast. `ProcessFeedEntries` is already admitted, while the worker pool next to it is the exact sort of refused queue shape that should become AUTO.

PocketBase shows why root selection matters. The app root is terminal, but its cron and subscriptions broker are perfectly serviceable narrower archetype surfaces.

Caddy is the negative control. Refused hot-path synchronization does not automatically imply a transform.

### Large Targets

Gitea's strongest evidence is concentrated, not everywhere. The queue manager, indexer queues, eventsource manager, and timer-driven refresh loop are real candidates. Much of the rest of the repo is orchestration or business logic, not new distribution vocabulary.

Mattermost is the richest single source of connection-hub and worker-runtime evidence. The websocket hub and reliable queue code is the best corpus evidence for a `connection-hub-buffer` state class, while the jobs package is the strongest multi-file confirmation of `queue-backed worker` plus `scheduled-reconciler`.

## Compiler Cannot Know This Statically

The source walks split evidence gaps into two kinds.

Threshold-tunable gaps:

- identifying queue payload serializability in more cases
- recognizing timer loops whose work body is already factored
- proving single-owner mutation when alias surface is small
- extracting explicit routing keys from connection-aware hub code

Those are classifier or evidence-signal follow-ups.

Irreducible or pragmatically irreducible gaps:

- whether duplicates or missed ticks are semantically harmless when the code does not say so
- whether a local event bus tolerates message loss, replay, or subscriber lag when those policies are implicit
- whether a mutable owner truly has no load-bearing alias leaks outside the visible package surface

Those are where pragma-as-evidence may be legitimate, but only as additive evidence, not override.

## Pragma Bridge

Across the survivors, the legitimate pragma role is narrow:

- pick the intended root when several queue, timer, or owner candidates exist
- name a routing key or job cadence the code already supports
- close a root-selection ambiguity, as in PocketBase where the app root is too coarse

The pragma should not be allowed to assert safety the source contradicts. It can supply load-bearing evidence only when the missing fact is about developer intent rather than hidden runtime behavior.

## Tensions Surfaced

- The same codebase can contain both excellent AUTO surfaces and honest terminal regions. PocketBase and Mattermost both demonstrate this strongly.
- Large-target fanout is useful, but only if the prompt is path-correct and thin-return discipline is enforced. This run had to re-dispatch both large targets and ignore one invalid Gitea return that slid into code-review mode.
- Scratch extract artifacts are still a tooling weak spot for corpus work. The source analysis survived that, but the extractor/toolchain mismatch should become a follow-up rather than recurring sprint friction.

## Bottom Line

The expanded auto-lift surface is concrete. This run would add four named archetype-driven transforms to the compiler roadmap, with `queue-backed worker` and `connection-hub-buffer` as the highest-yield near-term additions. The boundary with suggest is not fuzzy; it is archetype-specific and mostly reducible to whether ownership, cadence, and replay semantics are explicit enough in source.
