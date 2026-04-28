# Distribution-archetypes follow-ups (opus run)

**SPRINT-0013 deliverable.** Four buckets: candidate state-class
additions for ADR-0016 (primary engineering output), ADRs ripe to
draft, still-open empirical questions, implementation spikes that
should wait until ADRs exist.

## Bucket A — Candidate state-class additions for ADR-0016 (primary engineering output)

Each proposed state class names the archetype it enables, the evidence
conditions it requires, the transform it unlocks, and the corpus
targets where it earned its place.

### A1. `serialized-actor`

- **Archetype enabled:** `serialized-actor` — stateful struct with
  serialized access via mutex.
- **Evidence conditions (sound, gate):**
  - `effects.no-param-heap-mutation` Hold.
  - `effects.no-param-escape` Hold (advisory today; promote argument
    in the state-class addition).
  - `boundary.no-sync-primitives` Violate is tolerated at the
    receiver surface, with `syncPrimitiveRule` reinterpreted: mutex
    is semantic (actor serialization), not a boundary violation.
  - Every store on protected state lies inside Lock/Unlock span (new
    SSA check: `mutex-encloses-store-invariant`).
- **Transform unlocked:** wire-level serialized-actor harness. Single
  goroutine consuming a command mailbox; method calls become RPC.
- **Earned its place in:** caddy (C5, C7, C10, C11), pocketbase (P1,
  P5), gitea (G3, G4, G15, G18), mattermost (MM3, MM5, MM11), miniflux
  (M6).

### A2. `bounded-worker-pool`

- **Archetype enabled:** `bounded-worker-pool`.
- **Evidence conditions:**
  - Struct holds a `chan T` field; T satisfies
    `boundary.serializable-via-custom-encoding`.
  - A static-bounded loop spawns goroutines consuming the channel.
  - Consumer body holds `effects.no-global-writes` and
    `contract.error-last`.
  - Proposed new signal: `bounded-pool-invariant` — pool size is
    statically bounded (not growable on overflow).
- **Transform unlocked:** broker-backed queue + N replicas; per-job
  handler as stateless function.
- **Earned its place in:** listmonk (L2), gitea (G1), mattermost
  (MM6), miniflux (ADMITTED baseline).

### A3. `periodic-invocation`

- **Archetype enabled:** `periodic-invocation`.
- **Evidence conditions:**
  - `lifecycle.long-running-loop` Hold with a
    `time.Ticker.C`/`time.Sleep` on every branch of the loop body.
  - Body passes all `boundary.*` gates.
  - `effects.no-global-writes` Hold on body.
  - Pragma-supplied evidence: idempotency declaration (`idempotent=true`
    on the body). Load-bearing; not an override.
- **Transform unlocked:** platform-scheduler-triggered invocation
  (cron, k8s CronJob, serverless scheduled trigger).
- **Earned its place in:** every target — listmonk (L1, L3), caddy
  (C1, C2), pocketbase (P2), miniflux (M1–M4), gitea (G14), mattermost
  (MM8 post-transform).

### A4. `keyed-partitioned-state`

- **Archetype enabled:** `keyed-partitioned-state`.
- **Evidence conditions:**
  - Protected field is map-typed.
  - Proposed new signal: `keyed-access-invariant` — every call site
    reaching the map indexes by a key derived from input.
  - No iteration over all entries in hot paths (background
    cleanup-iteration is tolerated with advisory).
  - Existing gates: `effects.no-global-writes` Hold,
    `boundary.no-sync-primitives` Violate tolerated under the
    same "mutex is semantic" reinterpretation as serialized-actor.
- **Transform unlocked:** consistent-hash router + per-shard service,
  or managed KV store (Redis cluster / Dynamo).
- **Earned its place in:** listmonk (L5), caddy (C5 composite),
  pocketbase (P3), gitea (G2, G18 composite), mattermost (MM1
  composite).

### A5. `fanout-publisher`

- **Archetype enabled:** `fanout-publisher`.
- **Evidence conditions:**
  - Struct holds `[]chan T` or `map[K]chan T` under mutex.
  - A method iterates the collection sending T to each.
  - T satisfies `boundary.serializable-via-custom-encoding`.
  - `effects.no-global-writes` Hold on the subscriber collection
    (receiver-owned).
