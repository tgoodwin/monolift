# SPRINT-0013 — Research: distribution-archetype transforms (auto-lift vs. suggest boundary)

**Status:** planned
**Shape:** research sprint — output is written artifacts + a disciplined vocabulary. No production code.
**Brief:** `docs/sprints/SPRINT-0013-BRIEF.md`
**Corpus pin:** `evaluation/MANIFEST.yaml` as of 2026-04-19 (caddy, pocketbase, miniflux, listmonk, gitea, mattermost).
**Primary inputs:** ADR-0015 (canonical-shape classifier), ADR-0016 (state-class inference), ADR-0017 (classifier reasons about liftability), ADR-0018 (liftability-property taxonomy); `docs/specs/monolift-v2-contract.md`; `docs/specs/liftability-properties.md`; `pkg/compiler/stateclass/`; committed extract reports at `test/e2e/targets/*/golden/report.json`; `pkg/compiler/extract_integration_test.go`; prior research notes under `docs/research/`.

## Intent

**Expand what the compiler can auto-lift.** Today, Monolift lifts only narrow stateful patterns — the `immutable-captured-config` and `replicated` state classes defined in ADR-0016. Stateful code involving synchronization primitives (mutexes, channels, atomics), shared mutable state, or pointer aliasing is refused outright. But many of those refusals correspond to *distribution patterns with known transforms*: a mutex-protected struct is a singleton actor, a channel-fed goroutine pool is a worker queue, a keyed map under a lock is a sharded service, a periodic background goroutine is a scheduled invocation. The compiler currently refuses these not because they are fundamentally undistributable, but because it doesn't yet recognize the *archetype* that would justify the transform.

This research sprint is the evidence-gathering that closes that gap. Walk the full evaluation corpus, identify the distribution archetypes real Go code implements, and for each one establish: what **transform** would auto-lift it; what **evidence** the classifier can collect (today or plausibly) that is sufficient to apply the transform automatically; what **candidate state class** would need to be added to ADR-0016 to recognize the pattern; and what the **user-facing API** looks like before and after. When static evidence is strong but not conclusive for a given region, the archetype routes to a **suggest** surface — a fallback that keeps correctness while expanding reach. Terminal refusal remains only for patterns that genuinely cannot be distributed.

The primary research question is **"which currently-refused patterns have enough structure that the compiler could auto-lift them with a named transform, and what would the classifier need to learn to do it?"** — not "what's the boundary between auto-lift and suggest." The auto-lift surface is the main product; the suggest surface is the honest fallback; terminal refusal is what's left after both.

## Motivating examples (sparks, not a shopping list)

Concrete hypotheticals to ground what archetype-driven auto-lift could look like. These exist to **open the exploration aperture** — the research should modify them, replace them, or uncover entirely different patterns in the corpus. The examples are scaffolding for thinking, not a taxonomy to ratify.

Each follows the same four-part structure, and the catalog produced by this sprint should do the same:

> **Pattern seen today → archetype name → transform → evidence conditions for auto-lift**

- **Stateful struct with serialized access → singleton actor.**
  `type RateLimiter struct { mu sync.Mutex; buckets map[string]*bucket }` with pointer-receiver methods mutating `buckets` under the lock. Today: refused for the mutex. Transform: emit a service that owns the `RateLimiter` instance; serialize access at the request handler (wire-level serialization replaces in-process lock). Evidence for auto-lift: single-instance construction provable at the callsite; no cross-instance shared map; pointer escape analysis shows no alias leaks.

- **Channel-consumer goroutine pool → message queue + worker service.**
  Fixed pool of `go worker(jobs)` goroutines reading `jobs chan Job`, each processing a job independently. Today: refused for the channel. Transform range: broker-backed queue (SQS, NATS, Pub/Sub, RabbitMQ) feeding a pool of worker-service replicas — *or, at the extreme, serverless-function invocation per `Job`* if the job handler is self-contained enough. Evidence: jobs serializable; workers share no mutable state; processing order not semantically load-bearing; job handler is free of non-serializable reachability beyond what the transform would replace.

- **Periodic background goroutine → scheduled serverless invocation.**
  `go func() { for { time.Sleep(interval); doWork(ctx) } }()`. Today: refused for the goroutine state. Transform: cron-triggered serverless function or scheduled service job; the in-process timer becomes a platform scheduler. Evidence: `doWork` is idempotent or tolerates occasional skipped ticks; no shared state across invocations beyond what's externalized.

- **Pipeline stage → chained services via queues.**
  `go func() { for x := range in { out <- transform(x) } }()` — one goroutine reading one channel, writing another. Today: refused for the channels. Transform: two services connected by a queue, or a stream-processing topology. Evidence: transform is pure-ish; no cross-item shared state beyond what's externalizable.

