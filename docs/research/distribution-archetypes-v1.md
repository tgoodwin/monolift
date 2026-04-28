# Distribution archetypes v1 — SPRINT-0013 composite research note

**Status:** v1. Synthesized from three parallel independent runs:
- `docs/research/runs/opus/` — deep walk, 7 archetypes, structural boundary model, 5 evidence-signal proposals
- `docs/research/runs/gpt-5.4/` — concise walk, 4 archetypes with aggressive merging, per-archetype boundary framing
- `docs/research/runs/gemini/` — broad bundle coverage, 6 archetypes, contributes filesystem-bound state class

This note is the cross-run composite. Per-run artifacts are preserved at the paths above. Where runs converged, the finding is the claim. Where they diverged, both framings are presented with attribution and the synthesis judgment recorded.

## 1. The research question, restated

Monolift today auto-lifts narrowly. The ADR-0016 rule stack admits only `immutable-captured-config`, `replicated`, and the `externalized-durable` case where the developer explicitly declared `state=external`. Stateful code involving synchronization primitives (mutexes, channels, atomics), shared mutable state, or pointer aliasing is refused via `MLV2_SHARED_MUTABLE_STATE`, `MLV2_CHANNEL_BOUNDARY`, `MLV2_POINTER_ALIAS_UNSUPPORTED`, and related codes.

But many of those refusals correspond to **distribution patterns with known transforms**: a mutex-protected struct is a singleton actor; a channel-fed goroutine pool is a worker queue; a keyed map under a lock is a sharded service; a periodic goroutine is a scheduled invocation. The compiler refuses these not because they are fundamentally undistributable, but because it doesn't yet recognize the archetype that would justify the transform.

The research question was:

> Which currently-refused patterns have enough structure that the compiler could auto-lift them with a named transform, and what would the classifier need to learn to do it?

**The primary product is the AUTO surface** — currently-refused regions that would become auto-liftable if the classifier recognized a named archetype and applied its transform. SUGGEST is the honest fallback when static evidence is strong but not conclusive. TERMINAL is what's left.

## 2. Corpus walk at a glance

Six targets walked region-by-region under a uniform annotation schema. Cross-run AUTO counts (representative; see per-target annotations and per-run catalogs for exact cites):

| target | files | opus AUTO | gpt-5.4 AUTO | gemini AUTO |
|---|---|---|---|---|
| listmonk | 92 | 4 | 5 | 3 |
| caddy | 306 | 7 | 0* | 2 |
| pocketbase | 445 | 5 | 2 | 3 |
| miniflux | 407 | 6 | 1 | 2 |
| gitea | 2875 | 18 | 3 | 6 |
| mattermost | 2153 | 11 | 3 | 4 |
| **totals** | **6278** | **51** | **14** | **20** |

*gpt-5.4 explicitly classified Caddy's reverse-proxy hot path as a negative control — "refused sync primitives do not automatically imply a transform" — rather than enumerating finer-grained AUTO regions within handler subsystems. Directionally consistent with opus's finer-grained split between the admitted handler and its refused dependent state; not a disagreement on substance.

**Cross-run headline.** Opus's deeper fanout surfaced more AUTO regions inside each target (especially gitea's queue/eventsource/cron subsystems and mattermost's hub), while gpt-5.4's merging discipline collapsed multiple AUTO regions into fewer archetype labels with stronger evidence-condition tightness. Gemini sits in between on count but enumerated all bundles in both large targets. The true AUTO surface is closer to opus's count than gpt-5.4's; the disagreement is about label granularity, not about whether the regions are liftable.

## 3. The v1 archetype vocabulary

Eight archetypes survived all four gates (coverage, evidence, emission, boundary) against the combined corpus evidence. Each is a pair of (archetype name, candidate ADR-0016 state class) where the archetype is the source-level pattern and the state class is the classifier-internal vocabulary the compiler would add to recognize it.

