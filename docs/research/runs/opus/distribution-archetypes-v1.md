# Distribution archetypes: research note v1 (opus run)

**SPRINT-0013 narrative deliverable.** Written so a collaborator
reading cold — catalog not yet opened — can learn what currently-refused
patterns the corpus carries, what transforms each implies, what
state-class additions to ADR-0016 would unlock them, and where the
auto-lift-vs-suggest boundary sits per archetype and why. One of three
parallel runs; synthesis merges later.

Cross-links: `archetype-catalog-v1.md` (per-entry detail),
`annotations/<target>.md` (per-target walks),
`distribution-archetypes-followups.md` (candidate ADR-0016 additions,
open questions, implementation spikes).

---

## 1. The research question, restated

Monolift today auto-lifts narrowly: the ADR-0016 rule stack admits
`immutable-captured-config` and `replicated` (plus the
`externalized-durable` case where the developer declared
`state=external`) and refuses almost everything stateful. Mutexes,
channels, goroutines, shared mutable globals, pointer aliasing —
these reach the receiver-scope or boundary gates in
`docs/specs/liftability-properties.md` and route to refusal codes
(`MLV2_SHARED_MUTABLE_STATE`, `MLV2_CHANNEL_BOUNDARY`,
`MLV2_POINTER_ALIAS_UNSUPPORTED`, …).

But many of those refusals correspond to **distribution patterns with
known transforms**: a mutex-protected struct is a singleton actor, a
channel-fed goroutine pool is a worker queue, a keyed map under a lock
is a sharded service, a periodic background goroutine is a scheduled
invocation.

The question this sprint asked:

> Which currently-refused patterns have enough structure that the
> compiler could auto-lift them with a named transform, and what would
> the classifier need to learn to do it?

The primary product is the **AUTO surface**: currently-refused regions
that would become auto-liftable if the classifier recognized a named
archetype and applied its transform. The `SUGGEST` surface is the
honest fallback when static evidence is strong but not conclusive.
`TERMINAL` is what's left.

---

## 2. Corpus walk at a glance

Six targets were walked region-by-region under a uniform annotation
schema (`annotations/README.md`):

| target | files | AUTO | SUGGEST | TERMINAL |
|---|---|---|---|---|
| listmonk | 92 | 4 | 5 | 3 |
| caddy | 306 | 7 | 4 | 4 |
| pocketbase | 445 | 5 | 4 | 5 |
| miniflux | 407 | 6 | 2 | 3 |
| gitea | 2875 | 18 | 3 | 6 |
| mattermost | 2153 | 11 | 4 | 5 |
| **totals** | **6278** | **51** | **22** | **26** |

The AUTO column is the primary finding: 51 regions across the corpus
that the research argues would become auto-liftable if the classifier
learned a named archetype. See `annotations/<target>.md` for each.

A meta-observation: the large targets (gitea, mattermost) did not
inflate archetype vocabulary — they *concentrated* AUTO findings in a
small number of infrastructure bundles (modules/queue, modules/session,
modules/eventsource, server/channels/app/platform) and left the
thousands of domain-service files in the ADMITTED baseline. This is
what the sprint plan anticipated: owned-directory bundling kept
coverage honest while the vocabulary stayed small.

---

## 3. The v1 archetype vocabulary

Seven archetypes survived all four gates (coverage, evidence, emission,
boundary); see catalog for per-gate pass records. Each is a pair of
(archetype name, candidate ADR-0016 state class), where the archetype
is the pattern you see in the source and the state class is the
classifier-internal vocabulary that would recognize it.

| archetype | state class (ADR-0016 addition) | pattern in the source |
|---|---|---|
| `serialized-actor` | `serialized-actor` | struct + mutex; receiver-scoped state; no pointer escape |
| `bounded-worker-pool` | `bounded-worker-pool` | struct + chan + N goroutines consuming; serializable jobs |
| `periodic-invocation` | `periodic-invocation` | goroutine + `time.Ticker` / `time.Sleep` loop; idempotent body |
| `keyed-partitioned-state` | `keyed-partitioned-state` | `map[K]V` under mutex; every access keyed |
| `fanout-publisher` | `fanout-publisher` | `[]chan T` or `map[K]chan T` under mutex; Publish iterates |
| `ttl-cache` | `ttl-cache` | mutex-guarded map with TTL entries; cache-miss loader exists |
| `session-affinity-state` | `session-affinity-state` | state keyed by session/connection ID; lifetime is connection-scope |