- **Sharded keyed state → sharded service with key-based routing.**
  `var sessions = struct { sync.RWMutex; m map[UserID]*Session }{}` — keyed state protected by a lock, accessed by key. Today: refused. Transform: sharded service; key hash determines which replica owns the shard. Evidence: every access routes through the key; no key-free iteration that assumes all entries live in one process.

- **Channel fanout → managed pub/sub.**
  One producer distributing to N subscriber channels (`for _, sub := range subs { sub <- event }`). Today: refused. Transform: managed pub/sub broker with subscriber services. Evidence: subscribers are independent; event is serializable; subscriber set is discoverable (static registry or dynamic subscription API).

- **TTL cache with background expiry → managed distributed cache.**
  `sync.Map` plus a background goroutine doing periodic expiry. Today: refused. Transform: external cache (Redis / memcached) with managed eviction. Evidence: cache contents don't carry pointers to in-process state; TTL semantics match the target cache's eviction model.

**The corpus decides which of these survive the gates, and more importantly, what else is out there.** If the walk reveals patterns that don't fit any of the above — co-dependent state, pipelines with feedback loops, gossip-style convergent state, long-running heterogeneous sessions, stateful workflows — that is signal to investigate, not noise to reject. The examples are there to show the *shape* of the catalog entry, not to bound the taxonomy.

## Goals

1. A **v1 archetype catalog** at `docs/research/archetype-catalog-v1.md`. Each surviving entry pairs: (a) **definition**; (b) the **transform** — what auto-generated distribution code looks like in concrete Go terms (scaffolding, runtime deps, invariants preserved, user-visible API before/after the lift); (c) a **candidate state class** for ADR-0016 — the classifier output that would need to be added to recognize this pattern and justify the transform; (d) **evidence conditions** mapped to existing primitives in `docs/specs/liftability-properties.md` and `pkg/compiler/stateclass/`; (e) explicit **auto-lift / suggest / terminal-refusal thresholds** stated as evidence conditions; (f) ≥2 citations across ≥2 distinct targets. Retirements kept in the catalog with one-paragraph "why it didn't survive" notes.
2. **Exhaustive per-target annotations** at `docs/research/annotations/<target>.md` for all six targets, using uniform **AUTO / SUGGEST / TERMINAL** classification on every region. The headline finding per target is the **AUTO set**: currently-refused regions that would become auto-liftable if the classifier recognized a named archetype and applied its transform. gitea and mattermost include per-subsystem sections and a target-level synthesis.
3. A **narrative research note** at `docs/research/distribution-archetypes-v1.md` that a collaborator reads cold and understands: what currently-refused patterns the corpus contains; what transforms each implies; what state-class additions to ADR-0016 would unlock them; where the auto-lift-vs-suggest boundary sits per archetype and why.
4. A **follow-up list** at `docs/research/distribution-archetypes-followups.md`, split into four buckets:
   (a) **Candidate state-class additions** for ADR-0016 — the primary engineering output of the research. Each proposed state class names the archetype it enables, the evidence conditions it requires, and the transform it unlocks.
   (b) **ADRs ripe to draft**, starting with `ADR-0019: archetype-driven remediation surface` (the suggest path) and `ADR-0020: auto-lift evidence thresholds` (the auto path), plus any others surfaced by the research.
   (c) **Still-open empirical questions** with the sprint's current best characterization (not a verdict).
   (d) **Implementation spikes** that should wait until ADRs exist — runtime scaffolding, transform codegen, pragma bridging.

## Non-goals

- No changes to compiler, classifier, runtime, `reportv2` schema, pragma surface, or harness. Findings that would require code to validate become follow-ups, not in-scope work.
- No exhaustive archetype taxonomy. Smallest-surviving vocabulary is the discipline.
- No attempt to close the boundary question universally. Per-archetype clarity is the bar.
- No orthogonal perf/stability/test work (SPRINT-0012 territory).
- No sampling for gitea / mattermost — subagent delegation is a context-management mechanism, not a coverage shortcut.
- No drafting of ADRs themselves. The follow-up list *names* them; drafts are future work.

## Scope boundaries

**In scope**
- Reading and annotating extract reports for all six corpus targets at pinned revisions.
- Repo-level coverage ledgers per target (built from `rg --files evaluation/<target> -g '*.go'`) grouped into subsystem bundles.
- Building, challenging, merging, and retiring archetype names per the gates below.
- Sketching (prose + short Go pseudocode) what "generated scaffolding" means per archetype, what runtime deps it pulls in, what invariants it preserves, and what user-facing API remains stable.
- Subagent delegation for gitea / mattermost with owned-directory bundles and file-count responsibility.
- All writeups under `docs/research/` and this sprint file's closeout.