| archetype | state class (ADR-0016 addition) | source-level pattern | vocabulary provenance |
|---|---|---|---|
| `serialized-actor` | `serialized-actor` | struct + mutex; receiver-scoped state; no pointer escape | opus (all 3 converge on the concept) |
| `bounded-worker-pool` | `bounded-worker-pool` / `queued-workset` | struct + chan + N goroutines consuming; serializable jobs | all 3 converge (opus name, gpt-5.4 name alternative) |
| `periodic-invocation` | `periodic-invocation` / `scheduled-reconciler` | goroutine + `time.Ticker`/`time.Sleep` loop; idempotent body | opus + gpt-5.4 converge; gemini implicit |
| `keyed-partitioned-state` | `keyed-partitioned-state` | `map[K]V` under mutex; every access keyed | opus primary; gpt-5.4 retires as insufficient coverage |
| `fanout-publisher` | `fanout-publisher` | `[]chan T`/`map[K]chan T` under mutex; Publish iterates | opus + gemini; gpt-5.4 merges into connection-hub-buffer |
| `ttl-cache` | `ttl-cache` | mutex-guarded map with TTL; cache-miss loader exists | opus unique; gpt-5.4 folds into serialized-singleton |
| `session-affinity-state` | `session-affinity-state` | state keyed by session/connection ID; connection-scoped lifetime | all 3 converge on the concept |
| `filesystem-bound-singleton` | `filesystem-bound-singleton` | state/operations bound to local OS filesystem | gemini unique (worth keeping — distinguishing evidence is strong) |

**Alternative framing (gpt-5.4 / composite).** gpt-5.4's `connection-hub-buffer` compresses `session-affinity-state` + `fanout-publisher` + per-connection send queues into one archetype with explicit routing-key + register/unregister + replay-buffer semantics. This is a legitimate lens when all four signals co-occur (mattermost's Hub is the strongest example). The v1 composite keeps the three narrower archetypes but records `connection-hub-buffer` as a **named composite** (ADR-0022 territory) — for regions where the three archetypes co-occur and replay semantics are explicit in source, the compiler emits a composite transform rather than three separate transforms.

**Retirements (kept as research output, not deleted):**

| retired | outcome | reason | provenance |
|---|---|---|---|
| `pipeline-stage` | retired | collapses to `periodic-invocation` + `bounded-worker-pool` composition; no distinguishing emission sketch | all 3 runs retire |
| `ephemeral-worker` | fissioned | splits into `session-affinity-state` (lifecycled) or TERMINAL (fire-and-forget); gemini kept but the ambiguity was load-bearing | opus retires, gpt-5.4 implicit retire, gemini kept — overruled in synthesis |
| `lifecycle-state-machine` | retired for v1, flagged | coherent pattern (gitea `graceful.Manager`) but no v1 emission sketch for ordered distributed state transitions; Raft/CRDT territory | opus |
| `websocket-fanout-hub` | retired as composite | mattermost Hub — expressed as `keyed-partitioned-state` + `fanout-publisher` + `session-affinity-state`; connection-hub-buffer composite lens preserves it | opus |
| `keyed-queue-state-guard` | retired | gitea `baseChannel` uniqueness-set — broker dedup handles this, collapses into `bounded-worker-pool` + broker config | opus |
| `sharded-stateful-service` | kept (as `keyed-partitioned-state`) | opus kept under the renamed label; gpt-5.4 / gemini retired | synthesis: keep, with caveat that corpus evidence is thinner than other archetypes |

## 4. The per-archetype boundary model

Every archetype states AUTO / SUGGEST / TERMINAL thresholds in concrete evidence conditions. See the catalog for full per-entry detail; summary:

### `serialized-actor`
- **AUTO** iff protected state is wholly receiver-owned, no pointer-to-field escape, mutex span encloses every store, no reachability into reflect/unsafe.
- **SUGGEST** iff receiver-scope looks right but the mutex also protects external-client handles the compiler cannot verify externalizable.
- **TERMINAL** iff the mutex guards a structure embedding an un-externalizable durable client (SQLite handle).

### `bounded-worker-pool`
- **AUTO** iff job type serializable, handler holds `effects.no-global-writes`, pool size is static config, per-job ordering not load-bearing.
- **SUGGEST** iff per-key FIFO ordering required.
- **TERMINAL** iff handler has unbounded internal fanout or pool is unbounded (fallback-spawn-on-full).

### `periodic-invocation`
- **AUTO** iff body is idempotent or tolerates skip/duplicate; no captured mutable state; interval is config-driven.
- **SUGGEST** iff body carries counter-style state not reducible to external storage.
- **TERMINAL** iff interval is dynamically self-tuning from prior tick's result.

### `keyed-partitioned-state`
- **AUTO** iff every access keyed, iteration (if any) is idempotent-per-shard background cleanup.
- **SUGGEST** iff key-free iteration appears in hot paths with user-visible semantics.
- **TERMINAL** iff the map encodes cross-key invariants (sum-of-values).

### `fanout-publisher`
- **AUTO** iff event type serializable, subscribers independent, no cross-event ordering requirement.
- **SUGGEST** iff ordering across subscribers required or subscriber churn is high.
- **TERMINAL** iff fanout encodes a distributed transaction across subscribers.