Five retirements were recorded (pipeline-stage, ephemeral-worker,
lifecycle-state-machine, websocket-fanout-hub, keyed-queue-state-guard);
see the catalog's retirements section for "why it didn't survive"
paragraphs.

---

## 4. The per-archetype boundary model

For each archetype the research states AUTO / SUGGEST / TERMINAL
thresholds in concrete evidence conditions. Excerpting:

### `serialized-actor`

- **AUTO** iff protected state is wholly receiver-owned, no
  pointer-to-field escape, mutex span encloses every store,
  no reachability into reflect/unsafe.
- **SUGGEST** iff receiver-scope looks right but the mutex also
  protects external-client handles the compiler cannot verify are
  externalizable.
- **TERMINAL** iff the mutex guards a structure embedding an
  un-externalizable durable client (SQLite handle) — composite refusal.

### `bounded-worker-pool`

- **AUTO** iff job type serializable, handler holds
  `effects.no-global-writes`, pool size is static config, per-job
  ordering is not load-bearing.
- **SUGGEST** iff per-key FIFO ordering is required.
- **TERMINAL** iff handler has unbounded internal fanout or the pool
  itself is unbounded.

### `periodic-invocation`

- **AUTO** iff body is idempotent or tolerates skip/duplicate; no
  captured mutable state; interval is config-driven.
- **SUGGEST** iff body carries counter-style state not reducible to
  external storage.
- **TERMINAL** iff interval is dynamically self-tuning from prior
  tick's result.

### `keyed-partitioned-state`

- **AUTO** iff every access is keyed and iteration (if any) is
  idempotent-per-shard background cleanup.
- **SUGGEST** iff key-free iteration appears in hot paths with
  user-visible semantics.
- **TERMINAL** iff the map encodes cross-key invariants (sum-of-values).

### `fanout-publisher`

- **AUTO** iff event type serializable, subscribers independent, no
  cross-event ordering requirement.
- **SUGGEST** iff ordering across subscribers is required, or
  subscriber churn is high.
- **TERMINAL** iff fanout encodes a distributed transaction across
  subscribers.

### `ttl-cache`

- **AUTO** iff value serializable, no pointer-to-in-process-state in
  value, source-of-truth is elsewhere.
- **SUGGEST** iff value holds callback / function-pointer.
- **TERMINAL** iff the cache *is* the source of truth.

### `session-affinity-state`

- **AUTO** iff session ID stable for connection lifetime, state purely
  per-session, no cross-session invariants.
- **SUGGEST** iff state references cross-session shared objects.
- **TERMINAL** iff state lifetime is unbounded beyond connection (e.g.
  multi-connection user-level state with consistency).

---

## 5. Is the boundary a single threshold, per-archetype, or structural?

This is one of the open questions the brief asked the research to
characterize, not resolve. Our reading across the seven surviving
archetypes: **the boundary is structural, with two load-bearing axes**:

1. **Evidence locality.** When the distinguishing evidence is local
   and closed-form (visible in one SSA function without callgraph
   expansion), the archetype auto-lifts. When distinguishing evidence
   depends on runtime behavior or external-library contracts the
   compiler cannot inspect, the archetype routes to SUGGEST.
2. **Externalization affinity.** When the archetype's natural
   transform moves state to an external substrate (managed cache,
   broker, scheduler) whose semantics the archetype's invariants
   match one-for-one, auto-lift is safe. When the transform requires
   an internal substrate (serial actor harness, custom dispatch
   loop) *and* the state may cross the substrate boundary, the case
   routes to SUGGEST unless the compiler can prove the boundary is
   tight.

Concretely: `periodic-invocation`, `fanout-publisher`,
`bounded-worker-pool`, and `ttl-cache` auto-lift well because all
four externalize to managed substrates (platform scheduler, broker,
managed cache). `serialized-actor`, `keyed-partitioned-state`, and
`session-affinity-state` auto-lift only when the compiler can prove
state is receiver-owned with no pointer-escape — the "tight boundary"
condition.

So: not a single threshold. Not purely per-archetype either — the
two axes cut across archetypes. Structural in a small-dimensional way.

---

## 6. Compiler cannot know this statically — the evidence-gap separation