**Out of scope (each becomes a follow-up, not a scope expansion)**
- Modifying `reportv2` schema or adding classifier evidence signals (note the gap, do not close it).
- Writing ADRs. Candidates are named; drafts are subsequent work.
- Validating an archetype by building its runtime scaffolding. If an emission sketch is too vague to write down, that is a finding — not a cue to start coding.
- Regenerating extract reports for targets beyond what the pinned manifest specifies.
- Generalized cleanup, renaming, or refactors in the existing compiler packages.

**Halt rule.** If Phase 2 (small-target walks) reveals the v0 starting vocabulary is so wrong that labels are fluctuating target-to-target rather than converging, stop before Phase 3 (large-target fanout), tighten the catalog explicitly, then resume. Do not spend subagent budget ratifying a broken vocabulary.

**Blocker rule.** If answering a question appears to require compiler changes or new instrumentation, record the gap in the follow-up list and keep the sprint in research mode.

## Vocabulary discipline — the mechanism, not just the goal

The brief's core ask: *"make the archetype vocabulary discipline itself rather than accumulate words."* Every candidate archetype must pass all four gates in Phase 4 before landing in v1. These gates are not aspiration — they are executable steps with recorded pass/fail per archetype:

- **Coverage gate.** Labels ≥2 regions across ≥2 targets, or has an explicit argued exception (rare; the default answer to "one region one target" is demote to appendix example).
- **Evidence gate.** Distinguishable from its nearest neighbor by evidence the classifier already collects (citing `docs/specs/liftability-properties.md` or `pkg/compiler/stateclass/`) or by a single named signal we can point at adding. "Vibes" fails.
- **Emission gate.** A ≤30-line Go pseudocode emission sketch is writeable. If two archetypes produce essentially the same sketch, merge them.
- **Boundary gate.** Auto-lift / suggest / refuse threshold is stated in concrete evidence conditions, not "when the compiler is confident." `Never auto-lift, always suggest` is a legitimate answer but must be argued in one paragraph.

**Retirement rule.** Archetypes that fail any gate after Phase 3 are retired in Phase 4 with a one-paragraph "why it didn't survive" entry kept (not deleted) in the catalog. Retirements are research output; they prevent future redraws of the same map.

**AUTO / SUGGEST / TERMINAL region classification.** For every region in every target annotation, apply the triage, leading with the most important bucket:

- **AUTO** — currently-refused region where an archetype fits and evidence is sufficient for the compiler to auto-generate the transform. **This is the primary research finding.** Every AUTO entry names the archetype and the transform it unlocks.
- **SUGGEST** — archetype fits but static evidence is insufficient for safe auto-application; compiler surfaces the pattern as a remediation suggestion instead. Fallback mode.
- **TERMINAL** — no archetype fits; refusal stands with no plausible remediation in v1.

This is the uniform primitive — the longer annotation schema below records the *why*. Admitted regions (already liftable today) are tagged `ADMITTED` for completeness but are not part of the AUTO/SUGGEST/TERMINAL triage, which targets refusals.

**v0 vocabulary (input, not commitment):** *singleton actor, replicated stateless service, sharded stateful service, worker pool / queue consumer, event-bus publisher, event-bus subscriber, pipeline stage, session-scoped state, ephemeral worker*. Expect some to survive, some to merge, some to retire.

## Annotation schema (frozen in Phase 0; every target note and subagent return conforms)

For each region: `subsystem`, `owned directories`, `region or operation identity (module / package / symbol / kind / span)`, `admitted or refused`, `triage (ADMITTED / AUTO / SUGGEST / TERMINAL)`, `proposed archetype`, `proposed candidate state class (if different from existing ADR-0016 classes)`, `proposed transform (one-line sketch)`, `competing archetypes considered`, `evidence signals seen (cited to liftability-properties or stateclass)`, `missing evidence (what would move SUGGEST → AUTO, or TERMINAL → SUGGEST)`, `file references`.

## Tasks

### Phase 0 — Bootstrap: artifact layout, v0 catalog, annotation protocol, subagent template