### `ttl-cache`
- **AUTO** iff value serializable, no pointer-to-in-process-state in value, source-of-truth elsewhere.
- **SUGGEST** iff value holds callback or function-pointer.
- **TERMINAL** iff the cache *is* the source of truth.

### `session-affinity-state`
- **AUTO** iff session ID stable for connection lifetime, state purely per-session, no cross-session invariants.
- **SUGGEST** iff state references cross-session shared objects.
- **TERMINAL** iff state lifetime is unbounded beyond connection (multi-connection user state with consistency).

### `filesystem-bound-singleton` (gemini-sourced)
- **AUTO** iff filesystem operations are idempotent-on-retry, paths are config-driven, no in-memory state carries across operations (can be replaced by object-store client).
- **SUGGEST** iff filesystem operations have in-process caching or locking beyond the OS-level lock.
- **TERMINAL** iff filesystem access encodes invariants over local-disk state that volume-mapping cannot preserve.

## 5. Is the boundary a single threshold, per-archetype, or structural?

The three runs disagreed on the framing. **Synthesis view: both are useful lenses, with the structural model explaining *why* the per-archetype thresholds differ.**

**Opus's structural reading** (load-bearing in two axes):
1. **Evidence locality.** When distinguishing evidence is local and closed-form (visible in one SSA function without callgraph expansion), the archetype auto-lifts. When distinguishing evidence depends on runtime behavior or external-library contracts, the archetype routes to SUGGEST.
2. **Externalization affinity.** When the archetype's natural transform moves state to an external substrate (managed cache, broker, scheduler) whose semantics match the archetype's invariants one-for-one, auto-lift is safe. When the transform requires an internal substrate (serial actor harness, custom dispatch loop) *and* state may cross the substrate boundary, SUGGEST unless the compiler can prove a tight boundary.

Concretely: `periodic-invocation`, `fanout-publisher`, `bounded-worker-pool`, `ttl-cache`, `filesystem-bound-singleton` all auto-lift well because they externalize to managed substrates (platform scheduler, broker, managed cache, object store). `serialized-actor`, `keyed-partitioned-state`, `session-affinity-state` auto-lift only when the compiler can prove the tight boundary condition.

**GPT-5.4's per-archetype reading:** auto-lift-vs-suggest is **not** a universal rule; it is a per-archetype evidence threshold. This is the directly-actionable framing for implementation — ADR-0020 codifies the thresholds, not a universal scalar.

**Synthesis judgment:** the per-archetype thresholds (ADR-0020 territory) are what the implementation uses. The two-axis structural model is what explains *why* the thresholds cluster the way they do. Both belong in the narrative.

## 6. Evidence gaps — threshold-tunable vs. irreducible

The brief asked the research to separate evidence gaps the classifier could close by collecting more signals (threshold-tunable) from gaps where static analysis is provably incomplete (irreducible, pragma-territory).

### Threshold-tunable gaps — proposed new classifier signals

Signals the classifier *could* collect that would move SUGGEST regions into AUTO. All from opus; gpt-5.4 independently identified a roughly equivalent set in prose form.

- **`keyed-access-invariant`** — "every call site reaching this map indexes by key derived from input." SSA-visible. Moves `keyed-partitioned-state` SUGGEST → AUTO.
- **`cache-value-no-pointer-escape`** — "value type carries no pointer to other in-process state." `go/types` + SSA on struct fields. Moves `ttl-cache` SUGGEST → AUTO.
- **`session-id-keyed-access`** — "access invariant keyed by connection-accept-time ID, not request-time ID." SSA + callgraph reachability. Moves `session-affinity-state` SUGGEST → AUTO.
- **`bounded-pool-invariant`** — "pool size is provably bounded by a static constant or config value, not runtime-unbounded fallback." SSA on goroutine-spawning loops. Moves `bounded-worker-pool` SUGGEST → AUTO where the fallback-spawn was the reason for SUGGEST.
- **`mutex-encloses-store-invariant`** — "every store on protected state lies inside the Lock/Unlock span." SSA dataflow. Foundational for the `serialized-actor` state class.

### Irreducible gaps — pragma / annotation territory

Gaps where only user-declared evidence can close the classification. This is the largest tension the research surfaced and is the territory of ADR-0021.

