# Archetype catalog v1 — SPRINT-0013 composite

**Status:** v1. Composite across three parallel research runs. See `distribution-archetypes-v1.md` for the narrative and `runs/{opus,gpt-5.4,gemini}/archetype-catalog-v1.md` for per-run depth.

Eight archetypes survived all four gates (coverage, evidence, emission, boundary). Six retirements recorded with one-paragraph "why it didn't survive" notes. Five new evidence signals proposed for the classifier.

## Vocabulary discipline — the four gates

Every surviving archetype passes all four gates with per-gate outcome recorded:

1. **Coverage gate.** Labels ≥2 regions across ≥2 targets, or carries an argued exception.
2. **Evidence gate.** Distinguishable from nearest neighbor by an evidence signal the classifier already collects (citing `docs/specs/liftability-properties.md` or `pkg/compiler/stateclass/`) or a single named signal proposed for addition.
3. **Emission gate.** A ≤30-line Go pseudocode transform sketch is writeable. Two archetypes with essentially the same sketch merge.
4. **Boundary gate.** Auto-lift / suggest / refuse thresholds are stated as concrete evidence conditions, not "when the compiler is confident."

ADR / v2-contract naming-collision check executed across all surviving names against ADR-0015/0016/0017/0018 and `docs/specs/monolift-v2-contract.md`. No collisions; no renames.

## v0 → v1 vocabulary summary

| v0 label | v1 outcome | renamed to | note |
|---|---|---|---|
| singleton-actor | kept | `serialized-actor` | narrow sense: serialize access to receiver-scoped state |
| worker-pool-consumer / queue-consumer | kept | `bounded-worker-pool` | gpt-5.4 alternative name: `queued-workset` |
| periodic-scheduler | kept | `periodic-invocation` | gpt-5.4 alternative name: `scheduled-reconciler` |
| sharded-keyed-state / sharded-stateful-service | kept | `keyed-partitioned-state` | gpt-5.4/gemini retired; opus+synthesis kept |
| event-bus-publisher | kept | `fanout-publisher` | subscriber side is not a distinct archetype |
| event-bus-subscriber | merged | — | absorbed into `serialized-actor` or `bounded-worker-pool` per state effect |
| ttl-cache-managed | kept | `ttl-cache` | gpt-5.4 folds into singleton; opus+synthesis kept (distinct emission) |
| session-scoped-state | kept | `session-affinity-state` | gpt-5.4 alternative (broader): `connection-hub-buffer` |
| pipeline-stage | **retired** | — | collapses to periodic + worker-pool composition |
| ephemeral-worker | **retired (fissioned)** | — | splits into session-affinity (lifecycled) or TERMINAL (fire-and-forget) |
| replicated-stateless-service | kept as ADMITTED baseline | — | not part of AUTO/SUGGEST triage |

**Added by synthesis from gemini run:**

| proposed | outcome | note |
|---|---|---|
| `filesystem-bound-singleton` | kept | distinguishing evidence (OS/FS calls in closures) strong enough to warrant separate state class; transform differs (object-store/sidecar vs. actor harness) |

**Retired proposals:**

| proposed | outcome | reason |
|---|---|---|
| `lifecycle-state-machine` | retired for v1, flagged | fails emission gate: no v1 property captures distributed state-machine transitions. Gitea `graceful.Manager` is canonical; flag for ADR-0023. |
| `websocket-fanout-hub` | retired as composite | 1-target coverage (mattermost Hub); expressible as `keyed-partitioned-state` + `fanout-publisher` + `session-affinity-state`. Preserved as gpt-5.4's `connection-hub-buffer` composite lens. |
| `keyed-queue-state-guard` | retired | 1-target (gitea baseChannel); broker dedup subsumes. Collapses into `bounded-worker-pool` + broker config. |
| `sharded-stateful-service` (as distinct from keyed-partitioned-state) | retired | gpt-5.4/gemini both retired; opus merged into `keyed-partitioned-state` under renamed label. |
| `distributed-cache-wrapper` | retired | gpt-5.4-surfaced; merged into `serialized-singleton-owner` or left terminal. The transform boundary was never "generic distributed cache" with distinct evidence from a single-owner service. |
| `config / control plane` | retired | gpt-5.4-surfaced; real architecture but not an auto-lift surface. |

---

## v1 archetypes

For full emission sketches, cross-target citations, and gate pass records, see the opus run's per-entry catalog (`runs/opus/archetype-catalog-v1.md`) — it is the most exhaustive and was used as the spine. Summary entries follow; each notes where other runs diverge.

### 1. `serialized-actor`

**Definition.** Stateful struct whose operations all mutate receiver-scoped state under a single mutex. No cross-instance shared state. No pointer-to-field escape to external callers.