- [ ] Create the output scaffold: `docs/research/distribution-archetypes-v1.md`, `docs/research/archetype-catalog-v1.md`, `docs/research/annotations/README.md`, `docs/research/annotations/{caddy,pocketbase,miniflux,listmonk,gitea,mattermost}.md`, `docs/research/distribution-archetypes-followups.md`. Every target note uses the same section order so cross-target comparison is mechanical.
- [ ] Freeze the annotation schema (ten fields above) at the top of `docs/research/annotations/README.md`. Document how to flag ambiguity, how to distinguish `terminal refusal` from `archetype unclear pending evidence X` from `hybrid archetype needing split`.
- [ ] Collect every archetype name already in circulation: walk ADR-0015 / 0016 / 0017 / 0018, the v2 contract, prior research notes. List each with a one-line paraphrase and source citation. Input to v0, not a decision.
- [ ] Draft v0 catalog entries for each brief-starting archetype with placeholder fields (definition, differentiating signals from liftability-properties / stateclass, empty emission sketch, empty thresholds, empty citations). Mark the catalog "v0 — subject to revision."
- [ ] Write the subagent-delegation prompt template for large-target walks. Freeze it before Phase 3. Required return fields: region-by-region annotations per the schema; AUTO/SUGGEST/TERMINAL triage per region; archetype candidates *flagged* (not promoted — Phase 4 owns promotion); ambiguities escalated with named evidence gap. Must-read-all-files rule explicit. Thin-return re-dispatch rule explicit.
- [ ] Add the vocabulary-discipline rules (four gates + retirement rule) to `archetype-catalog-v1.md` so they are visible from the catalog itself, not buried in the sprint file.

### Phase 1 — Extract-artifact staging and coverage ledgers

- [ ] Inventory committed extract artifacts: `test/e2e/targets/caddy/golden/report.json`, `test/e2e/targets/miniflux/golden/report.json`, `test/e2e/targets/pocketbase/golden/report.json`, plus inline expectations in `pkg/compiler/extract_integration_test.go`. Record the pinned SHA and report path per target.
- [ ] Generate scratch extract artifacts outside git under `/tmp/monolift-sprint-0013/` for targets that do not have committed reports (listmonk, gitea, mattermost). Record the exact command line, root choice, and scratch path in each target's annotation note.
- [ ] Build a per-target coverage ledger from `rg --files evaluation/<target> -g '*.go'`, grouped into subsystem bundles. A target is incomplete until every bundle has either explicit findings or an explicit "no relevant archetype surface observed" note with a reason. Extract-report exhaustiveness is *not* automatically target exhaustiveness.
- [ ] Normalize region identifiers across targets (module path, package path, symbol name, kind, span) so Phase 6 synthesis can compare like with like.

### Phase 2 — Small-target walks (listmonk → caddy → pocketbase → miniflux)

Ordered shortest → longest so vocabulary pressure shows up early and cheaply. Each target ends with a target-level synthesis at the top of its annotation file naming dominant archetypes, hardest ambiguities, and most important evidence gaps.

- [ ] **listmonk (92 files).** Walk the extract report region-by-region. Apply the schema and AUTO/SUGGEST/TERMINAL triage (plus ADMITTED for already-lifted regions) to every region. Surface the AUTO set explicitly in the target synthesis. Write `docs/research/annotations/listmonk.md` including subsystem coverage ledger.
- [ ] **caddy (306 files).** Same protocol. Known refusal case — attend specifically to the admitted-handler + refused-state-dependencies pairing, and to the channel / sync-primitive patterns that drove `MLV2_CHANNEL_BOUNDARY`.
- [ ] **pocketbase (445 files).** Same protocol. Embedded DB is the known terminal refusal; the interesting question is whether service handlers around it cleanly fit any archetype once the DB layer is fenced.
- [ ] **miniflux (407 files).** Same protocol. First target where *worker pool / queue consumer* is expected to earn its keep — scrutinize whether evidence signals distinguish it from a plain goroutine-per-feed pattern.
- [ ] **Catalog pressure check (after miniflux, before Phase 3).** Spend a focused pass over every archetype label used so far: which are being applied loosely? Which have only one citation? Update `archetype-catalog-v1.md` with proposed merges/splits (do not retire yet — Phase 4 owns retirement). This is load-bearing: Phase 3 subagents must consume a v0.5 vocabulary sharpened by small-target contact.
- [ ] **Halt-rule checkpoint.** If labels are fluctuating target-to-target rather than converging, halt Phase 3 and do a targeted vocabulary tightening before resuming.

### Phase 3 — Large-target fanout (gitea, mattermost) via subagents

Delegation is **mandatory**, not optional, because linear walks of 2875 + 2153 Go files exceed context budgets and silently produce thin coverage. Parent agent's job is orchestration and synthesis; it spot-checks raw source to verify subagent returns — the parent does **not** read only summaries.