- **External-library contract atomicity.** Mattermost's `cache.Cache` interface documents Scan/GetMulti/RemoveMulti atomicity. The compiler cannot verify an interface contract across package boundaries. Only a pragma or a trusted-library allowlist can supply this evidence.
- **Idempotency declarations.** `periodic-invocation`'s body must be idempotent. Static analysis can rule out some non-idempotency (writes to global mutable state) but cannot affirmatively prove idempotency. `idempotent=true` is load-bearing evidence, not an override.
- **Per-key ordering.** `bounded-worker-pool` auto-lifts only when ordering is not load-bearing. The compiler sees ordering *dependence* (e.g. increment), but cannot know the caller's semantic requirement that increments be seen in order.
- **Connection-affinity contract.** `session-affinity-state`'s "session ID stable for connection lifetime" is a protocol contract with the client, not a compile-time fact.

**Research leaning, cross-run:** pragmas serve **two distinct roles** — load-bearing evidence (fills classifier gaps, may move SUGGEST → AUTO) vs. pure overrides (waives a specific refusal code). The two are not the same and the ADR-0021 draft must separate them. gpt-5.4's pragma-bridge framing ("additive evidence, not override") is narrower than opus's but compatible — gpt-5.4 is scoping the evidence role; opus is also acknowledging legitimate overrides for edge cases.

## 7. The TERMINAL class in v1 — what's left after eight archetypes

The terminal set shrinks meaningfully under this research. Before: all mutex-using code was terminal-by-refusal-code. After: most mutex-using code is AUTO or SUGGEST, with terminal reserved for a smaller set of genuine distribution obstacles. The residual class:

1. **Embedded durable-client composites.** Pocketbase's `MLV2_EMBEDDED_DB_APP_ROOT`. Unchanged; load-bearing.
2. **Fire-and-forget goroutine spawns over mutable closures.** Miniflux fever/googlereader `go func(){}()` without join. No v1 archetype captures anonymous spawn without lifecycle vocabulary.
3. **Distributed state machines.** Gitea `graceful.Manager` (init → running → shutdown → terminate). Pattern visible and coherent but no v1 archetype's emission sketch expresses ordered distributed transitions (Raft/CRDT territory).
4. **Cross-key invariant maps.** Maps whose semantics rely on sum-or-union of all entries in one process (e.g. "active campaign count = len(pipes)"). Sharding breaks the invariant.
5. **Self-tuning periodic loops.** Intervals derived from prior tick's result — not expressible in the platform-scheduler transform.

**Is this class shrinking, stable, or absorbing refusals as the vocabulary grows? Shrinking meaningfully.** This is load-bearing for the Monolift thesis: the refusal surface is not a stable property of the input program; it is a property of the classifier's vocabulary. As the vocabulary grows (disciplined by the gates), terminal refusal contracts.

## 8. Tensions the research surfaced

### 8.1 Archetype labels compete on the same region

Several regions fit multiple archetypes:

- Caddy `Handler.connections` fits `serialized-actor` (mutex-protected state on receiver) *and* `keyed-partitioned-state` (connections keyed by connection ID). The distinction is emission-driven. Either transform works.
- Pocketbase `tools/store.Store[K,T]` fits `keyed-partitioned-state` (always) and `ttl-cache` (when entries carry expiry). Distinguishing evidence is whether value-carries-expiry is SSA-visible at the value type.
- Mattermost Hub fits `keyed-partitioned-state` + `fanout-publisher` + `session-affinity-state` simultaneously — this is where gpt-5.4's `connection-hub-buffer` composite lens applies.

**Research finding: archetypes are overlapping lenses on the region space, not a partition.** The catalog does not force uniqueness. The compiler picks the more-constrained archetype when multiple fit, because its transform has more structure. ADR-0022 codifies the precedence.

### 8.2 The "admitted handler + refused state" pairing

Caddy is the clearest instance. `ServeHTTP` is pragma-admitted; the connections-map state it reaches is refused. In v1 the resolution is: **state travels with the archetype, not with the handler**. Handler stays in `replicated-stateless-service`; the connections-map state becomes a named `keyed-partitioned-state` service the handler calls.

### 8.3 Worker-pool-consumer is not a state class — it is a pairing

Miniflux validates this. A channel-fed worker pool *collapses into* `bounded-worker-pool` state class + `replicated-stateless-service` admission, coordinated by external state. The archetype vocabulary distinguishes the source-level pattern from the state class because the pattern is shared across multiple state-class-level outcomes.

### 8.4 Pragmas as load-bearing evidence vs. overrides

See §6. The largest tension. Two archetypes (`periodic-invocation`, `session-affinity-state`) cannot cleanly auto-lift without user-declared evidence. ADR-0021 should separate the two pragma roles.

### 8.5 Vocabulary size — how much compression is safe?