**Candidate state class (ADR-0016).** `serialized-actor`. Rule inference when (a) struct field is `sync.Mutex`/`sync.RWMutex`, (b) every store site on receiver-owned fields lies inside Lock/Unlock span, (c) no pointer-to-protected-field escape is observable in SSA.

**Evidence signals.** `effects.no-param-heap-mutation` Hold (gate), `effects.no-param-escape` Hold (advisory → promote), `boundary.no-sync-primitives` Violate *tolerated* under the reinterpretation "mutex is semantic, not a boundary violation", `effects.no-global-writes` Hold. New proposed signal: `mutex-encloses-store-invariant`.

**Transform.** Wire-level serialized-actor harness: single goroutine consuming a command mailbox; method calls become RPC. Runtime dep: serial-dispatch harness (no external broker). User-facing API: unchanged (method calls → RPC).

**Thresholds.** AUTO when protected state is wholly receiver-owned, no escape via returned pointers, mutex span encloses every store, no reachability into reflect/unsafe. SUGGEST when receiver-scope looks right but mutex also protects external state. TERMINAL when mutex guards an un-externalizable durable client (SQLite).

**Citations (≥2 targets required).** caddy C5, C7, C10, C11; pocketbase P1, P5; gitea G3, G4, G15, G18; mattermost MM11; miniflux M6. **Gates:** Coverage ✓ (6 targets), Evidence ✓, Emission ✓, Boundary ✓.

**Cross-run note.** gpt-5.4 calls this `serialized-singleton-owner` (broader, includes some TTL-cache territory); gemini calls this `Singleton Actor` (broadest).

---

### 2. `bounded-worker-pool`

**Definition.** N-goroutine pool reading serializable jobs from a shared channel (explicit or via semaphore); each worker processes a job independently; no mutable shared state with peers. Pool size statically bounded.

**Candidate state class.** `bounded-worker-pool` (synonym: gpt-5.4 `queued-workset`). Rule inference when (a) struct holds `chan T` field with serializable T, (b) fixed-count loop spawns goroutines consuming it, (c) handler accesses no package-global mutable state, (d) handler has error-return vocabulary.

**Evidence signals.** `boundary.no-streaming-values` Violate tolerated (channel on boundary replaced by broker), `boundary.serializable-via-custom-encoding` Hold (gate), `lifecycle.long-running-loop` bias (consumer loop), `effects.no-global-writes` Hold on handler. New proposed signal: `bounded-pool-invariant` — pool size statically bounded, not growable on overflow.

**Transform.** Broker-backed queue + N replicas; per-job handler as stateless function. Runtime dep: message broker client (NATS/SQS/Pub-Sub/Rabbit). User-facing API: enqueue site replaces direct channel send with broker publish; local callers unchanged.

**Thresholds.** AUTO when job type serializable, handler holds `effects.no-global-writes`, pool size static-config driven, per-job ordering not load-bearing. SUGGEST when ordering potentially load-bearing. TERMINAL when handler has unbounded internal fanout or pool is unbounded.

**Citations.** listmonk L2; gitea G1; mattermost MM6; miniflux ADMITTED baseline. **Gates:** Coverage ✓ (4 targets), Evidence ✓, Emission ✓, Boundary ✓.

**Cross-run note.** All three runs converge; strongest AUTO candidate universally. gpt-5.4 and gemini framed it slightly more broadly (queue-consumer generalized) but transforms converge.

---

### 3. `periodic-invocation`

**Definition.** Long-running goroutine whose body is a `time.Ticker`/`time.Sleep`-driven loop invoking an idempotent function. No captured mutable state beyond what is immediately consumed.

**Candidate state class.** `periodic-invocation` (synonym: gpt-5.4 `scheduled-reconciler`).

**Evidence signals.** `lifecycle.long-running-loop` Hold with `time.Ticker.C`/`time.Sleep` on every branch. `lifecycle.no-async-fork` Violate (admission-compatible; distribution moves goroutine to platform scheduler). `effects.no-global-writes` Hold. `contract.error-last` Hold. Pragma-supplied evidence: `idempotent=true` declaration — load-bearing, not override.

**Transform.** Platform-scheduler-triggered invocation (cron, k8s CronJob, serverless scheduled trigger). Runtime dep: platform scheduler. User-facing API: `Start`/`Stop` calls become no-ops — scheduler owns lifecycle.

**Thresholds.** AUTO when body is idempotent or tolerates skip/duplicate; no captured mutable state; interval config-driven. SUGGEST when body mutates local state persisting across invocations. TERMINAL when interval is self-tuning from prior tick's result.

**Citations.** listmonk L1, L3; caddy C1, C2; pocketbase P2; miniflux M1–M4; gitea G14; mattermost MM8. **Gates:** Coverage ✓ (6 targets — strongest coverage in the corpus), Evidence ✓, Emission ✓, Boundary ✓.