- **Transform unlocked:** managed pub/sub broker; subscribers are
  named services consuming the topic.
- **Earned its place in:** listmonk (L4), pocketbase (P4), gitea
  (G7), mattermost (MM7 — already ADMITTED, validates the shape).

### A6. `ttl-cache`

- **Archetype enabled:** `ttl-cache`.
- **Evidence conditions:**
  - Map value type carries expiry timestamp (or is `sync.Map` plus a
    periodic cleanup goroutine).
  - Proposed new signal: `cache-value-no-pointer-escape` — value type
    carries no pointer to in-process state.
  - Cache-miss loader exists (source-of-truth elsewhere).
  - Existing gates: standard mutex-on-map acceptance under
    serialized-actor reinterpretation.
- **Transform unlocked:** managed cache (Redis / memcached); background
  cleanup goroutine removed (managed eviction).
- **Earned its place in:** listmonk (L6, L7), caddy (C7 overlap with
  A1), pocketbase (P3 overlap with A4), gitea (G10), mattermost
  (MM4, MM5).

### A7. `session-affinity-state`

- **Archetype enabled:** `session-affinity-state`.
- **Evidence conditions:**
  - State map keyed by session-ID field.
  - Proposed new signal: `session-id-keyed-access` — key ingress is at
    connection-accept time, not request-time.
  - Per-session mutations are serialized.
  - State is removed at session close (observable via lifecycle API).
- **Transform unlocked:** session-affinity-aware load balancer
  (consistent-hash on session ID, sticky routing); per-session actor
  lives on the routed replica for session lifetime.
- **Earned its place in:** caddy (C6), gitea (G11, G12, G13),
  mattermost (MM2).

---

## Bucket B — ADRs ripe to draft

### B1. `ADR-0019: archetype-driven remediation surface` (the SUGGEST path)

The 22 SUGGEST-triage regions in the corpus share a shape: the
archetype is identifiable, but at least one evidence gap keeps
auto-lift unsafe. This ADR formalizes the SUGGEST surface:

- The compiler outputs a structured remediation with: archetype name,
  evidence-found list, evidence-missing list (named), transform
  proposal, pragma suggestions that would close each gap.
- Remediation is **not** a refusal. It is the admitted output when
  static evidence is strong but not sufficient for auto-apply.
- Draft should separate two remediation sub-classes:
  - *Threshold-tunable* — the classifier could collect the missing
    signal (link to the three proposed signals above).
  - *Pragma-bridgeable* — the missing evidence is structurally
    external (contract atomicity, idempotency, connection-affinity
    contract); a pragma is load-bearing evidence.

Should be drafted after the candidate state classes (Bucket A) land,
because the SUGGEST path's output format depends on the state-class
vocabulary.

### B2. `ADR-0020: auto-lift evidence thresholds`

The boundary-gate thresholds stated per-archetype in the catalog are
currently research output; they need a formal decision record before
implementation. This ADR codifies:

- The per-archetype AUTO thresholds (concrete evidence conditions).
- The structural two-axis model (evidence-locality ×
  externalization-affinity) as the framework that generalizes.
- The relationship to ADR-0017's sound-vs-heuristic containment rule:
  AUTO requires only sound-detector evidence; SUGGEST may use
  heuristic detectors.
- The relationship to ADR-0018's `gate`/`bias`/`advisory` outcome
  classes: some advisories become gates under the new state classes.

Draft order: after Bucket A state classes, before B1.

### B3. `ADR-0021: pragmas as load-bearing evidence vs. overrides`