Opus kept 7, gpt-5.4 compressed to 4, gemini kept 6. The synthesis settles on 8 (opus's 7 + gemini's `filesystem-bound-singleton`). The question is whether gpt-5.4's aggressive compression is safer than opus's finer splits. Synthesis view: **opus's splits earn their keep via distinguishing evidence signals and distinct transforms**. `ttl-cache` produces a different emission sketch (managed cache adapter) than `serialized-actor` (actor harness). Keeping them separate preserves this. gpt-5.4's merge is legitimate for reports; opus's split is legitimate for the classifier. Both can coexist if ADR-0019's remediation surface lets the compiler report at the composite level even when the classifier operates at the narrower level.

## 9. The primary engineering output

**Candidate state-class additions for ADR-0016** — the primary output of the research. Each is named by the archetype it enables, with evidence conditions cited to `docs/specs/liftability-properties.md` and proposed classifier signals.

1. `serialized-actor`
2. `bounded-worker-pool` (gpt-5.4 proposed name `queued-workset` — directly actionable alternative)
3. `periodic-invocation` (gpt-5.4 proposed name `scheduled-reconciler` — equivalent)
4. `keyed-partitioned-state`
5. `fanout-publisher`
6. `ttl-cache`
7. `session-affinity-state`
8. `filesystem-bound-singleton` (gemini-sourced)

Plus five new classifier evidence signals (§6). See `distribution-archetypes-followups.md` for the formal proposal structure per state class.

## 10. Cross-target matrix — research impact measurement

| archetype | listmonk | caddy | pocketbase | miniflux | gitea | mattermost | currently-refused-but-shown-auto-liftable |
|---|---|---|---|---|---|---|---|
| serialized-actor | — | 2 | 2 | 1 | 4 | 1 | 10 |
| bounded-worker-pool | 1 | — | — | (adm) | 1 | 1 | 3 |
| periodic-invocation | 2 | 2 | 1 | 4 | 1 | 1 | 11 |
| keyed-partitioned-state | 1 | 1 | 1 | — | 2 | 1 | 6 |
| fanout-publisher | 1 | — | 1 | — | 1 | (adm) | 3 |
| ttl-cache | 2 | 1 | 1 | — | 1 | 2 | 7 |
| session-affinity-state | — | 1 | — | — | 3 | 1 | 5 |
| filesystem-bound-singleton | — | 1 | — | — | 2 | — | 3 |
| **total AUTO** | **7** | **8** | **6** | **5** | **15** | **7** | **48** |

**Headline.** Approximately 48 currently-refused regions across the six evaluation targets would become auto-liftable if the classifier learned the eight archetypes in this catalog. This is the concrete measurement of what SPRINT-0013 buys when the follow-up ADR-0016 additions land.

Counts merge opus's and gemini's citations, cross-checked against gpt-5.4 where its narrower vocabulary generates equivalent AUTO labels via composition. Exact per-region citations live in the three run directories and the per-target annotations at `docs/research/annotations/<target>.md`.

## 11. Pointers and cross-links

- **Catalog (per-archetype detail):** `docs/research/archetype-catalog-v1.md`
- **Per-target annotations (composite, cross-run):** `docs/research/annotations/<target>.md`
- **Follow-ups (4 buckets):** `docs/research/distribution-archetypes-followups.md`
- **Individual parallel runs (preserved as source artifacts):**
  - `docs/research/runs/opus/` — deepest walk, structural framing, 5 evidence signals
  - `docs/research/runs/gpt-5.4/` — concise vocabulary, per-archetype boundaries, connection-hub-buffer composite
  - `docs/research/runs/gemini/` — broad bundle coverage, filesystem-bound-singleton, God-Object framing
  - `docs/research/runs/gemini-run-1/`, `docs/research/runs/gemini-run-2/` — earlier attempts preserved for transparency

## 12. What the next sprint should do

The research succeeded if the design space is navigable. It is. Concrete next moves, in suggested order:

1. **Draft ADR-0020** (auto-lift evidence thresholds) to codify the per-archetype AUTO conditions.
2. **Draft ADR-0019** (archetype-driven remediation surface) to formalize the SUGGEST output format.
3. **Draft ADR-0021** (pragmas as load-bearing evidence vs. overrides) to settle the largest surfaced tension.
4. **Land the first ADR-0016 state-class addition** — recommend `bounded-worker-pool` + `periodic-invocation` as the highest-yield first pair (strongest coverage, most externalization-affine, lowest evidence-gap risk).
5. **ADR-0022** (composite-archetype regions) after the first single-archetype state class lands, to codify how overlapping archetypes compose.

Implementation spikes (classifier signals, rule implementations, transform codegen, report-format additions) follow the ADRs per the scope fence. See follow-ups Bucket D.