---

### 4. `keyed-partitioned-state`

**Definition.** `map[K]V` protected by mutex, every access keyed. Keys partition state into independent shards.

**Candidate state class.** `keyed-partitioned-state`. Rule inference when (a) protected field is map-typed, (b) every access site uses a key from input, (c) no iteration over all entries in hot paths.

**Evidence signals.** `boundary.no-sync-primitives` Violate tolerated (semantic-mutex reinterpretation). `effects.no-global-writes` Hold. New proposed signal: `keyed-access-invariant`.

**Transform.** Consistent-hash router + per-shard service, or managed KV store (Redis cluster, DynamoDB). Per-key atomicity preserved; cross-key linearization not (original mutex impl already violates it if per-key locks). User-facing API: keyed operations become RPC; iteration requires new API.

**Thresholds.** AUTO when every call site keyed, iteration is background-cleanup idempotent-per-shard. SUGGEST when key-free iteration appears in hot paths. TERMINAL when map encodes cross-key invariants.

**Citations.** listmonk L5; caddy C5 composite; pocketbase P3; gitea G2, G18 composite; mattermost MM1 composite. **Gates:** Coverage ✓ (5 targets), Evidence ✓ (one proposed signal pending), Emission ✓, Boundary ✓.

**Cross-run note.** gpt-5.4 retired this as insufficient coverage in its walk; gemini also retired `Sharded Stateful Service`. Opus kept it and the synthesis agrees — evidence is thinner than other archetypes but the transform is distinct enough to warrant a separate state class.

---

### 5. `fanout-publisher`

**Definition.** Producer struct holding mutex-protected collection of subscriber channels (`[]chan T` or `map[K]chan T`). Publish iterates and sends. Subscribers independent; event serializable.

**Candidate state class.** `fanout-publisher`.

**Evidence signals.** `boundary.serializable-via-custom-encoding` Hold on event type. `effects.no-global-writes` Hold on subscriber set. `lifecycle.long-running-loop` absent on publisher (publish is synchronous per call).

**Transform.** Managed pub/sub broker; subscribers become named services consuming the topic. Invariants: at-least-once delivery, subscriber independence.

**Thresholds.** AUTO when event type serializable, subscribers independent, no cross-event ordering. SUGGEST when ordering across subscribers required or high subscriber churn. TERMINAL when fanout encodes distributed transaction.

**Citations.** listmonk L4; pocketbase P4; gitea G7; mattermost MM7 (ADMITTED — validates shape). **Gates:** Coverage ✓ (4 targets), Evidence ✓, Emission ✓, Boundary ✓.

**Cross-run note.** gpt-5.4 merges this into `connection-hub-buffer` when routing-key + register/unregister + replay co-occur. Opus keeps separate; synthesis preserves both — `fanout-publisher` for simple publish-to-channel-set; `connection-hub-buffer` (composite, ADR-0022 territory) when the three signals cluster.

---

### 6. `ttl-cache`

**Definition.** Key-value cache protected by mutex (or `sync.Map`) with TTL-based expiry via background cleanup or on-access check. Contents ephemeral; source of truth elsewhere.

**Candidate state class.** `ttl-cache`.

**Evidence signals.** `effects.no-global-writes` Hold. `boundary.serializable-via-custom-encoding` Hold on value type. New proposed signal: `cache-value-no-pointer-escape`.

**Transform.** Managed cache (Redis/memcached); background cleanup goroutine removed (managed eviction handles it). User-facing API: `Get/Set` preserved.

**Thresholds.** AUTO when value serializable, carries no in-process pointers, source-of-truth elsewhere. SUGGEST when value holds function-pointer/callback. TERMINAL when cache *is* source of truth.

**Citations.** listmonk L6, L7; caddy C7 (overlap with serialized-actor); pocketbase P3 (overlap with keyed-partitioned-state); gitea G10; mattermost MM4, MM5. **Gates:** Coverage ✓ (5 targets), Evidence ✓ (one proposed signal pending), Emission ✓, Boundary ✓.

**Cross-run note.** Only opus kept `ttl-cache` as a distinct archetype. gpt-5.4 merged into serialized-singleton; gemini didn't split. Synthesis keeps: the emission sketch (managed-cache adapter) is distinct enough from actor-harness to warrant separate state class, and several corpus regions (listmonk auth cache, mattermost session cache, gitea ephemeral cache) fit this shape precisely without fitting the broader singleton shape.

---

### 7. `session-affinity-state`

**Definition.** State keyed by session/connection ID (not request-input-derived). Lifetime bounded by connection lifecycle. State mutations per-session; no cross-session shared mutable state.

**Candidate state class.** `session-affinity-state`.