The brief specifically asks where auto-lift pressure reveals evidence
gaps, and separates *threshold-tunable* gaps (classifier could collect
the signal; it just doesn't yet) from *irreducible* gaps (static
analysis cannot decide; pragma or user annotation is the only bridge).

### Threshold-tunable gaps — proposed new classifier signals

These are signals the classifier *could* collect and would resolve
multiple SUGGEST cases into AUTO:

- **`keyed-access-invariant`** — "every call site reaching this map
  indexes by key derived from input". SSA-visible. Would move
  `keyed-partitioned-state` cases from SUGGEST → AUTO.
- **`cache-value-no-pointer-escape`** — "value type carries no pointer
  to other in-process state". `go/types` + SSA on struct fields.
  Would move `ttl-cache` cases with ambiguous value types → AUTO.
- **`session-id-keyed-access`** — "access invariant keyed by a
  connection-accept-time ID, not a request-time ID". SSA + callgraph
  reachability. Would move `session-affinity-state` SUGGEST → AUTO.
- **`bounded-pool-invariant`** — "pool size is provably bounded by a
  static constant or config value, not a runtime-unbounded fallback".
  SSA on goroutine-spawning loops. Would move `bounded-worker-pool`
  SUGGEST cases into AUTO where the fallback spawn was the reason for
  SUGGEST (pocketbase P6 JS VM pool, pocketbase P9 S3 uploader).

These live in the follow-up doc.

### Irreducible gaps — pragma / annotation territory

- **External-library contract atomicity.** Mattermost's `cache.Cache`
  interface (MM4, MM5) has external documentation claiming
  Scan/GetMulti/RemoveMulti atomicity. The compiler cannot verify an
  interface contract. Only a pragma (or a trusted-library allowlist)
  can supply this evidence. This is the same shape as the
  `external-client-type` rule in ADR-0016.
- **Idempotency declarations.** `periodic-invocation` requires the
  body to be idempotent. Static analysis can rule out some
  non-idempotency (writes to global mutable state), but cannot
  affirmatively prove idempotency in general. A pragma
  `idempotent=true` is load-bearing evidence here, not an override.
- **Per-key ordering declarations.** `bounded-worker-pool` can
  auto-lift only when ordering is not load-bearing across jobs. The
  compiler can identify an ordering *dependence* (e.g. `pipe.sent++`
  increment), but cannot know the caller's semantic requirement that
  increments must be seen in order. Pragma territory.
- **Connection-affinity contract.** `session-affinity-state`'s
  invariant ("session ID is stable for connection lifetime") is a
  contract with the client protocol, not a compile-time fact. The
  classifier can see the struct shape; the invariant is external.

These route to ADR-0019's pragma surface (see follow-ups).

---

## 7. What the terminal refusal class looks like in v1

After seven archetypes and five retirements, the TERMINAL set is:

1. **Embedded durable-client composites.** Pocketbase's
   `MLV2_EMBEDDED_DB_APP_ROOT` pattern. Unchanged; load-bearing.
2. **Fire-and-forget goroutine spawns over mutable closures.**
   Miniflux fever/googlereader `go func(){…}()` without join. No v1
   archetype captures anonymous spawn without lifecycle vocabulary.
3. **Distributed state machines.** Gitea `graceful.Manager` lifecycle
   (init → running → shutdown → terminate). Pattern is visible and
   coherent, but no v1 archetype's emission sketch expresses
   distributed ordered state transitions.
4. **Cross-key invariant maps.** Maps whose semantics rely on the
   sum or union of all entries in one process (e.g. "active campaign
   count = len(pipes)"). Sharding breaks the invariant.
5. **Self-tuning periodic loops.** Intervals derived from the prior
   tick's result — not expressible in the platform-scheduler transform.

Is this class shrinking, stable, or absorbing refusals as the
vocabulary grows? **Shrinking meaningfully.** Before this research,
all mutex-using code was terminal-by-refusal-code; after this
research, most mutex-using code is AUTO or SUGGEST, with terminal
reserved for a smaller set of genuine distribution obstacles (embedded
durables, fire-and-forget without lifecycle, distributed state machines).
This is load-bearing for the Monolift thesis: the refusal surface is
not a stable property of the input program; it is a property of the
classifier's vocabulary. As the vocabulary grows (disciplined by the
gates), terminal refusal contracts.

---

## 8. Tensions the research surfaced

### 8.1 Archetype labels compete on the same region

