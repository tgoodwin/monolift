# Distribution-archetypes follow-ups — SPRINT-0013 composite

**Status:** v1. Composite across three parallel research runs.

Four buckets, per the sprint plan:
- **A** — Candidate state-class additions for ADR-0016 (primary engineering output)
- **B** — ADRs ripe to draft
- **C** — Still-open empirical questions
- **D** — Implementation spikes (wait until ADRs exist)

Per-run followup files are preserved at `runs/{opus,gpt-5.4,gemini}/distribution-archetypes-followups.md`.

---

## Bucket A — Candidate state-class additions for ADR-0016

Primary engineering output. Each entry: archetype enabled, evidence conditions, transform unlocked, corpus citations, gate-pass summary.

### A1. `serialized-actor`

- **Archetype enabled:** `serialized-actor` — stateful struct with serialized mutex-protected access.
- **Evidence conditions (sound, gate):**
  - `effects.no-param-heap-mutation` Hold.
  - `effects.no-param-escape` Hold (promote from advisory in the state-class addition).
  - `boundary.no-sync-primitives` Violate *tolerated* — mutex is semantic (actor serialization), not a boundary violation.
  - New SSA check: `mutex-encloses-store-invariant` — every store on protected state lies inside the Lock/Unlock span.
- **Transform unlocked:** wire-level serial-actor harness. Single goroutine consuming a command mailbox; method calls become RPC. Runtime: serial-dispatch harness (no external broker).
- **Earned its place in:** caddy (C5, C7, C10, C11), pocketbase (P1, P5), gitea (G3, G4, G15, G18), mattermost (MM11), miniflux (M6).

### A2. `bounded-worker-pool` (synonym: `queued-workset`)

- **Archetype enabled:** `bounded-worker-pool`.
- **Evidence conditions:**
  - Struct field of kind `chan T` where T satisfies `boundary.serializable-via-custom-encoding`.
  - Static-bounded loop spawns goroutines consuming the channel.
  - Consumer body holds `effects.no-global-writes` and `contract.error-last`.
  - New proposed signal: `bounded-pool-invariant` — pool size is statically bounded, not growable on overflow.
- **Transform unlocked:** broker-backed queue + N replicas; per-job handler as stateless function. Runtime: message broker client (NATS/SQS/Pub-Sub/Rabbit).
- **Earned its place in:** listmonk (L2), gitea (G1), mattermost (MM6), miniflux (ADMITTED baseline).

### A3. `periodic-invocation` (synonym: `scheduled-reconciler`)

- **Archetype enabled:** `periodic-invocation`.
- **Evidence conditions:**
  - `lifecycle.long-running-loop` Hold with `time.Ticker.C`/`time.Sleep` on every loop-body branch.
  - Body passes all `boundary.*` gates.
  - `effects.no-global-writes` Hold on body.
  - **Pragma-supplied evidence:** `idempotent=true` on the body — load-bearing, not override.
- **Transform unlocked:** platform-scheduler-triggered invocation (cron, k8s CronJob, serverless scheduled trigger).
- **Earned its place in:** every target — listmonk (L1, L3), caddy (C1, C2), pocketbase (P2), miniflux (M1–M4), gitea (G14), mattermost (MM8).

### A4. `keyed-partitioned-state`

- **Archetype enabled:** `keyed-partitioned-state`.
- **Evidence conditions:**
  - Protected field is map-typed.
  - New proposed signal: `keyed-access-invariant` — every call site reaching the map indexes by key derived from input.
  - No iteration over all entries in hot paths (background cleanup tolerated).
  - Existing gates: `effects.no-global-writes` Hold; `boundary.no-sync-primitives` Violate tolerated under the "mutex is semantic" reinterpretation.
- **Transform unlocked:** consistent-hash router + per-shard service, OR managed KV store (Redis cluster, DynamoDB).
- **Earned its place in:** listmonk (L5), caddy (C5 composite), pocketbase (P3), gitea (G2, G18 composite), mattermost (MM1 composite).

### A5. `fanout-publisher`

- **Archetype enabled:** `fanout-publisher`.
- **Evidence conditions:**
  - Struct holds `[]chan T` or `map[K]chan T` under mutex.
  - A method iterates the collection sending T to each.
  - T satisfies `boundary.serializable-via-custom-encoding`.
  - `effects.no-global-writes` Hold on the subscriber collection.
- **Transform unlocked:** managed pub/sub broker; subscribers become named services consuming the topic.
- **Earned its place in:** listmonk (L4), pocketbase (P4), gitea (G7), mattermost (MM7 ADMITTED — validates shape).

