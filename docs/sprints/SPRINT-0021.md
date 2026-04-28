# SPRINT-0021 — ADR-0022 composite-archetype vertical slice on Mattermost Hub/WebConn

**Status:** planned
**Anchor ADRs:** ADR-0018 (frozen), ADR-0022 (the spec being implemented), ADR-0023 (cmd-inside-host emission, frozen).
**Predecessors:** SPRINT-0017 (candidate-set + `alternative_set` machinery on Caddy), SPRINT-0019 (cmd-inside-host emission), SPRINT-0020 (real-compiler/e2e-compile path on miniflux).

## Intent

Run the full ADR-0022 composite path end-to-end on a real Mattermost region. The Hub/WebConn region (`evaluation/mattermost/server/channels/app/platform/web_hub.go` + `web_conn.go`) was identified by SPRINT-0015 as the strongest PLOS §4.2 demo in the corpus and named in ADR-0022 as the canonical composite case (matches `keyed-partitioned-state`, `fanout-publisher`, and `session-affinity-state` simultaneously). This is the first time ADR-0022's compositional transform identity meets a real region.

The slice exercises:

- candidate-set construction with three component archetypes,
- subsumption + utility-tier fallback,
- compatible-refinement coherence check (codified, not prose),
- AND-rule eligibility inheritance,
- `archetype_kind: "composite"` on the report path,
- a four-layer e2e adapted to a stateful pub-sub region (workload exercises fanout to session-affine connections, not a pure function call).

## Win conditions

The sprint accepts iff one of three branches lands cleanly. None of them require relaxing a frozen contract.

- **(S) Successful lifted demo.** Classifier emits the composite on Hub/WebConn; frozen admission accepts the proposed boundary; the patcher emits a working `connection-hub-buffer` extracted-service via the cmd-inside-host emitter; the four-layer e2e runs against a lifted Mattermost and proves fanout-to-session-affine-conns end-to-end with counter delta + oracle equality + transcript parity + fail-closed/open.
- **(R) Report-only composite, with characterized blockers.** Classifier emits the composite, but admission refuses the boundary or the patcher API has no shape that fits. The report records `archetype_kind: "composite"` with `emittable: false`. Critically, this branch is **not** "admission said no, sprint over." It requires a refusal characterization that asks, for each refused property: *is this code fundamentally unliftable in any distributed-systems shape, or is our compiler's current understanding not yet sophisticated enough?* The classification is **fundamental** (genuine distribution-impossibility — e.g., requires shared-memory atomicity that no consensus protocol can preserve at acceptable cost, or assumes single-machine semantics that have no distributed analog) vs. **tooling-immaturity** (the shape has a known distributed analog — actors for goroutines, async messaging for channels, distributed locks/CRDTs/vector-clocks for sync primitives — but our admission rule, detector, or emitter doesn't yet recognize/handle it). Each refused property gets a classification + a paragraph naming what follow-up work would clear it. The baseline (un-lifted) workload still runs and its trace is pinned for a future sprint to diff against.
- **(C) Hard cliff before classification.** Compiler OOMs at Mattermost scale, the evidence model can't represent fanout/session evidence under frozen ADR-0018, the region boundary doesn't fit the classifier's region granularity, or some other contract collision. The sprint stops with a reproducible cliff doc + log capture + recorded RSS/wall-time.

A sprint that lands none of these is a process failure.

## On admission rules — read this before starting

**The goal of this sprint is not merely to get Mattermost working at all costs. The goal is to use Mattermost as a forcing function for making the compiler more sophisticated in a generalizable way.** Mattermost is a probe — it surfaces shapes in real OSS code that our current rules don't yet handle. Hopefully by doing so we can then successfully lift Mattermost. Any rule change motivated by this sprint must be one we'd want regardless of which target surfaced it; Mattermost-specific carve-outs, special cases, or hacks are exactly what "under discipline" excludes.

Admission rules in `pkg/compiler/transport/admission.go` are **not contracts**. They are our current best encoding of what we believe is liftable, and that belief is itself the research output of attempting lifts on real OSS code. This sprint is exactly that: an attempt that might surface incompleteness in the current rule.

If the Hub/WebConn boundary refuses, the question to ask is **not** "the rule said no, sprint over." The question is: **"is this code fundamentally unliftable, or is our compiler's current understanding not yet sophisticated enough?"** Long-running goroutines, channels, and sync primitives are not *fundamental* blockers to distribution — a goroutine is shaped like an actor, channels look like asynchronous network connections, sync primitives can be implemented in distributed form (consensus, distributed locks, vector clocks, CRDTs). They make distribution *trickier*, not impossible.

### The agent is empowered to change admission rules — under discipline

If, while characterizing a refusal, the agent forms a clear hypothesis that a current admission rule is overly conservative — and that hypothesis fits within the existing ADR framework (ADR-0017 liftability properties, ADR-0018 property taxonomy, ADR-0022 candidate sets / composite emission, ADR-0023 cmd-inside-host emission) — the agent **may** modify the rule inside this sprint. The discipline is:

1. **Document the hypothesis before changing anything.** Append a section to `docs/research/runs/SPRINT-0021-admission-characterization.md` (or `docs/decisions/0024-<name>.md` if it warrants a new ADR): which property is being changed, what the current encoding is, what the proposed encoding is, what distributed-systems shape justifies the change, which prior ADRs the change is consistent with, and what could go wrong (false positives — admitting things that genuinely shouldn't be lifted).
2. **Ground the change in prior ADRs.** A change that contradicts ADR-0018's taxonomy or ADR-0022's coherence rules is out of bounds without an explicit additive amendment to the relevant ADR. A change that *refines* an existing rule (e.g., narrowing what counts as evidence for an overly-broad refusal) is in bounds with documentation.
3. **No ad-hoc rule mutation to make Mattermost specifically pass.** The change must be principled and apply generally — it has to be the kind of refinement you'd want regardless of this particular target. If the only reason to make the change is "Mattermost would lift," it's not a research finding, it's a hack.
4. **All prior e2e (Caddy, miniflux, pocketbase, pragma) must still pass after the change.** Byte-identical goldens. If a rule refinement regresses prior targets, that's evidence the hypothesis was wrong; revert.
5. **If the hypothesis is weak or unclear, default to the (R) branch.** A characterization doc with deferred follow-up is a better outcome than a poorly-justified rule mutation.

The intent: the agent has the standing to act on its understanding of the research problem, but only when its understanding is articulated, defensible, and conservative enough not to break what already works. Empowerment without rigor isn't empowerment, it's noise.

## Boundaries this sprint will not cross

- `cmd/main.go` byte-identical (no exceptions; this is plumbing, not research).
- `evaluation/mattermost/` byte-identical pre/post compile. `make verify-evaluation-untouched` extended to cover Mattermost.
- `syscall.Flock` startup guard at `test/e2e/e2ecompile/main.go` preserved.
- SPRINT-0017 Caddy `alternative_set` goldens byte-identical after any rule changes. SPRINT-0019 + SPRINT-0020 e2e (caddy two-symbol + miniflux + pocketbase + pragma) pass unchanged after any rule changes.

### Boundaries that may move with documented hypothesis

- `pkg/compiler/transport/admission.go` — admission rules. Movable per the discipline in §"On admission rules" above.
- `pkg/compiler/transport/emit/liftpatch/` API — patcher signatures. Movable if the emission sketch surfaces a primitive that doesn't exist *and* the addition fits the existing API shape (additive, not breaking). Same discipline: document the hypothesis first.
- ADR-0018 property taxonomy. **Adding a new Layer-1 `liftability.PropertyID` is a high bar** — it requires an ADR-0018 additive amendment with rationale grounded in distributed-systems analog reasoning. If a new property is needed, write the amendment first, get the property-lint to accept it explicitly, then add the constant.

## Non-goals

- Do not modify transport admission *to make Mattermost specifically fit*. Principled refinements grounded in distributed-systems analog reasoning + prior ADRs are allowed per the discipline above; ad-hoc mutations to land a particular target are not. The test: would you make this same change if Mattermost weren't on your desk? If yes, document and proceed. If no, defer to (R).
- Do not add new ADR-0018 property IDs for fanout or session affinity.
- Do not change `liftpatch.PatchRequest` / `PatchResult` / `PatchSymbolBody`.
- Do not invent a new composite catalog identity beyond ADR-0022's `connection-hub-buffer` alias. Identity is the sorted component tuple; the alias is the report label.
- Do not create a generic receiver-state runtime if the Mattermost-specific sketch fails. Document the gap and stop.
- Do not modify Mattermost source in place.
- Do not synthesize a fake fail-mode or a pure-function workload. The e2e workload exercises websocket fanout to session-affine connections.
- Do not rewrite ADR-0022's decision text. Add additive addenda only.
- Do not pin a golden report before the region boundary is fixed (Block A.2).

## Anticipated cliffs

| # | Cliff | Symptom | Stop criterion |
|---|---|---|---|
| 1 | **Compiler OOM at Mattermost scale.** | `bin/e2e-compile` killed or exceeds 30 min on closure-only run. | Closure-only run with `MONOLIFT_PROFILE_DIR` enabled doesn't complete in **30 min wall / 16 GiB RSS**. Stop at A.1. Cliff doc + profile capture. |
| 2 | **Frozen evidence model can't represent fanout/session evidence.** Either archetype can't reach AUTO independently using only existing ADR-0018 properties. | Block A.4 archetype-recognition fixture fails on `fanout-publisher` or `session-affinity-state` after honest property-set selection. | Stop at A.4. Document the ADR-0022/ADR-0018 evidence-model gap. Do not invent properties. |
| 3 | **Region boundary doesn't fit classifier granularity.** Hub/WebConn spans two files + helper types; classifier may not scope a single region containing all three components. | A.2 enumeration shows the components scatter across multiple classifier regions. | Pin the region boundary explicitly; if the classifier can't represent it without code changes, that's the cliff. |
| 4 | **Admission refuses the Hub/WebConn boundary.** Long-running goroutines, websocket connections, channels, sync primitives are present in any plausible boundary. None of these are *fundamental* distribution blockers — they have known distributed analogs — but the current admission rule may not recognize them. | C.8 admission run records refused. | This is **expected**, not a hard stop. Characterize each refusal: fundamental (no distributed analog at acceptable cost) vs. tooling-immaturity (known distributed shape exists; our rule/detector/emitter doesn't handle it yet). For tooling-immaturity refusals, name the specific machinery a follow-up sprint would need. Sprint lands (R) with the characterization as the deliverable. |
| 5 | **Patcher API has no shape for the composite.** Even if admission accepts, the existing `PatchSymbolBody` may not express multi-replica adapters, sticky routing hints, or pub/sub bus declarations. | C.3 emission-sketch design surfaces `liftpatch/` primitives that don't exist. | Stop emission. Write `docs/research/runs/SPRINT-0021-emission-gap.md` listing missing primitives. Sprint lands branch (R). Do not extend the API. |
| 6 | **AND-rule kills eligibility.** `session-affinity-state` is unlikely to be `dynamic_delegate_eligible` (sticky routing breaks any-replica delegation). | Composite reports `dynamic_delegate_eligible: false`. | Not a stop. Assert it explicitly so the report is honest. Document in the addendum. |
| 7 | **Coherence predicate is hard to codify.** ADR-0022's "compatible refinement" is informal prose. | Codifying it as a checkable predicate over three archetypes surfaces cases the ADR didn't address. | Not a stop. Capture the predicate's formal shape — its surfacing is itself a contribution. |
| 8 | **Mattermost build is multi-stage and brittle.** Pre-bundled webapp assets, plugin manager, Elasticsearch, metrics. | A.3 baseline boot can't reach `/api/v4/users/me` 200. | Pin minimal config (`FileSettings.DriverName=local`, `EmailSettings.SendEmailNotifications=false`, no ES, `PluginSettings.Enable=false`, `RateLimitSettings.Enable=false`, `EmailSettings.RequireEmailVerification=false`). If still fails, stop at A.3. |
| 9 | **WS test flakiness.** Reconnect/dead-queue assertions are inherently racy. | Workload runs flake. | Use drain-then-reconnect protocol: workload waits for server ack of message N before triggering disconnect. |
| 10 | **`maphash` non-determinism in shard placement.** Hub-shard placement uses `maphash.Hash64` with a process-stable seed. | Workload picks userIDs that don't reliably split across shards. | Pre-compute shard assignments at workload-init time; don't monkey-patch the seed. |

## Sequencing

`A → B → C → D → E`, strict. Within blocks, the ordering is prescribed.

The Block-A OOM probe (Gate A.1) is the cheapest kill — run it first thing on day one before writing any new compiler code. Block B is pure unit-test work. Block C is the baseline workload + emission feasibility probe. Block D conditionally emits if C succeeds. Block E does four-layer verification with the right branch (S, R, or C).

### Block A — Reconnaissance, region pin, evidence audit

Goal: prove the real compiler can be pointed at Mattermost without OOM, pin the region boundary explicitly, and confirm the three component archetypes can be recognized in isolation under the frozen evidence model.

- [x] **A.1** Closure-only OOM probe. Read `evaluation/mattermost/server/go.mod`; capture exact module path. Add `packageDirFor` entry in `test/e2e/e2ecompile/main.go` mapping the Mattermost module path to `evaluation/mattermost/server`. Run `bin/e2e-compile` against Mattermost in **closure-only** mode (no emit, no apply) with `MONOLIFT_PROFILE_DIR` enabled and one process under the existing `syscall.Flock` guard. Capture closure size, peak RSS, wall time, CPU profile, heap profile.
  - Captured 2026-04-26 with module path `github.com/mattermost/mattermost/server/v8`, `GOWORK=$PWD/.tmp/sprint-0021-a1-go.work` to use the local `server/public` module, and target root `Hub` methods `Start,Broadcast,Register,Unregister,CheckConn`. Output: `.tmp/sprint-0021-a1-output/closure-report.json`; profiles: `.tmp/sprint-0021-a1-profiles/mattermost.cpu.pprof`, `.tmp/sprint-0021-a1-profiles/mattermost.heap.pprof`, `.tmp/sprint-0021-a1-profiles/mattermost.memstats.json`. Closure size: 2,956 included symbols / 4,838 excluded symbols. Wall time: 88.22s. Max RSS: 2,152,579,072 bytes. Peak memory footprint: 10,268,152,640 bytes. Runtime memstats: heap_alloc 4098.69 MiB, heap_sys 9650.81 MiB, sys 9841.48 MiB.
- [x] **A.1-gate** **Stop budget: 30 min wall / 16 GiB RSS.** If exceeded (Cliff 1), stop. Write `docs/research/runs/SPRINT-0021-oom.md` with profiles + transcript. Sprint lands branch (C).
  - Passed: 88.22s wall and 2,152,579,072-byte max RSS are below the stop budget.
- [x] **A.2** Pin the Hub/WebConn region boundary. Enumerate the exact symbol set in the region: `Hub`, `Hub.Start`, `Hub.Broadcast`, `Hub.Register`, `Hub.Unregister`, `hubConnectionIndex` + its methods (`Add`, `Remove`, `ForUser`, `ForChannel`), `WebConn`, `WebConn.send`, `WebConn.deadQueue`, `WebConn.Sequence`, `WebConn.connectionID`, `WebConn.writePump`, `CheckWebConn`, `PlatformService.GetHubForUserId`. Document the boundary in this sprint file. If A.1 produced a report, verify all of these are in `closure.includedSymbols`.
  - Boundary pinned: `Hub`; `(*Hub).Start`; `(*Hub).Broadcast`; `(*Hub).Register`; `(*Hub).Unregister`; `hubConnectionIndex`; `(*hubConnectionIndex).Add`; `(*hubConnectionIndex).Remove`; `(*hubConnectionIndex).ForUser`; `(*hubConnectionIndex).ForChannel`; `WebConn`; `WebConn.send`; `WebConn.deadQueue`; `WebConn.Sequence`; `WebConn.connectionID`; `(*WebConn).writePump`; `(*PlatformService).CheckWebConn` / `(*Hub).CheckConn`; `(*PlatformService).GetHubForUserId`.
  - A.1 closure verification: present in `closure.includedSymbols`: `Hub`, `(*Hub).Start`, `(*Hub).Broadcast`, `(*Hub).Register`, `(*Hub).Unregister`, `hubConnectionIndex`, `(*hubConnectionIndex).Add`, `(*hubConnectionIndex).Remove`, `(*hubConnectionIndex).ForUser`, `(*hubConnectionIndex).ForChannel`, `WebConn`, `(*PlatformService).GetHubForUserId`. Missing: `(*WebConn).writePump`. Field-level members `send`, `deadQueue`, `Sequence`, and `connectionID` are not represented as standalone closure symbols. Cliff documented in `docs/research/runs/SPRINT-0021-region-granularity.md`.
- [ ] **A.3** Refresh `test/e2e/targets/mattermost/target.go`: drop the stale `SkipReason: "deferred to SPRINT-0005"`. Replace stale `ExpectedRoot: "UserService"` with the Hub-rooted symbol pinned in A.2 (likely `Hub.Broadcast` or `PlatformService.GetHubForUserId` — pick after closure inspection). Leave the target skipped in CI until E gates pass.
- [ ] **A.4** Audit current `pkg/compiler/stateclass` evidence facts on the Hub/WebConn region. List which facts already exist for user-keyed routing, connection index maps, hub channels, and per-connection state. Decide whether `fanout-publisher` and `session-affinity-state` evidence can be represented as stateclass-internal structural evidence under the frozen no-new-Layer-1-property rule.
- [ ] **A.4-gate** If the evidence model cannot express independent AUTO matches for `fanout-publisher` and `session-affinity-state` without ADR-0018 changes (Cliff 2), stop. Document the evidence-model gap. Sprint lands branch (C).
- [ ] **A.5** Add `ArchetypeFanoutPublisher` and `ArchetypeSessionAffinityState` to `pkg/compiler/stateclass/archetype.go` registry, with required-property sets drawn from existing ADR-0018 properties only. Document each chosen property and why it's evidence of fanout / sticky-conn-state.
- [ ] **A.6** Update `harvestSeeds` in `stateclass.go` analysis pass to detect `fanout-publisher` (channel loops, hub broadcast send-to-all-conns shape) and `session-affinity-state` (WebConn session fields, sticky-routing seq/deadQueue shape) evidence on the Mattermost fixture.
- [ ] **A.7** Update `topologyTierPriority` in `tiers.go` with values for the two new archetypes.
- [ ] **A.8** Mattermost-specific stateclass fixtures under `pkg/compiler/stateclass/testdata/fixtures/`: model `Hub` register/unregister, user-keyed connection indexes, broadcast-to-connection send channels, and per-connection write state. Three fixtures, one per component archetype.
- [ ] **A.9** Property-lint gate: assert no new `liftability.PropertyID` constants were introduced. Mechanical check; fail the sprint test suite if violated.
- [ ] **A.10** Unit tests in `pkg/compiler/stateclass/archetype_test.go` cover the new archetype registry entries (presence, required-property sets, ID stability). All three archetypes recognized independently on the fixtures.
- [ ] **A.11** Closure-pin assertions on the regenerated Mattermost report (or against fixtures if A.1 OOMed but A.4 surfaced a different cliff): the closure includes the symbol set from A.2.
- [ ] **A.12** Run `make verify-evaluation-untouched` — extend it to cover Mattermost. Confirm `evaluation/mattermost/` byte-identical after A.1.

**Block A gate:** A.1 closure-only run completed under budget; region boundary pinned; three archetypes recognized independently on the fixtures; no new Layer-1 properties; SPRINT-0017 Caddy `alternative_set` goldens still byte-identical. If any fails, sprint lands (C) at the failing gate.

### Block B — Composite catalog, coherence, AND-rule

Goal: candidate-set extension produces the composite tuple, coherence predicate gates it, AND-rule computes eligibility, primary selection emits `archetype_kind: "composite"`. Unit-tested in isolation, no e2e yet.

- [ ] **B.1** Create `pkg/compiler/stateclass/composites.go` with a typed `Composite` struct: `Components []ArchetypeID`, `Alias string`, `CoherenceCheck func(...) bool`. Component order is canonical (sorted by `ArchetypeID`) so identity and reports are deterministic.
- [ ] **B.2** Register the `{fanout-publisher, keyed-partitioned-state, session-affinity-state}` composite (sorted) in the catalog with alias `"connection-hub-buffer"`.
- [ ] **B.3** Create `pkg/compiler/stateclass/coherence.go` codifying ADR-0022's "compatible refinement" as a predicate. Initial predicate: the three components must refine **disjoint axes** (ownership / routing / delivery) **and** must agree on the keying dimension — the same key (userID/connectionID) drives partition placement, sticky routing, and fanout-recipient selection.
- [ ] **B.4** Codify the keying-agreement check: the property carrying the partition key must be the same `PropertyID` across all three components' evidence. If ADR-0018 doesn't expose a key-identity property, surface that as a contract gap and stop. Do not invent a property to paper over it.
- [ ] **B.5** Unit-test the coherence predicate: at least three positive cases (the canonical composite under variation) and four negative cases — (a) two of the three components only, (b) mismatched key dimension, (c) same axis claimed twice, (d) `serialized-actor + keyed-partitioned-state` (the SPRINT-0017 Caddy case) — explicitly does not produce `connection-hub-buffer`.
- [ ] **B.6** Implement `ExtendWithComposites` at the SPRINT-0017 seam in `pkg/compiler/stateclass/candidates.go`. Iterate registered composites; for each, check all components are present in the base candidate set and the coherence predicate holds.
- [ ] **B.7** Composite subsumption: composite is preferred over its components when present; the components remain in `Alternatives` with non-empty rationales (mirrors SPRINT-0017's `alternative_set` retention rule).
- [ ] **B.8** AND-rule eligibility in `selection.go`: composite is `dynamic_delegate_eligible` iff every component is. Same for `runtime_selectable` and `emittable`.
- [ ] **B.9** Unit-test: composite eligibility is `false` when at least one component is ineligible. Specifically, `session-affinity-state` is **not** `dynamic_delegate_eligible` (sticky routing breaks any-replica dispatch); the expected outcome on Hub/WebConn is `dynamic_delegate_eligible: false`. Assert this explicitly.
- [ ] **B.10** Wire `archetypeKindForOutcome` in `selection.go` to return `"composite"` when the selected primary is a composite. Replace the `// SPRINT-0018: composite kind set here.` placeholder.
- [ ] **B.11** Populate `Primary.ContributingArchetypes` (sorted) and `Primary.Alias = "connection-hub-buffer"` on the report.
- [ ] **B.12** Confirm `pkg/compiler/reportv2/schema.json` enum already includes `"composite"`; add a schema-validation test against a hand-rolled composite report fixture.
- [ ] **B.13** Negative composite tests on non-Mattermost fixtures: any two of the three component archetypes must not produce `connection-hub-buffer`; Caddy `Handler.connections` still produces `alternative_set` with `serialized-actor` primary (byte-identical against pre-sprint goldens).
- [ ] **B.14** Subsumption regression: SPRINT-0017 Caddy `alternative_set` goldens byte-identical against pre-sprint.

**Block B gate:** all unit tests pass; SPRINT-0017 Caddy goldens byte-identical; coherence predicate codified (not prose). If any fails, sprint lands (C) with the failing predicate or contract gap as the deliverable.

### Block C — Mattermost baseline, baseline workload, admission probe

Goal: bring up un-lifted Mattermost in kind; run the stateful pub-sub workload against the baseline; pin a reference event trace; run frozen admission against the proposed Hub/WebConn boundary and record admitted/refused result.

- [ ] **C.1** Mattermost baseline manifests under `test/e2e/targets/mattermost/baseline/` reusing `test/e2e/fixtures/postgres.yaml`. Mattermost Dockerfile or host build spec that builds from `evaluation/mattermost` matching the repo's existing e2e image-builder conventions. Multi-stage build expected (pre-bundled webapp assets if needed).
- [ ] **C.2** Configure Mattermost minimal config: `FileSettings.DriverName=local`, `EmailSettings.SendEmailNotifications=false`, `EmailSettings.RequireEmailVerification=false`, no Elasticsearch, `PluginSettings.Enable=false`, `RateLimitSettings.Enable=false`. Pin in `baseline/configmap.yaml`.
- [ ] **C.3** Phased readiness: Postgres → migrations → Mattermost server → `/api/v4/system/ping` 200 → `/api/v4/users/me` 200 with bootstrap admin token (real readiness — DB+session+auth, not just HTTP listener). Hard cap 8 min on the readiness loop.
- [ ] **C.3-gate** If the server can't reach `/api/v4/users/me` 200 with the minimal config (Cliff 8), stop. Sprint lands (C).
- [ ] **C.4** Implement `test/e2e/targets/mattermost/workload/` (Go, not bash). Workload setup pre-computes shard assignments offline with a fixed `maphash` seed: pick userIDs (M=4) such that `maphash.Hash64(userID) % NumCPU` lands them on at least two different hub shards. Pin them in the fixture (Cliff 10 mitigation).
- [ ] **C.5** Workload action:
    - Open N=8 WebSocket clients across M=4 users (multiple conns per user → exercises fanout-to-multi-conn).
    - Wait for Mattermost hello/connected event on each conn.
    - Post N×K messages via REST → triggers `Hub.Broadcast`.
    - Verify each message is received by every subscribed conn **exactly once** (fanout-publisher assertion).
    - On a configurable subset, drop the WS connection mid-stream and reconnect with the same `connectionID` + last seqNum; assert `deadQueue` replay returns the missed events (session-affinity-state assertion).
- [ ] **C.6** Drain-then-reconnect protocol on the reconnect/replay subset: workload waits for server ack of message N before triggering disconnect (Cliff 9 mitigation).
- [ ] **C.7** Workload runs against the un-lifted baseline only in this block. Capture reference event trace at `test/e2e/targets/mattermost/golden/workload-trace.json` (event counts per user / per shard / replay counts). Pin it.
- [ ] **C.8** Admission probe: run `pkg/compiler/transport/admission.go` against the proposed Hub/WebConn lift boundary. Capture exact admitted/refused result with **per-property** rationale — for each of the six boundary properties + `lifecycle.execution-profile`, record Hold / Violate / NoEvidence + the specific evidence (or lack thereof) that drove the verdict. Do not modify admission in this sprint.
- [ ] **C.9** **Refusal characterization** (only if any property refused). For each refused property, write a structured entry in `docs/research/runs/SPRINT-0021-admission-characterization.md`. Approach the question as a researcher, not a rule-follower: ignore the current rule's verdict for a moment and ask whether the code shape it refused is *fundamentally* unliftable. Each entry contains:
    - **Property ID and verdict.**
    - **The specific Hub/WebConn code shape that triggered refusal** — exact symbols, control-flow features, state shapes. Be concrete: which goroutine, which channel, which mutex.
    - **Distribution-feasibility analysis.** Set aside the current admission rule. Reason from first principles: is there *any* distributed-systems shape under which this code's semantics could be preserved? Goroutines map to actors / per-shard workers. Channels map to async message passing or persistent queues. Mutexes map to distributed locks, CRDTs, vector-clock-ordered updates, or single-writer per-shard ownership. Long-running loops map to durable workers with checkpointed state. Document the analog you'd reach for, what its semantic gap to the original would be (latency, ordering, failure modes), and whether that gap is acceptable for *this* region's purpose.
    - **Classification:**
      - **fundamental** — no distributed analog preserves the semantics at acceptable cost (e.g., the code requires shared-memory atomicity that no consensus protocol can preserve in the latency budget the workload needs; the code is doing something that's intrinsically about a single failure domain).
      - **tooling-immaturity** — the distributed analog exists and is well-understood; the gap is that our admission rule, detector, or emitter doesn't yet recognize the shape or doesn't yet have the machinery to emit the analog. *Most refusals on a server like Mattermost are likely this category.*
    - **Follow-up sketch** — for tooling-immaturity refusals, name the specific machinery (a new property describing the shape, a detector that recognizes the actor pattern, an emitter that produces the distributed-lock/CRDT/queue infrastructure, etc.). For fundamental refusals, note whether the region's boundary could be redrawn to dodge the violation, or whether the region is just genuinely local.
- [ ] **C.9.summary** At the end of the characterization doc, write a synthesis paragraph: across the refused properties, what is the **dominant shape** of the gap? Is it admission-rule encoding, detector precision, emitter machinery, ADR-0018 property taxonomy, or some combination? This synthesis is the load-bearing input to whatever sprint comes next.
- [ ] **C.9.action** **In-sprint rule refinement (optional, gated).** If C.9 surfaced a tooling-immaturity refusal where the agent has formed a clear hypothesis grounded in prior ADRs (ADR-0017/0018/0022/0023), the agent may attempt the refinement inside this sprint. Required sequence: (a) write the hypothesis doc *before* touching code — what's changing, why, which ADRs justify it, what could break; (b) make the change; (c) re-run admission and full prior e2e (Caddy, miniflux, pocketbase, pragma) — all goldens byte-identical; (d) re-run C.8 admission probe on Hub/WebConn; (e) if Hub/WebConn now admits, proceed to Block D (S branch); (f) if it still refuses or if any prior target regresses, revert the change and stay in (R). Default behavior if no clear hypothesis: skip C.9.action; (R) is the right answer.
- [ ] **C.10** If admission **accepts** every property: proceed to Block D (S branch attempt).
- [ ] **C.11** If admission **refuses** any property (Cliff 4 — expected outcome): mark composite candidate `emittable: false` with the C.9 characterization referenced from the report path. Skip Block D. Sprint lands (R).

**Block C gate:** baseline boot reaches `/api/v4/users/me` 200; baseline workload runs green with N=8 conns / M=4 users / shard-split / fanout-exactly-once / dead-queue replay; reference trace pinned; admission probe completed with per-property documented result; if any refused, characterization doc exists with tackleability classification for each refused property.

### Block D — Composite emission attempt (only if C.9)

Skipped if C.10 fired (admission refused).

- [ ] **D.1** Write the concrete `connection-hub-buffer` emission sketch as a design note appended to this sprint file: sharded hub replicas keyed by userID hash, pub/sub bus shape (NATS / Redis Streams / Postgres LISTEN — pick one), per-conn state co-located with the routed replica, patcher payload shape.
- [ ] **D.2** Confirm the sketch fits the frozen `liftpatch/` API. Enumerate every `PatchRequest` / `PatchResult` field needed.
- [ ] **D.2-gate** If the sketch does **not** fit the frozen API (Cliff 5): write `docs/research/runs/SPRINT-0021-emission-gap.md` enumerating exactly which `liftpatch/` primitives are missing (e.g. multi-replica adapters, sticky-routing hint, pub/sub bus declaration). Skip remaining D tasks. Sprint lands (R).
- [ ] **D.3** Add Mattermost emit contexts in `pkg/compiler/extract_transport.go` for the composite (no admission/patcher API changes).
- [ ] **D.4** Extend `test/e2e/e2ecompile/main.go` lifted-tree materialization for Mattermost: emit `<output>/lifted/host-patch/server/` with patched Hub/WebConn region, `cmd/monolift-extracted-connection-hub-buffer/main.go`, `cmd/monolift-oracle-connection-hub-buffer/main.go`, Dockerfiles, manifests.
- [ ] **D.5** Static recursion-safety assertion: extracted-service and oracle Deployment YAMLs grep-clean for `MONOLIFT_LIFT_[A-Z_]+:` in env blocks.
- [ ] **D.6** e2e-compile integration tests: rendered extracted/oracle binaries import and call real Mattermost symbols (no synthetic mirrors); `evaluation/mattermost/` byte-identical post-emit.
- [ ] **D.7** Build host, extracted-service, and oracle from `lifted/host-patch/server/`; assert `go build` succeeds for all three.

**Block D gate:** lifted tree emitted; all three commands build; `evaluation/mattermost/` byte-identical. If sketch doesn't fit (D.2-gate), sprint lands (R) with the gap doc as deliverable.

### Block E — Four-layer verification

Goal: assertions branch on `emittable`. The test suite is honest in either world.

- [ ] **E.1 — Layer 1 (evidence).** Assert no new ADR-0018 properties added. Property-lint passes.
- [ ] **E.2 — Layer 2 (catalog).** Assert the Mattermost atomic candidate set is exactly `{fanout-publisher, keyed-partitioned-state, session-affinity-state}` (sorted) before composite insertion.
- [ ] **E.3 — Layer 3 (composite + report).** Assert the composite candidate is primary, `archetype_kind: "composite"`, alias `"connection-hub-buffer"`, contributing archetypes sorted, alternatives present with non-empty rationales, `dynamic_delegate_eligible: false` (per AND-rule).
- [ ] **E.4 — Layer 4, branch (S).** If lifted artifacts exist: deploy lifted Mattermost + extracted-service + oracle against Postgres baseline; run the C.5 workload; per-request `/calls` delta `>= 1`; aggregate `<= 50`; oracle equality on every `/invocations` record; transcript parity vs. the C.7 reference trace; recursion-safety runtime test (direct POST to extracted `/invoke` with no lift env increments `/calls` exactly once); fail-closed test (extracted scaled to 0 → expected degraded signal); fail-open test (`MONOLIFT_LIFT_FAILMODE=open`, extracted scaled to 0, workload succeeds, `/calls` stays 0).
- [ ] **E.5 — Layer 4, branch (R).** If `emittable: false`: assert no lifted artifacts emitted; the report records `emittable: false`; the C.7 baseline workload trace remains valid; **and** if the cause was admission refusal, the C.9 characterization doc exists with one entry per refused property, each entry having a tackleability classification (architecturally-necessary or conservative-encoding) and a follow-up sketch.
- [ ] **E.6** Run `go test ./pkg/compiler/stateclass/...`.
- [ ] **E.7** Run `go test ./pkg/compiler/reportv2/...`.
- [ ] **E.8** Run `go test ./test/e2e/e2ecompile/...` serially.
- [ ] **E.9** If lifted: `MONOLIFT_E2E=1 go test -tags=e2e ./test/e2e -run TestE2E/mattermost -count=1 -timeout 45m` serially.
- [ ] **E.10** Full-matrix regression: `MONOLIFT_E2E=1 go test -tags=e2e ./test/e2e -timeout 60m -count=1`. Caddy + miniflux + pocketbase + pragma + mattermost (whatever branch). SPRINT-0017/0019/0020 e2e byte-identical to pre-sprint.

**Block E gate:** assertions for the active branch (S or R) all pass; full-matrix regression green.

### Block F — Documentation, ADR addenda, ledger

- [ ] **F.1** Sprint closeout section in this file: which branch (S, R, C); what was lifted, what was admission-refused, what was the cliff if (C). Compile-time wallclock + peak RSS recorded regardless of branch.
- [ ] **F.2** ADR-0022 **addendum** (additive — do not rewrite): codified coherence predicate, AND-rule formalization, and either the emission sketch (S) or captured gap (R). Per project convention (memory: decision-log convention), preserve original ADR text.
- [ ] **F.3** ADR-0023 **addendum** (additive) only if a working emitted Mattermost path adds a reusable sidecar/oracle pattern (S branch only).
- [ ] **F.4** Do not amend ADR-0018.
- [ ] **F.5** Update `docs/evolution.md` narrative entry.
- [ ] **F.6** Create `docs/evaluation/targets/03-mattermost.md` with: pinned region symbol set, candidate-set evidence, branch outcome, compile-time metrics, rejected sketch shapes (if R), or working emission shape (if S).
- [ ] **F.7** Update `docs/sprints/ledger.yaml` status (`done` for S/R; `cliff-blocked` for C) at closeout.
- [ ] **F.8** `test/e2e/targets/mattermost/target.go` to its final form: non-skipped if S; skipped with `SkipReason` pointing to the cliff/gap doc if R or C.
- [ ] **F.9** Verify `cmd/main.go`, ADR-0018, admission rule, patcher API unchanged via `git diff` against sprint base.

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| Concurrent `bin/e2e-compile` invocations OOM-kill (SPRINT-0019 lesson). | `syscall.Flock` startup guard preserved; one process at a time. |
| `evaluation/mattermost/` byte-identical regression from stray dialer write. | A.12 + D.6 enforce. |
| `maphash` non-determinism breaks shard-split workload. | C.4 pre-computes assignments offline with fixed seed. |
| WS reconnect/replay test flakiness. | C.6 drain-then-reconnect protocol. |
| Mattermost dependency closure pulls plugin manager / Elasticsearch / metrics. | Pick entry symbol carefully (A.2/A.3); verify via `go list` before A.1. |
| Composite alias becomes identity-bearing by accident. | B.1 fixes identity to sorted component tuple; `Alias` is a separate string field. |
| Region boundary drifts during Block B fixture work. | A.2 pins the boundary in the sprint file before B starts. |
| Admission accepts a partial boundary that doesn't include the actual fanout/session work. | C.8 captures admitted/refused for the *complete* boundary; partial admittance is a refusal for sprint purposes. |
| Schema-validation test churns when composite fields change. | Hand-rolled fixture in B.12 is the source of truth; schema is the contract. |

## Acceptance criteria

The sprint accepts iff one of (S), (R), or (C) holds and all gates for that branch pass.

### (S) — Successful lifted demo
- All Block A, B, C, D, E gates pass. Branch (S) of E.4.
- Classifier emits `archetype_kind: "composite"`, alias `"connection-hub-buffer"`, contributing archetypes `{fanout-publisher, keyed-partitioned-state, session-affinity-state}` (sorted), `dynamic_delegate_eligible: false`.
- `lifted/host-patch/server/` contains patched Hub/WebConn region + extracted-service cmd + oracle cmd. `evaluation/mattermost/` byte-identical.
- Lifted Mattermost passes all four e2e layers including fanout-to-multi-conn exactly-once + dead-queue replay + counter delta + oracle equality + transcript parity + fail-closed + fail-open.
- ADR-0022 + ADR-0023 addenda landed; `cmd/main.go` unchanged.
- If admission rules, the patcher API, or ADR-0018 properties were modified to land (S): the hypothesis doc justifying the change is committed *before* the change, the change is principled (would have been made absent Mattermost), the relevant ADR has its additive amendment, and SPRINT-0017/0019/0020 e2e is byte-identical to pre-sprint goldens after the change.

### (R) — Report-only composite, with characterized blockers
- All Block A, B, C, E gates pass. Branch (R) of E.5. Block D either skipped (admission refused at C.11) or aborted at D.2-gate (sketch doesn't fit API).
- Report records `archetype_kind: "composite"`, alias, contributing archetypes, `emittable: false`.
- C.7 baseline workload trace pinned.
- ADR-0022 addendum landed (codified coherence + captured gap). No frozen contract modified *in this sprint*.
- **If admission refused:** `docs/research/runs/SPRINT-0021-admission-characterization.md` exists with one entry per refused property containing verdict, triggering code shape, distribution-feasibility analysis (what distributed analog would preserve the semantics, and at what cost), fundamental-vs-tooling-immaturity classification, follow-up sketch — plus a synthesis paragraph naming the dominant shape of the gap. This document is the load-bearing deliverable of the (R) branch: it tells a follow-up sprint which refusals are real walls and which are paper walls our compiler hasn't learned to walk through yet.
- **If sketch doesn't fit API:** `docs/research/runs/SPRINT-0021-emission-gap.md` enumerates missing `liftpatch/` primitives.

### (C) — Hard cliff
- Stop point identified at A.1 (OOM), A.4 (evidence model), A.2 (region granularity), or C.3 (baseline boot).
- Cliff doc at `docs/research/runs/SPRINT-0021-<cliff>.md` records reproduction steps + resource numbers (RSS, wallclock, log excerpts).
- All other targets (caddy, miniflux, pocketbase, pragma) still pass — the cliff did not regress anything.
- Compile-time metrics recorded as far as the run progressed.
- `test/e2e/targets/mattermost/target.go` `SkipReason` points to the cliff doc.
- Sprint ledger marked `cliff-blocked`.

A sprint that lands none of (S), (R), or (C) cleanly is a process failure.

## Blockers

- **A.2 region granularity cliff (branch C).** The intended Hub/WebConn composite boundary is multi-root: Hub fanout/index behavior is reachable from the synthetic `Hub` root, while per-connection replay/write state is rooted at `WebConn.Pump` / `WebConn.writePump`. The current compiler accepts one root pragma and emits one closure/report root, so it cannot represent the complete boundary without a new multi-root region model. A.1 did not OOM, but A.2 verification found `(*WebConn).writePump` missing from `closure.includedSymbols`, and field-level members `send`, `deadQueue`, `Sequence`, and `connectionID` are not standalone closure symbols. Repro and metrics are in `docs/research/runs/SPRINT-0021-region-granularity.md`.