**Evidence signals.** `effects.no-global-writes` Hold. `boundary.no-streaming-values` Hold if channel is internal to per-session actor. New proposed signal: `session-id-keyed-access` — access invariant is stronger than keyed-partitioned-state's because ingress is connection-accept, not request.

**Transform.** Session-affinity-aware load balancer (consistent-hash on session ID, sticky routing); per-session actor lives on routed replica for session lifetime.

**Thresholds.** AUTO when session ID stable across connection, state purely per-session, no cross-session invariants, replicas can carry session to close without migration. SUGGEST when state references cross-session objects (mattermost cluster). TERMINAL when session state is connection-lifetime-unbounded.

**Citations.** caddy C6; gitea G11, G12, G13; mattermost MM2. **Gates:** Coverage ✓ (3 targets), Evidence ✓ (one proposed signal pending), Emission ✓, Boundary ✓.

**Cross-run note.** gpt-5.4's broader `connection-hub-buffer` covers this territory plus replay-buffer + routing-key semantics. Synthesis keeps `session-affinity-state` narrow (per-connection lifecycled state) and treats `connection-hub-buffer` as a composite (ADR-0022) when the additional signals co-occur.

---

### 8. `filesystem-bound-singleton` (gemini-sourced)

**Definition.** State or operations bound to local OS filesystem (`os`, `filepath`, `os.File` / `os.Root` handles). Operations interact with disk directly.

**Candidate state class.** `filesystem-bound-singleton`. Rule inference when (a) struct holds file-handle or path-config fields, (b) methods invoke `os`/`filepath` functions, (c) no in-memory cache bridges between invocations.

**Evidence signals.** `effects.no-global-writes` Hold. New proposed signal: `filesystem-operations-idempotent` (SSA on method bodies — specific `os.Create`/`os.WriteFile` patterns).

**Transform.** Object-store client (S3, GCS, Azure Blob) OR sidecar with volume mapping. Runtime dep: object-store client (or sidecar runtime). User-facing API: paths become object keys; handle-based operations become request-scoped streams.

**Thresholds.** AUTO when filesystem operations are idempotent-on-retry, paths config-driven, no in-memory state across operations. SUGGEST when filesystem ops have in-process caching/locking beyond OS-level. TERMINAL when access encodes invariants over local-disk state that volume-mapping cannot preserve.

**Citations.** caddy (storage layer — `filestorage`); gitea (local storage module, process manager's local lock files); one more target citation weakens the coverage gate slightly — flagged as borderline but kept on emission-gate strength (transform is clearly distinct from other archetypes). **Gates:** Coverage ✓ (argued exception — evidence strength compensates for 2-target count), Evidence ✓, Emission ✓ (distinct from actor harness), Boundary ✓.

**Cross-run note.** Only gemini surfaced this archetype. Opus folded filesystem-bound state into `serialized-actor`; gpt-5.4 into `serialized-singleton-owner`. Synthesis kept as distinct because the transform (object-store adapter) and evidence signals (OS/FS calls) are differentiating enough that the compiler would generate meaningfully different code for these regions.

---

## v1 catalog state

- **8 archetypes survived** all four gates (opus contributed 7, gemini contributed 1 additional).
- **6 retirements** recorded (pipeline-stage, ephemeral-worker, lifecycle-state-machine, websocket-fanout-hub, keyed-queue-state-guard, distributed-cache-wrapper).
- **5 new evidence signals** proposed for the classifier (`keyed-access-invariant`, `cache-value-no-pointer-escape`, `session-id-keyed-access`, `bounded-pool-invariant`, `mutex-encloses-store-invariant`).
- **8 candidate state classes** for ADR-0016 (primary engineering output).
- **ADR / v2-contract naming collision check** executed — no collisions, no renames.

## Cross-target citation matrix

| archetype | listmonk | caddy | pocketbase | miniflux | gitea | mattermost | targets |
|---|---|---|---|---|---|---|---|
| serialized-actor | — | 2 | 2 | 1 | 4 | 1 | 5 |
| bounded-worker-pool | 1 | — | — | (adm) | 1 | 1 | 4 |
| periodic-invocation | 2 | 2 | 1 | 4 | 1 | 1 | 6 |
| keyed-partitioned-state | 1 | 1 | 1 | — | 2 | 1 | 5 |
| fanout-publisher | 1 | — | 1 | — | 1 | 1 (adm) | 4 |
| ttl-cache | 2 | 1 | 1 | — | 1 | 2 | 5 |
| session-affinity-state | — | 1 | — | — | 3 | 1 | 3 |
| filesystem-bound-singleton | — | 1 | — | — | 2 | — | 2 |

"(adm)" = ADMITTED baseline region (counts toward cross-target coverage for gate purposes).