### A6. `ttl-cache`

- **Archetype enabled:** `ttl-cache`.
- **Evidence conditions:**
  - Map value type carries expiry timestamp (or is `sync.Map` plus periodic cleanup goroutine).
  - New proposed signal: `cache-value-no-pointer-escape` — value type carries no pointer to in-process state.
  - Cache-miss loader exists (source of truth elsewhere).
- **Transform unlocked:** managed cache (Redis / memcached). Background cleanup goroutine removed (managed eviction handles it).
- **Earned its place in:** listmonk (L6, L7), caddy (C7 overlap), pocketbase (P3 overlap), gitea (G10), mattermost (MM4, MM5).

### A7. `session-affinity-state`

- **Archetype enabled:** `session-affinity-state`.
- **Evidence conditions:**
  - State map keyed by session-ID field.
  - New proposed signal: `session-id-keyed-access` — key ingress is at connection-accept time, not request-time.
  - Per-session mutations are serialized.
  - State removed at session close (observable via lifecycle API).
- **Transform unlocked:** session-affinity-aware load balancer (consistent-hash on session ID, sticky routing). Per-session actor lives on the routed replica for the session's lifetime.
- **Earned its place in:** caddy (C6), gitea (G11, G12, G13), mattermost (MM2).

### A8. `filesystem-bound-singleton` (gemini-sourced)

- **Archetype enabled:** `filesystem-bound-singleton`.
- **Evidence conditions:**
  - Struct fields hold file handles or filesystem paths.
  - Methods invoke `os`, `filepath`, or `os.File` / `os.Root` APIs.
  - No in-memory state bridges invocations (no caching between FS calls).
  - Paths are config-driven (not runtime-derived from mutable state).
- **Transform unlocked:** object-store client adapter (S3/GCS/Azure Blob) OR sidecar with volume mapping. Handle-based operations become request-scoped streams.
- **Earned its place in:** caddy (filestorage subsystem), gitea (local storage module, process manager lock files). Borderline coverage; kept on evidence-gate and emission-gate strength.

---

## Bucket B — ADRs ripe to draft

### B1. `ADR-0019: archetype-driven remediation surface` (the SUGGEST path)

The 22+ SUGGEST-triage regions in the corpus share a shape: archetype is identifiable, but at least one evidence gap keeps auto-lift unsafe. This ADR formalizes the SUGGEST surface.

- The compiler outputs a structured remediation with: archetype name, evidence-found list, evidence-missing list (named), transform proposal, pragma suggestions that would close each gap.
- Remediation is **not** a refusal — it is the admitted output when static evidence is strong but not sufficient for auto-apply.
- Draft separates two remediation sub-classes:
  - *Threshold-tunable* — classifier could collect the missing signal (link to the five proposed signals in Bucket D).
  - *Pragma-bridgeable* — missing evidence is structurally external; a pragma is load-bearing evidence.

**Draft order.** After Bucket A state classes land (so the SUGGEST output format references real state-class vocabulary).

### B2. `ADR-0020: auto-lift evidence thresholds`

Boundary-gate thresholds stated per-archetype in the catalog need a formal decision record before implementation. This ADR codifies:

- Per-archetype AUTO thresholds (concrete evidence conditions from the catalog).
- The structural two-axis model (evidence-locality × externalization-affinity) as the framework that explains the per-archetype clustering.
- Relationship to ADR-0017's sound-vs-heuristic containment rule: AUTO requires only sound-detector evidence; SUGGEST may use heuristic detectors.
- Relationship to ADR-0018's `gate`/`bias`/`advisory` outcome classes: some advisories become gates under the new state classes.

**Draft order.** After Bucket A, before B1.

### B3. `ADR-0021: pragmas as load-bearing evidence vs. overrides`

Largest tension the research surfaced. Some pragmas supply evidence the classifier cannot collect (idempotency, external-contract atomicity, connection-affinity). Others are pure overrides. The semantics diverge:

- **Evidence pragmas** make the classifier produce a *different* decision by combining the pragma's fact with static evidence.
- **Override pragmas** bypass a specific refusal code with an explicit waiver.

This ADR separates the two roles and states where each is allowed. gpt-5.4's "pragma as additive evidence, not override" framing is one legitimate reading; opus's finding that both roles are needed with explicit separation is another. Both research findings go into the draft as alternative positions.

### B4. `ADR-0022: composite-archetype regions`