Surfaced by the research as the largest tension. Some pragmas supply
evidence the classifier cannot collect (idempotency, external-contract
atomicity, connection-affinity). Others are pure overrides ("lift this
region even though the classifier refused"). The semantics diverge:

- Evidence pragmas should make the classifier produce a **different
  decision** by combining the pragma's fact with static evidence.
- Override pragmas should bypass a specific refusal code with an
  explicit waiver.

This ADR should separate the two and state where each is allowed.

### B4. `ADR-0022: composite-archetype regions`

Research finding: some regions cleanly fit multiple archetypes
(caddy connections map = serialized-actor + keyed-partitioned-state;
mattermost hub = keyed-partitioned-state + fanout-publisher +
session-affinity-state). The catalog does not force uniqueness; the
compiler should have a policy for choosing (or emitting a composite
transform). This ADR specifies:

- Precedence order when multiple archetypes match.
- When composite emission is justified (all contributing archetypes'
  invariants must hold).
- Report format for composite regions.

### B5. `ADR-0023: lifecycle-state-machine as a category`

Retired-from-v1 but flagged for a future decision record. Gitea's
`graceful.Manager` and `process.Manager` are the canonical instances.
The research concluded v1 lacked the evidence vocabulary for ordered
distributed state transitions, but the pattern is coherent and likely
to reappear. The ADR should either commit to a coordinator-backed
emission (e.g. etcd CAS / k8s leader-election) or explicitly declare
lifecycle-state-machine permanently out of scope.

---

## Bucket C — Still-open empirical questions

Each carries the research's current best characterization — not a
verdict.

### C1. Is the auto-lift-vs-suggest boundary a single threshold, per-archetype, or structural?

**Research current characterization:** structural with two axes —
evidence-locality (local/closed-form vs. runtime-dependent) ×
externalization-affinity (transform moves state to a managed substrate
whose semantics match one-for-one vs. requires internal substrate).
Not a single threshold; not purely per-archetype.

**Open:** does this two-axis model survive larger corpora (Kubernetes
controllers, Hashicorp stack)? Or does it fracture further?

### C2. How much does the user-facing API of a lifted archetype need to change, and is that compiler-owned or user-owned?

**Research current characterization:** varies per archetype.
`periodic-invocation` changes API zero (the Start/Stop calls become
no-ops). `bounded-worker-pool` changes Enqueue semantics (from
synchronous channel-send to broker-publish-may-block). `fanout-publisher`
preserves Publish/Subscribe shape. `serialized-actor` and
`keyed-partitioned-state` depend on whether the user expects
synchronous return (they do, so the RPC call is the only API change).

**Open:** who owns ordering, at-least-once, and error-return semantics
changes in the lifted API? Developer-declared via pragma, or
compiler-imposed with a refusal code if the user's expectation conflicts?

### C3. Are pragmas overrides only, or load-bearing evidence the compiler may rely on?

See B3. Research strongly leans toward **both roles are needed**, with
explicit separation. Still open: which specific pragmas belong to
which role, and how should the report format reflect the distinction?

### C4. Should "non-distributable" become an explicit archetype class, or stay as terminal refusal outside the vocabulary?

**Research current characterization:** stay as terminal refusal
outside the vocabulary. The TERMINAL set shrank meaningfully under
this research, and the remaining categories (embedded durables,
fire-and-forget, distributed state machines, cross-key-invariant maps,
self-tuning periodics) are heterogeneous enough that forcing them
under one "non-distributable" archetype would paper over the
differences. Prefer named TERMINAL classes per refusal (many already
exist as `MLV2_*` codes).

**Open:** is there value in an explicit "archetype: terminal" label
in reports, separate from refusal codes? This is a reporting-UX
question more than a compiler-semantics question.

### C5. Do the proposed new classifier signals introduce new false-refusal surfaces?

**Research current characterization:** low risk. Each proposed signal
(`keyed-access-invariant`, `cache-value-no-pointer-escape`,
`session-id-keyed-access`, `bounded-pool-invariant`) is framed as
*promotion evidence* — its absence keeps the region at SUGGEST,
its presence enables AUTO. Under ADR-0017's heuristic-containment
rule, heuristic Unknowns stay advisory. Therefore new signals should
only move SUGGEST → AUTO, never ADMITTED → REFUSED.

**Open:** in practice, do any of the new signals have unavoidable
Violate verdicts that regress existing ADMITTED regions? Requires
a spike to validate (Bucket D).

### C6. Is worker-pool-consumer a distinct archetype or a pair-of-others?

**Research current characterization:** a pairing —
`bounded-worker-pool` state class + `replicated-stateless-service`
admission. Miniflux validates this: once state is external, the pool
is the admitted baseline. But gitea's `modules/queue` shows the pool
as its own coherent source-level pattern with its own refusal codes
today. The catalog keeps `bounded-worker-pool` as a standalone state
class because the classifier work to recognize the pattern is the
same whether we name it standalone or as a pair.

**Open:** does this distinction cause confusion in reports? The
user sees "bounded-worker-pool" as both an archetype label and a
state class. Is that the right vocabulary, or should reports
separate "pattern in source" from "state class"?

### C7. What is the right expression of `session-affinity-state` when sessions span multiple connections (mattermost cluster)?

**Research current characterization:** out of v1 scope. When sessions
are per-connection (caddy hijack, gitea session stores), the archetype
is clean. Mattermost's cluster model with a single user holding
multiple connections across nodes stresses the archetype:
`session-affinity-state` stops being literal per-connection and
becomes per-user-across-cluster, which is a different class of
problem (user-level consistency across the cluster).

**Open:** does this deserve a `user-affinity-state` archetype in a
future sprint, or is it a different problem class entirely (distributed
user state)?

---

## Bucket D — Implementation spikes (wait until ADRs exist)

Scope fence: this research sprint fenced all code work. These are
named so they do not accidentally get started before Bucket A / B.

### D1. Classifier signal additions

Add the proposed signals to `pkg/compiler/stateclass/` and the
liftability-properties spec:

- `keyed-access-invariant`
- `cache-value-no-pointer-escape`
- `session-id-keyed-access`
- `bounded-pool-invariant`
- `mutex-encloses-store-invariant`

Wait until: ADR-0020 (B2) codifies the evidence-threshold model these
signals feed into.

### D2. State-class rule implementations

Implement the seven candidate state classes (A1–A7) as new rules or
post-passes in `pkg/compiler/stateclass/`, integrating with the
existing six-rule stack.

Wait until: Bucket A additions to ADR-0016 land.

### D3. Transform codegen

Emit the ≤30-line transforms from the catalog:

- serial-actor harness (single-goroutine mailbox)
- broker-publish wrapper
- scheduler-registration stub
- sharded-KV client adapter
- fanout-via-broker adapter
- managed-cache adapter
- session-affinity-routed-replica adapter

Wait until: ADR-0022 (B4) settles composite-archetype emission, so
the codegen has one policy rather than seven.

### D4. Pragma bridge

Add pragma surfaces for load-bearing evidence (idempotency,
external-contract atomicity, session-affinity contract).

Wait until: ADR-0021 (B3) lands the evidence-vs-override distinction.

### D5. Report-format additions

Update `reportv2` schema to carry archetype labels, SUGGEST-triage
remediation payload, and evidence provenance (which signals fired,
which were advisory).

Wait until: ADR-0019 (B1) defines the SUGGEST output format.

### D6. Runtime scaffolding

If auto-lift emission requires a Monolift-owned runtime library
(e.g. for serial-actor harness, or for session-affinity dispatch),
that library is its own spike. Wait until codegen (D3) is prototyped
and shows what the runtime contract looks like.

### D7. False-refusal-regression test set

Before landing D1–D3, build a per-target regression harness that
pins the current ADMITTED set across the six evaluation targets and
fails if any region drops out of ADMITTED under the new signals. Tied
to C5.

Wait until: D1 has prototype code to test against.

---

## Relationship to existing sprint artifacts

- ADR-0015 / 0016 / 0017 / 0018 are the foundation the Bucket A
  additions build on.
- SPRINT-0009 shipped the classifier rewrite this research uses as
  substrate.
- SPRINT-0007 shipped the state-class-inference rule stack the Bucket A
  additions extend.
- SPRINT-0010 classifier-perf and SPRINT-0012 test/stability sprints
  are orthogonal; this work does not conflict.

---

## Self-audit summary

- Every Bucket-A state class cites ≥2 targets from the corpus walk.
- Every Bucket-B ADR either directly addresses a research finding
  or resolves a tension explicitly surfaced in the narrative note.
- Every Bucket-C question is carried from the brief's open-questions
  list or surfaced anew by the research; each has a current
  characterization, not a verdict.
- Every Bucket-D spike is named to avoid it being started before the
  ADR that would justify it.
- No follow-up closes a scope-fenced compiler/classifier/runtime task
  as "do it now" — every engineering task waits on a decision record.