Several regions fit multiple archetypes:

- caddy `Handler.connections` fits `serialized-actor` (mutex-protected
  state on receiver) *and* `keyed-partitioned-state` (connections are
  keyed by connection ID). The distinction is emission-driven: a
  serial actor for the connection registry or a sharded-by-key
  service. Either works; the v1 catalog keeps both archetypes and
  notes the region as a composite citation.
- pocketbase `tools/store.Store[K,T]` fits `keyed-partitioned-state`
  (always) and `ttl-cache` (when entries carry expiry). The
  distinguishing evidence is whether value-carries-expiry is
  SSA-visible at the value type.

The research finding: **archetypes are not partitioning the region
space; they are overlapping lenses on it**. The catalog does not
force uniqueness. The compiler would pick the more-constrained
archetype when both fit, because its transform has more structure.

### 8.2 The "admitted handler + refused state" pairing

Caddy is the clearest instance. `ServeHTTP` is pragma-admitted; the
connections-map state it reaches is refused. In v1 the resolution is:
**state travels with the archetype, not with the handler**. Handler
stays in `replicated-stateless-service`; the connections-map state
becomes a named `keyed-partitioned-state` service the handler calls.
This is a research finding, not an engineering commitment.

### 8.3 Worker-pool-consumer is not a state class, it is a pairing

The miniflux walk proves this. A channel-fed worker pool *collapses
into* `bounded-worker-pool` state class + `replicated-stateless-service`
admission, coordinated by external state. The archetype vocabulary
distinguishes the input pattern (what the source looks like) from the
state class (what the classifier records), because the pattern is
shared across multiple state-class-level outcomes.

### 8.4 Pragmas as load-bearing evidence

Two archetypes (`periodic-invocation`, `session-affinity-state`)
cannot cleanly auto-lift without user-declared evidence — idempotency
and connection-affinity, respectively. This reopens a latent question
from ADR-0017: are pragmas overrides only (the developer saying "lift
this even though"), or can they supply evidence the compiler relies on
("this is idempotent — classifier, use this fact")?

The research leaning, based on the corpus: **some pragmas are
load-bearing evidence**, specifically at the boundaries where static
analysis is provably-incomplete (external-contract atomicity,
idempotency, connection-affinity semantics). Others are overrides.
The ADR-0019 draft should separate the two roles. This is the largest
surfaced tension; it lives in the follow-ups as an open question.

---

## 9. What the primary engineering output is

The **candidate state-class additions for ADR-0016** are the point of
the research. Each is:

- `serialized-actor`
- `bounded-worker-pool`
- `periodic-invocation`
- `keyed-partitioned-state`
- `fanout-publisher`
- `ttl-cache`
- `session-affinity-state`

Each is named by the archetype it enables; each carries evidence
conditions cited to `docs/specs/liftability-properties.md` and
proposed classifier signals (three new ones needed); each carries a
transform sketch the research argues is writeable in ≤30 lines of Go
pseudocode. See `archetype-catalog-v1.md` for per-entry detail and
`distribution-archetypes-followups.md` for the formal proposals.

---

## 10. Cross-target matrix — the research's measurement of impact

| archetype | listmonk | caddy | pocketbase | miniflux | gitea | mattermost | currently-refused-but-shown-auto-liftable |
|---|---|---|---|---|---|---|---|
| serialized-actor | — | 2 | 2 | 1 | 4 | 1 | 10 |
| bounded-worker-pool | 1 | — | — | (adm) | 1 | 1 | 3 |
| periodic-invocation | 2 | 2 | 1 | 4 | 1 | 1 | 11 |
| keyed-partitioned-state | 1 | 1 | 1 | — | 2 | 1 | 6 |
| fanout-publisher | 1 | — | 1 | — | 1 | (adm) | 3 |
| ttl-cache | 2 | 1 | 1 | — | 1 | 2 | 7 |
| session-affinity-state | — | 1 | — | — | 3 | 1 | 5 |
| **total AUTO** | **4** | **7** | **5** | **6** | **18** | **11** | **51** |

**Headline.** 51 currently-refused regions across the six evaluation
targets would become auto-liftable if the classifier learned the
seven archetypes in this catalog. That is the research's concrete
measurement of what this sprint buys when the follow-up ADR-0016
additions land.

(Numbers are this run's opus-side counts; the synthesis step
produces the canonical number after merging the three parallel runs.)