- [ ] **gitea owned-directory bundles (record in `annotations/gitea.md` before any dispatch, with `rg --files` file-count per bundle):**
  - boot/lifecycle: `cmd/`, `routers/install`, `modules/setting`, `modules/graceful`
  - ingress: `routers/api`, `routers/web`, `services/context`, `modules/web`, `modules/reqctx`
  - domain services: `services/auth`, `services/user`, `services/org`, `services/repository`, `services/pull`, `services/issue`, `services/packages`, `services/oauth2_provider`, `services/mirror`, `services/wiki`
  - background/async: `services/mailer`, `services/notify`, `services/task`, `services/webhook`, `services/cron`, `services/actions`, `modules/queue`
  - infra/runtime: `modules/cache`, `modules/storage`, `modules/indexer`, `modules/session`, `modules/eventsource`, `modules/private`, `modules/process`
  - persistence: `models/`
- [ ] Dispatch one subagent per gitea bundle using the frozen Phase 0 prompt template. Require the schema, AUTO/SUGGEST/TERMINAL triage per region, proposed transform and candidate state class for every AUTO entry, path-level citations, flagged-only archetype candidates, named evidence gaps.
- [ ] Thin-return re-dispatch: for each return, check (a) region count matches the extract-report slice for the bundle's owned directories, (b) specific symbols cited at path + line level, (c) AUTO/SUGGEST/TERMINAL triage applied uniformly, (d) every AUTO entry has a transform sketch and a candidate state-class name, (e) no undifferentiated "event bus" / "service" labels covering multiple subsystems. If any fail, re-dispatch with the gap named explicitly. Log every dispatch and re-dispatch (subsystem, prompt version, return summary, re-dispatch reason) in `annotations/gitea.md`.
- [ ] Synthesize gitea bundle returns into `annotations/gitea.md` — target-level synthesis at the top, per-bundle sections below, terminology clashes resolved with recorded parent judgment calls. Parent agent spot-checks raw source for any claim that feels thin.
- [ ] **mattermost owned-directory bundles (record in `annotations/mattermost.md` before any dispatch):**
  - ingress: `server/channels/api4`, `server/channels/web`, `server/channels/wsapi`
  - app/service logic: `server/channels/app`
  - long-lived / fanout: websocket broker, notification, event-routing paths (identified by `rg`-based discovery, not inferred from directory names alone)
  - jobs/workers: `server/channels/jobs`
  - persistence/search: `server/channels/store`, `server/channels/db`
  - platform/bootstrap: `server/platform`, `server/cmd`, `server/config`
- [ ] **Hypothesis priming at dispatch time (not conclusion):** mattermost is expected to stress *event-bus publisher/subscriber* and *session-scoped state*; note this to each subagent as a hypothesis it is empowered to disconfirm, not ratify.
- [ ] Dispatch mattermost subagents with the same protocol. Re-dispatch any bundle that collapses websocket + broadcast + event-routing into one undifferentiated archetype label. Log all dispatches.
- [ ] Synthesize mattermost returns into `annotations/mattermost.md`. Explicitly name where session affinity, singleton ownership, event distribution, and replicated-service boundaries *compete* rather than fit cleanly.
- [ ] **Cross-target archetype reconciliation.** After both large targets land, walk the combined label set. Any archetype whose evidence signals differ between small-target usage and large-target usage is a split candidate — flag for Phase 4. Any label that means different things across targets is an inconsistency, not a richness.

### Phase 4 — Vocabulary discipline pass (the gates, applied explicitly)

This is where "discipline itself rather than accumulate words" gets cashed in. Record gate pass/fail per archetype in the catalog.

- [ ] Apply **coverage gate** to every archetype: list citations across the six targets. < 2 regions or < 2 targets → retirement shortlist (unless an argued exception lands).
- [ ] Apply **evidence gate** to survivors: write the single distinguishing evidence signal (vs. nearest neighbor), citing `docs/specs/liftability-properties.md` or `pkg/compiler/stateclass/`. No distinguishing signal the classifier collects or could plausibly collect → merge or retire.
- [ ] Apply **emission gate**: write (or confirm from Phase 0) the ≤30-line Go pseudocode emission sketch. Two archetypes with essentially the same sketch → merge.
- [ ] Apply **boundary gate**: state auto-lift / suggest / refuse thresholds in concrete evidence conditions per archetype. "Never auto-lift, always suggest" acceptable with one-paragraph argument.
- [ ] Execute **retirements**: one-paragraph "why it didn't survive" per retired archetype, kept in the catalog as output.
- [ ] Execute **merges**: replace merged labels across all six annotation files; record the rename in each file's changelog so future readers can follow.
- [ ] **ADR / v2-contract naming-collision check.** Cross-check every surviving archetype name against ADR-0015 (canonical shapes), ADR-0016 (state classes), ADR-0017 (liftability properties), ADR-0018 (property taxonomy), and the v2 contract. Rename if a term already has a committed meaning.
- [ ] Record v1 catalog state: final archetype count, citations per archetype, gates passed.