Research finding: some regions cleanly fit multiple archetypes (caddy connections map = serialized-actor + keyed-partitioned-state; mattermost Hub = keyed-partitioned-state + fanout-publisher + session-affinity-state — gpt-5.4's `connection-hub-buffer` lens names this composite).

This ADR specifies:
- Precedence order when multiple archetypes match.
- When composite emission is justified (all contributing archetypes' invariants must hold).
- Report format for composite regions.
- Whether `connection-hub-buffer` gets a dedicated composite name or remains a documented pattern.

### B5. `ADR-0023: lifecycle-state-machine as a category`

Retired from v1, flagged. Gitea `graceful.Manager` and `process.Manager` are canonical instances. v1 lacks evidence vocabulary for ordered distributed state transitions (Raft/CRDT territory). This ADR should either commit to a coordinator-backed emission (etcd CAS / k8s leader-election) or explicitly declare the category permanently out of scope.

---

## Bucket C — Still-open empirical questions

Each carries the research's current best characterization — not a verdict.

### C1. Is the auto-lift-vs-suggest boundary a single threshold, per-archetype, or structural?

**Research characterization:** structural with two axes — *evidence-locality* (local/closed-form vs. runtime-dependent) × *externalization-affinity* (transform moves state to a managed substrate with matching semantics vs. requires internal substrate). The two-axis model explains why per-archetype thresholds cluster the way they do; per-archetype thresholds are the implementation primitive.

**Open:** does this model survive larger corpora (Kubernetes controllers, Hashicorp stack)? Or does it fracture further?

### C2. How much does the user-facing API of a lifted archetype need to change, and is that compiler-owned or user-owned?

**Research characterization:** varies per archetype. `periodic-invocation` changes API zero. `bounded-worker-pool` changes Enqueue semantics (synchronous channel-send → broker-publish-may-block). `fanout-publisher` preserves Publish/Subscribe. `serialized-actor` and `keyed-partitioned-state` depend on whether the user expects synchronous return (RPC round-trip is the only API change).

**Open:** who owns ordering, at-least-once, and error-return semantics changes in the lifted API? Developer-declared via pragma, or compiler-imposed with a refusal code when user expectation conflicts?

### C3. Are pragmas overrides only, or load-bearing evidence?

See B3. Research strongly leans toward **both roles needed with explicit separation**. gpt-5.4 argues for evidence-only; opus argues both. Synthesis position: both roles, per ADR-0021.

### C4. Should "non-distributable" become an explicit archetype class?

**Research characterization:** no — keep as terminal refusal outside the vocabulary. The TERMINAL set shrank meaningfully; the remaining categories (embedded durables, fire-and-forget, distributed state machines, cross-key-invariant maps, self-tuning periodics) are heterogeneous enough that one "non-distributable" label would paper over real differences.

**Open:** is there value in an explicit "archetype: terminal" label in reports separate from refusal codes? Reporting-UX question, not compiler-semantics.

### C5. Do the proposed new classifier signals introduce false-refusal surfaces?

**Research characterization:** low risk. Each proposed signal is framed as *promotion evidence* — absence keeps the region at SUGGEST, presence enables AUTO. Under ADR-0017's heuristic-containment rule, heuristic Unknowns stay advisory. New signals should only move SUGGEST → AUTO, never ADMITTED → REFUSED.

**Open:** in practice, do any proposed signals have unavoidable Violate verdicts that regress existing ADMITTED regions? Requires a validation spike (D7).

### C6. Is worker-pool-consumer a distinct archetype or a pairing?

**Research characterization:** a pairing — `bounded-worker-pool` state class + `replicated-stateless-service` admission, coordinated by external state. Miniflux validates this (external state → pool is ADMITTED baseline). But gitea's `modules/queue` shows the pool as its own coherent source-level pattern with its own refusal codes today. Catalog keeps `bounded-worker-pool` as a standalone state class because the classifier work is identical.

**Open:** does this distinction cause confusion in reports? "Bounded-worker-pool" is both an archetype label and a state class. Should reports separate "pattern in source" from "state class"?

### C7. Right expression of `session-affinity-state` when sessions span multiple connections?

**Research characterization:** out of v1 scope. Per-connection sessions (caddy hijack, gitea session stores) are clean. Mattermost's cluster model with a single user across multiple connections stresses the archetype.

**Open:** does this deserve a `user-affinity-state` archetype in a future sprint, or is it a different problem class (distributed user state)?

### C8. How much compression is safe in the archetype vocabulary?

**Research characterization:** opus's 7 + gemini's `filesystem-bound-singleton` = 8 is where the synthesis landed. gpt-5.4's 4 is safer for reports; opus's 8 is safer for the classifier (transforms are distinct). Both can coexist: ADR-0019's remediation surface can report at a composite granularity even when the classifier operates at the narrower one.

**Open:** when the first state class lands, does the user-facing language need to reflect the implementation's finer granularity, or can it compress to the human-readable 4-archetype framing?

---

## Bucket D — Implementation spikes (wait until ADRs exist)

Scope fence: SPRINT-0013 fenced all code work. These are named so they don't accidentally get started before the ADRs that justify them.

### D1. Classifier signal additions

Add the proposed signals to `pkg/compiler/stateclass/` and the liftability-properties spec:

- `keyed-access-invariant`
- `cache-value-no-pointer-escape`
- `session-id-keyed-access`
- `bounded-pool-invariant`
- `mutex-encloses-store-invariant`

Potentially: `filesystem-operations-idempotent` (supporting A8).

**Wait until:** ADR-0020 (B2) codifies the evidence-threshold model these signals feed.

### D2. State-class rule implementations

Implement the eight candidate state classes (A1–A8) as new rules / post-passes in `pkg/compiler/stateclass/`, integrating with the existing rule stack.

**Wait until:** Bucket A additions to ADR-0016 land.

### D3. Transform codegen

Emit the ≤30-line transforms from the catalog:
- serial-actor harness (single-goroutine mailbox)
- broker-publish wrapper
- scheduler-registration stub
- sharded-KV client adapter
- fanout-via-broker adapter
- managed-cache adapter
- session-affinity-routed-replica adapter
- object-store / sidecar adapter (A8)

**Wait until:** ADR-0022 (B4) settles composite-archetype emission.

### D4. Pragma bridge

Add pragma surfaces for load-bearing evidence (idempotency, external-contract atomicity, session-affinity contract).

**Wait until:** ADR-0021 (B3) lands the evidence-vs-override distinction.

### D5. Report-format additions

Update `reportv2` schema to carry archetype labels, SUGGEST-triage remediation payload, and evidence provenance.

**Wait until:** ADR-0019 (B1) defines the SUGGEST output format.

### D6. Runtime scaffolding

If auto-lift emission requires a Monolift-owned runtime library (serial-actor harness, session-affinity dispatch), that library is its own spike.

**Wait until:** codegen (D3) is prototyped and shows the runtime contract.

### D7. False-refusal-regression test set

Before landing D1–D3, build a per-target regression harness that pins the current ADMITTED set across the six evaluation targets and fails if any region drops out of ADMITTED under the new signals. Tied to C5.

**Wait until:** D1 has prototype code to test against.

### D8. Corpus-sanity tooling

Hardening opportunities surfaced during this research:
- Make `extract-report` corpus work tolerant of target-specific toolchain requirements, or pin a matching Go toolchain in research sessions.
- Harden subagent prompts for large-target fanout so path mistakes and non-terminating first passes are caught earlier.

Wait until: relevant (anytime after this sprint — these are research-ergonomics improvements).

---

## Relationship to existing sprint artifacts

- ADR-0015 / 0016 / 0017 / 0018 are the foundation the Bucket A additions build on.
- SPRINT-0009 shipped the classifier rewrite this research uses as substrate.
- SPRINT-0007 shipped the state-class-inference rule stack that Bucket A extends.
- SPRINT-0010 (classifier perf), SPRINT-0011 (goldens + dedup + stateclass RSS), SPRINT-0012 (gate stabilization) are orthogonal; this work does not conflict.

---

## Self-audit summary

- Every Bucket-A state class cites ≥2 targets from the corpus walk, or has an argued coverage exception (A8).
- Every Bucket-B ADR directly addresses a research finding or resolves a surfaced tension.
- Every Bucket-C question carries the brief's open-question list or a research-surfaced refinement; each has a current characterization, not a verdict.
- Every Bucket-D spike waits on a specific ADR before starting.
- No follow-up closes a scope-fenced compiler/classifier/runtime task as "do it now" — every engineering task waits on a decision record.
- Cross-run contributions attributed:
  - **Opus:** A1–A7 (7 state classes), 5 evidence signals, structural two-axis model, B1–B5.
  - **GPT-5.4:** `queued-workset`/`scheduled-reconciler` synonym naming; `connection-hub-buffer` composite; per-archetype-threshold framing; pragma-as-additive-evidence position for B3.
  - **Gemini:** A8 (`filesystem-bound-singleton`); "God Object" app-root framing (pocketbase core.App, listmonk App); high-level event-bus framing.