### Phase 5 — Per-archetype deep dives (survivors only)

- [ ] For each surviving archetype, flesh out the catalog entry: emission sketch covers what scaffolding gets generated, what runtime deps it introduces (serialization lib, pubsub client, actor harness), what invariants the generated code maintains, what the user-visible API looks like before vs. after the lift.
- [ ] Enumerate *failure modes* — evidence looks sufficient but the lift would be semantically wrong — and what additional evidence would catch them.
- [ ] **Pragma bridge** per archetype: where user annotations could legitimately close an evidence gap as *load-bearing evidence* (not override), and where they should not (static evidence must be the only safe basis).
- [ ] **Remediation surface** per archetype's suggest side: concrete sketch of what the compiler would output to a user — archetype name, evidence found, evidence missing, transform proposal. Concrete enough that a human reader can act on it.

### Phase 6 — Boundary synthesis and evidence-gap separation

- [ ] **Characterize the expanded auto-lift surface — the primary research finding.** For each surviving archetype in the AUTO set across the corpus, state: the archetype, its transform, its candidate state class, and the specific evidence conditions that make it safe to auto-apply. The union of these is what the compiler can now lift that it couldn't before.
- [ ] Draft the auto-lift-vs-suggest boundary characterization: is the line a single threshold, per-archetype, or structural (e.g., "auto-lift iff evidence is local and closed-form; suggest when it depends on runtime behavior")? Argue from Phase 5 data, not intuition.
- [ ] **"Compiler cannot know this statically" section.** Separate *threshold-tuning* evidence gaps (classifier could collect the signal; it just doesn't yet) from *irreducible* gaps (static analysis cannot decide; pragma or user annotation is the only bridge). These route to different follow-up buckets.
- [ ] Characterize the *terminal refusal* class: refusals that fit no archetype in v1 and have no plausible remediation. Is this class shrinking, stable, or absorbing refusals as the vocabulary grows? Load-bearing finding for the thesis direction.

### Phase 7 — Narrative note, follow-ups, closeout

- [ ] Write `docs/research/distribution-archetypes-v1.md` as narrative. Open with the research question; explain the archetype surface discovered in the corpus; present the per-archetype boundary model; document the tensions the research surfaced. A reader who hasn't seen the catalog or annotations should come away able to name the v1 vocabulary, explain why each archetype earned its place, describe the boundary in concrete terms, and locate corpus examples.
- [ ] Add a cross-target matrix with columns: archetype; per-target region count; AUTO / SUGGEST / TERMINAL counts per target; and a headline column **"currently refused but shown to be auto-liftable"** — the concrete measurement of the research's impact on what the compiler can lift.
- [ ] Write `docs/research/distribution-archetypes-followups.md` with three buckets: (a) ADRs ripe to draft — starting with `ADR-0019: archetype-driven remediation surface` and `ADR-0020: auto-lift evidence thresholds`, plus any others surfaced by the research; (b) still-open empirical questions with the sprint's current best characterization (not a verdict); (c) classifier evidence-signal gaps and implementation spikes that should wait until ADRs exist.
- [ ] Cross-link: narrative note → catalog + per-target annotations; catalog → narrative sections; per-target annotations → catalog entries they cite.
- [ ] Self-audit pass across all seven written artifacts: every target has a completed coverage ledger; every v1 archetype has cited corpus evidence; every follow-up traces back to an observed region.
- [ ] Append a **closeout section** to this sprint file: which starting archetypes survived / merged / retired (with one-line reasons); total regions labeled per surviving archetype; ambiguities flagged vs. resolved vs. escalated; subagent dispatches and re-dispatches per large target.

## Sequencing

1. **Phase 0 → Phase 1** strictly linear. Without frozen artifact layout, annotation schema, and coverage ledgers, later phases produce incomparable notes.
2. **Phase 2 before Phase 3.** Subagent budget is expensive; dispatching against an un-pressured vocabulary wastes it. The catalog pressure check after miniflux is load-bearing, and the halt rule kicks in here if needed.
3. **Phase 3 gitea and mattermost may run in parallel** (they are independent) but within each target the loop is serial: dispatch → verify → re-dispatch on thin returns → synthesize. Do not parallelize synthesis.
4. **Phase 4 before Phase 5.** Deep-diving an archetype that the gates would retire is wasted effort.
5. **Phase 6 depends on Phase 5 outputs.** Boundary claims have to be grounded in per-archetype threshold work.
6. **Phase 7 last.** The narrative distills; it does not generate.

## Risks

| Risk | Mitigation |
|---|---|
| Vocabulary inflation — every surprising region births a new archetype. | Phase 4 gates applied explicitly with per-archetype pass/fail record; coverage gate is a hard filter; retirements kept as output. |
| Gitea / mattermost coverage quietly degrades into sampled reading. | Owned-directory bundles with file-count per bundle registered before dispatch; thin-return re-dispatch; parent spot-checks raw source. |
| Subagent returns are inconsistent across subsystems. | Frozen prompt template from Phase 0; required ten-field schema; re-dispatch when a bundle collapses distinct findings. |
| Extract-report exhaustiveness mistaken for target exhaustiveness. | Repo-level coverage ledger per target; bundles with no relevant surface require an explicit "no relevant archetype surface observed" note, not silence. |
| Annotations drift from extract-report reality because reports are stale. | Phase 1 records pinned SHA and report path per target; Phase 2 walks cite line-level locations, not prose summaries. |
| Large-target synthesis becomes concatenated subagent returns rather than a target-level story. | Each target-level annotation requires a synthesis at the top, written by the parent after reviewing all subagent returns, not by stitching them. |
| Research note becomes a de-facto ADR by accident. | Non-goal fence is explicit; follow-ups only *name* ADRs, never draft them; the self-audit pass checks. |
| Scope creep into "let me just add this evidence signal to the classifier and re-check." | Out-of-scope fence; tooling gaps become follow-ups, not in-sprint work, even when the fix looks small. |
| "No clean archetype" becomes a catch-all that hides refusal categories. | Annotation protocol distinguishes `terminal refusal` from `archetype unclear pending evidence X` from `hybrid archetype needing split`. No generic unclassified bucket allowed. |
| Phase 3 subagents invent new archetype names that contaminate the vocabulary. | Subagents *flag* candidates only; Phase 4 gates decide promotion. The frozen prompt template enforces the flag-don't-promote rule. |
| Retirements omitted because "it feels wasteful to delete work." | Retirements are *research output*; they prevent future re-drawing of the same map. Phase 4 explicitly records them in the catalog. |
| Archetype names collide with already-committed shape / state terminology from ADRs 15–18 or the v2 contract. | Phase 4 explicit naming-collision check before freezing the v1 vocabulary. |
| Gitea / mattermost subsystem bundles miss a cross-cutting concern (e.g., event distribution that cuts across multiple services). | Cross-target reconciliation step in Phase 3; dedicated `rg`-based discovery for mattermost fanout paths rather than inference from directory names. |

## Acceptance criteria

- [ ] `docs/research/archetype-catalog-v1.md` exists. Every v1 archetype has: definition, evidence signals cited to `docs/specs/liftability-properties.md` or `pkg/compiler/stateclass/`, ≤30-line Go emission sketch, auto-lift / suggest / refuse thresholds in concrete evidence conditions, ≥2 citations across ≥2 targets. Retirements are recorded with one-paragraph "why it didn't survive" notes.
- [ ] `docs/research/annotations/<target>.md` exists for all six targets. Every admitted region (ADMITTED) and every refusal (AUTO / SUGGEST / TERMINAL) has a label. Every AUTO entry names its archetype, transform, and candidate state class. Every subsystem bundle in the coverage ledger has either findings or an explicit "no relevant archetype surface observed" note with a reason.
- [ ] For gitea and mattermost, owned-directory bundles with file counts are recorded before dispatch, and every dispatch + re-dispatch is logged (subsystem, prompt version, return summary, re-dispatch reason). The target-level synthesis is parent-written, not stitched from subagent returns. Parent-agent spot-checks of raw source are acknowledged.
- [ ] Every v1 archetype has passed all four gates (coverage, evidence, emission, boundary) with per-gate outcomes recorded, or has an argued exception for the coverage gate.
- [ ] The ADR / v2-contract naming-collision check has been executed and any collisions resolved (renamed or merged).
- [ ] Every target's annotation distinguishes AUTO / SUGGEST / TERMINAL regions clearly, with the AUTO set surfaced explicitly in the target-level synthesis. No region sits in a generic unclassified bucket without a stated reason.
- [ ] The follow-up list's candidate state-class additions bucket is populated with concrete proposals: each entry names the archetype it enables, the evidence it requires, the transform it unlocks, and the targets where it earned its place.
- [ ] `docs/research/distribution-archetypes-v1.md` is a narrative note cross-linked to the catalog and per-target annotations. Cross-target matrix is present.
- [ ] `docs/research/distribution-archetypes-followups.md` exists and is split into three named buckets: ADRs ripe to draft (including `ADR-0019: archetype-driven remediation surface` and `ADR-0020: auto-lift evidence thresholds`), still-open empirical questions, classifier evidence-signal gaps / implementation spikes.
- [ ] Pragma bridge and remediation surface sketched per surviving archetype.
- [ ] "Compiler cannot know this statically" section present in the narrative note, separating threshold-tunable from irreducible evidence gaps.
- [ ] Closeout section in this sprint file records: starting archetypes → survived / merged / retired; regions labeled per surviving archetype; subagent dispatch/re-dispatch counts for gitea and mattermost; ambiguities flagged vs. resolved vs. escalated.
- [ ] No commits touch compiler, classifier, report schema, pragma, or harness code. Every code-adjacent finding is a follow-up.

## Open questions (for the research to characterize, not resolve)

Carried from the brief, restated so the sprint output is expected to sharpen these rather than answer them. Each appears in the follow-up list with the sprint's current best characterization.

- Is the auto-lift-vs-suggest boundary a single threshold, per-archetype, or structural (local + closed-form vs. runtime-dependent)?
- How much does the user-facing API of a lifted archetype need to change, and is that compiler-owned or user-owned?
- Are pragmas overrides only, or load-bearing evidence the compiler may rely on?
- Should "non-distributable" become an explicit archetype class, or stay as terminal refusal outside the vocabulary?

## Closeout (2026-04-22)

**Executed as three parallel independent runs** (opus, gpt-5.4, gemini) per user direction. Parallel-run design was chosen because research benefits from multiple independent passes where each model sees different things; the composite captures what one run alone would have missed.

**Run outcomes.** Opus produced the deepest walk (~77 KB across deliverables, 7 archetypes, structural two-axis boundary model, 5 evidence-signal proposals, 5 ADR drafts ripe). gpt-5.4 produced a concise, aggressively-merged 4-archetype vocabulary (~57 KB) with the strongest per-archetype-threshold framing and the composite `connection-hub-buffer` lens. Gemini required three attempts: run-1 sampled despite the explicit no-sampling fence; run-2 hit an MCP tool-infrastructure failure blocking delegation; run-3 (dispatched with `--allowed-mcp-server-names=none`) completed all 12 gitea+mattermost owned-directory bundles at terser per-bundle depth (~22 KB). Run-1 and run-2 preserved as source artifacts for transparency.

**Synthesis performed this session** (opus in this Claude Code session) — read all three runs' narratives, catalogs, followups, and per-target annotations; produced canonical composite artifacts at `docs/research/`.

**Canonical composite artifacts:**
- `docs/research/distribution-archetypes-v1.md` — merged narrative note
- `docs/research/archetype-catalog-v1.md` — 8 archetypes (opus's 7 + gemini's `filesystem-bound-singleton`); 6 retirements; 5 proposed evidence signals; cross-run attribution throughout
- `docs/research/annotations/README.md` + six per-target composite files — cross-run convergence/divergence with pointers into per-run depth
- `docs/research/distribution-archetypes-followups.md` — four buckets: A (8 state-class additions — primary engineering output); B (5 ADRs ripe to draft including ADR-0019/0020/0021/0022/0023); C (8 still-open empirical questions); D (8 implementation spikes waiting on ADRs)

**Archetype vocabulary that survived the four gates (v1):** `serialized-actor`, `bounded-worker-pool`, `periodic-invocation`, `keyed-partitioned-state`, `fanout-publisher`, `ttl-cache`, `session-affinity-state`, `filesystem-bound-singleton`. Retirements: `pipeline-stage`, `ephemeral-worker` (fissioned), `lifecycle-state-machine`, `websocket-fanout-hub`, `keyed-queue-state-guard`, `distributed-cache-wrapper`.

**AUTO surface measurement:** approximately **48 currently-refused regions** across the six evaluation targets would become auto-liftable if the classifier learned the eight archetypes. The TERMINAL class shrinks meaningfully; the research confirms that terminal refusal is a property of the classifier's vocabulary, not a stable property of the input program.

**Largest surfaced tension:** pragmas as load-bearing evidence vs. overrides (ADR-0021 territory). The research strongly leans toward both roles being needed with explicit separation; gpt-5.4 argues for evidence-only; opus argues both — the synthesis keeps both positions in the ADR-0021 draft scope.

**Recommended next-sprint sequencing:** ADR-0020 (thresholds) → ADR-0019 (remediation surface) → ADR-0021 (pragma roles) → first state-class landing (recommend `bounded-worker-pool` + `periodic-invocation` as highest-yield first pair) → ADR-0022 (composite archetypes) after the first single-archetype state class lands.

**Self-audit:** every Bucket-A state class cites ≥2 targets (A8 has an argued coverage exception); every Bucket-B ADR addresses a research-surfaced finding or tension; every Bucket-D spike waits on a specific ADR. No compiler, classifier, runtime, report-schema, or harness code was modified by this sprint.
